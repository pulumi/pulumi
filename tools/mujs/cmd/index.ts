// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import * as minimist from "minimist";
import {log} from "nodets";
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
        console.log(result.formatDiagnostics());
    }

    if (result.pack) {
        // Now just print the output to the console.
        // TODO(joe): eventually we want a real compiler-like output scheme; for now, just print it.
        console.log(JSON.stringify(result.pack, null, 4));
    }

    return 0;
}

// exit is a workaround for nodejs/node#6456, a longstanding issue wherein there's no good way to synchronize with the
// flushing of console's asynchronous buffered output.  This leads to truncated output, particularly when redirecting.
// As a workaround, we will write to process.stdout/stderr and only invoke process.exit after they have run.
function exit(code: number): void {
    process.stdout.write("", () => {
        process.stderr.write("", () => {
            process.exit(code);
        });
    });
}

// Fire off the main process, and log any errors that go unhandled.
main(process.argv.slice(2)).then(
    (code: number) => {
        exit(code);
    },
    (err: Error) => {
        console.error("Unhandled exception:");
        console.error(err.stack);
        exit(-1);
    },
);

