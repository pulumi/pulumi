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
import { ComponentResourceOptions } from "../../resource";
import { ConstructResult, Provider } from "../provider";
import { Inputs } from "../../output";
import { main } from "../server";
import { generateSchema } from "./schema";
import { Analyzer } from "./analyzer";

class ComponentProvider implements Provider {
    packageJSON: Record<string, any>;
    version: string;
    path: string;

    constructor(readonly dir: string) {
        const absDir = path.resolve(dir);
        const packStr = readFileSync(`${absDir}/package.json`, { encoding: "utf-8" });
        this.packageJSON = JSON.parse(packStr);
        this.version = this.packageJSON.version;
        this.path = absDir;
    }

    async getSchema(): Promise<string> {
        const analyzer = new Analyzer(this.path);
        const { components, typeDefinitons } = analyzer.analyze();
        const schema = generateSchema(this.packageJSON, components, typeDefinitons);
        return JSON.stringify(schema);
    }

    async construct(
        name: string,
        type: string,
        inputs: Inputs,
        options: ComponentResourceOptions,
    ): Promise<ConstructResult> {
        throw new Error("Not implemented");
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
        const callerLine = stack?.split("\n")[2];
        const match = callerLine?.match(/\((.+):[0-9]+:[0-9]+\)/);
        if (match?.[1]) {
            dirname = path.dirname(match[1]);
        } else {
            throw new Error("Could not determine caller directory");
        }
    }

    const prov = new ComponentProvider(dirname);
    return main(prov, args);
}
