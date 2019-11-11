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

const asyncify = require("callback-to-async-iterator");
import { AsyncIterable } from "@pulumi/query/interfaces";
import * as fs from "fs";
import * as grpc from "grpc";

import * as asset from "../asset";
import { InvokeOptions } from "../invoke";
import * as log from "../log";
import { Inputs, Output } from "../output";
import { debuggablePromise } from "./debuggable";
import { deserializeProperties, serializeProperties } from "./rpc";
import { excessiveDebugOutput, getMonitor, rpcKeepAlive, SyncInvokes, tryGetSyncInvokes } from "./settings";

import { ProviderResource, Resource } from "../resource";
import * as utils from "../utils";

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
    if (opts.async) {
        // Use specifically requested async invoking.  Respect that.
        return invokeAsync(tok, props, opts);
    }

    const syncInvokes = tryGetSyncInvokes();
    if (!syncInvokes) {
        // We weren't launched from a pulumi CLI that supports sync-invokes.  Let the user know they
        // should update and fall back to synchronously blocking on the async invoke.
        return invokeFallbackToAsync(tok, props, opts);
    }

    return invokeSync(tok, props, opts, syncInvokes);
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
        const call = monitor.streamInvoke(req, {});

        // Transform the callback-oriented `streamInvoke` result into a plain-old (potentially
        // infinite) `AsyncIterable`.
        const listenForWatchEvents = (callback: (obj: any) => void) => {
            return new Promise(resolve => {
                call.on("data", function(thing: any) {
                    const live = deserializeResponse(tok, thing);
                    callback(live);
                });
                call.on("error", (err: any) => {
                    if (err.code === 1 && err.details === "Cancelled") {
                        return;
                    }
                    throw err;
                });
                // Infinite stream, never call `resolve`.
            });
        };
        const stream: AsyncIterable<any> = asyncify.default(listenForWatchEvents);

        // Return a cancellable handle to the stream.
        return new StreamInvokeResponse(
            stream,
            () => call.cancel());
    } finally {
        done();
    }
}

export function invokeFallbackToAsync(tok: string, props: Inputs, opts: InvokeOptions): Promise<any> {
    const asyncResult = invokeAsync(tok, props, opts);
    const syncResult = utils.promiseResult(asyncResult);
    return createLiftedPromise(syncResult);
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
            monitor.invoke(req, (err: grpc.StatusObject, innerResponse: any) => {
                log.debug(`Invoke RPC finished: tok=${tok}; err: ${err}, resp: ${innerResponse}`);
                if (err) {
                    // If the monitor is unavailable, it is in the process of shutting down or has already
                    // shut down. Don't emit an error and don't do any more RPCs, just exit.
                    if (err.code === grpc.status.UNAVAILABLE) {
                        log.debug("Resource monitor is terminating");
                        process.exit(0);
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

function invokeSync(tok: string, props: any, opts: InvokeOptions, syncInvokes: SyncInvokes): Promise<any> {
    const label = `Invoking function: tok=${tok} synchronously`;
    log.debug(label + (excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``));

    const serialized = serializePropertiesSync(props);
    log.debug(`Invoke RPC prepared: tok=${tok}` + excessiveDebugOutput ? `, obj=${JSON.stringify(serialized)}` : ``);

    const providerRef = getProviderRefSync();
    const req = createInvokeRequest(tok, serialized, providerRef, opts);

    // Encode the request.
    const reqBytes = Buffer.from(req.serializeBinary());

    // Write the request length.
    const reqLen = Buffer.alloc(4);
    reqLen.writeUInt32BE(reqBytes.length, /*offset:*/ 0);
    fs.writeSync(syncInvokes.requests, reqLen);
    fs.writeSync(syncInvokes.requests, reqBytes);

    // Read the response.
    const respLenBytes = Buffer.alloc(4);
    fs.readSync(syncInvokes.responses, respLenBytes, /*offset:*/ 0, /*length:*/ 4, /*position:*/ null);
    const respLen = respLenBytes.readUInt32BE(/*offset:*/ 0);
    const respBytes = Buffer.alloc(respLen);
    fs.readSync(syncInvokes.responses, respBytes, /*offset:*/ 0, /*length:*/ respLen, /*position:*/ null);

    // Decode the response.
    const resp = providerproto.InvokeResponse.deserializeBinary(new Uint8Array(respBytes));
    const resultValue = deserializeResponse(tok, resp);

    return createLiftedPromise(resultValue);

    function getProviderRefSync() {
        const provider = getProvider(tok, opts);

        if (provider === undefined) {
            return undefined;
        }

        if (provider.__registrationId === undefined) {
            log.warn(
`Synchronous call made to "${tok}" with an unregistered provider.
For more details see: https://www.pulumi.com/docs/troubleshooting/#synchronous-call`);
            utils.promiseResult(ProviderResource.register(provider));
        }

        return provider.__registrationId;
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

// Expose the properties of the actual result of invoke directly on the promise itself. Note this
// doesn't actually involve any asynchrony.  The promise will be created synchronously and the
// values copied to it can be used immediately.  We simply make a Promise so that any consumers that
// do a `.then()` on it continue to work even though we've switched from being async to sync.
function createLiftedPromise(value: any): Promise<any> {
    const promise = Promise.resolve(value);
    Object.assign(promise, value);
    return promise;
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
    return req;
}

function getProvider(tok: string, opts: InvokeOptions) {
    return opts.provider ? opts.provider :
           opts.parent ? opts.parent.getProvider(tok) : undefined;
}

function serializePropertiesSync(prop: any): any {
    if (prop === undefined ||
        prop === null ||
        typeof prop === "boolean" ||
        typeof prop === "number" ||
        typeof prop === "string") {

        return prop;
    }

    if (asset.Asset.isInstance(prop) || asset.Archive.isInstance(prop)) {
        throw new Error("Assets and Archives cannot be passed in as arguments to a data source call.");
    }

    if (prop instanceof Promise) {
        throw new Error("Promises cannot be passed in as arguments to a data source call.");
    }

    if (Output.isInstance(prop)) {
        throw new Error("Outputs cannot be passed in as arguments to a data source call.");
    }

    if (Resource.isInstance(prop)) {
        throw new Error("Resources cannot be passed in as arguments to a data source call.");
    }

    if (prop instanceof Array) {
        const result: any[] = [];
        for (let i = 0; i < prop.length; i++) {
            // When serializing arrays, we serialize any undefined values as `null`. This matches JSON semantics.
            const elem = serializePropertiesSync(prop[i]);
            result.push(elem === undefined ? null : elem);
        }
        return result;
    }

    return serializeAllKeys(prop, {});

    function serializeAllKeys(innerProp: any, obj: any) {
        for (const k of Object.keys(innerProp)) {
            // When serializing an object, we omit any keys with undefined values. This matches JSON semantics.
            const v = serializePropertiesSync(innerProp[k]);
            if (v !== undefined) {
                obj[k] = v;
            }
        }

        return obj;
    }
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
