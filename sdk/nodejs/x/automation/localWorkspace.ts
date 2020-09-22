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
import * as os from "os";
import * as upath from "upath";

import { CommandResult, runPulumiCmd } from "./cmd";
import { ConfigMap, ConfigValue } from "./config";
import { ProjectSettings } from "./projectSettings";
import { Stack } from "./stack";
import { StackSettings } from "./stackSettings";
import { PulumiFn, StackSummary, Workspace } from "./workspace";

export class LocalWorkspace implements Workspace {
    ready: Promise<any[]>;
    private workDir: string;
    private pulumiHome?: string;
    private program?: PulumiFn;
    private envVars: { [key: string]: string };
    private secretsProvider?: string;
    public static async NewStackLocalSource(
        stackName: string, workDir: string, opts?: LocalWorkspaceOpts,
    ): Promise<Stack> {
        return this.localSourceStackHelper(stackName, workDir, Stack.Create, opts);
    }
    public static async UpsertStackLocalSource(
        stackName: string, workDir: string, opts?: LocalWorkspaceOpts,
    ): Promise<Stack> {
        return this.localSourceStackHelper(stackName, workDir, Stack.Upsert, opts);
    }
    public static async SelectStackLocalSource(
        stackName: string, workDir: string, opts?: LocalWorkspaceOpts,
    ): Promise<Stack> {
        return this.localSourceStackHelper(stackName, workDir, Stack.Select, opts);
    }
    private static async localSourceStackHelper(
        stackName: string, workDir: string, initFn: stackInitFunc, opts?: LocalWorkspaceOpts,
    ): Promise<Stack> {
        let wsOpts = { workDir };
        if (opts) {
            wsOpts = { ...opts, workDir };
        }

        const ws = new LocalWorkspace(opts);
        await ws.ready;

        const stack = await initFn(stackName, ws);

        return Promise.resolve(stack);
    }
    public static async NewStackInlineSource(
        stackName: string, projectName: string, program: PulumiFn, opts?: LocalWorkspaceOpts,
    ): Promise<Stack> {
        return this.inlineSourceStackHelper(stackName, projectName, program, Stack.Create, opts);
    }
    public static async UpsertStackInlinSource(
        stackName: string, projectName: string, program: PulumiFn, opts?: LocalWorkspaceOpts,
    ): Promise<Stack> {
        return this.inlineSourceStackHelper(stackName, projectName, program, Stack.Upsert, opts);
    }
    public static async SelectStackInlineSource(
        stackName: string, projectName: string, program: PulumiFn, opts?: LocalWorkspaceOpts,
    ): Promise<Stack> {
        return this.inlineSourceStackHelper(stackName, projectName, program, Stack.Select, opts);
    }
    private static async inlineSourceStackHelper(
        stackName: string, projectName: string, program: PulumiFn, initFn: stackInitFunc, opts?: LocalWorkspaceOpts,
    ): Promise<Stack> {
        let wsOpts: LocalWorkspaceOpts = { program };
        if (opts) {
            wsOpts = { ...opts, program };
        }

        if (!wsOpts.projectSettings) {
            wsOpts.projectSettings = defaultProject(projectName);
        }

        const ws = new LocalWorkspace(opts);
        await ws.ready;

        const stack = await initFn(stackName, ws);

        return Promise.resolve(stack);
    }
    constructor(opts?: LocalWorkspaceOpts) {
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
                return Promise.resolve(ProjectSettings.fromJSON(JSON.parse(contents)));
            }
            return Promise.resolve(ProjectSettings.fromYAML(contents));
        }
        return Promise.reject(new Error(`failed to find project settings file in workdir: ${this.workDir}`));
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
            contents = settings.toYAML();
        }
        return Promise.resolve(fs.writeFileSync(path, contents));
    }
    async stackSettings(stackName: string): Promise<StackSettings> {
        const stackSettingsName = getStackSettingsName(stackName);
        for (const ext of settingsExtensions) {
            const isJSON = ext === ".json";
            const path = upath.joinSafe(this.workDir, `Pulumi.${stackSettingsName}${ext}`);
            if (!fs.existsSync(path)) { continue; }
            const contents = fs.readFileSync(path).toString();
            if (isJSON) {
                return Promise.resolve(StackSettings.fromJSON(JSON.parse(contents)));
            }
            return Promise.resolve(StackSettings.fromYAML(contents));
        }
        return Promise.reject(new Error(`failed to find stack settings file in workdir: ${this.workDir}`));
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
            contents = settings.toYAML();
        }
        return Promise.resolve(fs.writeFileSync(path, contents));
    }
    async createStack(stackName: string): Promise<void> {
        const args = ["stack", "init", stackName];
        if (this.secretsProvider) {
            args.push("--secrets-provider", this.secretsProvider);
        }
        try {
            const result = await this.runPulumiCmd(args);
            return Promise.resolve();
        } catch (error) {
            return Promise.reject(error);
        }
    }
    async selectStack(stackName: string): Promise<void> {
        try {
            const result = await this.runPulumiCmd(["stack", "select", stackName]);
            return Promise.resolve();
        } catch (error) {
            return Promise.reject(error);
        }
    }
    async removeStack(stackName: string): Promise<void> {
        try {
            const result = await this.runPulumiCmd(["stack", "rm", "--yes", stackName]);
            return Promise.resolve();
        } catch (error) {
            return Promise.reject(error);
        }
    }
    async getConfig(stackName: string, key: string): Promise<ConfigValue> {
        await this.selectStack(stackName);
        const result = await this.runPulumiCmd(["config", "get", key, "--json"]);
        const val = JSON.parse(result.stdout);
        return Promise.resolve(val);
    }
    async getAllConfig(stackName: string): Promise<ConfigMap> {
        await this.selectStack(stackName);
        const result = await this.runPulumiCmd(["config", "--show-secrets", "--json"]);
        const val = JSON.parse(result.stdout);
        return Promise.resolve(val);
    }
    async setConfig(stackName: string, key: string, value: ConfigValue): Promise<void> {
        await this.selectStack(stackName);
        const secretArg = value.secret ? "--secret" : "--plaintext";
        await this.runPulumiCmd(["config", "set", key, value.value, secretArg]);
        return Promise.resolve();
    }
    async setAllConfig(stackName: string, config: ConfigMap): Promise<void> {
        const promises: Promise<void>[] = [];
        for (const [key, value] of Object.entries(config)) {
            promises.push(this.setConfig(stackName, key, value));
        }
        await Promise.all(promises);
        return Promise.resolve();
    }
    async removeConfig(stackName: string, key: string): Promise<void> {
        await this.selectStack(stackName);
        await this.runPulumiCmd(["config", "rm", key]);
        return Promise.resolve();
    }
    async removeAllConfig(stackName: string, keys: string[]): Promise<void> {
        const promises: Promise<void>[] = [];
        for (const key of keys) {
            promises.push(this.removeConfig(stackName, key));
        }
        return Promise.resolve(<any>{});
    }
    async refreshConfig(stackName: string): Promise<ConfigMap> {
        await this.selectStack(stackName);
        await this.runPulumiCmd(["config", "refresh", "--force"]);
        return this.getAllConfig(stackName);
    }
    getEnvVars(): { [key: string]: string } {
        return this.envVars;
    }
    setEnvVars(envs: { [key: string]: string }): void {
        this.envVars = { ...this.envVars, ...envs };
    }
    setEnvVar(key: string, value: string): void {
        this.envVars[key] = value;
    }
    unsetEnvVar(key: string): void {
        delete this.envVars[key];
    }
    getWorkDir(): string {
        return this.workDir;
    }
    getPulumiHome(): string | undefined {
        return this.pulumiHome;
    }
    async whoAmI(): Promise<string> {
        const result = await this.runPulumiCmd(["whoami"]);
        return Promise.resolve(result.stdout.trim());
    }
    async stack(): Promise<StackSummary | undefined> {
        const stacks = await this.listStacks();
        for (const stack of stacks) {
            if (stack.current) {
                return Promise.resolve(stack);
            }
        }
        return Promise.resolve(undefined);
    }
    async listStacks(): Promise<StackSummary[]> {
        const result = await this.runPulumiCmd(["stack", "ls", "--json"]);
        const stacks: StackSummary[] = JSON.parse(result.stdout);
        return Promise.resolve(stacks);
    }
    getProgram(): PulumiFn | undefined {
        return this.program;
    }
    setProgram(program: PulumiFn): void {
        this.program = program;
    }
    serializeArgsForOp(_: string): Promise<string[]> {
        // LocalWorkspace does not take advantage of this extensibility point.
        return Promise.resolve([]);
    }
    postCommandCallback(_: string): Promise<void> {
        // LocalWorkspace does not take advantage of this extensibility point.
        return Promise.resolve();
    }
    private async runPulumiCmd(
        args: string[],
    ): Promise<CommandResult> {
        let envs: { [key: string]: string } = {};
        if (this.pulumiHome) {
            envs["PULUMI_HOME"] = this.pulumiHome;
        }
        envs = { ...envs, ...this.getEnvVars() };
        return runPulumiCmd(args, this.workDir, envs);
    }
}

export type LocalWorkspaceOpts = {
    workDir?: string,
    pulumiHome?: string,
    program?: PulumiFn,
    envVars?: { [key: string]: string },
    secretsProvider?: string,
    projectSettings?: ProjectSettings,
    stackSettings?: { [key: string]: StackSettings },
};

export const settingsExtensions = [".yaml", ".yml", ".json"];

const getStackSettingsName = (name: string): string => {
    const parts = name.split("/");
    if (parts.length < 1) {
        return name;
    }
    return parts[parts.length - 1];
};

type stackInitFunc = (name: string, workspace: Workspace) => Promise<Stack>;

const defaultProject = (projectName: string) => {
    const settings = new ProjectSettings();
    settings.name = projectName;
    settings.runtime.name = "nodejs";
    return settings;
};
