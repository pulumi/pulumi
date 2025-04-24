// Copyright 2016-2021, Pulumi Corporation.
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
import {
    ComponentResource,
    CustomResource,
    DependencyResource,
    Inputs,
    Output,
    Resource,
    ResourceOptions,
    runtime,
    secret,
} from "../../index";
import * as state from "../../runtime/state";

import * as gstruct from "google-protobuf/google/protobuf/struct_pb";

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
                return new TestComponentResource(name, { urn });
            case "test:index:custom":
                return new TestCustomResource(name, type, { urn });
            default:
                throw new Error(`unknown resource type ${type}`);
        }
    }
}

class TestMocks implements runtime.Mocks {
    call(args: runtime.MockCallArgs): Record<string, any> {
        throw new Error(`unknown function ${args.token}`);
    }

    newResource(args: runtime.MockResourceArgs): { id: string | undefined; state: Record<string, any> } {
        switch (args.type) {
            case "test:index:component":
                return { id: undefined, state: {} };
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

// eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
const TestStrEnum = {
    Foo: "foo",
    Bar: "bar",
} as const;

// eslint-disable-next-line @typescript-eslint/no-redeclare
type TestStrEnum = (typeof TestStrEnum)[keyof typeof TestStrEnum];

// eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
const TestIntEnum = {
    One: 1,
    Zero: 0,
} as const;

// eslint-disable-next-line @typescript-eslint/no-redeclare
type TestIntEnum = (typeof TestIntEnum)[keyof typeof TestIntEnum];

// eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
const TestNumEnum = {
    One: 1.0,
    ZeroPointOne: 0.1,
} as const;

// eslint-disable-next-line @typescript-eslint/no-redeclare
type TestNumEnum = (typeof TestNumEnum)[keyof typeof TestNumEnum];

// eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
const TestBoolEnum = {
    One: true,
    Zero: false,
} as const;

// eslint-disable-next-line @typescript-eslint/no-redeclare
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
        describe("output values", () => {
            function* generateTests() {
                const testValues = [
                    { value: undefined, expected: null },
                    { value: null, expected: null },
                    { value: 0, expected: 0 },
                    { value: 1, expected: 1 },
                    { value: "", expected: "" },
                    { value: "hi", expected: "hi" },
                    { value: {}, expected: {} },
                    { value: [], expected: [] },
                ];
                for (const tv of testValues) {
                    for (const deps of [[], ["fakeURN1", "fakeURN2"]]) {
                        for (const isKnown of [true, false]) {
                            for (const isSecret of [true, false]) {
                                const resources = deps.map((dep) => new DependencyResource(dep));
                                yield {
                                    name:
                                        `Output(${JSON.stringify(deps)}, ${JSON.stringify(tv.value)}, ` +
                                        `isKnown=${isKnown}, isSecret=${isSecret})`,
                                    input: new Output(
                                        resources,
                                        Promise.resolve(tv.value),
                                        Promise.resolve(isKnown),
                                        Promise.resolve(isSecret),
                                        Promise.resolve([]),
                                    ),
                                    expected: {
                                        [runtime.specialSigKey]: runtime.specialOutputValueSig,
                                        ...(isKnown && { value: tv.expected }),
                                        ...(isSecret && { secret: isSecret }),
                                        ...(deps.length > 0 && { dependencies: deps }),
                                    },
                                    expectedRoundTrip: new Output(
                                        resources,
                                        Promise.resolve(isKnown ? tv.expected : undefined),
                                        Promise.resolve(isKnown),
                                        Promise.resolve(isSecret),
                                        Promise.resolve([]),
                                    ),
                                };
                            }
                        }
                    }
                }
            }

            async function assertOutputsEqual<T>(a: Output<T>, e: Output<T>) {
                async function urns(res: Set<Resource>): Promise<Set<string>> {
                    const result = new Set<string>();
                    for (const r of res) {
                        result.add(await r.urn.promise());
                    }
                    return result;
                }

                assert.deepStrictEqual(await urns(a.resources()), await urns(e.resources()));
                assert.deepStrictEqual(await a.isKnown, await e.isKnown);
                assert.deepStrictEqual(await a.promise(), await e.promise());
                assert.deepStrictEqual(await a.isSecret, await e.isSecret);
                assert.deepStrictEqual(await urns(await a.allResources!()), await urns(await e.allResources!()));
            }

            for (const test of generateTests()) {
                it(`marshals ${test.name} correctly`, async () => {
                    state.getStore().supportsOutputValues = true;

                    const inputs = { value: test.input };
                    const expected = { value: test.expected };

                    const actual = await runtime.serializeProperties("test", inputs, { keepOutputValues: true });
                    assert.deepStrictEqual(actual, expected);

                    // Roundtrip.
                    const back = runtime.deserializeProperties(gstruct.Struct.fromJavaScript(actual));
                    await assertOutputsEqual(back.value, test.expectedRoundTrip);
                });
            }
        });

        it("marshals basic properties correctly", async () => {
            const inputs: TestInputs = {
                aNum: 42,
                bStr: "a string",
                cUnd: undefined,
                dArr: Promise.resolve(["x", 42, Promise.resolve(true), Promise.resolve(undefined)]),
                id: "foo",
                urn: "bar",
                strEnum: TestStrEnum.Foo,
                intEnum: TestIntEnum.One,
                numEnum: TestNumEnum.One,
                boolEnum: TestBoolEnum.One,
            };
            // Serialize and then deserialize all the properties, checking that they round-trip as expected.
            const transfer = gstruct.Struct.fromJavaScript(await runtime.serializeProperties("test", inputs));
            const result = runtime.deserializeProperties(transfer);
            assert.strictEqual(result.aNum, 42);
            assert.strictEqual(result.bStr, "a string");
            assert.strictEqual(result.cUnd, undefined);
            assert.deepStrictEqual(result.dArr, ["x", 42, true, null]);
            assert.strictEqual(result.id, "foo");
            assert.strictEqual(result.urn, "bar");
            assert.strictEqual(result.strEnum, TestStrEnum.Foo);
            assert.strictEqual(result.intEnum, TestIntEnum.One);
            assert.strictEqual(result.numEnum, TestNumEnum.One);
            assert.strictEqual(result.boolEnum, TestBoolEnum.One);
        });
        it("marshals secrets correctly", async () => {
            const inputs: Inputs = {
                secret1: secret(1),
                secret2: secret(undefined),
            };

            // Serialize and then deserialize all the properties, checking that they round-trip as expected.
            state.getStore().supportsSecrets = true;
            let transfer = gstruct.Struct.fromJavaScript(await runtime.serializeProperties("test", inputs));
            let result = runtime.deserializeProperties(transfer);
            assert.ok(runtime.isRpcSecret(result.secret1));
            assert.ok(runtime.isRpcSecret(result.secret2));
            assert.strictEqual(runtime.unwrapRpcSecret(result.secret1), 1);
            assert.strictEqual(runtime.unwrapRpcSecret(result.secret2), null);

            // Serialize and then deserialize all the properties, checking that they round-trip as expected.
            state.getStore().supportsSecrets = false;
            transfer = gstruct.Struct.fromJavaScript(await runtime.serializeProperties("test", inputs));
            result = runtime.deserializeProperties(transfer);
            assert.ok(!runtime.isRpcSecret(result.secret1));
            assert.ok(!runtime.isRpcSecret(result.secret2));
            assert.strictEqual(result.secret1, 1);
            assert.strictEqual(result.secret2, undefined);
        });
        it("marshals resource references correctly during preview", async () => {
            runtime._setIsDryRun(true);
            runtime.setMocks(new TestMocks());

            const component = new TestComponentResource("test");
            const custom = new TestCustomResource("test");

            const componentURN = await component.urn.promise();
            const customURN = await custom.urn.promise();
            const customID = await custom.id.promise();

            const inputs: Inputs = {
                component: component,
                custom: custom,
            };

            state.getStore().supportsResourceReferences = true;

            let serialized = await runtime.serializeProperties("test", inputs);
            assert.deepEqual(serialized, {
                component: {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    urn: componentURN,
                },
                custom: {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    urn: customURN,
                    id: customID,
                },
            });

            state.getStore().supportsResourceReferences = false;
            serialized = await runtime.serializeProperties("test", inputs);
            assert.deepEqual(serialized, {
                component: componentURN,
                custom: customID ? customID : runtime.unknownValue,
            });
        });

        it("marshals resource references correctly during update", async () => {
            runtime.setMocks(new TestMocks());

            const component = new TestComponentResource("test");
            const custom = new TestCustomResource("test");

            const componentURN = await component.urn.promise();
            const customURN = await custom.urn.promise();
            const customID = await custom.id.promise();

            const inputs: Inputs = {
                component: component,
                custom: custom,
            };

            state.getStore().supportsResourceReferences = true;

            let serialized = await runtime.serializeProperties("test", inputs);
            assert.deepEqual(serialized, {
                component: {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    urn: componentURN,
                },
                custom: {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    urn: customURN,
                    id: customID,
                },
            });

            state.getStore().supportsResourceReferences = false;
            serialized = await runtime.serializeProperties("test", inputs);
            assert.deepEqual(serialized, {
                component: componentURN,
                custom: customID,
            });
        });

        describe("determines resource reference dependencies correctly", async () => {
            runtime.setMocks(new TestMocks());

            const custom1 = new TestCustomResource("custom1");
            const custom2 = new TestCustomResource("custom1");

            const inputs: Inputs = {
                resources: [custom1, custom2],
            };

            const tests = [
                {
                    supports: true,
                    exclude: true,
                    expected: [],
                },
                {
                    supports: true,
                    exclude: false,
                    expected: [custom1, custom2],
                },
                {
                    supports: false,
                    exclude: true,
                    expected: [custom1, custom2],
                },
                {
                    supports: false,
                    exclude: false,
                    expected: [custom1, custom2],
                },
            ];

            for (const test of tests) {
                it(`supportsResourceRefs=${test.supports}, excludeResourceRefsFromDeps=${test.exclude}`, async () => {
                    state.getStore().supportsResourceReferences = test.supports;
                    const [_, deps] = await runtime.serializePropertiesReturnDeps("test", inputs, {
                        excludeResourceReferencesFromDependencies: test.exclude,
                    });
                    assert.deepEqual(deps, new Map().set("resources", new Set(test.expected)));
                });
            }
        });
    });

    describe("deserializeProperty", () => {
        it("fails on unsupported secret values", () => {
            assert.throws(() =>
                runtime.deserializeProperty({
                    [runtime.specialSigKey]: runtime.specialSecretSig,
                }),
            );
        });
        it("fails on unknown signature keys", () => {
            assert.throws(() =>
                runtime.deserializeProperty({
                    [runtime.specialSigKey]: "foobar",
                }),
            );
        });
        it("pushed secretness up correctly", () => {
            const secretValue = {
                [runtime.specialSigKey]: runtime.specialSecretSig,
                value: "a secret value",
            };

            const props = gstruct.Struct.fromJavaScript({
                regular: "a normal value",
                list: ["a normal value", "another value", secretValue],
                map: { regular: "a normal value", secret: secretValue },
                mapWithList: {
                    regular: "a normal value",
                    list: ["a normal value", secretValue],
                },
                listWithMap: [
                    {
                        regular: "a normal value",
                        secret: secretValue,
                    },
                ],
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
        it("deserializes resource references properly during preview", async () => {
            runtime.setMocks(new TestMocks());
            state.getStore().supportsResourceReferences = true;
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
                component: {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    urn: componentURN,
                },
                custom: {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    urn: customURN,
                    id: customID,
                },
                unregistered: {
                    [runtime.specialSigKey]: runtime.specialResourceSig,
                    urn: unregisteredURN,
                    id: unregisteredID,
                },
            };

            const deserialized = runtime.deserializeProperty(outputs);
            assert.ok((<ComponentResource>deserialized["component"]).__pulumiComponentResource);
            assert.ok((<CustomResource>deserialized["custom"]).__pulumiCustomResource);
            assert.deepEqual(deserialized["unregistered"], unregisteredID);
        });
    });

    describe("resource error handling", () => {
        it("registerResource errors propagate appropriately", async () => {
            runtime.setMocks(new TestMocks());

            await assert.rejects(
                async () => {
                    const errResource = new TestErrorResource("test");
                    const customURN = await errResource.urn.promise();
                    const customID = await errResource.id.promise();
                },
                (err: Error) => {
                    const containsMessage = err.stack!.indexOf("this is an intentional error") >= 0;
                    const containsRegisterResource = err.stack!.indexOf("registerResource") >= 0;
                    return containsMessage && containsRegisterResource;
                },
            );
        });
    });
});
