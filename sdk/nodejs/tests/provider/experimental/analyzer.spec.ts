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

import * as assert from "node:assert";
import * as path from "node:path";
import * as execa from "execa";
import { Analyzer } from "../../../provider/experimental/analyzer";
import { InputOutput } from "../../../provider/experimental/analyzer";

const packageJSON = { name: "@my-namespace/provider" };

describe("Analyzer", function () {
    before(() => {
        // We need to link in the pulumi package to the testdata directories so
        // that the analyzer can find it and determine pulumi types like
        // ComponentResource or Output.
        // We have a .yarnrc at the repo root that sets a mutex to prevent
        // concurrent yarn installs. This avoids issues in integration tests.
        // However, for these tests we want to run inside yarn, which causes a
        // deadlock. Passing --no-default-rc makes yarn ignore the .yarnrc.
        // There are no issues here with concurrent yarn runs.
        const dir = path.join(__dirname, "testdata");
        execa.sync("yarn", ["install", "--no-default-rc", "--non-interactive"], { cwd: dir });
        execa.sync("yarn", ["link", "@pulumi/pulumi", "--no-default-rc", "--non-interactive"], { cwd: dir });
    });

    after(() => {
        const dir = path.join(__dirname, "testdata");
        execa.sync("yarn", ["unlink", "@pulumi/pulumi", "--no-default-rc", "--non-interactive"], { cwd: dir });
    });

    it("infers simple types", async function () {
        const dir = path.join(__dirname, "testdata", "simple-types");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    aNumber: { type: "number", plain: true },
                    aString: { type: "string", plain: true },
                    aBoolean: { type: "boolean", plain: true },
                },
                outputs: {
                    outNumber: { type: "number" },
                    outString: { type: "string" },
                    outStringNotOutput: { type: "string" },
                    outBoolean: { type: "boolean" },
                },
            },
        });
    });

    it("infers optional types", async function () {
        const dir = path.join(__dirname, "testdata", "optional-types");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    optionalNumber: { type: "number", optional: true, plain: true },
                    optionalNumberType: { type: "number", optional: true, plain: true },
                    optionalBoolean: { type: "boolean", optional: true, plain: true },
                    optionalBooleanType: { type: "boolean", optional: true, plain: true },
                },
                outputs: {
                    optionalOutputNumber: { type: "number", optional: true },
                    optionalOutputType: { type: "number", optional: true },
                    optionalOutputBoolean: { type: "boolean", optional: true },
                    optionalOutputBooleanType: { type: "boolean", optional: true },
                },
            },
        });
    });

    it("infers input types", async function () {
        const dir = path.join(__dirname, "testdata", "input-types");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    aNumber: { type: "number" },
                    anOptionalString: { type: "string", optional: true },
                },
                outputs: {},
            },
        });
    });

    it("infers complex types", async function () {
        const dir = path.join(__dirname, "testdata", "complex-types");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components, typeDefinitions } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    anInterfaceType: { $ref: "#/types/provider:index:MyInterfaceType", plain: true },
                    aClassType: { $ref: "#/types/provider:index:MyClassType", plain: true },
                },
                outputs: {
                    anInterfaceTypeOutput: { $ref: "#/types/provider:index:MyInterfaceType" },
                    aClassTypeOutput: { $ref: "#/types/provider:index:MyClassType" },
                },
            },
        });
        assert.deepStrictEqual(typeDefinitions, {
            MyInterfaceType: {
                name: "MyInterfaceType",
                properties: { aNumber: { type: "number", plain: true } },
                type: "object",
            },
            MyClassType: {
                name: "MyClassType",
                properties: { aString: { type: "string", plain: true } },
                type: "object",
            },
        });
    });

    it("infers self recursive complex types", async function () {
        const dir = path.join(__dirname, "testdata", "recursive-complex-types");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components, typeDefinitions } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    theSelfRecursiveTypeInput: { $ref: "#/types/provider:index:SelfRecursive" },
                },
                outputs: {
                    theSelfRecursiveTypeOutput: { $ref: "#/types/provider:index:SelfRecursiveComponentOutput" },
                },
            },
        });
        assert.deepStrictEqual(typeDefinitions, {
            SelfRecursive: {
                name: "SelfRecursive",
                properties: { self: { $ref: "#/types/provider:index:SelfRecursive", plain: true } },
                type: "object",
            },
            SelfRecursiveComponentOutput: {
                name: "SelfRecursiveComponentOutput",
                properties: { self: { $ref: "#/types/provider:index:SelfRecursiveComponentOutput" } },
                type: "object",
            },
        });
    });

    it("infers mutually recursive complex types", async function () {
        const dir = path.join(__dirname, "testdata", "mutually-recursive-types");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components, typeDefinitions } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    typeAInput: { $ref: "#/types/provider:index:TypeA" },
                },
                outputs: {},
            },
        });
        assert.deepStrictEqual(typeDefinitions, {
            TypeA: {
                name: "TypeA",
                properties: { b: { $ref: "#/types/provider:index:TypeB", plain: true } },
                type: "object",
            },
            TypeB: {
                name: "TypeB",
                properties: { a: { $ref: "#/types/provider:index:TypeA", plain: true } },
                type: "object",
            },
        });
    });

    it("rejects bad args", async function () {
        const dir = path.join(__dirname, "testdata", "bad-args");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        assert.throws(
            () => analyzer.analyze(),
            /Error: Component 'MyComponent' constructor 'args' parameter must be an interface/,
        );
    });

    it("infers array types", async function () {
        const dir = path.join(__dirname, "testdata", "array-types");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    anArrayOfStrings: { type: "array", items: { type: "string", plain: true }, plain: true },
                    anArrayOfNumbers: {
                        type: "array",
                        items: { type: "number", plain: true },
                        optional: true,
                        plain: true,
                    },
                    anArrayOfBooleans: { type: "array", items: { type: "boolean", plain: true }, plain: true },
                    inputArrayOfStrings: { type: "array", items: { type: "string", plain: true } },
                    inputArrayOfNumbers: { type: "array", items: { type: "number", plain: true }, optional: true },
                    inputArrayOfBooleans: { type: "array", items: { type: "boolean", plain: true } },
                    inputOfInputStrings: { type: "array", items: { type: "string" } },
                    inputOfInputNumbers: { type: "array", items: { type: "number" }, optional: true },
                    inputOfInputBooleans: { type: "array", items: { type: "boolean" } },
                    anArrayOfInputStrings: { type: "array", items: { type: "string" }, plain: true },
                    anArrayOfInputNumbers: { type: "array", items: { type: "number" }, optional: true, plain: true },
                    anArrayOfInputBooleans: { type: "array", items: { type: "boolean" }, plain: true },
                    aListOfStrings: { type: "array", items: { type: "string", plain: true }, plain: true },
                    aListOfNumbers: { type: "array", items: { type: "number", plain: true }, plain: true },
                    aListOfBooleans: {
                        type: "array",
                        items: { type: "boolean", plain: true },
                        optional: true,
                        plain: true,
                    },
                },
                outputs: {
                    outArrayOfStrings: { type: "array", items: { type: "string" } },
                    outArrayOfNumbers: { type: "array", items: { type: "number" } },
                    outArrayOfBooleans: { type: "array", items: { type: "boolean" } },
                },
            },
        });
    });

    it("infers map types", async function () {
        const dir = path.join(__dirname, "testdata", "map-types");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    aMapOfStrings: {
                        type: "object",
                        additionalProperties: { type: "string", plain: true },
                        plain: true,
                    },
                    aMapOfNumbers: {
                        type: "object",
                        additionalProperties: { type: "number", plain: true },
                        plain: true,
                    },
                    aMapOfBooleans: {
                        type: "object",
                        additionalProperties: { type: "boolean", plain: true },
                        optional: true,
                        plain: true,
                    },
                    mapOfStringInputs: { type: "object", additionalProperties: { type: "string" }, plain: true },
                    mapOfNumberInputs: { type: "object", additionalProperties: { type: "number" }, plain: true },
                    mapOfBooleanInputs: { type: "object", additionalProperties: { type: "boolean" }, plain: true },
                    inputMapOfStringInputs: { type: "object", additionalProperties: { type: "string" } },
                    inputMapOfNumberInputs: { type: "object", additionalProperties: { type: "number" } },
                    inputMapOfBooleanInputs: { type: "object", additionalProperties: { type: "boolean" } },
                },
                outputs: {
                    outMapOfStrings: { type: "object", additionalProperties: { type: "string" } },
                    outMapOfNumbers: { type: "object", additionalProperties: { type: "number" } },
                    outMapOfBooleans: { type: "object", additionalProperties: { type: "boolean" } },
                    outMapOptionalStrings: { type: "object", additionalProperties: { type: "string" }, optional: true },
                },
            },
        });
    });

    it("infers any type", async function () {
        const dir = path.join(__dirname, "testdata", "any-type");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    anAny: { $ref: "pulumi.json#/Any" },
                    anyInput: { $ref: "pulumi.json#/Any" },
                },
                outputs: {
                    outAny: { $ref: "pulumi.json#/Any" },
                },
            },
        });
    });

    it("infers asset/archive types", async function () {
        const dir = path.join(__dirname, "testdata", "asset-archive-types");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    anAsset: { $ref: "pulumi.json#/Asset", plain: true },
                    anArchive: { $ref: "pulumi.json#/Archive", plain: true },
                    inputAsset: { $ref: "pulumi.json#/Asset" },
                    inputArchive: { $ref: "pulumi.json#/Archive" },
                },
                outputs: {
                    outAsset: { $ref: "pulumi.json#/Asset" },
                    outArchive: { $ref: "pulumi.json#/Archive" },
                },
            },
        });
    });

    it("infers resource references", async function () {
        const dir = path.join(__dirname, "testdata", "resource-reference");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components, dependencies } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    inputResource: {
                        $ref: "/tls/v4.11.1/schema.json#/resources/tls:index%2FprivateKey:PrivateKey",
                        description: "A reference to a resource in the TLS package.",
                    },
                    inputPlainResource: {
                        $ref: "/tls/v4.11.1/schema.json#/resources/tls:index%2FprivateKey:PrivateKey",
                        plain: true,
                        optional: true,
                    },
                    inputResourceOrUndefined: {
                        $ref: "/tls/v4.11.1/schema.json#/resources/tls:index%2FprivateKey:PrivateKey",
                        optional: true,
                    },
                },
                outputs: {
                    outputResource: { $ref: "/tls/v4.11.1/schema.json#/resources/tls:index%2FprivateKey:PrivateKey" },
                    outputPlainResource: {
                        $ref: "/tls/v4.11.1/schema.json#/resources/tls:index%2FprivateKey:PrivateKey",
                    },
                    outputResourceOrUndefined: {
                        $ref: "/tls/v4.11.1/schema.json#/resources/tls:index%2FprivateKey:PrivateKey",
                        optional: true,
                    },
                    outputParameterized: {
                        $ref: "/terraform-provider/v0.10.0/schema.json#/resources/netlify:index%2FdeployKey:DeployKey",
                    },
                },
            },
        });
        assert.deepStrictEqual(
            new Set(dependencies),
            new Set([
                {
                    name: "tls",
                    version: "4.11.1",
                },
                {
                    name: "terraform-provider",
                    version: "0.10.0",
                    parameterization: {
                        name: "netlify",
                        value: "eyJyZW1vdGUiOnsidXJsIjoicmVnaXN0cnkub3BlbnRvZnUub3JnL25ldGxpZnkvbmV0bGlmeSIsInZlcnNpb24iOiIwLjIuMiJ9fQ==",
                        version: "0.2.2",
                    },
                },
            ]),
        );
    });

    it("errors nicely for invalid property types for top-level properties", async function () {
        const dir = path.join(__dirname, "testdata", "bad-property-type", "top-level");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        assert.throws(
            () => analyzer.analyze(),
            (err) =>
                err.message ===
                "Union types are not supported for component 'MyComponent' input 'MyComponentArgs.invalidProp': type 'string | boolean'",
        );
    });

    it("errors nicely for invalid property types for sub-type properties", async function () {
        const dir = path.join(__dirname, "testdata", "bad-property-type", "sub-type");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        assert.throws(
            () => analyzer.analyze(),
            (err) =>
                err.message ===
                "Unsupported type for component 'MyComponent' input 'MyOtherArgs.invalidProp values': type '\"fixed value\"'",
        );
    });

    it("infers component description", async function () {
        const dir = path.join(__dirname, "testdata", "component-description");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components, typeDefinitions } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                description: "This is a description of MyComponent\nIt can span multiple lines",
                inputs: {
                    anInterfaceType: {
                        $ref: "#/types/provider:index:MyInterfaceType",
                        plain: true,
                        description: "anInterfaceType doc comment",
                    },
                    aClassType: {
                        $ref: "#/types/provider:index:MyClassType",
                        plain: true,
                        description: "aClassType comment",
                    },
                    inputMapOfInterfaceTypes: {
                        type: "object",
                        additionalProperties: { $ref: "#/types/provider:index:MyInterfaceType" },
                        description: "inputMap comment",
                    },
                    anArchive: {
                        $ref: "pulumi.json#/Archive",
                        plain: true,
                        description: "anArchive comment",
                    },
                    anAsset: {
                        $ref: "pulumi.json#/Asset",
                        plain: true,
                        description: "anAsset comment",
                    },
                    anArray: {
                        description: "anArray comment",
                        items: {
                            plain: true,
                            type: "string",
                        },
                        plain: true,
                        type: "array",
                    },
                },
                outputs: {
                    outStringMap: {
                        type: "object",
                        additionalProperties: { type: "number" },
                        description: "out_string_map comment",
                    },
                },
            },
        });
        assert.deepStrictEqual(typeDefinitions, {
            MyInterfaceType: {
                name: "MyInterfaceType",
                properties: { aNumber: { type: "number", plain: true, description: "aNumber comment" } },
                description: "myInterfaceType comment",
                type: "object",
            },
            MyClassType: {
                name: "MyClassType",
                properties: { aString: { type: "string", plain: true, description: "aString comment" } },
                description: "myClassType comment",
                type: "object",
            },
        });
    });

    it("finds entry point from package.json exports string", async function () {
        const dir = path.join(__dirname, "testdata", "uncommon-main");
        const analyzer = new Analyzer(
            dir,
            "provider",
            { name: "provider", exports: "./src/mymain.js" },
            new Set(["MainComponent"]),
        );
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MainComponent: {
                name: "MainComponent",
                inputs: {
                    message: { type: "string", plain: true },
                },
                outputs: {
                    formattedMessage: { type: "string" },
                },
            },
        });
    });

    it("finds entry point from package.json exports object with dot", async function () {
        const dir = path.join(__dirname, "testdata", "uncommon-main");
        const analyzer = new Analyzer(
            dir,
            "provider",
            {
                name: "provider",
                exports: {
                    ".": "./src/mymain.js",
                },
            },
            new Set(["MainComponent"]),
        );
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MainComponent: {
                name: "MainComponent",
                inputs: {
                    message: { type: "string", plain: true },
                },
                outputs: {
                    formattedMessage: { type: "string" },
                },
            },
        });
    });

    it("finds entry point from package.json exports with conditions", async function () {
        const dir = path.join(__dirname, "testdata", "uncommon-main");
        const analyzer = new Analyzer(
            dir,
            "provider",
            {
                name: "provider",
                exports: {
                    ".": {
                        default: "./src/mymain.ts",
                        require: "./lib/index.js", // These won't exist but that's ok since we use default
                        import: "./lib/index.mjs",
                    },
                },
            },
            new Set(["MainComponent"]),
        );
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MainComponent: {
                name: "MainComponent",
                inputs: {
                    message: { type: "string", plain: true },
                },
                outputs: {
                    formattedMessage: { type: "string" },
                },
            },
        });
    });

    it("finds entry point from package.json main property", async function () {
        const dir = path.join(__dirname, "testdata", "uncommon-main");
        const analyzer = new Analyzer(
            dir,
            "provider",
            { name: "provider", main: "./src/mymain.js" },
            new Set(["MainComponent"]),
        );
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MainComponent: {
                name: "MainComponent",
                inputs: {
                    message: { type: "string", plain: true },
                },
                outputs: {
                    formattedMessage: { type: "string" },
                },
            },
        });
    });

    it("prefers exports over main when both are present", async function () {
        const dir = path.join(__dirname, "testdata", "uncommon-main");
        const analyzer = new Analyzer(
            dir,
            "provider",
            {
                name: "provider",
                exports: "./src/mymain.ts",
                main: "./wrong/path.ts", // This should be ignored when exports is present
            },
            new Set(["MainComponent"]),
        );
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MainComponent: {
                name: "MainComponent",
                inputs: {
                    message: { type: "string", plain: true },
                },
                outputs: {
                    formattedMessage: { type: "string" },
                },
            },
        });
    });

    it("throws error when no entry points found", async function () {
        const tempDir = path.join(__dirname, "testdata", "no-entry");
        const analyzer = new Analyzer(tempDir, "provider", { name: "provider" }, new Set(["MainComponent"]));

        assert.throws(
            () => analyzer.analyze(),
            (err) => {
                return (
                    err.message ===
                    `No entry points found in ${tempDir}. Expected either 'exports' or 'main' in package.json, or an index.ts file in root or src directory.`
                );
            },
        );
    });

    it("resolves components through complex import chains", async function () {
        const dir = path.join(__dirname, "testdata", "complex-imports");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["DeepComponent"]));
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            DeepComponent: {
                name: "DeepComponent",
                inputs: {
                    level: { type: "number", plain: true },
                    name: { type: "string", plain: true, optional: true },
                },
                outputs: {
                    path: { type: "string" },
                },
            },
        });
    });

    it("reports missing components with clear error message", async function () {
        const dir = path.join(__dirname, "testdata", "simple-types");
        const analyzer = new Analyzer(
            dir,
            "provider",
            packageJSON,
            new Set(["MyComponent", "NonExistentComponent", "AnotherMissingOne"]),
        );

        assert.throws(
            () => analyzer.analyze(),
            (err) => {
                return (
                    err.message.includes(
                        "Failed to find the following components: NonExistentComponent, AnotherMissingOne",
                    ) && !err.message.includes("MyComponent")
                ); // MyComponent exists, so it shouldn't be in the error
            },
        );
    });

    it("throws error when componentNames is empty", async function () {
        const dir = path.join(__dirname, "testdata", "simple-types");
        assert.throws(
            () => new Analyzer(dir, "provider", packageJSON, new Set()),
            (err) => {
                return err.message === "componentNames cannot be empty - at least one component name must be provided";
            },
        );
    });

    it("finds components through re-exports", async function () {
        const dir = path.join(__dirname, "testdata", "re-exports");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent", "AnotherComponent"]));
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    message: { type: "string", plain: true },
                },
                outputs: {
                    output: { type: "string" },
                },
            },
            AnotherComponent: {
                name: "AnotherComponent",
                inputs: {
                    count: { type: "number", plain: true },
                },
                outputs: {
                    result: { type: "number" },
                },
            },
        });
    });

    it("supports `as const` object pattern enums", async function () {
        const dir = path.join(__dirname, "testdata", "as-const-object-enum");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components, typeDefinitions } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    status: {
                        $ref: "#/types/provider:index:ResourceStatus",
                        optional: true,
                        description: "The status of the component",
                    },
                },
                outputs: {
                    status: {
                        $ref: "#/types/provider:index:ResourceStatus",
                        description: "The current status of the resource",
                    },
                },
            },
        });
        assert.deepStrictEqual(typeDefinitions, {
            ResourceStatus: {
                name: "ResourceStatus",
                description: "This demonstrates const enums",
                enum: [
                    { name: "Provisioning", value: "provisioning", description: "The provisioning status" },
                    { name: "Active", value: "active", description: "The active status" },
                    { name: "Deleting", value: "deleting", description: "The deleting status" },
                    { name: "Failed", value: "failed", description: "The failed status" },
                ],
                type: "string",
            },
        });
    });

    it("supports enums", async function () {
        const dir = path.join(__dirname, "testdata", "enum");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components, typeDefinitions } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    status: {
                        $ref: "#/types/provider:index:ResourceStatus",
                        optional: true,
                        description: "The status of the component",
                    },
                    priority: {
                        $ref: "#/types/provider:index:Priority",
                        optional: true,
                        description: "The priority level",
                    },
                    level: {
                        $ref: "#/types/provider:index:Level",
                        optional: true,
                        description: "The level",
                    },
                },
                outputs: {
                    status: {
                        $ref: "#/types/provider:index:ResourceStatus",
                        description: "The current status of the resource",
                    },
                    priority: {
                        $ref: "#/types/provider:index:Priority",
                        description: "The priority of the resource",
                    },
                    level: {
                        $ref: "#/types/provider:index:Level",
                        description: "The level of the resource",
                    },
                },
            },
        });
        assert.deepStrictEqual(typeDefinitions, {
            ResourceStatus: {
                name: "ResourceStatus",
                description: "This demonstrates TypeScript string enums",
                enum: [
                    { name: "Provisioning", value: "provisioning", description: "The provisioning status" },
                    { name: "Active", value: "active", description: "The active status" },
                    { name: "Deleting", value: "deleting", description: "The deleting status" },
                    { name: "Failed", value: "failed", description: "The failed status" },
                ],
                type: "string",
            },
            Priority: {
                name: "Priority",
                description: "This demonstrates TypeScript numeric enums",
                enum: [
                    { name: "Low", value: 0 },
                    { name: "Medium", value: 1 },
                    { name: "High", value: 2 },
                    { name: "Critical", value: 3 },
                ],
                type: "number",
            },
            Level: {
                name: "Level",
                description: "This demonstrates TypeScript numeric enums with computed values",
                enum: [
                    { name: "A", value: 2, description: "Starting at 2" },
                    { name: "B", value: 3, description: "Auto-incremented to 3" },
                    { name: "C", value: 4, description: "Auto-incremented to 4" },
                ],
                type: "number",
            },
        });
    });

    it("supports imported `as const` object pattern enums", async function () {
        const dir = path.join(__dirname, "testdata", "as-const-enum-imported");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        const { components, typeDefinitions } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    mode: {
                        $ref: "#/types/provider:index:DeploymentMode",
                        optional: true,
                        description: "The deployment mode for the component",
                    },
                    retries: {
                        $ref: "#/types/provider:index:RetryCount",
                        optional: true,
                        description: "The retry count for the component",
                    },
                },
                outputs: {
                    mode: {
                        $ref: "#/types/provider:index:DeploymentMode",
                        description: "The current deployment mode",
                    },
                    retries: {
                        $ref: "#/types/provider:index:RetryCount",
                        description: "The current retry count",
                    },
                },
            },
        });
        assert.deepStrictEqual(typeDefinitions, {
            DeploymentMode: {
                name: "DeploymentMode",
                description: "This demonstrates const enums defined in a separate file",
                enum: [
                    { name: "Development", value: "dev" },
                    { name: "Staging", value: "staging" },
                    { name: "Production", value: "prod" },
                ],
                type: "string",
            },
            RetryCount: {
                name: "RetryCount",
                description: "This demonstrates numeric const enums defined in a separate file",
                enum: [
                    { name: "Low", value: 3 },
                    { name: "Medium", value: 5 },
                    { name: "High", value: 10 },
                ],
                type: "number",
            },
        });
    });

    it("rejects enums with computed initializer values", async function () {
        const dir = path.join(__dirname, "testdata", "enum-computed-value");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        assert.throws(
            () => analyzer.analyze(),
            (err: Error) => {
                assert.match(
                    err.message,
                    /Unsupported type for component 'MyComponent' input 'MyComponentArgs.priority': type 'Priority'/,
                );
                return true;
            },
        );
    });

    it("rejects invalid union types (mixed string and number)", async function () {
        const dir = path.join(__dirname, "testdata", "invalid-union");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        assert.throws(
            () => analyzer.analyze(),
            (err: Error) => {
                assert.match(
                    err.message,
                    /Union types are not supported for component 'MyComponent' input 'MyComponentArgs.mixedUnion'/,
                );
                return true;
            },
        );
    });

    it("rejects invalid union types (union of objects)", async function () {
        const dir = path.join(__dirname, "testdata", "invalid-union-objects");
        const analyzer = new Analyzer(dir, "provider", packageJSON, new Set(["MyComponent"]));
        assert.throws(
            () => analyzer.analyze(),
            (err: Error) => {
                assert.match(
                    err.message,
                    /Union types are not supported for component 'MyComponent' input 'MyComponentArgs.objectUnion'/,
                );
                return true;
            },
        );
    });
});

describe("formatErrorContext", () => {
    // We need to create an analyzer instance to test the private method
    const analyzer = new Analyzer(__dirname, "provider", packageJSON, new Set(["MyComponent"]));
    // Use any to access private method
    const formatErrorContext = (analyzer as any).formatErrorContext.bind(analyzer);

    it("formats basic component context", () => {
        assert.strictEqual(formatErrorContext({ component: "MyComponent" }), "component 'MyComponent'");
    });

    it("formats property context", () => {
        assert.strictEqual(
            formatErrorContext({ component: "MyComponent", property: "myProp" }),
            "component 'MyComponent' property 'myProp'",
        );
    });

    it("formats input property context", () => {
        assert.strictEqual(
            formatErrorContext({
                component: "MyComponent",
                property: "myProp",
                inputOutput: InputOutput.Input,
            }),
            "component 'MyComponent' input 'myProp'",
        );
    });

    it("formats output property context", () => {
        assert.strictEqual(
            formatErrorContext({
                component: "MyComponent",
                property: "myProp",
                inputOutput: InputOutput.Output,
            }),
            "component 'MyComponent' output 'myProp'",
        );
    });

    it("formats type context", () => {
        assert.strictEqual(
            formatErrorContext({
                component: "MyComponent",
                typeName: "MyType",
                property: "myProp",
            }),
            "component 'MyComponent' property 'MyType.myProp'",
        );
    });
});
