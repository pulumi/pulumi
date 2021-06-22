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

import * as log from "../../log";

// The very first thing we do is set up unhandled exception and rejection hooks to ensure that these
// events cause us to exit with a non-zero code. It is critically important that we do this early:
// if we do not, unhandled rejections in particular may cause us to exit with a 0 exit code, which
// will trick the engine into thinking that the program ran successfully. This can cause the engine
// to decide to delete all of a stack's resources.
//
// We track all uncaught errors here.  If we have any, we will make sure we always have a non-0 exit
// code.
const uncaughtErrors = new Set<Error>();

// We also track errors we know were logged to the user using our standard `log.error` call from
// inside our uncaught-error-handler in run.ts.  If all uncaught-errors above were also known to all
// be logged properly to the user, then we know the user has the information they need to proceed.
// We can then report the langhost that it should just stop running immediately and not print any
// additional superfluous information.
const loggedErrors = new Set<Error>();

let programRunning = false;
const uncaughtHandler = (err: Error) => {
    uncaughtErrors.add(err);
    if (!programRunning && !loggedErrors.has(err)) {
        log.error(err.stack || err.message || ("" + err));
        // dedupe errors that we're reporting when the program is not running
        loggedErrors.add(err);
    }
};

// Keep track if we already logged the information about an unhandled error to the user..  If
// so, we end with a different exit code.  The language host recognizes this and will not print
// any further messages to the user since we already took care of it.
//
// 32 was picked so as to be very unlikely to collide with any of the error codes documented by
// nodejs here:
// https://github.com/nodejs/node-v0.x-archive/blob/master/doc/api/process.markdown#exit-codes
/** @internal */
export const nodeJSProcessExitedAfterLoggingUserActionableMessage = 32;

process.on("uncaughtException", uncaughtHandler);
// @ts-ignore 'unhandledRejection' will almost always invoke uncaughtHandler with an Error. so just
// suppress the TS strictness here.
process.on("unhandledRejection", uncaughtHandler);
process.on("exit", (code: number) => {
    // If there were any uncaught errors at all, we always want to exit with an error code. If we
    // did not, it could be disastrous for the user.  i.e. not all resources may have been created,
    // but the 0 code would indicate we could proceed.  That could lead to many (or all) of the
    // user resources being deleted.
    if (code === 0 && uncaughtErrors.size > 0) {
        // Now Check if this error was already logged to the user in a visible fashion.  If not
        // we will exit with '1', indicating that the host should give a generic message about
        // things not working.
        for (const err of uncaughtErrors) {
            if (!loggedErrors.has(err)) {
                process.exitCode = 1;
                return;
            }
        }

        process.exitCode = nodeJSProcessExitedAfterLoggingUserActionableMessage;
    }
});

// As the second thing we do, ensure that we're connected to v8's inspector API.  We need to do
// this as some information is only sent out as events, without any way to query for it after the
// fact.  For example, we want to keep track of ScriptId->FileNames so that we can appropriately
// report errors for Functions we cannot serialize.  This can only be done (up to Node11 at least)
// by register to hear about scripts being parsed.
import * as v8Hooks from "../../runtime/closure/v8Hooks";

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
    console.error(`        --query-mode        true to run pulumi in query mode`);
    console.error(`        --dry-run           true to simulate resource changes, but without making them`);
    console.error(`        --pwd=pwd           change the working directory before running the program`);
    console.error(`        --monitor=addr      [required] the RPC address for a resource monitor to connect to`);
    console.error(`        --engine=addr       the RPC address for a resource engine to connect to`);
    console.error(`        --sync=path         path to synchronous 'invoke' endpoints`);
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
        boolean: [ "dry-run", "query-mode" ],
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
    addToEnvIfDefined("PULUMI_NODEJS_QUERY_MODE", argv["query-mode"]);
    addToEnvIfDefined("PULUMI_NODEJS_PARALLEL", argv["parallel"]);
    addToEnvIfDefined("PULUMI_NODEJS_MONITOR", argv["monitor"]);
    addToEnvIfDefined("PULUMI_NODEJS_ENGINE", argv["engine"]);
    addToEnvIfDefined("PULUMI_NODEJS_SYNC", argv["sync"]);

    // Ensure that our v8 hooks have been initialized.  Then actually load and run the user program.
    v8Hooks.isInitializedAsync().then(() => {
        const promise: Promise<void> = require("./run").run(
            argv,
            /*programStarted:   */ () => programRunning = true,
            /*reportLoggedError:*/ (err: Error) => loggedErrors.add(err),
            /*isErrorReported:  */ (err: Error) => loggedErrors.has(err));

        // when the user's program completes successfully, set programRunning back to false.  That way, if the Pulumi
        // scaffolding code ends up throwing an exception during teardown, it will get printed directly to the console.
        //
        // Note: we only do this in the 'resolved' arg of '.then' (not the 'rejected' arg).  If the users code throws
        // an exception, this promise will get rejected, and we don't want touch or otherwise intercept the exception
        // or change the programRunning state here at all.
        promise.then(() => { programRunning = false; });
    });
}

function addToEnvIfDefined(key: string, value: string | undefined) {
    if (value) {
        process.env[key] = value;
    }
}

main(process.argv.slice(2));
