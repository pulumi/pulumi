// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// monitor is a live connection to the resource monitor connection.
// IDEA: it would be nice to mirror the Protobuf structures as TypeScript interfaces.
let monitor: any;

// hasMonitor returns true if we are currently connected to a resource monitoring service.
export function hasMonitor(): boolean {
    return !!monitor;
}

// getMonitor returns the current resource monitoring service client for RPC communications.
export function getMonitor(): any {
    if (!monitor) {
        throw new Error("Resource monitor is not initialized");
    }
    return monitor;
}

// setMonitor initializes the current resource monitoring RPC connection.  It may be called only once.
export function setMonitor(m: any): void {
    if (monitor) {
        throw new Error("Cannot set the resource monitor more than once");
    }
    monitor = m;
}

