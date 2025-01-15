// Copyright 2016-2024, Pulumi Corporation.

import { readFileSync } from "fs";
import * as pulumi from "@pulumi/pulumi";
import * as provider from "@pulumi/pulumi/provider";
import { generateSchema } from "./schema";
import { instantiateComponent } from "./instantiator";
import * as path from "path";
import { ComponentResource } from "@pulumi/pulumi";


type OutputsToInputs<T> = {
    [K in keyof T]: T[K] extends pulumi.Output<infer U> ? pulumi.Input<U> : never;
};

function getInputsFromOutputs<T extends ComponentResource>(resource: T): OutputsToInputs<T> {
    const result: any = {};
    for (const key of Object.keys(resource)) {
        const value = resource[key as keyof T];
        if (pulumi.Output.isInstance(value)) {
            result[key] = value;
        }
    }
    return result as OutputsToInputs<T>;
}

class ComponentProvider implements provider.Provider {
    pack: any;
    version: string;
    path: string;
    constructor(readonly dir: string) {
        const absDir = path.resolve(dir)
        const packStr = readFileSync(`${absDir}/package.json`, {encoding: "utf-8"});
        this.pack = JSON.parse(packStr);
        this.version = this.pack.version;
        this.path = absDir;
    }

    async getSchema(): Promise<string> {
        const schema = generateSchema(this.pack, this.path);
        return JSON.stringify(schema);
    }

    async construct(name: string, type: string, inputs: pulumi.Inputs,
        options: pulumi.ComponentResourceOptions): Promise<provider.ConstructResult> {
        const className = type.split(":")[2];
        const comp = await instantiateComponent(this.path, className, name, inputs, options);
        return {
            urn: comp.urn,
            state: getInputsFromOutputs(comp),
        }
    }
}

export function componentProviderHost(dirname?: string): Promise<void> {
    const args = process.argv.slice(2);
    // If dirname is not provided, get it from the call stack
    if (!dirname) {
        // Get the stack trace
        const stack = new Error().stack;
        // Parse the stack to get the caller's file
        // Stack format is like:
        // Error
        //     at componentProviderHost (.../src/index.ts:3:16)
        //     at Object.<anonymous> (.../caller/index.ts:4:1)
        const callerLine = stack?.split('\n')[2];
        const match = callerLine?.match(/\((.+):[0-9]+:[0-9]+\)/);
        if (match && match[1]) {
            dirname = path.dirname(match[1]);
        } else {
            throw new Error('Could not determine caller directory');
        }
    }

    const prov = new ComponentProvider(dirname);
    return provider.main(prov, args);
}