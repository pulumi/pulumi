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

import * as fs from "fs";
import * as minimist from "minimist";
import * as path from "path";
import * as util from "util";
import * as pulumi from "../../";
import { RunError } from "../../errors";
import * as log from "../../log";
import * as runtime from "../../runtime";

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

/**
 * Attempts to provide a detailed error message for module load failure if the
 * module that failed to load is the top-level module.
 * @param program The name of the program given to `run`, i.e. the top level module
 * @param error The error that occured. Must be a module load error.
 */
function reportModuleLoadFailure(program: string, error: Error): never {
    // error is guaranteed to be a Node module load error. Node emits a very
    // specific string in its error message for module load errors, which includes
    // the module it was trying to load.
    const errorRegex = /Cannot find module '(.*)'/;

    // If there's no match, who knows what this exception is; it's not something
    // we can provide an intelligent diagnostic for.
    const moduleNameMatches = errorRegex.exec(error.message);
    if (moduleNameMatches === null) {
        throw error;
    }

    // Is the module that failed to load exactly the one that this script considered to
    // be the top-level module for this program?
    //
    // We are only interested in producing good diagnostics for top-level module loads,
    // since anything else are probably user code issues.
    const moduleName = moduleNameMatches[1];
    if (moduleName !== program) {
        throw error;
    }

    console.error(`We failed to locate the entry point for your program: ${program}`);

    // From here on out, we're going to try to inspect the program we're being asked to run
    // a little to see what sort of details we can glean from it, in the hopes of producing
    // a better error message.
    //
    // The first step of this is trying to slurp up a package.json for this program, if
    // one exists.
    const stat = fs.lstatSync(program);
    let projectRoot: string;
    if (stat.isDirectory()) {
        projectRoot = program;
    } else {
        projectRoot = path.dirname(program);
    }

    let packageObject: Record<string, any>;
    try {
        const packageJson = path.join(projectRoot, "package.json");
        packageObject = require(packageJson);
    } catch {
        // This is all best-effort so if we can't load the package.json file, that's
        // fine.
        return process.exit(1);
    }

    console.error("Here's what we think went wrong:");

    // The objective here is to emit the best diagnostic we can, starting from the
    // most specific to the least specific.
    const deps = packageObject["dependencies"] || {};
    const devDeps = packageObject["devDependencies"] || {};
    const scripts = packageObject["scripts"] || {};
    const mainProperty  = packageObject["main"] || "index.js";

    // Is there a build script associated with this program? It's a little confusing that the
    // Pulumi CLI doesn't run build scripts before running the program so call that out
    // explicitly.
    if ("build" in scripts) {
        const command = scripts["build"];
        console.error(`  * Your program looks like it has a build script associated with it ('${command}').\n`);
        console.error("Pulumi does not run build scripts before running your program. " +
                        `Please run '${command}', 'yarn build', or 'npm run build' and try again.`);
        return process.exit(1);
    }

    // Not all typescript programs have build scripts. If we think it's a typescript program,
    // tell the user to run tsc.
    if ("typescript" in deps || "typescript" in devDeps) {
        console.error("  * Your program looks like a TypeScript program. Have you run 'tsc'?");
        return process.exit(1);
    }

    // Not all projects are typescript. If there's a main property, check that the file exists.
    if (mainProperty !== undefined && typeof mainProperty === "string") {
        const mainFile = path.join(projectRoot, mainProperty);
        if (!fs.existsSync(mainFile)) {
            console.error(`  * Your program's 'main' file (${mainFile}) does not exist.`);
            return process.exit(1);
        }
    }

    console.error("  * Yowzas, our sincere apologies, we haven't seen this before!");
    console.error(`    Here is the raw exception message we received: ${error.message}`);
    return process.exit(1);
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

    // Load configuration passed from the language plugin
    runtime.ensureConfig();

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

    // If there is an engine argument, connect to it too.
    const engineAddr: string | undefined = argv["engine"];

    // Now configure the runtime and get it ready to run the program.
    runtime.setOptions({
        project: project,
        stack: stack,
        dryRun: dryRun,
        parallel: parallel,
        monitorAddr: monitorAddr,
        engineAddr: engineAddr,
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
        if (RunError.isInstance(err)) {
            // For errors that are subtypes of RunError, we will print the message without hitting the unhandled error
            // logic, which will dump all sorts of verbose spew like the origin source and stack trace.
            log.error(err.message);
        }
        else {
            log.error(`Running program '${program}' failed with an unhandled exception:`);
            log.error(util.format(err));
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
        try {
            return require(program);
        } catch (e) {
            // User JavaScript can throw anything, so if it's not an Error it's definitely
            // not something we want to catch up here.
            if (!(e instanceof Error)) {
                throw e;
            }

            // Give a better error message, if we can.
            const errorCode = (<any>e).code;
            if (errorCode === "MODULE_NOT_FOUND") {
                reportModuleLoadFailure(program, e);
            }

            throw e;
        }
    });
}

main(process.argv.slice(2));
