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

import { CommandResult, runPulumiCmd } from "./cmd";
import { ConfigMap, ConfigValue } from "./config";
import { Workspace } from "./workspace";

export type StackInitMode = "create" | "select" | "upsert";

export class Stack {
    ready: Promise<any>;
    private name: string;
    private workspace: Workspace;
    public static async Create(name: string, workspace: Workspace): Promise<Stack> {
        const stack = new Stack(name, workspace, "create");
        await stack.ready;
        return Promise.resolve(stack);
    }
    public static async Select(name: string, workspace: Workspace): Promise<Stack> {
        const stack = new Stack(name, workspace, "select");
        await stack.ready;
        return Promise.resolve(stack);
    }
    public static async Upsert(name: string, workspace: Workspace): Promise<Stack> {
        const stack = new Stack(name, workspace, "upsert");
        await stack.ready;
        return Promise.resolve(stack);
    }
    constructor(name: string, workspace: Workspace, mode: StackInitMode) {
        this.name = name;
        this.workspace = workspace;

        switch (mode) {
            case "create":
                this.ready = workspace.createStack(name);
                return this;
            case "select":
                this.ready = workspace.selectStack(name);
                return this;
            case "upsert":
                this.ready = workspace.createStack(name).catch(() => {
                    return workspace.selectStack(name);
                });
                return this;
            default:
                throw new Error(`unexpected Stack creation mode: ${mode}`);
        }
    }
    getName(): string { return this.name; }
    getWorkspace(): Workspace { return this.workspace; }
    async getConfig(key: string): Promise<ConfigValue> {
        return this.workspace.getConfig(this.name, key);
    }
    async getAllConfig(): Promise<ConfigMap> {
        return this.workspace.getAllConfig(this.name);
    }
    async setConfig(key: string, value: ConfigValue): Promise<void> {
        return this.workspace.setConfig(this.name, key, value);
    }
    async setAllConfig(config: ConfigMap): Promise<void> {
        return this.workspace.setAllConfig(this.name, config);
    }
    async removeConfig(key: string): Promise<void> {
        return this.workspace.removeConfig(this.name, key);
    }
    async removeAllConfig(keys: string[]): Promise<void> {
        return this.workspace.removeAllConfig(this.name, keys);
    }
    async refreshConfig(): Promise<ConfigMap> {
        return this.workspace.refreshConfig(this.name);
    }
    async outputs(): Promise<OutputMap> {
        await this.workspace.selectStack(this.name);
        const maskedPromise = this.runPulumiCmd(["stack", "output", "--json"]);
        const plaintextPromise = this.runPulumiCmd(["stack", "output", "--json", "--show-secrets"]);
        const results = await Promise.all([maskedPromise, plaintextPromise]);
        const maskedOuts = JSON.parse(results[0].stdout);
        const plaintextOuts = JSON.parse(results[1].stdout);
        const outputs: OutputMap = {};
        const secretSentinal = "[secret]";
        for (const [key, value] of plaintextOuts) {
            const secret = maskedOuts[key] === secretSentinal;
            outputs[key] = { value, secret };
        }

        return Promise.resolve(outputs);
    }
    async history(): Promise<UpdateSummary[]> {
        const result = await this.runPulumiCmd(["history", "--json", "--show-secrets"]);
        const summaries: UpdateSummary[] = JSON.parse(result.stdout);
        return Promise.resolve(summaries);
    }
    async info(): Promise<UpdateSummary | undefined> {
        const history = await this.history();
        if (!history || history.length === 0) {
            return Promise.resolve(undefined);
        }
        return Promise.resolve(history[0]);
    }
    private async runPulumiCmd(args: string[], onOutput?: (out: string) => void): Promise<CommandResult> {
        const ws = this.getWorkspace();
        let envs: { [key: string]: string } = {};
        const pulumiHome = ws.getPulumiHome();
        if (pulumiHome) {
            envs["PULUMI_HOME"] = pulumiHome;
        }
        const additionalEnvs = await ws.getEnvVars();
        envs = { ...envs, ...additionalEnvs };
        const additionalArgs = await ws.serializeArgsForOp(this.name);
        args = [...args, ...additionalArgs];
        const result = await runPulumiCmd(args, ws.getWorkDir(), envs, onOutput);
        await ws.postCommandCallback(this.name);
        return Promise.resolve(result);
    }
}

export function FullyQualifiedStackName(org: string, project: string, stack: string): string {
    return `${org}/${project}/${stack}`;
}

export type OutputValue = {
    value: any;
    secret: boolean;
};

export type OutputMap = { [key: string]: OutputValue };

export type UpdateSummary = {
    // pre-update info
    kind: UpdateKind;
    startTime: number;
    message: string;
    environment: { [key: string]: string };
    config: ConfigMap;

    // post-update info
    result: UpdateResult;
    endTime: number;
    version: number;
    Deployment?: RawJSON;
    resourceChanges?: OpMap;
};

export type UpdateKind = "update" | "preview" | "refresh" | "rename" | "destroy" | "import";

export type UpdateResult = "not-started" | "in-progress" | "succeeded" | "failed";

export type OpType = "same" | "create" | "update" | "delete" | "replace" | "create-replacement" | "delete-replaced";

export type OpMap = {
    [key in OpType]: number;
};

export type RawJSON = string;
