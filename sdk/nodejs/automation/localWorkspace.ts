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

import * as fs from "fs";
import * as yaml from "js-yaml";
import * as os from "os";
import * as semver from "semver";
import * as upath from "upath";

import { CommandResult, runPulumiCmd } from "./cmd";
import { ConfigMap, ConfigValue } from "./config";
import { minimumVersion } from "./minimumVersion";
import { ProjectSettings } from "./projectSettings";
import { OutputMap, Stack } from "./stack";
import { StackSettings, stackSettingsSerDeKeys } from "./stackSettings";
import { Deployment, PluginInfo, PulumiFn, StackSummary, WhoAmIResult, Workspace } from "./workspace";

const SKIP_VERSION_CHECK_VAR = "PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK";

/**
 * LocalWorkspace is a default implementation of the Workspace interface.
 * A Workspace is the execution context containing a single Pulumi project, a program,
 * and multiple stacks. Workspaces are used to manage the execution environment,
 * providing various utilities such as plugin installation, environment configuration
 * ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
 * LocalWorkspace relies on Pulumi.yaml and Pulumi.<stack>.yaml as the intermediate format
 * for Project and Stack settings. Modifying ProjectSettings will
 * alter the Workspace Pulumi.yaml file, and setting config on a Stack will modify the Pulumi.<stack>.yaml file.
 * This is identical to the behavior of Pulumi CLI driven workspaces.
 *
 * @alpha
 */
export class LocalWorkspace implements Workspace {
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
     *  The inline program `PulumiFn` to be used for Preview/Update operations if any.
     *  If none is specified, the stack will refer to ProjectSettings for this information.
     */
    program?: PulumiFn;
    /**
     * Environment values scoped to the current workspace. These will be supplied to every Pulumi command.
     */
    envVars: { [key: string]: string };
    private _pulumiVersion?: semver.SemVer;
    /**
     * The version of the underlying Pulumi CLI/Engine.
     */
    public get pulumiVersion(): string {
        return this._pulumiVersion!.toString();
    }
    private ready: Promise<any[]>;
    /**
     * Creates a workspace using the specified options. Used for maximal control and customization
     * of the underlying environment before any stacks are created or selected.
     *
     * @param opts Options used to configure the Workspace
     */
    static async create(opts: LocalWorkspaceOptions): Promise<LocalWorkspace> {
        const ws = new LocalWorkspace(opts);
        await ws.ready;
        return ws;
    }
    /**
     * Creates a Stack with a LocalWorkspace utilizing the local Pulumi CLI program from the specified workDir.
     * This is a way to create drivers on top of pre-existing Pulumi programs. This Workspace will pick up
     * any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml).
     *
     * @param args A set of arguments to initialize a Stack with a pre-configured Pulumi CLI program that already exists on disk.
     * @param opts Additional customizations to be applied to the Workspace.
     */
    static async createStack(args: LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    /**
     * Creates a Stack with a LocalWorkspace utilizing the specified inline (in process) Pulumi program.
     * This program is fully debuggable and runs in process. If no Project option is specified, default project settings
     * will be created on behalf of the user. Similarly, unless a `workDir` option is specified, the working directory
     * will default to a new temporary directory provided by the OS.
     *
     * @param args A set of arguments to initialize a Stack with and inline `PulumiFn` program that runs in process.
     * @param opts Additional customizations to be applied to the Workspace.
     */
    static async createStack(args: InlineProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    static async createStack(args: InlineProgramArgs | LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack> {
        if (isInlineProgramArgs(args)) {
            return await this.inlineSourceStackHelper(args, Stack.create, opts);
        } else if (isLocalProgramArgs(args)) {
            return await this.localSourceStackHelper(args, Stack.create, opts);
        }
        throw new Error(`unexpected args: ${args}`);
    }
    /**
     * Selects a Stack with a LocalWorkspace utilizing the local Pulumi CLI program from the specified workDir.
     * This is a way to create drivers on top of pre-existing Pulumi programs. This Workspace will pick up
     * any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml).
     *
     * @param args A set of arguments to initialize a Stack with a pre-configured Pulumi CLI program that already exists on disk.
     * @param opts Additional customizations to be applied to the Workspace.
     */
    static async selectStack(args: LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    /**
     * Selects an existing Stack with a LocalWorkspace utilizing the specified inline (in process) Pulumi program.
     * This program is fully debuggable and runs in process. If no Project option is specified, default project settings
     * will be created on behalf of the user. Similarly, unless a `workDir` option is specified, the working directory
     * will default to a new temporary directory provided by the OS.
     *
     * @param args A set of arguments to initialize a Stack with and inline `PulumiFn` program that runs in process.
     * @param opts Additional customizations to be applied to the Workspace.
     */
    static async selectStack(args: InlineProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    static async selectStack(args: InlineProgramArgs | LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack> {
        if (isInlineProgramArgs(args)) {
            return await this.inlineSourceStackHelper(args, Stack.select, opts);
        } else if (isLocalProgramArgs(args)) {
            return await this.localSourceStackHelper(args, Stack.select, opts);
        }
        throw new Error(`unexpected args: ${args}`);
    }
    /**
     * Creates or selects an existing Stack with a LocalWorkspace utilizing the specified inline (in process) Pulumi CLI program.
     * This program is fully debuggable and runs in process. If no Project option is specified, default project settings
     * will be created on behalf of the user. Similarly, unless a `workDir` option is specified, the working directory
     * will default to a new temporary directory provided by the OS.
     *
     * @param args A set of arguments to initialize a Stack with a pre-configured Pulumi CLI program that already exists on disk.
     * @param opts Additional customizations to be applied to the Workspace.
     */
    static async createOrSelectStack(args: LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    /**
     * Creates or selects an existing Stack with a LocalWorkspace utilizing the specified inline Pulumi CLI program.
     * This program is fully debuggable and runs in process. If no Project option is specified, default project settings will be created
     * on behalf of the user. Similarly, unless a `workDir` option is specified, the working directory will default
     * to a new temporary directory provided by the OS.
     *
     * @param args A set of arguments to initialize a Stack with and inline `PulumiFn` program that runs in process.
     * @param opts Additional customizations to be applied to the Workspace.
     */
    static async createOrSelectStack(args: InlineProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    static async createOrSelectStack(args: InlineProgramArgs | LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack> {
        if (isInlineProgramArgs(args)) {
            return await this.inlineSourceStackHelper(args, Stack.createOrSelect, opts);
        } else if (isLocalProgramArgs(args)) {
            return await this.localSourceStackHelper(args, Stack.createOrSelect, opts);
        }
        throw new Error(`unexpected args: ${args}`);
    }
    private static async localSourceStackHelper(
        args: LocalProgramArgs, initFn: StackInitializer, opts?: LocalWorkspaceOptions,
    ): Promise<Stack> {
        let wsOpts = { workDir: args.workDir };
        if (opts) {
            wsOpts = { ...opts, workDir: args.workDir };
        }

        const ws = new LocalWorkspace(wsOpts);
        await ws.ready;

        return await initFn(args.stackName, ws);
    }
    private static async inlineSourceStackHelper(
        args: InlineProgramArgs, initFn: StackInitializer, opts?: LocalWorkspaceOptions,
    ): Promise<Stack> {
        let wsOpts: LocalWorkspaceOptions = { program: args.program };
        if (opts) {
            wsOpts = { ...opts, program: args.program };
        }

        if (!wsOpts.projectSettings) {
            if (!!wsOpts.workDir) {
                try {
                    // Try to load the project settings.
                    loadProjectSettings(wsOpts.workDir);
                } catch (e) {
                    // If it failed to find the project settings file, set a default project.
                    if (e.toString().includes("failed to find project settings")) {
                        wsOpts.projectSettings = defaultProject(args.projectName);
                    } else {
                        throw e;
                    }
                }
            } else {
                wsOpts.projectSettings = defaultProject(args.projectName);
            }
        }

        const ws = new LocalWorkspace(wsOpts);
        await ws.ready;

        return await initFn(args.stackName, ws);
    }
    private constructor(opts?: LocalWorkspaceOptions) {
        let dir = "";
        let envs = {};

        if (opts) {
            const { workDir, pulumiHome, program, envVars, secretsProvider } = opts;
            if (workDir) {
                dir = workDir;
            }
            this.pulumiHome = pulumiHome;
            this.program = program;
            this.secretsProvider = secretsProvider;
            envs = { ...envVars };
        }

        if (!dir) {
            dir = fs.mkdtempSync(upath.joinSafe(os.tmpdir(), "automation-"));
        }
        this.workDir = dir;
        this.envVars = envs;

        const readinessPromises: Promise<any>[] = [this.getPulumiVersion(minimumVersion)];

        if (opts && opts.projectSettings) {
            readinessPromises.push(this.saveProjectSettings(opts.projectSettings));
        }
        if (opts && opts.stackSettings) {
            for (const [name, value] of Object.entries(opts.stackSettings)) {
                readinessPromises.push(this.saveStackSettings(name, value));
            }
        }

        this.ready = Promise.all(readinessPromises);
    }
    /**
     * Returns the settings object for the current project if any
     * LocalWorkspace reads settings from the Pulumi.yaml in the workspace.
     * A workspace can contain only a single project at a time.
     */
    async projectSettings(): Promise<ProjectSettings> {
        return loadProjectSettings(this.workDir);
    }
    /**
     * Overwrites the settings object in the current project.
     * There can only be a single project per workspace. Fails if new project name does not match old.
     * LocalWorkspace writes this value to a Pulumi.yaml file in Workspace.WorkDir().
     *
     * @param settings The settings object to save to the Workspace.
     */
    async saveProjectSettings(settings: ProjectSettings): Promise<void> {
        let foundExt = ".yaml";
        for (const ext of settingsExtensions) {
            const testPath = upath.joinSafe(this.workDir, `Pulumi${ext}`);
            if (fs.existsSync(testPath)) {
                foundExt = ext;
                break;
            }
        }
        const path = upath.joinSafe(this.workDir, `Pulumi${foundExt}`);
        let contents;
        if (foundExt === ".json") {
            contents = JSON.stringify(settings, null, 4);
        }
        else {
            contents = yaml.safeDump(settings, { skipInvalid: true });
        }
        return fs.writeFileSync(path, contents);
    }
    /**
     * Returns the settings object for the stack matching the specified stack name if any.
     * LocalWorkspace reads this from a Pulumi.<stack>.yaml file in Workspace.WorkDir().
     *
     * @param stackName The stack to retrieve settings from.
     */
    async stackSettings(stackName: string): Promise<StackSettings> {
        const stackSettingsName = getStackSettingsName(stackName);
        for (const ext of settingsExtensions) {
            const isJSON = ext === ".json";
            const path = upath.joinSafe(this.workDir, `Pulumi.${stackSettingsName}${ext}`);
            if (!fs.existsSync(path)) { continue; }
            const contents = fs.readFileSync(path).toString();
            let stackSettings: any;
            if (isJSON) {
                stackSettings = JSON.parse(contents);
            }
            stackSettings = yaml.safeLoad(contents) as StackSettings;

            // Transform the serialized representation back to what we expect.
            for (const key of stackSettingsSerDeKeys) {
                if (stackSettings.hasOwnProperty(key[0])) {
                    stackSettings[key[1]] = stackSettings[key[0]];
                    delete stackSettings[key[0]];
                }
            }
            return stackSettings as StackSettings;
        }
        throw new Error(`failed to find stack settings file in workdir: ${this.workDir}`);
    }
    /**
     * Overwrites the settings object for the stack matching the specified stack name.
     * LocalWorkspace writes this value to a Pulumi.<stack>.yaml file in Workspace.WorkDir()
     *
     * @param stackName The stack to operate on.
     * @param settings The settings object to save.
     */
    async saveStackSettings(stackName: string, settings: StackSettings): Promise<void> {
        const stackSettingsName = getStackSettingsName(stackName);
        let foundExt = ".yaml";
        for (const ext of settingsExtensions) {
            const testPath = upath.joinSafe(this.workDir, `Pulumi.${stackSettingsName}${ext}`);
            if (fs.existsSync(testPath)) {
                foundExt = ext;
                break;
            }
        }
        const path = upath.joinSafe(this.workDir, `Pulumi.${stackSettingsName}${foundExt}`);
        const serializeSettings = settings as any;
        let contents;

        // Transform the keys to the serialized representation that we expect.
        for (const key of stackSettingsSerDeKeys) {
            if (serializeSettings.hasOwnProperty(key[1])) {
                serializeSettings[key[0]] = serializeSettings[key[1]];
                delete serializeSettings[key[1]];
            }
        }

        if (foundExt === ".json") {
            contents = JSON.stringify(serializeSettings, null, 4);
        }
        else {
            contents = yaml.safeDump(serializeSettings, { skipInvalid: true });
        }
        return fs.writeFileSync(path, contents);
    }
    /**
     * Creates and sets a new stack with the stack name, failing if one already exists.
     *
     * @param stackName The stack to create.
     */
    async createStack(stackName: string): Promise<void> {
        const args = ["stack", "init", stackName];
        if (this.secretsProvider) {
            args.push("--secrets-provider", this.secretsProvider);
        }
        await this.runPulumiCmd(args);
    }
    /**
     * Selects and sets an existing stack matching the stack name, failing if none exists.
     *
     * @param stackName The stack to select.
     */
    async selectStack(stackName: string): Promise<void> {
        await this.runPulumiCmd(["stack", "select", stackName]);
    }
    /**
     * Deletes the stack and all associated configuration and history.
     *
     * @param stackName The stack to remove
     */
    async removeStack(stackName: string): Promise<void> {
        await this.runPulumiCmd(["stack", "rm", "--yes", stackName]);
    }
    /**
     * Returns the value associated with the specified stack name and key,
     * scoped to the current workspace. LocalWorkspace reads this config from the matching Pulumi.stack.yaml file.
     *
     * @param stackName The stack to read config from
     * @param key The key to use for the config lookup
     */
    async getConfig(stackName: string, key: string): Promise<ConfigValue> {
        const result = await this.runPulumiCmd(["config", "get", key, "--json", "--stack", stackName]);
        return JSON.parse(result.stdout);
    }
    /**
     * Returns the config map for the specified stack name, scoped to the current workspace.
     * LocalWorkspace reads this config from the matching Pulumi.stack.yaml file.
     *
     * @param stackName The stack to read config from
     */
    async getAllConfig(stackName: string): Promise<ConfigMap> {
        const result = await this.runPulumiCmd(["config", "--show-secrets", "--json", "--stack", stackName]);
        return JSON.parse(result.stdout);
    }
    /**
     * Sets the specified key-value pair on the provided stack name.
     * LocalWorkspace writes this value to the matching Pulumi.<stack>.yaml file in Workspace.WorkDir().
     *
     * @param stackName The stack to operate on
     * @param key The config key to set
     * @param value The value to set
     */
    async setConfig(stackName: string, key: string, value: ConfigValue): Promise<void> {
        const secretArg = value.secret ? "--secret" : "--plaintext";
        await this.runPulumiCmd(["config", "set", key, value.value, secretArg, "--stack", stackName]);
    }
    /**
     * Sets all values in the provided config map for the specified stack name.
     * LocalWorkspace writes the config to the matching Pulumi.<stack>.yaml file in Workspace.WorkDir().
     *
     * @param stackName The stack to operate on
     * @param config The `ConfigMap` to upsert against the existing config.
     */
    async setAllConfig(stackName: string, config: ConfigMap): Promise<void> {
        let args = ["config", "set-all", "--stack", stackName];
        for (const [key, value] of Object.entries(config)) {
            const secretArg = value.secret ? "--secret" : "--plaintext";
            args = [...args, secretArg, `${key}=${value.value}`];
        }

        await this.runPulumiCmd(args);
    }
    /**
     * Removes the specified key-value pair on the provided stack name.
     * It will remove any matching values in the Pulumi.<stack>.yaml file in Workspace.WorkDir().
     *
     * @param stackName The stack to operate on
     * @param key The config key to remove
     */
    async removeConfig(stackName: string, key: string): Promise<void> {
        await this.runPulumiCmd(["config", "rm", key, "--stack", stackName]);
    }
    /**
     *
     * Removes all values in the provided key list for the specified stack name
     * It will remove any matching values in the Pulumi.<stack>.yaml file in Workspace.WorkDir().
     *
     * @param stackName The stack to operate on
     * @param keys The list of keys to remove from the underlying config
     */
    async removeAllConfig(stackName: string, keys: string[]): Promise<void> {
        await this.runPulumiCmd(["config", "rm-all", "--stack", stackName, ...keys]);
    }
    /**
     * Gets and sets the config map used with the last update for Stack matching stack name.
     * It will overwrite all configuration in the Pulumi.<stack>.yaml file in Workspace.WorkDir().
     *
     * @param stackName The stack to refresh
     */
    async refreshConfig(stackName: string): Promise<ConfigMap> {
        await this.runPulumiCmd(["config", "refresh", "--force", "--stack", stackName]);
        return this.getAllConfig(stackName);
    }
    /**
     * Returns the currently authenticated user.
     */
    async whoAmI(): Promise<WhoAmIResult> {
        const result = await this.runPulumiCmd(["whoami"]);
        return { user: result.stdout.trim() };
    }
    /**
     * Returns a summary of the currently selected stack, if any.
     */
    async stack(): Promise<StackSummary | undefined> {
        const stacks = await this.listStacks();
        for (const stack of stacks) {
            if (stack.current) {
                return stack;
            }
        }
        return undefined;
    }
    /**
     * Returns all Stacks created under the current Project.
     * This queries underlying backend and may return stacks not present in the Workspace (as Pulumi.<stack>.yaml files).
     */
    async listStacks(): Promise<StackSummary[]> {
        const result = await this.runPulumiCmd(["stack", "ls", "--json"]);
        return JSON.parse(result.stdout);
    }
    /**
     * Installs a plugin in the Workspace, for example to use cloud providers like AWS or GCP.
     *
     * @param name the name of the plugin.
     * @param version the version of the plugin e.g. "v1.0.0".
     * @param kind the kind of plugin, defaults to "resource"
     */
    async installPlugin(name: string, version: string, kind = "resource"): Promise<void> {
        await this.runPulumiCmd(["plugin", "install", kind, name, version]);
    }
    /**
     * Removes a plugin from the Workspace matching the specified name and version.
     *
     * @param name the optional name of the plugin.
     * @param versionRange optional semver range to check when removing plugins matching the given name
     *  e.g. "1.0.0", ">1.0.0".
     * @param kind he kind of plugin, defaults to "resource".
     */
    async removePlugin(name?: string, versionRange?: string, kind = "resource"): Promise<void> {
        const args = ["plugin", "rm", kind];
        if (name) {
            args.push(name);
        }
        if (versionRange) {
            args.push(versionRange);
        }
        args.push("--yes");
        await this.runPulumiCmd(args);
    }
    /**
     * Returns a list of all plugins installed in the Workspace.
     */
    async listPlugins(): Promise<PluginInfo[]> {
        const result = await this.runPulumiCmd(["plugin", "ls", "--json"]);
        return JSON.parse(result.stdout, (key, value) => {
            if (key === "installTime" || key === "lastUsedTime") {
                return new Date(value);
            }
            return value;
        });
    }
    /**
     * exportStack exports the deployment state of the stack.
     * This can be combined with Workspace.importStack to edit a stack's state (such as recovery from failed deployments).
     *
     * @param stackName the name of the stack.
     */
    async exportStack(stackName: string): Promise<Deployment> {
        const result = await this.runPulumiCmd(["stack", "export", "--show-secrets", "--stack", stackName]);
        return JSON.parse(result.stdout);
    }
    /**
     * importStack imports the specified deployment state into a pre-existing stack.
     * This can be combined with Workspace.exportStack to edit a stack's state (such as recovery from failed deployments).
     *
     * @param stackName the name of the stack.
     * @param state the stack state to import.
     */
    async importStack(stackName: string, state: Deployment): Promise<void> {
        const randomSuffix = Math.floor(100000 + Math.random() * 900000);
        const filepath = upath.joinSafe(os.tmpdir(), `automation-${randomSuffix}`);
        const contents = JSON.stringify(state, null, 4);
        fs.writeFileSync(filepath, contents);
        await this.runPulumiCmd(["stack", "import", "--file", filepath, "--stack", stackName]);
        fs.unlinkSync(filepath);
    }
    /**
     * Gets the current set of Stack outputs from the last Stack.up().
     * @param stackName the name of the stack.
     */
    async stackOutputs(stackName: string): Promise<OutputMap> {
        // TODO: do this in parallel after this is fixed https://github.com/pulumi/pulumi/issues/6050
        const maskedResult = await this.runPulumiCmd(["stack", "output", "--json", "--stack", stackName]);
        const plaintextResult = await this.runPulumiCmd(["stack", "output", "--json", "--show-secrets", "--stack", stackName]);
        const maskedOuts = JSON.parse(maskedResult.stdout);
        const plaintextOuts = JSON.parse(plaintextResult.stdout);
        const outputs: OutputMap = {};

        for (const [key, value] of Object.entries(plaintextOuts)) {
            const secret = maskedOuts[key] === "[secret]";
            outputs[key] = { value, secret };
        }

        return outputs;
    }

    /**
     * serializeArgsForOp is hook to provide additional args to every CLI commands before they are executed.
     * Provided with stack name,
     * returns a list of args to append to an invoked command ["--config=...", ]
     * LocalWorkspace does not utilize this extensibility point.
     */
    async serializeArgsForOp(_: string): Promise<string[]> {
        // LocalWorkspace does not utilize this extensibility point.
        return [];
    }
    /**
     * postCommandCallback is a hook executed after every command. Called with the stack name.
     * An extensibility point to perform workspace cleanup (CLI operations may create/modify a Pulumi.stack.yaml)
     * LocalWorkspace does not utilize this extensibility point.
     */
    async postCommandCallback(_: string): Promise<void> {
        // LocalWorkspace does not utilize this extensibility point.
        return;
    }
    private async getPulumiVersion(minVersion: semver.SemVer) {
        const result = await this.runPulumiCmd(["version"]);
        const version = new semver.SemVer(result.stdout.trim());
        const optOut = !!this.envVars[SKIP_VERSION_CHECK_VAR] || !!process.env[SKIP_VERSION_CHECK_VAR];
        validatePulumiVersion(minVersion, version, optOut);
        this._pulumiVersion = version;
    }
    private async runPulumiCmd(
        args: string[],
    ): Promise<CommandResult> {
        let envs: { [key: string]: string } = {};
        if (this.pulumiHome) {
            envs["PULUMI_HOME"] = this.pulumiHome;
        }
        envs = { ...envs, ...this.envVars };
        return runPulumiCmd(args, this.workDir, envs);
    }
}

/**
 * Description of a stack backed by an inline (in process) Pulumi program.
 */
export interface InlineProgramArgs {
    /**
     * The name of the associated Stack
     */
    stackName: string;
    /**
     * The name of the associated project
     */
    projectName: string;
    /**
     * The inline (in process) Pulumi program to use with Update and Preview operations.
     */
    program: PulumiFn;
}

/**
 * Description of a stack backed by pre-existing local Pulumi CLI program.
 */
export interface LocalProgramArgs {
    stackName: string;
    workDir: string;
}

/**
 * Extensibility options to configure a LocalWorkspace; e.g: settings to seed
 * and environment variables to pass through to every command.
 */
export interface LocalWorkspaceOptions {
    /**
     * The directory to run Pulumi commands and read settings (Pulumi.yaml and Pulumi.<stack>.yaml)l.
     */
    workDir?: string;
    /**
     * The directory to override for CLI metadata
     */
    pulumiHome?: string;
    /**
     *  The inline program `PulumiFn` to be used for Preview/Update operations if any.
     *  If none is specified, the stack will refer to ProjectSettings for this information.
     */
    program?: PulumiFn;
    /**
     * Environment values scoped to the current workspace. These will be supplied to every Pulumi command.
     */
    envVars?: { [key: string]: string };
    /**
     * The secrets provider to use for encryption and decryption of stack secrets.
     * See: https://www.pulumi.com/docs/intro/concepts/config/#available-encryption-providers
     */
    secretsProvider?: string;
    /**
     * The settings object for the current project.
     */
    projectSettings?: ProjectSettings;
    /**
     * A map of Stack names and corresponding settings objects.
     */
    stackSettings?: { [key: string]: StackSettings };
}

/**
 * Returns true if the provided `args` object satisfies the `LocalProgramArgs` interface.
 *
 * @param args The args object to evaluate
 */
function isLocalProgramArgs(args: LocalProgramArgs | InlineProgramArgs): args is LocalProgramArgs {
    return (args as LocalProgramArgs).workDir !== undefined;
}

/**
 * Returns true if the provided `args` object satisfies the `InlineProgramArgs` interface.
 *
 * @param args The args object to evaluate
 */
function isInlineProgramArgs(args: LocalProgramArgs | InlineProgramArgs): args is InlineProgramArgs {
    return (args as InlineProgramArgs).projectName !== undefined &&
        (args as InlineProgramArgs).program !== undefined;
}

const settingsExtensions = [".yaml", ".yml", ".json"];

function getStackSettingsName(name: string): string {
    const parts = name.split("/");
    if (parts.length < 1) {
        return name;
    }
    return parts[parts.length - 1];
}

type StackInitializer = (name: string, workspace: Workspace) => Promise<Stack>;

function defaultProject(projectName: string) {
    const settings: ProjectSettings = { name: projectName, runtime: "nodejs", main: process.cwd() };
    return settings;
}

function loadProjectSettings(workDir: string) {
    for (const ext of settingsExtensions) {
        const isJSON = ext === ".json";
        const path = upath.joinSafe(workDir, `Pulumi${ext}`);
        if (!fs.existsSync(path)) { continue; }
        const contents = fs.readFileSync(path).toString();
        if (isJSON) {
            return JSON.parse(contents);
        }
        return yaml.safeLoad(contents) as ProjectSettings;
    }
    throw new Error(`failed to find project settings file in workdir: ${workDir}`);
}

/** @internal */
export function validatePulumiVersion(minVersion: semver.SemVer, currentVersion: semver.SemVer, optOut: boolean) {
    if (optOut) {
        return;
    }
    if (minVersion.major < currentVersion.major) {
        throw new Error(`Major version mismatch. You are using Pulumi CLI version ${currentVersion.toString()} with Automation SDK v${minVersion.major}. Please update the SDK.`);
    }
    if (minVersion.compare(currentVersion) === 1) {
        throw new Error(`Minimum version requirement failed. The minimum CLI version requirement is ${minVersion.toString()}, your current CLI version is ${currentVersion.toString()}. Please update the Pulumi CLI.`);
    }
}
