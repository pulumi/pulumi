// Copyright 2025-2025, Pulumi Corporation.
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
import * as path from "node:path";
import { ComponentProvider, getPulumiComponents } from "../../../provider/experimental/provider";
import { ComponentResource, ComponentResourceOptions } from "../../../resource";
import { Output, Input } from "../../../output";

describe("validateResourceType", function () {
    it("throws", function () {
        for (const resourceType of ["not-valid", "not:valid", "pkg:not-valid-module:type:", "pkg:index"]) {
            try {
                ComponentProvider.validateResourceType("a", resourceType);
                assert.fail("expected error");
            } catch (err) {
                // pass
            }
        }
    });

    it("accepts", function () {
        for (const resourceType of ["pkg:index:type", "pkg::type", "pkg:index:Type123"]) {
            ComponentProvider.validateResourceType("pkg", resourceType);
        }
    });
});

describe("getSchema", function () {
    it("generates schema for provider-component", async function () {
        // Set up a test provider with the simple-types test data
        const testDir = path.join(__dirname, "testdata", "provider-component");
        const { TestComponent } = require(testDir);
        const provider = new ComponentProvider({
            components: [TestComponent],
            dirname: testDir,
            name: "provider-component",
        });

        // Call getSchema to generate the schema for this component
        const schemaResponse = await provider.getSchema();

        // Parse the returned schema
        const schema = JSON.parse(schemaResponse);

        // Basic schema validation
        assert.strictEqual(schema.name, "provider-component");

        // Verify the component definition is in the schema
        const componentType = "provider-component:index:TestComponent";
        assert.ok(schema.resources?.[componentType], "Component should be in schema resources");

        // Verify the component properties
        const component = schema.resources[componentType];
        assert.strictEqual(component.inputProperties.message.type, "string");
        assert.strictEqual(component.properties.messageBack.type, "string");
        assert.strictEqual(component.properties.notAnOutput.type, "string");
    });
});

describe("construct", function () {
    it("creates component instance with provided inputs", async function () {
        // Set up a test provider
        const testDir = path.join(__dirname, "testdata", "provider-component");
        const { TestComponent } = require(testDir);
        const provider = new ComponentProvider({
            components: [TestComponent],
            dirname: testDir,
            name: "provider-component",
        });

        // Call construct to create the component
        const result = await provider.construct(
            "myInstance",
            "provider-component:index:TestComponent",
            { message: "world" },
            {},
        );

        // Verify the result
        assert.ok(result.urn, "URN should be defined");
        assert.ok(result.state, "State should be defined");

        // Verify the output properties in the state
        assert.ok(result.state.messageBack, "messageBack output should exist");
        assert.ok(result.state.notAnOutput, "notAnOutput should exist");
        assert.strictEqual(result.state.notAnOutput, "Hello, world!");

        // Resolve the output values to verify them
        const messageValue = await new Promise((resolve) => result.state.messageBack.apply((m: string) => resolve(m)));

        assert.strictEqual(messageValue, "Hello, world!");
    });

    it("throws for invalid component type", async function () {
        // Set up a test provider with no components
        const testDir = path.join(__dirname, "testdata", "provider-component");
        const provider = new ComponentProvider({
            components: [],
            dirname: testDir,
            name: "provider-component",
        });

        // Should throw for missing component class
        await assert.rejects(
            () => provider.construct("myInstance", "provider-component:index:MissingComponent", {}, {}),
            /Component class not found for 'MissingComponent'/,
        );

        // Should throw for invalid resource type format
        await assert.rejects(
            () => provider.construct("myInstance", "invalid-resource-type", {}, {}),
            /Invalid resource type/,
        );

        // Should throw for mismatched package name
        await assert.rejects(
            () => provider.construct("myInstance", "wrong-package:index:Component", {}, {}),
            /Invalid package name/,
        );
    });
});

describe("getPulumiComponents", () => {
    class TestComponent extends ComponentResource {
        constructor(name: string, args: any, opts: any) {
            super("test:index:TestComponent", name, args, opts);
        }
    }

    class TestComponent1 extends ComponentResource {
        constructor(name: string, args: any, opts: any) {
            super("test:index:TestComponent1", name, args, opts);
        }
    }

    class TestComponent2 extends ComponentResource {
        constructor(name: string, args: any, opts: any) {
            super("test:index:TestComponent2", name, args, opts);
        }
    }

    class RegularClass {}

    it("returns empty array for empty objects", () => {
        assert.deepStrictEqual(getPulumiComponents(null), []);
        assert.deepStrictEqual(getPulumiComponents(undefined), []);
        assert.deepStrictEqual(getPulumiComponents({}), []);
    });

    it("returns empty array for objects without component classes", () => {
        const exports = {
            foo: "string",
            bar: 123,
            baz: function () {
                return true;
            },
            regularClass: RegularClass,
        };
        assert.deepStrictEqual(getPulumiComponents(exports), []);
    });

    it("returns array with single component for single export", () => {
        const exports = {
            TestComponent,
        };

        const result = getPulumiComponents(exports);
        assert.strictEqual(result.length, 1);
        assert.strictEqual(result[0], TestComponent);
    });

    it("returns array with multiple components", () => {
        const exports = {
            TestComponent1,
            TestComponent2,
        };

        const result = getPulumiComponents(exports);
        assert.strictEqual(result.length, 2);
        assert.ok(result.includes(TestComponent1));
        assert.ok(result.includes(TestComponent2));
    });

    it("handles nested objects", () => {
        const exports = {
            nested: {
                deeper: {
                    component: TestComponent1,
                },
                another: TestComponent2,
            },
            top: TestComponent,
        };

        const result = getPulumiComponents(exports);
        assert.strictEqual(result.length, 3);
        assert.ok(result.includes(TestComponent));
        assert.ok(result.includes(TestComponent1));
        assert.ok(result.includes(TestComponent2));
    });

    it("handles nested arrays", () => {
        const exports = {
            components: [TestComponent1, [TestComponent2, [TestComponent]]],
        };

        const result = getPulumiComponents(exports);
        assert.strictEqual(result.length, 3);
        assert.ok(result.includes(TestComponent));
        assert.ok(result.includes(TestComponent1));
        assert.ok(result.includes(TestComponent2));
    });

    it("handles mixed nested structures", () => {
        const exports = {
            components: [{ comp: TestComponent1 }, [{ nested: TestComponent2 }], TestComponent],
            extra: {
                array: [TestComponent1],
            },
        };

        const result = getPulumiComponents(exports);
        assert.strictEqual(result.length, 3);
        assert.ok(result.includes(TestComponent));
        assert.ok(result.includes(TestComponent1));
        assert.ok(result.includes(TestComponent2));
    });

    it("handles direct component constructor input", () => {
        const result = getPulumiComponents(TestComponent);
        assert.strictEqual(result.length, 1);
        assert.strictEqual(result[0], TestComponent);
    });

    it("handles array input", () => {
        const result = getPulumiComponents([TestComponent1, TestComponent2, RegularClass]);
        assert.strictEqual(result.length, 2);
        assert.ok(result.includes(TestComponent1));
        assert.ok(result.includes(TestComponent2));
    });

    it("preserves order in flat structures", () => {
        const exports = {
            second: TestComponent2,
            first: TestComponent1,
        };

        const result = getPulumiComponents(exports);
        assert.strictEqual(result.length, 2);
        assert.strictEqual(result[0], TestComponent2);
        assert.strictEqual(result[1], TestComponent1);
    });

    it("preserves order in nested structures", () => {
        const exports = {
            nested: {
                second: TestComponent2,
            },
            first: TestComponent1,
            array: [TestComponent],
        };

        const result = getPulumiComponents(exports);
        assert.strictEqual(result.length, 3);
        assert.strictEqual(result[0], TestComponent2);
        assert.strictEqual(result[1], TestComponent1);
        assert.strictEqual(result[2], TestComponent);
    });

    it("handles non-component class constructors", () => {
        class NonComponent {
            public readonly value: number;
            constructor(value: number) {
                this.value = value;
            }
        }
        const result = getPulumiComponents(NonComponent);
        assert.strictEqual(result.length, 0);
    });

    it("handles edge cases", () => {
        assert.deepStrictEqual(getPulumiComponents(0), []);
        assert.deepStrictEqual(getPulumiComponents("string"), []);
        assert.deepStrictEqual(getPulumiComponents(true), []);
        assert.deepStrictEqual(getPulumiComponents(Symbol()), []);
        assert.deepStrictEqual(getPulumiComponents(new Date()), []);
        assert.deepStrictEqual(getPulumiComponents(/regex/), []);
        assert.deepStrictEqual(getPulumiComponents(new Map()), []);
        assert.deepStrictEqual(getPulumiComponents(new Set()), []);
    });

    it("deduplicates repeated components", () => {
        const exports = {
            first: TestComponent1,
            second: TestComponent1,
            nested: {
                another: TestComponent1,
                array: [TestComponent1],
            },
        };

        const result = getPulumiComponents(exports);
        assert.strictEqual(result.length, 1);
        assert.strictEqual(result[0], TestComponent1);
    });
});
