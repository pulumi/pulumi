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
import { ComponentProvider } from "../../../provider/experimental/provider";
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
        // Define a stub for MyComponent because importing testdata directly will conflict on type versions.
        class MyComponent extends ComponentResource {
            constructor(name: string, args: any, opts?: ComponentResourceOptions) {
                super("provider:index:MyComponent", name, args, opts);
            }
        }

        // Set up a test provider with the simple-types test data
        const testDir = path.join(__dirname, "testdata", "provider-component");
        const provider = new ComponentProvider({
            components: [MyComponent],
            dirname: testDir,
        });

        // Call getSchema to generate the schema for this component
        const schemaResponse = await provider.getSchema();

        // Parse the returned schema
        const schema = JSON.parse(schemaResponse);

        // Basic schema validation
        assert.strictEqual(schema.name, "provider-component");

        // Verify the component definition is in the schema
        const componentType = "provider-component:index:MyComponent";
        assert.ok(schema.resources?.[componentType], "Component should be in schema resources");

        // Verify the component properties
        const component = schema.resources[componentType];
        assert.strictEqual(component.inputProperties.message.type, "string");
        assert.strictEqual(component.properties.formattedMessage.type, "string");
    });
});

describe("construct", function () {
    it("creates component instance with provided inputs", async function () {
        // Define a simple component with properties we can verify
        class TestComponent extends ComponentResource {
            public readonly messageBack: Output<string>;

            constructor(name: string, args: { message: Input<string> }, opts?: ComponentResourceOptions) {
                super("provider-component:index:TestComponent", name, args, opts);

                this.messageBack = Output.create(`Hello, ${args.message}!`);

                this.registerOutputs({
                    message: this.messageBack,
                });
            }
        }

        // Set up a test provider
        const testDir = path.join(__dirname, "testdata", "provider-component");
        const provider = new ComponentProvider({
            components: [TestComponent],
            dirname: testDir,
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
