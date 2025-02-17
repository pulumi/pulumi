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

import { ComponentDefinition, TypeDefinition, PropertyDefinition } from "./analyzer";

export type PropertyType = "string" | "integer" | "number" | "boolean" | "array" | "object";

/**
 * https://www.pulumi.com/docs/iac/using-pulumi/pulumi-packages/schema/#type
 */
export type Type = ({ type: PropertyType } | { $ref: string }) & {
    items?: Type;
    additionalProperties?: Type;
    plain?: boolean;
};

/**
 * https://www.pulumi.com/docs/iac/using-pulumi/pulumi-packages/schema/#property
 */
export type Property = Type & {
    description?: string;
};

/**
 * https://www.pulumi.com/docs/iac/using-pulumi/pulumi-packages/schema/#objecttype
 */
export interface ObjectType {
    type: PropertyType;
    description?: string;
    properties?: { [key: string]: Property };
    required?: string[];
}

/**
 * https://www.pulumi.com/docs/iac/using-pulumi/pulumi-packages/schema/#complextype
 */
export interface ComplexType extends ObjectType {
    enum?: string[];
}

/**
 * https://www.pulumi.com/docs/iac/using-pulumi/pulumi-packages/schema/#resource
 */
export interface Resource extends ObjectType {
    isComponent?: boolean;
    inputProperties?: { [key: string]: Property };
    requiredInputs?: string[];
}

/**
 * https://www.pulumi.com/docs/iac/using-pulumi/pulumi-packages/schema/#package
 */
export interface PackageSpec {
    name: string;
    version?: string;
    description?: string;
    resources: { [key: string]: Resource };
    types: { [key: string]: ComplexType };
    language?: { [key: string]: any };
}

export function generateSchema(
    packageJSON: Record<string, any>,
    components: Record<string, ComponentDefinition>,
    typeDefinitions: Record<string, TypeDefinition>,
): PackageSpec {
    const providerName = packageJSON.name;
    const result: PackageSpec = {
        name: providerName,
        version: packageJSON.version,
        description: packageJSON.description,
        resources: {},
        types: {},
        language: {
            nodejs: {
                dependencies: {},
                devDependencies: {
                    typescript: "^5.0.0",
                },
                respectSchemaVersion: true,
            },
            python: {
                respectSchemaVersion: true,
            },
            csharp: {
                respectSchemaVersion: true,
            },
            java: {
                respectSchemaVersion: true,
            },
            go: {
                respectSchemaVersion: true,
            },
        },
    };

    for (const [name, component] of Object.entries(components)) {
        result.resources[`${providerName}:index:${name}`] = {
            type: "object",
            isComponent: true,
            inputProperties: component.inputs,
            requiredInputs: required(component.inputs),
            properties: component.outputs,
            required: required(component.outputs),
        };
    }

    for (const [name, type] of Object.entries(typeDefinitions)) {
        result.types[`${providerName}:index:${name}`] = {
            type: "object",
            properties: type.properties,
            required: required(type.properties),
        };
    }

    return result;
}

function required(properties: Record<string, PropertyDefinition>): string[] {
    return Object.entries(properties)
        .filter(([_, def]) => !def.optional)
        .map(([propName, _]) => propName)
        .sort();
}
