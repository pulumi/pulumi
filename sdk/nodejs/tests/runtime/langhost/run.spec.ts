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

import * as grpc from "@grpc/grpc-js";

const enginerpc = require("../../../proto/engine_grpc_pb.js");
const engineproto = require("../../../proto/engine_pb.js");
const gempty = require("google-protobuf/google/protobuf/empty_pb.js");
const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
const langrpc = require("../../../proto/language_grpc_pb.js");
const langproto = require("../../../proto/language_pb.js");
const resrpc = require("../../../proto/resource_grpc_pb.js");
const resproto = require("../../../proto/resource_pb.js");
const providerproto = require("../../../proto/provider_pb.js");

interface RunCase {
    project?: string;
    stack?: string;
    pwd?: string;
    program?: string;
    args?: string[];
    config?: { [key: string]: any };
    expectError?: string;
    expectBail?: boolean;
    expectResourceCount?: number;
    expectedLogs?: {
        count?: number;
        ignoreDebug?: boolean;
    };
    skipRootResourceEndpoints?: boolean;
    showRootResourceRegistration?: boolean;
    invoke?: (ctx: any, tok: string, args: any, version: string, provider: string) => { failures: any, ret: any };
    readResource?: (ctx: any, t: string, name: string, id: string, par: string, state: any, version: string) => {
        urn: URN | undefined, props: any | undefined,
    };
    registerResource?: (ctx: any, dryrun: boolean, t: string, name: string, res: any, dependencies?: string[],
                        custom?: boolean, protect?: boolean, parent?: string, provider?: string,
                        propertyDeps?: any, ignoreChanges?: string[], version?: string, importID?: string,
                        replaceOnChanges?: string[]) => {
        urn: URN | undefined, id: ID | undefined, props: any | undefined,
    };
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
    beforeEach(() => {
        runtime._reset();
    });
    const base: string = path.join(path.dirname(__filename), "cases");
    const cases: { [key: string]: RunCase } = {
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
                assert.deepStrictEqual(res, {
                    inpropB1: false,
                    inpropB2: true,
                    inpropN: 42,
                    inpropS: "a string",
                    inpropA: [true, 99, "what a great property"],
                    inpropO: {
                        b1: false,
                        b2: true,
                        n: 42,
                        s: "another string",
                        a: [66, false, "strings galore"],
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
                assert.deepStrictEqual(res, {
                    inseq: seqnum,
                    inpropB1: false,
                    inpropB2: true,
                    inpropN: 42,
                    inpropS: "a string",
                    inpropA: [true, 99, "what a great property"],
                    inpropO: {
                        b1: false,
                        b2: true,
                        n: 42,
                        s: "another string",
                        a: [66, false, "strings galore"],
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
            registerResource: (ctx, dryrun, t, name, res, dependencies) => {
                let id: ID | undefined;
                let props: any | undefined;
                switch (t) {
                    case "test:index:ResourceA": {
                        assert.strictEqual(name, "resourceA");
                        assert.deepStrictEqual(res, { inprop: 777 });
                        if (!dryrun) {
                            id = name;
                            props = { outprop: "output yeah" };
                        }
                        break;
                    }
                    case "test:index:ResourceB": {
                        assert.strictEqual(name, "resourceB");
                        assert.deepStrictEqual(dependencies, ["test:index:ResourceA::resourceA"]);

                        if (dryrun) {
                            // If this is a dry-run, we will have no known values.
                            assert.deepStrictEqual(res, {
                                otherIn: runtime.unknownValue,
                                otherOut: runtime.unknownValue,
                            });
                        }
                        else {
                            // Otherwise, we will:
                            assert.deepStrictEqual(res, {
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
                assert.deepStrictEqual(res, {
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
                assert.deepStrictEqual(res, {
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
            invoke: (ctx: any, tok: string, args: any, version: string, provider: string) => {
                assert.strictEqual(provider, "");
                assert.strictEqual(tok, "invoke:index:echo");
                assert.deepStrictEqual(args, {
                    a: "hello",
                    b: true,
                    c: [0.99, 42, { z: "x" }],
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
            expectError: "",
            expectBail: true,
            expectedLogs: {
                count: 1,
                ignoreDebug: true,
            },
            log: (ctx: any, severity: any, message: string) => {
                if (severity === engineproto.LogSeverity.ERROR) {
                    if (message.indexOf("failed with an unhandled exception") < 0 &&
                        message.indexOf("es the dynamite") < 0) {

                        throw new Error("Unexpected error: " + message);
                    }
                }
            },
        },
        // A program that creates one resource that contains an assets archive.
        "assets_archive": {
            program: path.join(base, "012.assets_archive"),
            expectResourceCount: 1,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                assert.deepStrictEqual(res, {
                    "archive": {
                        "4dabf18193072939515e22adb298388d": "0def7320c3a5731c473e5ecbe6d01bc7",
                        "__pulumiArchive": true,
                        "assets": {
                            "archive": {
                                "4dabf18193072939515e22adb298388d": "0def7320c3a5731c473e5ecbe6d01bc7",
                                "__pulumiArchive": true,
                                "assets": {},
                            },
                            "asset": {
                                "4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
                                "__pulumiAsset": true,
                                "text": "foo",
                            },
                        },
                    },
                    "archiveP": {
                        "4dabf18193072939515e22adb298388d": "0def7320c3a5731c473e5ecbe6d01bc7",
                        "__pulumiArchive": true,
                        "assets": {
                            "foo": {
                                "4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
                                "__pulumiAsset": true,
                                "text": "bar",
                            },
                        },
                    },
                    "assetP": {
                        "4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
                        "__pulumiAsset": true,
                        "text": "baz",
                    },
                });
                return { urn: makeUrn(t, name), id: undefined, props: res };
            },
        },
        // A program that contains an unhandled promise rejection.
        "unhandled_promise_rejection": {
            program: path.join(base, "013.unhandled_promise_rejection"),
            expectResourceCount: 0,
            expectError: "",
            expectBail: true,
            expectedLogs: {
                count: 1,
                ignoreDebug: true,
            },
            log: (ctx: any, severity: any, message: string) => {
                if (severity === engineproto.LogSeverity.ERROR) {
                    if (message.indexOf("failed with an unhandled exception") < 0 &&
                        message.indexOf("es the dynamite") < 0) {

                        throw new Error("Unexpected error: " + message);
                    }
                }
            },
        },
        // A simple test of the read resource behavior.
        "read_resource": {
            program: path.join(base, "014.read_resource"),
            expectResourceCount: 0,
            readResource: (ctx: any, t: string, name: string, id: string, par: string, state: any) => {
                assert.strictEqual(t, "test:read:resource");
                assert.strictEqual(name, "foo");
                assert.strictEqual(id, "abc123");
                assert.deepStrictEqual(state, {
                    a: "fizzz",
                    b: false,
                    c: [0.73, "x", { zed: 923 }],
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
            registerResource: parentDefaultsRegisterResource,
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

                return { urn: makeUrn(t, name), id: name, props: res };
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
            expectResourceCount: 1,
            showRootResourceRegistration: true,
            registerResource: (ctx, dryrun, t, name, res, deps, custom, protect, parent) => {
                if (t === "pulumi:pulumi:Stack") {
                    ctx.stackUrn = makeUrn(t, name);
                    return { urn: makeUrn(t, name), id: undefined, props: undefined };
                }
                throw new Error();
            },
            registerResourceOutputs: (ctx: any, dryrun: boolean, urn: URN,
                                      t: string, name: string, res: any, outputs: any | undefined) => {
                assert.strictEqual(t, "pulumi:pulumi:Stack");
                assert.deepStrictEqual(outputs, {
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
            registerResource: (ctx, dryrun, t, name, res, deps, custom, protect, parent) => {
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
            registerResource: (ctx, dryrun, t, name, res, deps, custom, protect, parent) => {
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
            registerResource: (ctx, dryrun, t, name, res, deps, custom, protect, parent, provider, propertyDeps) => {
                assert.strictEqual(t, "test:index:MyResource");

                switch (name) {
                    case "resA":
                        assert.deepStrictEqual(deps, []);
                        assert.deepStrictEqual(propertyDeps, {});
                        break;
                    case "resB":
                        assert.deepStrictEqual(deps, ["resA"]);
                        assert.deepStrictEqual(propertyDeps, {});
                        break;
                    case "resC":
                        assert.deepStrictEqual(deps, ["resA", "resB"]);
                        assert.deepStrictEqual(propertyDeps, {
                            "propA": ["resA"],
                            "propB": ["resB"],
                            "propC": [],
                        });
                        break;
                    case "resD":
                        assert.deepStrictEqual(deps, ["resA", "resB", "resC"]);
                        assert.deepStrictEqual(propertyDeps, {
                            "propA": ["resA", "resB"],
                            "propB": ["resC"],
                            "propC": [],
                        });
                        break;
                    case "resE":
                        assert.deepStrictEqual(deps, ["resA", "resB", "resC", "resD"]);
                        assert.deepStrictEqual(propertyDeps, {
                            "propA": ["resC"],
                            "propB": ["resA", "resB"],
                            "propC": [],
                        });
                        break;
                    default:
                        break;
                }

                return { urn: name, id: undefined, props: { "outprop": "qux" } };
            },
        },
        "parent_child_dependencies": {
            pwd: path.join(base, "021.parent_child_dependencies"),
            program: "./index.js",
            expectResourceCount: 2,
            registerResource: (ctx, dryrun, t, name, res, deps) => {
                switch (name) {
                    case "cust1": assert.deepStrictEqual(deps, []); break;
                    case "cust2": assert.deepStrictEqual(deps, ["test:index:MyResource::cust1"]); break;
                    default: throw new Error("Didn't check: " + name);
                }
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        "parent_child_dependencies_2": {
            pwd: path.join(base, "022.parent_child_dependencies_2"),
            program: "./index.js",
            expectResourceCount: 3,
            registerResource: (ctx, dryrun, t, name, res, deps) => {
                switch (name) {
                    case "cust1": assert.deepStrictEqual(deps, []); break;
                    case "cust2": assert.deepStrictEqual(deps, ["test:index:MyResource::cust1"]); break;
                    case "cust3": assert.deepStrictEqual(deps, ["test:index:MyResource::cust1"]); break;
                    default: throw new Error("Didn't check: " + name);
                }
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        "parent_child_dependencies_3": {
            pwd: path.join(base, "023.parent_child_dependencies_3"),
            program: "./index.js",
            expectResourceCount: 1,
            expectError: "Program exited with non-zero exit code: 1",
        },
        "parent_child_dependencies_4": {
            pwd: path.join(base, "024.parent_child_dependencies_4"),
            program: "./index.js",
            expectResourceCount: 3,
            registerResource: (ctx, dryrun, t, name, res, deps) => {
                switch (name) {
                    case "cust1": assert.deepStrictEqual(deps, []); break;
                    case "cust2": assert.deepStrictEqual(deps, []); break;
                    case "comp1": assert.deepStrictEqual(deps, []); break;
                    default: throw new Error("Didn't check: " + name);
                }
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        "parent_child_dependencies_5": {
            pwd: path.join(base, "025.parent_child_dependencies_5"),
            program: "./index.js",
            expectResourceCount: 4,
            registerResource: (ctx, dryrun, t, name, res, deps) => {
                switch (name) {
                    case "cust1": assert.deepStrictEqual(deps, []); break;
                    case "cust2": assert.deepStrictEqual(deps, []); break;
                    case "comp1": assert.deepStrictEqual(deps, []); break;
                    case "res1": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust1", "test:index:MyCustomResource::cust2"]); break;
                    default: throw new Error("Didn't check: " + name);
                }
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        "parent_child_dependencies_6": {
            pwd: path.join(base, "026.parent_child_dependencies_6"),
            program: "./index.js",
            expectResourceCount: 6,
            registerResource: (ctx, dryrun, t, name, res, deps) => {
                switch (name) {
                    case "comp1": assert.deepStrictEqual(deps, []); break;
                    case "cust1": assert.deepStrictEqual(deps, []); break;
                    case "comp2": assert.deepStrictEqual(deps, []); break;
                    case "cust2": assert.deepStrictEqual(deps, []); break;
                    case "cust3": assert.deepStrictEqual(deps, []); break;
                    case "res1": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust1", "test:index:MyCustomResource::cust2", "test:index:MyCustomResource::cust3"]); break;
                    default: throw new Error("Didn't check: " + name);
                }
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        "parent_child_dependencies_7": {
            pwd: path.join(base, "027.parent_child_dependencies_7"),
            program: "./index.js",
            expectResourceCount: 10,
            registerResource: (ctx, dryrun, t, name, res, deps) => {
                switch (name) {
                    case "comp1": assert.deepStrictEqual(deps, []); break;
                    case "cust1": assert.deepStrictEqual(deps, []); break;
                    case "comp2": assert.deepStrictEqual(deps, []); break;
                    case "cust2": assert.deepStrictEqual(deps, []); break;
                    case "cust3": assert.deepStrictEqual(deps, []); break;
                    case "cust4": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust2"]); break;
                    case "res1": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust1", "test:index:MyCustomResource::cust2", "test:index:MyCustomResource::cust3"]); break;
                    case "res2": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust2", "test:index:MyCustomResource::cust3"]); break;
                    case "res3": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust2"]); break;
                    case "res4": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust4"]); break;
                    default: throw new Error("Didn't check: " + name);
                }
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        "parent_child_dependencies_8": {
            pwd: path.join(base, "028.parent_child_dependencies_8"),
            program: "./index.js",
            expectResourceCount: 6,
            registerResource: (ctx, dryrun, t, name, res, deps) => {
                switch (name) {
                    case "comp1": assert.deepStrictEqual(deps, []); break;
                    case "cust1": assert.deepStrictEqual(deps, []); break;
                    case "cust2": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust1"]); break;
                    case "res1": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust1"]); break;
                    case "res2": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust1"]); break;
                    case "res3": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust2"]); break;
                    default: throw new Error("Didn't check: " + name);
                }
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        "parent_child_dependencies_9": {
            pwd: path.join(base, "029.parent_child_dependencies_9"),
            program: "./index.js",
            expectResourceCount: 3,
            registerResource: (ctx, dryrun, t, name, res, deps) => {
                switch (name) {
                    case "cust1": assert.deepStrictEqual(deps, []); break;
                    case "cust2": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust1"]); break;
                    case "res1": assert.deepStrictEqual(deps, ["test:index:MyCustomResource::cust1"]); break;
                    default: throw new Error("Didn't check: " + name);
                }
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        "run_error": {
            program: path.join(base, "040.run_error"),
            expectResourceCount: 0,
            // We should get the error message saying that a message was reported and the
            // host should bail.
            expectBail: true,
        },
        "component_opt_single_provider": {
            program: path.join(base, "041.component_opt_single_provider"),
            expectResourceCount: 240,
            registerResource: parentDefaultsRegisterResource,
        },
        "component_opt_providers_array": {
            program: path.join(base, "042.component_opt_providers_array"),
            expectResourceCount: 240,
            registerResource: parentDefaultsRegisterResource,
        },
        "depends_on_non_resource": {
            program: path.join(base, "043.depends_on_non_resource"),
            expectResourceCount: 0,
            // We should get the error message saying that a message was reported and the
            // host should bail.
            expectBail: true,
            expectedLogs: {
                count: 1,
                ignoreDebug: true,
            },
            log: (ctx: any, severity: any, message: string) => {
                if (severity === engineproto.LogSeverity.ERROR) {
                    if (message.indexOf("'dependsOn' was passed a value that was not a Resource.") < 0) {
                        throw new Error("Unexpected error: " + message);
                    }
                }
            },
        },
        "ignore_changes": {
            program: path.join(base, "045.ignore_changes"),
            expectResourceCount: 1,
            registerResource: (
                ctx: any, dryrun: boolean, t: string, name: string, res: any, dependencies?: string[],
                custom?: boolean, protect?: boolean, parent?: string, provider?: string,
                propertyDeps?: any, ignoreChanges?: string[],
            ) => {
                if (name === "testResource") {
                    assert.deepStrictEqual(ignoreChanges, ["ignoredProperty"]);
                }
                return {
                    urn: makeUrn(t, name),
                    id: name,
                    props: {},
                };
            },
        },
        "versions": {
            program: path.join(base, "044.versions"),
            expectResourceCount: 3,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any, dependencies?: string[],
                               custom?: boolean, protect?: boolean, parent?: string, provider?: string,
                               propertyDeps?: any, ignoreChanges?: string[], version?: string, importID?: string,
                               replaceOnChanges?: string[],
                               ) => {
                switch (name) {
                    case "testResource":
                        assert.strictEqual("0.19.1", version);
                        break;
                    case "testResource2":
                        assert.strictEqual("0.19.2", version);
                        break;
                    case "testResource3":
                        assert.strictEqual("", version);
                        break;
                    default:
                        assert.fail(`unknown resource: ${name}`);
                }
                return {
                    urn: makeUrn(t, name),
                    id: name,
                    props: {},
                };
            },
            invoke: (ctx: any, tok: string, args: any, version: string) => {
                switch (tok) {
                    case "invoke:index:doit":
                        assert.strictEqual(version, "0.19.1");
                        break;
                    case "invoke:index:doit_v2":
                        assert.strictEqual(version, "0.19.2");
                        break;
                    case "invoke:index:doit_noversion":
                        assert.strictEqual(version, "");
                        break;
                    default:
                        assert.fail(`unknown invoke: ${tok}`);
                }

                return {
                    failures: [],
                    ret: args,
                };
            },
            readResource: (ctx: any, t: string, name: string, id: string, par: string, state: any, version: string) => {
                switch (name) {
                    case "foo":
                        assert.strictEqual(version, "0.20.0");
                        break;
                    case "foo_noversion":
                        assert.strictEqual(version, "");
                        break;
                    default:
                        assert.fail(`unknown read: ${name}`);
                }
                return {
                    urn: makeUrn(t, name),
                    props: state,
                };
            },
        },
        // A program that imports a single resource.
        "import_resource": {
            program: path.join(base, "030.import_resource"),
            expectResourceCount: 1,
            registerResource: (ctx, dryrun, t, name, res, deps, custom, protect, parent, provider, propertyDeps, ignoreChanges, version, importID) => {
                assert.strictEqual(t, "test:index:MyResource");
                assert.strictEqual(name, "testResource1");
                assert.strictEqual(importID, "testID");
                return { urn: makeUrn(t, name), id: importID, props: {} };
            },
        },
        // Test stack outputs via exports.
        "recursive_stack_exports": {
            program: path.join(base, "046.recursive_stack_exports"),
            expectResourceCount: 1,
            showRootResourceRegistration: true,
            registerResource: (ctx, dryrun, t, name, res, deps, custom, protect, parent) => {
                if (t === "pulumi:pulumi:Stack") {
                    ctx.stackUrn = makeUrn(t, name);
                    return { urn: makeUrn(t, name), id: undefined, props: undefined };
                }
                throw new Error();
            },
            registerResourceOutputs: (ctx: any, dryrun: boolean, urn: URN,
                                      t: string, name: string, res: any, outputs: any | undefined) => {
                assert.strictEqual(t, "pulumi:pulumi:Stack");
                assert.deepStrictEqual(outputs, {
                    "m": { "a": { "b": 1 } },
                    "n": { "a": { "b": 1 } },
                    "o": { "b": 1 },
                    "obj2": { "x": { "y": 1 } },
                    "obj2_x": { "y": 1 },
                    "obj2_x_y": 1,
                    "p": 1,
                });
            },
        },
        "exported_function": {
            program: path.join(base, "047.exported_function"),
            expectResourceCount: 1,
            showRootResourceRegistration: true,
            registerResource: (ctx, dryrun, t, name, res, deps, custom, protect, parent) => {
                if (t === "pulumi:pulumi:Stack") {
                    ctx.stackUrn = makeUrn(t, name);
                    return { urn: makeUrn(t, name), id: undefined, props: undefined };
                }
                throw new Error();
            },
            registerResourceOutputs: (ctx: any, dryrun: boolean, urn: URN,
                                      t: string, name: string, res: any, outputs: any | undefined) => {
                assert.strictEqual(t, "pulumi:pulumi:Stack");
                assert.deepStrictEqual(outputs, {
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
        "exported_promise_function": {
            program: path.join(base, "048.exported_promise_function"),
            expectResourceCount: 1,
            showRootResourceRegistration: true,
            registerResource: (ctx, dryrun, t, name, res, deps, custom, protect, parent) => {
                if (t === "pulumi:pulumi:Stack") {
                    ctx.stackUrn = makeUrn(t, name);
                    return { urn: makeUrn(t, name), id: undefined, props: undefined };
                }
                throw new Error();
            },
            registerResourceOutputs: (ctx: any, dryrun: boolean, urn: URN,
                                      t: string, name: string, res: any, outputs: any | undefined) => {
                assert.strictEqual(t, "pulumi:pulumi:Stack");
                assert.deepStrictEqual(outputs, {
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
        "exported_async_function": {
            program: path.join(base, "049.exported_async_function"),
            expectResourceCount: 1,
            showRootResourceRegistration: true,
            registerResource: (ctx, dryrun, t, name, res, deps, custom, protect, parent) => {
                if (t === "pulumi:pulumi:Stack") {
                    ctx.stackUrn = makeUrn(t, name);
                    return { urn: makeUrn(t, name), id: undefined, props: undefined };
                }
                throw new Error();
            },
            registerResourceOutputs: (ctx: any, dryrun: boolean, urn: URN,
                                      t: string, name: string, res: any, outputs: any | undefined) => {
                assert.strictEqual(t, "pulumi:pulumi:Stack");
                assert.deepStrictEqual(outputs, {
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
        "resource_creation_in_function": {
            program: path.join(base, "050.resource_creation_in_function"),
            expectResourceCount: 2,
            showRootResourceRegistration: true,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                if (t === "pulumi:pulumi:Stack") {
                    ctx.stackUrn = makeUrn(t, name);
                    return { urn: makeUrn(t, name), id: undefined, props: undefined };
                }
                assert.strictEqual(t, "test:index:MyResource");
                assert.strictEqual(name, "testResource1");
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
            registerResourceOutputs: (ctx: any, dryrun: boolean, urn: URN,
                                      t: string, name: string, res: any, outputs: any | undefined) => {
                assert.strictEqual(t, "pulumi:pulumi:Stack");
                assert.deepStrictEqual(outputs, {});
            },
        },
        "resource_creation_in_function_with_result": {
            program: path.join(base, "051.resource_creation_in_function_with_result"),
            expectResourceCount: 2,
            showRootResourceRegistration: true,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                if (t === "pulumi:pulumi:Stack") {
                    ctx.stackUrn = makeUrn(t, name);
                    return { urn: makeUrn(t, name), id: undefined, props: undefined };
                }
                assert.strictEqual(t, "test:index:MyResource");
                assert.strictEqual(name, "testResource1");
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
            registerResourceOutputs: (ctx: any, dryrun: boolean, urn: URN,
                                      t: string, name: string, res: any, outputs: any | undefined) => {
                assert.strictEqual(t, "pulumi:pulumi:Stack");
                assert.deepStrictEqual(outputs, { a: 1 });
            },
        },
        "resource_creation_in_async_function_with_result": {
            program: path.join(base, "052.resource_creation_in_async_function_with_result"),
            expectResourceCount: 2,
            showRootResourceRegistration: true,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                if (t === "pulumi:pulumi:Stack") {
                    ctx.stackUrn = makeUrn(t, name);
                    return { urn: makeUrn(t, name), id: undefined, props: undefined };
                }
                assert.strictEqual(t, "test:index:MyResource");
                assert.strictEqual(name, "testResource1");
                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
            registerResourceOutputs: (ctx: any, dryrun: boolean, urn: URN,
                                      t: string, name: string, res: any, outputs: any | undefined) => {
                assert.strictEqual(t, "pulumi:pulumi:Stack");
                assert.deepStrictEqual(outputs, { a: 1 });
            },
        },
        "provider_invokes": {
            program: path.join(base, "060.provider_invokes"),
            expectResourceCount: 1,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                return { urn: makeUrn(t, name), id: name === "p" ? "1" : undefined, props: undefined };
            },
            invoke: (ctx: any, tok: string, args: any, version: string, provider: string) => {
                assert.strictEqual(provider, "pulumi:providers:test::p::1");
                assert.strictEqual(tok, "test:index:echo");
                assert.deepStrictEqual(args, {
                    a: "hello",
                    b: true,
                    c: [0.99, 42, { z: "x" }],
                    id: "some-id",
                    urn: "some-urn",
                });
                return { failures: undefined, ret: args };
            },
        },
        "provider_in_parent_invokes": {
            program: path.join(base, "061.provider_in_parent_invokes"),
            expectResourceCount: 2,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any, dependencies?: string[],
                               custom?: boolean, protect?: boolean, parent?: string, provider?: string) => {
                return { urn: makeUrn(t, name), id: name === "p" ? "1" : undefined, props: undefined };
            },
            invoke: (ctx: any, tok: string, args: any, version: string, provider: string) => {
                assert.strictEqual(provider, "pulumi:providers:test::p::1");
                assert.strictEqual(tok, "test:index:echo");
                assert.deepStrictEqual(args, {
                    a: "hello",
                    b: true,
                    c: [0.99, 42, { z: "x" }],
                    id: "some-id",
                    urn: "some-urn",
                });
                return { failures: undefined, ret: args };
            },
        },
        "providerref_invokes": {
            program: path.join(base, "062.providerref_invokes"),
            expectResourceCount: 1,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                return { urn: makeUrn(t, name), id: name === "p" ? "1" : undefined, props: undefined };
            },
            invoke: (ctx: any, tok: string, args: any, version: string, provider: string) => {
                assert.strictEqual(provider, "pulumi:providers:test::p::1");
                assert.strictEqual(tok, "test:index:echo");
                assert.deepStrictEqual(args, {
                    a: "hello",
                    b: true,
                    c: [0.99, 42, { z: "x" }],
                    id: "some-id",
                    urn: "some-urn",
                });
                return { failures: undefined, ret: args };
            },
        },
        "providerref_in_parent_invokes": {
            program: path.join(base, "063.providerref_in_parent_invokes"),
            expectResourceCount: 2,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any, dependencies?: string[],
                               custom?: boolean, protect?: boolean, parent?: string, provider?: string) => {
                if (name === "c") {
                    assert.strictEqual(provider, "");
                }

                return { urn: makeUrn(t, name), id: name === "p" ? "1" : undefined, props: undefined };
            },
            invoke: (ctx: any, tok: string, args: any, version: string, provider: string) => {
                assert.strictEqual(provider, "pulumi:providers:test::p::1");
                assert.strictEqual(tok, "test:index:echo");
                assert.deepStrictEqual(args, {
                    a: "hello",
                    b: true,
                    c: [0.99, 42, { z: "x" }],
                    id: "some-id",
                    urn: "some-urn",
                });
                return { failures: undefined, ret: args };
            },
        },
        "async_components": {
            program: path.join(base, "064.async_components"),
            expectResourceCount: 5,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any, dependencies?: string[],
                               custom?: boolean, protect?: boolean, parent?: string, provider?: string) => {

                if (name === "c" || name === "d") {
                    dependencies = dependencies || [];
                    dependencies.sort();
                    // resources 'c' and 'd' should see resources 'a' and 'b' as dependencies (even
                    // though they are async constructed by the component)
                    assert.deepStrictEqual(dependencies, ["test:index:CustResource::a", "test:index:CustResource::b"]);
                }

                return { urn: makeUrn(t, name), id: undefined, props: undefined };
            },
        },
        // Create a resource with a large string to test grpcMaxMessageSize increase.
        "large_resource": {
            program: path.join(base, "065.large_resource"),
            expectResourceCount: 1,
            registerResource: (ctx: any, dryrun: boolean, t: string, name: string, res: any) => {
                const longString = "a".repeat(1024 * 1024 * 5);
                assert.strictEqual(t, "test:index:MyLargeStringResource");
                assert.strictEqual(name, "testResource1");
                assert.deepStrictEqual(res, { "largeStringProp": longString });
                return {
                    urn: makeUrn(t, name),
                    id: name,
                    props: {
                        "largeStringProp": "a".repeat(1024 * 1024 * 5),
                    },
                };
            },
        },
        "replace_on_changes": {
            program: path.join(base, "066.replace_on_changes"),
            expectResourceCount: 1,
            registerResource: (
                ctx: any, dryrun: boolean, t: string, name: string, res: any, dependencies?: string[],
                custom?: boolean, protect?: boolean, parent?: string, provider?: string,
                propertyDeps?: any, ignoreChanges?: string[], version?: string, importID?: string,
                replaceOnChanges?: string[],
            ) => {
                if (name === "testResource") {
                    assert.deepStrictEqual(replaceOnChanges, ["foo"]);
                }
                return {
                    urn: makeUrn(t, name),
                    id: name,
                    props: {},
                };
            },
        },
    };

    for (const casename of Object.keys(cases)) {
        // if (casename.indexOf("async_components") < 0) {
        //     continue;
        // }

        const opts: RunCase = cases[casename];
        it(`run test: ${casename} (pwd=${opts.pwd},prog=${opts.program})`, asyncTest(async () => {
            // For each test case, run it twice: first to preview and then to update.
            for (const dryrun of [true, false]) {
                // console.log(dryrun ? "PREVIEW:" : "UPDATE:");

                // First we need to mock the resource monitor.
                const ctx: any = {};
                const regs: any = {};
                let rootResource: string | undefined;
                let regCnt = 0;
                let logCnt = 0;
                const monitor = await createMockEngineAsync(opts,
                    // Invoke callback
                    (call: any, callback: any) => {
                        const resp = new providerproto.InvokeResponse();
                        if (opts.invoke) {
                            const req: any = call.request;
                            const args: any = req.getArgs().toJavaScript();
                            const version: string = req.getVersion();
                            const { failures, ret } =
                                opts.invoke(ctx, req.getTok(), args, version, req.getProvider());
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
                            const version = req.getVersion();
                            const { urn, props } = opts.readResource(ctx, t, name, id, par, state, version);
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
                                const ignoreChanges: string[] = req.getIgnorechangesList().sort();
                                const replaceOnChanges: string[] = req.getReplaceonchangesList().sort();
                                const propertyDeps: any = Array.from(req.getPropertydependenciesMap().entries())
                                    .reduce((o: any, [key, value]: any) => {
                                        return { ...o, [key]: value.getUrnsList().sort() };
                                    }, {});
                                const version: string = req.getVersion();
                                const importID: string = req.getImportid();
                                const { urn, id, props } = opts.registerResource(ctx, dryrun, t, name, res, deps,
                                    custom, protect, parent, provider, propertyDeps, ignoreChanges, version, importID, replaceOnChanges);
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
                    // SupportsFeature callback
                    (call: any, callback: any) => {
                        const resp = new resproto.SupportsFeatureResponse();
                        resp.setHassupport(false);
                        callback(undefined, resp);
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
                const [runError, runBail] = await mockRun(langHostClient, monitor.addr, opts, dryrun);

                // Validate that everything looks right.
                let expectError = opts.expectError;
                if (expectError === undefined) {
                    expectError = "";
                }
                assert.strictEqual(runError, expectError,
                    `Expected an error of "${expectError}"; got "${runError}"`);

                let expectBail = opts.expectBail;
                if (expectBail === undefined) {
                    expectBail = false;
                }
                assert.strictEqual(runBail, expectBail,
                    `Expected an 'bail' of "${expectBail}"; got "${runBail}"`);

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
                monitor.server.forceShutdown();
            }
        }));
    }
});

function parentDefaultsRegisterResource(
    ctx: any, dryrun: boolean, t: string, name: string, res: any, dependencies?: string[],
    custom?: boolean, protect?: boolean, parent?: string, provider?: string) {

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
        if (rpath[rpath.length - 1] === "r3") {
            expectProviderName = `${rpath.slice(0, rpath.length - 1).join("/")}-p`;
        }

        const providerName = provider!.split("::").reduce((_, v) => v);

        assert.strictEqual(`${name}.protect: ${protect!}`, `${name}.protect: ${expectProtect}`);
        assert.strictEqual(`${name}.provider: ${providerName}`, `${name}.provider: ${expectProviderName}`);
    }

    return { urn: makeUrn(t, name), id: name, props: {} };
}

function mockRun(langHostClient: any, monitor: string, opts: RunCase, dryrun: boolean): Promise<[string | undefined, boolean]> {
    return new Promise<[string | undefined, boolean]>(
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
                    resolve([res.getError(), res.getBail()]);
                }
            });
        },
    );
}

// Despite the name, the "engine" RPC endpoint is only a logging endpoint. createMockEngine fires up a fake
// logging server so tests can assert that certain things get logged.
async function createMockEngineAsync(
    opts: RunCase,
    invokeCallback: (call: any, request: any) => any,
    readResourceCallback: (call: any, request: any) => any,
    registerResourceCallback: (call: any, request: any) => any,
    registerResourceOutputsCallback: (call: any, request: any) => any,
    logCallback: (call: any, request: any) => any,
    getRootResourceCallback: (call: any, request: any) => any,
    setRootResourceCallback: (call: any, request: any) => any,
    supportsFeatureCallback: (call: any, request: any) => any) {
    // The resource monitor is hosted in the current process so it can record state, etc.
    const server = new grpc.Server({
        "grpc.max_receive_message_length": runtime.maxRPCMessageSize,
    });
    server.addService(resrpc.ResourceMonitorService, {
        supportsFeature: supportsFeatureCallback,
        invoke: invokeCallback,
        streamInvoke: () => {throw new Error("StreamInvoke not implemented in mock engine"); },
        readResource: readResourceCallback,
        registerResource: registerResourceCallback,
        registerResourceOutputs: registerResourceOutputsCallback,
    });

    let engineImpl: grpc.UntypedServiceImplementation = {
        log: logCallback,
    };

    if (!opts.skipRootResourceEndpoints) {
        engineImpl = {
            ...engineImpl,
            getRootResource: getRootResourceCallback,
            setRootResource: setRootResourceCallback,
        };
    }

    server.addService(enginerpc.EngineService, engineImpl);

    const port = await new Promise<number>((resolve, reject) => {
        server.bindAsync("0.0.0.0:0", grpc.ServerCredentials.createInsecure(), (err, p) => {
            if (err) {
                reject(err);
            } else {
                resolve(p);
            }
        });
    });

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
        } else {
            console.log(`langhost.stdout: ${dataString}`);
        }
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

