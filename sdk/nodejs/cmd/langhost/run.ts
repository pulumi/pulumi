// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This is the entrypoint for running a Node.js program with minimal scaffolding.

import * as minimist from "minimist";
import * as path from "path";
import * as runtime from "../../runtime";

let grpc = require("grpc");
let engrpc = require("../../proto/nodejs/engine_grpc_pb");
let langproto = require("../../proto/nodejs/languages_pb");
let langrpc = require("../../proto/nodejs/languages_grpc_pb");

function usage(): void {
    console.error(`usage: RUN <flags> [program] <[arg]...>`);
    console.error(``);
    console.error(`    where [flags] may include`);
    console.error(`        --config.k=v...     set runtime config key k to value v`);
    console.error(`        --dry-run           true to simulate resource changes, but without making them`);
    console.error(`        --pwd=pwd           change the working directory before running the program`);
    console.error(`        --monitor=addr      the RPC address for a resource monitor to connect to`);
    console.error(`        --engine=addr       the RPC address for a resource engine to connect to`);
    console.error(``);
    console.error(`    and [program] is a JavaScript program to run in Node.js, and [arg]... optional args to it.`);
}

export function main(args: string[]): void {
    // See usage above for the intended usage of this program, including flags and required args.
    let config: {[key: string]: string} = {};
    let argv: minimist.ParsedArgs = minimist(args, {
        boolean: [ "dry-run" ],
        string: [ "pwd", "monitor", "engine" ],
        unknown: (arg: string) => {
            // If unknown, first see if it's a --config.k=v flag.
            let cix = arg.indexOf("-config");
            if (cix === 0 || cix === 1) {
                let kix = arg.indexOf(".");
                let vix = arg.indexOf("=");
                if (kix === -1 || vix === -1) {
                    console.error(`fatal: --config flag malformed (expected '--config.key=val')`);
                    usage();
                    process.exit(-1);
                }
                config[arg.substring(kix+1, vix)] = arg.substring(vix+1);
                return false;
            } else if (arg.indexOf("-") === 0) {
                console.error(`fatal: Unrecognized flag ${arg}`);
                usage();
                process.exit(-1);
                return false;
            }
            return true;
        },
        stopEarly: true,
    });

    // Set any configuration keys/values that were found.
    for (let key of Object.keys(config)) {
        runtime.setConfig(key, config[key]);
    }

    // If there is a --pwd directive, switch directories.
    let pwd: string | undefined = argv["pwd"];
    if (pwd) {
        process.chdir(pwd);
    }

    // If ther is a --dry-run directive, flip the switch.  This controls whether we are planning vs. really doing it.
    let dryrun = false;
    if (argv["dry-run"]) {
        dryrun = true;
    }

    // Ensure the monitor argument is present and, if so, connect to it.
    let monitor: Object | undefined;
    let monitorAddr: string | undefined = argv["monitor"];
    if (!monitorAddr) {
        console.error("fatal: Missing required --monitor flag to connect to the resource monitor's RPC");
        usage();
        process.exit(-1);
    } else {
        monitor = new langrpc.ResourceMonitorClient(monitorAddr, grpc.credentials.createInsecure());
    }

    // If there is an engine argument, connect to it too.
    let engine: Object | undefined;
    let engineAddr: string | undefined = argv["engine"];
    if (engineAddr) {
        engine = new engrpc.EngineClient(engineAddr, grpc.credentials.createInsecure());
    }

    // Now configure the runtime and get it ready to run the program.
    runtime.configure(monitor, engine, dryrun);

    // Pluck out the program and arguments.
    if (argv._.length === 0) {
        console.error("fatal: Missing program to execute");
        usage();
        process.exit(-1);
    }
    let program: string = argv._[0];
    if (program.indexOf(".") === 0) {
        // If there was a pwd change, make this relative to it.
        if (pwd) {
            program = path.join(pwd, program);
        }
    } else if (program.indexOf("/") !== 0) {
        // Neither absolute nor relative module, we refuse to execute it.
        console.error(`fatal: Program path '${program}' must be an absolute or relative path to the program`);
        usage();
        process.exit(-1);
    }

    // Now fake out the process-wide argv, to make the program think it was run normally.
    let programArgs: string[] = argv._.slice(1);
    process.argv = [ process.argv[0], process.argv[1], ...programArgs ];

    // Now go ahead and execute the code.  This keeps the process alive until the message loop exits.
    runtime.Log.debug(`Running program '${program}' in pwd '${process.cwd()}' w/ args: ${programArgs}`);
    try {
        require(program);
    }
    catch (err) {
        runtime.Log.debug(`Running program '${program}' failed with an unhandled exception:`);
        runtime.Log.debug(err);
        throw err;
    }
}

main(process.argv.slice(2));

