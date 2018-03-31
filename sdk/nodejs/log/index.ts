// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

// The log module logs messages in a way that tightly integrates with the resource engine's interface.

import * as util from "util";
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
export function debugX(urn: string, format: any, ...args: any[]): void {
    const msg: string = util.format(format, ...args);
    const engine: Object | undefined = getEngine();
    if (engine) {
        logX(engine, engproto.LogSeverity.DEBUG, urn, msg);
    }
    else {
        // ignore debug messages when no engine is available.
    }
}

/**
 * info logs an informational message that is generally printed to stdout during resource operations.
 */
export function infoX(urn: string, format: any, ...args: any[]): void {
    const msg: string = util.format(format, ...args);
    const engine: Object | undefined = getEngine();
    if (engine) {
        logX(engine, engproto.LogSeverity.INFO, urn, msg);
    }
    else {
        console.log(`info: [runtime] ${msg}`);
    }
}

/**
 * warn logs a warning to indicate that something went wrong, but not catastrophically so.
 */
export function warnX(urn: string, format: any, ...args: any[]): void {
    const msg: string = util.format(format, ...args);
    const engine: Object | undefined = getEngine();
    if (engine) {
        logX(engine, engproto.LogSeverity.WARNING, urn, msg);
    }
    else {
        console.warn(`warning: [runtime] ${msg}`);
    }
}

/**
 * error logs a fatal error to indicate that the tool should stop processing resource operations immediately.
 */
export function errorX(urn: string, format: any, ...args: any[]): void {
    errcnt++; // remember the error so we can suppress leaks.

    const msg: string = util.format(format, ...args);
    const engine: Object | undefined = getEngine();
    if (engine) {
        logX(engine, engproto.LogSeverity.ERROR, urn, msg);
    }
    else {
        console.error(`error: [runtime] ${msg}`);
    }
}

export function logX(engine: any, sev: any, urn: string, format: any, ...args: any[]): void {
    // Ensure we log everything in serial order.
    const msg: string = util.format(format, ...args);
    const keepAlive: () => void = rpcKeepAlive();
    lastLog = lastLog.then(() => {
        return new Promise((resolve) => {
            const req = new engproto.LogRequest();
            req.setSeverity(sev);
            req.setMessage(msg);
            req.setUrn(urn);
            engine.log(req, () => {
                resolve(); // let the next log through
                keepAlive(); // permit RPC channel tear-downs
            });
        });
    });
}

