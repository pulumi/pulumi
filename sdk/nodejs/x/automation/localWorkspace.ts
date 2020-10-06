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
import * as upath from "upath";

import { CommandResult, runPulumiCmd } from "./cmd";
import { ConfigMap, ConfigValue } from "./config";
import { ProjectSettings } from "./projectSettings";
import { Stack } from "./stack";
import { StackSettings } from "./stackSettings";
import { PulumiFn, StackSummary, WhoAmIResult, Workspace } from "./workspace";

export class LocalWorkspace implements Workspace {
    readonly workDir: string;
    readonly pulumiHome?: string;
    readonly secretsProvider?: string;
    public program?: PulumiFn;
    public envVars: { [key: string]: string };
    private ready: Promise<any[]>;
    public static async create(opts: LocalWorkspaceOptions): Promise<LocalWorkspace> {
        const ws = new LocalWorkspace(opts);
        await ws.ready;
        return ws;
    }
    public static async createStack(args: LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    public static async createStack(args: InlineProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    public static async createStack(args: InlineProgramArgs | LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack> {
        if (isInlineProgramArgs(args)) {
            return await this.inlineSourceStackHelper(args, Stack.create, opts);
        } else if (isLocalProgramArgs(args)) {
            return await this.localSourceStackHelper(args, Stack.create, opts);
        }
        throw new Error(`unexpected args: ${args}`);
    }
    public static async selectStack(args: LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    public static async selectStack(args: InlineProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    public static async selectStack(args: InlineProgramArgs | LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack> {
        if (isInlineProgramArgs(args)) {
            return await this.inlineSourceStackHelper(args, Stack.select, opts);
        } else if (isLocalProgramArgs(args)) {
            return await this.localSourceStackHelper(args, Stack.select, opts);
        }
        throw new Error(`unexpected args: ${args}`);
    }
    public static async createOrSelectStack(args: LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    public static async createOrSelectStack(args: InlineProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack>;
    public static async createOrSelectStack(args: InlineProgramArgs | LocalProgramArgs, opts?: LocalWorkspaceOptions): Promise<Stack> {
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
            wsOpts.projectSettings = defaultProject(args.projectName);
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

        const readinessPromises: Promise<any>[] = [];

        if (opts && opts.projectSettings) {
            readinessPromises.push(this.saveProjectSettings(opts.projectSettings));
        }
        if (opts && opts.stackSettings) {
            for (const [name, value] of Object.entries(opts.stackSettings)) {
                readinessPromises.push(this.saveStackSettings(value, name));
            }
        }

        this.ready = Promise.all(readinessPromises);
    }
    async projectSettings(): Promise<ProjectSettings> {
        for (const ext of settingsExtensions) {
            const isJSON = ext === ".json";
            const path = upath.joinSafe(this.workDir, `Pulumi${ext}`);
            if (!fs.existsSync(path)) { continue; }
            const contents = fs.readFileSync(path).toString();
            if (isJSON) {
                return JSON.parse(contents);
            }
            return yaml.safeLoad(contents) as ProjectSettings;
        }
        throw new Error(`failed to find project settings file in workdir: ${this.workDir}`);
    }
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
    async stackSettings(stackName: string): Promise<StackSettings> {
        const stackSettingsName = getStackSettingsName(stackName);
        for (const ext of settingsExtensions) {
            const isJSON = ext === ".json";
            const path = upath.joinSafe(this.workDir, `Pulumi.${stackSettingsName}${ext}`);
            if (!fs.existsSync(path)) { continue; }
            const contents = fs.readFileSync(path).toString();
            if (isJSON) {
                return JSON.parse(contents);
            }
            return yaml.safeLoad(contents) as StackSettings;
        }
        throw new Error(`failed to find stack settings file in workdir: ${this.workDir}`);
    }
    async saveStackSettings(settings: StackSettings, stackName: string): Promise<void> {
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
        let contents;
        if (foundExt === ".json") {
            contents = JSON.stringify(settings, null, 4);
        }
        else {
            contents = yaml.safeDump(settings, { skipInvalid: true });
        }
        return fs.writeFileSync(path, contents);
    }
    async createStack(stackName: string): Promise<void> {
        const args = ["stack", "init", stackName];
        if (this.secretsProvider) {
            args.push("--secrets-provider", this.secretsProvider);
        }
        await this.runPulumiCmd(args);
    }
    async selectStack(stackName: string): Promise<void> {
        await this.runPulumiCmd(["stack", "select", stackName]);
    }
    async removeStack(stackName: string): Promise<void> {
            await this.runPulumiCmd(["stack", "rm", "--yes", stackName]);
    }
    async getConfig(stackName: string, key: string): Promise<ConfigValue> {
        await this.selectStack(stackName);
        const result = await this.runPulumiCmd(["config", "get", key, "--json"]);
        const val = JSON.parse(result.stdout);
        return val;
    }
    async getAllConfig(stackName: string): Promise<ConfigMap> {
        await this.selectStack(stackName);
        const result = await this.runPulumiCmd(["config", "--show-secrets", "--json"]);
        const val = JSON.parse(result.stdout);
        return val;
    }
    async setConfig(stackName: string, key: string, value: ConfigValue): Promise<void> {
        await this.selectStack(stackName);
        const secretArg = value.secret ? "--secret" : "--plaintext";
        await this.runPulumiCmd(["config", "set", key, value.value, secretArg]);
    }
    async setAllConfig(stackName: string, config: ConfigMap): Promise<void> {
        // TODO: do this in parallel after this is fixed https://github.com/pulumi/pulumi/issues/3877
        for (const [key, value] of Object.entries(config)) {
            await this.setConfig(stackName, key, value);
        }
    }
    async removeConfig(stackName: string, key: string): Promise<void> {
        await this.selectStack(stackName);
        await this.runPulumiCmd(["config", "rm", key]);
    }
    async removeAllConfig(stackName: string, keys: string[]): Promise<void> {
        // TODO: do this in parallel after this is fixed https://github.com/pulumi/pulumi/issues/3877
        for (const key of keys) {
            await this.removeConfig(stackName, key);
        }
    }
    async refreshConfig(stackName: string): Promise<ConfigMap> {
        await this.selectStack(stackName);
        await this.runPulumiCmd(["config", "refresh", "--force"]);
        return this.getAllConfig(stackName);
    }
    async whoAmI(): Promise<WhoAmIResult> {
        const result = await this.runPulumiCmd(["whoami"]);
        return { user: result.stdout.trim() };
    }
    async stack(): Promise<StackSummary | undefined> {
        const stacks = await this.listStacks();
        for (const stack of stacks) {
            if (stack.current) {
                return stack;
            }
        }
        return undefined;
    }
    async listStacks(): Promise<StackSummary[]> {
        const result = await this.runPulumiCmd(["stack", "ls", "--json"]);
        const stacks: StackSummary[] = JSON.parse(result.stdout);
        return stacks;
    }
    async serializeArgsForOp(_: string): Promise<string[]> {
        // LocalWorkspace does not take advantage of this extensibility point.
            return [];
    }
    async postCommandCallback(_: string): Promise<void> {
        // LocalWorkspace does not take advantage of this extensibility point.
        return;
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

export interface InlineProgramArgs {
    stackName: string;
    projectName: string;
    program: PulumiFn;
}

export interface LocalProgramArgs {
    stackName: string;
    workDir: string;
}

export type LocalWorkspaceOptions = {
    workDir?: string,
    pulumiHome?: string,
    program?: PulumiFn,
    envVars?: { [key: string]: string },
    secretsProvider?: string,
    projectSettings?: ProjectSettings,
    stackSettings?: { [key: string]: StackSettings },
};

function isLocalProgramArgs(args: LocalProgramArgs | InlineProgramArgs): args is LocalProgramArgs {
    return (args as LocalProgramArgs).workDir !== undefined;
}

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
    const settings: ProjectSettings = { name: projectName, runtime: "nodejs"};
    return settings;
}
