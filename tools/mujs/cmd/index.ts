// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import * as minimist from "minimist";
import {log} from "nodejs-ts";
import * as mujs from "../lib";

async function main(args: string[]): Promise<number> {
    // Parse options.
    let failed: boolean = false;
    let parsed: minimist.ParsedArgs = minimist(args, {
        boolean: [ "debug", "verbose" ],
        string: [ "loglevel" ],
        alias: {
            "ll": "loglevel",
        },
        unknown: (arg: string) => {
            if (arg[0] === "-") {
                console.error(`Unrecognized option '${arg}'`);
                failed = true;
                return false;
            }
            return true;
        },
    });
    if (failed) {
        return -2;
    }
    args = parsed._;

    // If higher logging levels were requested, set them.
    if (parsed["debug"]) {
        log.configure(7);
    }
    else if (parsed["verbose"]) {
        log.configure(5);
    }
    else if (parsed["loglevel"]) {
        let ll: number = parseInt(parsed["loglevel"], 10);
        log.configure(ll);
    }

    // Now check for required arguments.
    let path: string =
        args.length > 0 ?
            args[0] :
            // Default to pwd if no argument was supplied.
            process.cwd();

    let result: mujs.compiler.CompileResult = await mujs.compiler.compile(path);
    if (result.diagnostics.length > 0) {
        // If any errors occurred, print them out, and skip pretty-printing the AST.
        console.log(result.formatDiagnostics({ colors: true }));
    }

    if (result.pkg) {
        // Now just print the output to the console, but only if there weren't any errors.
        let hadError: boolean = false;
        for (let diag of result.diagnostics) {
            if (diag.category === mujs.diag.DiagnosticCategory.Error) {
                hadError = true;
                break;
            }
        }
        if (!hadError) {
            // TODO(joe): eventually we want a real compiler-like output scheme; for now, just print it.
            console.log(JSON.stringify(result.pkg, null, 4));
        }
    }

    return 0;
}

// Fire off the main process, and log any errors that go unhandled.
main(process.argv.slice(2)).then(
    (code: number) => {
        // To exit gracefully, simply set the `process.exitCode` and allow the process to exit naturally.  We presume
        // that the return of the `main` method indicates that the event loop has quiesced.  Doing it this way avoids
        // truncating stdout output (see https://nodejs.org/api/process.html#process_process_exit_code).
        process.exitCode = code;
    },
    (err: Error) => {
        // An unhandled exception will lead to a rude process termination.  Although this might not flush all pending
        // stdout output, we are guaranteed that pending stderrs will have been flushed.  This ensures that we exit
        // immediately, even if there are asynchronous activities in flight (plausible given the unhandled exception).
        console.error("Unhandled exception:");
        console.error(err.stack);
        process.exit(-1);
    },
);

