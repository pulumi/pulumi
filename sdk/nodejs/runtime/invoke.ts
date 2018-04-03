// Copyright 2017-2018, Pulumi Corporation.
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

import * as log from "../log";
import { Inputs } from "../resource";
import { debuggablePromise } from "./debuggable";
import { deserializeProperties, serializeProperties } from "./rpc";
import { excessiveDebugOutput, getMonitor, options, rpcKeepAlive, serialize } from "./settings";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
const resproto = require("../proto/resource_pb.js");

/**
 * invoke dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs
 * can be a bag of computed values (Ts or Promise<T>s), and the result is a Promise<any> that
 * resolves when the invoke finishes.
 */
export async function invoke(tok: string, props: Inputs): Promise<any> {
    log.debug(`Invoking function: tok=${tok}` +
        excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``);

    // Wait for all values to be available, and then perform the RPC.
    const done = rpcKeepAlive();
    try {
        const obj = gstruct.Struct.fromJavaScript(
            await serializeProperties(`invoke:${tok}`, props));
        log.debug(`Invoke RPC prepared: tok=${tok}` + excessiveDebugOutput ? `, obj=${JSON.stringify(obj)}` : ``);

        // Fetch the monitor and make an RPC request.
        const monitor: any = getMonitor();

        const req = new resproto.InvokeRequest();
        req.setTok(tok);
        req.setArgs(obj);
        const resp: any = await debuggablePromise(new Promise((innerResolve, innerReject) =>
            monitor.invoke(req, (err: Error, innerResponse: any) => {
                log.debug(`Invoke RPC finished: tok=${tok}; err: ${err}, resp: ${innerResponse}`);
                if (err) {
                    innerReject(err);
                }
                else {
                    innerResolve(innerResponse);
                }
            })));

        // If there were failures, propagate them.
        const failures: any = resp.getFailuresList();
        if (failures && failures.length) {
            throw new Error(`Invoke of '${tok}' failed: ${failures[0].reason} (${failures[0].property})`);
        }

        // Finally propagate any other properties that were given to us as outputs.
        return deserializeProperties(resp.getReturn());
    }
    finally {
        done();
    }
}

