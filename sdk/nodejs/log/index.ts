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

// The log module logs messages in a way that tightly integrates with the resource engine's interface.

import * as engrpc from "../proto/engine_grpc_pb";
import * as engproto from "../proto/engine_pb";
import * as resourceTypes from "../resource";
import { getEngine, rpcKeepAlive } from "../runtime/settings";
import { getStore } from "../runtime/state";

let lastLog: Promise<any> = Promise.resolve();

const messageLevels = {
    [engproto.LogSeverity.DEBUG]: "debug",
    [engproto.LogSeverity.INFO]: "info",
    [engproto.LogSeverity.WARNING]: "warn",
    [engproto.LogSeverity.ERROR]: "error",
};

/**
 * Returns true if any errors have occurred in the program.
 */
export function hasErrors(): boolean {
    return getStore().logErrorCount > 0;
}

/**
 * Logs a debug-level message that is generally hidden from end-users.
 */
export function debug(msg: string, resource?: resourceTypes.Resource, streamId?: number, ephemeral?: boolean) {
    const engine = getEngine();
    if (engine) {
        return log(engine, engproto.LogSeverity.DEBUG, msg, resource, streamId, ephemeral);
    } else {
        return Promise.resolve();
    }
}

/**
 * Logs an informational message that is generally printed to standard output
 * during resource operations.
 */
export function info(msg: string, resource?: resourceTypes.Resource, streamId?: number, ephemeral?: boolean) {
    const engine = getEngine();
    if (engine) {
        return log(engine, engproto.LogSeverity.INFO, msg, resource, streamId, ephemeral);
    } else {
        console.log(`info: [runtime] ${msg}`);
        return Promise.resolve();
    }
}

/**
 * Logs a warning to indicate that something went wrong, but not
 * catastrophically so.
 */
export function warn(msg: string, resource?: resourceTypes.Resource, streamId?: number, ephemeral?: boolean) {
    const engine = getEngine();
    if (engine) {
        return log(engine, engproto.LogSeverity.WARNING, msg, resource, streamId, ephemeral);
    } else {
        console.warn(`warning: [runtime] ${msg}`);
        return Promise.resolve();
    }
}

/**
 * Logs a fatal condition. Consider raising an exception after calling error to
 * stop the Pulumi program.
 */
export function error(msg: string, resource?: resourceTypes.Resource, streamId?: number, ephemeral?: boolean) {
    getStore().logErrorCount++; // remember the error so we can suppress leaks.

    const engine = getEngine();
    if (engine) {
        return log(engine, engproto.LogSeverity.ERROR, msg, resource, streamId, ephemeral);
    } else {
        console.error(`error: [runtime] ${msg}`);
        return Promise.resolve();
    }
}

function log(
    engine: engrpc.IEngineClient,
    sev: engproto.LogSeverity,
    msg: string,
    resource: resourceTypes.Resource | undefined,
    streamId: number | undefined,
    ephemeral: boolean | undefined,
): Promise<void> {
    // Ensure we log everything in serial order.
    const keepAlive: () => void = rpcKeepAlive();

    const urnPromise = resource ? resource.urn.promise() : Promise.resolve("");

    lastLog = Promise.all([lastLog, urnPromise]).then(([_, urn]) => {
        return new Promise<void>((resolve, reject) => {
            try {
                const req = new engproto.LogRequest();
                req.setSeverity(sev);
                req.setMessage(msg);
                req.setUrn(urn);
                req.setStreamid(streamId === undefined ? 0 : streamId);
                req.setEphemeral(ephemeral === true);
                engine.log(req, () => {
                    resolve(); // let the next log through
                    keepAlive(); // permit RPC channel tear-downs
                });
            } catch (err) {
                reject(err);
            }
        });
    });

    return lastLog.catch((err) => {
        // debug messages never go to stdout/err
        if (sev !== engproto.LogSeverity.DEBUG) {
            // if we're unable to deliver the log message, deliver to stderr instead
            console.error(
                `failed to deliver log message. \nerror: ${err} \noriginal message: ${msg}\n message severity: ${messageLevels[sev]}`,
            );
        }
        // we still need to free up the outstanding promise chain, whether or not delivery succeeded.
        keepAlive();
    });
}
