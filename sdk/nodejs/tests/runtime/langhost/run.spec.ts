// Copyright 2016-2018, Pulumi Corporation.
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

import * as assert from "assert";
import * as childProcess from "child_process";
import * as os from "os";
import * as path from "path";
import { ID, runtime, URN } from "../../../index";
import { asyncTest } from "../../util";

const enginerpc = require("../../../proto/engine_grpc_pb.js");
const engineproto = require("../../../proto/engine_pb.js");
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
    expectedLogs?: {
        count?: number;
        ignoreDebug?: boolean;
    };
    skipRootResourceEndpoints?: boolean;
    showRootResourceRegistration?: boolean;
    invoke?: (ctx: any, tok: string, args: any) => { failures: any, ret: any };
    readResource?: (ctx: any, t: string, name: string, id: string, par: string, state: any) => {
        urn: URN | undefined, props: any | undefined };
    registerResource?: (ctx: any, dryrun: boolean, t: string, name: string, res: any, dependencies?: string[],
                        custom?: boolean, protect?: boolean, parent?: string, provider?: string,
                        propertyDeps?: any) => { urn: URN | undefined, id: ID | undefined, props: any | undefined };
    registerResourceOutputs?: (ctx: any, dryrun: boolean, urn: URN,
                               t: string, name: string, res: any, outputs: any | undefined) => void;
    log?: (ctx: any, severity: any, message: string, urn: URN, streamId: number) => void;
    getRootResource?: (ctx: any) => { urn: string };
    setRootResource?: (ctx: any, urn: string) => void;
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
                            // If this is a dry-run, we will have the values of the original
                            // resource copied over as outputs.  Note: this should really
                            // only be done for values known to be stable.  This is tracked
                            // by: https://github.com/pulumi/pulumi/issues/1055
                            assert.deepEqual(res, {
                                otherIn: 777,
                                otherOut: runtime.unknownValue,
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
                        __pulumiAsset: true,
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
        // A program that allocates ten simple resources that use dependsOn to depend on one another, 10 different ways.
        "ten_depends_on_resources": {
            program: path.join(base, "008.ten_depends_on_resources"),
            expectResourceCount: 100,
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
                    id: "some-id",
                    urn: "some-urn",
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
                "myBag:A": "42",
                "myBag:bbbb": "a string o' b's",
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
        // A simple test of the read resource behavior.
        "read_resource": {
            program: path.join(base, "014.read_resource"),
            expectResourceCount: 0,
            readResource: (ctx: any, t: string, name: string, id: string, par: string, state: any) => {
                assert.strictEqual(t, "test:read:resource");
                assert.strictEqual(name, "foo");
                assert.strictEqual(id, "abc123");
                assert.deepEqual(state, {
                    a: "fizzz",
                    b: false,
                    c: [ 0.73, "x", { zed: 923 } ],
                });
                return {
                    urn: makeUrn(t, name),
                    props: {
                        b: true,
                        d: "and then, out of nowhere ...",
                    },
                };
            },
        },
        // Test that the runtime can be loaded twice.
        "runtime_sxs": {
            program: path.join(base, "015.runtime_sxs"),
            config: {
                "sxs:message": "SxS config works!",
            },
            expectResourceCount: 2,
            expectedLogs: {
                count: 2,
                ignoreDebug: true,
            },
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                return { urn: makeUrn(t, name), id: name, props: undefined };
            },
            log: (ctx: any, severity: number, message: string, urn: URN, streamId: number) => {
                assert.strictEqual(severity, engineproto.LogSeverity.INFO);
                assert.strictEqual(/logging via (.*) works/.test(message), true);
            },
        },
        // Test that leaked debuggable promises fail the deployment.
        "promise_leak": {
            program: path.join(base, "016.promise_leak"),
            expectError: "Program exited with non-zero exit code: 1",
        },
        // A test of parent default behaviors.
        "parent_defaults": {
            program: path.join(base, "017.parent_defaults"),
            expectResourceCount: 240,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any, dependencies?: string[],
                               custom?: boolean, protect?: boolean, parent?: string, provider?: string) => {

                if (custom && !t.startsWith("pulumi:providers:")) {
                    let expectProtect = false;
                    let expectProviderName = "";

                    const rpath = name.split("/");
                    for (let i = 1; i < rpath.length; i++) {
                        switch (rpath[i]) {
                        case "c0":
                        case "r0":
                            // Pass through parent values
                            break;
                        case "c1":
                        case "r1":
                            // Force protect to false
                            expectProtect = false;
                            break;
                        case "c2":
                        case "r2":
                            // Force protect to true
                            expectProtect = true;
                            break;
                        case "c3":
                        case "r3":
                            // Force provider
                            expectProviderName = `${rpath.slice(0, i).join("/")}-p`;
                            break;
                        default:
                            assert.fail(`unexpected path element in name: ${rpath[i]}`);
                        }
                    }

                    // r3 explicitly overrides its provider.
                    if (rpath[rpath.length-1] === "r3") {
                        expectProviderName = `${rpath.slice(0, rpath.length-1).join("/")}-p`;
                    }

                    const providerName = provider!.split("::").reduce((_, v) => v);

                    assert.strictEqual(`${name}.protect: ${protect!}`, `${name}.protect: ${expectProtect}`);
                    assert.strictEqual(`${name}.provider: ${providerName}`, `${name}.provider: ${expectProviderName}`);
                }

                return { urn: makeUrn(t, name), id: name, props: {} };
            },
        },
        "logging": {
            program: path.join(base, "018.logging"),
            expectResourceCount: 1,
            expectedLogs: {
                count: 5,
                ignoreDebug: true,
            },
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                // "test" is the one resource this test creates - save the URN so we can assert
                // it gets passed to log later on.
                if (name === "test") {
                    ctx.testUrn = makeUrn(t, name);
                }

                return { urn: makeUrn(t, name), id: name, props: res};
            },
            log: (ctx: any, severity: number, message: string, urn: URN, streamId: number) => {
                switch (message) {
                    case "info message":
                        assert.strictEqual(severity, engineproto.LogSeverity.INFO);
                        return;
                    case "warning message":
                        assert.strictEqual(severity, engineproto.LogSeverity.WARNING);
                        return;
                    case "error message":
                        assert.strictEqual(severity, engineproto.LogSeverity.ERROR);
                        return;
                    case "attached to resource":
                        assert.strictEqual(severity, engineproto.LogSeverity.INFO);
                        assert.strictEqual(urn, ctx.testUrn);
                        return;
                    case "with streamid":
                        assert.strictEqual(severity, engineproto.LogSeverity.INFO);
                        assert.strictEqual(urn, ctx.testUrn);
                        assert.strictEqual(streamId, 42);
                        return;
                    default:
                        assert.fail("unexpected message: " + message);
                        break;
                }
            },
        },
        // Test stack outputs via exports.
        "stack_exports": {
            program: path.join(base, "019.stack_exports"),
            expectResourceCount: 0,
            registerResourceOutputs: (ctx: any, dryrun: boolean, urn: URN,
                                      t: string, name: string, res: any, outputs: any | undefined) => {
                assert.strictEqual(t, "pulumi:pulumi:Stack");
                assert.strictEqual(outputs, {
                    a: {
                        x: 99,
                        y: "z",
                    },
                    b: 42,
                    c: {
                        d: "a",
                        e: false,
                    },
                });
            },
        },
        "root_resource": {
            program: path.join(base, "001.one_resource"),
            expectResourceCount: 2,
            showRootResourceRegistration: true,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any, deps: string[],
                               custom: boolean, protect: boolean, parent: string) => {
                if (t === "pulumi:pulumi:Stack") {
                    ctx.stackUrn = makeUrn(t, name);
                    return { urn: makeUrn(t, name), id: undefined, props: undefined };
                }

                assert.strictEqual(t, "test:index:MyResource");
                assert.strictEqual(name, "testResource1");
                assert.strictEqual(parent, ctx.stackUrn);
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        "backcompat_root_resource": {
            program: path.join(base, "001.one_resource"),
            expectResourceCount: 2,
            skipRootResourceEndpoints: true,
            showRootResourceRegistration: true,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any, deps: string[],
                               custom: boolean, protect: boolean, parent: string) => {
                if (t === "pulumi:pulumi:Stack") {
                    ctx.stackUrn = makeUrn(t, name);
                    return { urn: makeUrn(t, name), id: undefined, props: undefined };
                }

                assert.strictEqual(t, "test:index:MyResource");
                assert.strictEqual(name, "testResource1");
                assert.strictEqual(parent, ctx.stackUrn);
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        "property_dependencies": {
            program: path.join(base, "020.property_dependencies"),
            expectResourceCount: 5,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any, deps: string[],
                               custom: boolean, protect: boolean, parent: string, provider: string,
                               propertyDeps: any) => {

                assert.strictEqual(t, "test:index:MyResource");

                switch (name) {
                    case "resA":
                        assert.deepStrictEqual(deps, []);
                        assert.deepStrictEqual(propertyDeps, {});
                        break;
                    case "resB":
                        assert.deepStrictEqual(deps, [ "resA" ]);
                        assert.deepStrictEqual(propertyDeps, {});
                        break;
                    case "resC":
                        assert.deepStrictEqual(deps, [ "resA", "resB" ]);
                        assert.deepStrictEqual(propertyDeps, {
                            "propA": [ "resA" ],
                            "propB": [ "resB" ],
                            "propC": [],
                        });
                        break;
                    case "resD":
                        assert.deepStrictEqual(deps, [ "resA", "resB", "resC" ]);
                        assert.deepStrictEqual(propertyDeps, {
                            "propA": [ "resA", "resB" ],
                            "propB": [ "resC" ],
                            "propC": [],
                        });
                        break;
                    case "resE":
                        assert.deepStrictEqual(deps, [ "resA", "resB", "resC", "resD" ]);
                        assert.deepStrictEqual(propertyDeps, {
                            "propA": [ "resC" ],
                            "propB": [ "resA", "resB" ],
                            "propC": [],
                        });
                        break;
                    default:
                        break;
                }

                return { urn: name, id: undefined, props: { "outprop": "qux" } };
            },
        },
    };

    for (const casename of Object.keys(cases)) {
        if (casename !== "property_dependencies") {
            continue;
        }
        const opts: RunCase = cases[casename];
        it(`run test: ${casename} (pwd=${opts.pwd},prog=${opts.program})`, asyncTest(async () => {
            // For each test case, run it twice: first to preview and then to update.
            for (const dryrun of [ true, false ]) {
                console.log(dryrun ? "PREVIEW:" : "UPDATE:");

                // First we need to mock the resource monitor.
                const ctx: any = {};
                const regs: any = {};
                let rootResource: string | undefined;
                let regCnt = 0;
                let logCnt = 0;
                const monitor = createMockEngine(opts,
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
                    // ReadResource callback.
                    (call: any, callback: any) => {
                        const req: any = call.request;
                        const resp = new resproto.ReadResourceResponse();
                        if (opts.readResource) {
                            const t = req.getType();
                            const name = req.getName();
                            const id = req.getId();
                            const par = req.getParent();
                            const state = req.getProperties().toJavaScript();
                            const { urn, props } = opts.readResource(ctx, t, name, id, par, state);
                            resp.setUrn(urn);
                            resp.setProperties(gstruct.Struct.fromJavaScript(props));
                        }
                        callback(undefined, resp);
                    },
                    // RegisterResource callback
                    (call: any, callback: any) => {
                        const resp = new resproto.RegisterResourceResponse();
                        const req: any = call.request;
                        // Skip the automatically generated root component resource.
                        if (req.getType() !== runtime.rootPulumiStackTypeName || opts.showRootResourceRegistration) {
                            if (opts.registerResource) {
                                const t = req.getType();
                                const name = req.getName();
                                const res: any = req.getObject().toJavaScript();
                                const deps: string[] = req.getDependenciesList().sort();
                                const custom: boolean = req.getCustom();
                                const protect: boolean = req.getProtect();
                                const parent: string = req.getParent();
                                const provider: string = req.getProvider();
                                const propertyDeps: any = Array.from(req.getPropertydependenciesMap().entries())
                                    .reduce((o: any, [key, value]: [any, any]) => {
                                        return { ...o, [key]: value.getUrnsList().sort() };
                                    }, {});
                                const { urn, id, props } = opts.registerResource(ctx, dryrun, t, name, res, deps,
                                    custom, protect, parent, provider, propertyDeps);
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
                    // Log callback
                    (call: any, callback: any) => {
                        const req: any = call.request;
                        const severity = req.getSeverity();
                        const message = req.getMessage();
                        const urn = req.getUrn();
                        const streamId = req.getStreamid();
                        if (severity === engineproto.LogSeverity.ERROR) {
                            console.log("log: " + message);
                        }
                        if (opts.expectedLogs) {
                            if (!opts.expectedLogs.ignoreDebug || severity !== engineproto.LogSeverity.DEBUG) {
                                logCnt++;
                                if (opts.log) {
                                    opts.log(ctx, severity, message, urn, streamId);
                                }
                            }
                        }

                        callback(undefined, new gempty.Empty());
                    },
                    // GetRootResource callback
                    (call: any, callback: any) => {
                        let root: { urn: string };
                        if (opts.getRootResource) {
                            root = opts.getRootResource(ctx);
                        } else {
                            root = { urn: rootResource! };
                        }

                        const resp = new engineproto.GetRootResourceResponse();
                        resp.setUrn(root.urn);
                        callback(undefined, resp);
                    },
                    // SetRootResource callback
                    (call: any, callback: any) => {
                        const req: any = call.request;
                        const urn: string = req.getUrn();
                        if (opts.setRootResource) {
                            opts.setRootResource(ctx, urn);
                        } else {
                            rootResource = urn;
                        }

                        callback(undefined, new engineproto.SetRootResourceResponse());
                    },
                );

                // Next, go ahead and spawn a new language host that connects to said monitor.
                const langHost = serveLanguageHostProcess(monitor.addr);
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

                if (opts.expectedLogs) {
                    const logs = opts.expectedLogs;
                    if (logs.count) {
                        assert.strictEqual(logCnt, logs.count,
                            `Expected exactly ${logs.count} logs; got ${logCnt}`);
                    }
                }

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
            runReq.setProject(opts.project || "project");
            runReq.setStack(opts.stack || "stack");
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

// Despite the name, the "engine" RPC endpoint is only a logging endpoint. createMockEngine fires up a fake
// logging server so tests can assert that certain things get logged.
function createMockEngine(
        opts: RunCase,
        invokeCallback: (call: any, request: any) => any,
        readResourceCallback: (call: any, request: any) => any,
        registerResourceCallback: (call: any, request: any) => any,
        registerResourceOutputsCallback: (call: any, request: any) => any,
        logCallback: (call: any, request: any) => any,
        getRootResourceCallback: (call: any, request: any) => any,
        setRootResourceCallback: (call: any, request: any) => any): { server: any, addr: string } {
    // The resource monitor is hosted in the current process so it can record state, etc.
    const server = new grpc.Server();
    server.addService(resrpc.ResourceMonitorService, {
        invoke: invokeCallback,
        readResource: readResourceCallback,
        registerResource: registerResourceCallback,
        registerResourceOutputs: registerResourceOutputsCallback,
    });

    let engineImpl: Object = {
        log: logCallback,
    };

    if (!opts.skipRootResourceEndpoints) {
        engineImpl = {
            ... engineImpl,
            getRootResource: getRootResourceCallback,
            setRootResource: setRootResourceCallback,
        };
    }

    server.addService(enginerpc.EngineService, engineImpl);
    const port = server.bind("0.0.0.0:0", grpc.ServerCredentials.createInsecure());
    server.start();
    return { server: server, addr: `0.0.0.0:${port}` };
}

function serveLanguageHostProcess(engineAddr: string): { proc: childProcess.ChildProcess, addr: Promise<string> } {
    // A quick note about this:
    //
    // Normally, `pulumi-language-nodejs` launches `./node-modules/@pulumi/pulumi/cmd/run` which is responsible
    // for setting up some state and then running the actual user program.  However, in this case, we don't
    // have a folder structure like the above because we are seting the package as we've built it, not it installed
    // in another application.
    //
    // `pulumi-language-nodejs` allows us to set `PULUMI_LANGUAGE_NODEJS_RUN_PATH` in the environment, and when
    // set, it will use that path instead of the default value. For our tests here, we set it and point at the
    // just built version of run.
    process.env.PULUMI_LANGUAGE_NODEJS_RUN_PATH = "./bin/cmd/run";
    const proc = childProcess.spawn("pulumi-language-nodejs", [engineAddr]);

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

