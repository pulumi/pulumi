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
import { deserializeProperties, serializeProperties } from "./rpc";
import { excessiveDebugOutput, getMonitor, rpcKeepAlive, SyncInvokes, tryGetSyncInvokes } from "./settings";

import { ProviderResource, Resource } from "../resource";
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
        const call = monitor.streamInvoke(req, {});

        const queue = new PushableAsyncIterable();
        call.on("data", function(thing: any) {
            const live = deserializeResponse(tok, thing);
            queue.push(live);
        });
        call.on("error", (err: any) => {
            if (err.code === 1 && err.details === "Cancelled") {
                return;
            }
            throw err;
        });
        call.on("end", () => {
            queue.complete();
        });

        // Return a cancellable handle to the stream.
        return new StreamInvokeResponse(
            queue,
            () => call.cancel());
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
