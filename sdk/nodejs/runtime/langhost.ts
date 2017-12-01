// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as childprocess from "child_process";
import * as os from "os";
import * as path from "path";
import * as runtime from "../runtime";
import { version } from "../version";

const grpc = require("grpc");
const langproto = require("../proto/language_pb.js");
const langrpc = require("../proto/language_grpc_pb.js");
const plugproto = require("../proto/plugin_pb.js");

/**
 * monitorAddr is the current resource monitor address.
 */
let monitorAddr: string | undefined;
/**
 * engineAddr is the current resource engine address, if any.
 */
let engineAddr: string | undefined;
/**
 * tracingUrl is the current resource engine address, if any.
 */
let tracingUrl: string | undefined;

/**
 * serveLanguageHost spawns a language host that connects to the resource monitor and listens on port.
 */
export function serveLanguageHost(monitor: string, engine?: string, tracing?: string): { server: any, port: number } {
    if (monitorAddr) {
        throw new Error("Already connected to a resource monitor; cannot serve two hosts in one process");
    }
    monitorAddr = monitor;
    engineAddr = engine;
    tracingUrl = tracing;

    // TODO[pulumi/pulumi#545]: Wire up to OpenTracing. Automatic tracing of gRPC calls themselves is pending
    // https://github.com/grpc-ecosystem/grpc-opentracing/issues/11 which is pending
    // https://github.com/grpc/grpc-node/pull/59.

    // Now fire up the gRPC server and begin serving!
    const server = new grpc.Server();
    server.addService(langrpc.LanguageRuntimeService, {
        run: runRPC,
        getPluginInfo: getPluginInfoRPC,
    });
    const port: number = server.bind(`0.0.0.0:0`, grpc.ServerCredentials.createInsecure());

    // Now we're done: the server is started, and gRPC keeps the even loop alive.
    server.start();
    return { server: server, port: port }; // return the port for callers.
}

/**
 * runRPC implements the core "run" logic for both planning and deploying.
 */
function runRPC(call: any, callback: any): void {
    // Unpack the request and fire up the program.
    // IDEA: stick the monitor address in Run's RPC so that it's per invocation.
    const req: any = call.request;
    const resp = new langproto.RunResponse();
    let proc: childprocess.ChildProcess | undefined;
    try {
        // Create an args array to pass to spawn, starting with just the run.js program.
        const args: string[] = [
            path.join(__filename, "..", "..", "cmd", "run"),
        ];

        // Serialize the config args using an environment variable.
        const env: {[key: string]: string} = {};
        const config: any = req.getConfigMap();
        if (config) {
            // First flatten the config into a regular (non-RPC) object.
            const configForEnv: {[key: string]: string} = {};
            for (const entry of config.entries()) {
                configForEnv[(entry[0] as string)] = (entry[1] as string);
            }
            // Now JSON serialize the config into an environment variable.
            env[runtime.configEnvKey] = JSON.stringify(configForEnv);
        }

        const project: string | undefined = req.getProject();
        if (project) {
            args.push("--project");
            args.push(project);
        }

        const stack: string | undefined = req.getStack();
        if (stack) {
            args.push("--stack");
            args.push(stack);
        }

        // If this is a dry-run, tell the program so.
        if (req.getDryrun()) {
            args.push("--dry-run");
        }

        // If parallel execution has been requested, propagate it.
        const parallel: number | undefined = req.getParallel();
        if (parallel !== undefined) {
            args.push("--parallel");
            args.push(parallel.toString());
        }

        // If a different working directory was requested, make sure to pass it too.
        const pwd: string | undefined = req.getPwd();
        if (pwd) {
            args.push("--pwd");
            args.push(pwd);
        }

        // Push the resource monitor address to connect up to.
        if (!monitorAddr) {
            throw new Error("No resource monitor known; please ensure the language host is alive");
        }
        args.push("--monitor");
        args.push(monitorAddr);

        // Push the resource engine address, for logging, etc., if there is one.
        if (engineAddr) {
            args.push("--engine");
            args.push(engineAddr);
        }

        // Push the tracing url, if there is one.
        if (tracingUrl) {
            args.push("--tracing");
            args.push(tracingUrl);
        }

        // Now get a path to the program.
        let program: string | undefined = req.getProgram();
        if (!program) {
            // If the program path is empty, just use "."; this will cause Node to try to load the default module
            // file, by default ./index.js, but possibly overridden in the "main" element inside of package.json.
            program = ".";
        }
        args.push(program);

        // Serialize the args plainly, following the program.
        const argsList: string[] | undefined = req.getArgsList();
        if (argsList) {
            for (const arg of argsList) {
                args.push(arg);
            }
        }

        // We spawn a new process to run the program.  This is required because we don't want the run to complete
        // until the Node message loop quiesces.  It also gives us an extra level of isolation.
        proc = childprocess.spawn(process.argv[0], args, {
            env: Object.assign({}, process.env, env),
        });
        proc.stdout.on("data", (data: string | Buffer) => {
            console.log(stripEOL(data));
        });
        proc.stderr.on("data", (data: string | Buffer) => {
            console.error(stripEOL(data));
        });

        // If we got this far, make sure to communicate completion when the process terminates.
        proc.on("close", (code: number, signal: string) => {
            if (callback !== undefined) {
                if (code !== 0) {
                    if (signal) {
                        resp.setError(`Program exited due to a signal: ${signal}`);
                    }
                    else {
                        resp.setError(`Program exited with non-zero exit code: ${code}`);
                    }
                }
                callback(undefined, resp);
                callback = undefined;
            }
        });

    }
    catch (err) {
        if (callback !== undefined) {
            resp.setError(err.message);
            callback(undefined, resp);
            callback = undefined;
        }
    }
}

function stripEOL(data: string | Buffer): string {
    let dataString: string;
    if (typeof data === "string") {
        dataString = data;
    }
    else {
        dataString = data.toString("utf-8");
    }
    const eolIndex = dataString.lastIndexOf(os.EOL);
    if (eolIndex !== -1) {
        dataString = dataString.substring(0, eolIndex);
    }
    return dataString;
}

/**
 * getPluginInfoRPC implements the RPC interface for plugin introspection.
 */
function getPluginInfoRPC(call: any, callback: any): void {
    const resp = new plugproto.PluginInfo();
    resp.setVersion(version);
    callback(undefined, resp);
}
