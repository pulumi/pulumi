// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import { RunError } from "../errors";
import { Resource } from "../resource";
import { debuggablePromise } from "./debuggable";

/**
 * excessiveDebugOutput enables, well, pretty excessive debug output pertaining to resources and properties.
 */
export let excessiveDebugOutput: boolean = false;

/**
 * Options is a bag of settings that controls the behavior of previews and deployments
 */
export interface Options {
    readonly project?: string; // the name of the current project.
    readonly stack?: string; // the name of the current stack being deployed into.
    readonly engine?: Object; // a live connection to the engine, used for logging, etc.
    readonly monitor: Object; // a live connection to the resource monitor that tracks deployments.
    readonly parallel?: number; // the degree of parallelism for resource operations (default is serial).
    readonly dryRun?: boolean; // whether we are performing a preview (true) or a real deployment (false).
    readonly includeStackTraces?: boolean; // whether we include full stack traces in resource errors or not.
}

/**
 * options are the current deployment options being used for this entire session.
 */
export let options: Options = <any>{
    dryRun: false,
    includeStackTraces: true,
};

/**
 * hasMonitor returns true if we are currently connected to a resource monitoring service.
 */
export function hasMonitor(): boolean {
    return !!options.monitor;
}

/**
 * getMonitor returns the current resource monitoring service client for RPC communications.
 */
export function getMonitor(): Object {
    if (!options.monitor) {
        throw new RunError(/*urn:*/ "",
            "Pulumi program not connected to the engine -- are you running with the `pulumi` CLI?\n" +
            "This can also happen if you've loaded the Pulumi SDK module multiple times into the same proces");
    }
    return options.monitor;
}

/**
 * getEngine returns the current engine, if any, for RPC communications back to the resource engine.
 */
export function getEngine(): Object | undefined {
    return options.engine;
}

/**
 * serialize returns true if resource operations should be serialized.
 */
export function serialize(): boolean {
    return !options.parallel || options.parallel <= 1;
}

/**
 * configured is set to true once configuration has been set.
 */
let configured: boolean;

/**
 * configure initializes the current resource monitor and engine RPC connections, and whether we are performing a "dry
 * run" (preview), versus a real deployment, and so on.  It may only be called once.
 */
export function configure(opts: Options): void {
    if (configured) {
        throw new Error("Cannot configure runtime settings more than once");
    }
    Object.assign(options, opts);
    configured = true;
}

/**
 * disconnect permanently disconnects from the server, closing the connections.  It waits for the existing RPC
 * queue to drain.  If any RPCs come in afterwards, however, they will crash the process.
 */
export function disconnect(): void {
    let done: Promise<any> | undefined;
    const closeCallback: () => Promise<void> = () => {
        if (done !== rpcDone) {
            // If the done promise has changed, some activity occurred in between callbacks.  Wait again.
            done = rpcDone;
            return debuggablePromise(done.then(closeCallback));
        }
        disconnectSync();
        return Promise.resolve();
    };
    closeCallback();
}

/**
 * disconnectSync permanently disconnects from the server, closing the connections. Unlike `disconnect`. it does not
 * wait for the existing RPC queue to drain. Any RPCs that come in after this call will crash the process.
 */
export function disconnectSync(): void {
    // Otherwise, actually perform the close activities.
    try {
        if (options.monitor) {
            (<any>options.monitor).close();
            (<any>options).monitor = null;
        }
        if (options.engine) {
            (<any>options.engine).close();
            (<any>options).engine = null;
        }
    }
    catch (err) {
        // ignore all failures to avoid crashes during exit.
    }
}

/**
 * rpcDone resolves when the last known client-side RPC call finishes.
 */
let rpcDone: Promise<any> = Promise.resolve();

/**
 * rpcKeepAlive registers a pending call to ensure that we don't prematurely disconnect from the server.  It returns
 * a function that, when invoked, signals that the RPC has completed.
 */
export function rpcKeepAlive(): () => void {
    let done: (() => void) | undefined = undefined;
    const donePromise = debuggablePromise(new Promise<void>((resolve) => { done = resolve; }));
    rpcDone = rpcDone.then(() => donePromise);
    return done!;
}

let rootResource: Resource | undefined;

/**
 * getRootResource returns a root resource that will automatically become the default parent of all resources.  This
 * can be used to ensure that all resources without explicit parents are parented to a common parent resource.
 */
export function getRootResource(): Resource | undefined {
    return rootResource;
}

/**
 * setRootResource registers a resource that will become the default parent for all resources without explicit parents.
 */
export function setRootResource(res: Resource | undefined): void {
    if (rootResource && res) {
        throw new Error("Cannot set multiple root resources");
    }
    rootResource = res;
}
