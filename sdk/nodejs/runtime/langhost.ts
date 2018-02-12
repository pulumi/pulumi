// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as childprocess from "child_process";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import * as runtime from "../runtime";
import { version } from "../version";

const grpc = require("grpc");
const langproto = require("../proto/language_pb.js");
const langrpc = require("../proto/language_grpc_pb.js");
const plugproto = require("../proto/plugin_pb.js");

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
export function serveLanguageHost(engine?: string, tracing?: string): { server: any, port: number } {
    engineAddr = engine;
    tracingUrl = tracing;

    // TODO[pulumi/pulumi#545]: Wire up to OpenTracing. Automatic tracing of gRPC calls themselves is pending
    // https://github.com/grpc-ecosystem/grpc-opentracing/issues/11 which is pending
    // https://github.com/grpc/grpc-node/pull/59.

    // Now fire up the gRPC server and begin serving!
    const server = new grpc.Server();
    server.addService(langrpc.LanguageRuntimeService, {
        getRequiredPlugins: getRequiredPluginsRPC,
        run: runRPC,
        getPluginInfo: getPluginInfoRPC,
    });
    const port: number = server.bind(`0.0.0.0:0`, grpc.ServerCredentials.createInsecure());

    // Now we're done: the server is started, and gRPC keeps the even loop alive.
    server.start();
    return { server: server, port: port }; // return the port for callers.
}

/**
 * getRequiredPluginsRPC returns a list of plugins required by the given program.
 */
function getRequiredPluginsRPC(call: any, callback: any): void {
    // To get the plugins required by a program, find all node_modules/ packages that contain { "pulumi": true }
    // inside of their packacge.json files.  We begin this search in the same directory that contains the project.
    // It's possible that a developer would do a `require("../../elsewhere")` and that we'd miss this as a
    // dependency, however the solution for that is simple: install the package in the project root.
    getPluginsFromDir(call.request.getProgram()).then(
        (plugins: any[]) => {
            const resp = new langproto.GetRequiredPluginsResponse();
            resp.setPluginsList(plugins);
            callback(undefined, resp);
        },
        (err: any) => {
            callback(err);
        },
    );
}

/**
 * getPluginsFromDir enumerates all node_modules/ directories, deeply, and returns the fully concatenated results.
 */
async function getPluginsFromDir(dir: string, pardir: string | undefined): Promise<any[]> {
    const plugins: any[] = [];
    const files = await new Promise<string[]>((resolve, reject) => {
        fs.readdir(dir, (err, ret) => {
            if (err) {
                reject(err);
            }
            else {
                resolve(ret);
            }
        });
    });
    for (const file of files) {
        const curr = path.join(dir, file);
        const stats = await new Promise<fs.Stats>((resolve, reject) => {
            fs.stat(curr, (err, ret) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(ret);
                }
            });
        });
        if (stats.isDirectory()) {
            // if a directory, recurse.
            plugins.push(...await getPluginsFromDir(curr, dir));
        }
        else if (pardir === "node_modules" && file === "package.json") {
            // if a package.json file within a node_modules package, parse it, and see if it's a source of plugins.
            const data = await new Promise<Buffer>((resolve, reject) => {
                fs.readFile(curr, (err, ret) => {
                    if (err) {
                        reject(err);
                    }
                    else {
                        resolve(ret);
                    }
                });
            });
            const json = JSON.parse(data.toString());
            if (json.pulumi && json.pulumi.resource) {
                const dep = new langproto.PluginDependency();
                dep.setName(getPluginName(curr, json));
                dep.setKind("resource");
                dep.setVersion(getPluginVersion(curr, json));
                plugins.push(dep);
            }
        }

    }
    return plugins;
}

function getPluginName(file: string, packageJson: any): string {
    const name = packageJson.name;
    if (!name) {
        throw new Error(`Missing expected "name" property in ${file}`);
    }

    // If the name has a @pulumi scope, we will just use its simple name.  Otherwise, we use the fullly scoped name.
    // We do trim the leading @, however, since Pulumi resource providers do not use the same NPM convention.
    if (name.indexOf("@pulumi/") === 0) {
        return name.substring(name.indexOf("/")+1);
    }
    if (name.indexOf("@") === 0) {
        return name.substring(1);
    }
    return name;
}

function getPluginVersion(file: string, packageJson: any): string {
    const vers = packageJson.version;
    if (!vers) {
        throw new Error(`Missing expected "version" property in ${file}`);
    }
    if (vers.indexOf("v") !== 0) {
        return `v${vers}`;
    }
    return vers;
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
        const monitorAddr: string | undefined = req.getMonitorAddress();
        if (!monitorAddr) {
            throw new Error("Missing resource monitor address in the RPC Run request");
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
    // Only give back a version if we have a valid semver tag.
    if (version !== "" && version !== "${VERSION}") {
        resp.setVersion(version);
    }
    callback(undefined, resp);
}
