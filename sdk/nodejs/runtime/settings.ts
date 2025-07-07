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
import { CallbackServer, ICallbackServer } from "./callbacks";
import { debuggablePromise } from "./debuggable";
import { getLocalStore, getStore } from "./state";

import * as engrpc from "../proto/engine_grpc_pb";
import * as engproto from "../proto/engine_pb";
import * as resrpc from "../proto/resource_grpc_pb";
import * as resproto from "../proto/resource_pb";
import * as emptyproto from "google-protobuf/google/protobuf/empty_pb";

/**
 * Raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb).
 *
 * @internal
 */
export const maxRPCMessageSize: number = 1024 * 1024 * 400;
const grpcChannelOptions = { "grpc.max_receive_message_length": maxRPCMessageSize };

/**
 * excessiveDebugOutput enables, well, pretty excessive debug output pertaining
 * to resources and properties.
 */
export const excessiveDebugOutput: boolean = false;

/**
 * {@link Options} is a bag of settings that controls the behavior of previews
 * and deployments.
 */
export interface Options {
    /**
     * The name of the current project.
     */
    readonly project?: string;

    /**
     * The root directory of the current project. This is the location of the Pulumi.yaml file.
     */
    readonly rootDirectory?: string;

    /**
     * The name of the current stack being deployed into.
     */
    readonly stack?: string;

    /**
     * The degree of parallelism for resource operations (default is serial).
     */
    readonly parallel?: number;

    /**
     * A connection string to the engine's RPC, in case we need to reestablish.
     */
    readonly engineAddr?: string;

    /**
     * A connection string to the monitor's RPC, in case we need to reestablish.
     */
    readonly monitorAddr?: string;

    /**
     * Whether we are performing a preview (true) or a real deployment (false).
     */
    readonly dryRun?: boolean;

    /**
     * True if we're in testing mode (allows execution without the CLI).
     */
    readonly testModeEnabled?: boolean;

    /**
     * True if we will resolve missing outputs to inputs during preview.
     */
    readonly legacyApply?: boolean;

    /**
     * True if we will cache serialized dynamic providers on the program side.
     */
    readonly cacheDynamicProviders?: boolean;

    /**
     * The name of the current organization.
     */
    readonly organization?: string;

    /**
     * A directory containing the send/receive files for making synchronous
     * invokes to the engine.
     */
    readonly syncDir?: string;
}

let monitor: resrpc.ResourceMonitorClient | undefined;
let engine: engrpc.EngineClient | undefined;

/**
 * Resets NodeJS runtime global state (such as RPC clients), and sets NodeJS
 * runtime option environment variables to the specified values.
 */
export function resetOptions(
    project: string,
    stack: string,
    parallel: number,
    engineAddr: string,
    monitorAddr: string,
    preview: boolean,
    organization: string,
) {
    const store = getStore();

    monitor = undefined;
    engine = undefined;

    store.settings.monitor = undefined;
    store.settings.engine = undefined;
    store.settings.rpcDone = Promise.resolve();
    store.settings.featureSupport = {};

    // reset node specific environment variables in the process
    store.settings.options.project = project;
    store.settings.options.stack = stack;
    store.settings.options.dryRun = preview;
    store.settings.options.parallel = parallel;
    store.settings.options.monitorAddr = monitorAddr;
    store.settings.options.engineAddr = engineAddr;
    store.settings.options.organization = organization;

    store.leakCandidates = new Set<Promise<any>>();
    store.logErrorCount = 0;
    store.stackResource = undefined;
    store.supportsSecrets = false;
    store.supportsResourceReferences = false;
    store.supportsOutputValues = false;
    store.supportsDeletedWith = false;
    store.supportsAliasSpecs = false;
    store.supportsTransforms = false;
    store.supportsInvokeTransforms = false;
    store.supportsParameterization = false;
    store.callbacks = undefined;
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
        "mock",
        preview || false,
        organization || "",
    );

    const { settings } = getStore();
    settings.monitor = mockMonitor;
    monitor = mockMonitor;
}

/**
 * @internal
 *  Used only for testing purposes.
 */
export function _setIsDryRun(val: boolean) {
    const { settings } = getStore();
    settings.options.dryRun = val;
}

/**
 * Returns true if we are currently doing a preview.
 *
 * When writing unit tests, you can set this flag via either `setMocks` or
 * `_setIsDryRun`.
 */
export function isDryRun(): boolean {
    return options().dryRun === true;
}

/**
 * Returns a promise that when resolved tells you if the resource monitor we are
 * connected to is able to support a particular feature.
 *
 * @internal
 */
async function monitorSupportsFeature(monitorClient: resrpc.IResourceMonitorClient, feature: string): Promise<boolean> {
    const req = new resproto.SupportsFeatureRequest();
    req.setId(feature);

    const result = await new Promise<boolean>((resolve, reject) => {
        monitorClient.supportsFeature(
            req,
            (err: grpc.ServiceError | null, resp: resproto.SupportsFeatureResponse | undefined) => {
                // Back-compat case - if the monitor doesn't let us ask if it supports a feature, it doesn't support
                // any features.
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

    return result;
}

/**
 * Queries the resource monitor for its capabilities and sets the appropriate
 * flags in the store.
 *
 * @internal
 **/
export async function awaitFeatureSupport(): Promise<void> {
    const monitorRef = getMonitor();
    if (monitorRef !== undefined) {
        const store = getStore();
        const [
            secrets,
            resourceReferences,
            outputValues,
            deletedWith,
            aliasSpecs,
            transforms,
            invokeTransforms,
            parameterization,
            resourceHooks,
        ] = await Promise.all(
            [
                "secrets",
                "resourceReferences",
                "outputValues",
                "deletedWith",
                "aliasSpecs",
                "transforms",
                "invokeTransforms",
                "parameterization",
                "resourceHooks",
            ].map((feature) => monitorSupportsFeature(monitorRef, feature)),
        );

        store.supportsSecrets = secrets;
        store.supportsResourceReferences = resourceReferences;
        store.supportsOutputValues = outputValues;
        store.supportsDeletedWith = deletedWith;
        store.supportsAliasSpecs = aliasSpecs;
        store.supportsTransforms = transforms;
        store.supportsInvokeTransforms = invokeTransforms;
        store.supportsParameterization = parameterization;
        store.supportsResourceHooks = resourceHooks;
    }
}

/**
 * @internal
 *  Used only for testing purposes.
 */
export function _reset(): void {
    resetOptions("", "", -1, "", "", false, "");
}

/**
 * Returns true if we will resolve missing outputs to inputs during preview
 * (`PULUMI_ENABLE_LEGACY_APPLY`).
 */
export function isLegacyApplyEnabled(): boolean {
    return options().legacyApply === true;
}

/**
 * Returns true if we will cache serialized dynamic providers on the program
 * side (the default is true).
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

/**
 * @internal
 *  Used only for testing purposes.
 */
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

/**
 * Get the project root directory.  This is the location of the Pulumi.yaml file.
 */
export function getRootDirectory(): string {
    const { rootDirectory: rootDirectory } = options();
    return rootDirectory || "";
}

/**
 * @internal
 *  Used only for testing purposes.
 */
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

/**
 * @internal
 *  Used only for testing purposes.
 */
export function _setStack(val: string | undefined) {
    const { settings } = getStore();
    settings.options.stack = val;
    return settings.options.stack;
}

/**
 * Returns true if we are currently connected to a resource monitoring service.
 */
export function hasMonitor(): boolean {
    const { settings } = getStore();
    return (!!monitor && !!options().monitorAddr) || !!settings.monitor;
}

/**
 * Returns the current resource monitoring service client for RPC
 * communications.
 */
export function getMonitor(): resrpc.IResourceMonitorClient | undefined {
    const { settings } = getStore();
    const addr = options().monitorAddr;
    if (getLocalStore() === undefined && addr !== "mock") {
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

/**
 * Waits for any pending stack transforms to register.
 */
export async function awaitStackRegistrations(): Promise<void> {
    const store = getStore();
    const callbacks = store.callbacks;
    if (callbacks === undefined) {
        return;
    }
    return await callbacks.awaitStackRegistrations();
}

/**
 * Returns the current callbacks for RPC communications.
 */
export function getCallbacks(): ICallbackServer | undefined {
    const store = getStore();
    const callbacks = store.callbacks;
    if (callbacks !== undefined) {
        return callbacks;
    }

    const monitorRef = getMonitor();
    if (monitorRef === undefined) {
        return undefined;
    }

    const callbackServer = new CallbackServer(monitorRef);
    store.callbacks = callbackServer;
    return callbackServer;
}

/**
 * @internal
 */
export interface SyncInvokes {
    requests: number;
    responses: number;
}

let syncInvokes: SyncInvokes | undefined;

/**
 * @internal
 */
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
 * Returns true if we are currently connected to an engine.
 */
export function hasEngine(): boolean {
    return !!engine && !!options().engineAddr;
}

/**
 * Returns the current engine, if any, for RPC communications back to the
 * resource engine.
 */
export function getEngine(): engrpc.IEngineClient | undefined {
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
 * Returns true if resource operations should be serialized.
 */
export function serialize(): boolean {
    return options().parallel === 1;
}

/**
 * Returns the options from the environment, which is the source of truth.
 * Options are global per process.
 *
 * For CLI driven programs, `pulumi-language-nodejs` sets environment variables
 * prior to the user program loading, meaning that options could be loaded up
 * front and cached. Automation API and multi-language components introduced
 * more complex lifecycles for runtime `options()`. These language hosts manage
 * the lifecycle of options manually throughout the lifetime of the NodeJS
 * process. In addition, NodeJS module resolution can lead to duplicate copies
 * of `@pulumi/pulumi` and thus duplicate options objects that may not be synced
 * if options are cached upfront. Mutating options must write to the environment
 * and reading options must always read directly from the environment.
 */
function options(): Options {
    const { settings } = getStore();

    return settings.options;
}

/**
 * Permanently disconnects from the server, closing the connections. It waits
 * for the existing RPC queue to drain.  If any RPCs come in afterwards,
 * however, they will crash the process.
 */
export function disconnect(): Promise<void> {
    return waitForRPCs(/*disconnectFromServers*/ true);
}

/**
 * @internal
 */
export function waitForRPCs(disconnectFromServers = false): Promise<void> {
    const localStore = getStore();
    let done: Promise<any> | undefined;
    const closeCallback: () => Promise<void> = async () => {
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
 * Returns the configured number of process listeners available.
 */
export function getMaximumListeners(): number {
    const { settings } = getStore();
    return settings.options.maximumProcessListeners;
}

/**
 * Permanently disconnects from the server, closing the connections. Unlike
 * `disconnect`. it does not wait for the existing RPC queue to drain. Any RPCs
 * that come in after this call will crash the process.
 */
export function disconnectSync(): void {
    // Otherwise, actually perform the close activities (ignoring errors and crashes).
    const store = getStore();
    if (store.callbacks) {
        store.callbacks.shutdown();
        store.callbacks = undefined;
    }

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
 * Registers a pending call to ensure that we don't prematurely disconnect from
 * the server.  It returns a function that, when invoked, signals that the RPC
 * has completed.
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
 * Returns if the engine supports package references and parameterized providers.
 */
export function supportsParameterization(): boolean {
    return getStore().supportsParameterization;
}

/**
 * Registers a resource that will become the default parent for all resources
 * without explicit parents.
 */
export async function setRootResource(res: ComponentResource): Promise<void> {
    // This is the first async point of program startup where we can query the resource monitor for its capabilities.
    await awaitFeatureSupport();

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
