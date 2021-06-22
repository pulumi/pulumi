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
import { ComponentResource, CustomResource, Inputs, Resource, ResourceOptions, runtime, secret } from "../../index";
import { asyncTest } from "../util";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

class TestComponentResource extends ComponentResource {
    constructor(name: string, opts?: ResourceOptions) {
        super("test:index:component", name, {}, opts);

        super.registerOutputs({});
    }
}

class TestCustomResource extends CustomResource {
    constructor(name: string, type?: string, opts?: ResourceOptions) {
        super(type || "test:index:custom", name, {}, opts);
    }
}

class TestErrorResource extends CustomResource {
    constructor(name: string) {
        super("error", name, {});
    }
}

class TestResourceModule implements runtime.ResourceModule {
    construct(name: string, type: string, urn: string): Resource {
        switch (type) {
            case "test:index:component":
                return new TestComponentResource(name, {urn});
            case "test:index:custom":
                return new TestCustomResource(name, type, {urn});
            default:
                throw new Error(`unknown resource type ${type}`);
        }
    }
}

class TestMocks implements runtime.Mocks {
    call(args: runtime.MockCallArgs): Record<string, any> {
        throw new Error(`unknown function ${args.token}`);
    }

    newResource(args: runtime.MockResourceArgs): { id: string | undefined, state: Record<string, any> } {
        switch (args.type) {
            case "test:index:component":
                return {id: undefined, state: {}};
            case "test:index:custom":
            case "test2:index:custom":
                return {
                    id: runtime.isDryRun() ? undefined : "test-id",
                    state: {},
                };
            case "error":
                throw new Error("this is an intentional error");
            default:
                throw new Error(`unknown resource type ${args.type}`);
        }
    }
}

// tslint:disable-next-line:variable-name
const TestStrEnum = {
    Foo: "foo",
    Bar: "bar",
} as const;

type TestStrEnum = (typeof TestStrEnum)[keyof typeof TestStrEnum];

// tslint:disable-next-line:variable-name
const TestIntEnum = {
    One: 1,
    Zero: 0,
} as const;

type TestIntEnum = (typeof TestIntEnum)[keyof typeof TestIntEnum];

// tslint:disable-next-line:variable-name
const TestNumEnum = {
    One: 1.0,
    ZeroPointOne: 0.1,
} as const;

type TestNumEnum = (typeof TestNumEnum)[keyof typeof TestNumEnum];

// tslint:disable-next-line:variable-name
const TestBoolEnum = {
    One: true,
    Zero: false,
} as const;

type TestBoolEnum = (typeof TestBoolEnum)[keyof typeof TestBoolEnum];

interface TestInputs {
    aNum: number;
    bStr: string;
    cUnd: undefined;
    dArr: Promise<Array<any>>;
    id: string;
    urn: string;
    strEnum: TestStrEnum;
    intEnum: TestIntEnum;
    numEnum: TestNumEnum;
    boolEnum: TestBoolEnum;
}

describe("runtime", () => {
    beforeEach(() => {
        runtime._reset();
        runtime._resetResourcePackages();
        runtime._resetResourceModules();
    });

    describe("transferProperties", () => {
        it("marshals basic properties correctly", asyncTest(async () => {
            const inputs: TestInputs = {
                "aNum": 42,
                "bStr": "a string",
                "cUnd": undefined,
                "dArr": Promise.resolve([ "x", 42, Promise.resolve(true), Promise.resolve(undefined) ]),
                "id": "foo",
                "urn": "bar",
                "strEnum": TestStrEnum.Foo,
                "intEnum": TestIntEnum.One,
                "numEnum": TestNumEnum.One,
                "boolEnum": TestBoolEnum.One,
            };
            // Serialize and then deserialize all the properties, checking that they round-trip as expected.
            const transfer = gstruct.Struct.fromJavaScript(
                await runtime.serializeProperties("test", inputs));
            const result = runtime.deserializeProperties(transfer);
            assert.strictEqual(result.aNum, 42);
            assert.strictEqual(result.bStr, "a string");
            assert.strictEqual(result.cUnd, undefined);
            assert.deepStrictEqual(result.dArr, [ "x", 42, true, null ]);
            assert.strictEqual(result.id, "foo");
            assert.strictEqual(result.urn, "bar");
            assert.strictEqual(result.strEnum, TestStrEnum.Foo);
            assert.strictEqual(result.intEnum, TestIntEnum.One);
            assert.strictEqual(result.numEnum, TestNumEnum.One);
            assert.strictEqual(result.boolEnum, TestBoolEnum.One);
        }));
        it("marshals secrets correctly", asyncTest(async () => {
            runtime._setTestModeEnabled(true);

            const inputs: Inputs = {
                "secret1": secret(1),
                "secret2": secret(undefined),
            };

            // Serialize and then deserialize all the properties, checking that they round-trip as expected.
            runtime._setFeatureSupport("secrets", true);
            let transfer = gstruct.Struct.fromJavaScript(
                await runtime.serializeProperties("test", inputs));
            let result = runtime.deserializeProperties(transfer);
            assert.ok(runtime.isRpcSecret(result.secret1));
            assert.ok(runtime.isRpcSecret(result.secret2));
            assert.strictEqual(runtime.unwrapRpcSecret(result.secret1), 1);
            assert.strictEqual(runtime.unwrapRpcSecret(result.secret2), null);

            // Serialize and then deserialize all the properties, checking that they round-trip as expected.
            runtime._setFeatureSupport("secrets", false);
            transfer = gstruct.Struct.fromJavaScript(
                await runtime.serializeProperties("test", inputs));
            result = runtime.deserializeProperties(transfer);
            assert.ok(!runtime.isRpcSecret(result.secret1));
            assert.ok(!runtime.isRpcSecret(result.secret2));
            assert.strictEqual(result.secret1, 1);
            assert.strictEqual(result.secret2, undefined);
        }));
        it("marshals resource references correctly during preview", asyncTest(async () => {
            runtime._setIsDryRun(true);
            runtime.setMocks(new TestMocks());

            const component = new TestComponentResource("test");
            const custom = new TestCustomResource("test");

            const componentURN = await component.urn.promise();
            const customURN = await custom.urn.promise();
            const customID = await custom.id.promise();

            const inputs: Inputs = {
                "component": component,
                "custom": custom,
            };

            runtime._setFeatureSupport("resourceReferences", true);

            let serialized = await runtime.serializeProperties("test", inputs);
            assert.deepEqual(serialized, {
                "component": {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    "urn": componentURN,
                },
                "custom": {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    "urn": customURN,
                    "id": customID,
                },
            });

            runtime._setFeatureSupport("resourceReferences", false);
            serialized = await runtime.serializeProperties("test", inputs);
            assert.deepEqual(serialized, {
                "component": componentURN,
                "custom": customID ? customID : runtime.unknownValue,
            });
        }));

        it("marshals resource references correctly during update", asyncTest(async () => {
            runtime.setMocks(new TestMocks());

            const component = new TestComponentResource("test");
            const custom = new TestCustomResource("test");

            const componentURN = await component.urn.promise();
            const customURN = await custom.urn.promise();
            const customID = await custom.id.promise();

            const inputs: Inputs = {
                "component": component,
                "custom": custom,
            };

            runtime._setFeatureSupport("resourceReferences", true);

            let serialized = await runtime.serializeProperties("test", inputs);
            assert.deepEqual(serialized, {
                "component": {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    "urn": componentURN,
                },
                "custom": {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    "urn": customURN,
                    "id": customID,
                },
            });

            runtime._setFeatureSupport("resourceReferences", false);
            serialized = await runtime.serializeProperties("test", inputs);
            assert.deepEqual(serialized, {
                "component": componentURN,
                "custom": customID,
            });
        }));
    });

    describe("deserializeProperty", () => {
        it("fails on unsupported secret values", () => {
            assert.throws(() => runtime.deserializeProperty({
                [runtime.specialSigKey]: runtime.specialSecretSig,
            }));
        });
        it("fails on unknown signature keys", () => {
            assert.throws(() => runtime.deserializeProperty({
                [runtime.specialSigKey]: "foobar",
            }));
        });
        it("pushed secretness up correctly", () => {
            const secretValue = {
                [runtime.specialSigKey]: runtime.specialSecretSig,
                "value": "a secret value",
            };

            const props = gstruct.Struct.fromJavaScript({
                "regular": "a normal value",
                "list": [ "a normal value", "another value", secretValue ],
                "map":  { "regular": "a normal value", "secret": secretValue },
                "mapWithList": {
                    "regular": "a normal value",
                    "list": [ "a normal value", secretValue ],
                },
                "listWithMap": [{
                    "regular": "a normal value",
                    "secret": secretValue,
                }],
            });

            const result = runtime.deserializeProperties(props);

            // Regular had no secrets in it, so it is returned as is.
            assert.strictEqual(result.regular, "a normal value");

            // One of the elements in the list was a secret, so the secretness is promoted to top level.
            assert.strictEqual(result.list[runtime.specialSigKey], runtime.specialSecretSig);
            assert.strictEqual(result.list.value[0], "a normal value");
            assert.strictEqual(result.list.value[1], "another value");
            assert.strictEqual(result.list.value[2], "a secret value");

            // One of the values of the map was a secret, so the secretness is promoted to top level.
            assert.strictEqual(result.map[runtime.specialSigKey], runtime.specialSecretSig);
            assert.strictEqual(result.map.value.regular, "a normal value");
            assert.strictEqual(result.map.value.secret, "a secret value");

            // The nested map had a secret in one of the values, so the entire thing becomes a secret.
            assert.strictEqual(result.mapWithList[runtime.specialSigKey], runtime.specialSecretSig);
            assert.strictEqual(result.mapWithList.value.regular, "a normal value");
            assert.strictEqual(result.mapWithList.value.list[0], "a normal value");
            assert.strictEqual(result.mapWithList.value.list[1], "a secret value");

            // An array element contained a secret (via a nested map), so the entrie array becomes a secret.
            assert.strictEqual(result.listWithMap[runtime.specialSigKey], runtime.specialSecretSig);
            assert.strictEqual(result.listWithMap.value[0].regular, "a normal value");
            assert.strictEqual(result.listWithMap.value[0].secret, "a secret value");
        });
        it("deserializes resource references properly during preview", asyncTest(async () => {
            runtime.setMocks(new TestMocks());
            runtime._setFeatureSupport("resourceReferences", true);
            runtime.registerResourceModule("test", "index", new TestResourceModule());

            const component = new TestComponentResource("test");
            const custom = new TestCustomResource("test");
            const unregistered = new TestCustomResource("test", "test2:index:custom");

            const componentURN = await component.urn.promise();
            const customURN = await custom.urn.promise();
            const customID = await custom.id.promise();
            const unregisteredURN = await unregistered.urn.promise();
            const unregisteredID = await unregistered.id.promise();

            const outputs = {
                "component": {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    "urn": componentURN,
                },
                "custom": {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    "urn": customURN,
                    "id": customID,
                },
                "unregistered": {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    "urn": unregisteredURN,
                    "id": unregisteredID,
                },
            };

            const deserialized = runtime.deserializeProperty(outputs);
            assert.ok((<ComponentResource>deserialized["component"]).__pulumiComponentResource);
            assert.ok((<CustomResource>deserialized["custom"]).__pulumiCustomResource);
            assert.deepEqual(deserialized["unregistered"], unregisteredID);
        }));
    });

    describe("resource error handling", () => {
        it("registerResource errors propagate appropriately", asyncTest(async () => {
            runtime.setMocks(new TestMocks());

            await assert.rejects(async () => {
                const errResource = new TestErrorResource("test");
                const customURN = await errResource.urn.promise();
                const customID = await errResource.id.promise();
            }, (err: Error) => {
                const containsMessage = err.stack!.indexOf("this is an intentional error") >= 0;
                const containsRegisterResource = err.stack!.indexOf("registerResource") >= 0;
                return containsMessage && containsRegisterResource;
            });
        }));
    });
});
