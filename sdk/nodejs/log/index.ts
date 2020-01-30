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

import * as resourceTypes from "../resource";
import { getEngine, rpcKeepAlive } from "../runtime/settings";
const engproto = require("../proto/engine_pb.js");

let errcnt = 0;
let lastLog: Promise<any> = Promise.resolve();

/**
 * hasErrors returns true if any errors have occurred in the program.
 */
export function hasErrors(): boolean {
    return errcnt > 0;
}

/**
 * debug logs a debug-level message that is generally hidden from end-users.
 */
export async function debug(msg: string, resource?: resourceTypes.Resource, streamId?: number, ephemeral?: boolean) {
    const engine: Object | undefined = getEngine();
    if (engine) {
        log(engine, engproto.LogSeverity.DEBUG, msg, resource, streamId, ephemeral);
    }
    else {
        // ignore debug messages when no engine is available.
    }
}

/**
 * info logs an informational message that is generally printed to stdout during resource operations.
 */
export function info(msg: string, resource?: resourceTypes.Resource, streamId?: number, ephemeral?: boolean) {
    const engine: Object | undefined = getEngine();
    if (engine) {
        return log(engine, engproto.LogSeverity.INFO, msg, resource, streamId, ephemeral);
    }
    else {
        console.log(`info: [runtime] ${msg}`);
        return Promise.resolve();
    }
}

/**
 * warn logs a warning to indicate that something went wrong, but not catastrophically so.
 */
export function warn(msg: string, resource?: resourceTypes.Resource, streamId?: number, ephemeral?: boolean) {
    const engine: Object | undefined = getEngine();
    if (engine) {
        return log(engine, engproto.LogSeverity.WARNING, msg, resource, streamId, ephemeral);
    }
    else {
        console.warn(`warning: [runtime] ${msg}`);
        return Promise.resolve();
    }
}

/**
 * error logs a fatal error to indicate that the tool should stop processing resource operations immediately.
 */
export function error(msg: string, resource?: resourceTypes.Resource, streamId?: number, ephemeral?: boolean) {
    errcnt++; // remember the error so we can suppress leaks.

    const engine: Object | undefined = getEngine();
    if (engine) {
        return log(engine, engproto.LogSeverity.ERROR, msg, resource, streamId, ephemeral);
    }
    else {
        console.error(`error: [runtime] ${msg}`);
        return Promise.resolve();
    }
}

function log(
        engine: any, sev: any, msg: string,
        resource: resourceTypes.Resource | undefined,
        streamId: number | undefined,
        ephemeral: boolean | undefined): Promise<void> {

    // Ensure we log everything in serial order.
    const keepAlive: () => void = rpcKeepAlive();

    const urnPromise = resource
        ? resource.urn.promise()
        : Promise.resolve("");

    lastLog = Promise.all([lastLog, urnPromise]).then(([_, urn]) => {
        return new Promise((resolve, reject) => {
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
            }
            catch (err) {
                reject(err);
            }
        });
    });

    return lastLog;
}
