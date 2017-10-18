// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// The log module logs messages in a way that tightly integrates with the resource engine's interface.

import { getEngine, rpcKeepAlive } from "../runtime";
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
export function debug(msg: string): void {
    const engine: Object | undefined = getEngine();
    if (engine) {
        log(engine, engproto.LogSeverity.DEBUG, msg);
    }
    else {
        // ignore debug messages when no engine is available.
    }
}

/**
 * info logs an informational message that is generally printed to stdout during resource operations.
 */
export function info(msg: string): void {
    const engine: Object | undefined = getEngine();
    if (engine) {
        log(engine, engproto.LogSeverity.INFO, msg);
    }
    else {
        console.log(`info: [runtime] ${msg}`);
    }
}

/**
 * warn logs a warning to indicate that something went wrong, but not catastrophically so.
 */
export function warn(msg: string): void {
    const engine: Object | undefined = getEngine();
    if (engine) {
        log(engine, engproto.LogSeverity.WARNING, msg);
    }
    else {
        console.warn(`warning: [runtime] ${msg}`);
    }
}

/**
 * error logs a fatal error to indicate that the tool should stop processing resource operations immediately.
 */
export function error(msg: string): void {
    errcnt++; // remember the error so we can suppress leaks.

    const engine: Object | undefined = getEngine();
    if (engine) {
        log(engine, engproto.LogSeverity.ERROR, msg);
    }
    else {
        console.error(`error: [runtime] ${msg}`);
    }
}

export function log(engine: any, sev: any, msg: string): void {
    // Ensure we log everything in serial order.
    const keepAlive: () => void = rpcKeepAlive();
    lastLog = lastLog.then(() => {
        return new Promise((resolve) => {
            const req = new engproto.LogRequest();
            req.setSeverity(sev);
            req.setMessage(msg);
            engine.log(req, () => {
                resolve(); // let the next log through
                keepAlive(); // permit RPC channel tear-downs
            });
        });
    });
}

