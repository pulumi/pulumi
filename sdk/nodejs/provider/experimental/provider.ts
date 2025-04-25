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

import { readFileSync } from "fs";
import * as path from "path";
import { ComponentResource, ComponentResourceOptions } from "../../resource";
import { ConstructResult, Provider } from "../provider";
import { Input, Inputs, Output } from "../../output";
import { main } from "../server";
import { generateSchema } from "./schema";
import { Analyzer, ComponentDefinition } from "./analyzer";

function getComponentOutputs<T extends ComponentResource>(
    componentDefinition: ComponentDefinition,
    resource: T,
): Record<keyof T, Input<T[keyof T]>> {
    const result: any = {};
    for (const key of Object.keys(componentDefinition.outputs)) {
        result[key] = resource[key as keyof T];
    }
    return result;
}

export type ComponentResourceConstructor = {
    // The ComponentResource base class has a 4 argument constructor, but
    // the user defined component has a 3 argument constructor without the
    // typestring.
    new (name: string, args: any, opts?: ComponentResourceOptions): ComponentResource;
};

/**
 * Get all Pulumi Component constructors from a module's exports.
 * @param moduleExports The exports object of the module to check.
 * @returns Array of Pulumi Component constructors found in the exports.
 */
export function getPulumiComponents(moduleExports: any): ComponentResourceConstructor[] {
    // Use a Set to track seen components and maintain uniqueness
    const seen = new Set<ComponentResourceConstructor>();

    function getComponents(value: any): ComponentResourceConstructor[] {
        // If null/undefined, return empty array
        if (!value) {
            return [];
        }

        // If it's a single component constructor
        if (typeof value === "function" && value.prototype) {
            let proto = value.prototype;
            while (proto?.__proto__) {
                proto = proto.__proto__;
                if (
                    proto.constructor &&
                    (proto.constructor.name === "ComponentResource" ||
                        proto.constructor.__pulumiComponentResource === true)
                ) {
                    if (!seen.has(value)) {
                        seen.add(value);
                        return [value];
                    }
                    return [];
                }
            }
            return [];
        }

        const components: ComponentResourceConstructor[] = [];

        // If it's an array, process each item
        if (Array.isArray(value)) {
            for (const item of value) {
                components.push(...getComponents(item));
            }
            return components;
        }

        // If it's an object, process each property
        if (typeof value === "object") {
            for (const key in value) {
                // Filter unwanted properties from the prototype
                if (!Object.prototype.hasOwnProperty.call(value, key)) {
                    continue;
                }

                components.push(...getComponents(value[key]));
            }
        }

        return components;
    }

    return getComponents(moduleExports);
}

export interface ComponentProviderOptions {
    components: ComponentResourceConstructor[];
    dirname?: string;
    name: string;
    namespace?: string;
}

export class ComponentProvider implements Provider {
    private packageJSON: Record<string, any>;
    private path: string;
    private componentConstructors: Record<string, ComponentResourceConstructor>;
    private name: string;
    private namespace?: string;
    private componentDefinitions?: Record<string, ComponentDefinition>;
    private cachedSchema?: string;
    public version: string;

    public static validateResourceType(packageName: string, resourceType: string): void {
        const parts = resourceType.split(":");
        if (parts.length !== 3) {
            throw new Error(`Invalid resource type ${resourceType}`);
        }
        if (parts[0] !== packageName) {
            throw new Error(`Invalid package name ${parts[0]}, expected '${packageName}'`);
        }
        // We might want to relax this limitation, but for now we only support the "index" module.
        if (parts[1] !== "index" && parts[1] !== "") {
            throw new Error(
                `Invalid module '${parts[1]}' in resource type '${resourceType}', expected 'index' or empty string`,
            );
        }
        if (parts[2].length === 0) {
            throw new Error(`Empty resource name in resource type '${resourceType}'`);
        }
    }

    constructor(readonly options: ComponentProviderOptions & { version?: string }) {
        if (!options.dirname) {
            throw new Error("dirname is required");
        }

        const absDir = path.resolve(options.dirname);
        const packStr = readFileSync(`${absDir}/package.json`, { encoding: "utf-8" });
        this.packageJSON = JSON.parse(packStr);
        this.path = absDir;
        this.name = options.name;
        this.version = options.version ?? "0.0.0";
        this.namespace = options.namespace;
        this.componentConstructors = options.components.reduce(
            (acc, component) => {
                acc[component.name] = component;
                return acc;
            },
            {} as Record<string, ComponentResourceConstructor>,
        );
    }

    async getSchema(): Promise<string> {
        if (this.cachedSchema) {
            return this.cachedSchema;
        }
        const analyzer = new Analyzer(
            this.path,
            this.name,
            this.packageJSON,
            new Set(Object.keys(this.componentConstructors)),
        );
        const { components, typeDefinitions, dependencies } = analyzer.analyze();
        const schema = generateSchema(
            this.name,
            this.version,
            this.packageJSON.description,
            components,
            typeDefinitions,
            dependencies,
            this.namespace,
        );
        this.cachedSchema = JSON.stringify(schema);
        this.componentDefinitions = components;
        return this.cachedSchema;
    }

    async construct(
        name: string,
        type: string,
        inputs: Inputs,
        options: ComponentResourceOptions,
    ): Promise<ConstructResult> {
        ComponentProvider.validateResourceType(this.name, type);
        const componentName = type.split(":")[2];
        const constructor = this.componentConstructors[componentName];
        if (!constructor) {
            throw new Error(`Component class not found for '${componentName}'`);
        }
        const instance = new constructor(name, inputs, options);
        if (!this.componentDefinitions) {
            await this.getSchema();
        }
        const componentDefinition = this.componentDefinitions![componentName];
        return {
            urn: instance.urn,
            state: getComponentOutputs(componentDefinition, instance),
        };
    }
}

// Add a flag to track if componentProviderHost has been called
let isHosting = false;

export function componentProviderHost(options: ComponentProviderOptions): Promise<void> {
    if (isHosting) {
        // If we're already hosting, just return and don't start another host.
        return Promise.resolve();
    }
    isHosting = true;

    const args = process.argv.slice(2);
    // If dirname is not provided, get it from the call stack
    if (!options.dirname) {
        // Get the stack trace
        const stack = new Error().stack;
        // Parse the stack to get the caller's file
        // Stack format is like:
        // Error
        //     at componentProviderHost (.../src/index.ts:3:16)
        //     at Object.<anonymous> (.../caller/index.ts:4:1)
        const callerLine = stack?.split("\n")[2];
        const match = callerLine?.match(/\((.+):[0-9]+:[0-9]+\)/);
        if (match?.[1]) {
            options.dirname = path.dirname(match[1]);
        } else {
            throw new Error("Could not determine caller directory");
        }
    }
    // Default the version to "0.0.0" for now, otherwise SDK codegen gets
    // confused without a version.
    const version = "0.0.0";
    const prov = new ComponentProvider(options);
    return main(prov, args);
}
