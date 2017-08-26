// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Ensure we run the top-level module initializer so we get source-maps, etc.
import "../../lib";
import * as runtime from "../../lib/runtime";

import { asyncTest } from "../util";
import * as assert from "assert";
import * as childProcess from "child_process";
import * as path from "path";
import * as os from "os";

let grpc = require("grpc");
let langrpc = require("../../lib/proto/nodejs/languages_grpc_pb");
let langproto = require("../../lib/proto/nodejs/languages_pb");

interface RunCase {
    pwd?: string;
    program: string;
    args?: string[];
    config?: {[key: string]: string};
    expectError?: string;
    expectResourceCount?: number;
}

describe("rpc", () => {
    let base: string = path.join(path.dirname(__filename), "cases");
    let cases: {[key: string]: RunCase} = {
        // An empty program.
        "empty": {
            program: path.join(base, "000.empty"),
            expectResourceCount: 0,
        },
        // The same thing, just using pwd rather than an absolute program path.
        "empty.pwd": {
            pwd: path.join(base, "000.empty"),
            program: "./",
            expectResourceCount: 0,
        },
        // The same thing, just using pwd and the filename rather than an absolute program path.
        "empty.pwd.index.js": {
            pwd: path.join(base, "000.empty"),
            program: "./index.js",
            expectResourceCount: 0,
        },
        // A program that allocates a single resource.
        "one_resource": {
            program: path.join(base, "001.one_resource"),
            expectResourceCount: 1,
        },
    };

    for (let casename of Object.keys(cases)) {
        let opts: RunCase = cases[casename];
        it(`run test: ${casename} (pwd=${opts.pwd},prog=${opts.program})`, asyncTest(async () => {
            // First we need to mock the resource monitor.
            let rescnt = 0;
            let monitor = createMockResourceMonitor((call: any, callback: any) => {
                // TODO: set Id, Urn, Object.
                rescnt++;
                callback(undefined, new langproto.NewResourceResponse());
            });

            // Next, go ahead and spawn a new language host that connects to said monitor.
            let langHost = serveLanguageHostProcess(monitor.addr);
            let langHostAddr: string = await langHost.addr;

            // Fake up a client RPC connection to the language host so that we can invoke run.
            let langHostClient = new langrpc.LanguageRuntimeClient(langHostAddr, grpc.credentials.createInsecure());

            // Now invoke our little test program; it will allocate a few resources, which we will record.  It will
            // throw an error if anything doesn't look right, which gets reflected back in the run results.
            let runError: string | undefined = await new Promise<string | undefined>(
                (resolve, reject) => {
                    let runReq = new langproto.RunRequest();
                    if (opts.pwd) {
                        runReq.setPwd(opts.pwd);
                    }
                    runReq.setProgram(opts.program);
                    if (opts.args) {
                        runReq.setArgsList(opts.args);
                    }
                    if (opts.config) {
                        let cfgmap = runReq.getConfigMap();
                        for (let cfgkey of Object.keys(opts.config)) {
                            cfgmap.set(cfgkey, opts.config[cfgkey]);
                        }
                    }
                    langHostClient.run(runReq, (err: Error, res: any) => {
                        if (err) {
                            reject(err);
                        }
                        else {
                            // The response has a single field, the error, if any, that occurred (blank means success).
                            resolve(res.getError());
                        }
                    });
                }
            );

            // Validate that everything looks right.
            let expectError: string | undefined = opts.expectError;
            if (expectError === undefined) {
                expectError = "";
            }
            assert.strictEqual(runError, expectError,
                `Expected an error of "${expectError}"; got "${runError}"`);

            let expectResourceCount: number | undefined = opts.expectResourceCount;
            if (expectResourceCount === undefined) {
                expectResourceCount = 0;
            }
            assert.strictEqual(rescnt, expectResourceCount,
                `Expected exactly ${expectResourceCount} resources; got ${rescnt}`);

            // Finally, tear down everything.
            await new Promise<void>((resolve, reject) => {
                langHost.proc.kill();
                langHost.proc.on("close", () => { resolve(); });
            });
            await new Promise<void>((resolve, reject) => {
                monitor.server.tryShutdown((err: Error) => {
                    if (err) {
                        reject(err);
                    }
                    else {
                        resolve();
                    }
                });
            });
        }));
    }
});

function createMockResourceMonitor(
        newResourceCallback: (call: any, request: any) => any): { server: any, addr: string } {
    // The resource monitor is hosted in the current process so it can record state, etc.
    let server = new grpc.Server();
    server.addService(langrpc.ResourceMonitorService, { newResource: newResourceCallback });
    let port = server.bind("0.0.0.0:0", grpc.ServerCredentials.createInsecure());
    server.start();
    return { server: server, addr: `0.0.0.0:${port}` };
}

function serveLanguageHostProcess(monitorAddr: string): { proc: childProcess.ChildProcess, addr: Promise<string> } {
    // Spawn the language host in a separate process so that each test case gets an isolated heap, globals, etc.
    let proc = childProcess.spawn(process.argv[0], [
        path.join(__filename, "..", "..", "..", "cmd", "langhost", "host.js"),
        "--monitor",
        monitorAddr,
    ]);
    // Hook the first line so we can parse the address.  Then we hook the rest to print for debugging purposes, and
    // hand back the resulting process object plus the address we plucked out.
    let addrResolve: ((addr: string) => void) | undefined;
    let addr = new Promise<string>((resolve) => { addrResolve = resolve })
    proc.stdout.on("data", (data) => {
        let dataString: string = stripEOL(data);
        if (addrResolve) {
            // The first line is the address; strip off the newline and resolve the promise.
            addrResolve(dataString);
            addrResolve = undefined;
        }
        console.log(`langhost.stdout: ${dataString}`);
    });
    proc.stderr.on("data", (data) => {
        console.error(`langhost.stderr: ${stripEOL(data)}`);
    });
    return { proc: proc, addr: addr };
}

function stripEOL(data: string | Buffer): string {
    let dataString: string;
    if (typeof data === "string") {
        dataString = data;
    }
    else {
        dataString = data.toString("utf-8");
    }
    let newLineIndex = dataString.lastIndexOf(os.EOL);
    if (newLineIndex !== -1) {
        dataString = dataString.substring(0, newLineIndex);
    }
    return dataString;
}

