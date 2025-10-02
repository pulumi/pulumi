// Copyright 2016-2022, Pulumi Corporation.
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
import { isGrpcError, ResourceError, RunError } from "../errors";
import * as log from "../log";
import * as runtimeConfig from "../runtime/config";
import * as debuggable from "../runtime/debuggable";
import * as settings from "../runtime/settings";
import * as stack from "../runtime/stack";
import * as localState from "../runtime/state";

import * as langproto from "../proto/language_pb";
import * as plugproto from "../proto/plugin_pb";

/**
 * Raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb).
 *
 * @internal
 */
export const maxRPCMessageSize: number = 1024 * 1024 * 400;

/**
 * @internal
 */
export class LanguageServer<T> implements grpc.UntypedServiceImplementation {
    readonly program: () => Promise<T>;

    // Satisfy the grpc.UntypedServiceImplementation interface.
    [name: string]: any;

    constructor(program: () => Promise<T>) {
        this.program = program;
    }

    onPulumiExit(hasError: boolean) {
        // Check for leaks once the CLI exits but skip if the program otherwise
        // errored to keep error output clean
        if (!hasError) {
            const [leaks, leakMessage] = debuggable.leakedPromises();
            if (leaks.size !== 0) {
                throw new Error(leakMessage);
            }
        }
    }

    getRequiredPlugins(call: any, callback: any): void {
        const resp: any = new langproto.GetRequiredPluginsResponse();
        resp.setPluginsList([]);
        callback(undefined, resp);
    }

    run(call: any, callback: any): Promise<void> {
        const req: any = call.request;
        const resp: any = new langproto.RunResponse();

        // Setup a new async state store for this run
        const store = new localState.LocalStore();
        return localState.asyncLocalStorage.run(store, async () => {
            const errorSet = new Set<Error>();
            const uncaughtHandler = newUncaughtHandler(errorSet);
            try {
                const args = req.getArgsList();
                const engineAddr = args && args.length > 0 ? args[0] : "";

                settings.resetOptions(
                    req.getProject(),
                    req.getStack(),
                    req.getParallel(),
                    engineAddr,
                    req.getMonitorAddress(),
                    req.getDryrun(),
                    req.getOrganization(),
                );

                const config: { [key: string]: string } = {};
                for (const [k, v] of req.getConfigMap()?.entries() || []) {
                    config[<string>k] = <string>v;
                }
                runtimeConfig.setAllConfig(config, req.getConfigsecretkeysList() || []);

                process.setMaxListeners(settings.getMaximumListeners());

                process.on("uncaughtException", uncaughtHandler);
                // @ts-ignore 'unhandledRejection' will almost always invoke uncaughtHandler with an Error. so
                // just suppress the TS strictness here.
                process.on("unhandledRejection", uncaughtHandler);

                try {
                    await stack.runInPulumiStack(this.program);
                    await settings.disconnect(true /* signalShutdown */);
                    process.off("uncaughtException", uncaughtHandler);
                    process.off("unhandledRejection", uncaughtHandler);
                } catch (e) {
                    await settings.disconnect(false /* signalShutdown */);
                    process.off("uncaughtException", uncaughtHandler);
                    process.off("unhandledRejection", uncaughtHandler);

                    if (!isGrpcError(e)) {
                        throw e;
                    }
                }

                if (errorSet.size !== 0 || log.hasErrors()) {
                    let errorMessage: string = "";
                    if (errorSet.size !== 0) {
                        errorMessage = ": ";
                        errorSet.forEach((error) => {
                            errorMessage += `${error.message}, `;
                        });
                        errorMessage = errorMessage.slice(0, -2);
                    } else {
                        errorMessage = ". Check logs for more details";
                    }
                    throw new Error(`One or more errors occurred${errorMessage}`);
                }
            } catch (e) {
                const err = e instanceof Error ? e : new Error(`unknown error ${e}`);
                resp.setError(err.message);
                callback(err, undefined);
            }

            callback(undefined, resp);
        });
    }

    getPluginInfo(call: any, callback: any): void {
        const resp: any = new plugproto.PluginInfo();
        resp.setVersion("1.0.0");
        callback(undefined, resp);
    }
}

function newUncaughtHandler(errorSet: Set<Error>): (err: Error) => void {
    return (err: Error) => {
        // In node, if you throw an error in a chained promise, but the exception is not finally
        // handled, then you can end up getting an unhandledRejection for each exception/promise
        // pair.  Because the exception is the same through all of these, we keep track of it and
        // only report it once so the user doesn't get N messages for the same thing.
        if (errorSet.has(err)) {
            return;
        }

        errorSet.add(err);

        // Default message should be to include the full stack (which includes the message), or
        // fallback to just the message if we can't get the stack.
        //
        // If both the stack and message are empty, then just stringify the err object itself. This
        // is also necessary as users can throw arbitrary things in JS (including non-Errors).
        let defaultMessage = "";
        if (err) {
            defaultMessage = err.stack || err.message || "" + err;
        }

        // First, log the error.
        if (RunError.isInstance(err)) {
            // Always hide the stack for RunErrors.
            log.error(err.message);
        } else if (ResourceError.isInstance(err)) {
            // Hide the stack if requested to by the ResourceError creator.
            const message = err.hideStack ? err.message : defaultMessage;
            log.error(message, err.resource);
        } else if (!isGrpcError(err)) {
            log.error(`Unhandled exception: ${defaultMessage}`);
        }
    };
}
