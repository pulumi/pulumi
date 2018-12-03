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

import * as fs from "fs";
import * as grpc from "grpc";
import { InvokeOptions } from "../invoke";
import * as log from "../log";
import { Inputs } from "../output";
import { debuggablePromise } from "./debuggable";
import { deserializeProperties, serializeProperties, unknownValue } from "./rpc";
import { excessiveDebugOutput, getMonitor, getSyncInvokes, rpcKeepAlive } from "./settings";

import { ProviderRef, Resource } from "../resource";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
const providerproto = require("../proto/provider_pb.js");

/**
 * invoke dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs
 * can be a bag of computed values (Ts or Promise<T>s), and the result is a Promise<any> that
 * resolves when the invoke finishes.
 */
export async function invoke(tok: string, props: Inputs, opts: InvokeOptions = {}): Promise<any> {
    const label = `Invoking function: tok=${tok}`;
    log.debug(label +
        excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``);

    const provider =
        opts.provider ? opts.provider :
        opts.parent ? opts.parent.getProvider(tok) : undefined;

    // Wait for all values to be available, and then perform the RPC.
    const done = rpcKeepAlive();
    try {
        const obj = gstruct.Struct.fromJavaScript(
            await serializeProperties(`invoke:${tok}`, props));
        log.debug(`Invoke RPC prepared: tok=${tok}` + excessiveDebugOutput ? `, obj=${JSON.stringify(obj)}` : ``);

        // Fetch the monitor and make an RPC request.
        const monitor: any = getMonitor();

        let providerRef: string | undefined;
        if (ProviderRef.isInstance(provider)) {
            providerRef = provider.getValue();
        }
        else if (Resource.isInstance(provider)) {
            providerRef = (await ProviderRef.get(provider)).getValue();
        }

        const req = new providerproto.InvokeRequest();
        req.setTok(tok);
        req.setArgs(obj);
        req.setProvider(providerRef);
        req.setVersion(opts.version || "");
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

        // If there were failures, propagate them.
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

        // Finally propagate any other properties that were given to us as outputs.
        return deserializeProperties(resp.getReturn());
    }
    finally {
        done();
    }
}

/**
 * invokeSync dynamically invokes the function, tok, which is offered by a provider plugin.
 */
export function invokeSync(tok: string, props: any): any {
    const label = `Invoking function: tok=${tok}`;
    log.debug(label +
        excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``);

    const obj = gstruct.Struct.fromJavaScript(props);
    log.debug(`Invoke RPC prepared: tok=${tok}` + excessiveDebugOutput ? `, obj=${JSON.stringify(obj)}` : ``);

    // Fetch the sync monitor and make an RPC request.
    const syncInvokes = getSyncInvokes();

    const req = new providerproto.InvokeRequest();
    req.setTok(tok);
    req.setArgs(obj);

    // Encode the request.
    const reqBytes = Buffer.from(req.serializeBinary());

    // Write the request length.
    const reqLen = Buffer.alloc(4);
    reqLen.writeUInt32BE(reqBytes.length, 0);
    fs.writeSync(syncInvokes.requests, reqLen);
    fs.writeSync(syncInvokes.requests, reqBytes);

    // Read the response.
    const respLenBytes = Buffer.alloc(4);
    fs.readSync(syncInvokes.responses, respLenBytes, 0, 4, null);
    const respLen = respLenBytes.readUInt32BE(0);
    const respBytes = Buffer.alloc(respLen);
    fs.readSync(syncInvokes.responses, respBytes, 0, respLen, null);

    // Decode the response.
    const resp = providerproto.InvokeResponse.deserializeBinary(new Uint8Array(respBytes));

    // If there were failures, propagate them.
    const failures: any = resp.getFailuresList();
    if (failures && failures.length) {
        throw new Error(`Invoke of '${tok}' failed: ${failures[0].reason} (${failures[0].property})`);
    }

    // Finally propagate any other properties that were given to us as outputs.
    return deserializeProperties(resp.getReturn());
}
