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

import * as grpc from "@grpc/grpc-js";

import { CommandResult, runPulumiCmd } from "./cmd";
import { ConfigMap, ConfigValue } from "./config";
import { StackAlreadyExistsError } from "./errors";
import { LanguageServer, maxRPCMessageSize } from "./server";
import { PulumiFn, Workspace } from "./workspace";

const langrpc = require("../../proto/language_grpc_pb.js");

/**
 * Stack is an isolated, independently configurable instance of a Pulumi program.
 * Stack exposes methods for the full pulumi lifecycle (up/preview/refresh/destroy), as well as managing configuration.
 * Multiple Stacks are commonly used to denote different phases of development
 * (such as development, staging and production) or feature branches (such as feature-x-dev, jane-feature-x-dev).
 *
 * @alpha
 */
export class Stack {
    /**
     * The name identifying the Stack.
     */
    readonly name: string;
    /**
     * The Workspace the Stack was created from.
     */
    readonly workspace: Workspace;
    private ready: Promise<any>;
    /**
     * Creates a new stack using the given workspace, and stack name.
     * It fails if a stack with that name already exists
     *
     * @param name The name identifying the Stack.
     * @param workspace The Workspace the Stack was created from.
     */
    static async create(name: string, workspace: Workspace): Promise<Stack> {
        const stack = new Stack(name, workspace, "create");
        await stack.ready;
        return stack;
    }
    /**
     * Selects stack using the given workspace, and stack name.
     * It returns an error if the given Stack does not exist. All LocalWorkspace operations will call `select`
     * before running.
     *
     * @param name The name identifying the Stack.
     * @param workspace The Workspace the Stack was created from.
     */
    static async select(name: string, workspace: Workspace): Promise<Stack> {
        const stack = new Stack(name, workspace, "select");
        await stack.ready;
        return stack;
    }
    /**
     * Tries to create a new stack using the given workspace and
     * stack name if the stack does not already exist,
     * or falls back to selecting the existing stack. If the stack does not exist,
     * it will be created and selected.
     *
     * @param name The name identifying the Stack.
     * @param workspace The Workspace the Stack was created from.
     */
    static async createOrSelect(name: string, workspace: Workspace): Promise<Stack> {
        const stack = new Stack(name, workspace, "createOrSelect");
        await stack.ready;
        return stack;
    }
    private constructor(name: string, workspace: Workspace, mode: StackInitMode) {
        this.name = name;
        this.workspace = workspace;

        switch (mode) {
            case "create":
                this.ready = workspace.createStack(name);
                return this;
            case "select":
                this.ready = workspace.selectStack(name);
                return this;
            case "createOrSelect":
                this.ready = workspace.createStack(name).catch((err) => {
                    if (err instanceof StackAlreadyExistsError) {
                        return workspace.selectStack(name);
                    }
                    throw err;
                });
                return this;
            default:
                throw new Error(`unexpected Stack creation mode: ${mode}`);
        }
    }
    /**
     * Creates or updates the resources in a stack by executing the program in the Workspace.
     * https://www.pulumi.com/docs/reference/cli/pulumi_up/
     *
     * @param opts Options to customize the behavior of the update.
     */
    async up(opts?: UpOptions): Promise<UpResult> {
        const args = ["up", "--yes", "--skip-preview"];
        let kind = execKind.local;
        let program = this.workspace.program;
        await this.workspace.selectStack(this.name);

        if (opts) {
            if (opts.program) {
                program = opts.program;
            }
            if (opts.message) {
                args.push("--message", opts.message);
            }
            if (opts.expectNoChanges) {
                args.push("--expect-no-changes");
            }
            if (opts.replace) {
                for (const rURN of opts.replace) {
                    args.push("--replace", rURN);
                }
            }
            if (opts.target) {
                for (const tURN of opts.target) {
                    args.push("--target", tURN);
                }
            }
            if (opts.targetDependents) {
                args.push("--target-dependents");
            }
            if (opts.parallel) {
                args.push("--parallel", opts.parallel.toString());
            }
        }

        let onExit = (code: number) => { return; };

        if (program) {
            kind = execKind.inline;
            const server = new grpc.Server({
                "grpc.max_receive_message_length": maxRPCMessageSize,
            });
            const languageServer = new LanguageServer(program);
            server.addService(langrpc.LanguageRuntimeService, languageServer);
            const port: number = await new Promise<number>((resolve, reject) => {
                server.bindAsync(`0.0.0.0:0`, grpc.ServerCredentials.createInsecure(), (err, p) => {
                    if (err) {
                        reject(err);
                    } else {
                        resolve(p);
                    }
                });
            });
            server.start();
            onExit = (code: number) => {
                languageServer.onPulumiExit(code, false /* preview */);
                server.forceShutdown();
            };
            args.push(`--client=127.0.0.1:${port}`);
        }

        args.push("--exec-kind", kind);
        const upResult = await this.runPulumiCmd(args, opts?.onOutput);
        onExit(upResult.code);
        // TODO: do this in parallel after this is fixed https://github.com/pulumi/pulumi/issues/3877
        const outputs = await this.outputs();
        const summary = await this.info();
        const result: UpResult = {
            stdout: upResult.stdout,
            stderr: upResult.stderr,
            summary: summary!,
            outputs,
        };
        return result;
    }
    /**
     * Preforms a dry-run update to a stack, returning pending changes.
     * https://www.pulumi.com/docs/reference/cli/pulumi_preview/
     *
     * @param opts Options to customize the behavior of the preview.
     */
    async preview(opts?: PreviewOptions): Promise<PreviewResult> {
        // TODO JSON
        const args = ["preview"];
        let kind = execKind.local;
        let program = this.workspace.program;
        await this.workspace.selectStack(this.name);

        if (opts) {
            if (opts.program) {
                program = opts.program;
            }
            if (opts.message) {
                args.push("--message", opts.message);
            }
            if (opts.expectNoChanges) {
                args.push("--expect-no-changes");
            }
            if (opts.replace) {
                for (const rURN of opts.replace) {
                    args.push("--replace", rURN);
                }
            }
            if (opts.target) {
                for (const tURN of opts.target) {
                    args.push("--target", tURN);
                }
            }
            if (opts.targetDependents) {
                args.push("--target-dependents");
            }
            if (opts.parallel) {
                args.push("--parallel", opts.parallel.toString());
            }
        }

        let onExit = (code: number) => { return; };

        if (program) {
            kind = execKind.inline;
            const server = new grpc.Server({
                "grpc.max_receive_message_length": maxRPCMessageSize,
            });
            const languageServer = new LanguageServer(program);
            server.addService(langrpc.LanguageRuntimeService, languageServer);
            const port: number = await new Promise<number>((resolve, reject) => {
                server.bindAsync(`0.0.0.0:0`, grpc.ServerCredentials.createInsecure(), (err, p) => {
                    if (err) {
                        reject(err);
                    } else {
                        resolve(p);
                    }
                });
            });
            server.start();
            onExit = (code: number) => {
                languageServer.onPulumiExit(code, false /* preview */);
                server.forceShutdown();
            };
            args.push(`--client=127.0.0.1:${port}`);
        }

        args.push("--exec-kind", kind);
        const preResult = await this.runPulumiCmd(args);
        onExit(preResult.code);
        const summary = await this.info();
        const result: PreviewResult = {
            stdout: preResult.stdout,
            stderr: preResult.stderr,
            summary: summary!,
        };
        return result;
    }
    /**
     * Compares the current stack’s resource state with the state known to exist in the actual
     * cloud provider. Any such changes are adopted into the current stack.
     *
     * @param opts Options to customize the behavior of the refresh.
     */
    async refresh(opts?: RefreshOptions): Promise<RefreshResult> {
        const args = ["refresh", "--yes", "--skip-preview"];
        await this.workspace.selectStack(this.name);

        if (opts) {
            if (opts.message) {
                args.push("--message", opts.message);
            }
            if (opts.expectNoChanges) {
                args.push("--expect-no-changes");
            }
            if (opts.target) {
                for (const tURN of opts.target) {
                    args.push("--target", tURN);
                }
            }
            if (opts.parallel) {
                args.push("--parallel", opts.parallel.toString());
            }
        }

        const refResult = await this.runPulumiCmd(args);
        const summary = await this.info();
        const result: RefreshResult = {
            stdout: refResult.stdout,
            stderr: refResult.stderr,
            summary: summary!,
        };
        return result;
    }
    /**
     * Destroy deletes all resources in a stack, leaving all history and configuration intact.
     *
     * @param opts Options to customize the behavior of the destroy.
     */
    async destroy(opts?: DestroyOptions): Promise<DestroyResult> {
        const args = ["destroy", "--yes", "--skip-preview"];
        await this.workspace.selectStack(this.name);

        if (opts) {
            if (opts.message) {
                args.push("--message", opts.message);
            }
            if (opts.target) {
                for (const tURN of opts.target) {
                    args.push("--target", tURN);
                }
            }
            if (opts.targetDependents) {
                args.push("--target-dependents");
            }
            if (opts.parallel) {
                args.push("--parallel", opts.parallel.toString());
            }
        }

        const preResult = await this.runPulumiCmd(args);
        const summary = await this.info();
        const result: DestroyResult = {
            stdout: preResult.stdout,
            stderr: preResult.stderr,
            summary: summary!,
        };
        return result;
    }
    /**
     * Returns the config value associated with the specified key.
     *
     * @param key The key to use for the config lookup
     */
    async getConfig(key: string): Promise<ConfigValue> {
        return this.workspace.getConfig(this.name, key);
    }
    /**
     * Returns the full config map associated with the stack in the Workspace.
     */
    async getAllConfig(): Promise<ConfigMap> {
        return this.workspace.getAllConfig(this.name);
    }
    /**
     * Sets a config key-value pair on the Stack in the associated Workspace.
     *
     * @param key The key to set.
     * @param value The config value to set.
     */
    async setConfig(key: string, value: ConfigValue): Promise<void> {
        return this.workspace.setConfig(this.name, key, value);
    }
    /**
     * Sets all specified config values on the stack in the associated Workspace.
     *
     * @param config The map of config key-value pairs to set.
     */
    async setAllConfig(config: ConfigMap): Promise<void> {
        return this.workspace.setAllConfig(this.name, config);
    }
    /**
     * Removes the specified config key from the Stack in the associated Workspace.
     *
     * @param key The config key to remove.
     */
    async removeConfig(key: string): Promise<void> {
        return this.workspace.removeConfig(this.name, key);
    }
    /**
     * Removes the specified config keys from the Stack in the associated Workspace.
     *
     * @param keys The config keys to remove.
     */
    async removeAllConfig(keys: string[]): Promise<void> {
        return this.workspace.removeAllConfig(this.name, keys);
    }
    /**
     * Gets and sets the config map used with the last update.
     */
    async refreshConfig(): Promise<ConfigMap> {
        return this.workspace.refreshConfig(this.name);
    }
    /**
     * Gets the current set of Stack outputs from the last Stack.up().
     */
    async outputs(): Promise<OutputMap> {
        await this.workspace.selectStack(this.name);
        // TODO: do this in parallel after this is fixed https://github.com/pulumi/pulumi/issues/3877
        const maskedResult = await this.runPulumiCmd(["stack", "output", "--json"]);
        const plaintextResult = await this.runPulumiCmd(["stack", "output", "--json", "--show-secrets"]);
        const maskedOuts = JSON.parse(maskedResult.stdout);
        const plaintextOuts = JSON.parse(plaintextResult.stdout);
        const outputs: OutputMap = {};
        const secretSentinal = "[secret]";
        for (const [key, value] of Object.entries(plaintextOuts)) {
            const secret = maskedOuts[key] === secretSentinal;
            outputs[key] = { value, secret };
        }

        return outputs;
    }
    /**
     * Returns a list summarizing all previous and current results from Stack lifecycle operations
     * (up/preview/refresh/destroy).
     */
    async history(): Promise<UpdateSummary[]> {
        const result = await this.runPulumiCmd(["history", "--json", "--show-secrets"]);
        const summaries: UpdateSummary[] = JSON.parse(result.stdout, (key, value) => {
            if (key === "startTime" || key === "endTime") {
                return new Date(value);
            }
            return value;
        });
        return summaries;
    }
    async info(): Promise<UpdateSummary | undefined> {
        const history = await this.history();
        if (!history || history.length === 0) {
            return undefined;
        }
        return history[0];
    }
    private async runPulumiCmd(args: string[], onOutput?: (out: string) => void): Promise<CommandResult> {
        let envs: { [key: string]: string } = {};
        const pulumiHome = this.workspace.pulumiHome;
        if (pulumiHome) {
            envs["PULUMI_HOME"] = pulumiHome;
        }
        envs = { ...envs, ...this.workspace.envVars };
        const additionalArgs = await this.workspace.serializeArgsForOp(this.name);
        args = [...args, ...additionalArgs];
        const result = await runPulumiCmd(args, this.workspace.workDir, envs, onOutput);
        await this.workspace.postCommandCallback(this.name);
        return result;
    }
}

/**
 * Returns a stack name formatted with the greatest possible specificity:
 * org/project/stack or user/project/stack
 * Using this format avoids ambiguity in stack identity guards creating or selecting the wrong stack.
 * Note that filestate backends (local file, S3, Azure Blob) do not support stack names in this
 * format, and instead only use the stack name without an org/user or project to qualify it.
 * See: https://github.com/pulumi/pulumi/issues/2522
 *
 * @param org The org (or user) that contains the Stack.
 * @param project The project that parents the Stack.
 * @param stack The name of the Stack.
 */
export function fullyQualifiedStackName(org: string, project: string, stack: string): string {
    return `${org}/${project}/${stack}`;
}

export interface OutputValue {
    value: any;
    secret: boolean;
}

export type OutputMap = { [key: string]: OutputValue };

export interface UpdateSummary {
    // pre-update info
    kind: UpdateKind;
    startTime: Date;
    message: string;
    environment: { [key: string]: string };
    config: ConfigMap;

    // post-update info
    result: UpdateResult;
    endTime: Date;
    version: number;
    Deployment?: RawJSON;
    resourceChanges?: OpMap;
}

/**
 * The kind of update that was performed on the stack.
 */
export type UpdateKind = "update" | "preview" | "refresh" | "rename" | "destroy" | "import";

/**
 * Represents the current status of a given update.
 */
export type UpdateResult = "not-started" | "in-progress" | "succeeded" | "failed";

/**
 * The granular CRUD operation performed on a particular resource during an update.
 */
export type OpType = "same" | "create" | "update" | "delete" | "replace" | "create-replacement" | "delete-replaced";

/**
 * A map of operation types and their corresponding counts.
 */
export type OpMap = {
    [key in OpType]: number;
};

/**
 * An unstructured JSON string used for back-compat with versioned APIs (such as Deployment).
 */
export type RawJSON = string;

/**
 * The deployment output from running a Pulumi program update.
 */
export interface UpResult {
    stdout: string;
    stderr: string;
    outputs: OutputMap;
    summary: UpdateSummary;
}

/**
 * Output from running a Pulumi program preview.
 */
export interface PreviewResult {
    stdout: string;
    stderr: string;
    summary: UpdateSummary;
}

/**
 * Output from refreshing the resources in a given Stack.
 */
export interface RefreshResult {
    stdout: string;
    stderr: string;
    summary: UpdateSummary;
}

/**
 * Output from destroying all resources in a Stack.
 */
export interface DestroyResult {
    stdout: string;
    stderr: string;
    summary: UpdateSummary;
}

/**
 * Options controlling the behavior of a Stack.up() operation.
 */
export interface UpOptions {
    parallel?: number;
    message?: string;
    expectNoChanges?: boolean;
    replace?: string[];
    target?: string[];
    targetDependents?: boolean;
    onOutput?: (out: string) => void;
    program?: PulumiFn;
}

/**
 * Options controlling the behavior of a Stack.preview() operation.
 */
export interface PreviewOptions {
    parallel?: number;
    message?: string;
    expectNoChanges?: boolean;
    replace?: string[];
    target?: string[];
    targetDependents?: boolean;
    program?: PulumiFn;
}

/**
 * Options controlling the behavior of a Stack.refresh() operation.
 */
export interface RefreshOptions {
    parallel?: number;
    message?: string;
    expectNoChanges?: boolean;
    target?: string[];
    onOutput?: (out: string) => void;
}

/**
 * Options controlling the behavior of a Stack.destroy() operation.
 */
export interface DestroyOptions {
    parallel?: number;
    message?: string;
    target?: string[];
    targetDependents?: boolean;
    onOutput?: (out: string) => void;
}

const execKind = {
    local: "auto.local",
    inline: "auto.inline",
};

type StackInitMode = "create" | "select" | "createOrSelect";
