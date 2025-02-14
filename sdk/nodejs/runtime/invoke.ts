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

import { AsyncIterable } from "@pulumi/query/interfaces";

import { InvokeOptions, InvokeOutputOptions } from "../invoke";
import * as log from "../log";
import { Inputs, Output } from "../output";
import { debuggablePromise } from "./debuggable";
import {
    deserializeProperties,
    isRpcSecret,
    serializeProperties,
    serializePropertiesReturnDeps,
    unwrapRpcSecret,
    unwrapSecretValues,
    containsUnknownValues,
} from "./rpc";
import { awaitStackRegistrations, excessiveDebugOutput, getMonitor, rpcKeepAlive, terminateRpcs } from "./settings";

import { CustomResource, DependencyResource, ProviderResource, Resource } from "../resource";
import * as utils from "../utils";
import { PushableAsyncIterable } from "./asyncIterableUtil";
import { gatherExplicitDependencies, getAllTransitivelyReferencedResources } from "./dependsOn";

import * as gstruct from "google-protobuf/google/protobuf/struct_pb";
import * as resourceproto from "../proto/resource_pb";
import * as providerproto from "../proto/provider_pb";

/**
 * Dynamically invokes the function `tok`, which is offered by a provider
 * plugin. `invoke` behaves differently in the case that options contains
 * `{async:true}` or not.
 *
 * In the case where `{async:true}` is present in the options bag:
 *
 * 1. the result of `invoke` will be a Promise resolved to the result value of
 *    the provider plugin.
 *
 * 2. the `props` inputs can be a bag of computed values (including, `T`s,
 *   `Promise<T>`s, `Output<T>`s etc.).
 *
 * In the case where `{ async:true }` is not present in the options bag:
 *
 * 1. the result of `invoke` will be a Promise resolved to the result value of
 *    the provider call. However, that Promise will *also* have the respective
 *    values of the Provider result exposed directly on it as properties.
 *
 * 2. The inputs must be a bag of simple values, and the result is the result
 *    that the Provider produced.
 *
 * Simple values are:
 *
 *  1. `undefined`, `null`, string, number or boolean values.
 *  2. arrays of simple values.
 *  3. objects containing only simple values.
 *
 * Importantly, simple values do *not* include:
 *
 *  1. `Promise`s
 *  2. `Output`s
 *  3. `Asset`s or `Archive`s
 *  4. `Resource`s.
 *
 * All of these contain async values that would prevent `invoke from being able
 * to operate synchronously.
 */
export function invoke(
    tok: string,
    props: Inputs,
    opts: InvokeOptions = {},
    packageRef?: Promise<string | undefined>,
): Promise<any> {
    const optsCopy = { ...opts };
    if ("dependsOn" in optsCopy) {
        // DependsOn is only allowed for invokeOutput.
        //@ts-ignore
        optsCopy["dependsOn"] = undefined;
    }
    return invokeAsync(tok, props, optsCopy, packageRef).then((response) => {
        // ignore secrets for plain invoke
        const { result } = response;
        return result;
    });
}

/**
 * Similar to the plain `invoke` but returns the response as an output, maintaining
 * secrets of the response, if any.
 */
export function invokeOutput<T>(
    tok: string,
    props: Inputs,
    opts: InvokeOutputOptions = {},
    packageRef?: Promise<string | undefined>,
): Output<T> {
    const [output, resolve] = createOutput<T>(`invoke(${tok})`);
    invokeAsync(tok, props, opts, packageRef, true /* checkDependencies */)
        .then((response) => {
            const { result, isKnown, containsSecrets, dependencies } = response;
            resolve(<T>result, isKnown, containsSecrets, dependencies, undefined);
        })
        .catch((err) => {
            resolve(<any>undefined, true, false, [], err);
        });

    return output;
}

function extractSingleValue(result: Inputs | undefined): any {
    if (result === undefined) {
        return result;
    }
    // assume outputs has at least one key
    const keys = Object.keys(result);
    // return the first key's value from the outputs
    return result[keys[0]];
}

/*
 * Dynamically invokes the function `tok`, which is offered by a
 * provider plugin. Similar to `invoke`, but returns a single value instead of
 * an object with a single key.
 */
export function invokeSingle(
    tok: string,
    props: Inputs,
    opts: InvokeOptions = {},
    packageRef?: Promise<string | undefined>,
): Promise<any> {
    return invokeAsync(tok, props, opts, packageRef).then((response) => {
        // ignore secrets for plain invoke
        const { result } = response;
        return extractSingleValue(result);
    });
}

/**
 * Similar to the plain `invokeSingle` but returns the response as an output, maintaining
 * secrets of the response, if any.
 */
export function invokeSingleOutput<T>(
    tok: string,
    props: Inputs,
    opts: InvokeOptions = {},
    packageRef?: Promise<string | undefined>,
): Output<T> {
    const [output, resolve] = createOutput<T>(`invokeSingleOutput(${tok})`);
    invokeAsync(tok, props, opts, packageRef, true /* checkDependencies */)
        .then((response) => {
            const { result, isKnown, containsSecrets, dependencies } = response;
            const value = extractSingleValue(result);
            resolve(<T>value, isKnown, containsSecrets, dependencies, undefined);
        })
        .catch((err) => {
            resolve(<any>undefined, true, false, [], err);
        });

    return output;
}

export async function streamInvoke(
    tok: string,
    props: Inputs,
    opts: InvokeOptions = {},
): Promise<StreamInvokeResponse<any>> {
    const label = `StreamInvoking function: tok=${tok} asynchronously`;
    log.debug(label + (excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``));

    // Wait for all values to be available, and then perform the RPC.
    const done = rpcKeepAlive();
    try {
        const serialized = await serializeProperties(`streamInvoke:${tok}`, props);
        log.debug(
            `StreamInvoke RPC prepared: tok=${tok}` + excessiveDebugOutput ? `, obj=${JSON.stringify(serialized)}` : ``,
        );

        // Fetch the monitor and make an RPC request.
        const monitor: any = getMonitor();

        const provider = await ProviderResource.register(getProvider(tok, opts));
        const req = await createInvokeRequest(tok, serialized, provider, opts);

        // Call `streamInvoke`.
        const result = monitor.streamInvoke(req, {});

        const queue = new PushableAsyncIterable();
        result.on("data", function (thing: any) {
            const live = deserializeResponse(tok, thing);
            queue.push(live);
        });
        result.on("error", (err: any) => {
            if (err.code === 1) {
                return;
            }
            throw err;
        });
        result.on("end", () => {
            queue.complete();
        });

        // Return a cancellable handle to the stream.
        return new StreamInvokeResponse(queue, () => result.cancel());
    } finally {
        done();
    }
}

async function invokeAsync(
    tok: string,
    props: Inputs,
    opts: InvokeOutputOptions,
    packageRef?: Promise<string | undefined>,
    checkDependencies?: boolean,
): Promise<{
    result: Inputs | undefined;
    isKnown: boolean;
    containsSecrets: boolean;
    dependencies: Resource[];
}> {
    const label = `Invoking function: tok=${tok} asynchronously`;
    log.debug(label + (excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``));

    await awaitStackRegistrations();

    // Wait for all values to be available, and then perform the RPC.
    const done = rpcKeepAlive();
    try {
        // The direct dependencies of the invoke call from the dependsOn option.
        const dependsOnDeps = await gatherExplicitDependencies(opts.dependsOn);
        // The dependencies of the inputs to the invoke call.
        const [serialized, deps] = await serializePropertiesReturnDeps(`invoke:${tok}`, props);
        if (containsUnknownValues(serialized)) {
            // if any of the input properties are unknown,
            // make sure the entire response is marked as unknown
            return {
                result: {},
                isKnown: false,
                containsSecrets: false,
                dependencies: [],
            };
        }

        // Only check the resource dependencies for output form invokes. For
        // plain invokes, we do not want to check the dependencies. Technically,
        // these should only receive plain arguments, but this is not strictly
        // enforced, and in practice people pass in outputs. This happens to
        // work because we serialize the arguments.
        if (checkDependencies) {
            // If we depend on any CustomResources, we need to ensure that their
            // ID is known before proceeding. If it is not known, we will return
            // an unknown result.
            const resourcesToWaitFor = new Set<Resource>(dependsOnDeps);
            // Add the dependencies from the inputs to the set of resources to wait for.
            for (const resourceDeps of deps.values()) {
                for (const value of resourceDeps.values()) {
                    resourcesToWaitFor.add(value);
                }
            }
            // The expanded set of dependencies, including children of components.
            const expandedDeps = await getAllTransitivelyReferencedResources(resourcesToWaitFor, new Set());
            // Ensure that all resource IDs are known before proceeding.
            for (const dep of expandedDeps.values()) {
                // DependencyResources inherit from CustomResource, but they don't set the id. Skip them.
                if (CustomResource.isInstance(dep) && dep.id) {
                    const known = await dep.id.isKnown;
                    if (!known) {
                        return {
                            result: {},
                            isKnown: false,
                            containsSecrets: false,
                            dependencies: [],
                        };
                    }
                }
            }
        }

        log.debug(
            `Invoke RPC prepared: tok=${tok}` + excessiveDebugOutput ? `, obj=${JSON.stringify(serialized)}` : ``,
        );

        // Fetch the monitor and make an RPC request.
        const monitor: any = getMonitor();

        const provider = await ProviderResource.register(getProvider(tok, opts));
        // keep track of the the secretness of the inputs
        // if any of the inputs are secret, the invoke response should be marked as secret
        const [plainInputs, inputsContainSecrets] = unwrapSecretValues(serialized);
        const req = await createInvokeRequest(tok, plainInputs, provider, opts, packageRef);

        const resp: any = await debuggablePromise(
            new Promise((innerResolve, innerReject) =>
                monitor.invoke(req, (err: grpc.ServiceError, innerResponse: any) => {
                    log.debug(`Invoke RPC finished: tok=${tok}; err: ${err}, resp: ${innerResponse}`);
                    if (err) {
                        // If the monitor is unavailable, it is in the process of shutting down or has already
                        // shut down. Don't emit an error and don't do any more RPCs, just exit.
                        if (err.code === grpc.status.UNAVAILABLE || err.code === grpc.status.CANCELLED) {
                            terminateRpcs();
                            err.message = "Resource monitor is terminating";
                            innerReject(err);
                            return;
                        }

                        // If the RPC failed, rethrow the error with a native exception and the message that
                        // the engine provided - it's suitable for user presentation.
                        innerReject(new Error(err.details));
                    } else {
                        innerResolve(innerResponse);
                    }
                }),
            ),
            label,
        );

        const flatDependencies: Resource[] = dependsOnDeps;
        for (const dep of deps.values()) {
            for (const d of dep) {
                if (!flatDependencies.includes(d)) {
                    flatDependencies.push(d);
                }
            }
        }

        // Finally propagate any other properties that were given to us as outputs.
        const deserialized = deserializeResponse(tok, resp);
        return {
            result: deserialized.result,
            containsSecrets: deserialized.containsSecrets || inputsContainSecrets,
            dependencies: flatDependencies,
            isKnown: true,
        };
    } finally {
        done();
    }
}

/**
 * {@link StreamInvokeResponse} represents a (potentially infinite) streaming
 * response to `streamInvoke`, with facilities to gracefully cancel and clean up
 * the stream.
 */
export class StreamInvokeResponse<T> implements AsyncIterable<T> {
    constructor(
        private source: AsyncIterable<T>,
        private cancelSource: () => void,
    ) {}

    // cancel signals the `streamInvoke` should be cancelled and cleaned up gracefully.
    public cancel() {
        this.cancelSource();
    }

    [Symbol.asyncIterator]() {
        return this.source[Symbol.asyncIterator]();
    }
}

async function createInvokeRequest(
    tok: string,
    serialized: any,
    provider: string | undefined,
    opts: InvokeOptions,
    packageRef?: Promise<string | undefined>,
) {
    if (provider !== undefined && typeof provider !== "string") {
        throw new Error("Incorrect provider type.");
    }

    const obj = gstruct.Struct.fromJavaScript(serialized);
    let packageRefStr = undefined;
    if (packageRef !== undefined) {
        packageRefStr = await packageRef;
        if (packageRefStr !== undefined) {
            // If we have a package reference we can clear some of the resource options
            opts.version = undefined;
            opts.pluginDownloadURL = undefined;
        }
    }
    const req = new resourceproto.ResourceInvokeRequest();
    req.setTok(tok);
    req.setArgs(obj);
    req.setProvider(provider || "");
    req.setVersion(opts.version || "");
    req.setPlugindownloadurl(opts.pluginDownloadURL || "");
    req.setAcceptresources(!utils.disableResourceReferences);
    req.setPackageref(packageRefStr || "");
    return req;
}

function getProvider(tok: string, opts: InvokeOptions) {
    return opts.provider ? opts.provider : opts.parent ? opts.parent.getProvider(tok) : undefined;
}

function deserializeResponse(
    tok: string,
    resp: { getFailuresList(): Array<providerproto.CheckFailure>; getReturn(): gstruct.Struct | undefined },
): {
    result: Inputs | undefined;
    containsSecrets: boolean;
} {
    const failures = resp.getFailuresList();
    if (failures?.length) {
        let reasons = "";
        for (let i = 0; i < failures.length; i++) {
            if (reasons !== "") {
                reasons += "; ";
            }

            reasons += `${failures[i].getReason()} (${failures[i].getProperty()})`;
        }

        throw new Error(`Invoke of '${tok}' failed: ${reasons}`);
    }

    let containsSecrets = false;
    const result = resp.getReturn();
    if (result === undefined) {
        return {
            result,
            containsSecrets,
        };
    }

    const properties = deserializeProperties(result);
    // Keep track of whether we need to mark the resulting output a secret.
    // and unwrap each individual value if it is a secret.
    for (const key of Object.keys(properties)) {
        containsSecrets = containsSecrets || isRpcSecret(properties[key]);
        properties[key] = unwrapRpcSecret(properties[key]);
    }

    return {
        result: properties,
        containsSecrets,
    };
}

/**
 * Dynamically calls the function `tok`, which is offered by a provider plugin.
 */
export function call<T>(
    tok: string,
    props: Inputs,
    res?: Resource,
    packageRef?: Promise<string | undefined>,
): Output<T> {
    const label = `Calling function: tok=${tok}`;
    log.debug(label + (excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``));

    const [out, resolver] = createOutput<T>(`call(${tok})`);

    debuggablePromise(
        Promise.resolve().then(async () => {
            const done = rpcKeepAlive();
            try {
                // Construct a provider reference from the given provider, if one is available on the resource.
                let provider: string | undefined = undefined;
                let version: string | undefined = undefined;
                let pluginDownloadURL: string | undefined = undefined;
                if (res) {
                    if (res.__prov) {
                        provider = await ProviderResource.register(res.__prov);
                    }
                    version = res.__version;
                    pluginDownloadURL = res.__pluginDownloadURL;
                }

                const [serialized, propertyDepsResources] = await serializePropertiesReturnDeps(`call:${tok}`, props, {
                    // We keep output values when serializing inputs for call.
                    keepOutputValues: true,
                    // We exclude resource references from 'argDependencies' when serializing inputs for call.
                    // This way, component providers creating outputs for component inputs based on
                    // 'argDependencies' won't create outputs for properties that only contain resource references.
                    excludeResourceReferencesFromDependencies: true,
                });
                log.debug(
                    `Call RPC prepared: tok=${tok}` + excessiveDebugOutput ? `, obj=${JSON.stringify(serialized)}` : ``,
                );

                const req = await createCallRequest(
                    tok,
                    serialized,
                    propertyDepsResources,
                    provider,
                    version,
                    pluginDownloadURL,
                    packageRef,
                );

                const monitor = getMonitor();
                const resp = await debuggablePromise(
                    new Promise<providerproto.CallResponse>((innerResolve, innerReject) => {
                        if (monitor === undefined) {
                            throw new Error("No monitor available");
                        }

                        monitor.call(
                            req,
                            (err: grpc.ServiceError | null, innerResponse: providerproto.CallResponse) => {
                                log.debug(`Call RPC finished: tok=${tok}; err: ${err}, resp: ${innerResponse}`);
                                if (err) {
                                    // If the monitor is unavailable, it is in the process of shutting down or has already
                                    // shut down. Don't emit an error and don't do any more RPCs, just exit.
                                    if (err.code === grpc.status.UNAVAILABLE || err.code === grpc.status.CANCELLED) {
                                        terminateRpcs();
                                        err.message = "Resource monitor is terminating";
                                        innerReject(err);
                                        return;
                                    }

                                    // If the RPC failed, rethrow the error with a native exception and the message that
                                    // the engine provided - it's suitable for user presentation.
                                    innerReject(new Error(err.details));
                                } else {
                                    innerResolve(innerResponse);
                                }
                            },
                        );
                    }),
                    label,
                );

                // Deserialize the response and resolve the output.
                const { result, containsSecrets } = deserializeResponse(tok, resp);
                const deps: Resource[] = [];

                // Combine the individual dependencies into a single set of dependency resources.
                const rpcDeps = resp.getReturndependenciesMap();
                if (rpcDeps) {
                    const urns = new Set<string>();
                    for (const [, returnDeps] of rpcDeps.entries()) {
                        for (const urn of returnDeps.getUrnsList()) {
                            urns.add(urn);
                        }
                    }
                    for (const urn of urns) {
                        deps.push(new DependencyResource(urn));
                    }
                }

                // If the value the engine handed back is or contains an unknown value, the resolver will mark its value as
                // unknown automatically, so we just pass true for isKnown here. Note that unknown values will only be
                // present during previews (i.e. isDryRun() will be true).
                resolver(<any>result, true, containsSecrets, deps);
            } catch (e) {
                resolver(<any>undefined, true, false, undefined, e);
            } finally {
                done();
            }
        }),
        label,
    );

    return out;
}

function createOutput<T>(
    label: string,
): [Output<T>, (v: T, isKnown: boolean, isSecret: boolean, deps?: Resource[], err?: Error | undefined) => void] {
    let resolveValue: (v: T) => void;
    let rejectValue: (err: Error) => void;
    let resolveIsKnown: (v: boolean) => void;
    let rejectIsKnown: (err: Error) => void;
    let resolveIsSecret: (v: boolean) => void;
    let rejectIsSecret: (err: Error) => void;
    let resolveDeps: (v: Resource[]) => void;
    let rejectDeps: (err: Error) => void;

    const resolver = (v: T, isKnown: boolean, isSecret: boolean, deps: Resource[] = [], err?: Error) => {
        if (err) {
            rejectValue(err);
            rejectIsKnown(err);
            rejectIsSecret(err);
            rejectDeps(err);
        } else {
            resolveValue(v);
            resolveIsKnown(isKnown);
            resolveIsSecret(isSecret);
            resolveDeps(deps);
        }
    };

    const out = new Output(
        [],
        debuggablePromise(
            new Promise<T>((resolve, reject) => {
                resolveValue = resolve;
                rejectValue = reject;
            }),
            `${label}Value`,
        ),
        debuggablePromise(
            new Promise<boolean>((resolve, reject) => {
                resolveIsKnown = resolve;
                rejectIsKnown = reject;
            }),
            `${label}IsKnown`,
        ),
        debuggablePromise(
            new Promise<boolean>((resolve, reject) => {
                resolveIsSecret = resolve;
                rejectIsSecret = reject;
            }),
            `${label}IsSecret`,
        ),
        debuggablePromise(
            new Promise<Resource[]>((resolve, reject) => {
                resolveDeps = resolve;
                rejectDeps = reject;
            }),
            `${label}Deps`,
        ),
    );

    return [out, resolver];
}

async function createCallRequest(
    tok: string,
    serialized: Record<string, any>,
    serializedDeps: Map<string, Set<Resource>>,
    provider?: string,
    version?: string,
    pluginDownloadURL?: string,
    packageRef?: Promise<string | undefined>,
) {
    if (provider !== undefined && typeof provider !== "string") {
        throw new Error("Incorrect provider type.");
    }

    const obj = gstruct.Struct.fromJavaScript(serialized);
    let packageRefStr = undefined;
    if (packageRef !== undefined) {
        packageRefStr = await packageRef;
        if (packageRefStr !== undefined) {
            // If we have a package reference we can clear some of the resource options
            version = undefined;
            pluginDownloadURL = undefined;
        }
    }

    const req = new resourceproto.ResourceCallRequest();
    req.setTok(tok);
    req.setArgs(obj);
    req.setProvider(provider || "");
    req.setVersion(version || "");
    req.setPlugindownloadurl(pluginDownloadURL || "");
    req.setPackageref(packageRefStr || "");

    const argDependencies = req.getArgdependenciesMap();
    for (const [key, propertyDeps] of serializedDeps) {
        const urns = new Set<string>();
        for (const dep of propertyDeps) {
            const urn = await dep.urn.promise();
            urns.add(urn);
        }
        const deps = new resourceproto.ResourceCallRequest.ArgumentDependencies();
        deps.setUrnsList(Array.from(urns));
        argDependencies.set(key, deps);
    }

    return req;
}
