// Copyright 2016-2025, Pulumi Corporation.
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

import { CommandResult, PulumiCommand } from "./cmd";
import { ConfigMap, ConfigValue } from "./config";
import { ProjectSettings } from "./projectSettings";
import { ExecutorImage, RemoteGitProgramArgs } from "./remoteWorkspace";
import { OutputMap, Stack } from "./stack";
import { StackSettings, stackSettingsSerDeKeys } from "./stackSettings";
import { TagMap } from "./tag";
import { Deployment, PluginInfo, PulumiFn, StackSummary, WhoAmIResult, Workspace } from "./workspace";

const SKIP_VERSION_CHECK_VAR = "PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK";

/**
 * {@link LocalWorkspace} is a default implementation of the {@link Workspace} interface.
 *
 * A {@link Workspace} is the execution context containing a single Pulumi
 * project, a program, and multiple stacks. Workspaces are used to manage the
 * execution environment, providing various utilities such as plugin
 * installation, environment configuration (`$PULUMI_HOME`), and creation,
 * deletion, and listing of Stacks.
 *
 * {@link LocalWorkspace} relies on `Pulumi.yaml` and `Pulumi.<stack>.yaml` as
 * the intermediate format for Project and Stack settings. Modifying the
 * workspace's {@link ProjectSettings} will alter the workspace's `Pulumi.yaml`
 * file, and setting config on a Stack will modify the relevant
 * `Pulumi.<stack>.yaml` file. This is identical to the behavior of Pulumi CLI
 * driven workspaces.
 *
 * @alpha
 */
export class LocalWorkspace implements Workspace {
    /**
     * The working directory to run Pulumi CLI commands in.
     */
    readonly workDir: string;

    /**
     * The directory override for CLI metadata if set. This customizes the
     * location of `$PULUMI_HOME` where metadata is stored and plugins are
     * installed.
     */
    readonly pulumiHome?: string;

    /**
     * The secrets provider to use for encryption and decryption of stack secrets.
     * See: https://www.pulumi.com/docs/intro/concepts/secrets/#available-encryption-providers
     */
    readonly secretsProvider?: string;

    private _pulumiCommand?: PulumiCommand;

    /**
     * The underlying Pulumi CLI.
     */
    public get pulumiCommand(): PulumiCommand {
        if (this._pulumiCommand === undefined) {
            throw new Error("Failed to get Pulumi CLI");
        }
        return this._pulumiCommand;
    }

    /**
     * The image to use for the remote Pulumi operation.
     */
    remoteExecutorImage?: ExecutorImage;

    /**
     * The inline program {@link PulumiFn} to be used for preview/update
     * operations if any. If none is specified, the stack will refer to
     * {@link ProjectSettings} for this information.
     */
    program?: PulumiFn;

    /**
     * Environment values scoped to the current workspace. These will be supplied to every Pulumi command.
     */
    envVars: { [key: string]: string };

    private _pulumiVersion?: semver.SemVer;

    /**
     * The version of the underlying Pulumi CLI/engine.
     *
     * @returns A string representation of the version, if available. `null` otherwise.
     */
    public get pulumiVersion(): string {
        if (this._pulumiVersion === undefined) {
            throw new Error(`Failed to get Pulumi version`);
        }
        return this._pulumiVersion.toString();
    }

    private ready: Promise<any[]>;

    /**
     * True if the workspace is a remote workspace.
     */
    private remote?: boolean;

    /**
     * Remote Git source info.
     */
    private remoteGitProgramArgs?: RemoteGitProgramArgs;

    /**
     * An optional list of arbitrary commands to run before the remote Pulumi operation is invoked.
     */
    private remotePreRunCommands?: string[];

    /**
     * The environment variables to pass along when running remote Pulumi operations.
     */
    private remoteEnvVars?: { [key: string]: string | { secret: string } };

    /**
     * Whether to skip the default dependency installation step.
     */
    private remoteSkipInstallDependencies?: boolean;

    /**
     * Whether to inherit the deployment settings set on the stack.
     */
    private remoteInheritSettings?: boolean;

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
     * Creates a {@link Stack} with a {@link LocalWorkspace} utilizing the local
     * Pulumi CLI program from the specified working directory. This is a way to
     * create drivers on top of pre-existing Pulumi programs. This workspace
     * will pick up any available settings files (`Pulumi.yaml`,
     * `Pulumi.<stack>.yaml`).
     *
     * @param args
     *  A set of arguments to initialize a stack with a pre-configured Pulumi
     *  CLI program that already exists on disk.
     *
     * @param opts
     *  Additional customizations to be applied to the Workspace.
     */
    static async createStack(args: LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;

    /**
     * Creates a {@link Stack} with a {@link LocalWorkspace} utilizing the
     * specified inline (in process) Pulumi program. This program is fully
     * debuggable and runs in process. If no Project option is specified,
     * default project settings will be created on behalf of the user.
     * Similarly, unless a `workDir` option is specified, the working directory
     * will default to a new temporary directory provided by the OS.
     *
     * @param args
     *  A set of arguments to initialize a stack with and an inline
     *  {@link PulumiFn} program that runs in process.
     * @param opts
     *  Additional customizations to be applied to the Workspace.
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
     * Selects a {@link Stack} with a {@link LocalWorkspace} utilizing the local
     * Pulumi CLI program from the specified working directory. This is a way to
     * create drivers on top of pre-existing Pulumi programs. This Workspace
     * will pick up any available Settings files (`Pulumi.yaml`,
     * `Pulumi.<stack>.yaml`).
     *
     * @param args
     *  A set of arguments to initialize a stack with a pre-configured Pulumi
     *  CLI program that already exists on disk.
     * @param opts
     *  Additional customizations to be applied to the Workspace.
     */
    static async selectStack(args: LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;

    /**
     * Selects an existing {@link Stack} with a {@link LocalWorkspace} utilizing
     * the specified inline (in process) Pulumi program. This program is fully
     * debuggable and runs in process. If no Project option is specified,
     * default project settings will be created on behalf of the user.
     * Similarly, unless a `workDir` option is specified, the working directory
     * will default to a new temporary directory provided by the OS.
     *
     * @param args
     *  A set of arguments to initialize a Stack with and inline `PulumiFn`
     *  program that runs in process.
     * @param opts
     *  Additional customizations to be applied to the Workspace.
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
     * Creates or selects an existing {@link Stack} with a {@link LocalWorkspace}
     * utilizing the specified inline (in process) Pulumi CLI program. This
     * program is fully debuggable and runs in process. If no project is
     * specified, default project settings will be created on behalf of the
     * user. Similarly, unless a `workDir` option is specified, the working
     * directory will default to a new temporary directory provided by the OS.
     *
     * @param args
     *  A set of arguments to initialize a Stack with a pre-configured Pulumi
     *  CLI program that already exists on disk.
     * @param opts
     *  Additional customizations to be applied to the Workspace.
     */
    static async createOrSelectStack(args: LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;

    /**
     * Creates or selects an existing {@link Stack} with a {@link
     * LocalWorkspace} utilizing the specified inline Pulumi CLI program. This
     * program is fully debuggable and runs in process. If no Project option is
     * specified, default project settings will be created on behalf of the
     * user. Similarly, unless a `workDir` option is specified, the working
     * directory will default to a new temporary directory provided by the OS.
     *
     * @param args
     *  A set of arguments to initialize a Stack with and inline `PulumiFn`
     *  program that runs in process.
     * @param opts
     *  Additional customizations to be applied to the Workspace.
     */
    static async createOrSelectStack(args: InlineProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    static async createOrSelectStack(
        args: InlineProgramArgs | LocalProgramArgs,
        opts?: LocalWorkspaceOptions,
    ): Promise<Stack> {
        if (isInlineProgramArgs(args)) {
            return await this.inlineSourceStackHelper(args, Stack.createOrSelect, opts);
        } else if (isLocalProgramArgs(args)) {
            return await this.localSourceStackHelper(args, Stack.createOrSelect, opts);
        }
        throw new Error(`unexpected args: ${args}`);
    }

    private static async localSourceStackHelper(
        args: LocalProgramArgs,
        initFn: StackInitializer,
        opts?: LocalWorkspaceOptions,
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
        args: InlineProgramArgs,
        initFn: StackInitializer,
        opts?: LocalWorkspaceOptions,
    ): Promise<Stack> {
        let wsOpts: LocalWorkspaceOptions = { program: args.program };
        if (opts) {
            wsOpts = { ...opts, program: args.program };
        }

        if (!wsOpts.projectSettings) {
            if (wsOpts.workDir) {
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

    /**
     * Constructs a new {@link LocalWorkspace}.
     */
    private constructor(opts?: LocalWorkspaceOptions) {
        let dir = "";
        let envs = {};

        if (opts) {
            const {
                workDir,
                pulumiHome,
                program,
                remoteExecutorImage,
                envVars,
                secretsProvider,
                remote,
                remoteGitProgramArgs,
                remotePreRunCommands,
                remoteEnvVars,
                remoteSkipInstallDependencies,
                remoteInheritSettings,
            } = opts;
            if (workDir) {
                // Verify that the workdir exists.
                if (!fs.existsSync(workDir)) {
                    throw new Error(`Invalid workDir passed to local workspace: '${workDir}' does not exist`);
                }
                dir = workDir;
            }
            this.pulumiHome = pulumiHome;
            this.remoteExecutorImage = remoteExecutorImage;
            this.program = program;
            this.secretsProvider = secretsProvider;
            this.remote = remote;
            this.remoteGitProgramArgs = remoteGitProgramArgs;
            this.remotePreRunCommands = remotePreRunCommands;
            this.remoteEnvVars = { ...remoteEnvVars };
            this.remoteSkipInstallDependencies = remoteSkipInstallDependencies;
            this.remoteInheritSettings = remoteInheritSettings;
            envs = { ...envVars };
        }

        if (!dir) {
            dir = fs.mkdtempSync(upath.joinSafe(os.tmpdir(), "automation-"));
        }
        this.workDir = dir;
        this.envVars = envs;

        const skipVersionCheck = !!this.envVars[SKIP_VERSION_CHECK_VAR] || !!process.env[SKIP_VERSION_CHECK_VAR];
        const pulumiCommand = opts?.pulumiCommand
            ? Promise.resolve(opts.pulumiCommand)
            : PulumiCommand.get({ skipVersionCheck });

        const readinessPromises: Promise<any>[] = [
            pulumiCommand.then((p) => {
                this._pulumiCommand = p;
                if (p.version) {
                    this._pulumiVersion = p.version;
                }
                return this.checkRemoteSupport();
            }),
        ];

        if (opts?.projectSettings) {
            readinessPromises.push(this.saveProjectSettings(opts.projectSettings));
        }
        if (opts?.stackSettings) {
            for (const [name, value] of Object.entries(opts.stackSettings)) {
                readinessPromises.push(this.saveStackSettings(name, value));
            }
        }

        this.ready = Promise.all(readinessPromises);
    }

    /**
     * Returns the settings object for the current project if any
     * {@link LocalWorkspace} reads settings from the `Pulumi.yaml`
     * in the workspace. A workspace can contain only a single project at a
     * time.
     */
    async projectSettings(): Promise<ProjectSettings> {
        return loadProjectSettings(this.workDir);
    }

    /**
     * Overwrites the settings object in the current project. There can only be
     * a single project per workspace. Fails if new project name does not match
     * old. {@link LocalWorkspace} writes this value to a `Pulumi.yaml` file in
     * `Workspace.WorkDir()`.
     *
     * @param settings
     *  The settings object to save to the Workspace.
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
        } else {
            contents = yaml.safeDump(settings, { skipInvalid: true });
        }
        return fs.writeFileSync(path, contents);
    }

    /**
     * Returns the settings object for the stack matching the specified stack
     * name if any. {@link LocalWorkspace} reads this from a
     * `Pulumi.<stack>.yaml` file in `Workspace.WorkDir()`.
     *
     * @param stackName
     *  The stack to retrieve settings from.
     */
    async stackSettings(stackName: string): Promise<StackSettings> {
        const stackSettingsName = getStackSettingsName(stackName);
        for (const ext of settingsExtensions) {
            const isJSON = ext === ".json";
            const path = upath.joinSafe(this.workDir, `Pulumi.${stackSettingsName}${ext}`);
            if (!fs.existsSync(path)) {
                continue;
            }
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
     * Overwrites the settings object for the stack matching the specified stack
     * name. {@link LocalWorkspace} writes this value to a `Pulumi.<stack>.yaml`
     * file in `Workspace.WorkDir()`
     *
     * @param stackName
     *  The stack to operate on.
     * @param settings
     *  The settings object to save.
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
        const serializeSettings = { ...settings } as any;
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
        } else {
            contents = yaml.safeDump(serializeSettings, { skipInvalid: true });
        }
        return fs.writeFileSync(path, contents);
    }

    /**
     * Creates and sets a new stack with the stack name, failing if one already
     * exists.
     *
     * @param stackName The stack to create.
     */
    async createStack(stackName: string): Promise<void> {
        const args = ["stack", "init", stackName];
        if (this.secretsProvider) {
            args.push("--secrets-provider", this.secretsProvider);
        }
        if (this.isRemote) {
            args.push("--no-select");
        }
        await this.runPulumiCmd(args);
    }

    /**
     * Selects and sets an existing stack matching the stack name, failing if
     * none exists.
     *
     * @param stackName The stack to select.
     */
    async selectStack(stackName: string): Promise<void> {
        // If this is a remote workspace, we don't want to actually select the stack (which would modify global state);
        // but we will ensure the stack exists by calling `pulumi stack`.
        const args = ["stack"];
        if (!this.isRemote) {
            args.push("select");
        }
        args.push("--stack", stackName);

        await this.runPulumiCmd(args);
    }

    /**
     * Deletes the stack and all associated configuration and history.
     *
     * @param stackName The stack to remove
     */
    async removeStack(stackName: string, opts?: RemoveOptions): Promise<void> {
        const args = ["stack", "rm", "--yes"];

        if (opts?.force) {
            args.push("--force");
        }

        if (opts?.preserveConfig) {
            args.push("--preserve-config");
        }

        if (opts?.removeBackups) {
            const ver = this._pulumiVersion ?? semver.parse("3.0.0")!;
            if (ver.compare("3.188.0") < 0) {
                // Pulumi 3.188.0 introduced the `--remove-backups` flag.
                // https://github.com/pulumi/pulumi/releases/tag/v3.188.0
                throw new Error(`removeBackups requires Pulumi version >= 3.188.0`);
            }
            args.push("--remove-backups");
        }

        args.push(stackName);

        await this.runPulumiCmd(args);
    }

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
    async addEnvironments(stackName: string, ...environments: string[]): Promise<void> {
        let ver = this._pulumiVersion;
        if (ver === undefined) {
            // Assume an old version. Doesn't really matter what this is as long as it's pre-3.95.
            ver = semver.parse("3.0.0")!;
        }

        // 3.95 added this command (https://github.com/pulumi/pulumi/releases/tag/v3.95.0)
        if (ver.compare("3.95.0") < 0) {
            throw new Error(`addEnvironments requires Pulumi version >= 3.95.0`);
        }

        await this.runPulumiCmd(["config", "env", "add", ...environments, "--stack", stackName, "--yes"]);
    }

    /**
     * Returns the list of environments associated with the specified stack name.
     *
     * @param stackName The stack to operate on
     */
    async listEnvironments(stackName: string): Promise<string[]> {
        let ver = this._pulumiVersion;
        if (ver === undefined) {
            // Assume an old version. Doesn't really matter what this is as long as it's pre-3.99.
            ver = semver.parse("3.0.0")!;
        }

        // 3.99 added this command (https://github.com/pulumi/pulumi/releases/tag/v3.99.0)
        if (ver.compare("3.99.0") < 0) {
            throw new Error(`listEnvironments requires Pulumi version >= 3.99.0`);
        }

        const result = await this.runPulumiCmd(["config", "env", "ls", "--stack", stackName, "--json"]);
        return JSON.parse(result.stdout);
    }

    /**
     * Removes an environment from a stack's import list.
     *
     * @param stackName
     *  The stack to operate on
     * @param environment
     *  The name of the environment to remove from the stack's configuration
     */
    async removeEnvironment(stackName: string, environment: string): Promise<void> {
        let ver = this._pulumiVersion;
        if (ver === undefined) {
            // Assume an old version. Doesn't really matter what this is as long as it's pre-3.95.
            ver = semver.parse("3.0.0")!;
        }

        // 3.95 added this command (https://github.com/pulumi/pulumi/releases/tag/v3.95.0)
        if (ver.compare("3.95.0") < 0) {
            throw new Error(`removeEnvironments requires Pulumi version >= 3.95.0`);
        }

        await this.runPulumiCmd(["config", "env", "rm", environment, "--stack", stackName, "--yes"]);
    }

    /**
     * Returns the value associated with the specified stack name and key,
     * scoped to the current workspace. {@link LocalWorkspace} reads this config
     * from the matching `Pulumi.stack.yaml` file.
     *
     * @param stackName
     *  The stack to read config from
     * @param key
     *  The key to use for the config lookup
     * @param path
     *  The key contains a path to a property in a map or list to get
     */
    async getConfig(stackName: string, key: string, path?: boolean): Promise<ConfigValue> {
        const args = ["config", "get"];
        if (path) {
            args.push("--path");
        }
        args.push(key, "--json", "--stack", stackName);
        const result = await this.runPulumiCmd(args);
        return JSON.parse(result.stdout);
    }

    /**
     * Returns the config map for the specified stack name, scoped to the
     * current workspace. {@link LocalWorkspace} reads this config from the
     * matching `Pulumi.stack.yaml` file.
     *
     * @param stackName
     *  The stack to read config from
     */
    async getAllConfig(stackName: string): Promise<ConfigMap> {
        const result = await this.runPulumiCmd(["config", "--show-secrets", "--json", "--stack", stackName]);
        return JSON.parse(result.stdout);
    }

    /**
     * Sets the specified key-value pair on the provided stack name. {@link
     * LocalWorkspace} writes this value to the matching `Pulumi.<stack>.yaml`
     * file in `Workspace.WorkDir()`.
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
    async setConfig(stackName: string, key: string, value: ConfigValue, path?: boolean): Promise<void> {
        const args = ["config", "set"];
        if (path) {
            args.push("--path");
        }
        const secretArg = value.secret ? "--secret" : "--plaintext";
        args.push(key, "--stack", stackName, secretArg, "--non-interactive", "--", value.value);
        await this.runPulumiCmd(args);
    }

    /**
     * Sets all values in the provided config map for the specified stack name.
     * {@link LocalWorkspace} writes the config to the matching
     * `Pulumi.<stack>.yaml` file in `Workspace.WorkDir()`.
     *
     * @param stackName
     *  The stack to operate on
     * @param config
     *  The {@link ConfigMap} to upsert against the existing config
     * @param path
     *  The keys contain a path to a property in a map or list to set
     */
    async setAllConfig(stackName: string, config: ConfigMap, path?: boolean): Promise<void> {
        const args = ["config", "set-all", "--stack", stackName];
        if (path) {
            args.push("--path");
        }
        for (const [key, value] of Object.entries(config)) {
            const secretArg = value.secret ? "--secret" : "--plaintext";
            args.push(secretArg, `${key}=${value.value}`);
        }

        await this.runPulumiCmd(args);
    }

    /**
     * Removes the specified key-value pair on the provided stack name. Will
     * remove any matching values in the `Pulumi.<stack>.yaml` file in
     * `Workspace.WorkDir()`.
     *
     * @param stackName
     *  The stack to operate on
     * @param key
     *  The config key to remove
     * @param path
     *  The key contains a path to a property in a map or list to remove
     */
    async removeConfig(stackName: string, key: string, path?: boolean): Promise<void> {
        const args = ["config", "rm", key, "--stack", stackName];
        if (path) {
            args.push("--path");
        }
        await this.runPulumiCmd(args);
    }

    /**
     * Removes all values in the provided key list for the specified stack name
     * Will remove any matching values in the `Pulumi.<stack>.yaml` file in
     * `Workspace.WorkDir()`.
     *
     * @param stackName
     *  The stack to operate on
     * @param keys
     *  The list of keys to remove from the underlying config
     * @param path
     *  The keys contain a path to a property in a map or list to remove
     */
    async removeAllConfig(stackName: string, keys: string[], path?: boolean): Promise<void> {
        const args = ["config", "rm-all", "--stack", stackName];
        if (path) {
            args.push("--path");
        }
        args.push(...keys);
        await this.runPulumiCmd(args);
    }

    /**
     * Gets and sets the config map used with the last update for the stack
     * matching the given name. This will overwrite all configuration in the
     * `Pulumi.<stack>.yaml` file in `Workspace.WorkDir()`.
     *
     * @param stackName
     *  The stack to refresh
     */
    async refreshConfig(stackName: string): Promise<ConfigMap> {
        await this.runPulumiCmd(["config", "refresh", "--force", "--stack", stackName]);
        return this.getAllConfig(stackName);
    }

    /**
     * Returns the value associated with the specified stack name and key,
     * scoped to the {@link LocalWorkspace.}
     *
     * @param stackName
     *  The stack to read tag metadata from.
     * @param key
     *  The key to use for the tag lookup.
     */
    async getTag(stackName: string, key: string): Promise<string> {
        const result = await this.runPulumiCmd(["stack", "tag", "get", key, "--stack", stackName]);
        return result.stdout.trim();
    }

    /**
     * Sets the specified key-value pair on the stack with the given name.
     *
     * @param stackName
     *  The stack to operate on.
     * @param key
     *  The tag key to set.
     * @param value
     *  The tag value to set.
     */
    async setTag(stackName: string, key: string, value: string): Promise<void> {
        await this.runPulumiCmd(["stack", "tag", "set", key, value, "--stack", stackName]);
    }

    /**
     * Removes the specified key-value pair on the stack with the given name.
     *
     * @param stackName
     *  The stack to operate on.
     * @param key
     *  The tag key to remove.
     */
    async removeTag(stackName: string, key: string): Promise<void> {
        await this.runPulumiCmd(["stack", "tag", "rm", key, "--stack", stackName]);
    }

    /**
     * Returns the tag map for the specified stack, scoped to the current
     * {@link LocalWorkspace.}
     *
     * @param stackName
     *  The stack to read tag metadata from.
     */
    async listTags(stackName: string): Promise<TagMap> {
        const result = await this.runPulumiCmd(["stack", "tag", "ls", "--json", "--stack", stackName]);
        return JSON.parse(result.stdout);
    }

    /**
     * Returns information about the currently authenticated user.
     */
    async whoAmI(): Promise<WhoAmIResult> {
        let ver = this._pulumiVersion;
        if (ver === undefined) {
            // Assume an old version. Doesn't really matter what this is as long as it's pre-3.58.
            ver = semver.parse("3.0.0")!;
        }

        // 3.58 added the --json flag (https://github.com/pulumi/pulumi/releases/tag/v3.58.0)
        if (ver.compare("3.58.0") >= 0) {
            const result = await this.runPulumiCmd(["whoami", "--json"]);
            return JSON.parse(result.stdout);
        } else {
            const result = await this.runPulumiCmd(["whoami"]);
            return { user: result.stdout.trim() };
        }
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
     * Returns all stacks from the underlying backend based on the provided
     * options. This queries the underlying backend and may return stacks not
     * present in the workspace as `Pulumi.<stack>.yaml` files.
     *
     * @param opts
     *  Options to customize the behavior of the list.
     */
    async listStacks(opts?: ListOptions): Promise<StackSummary[]> {
        const args = ["stack", "ls", "--json"];
        if (opts) {
            if (opts.all) {
                args.push("--all");
            }
        }
        const result = await this.runPulumiCmd(args);
        return JSON.parse(result.stdout);
    }

    /**
     * Install plugin and language dependencies needed for the project.
     *
     * @param opts Options to customize the behavior of install.
     */
    async install(opts?: InstallOptions): Promise<void> {
        let ver = this._pulumiVersion;
        if (ver === undefined) {
            ver = semver.parse("3.0.0")!;
        }

        if (ver.compare("3.91.0") < 0) {
            // Pulumi 3.91.0 added the `pulumi install` command.
            // https://github.com/pulumi/pulumi/releases/tag/v3.91.0
            throw new Error(`pulumi install requires Pulumi version >= 3.91.0`);
        }

        const args: string[] = [];
        if (opts?.useLanguageVersionTools) {
            if (ver.compare("3.130.0") < 0) {
                // Pulumi 3.130.0 introduced the `--use-language-version-tools` flag.
                // https://github.com/pulumi/pulumi/releases/tag/v3.130.0
                throw new Error(`useLanguageVersionTools requires Pulumi version >= 3.130.0`);
            }
            args.push("--use-language-version-tools");
        }
        if (opts?.noPlugins) {
            args.push("--no-plugins");
        }
        if (opts?.noDependencies) {
            args.push("--no-dependencies");
        }
        if (opts?.reinstall) {
            args.push("--reinstall");
        }
        await this.runPulumiCmd(["install", ...args]);
    }

    /**
     * Installs a plugin in the workspace, for example to use cloud providers
     * like AWS or GCP.
     *
     * @param name
     *  The name of the plugin.
     * @param version
     *  The version of the plugin e.g. "v1.0.0".
     * @param kind
     *  The kind of plugin, defaults to "resource"
     */
    async installPlugin(name: string, version: string, kind = "resource"): Promise<void> {
        await this.runPulumiCmd(["plugin", "install", kind, name, version]);
    }

    /**
     * Installs a plugin in the workspace from a third party server.
     *
     * @param name
     *  The name of the plugin.
     * @param version
     *  The version of the plugin e.g. "v1.0.0".
     * @param server
     *  The server to install the plugin from
     */
    async installPluginFromServer(name: string, version: string, server: string): Promise<void> {
        await this.runPulumiCmd(["plugin", "install", "resource", name, version, "--server", server]);
    }

    /**
     * Removes a plugin from the workspace matching the specified name and version.
     *
     * @param name
     *  The optional name of the plugin.
     * @param versionRange
     *  An optional semver range to check when removing plugins matching the
     *  given name e.g. "1.0.0", ">1.0.0".
     * @param kind
     *  The kind of plugin, defaults to "resource".
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
     * Returns a list of all plugins installed in the workspace.
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
     * Exports the deployment state of the stack. This can be combined with
     * {@link importStack} to edit a stack's state (such as recovery from failed
     * deployments).
     *
     * @param stackName
     *  The name of the stack.
     */
    async exportStack(stackName: string): Promise<Deployment> {
        const result = await this.runPulumiCmd(["stack", "export", "--show-secrets", "--stack", stackName]);
        return JSON.parse(result.stdout);
    }

    /**
     * Imports the given deployment state into a pre-existing stack. This can be
     * combined with {@link exportStack} to edit a stack's state (such as
     * recovery from failed deployments).
     *
     * @param stackName
     *  The name of the stack.
     * @param state
     *  The stack state to import.
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
     * Gets the current set of Stack outputs from the last {@link Stack.up}.
     *
     * @param stackName The name of the stack.
     */
    async stackOutputs(stackName: string): Promise<OutputMap> {
        // TODO: do this in parallel after this is fixed https://github.com/pulumi/pulumi/issues/6050
        const maskedResult = await this.runPulumiCmd(["stack", "output", "--json", "--stack", stackName]);
        const plaintextResult = await this.runPulumiCmd([
            "stack",
            "output",
            "--json",
            "--show-secrets",
            "--stack",
            stackName,
        ]);
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
     * A hook to provide additional args to every CLI commands before they are
     * executed. Provided with stack name, this function returns a list of
     * arguments to append to an invoked command (e.g. `["--config=...", ...]`)
     * Presently, {@link LocalWorkspace} does not utilize this extensibility
     * point.
     */
    async serializeArgsForOp(_: string): Promise<string[]> {
        return [];
    }

    /**
     * A hook executed after every command. Called with the stack name. An
     * extensibility point to perform workspace cleanup (CLI operations may
     * create/modify a `Pulumi.stack.yaml`) {@link LocalWorkspace} does not
     * utilize this extensibility point.
     */
    async postCommandCallback(_: string): Promise<void> {
        return;
    }

    private async checkRemoteSupport() {
        const optOut = !!this.envVars[SKIP_VERSION_CHECK_VAR] || !!process.env[SKIP_VERSION_CHECK_VAR];
        // If remote was specified, ensure the CLI supports it.
        if (!optOut && this.isRemote) {
            // See if `--remote` is present in `pulumi preview --help`'s output.
            const previewResult = await this.runPulumiCmd(["preview", "--help"]);
            const previewOutput = previewResult.stdout.trim();
            if (!previewOutput.includes("--remote")) {
                throw new Error("The Pulumi CLI does not support remote operations. Please upgrade.");
            }
        }
    }

    private async runPulumiCmd(args: string[]): Promise<CommandResult> {
        let envs: { [key: string]: string } = {};
        if (this.pulumiHome) {
            envs["PULUMI_HOME"] = this.pulumiHome;
        }
        if (this.isRemote) {
            envs["PULUMI_EXPERIMENTAL"] = "true";
        }
        envs = { ...envs, ...this.envVars };
        return this.pulumiCommand.run(args, this.workDir, envs);
    }

    /**
     * @internal
     */
    get isRemote(): boolean {
        return !!this.remote;
    }

    /**
     * @internal
     */
    remoteArgs(): string[] {
        const args: string[] = [];
        if (!this.isRemote) {
            return args;
        }

        args.push("--remote");
        if (this.remoteGitProgramArgs) {
            const { url, projectPath, branch, commitHash, auth } = this.remoteGitProgramArgs;
            if (url) {
                args.push(url);
            }
            if (projectPath) {
                args.push("--remote-git-repo-dir", projectPath);
            }
            if (branch) {
                args.push("--remote-git-branch", branch);
            }
            if (commitHash) {
                args.push("--remote-git-commit", commitHash);
            }
            if (auth) {
                const { personalAccessToken, sshPrivateKey, sshPrivateKeyPath, password, username } = auth;
                if (personalAccessToken) {
                    args.push("--remote-git-auth-access-token", personalAccessToken);
                }
                if (sshPrivateKey) {
                    args.push("--remote-git-auth-ssh-private-key", sshPrivateKey);
                }
                if (sshPrivateKeyPath) {
                    args.push("--remote-git-auth-ssh-private-key-path", sshPrivateKeyPath);
                }
                if (password) {
                    args.push("--remote-git-auth-password", password);
                }
                if (username) {
                    args.push("--remote-git-auth-username", username);
                }
            }
        }

        for (const key of Object.keys(this.remoteEnvVars ?? {})) {
            const val = this.remoteEnvVars![key];
            if (typeof val === "string") {
                args.push("--remote-env", `${key}=${val}`);
            } else if ("secret" in val) {
                args.push("--remote-env-secret", `${key}=${val.secret}`);
            } else {
                throw new Error(`unexpected env value '${val}' for key '${key}'`);
            }
        }

        for (const command of this.remotePreRunCommands ?? []) {
            args.push("--remote-pre-run-command", command);
        }

        if (this.remoteSkipInstallDependencies) {
            args.push("--remote-skip-install-dependencies");
        }

        if (this.remoteExecutorImage) {
            args.push("--remote-executor-image=" + this.remoteExecutorImage.image);

            if (this.remoteExecutorImage.credentials) {
                args.push("--remote-executor-image-username=" + this.remoteExecutorImage.credentials.username);
                args.push("--remote-executor-image-password=" + this.remoteExecutorImage.credentials.password);
            }
        }

        if (this.remoteInheritSettings) {
            args.push("--remote-inherit-settings");
        }

        return args;
    }
}

/**
 * Description of a stack backed by an inline (in process) Pulumi program.
 */
export interface InlineProgramArgs {
    /**
     * The associated stack name.
     */
    stackName: string;

    /**
     * The associated project name.
     */
    projectName: string;

    /**
     * The inline (in-process) Pulumi program to use with update and preview operations.
     */
    program: PulumiFn;
}

/**
 * Description of a stack backed by pre-existing local Pulumi CLI program.
 */
export interface LocalProgramArgs {
    /**
     * The associated stack name.
     */
    stackName: string;

    /**
     * The working directory of the program.
     */
    workDir: string;
}

/**
 * Extensibility options to configure a {@link LocalWorkspace;} e.g: settings to
 * seed and environment variables to pass through to every command.
 */
export interface LocalWorkspaceOptions {
    /**
     * The directory to run Pulumi commands and read settings (`Pulumi.yaml` and
     * `Pulumi.<stack>.yaml`).
     */
    workDir?: string;

    /**
     * The directory to override for CLI metadata
     */
    pulumiHome?: string;

    /**
     * The underlying Pulumi CLI.
     */
    pulumiCommand?: PulumiCommand;

    /**
     * The inline program {@link PulumiFn} to be used for preview/update
     * operations, if any. If none is specified, the stack will refer to
     * {@link ProjectSettings} for this information.
     */
    program?: PulumiFn;

    /**
     * The image to use for the remote Pulumi operation.
     */
    remoteExecutorImage?: ExecutorImage;

    /**
     * Environment values scoped to the current workspace. These will be supplied to every Pulumi command.
     */
    envVars?: { [key: string]: string };

    /**
     * The secrets provider to use for encryption and decryption of stack secrets.
     * See: https://www.pulumi.com/docs/intro/concepts/secrets/#available-encryption-providers
     */
    secretsProvider?: string;

    /**
     * The settings object for the current project.
     */
    projectSettings?: ProjectSettings;

    /**
     * A map of stack names and corresponding settings objects.
     */
    stackSettings?: { [key: string]: StackSettings };

    /**
     * True if workspace is a remote workspace.
     *
     * @internal
     */
    remote?: boolean;

    /**
     * The remote Git source info.
     *
     * @internal
     */
    remoteGitProgramArgs?: RemoteGitProgramArgs;

    /**
     * An optional list of arbitrary commands to run before a remote Pulumi operation is invoked.
     *
     * @internal
     */
    remotePreRunCommands?: string[];

    /**
     * The environment variables to pass along when running remote Pulumi operations.
     *
     * @internal
     */
    remoteEnvVars?: { [key: string]: string | { secret: string } };

    /**
     * Whether to skip the default dependency installation step.
     *
     * @internal
     */
    remoteSkipInstallDependencies?: boolean;

    /**
     * Whether to inherit deployment settings from the stack.
     *
     * @internal
     */
    remoteInheritSettings?: boolean;
}

/**
 * Returns true if the provided arguments satisfy the {@link LocalProgramArgs} interface.
 *
 * @param args
 *  The args object to evaluate
 */
function isLocalProgramArgs(args: LocalProgramArgs | InlineProgramArgs): args is LocalProgramArgs {
    return (args as LocalProgramArgs).workDir !== undefined;
}

/**
 * Returns true if the provided arguments satisfy the {@link InlineProgramArgs} interface.
 *
 * @param args
 *  The args object to evaluate
 */
function isInlineProgramArgs(args: LocalProgramArgs | InlineProgramArgs): args is InlineProgramArgs {
    return (args as InlineProgramArgs).projectName !== undefined && (args as InlineProgramArgs).program !== undefined;
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
        if (!fs.existsSync(path)) {
            continue;
        }
        const contents = fs.readFileSync(path).toString();
        if (isJSON) {
            return JSON.parse(contents);
        }
        return yaml.safeLoad(contents) as ProjectSettings;
    }
    throw new Error(`failed to find project settings file in workdir: ${workDir}`);
}

export interface InstallOptions {
    /**
     * Skip installing plugins
     */
    noPlugins?: boolean;
    /**
     * Skip installing dependencies
     */
    noDependencies?: boolean;
    /**
     * Reinstall plugins even if they already exist
     */
    reinstall?: boolean;
    /**
     * Use language version tools to setup the language runtime before installing the dependencies.
     * For Python this will use `pyenv` to install the Python version specified in a
     * `.python-version` file. For Nodejs this will use `fnm` to install the Node.js version
     * specified in a `.nvmrc` or `.node-version file.
     */
    useLanguageVersionTools?: boolean;
}

export interface ListOptions {
    /**
     * List all stacks instead of just stacks for the current project
     */
    all?: boolean;
}

export interface RemoveOptions {
    /**
     * Forces deletion of the stack, leaving behind any resources managed by the stack
     */
    force?: boolean;

    /**
     * Do not delete the corresponding Pulumi.<stack-name>.yaml configuration file for the stack
     */
    preserveConfig?: boolean;

    /**
     * Remove backups of the stack, if using the DIY backend
     */
    removeBackups?: boolean;
}
