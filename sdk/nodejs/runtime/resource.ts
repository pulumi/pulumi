// Copyright 2016-2021, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as grpc from "@grpc/grpc-js";
import * as query from "@pulumi/query";
import * as log from "../log";
import * as utils from "../utils";

import { getAllResources, Input, Inputs, Output, output } from "../output";
import { ResolvedResource } from "../queryable";
import {
    Alias,
    allAliases,
    ComponentResource,
    ComponentResourceOptions,
    createUrn,
    CustomResource,
    CustomResourceOptions,
    expandProviders,
    ID,
    pkgFromType,
    ProviderResource,
    Resource,
    ResourceOptions,
    URN,
} from "../resource";
import { gatherExplicitDependencies, getAllTransitivelyReferencedResourceURNs } from "./dependsOn";
import { debuggablePromise, debugPromiseLeaks } from "./debuggable";
import { invoke } from "./invoke";
import { getStore } from "./state";

import { isGrpcError } from "../errors";
import {
    deserializeProperties,
    deserializeProperty,
    OutputResolvers,
    resolveProperties,
    serializeProperties,
    serializeProperty,
    serializeResourceProperties,
    suppressUnhandledGrpcRejections,
    transferProperties,
} from "./rpc";
import {
    awaitStackRegistrations,
    excessiveDebugOutput,
    getCallbacks,
    getMonitor,
    getStack,
    isDryRun,
    isLegacyApplyEnabled,
    rpcKeepAlive,
    serialize,
    terminateRpcs,
} from "./settings";

import * as gempty from "google-protobuf/google/protobuf/empty_pb";
import * as gstruct from "google-protobuf/google/protobuf/struct_pb";
import * as aliasproto from "../proto/alias_pb";
import { Callback } from "../proto/callback_pb";
import * as provproto from "../proto/provider_pb";
import * as resproto from "../proto/resource_pb";
import * as sourceproto from "../proto/source_pb";

export interface SourcePosition {
    uri: string;
    line: number;
    column: number;
}

function marshalSourcePosition(sourcePosition?: SourcePosition) {
    if (sourcePosition === undefined) {
        return undefined;
    }
    const pos = new sourceproto.SourcePosition();
    pos.setUri(sourcePosition.uri);
    pos.setLine(sourcePosition.line);
    pos.setColumn(sourcePosition.column);
    return pos;
}

interface ResourceResolverOperation {
    /**
     * A resolver for a resource's URN.
     */
    resolveURN: (urn: URN, err?: Error) => void;

    /**
     * A resolver for a resource's ID (for custom resources only).
     */
    resolveID: ((v: ID, performApply: boolean, err?: Error) => void) | undefined;

    /**
     * A collection of resolvers for a resource's properties.
     */
    resolvers: OutputResolvers;

    /**
     * A fully-resolved parent URN, if any.
     */
    parentURN: URN | undefined;

    /**
     * A fully-resolved provider reference, if any.
     */
    providerRef: string | undefined;

    /**
     * A map of fully-resolved provider references, if any.
     */
    providerRefs: Map<string, string>;

    /**
     * All serialized properties, fully awaited, serialized, and ready to go.
     */
    serializedProps: Record<string, any>;

    /**
     * A set of URNs that this resource is directly dependent upon. These will
     * all be URNs of custom resources, not component resources.
     */
    allDirectDependencyURNs: Set<URN>;

    /**
     * A set of URNs that this resource is directly dependent upon, keyed by the
     * property that causes the dependency. All URNs in this map must exist in
     * {@link allDirectDependencyURNs}. These will all be URNs of custom
     * resources, not component resources.
     */
    propertyToDirectDependencyURNs: Map<string, Set<URN>>;

    /**
     * A list of aliases applied to this resource.
     */
    aliases: (Alias | URN)[];

    /**
     * An ID to import, if any.
     */
    import: ID | undefined;

    /**
     * Any important feature support from the monitor.
     */
    monitorSupportsStructuredAliases: boolean;

    /**
     * If set, the provider's `Delete` method will not be called for this
     * resource if the URN specified is being deleted as well.
     */
    deletedWithURN: URN | undefined;
}

/**
 * Get an existing resource's state from the engine.
 */
export function getResource(
    res: Resource,
    parent: Resource | undefined,
    props: Inputs,
    custom: boolean,
    urn: string,
): void {
    // Extract the resource type from the URN.
    const urnParts = urn.split("::");
    const qualifiedType = urnParts[2];
    const urnName = urnParts[3];
    const type = qualifiedType.split("$").pop()!;

    const label = `resource:urn=${urn}`;
    log.debug(`Getting resource: urn=${urn}`);

    // Wait for all values to be available, and then perform the RPC.
    const done = rpcKeepAlive();

    const monitor = getMonitor();
    const resopAsync = prepareResource(label, res, parent, custom, false, props, {});

    const preallocError = new Error();
    debuggablePromise(
        resopAsync
            .then(async (resop) => {
                const inputs = await serializeProperties(label, { urn });

                const req = new resproto.ResourceInvokeRequest();
                req.setTok("pulumi:pulumi:getResource");
                req.setArgs(gstruct.Struct.fromJavaScript(inputs));
                req.setProvider("");
                req.setVersion("");
                req.setAcceptresources(!utils.disableResourceReferences);

                // Now run the operation, serializing the invocation if necessary.
                const opLabel = `monitor.getResource(${label})`;
                runAsyncResourceOp(opLabel, async () => {
                    let resp: any = {};
                    let err: Error | undefined;
                    try {
                        if (monitor) {
                            resp = await debuggablePromise(
                                new Promise((resolve, reject) =>
                                    monitor.invoke(
                                        req,
                                        (
                                            rpcError: grpc.ServiceError | null,
                                            innerResponse: provproto.InvokeResponse | undefined,
                                        ) => {
                                            log.debug(
                                                `getResource Invoke RPC finished: err: ${rpcError}, resp: ${innerResponse}`,
                                            );
                                            if (rpcError) {
                                                if (
                                                    rpcError.code === grpc.status.UNAVAILABLE ||
                                                    rpcError.code === grpc.status.CANCELLED
                                                ) {
                                                    err = rpcError;
                                                    terminateRpcs();
                                                    rpcError.message = "Resource monitor is terminating";
                                                    (<any>preallocError).code = rpcError.code;
                                                }

                                                preallocError.message = `failed to get resource:urn=${urn}: ${rpcError.message}`;
                                                reject(new Error(rpcError.details));
                                            } else {
                                                resolve(innerResponse);
                                            }
                                        },
                                    ),
                                ),
                                opLabel,
                            );

                            // If the invoke failed, raise an error
                            const failures = resp.getFailuresList();
                            if (failures?.length) {
                                let reasons = "";
                                for (let i = 0; i < failures.length; i++) {
                                    if (reasons !== "") {
                                        reasons += "; ";
                                    }
                                    reasons += `${failures[i].getReason()} (${failures[i].getProperty()})`;
                                }
                                throw new Error(`getResource Invoke failed: ${reasons}`);
                            }

                            // Otherwise, return the response.
                            const m = resp.getReturn().getFieldsMap();
                            resp = {
                                urn: m.get("urn").toJavaScript(),
                                id: m.get("id").toJavaScript() || undefined,
                                state: m.get("state").getStructValue(),
                            };
                        }
                    } catch (e) {
                        err = e;
                        resp = {
                            urn: "",
                            id: undefined,
                            state: undefined,
                        };
                    }

                    resop.resolveURN(resp.urn, err);

                    // Note: 'id || undefined' is intentional.  We intentionally collapse falsy values to
                    // undefined so that later parts of our system don't have to deal with values like 'null'.
                    if (resop.resolveID) {
                        const id = resp.id || undefined;
                        resop.resolveID(id, id !== undefined, err);
                    }

                    await resolveOutputs(res, type, urnName, props, resp.state, {}, resop.resolvers, err);
                    done();
                });
            })
            .catch((err) => {
                done();
                throw err;
            }),
        label,
    );
}

/**
 * Reads an existing custom resource's state from the resource monitor.  Note
 * that resources read in this way will not be part of the resulting stack's
 * state, as they are presumed to belong to another.
 */
export function readResource(
    res: Resource,
    parent: Resource | undefined,
    t: string,
    name: string,
    props: Inputs,
    opts: ResourceOptions,
    sourcePosition?: SourcePosition,
    packageRef?: Promise<string | undefined>,
): void {
    if (!opts.id) {
        throw new Error("Cannot read resource whose options are lacking an ID value");
    }
    const id: Promise<Input<ID>> = output(opts.id).promise(true);

    const label = `resource:${name}[${t}]#...`;
    log.debug(`Reading resource: t=${t}, name=${name}`);

    // Wait for all values to be available, and then perform the RPC.
    const done = rpcKeepAlive();

    const monitor = getMonitor();
    const resopAsync = prepareResource(label, res, parent, true, false, props, opts);

    const preallocError = new Error();
    debuggablePromise(
        resopAsync
            .then(async (resop) => {
                const resolvedID = await serializeProperty(label, await id, new Set(), { keepOutputValues: false });
                log.debug(
                    `ReadResource RPC prepared: id=${resolvedID}, t=${t}, name=${name}` +
                        (excessiveDebugOutput ? `, obj=${JSON.stringify(resop.serializedProps)}` : ``),
                );
                let packageRefStr = undefined;
                if (packageRef !== undefined) {
                    packageRefStr = await packageRef;
                    if (packageRefStr !== undefined) {
                        // If we have a package reference we can clear some of the resource options
                        opts.version = undefined;
                        opts.pluginDownloadURL = undefined;
                    }
                }
                // Create a resource request and do the RPC.
                const req = new resproto.ReadResourceRequest();
                req.setType(t);
                req.setName(name);
                req.setId(resolvedID);
                req.setParent(resop.parentURN || "");
                req.setProvider(resop.providerRef || "");
                req.setProperties(gstruct.Struct.fromJavaScript(resop.serializedProps));
                req.setDependenciesList(Array.from(resop.allDirectDependencyURNs));
                req.setVersion(opts.version || "");
                req.setPlugindownloadurl(opts.pluginDownloadURL || "");
                req.setAcceptsecrets(true);
                req.setAcceptresources(!utils.disableResourceReferences);
                req.setAdditionalsecretoutputsList((<any>opts).additionalSecretOutputs || []);
                req.setSourceposition(marshalSourcePosition(sourcePosition));
                req.setPackageref(packageRefStr || "");

                // Now run the operation, serializing the invocation if necessary.
                const opLabel = `monitor.readResource(${label})`;
                runAsyncResourceOp(opLabel, async () => {
                    let resp: any = {};
                    let err: Error | undefined;
                    try {
                        if (monitor) {
                            // If we're attached to the engine, make an RPC call and wait for it to resolve.
                            resp = await debuggablePromise(
                                new Promise((resolve, reject) =>
                                    monitor.readResource(
                                        req,
                                        (
                                            rpcError: grpc.ServiceError | null,
                                            innerResponse: resproto.ReadResourceResponse | undefined,
                                        ) => {
                                            log.debug(
                                                `ReadResource RPC finished: ${label}; err: ${rpcError}, resp: ${innerResponse}`,
                                            );
                                            if (rpcError) {
                                                if (
                                                    rpcError.code === grpc.status.UNAVAILABLE ||
                                                    rpcError.code === grpc.status.CANCELLED
                                                ) {
                                                    err = rpcError;
                                                    terminateRpcs();
                                                    rpcError.message = "Resource monitor is terminating";
                                                    (<any>preallocError).code = rpcError.code;
                                                }

                                                preallocError.message = `failed to read resource #${resolvedID} '${name}' [${t}]: ${rpcError.message}`;
                                                reject(preallocError);
                                            } else {
                                                resolve(innerResponse);
                                            }
                                        },
                                    ),
                                ),
                                opLabel,
                            );
                        } else {
                            // If we aren't attached to the engine, in test mode, mock up a fake response for testing purposes.
                            const mockurn = await createUrn(req.getName(), req.getType(), req.getParent()).promise();
                            resp = {
                                getUrn: () => mockurn,
                                getProperties: () => req.getProperties(),
                            };
                        }
                    } catch (e) {
                        err = e;
                        resp = {
                            getUrn: () => "",
                            getProperties: () => undefined,
                        };
                    }

                    // Now resolve everything: the URN, the ID (supplied as input), and the output properties.
                    resop.resolveURN(resp.getUrn(), err);
                    resop.resolveID!(resolvedID, resolvedID !== undefined, err);
                    await resolveOutputs(res, t, name, props, resp.getProperties(), {}, resop.resolvers, err);
                    done();
                });
            })
            .catch((err) => {
                done();
                throw err;
            }),
        label,
    );
}

function getParentURN(parent?: Resource | Input<string>) {
    if (Resource.isInstance(parent)) {
        return parent.urn;
    }
    return output(parent);
}

export function mapAliasesForRequest(
    aliases: (URN | Alias)[] | undefined,
    parentURN?: URN,
): Promise<aliasproto.Alias[]> {
    if (aliases === undefined) {
        return Promise.resolve([]);
    }

    return Promise.all(
        aliases.map(async (a) => {
            const newAlias = new aliasproto.Alias();
            if (typeof a === "string") {
                newAlias.setUrn(a);
            } else {
                const newAliasSpec = new aliasproto.Alias.Spec();
                const name = a.name === undefined ? undefined : await output(a.name).promise();
                const type = a.type === undefined ? undefined : await output(a.type).promise();
                const stack = a.stack === undefined ? undefined : await output(a.stack).promise();
                const project = a.project === undefined ? undefined : await output(a.project).promise();

                newAliasSpec.setName(name || "");
                newAliasSpec.setType(type || "");
                newAliasSpec.setStack(stack || "");
                newAliasSpec.setProject(project || "");
                if (a.hasOwnProperty("parent")) {
                    if (a.parent === undefined) {
                        newAliasSpec.setNoparent(true);
                    } else {
                        const aliasParentUrn = getParentURN(a.parent);
                        const urn = await aliasParentUrn.promise();
                        if (urn !== undefined) {
                            newAliasSpec.setParenturn(urn);
                        }
                    }
                } else if (parentURN) {
                    // If a parent isn't specified for the alias and the resource has a parent,
                    // pass along the resource's parent in the alias spec.
                    // It shouldn't be necessary to do this because the engine should fill-in the
                    // resource's parent if one wasn't specified for the alias.
                    // However, some older versions of the CLI don't do this correctly, and this
                    // SDK has always passed along the parent in this way, so we continue doing it
                    // to maintain compatibility with these versions of the CLI.
                    newAliasSpec.setParenturn(parentURN);
                }
                newAlias.setSpec(newAliasSpec);
            }
            return newAlias;
        }),
    );
}

/**
 * registerResource registers a new resource object with a given type `t` and
 * `name`. It returns the auto-generated URN and the ID that will resolve after
 * the deployment has completed.  All properties will be initialized to property
 * objects that the registration operation will resolve at the right time (or
 * remain unresolved for deployments).
 */
export function registerResource(
    res: Resource,
    parent: Resource | undefined,
    t: string,
    name: string,
    custom: boolean,
    remote: boolean,
    newDependency: (urn: URN) => Resource,
    props: Inputs,
    opts: ResourceOptions,
    sourcePosition?: SourcePosition,
    packageRef?: Promise<string | undefined>,
): void {
    const label = `resource:${name}[${t}]`;
    log.debug(`Registering resource: t=${t}, name=${name}, custom=${custom}, remote=${remote}`);

    // Wait for all values to be available, and then perform the RPC.
    const done = rpcKeepAlive();

    const monitor = getMonitor();
    const resopAsync = prepareResource(label, res, parent, custom, remote, props, opts, t, name);

    // In order to present a useful stack trace if an error does occur, we preallocate potential
    // errors here. V8 captures a stack trace at the moment an Error is created and this stack
    // trace will lead directly to user code. Throwing in `runAsyncResourceOp` results in an Error
    // with a non-useful stack trace.
    const preallocError = new Error();
    debuggablePromise(
        resopAsync
            .then(async (resop) => {
                log.debug(
                    `RegisterResource RPC prepared: t=${t}, name=${name}` +
                        (excessiveDebugOutput ? `, obj=${JSON.stringify(resop.serializedProps)}` : ``),
                );

                await awaitStackRegistrations();

                const callbacks: Callback[] = [];
                if (opts.transforms !== undefined && opts.transforms.length > 0) {
                    if (!getStore().supportsTransforms) {
                        throw new Error("The Pulumi CLI does not support transforms. Please update the Pulumi CLI");
                    }

                    const callbackServer = getCallbacks();
                    if (callbackServer === undefined) {
                        throw new Error("Callback server could not initialize");
                    }

                    for (const transform of opts.transforms) {
                        callbacks.push(await callbackServer.registerTransform(transform));
                    }
                }

                // If we have a package reference, we need to wait for it to resolve.
                let packageRefStr = undefined;
                if (packageRef !== undefined) {
                    packageRefStr = await packageRef;
                    if (packageRefStr !== undefined) {
                        // If we have a package reference we can clear some of the resource options
                        opts.version = undefined;
                        opts.pluginDownloadURL = undefined;
                    }
                }

                const req = new resproto.RegisterResourceRequest();
                req.setPackageref(packageRefStr || "");
                req.setType(t);
                req.setName(name);
                req.setParent(resop.parentURN || "");
                req.setCustom(custom);
                req.setObject(gstruct.Struct.fromJavaScript(resop.serializedProps));
                if (opts.protect !== undefined) {
                    req.setProtect(opts.protect);
                }
                req.setProvider(resop.providerRef || "");
                req.setDependenciesList(Array.from(resop.allDirectDependencyURNs));
                req.setDeletebeforereplace((<any>opts).deleteBeforeReplace || false);
                req.setDeletebeforereplacedefined((<any>opts).deleteBeforeReplace !== undefined);
                req.setIgnorechangesList(opts.ignoreChanges || []);
                req.setVersion(opts.version || "");
                req.setAcceptsecrets(true);
                req.setAcceptresources(!utils.disableResourceReferences);
                req.setAdditionalsecretoutputsList((<any>opts).additionalSecretOutputs || []);
                if (resop.monitorSupportsStructuredAliases) {
                    const aliasesList = await mapAliasesForRequest(resop.aliases, resop.parentURN);
                    req.setAliasesList(aliasesList);
                } else {
                    const urns = new Array<string>();
                    resop.aliases.forEach((v) => {
                        if (typeof v === "string") {
                            urns.push(v);
                        }
                    });
                    req.setAliasurnsList(urns);
                }
                req.setImportid(resop.import || "");
                req.setSupportspartialvalues(true);
                req.setRemote(remote);
                req.setReplaceonchangesList(opts.replaceOnChanges || []);
                req.setPlugindownloadurl(opts.pluginDownloadURL || "");
                req.setRetainondelete(opts.retainOnDelete || false);
                req.setDeletedwith(resop.deletedWithURN || "");
                req.setAliasspecs(true);
                req.setSourceposition(marshalSourcePosition(sourcePosition));
                req.setTransformsList(callbacks);
                req.setSupportsresultreporting(true);

                if (resop.deletedWithURN && !getStore().supportsDeletedWith) {
                    throw new Error(
                        "The Pulumi CLI does not support the DeletedWith option. Please update the Pulumi CLI.",
                    );
                }

                const customTimeouts = new resproto.RegisterResourceRequest.CustomTimeouts();
                if (opts.customTimeouts != null) {
                    customTimeouts.setCreate(opts.customTimeouts.create || "");
                    customTimeouts.setUpdate(opts.customTimeouts.update || "");
                    customTimeouts.setDelete(opts.customTimeouts.delete || "");
                }
                req.setCustomtimeouts(customTimeouts);

                const propertyDependencies = req.getPropertydependenciesMap();
                for (const [key, resourceURNs] of resop.propertyToDirectDependencyURNs) {
                    const deps = new resproto.RegisterResourceRequest.PropertyDependencies();
                    deps.setUrnsList(Array.from(resourceURNs));
                    propertyDependencies.set(key, deps);
                }

                const providerRefs = req.getProvidersMap();
                for (const [key, ref] of resop.providerRefs) {
                    providerRefs.set(key, ref);
                }

                // Now run the operation, serializing the invocation if necessary.
                const opLabel = `monitor.registerResource(${label})`;
                runAsyncResourceOp(opLabel, async () => {
                    let resp: any = {};
                    let err: Error | undefined;
                    try {
                        if (monitor) {
                            // If we're running with an attachment to the engine, perform the operation.
                            resp = await debuggablePromise(
                                new Promise((resolve, reject) =>
                                    monitor.registerResource(
                                        req,
                                        (
                                            rpcErr: grpc.ServiceError | null,
                                            innerResponse: resproto.RegisterResourceResponse | undefined,
                                        ) => {
                                            if (rpcErr) {
                                                err = rpcErr;
                                                // If the monitor is unavailable, it is in the process of shutting down or has already
                                                // shut down. Don't emit an error and don't do any more RPCs, just exit.
                                                if (
                                                    rpcErr.code === grpc.status.UNAVAILABLE ||
                                                    rpcErr.code === grpc.status.CANCELLED
                                                ) {
                                                    // Re-emit the message
                                                    terminateRpcs();
                                                    rpcErr.message = "Resource monitor is terminating";
                                                    (<any>preallocError).code = rpcErr.code;
                                                }

                                                // Node lets us hack the message as long as we do it before accessing the `stack` property.
                                                log.debug(
                                                    `RegisterResource RPC finished: ${label}; err: ${rpcErr}, resp: ${innerResponse}`,
                                                );
                                                preallocError.message = `failed to register new resource ${name} [${t}]: ${rpcErr.message}`;
                                                reject(preallocError);
                                            } else {
                                                log.debug(
                                                    `RegisterResource RPC finished: ${label}; err: ${rpcErr}, resp: ${innerResponse}`,
                                                );
                                                resolve(innerResponse);
                                            }
                                        },
                                    ),
                                ),
                                opLabel,
                            );
                        } else {
                            // If we aren't attached to the engine, in test mode, mock up a fake response for testing purposes.
                            const mockurn = await createUrn(req.getName(), req.getType(), req.getParent()).promise();
                            resp = {
                                getUrn: () => mockurn,
                                getId: () => undefined,
                                getObject: () => req.getObject(),
                                getPropertydependenciesMap: () => undefined,
                                getResult: () => 0,
                            };
                        }
                    } catch (e) {
                        err = e;
                        resp = {
                            getUrn: () => "",
                            getId: () => undefined,
                            getObject: () => req.getObject(),
                            getPropertydependenciesMap: () => undefined,
                            getResult: () => 0,
                        };
                    }

                    resop.resolveURN(resp.getUrn(), err);

                    // Note: 'id || undefined' is intentional.  We intentionally collapse falsy values to
                    // undefined so that later parts of our system don't have to deal with values like 'null'.
                    if (resop.resolveID) {
                        const id = resp.getId() || undefined;
                        resop.resolveID(id, id !== undefined, err);
                    }

                    const deps: Record<string, Resource[]> = {};
                    const rpcDeps = resp.getPropertydependenciesMap();
                    if (rpcDeps) {
                        for (const [k, propertyDeps] of resp.getPropertydependenciesMap().entries()) {
                            const urns = <URN[]>propertyDeps.getUrnsList();
                            deps[k] = urns.map((urn) => newDependency(urn));
                        }
                    }

                    // Now resolve the output properties.
                    const keepUnknowns = resp.getResult() !== resproto.Result.SUCCESS;
                    await resolveOutputs(
                        res,
                        t,
                        name,
                        props,
                        resp.getObject(),
                        deps,
                        resop.resolvers,
                        err,
                        keepUnknowns,
                    );
                    done();
                });
            })
            .catch((err) => {
                // If we fail to prepare the resource, we need to ensure that we still call done to prevent a hang.
                done();
                throw err;
            }),
        label,
    );
}

/**
 * Prepares for an RPC that will manufacture a resource, and hence deals with
 * input and output properties.
 *
 * @internal
 */
export async function prepareResource(
    label: string,
    res: Resource,
    parent: Resource | undefined,
    custom: boolean,
    remote: boolean,
    props: Inputs,
    opts: ResourceOptions,
    type?: string,
    name?: string,
): Promise<ResourceResolverOperation> {
    // Simply initialize the URN property and get prepared to resolve it later on.
    // Note: a resource urn will always get a value, and thus the output property
    // for it can always run .apply calls.
    let resolveURN: (urn: URN, err?: Error) => void;
    {
        let resolveValue: (urn: URN) => void;
        let rejectValue: (err: Error) => void;
        let resolveIsKnown: (isKnown: boolean) => void;
        let rejectIsKnown: (err: Error) => void;
        (res as any).urn = new Output(
            res,
            debuggablePromise(
                new Promise<URN>((resolve, reject) => {
                    resolveValue = resolve;
                    rejectValue = reject;
                }),
                `resolveURN(${label})`,
            ),
            debuggablePromise(
                new Promise<boolean>((resolve, reject) => {
                    resolveIsKnown = resolve;
                    rejectIsKnown = reject;
                }),
                `resolveURNIsKnown(${label})`,
            ),
            /*isSecret:*/ Promise.resolve(false),
            Promise.resolve(res),
        );

        resolveURN = (v, err) => {
            if (err) {
                if (isGrpcError(err)) {
                    if (debugPromiseLeaks) {
                        console.error("info: skipped rejection in resolveURN");
                    }
                    return;
                }
                rejectValue(err);
                rejectIsKnown(err);
            } else {
                resolveValue(v);
                resolveIsKnown(true);
            }
        };
    }

    // If a custom resource, make room for the ID property.
    let resolveID: ((v: any, performApply: boolean, err?: Error) => void) | undefined;
    if (custom) {
        let resolveValue: (v: ID) => void;
        let rejectValue: (err: Error) => void;
        let resolveIsKnown: (v: boolean) => void;
        let rejectIsKnown: (err: Error) => void;

        (res as any).id = new Output(
            res,
            debuggablePromise(
                new Promise<ID>((resolve, reject) => {
                    resolveValue = resolve;
                    rejectValue = reject;
                }),
                `resolveID(${label})`,
            ),
            debuggablePromise(
                new Promise<boolean>((resolve, reject) => {
                    resolveIsKnown = resolve;
                    rejectIsKnown = reject;
                }),
                `resolveIDIsKnown(${label})`,
            ),
            Promise.resolve(false),
            Promise.resolve(res),
        );

        resolveID = (v, isKnown, err) => {
            if (err) {
                if (isGrpcError(err)) {
                    if (debugPromiseLeaks) {
                        console.error("info: skipped rejection in resolveID");
                    }
                    return;
                }
                rejectValue(err);
                rejectIsKnown(err);
            } else {
                resolveValue(v);
                resolveIsKnown(isKnown);
            }
        };
    }

    // Now "transfer" all input properties into unresolved Promises on res.  This way,
    // this resource will look like it has all its output properties to anyone it is
    // passed to.  However, those promises won't actually resolve until the registerResource
    // RPC returns
    const resolvers = transferProperties(res, label, props);

    /** IMPORTANT!  We should never await prior to this line, otherwise the Resource will be partly uninitialized. */

    // Before we can proceed, all our dependencies must be finished.
    const explicitDirectDependencies = new Set(await gatherExplicitDependencies(opts.dependsOn));

    // Serialize out all our props to their final values.  In doing so, we'll also collect all
    // the Resources pointed to by any Dependency objects we encounter, adding them to 'propertyDependencies'.
    const [serializedProps, propertyToDirectDependencies] = await serializeResourceProperties(label, props, {
        // To initially scope the use of this new feature, we only keep output values when
        // remote is true (for multi-lang components, i.e. MLCs).
        keepOutputValues: remote,
        // When remote is true, exclude resource references from 'propertyDependencies'.
        // This way, component providers creating outputs for component inputs based
        // on 'propertyDependencies' won't create outputs for properties that only
        // contain resource references.
        excludeResourceReferencesFromDependencies: remote,
    });

    // Wait for the parent to complete.
    // If no parent was provided, parent to the root resource.
    const parentURN = parent ? await parent.urn.promise() : undefined;

    let importID: ID | undefined;
    if (custom) {
        const customOpts = <CustomResourceOptions>opts;
        importID = customOpts.import;
    }

    let providerRef: string | undefined;
    let sendProvider = custom;
    if (remote && opts.provider) {
        // If it's a remote component and a provider was specified, only
        // send the provider in the request if the provider's package is
        // the same as the component's package. Otherwise, don't send it
        // because the user specified `provider: someProvider` as shorthand
        // for `providers: [someProvider]`.
        const pkg = pkgFromType(type!);
        if (pkg && pkg === opts.provider.getPackage()) {
            sendProvider = true;
        }
    }
    if (sendProvider) {
        providerRef = await ProviderResource.register(opts.provider);
    }

    const providerRefs: Map<string, string> = new Map<string, string>();
    if (remote || !custom) {
        const componentOpts = <ComponentResourceOptions>opts;
        expandProviders(componentOpts);
        // the <ProviderResource[]> casts are safe because expandProviders
        // /always/ leaves providers as an array.
        if (componentOpts.provider !== undefined) {
            if (componentOpts.providers === undefined) {
                // We still want to do the promotion, so we define providers
                componentOpts.providers = [componentOpts.provider];
            } else if ((<ProviderResource[]>componentOpts.providers)?.indexOf(componentOpts.provider) !== -1) {
                const pkg = componentOpts.provider.getPackage();
                const message = `There is a conflict between the 'provider' field (${pkg}) and a member of the 'providers' map'. `;
                const deprecationd =
                    "This will become an error in a future version. See https://github.com/pulumi/pulumi/issues/8799 for more details";
                log.warn(message + deprecationd);
            } else {
                (<ProviderResource[]>componentOpts.providers).push(componentOpts.provider);
            }
        }
        if (componentOpts.providers) {
            for (const provider of componentOpts.providers as ProviderResource[]) {
                const pref = await ProviderResource.register(provider);
                if (pref) {
                    providerRefs.set(provider.getPackage(), pref);
                }
            }
        }
    }

    // Collect the URNs for explicit/implicit dependencies for the engine so that it can understand
    // the dependency graph and optimize operations accordingly.

    // The list of all dependencies (implicit or explicit).
    const allDirectDependencies = new Set<Resource>(explicitDirectDependencies);

    const exclude = new Set<Resource>([res]);
    const allDirectDependencyURNs = await getAllTransitivelyReferencedResourceURNs(explicitDirectDependencies, exclude);
    const propertyToDirectDependencyURNs = new Map<string, Set<URN>>();

    for (const [propertyName, directDependencies] of propertyToDirectDependencies) {
        addAll(allDirectDependencies, directDependencies);

        const urns = await getAllTransitivelyReferencedResourceURNs(directDependencies, exclude);
        addAll(allDirectDependencyURNs, urns);
        propertyToDirectDependencyURNs.set(propertyName, urns);
    }

    const monitorSupportsStructuredAliases = getStore().supportsAliasSpecs;
    let computedAliases;
    if (!monitorSupportsStructuredAliases && parent) {
        computedAliases = allAliases(opts.aliases || [], name!, type!, parent, parent.__name!);
    } else {
        computedAliases = opts.aliases || [];
    }

    // Wait for all aliases.
    const aliases = [];
    const uniqueAliases = new Set<Alias | URN>();
    for (const alias of computedAliases || []) {
        const aliasVal = await output(alias).promise();
        if (!uniqueAliases.has(aliasVal)) {
            uniqueAliases.add(aliasVal);
            aliases.push(aliasVal);
        }
    }

    const deletedWithURN = opts?.deletedWith ? await opts.deletedWith.urn.promise() : undefined;

    return {
        resolveURN: resolveURN,
        resolveID: resolveID,
        resolvers: resolvers,
        serializedProps: serializedProps,
        parentURN: parentURN,
        providerRef: providerRef,
        providerRefs: providerRefs,
        allDirectDependencyURNs: allDirectDependencyURNs,
        propertyToDirectDependencyURNs: propertyToDirectDependencyURNs,
        aliases: aliases,
        import: importID,
        monitorSupportsStructuredAliases,
        deletedWithURN,
    };
}

function addAll<T>(to: Set<T>, from: Set<T>) {
    for (const val of from) {
        to.add(val);
    }
}

/**
 * Finishes a resource creation RPC operation by resolving its outputs to the
 * resulting RPC payload.
 */
async function resolveOutputs(
    res: Resource,
    t: string,
    name: string,
    props: Inputs,
    outputs: gstruct.Struct | undefined,
    deps: Record<string, Resource[]>,
    resolvers: OutputResolvers,
    err?: Error,
    keepUnknowns?: boolean,
): Promise<void> {
    // Produce a combined set of property states, starting with inputs and then applying
    // outputs.  If the same property exists in the inputs and outputs states, the output wins.
    const allProps: Record<string, any> = {};
    if (outputs) {
        Object.assign(allProps, deserializeProperties(outputs, keepUnknowns));
    }

    const label = `resource:${name}[${t}]#...`;
    if (!isDryRun() || isLegacyApplyEnabled()) {
        for (const key of Object.keys(props)) {
            if (!allProps.hasOwnProperty(key)) {
                // input prop the engine didn't give us a final value for.  Just use the value passed into the resource
                // after round-tripping it through serialization. We do the round-tripping primarily s.t. we ensure that
                // Output values are handled properly w.r.t. unknowns.
                const inputProp = await serializeProperty(label, props[key], new Set(), { keepOutputValues: false });
                if (inputProp === undefined) {
                    continue;
                }
                allProps[key] = deserializeProperty(inputProp, keepUnknowns);
            }
        }
    }

    resolveProperties(res, resolvers, t, name, allProps, deps, err, keepUnknowns);
}

/**
 * Completes a resource registration, attaching an optional set of computed
 * outputs.
 */
export function registerResourceOutputs(res: Resource, outputs: Inputs | Promise<Inputs> | Output<Inputs>) {
    // Now run the operation. Note that we explicitly do not serialize output registration with
    // respect to other resource operations, as outputs may depend on properties of other resources
    // that will not resolve until later turns. This would create a circular promise chain that can
    // never resolve.
    const opLabel = `monitor.registerResourceOutputs(...)`;
    const done = rpcKeepAlive();
    runAsyncResourceOp(
        opLabel,
        async () => {
            try {
                // The registration could very well still be taking place, so we will need to wait for its URN.
                // Additionally, the output properties might have come from other resources, so we must await those too.
                const urn = await res.urn.promise();
                const resolved = await serializeProperties(opLabel, { outputs });
                const outputsObj = gstruct.Struct.fromJavaScript(resolved.outputs);
                log.debug(
                    `RegisterResourceOutputs RPC prepared: urn=${urn}` +
                        (excessiveDebugOutput ? `, outputs=${JSON.stringify(outputsObj)}` : ``),
                );

                // Fetch the monitor and make an RPC request.
                const monitor = getMonitor();
                if (monitor) {
                    const req = new resproto.RegisterResourceOutputsRequest();
                    req.setUrn(urn);
                    req.setOutputs(outputsObj);

                    const label = `monitor.registerResourceOutputs(${urn}, ...)`;
                    await debuggablePromise(
                        new Promise<void>((resolve, reject) =>
                            monitor.registerResourceOutputs(
                                req,
                                (err: grpc.ServiceError | null, innerResponse: gempty.Empty | undefined) => {
                                    log.debug(
                                        `RegisterResourceOutputs RPC finished: urn=${urn}; ` +
                                            `err: ${err}, resp: ${innerResponse}`,
                                    );
                                    if (err) {
                                        // If the monitor is unavailable, it is in the process of shutting down or has already
                                        // shut down. Don't emit an error and don't do any more RPCs, just exit.
                                        if (
                                            err.code === grpc.status.UNAVAILABLE ||
                                            err.code === grpc.status.CANCELLED
                                        ) {
                                            terminateRpcs();
                                            err.message = "Resource monitor is terminating";
                                        }

                                        reject(err);
                                    } else {
                                        log.debug(
                                            `RegisterResourceOutputs RPC finished: urn=${urn}; ` +
                                                `err: ${err}, resp: ${innerResponse}`,
                                        );
                                        resolve();
                                    }
                                },
                            ),
                        ),
                        label,
                    );
                }
            } finally {
                done();
            }
        },
        false,
    );
}

function isAny(o: any): o is any {
    return true;
}

/**
 * Returns the resource outputs (if any) for a stack, or an error if the stack
 * cannot be found. Resources are retrieved from the latest stack snapshot,
 * which may include ongoing updates. For example:
 *
 * ```typescript
 * const buckets = pulumi.runtime.listResourceOutput(aws.s3.Bucket.isInstance);
 * ```
 *
 * @param stackName
 *  Name of stack to retrieve resource outputs for. Defaults to the current stack.
 * @param typeFilter
 *  A [type guard](https://www.typescriptlang.org/docs/handbook/advanced-types.html#user-defined-type-guards)
 *  that specifies which resource types to list outputs of.
 */
export function listResourceOutputs<U extends Resource>(
    typeFilter?: (o: any) => o is U,
    stackName?: string,
): query.AsyncQueryable<ResolvedResource<U>> {
    if (typeFilter === undefined) {
        typeFilter = isAny;
    }

    return query
        .from(
            invoke("pulumi:pulumi:readStackResourceOutputs", {
                stackName: stackName || getStack(),
            }).then<any[]>(({ outputs }) => utils.values(outputs)),
        )
        .map<ResolvedResource<U>>(({ type: typ, outputs }) => {
            return { ...outputs, __pulumiType: typ };
        })
        .filter(typeFilter);
}

/**
 * resourceChain is used to serialize all resource requests.  If we don't do
 * this, all resource operations will be entirely asynchronous, meaning the
 * dataflow graph that results will determine ordering of operations.  This
 * causes problems with some resource providers, so for now we will serialize
 * all of them.  The issue pulumi/pulumi#335 tracks coming up with a long-term
 * solution here.
 */
let resourceChain: Promise<void> = Promise.resolve();
let resourceChainLabel: string | undefined = undefined;

/**
 * Runs an asynchronous resource operation, possibly serializing it as
 * necessary.
 */
function runAsyncResourceOp(label: string, callback: () => Promise<void>, serial?: boolean): void {
    // Serialize the invocation if necessary.
    if (serial === undefined) {
        serial = serialize();
    }
    const resourceOp: Promise<void> = suppressUnhandledGrpcRejections(
        debuggablePromise(
            resourceChain.then(async () => {
                if (serial) {
                    resourceChainLabel = label;
                    log.debug(`Resource RPC serialization requested: ${label} is current`);
                }
                return callback();
            }),
            label + "-initial",
        ),
    );

    const finalOp: Promise<void> = debuggablePromise(resourceOp, label + "-final");

    // Set up another promise that propagates the error, if any, so that it triggers unhandled rejection logic.
    resourceOp.catch((err) => Promise.reject(err));

    // If serialization is requested, wait for the prior resource operation to finish before we proceed, serializing
    // them, and make this the current resource operation so that everybody piles up on it.
    if (serial) {
        resourceChain = finalOp;
        if (resourceChainLabel) {
            log.debug(`Resource RPC serialization requested: ${label} is behind ${resourceChainLabel}`);
        }
    }
}
