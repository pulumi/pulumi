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

import { PulumiCommand } from "./cmd";
import { ConfigMap, ConfigValue } from "./config";
import { ListOptions, RemoveOptions } from "./localWorkspace";
import { ProjectSettings } from "./projectSettings";
import { OutputMap } from "./stack";
import { StackSettings } from "./stackSettings";
import { TagMap } from "./tag";

/**
 * {@link Workspace} is the execution context containing a single Pulumi
 * project, a program, and multiple {@link Stack}s. Workspaces are used to
 * manage the execution environment, providing various utilities such as plugin
 * installation, environment configuration (`$PULUMI_HOME`), and creation,
 * deletion, and listing of Stacks.
 *
 * @alpha
 */
export interface Workspace {
    /**
     * The working directory to run Pulumi CLI commands.
     */
    readonly workDir: string;

    /**
     * The directory override for CLI metadata if set. This customizes the
     * location of `$PULUMI_HOME` where metadata is stored and plugins are
     * installed.
     */
    readonly pulumiHome?: string;

    /**
     * The secrets provider to use for encryption and decryption of stack
     * secrets.
     *
     * @see https://www.pulumi.com/docs/intro/concepts/secrets/#available-encryption-providers
     */
    readonly secretsProvider?: string;

    /**
     * The version of the underlying Pulumi CLI/engine.
     */
    readonly pulumiVersion: string;

    /**
     * The underlying Pulumi CLI.
     */
    readonly pulumiCommand: PulumiCommand;

    /**
     * The inline program {@link PulumiFn} to be used for preview/update
     * operations, if any. If none is specified, the stack will refer to {@link
     * ProjectSettings} for this information.
     */
    program?: PulumiFn;

    /**
     * Environment values scoped to the current workspace. These will be
     * supplied to every Pulumi command.
     */
    envVars: { [key: string]: string };

    /**
     * Returns the settings object for the current project, if any.
     */
    projectSettings(): Promise<ProjectSettings>;

    /**
     * Overwrites the settings object in the current project. There can only be
     * a single project per workspace. Fails if the new project name does not
     * match the old one.
     *
     * @param settings
     *  The settings object to save.
     */
    saveProjectSettings(settings: ProjectSettings): Promise<void>;

    /**
     * Returns the settings object for the stack matching the specified stack
     * name, if any.
     *
     * @param stackName
     *  The name of the stack.
     */
    stackSettings(stackName: string): Promise<StackSettings>;

    /**
     * Overwrites the settings object for the stack matching the specified stack
     * name.
     *
     * @param stackName
     *  The name of the stack to operate on.
     * @param settings
     *  The settings object to save.
     */
    saveStackSettings(stackName: string, settings: StackSettings): Promise<void>;

    /**
     * A hook to provide additional arguments to every CLI command before they
     * are executed. Provided with the stack name, this should return a list of
     * arguments to append to an invoked command (e.g. `["--config=...", ...]`).
     */
    serializeArgsForOp(stackName: string): Promise<string[]>;

    /**
     * A hook executed after every command. Called with the stack name. An
     * extensibility point to perform workspace cleanup (CLI operations may
     * create/modify a `Pulumi.stack.yaml`)
     */
    postCommandCallback(stackName: string): Promise<void>;

    /**
     * Adds environments to the end of a stack's import list. Imported
     * environments are merged in order per the ESC merge rules. The list of
     * environments behaves as if it were the import list in an anonymous
     * environment.
     *
     * @param stackName
     *  The stack to operate on
     * @param environments
     *  The names of the environments to add to the stack's configuration
     */
    addEnvironments(stackName: string, ...environments: string[]): Promise<void>;

    /**
     * Returns the list of environments associated with the specified stack
     * name.
     *
     * @param stackName
     *  The stack to operate on
     */
    listEnvironments(stackName: string): Promise<string[]>;

    /**
     * Removes an environment from a stack's import list.
     *
     * @param stackName
     *  The stack to operate on
     * @param environment
     *  The name of the environment to remove from the stack's configuration
     */
    removeEnvironment(stackName: string, environment: string): Promise<void>;

    /**
     * Returns the value associated with the specified stack name and key,
     * scoped to the Workspace.
     *
     * @param stackName
     *  The stack to read config from
     * @param key
     *  The key to use for the config lookup
     * @param path
     *  The key contains a path to a property in a map or list to get
     */
    getConfig(stackName: string, key: string, path?: boolean): Promise<ConfigValue>;

    /**
     * Returns the value associated with the specified stack name and key,
     * scoped to the Workspace.
     *
     * @param stackName
     *  The stack to read config from
     * @param key
     *  The key to use for the config lookup
     * @param opts
     *  The options to use for the config lookup
     */
    getConfigWithOptions(stackName: string, key: string, opts?: ConfigOptions): Promise<ConfigValue>;

    /**
     * Returns the config map for the specified stack name, scoped to the
     * current Workspace.
     *
     * @param stackName
     *  The stack to read config from
     */
    getAllConfig(stackName: string): Promise<ConfigMap>;

    /**
     * Returns the config map for the specified stack name with GetAllConfigOptions
     * @param stackName
     *  The stack to read config from
     * @param opts
     *  The options to use for the config lookup
     */
    getAllConfigWithOptions(stackName: string, opts?: GetAllConfigOptions): Promise<ConfigMap>;

    /**
     * Sets the specified key-value pair on the provided stack name.
     *
     * @param stackName
     *  The stack to operate on
     * @param key
     *  The config key to set
     * @param value
     *  The value to set
     * @param path
     *  The key contains a path to a property in a map or list to set
     */
    setConfig(stackName: string, key: string, value: ConfigValue, path?: boolean): Promise<void>;

    /**
     * Sets the specified key-value pair on the provided stack name, with options
     * @param stackName
     *  The stack to operate on
     * @param key
     *  The config key to set
     * @param value
     *  The value to set
     * @param opts
     *  The options to use for the config lookup
     **/
    setConfigWithOptions(stackName: string, key: string, value: ConfigValue, opts?: ConfigOptions): Promise<void>;

    /**
     * Sets all values in the provided config map for the specified stack name.
     *
     * @param stackName
     *  The stack to operate on
     * @param config
     *  The {@link ConfigMap} to upsert against the existing config
     * @param path
     *  The keys contain a path to a property in a map or list to set
     */
    setAllConfig(stackName: string, config: ConfigMap, path?: boolean): Promise<void>;

    /**
     * Sets all values in the provided config map for the specified stack name, with options. {@link LocalWorkspace} writes the config to the matching config file.
     * @param stackName
     *  The stack to operate on
     * @param config
     *  The {@link ConfigMap} to upsert against the existing config
     * @param opts
     *  The options to use for the config lookup
     **/
    setAllConfigWithOptions(stackName: string, config: ConfigMap, opts?: ConfigOptions): Promise<void>;

    /**
     * Removes the specified key-value pair on the provided stack name.
     *
     * @param stackName
     *  The stack to operate on
     * @param key
     *  The config key to remove
     * @param path
     *  The key contains a path to a property in a map or list to remove
     */
    removeConfig(stackName: string, key: string, path?: boolean): Promise<void>;

    /**
     * Removes the specified key-value pair on the provided stack name with options.
     *
     * @param stackName
     *  The stack to operate on
     * @param key
     *  The config key to remove
     * @param opts
     *  The options to use for the config lookup
     */
    removeConfigWithOptions(stackName: string, key: string, opts?: ConfigOptions): Promise<void>;

    /**
     * Removes all values in the provided key list for the specified stack name.
     *
     * @param stackName
     *  The stack to operate on
     * @param keys
     *  The list of keys to remove from the underlying config
     * @param path
     *  The keys contain a path to a property in a map or list to remove
     */
    removeAllConfig(stackName: string, keys: string[], path?: boolean): Promise<void>;

    /**
     * Removes all values in the provided key list for the specified stack name with options.
     *
     * @param stackName
     *  The stack to operate on
     * @param keys
     *  The list of keys to remove from the underlying config
     * @param opts
     *  The options to use for the config lookup
     */
    removeAllConfigWithOptions(stackName: string, keys: string[], opts?: ConfigOptions): Promise<void>;

    /**
     * Gets and sets the config map used with the last update for Stack matching
     * stack name.
     *
     * @param stackName
     *  The stack to refresh
     */
    refreshConfig(stackName: string): Promise<ConfigMap>;

    /**
     * Returns the value associated with the specified stack name and key,
     * scoped to the {@link Workspace}.
     *
     * @param stackName
     *  The stack to read tag metadata from.
     * @param key
     *  The key to use for the tag lookup.
     */
    getTag(stackName: string, key: string): Promise<string>;

    /**
     * Sets the specified key-value pair on the provided stack name.
     *
     * @param stackName
     *  The stack to operate on.
     * @param key
     *  The tag key to set.
     * @param value
     *  The tag value to set.
     */
    setTag(stackName: string, key: string, value: string): Promise<void>;

    /**
     * Removes the specified key-value pair on the provided stack name.
     *
     * @param stackName
     *  The stack to operate on.
     * @param key
     *  The tag key to remove.
     */
    removeTag(stackName: string, key: string): Promise<void>;

    /**
     * Returns the tag map for the specified tag name, scoped to the current
     * {@link Workspace.}
     *
     * @param stackName
     *  The stack to read tag metadata from.
     */
    listTags(stackName: string): Promise<TagMap>;

    /**
     * Returns information about the currently authenticated user.
     */
    whoAmI(): Promise<WhoAmIResult>;

    /**
     * Returns a summary of the currently selected stack, if any.
     */
    stack(): Promise<StackSummary | undefined>;

    /**
     * Creates and sets a new stack with the stack name, failing if one already
     * exists.
     *
     * @param stackName
     *  The stack to create.
     */
    createStack(stackName: string): Promise<void>;

    /**
     * Selects and sets an existing stack matching the stack name, failing if
     * none exists.
     *
     * @param stackName
     *  The stack to select.
     */
    selectStack(stackName: string): Promise<void>;
    /**
     * Deletes the stack and all associated configuration and history.
     *
     * @param stackName
     *  The stack to remove
     */
    removeStack(stackName: string, opts?: RemoveOptions): Promise<void>;
    /**
     * Returns all stacks from the underlying backend based on the provided
     * options. This queries backend and may return stacks not present in the
     * {@link Workspace} as `Pulumi.<stack>.yaml` files.
     *
     * @param opts
     *  Options to customize the behavior of the list.
     */
    listStacks(opts?: ListOptions): Promise<StackSummary[]>;

    /**
     * Installs a plugin in the workspace, for example to use cloud providers
     * like AWS or GCP.
     *
     * @param name
     *  The name of the plugin.
     * @param version
     *  The version of the plugin e.g. "v1.0.0".
     * @param server
     *  The server to install the plugin into
     */
    installPluginFromServer(name: string, version: string, server: string): Promise<void>;

    /**
     * Installs a plugin in the workspace from a remote server, for example a
     * third-party plugin.
     *
     * @param name
     *  The name of the plugin.
     * @param version
     *  The version of the plugin e.g. "v1.0.0".
     * @param kind
     *  The kind of plugin e.g. "resource"
     */
    installPlugin(name: string, version: string, kind?: string): Promise<void>;

    /**
     * Removes a plugin from the workspace matching the specified name and
     * version.
     *
     * @param name
     *  The optional name of the plugin.
     * @param versionRange
     *  An optional semver range to check when removing plugins matching the
     *  given name e.g. "1.0.0", ">1.0.0".
     * @param kind
     *  The kind of plugin e.g. "resource"
     */
    removePlugin(name?: string, versionRange?: string, kind?: string): Promise<void>;

    /**
     * Returns a list of all plugins installed in the workspace.
     */
    listPlugins(): Promise<PluginInfo[]>;

    /**
     * Exports the deployment state of the stack. This can be combined with
     * {@link Workspace.importStack} to edit a stack's state (such as recovery
     * from failed deployments).
     *
     * @param stackName the name of the stack.
     */
    exportStack(stackName: string): Promise<Deployment>;

    /**
     * Imports the specified deployment state into a pre-existing stack. This
     * can be combined with {@link Workspace.exportStack} to edit a stack's
     * state (such as recovery from failed deployments).
     *
     * @param stackName
     *  The name of the stack.
     * @param state
     *  The stack state to import.
     */
    importStack(stackName: string, state: Deployment): Promise<void>;

    /**
     * Gets the current set of Stack outputs from the last {@link Stack.up}.
     *
     * @param stackName
     *  The name of the stack.
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
    updateInProgress?: boolean;
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
 * The currently logged-in Pulumi access token.
 */
export interface TokenInfomation {
    name: string;
    organization?: string;
    team?: string;
}

/**
 * The currently logged-in Pulumi identity.
 */
export interface WhoAmIResult {
    user: string;
    url?: string;
    organizations?: string[];
    tokenInformation?: TokenInfomation;
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

export interface GetAllConfigOptions {
    /**
     * Use the configuration values in the specified file rather than detecting the file name.
     */
    configFile?: string;
    /**
     * Show secret values when getting config.
     */
    showSecrets?: boolean;
}

export interface ConfigOptions {
    /**
     * Allows to use the path flag while getting/setting the configuration.
     */
    path?: boolean;
    /**
     * Use the configuration values in the specified file rather than detecting the file name.
     */
    configFile?: string;
}
