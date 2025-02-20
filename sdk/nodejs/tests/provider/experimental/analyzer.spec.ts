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

    it("infers simple types", async function () {
        const dir = path.join(__dirname, "testdata", "simple-types");
        const analyzer = new Analyzer(dir, "provider");
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
                    outBoolean: { type: "boolean" },
                },
            },
        });
    });

    it("infers optional types", async function () {
        const dir = path.join(__dirname, "testdata", "optional-types");
        const analyzer = new Analyzer(dir, "provider");
        const { components } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    optionalNumber: { type: "number", optional: true, plain: true },
                },
                outputs: {
                    optionalOutputNumber: { type: "number", optional: true },
                },
            },
        });
    });

    it("infers input types", async function () {
        const dir = path.join(__dirname, "testdata", "input-types");
        const analyzer = new Analyzer(dir, "provider");
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
        const analyzer = new Analyzer(dir, "provider");
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
            },
            MyClassType: {
                name: "MyClassType",
                properties: { aString: { type: "string", plain: true } },
            },
        });
    });

    it("infers self recursive complex types", async function () {
        const dir = path.join(__dirname, "testdata", "recursive-complex-types");
        const analyzer = new Analyzer(dir, "provider");
        const { components, typeDefinitions } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    theSelfRecursiveTypeInput: { $ref: "#/types/provider:index:SelfRecursive" },
                },
                outputs: {
                    theSelfRecursiveTypeOutput: { $ref: "#/types/provider:index:SelfRecursive" },
                },
            },
        });
        assert.deepStrictEqual(typeDefinitions, {
            SelfRecursive: {
                name: "SelfRecursive",
                properties: { self: { $ref: "#/types/provider:index:SelfRecursive", plain: true } },
            },
        });
    });

    it("infers mutually recursive complex types", async function () {
        const dir = path.join(__dirname, "testdata", "mutually-recursive-types");
        const analyzer = new Analyzer(dir, "provider");
        const { components, typeDefinitions } = analyzer.analyze();
        assert.deepStrictEqual(components, {
            MyComponent: {
                name: "MyComponent",
                inputs: {
                    typeAInput: { $ref: "#/types/provider:index:TypeA" },
                },
                outputs: {
                    typeBOutput: { $ref: "#/types/provider:index:TypeB" },
                },
            },
        });
        assert.deepStrictEqual(typeDefinitions, {
            TypeA: {
                name: "TypeA",
                properties: { b: { $ref: "#/types/provider:index:TypeB", plain: true } },
            },
            TypeB: {
                name: "TypeB",
                properties: { a: { $ref: "#/types/provider:index:TypeA", plain: true } },
            },
        });
    });

    it("rejects bad args", async function () {
        const dir = path.join(__dirname, "testdata", "bad-args");
        const analyzer = new Analyzer(dir, "provider");
        assert.throws(
            () => analyzer.analyze(),
            /Error: Component 'MyComponent' constructor 'args' parameter must be an interface/,
        );
    });

    it("infers map types", async function () {
        const dir = path.join(__dirname, "testdata", "map-types");
        const analyzer = new Analyzer(dir, "provider");
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
                },
            },
        });
    });

    it("infers asset/archive types", async function () {
        const dir = path.join(__dirname, "testdata", "asset-archive-types");
        const analyzer = new Analyzer(dir, "provider");
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
});
