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
import * as path from "path";
import * as readline from "readline";
import * as upath from "upath";

import * as grpc from "@grpc/grpc-js";
import * as TailFile from "@logdna/tail-file";

import * as log from "../log";
import { CommandResult, runPulumiCmd } from "./cmd";
import { ConfigMap, ConfigValue } from "./config";
import { StackAlreadyExistsError } from "./errors";
import { EngineEvent, SummaryEvent } from "./events";
import { LanguageServer, maxRPCMessageSize } from "./server";
import { Deployment, PulumiFn, Workspace } from "./workspace";

const langrpc = require("../proto/language_grpc_pb.js");

interface ReadlineResult {
    tail: TailFile;
    rl: readline.Interface;
}

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
     * It returns an error if the given Stack does not exist.
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
    private async readLines(logPath: string, callback: (event: EngineEvent) => void): Promise<ReadlineResult> {
        const eventLogTail = new TailFile(logPath, { startPos: 0, pollFileIntervalMs: 200 })
            .on("tail_error", (err) => {
                throw err;
            });
        await eventLogTail.start();
        const lineSplitter = readline.createInterface({ input: eventLogTail });
        lineSplitter.on("line", (line) => {
            let event: EngineEvent;
            try {
                event = JSON.parse(line);
                callback(event);
            } catch (e) {
                log.warn(`Failed to parse engine event
If you're seeing this warning, please comment on https://github.com/pulumi/pulumi/issues/6768 with the event and any
details about your environment.

Event: ${line}\n${e.toString()}`);
            }
        });

        return {
            tail: eventLogTail,
            rl: lineSplitter,
        };
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
            if (opts.diff) {
                args.push("--diff");
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
            if (opts.userAgent) {
                args.push("--exec-agent", opts.userAgent);
            }
        }

        let onExit = (hasError: boolean) => { return; };
        let didError = false;

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
            onExit = (hasError: boolean) => {
                languageServer.onPulumiExit(hasError);
                server.forceShutdown();
            };
            args.push(`--client=127.0.0.1:${port}`);
        }

        args.push("--exec-kind", kind);

        let logPromise: Promise<ReadlineResult> | undefined;
        let logFile: string | undefined;
        // Set up event log tailing
        if (opts?.onEvent) {
            const onEvent = opts.onEvent;
            logFile = createLogFile("up");
            args.push("--event-log", logFile);

            logPromise = this.readLines(logFile, (event) => {
                onEvent(event);
            });
        }

        const upPromise = this.runPulumiCmd(args, opts?.onOutput);
        let upResult: CommandResult;
        let logResult: ReadlineResult | undefined;
        try {
            [upResult, logResult] = await Promise.all([upPromise, logPromise]);
        } catch (e) {
            didError = true;
            throw e;
        } finally {
            onExit(didError);
            await cleanUp(logFile, logResult);
        }

        // TODO: do this in parallel after this is fixed https://github.com/pulumi/pulumi/issues/6050
        const outputs = await this.outputs();
        const summary = await this.info();

        return {
            stdout: upResult.stdout,
            stderr: upResult.stderr,
            summary: summary!,
            outputs: outputs!,
        };
    }
    /**
     * Performs a dry-run update to a stack, returning pending changes.
     * https://www.pulumi.com/docs/reference/cli/pulumi_preview/
     *
     * @param opts Options to customize the behavior of the preview.
     */
    async preview(opts?: PreviewOptions): Promise<PreviewResult> {
        const args = ["preview"];
        let kind = execKind.local;
        let program = this.workspace.program;

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
            if (opts.diff) {
                args.push("--diff");
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
            if (opts.userAgent) {
                args.push("--exec-agent", opts.userAgent);
            }
        }

        let onExit = (hasError: boolean) => { return; };
        let didError = false;

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
            onExit = (hasError: boolean) => {
                languageServer.onPulumiExit(hasError);
                server.forceShutdown();
            };
            args.push(`--client=127.0.0.1:${port}`);
        }

        args.push("--exec-kind", kind);

        // Set up event log tailing
        const logFile = createLogFile("preview");
        args.push("--event-log", logFile);
        let summaryEvent: SummaryEvent | undefined;
        const rlPromise = this.readLines(logFile, (event) => {
            if (event.summaryEvent) {
                summaryEvent = event.summaryEvent;
            }
            if (opts?.onEvent) {
                const onEvent = opts.onEvent;
                onEvent(event);
            }
        });
        const prePromise = this.runPulumiCmd(args, opts?.onOutput);

        let preResult: CommandResult;
        let rlResult: ReadlineResult | undefined;
        try {
            [preResult, rlResult] = await Promise.all([prePromise, rlPromise]);
        } catch (e) {
            didError = true;
            throw e;
        } finally {
            onExit(didError);
            await cleanUp(logFile, rlResult);
        }

        if (!summaryEvent) {
            log.warn("Failed to parse summary event, but preview succeeded. PreviewResult `changeSummary` will be empty.");
        }

        return {
            stdout: preResult.stdout,
            stderr: preResult.stderr,
            changeSummary: summaryEvent?.resourceChanges || {},
        };
    }
    /**
     * Compares the current stack’s resource state with the state known to exist in the actual
     * cloud provider. Any such changes are adopted into the current stack.
     *
     * @param opts Options to customize the behavior of the refresh.
     */
    async refresh(opts?: RefreshOptions): Promise<RefreshResult> {
        const args = ["refresh", "--yes", "--skip-preview"];

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
            if (opts.userAgent) {
                args.push("--exec-agent", opts.userAgent);
            }
        }

        let logPromise: Promise<ReadlineResult> | undefined;
        let logFile: string | undefined;
        // Set up event log tailing
        if (opts?.onEvent) {
            const onEvent = opts.onEvent;
            logFile = createLogFile("refresh");
            args.push("--event-log", logFile);

            logPromise = this.readLines(logFile, (event) => {
                onEvent(event);
            });
        }

        const kind = this.workspace.program ? execKind.inline : execKind.local;
        args.push("--exec-kind", kind);

        const refPromise = this.runPulumiCmd(args, opts?.onOutput);
        const [refResult, logResult] = await Promise.all([refPromise, logPromise]);
        await cleanUp(logFile, logResult);

        const summary = await this.info();
        return {
            stdout: refResult.stdout,
            stderr: refResult.stderr,
            summary: summary!,
        };
    }
    /**
     * Destroy deletes all resources in a stack, leaving all history and configuration intact.
     *
     * @param opts Options to customize the behavior of the destroy.
     */
    async destroy(opts?: DestroyOptions): Promise<DestroyResult> {
        const args = ["destroy", "--yes", "--skip-preview"];

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
            if (opts.userAgent) {
                args.push("--exec-agent", opts.userAgent);
            }
        }

        let logPromise: Promise<ReadlineResult> | undefined;
        let logFile: string | undefined;
        // Set up event log tailing
        if (opts?.onEvent) {
            const onEvent = opts.onEvent;
            logFile = createLogFile("destroy");
            args.push("--event-log", logFile);

            logPromise = this.readLines(logFile, (event) => {
                onEvent(event);
            });
        }

        const kind = this.workspace.program ? execKind.inline : execKind.local;
        args.push("--exec-kind", kind);

        const desPromise = this.runPulumiCmd(args, opts?.onOutput);
        const [desResult, logResult] = await Promise.all([desPromise, logPromise]);
        await cleanUp(logFile, logResult);

        const summary = await this.info();
        return {
            stdout: desResult.stdout,
            stderr: desResult.stderr,
            summary: summary!,
        };
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
        return this.workspace.stackOutputs(this.name);
    }
    /**
     * Returns a list summarizing all previous and current results from Stack lifecycle operations
     * (up/preview/refresh/destroy).
     */
    async history(pageSize?: number, page?: number): Promise<UpdateSummary[]> {
        const args = ["stack", "history", "--json", "--show-secrets"];
        if (pageSize) {
            if (!page || page < 1) {
                page = 1;
            }
            args.push("--page-size", Math.floor(pageSize).toString(), "--page", Math.floor(page).toString());
        }
        const result = await this.runPulumiCmd(args);

        return JSON.parse(result.stdout, (key, value) => {
            if (key === "startTime" || key === "endTime") {
                return new Date(value);
            }
            return value;
        });
    }
    async info(): Promise<UpdateSummary | undefined> {
        const history = await this.history(1 /*pageSize*/);
        if (!history || history.length === 0) {
            return undefined;
        }
        return history[0];
    }
    /**
     * Cancel stops a stack's currently running update. It returns an error if no update is currently running.
     * Note that this operation is _very dangerous_, and may leave the stack in an inconsistent state
     * if a resource operation was pending when the update was canceled.
     * This command is not supported for local backends.
     */
    async cancel(): Promise<void> {
        await this.runPulumiCmd(["cancel", "--yes"]);
    }

    /**
     * exportStack exports the deployment state of the stack.
     * This can be combined with Stack.importStack to edit a stack's state (such as recovery from failed deployments).
     */
    async exportStack(): Promise<Deployment> {
        return this.workspace.exportStack(this.name);
    }

    /**
     * importStack imports the specified deployment state into a pre-existing stack.
     * This can be combined with Stack.exportStack to edit a stack's state (such as recovery from failed deployments).
     *
     * @param state the stack state to import.
     */
    async importStack(state: Deployment): Promise<void> {
        return this.workspace.importStack(this.name, state);
    }

    private async runPulumiCmd(args: string[], onOutput?: (out: string) => void): Promise<CommandResult> {
        let envs: { [key: string]: string } = {
            "PULUMI_DEBUG_COMMANDS": "true",
        };
        const pulumiHome = this.workspace.pulumiHome;
        if (pulumiHome) {
            envs["PULUMI_HOME"] = pulumiHome;
        }
        envs = { ...envs, ...this.workspace.envVars };
        const additionalArgs = await this.workspace.serializeArgsForOp(this.name);
        args = [...args, "--stack", this.name, ...additionalArgs];
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
export type OpType = "same"
    | "create"
    | "update"
    | "delete"
    | "replace"
    | "create-replacement"
    | "delete-replaced"
    | "read"
    | "read-replacement"
    | "refresh"
    | "discard"
    | "discard-replaced"
    | "remove-pending-replace"
    | "import"
    | "import-replacement";

/**
 * A map of operation types and their corresponding counts.
 */
export type OpMap = {
    [key in OpType]?: number;
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
    changeSummary: OpMap;
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
    diff?: boolean;
    replace?: string[];
    target?: string[];
    targetDependents?: boolean;
    userAgent?: string;
    onOutput?: (out: string) => void;
    onEvent?: (event: EngineEvent) => void;
    program?: PulumiFn;
}

/**
 * Options controlling the behavior of a Stack.preview() operation.
 */
export interface PreviewOptions {
    parallel?: number;
    message?: string;
    expectNoChanges?: boolean;
    diff?: boolean;
    replace?: string[];
    target?: string[];
    targetDependents?: boolean;
    userAgent?: string;
    program?: PulumiFn;
    onOutput?: (out: string) => void;
    onEvent?: (event: EngineEvent) => void;
}

/**
 * Options controlling the behavior of a Stack.refresh() operation.
 */
export interface RefreshOptions {
    parallel?: number;
    message?: string;
    expectNoChanges?: boolean;
    target?: string[];
    userAgent?: string;
    onOutput?: (out: string) => void;
    onEvent?: (event: EngineEvent) => void;
}

/**
 * Options controlling the behavior of a Stack.destroy() operation.
 */
export interface DestroyOptions {
    parallel?: number;
    message?: string;
    target?: string[];
    targetDependents?: boolean;
    userAgent?: string;
    onOutput?: (out: string) => void;
    onEvent?: (event: EngineEvent) => void;
}

const execKind = {
    local: "auto.local",
    inline: "auto.inline",
};

type StackInitMode = "create" | "select" | "createOrSelect";

const delay = (duration: number) => new Promise(resolve => setTimeout(resolve, duration));

const createLogFile = (command: string) => {
    const logDir = fs.mkdtempSync(upath.joinSafe(os.tmpdir(), `automation-logs-${command}-`));
    const logFile = upath.joinSafe(logDir, "eventlog.txt");
    // just open/close the file to make sure it exists when we start polling.
    fs.closeSync(fs.openSync(logFile, "w"));
    return logFile;
};

const cleanUp = async (logFile?: string, rl?: ReadlineResult) => {
    if (rl) {
        // stop tailing
        await rl.tail.quit();
        // close the readline interface
        rl.rl.close();
    }
    if (logFile) {
        // remove the logfile
        fs.rmdir(path.dirname(logFile), { recursive: true }, () => { return; });
    }
};
