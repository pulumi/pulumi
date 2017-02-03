// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import * as minimist from "minimist";
import {fs, log} from "nodejs-ts";
import * as fspath from "path";
import * as mujs from "../lib";

async function main(args: string[]): Promise<number> {
    // Parse options.
    let failed: boolean = false;
    let parsed: minimist.ParsedArgs = minimist(args, {
        boolean: [ "debug", "verbose" ],
        string: [ "format", "loglevel", "out" ],
        alias: {
            "f": "format",
            "ll": "loglevel",
            "o": "output",
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

    // By default, store the output in the current directory.
    let output: string =
        parsed["out"] ||
        mujs.pack.mupackBase + mujs.pack.defaultFormatExtension;

    // Pluck out the output format and path information.
    let format: string;
    let marshaler: ((out: any) => string) | undefined;
    if (parsed["format"]) {
        format = parsed["format"];
        if (format && format[0] !== ".") {
            format = "." + format;
        }
        marshaler = mujs.pack.marshalers.get(format)!;
    }
    else {
        format = fspath.extname(output);
        if (!format) {
            format = mujs.pack.defaultFormatExtension;
        }
        marshaler = mujs.pack.marshalers.get(format)!;
    }
    if (!marshaler) {
        let formats: string = "";
        for (let supported of mujs.pack.marshalers) {
            if (formats) {
                formats += ", ";
            }
            formats += supported[0];
        }
        console.error(`Unrecognized MuPackage format extension: ${format};`);
        console.error(`    available formats: ${formats}`);
        return -3;
    }

    // Now go ahead and compile.
    let result: mujs.compiler.CompileResult = await mujs.compiler.compile(path);
    if (result.diagnostics.length > 0) {
        // If any errors occurred, print them out, and skip pretty-printing the AST.
        console.log(result.formatDiagnostics({ colors: true }));
    }

    // Now save (or print) the output so long as there weren't any errors.
    if (result.pkg && mujs.diag.success(result.diagnostics)) {
        let blob: string = marshaler(result.pkg);
        if (output === "-") {
            // "-" means print to stdout.
            console.log(blob);
        }
        else {
            await fs.writeFile(output, blob);
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

