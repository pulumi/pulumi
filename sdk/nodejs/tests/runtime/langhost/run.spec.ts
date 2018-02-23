// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import * as childProcess from "child_process";
import * as os from "os";
import * as path from "path";
import { ID, runtime, URN } from "../../../index";
import { asyncTest } from "../../util";

const gempty = require("google-protobuf/google/protobuf/empty_pb.js");
const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
const grpc = require("grpc");
const langrpc = require("../../../proto/language_grpc_pb.js");
const langproto = require("../../../proto/language_pb.js");
const resrpc = require("../../../proto/resource_grpc_pb.js");
const resproto = require("../../../proto/resource_pb.js");

interface RunCase {
    project?: string;
    stack?: string;
    pwd?: string;
    program?: string;
    args?: string[];
    config?: {[key: string]: any};
    expectError?: string;
    expectResourceCount?: number;
    invoke?: (ctx: any, tok: string, args: any) => { failures: any, ret: any };
    registerResource?: (ctx: any, dryrun: boolean, t: string, name: string, res: any, dependencies?: string[]) => {
        urn: URN | undefined, id: ID | undefined, props: any | undefined };
    registerResourceOutputs?: (ctx: any, dryrun: boolean, urn: URN,
                               t: string, name: string, res: any, outputs: any | undefined) => void;
}

function makeUrn(t: string, name: string): URN {
    return `${t}::${name}`;
}

describe("rpc", () => {
    const base: string = path.join(path.dirname(__filename), "cases");
    const cases: {[key: string]: RunCase} = {
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
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                assert.strictEqual(t, "test:index:MyResource");
                assert.strictEqual(name, "testResource1");
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        // A program that allocates ten simple resources.
        "ten_resources": {
            program: path.join(base, "002.ten_resources"),
            expectResourceCount: 10,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
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
                const seqnum = parseInt(name.substring(prefix.length), 10);
                assert(!isNaN(seqnum),
                       `Expected ${name} to be of the form ${prefix}N; missing N seqnum`);
                ctx.seen[name] = true;
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        // A program that allocates a complex resource with lots of input and output properties.
        "one_complex_resource": {
            program: path.join(base, "003.one_complex_resource"),
            expectResourceCount: 1,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
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
                    urn: makeUrn(t, name),
                    id: name,
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
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
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
                const seqnum = parseInt(name.substring(prefix.length), 10);
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
                    urn: makeUrn(t, name),
                    id: name,
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
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string,
                               res: any, dependencies: string[]) => {
                let id: ID | undefined;
                let props: any | undefined;
                switch (t) {
                    case "test:index:ResourceA": {
                        assert.strictEqual(name, "resourceA");
                        assert.deepEqual(res, { inprop: 777 });
                        if (!dryrun) {
                            id = name;
                            props = { outprop: "output yeah" };
                        }
                        break;
                    }
                    case "test:index:ResourceB": {
                        assert.strictEqual(name, "resourceB");
                        assert.deepEqual(dependencies, ["test:index:ResourceA::resourceA"]);
                        if (dryrun) {
                            // If this is a dry-run, we won't have the real values:
                            assert.deepEqual(res, {
                                otherIn: runtime.undefinedValue,
                                otherOut: runtime.undefinedValue,
                            });
                        }
                        else {
                            // Otherwise, we will:
                            assert.deepEqual(res, {
                                otherIn: 777,
                                otherOut: "output yeah",
                            });
                        }
                        if (!dryrun) {
                            id = name;
                        }
                        break;
                    }
                    default:
                        assert.fail(`Unrecognized resource type ${t}`);
                        throw new Error();
                }
                return {
                    urn: makeUrn(t, name),
                    id: id,
                    props: props,
                };
            },
        },
        "input_output": {
            pwd: path.join(base, "006.asset"),
            expectResourceCount: 1,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                assert.strictEqual(t, "test:index:FileResource");
                assert.strictEqual(name, "file1");
                assert.deepEqual(res, {
                    data: {
                        [runtime.specialSigKey]: runtime.specialAssetSig,
                        path: "./testdata.txt",
                    },
                });
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        "promises_io": {
            pwd: path.join(base, "007.promises_io"),
            expectResourceCount: 1,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                assert.strictEqual(t, "test:index:FileResource");
                assert.strictEqual(name, "file1");
                assert.deepEqual(res, {
                    data: "The test worked!\n\nIf you can see some data!\n\n",
                });
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        // A program that allocates ten simple resources that use dependsOn to depend on one another.
        "ten_depends_on_resources": {
            program: path.join(base, "008.ten_depends_on_resources"),
            expectResourceCount: 10,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
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
                const seqnum = parseInt(name.substring(prefix.length), 10);
                assert(!isNaN(seqnum),
                       `Expected ${name} to be of the form ${prefix}N; missing N seqnum`);
                ctx.seen[name] = true;
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        // A simple test of the invocation RPC pathways.
        "invoke": {
            program: path.join(base, "009.invoke"),
            expectResourceCount: 0,
            invoke: (ctx: any, tok: string, args: any) => {
                assert.strictEqual(tok, "invoke:index:echo");
                assert.deepEqual(args, {
                    a: "hello",
                    b: true,
                    c: [ 0.99, 42, { z: "x" } ],
                });
                return { failures: undefined, ret: args };
            },
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                assert.strictEqual(t, "test:index:MyResource");
                assert.strictEqual(name, "testResource1");
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        // Simply test that certain runtime properties are available.
        "runtimeSettings": {
            project: "runtimeSettingsProject",
            stack: "runtimeSettingsStack",
            config: {
                "myBag:config:A": "42",
                "myBag:config:bbbb": "a string o' b's",
            },
            program: path.join(base, "010.runtime_settings"),
            expectResourceCount: 0,
        },
        // A program that throws an ordinary unhandled error.
        "unhandled_error": {
            program: path.join(base, "011.unhandled_error"),
            expectResourceCount: 0,
            expectError: "Program exited with non-zero exit code: 1",
        },
        // A program that creates one resource that contains an assets archive.
        "assets_archive": {
            program: path.join(base, "012.assets_archive"),
            expectResourceCount: 1,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                return { urn: makeUrn(t, name), id: undefined, props: res };
            },
        },
        // A program that contains an unhandled promise rejection.
        "unhandled_promise_rejection": {
            program: path.join(base, "013.unhandled_promise_rejection"),
            expectResourceCount: 0,
            expectError: "Program exited with non-zero exit code: 1",
        },
    };

    for (const casename of Object.keys(cases)) {
        const opts: RunCase = cases[casename];
        it(`run test: ${casename} (pwd=${opts.pwd},prog=${opts.program})`, asyncTest(async () => {
            // For each test case, run it twice: first to preview and then to update.
            for (const dryrun of [ true, false ]) {
                console.log(dryrun ? "PREVIEW:" : "UPDATE:");

                // First we need to mock the resource monitor.
                const ctx: any = {};
                const regs: any = {};
                let regCnt = 0;
                const monitor = createMockResourceMonitor(
                    // Invoke callback
                    (call: any, callback: any) => {
                        const resp = new resproto.InvokeResponse();
                        if (opts.invoke) {
                            const req: any = call.request;
                            const args: any = req.getArgs().toJavaScript();
                            const { failures, ret } =
                                opts.invoke(ctx, req.getTok(), args);
                            resp.setFailuresList(failures);
                            resp.setReturn(gstruct.Struct.fromJavaScript(ret));
                        }
                        callback(undefined, resp);
                    },
                    // RegisterResource callback
                    (call: any, callback: any) => {
                        const resp = new resproto.RegisterResourceResponse();
                        const req: any = call.request;
                        // Skip the automatically generated root component resource.
                        if (req.getType() !== runtime.rootPulumiStackTypeName) {
                            if (opts.registerResource) {
                                const t = req.getType();
                                const name = req.getName();
                                const res: any = req.getObject().toJavaScript();
                                const deps: string[] = req.getDependenciesList();
                                const { urn, id, props } = opts.registerResource(ctx, dryrun, t, name, res, deps);
                                resp.setUrn(urn);
                                resp.setId(id);
                                resp.setObject(gstruct.Struct.fromJavaScript(props));
                                if (urn) {
                                    regs[urn] = { t: t, name: name, props: props };
                                }
                            }
                            regCnt++;
                        }
                        callback(undefined, resp);
                    },
                    // RegisterResourceOutputs callback
                    (call: any, callback: any) => {
                        const req: any = call.request;
                        const urn = req.getUrn();
                        const res = regs[urn];
                        if (res) {
                            if (opts.registerResourceOutputs) {
                                const outs: any = req.getOutputs().toJavaScript();
                                opts.registerResourceOutputs(ctx, dryrun, urn, res.t, res.name, res.props, outs);
                            }
                        }
                        callback(undefined, new gempty.Empty());
                    },
                );

                // Next, go ahead and spawn a new language host that connects to said monitor.
                const langHost = serveLanguageHostProcess();
                const langHostAddr: string = await langHost.addr;

                // Fake up a client RPC connection to the language host so that we can invoke run.
                const langHostClient = new langrpc.LanguageRuntimeClient(
                    langHostAddr, grpc.credentials.createInsecure());

                // Invoke our little test program; it will allocate a few resources, which we will record.  It will
                // throw an error if anything doesn't look right, which gets reflected back in the run results.
                const runError: string | undefined = await mockRun(langHostClient, monitor.addr, opts, dryrun);

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
                assert.strictEqual(regCnt, expectResourceCount,
                                   `Expected exactly ${expectResourceCount} resource registrations; got ${regCnt}`);

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

function mockRun(langHostClient: any, monitor: string, opts: RunCase, dryrun: boolean): Promise<string | undefined> {
    return new Promise<string | undefined>(
        (resolve, reject) => {
            const runReq = new langproto.RunRequest();
            runReq.setMonitorAddress(monitor);
            if (opts.project) {
                runReq.setProject(opts.project);
            }
            if (opts.stack) {
                runReq.setStack(opts.stack);
            }
            if (opts.pwd) {
                runReq.setPwd(opts.pwd);
            }
            runReq.setProgram(opts.program);
            if (opts.args) {
                runReq.setArgsList(opts.args);
            }
            if (opts.config) {
                const cfgmap = runReq.getConfigMap();
                for (const cfgkey of Object.keys(opts.config)) {
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
        },
    );
}

function createMockResourceMonitor(
        invokeCallback: (call: any, request: any) => any,
        registerResourceCallback: (call: any, request: any) => any,
        registerResourceOutputsCallback: (call: any, request: any) => any): { server: any, addr: string } {
    // The resource monitor is hosted in the current process so it can record state, etc.
    const server = new grpc.Server();
    server.addService(resrpc.ResourceMonitorService, {
        invoke: invokeCallback,
        registerResource: registerResourceCallback,
        registerResourceOutputs: registerResourceOutputsCallback,
    });
    const port = server.bind("0.0.0.0:0", grpc.ServerCredentials.createInsecure());
    server.start();
    return { server: server, addr: `0.0.0.0:${port}` };
}

function serveLanguageHostProcess(): { proc: childProcess.ChildProcess, addr: Promise<string> } {
    // A quick note about this:
    //
    // Normally, pulumi-language-nodejs probes the path in order to
    // find the nodejs executor, pulumi-language-nodejs-exec. This works
    // great in all scenarios other than testing within this file. If the executor
    // that it founds resides in the Pulumi install dir (which it will, if these tests
    // are being executed by `make`), then Node will execute it by resolving our relative
    // path requires to the Pulumi install directory. However, the programs being evaluated
    // by the language host are using the current directory to resolve relative path requires.
    //
    // Normally, this isn't a problem - ostensibly the stuff in the Pulumi install directory
    // is the same as the stuff that we are currently testing. However, the "settings.ts" module
    // contains some global state that is expected to be shared between the language host stub
    // that is connecting to our RPC endpoints (run/index.ts) and the Pulumi program being
    // evaluated. Node, when resolving the require, will pick different modules to load depending
    // on which context the require occured; if it happened in run/index.ts, it'll load
    // from Pulumi install directory, while requires coming from anywhere else will load
    // from the source directory. Because these are two different files, Node instantiates two
    // separate module objects and the state that we are expecting to share is not actually shared,
    // ultimately resulting in extremely wacky errors.
    //
    // In order to work around this problem, the langhost is explicitly instructed
    // (through --use-executor) to use a specific executor which will load modules from
    // the source directory and not the install directory.
    const proc = childProcess.spawn("pulumi-language-nodejs", [
        "--use-executor",
        path.join(__filename, "..", "..", "..", "..", "pulumi-language-nodejs-exec-test"),
    ]);
    // Hook the first line so we can parse the address.  Then we hook the rest to print for debugging purposes, and
    // hand back the resulting process object plus the address we plucked out.
    let addrResolve: ((addr: string) => void) | undefined;
    const addr = new Promise<string>((resolve) => { addrResolve = resolve; });
    proc.stdout.on("data", (data) => {
        const dataString: string = stripEOL(data);
        if (addrResolve) {
            // The first line is the address; strip off the newline and resolve the promise.
            addrResolve(`0.0.0.0:${dataString}`);
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
    const newLineIndex = dataString.lastIndexOf(os.EOL);
    if (newLineIndex !== -1) {
        dataString = dataString.substring(0, newLineIndex);
    }
    return dataString;
}

