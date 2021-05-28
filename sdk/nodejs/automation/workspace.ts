// Copyright 2016-2020, Pulumi Corporation.
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

import { ConfigMap, ConfigValue } from "./config";
import { ProjectSettings } from "./projectSettings";
import { OutputMap } from "./stack";
import { StackSettings } from "./stackSettings";

/**
 * Workspace is the execution context containing a single Pulumi project, a program, and multiple stacks.
 * Workspaces are used to manage the execution environment, providing various utilities such as plugin
 * installation, environment configuration ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
 *
 * @alpha
 */
export interface Workspace {
    /**
     * The working directory to run Pulumi CLI commands
     */
    readonly workDir: string;
    /**
     * The directory override for CLI metadata if set.
     * This customizes the location of $PULUMI_HOME where metadata is stored and plugins are installed.
     */
    readonly pulumiHome?: string;
    /**
     * The secrets provider to use for encryption and decryption of stack secrets.
     * See: https://www.pulumi.com/docs/intro/concepts/config/#available-encryption-providers
     */
    readonly secretsProvider?: string;
    /**
     * The version of the underlying Pulumi CLI/Engine.
     */
    readonly pulumiVersion: string;
    /**
     *  The inline program `PulumiFn` to be used for Preview/Update operations if any.
     *  If none is specified, the stack will refer to ProjectSettings for this information.
     */
    program?: PulumiFn;
    /**
     * Environment values scoped to the current workspace. These will be supplied to every Pulumi command.
     */
    envVars: { [key: string]: string };
    /**
     * Returns the settings object for the current project if any.
     */
    projectSettings(): Promise<ProjectSettings>;

    /**
     * Overwrites the settings object in the current project.
     * There can only be a single project per workspace. Fails is new project name does not match old.
     *
     * @param settings The settings object to save.
     */
    saveProjectSettings(settings: ProjectSettings): Promise<void>;
    /**
     * Returns the settings object for the stack matching the specified stack name if any.
     *
     * @param stackName The name of the stack.
     */
    stackSettings(stackName: string): Promise<StackSettings>;
    /**
     * overwrites the settings object for the stack matching the specified stack name.
     *
     * @param stackName The name of the stack to operate on.
     * @param settings The settings object to save.
     */
    saveStackSettings(stackName: string, settings: StackSettings): Promise<void>;
    /**
     * serializeArgsForOp is hook to provide additional args to every CLI commands before they are executed.
     * Provided with stack name,
     * returns a list of args to append to an invoked command ["--config=...", ]
     * LocalWorkspace does not utilize this extensibility point.
     */
    serializeArgsForOp(stackName: string): Promise<string[]>;
    /**
     * postCommandCallback is a hook executed after every command. Called with the stack name.
     * An extensibility point to perform workspace cleanup (CLI operations may create/modify a Pulumi.stack.yaml)
     * LocalWorkspace does not utilize this extensibility point.
     */
    postCommandCallback(stackName: string): Promise<void>;
    /**
     * Returns the value associated with the specified stack name and key,
     * scoped to the Workspace.
     *
     * @param stackName The stack to read config from
     * @param key The key to use for the config lookup
     */
    getConfig(stackName: string, key: string): Promise<ConfigValue>;
    /**
     * Returns the config map for the specified stack name, scoped to the current Workspace.
     *
     * @param stackName The stack to read config from
     */
    getAllConfig(stackName: string): Promise<ConfigMap>;
    /**
     * Sets the specified key-value pair on the provided stack name.
     *
     * @param stackName The stack to operate on
     * @param key The config key to set
     * @param value The value to set
     */
    setConfig(stackName: string, key: string, value: ConfigValue): Promise<void>;
    /**
     * Sets all values in the provided config map for the specified stack name.
     *
     * @param stackName The stack to operate on
     * @param config The `ConfigMap` to upsert against the existing config.
     */
    setAllConfig(stackName: string, config: ConfigMap): Promise<void>;
    /**
     * Removes the specified key-value pair on the provided stack name.
     *
     * @param stackName The stack to operate on
     * @param key The config key to remove
     */
    removeConfig(stackName: string, key: string): Promise<void>;
    /**
     *
     * Removes all values in the provided key list for the specified stack name.
     *
     * @param stackName The stack to operate on
     * @param keys The list of keys to remove from the underlying config
     */
    removeAllConfig(stackName: string, keys: string[]): Promise<void>;
    /**
     * Gets and sets the config map used with the last update for Stack matching stack name.
     *
     * @param stackName The stack to refresh
     */
    refreshConfig(stackName: string): Promise<ConfigMap>;
    /**
     * Returns the currently authenticated user.
     */
    whoAmI(): Promise<WhoAmIResult>;
    /**
     * Returns a summary of the currently selected stack, if any.
     */
    stack(): Promise<StackSummary | undefined>;
    /**
     * Creates and sets a new stack with the stack name, failing if one already exists.
     *
     * @param stackName The stack to create.
     */
    createStack(stackName: string): Promise<void>;
    /**
     * Selects and sets an existing stack matching the stack name, failing if none exists.
     *
     * @param stackName The stack to select.
     */
    selectStack(stackName: string): Promise<void>;
    /**
     * Deletes the stack and all associated configuration and history.
     *
     * @param stackName The stack to remove
     */
    removeStack(stackName: string): Promise<void>;
    /**
     * Returns all Stacks created under the current Project.
     * This queries underlying backend and may return stacks not present in the Workspace (as Pulumi.<stack>.yaml files).
     */
    listStacks(): Promise<StackSummary[]>;
    /**
     * Installs a plugin in the Workspace, for example to use cloud providers like AWS or GCP.
     *
     * @param name the name of the plugin.
     * @param version the version of the plugin e.g. "v1.0.0".
     * @param kind the kind of plugin e.g. "resource"
     */
    installPlugin(name: string, version: string, kind?: string): Promise<void>;
    /**
     * Removes a plugin from the Workspace matching the specified name and version.
     *
     * @param name the optional name of the plugin.
     * @param versionRange optional semver range to check when removing plugins matching the given name
     *  e.g. "1.0.0", ">1.0.0".
     * @param kind he kind of plugin e.g. "resource"
     */
    removePlugin(name?: string, versionRange?: string, kind?: string): Promise<void>;
    /**
     * Returns a list of all plugins installed in the Workspace.
     */
    listPlugins(): Promise<PluginInfo[]>;
    /**
     * exportStack exports the deployment state of the stack.
     * This can be combined with Workspace.importStack to edit a stack's state (such as recovery from failed deployments).
     *
     * @param stackName the name of the stack.
     */
    exportStack(stackName: string): Promise<Deployment>;
    /**
     * importStack imports the specified deployment state into a pre-existing stack.
     * This can be combined with Workspace.exportStack to edit a stack's state (such as recovery from failed deployments).
     *
     * @param stackName the name of the stack.
     * @param state the stack state to import.
     */
    importStack(stackName: string, state: Deployment): Promise<void>;
    /**
     * Gets the current set of Stack outputs from the last Stack.up().
     * @param stackName the name of the stack.
     */
    stackOutputs(stackName: string): Promise<OutputMap>;
}

/**
 * A summary of the status of a given stack.
 */
export interface StackSummary {
    name: string;
    current: boolean;
    lastUpdate?: string;
    updateInProgress: boolean;
    resourceCount?: number;
    url?: string;
}

/**
 * Deployment encapsulates the state of a stack deployment.
 */
export interface Deployment {
    /**
     * Version indicates the schema of the encoded deployment.
     */
    version: number;
    /**
     * The pulumi deployment.
     */
    // TODO: Expand type to encapsulate deployment.
    deployment: any;
}

/**
 * A Pulumi program as an inline function (in process).
 */
export type PulumiFn = () => Promise<Record<string, any> | void>;

/**
 * The currently logged-in Pulumi identity.
 */
export interface WhoAmIResult {
    user: string;
}

export interface PluginInfo {
    name: string;
    path: string;
    kind: PluginKind;
    version?: string;
    size: number;
    installTime: Date;
    lastUsedTime: Date;
    serverURL: string;
}

export type PluginKind = "analyzer" | "language" | "resource";
