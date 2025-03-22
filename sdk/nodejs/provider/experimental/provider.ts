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
import { Inputs, Input, Output } from "../../output";
import { main } from "../server";
import { generateSchema } from "./schema";
import { Analyzer } from "./analyzer";

type OutputsToInputs<T> = {
    [K in keyof T]: T[K] extends Output<infer U> ? Input<U> : never;
};

function getInputsFromOutputs<T extends ComponentResource>(resource: T): OutputsToInputs<T> {
    const result: any = {};
    for (const key of Object.keys(resource)) {
        const value = resource[key as keyof T];
        if (Output.isInstance(value)) {
            result[key] = value;
        }
    }
    return result as OutputsToInputs<T>;
}

export type ComponentResourceConstructor = {
    // The ComponentResource base class has a 4 argument constructor, but
    // the user defined component has a 3 argument constructor without the
    // typestring.
    new (name: string, args: any, opts?: ComponentResourceOptions): ComponentResource;
};

export interface ComponentProviderOptions {
    components: ComponentResourceConstructor[];
    dirname?: string;
}

export class ComponentProvider implements Provider {
    private packageJSON: Record<string, any>;
    private path: string;
    private componentConstructors: Record<string, ComponentResourceConstructor>;

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

    constructor(readonly options: ComponentProviderOptions) {
        if (!options.dirname) {
            throw new Error("dirname is required");
        }

        const absDir = path.resolve(options.dirname);
        const packStr = readFileSync(`${absDir}/package.json`, { encoding: "utf-8" });
        this.packageJSON = JSON.parse(packStr);
        this.path = absDir;
        this.componentConstructors = options.components.reduce(
            (acc, component) => {
                acc[component.name] = component;
                return acc;
            },
            {} as Record<string, ComponentResourceConstructor>,
        );
    }

    async getSchema(): Promise<string> {
        const analyzer = new Analyzer(this.path, this.packageJSON, new Set(Object.keys(this.componentConstructors)));
        const { components, typeDefinitions, packageReferences } = analyzer.analyze();
        const schema = generateSchema(this.packageJSON, components, typeDefinitions, packageReferences);
        return JSON.stringify(schema);
    }

    async construct(
        name: string,
        type: string,
        inputs: Inputs,
        options: ComponentResourceOptions,
    ): Promise<ConstructResult> {
        ComponentProvider.validateResourceType(this.packageJSON.name, type);
        const componentName = type.split(":")[2];
        const constructor = this.componentConstructors[componentName];
        if (!constructor) {
            throw new Error(`Component class not found for '${componentName}'`);
        }
        const instance = new constructor(name, inputs, options);
        return {
            urn: instance.urn,
            state: getInputsFromOutputs(instance),
        };
    }
}

export function componentProviderHost(options: ComponentProviderOptions): Promise<void> {
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

    const prov = new ComponentProvider(options);
    return main(prov, args);
}
