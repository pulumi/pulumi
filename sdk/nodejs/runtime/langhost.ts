// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as childprocess from "child_process";
import * as os from "os";
import * as path from "path";

let grpc = require("grpc");
let langproto = require("../proto/languages_pb.js");
let langrpc = require("../proto/languages_grpc_pb.js");

// monitorAddr is the current resource monitor address.
let monitorAddr: string | undefined;
// engineAddr is the current resource engine address, if any.
let engineAddr: string | undefined;

// serveLanguageHost spawns a language host that connects to the resource monitor and listens on port.
export function serveLanguageHost(monitor: string, engine: string | undefined): { server: any, port: number } {
    if (monitorAddr) {
        throw new Error("Already connected to a resource monitor; cannot serve two hosts in one process");
    }
    monitorAddr = monitor;
    engineAddr = engine;

    // Now fire up the gRPC server and begin serving!
    let server = new grpc.Server();
    server.addService(langrpc.LanguageRuntimeService, { run: runRPC });
    let port: number = server.bind(`0.0.0.0:0`, grpc.ServerCredentials.createInsecure());

    // Now we're done: the server is started, and gRPC keeps the even loop alive.
    server.start();
    return { server: server, port: port }; // return the port for callers.
}

// runRPC implements the core "run" logic for both planning and deploying.
function runRPC(call: any, callback: any): void {
    // Unpack the request and fire up the program.
    // IDEA: stick the monitor address in Run's RPC so that it's per invocation.
    let req: any = call.request;
    let resp = new langproto.RunResponse();
    let proc: childprocess.ChildProcess | undefined;
    try {
        // Create an args array to pass to spawn, starting with just the run.js program.
        let args: string[] = [
            path.join(__filename, "..", "..", "cmd", "run"),
        ];

        // Serialize the config args using "--config.k=v" flags.
        let config: any = req.getConfigMap();
        if (config) {
            for (let entry of config.entries()) {
                args.push(`--config.${entry[0]}=${entry[1]}`);
            }
        }

        // If this is a dry-run, tell the program so.
        if (req.getDryrun()) {
            args.push("--dry-run");
        }

        // If parallel execution has been requested, propagate it.
        let parallel: number | undefined = req.getParallel();
        if (parallel !== undefined) {
            args.push("--parallel");
            args.push(parallel.toString());
        }

        // If a different working directory was requested, make sure to pass it too.
        let pwd: string | undefined = req.getPwd();
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

        // Now get a path to the program.
        let program: string | undefined = req.getProgram();
        if (!program) {
            // If the program path is empty, just use "."; this will cause Node to try to load the default module
            // file, by default ./index.js, but possibly overridden in the "main" element inside of package.json.
            program = ".";
        }
        args.push(program);

        // Serialize the args plainly, following the program.
        let argsList: string[] | undefined = req.getArgsList();
        if (argsList) {
            for (let arg of argsList) {
                args.push(arg);
            }
        }

        // We spawn a new process to run the program.  This is required because we don't want the run to complete
        // until the Node message loop quiesces.  It also gives us an extra level of isolation.
        proc = childprocess.spawn(process.argv[0], args);
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
    let eolIndex = dataString.lastIndexOf(os.EOL);
    if (eolIndex !== -1) {
        dataString = dataString.substring(0, eolIndex);
    }
    return dataString;
}

