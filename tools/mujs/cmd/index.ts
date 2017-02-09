// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import * as minimist from "minimist";
import {fs, log} from "nodejs-ts";
import * as os from "os";
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
            "o": "out",
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

    // Fetch the format and output arguments specified by the user.
    let format: string | undefined = parsed["format"];
    let outfile: string | undefined = parsed["out"];
    if (format) {
        // Ensure that the format has a leading ".".
        if (format && format[0] !== ".") {
            format = "." + format;
        }
    }
    else {
        // If there's an output path, try using its extension name.
        if (outfile) {
            format = fspath.extname(outfile);
        }

        // Otherwise, by default, just use the MuPackage default extension.
        if (!format) {
            format = mujs.pack.defaultFormatExtension;
        }
    }

    // Look up the marshaler for the given extension.  If none is found, it's an invalid extension.
    let marshaler: ((out: any) => string) | undefined = mujs.pack.marshalers.get(format);
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
        if (!outfile) {
            // If no output file was specified, try to use the output specified in the target project file, if any;
            // otherwise, simply store the output in the current directory using the standard Mupack.<ext> name.
            outfile = mujs.pack.mupackBase + format;
            if (result.preferredOut) {
                await fs.mkdirp(result.preferredOut);                // ensure the output directory exists.
                outfile = fspath.join(result.preferredOut, outfile); // and make a file path out of it.
            }
        }

        let blob: string = marshaler(result.pkg);
        if (outfile === "-") {
            // "-" means print to stdout.
            console.log(blob);
        }
        else {
            // Make sure to append a newline to the blob, and write it to disk.
            await fs.writeFile(outfile, blob + os.EOL);
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

