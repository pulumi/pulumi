// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Ensure we run the top-level module initializer so we get source-maps, etc.
import "../../lib";

import { ID, URN } from "../../lib";
import * as runtime from "../../lib/runtime";

import { asyncTest } from "../util";
import * as assert from "assert";
import * as childProcess from "child_process";
import * as path from "path";
import * as os from "os";

let gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
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
    createResource?: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
        id?: ID, urn?: URN, props?: any };
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
            createResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                assert.strictEqual(t, "test:index:MyResource");
                assert.strictEqual(name, "testResource1");
                return { id: undefined, urn: undefined, props: undefined };
            },
        },
        // A program that allocates ten simple resources.
        "ten_resources": {
            program: path.join(base, "002.ten_resources"),
            expectResourceCount: 10,
            createResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                assert.strictEqual(t, "test:index:MyResource");
                if (ctx.seen) {
                    assert(!ctx.seen[name],
                        `Got multiple resources with same name ${name}`);
                }
                else {
                    ctx.seen = {};
                }
                const prefix = "testResource";
                assert.strictEqual(name.substring(0, prefix.length), prefix,
                    `Expected ${name} to be of the form ${prefix}N; missing prefix`);
                let seqnum = parseInt(name.substring(prefix.length));
                assert(!isNaN(seqnum),
                    `Expected ${name} to be of the form ${prefix}N; missing N seqnum`);
                ctx.seen[name] = true;
                return { id: undefined, urn: undefined, props: undefined };
            },
        },
        // A program that allocates a complex resource with lots of input and output properties.
        "one_complex_resource": {
            program: path.join(base, "003.one_complex_resource"),
            expectResourceCount: 1,
            createResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                assert.strictEqual(t, "test:index:MyResource");
                assert.strictEqual(name, "testResource1");
                assert.deepEqual(res, {
                    inpropB1: false,
                    inpropB2: true,
                    inpropN: 42,
                    inpropS: "a string",
                    inpropA: [ true, 99, "what a great property" ],
                    inpropO: {
                        b1: false,
                        b2: true,
                        n: 42,
                        s: "another string",
                        a: [ 66, false, "strings galore" ],
                        o: { z: "x" },
                    },
                });
                return {
                    id: name,
                    urn: t + "::" + name,
                    props: {
                        outprop1: "output properties ftw",
                        outprop2: 998.6,
                    },
                };
            },
        },
        // A program that allocates 10 complex resources with lots of input and output properties.
        "ten_complex_resources": {
            program: path.join(base, "004.ten_complex_resources"),
            expectResourceCount: 10,
            createResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                assert.strictEqual(t, "test:index:MyResource");
                if (ctx.seen) {
                    assert(!ctx.seen[name],
                        `Got multiple resources with same name ${name}`);
                }
                else {
                    ctx.seen = {};
                }
                const prefix = "testResource";
                assert.strictEqual(name.substring(0, prefix.length), prefix,
                    `Expected ${name} to be of the form ${prefix}N; missing prefix`);
                let seqnum = parseInt(name.substring(prefix.length));
                assert(!isNaN(seqnum),
                    `Expected ${name} to be of the form ${prefix}N; missing N seqnum`);
                ctx.seen[name] = true;
                assert.deepEqual(res, {
                    inseq: seqnum,
                    inpropB1: false,
                    inpropB2: true,
                    inpropN: 42,
                    inpropS: "a string",
                    inpropA: [ true, 99, "what a great property" ],
                    inpropO: {
                        b1: false,
                        b2: true,
                        n: 42,
                        s: "another string",
                        a: [ 66, false, "strings galore" ],
                        o: { z: "x" },
                    },
                });
                return {
                    id: name,
                    urn: t + "::" + name,
                    props: {
                        outseq: seqnum,
                        outprop1: "output properties ftw",
                        outprop2: 998.6,
                    },
                };
            },
        },
        // A program that allocates a single resource.
        "resource_thens": {
            program: path.join(base, "005.resource_thens"),
            expectResourceCount: 2,
            createResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                switch (t) {
                    case "test:index:ResourceA":
                        assert.strictEqual(name, "resourceA");
                        assert.deepEqual(res, { inprop: 777 });
                        return { id: name, urn: t + "::" + name, props: { outprop: "output yeah" } };
                    case "test:index:ResourceB":
                        assert.strictEqual(name, "resourceB");
                        if (dryrun) {
                            // If this is a dry-run, we won't have the real values:
                            assert.deepEqual(res, {
                                otherIn: runtime.unknownPropertyValue,
                                otherOut: runtime.unknownPropertyValue,
                            });
                        }
                        else {
                            // Otherwise, we will:
                            assert.deepEqual(res, {
                                otherIn: 777,
                                otherOut: "output yeah",
                            });
                        }
                        return { id: name, urn: t + "::" + name };
                    default:
                        assert.fail(`Unrecognized resource type ${t}`);
                        throw new Error();
                }
            },
        },
    };

    for (let casename of Object.keys(cases)) {
        let opts: RunCase = cases[casename];
        it(`run test: ${casename} (pwd=${opts.pwd},prog=${opts.program})`, asyncTest(async () => {
            // For each test case, run it twice: first to plan and then to deploy.
            for (let dryrun of [ true, false ]) {
                console.log(dryrun ? "PLAN:" : "DEPLOY:");

                // First we need to mock the resource monitor.
                let ctx = {};
                let rescnt = 0;
                let monitor = createMockResourceMonitor((call: any, callback: any) => {
                    let resp = new langproto.NewResourceResponse();
                    if (opts.createResource) {
                        let req: any = call.request;
                        let res: any = req.getObject().toJavaScript();
                        let { id, urn, props } = opts.createResource(ctx, dryrun, req.getType(), req.getName(), res);
                        resp.setId(id);
                        resp.setUrn(urn);
                        resp.setObject(gstruct.Struct.fromJavaScript(props));
                    }
                    rescnt++;
                    callback(undefined, resp);
                });

                // Next, go ahead and spawn a new language host that connects to said monitor.
                let langHost = serveLanguageHostProcess(monitor.addr);
                let langHostAddr: string = await langHost.addr;

                // Fake up a client RPC connection to the language host so that we can invoke run.
                let langHostClient = new langrpc.LanguageRuntimeClient(langHostAddr, grpc.credentials.createInsecure());

                // Invoke our little test program; it will allocate a few resources, which we will record.  It will
                // throw an error if anything doesn't look right, which gets reflected back in the run results.
                let runError: string | undefined = await mockRun(langHostClient, opts, dryrun);

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

                // Finally, tear down everything so each test case starts anew.
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
            }
        }));
    }
});

function mockRun(langHostClient: any, opts: RunCase, dryrun: boolean): Promise<string | undefined> {
    return new Promise<string | undefined>(
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
            runReq.setDryrun(dryrun);
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
}

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

