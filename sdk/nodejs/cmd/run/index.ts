// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This is the entrypoint for running a Node.js program with minimal scaffolding.

import * as minimist from "minimist";
import * as path from "path";
import * as pulumi from "../../";
import { RunError } from "../../errors";
import * as log from "../../log";
import * as runtime from "../../runtime";

const grpc = require("grpc");
const engrpc = require("../../proto/engine_grpc_pb.js");
const resrpc = require("../../proto/resource_grpc_pb.js");

function usage(): void {
    console.error(`usage: RUN <flags> [program] <[arg]...>`);
    console.error(``);
    console.error(`    where [flags] may include`);
    console.error(`        --project=p         set the project name to p`);
    console.error(`        --stack=s           set the stack name to s`);
    console.error(`        --config.k=v...     set runtime config key k to value v`);
    console.error(`        --parallel=p        run up to p resource operations in parallel (default is serial)`);
    console.error(`        --dry-run           true to simulate resource changes, but without making them`);
    console.error(`        --pwd=pwd           change the working directory before running the program`);
    console.error(`        --monitor=addr      [required] the RPC address for a resource monitor to connect to`);
    console.error(`        --engine=addr       the RPC address for a resource engine to connect to`);
    console.error(`        --tracing=url       a Zipkin-compatible endpoint to send tracing data to`);
    console.error(``);
    console.error(`    and [program] is a JavaScript program to run in Node.js, and [arg]... optional args to it.`);
}

function printErrorUsageAndExit(message: string): never {
    console.error(message);
    usage();
    return process.exit(-1);
}

export function main(args: string[]): void {
    // See usage above for the intended usage of this program, including flags and required args.
    const config: {[key: string]: string} = {};
    const argv: minimist.ParsedArgs = minimist(args, {
        boolean: [ "dry-run" ],
        string: [ "project", "stack", "parallel", "pwd", "monitor", "engine", "tracing" ],
        unknown: (arg: string) => {
            if (arg.indexOf("-") === 0) {
                return printErrorUsageAndExit(`error: Unrecognized flag ${arg}`);
            }
            return true;
        },
        stopEarly: true,
    });

    // If any config variables are present, parse and set them, so the subsequent accesses are fast.
    const envObject: {[key: string]: string} = runtime.getConfigEnv();
    for (const key of Object.keys(envObject)) {
        runtime.setConfig(key, envObject[key]);
    }

    // If there is a --project=p, and/or a --stack=s, use them in the options.
    const project: string | undefined = argv["project"];
    const stack: string | undefined = argv["stack"];

    // If there is a --pwd directive, switch directories.
    const pwd: string | undefined = argv["pwd"];
    if (pwd) {
        process.chdir(pwd);
    }

    // If resource parallelism was requested, turn it on.
    let parallel: number | undefined;
    if (argv["parallel"]) {
        parallel = parseInt(argv["parallel"], 10);
        if (isNaN(parallel)) {
            return printErrorUsageAndExit(
                `error: --parallel flag must specify a number: ${argv["parallel"]} is not a number`);
        }
    }

    // If ther is a --dry-run directive, flip the switch.  This controls whether we are planning vs. really doing it.
    const dryRun: boolean = !!(argv["dry-run"]);

    // If there is a monitor argument, connect to it.
    const monitorAddr = argv["monitor"];
    if (!monitorAddr) {
        return printErrorUsageAndExit(`error: --monitor=addr must be provided.`);
    }

    const monitor = new resrpc.ResourceMonitorClient(monitorAddr, grpc.credentials.createInsecure());

    // If there is an engine argument, connect to it too.
    let engine: Object | undefined;
    const engineAddr: string | undefined = argv["engine"];
    if (engineAddr) {
        engine = new engrpc.EngineClient(engineAddr, grpc.credentials.createInsecure());
    }

    // Now configure the runtime and get it ready to run the program.
    runtime.configure({
        project: project,
        stack: stack,
        dryRun: dryRun,
        parallel: parallel,
        monitor: monitor,
        engine: engine,
    });

    // Pluck out the program and arguments.
    if (argv._.length === 0) {
        return printErrorUsageAndExit("error: Missing program to execute");
    }
    let program: string = argv._[0];
    if (program.indexOf("/") !== 0) {
        // If this isn't an absolute path, make it relative to the working directory.
        program = path.join(process.cwd(), program);
    }

    // Now fake out the process-wide argv, to make the program think it was run normally.
    const programArgs: string[] = argv._.slice(1);
    process.argv = [ process.argv[0], process.argv[1], ...programArgs ];

    // Set up the process uncaught exception, unhandled rejection, and program exit handlers.
    let uncaught: Error | undefined;
    const uncaughtHandler = (err: Error) => {
        // First, log the error.
        if (err instanceof RunError) {
            // For errors that are subtypes of RunError, we will print the message without hitting the unhandled error
            // logic, which will dump all sorts of verbose spew like the origin source and stack trace.
            log.error(err.message);
        }
        else {
            log.error(`Running program '${program}' failed with an unhandled exception:`);
            log.error(err);
        }

        // Remember that we failed with an error.  Don't quit just yet so we have a chance to drain the message loop.
        uncaught = err;
    };
    process.on("uncaughtException", uncaughtHandler);
    process.on("unhandledRejection", uncaughtHandler);

    process.on("exit", (code: number) => {
        runtime.disconnectSync();

        // If we don't already have an exit code, and we had an unhandled error, exit with a non-success.
        if (code === 0 && uncaught) {
            process.exit(1);
        }
    });

    // Construct a `Stack` resource to represent the outputs of the program.
    runtime.runInPulumiStack(() => {
        // We run the program inside this context so that it adopts all resources.
        //
        // IDEA: This will miss any resources created on other turns of the event loop.  I think that's a fundamental
        // problem with the current Component design though - not sure what else we could do here.
        //
        // Now go ahead and execute the code. The process will remain alive until the message loop empties.
        log.debug(`Running program '${program}' in pwd '${process.cwd()}' w/ args: ${programArgs}`);
        return require(program);
    });
}

main(process.argv.slice(2));
