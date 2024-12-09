// Copyright 2016-2024, Pulumi Corporation.

import { readFileSync, writeFileSync } from "fs";
import * as path from "path";
import {Output, Input, Inputs} from "../../../output";

import {ComponentResource,ComponentResourceOptions} from "../../../resource";
import * as provider from "../../../provider";
import { generateSchema } from "./schema";
import { instantiateComponent } from "./instantiator";

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

class ComponentProvider implements provider.Provider {
    pack: any;
    version: string;
    path: string;
    constructor(readonly dir: string, readonly reqRequest: any) {
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

    async construct(name: string, type: string, inputs: Inputs,
        options: ComponentResourceOptions): Promise<provider.ConstructResult> {
        const className = type.split(":")[2];
        const comp = await instantiateComponent(this.reqRequest, className, name, inputs, options);
        return {
            urn: comp.urn,
            state: getInputsFromOutputs(comp),
        }
    }
}

export function componentProviderHost(path: string, reqRequest: any): Promise<void> {
    const args = process.argv.slice(2);
    const prov = new ComponentProvider(path, reqRequest);
    return provider.main(prov, args);
}