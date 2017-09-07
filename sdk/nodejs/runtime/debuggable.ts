// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { Log } from "./log";

// debugPromiseTimeout can be set to enable promises debugging.  If it is -1, it has no effect.  Be careful setting
// this to other values, because too small a value will cause legitimate promise resolutions to appear as timeouts.
const debugPromiseTimeout: number = -1;

// leakDetectorScheduled is true when the promise leak detector is scheduled for process exit.
let leakDetectorScheduled: boolean = false;
// leakCandidates tracks the list of potential leak candidates.
let leakCandidates: Set<Promise<any>> = new Set<Promise<any>>();

// debuggablePromise optionally wraps a promise with some goo to make it easier to debug common problems.
export function debuggablePromise<T>(p: Promise<T>, ctx?: any): Promise<T> {
    // Whack some stack onto the promise.
    (<any>p)._debugCtx = ctx;
    (<any>p)._debugStackTrace = new Error().stack;

    // Setup leak detection.
    if (!leakDetectorScheduled) {
        process.on('exit', () => {
            for (let leaked of leakCandidates) {
                Log.error(
                    `Promise leak detected:\n` +
                    `CONTEXT: ${(<any>leaked)._debugCtx}\n` +
                    `STACK_TRACE:\n` +
                    `${(<any>leaked)._debugStackTrace}`
                );
            }
        });
        leakDetectorScheduled = true;
    }
    leakCandidates.add(p);
    p.then((v: any) => leakCandidates.delete(p), (err: any) => leakCandidates.delete(p));

    // If the timeout is -1, don't register a timer.
    if (debugPromiseTimeout === -1) {
        return p;
    }

    // Create a timer that we race against the original promise.
    let timetok: any;
    let timeout = new Promise<T>((resolve, reject) => {
        timetok = setTimeout(() => {
            clearTimeout(timetok);
            reject(
                `Promise timeout after ${debugPromiseTimeout}ms;\n` +
                `CONTEXT: ${ctx}\n` +
                `STACK TRACE:\n` +
                `${(<any>p)._debugStackTrace}`
            );
        }, debugPromiseTimeout);
    });

    // Ensure to cancel the timer should the promise actually resolve.
    p.then((v: any) => clearTimeout(timetok), (err: any) => clearTimeout(timetok));

    // Now race them; first one wins!
    return Promise.race([ p, timeout ]);
}

