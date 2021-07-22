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
import * as fs from "fs";

import { AsyncIterable } from "@pulumi/query/interfaces";

import * as asset from "../asset";
import { Config } from "../config";
import { InvokeOptions } from "../invoke";
import * as log from "../log";
import { Inputs, Output } from "../output";
import { debuggablePromise } from "./debuggable";
import { deserializeProperties, isRpcSecret, serializeProperties, serializePropertiesReturnDeps, unwrapRpcSecret } from "./rpc";
import {
    excessiveDebugOutput,
    getMonitor,
    rpcKeepAlive,
    terminateRpcs,
} from "./settings";

import { DependencyResource, ProviderResource, Resource } from "../resource";
import * as utils from "../utils";
import { PushableAsyncIterable } from "./asyncIterableUtil";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
const providerproto = require("../proto/provider_pb.js");

/**
 * `invoke` dynamically invokes the function, `tok`, which is offered by a provider plugin. `invoke`
 * behaves differently in the case that options contains `{async:true}` or not.
 *
 * In the case where `{async:true}` is present in the options bag:
 *
 * 1. the result of `invoke` will be a Promise resolved to the result value of the provider plugin.
 * 2. the `props` inputs can be a bag of computed values (including, `T`s, `Promise<T>`s,
 *    `Output<T>`s etc.).
 *
 *
 * In the case where `{async:true}` is not present in the options bag:
 *
 * 1. the result of `invoke` will be a Promise resolved to the result value of the provider call.
 *    However, that Promise will *also* have the respective values of the Provider result exposed
 *    directly on it as properties.
 *
 * 2. The inputs must be a bag of simple values, and the result is the result that the Provider
 *    produced.
 *
 * Simple values are:
 *  1. `undefined`, `null`, string, number or boolean values.
 *  2. arrays of simple values.
 *  3. objects containing only simple values.
 *
 * Importantly, simple values do *not* include:
 *  1. `Promise`s
 *  2. `Output`s
 *  3. `Asset`s or `Archive`s
 *  4. `Resource`s.
 *
 * All of these contain async values that would prevent `invoke from being able to operate
 * synchronously.
 */
export function invoke(tok: string, props: Inputs, opts: InvokeOptions = {}): Promise<any> {
    return invokeAsync(tok, props, opts);
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
            `StreamInvoke RPC prepared: tok=${tok}` + excessiveDebugOutput
                ? `, obj=${JSON.stringify(serialized)}`
                : ``,
        );

        // Fetch the monitor and make an RPC request.
        const monitor: any = getMonitor();

        const provider = await ProviderResource.register(getProvider(tok, opts));
        const req = createInvokeRequest(tok, serialized, provider, opts);

        // Call `streamInvoke`.
        const result = monitor.streamInvoke(req, {});

        const queue = new PushableAsyncIterable();
        result.on("data", function(thing: any) {
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
        return new StreamInvokeResponse(
            queue,
            () => result.cancel());
    } finally {
        done();
    }
}

async function invokeAsync(tok: string, props: Inputs, opts: InvokeOptions): Promise<any> {
    const label = `Invoking function: tok=${tok} asynchronously`;
    log.debug(label + (excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``));

    // Wait for all values to be available, and then perform the RPC.
    const done = rpcKeepAlive();
    try {
        const serialized = await serializeProperties(`invoke:${tok}`, props);
        log.debug(`Invoke RPC prepared: tok=${tok}` + excessiveDebugOutput ? `, obj=${JSON.stringify(serialized)}` : ``);

        // Fetch the monitor and make an RPC request.
        const monitor: any = getMonitor();

        const provider = await ProviderResource.register(getProvider(tok, opts));
        const req = createInvokeRequest(tok, serialized, provider, opts);

        const resp: any = await debuggablePromise(new Promise((innerResolve, innerReject) =>
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
                }
                else {
                    innerResolve(innerResponse);
                }
            })), label);

        // Finally propagate any other properties that were given to us as outputs.
        return deserializeResponse(tok, resp);
    }
    finally {
        done();
    }
}

// StreamInvokeResponse represents a (potentially infinite) streaming response to `streamInvoke`,
// with facilities to gracefully cancel and clean up the stream.
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

function createInvokeRequest(tok: string, serialized: any, provider: string | undefined, opts: InvokeOptions) {
    if (provider !== undefined && typeof provider !== "string") {
        throw new Error("Incorrect provider type.");
    }

    const obj = gstruct.Struct.fromJavaScript(serialized);

    const req = new providerproto.InvokeRequest();
    req.setTok(tok);
    req.setArgs(obj);
    req.setProvider(provider);
    req.setVersion(opts.version || "");
    req.setAcceptresources(!utils.disableResourceReferences);
    return req;
}

function getProvider(tok: string, opts: InvokeOptions) {
    return opts.provider ? opts.provider :
           opts.parent ? opts.parent.getProvider(tok) : undefined;
}

function deserializeResponse(tok: string, resp: any): any {
    const failures: any = resp.getFailuresList();
    if (failures && failures.length) {
        let reasons = "";
        for (let i = 0; i < failures.length; i++) {
            if (reasons !== "") {
                reasons += "; ";
            }

            reasons += `${failures[i].getReason()} (${failures[i].getProperty()})`;
        }

        throw new Error(`Invoke of '${tok}' failed: ${reasons}`);
    }

    const ret = resp.getReturn();
    return ret === undefined
        ? ret
        : deserializeProperties(ret);
}

/**
 * `call` dynamically calls the function, `tok`, which is offered by a provider plugin.
 */
export function call<T>(tok: string, props: Inputs, res?: Resource): Output<T> {
    const label = `Calling function: tok=${tok}`;
    log.debug(label + (excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``));

    const [out, resolver] = createOutput<T>(`call(${tok})`);

    debuggablePromise(Promise.resolve().then(async () => {
        const done = rpcKeepAlive();
        try {
            // Construct a provider reference from the given provider, if one is available on the resource.
            let provider: string | undefined = undefined;
            let version: string | undefined = undefined;
            if (res) {
                if (res.__prov) {
                    provider = await ProviderResource.register(res.__prov);
                }
                version = res.__version;
            }

            const [serialized, propertyDepsResources] = await serializePropertiesReturnDeps(`call:${tok}`, props);
            log.debug(`Call RPC prepared: tok=${tok}` + excessiveDebugOutput ? `, obj=${JSON.stringify(serialized)}` : ``);

            const req = await createCallRequest(tok, serialized, propertyDepsResources, provider, version);

            const monitor: any = getMonitor();
            const resp: any = await debuggablePromise(new Promise((innerResolve, innerReject) =>
                monitor.call(req, (err: grpc.ServiceError, innerResponse: any) => {
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
                    }
                    else {
                        innerResolve(innerResponse);
                    }
                })), label);

            // Deserialize the response and resolve the output.
            const deserialized = deserializeResponse(tok, resp);
            let isSecret = false;
            const deps: Resource[] = [];

            // Keep track of whether we need to mark the resulting output a secret.
            // and unwrap each individual value.
            for (const k of Object.keys(deserialized)) {
                const v = deserialized[k];
                if (isRpcSecret(v)) {
                    isSecret = true;
                    deserialized[k] = unwrapRpcSecret(v);
                }
            }

            // Combine the individual dependencies into a single set of dependency resources.
            const rpcDeps = resp.getReturndependenciesMap();
            if (rpcDeps) {
                const urns = new Set<string>();
                for (const [k, returnDeps] of rpcDeps.entries()) {
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
            resolver(deserialized, true, isSecret, deps);
        }
        catch (e) {
            resolver(<any>undefined, true, false, undefined, e);
        }
        finally {
            done();
        }
    }), label);

    return out;
}

function createOutput<T>(label: string):
    [Output<T>, (v: T, isKnown: boolean, isSecret: boolean, deps?: Resource[], err?: Error | undefined) => void] {
    let resolveValue: (v: T) => void;
    let rejectValue: (err: Error) => void;
    let resolveIsKnown: (v: boolean) => void;
    let rejectIsKnown: (err: Error) => void;
    let resolveIsSecret: (v: boolean) => void;
    let rejectIsSecret: (err: Error) => void;
    let resolveDeps: (v: Resource[]) => void;
    let rejectDeps: (err: Error) => void;

    const resolver = (v: T, isKnown: boolean, isSecret: boolean, deps: Resource[] = [], err?: Error) => {
        if (!!err) {
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
            `${label}Value`),
        debuggablePromise(
            new Promise<boolean>((resolve, reject) => {
                resolveIsKnown = resolve;
                rejectIsKnown = reject;
            }),
            `${label}IsKnown`),
        debuggablePromise(
            new Promise<boolean>((resolve, reject) => {
                resolveIsSecret = resolve;
                rejectIsSecret = reject;
            }),
            `${label}IsSecret`),
        debuggablePromise(
            new Promise<Resource[]>((resolve, reject) => {
                resolveDeps = resolve;
                rejectDeps = reject;
            }),
            `${label}Deps`));

    return [out, resolver];
}

async function createCallRequest(tok: string, serialized: Record<string, any>,
                                 serializedDeps: Map<string, Set<Resource>>, provider?: string, version?: string) {
    if (provider !== undefined && typeof provider !== "string") {
        throw new Error("Incorrect provider type.");
    }

    const obj = gstruct.Struct.fromJavaScript(serialized);

    const req = new providerproto.CallRequest();
    req.setTok(tok);
    req.setArgs(obj);
    req.setProvider(provider);
    req.setVersion(version || "");

    const argDependencies = req.getArgdependenciesMap();
    for (const [key, propertyDeps] of serializedDeps) {
        const urns = new Set<string>();
        for (const dep of propertyDeps) {
            const urn = await dep.urn.promise();
            urns.add(urn);
        }
        const deps = new providerproto.CallRequest.ArgumentDependencies();
        deps.setUrnsList(Array.from(urns));
        argDependencies.set(key, deps);
    }

    return req;
}
