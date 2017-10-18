// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as log from "../log";
import { ComputedValues } from "../resource";
import { debuggablePromise } from "./debuggable";
import { deserializeProperties, PropertyTransfer, transferProperties } from "./rpc";
import { excessiveDebugOutput, getMonitor, options, rpcKeepAlive, serialize } from "./settings";

const langproto = require("../proto/languages_pb.js");

/**
 * invoke dynamically invokes the function, tok, which is offered by a provider plugin.  The inputs can be a bag of
 * computed values (Ts or Promise<T>s), and the result is a Promise<any> that resolves when the invoke finishes.
 */
export function invoke(tok: string, props: ComputedValues | undefined): Promise<any> {
    log.debug(`Invoking function: tok=${tok}` +
        excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``);

    // Pre-allocate an error so we have a clean stack to print even if an asynchronous operation occurs.
    const invokeError: Error = new Error(`Invoke of '${tok}' failed`);

    // Now "transfer" all input properties; this simply awaits any promises and resolves when they all do.
    const transfer: Promise<PropertyTransfer> = debuggablePromise(
        transferProperties(undefined, `invoke:${tok}`, props, undefined));

    const done: () => void = rpcKeepAlive();
    return new Promise<any>(async (resolve, reject) => {
        // Wait for all values to be available, and then perform the RPC.
        try {
            const result: PropertyTransfer = await transfer;
            const obj: any = result.obj;
            log.debug(`Invoke RPC prepared: tok=${tok}` + excessiveDebugOutput ? `, obj=${JSON.stringify(obj)}` : ``);

            // Fetch the monitor and make an RPC request.
            const monitor: any = getMonitor();
            if (monitor) {
                const req = new langproto.InvokeRequest();
                req.setTok(tok);
                req.setArgs(obj);
                const resp: any = await debuggablePromise(new Promise((innerResolve, innerReject) => {
                    monitor.invoke(req, (err: Error, innerResponse: any) => {
                        log.debug(`Invoke RPC finished: tok=${tok}; err: ${err}, resp: ${innerResponse}`);
                        if (err) {
                            innerReject(err);
                        }
                        else {
                            innerResolve(innerResponse);
                        }
                    });
                }));

                // If there were failures, propagate them.
                const failures: any = resp.getFailuresList();
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

