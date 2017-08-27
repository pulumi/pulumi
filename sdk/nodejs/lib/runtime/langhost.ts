// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as childprocess from "child_process";
import * as os from "os";
import * as path from "path";

let grpc = require("grpc");
let langproto = require("../../lib/proto/nodejs/languages_pb");
let langrpc = require("../../lib/proto/nodejs/languages_grpc_pb");

// monitorAddr is the current resource monitor address.
let monitorAddr: string | undefined;

// serveLanguageHost spawns a language host that connects to the resource monitor and listens on port.
export function serveLanguageHost(monitor: string, port?: number): { server: any, addr: string } {
    if (monitorAddr) {
        throw new Error("Already connected to a resource monitor; cannot serve two hosts in one process");
    }
    monitorAddr = monitor;

    if (port === undefined) {
        port = 0; // default to port 0 to let the kernel choose one for us.
    }

    // Now fire up the gRPC server and begin serving!
    let server = new grpc.Server();
    server.addService(langrpc.LanguageRuntimeService, {
        runPlan: runPlanRPC,
        runDeploy: runDeployRPC,
    });
    port = server.bind(`0.0.0.0:${port}`, grpc.ServerCredentials.createInsecure());

    // Now we're done: the server is started, and gRPC keeps the even loop alive.
    server.start();
    return { server: server, addr: `0.0.0.0:${port}` }; // return the port for callers.
}

// runPlanRPC implements the RPC interface that the resource monitor calls to execute a program when planning.
function runPlanRPC(call: any, callback: any): void {
    runRPC(call.request, true, callback);
}

// runDeployRPC implements the RPC interface that the resource monitor calls to execute a program when deploying.
function runDeployRPC(call: any, callback: any): void {
    runRPC(call.request, false, callback);
}

// runRPC implements the core "run" logic for both planning and deploying.
function runRPC(req: any, dryrun: boolean, callback: any): void {
    // Unpack the request and fire up the program.
    let resp = new langproto.RunResponse();
    let proc: childprocess.ChildProcess | undefined;
    try {
        // Create an args array to pass to spawn, starting with just the run.js program.
        let args: string[] = [
            path.join(__filename, "..", "..", "..", "cmd", "langhost", "run.js"),
        ];

        // Serialize the config args using [ "--config.k", "v" ] pairs.
        let config: any = req.getConfigMap();
        if (config) {
            for (let entry of config.entries()) {
                args.push(`--config.${entry[0]}`);
                args.push(entry[1]);
            }
        }

        // If this is a dry-run, tell the program so.
        if (dryrun) {
            args.push("--dry-run");
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
        args.push(monitorAddr);

        // Now get a path to the program.  It must be an absolute path.
        let program: string | undefined = req.getProgram();
        if (!program) {
            throw new Error("Expected a non-empty path to the program to run");
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
    }
    catch (err) {
        resp.setError(err.message);
        callback(undefined, resp);
        return;
    }

    // If we got this far, make sure to communicate completion when the process terminates.
    proc.on("close", (code: number) => {
        if (code !== 0) {
            resp.setError(`Program exited with non-zero exit code: ${code}`);
        }
        callback(undefined, resp);
    });
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

