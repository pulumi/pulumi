// Copyright 2025, Pulumi Corporation.
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
import { generateSchema } from "../../../provider/experimental/schema";
import { ComponentDefinition, TypeDefinition } from "../../../provider/experimental/analyzer";

describe("Schema", function () {
    it("should generate schema dependencies", function () {
        const components: Record<string, ComponentDefinition> = {};
        const typeDefinitions: Record<string, TypeDefinition> = {};

        // Mock package references
        const dependencies = [
            { name: "aws", version: "5.0.0" },
            {
                name: "terraform-provider",
                version: "0.10.0",
                parameterization: {
                    name: "parameterized",
                    version: "0.2.2",
                    value: "eyJyZW1vdGUiOnsidXJsIjoicmVnaXN0cnkub3BlbnRvZnUub3JnL25ldGxpZnkvbmV0bGlmeSIsInZlcnNpb24iOiIwLjIuMiJ9fQ==",
                },
            },
            { name: "kubernetes", version: "3.0.0", downloadURL: "example.com/download" },
        ];

        // Generate schema
        const schema = generateSchema(
            "test-provider",
            "1.0.0",
            "Test provider for Pulumi",
            components,
            typeDefinitions,
            dependencies,
        );

        assert.deepStrictEqual(schema.dependencies, [
            { name: "aws", version: "5.0.0" },
            {
                name: "terraform-provider",
                version: "0.10.0",
                parameterization: {
                    name: "parameterized",
                    version: "0.2.2",
                    value: "eyJyZW1vdGUiOnsidXJsIjoicmVnaXN0cnkub3BlbnRvZnUub3JnL25ldGxpZnkvbmV0bGlmeSIsInZlcnNpb24iOiIwLjIuMiJ9fQ==",
                },
            },
            { name: "kubernetes", version: "3.0.0", downloadURL: "example.com/download" },
        ]);
    });

    it("should use the namespace if there is one", function () {
        const components: Record<string, ComponentDefinition> = {};
        const typeDefinitions: Record<string, TypeDefinition> = {};

        const schema = generateSchema(
            "test-provider",
            "1.0.0",
            "my-description",
            components,
            typeDefinitions,
            [],
            "my-namespace",
        );

        assert.strictEqual(schema.namespace, "my-namespace");
        assert.strictEqual(schema.name, "test-provider");
    });

    it("emits a local extends ref and no requiredFeatures for a locally flattened subclass", function () {
        const components: Record<string, ComponentDefinition> = {
            Base: {
                name: "Base",
                inputs: { baseInput: { type: "string" } },
                outputs: { baseOutput: { type: "string" } },
            },
            Derived: {
                name: "Derived",
                inputs: { baseInput: { type: "string" }, derivedInput: { type: "number" } },
                outputs: { baseOutput: { type: "string" }, derivedOutput: { type: "number" } },
                extends: { $ref: "#/resources/test-provider:index:Base" },
            },
        };

        const schema = generateSchema("test-provider", "1.0.0", "desc", components, {}, []);

        assert.deepStrictEqual(schema.resources["test-provider:index:Derived"].extends, {
            $ref: "#/resources/test-provider:index:Base",
        });
        // A locally flattened schema carries the full member set, so it must not require the inheritance feature.
        assert.strictEqual(schema.requiredFeatures, undefined);
    });

    it("emits an external extends ref and requiredFeatures for a sparse subclass", function () {
        const components: Record<string, ComponentDefinition> = {
            MyService: {
                name: "MyService",
                inputs: { replicas: { type: "number" } },
                outputs: { endpoint: { type: "string" } },
                extends: { $ref: "/basecomponent/v1.0.0/schema.json#/resources/basecomponent:index:Service" },
            },
        };

        const schema = generateSchema("test-provider", "1.0.0", "desc", components, {}, [
            { name: "basecomponent", version: "1.0.0" },
        ]);

        assert.deepStrictEqual(schema.resources["test-provider:index:MyService"].extends, {
            $ref: "/basecomponent/v1.0.0/schema.json#/resources/basecomponent:index:Service",
        });
        // A sparse schema relies on the binder to materialize inherited members, so it must declare the feature.
        assert.deepStrictEqual(schema.requiredFeatures, ["inheritance"]);
    });

    it("emits abstract on an abstract component", function () {
        const components: Record<string, ComponentDefinition> = {
            MyComponent: {
                name: "MyComponent",
                inputs: {},
                outputs: { outResult: { type: "string" } },
                abstract: true,
            },
        };

        const schema = generateSchema("test-provider", "1.0.0", "desc", components, {}, []);

        assert.strictEqual(schema.resources["test-provider:index:MyComponent"].abstract, true);
    });
});
