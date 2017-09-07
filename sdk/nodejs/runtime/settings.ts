// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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

// disconnect permanently disconnects from the server, closing the connections.
export function disconnect(): void {
    if (monitor) {
        (<any>monitor).close();
        monitor = undefined;
    }
    if (engine) {
        (<any>engine).close();
        engine = undefined;
    }
}

