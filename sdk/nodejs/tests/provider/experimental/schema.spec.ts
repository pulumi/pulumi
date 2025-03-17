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
        const packageJSON = {
            name: "test-provider",
            version: "1.0.0",
            description: "Test provider for Pulumi",
        };

        const components: Record<string, ComponentDefinition> = {};
        const typeDefinitions: Record<string, TypeDefinition> = {};

        // Mock package references
        const packageReferences = {
            aws: "5.0.0",
            "azure-native": "4.0.0",
            kubernetes: "3.0.0",
        };

        // Generate schema
        const schema = generateSchema(packageJSON, components, typeDefinitions, packageReferences);

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
    });
});
