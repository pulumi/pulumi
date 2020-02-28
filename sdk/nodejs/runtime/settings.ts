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

import * as fs from "fs";
import * as grpc from "grpc";
import * as path from "path";
import { ComponentResource, URN } from "../resource";
import { debuggablePromise } from "./debuggable";

const engrpc = require("../proto/engine_grpc_pb.js");
const engproto = require("../proto/engine_pb.js");
const provproto = require("../proto/provider_pb.js");
const resrpc = require("../proto/resource_grpc_pb.js");
const resproto = require("../proto/resource_pb.js");
const structproto = require("google-protobuf/google/protobuf/struct_pb.js");

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
    readonly parallel?: number; // the degree of parallelism for resource operations (default is serial).
    readonly engineAddr?: string; // a connection string to the engine's RPC, in case we need to reestablish.
    readonly monitorAddr?: string; // a connection string to the monitor's RPC, in case we need to reestablish.
    readonly dryRun?: boolean; // whether we are performing a preview (true) or a real deployment (false).
    readonly testModeEnabled?: boolean; // true if we're in testing mode (allows execution without the CLI).
    readonly queryMode?: boolean; // true if we're in query mode (does not allow resource registration).
    readonly legacyApply?: boolean; // true if we will resolve missing outputs to inputs during preview.

    /**
     * Directory containing the send/receive files for making synchronous invokes to the engine.
     */
    readonly syncDir?: string;
}

/**
 * options are the current deployment options being used for this entire session.
 */
let options = loadOptions();


export function setMockOptions(mockMonitor: any, project: string, stack: string, preview: boolean | undefined) {
    options = {
        project: "project",
        stack: "stack",
        dryRun: preview,
        queryMode: options.queryMode,
        parallel: options.parallel,
        monitorAddr: options.monitorAddr,
        engineAddr: options.engineAddr,
        testModeEnabled: true,
        legacyApply: options.legacyApply,
        syncDir: options.syncDir,
    };

    monitor = mockMonitor;
}

/** @internal Used only for testing purposes. */
export function _setIsDryRun(val: boolean) {
    (options as any).dryRun = val;
}

/**
 * Returns true if we're currently performing a dry-run, or false if this is a true update. Note that we
 * always consider executions in test mode to be "dry-runs", since we will never actually carry out an update,
 * and therefore certain output properties will never be resolved.
 */
export function isDryRun(): boolean {
    return options.dryRun === true;
}

/** @internal Used only for testing purposes */
export function _setTestModeEnabled(val: boolean) {
    (options as any).testModeEnabled = val;
}

/**
 * Returns true if test mode is enabled (PULUMI_TEST_MODE).
 */
export function isTestModeEnabled(): boolean {
    return options.testModeEnabled === true;
}

/**
 * Checks that test mode is enabled and, if not, throws an error.
 */
function requireTestModeEnabled(): void {
    if (!isTestModeEnabled()) {
        throw new Error("Program run without the Pulumi engine available; re-run using the `pulumi` CLI");
    }
}

/** @internal Used only for testing purposes. */
export function _setQueryMode(val: boolean) {
    (options as any).queryMode = val;
}

/**
 * Returns true if query mode is enabled.
 */
export function isQueryMode(): boolean {
    return options.queryMode === true;
}

/**
 * Returns true if we will resolve missing outputs to inputs during preview (PULUMI_ENABLE_LEGACY_APPLY).
 */
export function isLegacyApplyEnabled(): boolean {
    return options.legacyApply === true;
}

/**
 * Get the project being run by the current update.
 */
export function getProject(): string {
    if (options.project) {
        return options.project;
    }

    // If the project is missing, specialize the error. First, if test mode is disabled:
    requireTestModeEnabled();

    // And now an error if test mode is enabled, instructing how to manually configure the project:
    throw new Error("Missing project name; for test mode, please set PULUMI_NODEJS_PROJECT");
}

/** @internal Used only for testing purposes. */
export function _setProject(val: string | undefined) {
    (options as any).project = val;
}

/**
 * Get the stack being targeted by the current update.
 */
export function getStack(): string {
    if (options.stack) {
        return options.stack;
    }

    // If the stack is missing, specialize the error. First, if test mode is disabled:
    requireTestModeEnabled();

    // And now an error if test mode is enabled, instructing how to manually configure the stack:
    throw new Error("Missing stack name; for test mode, please set PULUMI_NODEJS_STACK");
}

/** @internal Used only for testing purposes. */
export function _setStack(val: string | undefined) {
    (options as any).stack = val;
}

/**
 * monitor is a live connection to the resource monitor that tracks deployments (lazily initialized).
 */
let monitor: any | undefined;

/**
 * hasMonitor returns true if we are currently connected to a resource monitoring service.
 */
export function hasMonitor(): boolean {
    return !!monitor && !!options.monitorAddr;
}

/**
 * getMonitor returns the current resource monitoring service client for RPC communications.
 */
export function getMonitor(): Object | undefined {
    if (!monitor) {
        const addr = options.monitorAddr;
        if (addr) {
            // Lazily initialize the RPC connection to the monitor.
            monitor = new resrpc.ResourceMonitorClient(addr, grpc.credentials.createInsecure());
        } else {
            // If test mode isn't enabled, we can't run the program without an engine.
            requireTestModeEnabled();
        }
    }
    return monitor;
}

/** @internal */
export interface SyncInvokes {
    requests: number;
    responses: number;
}

let syncInvokes: SyncInvokes | undefined;

/** @internal */
export function tryGetSyncInvokes(): SyncInvokes | undefined {
    if (syncInvokes === undefined && options.syncDir) {
        const requests = fs.openSync(path.join(options.syncDir, "invoke_req"), fs.constants.O_WRONLY|fs.constants.O_SYNC);
        const responses = fs.openSync(path.join(options.syncDir, "invoke_res"), fs.constants.O_RDONLY|fs.constants.O_SYNC);
        syncInvokes = { requests, responses };
    }

    return syncInvokes;
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
        const addr = options.engineAddr;
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
    return options.parallel === 1;
}

/**
 * loadOptions recovers the options from the environment, which is set before we begin executing. This ensures
 * that even when multiple copies of this module are loaded, they all get the same values.
 */
function loadOptions(): Options {
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
        queryMode: (process.env["PULUMI_NODEJS_QUERY_MODE"] === "true"),
        parallel: parallel,
        monitorAddr: process.env["PULUMI_NODEJS_MONITOR"],
        engineAddr: process.env["PULUMI_NODEJS_ENGINE"],
        testModeEnabled: (process.env["PULUMI_TEST_MODE"] === "true"),
        legacyApply: (process.env["PULUMI_ENABLE_LEGACY_APPLY"] === "true"),
        syncDir: process.env["PULUMI_NODEJS_SYNC"],
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
            return debuggablePromise(done.then(closeCallback), "disconnect");
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
    const donePromise = debuggablePromise(new Promise<void>((resolve) => { done = resolve; }), "rpcKeepAlive");
    rpcDone = rpcDone.then(() => donePromise);
    return done!;
}

let rootResource: Promise<URN> | undefined;

/**
 * getRootResource returns a root resource URN that will automatically become the default parent of all resources.  This
 * can be used to ensure that all resources without explicit parents are parented to a common parent resource.
 */
export function getRootResource(): Promise<URN | undefined> {
    const engineRef: any = getEngine();
    if (!engineRef) {
        return Promise.resolve(undefined);
    }

    const req = new engproto.GetRootResourceRequest();
    return new Promise<URN | undefined>((resolve, reject) => {
        engineRef.getRootResource(req, (err: grpc.ServiceError, resp: any) => {
            // Back-compat case - if the engine we're speaking to isn't aware that it can save and load root resources,
            // fall back to the old behavior.
            if (err && err.code === grpc.status.UNIMPLEMENTED) {
                if (rootResource) {
                    rootResource.then(resolve);
                    return;
                }

                resolve(undefined);
            }

            if (err) {
                return reject(err);
            }

            const urn = resp.getUrn();
            if (urn) {
                return resolve(urn);
            }

            return resolve(undefined);
        });
    });
}

/**
 * setRootResource registers a resource that will become the default parent for all resources without explicit parents.
 */
export async function setRootResource(res: ComponentResource): Promise<void> {
    const engineRef: any = getEngine();
    if (!engineRef) {
        return Promise.resolve();
    }

    const req = new engproto.SetRootResourceRequest();
    const urn = await res.urn.promise();
    req.setUrn(urn);
    return new Promise<void>((resolve, reject) => {
        engineRef.setRootResource(req, (err: grpc.ServiceError, resp: any) => {
            // Back-compat case - if the engine we're speaking to isn't aware that it can save and load root resources,
            // fall back to the old behavior.
            if (err && err.code === grpc.status.UNIMPLEMENTED) {
                rootResource = res.urn.promise();
                return resolve();
            }

            if (err) {
                return reject(err);
            }

            return resolve();
        });
    });
}

/**
 * monitorSupportsSecrets returns a promise that when resolved tells you if the resource monitor we are connected
 * to is able to support secrets across it's RPC interface. When it does, we marshal outputs marked with the secret
 * bit in a special way.
 */
export function monitorSupportsSecrets(): Promise<boolean> {
    const monitorRef: any = getMonitor();
    if (!monitorRef) {
        return Promise.resolve(false);
    }

    const req = new resproto.SupportsFeatureRequest();
    req.setId("secrets");

    return new Promise<boolean>((resolve, reject) => {
        monitorRef.supportsFeature(req, (err: grpc.ServiceError, resp: any) => {
            // Back-compat case - if the monitor doesn't let us ask if it supports a feature, it doesn't support
            // secrets.
            if (err && err.code === grpc.status.UNIMPLEMENTED) {
                return resolve(false);
            }

            if (err) {
                return reject(err);
            }

            return resolve(resp.getHassupport());
        });
    });
}
