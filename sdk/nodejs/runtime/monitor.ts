// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// monitor is a live connection to the resource monitor connection.
// IDEA: it would be nice to mirror the Protobuf structures as TypeScript interfaces.
let monitor: any;

// dryRun tells us whether we're performing a plan (true) versus a real deployment (false).
export let dryRun: boolean;

// warnAboutMissingMonitor is true, by default, but gets set to false (or can be manually), so we only warn once.
export let warnAboutMissingMonitor: boolean = true;

// hasMonitor returns true if we are currently connected to a resource monitoring service.
export function hasMonitor(): boolean {
    return !!monitor;
}

// isDryRun returns true if we are planning.
export function isDryRun(): boolean {
    return dryRun;
}

// getMonitor returns the current resource monitoring service client for RPC communications.
export function getMonitor(): any {
    if (!monitor && warnAboutMissingMonitor) {
        warnAboutMissingMonitor = false;
        console.warn("warning: Pulumi Fabric monitor is missing; no resources will be created");
    }
    return monitor;
}

// setMonitor initializes the current resource monitoring RPC connection.  It may be called only once.  This also
// tells the runtime whether we are performing a "dry run" (plan) or whether we are performing a real deployment.
export function configureMonitor(m: any, dr: boolean): void {
    if (monitor) {
        throw new Error("Cannot set the resource monitor more than once");
    }
    monitor = m;
    dryRun = dr;
}

