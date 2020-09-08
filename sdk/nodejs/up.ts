import * as grpc from "@grpc/grpc-js";
import { isGrpcError, ResourceError, RunError } from "./errors";
import * as log from "./log";
import { Unwrap, output } from "./output";
import * as runtime from "./runtime";
import * as child_process from "child_process";
import * as events from "events";
import * as stream from "stream";

const anyproto = require("google-protobuf/google/protobuf/any_pb.js");
const emptyproto = require("google-protobuf/google/protobuf/empty_pb.js");
const structproto = require("google-protobuf/google/protobuf/struct_pb.js");
const langproto = require("@pulumi/pulumi/proto/language_pb.js");
const langrpc = require("@pulumi/pulumi/proto/language_grpc_pb.js");
const plugproto = require("@pulumi/pulumi/proto/plugin_pb.js");
const statusproto = require("@pulumi/pulumi/proto/status_pb.js");

// maxRPCMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
const maxRPCMessageSize: number = 1024 * 1024 * 400;

class LanguageServer<T> implements grpc.UntypedServiceImplementation {
    readonly program: () => Promise<T>;
    readonly result: Promise<Unwrap<T>>;

    resolveResult!: (v: any) => void;
    rejectResult!: (e: Error) => void;
    pulumiExitCode?: number;
    running: boolean;

    // Satisfy the grpc.UntypedServiceImplementation interface.
    [name: string]: any;

    constructor(program: () => Promise<T>) {
        this.program = program;
        this.result = new Promise<any>((resolve, reject) => {
            this.resolveResult = resolve;
            this.rejectResult = reject;
        });
        this.running = false;
    }

    getExitError(preview: boolean): Error {
        return new Error(this.pulumiExitCode === 0 ?
            "pulumi exited prematurely" :
            `${preview ? "preview" : "update"} failed with code ${this.pulumiExitCode}`);
    }

    public onPulumiExit(code: number, preview: boolean) {
        this.pulumiExitCode = code;
        if (!this.running) {
            this.rejectResult(this.getExitError(preview));
        }
    }

    public getRequiredPlugins(call: any, callback: any): void {
        const resp: any = new langproto.GetRequiredPluginsResponse();
        resp.setPluginsList([]);
        callback(undefined, resp);
    }

    public async run(call: any, callback: any): Promise<void> {
        const req: any = call.request;
        const resp: any = new langproto.RunResponse();

        if (this.pulumiExitCode !== undefined) {
            callback(this.getExitError(req.getDryrun()));
            return;
        }
        this.running = true;

        const errorSet = new Set<Error>();
        const uncaughtHandler = newUncaughtHandler(errorSet);
        let result: Unwrap<T> | undefined;
        try {
            const args = req.getArgsList();
            const engineAddr = args && args.length > 0 ? args[0] : "";

            runtime.resetOptions(req.getProject(), req.getStack(), req.getParallel(), engineAddr,
                                 req.getMonitorAddress(), req.getDryrun());

            const config: {[key: string]: string} = {};
            for (const [k, v] of req.getConfigMap()?.entries() || []) {
                config[<string>k] = <string>v;
            }
            runtime.setAllConfig(config);

            process.on("uncaughtException", uncaughtHandler);
            // @ts-ignore 'unhandledRejection' will almost always invoke uncaughtHandler with an Error. so
            // just suppress the TS strictness here.
            process.on("unhandledRejection", uncaughtHandler);

            try {
                await runtime.runInPulumiStack(async () => {
                    result = await output(this.program()).promise();
                });

                await runtime.disconnect();
                process.off("uncaughtException", uncaughtHandler);
                process.off("unhandledRejection", uncaughtHandler);
            } catch (e) {
                await runtime.disconnect();
                process.off("uncaughtException", uncaughtHandler);
                process.off("unhandledRejection", uncaughtHandler);

                if (!isGrpcError(e)) {
                    throw e;
                }
            }

            if (errorSet.size !== 0 || log.hasErrors()) {
                throw new Error("One or more errors occurred");
            }

            const [leaks, leakMessage] = runtime.leakedPromises();
            if (leaks.size !== 0) {
                throw new Error(leakMessage);
            }

            this.resolveResult(result);
        } catch (e) {
            const err = e instanceof Error ? e : new Error(`unknown error ${e}`);
            resp.setError(err.message);
            this.rejectResult(err);
        }

        callback(undefined, resp);
    }

    public getPluginInfo(call: any, callback: any): void {
        const resp: any = new plugproto.PluginInfo();
        resp.setVersion("1.0.0");
        callback(undefined, resp);
    }
}

function newUncaughtHandler(errorSet: Set<Error>): (err: Error) => void {
    return (err: Error) => {
        // In node, if you throw an error in a chained promise, but the exception is not finally
        // handled, then you can end up getting an unhandledRejection for each exception/promise
        // pair.  Because the exception is the same through all of these, we keep track of it and
        // only report it once so the user doesn't get N messages for the same thing.
        if (errorSet.has(err)) {
            return;
        }

        errorSet.add(err);

        // Default message should be to include the full stack (which includes the message), or
        // fallback to just the message if we can't get the stack.
        //
        // If both the stack and message are empty, then just stringify the err object itself. This
        // is also necessary as users can throw arbitrary things in JS (including non-Errors).
        const defaultMessage = err.stack || err.message || ("" + err);

        // First, log the error.
        if (RunError.isInstance(err)) {
            // Always hide the stack for RunErrors.
            log.error(err.message);
        }
        else if (ResourceError.isInstance(err)) {
            // Hide the stack if requested to by the ResourceError creator.
            const message = err.hideStack ? err.message : defaultMessage;
            log.error(message, err.resource);
        }
        else if (!isGrpcError(err)) {
            log.error(`Unhandled exception: ${defaultMessage}`);
        }
    };
}

export class Update<T> extends events.EventEmitter {
    public readonly stdout: stream.Readable;
    public readonly stderr: stream.Readable;

    result: Promise<T> | undefined;

    constructor(stdout: stream.Readable, stderr: stream.Readable) {
        super();

        this.stdout = stdout;
        this.stderr = stderr;
    }

    finish(value?: T, error?: Error) {
        if (error) {
            this.emit("error", error);
        } else {
            this.emit("finish", value);
        }
    }

    public promise(): Promise<T> {
        if (!this.result) {
            this.result = new Promise<T>((resolve, reject) => {
                this.on("finish", value => resolve(<T>value));
                this.on("error", error => reject(<Error>error));
            });
        }
        return this.result;
    }
}

export interface UpOptions {
    preview?: boolean;
    echo?: boolean;
    verbosity?: number;
    debugLogging?: boolean;
}

export function up<T>(callback: () => Promise<T> | T, options?: UpOptions): Update<Unwrap<T>> {
    const [stdout, stderr] = [new stream.PassThrough(), new stream.PassThrough()];

    async function runUpdate() {
        // Connect up the gRPC client/server and listen for incoming requests.
        const server = new grpc.Server({
            "grpc.max_receive_message_length": maxRPCMessageSize,
        });
        const languageServer = new LanguageServer<T>(async () => await callback());
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

        try {
            const args = options?.preview ? ["preview"] : ["up", "--skip-preview"];

            const spawnOptions: child_process.SpawnOptions = {};
            if (options?.echo) {
                spawnOptions.stdio = ["inherit", "inherit", "inherit"];
                stdout.end();
                stderr.end();
            }

            const isInteractive = options?.echo && process.stdout.isTTY;
            if (!options?.preview && !isInteractive) {
                args.push("--yes");
            }

            if (options?.verbosity) {
                args.push(`-v=${options!.verbosity}`);
                args.push("--logtostderr")
            }
            if (options?.debugLogging) {
                args.push("-d");
            }

            args.push(`--client=127.0.0.1:${port}`);
            const child = child_process.spawn("pulumi", args, spawnOptions);
            if (!options?.echo) {
                child.stdout!.pipe(stdout);
                child.stderr!.pipe(stderr);
            }

            const pulumiExited = new Promise(resolve => {
                child.on("exit", code => {
                    languageServer.onPulumiExit(code || -1, !!options?.preview);
                    resolve(code || -1);
                });
            });

            const result = await languageServer.result;
            await pulumiExited;
            return result;
        } finally {
            server.forceShutdown();
        }
    }

    const update = new Update<Unwrap<T>>(stdout, stderr);
    runUpdate().then(value => update.finish(value), error => update.finish(undefined, error));
    return update;
}

export interface PreviewOptions {
    echo?: boolean;
    verbosity?: number;
    debugLogging?: boolean;
}

export function preview<T>(callback: () => Promise<T> | T, options?: PreviewOptions): Update<Unwrap<T>> {
    return up(callback, {preview: true, ...options});
}
