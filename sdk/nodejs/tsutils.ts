// Copyright 2016-2022, Pulumi Corporation.
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

// The typescript and tsnode imports are used for type-checking only. Do not reference them in the emitted code.
import * as typescript from "typescript";
import * as tsnode from "ts-node";
import * as fs from "fs";
import * as path from "path";
import * as log from "./log";

/**
 * @internal
 */
export function loadTypeScriptCompilerOptions(tsConfigPath: string): object {
    try {
        const tsConfigString = fs.readFileSync(tsConfigPath).toString();
        // Using local `require("./typescript-shim")` to avoid always loading
        // and only load on-demand, avoid up to 300s overhead in Node runtime.
        const ts: typeof typescript = require("./typescript-shim");
        const tsConfig = ts.parseConfigFileTextToJson(tsConfigPath, tsConfigString).config;
        return tsConfig["compilerOptions"] ?? {};
    } catch (err) {
        log.debug(`Ignoring error in loadCompilerOptions(${tsConfigPath}}): ${err}`);
        return {};
    }
}

/**
 * Determine the strings used for requiring `typescript` and `ts-node`.
 *
 * @internal
 */
export function typeScriptRequireStrings(): { typescriptRequire: string; tsnodeRequire: string } {
    let typescriptRequire = "typescript";
    let tsnodeRequire = "ts-node";

    try {
        // Try loading typescript from node_modules
        const ts: typeof typescript = require("typescript");
        const tsPath = require.resolve("typescript");
        log.debug(`Found typescript version ${ts.version} at ${tsPath}`);
    } catch (error) {
        // Fallback to the vendored version
        typescriptRequire = path.join(__dirname, "vendor", "typescript@3.8.3", "typescript.js");
        log.debug(`Using vendored typescript@3.8.3`);
    }
    try {
        // Try loading ts-node from node_modules
        const tsn: typeof tsnode = require(tsnodeRequire);
        const tsnPath = require.resolve("ts-node");
        log.debug(`Found ts-node version: ${tsn.VERSION} at ${tsnPath}`);
    } catch (error) {
        // Fallback to the vendored version
        tsnodeRequire = path.join(__dirname, "vendor", "ts-node@7.0.1");
        log.debug(`Using vendored ts-node@7.0.1`);
    }

    return {
        typescriptRequire,
        tsnodeRequire,
    };
}
