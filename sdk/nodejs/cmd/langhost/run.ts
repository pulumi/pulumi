// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This is the entrypoint for running a Node.js program with minimal scaffolding.

import * as path from "path";
import * as runtime from "../../runtime";

let grpc = require("grpc");
let langproto = require("../../proto/nodejs/languages_pb");
let langrpc = require("../../proto/nodejs/languages_grpc_pb");

export function main(args: string[]): void {
    // The format of this program is as follows:
    //     run.js [--config.k v]... [--dry-run] [--pwd <pwd>] <monitor> <program> [arg]...
    // where <monitor> tells us how to talk back to the resource monitor, --config.k=v is an optional repeating set
    // of config flags, pwd is an optional working directory to run the script from, <program> is the Node.js program
    // to execute with args to pass to the program.

    // Pluck out any configuration keys/values.
    let i = 0;
    for (; i < args.length; i++) {
        let arg: string = args[i];
        if (arg.indexOf("--config.") !== 0 && arg.indexOf("-config.") !== 0) {
            break;
        }
        let key: string = arg.substring(arg.indexOf(".")+1);
        if (++i >= args.length) {
            console.error(`fatal: Missing value for configuration key '${key}'`);
            usage();
            process.exit(-1);
        }
        runtime.setConfig(key, args[i]);
    }

    // If ther is a --dry-run directive, flip the switch.  This controls whether we are planning vs. really doing it.
    let dryrun = false;
    if (i < args.length && (args[i] === "--dry-run" || args[i] === "-dry-run")) {
        dryrun = true;
        i++;
    }

    // If there is a --pwd directive, switch directories.
    let pwd: string | undefined;
    if (i < args.length && (args[i] === "--pwd" || args[i] === "-pwd")) {
        if (++i >= args.length) {
            console.error("fatal: Missing directory after --pwd flag");
            usage();
            process.exit(-1);
        }
        pwd = args[i++];
        process.chdir(pwd);
    }

    // Ensure the monitor argument is present and, if so, connect to it.
    if (i >= args.length) {
        console.error("fatal: Missing required monitor argument");
        usage();
        process.exit(-1);
    }
    let monitor = new langrpc.ResourceMonitorClient(args[i++], grpc.credentials.createInsecure());
    runtime.configureMonitor(monitor, dryrun);

    // Pluck out the program and arguments.
    if (i >= args.length) {
        console.error("fatal: Missing program to execute");
        usage();
        process.exit(-1);
    }
    let program: string = args[i++];
    if (program.indexOf(".") === 0) {
        // If there was a pwd change, make this relative to it.
        if (pwd) {
            program = path.join(pwd, program);
        }
    }
    else if (program.indexOf("/") !== 0) {
        // Neither absolute nor relative module, we refuse to execute it.
        console.error(`fatal: Program path '${program}' must be an absolute or relative path to the program`);
        usage();
        process.exit(-1);
    }

    let programArgs: string[] = [];
    for (; i < args.length; i++) {
        programArgs.push(args[i]);
    }
    process.argv = [ process.argv[0], process.argv[1], ...programArgs ];

    // Now go ahead and execute the code.  This keeps the process alive until the message loop exits.
    require(program);
}

function usage(): void {
    console.error(`usage: RUN [--config.k v]... [--dry-run] [--pwd <pwd>] <monitor> <program> [arg]...`);
}

main(process.argv.slice(2));

