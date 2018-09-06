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

import * as log from "../log";

/**
 * debugPromiseLeaks can be set to enable promises leaks debugging.
 */
const debugPromiseLeaks: boolean = !!process.env.PULUMI_DEBUG_PROMISE_LEAKS;

/**
 * leakDetectorScheduled is true when the promise leak detector is scheduled for process exit.
 */
let leakDetectorScheduled: boolean = false;
/**
 * leakCandidates tracks the list of potential leak candidates.
 */
const leakCandidates: Set<Promise<any>> = new Set<Promise<any>>();

function promiseDebugString(p: Promise<any>): string {
    return `CONTEXT: ${(<any>p)._debugCtx}\n` +
        `STACK_TRACE:\n` +
        `${(<any>p)._debugStackTrace}`;
}

/**
 * debuggablePromise optionally wraps a promise with some goo to make it easier to debug common problems.
 */
export function debuggablePromise<T>(p: Promise<T>, ctx?: any): Promise<T> {
    // Whack some stack onto the promise.  Leave them non-enumerable to avoid awkward rendering.
    Object.defineProperty(p, "_debugCtx", { writable: true, value: ctx });
    Object.defineProperty(p, "_debugStackTrace", { writable: true, value: new Error().stack });

    if (!leakDetectorScheduled) {
        process.on("exit", (code: number) => {
            // Only print leaks if we're exiting normally.  Otherwise, it could be a crash, which of
            // course yields things that look like "leaks".
            //
            // process.exitCode is undefined unless set, in which case it's the exit code that was
            // passed to process.exit.
            if ((process.exitCode === undefined || process.exitCode === 0) && !log.hasErrors()) {
                const leakedCount = leakCandidates.size;
                if (leakedCount === 0) {
                    // No leaks - proceed with the exit.
                    return;
                }

                // If we've opted-in to the debug error message, send it out here.
                if (debugPromiseLeaks) {
                    for (const leaked of leakCandidates) {
                        console.error("Promise leak detected:");
                        console.error(promiseDebugString(leaked));
                    }
                } else {
                    // Otherwise, issue a more user-friendly message.
                    const promisePlural = leakedCount === 1 ? "promise was" : "promises were";
                    console.error(`The Pulumi runtime detected that ${leakedCount} ${promisePlural} still active`);
                    console.error("at the time that the process exited. There are a few ways that this can occur:");
                    console.error("  * Not using `await` or `.then` on a Promise returned from a Pulumi API");
                    console.error("  * Introducing a cyclic dependency between two Pulumi Resources");
                    console.error("  * A bug in the Pulumi Runtime");
                    console.error("");
                    console.error("Leaving promises active is probably not what you want. If you are unsure about");
                    console.error("why you are seeing this message, re-run your program "
                        + "with the `PULUMI_DEBUG_PROMISE_LEAKS`");
                    console.error("environment variable. The Pulumi runtime will then print out additional");
                    console.error("debug information about the leaked promises.");
                }

                // Fail the deployment if we leaked any promises.
                process.exitCode = 1;
            }
        });
        leakDetectorScheduled = true;
    }

    // Add this promise to the leak candidates list, and schedule it for removal if it resolves.
    leakCandidates.add(p);
    return p.then((val: any) => {
        leakCandidates.delete(p);
        return val;
    }).catch((err: any) => {
        leakCandidates.delete(p);
        throw err;
    });
}

/**
 * errorString produces a string from an error, conditionally including additional diagnostics.
 */
export function errorString(err: Error): string {
    if (err.stack) {
        return err.stack;
    }
    return err.toString();
}

