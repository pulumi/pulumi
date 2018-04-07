// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as log from "../log";

/**
 * debugPromiseTimeout can be set to enable promises debugging.  If it is -1, it has no effect.  Be careful setting
 * this to other values, because too small a value will cause legitimate promise resolutions to appear as timeouts.
 */
const debugPromiseTimeout: number = -1;

/**
 * debugPromiseLeaks can be set to enable promises leaks debugging.
 */
const debugPromiseLeaks: boolean = true;

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
    // Whack some stack onto the promise.
    (<any>p)._debugCtx = ctx;
    (<any>p)._debugStackTrace = new Error().stack;

    if (debugPromiseLeaks) {
        // Setup leak detection.
        if (!leakDetectorScheduled) {
            process.on("exit", (code: number) => {
                // Only print leaks if we're exiting normally.  Otherwise, it could be a crash, which of
                // course yields things that look like "leaks".
                if (code === 0 && !log.hasErrors()) {
                    for (const leaked of leakCandidates) {
                        console.error("Promise leak detected:");
                        console.error(promiseDebugString(leaked));
                    }
                }
            });
            leakDetectorScheduled = true;
        }

        // Add this promise to the leak candidates list, and schedule it for removal if it resolves.
        leakCandidates.add(p);
        p.then((v: any) => leakCandidates.delete(p), (err: any) => leakCandidates.delete(p));
    }

    // If the timeout isn't -1, register a timer.
    if (debugPromiseTimeout !== -1) {
        // Create a timer that we race against the original promise.
        let timetok: any;
        const timeout = new Promise<T>((resolve, reject) => {
            timetok = setTimeout(() => {
                clearTimeout(timetok);
                reject(
                    `Promise timeout after ${debugPromiseTimeout}ms;\n` +
                    `CONTEXT: ${ctx}\n` +
                    `STACK TRACE:\n` +
                    `${(<any>p)._debugStackTrace}`,
                );
            }, debugPromiseTimeout);
        });

        // Ensure to cancel the timer should the promise actually resolve.
        p.then((v: any) => clearTimeout(timetok), (err: any) => clearTimeout(timetok));

        // Now race them; first one wins!
        p = Promise.race([ p, timeout ]);
    }

    return p;
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

