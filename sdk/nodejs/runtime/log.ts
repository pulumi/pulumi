// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { getEngine, rpcKeepAlive } from "./settings";
let engproto = require("../proto/engine_pb.js");

// Log offers the ability to log messages in a way that integrate tightly with the resource engine's interface.
export class Log {
    private static errcnt = 0;
    private static lastLog: Promise<any> = Promise.resolve();

    // hasErrors returns true if any errors have occurred in the program.
    public static hasErrors(): boolean {
        return Log.errcnt > 0;
    }

    // debug logs a debug-level message that is generally hidden from end-users.
    public static debug(msg: string): void {
        let engine: Object | undefined = getEngine();
        if (engine) {
            Log.log(engine, engproto.LogSeverity.DEBUG, msg);
        }
        else {
            // ignore debug messages when no engine is available.
        }
    }

    // info logs an informational message that is generally printed to stdout during resource operations.
    public static info(msg: string): void {
        let engine: Object | undefined = getEngine();
        if (engine) {
            Log.log(engine, engproto.LogSeverity.INFO, msg);
        }
        else {
            console.log(`info: [runtime] ${msg}`);
        }
    }

    // warn logs a warning to indicate that something went wrong, but not catastrophically so.
    public static warn(msg: string): void {
        let engine: Object | undefined = getEngine();
        if (engine) {
            Log.log(engine, engproto.LogSeverity.WARNING, msg);
        }
        else {
            console.warn(`warning: [runtime] ${msg}`);
        }
    }

    // error logs a fatal error to indicate that the tool should stop processing resource operations immediately.
    public static error(msg: string): void {
        Log.errcnt++; // remember the error so we can suppress leaks.

        let engine: Object | undefined = getEngine();
        if (engine) {
            Log.log(engine, engproto.LogSeverity.ERROR, msg);
        }
        else {
            console.error(`error: [runtime] ${msg}`);
        }
    }

    private static log(engine: any, sev: any, msg: string): void {
        // Ensure we log everything in serial order.
        Log.lastLog = Log.lastLog.then(() => {
            return new Promise((resolve) => {
                let req = new engproto.LogRequest();
                req.setSeverity(sev);
                req.setMessage(msg);
                engine.log(req, () => { resolve(); });
            });
        });
    }
}

