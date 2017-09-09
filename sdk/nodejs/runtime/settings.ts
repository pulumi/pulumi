// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { debuggablePromise } from "./debuggable";

// includeStacks dictates whether we include full stack traces in resource errors or not.
let includeStacks: boolean = true;

export function errorString(err: Error): string {
    if (includeStacks && err.stack) {
        return err.stack;
    }
    return err.toString();
}

// configured is set to true once configuration has been set.
let configured: boolean;

// monitor is a live connection to the resource monitor connection.
// IDEA: it would be nice to mirror the Protobuf structures as TypeScript interfaces.
let monitor: Object | undefined;

// engine is an optional live connection to the engine.  This can be used for logging, etc.
let engine: Object | undefined;

// dryRun tells us whether we're performing a plan (true) versus a real deployment (false).
export let dryRun: boolean;

// hasMonitor returns true if we are currently connected to a resource monitoring service.
export function hasMonitor(): boolean {
    return !!monitor;
}

// isDryRun returns true if we are planning.
export function isDryRun(): boolean {
    return dryRun;
}

// getMonitor returns the current resource monitoring service client for RPC communications.
export function getMonitor(): Object | undefined {
    if (!configured) {
        configured = true;
        console.warn("warning: Pulumi Fabric monitor is missing; no resources will be created");
    }
    return monitor;
}

// getEngine returns the current engine, if any, for RPC communications back to the resource engine.
export function getEngine(): Object | undefined {
    return engine;
}

// configure initializes the current resource monitor and engine RPC connections, and whether we are performing a "dry
// run" (plan), versus a real deployment.  It may only be called once.
export function configure(m: Object | undefined, e: Object | undefined, dr: boolean): void {
    if (configured) {
        throw new Error("Cannot configure runtime settings more than once");
    }
    monitor = m;
    engine = e;
    dryRun = dr;
    configured = true;
}

// disconnect permanently disconnects from the server, closing the connections.  It waits for the existing RPC
// queue to drain.  If any RPCs come in afterwards, however, they will crash the process.
export function disconnect(): void {
    let done: Promise<any> | undefined;
    let closeCallback: () => Promise<void> = () => {
        if (done !== rpcDone) {
            // If the done promise has changed, some activity occurred in between callbacks.  Wait again.
            done = rpcDone;
            return debuggablePromise(done.then(closeCallback));
        }
        // Otherwise, actually perform the close activities.
        if (monitor) {
            (<any>monitor).close();
            monitor = undefined;
        }
        if (engine) {
            (<any>engine).close();
            engine = undefined;
        }
        return Promise.resolve();
    };
    closeCallback();
}

// rpcDone resolves when the last known client-side RPC call finishes.
let rpcDone: Promise<any> = Promise.resolve();

// rpcKeepAlive registers a pending call to ensure that we don't prematurely disconnect from the server.  It returns
// a function that, when invoked, signals that the RPC has completed.
export function rpcKeepAlive(): () => void {
    let done: (() => void) | undefined = undefined;
    let donePromise = debuggablePromise(new Promise<void>((resolve) => { done = resolve; }));
    rpcDone = rpcDone.then(() => donePromise);
    return done!;
}

