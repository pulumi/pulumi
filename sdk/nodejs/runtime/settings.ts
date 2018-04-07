// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as minimist from "minimist";
import { RunError } from "../errors";
import { Resource } from "../resource";
import { loadConfig } from "./config";
import { debuggablePromise } from "./debuggable";

const grpc = require("grpc");
const engrpc = require("../proto/engine_grpc_pb.js");
const resrpc = require("../proto/resource_grpc_pb.js");

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
    readonly dryRun?: boolean; // whether we are performing a preview (true) or a real deployment (false).
    readonly parallel?: number; // the degree of parallelism for resource operations (default is serial).
    readonly engineAddr?: string; // a connection string to the engine's RPC, in case we need to reestablish.
    readonly monitorAddr?: string; // a connection string to the monitor's RPC, in case we need to reestablish.
}

/**
 * _options are the current deployment options being used for this entire session.
 */
let _options: Options | undefined;

/**
 * options fetches the current configured options and, if required, lazily initializes them.
 */
function options(): Options {
    if (!_options) {
        // See if the options are available in memory.  This would happen if we load the pulumi SDK multiple
        // times into the same heap.  In this case, the entry point would have configured one copy of the library,
        // which has an independent set of statics.  But it left behind the configured state in environment variables.
        _options = loadOptions();
    }
    return _options;
}

/**
 * Returns true if we're currently performing a dry-run, or false if this is a true update.
 */
export function isDryRun(): boolean {
    return options().dryRun === true;
}

/**
 * Get the project being run by the current update.
 */
export function getProject(): string | undefined {
    return options().project;
}

/**
 * Get the stack being targeted by the current update.
 */
export function getStack(): string | undefined {
    return options().stack;
}

/**
 * monitor is a live connection to the resource monitor that tracks deployments (lazily initialized).
 */
let monitor: any | undefined;

/**
 * hasMonitor returns true if we are currently connected to a resource monitoring service.
 */
export function hasMonitor(): boolean {
    return !!monitor && !!options().monitorAddr;
}

/**
 * getMonitor returns the current resource monitoring service client for RPC communications.
 */
export function getMonitor(): Object {
    if (!monitor) {
        const addr = options().monitorAddr;
        if (addr) {
            // Lazily initialize the RPC connection to the monitor.
            monitor = new resrpc.ResourceMonitorClient(addr, grpc.credentials.createInsecure());
        } else {
            // Otherwise, this is an error.
            throw new RunError(
                "Pulumi program not connected to the engine -- are you running with the `pulumi` CLI?");
        }
    }
    return monitor!;
}

/**
 * engine is a live connection to the engine, used for logging, etc. (lazily initialized).
 */
let engine: any | undefined;

/**
 * getEngine returns the current engine, if any, for RPC communications back to the resource engine.
 */
export function getEngine(): Object | undefined {
    if (!engine) {
        const addr = options().engineAddr;
        if (addr) {
            // Lazily initialize the RPC connection to the engine.
            engine = new engrpc.EngineClient(addr, grpc.credentials.createInsecure());
        }
    }
    return engine;
}

/**
 * serialize returns true if resource operations should be serialized.
 */
export function serialize(): boolean {
    const p = options().parallel;
    return !p || p <= 1;
}

/**
 * setOptions initializes the current runtime with information about whether we are performing a "dry
 * run" (preview), versus a real deployment, RPC addresses, and so on.   It may only be called once.
 */
export function setOptions(opts: Options): void {
    if (_options) {
        throw new Error("Cannot configure runtime settings more than once");
    }

    // Set environment variables so other copies of the library can do the right thing.
    if (opts.project !== undefined) {
        process.env["PULUMI_NODEJS_PROJECT"] = opts.project;
    }
    if (opts.stack !== undefined) {
        process.env["PULUMI_NODEJS_STACK"] = opts.stack;
    }
    if (opts.dryRun !== undefined) {
        process.env["PULUMI_NODEJS_DRY_RUN"] = opts.dryRun.toString();
    }
    if (opts.parallel !== undefined) {
        process.env["PULUMI_NODEJS_PARALLEL"] = opts.parallel.toString();
    }
    if (opts.monitorAddr !== undefined) {
        process.env["PULUMI_NODEJS_MONITOR"] = opts.monitorAddr;
    }
    if (opts.engineAddr !== undefined) {
        process.env["PULUMI_NODEJS_ENGINE"] = opts.engineAddr;
    }

    // Now, save the in-memory static state.  All RPC connections will be created lazily as required.
    _options = opts;
}

/**
 * loadOptions recovers previously configured options in the case that a copy of the runtime SDK library
 * is loaded without going through the entry point shim, as happens when multiple copies are loaded.
 */
function loadOptions(): Options {
    // Load the config from the environment.
    loadConfig();

    // The only option that needs parsing is the parallelism flag.  Ignore any failures.
    let parallel: number | undefined;
    const parallelOpt = process.env["PULUMI_NODEJS_PARALLEL"];
    if (parallelOpt) {
        try {
           parallel = parseInt(parallelOpt, 10);
        }
        catch (err) {
            // ignore.
        }
    }

    // Now just hydrate the rest from environment variables.  These might be missing, in which case
    // we will fail later on when we actually need to create an RPC connection back to the engine.
    return {
        project: process.env["PULUMI_NODEJS_PROJECT"],
        stack: process.env["PULUMI_NODEJS_STACK"],
        dryRun: (process.env["PULUMI_NODEJS_DRY_RUN"] === "true"),
        parallel: parallel,
        monitorAddr: process.env["PULUMI_NODEJS_MONITOR"],
        engineAddr: process.env["PULUMI_NODEJS_ENGINE"],
    };
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
    // Otherwise, actually perform the close activities (ignoring errors and crashes).
    if (monitor) {
        try {
            monitor.close();
        }
        catch (err) {
            // ignore.
        }
        monitor = null;
    }
    if (engine) {
        try {
            engine.close();
        }
        catch (err) {
            // ignore.
        }
        engine = null;
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
