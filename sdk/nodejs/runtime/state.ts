// Copyright 2016-2022, Pulumi Corporation.
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

import { AsyncLocalStorage } from "async_hooks";
import { ICallbackServer } from "./callbacks";
import * as config from "./config";
import { Stack } from "./stack";

import * as engrpc from "../proto/engine_grpc_pb";
import * as resrpc from "../proto/resource_grpc_pb";
import type { ResourceModule, ResourcePackage } from "./rpc";

const nodeEnvKeys = {
    project: "PULUMI_NODEJS_PROJECT",
    projectRoot: "PULUMI_NODEJS_PROJECT_ROOT",
    stack: "PULUMI_NODEJS_STACK",
    dryRun: "PULUMI_NODEJS_DRY_RUN",
    queryMode: "PULUMI_NODEJS_QUERY_MODE",
    parallel: "PULUMI_NODEJS_PARALLEL",
    monitorAddr: "PULUMI_NODEJS_MONITOR",
    engineAddr: "PULUMI_NODEJS_ENGINE",
    syncDir: "PULUMI_NODEJS_SYNC",

    // Unlike the values above, this value is not set by the CLI and is
    // controlled via a user-set environment variable.
    cacheDynamicProviders: "PULUMI_NODEJS_CACHE_DYNAMIC_PROVIDERS",

    organization: "PULUMI_NODEJS_ORGANIZATION",
};

const pulumiEnvKeys = {
    legacyApply: "PULUMI_ENABLE_LEGACY_APPLY",
};

/**
 * @internal
 */
export const asyncLocalStorage = new AsyncLocalStorage<Store>();

/**
 * @internal
 */
export interface WriteableOptions {
    /**
     * The name of the current project.
     */
    project?: string;

    /**
     * The name of the current stack being deployed into.
     */
    stack?: string;

    /**
     * The degree of parallelism for resource operations (default is serial).
     */
    parallel?: number;

    /**
     * A connection string to the engine's RPC, in case we need to reestablish.
     */
    engineAddr?: string;

    /**
     * A connection string to the monitor's RPC, in case we need to reestablish.
     */
    monitorAddr?: string;

    /**
     * Whether we are performing a preview (true) or a real deployment (false).
     */
    dryRun?: boolean;

    /**
     * True if we're in testing mode (allows execution without the CLI).
     */
    testModeEnabled?: boolean;

    /**
     * True if we're in query mode (does not allow resource registration).
     */
    queryMode?: boolean;

    /**
     * True if we will resolve missing outputs to inputs during preview.
     */
    legacyApply?: boolean;

    /**
     * True if we will cache serialized dynamic providers on the program side.
     */
    cacheDynamicProviders?: boolean;

    /**
     * The name of the current organization (if available).
     */
    organization?: string;

    /**
     * The number of process listeners which can be registered before writing a
     * warning.
     */
    maximumProcessListeners: number;

    /**
     * A directory containing the send/receive files for making synchronous
     * invokes to the engine.
     */
    syncDir?: string;
}

/**
 * @internal
 */
export interface Store {
    settings: {
        options: WriteableOptions;
        monitor?: resrpc.IResourceMonitorClient;
        engine?: engrpc.IEngineClient;
        rpcDone: Promise<any>;
        // Needed for legacy @pulumi/pulumi packages doing async feature checks.
        featureSupport: Record<string, boolean>;
    };
    config: Record<string, string>;
    stackResource?: Stack;
    leakCandidates: Set<Promise<any>>;
    logErrorCount: number;

    /**
     * Tells us if the resource monitor we are connected to is able to support
     * secrets across its RPC interface. When it does, we marshal outputs marked
     * with the secret bit in a special way.
     */
    supportsSecrets: boolean;

    /**
     * Tells us if the resource monitor we are connected to is able to support
     * resource references across its RPC interface. When it does, we marshal
     * resources in a special way.
     */
    supportsResourceReferences: boolean;

    /**
     * Tells u if the resource monitor we are connected to is able to support
     * output values across its RPC interface. When it does, we marshal outputs
     * in a special way.
     */
    supportsOutputValues: boolean;

    /**
     * Tells us if the resource monitor we are connected to is able to support
     * the `deletedWith` resource option across its RPC interface.
     */
    supportsDeletedWith: boolean;

    /**
     * Tells us if the resource monitor we are connected to is able to support
     * alias specs across its RPC interface. When it does, we marshal aliases in
     * a special way.
     */
    supportsAliasSpecs: boolean;

    /**
     * Tells us if the resource monitor we are connected to is able to support
     * remote transforms across its RPC interface. When it does, we marshal
     * transforms to the monitor instead of running them locally.
     */
    supportsTransforms: boolean;

    /**
     * Tells us if the resource monitor we are connected to is able to support
     * remote invoke transforms across its RPC interface. When it does, we marshal
     * transforms to the monitor instead of running them locally.
     */
    supportsInvokeTransforms: boolean;

    /**
     * Tells us if the resource monitor we are connected to is able to support
     * package references and parameterized providers.
     */
    supportsParameterization: boolean;

    /**
     * The callback service running for this deployment. This registers
     * callbacks and forwards them to the engine.
     */
    callbacks?: ICallbackServer;

    /**
     * Tracks the list of resource packages that have been registered.
     */
    resourcePackages: Map<string, ResourcePackage[]>;

    /**
     * Tracks the list of resource modules that have been registered.
     */
    resourceModules: Map<string, ResourceModule[]>;
}

/**
 * @internal
 */
export class LocalStore implements Store {
    settings = {
        options: {
            organization: process.env[nodeEnvKeys.organization],
            project: process.env[nodeEnvKeys.project] || "project",
            projectRoot: process.env[nodeEnvKeys.projectRoot] || "projectRoot",
            stack: process.env[nodeEnvKeys.stack] || "stack",
            dryRun: process.env[nodeEnvKeys.dryRun] === "true",
            queryMode: process.env[nodeEnvKeys.queryMode] === "true",
            monitorAddr: process.env[nodeEnvKeys.monitorAddr],
            engineAddr: process.env[nodeEnvKeys.engineAddr],
            syncDir: process.env[nodeEnvKeys.syncDir],
            cacheDynamicProviders: process.env[nodeEnvKeys.cacheDynamicProviders] !== "false",
            legacyApply: process.env[pulumiEnvKeys.legacyApply] === "true",
            maximumProcessListeners: 30,
        },
        rpcDone: Promise.resolve(),
        featureSupport: {},
    };
    config = {
        [config.configEnvKey]: process.env[config.configEnvKey] || "",
        [config.configSecretKeysEnvKey]: process.env[config.configSecretKeysEnvKey] || "",
    };
    stackResource = undefined;

    /**
     * Tracks the list of potential leak candidates.
     */
    leakCandidates = new Set<Promise<any>>();

    logErrorCount = 0;

    supportsSecrets = false;
    supportsResourceReferences = false;
    supportsOutputValues = false;
    supportsDeletedWith = false;
    supportsAliasSpecs = false;
    supportsTransforms = false;
    supportsInvokeTransforms = false;
    supportsParameterization = false;
    resourcePackages = new Map<string, ResourcePackage[]>();
    resourceModules = new Map<string, ResourceModule[]>();
}

/**
 * Get the root stack resource for the current stack deployment.
 *
 * @internal
 */
export function getStackResource(): Stack | undefined {
    const { stackResource } = getStore();
    return stackResource;
}

/**
 * Get the resource package map for the current stack deployment.
 *
 * @internal
 */
export function getResourcePackages(): Map<string, ResourcePackage[]> {
    const store = getGlobalStore();
    if (store.resourcePackages === undefined) {
        // resourcePackages can be undefined if an older SDK where it was not defined is created it.
        // In this case, we should initialize it to an empty map.
        store.resourcePackages = new Map<string, ResourcePackage[]>();
    }
    return store.resourcePackages;
}

/**
 * Get the resource module map for the current stack deployment.
 *
 * @internal
 */
export function getResourceModules(): Map<string, ResourceModule[]> {
    const store = getGlobalStore();
    if (store.resourceModules === undefined) {
        // resourceModules can be undefined if an older SDK where it was not defined is created it.
        // In this case, we should initialize it to an empty map.
        store.resourceModules = new Map<string, ResourceModule[]>();
    }
    return store.resourceModules;
}

/**
 * @internal
 */
export function setStackResource(newStackResource?: Stack) {
    const localStore = getStore();
    globalThis.stackResource = newStackResource;
    localStore.stackResource = newStackResource;
}

declare global {
    /* eslint-disable no-var */
    var globalStore: Store;
    var stackResource: Stack | undefined;
}

/**
 * @internal
 */
export function getLocalStore(): Store | undefined {
    return asyncLocalStorage.getStore();
}

(<any>getLocalStore).captureReplacement = () => {
    const returnFunc = () => {
        if (global.globalStore === undefined) {
            global.globalStore = new LocalStore();
        }
        return global.globalStore;
    };
    return returnFunc;
};

/**
 * @internal
 */
export const getStore = () => {
    const localStore = getLocalStore();
    if (localStore === undefined) {
        if (global.globalStore === undefined) {
            global.globalStore = new LocalStore();
        }
        return global.globalStore;
    }
    return localStore;
};

(<any>getStore).captureReplacement = () => {
    const returnFunc = () => {
        if (global.globalStore === undefined) {
            global.globalStore = new LocalStore();
        }
        return global.globalStore;
    };
    return returnFunc;
};

/**
 * @internal
 */
export const getGlobalStore = () => {
    if (global.globalStore === undefined) {
        global.globalStore = new LocalStore();
    }
    return global.globalStore;
};
