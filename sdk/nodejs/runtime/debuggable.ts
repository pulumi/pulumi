// Copyright 2016-2022, Pulumi Corporation.
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
import * as state from "./state";

/**
 * debugPromiseLeaks can be set to enable promises leaks debugging.
 *
 * @internal
 */
export const debugPromiseLeaks: boolean = !!process.env.PULUMI_DEBUG_PROMISE_LEAKS;

/**
 * leakDetectorScheduled is true when the promise leak detector is scheduled for
 * process exit.
 */
let leakDetectorScheduled: boolean = false;

/**
 * @internal
 */
export function leakedPromises(): [Set<Promise<any>>, string] {
    const localStore = state.getStore();
    const leaked = localStore.leakCandidates;
    const promisePlural = leaked.size === 1 ? "promise was" : "promises were";
    const message =
        leaked.size === 0
            ? ""
            : `The Pulumi runtime detected that ${leaked.size} ${promisePlural} still active\n` +
              "at the time that the process exited. There are a few ways that this can occur:\n" +
              "  * Not using `await` or `.then` on a Promise returned from a Pulumi API\n" +
              "  * Introducing a cyclic dependency between two Pulumi Resources\n" +
              "  * A bug in the Pulumi Runtime\n" +
              "\n" +
              "Leaving promises active is probably not what you want. If you are unsure about\n" +
              "why you are seeing this message, re-run your program " +
              "with the `PULUMI_DEBUG_PROMISE_LEAKS`\n" +
              "environment variable. The Pulumi runtime will then print out additional\n" +
              "debug information about the leaked promises.";

    if (debugPromiseLeaks) {
        for (const leak of leaked) {
            console.error("Promise leak detected:");
            console.error(promiseDebugString(leak));
        }
    }

    localStore.leakCandidates = new Set();
    return [leaked, message];
}

/**
 * @internal
 */
export function promiseDebugString(p: Promise<any>): string {
    return `CONTEXT(${(<any>p)._debugId}): ${(<any>p)._debugCtx}\n` + `STACK_TRACE:\n` + `${(<any>p)._debugStackTrace}`;
}

let promiseId = 0;

/**
 * Optionally wraps a promise with some goo to make it easier to debug common
 * problems.
 *
 * @internal
 */
export function debuggablePromise<T>(p: Promise<T>, ctx: any): Promise<T> {
    const localStore = state.getStore();
    // Whack some stack onto the promise.  Leave them non-enumerable to avoid awkward rendering.
    Object.defineProperty(p, "_debugId", { writable: true, value: promiseId });
    Object.defineProperty(p, "_debugCtx", { writable: true, value: ctx });
    Object.defineProperty(p, "_debugStackTrace", { writable: true, value: new Error().stack });

    promiseId++;

    if (!leakDetectorScheduled) {
        process.on("exit", (code: number) => {
            // Only print leaks if we're exiting normally.  Otherwise, it could be a crash, which of
            // course yields things that look like "leaks".
            //
            // process.exitCode is undefined unless set, in which case it's the exit code that was
            // passed to process.exit.
            //
            // If `localStore.terminated` is true, the monitor was terminated while we were waiting
            // for an operation to complete. In this case we have in flight promises (the ones tied
            // to the operation), but we don't to report the leaks message. Pulumi will already have
            // notified the user of the error reason that lead to the monitor shutdown.
            if (
                (process.exitCode === undefined || process.exitCode === 0) &&
                !log.hasErrors() &&
                !localStore.terminated
            ) {
                const [leaks, message] = leakedPromises();
                if (leaks.size === 0) {
                    // No leaks - proceed with the exit.
                    return;
                }

                // If we haven't opted-in to the debug error message, print a more user-friendly message.
                if (!debugPromiseLeaks) {
                    console.error(message);
                }

                // Fail the deployment if we leaked any promises.
                process.exitCode = 1;
            }
        });
        leakDetectorScheduled = true;
    }

    // Add this promise to the leak candidates list, and schedule it for removal if it resolves.
    localStore.leakCandidates.add(p);
    return p
        .then((val: any) => {
            localStore.leakCandidates.delete(p);
            return val;
        })
        .catch((err: any) => {
            localStore.leakCandidates.delete(p);
            err.promise = p;
            throw err;
        });
}

/**
 * Produces a string from an error, conditionally including additional
 * diagnostics.
 *
 * @internal
 */
export function errorString(err: Error): string {
    if (err.stack) {
        return err.stack;
    }
    return err.toString();
}
