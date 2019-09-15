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
    if (!programRunning) {
        console.error(err.stack || err.message);
    }
};

// Keep track if we already logged the information about an unhandled error to the user..  If
// so, we end with a different exit code.  The language host recognizes this and will not print
// any further messages to the user since we already took care of it.
//
// 32 was picked so as to be very unlikely to collide with any of the error codes documented by
// nodejs here:
// https://github.com/nodejs/node-v0.x-archive/blob/master/doc/api/process.markdown#exit-codes
const nodeJSProcessExitedAfterLoggingUserActionableMessage = 32;

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
    console.error(`usage: RUN <engine-address> <program>`);
}

function printErrorUsageAndExit(message: string): never {
    console.error(message);
    usage();
    return process.exit(-1);
}

function main(args: string[]): void {
    // See usage above for the intended usage of this program, including flags and required args.
    const argv: minimist.ParsedArgs = minimist(args, {});

    // Finally, ensure we have a program to run.
    if (argv._.length !== 2) {
        return printErrorUsageAndExit("error: Usage: RUN <engine-address> <program>");
    }

    // Remove <engine-address> so we simply execute the program.
    argv._.shift();

    // Ensure that our v8 hooks have been initialized.  Then actually load and run the user program.
    v8Hooks.isInitializedAsync().then(() => {
        const promise: Promise<void> = require("./run").run({
            argv,
            programStarted: () => (programRunning = true),
            reportLoggedError: (err: Error) => loggedErrors.add(err),
            runInStack: false,
            typeScript: true, // Should have no deleterious impact on JS codebases.
        });

        // when the user's program completes successfully, set programRunning back to false.  That
        // way, if the Pulumi scaffolding code ends up throwing an exception during teardown, it
        // will get printed directly to the console.
        //
        // Note: we only do this in the 'resolved' arg of '.then' (not the 'rejected' arg).  If the
        // users code throws an exception, this promise will get rejected, and we don't want touch
        // or otherwise intercept the exception or change the programRunning state here at all.
        promise.then(() => {
            programRunning = false;
        });
    });
}

main(process.argv.slice(2));
