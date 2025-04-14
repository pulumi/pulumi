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
import { generateSchema } from "../../../provider/experimental/schema";
import { ComponentDefinition, TypeDefinition } from "../../../provider/experimental/analyzer";

describe("Schema", function () {
    it("should generate schema with correct language dependencies", function () {
        const components: Record<string, ComponentDefinition> = {};
        const typeDefinitions: Record<string, TypeDefinition> = {};

        // Mock package references
        const packageReferences = {
            aws: "5.0.0",
            "azure-native": "4.0.0",
            kubernetes: "3.0.0",
        };

        // Generate schema
        const schema = generateSchema(
            "test-provider",
            "1.0.0",
            "Test provider for Pulumi",
            components,
            typeDefinitions,
            packageReferences,
        );

        // Verify NodeJS dependencies
        assert.deepStrictEqual(schema.language?.nodejs.dependencies, {
            "@pulumi/aws": "5.0.0",
            "@pulumi/azure-native": "4.0.0",
            "@pulumi/kubernetes": "3.0.0",
        });

        // Verify Python dependencies
        assert.deepStrictEqual(schema.language?.python.requires, {
            "pulumi-aws": "==5.0.0",
            "pulumi-azure-native": "==4.0.0",
            "pulumi-kubernetes": "==3.0.0",
        });

        // Verify C# dependencies
        assert.deepStrictEqual(schema.language?.csharp.packageReferences, {
            "Pulumi.Aws": "5.0.0",
            "Pulumi.AzureNative": "4.0.0",
            "Pulumi.Kubernetes": "3.0.0",
        });

        // Verify Java dependencies
        assert.deepStrictEqual(schema.language?.java.dependencies, {
            "com.pulumi:aws": "5.0.0",
            "com.pulumi:azure-native": "4.0.0",
            "com.pulumi:kubernetes": "3.0.0",
        });

        assert.strictEqual(schema.description, "Test provider for Pulumi");
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
            {},
            "my-namespace",
        );

        assert.strictEqual(schema.namespace, "my-namespace");
        assert.strictEqual(schema.name, "test-provider");
    });

    it("should map object and enum types correctly", function () {
        const components: Record<string, ComponentDefinition> = {};
        const typeDefinitions: Record<string, TypeDefinition> = {
            MyObject: {
                name: "MyObject",
                properties: {
                    name: { type: "string", optional: false },
                    count: { type: "number", optional: true },
                },
            },
            MyEnum: {
                name: "MyEnum",
                enum: [
                    { name: "Option1", value: "option1" },
                    { name: "Option2", value: "option2" },
                ],
            },
        };

        const schema = generateSchema(
            "test-provider",
            "1.0.0",
            "Test provider with object and enum types",
            components,
            typeDefinitions,
            {},
        );

        // Verify object type mapping
        assert.deepStrictEqual(schema.types["test-provider:index:MyObject"], {
            type: "object",
            properties: {
                name: { type: "string" },
                count: { type: "number" },
            },
            required: ["name"],
        });

        // Verify enum type mapping
        assert.deepStrictEqual(schema.types["test-provider:index:MyEnum"], {
            type: "string",
            enum: [
                { name: "Option1", value: "option1" },
                { name: "Option2", value: "option2" },
            ],
        });
    });
});
