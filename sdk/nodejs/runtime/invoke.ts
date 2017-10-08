// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as log from "../log";
import { ComputedValues } from "../resource";
import { debuggablePromise } from "./debuggable";
import { deserializeProperties, PropertyTransfer, transferProperties } from "./rpc";
import { excessiveDebugOutput, getMonitor, options, rpcKeepAlive, serialize } from "./settings";

let langproto = require("../proto/languages_pb.js");

/**
 * invoke dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs can be a bag of
 * computed values (Ts or Promise<T>s), and the result is a Promise<any> that resolves when the invoke finishes.
 */
export function invoke(tok: string, props: ComputedValues | undefined): Promise<any> {
    log.debug(`Invoking function: tok=${tok}` +
        excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``);

    // Pre-allocate an error so we have a clean stack to print even if an asynchronous operation occurs.
    let invokeError: Error = new Error(`Invoke of '${tok}' failed`);

    // Now "transfer" all input properties; this simply awaits any promises and resolves when they all do.
    let transfer: Promise<PropertyTransfer> = debuggablePromise(
        transferProperties(undefined, `invoke:${tok}`, props, undefined));

    let done: () => void = rpcKeepAlive();
    return new Promise<any>(async (resolve, reject) => {
        // Wait for all values to be available, and then perform the RPC.
        try {
            let result: PropertyTransfer = await transfer;
            let obj: any = result.obj;
            log.debug(`Invoke RPC prepared: tok=${tok}` + excessiveDebugOutput ? `, obj=${JSON.stringify(obj)}` : ``);

            // Fetch the monitor and make an RPC request.
            let monitor: any = getMonitor();
            if (monitor) {
                let req = new langproto.InvokeRequest();
                req.setTok(tok);
                req.setArgs(obj);
                let resp: any = await debuggablePromise(new Promise((resolve, reject) => {
                    monitor.invoke(req, (err: Error, resp: any) => {
                        log.debug(`Invoke RPC finished: tok=${tok}; err: ${err}, resp: ${resp}`);
                        if (err) {
                            reject(err);
                        }
                        else {
                            resolve(resp);
                        }
                    });
                }));

                // If there were failures, propagate them.
                let failures: any = resp.getFailuresList();
                if (failures && failures.length) {
                    throw new Error(`Invoke of '${tok}' failed: ${failures[0].reason} (${failures[0].property})`);
                }

                // Finally propagate any other properties that were given to us as outputs.
                resolve(deserializeProperties(resp.getReturn()));
            }
            else {
                // If the monitor doesn't exist, still make sure to resolve all properties to undefined.
                log.debug(`Not sending Invoke RPC to monitor -- it doesn't exist: invoke tok=${tok}`);
                resolve(undefined);
            }
        }
        catch (err) {
            reject(err);
        }
        finally {
            done();
        }
    });
}

