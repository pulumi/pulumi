// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This is the entrypoint for running a Node.js program with minimal scaffolding.
import * as minimist from "minimist";

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

function main(args: string[]): void {
    // See usage above for the intended usage of this program, including flags and required args.
    const argv: minimist.ParsedArgs = minimist(args, {
        boolean: [ "dry-run" ],
        string: [ "project", "stack", "parallel", "pwd", "monitor", "engine", "tracing" ],
        unknown: (arg: string) => {
            return true;
        },
        stopEarly: true,
    });

    // If parallel was passed, validate it is an number
    if (argv["parallel"]) {
        if (isNaN(parseInt(argv["parallel"], 10))) {
            return printErrorUsageAndExit(
                `error: --parallel flag must specify a number: ${argv["parallel"]} is not a number`);
        }
    }

    // Ensure a monitor address was passed
    const monitorAddr = argv["monitor"];
    if (!monitorAddr) {
        return printErrorUsageAndExit(`error: --monitor=addr must be provided.`);
    }

    // Finally, ensure we have a program to run.
    if (argv._.length === 0) {
        return printErrorUsageAndExit("error: Missing program to execute");
    }

    // Due to node module loading semantics, multiple copies of @pulumi/pulumi could be loaded at runtime. So we need
    // to squirel these settings in the environment such that other copies which may be loaded later can recover them.
    //
    // Config is already an environment variaible set by the language plugin.
    addToEnvIfDefined("PULUMI_NODEJS_PROJECT", argv["project"]);
    addToEnvIfDefined("PULUMI_NODEJS_STACK", argv["stack"]);
    addToEnvIfDefined("PULUMI_NODEJS_DRY_RUN", argv["dry-run"]);
    addToEnvIfDefined("PULUMI_NODEJS_PARALLEL", argv["parallel"]);
    addToEnvIfDefined("PULUMI_NODEJS_MONITOR", argv["monitor"]);
    addToEnvIfDefined("PULUMI_NODEJS_ENGINE", argv["engine"]);

    require("./run").run(argv);
}

function addToEnvIfDefined(key: string, value: string | undefined) {
    if (value) {
        process.env[key] = value;
    }
}

main(process.argv.slice(2));
