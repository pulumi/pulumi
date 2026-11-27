// Copyright 2026, Pulumi Corporation.
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

// The tsnode import is used for type-checking only. Do not reference it in the emitted code.
import * as tsnode from "ts-node";
import * as fs from "fs";
import * as semver from "semver";
import * as tsutils from "../../tsutils";

/**
 * Registers TypeScript transpilation for a plugin or policy pack program.
 *
 * @internal
 */
export function registerTypeScript(tsConfigPath: string): void {
    const { tsnodeRequire, typescriptRequire } = tsutils.typeScriptRequireStrings();
    const skipProject = !fs.existsSync(tsConfigPath);
    const compilerOptions: object = tsutils.loadTypeScriptCompilerOptions(tsConfigPath);
    const tsn: typeof tsnode = require(tsnodeRequire);
    const ts = require(typescriptRequire);
    const tsVersion = semver.parse(ts.version);
    // Use nodenext for TS >= 4.7, fall back to commonjs/node for older versions (e.g. vendored TS 3.8.3).
    const useNodeNext = tsVersion && tsVersion.compare(new semver.SemVer("4.7.0")) >= 0;

    // transpileOnly controls whether ts-node should do type checking or not.
    // Users might have a separate build step that runs tsc for type
    // checking, and don't want to pay the performance cost of type checking
    // twice. This also enables using swc, which doesn't support type
    // checking, with ts-node.
    //
    // If the `PULUMI_NODEJS_TRANSPILE_ONLY `env variable is set, we use
    // that to determine the value of `transpileOnly.` Otherwise we use the
    // `noCheck `compiler option from the tsconfig. Otherwise we default to
    // ts-node's default, which is to type check.
    let transpileOnly = undefined;
    const transpileOnlyEnv = process.env["PULUMI_NODEJS_TRANSPILE_ONLY"];
    if (transpileOnlyEnv) {
        transpileOnly = transpileOnlyEnv === "true";
    } else {
        // @ts-ignore
        transpileOnly = compilerOptions.noCheck;
    }

    tsn.register({
        transpileOnly,
        skipProject: skipProject,
        compiler: typescriptRequire,
        compilerOptions: {
            target: "ES2020", // TypeScript 3.8 supports this
            module: useNodeNext ? "nodenext" : "commonjs",
            moduleResolution: useNodeNext ? "nodenext" : "node",
            sourceMap: "true",
            ...compilerOptions,
        },
    });
}
