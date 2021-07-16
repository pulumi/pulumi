// Copyright 2016-2018, Pulumi Corporation.
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
import { expandProviders } from "../resource";
import {
    ComponentResource,
    ComponentResourceOptions,
    createUrn,
    CustomResource,
    CustomResourceOptions,
    ID,
    ProviderResource,
    Resource,
    ResourceOptions,
    URN,
} from "../resource";
import { debuggablePromise } from "./debuggable";
import { invoke } from "./invoke";

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
    excessiveDebugOutput,
    getMonitor,
    getProject,
    getRootResource,
    getStack,
    isDryRun,
    isLegacyApplyEnabled,
    rpcKeepAlive,
    serialize,
    terminateRpcs,
} from "./settings";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
const providerproto = require("../proto/provider_pb.js");
const resproto = require("../proto/resource_pb.js");

interface ResourceResolverOperation {
    // A resolver for a resource's URN.
    resolveURN: (urn: URN, err?: Error) => void;
    // A resolver for a resource's ID (for custom resources only).
    resolveID: ((v: ID, performApply: boolean, err?: Error) => void) | undefined;
    // A collection of resolvers for a resource's properties.
    resolvers: OutputResolvers;
    // A parent URN, fully resolved, if any.
    parentURN: URN | undefined;
    // A provider reference, fully resolved, if any.
    providerRef: string | undefined;
    // A map of provider references, fully resolved, if any.
    providerRefs: Map<string, string>;
    // All serialized properties, fully awaited, serialized, and ready to go.
    serializedProps: Record<string, any>;
    // A set of URNs that this resource is directly dependent upon.  These will all be URNs of
    // custom resources, not component resources.
    allDirectDependencyURNs: Set<URN>;
    // Set of URNs that this resource is directly dependent upon, keyed by the property that causes
    // the dependency.  All urns in this map must exist in [allDirectDependencyURNs].  These will
    // all be URNs of custom resources, not component resources.
    propertyToDirectDependencyURNs: Map<string, Set<URN>>;
    // A list of aliases applied to this resource.
    aliases: URN[];
    // An ID to import, if any.
    import: ID | undefined;
}

/**
 * Get an existing resource's state from the engine.
 */
export function getResource(res: Resource, props: Inputs, custom: boolean, urn: string): void {
    // Extract the resource type from the URN.
    const urnParts = urn.split("::");
    const qualifiedType = urnParts[2];
    const urnName = urnParts[3];
    const type = qualifiedType.split("$").pop()!;

    const label = `resource:urn=${urn}`;
    log.debug(`Getting resource: urn=${urn}`);

    const monitor: any = getMonitor();
    const resopAsync = prepareResource(label, res, custom, false, props, {});

    const preallocError = new Error();
    debuggablePromise(resopAsync.then(async (resop) => {
        const inputs = await serializeProperties(label, { urn });

        const req = new providerproto.InvokeRequest();
        req.setTok("pulumi:pulumi:getResource");
        req.setArgs(gstruct.Struct.fromJavaScript(inputs));
        req.setProvider("");
        req.setVersion("");

        // Now run the operation, serializing the invocation if necessary.
        const opLabel = `monitor.getResource(${label})`;
        runAsyncResourceOp(opLabel, async () => {
            let resp: any = {};
            let err: Error | undefined;
            try {
                if (monitor) {
                    resp = await debuggablePromise(new Promise((resolve, reject) =>
                        monitor.invoke(req, (rpcError: grpc.ServiceError, innerResponse: any) => {
                            log.debug(`getResource Invoke RPC finished: err: ${rpcError}, resp: ${innerResponse}`);
                            if (rpcError) {
                                if (rpcError.code === grpc.status.UNAVAILABLE || rpcError.code === grpc.status.CANCELLED) {
                                    err = rpcError;
                                    terminateRpcs();
                                    rpcError.message = "Resource monitor is terminating";
                                    (<any>preallocError).code = rpcError.code;
                                }

                                preallocError.message = `failed to get resource:urn=${urn}: ${rpcError.message}`;
                                reject(new Error(rpcError.details));
                            }
                            else {
                                resolve(innerResponse);
                            }
                        })), opLabel);

                    // If the invoke failed, raise an error
                    const failures: any = resp.getFailuresList();
                    if (failures && failures.length) {
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
        });
    }), label);
}

/**
 * Reads an existing custom resource's state from the resource monitor.  Note that resources read in this way
 * will not be part of the resulting stack's state, as they are presumed to belong to another.
 */
export function readResource(res: Resource, t: string, name: string, props: Inputs, opts: ResourceOptions): void {
    const id: Input<ID> | undefined = opts.id;
    if (!id) {
        throw new Error("Cannot read resource whose options are lacking an ID value");
    }

    const label = `resource:${name}[${t}]#...`;
    log.debug(`Reading resource: id=${Output.isInstance(id) ? "Output<T>" : id}, t=${t}, name=${name}`);

    const monitor = getMonitor();
    const resopAsync = prepareResource(label, res, true, false, props, opts);

    const preallocError = new Error();
    debuggablePromise(resopAsync.then(async (resop) => {
        const resolvedID = await serializeProperty(label, id, new Set());
        log.debug(`ReadResource RPC prepared: id=${resolvedID}, t=${t}, name=${name}` +
            (excessiveDebugOutput ? `, obj=${JSON.stringify(resop.serializedProps)}` : ``));

        // Create a resource request and do the RPC.
        const req = new resproto.ReadResourceRequest();
        req.setType(t);
        req.setName(name);
        req.setId(resolvedID);
        req.setParent(resop.parentURN);
        req.setProvider(resop.providerRef);
        req.setProperties(gstruct.Struct.fromJavaScript(resop.serializedProps));
        req.setDependenciesList(Array.from(resop.allDirectDependencyURNs));
        req.setVersion(opts.version || "");
        req.setAcceptsecrets(true);
        req.setAcceptresources(!utils.disableResourceReferences);
        req.setAdditionalsecretoutputsList((<any>opts).additionalSecretOutputs || []);

        // Now run the operation, serializing the invocation if necessary.
        const opLabel = `monitor.readResource(${label})`;
        runAsyncResourceOp(opLabel, async () => {
            let resp: any = {};
            let err: Error | undefined;
            try {
                if (monitor) {
                    // If we're attached to the engine, make an RPC call and wait for it to resolve.
                    resp = await debuggablePromise(new Promise((resolve, reject) =>
                        (monitor as any).readResource(req, (rpcError: grpc.ServiceError, innerResponse: any) => {
                            log.debug(`ReadResource RPC finished: ${label}; err: ${rpcError}, resp: ${innerResponse}`);
                            if (rpcError) {
                                if (rpcError.code === grpc.status.UNAVAILABLE || rpcError.code === grpc.status.CANCELLED) {
                                    err = rpcError;
                                    terminateRpcs();
                                    rpcError.message = "Resource monitor is terminating";
                                    (<any>preallocError).code = rpcError.code;
                                }

                                preallocError.message =
                                    `failed to read resource #${resolvedID} '${name}' [${t}]: ${rpcError.message}`;
                                reject(preallocError);
                            }
                            else {
                                resolve(innerResponse);
                            }
                        })), opLabel);
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
        });
    }), label);
}

/**
 * registerResource registers a new resource object with a given type t and name.  It returns the auto-generated
 * URN and the ID that will resolve after the deployment has completed.  All properties will be initialized to property
 * objects that the registration operation will resolve at the right time (or remain unresolved for deployments).
 */
export function registerResource(res: Resource, t: string, name: string, custom: boolean, remote: boolean,
                                 newDependency: (urn: URN) => Resource, props: Inputs, opts: ResourceOptions): void {
    const label = `resource:${name}[${t}]`;
    log.debug(`Registering resource: t=${t}, name=${name}, custom=${custom}, remote=${remote}`);

    const monitor = getMonitor();
    const resopAsync = prepareResource(label, res, custom, remote, props, opts);

    // In order to present a useful stack trace if an error does occur, we preallocate potential
    // errors here. V8 captures a stack trace at the moment an Error is created and this stack
    // trace will lead directly to user code. Throwing in `runAsyncResourceOp` results in an Error
    // with a non-useful stack trace.
    const preallocError = new Error();
    debuggablePromise(resopAsync.then(async (resop) => {
        log.debug(`RegisterResource RPC prepared: t=${t}, name=${name}` +
            (excessiveDebugOutput ? `, obj=${JSON.stringify(resop.serializedProps)}` : ``));

        const req = new resproto.RegisterResourceRequest();
        req.setType(t);
        req.setName(name);
        req.setParent(resop.parentURN);
        req.setCustom(custom);
        req.setObject(gstruct.Struct.fromJavaScript(resop.serializedProps));
        req.setProtect(opts.protect);
        req.setProvider(resop.providerRef);
        req.setDependenciesList(Array.from(resop.allDirectDependencyURNs));
        req.setDeletebeforereplace((<any>opts).deleteBeforeReplace || false);
        req.setDeletebeforereplacedefined((<any>opts).deleteBeforeReplace !== undefined);
        req.setIgnorechangesList(opts.ignoreChanges || []);
        req.setVersion(opts.version || "");
        req.setAcceptsecrets(true);
        req.setAcceptresources(!utils.disableResourceReferences);
        req.setAdditionalsecretoutputsList((<any>opts).additionalSecretOutputs || []);
        req.setAliasesList(resop.aliases);
        req.setImportid(resop.import || "");
        req.setSupportspartialvalues(true);
        req.setRemote(remote);
        req.setReplaceonchangesList(opts.replaceOnChanges || []);

        const customTimeouts = new resproto.RegisterResourceRequest.CustomTimeouts();
        if (opts.customTimeouts != null) {
            customTimeouts.setCreate(opts.customTimeouts.create);
            customTimeouts.setUpdate(opts.customTimeouts.update);
            customTimeouts.setDelete(opts.customTimeouts.delete);
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
                    resp = await debuggablePromise(new Promise((resolve, reject) =>
                        (monitor as any).registerResource(req, (rpcErr: grpc.ServiceError, innerResponse: any) => {
                            if (rpcErr) {
                                err = rpcErr;
                                // If the monitor is unavailable, it is in the process of shutting down or has already
                                // shut down. Don't emit an error and don't do any more RPCs, just exit.
                                if (rpcErr.code === grpc.status.UNAVAILABLE || rpcErr.code === grpc.status.CANCELLED) {
                                    // Re-emit the message
                                    terminateRpcs();
                                    rpcErr.message = "Resource monitor is terminating";
                                    (<any>preallocError).code = rpcErr.code;
                                }

                                // Node lets us hack the message as long as we do it before accessing the `stack` property.
                                log.debug(`RegisterResource RPC finished: ${label}; err: ${rpcErr}, resp: ${innerResponse}`);
                                preallocError.message = `failed to register new resource ${name} [${t}]: ${rpcErr.message}`;
                                reject(preallocError);
                            }
                            else {
                                log.debug(`RegisterResource RPC finished: ${label}; err: ${rpcErr}, resp: ${innerResponse}`);
                                resolve(innerResponse);
                            }
                        })), opLabel);
                } else {
                    // If we aren't attached to the engine, in test mode, mock up a fake response for testing purposes.
                    const mockurn = await createUrn(req.getName(), req.getType(), req.getParent()).promise();
                    resp = {
                        getUrn: () => mockurn,
                        getId: () => undefined,
                        getObject: () => req.getObject(),
                        getPropertydependenciesMap: () => undefined,
                    };
                }
            } catch (e) {
                err = e;
                resp = {
                    getUrn: () => "",
                    getId: () => undefined,
                    getObject: () => req.getObject(),
                    getPropertydependenciesMap: () => undefined,
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
                    deps[k] = urns.map(urn => newDependency(urn));
                }
            }

            // Now resolve the output properties.
            await resolveOutputs(res, t, name, props, resp.getObject(), deps, resop.resolvers, err);
        });
    }), label);
}

/**
 * Prepares for an RPC that will manufacture a resource, and hence deals with input and output
 * properties.
 */
async function prepareResource(label: string, res: Resource, custom: boolean, remote: boolean,
                               props: Inputs, opts: ResourceOptions): Promise<ResourceResolverOperation> {

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
                `resolveURN(${label})`),
            debuggablePromise(
                new Promise<boolean>((resolve, reject) => {
                    resolveIsKnown = resolve;
                    rejectIsKnown = reject;
                }),
                `resolveURNIsKnown(${label})`),
            /*isSecret:*/ Promise.resolve(false),
            Promise.resolve(res));

        resolveURN = (v, err) => {
            if (!!err) {
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
            debuggablePromise(new Promise<ID>((resolve, reject) => {
                resolveValue = resolve;
                rejectValue = reject;
            }),
                `resolveID(${label})`),
            debuggablePromise(new Promise<boolean>((resolve, reject) => {
                resolveIsKnown = resolve;
                rejectIsKnown = reject;
            }), `resolveIDIsKnown(${label})`),
            Promise.resolve(false),
            Promise.resolve(res));

        resolveID = (v, isKnown, err) => {
            if (!!err) {
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
    const [serializedProps, propertyToDirectDependencies] = await serializeResourceProperties(label, props);

    // Wait for the parent to complete.
    // If no parent was provided, parent to the root resource.
    const parentURN = opts.parent
        ? await opts.parent.urn.promise()
        : await getRootResource();

    let providerRef: string | undefined;
    let importID: ID | undefined;
    if (custom) {
        const customOpts = <CustomResourceOptions>opts;
        importID = customOpts.import;
        providerRef = await ProviderResource.register(opts.provider);
    }

    const providerRefs: Map<string, string> = new Map<string, string>();
    if (remote) {
        const componentOpts = <ComponentResourceOptions>opts;
        expandProviders(componentOpts);
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

    const allDirectDependencyURNs = await getAllTransitivelyReferencedResourceURNs(explicitDirectDependencies);
    const propertyToDirectDependencyURNs = new Map<string, Set<URN>>();

    for (const [propertyName, directDependencies] of propertyToDirectDependencies) {
        addAll(allDirectDependencies, directDependencies);

        const urns = await getAllTransitivelyReferencedResourceURNs(directDependencies);
        addAll(allDirectDependencyURNs, urns);
        propertyToDirectDependencyURNs.set(propertyName, urns);
    }

    // Wait for all aliases. Note that we use `res.__aliases` instead of `opts.aliases` as the former has been processed
    // in the Resource constructor prior to calling `registerResource` - both adding new inherited aliases and
    // simplifying aliases down to URNs.
    const aliases = [];
    const uniqueAliases = new Set<string>();
    for (const alias of (res.__aliases || [])) {
        const aliasVal = await output(alias).promise();
        if (!uniqueAliases.has(aliasVal)) {
            uniqueAliases.add(aliasVal);
            aliases.push(aliasVal);
        }
    }

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
    };
}

function addAll<T>(to: Set<T>, from: Set<T>) {
    for (const val of from) {
        to.add(val);
    }
}

async function getAllTransitivelyReferencedResourceURNs(resources: Set<Resource>): Promise<Set<string>> {
    // Go through 'resources', but transitively walk through **Component** resources, collecting any
    // of their child resources.  This way, a Component acts as an aggregation really of all the
    // reachable resources it parents.  This walking will stop when it hits custom resources.
    //
    // This function also terminates at remote components, whose children are not known to the Node SDK directly.
    // Remote components will always wait on all of their children, so ensuring we return the remote component
    // itself here and waiting on it will accomplish waiting on all of it's children regardless of whether they
    // are returned explicitly here.
    //
    // In other words, if we had:
    //
    //                  Comp1
    //              /     |     \
    //          Cust1   Comp2  Remote1
    //                  /   \       \
    //              Cust2   Cust3  Comp3
    //              /                 \
    //          Cust4                Cust5
    //
    // Then the transitively reachable resources of Comp1 will be [Cust1, Cust2, Cust3, Remote1].
    // It will *not* include:
    // * Cust4 because it is a child of a custom resource
    // * Comp2 because it is a non-remote component resoruce
    // * Comp3 and Cust5 because Comp3 is a child of a remote component resource

    // To do this, first we just get the transitively reachable set of resources (not diving
    // into custom resources).  In the above picture, if we start with 'Comp1', this will be
    // [Comp1, Cust1, Comp2, Cust2, Cust3]
    const transitivelyReachableResources = await getTransitivelyReferencedChildResourcesOfComponentResources(resources);

    // Then we filter to only include Custom and Remote resources.
    const transitivelyReachableCustomResources =
        [...transitivelyReachableResources]
        .filter(r => CustomResource.isInstance(r) || (r as ComponentResource).__remote);
    const promises = transitivelyReachableCustomResources.map(r => r.urn.promise());
    const urns = await Promise.all(promises);
    return new Set<string>(urns);
}

/**
 * Recursively walk the resources passed in, returning them and all resources reachable from
 * [Resource.__childResources] through any **Component** resources we encounter.
 */
async function getTransitivelyReferencedChildResourcesOfComponentResources(resources: Set<Resource>) {
    // Recursively walk the dependent resources through their children, adding them to the result set.
    const result = new Set<Resource>();
    await addTransitivelyReferencedChildResourcesOfComponentResources(resources, result);
    return result;
}

async function addTransitivelyReferencedChildResourcesOfComponentResources(resources: Set<Resource> | undefined, result: Set<Resource>) {
    if (resources) {
        for (const resource of resources) {
            if (!result.has(resource)) {
                result.add(resource);

                if (ComponentResource.isInstance(resource)) {
                    // This await is safe even if __isConstructed is undefined. Ensure that the
                    // resource has completely finished construction.  That way all parent/child
                    // relationships will have been setup.
                    await resource.__data;
                    addTransitivelyReferencedChildResourcesOfComponentResources(resource.__childResources, result);
                }
            }
        }
    }
}

/**
 * Gathers explicit dependent Resources from a list of Resources (possibly Promises and/or Outputs).
 */
async function gatherExplicitDependencies(
    dependsOn: Input<Input<Resource>[]> | Input<Resource> | undefined): Promise<Resource[]> {

    if (dependsOn) {
        if (Array.isArray(dependsOn)) {
            const dos: Resource[] = [];
            for (const d of dependsOn) {
                dos.push(...(await gatherExplicitDependencies(d)));
            }
            return dos;
        } else if (dependsOn instanceof Promise) {
            return gatherExplicitDependencies(await dependsOn);
        } else if (Output.isInstance(dependsOn)) {
            // Recursively gather dependencies, await the promise, and append the output's dependencies.
            const dos = (dependsOn as Output<Input<Resource>[] | Input<Resource>>).apply(v => gatherExplicitDependencies(v));
            const urns = await dos.promise();
            const dosResources = await getAllResources(dos);
            const implicits = await gatherExplicitDependencies([...dosResources]);
            return (urns ?? []).concat(implicits);
        } else {
            if (!Resource.isInstance(dependsOn)) {
                throw new Error("'dependsOn' was passed a value that was not a Resource.");
            }

            return [dependsOn];
        }
    }

    return [];
}

/**
 * Finishes a resource creation RPC operation by resolving its outputs to the resulting RPC payload.
 */
async function resolveOutputs(res: Resource, t: string, name: string,
                              props: Inputs, outputs: any, deps: Record<string, Resource[]>,
                              resolvers: OutputResolvers, err?: Error): Promise<void> {

    // Produce a combined set of property states, starting with inputs and then applying
    // outputs.  If the same property exists in the inputs and outputs states, the output wins.
    const allProps: Record<string, any> = {};
    if (outputs) {
        Object.assign(allProps, deserializeProperties(outputs));
    }

    const label = `resource:${name}[${t}]#...`;
    if (!isDryRun() || isLegacyApplyEnabled()) {
        for (const key of Object.keys(props)) {
            if (!allProps.hasOwnProperty(key)) {
                // input prop the engine didn't give us a final value for.  Just use the value passed into the resource
                // after round-tripping it through serialization. We do the round-tripping primarily s.t. we ensure that
                // Output values are handled properly w.r.t. unknowns.
                const inputProp = await serializeProperty(label, props[key], new Set());
                if (inputProp === undefined) {
                    continue;
                }
                allProps[key] = deserializeProperty(inputProp);
            }
        }
    }

    resolveProperties(res, resolvers, t, name, allProps, deps, err);
}

/**
 * registerResourceOutputs completes the resource registration, attaching an optional set of computed outputs.
 */
export function registerResourceOutputs(res: Resource, outputs: Inputs | Promise<Inputs> | Output<Inputs>) {
    // Now run the operation. Note that we explicitly do not serialize output registration with
    // respect to other resource operations, as outputs may depend on properties of other resources
    // that will not resolve until later turns. This would create a circular promise chain that can
    // never resolve.
    const opLabel = `monitor.registerResourceOutputs(...)`;
    runAsyncResourceOp(opLabel, async () => {
        // The registration could very well still be taking place, so we will need to wait for its URN.
        // Additionally, the output properties might have come from other resources, so we must await those too.
        const urn = await res.urn.promise();
        const resolved = await serializeProperties(opLabel, { outputs });
        const outputsObj = gstruct.Struct.fromJavaScript(resolved.outputs);
        log.debug(`RegisterResourceOutputs RPC prepared: urn=${urn}` +
            (excessiveDebugOutput ? `, outputs=${JSON.stringify(outputsObj)}` : ``));

        // Fetch the monitor and make an RPC request.
        const monitor = getMonitor();
        if (monitor) {
            const req = new resproto.RegisterResourceOutputsRequest();
            req.setUrn(urn);
            req.setOutputs(outputsObj);

            const label = `monitor.registerResourceOutputs(${urn}, ...)`;
            await debuggablePromise(new Promise((resolve, reject) =>
                (monitor as any).registerResourceOutputs(req, (err: grpc.ServiceError, innerResponse: any) => {
                    log.debug(`RegisterResourceOutputs RPC finished: urn=${urn}; ` +
                        `err: ${err}, resp: ${innerResponse}`);
                    if (err) {
                        // If the monitor is unavailable, it is in the process of shutting down or has already
                        // shut down. Don't emit an error and don't do any more RPCs, just exit.
                        if (err.code === grpc.status.UNAVAILABLE || err.code === grpc.status.CANCELLED) {
                            terminateRpcs();
                            err.message = "Resource monitor is terminating";
                        }

                        reject(err);
                    }
                    else {
                        log.debug(`RegisterResourceOutputs RPC finished: urn=${urn}; ` +
                            `err: ${err}, resp: ${innerResponse}`);
                        resolve();
                    }
                })), label);
        }
    }, false);
}

function isAny(o: any): o is any {
    return true;
}

/**
 * listResourceOutputs returns the resource outputs (if any) for a stack, or an error if the stack
 * cannot be found. Resources are retrieved from the latest stack snapshot, which may include
 * ongoing updates.
 *
 * @param stackName Name of stack to retrieve resource outputs for. Defaults to the current stack.
 * @param typeFilter A [type
 * guard](https://www.typescriptlang.org/docs/handbook/advanced-types.html#user-defined-type-guards)
 * that specifies which resource types to list outputs of.
 *
 * @example
 * const buckets = pulumi.runtime.listResourceOutput(aws.s3.Bucket.isInstance);
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
 * resourceChain is used to serialize all resource requests.  If we don't do this, all resource operations will be
 * entirely asynchronous, meaning the dataflow graph that results will determine ordering of operations.  This
 * causes problems with some resource providers, so for now we will serialize all of them.  The issue
 * pulumi/pulumi#335 tracks coming up with a long-term solution here.
 */
let resourceChain: Promise<void> = Promise.resolve();
let resourceChainLabel: string | undefined = undefined;

// runAsyncResourceOp runs an asynchronous resource operation, possibly serializing it as necessary.
function runAsyncResourceOp(label: string, callback: () => Promise<void>, serial?: boolean): void {
    // Serialize the invocation if necessary.
    if (serial === undefined) {
        serial = serialize();
    }
    const resourceOp: Promise<void> = suppressUnhandledGrpcRejections(debuggablePromise(resourceChain.then(async () => {
        if (serial) {
            resourceChainLabel = label;
            log.debug(`Resource RPC serialization requested: ${label} is current`);
        }
        return callback();
    }), label + "-initial"));

    // Ensure the process won't exit until this RPC call finishes and resolve it when appropriate.
    const done: () => void = rpcKeepAlive();
    const finalOp: Promise<void> = debuggablePromise(
        resourceOp.then(() => { done(); }, () => { done(); }),
        label + "-final");

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
