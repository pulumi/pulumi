// Copyright 2016-2021, Pulumi Corporation.
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

import * as grpc from "@grpc/grpc-js";
import * as fs from "fs";
import * as path from "path";
import { ComponentResource } from "../resource";
import { debuggablePromise } from "./debuggable";
import { getLocalStore, getStore } from "./state";

import * as engrpc from "../proto/engine_grpc_pb";
import * as engproto from "../proto/engine_pb";
import * as resrpc from "../proto/resource_grpc_pb";
import * as resproto from "../proto/resource_pb";

// maxRPCMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
/** @internal */
export const maxRPCMessageSize: number = 1024 * 1024 * 400;
const grpcChannelOptions = { "grpc.max_receive_message_length": maxRPCMessageSize };

/**
 * excessiveDebugOutput enables, well, pretty excessive debug output pertaining to resources and properties.
 */
export const excessiveDebugOutput: boolean = false;

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
    readonly cacheDynamicProviders?: boolean; // true if we will cache serialized dynamic providers on the program side.
    readonly organization?: string; // the name of the current organization.

    /**
     * Directory containing the send/receive files for making synchronous invokes to the engine.
     */
    readonly syncDir?: string;
}

let monitor: resrpc.ResourceMonitorClient | undefined;
let engine: engrpc.EngineClient | undefined;

// reset options resets nodejs runtime global state (such as rpc clients),
// and sets nodejs runtime option env vars to the specified values.
export function resetOptions(
    project: string,
    stack: string,
    parallel: number,
    engineAddr: string,
    monitorAddr: string,
    preview: boolean,
    organization: string,
) {
    const { settings } = getStore();

    monitor = undefined;
    engine = undefined;
    settings.monitor = undefined;
    settings.engine = undefined;
    settings.rpcDone = Promise.resolve();
    settings.featureSupport = {};

    // reset node specific environment variables in the process
    settings.options.project = project;
    settings.options.stack = stack;
    settings.options.dryRun = preview;
    settings.options.queryMode = isQueryMode();
    settings.options.parallel = parallel;
    settings.options.monitorAddr = monitorAddr;
    settings.options.engineAddr = engineAddr;
    settings.options.organization = organization;
}

export function setMockOptions(
    mockMonitor: any,
    project?: string,
    stack?: string,
    preview?: boolean,
    organization?: string,
) {
    const opts = options();
    resetOptions(
        project || opts.project || "project",
        stack || opts.stack || "stack",
        opts.parallel || -1,
        opts.engineAddr || "",
        opts.monitorAddr || "",
        preview || false,
        organization || "",
    );

    monitor = mockMonitor;
}

/** @internal Used only for testing purposes. */
export function _setIsDryRun(val: boolean) {
    const { settings } = getStore();
    settings.options.dryRun = val;
}

/**
 * Returns whether or not we are currently doing a preview.
 *
 * When writing unit tests, you can set this flag via either `setMocks` or `_setIsDryRun`.
 */
export function isDryRun(): boolean {
    return options().dryRun === true;
}

/** @internal Used only for testing purposes */
export function _setFeatureSupport(key: string, val: boolean) {
    const { featureSupport } = getStore().settings;
    featureSupport[key] = val;
}

/** @internal Used only for testing purposes. */
export function _setQueryMode(val: boolean) {
    const { settings } = getStore();
    settings.options.queryMode = val;
}

/** @internal Used only for testing purposes */
export function _reset(): void {
    resetOptions("", "", -1, "", "", false, "");
}

/**
 * Returns true if query mode is enabled.
 */
export function isQueryMode(): boolean {
    return options().queryMode === true;
}

/**
 * Returns true if we will resolve missing outputs to inputs during preview (PULUMI_ENABLE_LEGACY_APPLY).
 */
export function isLegacyApplyEnabled(): boolean {
    return options().legacyApply === true;
}

/**
 * Returns true (default) if we will cache serialized dynamic providers on the program side
 */
export function cacheDynamicProviders(): boolean {
    return options().cacheDynamicProviders === true;
}

/**
 * Get the organization being run by the current update.
 */
export function getOrganization(): string {
    const organization = options().organization;
    if (organization) {
        return organization;
    }

    // If the organization is missing, specialize the error.
    // Throw an error if test mode is enabled, instructing how to manually configure the organization:
    throw new Error("Missing organization name; for test mode, please call `pulumi.runtime.setMocks`");
}

/** @internal Used only for testing purposes. */
export function _setOrganization(val: string | undefined) {
    const { settings } = getStore();
    settings.options.organization = val;
    return settings.options.organization;
}

/**
 * Get the project being run by the current update.
 */
export function getProject(): string {
    const { project } = options();
    return project || "";
}

/** @internal Used only for testing purposes. */
export function _setProject(val: string | undefined) {
    const { settings } = getStore();
    settings.options.project = val;
    return settings.options.project;
}

/**
 * Get the stack being targeted by the current update.
 */
export function getStack(): string {
    const { stack } = options();
    return stack || "";
}

/** @internal Used only for testing purposes. */
export function _setStack(val: string | undefined) {
    const { settings } = getStore();
    settings.options.stack = val;
    return settings.options.stack;
}

/**
 * hasMonitor returns true if we are currently connected to a resource monitoring service.
 */
export function hasMonitor(): boolean {
    return !!monitor && !!options().monitorAddr;
}

/**
 * getMonitor returns the current resource monitoring service client for RPC communications.
 */
export function getMonitor(): resrpc.ResourceMonitorClient | undefined {
    const { settings } = getStore();
    const addr = options().monitorAddr;
    if (getLocalStore() === undefined) {
        if (monitor === undefined) {
            if (addr) {
                // Lazily initialize the RPC connection to the monitor.
                monitor = new resrpc.ResourceMonitorClient(addr, grpc.credentials.createInsecure(), grpcChannelOptions);
                settings.options.monitorAddr = addr;
            }
        }
        return monitor;
    } else {
        if (settings.monitor === undefined) {
            if (addr) {
                // Lazily initialize the RPC connection to the monitor.
                settings.monitor = new resrpc.ResourceMonitorClient(
                    addr,
                    grpc.credentials.createInsecure(),
                    grpcChannelOptions,
                );
                settings.options.monitorAddr = addr;
            }
        }
        return settings.monitor;
    }
}

/** @internal */
export interface SyncInvokes {
    requests: number;
    responses: number;
}

let syncInvokes: SyncInvokes | undefined;

/** @internal */
export function tryGetSyncInvokes(): SyncInvokes | undefined {
    const syncDir = options().syncDir;
    if (syncInvokes === undefined && syncDir) {
        const requests = fs.openSync(path.join(syncDir, "invoke_req"), fs.constants.O_WRONLY | fs.constants.O_SYNC);
        const responses = fs.openSync(path.join(syncDir, "invoke_res"), fs.constants.O_RDONLY | fs.constants.O_SYNC);
        syncInvokes = { requests, responses };
    }

    return syncInvokes;
}

/**
 * hasEngine returns true if we are currently connected to an engine.
 */
export function hasEngine(): boolean {
    return !!engine && !!options().engineAddr;
}

/**
 * getEngine returns the current engine, if any, for RPC communications back to the resource engine.
 */
export function getEngine(): engrpc.EngineClient | undefined {
    const { settings } = getStore();
    if (getLocalStore() === undefined) {
        if (engine === undefined) {
            const addr = options().engineAddr;
            if (addr) {
                // Lazily initialize the RPC connection to the engine.
                engine = new engrpc.EngineClient(addr, grpc.credentials.createInsecure(), grpcChannelOptions);
            }
        }
        return engine;
    } else {
        if (settings.engine === undefined) {
            const addr = options().engineAddr;
            if (addr) {
                // Lazily initialize the RPC connection to the engine.
                settings.engine = new engrpc.EngineClient(addr, grpc.credentials.createInsecure(), grpcChannelOptions);
            }
        }
        return settings.engine;
    }
}

export function terminateRpcs() {
    disconnectSync();
}

/**
 * serialize returns true if resource operations should be serialized.
 */
export function serialize(): boolean {
    return options().parallel === 1;
}

/**
 * options returns the options from the environment, which is the source of truth. Options are global per process.
 * For CLI driven programs, pulumi-language-nodejs sets environment variables prior to the user program loading,
 * meaning that options could be loaded up front and cached.
 * Automation API and multi-language components introduced more complex lifecycles for runtime options().
 * These language hosts manage the lifecycle of options manually throughout the lifetime of the nodejs process.
 * In addition, node module resolution can lead to duplicate copies of @pulumi/pulumi and thus duplicate options
 *  objects that may not be synced if options are cached upfront. Mutating options must write to the environment
 * and reading options must always read directly from the environment.

 */
function options(): Options {
    const { settings } = getStore();

    return settings.options;
}

/**
 * disconnect permanently disconnects from the server, closing the connections.  It waits for the existing RPC
 * queue to drain.  If any RPCs come in afterwards, however, they will crash the process.
 */
export function disconnect(): Promise<void> {
    return waitForRPCs(/*disconnectFromServers*/ true);
}

/** @internal */
export function waitForRPCs(disconnectFromServers = false): Promise<void> {
    const localStore = getStore();
    let done: Promise<any> | undefined;
    const closeCallback: () => Promise<void> = () => {
        if (done !== localStore.settings.rpcDone) {
            // If the done promise has changed, some activity occurred in between callbacks.  Wait again.
            done = localStore.settings.rpcDone;
            return debuggablePromise(done.then(closeCallback), "disconnect");
        }
        if (disconnectFromServers) {
            disconnectSync();
        }
        return Promise.resolve();
    };
    return closeCallback();
}

/**
 * getMaximumListeners returns the configured number of process listeners available
 */
export function getMaximumListeners(): number {
    const { settings } = getStore();
    return settings.options.maximumProcessListeners;
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
        } catch (err) {
            // ignore.
        }
        monitor = undefined;
    }
    if (engine) {
        try {
            engine.close();
        } catch (err) {
            // ignore.
        }
        engine = undefined;
    }
}

/**
 * rpcKeepAlive registers a pending call to ensure that we don't prematurely disconnect from the server.  It returns
 * a function that, when invoked, signals that the RPC has completed.
 */
export function rpcKeepAlive(): () => void {
    const localStore = getStore();
    let done: (() => void) | undefined = undefined;
    const donePromise = debuggablePromise(
        new Promise<void>((resolve) => {
            done = resolve;
            return done;
        }),
        "rpcKeepAlive",
    );
    localStore.settings.rpcDone = localStore.settings.rpcDone.then(() => donePromise);
    return done!;
}

/**
 * setRootResource registers a resource that will become the default parent for all resources without explicit parents.
 */
export async function setRootResource(res: ComponentResource): Promise<void> {
    const engineRef = getEngine();
    if (!engineRef) {
        return Promise.resolve();
    }

    // Back-compat case - Try to set the root URN for SxS old SDKs that expect the engine to roundtrip the
    // stack URN.
    const req = new engproto.SetRootResourceRequest();
    const urn = await res.urn.promise();
    req.setUrn(urn);
    return new Promise<void>((resolve, reject) => {
        engineRef.setRootResource(
            req,
            (err: grpc.ServiceError | null, resp: engproto.SetRootResourceResponse | undefined) => {
                // Back-compat case - if the engine we're speaking to isn't aware that it can save and load root
                // resources, just ignore there's nothing we can do.
                if (err && err.code === grpc.status.UNIMPLEMENTED) {
                    return resolve();
                }

                if (err) {
                    return reject(err);
                }

                return resolve();
            },
        );
    });
}

/**
 * monitorSupportsFeature returns a promise that when resolved tells you if the resource monitor we are connected
 * to is able to support a particular feature.
 */
export async function monitorSupportsFeature(feature: string): Promise<boolean> {
    const localStore = getStore();
    const monitorRef = getMonitor();
    if (!monitorRef) {
        return localStore.settings.featureSupport[feature];
    }

    if (localStore.settings.featureSupport[feature] === undefined) {
        const req = new resproto.SupportsFeatureRequest();
        req.setId(feature);

        const result = await new Promise<boolean>((resolve, reject) => {
            monitorRef.supportsFeature(
                req,
                (err: grpc.ServiceError | null, resp: resproto.SupportsFeatureResponse | undefined) => {
                    // Back-compat case - if the monitor doesn't let us ask if it supports a feature, it doesn't support
                    // secrets.
                    if (err && err.code === grpc.status.UNIMPLEMENTED) {
                        return resolve(false);
                    }

                    if (err) {
                        return reject(err);
                    }

                    if (resp === undefined) {
                        return reject(new Error("No response from resource monitor"));
                    }

                    return resolve(resp.getHassupport());
                },
            );
        });

        localStore.settings.featureSupport[feature] = result;
    }

    return localStore.settings.featureSupport[feature];
}

/**
 * monitorSupportsSecrets returns a promise that when resolved tells you if the resource monitor we are connected
 * to is able to support secrets across its RPC interface. When it does, we marshal outputs marked with the secret
 * bit in a special way.
 */
export function monitorSupportsSecrets(): Promise<boolean> {
    return monitorSupportsFeature("secrets");
}

/**
 * monitorSupportsResourceReferences returns a promise that when resolved tells you if the resource monitor we are
 * connected to is able to support resource references across its RPC interface. When it does, we marshal resources
 * in a special way.
 */
export async function monitorSupportsResourceReferences(): Promise<boolean> {
    return monitorSupportsFeature("resourceReferences");
}

/**
 * monitorSupportsOutputValues returns a promise that when resolved tells you if the resource monitor we are
 * connected to is able to support output values across its RPC interface. When it does, we marshal outputs
 * in a special way.
 */
export async function monitorSupportsOutputValues(): Promise<boolean> {
    return monitorSupportsFeature("outputValues");
}

/**
 * monitorSupportsDeletedWith returns a promise that when resolved tells you if the resource monitor we are
 * connected to is able to support the deletedWith resource option across its RPC interface.
 */
export async function monitorSupportsDeletedWith(): Promise<boolean> {
    return monitorSupportsFeature("deletedWith");
}

/**
 * monitorSupportsAliasSpecs returns a promise that when resolved tells you if the resource monitor we are
 * connected to is able to support alias specs across its RPC interface. When it does, we marshal aliases
 * in a special way.
 *
 * @internal
 */
export async function monitorSupportsAliasSpecs(): Promise<boolean> {
    return monitorSupportsFeature("aliasSpecs");
}
