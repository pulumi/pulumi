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
import { Stack } from "./stack";
import * as config from "./config";
import type { ResourceModule, ResourcePackage } from "./rpc";

const nodeEnvKeys = {
    project: "PULUMI_NODEJS_PROJECT",
    stack: "PULUMI_NODEJS_STACK",
    dryRun: "PULUMI_NODEJS_DRY_RUN",
    queryMode: "PULUMI_NODEJS_QUERY_MODE",
    parallel: "PULUMI_NODEJS_PARALLEL",
    monitorAddr: "PULUMI_NODEJS_MONITOR",
    engineAddr: "PULUMI_NODEJS_ENGINE",
    syncDir: "PULUMI_NODEJS_SYNC",
    // this value is not set by the CLI and is controlled via a user set env var unlike the values above
    cacheDynamicProviders: "PULUMI_NODEJS_CACHE_DYNAMIC_PROVIDERS",
    organization: "PULUMI_NODEJS_ORGANIZATION",
};

const pulumiEnvKeys = {
    legacyApply: "PULUMI_ENABLE_LEGACY_APPLY",
};

/** @internal */
export const asyncLocalStorage = new AsyncLocalStorage<Store>();

/** @internal */
export interface WriteableOptions {
    project?: string; // the name of the current project.
    stack?: string; // the name of the current stack being deployed into.
    parallel?: number; // the degree of parallelism for resource operations (default is serial).
    engineAddr?: string; // a connection string to the engine's RPC, in case we need to reestablish.
    monitorAddr?: string; // a connection string to the monitor's RPC, in case we need to reestablish.
    dryRun?: boolean; // whether we are performing a preview (true) or a real deployment (false).
    testModeEnabled?: boolean; // true if we're in testing mode (allows execution without the CLI).
    queryMode?: boolean; // true if we're in query mode (does not allow resource registration).
    legacyApply?: boolean; // true if we will resolve missing outputs to inputs during preview.
    cacheDynamicProviders?: boolean; // true if we will cache serialized dynamic providers on the program side.
    organization?: string; // the name of the current organization (if available).
    maximumProcessListeners: number; // the number of process listeners which can be registered before writing a warning.
    /**
     * Directory containing the send/receive files for making synchronous invokes to the engine.
     */
    syncDir?: string;
}

/** @internal */
export interface Store {
    settings: {
        options: WriteableOptions;
        monitor?: any;
        engine?: any;
        rpcDone: Promise<any>;
        featureSupport: Record<string, boolean>;
    };
    config: Record<string, string>;
    stackResource?: Stack;
    leakCandidates: Set<Promise<any>>;
    resourcePackages: Map<string, ResourcePackage[]>;
    resourceModules: Map<string, ResourceModule[]>;
}

/** @internal */
export class LocalStore implements Store {
    settings = {
        options: {
            organization: process.env[nodeEnvKeys.organization],
            project: process.env[nodeEnvKeys.project] || "project",
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
     * leakCandidates tracks the list of potential leak candidates.
     */
    leakCandidates = new Set<Promise<any>>();

    resourcePackages = new Map<string, ResourcePackage[]>();
    resourceModules = new Map<string, ResourceModule[]>();
}

/** Get the root stack resource for the current stack deployment
 * @internal
 */
export function getStackResource(): Stack | undefined {
    const { stackResource } = getStore();
    return stackResource;
}

/** @internal */
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

/** @internal */
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

/** @internal */
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
