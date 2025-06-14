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

import * as fs from "fs";
import * as os from "os";
import * as pathlib from "path";
import * as readline from "readline";
import * as upath from "upath";

import * as grpc from "@grpc/grpc-js";
import TailFile from "@logdna/tail-file";

import * as log from "../log";
import { CommandResult } from "./cmd";
import { ConfigMap, ConfigValue } from "./config";
import { StackNotFoundError } from "./errors";
import { EngineEvent, SummaryEvent } from "./events";
import { LocalWorkspace } from "./localWorkspace";
import { LanguageServer, maxRPCMessageSize } from "./server";
import { TagMap } from "./tag";
import { Deployment, PulumiFn, Workspace } from "./workspace";

import * as langrpc from "../proto/language_grpc_pb";

/**
 * {@link Stack} is an isolated, independently configurable instance of a Pulumi
 * program. {@link Stack} exposes methods for the full Pulumi lifecycle
 * (up/preview/refresh/destroy), as well as managing configuration. Multiple
 * {@link Stacks} are commonly used to denote different phases of development
 * (such as development, staging and production) or feature branches (such as
 * feature-x-dev, jane-feature-x-dev).
 *
 * @alpha
 */
export class Stack {
    /**
     * The name identifying the stack.
     */
    name: string;

    /**
     * The {@link Workspace} the stack was created from.
     */
    readonly workspace: Workspace;

    private ready: Promise<any>;

    /**
     * Creates a new stack using the given workspace, and stack name.
     * It fails if a stack with that name already exists
     *
     * @param name
     *  The name identifying the Stack.
     * @param workspace
     *  The Workspace the Stack was created from.
     */
    static async create(name: string, workspace: Workspace): Promise<Stack> {
        const stack = new Stack(name, workspace, "create");
        await stack.ready;
        return stack;
    }

    /**
     * Selects stack using the given workspace and stack name. It returns an
     * error if the given stack does not exist.
     *
     * @param name
     *  The name identifying the Stack.
     * @param workspace
     *  The {@link Workspace} the stack will be created from.
     */
    static async select(name: string, workspace: Workspace): Promise<Stack> {
        const stack = new Stack(name, workspace, "select");
        await stack.ready;
        return stack;
    }

    /**
     * Creates a new stack using the given workspace and stack name if the stack
     * does not already exist, or falls back to selecting the existing stack. If
     * the stack does not exist, it will be created and selected.
     *
     * @param name
     *  The name identifying the Stack.
     * @param workspace
     *  The {@link Workspace} the stack will be created from.
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
                this.ready = workspace.selectStack(name).catch((err) => {
                    if (err instanceof StackNotFoundError) {
                        return workspace.createStack(name);
                    }
                    throw err;
                });
                return this;
            default:
                throw new Error(`unexpected Stack creation mode: ${mode}`);
        }
    }

    private async readLines(logPath: string, callback: (event: EngineEvent) => void): Promise<ReadlineResult> {
        const eventLogTail = new TailFile(logPath, { startPos: 0, pollFileIntervalMs: 200 }).on("tail_error", (err) => {
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
     * Creates or updates the resources in a stack by executing the program in
     * the {@link Workspace.}
     *
     * @param opts
     *  Options to customize the behavior of the update.
     *
     * @see https://www.pulumi.com/docs/cli/commands/pulumi_up/
     */
    async up(opts?: UpOptions): Promise<UpResult> {
        const args = ["up", "--yes", "--skip-preview"];
        let kind = execKind.local;
        let program = this.workspace.program;

        args.push(...this.remoteArgs());

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
            if (opts.refresh) {
                args.push("--refresh");
            }
            if (opts.diff) {
                args.push("--diff");
            }
            if (opts.replace) {
                for (const rURN of opts.replace) {
                    args.push("--replace", rURN);
                }
            }
            if (opts.exclude) {
                for (const eURN of opts.exclude) {
                    args.push("--exclude", eURN);
                }
            }
            if (opts.target) {
                for (const tURN of opts.target) {
                    args.push("--target", tURN);
                }
            }
            if (opts.policyPacks) {
                for (const pack of opts.policyPacks) {
                    args.push("--policy-pack", pack);
                }
            }
            if (opts.policyPackConfigs) {
                for (const packConfig of opts.policyPackConfigs) {
                    args.push("--policy-pack-config", packConfig);
                }
            }
            if (opts.excludeDependents) {
                args.push("--exclude-dependents");
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
            if (opts.plan) {
                args.push("--plan", opts.plan);
            }
            if (opts.continueOnError) {
                args.push("--continue-on-error");
            }
            if (opts.attachDebugger) {
                args.push("--attach-debugger");
            }
            applyGlobalOpts(opts, args);
        }

        let onExit = (hasError: boolean) => {
            return;
        };
        let didError = false;

        if (program) {
            kind = execKind.inline;
            const server = new grpc.Server({
                "grpc.max_receive_message_length": maxRPCMessageSize,
            });
            const languageServer = new LanguageServer(program);
            server.addService(langrpc.LanguageRuntimeService, languageServer);
            const port: number = await new Promise<number>((resolve, reject) => {
                server.bindAsync(`127.0.0.1:0`, grpc.ServerCredentials.createInsecure(), (err, p) => {
                    if (err) {
                        reject(err);
                    } else {
                        resolve(p);
                    }
                });
            });
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

        let upResult: CommandResult;
        try {
            upResult = await this.runPulumiCmd(args, opts?.onOutput, opts?.onError, opts?.signal);
        } catch (e) {
            didError = true;
            throw e;
        } finally {
            onExit(didError);
            await cleanUp(logFile, await logPromise);
        }

        // TODO: do this in parallel after this is fixed https://github.com/pulumi/pulumi/issues/6050
        const outputs = await this.outputs();
        // If it's a remote workspace, explicitly set showSecrets to false to prevent attempting to
        // load the project file.
        const summary = await this.info(!this.isRemote && opts?.showSecrets);

        return {
            stdout: upResult.stdout,
            stderr: upResult.stderr,
            summary: summary!,
            outputs: outputs!,
        };
    }

    /**
     * Performs a dry-run update to a stack, returning pending changes.
     *
     * @param opts Options to customize the behavior of the preview.
     *
     * @see https://www.pulumi.com/docs/cli/commands/pulumi_preview/
     */
    async preview(opts?: PreviewOptions): Promise<PreviewResult> {
        const args = ["preview"];
        let kind = execKind.local;
        let program = this.workspace.program;

        args.push(...this.remoteArgs());

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
            if (opts.refresh) {
                args.push("--refresh");
            }
            if (opts.diff) {
                args.push("--diff");
            }
            if (opts.replace) {
                for (const rURN of opts.replace) {
                    args.push("--replace", rURN);
                }
            }
            if (opts.exclude) {
                for (const eURN of opts.exclude) {
                    args.push("--exclude", eURN);
                }
            }
            if (opts.target) {
                for (const tURN of opts.target) {
                    args.push("--target", tURN);
                }
            }
            if (opts.policyPacks) {
                for (const pack of opts.policyPacks) {
                    args.push("--policy-pack", pack);
                }
            }
            if (opts.policyPackConfigs) {
                for (const packConfig of opts.policyPackConfigs) {
                    args.push("--policy-pack-config", packConfig);
                }
            }
            if (opts.excludeDependents) {
                args.push("--exclude-dependents");
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
            if (opts.plan) {
                args.push("--save-plan", opts.plan);
            }
            if (opts.importFile) {
                args.push("--import-file", opts.importFile);
            }
            if (opts.attachDebugger) {
                args.push("--attach-debugger");
            }
            applyGlobalOpts(opts, args);
        }

        let onExit = (hasError: boolean) => {
            return;
        };
        let didError = false;

        if (program) {
            kind = execKind.inline;
            const server = new grpc.Server({
                "grpc.max_receive_message_length": maxRPCMessageSize,
            });
            const languageServer = new LanguageServer(program);
            server.addService(langrpc.LanguageRuntimeService, languageServer);
            const port: number = await new Promise<number>((resolve, reject) => {
                server.bindAsync(`127.0.0.1:0`, grpc.ServerCredentials.createInsecure(), (err, p) => {
                    if (err) {
                        reject(err);
                    } else {
                        resolve(p);
                    }
                });
            });
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
        const logPromise = this.readLines(logFile, (event) => {
            if (event.summaryEvent) {
                summaryEvent = event.summaryEvent;
            }
            if (opts?.onEvent) {
                const onEvent = opts.onEvent;
                onEvent(event);
            }
        });

        let previewResult: CommandResult;
        try {
            previewResult = await this.runPulumiCmd(args, opts?.onOutput, opts?.onError, opts?.signal);
        } catch (e) {
            didError = true;
            throw e;
        } finally {
            onExit(didError);
            await cleanUp(logFile, await logPromise);
        }

        if (!summaryEvent) {
            log.warn(
                "Failed to parse summary event, but preview succeeded. PreviewResult `changeSummary` will be empty.",
            );
        }

        return {
            stdout: previewResult.stdout,
            stderr: previewResult.stderr,
            changeSummary: summaryEvent?.resourceChanges || {},
        };
    }

    /**
     * Compares the current stack’s resource state with the state known to exist
     * in the actual cloud provider. Any such changes are adopted into the
     * current stack.
     *
     * @param opts
     *  Options to customize the behavior of the refresh.
     */
    async refresh(opts?: RefreshOptions): Promise<RefreshResult> {
        const args = ["refresh"];

        if (opts?.previewOnly) {
            args.push("--preview-only");
        } else {
            args.push("--skip-preview", "--yes");
        }

        args.push(...this.remoteArgs());

        if (opts) {
            if (opts.message) {
                args.push("--message", opts.message);
            }
            if (opts.expectNoChanges) {
                args.push("--expect-no-changes");
            }
            if (opts.clearPendingCreates) {
                args.push("--clear-pending-creates");
            }
            if (opts.exclude) {
                for (const eURN of opts.exclude) {
                    args.push("--exclude", eURN);
                }
            }
            if (opts.excludeDependents) {
                args.push("--exclude-dependents");
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
            if (opts.runProgram !== undefined) {
                if (opts.runProgram) {
                    args.push("--run-program=true");
                } else {
                    args.push("--run-program=false");
                }
            }
            applyGlobalOpts(opts, args);
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

        let refResult: CommandResult;
        try {
            refResult = await this.runPulumiCmd(args, opts?.onOutput, opts?.onError, opts?.signal);
        } finally {
            await cleanUp(logFile, await logPromise);
        }

        // If it's a remote workspace, explicitly set showSecrets to false to prevent attempting to
        // load the project file.
        const summary = await this.info(!this.isRemote && opts?.showSecrets);
        return {
            stdout: refResult.stdout,
            stderr: refResult.stderr,
            summary: summary!,
        };
    }

    /**
     * Deletes all resources in a stack. By default, this method will leave all
     * history and configuration intact. If `opts.remove` is set, the entire
     * stack and its configuration will also be deleted.
     *
     * @param opts
     *  Options to customize the behavior of the destroy.
     */
    async destroy(opts?: DestroyOptions): Promise<DestroyResult> {
        const args = ["destroy"];

        if (opts?.previewOnly) {
            args.push("--preview-only");
        } else {
            args.push("--yes", "--skip-preview");
        }

        args.push(...this.remoteArgs());

        if (opts) {
            if (opts.message) {
                args.push("--message", opts.message);
            }
            if (opts.exclude) {
                for (const eURN of opts.exclude) {
                    args.push("--exclude", eURN);
                }
            }
            if (opts.target) {
                for (const tURN of opts.target) {
                    args.push("--target", tURN);
                }
            }
            if (opts.excludeDependents) {
                args.push("--exclude-dependents");
            }
            if (opts.targetDependents) {
                args.push("--target-dependents");
            }
            if (opts.excludeProtected) {
                args.push("--exclude-protected");
            }
            if (opts.continueOnError) {
                args.push("--continue-on-error");
            }
            if (opts.parallel) {
                args.push("--parallel", opts.parallel.toString());
            }
            if (opts.userAgent) {
                args.push("--exec-agent", opts.userAgent);
            }
            if (opts.refresh) {
                args.push("--refresh");
            }
            if (opts.runProgram !== undefined) {
                if (opts.runProgram) {
                    args.push("--run-program=true");
                } else {
                    args.push("--run-program=false");
                }
            }
            applyGlobalOpts(opts, args);
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

        let desResult: CommandResult;
        try {
            desResult = await this.runPulumiCmd(args, opts?.onOutput, opts?.onError, opts?.signal);
        } finally {
            await cleanUp(logFile, await logPromise);
        }

        // If it's a remote workspace, explicitly set showSecrets to false to prevent attempting to
        // load the project file.
        const summary = await this.info(!this.isRemote && opts?.showSecrets);

        // If `opts.remove` was set, remove the stack now. We take this approach
        // rather than passing `--remove` to `pulumi destroy` because the latter
        // would make it impossible for us to retrieve a summary of the
        // operation above for returning to the caller.
        if (opts?.remove) {
            await this.workspace.removeStack(this.name);
        }

        return {
            stdout: desResult.stdout,
            stderr: desResult.stderr,
            summary: summary!,
        };
    }

    /**
     * Rename an existing stack
     */
    async rename(options: RenameOptions): Promise<RenameResult> {
        const args = ["stack", "rename", options.stackName];
        args.push(...this.remoteArgs());

        applyGlobalOpts(options, args);

        const renameResult = await this.runPulumiCmd(args, options?.onOutput, options?.onError, options?.signal);

        if (this.isRemote && options?.showSecrets) {
            throw new Error("can't enable `showSecrets` for remote workspaces");
        }

        this.name = options.stackName;

        const summary = await this.info(!this.isRemote && options?.showSecrets);

        return {
            stdout: renameResult.stdout,
            stderr: renameResult.stderr,
            summary: summary!,
        };
    }

    /**
     * Import resources into the stack
     *
     * @param options Options to specify resources and customize the behavior of the import.
     */
    async import(options: ImportOptions): Promise<ImportResult> {
        const args = ["import", "--yes", "--skip-preview"];

        if (options.message) {
            args.push("--message", options.message);
        }

        applyGlobalOpts(options, args);

        let importResult: CommandResult;
        // for import operations, generate a temporary directory to store the following:
        //   - the import file when the user specifies resources to import
        //   - the output file for the generated code
        // we use the import file as input to the import command
        // we the output file to read the generated code and return it to the user
        let tempDir: string = "";
        try {
            tempDir = await fs.promises.mkdtemp(pathlib.join(os.tmpdir(), "pulumi-import-"));
            const tempGeneratedCodeFile = pathlib.join(tempDir, "generated-code.txt");
            if (options.resources) {
                // user has specified resources to import, write them to a temp import file
                const tempImportFile = pathlib.join(tempDir, "import.json");
                await fs.promises.writeFile(
                    tempImportFile,
                    JSON.stringify({
                        nameTable: options.nameTable,
                        resources: options.resources,
                    }),
                );

                args.push("--file", tempImportFile);
            }

            if (options.generateCode === false) {
                // --generate-code is set to true by default
                // only use --generate-code=false if the user explicitly sets it to false
                args.push("--generate-code=false");
            } else {
                args.push("--out", tempGeneratedCodeFile);
            }

            if (options.protect === false) {
                // --protect is set to true by default
                // only use --protect=false if the user explicitly sets it to false
                args.push(`--protect=false`);
            }

            if (options.converter) {
                // if the user specifies a converter, pass it to `--from <converter>` argument of import
                args.push("--from", options.converter);
                if (options.converterArgs) {
                    // pass any additional arguments to the converter
                    // for example using {
                    //   converter: "terraform"
                    //   converterArgs: "./tfstate.json"
                    // }
                    // would be equivalent to `pulumi import --from terraform ./tfstate.json`
                    args.push("--");
                    args.push(...options.converterArgs);
                }
            }

            importResult = await this.runPulumiCmd(args, options.onOutput);

            const summary = await this.info(!this.isRemote && options.showSecrets);

            let generatedCode = "";
            if (options.generateCode !== false) {
                generatedCode = await fs.promises.readFile(tempGeneratedCodeFile, "utf8");
            }

            return {
                stdout: importResult.stdout,
                stderr: importResult.stderr,
                generatedCode: generatedCode,
                summary: summary!,
            };
        } finally {
            if (tempDir !== "") {
                // clean up temp directory we used for the import file
                await fs.promises.rm(tempDir, { recursive: true });
            }
        }
    }

    /**
     * Adds environments to the end of a stack's import list. Imported
     * environments are merged in order per the ESC merge rules. The list of
     * environments behaves as if it were the import list in an anonymous
     * environment.
     *
     * @param environments
     *  The names of the environments to add to the stack's configuration
     */
    async addEnvironments(...environments: string[]): Promise<void> {
        await this.workspace.addEnvironments(this.name, ...environments);
    }

    /**
     * Returns the list of environments currently in the stack's import list.
     */
    async listEnvironments(): Promise<string[]> {
        return this.workspace.listEnvironments(this.name);
    }

    /**
     * Removes an environment from a stack's import list.
     *
     * @param environment
     *  The name of the environment to remove from the stack's configuration
     */
    async removeEnvironment(environment: string): Promise<void> {
        await this.workspace.removeEnvironment(this.name, environment);
    }

    /**
     * Returns the config value associated with the specified key.
     *
     * @param key
     *  The key to use for the config lookup
     * @param path
     *  The key contains a path to a property in a map or list to get
     */
    async getConfig(key: string, path?: boolean): Promise<ConfigValue> {
        return this.workspace.getConfig(this.name, key, path);
    }

    /**
     * Returns the full config map associated with the stack in the workspace.
     */
    async getAllConfig(): Promise<ConfigMap> {
        return this.workspace.getAllConfig(this.name);
    }

    /**
     * Sets a config key-value pair on the stack in the associated Workspace.
     *
     * @param key
     *  The key to set.
     * @param value
     *  The config value to set.
     * @param path
     *  The key contains a path to a property in a map or list to set.
     */
    async setConfig(key: string, value: ConfigValue, path?: boolean): Promise<void> {
        return this.workspace.setConfig(this.name, key, value, path);
    }

    /**
     * Sets all specified config values on the stack in the associated
     * workspace.
     *
     * @param config
     *  The map of config key-value pairs to set.
     * @param path
     *  The keys contain a path to a property in a map or list to set.
     */
    async setAllConfig(config: ConfigMap, path?: boolean): Promise<void> {
        return this.workspace.setAllConfig(this.name, config, path);
    }

    /**
     * Removes the specified config key from the stack in the associated workspace.
     *
     * @param key
     *  The config key to remove.
     * @param path
     *  The key contains a path to a property in a map or list to remove.
     */
    async removeConfig(key: string, path?: boolean): Promise<void> {
        return this.workspace.removeConfig(this.name, key, path);
    }

    /**
     * Removes the specified config keys from the stack in the associated workspace.
     *
     * @param keys
     *  The config keys to remove.
     * @param path
     *  The keys contain a path to a property in a map or list to remove.
     */
    async removeAllConfig(keys: string[], path?: boolean): Promise<void> {
        return this.workspace.removeAllConfig(this.name, keys, path);
    }

    /**
     * Gets and sets the config map used with the last update.
     */
    async refreshConfig(): Promise<ConfigMap> {
        return this.workspace.refreshConfig(this.name);
    }

    /**
     * Returns the tag value associated with specified key.
     *
     * @param key The key to use for the tag lookup.
     */
    async getTag(key: string): Promise<string> {
        return this.workspace.getTag(this.name, key);
    }

    /**
     * Sets a tag key-value pair on the stack in the associated workspace.
     *
     * @param key
     *  The tag key to set.
     * @param value
     *  The tag value to set.
     */
    async setTag(key: string, value: string): Promise<void> {
        await this.workspace.setTag(this.name, key, value);
    }

    /**
     * Removes the specified tag key-value pair from the stack in the associated
     * workspace.
     *
     * @param key The tag key to remove.
     */
    async removeTag(key: string): Promise<void> {
        await this.workspace.removeTag(this.name, key);
    }

    /**
     * Returns the full tag map associated with the stack in the workspace.
     */
    async listTags(): Promise<TagMap> {
        return this.workspace.listTags(this.name);
    }

    /**
     * Gets the current set of stack outputs from the last {@link Stack.up}.
     */
    async outputs(): Promise<OutputMap> {
        return this.workspace.stackOutputs(this.name);
    }

    /**
     * Returns a list summarizing all previous and current results from Stack
     * lifecycle operations (up/preview/refresh/destroy).
     */
    async history(pageSize?: number, page?: number, showSecrets?: boolean): Promise<UpdateSummary[]> {
        const args = ["stack", "history", "--json"];
        if (showSecrets ?? true) {
            args.push("--show-secrets");
        }
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

    async info(showSecrets?: boolean): Promise<UpdateSummary | undefined> {
        const history = await this.history(1 /*pageSize*/, undefined, showSecrets);
        if (!history || history.length === 0) {
            return undefined;
        }
        return history[0];
    }

    /**
     * Stops a stack's currently running update. It returns an error if no
     * update is currently running. Note that this operation is _very
     * dangerous_, and may leave the stack in an inconsistent state if a
     * resource operation was pending when the update was canceled.
     */
    async cancel(): Promise<void> {
        await this.runPulumiCmd(["cancel", "--yes"]);
    }

    /**
     * Exports the deployment state of the stack. This can be combined with
     * {@link Stack.importStack} to edit a stack's state (such as recovery from
     * failed deployments).
     */
    async exportStack(): Promise<Deployment> {
        return this.workspace.exportStack(this.name);
    }

    /**
     * Imports the specified deployment state into a pre-existing stack. This
     * can be combined with {@link Stack.exportStack} to edit a stack's state
     * (such as recovery from failed deployments).
     *
     * @param state
     *  The stack state to import.
     */
    async importStack(state: Deployment): Promise<void> {
        return this.workspace.importStack(this.name, state);
    }

    private async runPulumiCmd(
        args: string[],
        onOutput?: (out: string) => void,
        onError?: (err: string) => void,
        signal?: AbortSignal,
    ): Promise<CommandResult> {
        let envs: { [key: string]: string } = {
            PULUMI_DEBUG_COMMANDS: "true",
        };
        if (this.isRemote) {
            envs["PULUMI_EXPERIMENTAL"] = "true";
        }
        const pulumiHome = this.workspace.pulumiHome;
        if (pulumiHome) {
            envs["PULUMI_HOME"] = pulumiHome;
        }
        envs = { ...envs, ...this.workspace.envVars };
        const additionalArgs = await this.workspace.serializeArgsForOp(this.name);
        args = [...args, "--stack", this.name, ...additionalArgs];
        const result = await this.workspace.pulumiCommand.run(
            args,
            this.workspace.workDir,
            envs,
            onOutput,
            onError,
            signal,
        );
        await this.workspace.postCommandCallback(this.name);
        return result;
    }

    private get isRemote(): boolean {
        const ws = this.workspace;
        return ws instanceof LocalWorkspace ? ws.isRemote : false;
    }

    private remoteArgs(): string[] {
        const ws = this.workspace;
        return ws instanceof LocalWorkspace ? ws.remoteArgs() : [];
    }
}

interface ReadlineResult {
    tail: TailFile;
    rl: readline.Interface;
}

function applyGlobalOpts(opts: GlobalOpts, args: string[]) {
    if (opts.color) {
        args.push("--color", opts.color);
    }
    if (opts.logFlow) {
        args.push("--logflow");
    }
    if (opts.logVerbosity) {
        args.push("--verbose", opts.logVerbosity.toString());
    }
    if (opts.logToStdErr) {
        args.push("--logtostderr");
    }
    if (opts.tracing) {
        args.push("--tracing", opts.tracing);
    }
    if (opts.debug) {
        args.push("--debug");
    }
    if (opts.suppressOutputs) {
        args.push("--suppress-outputs");
    }
    if (opts.suppressProgress) {
        args.push("--suppress-progress");
    }
    if (opts.configFile) {
        args.push("--config-file", opts.configFile);
    }
}

/**
 * Returns a stack name formatted with the greatest possible specificity:
 * `org/project/stack` or `user/project/stack` Using this format avoids
 * ambiguity in stack identity guards creating or selecting the wrong stack.
 *
 * Note: legacy DIY backends (local file, S3, Azure Blob) do not support
 * stack names in this format, and instead only use the stack name without an
 * org/user or project to qualify it.
 *
 * See: https://github.com/pulumi/pulumi/issues/2522
 *
 * Non-legacy DIY backends do support the `org/project/stack` format, but `org`
 * must be set to "organization".
 *
 * @param org
 *  The org (or user) that contains the Stack.
 * @param project
 *  The project that parents the Stack.
 * @param stack
 *  The name of the Stack.
 */
export function fullyQualifiedStackName(org: string, project: string, stack: string): string {
    return `${org}/${project}/${stack}`;
}

/**
 * A set of outputs, keyed by name, that might be returned by a Pulumi program
 * as part of a stack operation.
 */
export type OutputMap = { [key: string]: OutputValue };

/**
 * An output produced by a Pulumi program as part of a stack operation.
 */
export interface OutputValue {
    /**
     * The underlying output value.
     */
    value: any;

    /**
     * True if and only if the value represents a secret.
     */
    secret: boolean;
}

/**
 * A summary of a stack operation.
 */
export interface UpdateSummary {
    // Pre-update information.

    /**
     * The kind of operation to be executed/that was executed.
     */
    kind: UpdateKind;

    /**
     * The time at which the operation started.
     */
    startTime: Date;

    /**
     * An optional message associated with the operation.
     */
    message: string;

    /**
     * The environment supplied to the operation.
     */
    environment: { [key: string]: string };

    /**
     * The configuration used for the operation.
     */
    config: ConfigMap;

    // Post-update information.

    /**
     * The operation result.
     */
    result: UpdateResult;

    /**
     * The time at which the operation completed.
     */
    endTime: Date;

    /**
     * The version of the stack created by the operation.
     */
    version: number;

    /**
     * A raw JSON blob detailing the deployment.
     */
    Deployment?: RawJSON;

    /**
     * A summary of the changes yielded by the operation (e.g. 4 unchanged, 3
     * created, etc.).
     */
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
export type OpType =
    | "same"
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
    /**
     * The standard output from the update.
     */
    stdout: string;

    /**
     * The standard error output from the update.
     */
    stderr: string;

    /**
     * The outputs from the update.
     */
    outputs: OutputMap;

    /**
     * A summary of the update.
     */
    summary: UpdateSummary;
}

/**
 * Output from running a Pulumi program preview.
 */
export interface PreviewResult {
    /**
     * The standard output from the preview.
     */
    stdout: string;

    /**
     * The standard error output from the preview.
     */
    stderr: string;

    /**
     * A summary of the changes yielded by the operation (e.g. 4 unchanged, 3
     * created, etc.).
     */
    changeSummary: OpMap;
}

/**
 * Output from refreshing the resources in a given Stack.
 */
export interface RefreshResult {
    /**
     * The standard output from the refresh.
     */
    stdout: string;

    /**
     * The standard error output from the refresh.
     */
    stderr: string;

    /**
     * A summary of the refresh.
     */
    summary: UpdateSummary;
}

/**
 * Output from destroying all resources in a Stack.
 */
export interface DestroyResult {
    /**
     * The standard output from the destroy.
     */
    stdout: string;

    /**
     * The standard error output from the destroy.
     */
    stderr: string;

    /**
     * A summary of the destroy.
     */
    summary: UpdateSummary;
}

/**
 * Output from renaming the Stack.
 */
export interface RenameResult {
    /**
     * The standard output from the rename.
     */
    stdout: string;

    /**
     * The standard error output from the rename.
     */
    stderr: string;

    /**
     * A summary of the rename.
     */
    summary: UpdateSummary;
}

/**
 * The output from performing an import operation.
 */
export interface ImportResult {
    stdout: string;
    stderr: string;
    generatedCode: string;
    summary: UpdateSummary;
}

export interface GlobalOpts {
    /**
     * Colorize output.
     */
    color?: "always" | "never" | "raw" | "auto";

    /**
     * Flow log settings to child processes (like plugins)
     */
    logFlow?: boolean;

    /**
     * Enable verbose logging (e.g., v=3); anything >3 is very verbose.
     */
    logVerbosity?: number;

    /**
     * Log to stderr instead of to files.
     */
    logToStdErr?: boolean;

    /**
     * Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local files.
     */
    tracing?: string;

    /**
     * Print detailed debugging output during resource operations.
     * */
    debug?: boolean;

    /**
     * Suppress display of stack outputs (in case they contain sensitive values).
     */
    suppressOutputs?: boolean;

    /**
     * Suppress display of periodic progress dots.
     */
    suppressProgress?: boolean;

    /**
     * Use the configuration values in the specified file rather than detecting the file name.
     */
    configFile?: string;

    /**
     * Save any creates seen during the preview into an import file to use with `pulumi import`.
     */
    importFile?: string;
}

/**
 * Options controlling the behavior of a Stack.up() operation.
 */
export interface UpOptions extends GlobalOpts {
    /**
     * Allow P resource operations to run in parallel at once (1 for no parallelism).
     */
    parallel?: number;

    /**
     * Optional message to associate with the operation.
     */
    message?: string;

    /**
     * Return an error if any changes occur during this operation.
     */
    expectNoChanges?: boolean;

    /**
     * Refresh the state of the stack's resources before this update.
     */
    refresh?: boolean;

    /**
     * Display the operation as a rich diff showing the overall change.
     */
    diff?: boolean;

    /**
     * Specify a set of resource URNs to replace.
     */
    replace?: string[];

    /**
     * Run one or more policy packs as part of this operation.
     */
    policyPacks?: string[];

    /**
     * A set of paths to JSON files containing configuration for the supplied `policyPacks`.
     */
    policyPackConfigs?: string[];

    /**
     * Specify a set of resource URNs to exclude from operations.
     */
    exclude?: string[];

    /**
     * Exclude dependents of targets specified with `exclude`.
     */
    excludeDependents?: boolean;

    /**
     * Specify a set of resource URNs to operate on. Other resources will not be updated.
     */
    target?: string[];

    /**
     * Operate on dependent targets discovered but not specified in `targets`.
     */
    targetDependents?: boolean;

    /**
     * A custom user agent to use when executing the operation.
     */
    userAgent?: string;

    /**
     * A callback to be executed when the operation produces stderr output.
     */
    onError?: (err: string) => void;

    /**
     * A callback to be executed when the operation produces stdout output.
     */
    onOutput?: (out: string) => void;

    /**
     * A callback to be executed when the operation yields an event.
     */
    onEvent?: (event: EngineEvent) => void;

    /**
     * An inline (in-process) Pulumi program to execute the operation against.
     */
    program?: PulumiFn;

    /**
     * Plan specifies the path to an update plan to use for the update.
     */
    plan?: string;

    /**
     * Include secrets in the UpSummary.
     */
    showSecrets?: boolean;

    /**
     * Continue the operation to completion even if errors occur.
     */
    continueOnError?: boolean;

    /**
     * Run the process under a debugger, and pause until a debugger is attached.
     */
    attachDebugger?: boolean;

    /**
     * A signal to abort an ongoing operation.
     */
    signal?: AbortSignal;
}

/**
 * Options controlling the behavior of a Stack.preview() operation.
 */
export interface PreviewOptions extends GlobalOpts {
    /**
     * Allow P resource operations to run in parallel at once (1 for no parallelism).
     */
    parallel?: number;

    /**
     * Optional message to associate with the operation.
     */
    message?: string;

    /**
     * Return an error if any changes occur during this operation.
     */
    expectNoChanges?: boolean;

    /**
     * Refresh the state of the stack's resources against the cloud provider before running preview.
     */
    refresh?: boolean;

    /**
     * Display the operation as a rich diff showing the overall change.
     */
    diff?: boolean;

    /**
     * Specify a set of resource URNs to replace.
     */
    replace?: string[];

    /**
     * Run one or more policy packs as part of this operation.
     */
    policyPacks?: string[];

    /**
     * A set of paths to JSON files containing configuration for the supplied `policyPacks`.
     */
    policyPackConfigs?: string[];

    /**
     * Specify a set of resource URNs to exclude from operations.
     */
    exclude?: string[];

    /**
     * Exclude dependents of targets specified with `exclude`.
     */
    excludeDependents?: boolean;

    /**
     * Specify a set of resource URNs to operate on. Other resources will not be updated.
     */
    target?: string[];

    /**
     * Operate on dependent targets discovered but not specified in `targets`.
     */
    targetDependents?: boolean;

    /**
     * A custom user agent to use when executing the operation.
     */
    userAgent?: string;

    /**
     * An inline (in-process) Pulumi program to execute the operation against.
     */
    program?: PulumiFn;

    /**
     * A callback to be executed when the operation produces stderr output.
     */
    onOutput?: (out: string) => void;

    /**
     * A callback to be executed when the operation produces stdout output.
     */
    onError?: (err: string) => void;

    /**
     * A callback to be executed when the operation yields an event.
     */
    onEvent?: (event: EngineEvent) => void;

    /**
     * Plan specifies the path where the update plan should be saved.
     */
    plan?: string;

    /**
     * Run the process under a debugger, and pause until a debugger is attached.
     */
    attachDebugger?: boolean;

    /**
     * A signal to abort an ongoing operation.
     */
    signal?: AbortSignal;
}

/**
 * Options controlling the behavior of a Stack.refresh() operation.
 */
export interface RefreshOptions extends GlobalOpts {
    /**
     * Allow P resource operations to run in parallel at once (1 for no parallelism).
     */
    parallel?: number;

    /**
     * Optional message to associate with the operation.
     */
    message?: string;

    /**
     * Only show a preview of the refresh, but don't perform the refresh itself.
     */
    previewOnly?: boolean;

    /**
     * Return an error if any changes occur during this operation.
     */
    expectNoChanges?: boolean;

    /**
     * Clear all pending creates, dropping them from the state
     */
    clearPendingCreates?: boolean;

    /**
     * Specify a set of resource URNs to exclude from operations.
     */
    exclude?: string[];

    /**
     * Exclude dependents of targets specified with `exclude`.
     */
    excludeDependents?: boolean;

    /**
     * Specify a set of resource URNs to operate on. Other resources will not be updated.
     */
    target?: string[];

    /**
     * Operate on dependent targets discovered but not specified in `targets`.
     */
    targetDependents?: boolean;

    /**
     * A custom user agent to use when executing the operation.
     */
    userAgent?: string;

    /**
     * A callback to be executed when the operation produces stderr output.
     */
    onError?: (err: string) => void;

    /**
     * A callback to be executed when the operation produces stdout output.
     */
    onOutput?: (out: string) => void;

    /**
     * A callback to be executed when the operation yields an event.
     */
    onEvent?: (event: EngineEvent) => void;

    /**
     * Include secrets in the operation summary.
     */
    showSecrets?: boolean;
    /**
     * A signal to abort an ongoing operation.
     */
    signal?: AbortSignal;

    /**
     * Run the program in the workspace to perform the refresh.
     */
    runProgram?: boolean;
}

/**
 * Options controlling the behavior of a Stack.destroy() operation.
 */
export interface DestroyOptions extends GlobalOpts {
    /**
     * Allow P resource operations to run in parallel at once (1 for no parallelism).
     */
    parallel?: number;

    /**
     * Optional message to associate with the operation.
     */
    message?: string;

    /**
     * Refresh the state of the stack's resources against the cloud provider before running destroy.
     */
    refresh?: boolean;

    /**
     * Specify a set of resource URNs to exclude from operations.
     */
    exclude?: string[];

    /**
     * Exclude dependents of targets specified with `exclude`.
     */
    excludeDependents?: boolean;

    /**
     * Specify a set of resource URNs to operate on. Other resources will not be updated.
     */
    target?: string[];

    /**
     * Operate on dependent targets discovered but not specified in `targets`.
     */
    targetDependents?: boolean;

    /**
     * A custom user agent to use when executing the operation.
     */
    userAgent?: string;

    /**
     * A callback to be executed when the operation produces stderr output.
     */
    onError?: (err: string) => void;

    /**
     * A callback to be executed when the operation produces stdout output.
     */
    onOutput?: (out: string) => void;

    /**
     * A callback to be executed when the operation yields an event.
     */
    onEvent?: (event: EngineEvent) => void;

    /**
     * Include secrets in the operation summary.
     */
    showSecrets?: boolean;

    /**
     * Do not destroy protected resources.
     */
    excludeProtected?: boolean;

    /**
     * Continue the operation to completion even if errors occur.
     */
    continueOnError?: boolean;

    /**
     * Only show a preview of the destroy, but don't perform the destroy itself.
     */
    previewOnly?: boolean;

    /**
     * Remove the stack and its configuration after all resources in the stack have been deleted.
     */
    remove?: boolean;
    /**
     * A signal to abort an ongoing operation.
     */
    signal?: AbortSignal;

    /**
     * Run the program in the workspace to perform the destroy.
     */
    runProgram?: boolean;
}

/**
 * Options controlling the behavior of a Stack.rename() operation.
 */
export interface RenameOptions extends GlobalOpts {
    /**
     * The new name for the stack.
     */
    stackName: string;

    /**
     * A callback to be executed when the operation produces stderr output.
     */
    onError?: (err: string) => void;

    /**
     * A callback to be executed when the operation produces stdout output.
     */
    onOutput?: (out: string) => void;

    /**
     * Include secrets in the UpSummary.
     */
    showSecrets?: boolean;

    /**
     * A signal to abort an ongoing operation.
     */
    signal?: AbortSignal;
}

export interface ImportResource {
    /**
     * The type of the resource to import
     */
    type: string;
    /**
     * The name of the resource to import
     */
    name: string;
    /**
     * The ID of the resource to import. The format of the ID is specific to the resource type.
     */
    id?: string;
    parent?: string;
    provider?: string;
    version?: string;
    pluginDownloadUrl?: string;
    logicalName?: string;
    properties?: string[];
    component?: boolean;
    remote?: boolean;
}

/**
 * Options controlling the behavior of a Stack.import() operation.
 */
export interface ImportOptions extends GlobalOpts {
    /**
     * The resource definitions to import into the stack
     */
    resources?: ImportResource[];
    /**
     * The name table maps language names to parent and provider URNs. These names are
     * used in the generated definitions, and should match the corresponding declarations
     * in the source program. This table is required if any parents or providers are
     * specified by the resources to import.
     */
    nameTable?: { [key: string]: string };
    /**
     * Allow resources to be imported with protection from deletion enabled. Set to true by default.
     */
    protect?: boolean;
    /**
     * Generate resource declaration code for the imported resources. Set to true by default.
     */
    generateCode?: boolean;
    /**
     * Specify the name of a converter to import resources from.
     */
    converter?: string;
    /**
     * Additional arguments to pass to the converter, if the user specified one.
     */
    converterArgs?: string[];
    /**
     * Optional message to associate with the import operation
     */
    message?: string;
    /**
     * Include secrets in the import result summary
     */
    showSecrets?: boolean;
    onOutput?: (out: string) => void;
}

const execKind = {
    local: "auto.local",
    inline: "auto.inline",
};

type StackInitMode = "create" | "select" | "createOrSelect";

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
        if (fs.rm) {
            // remove with Node JS 15.X+
            fs.rm(pathlib.dirname(logFile), { recursive: true }, () => {
                return;
            });
        } else {
            // remove with Node JS 14.X
            fs.rmdir(pathlib.dirname(logFile), { recursive: true }, () => {
                return;
            });
        }
    }
};
