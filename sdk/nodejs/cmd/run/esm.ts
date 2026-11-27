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
import * as fspromises from "fs/promises";
import * as path from "path";
import * as semver from "semver";
import * as url from "url";
// @ts-ignore This is available in all of our supported Node.js versions, but
// our version of @types/node does not know this. However we can't update our
// @types/node version because we need to continue supporting TypeScript 3.8.3.
// https://nodejs.org/api/module.html#moduleregisterspecifier-parenturl-options
import { register } from "module";
import * as log from "../../log";
import { readPackageManifest } from "../../runtime/manifest";
import * as tsutils from "../../tsutils";

// Workaround for typescript transpiling dynamic import into `Promise.resolve().then(() => require`
// Follow this issue for progress on when we can remove this:
// https://github.com/microsoft/TypeScript/issues/43329
// eslint-disable-next-line no-eval
const dynamicImport = (0, eval)("u=>import(u)");

/**
 * Reads the package manifest of a plugin or policy pack program. Returns an
 * empty object if there is no manifest or we cannot read it.
 *
 * @internal
 */
export function packageObjectFromProgram(program: string): Record<string, any> {
    try {
        return readPackageManifest(programDirectory(program)).data;
    } catch {
        return {};
    }
}

/**
 * Returns the directory of a plugin or policy pack program. The program is
 * usually a directory already, but it can also be a file.
 */
function programDirectory(program: string): string {
    try {
        return fs.lstatSync(program).isDirectory() ? program : path.dirname(program);
    } catch {
        return path.dirname(program);
    }
}

/**
 * Registers TypeScript transpilation for a plugin or policy pack program.
 *
 * If the program is an ES module and a recent `ts-node/esm` is available, this
 * registers the ESM loader hooks from `hooks.ts`. Otherwise it registers
 * ts-node for CommonJS.
 *
 * @internal
 */
export function registerTypeScript(tsConfigPath: string, isModule: boolean): void {
    const { tsnodeRequire, typescriptRequire } = tsutils.typeScriptRequireStrings();
    const skipProject = !fs.existsSync(tsConfigPath);
    const compilerOptions: object = tsutils.loadTypeScriptCompilerOptions(tsConfigPath);
    const ts = require(typescriptRequire);
    const tsVersion = semver.parse(ts.version);

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

    // See if we can use the automatic ESM mode. We require that:
    // * the program is an ES module (package.json specifies `type: "module"`).
    // * have TypeScript >= 4.7, which introduces support for `module: "nodenext"` in tsconfig.json.
    // * node is running without any other custom loader, we don't want to interfere with any custom setup.
    // * ts-node/esm is available, this is only available in recent ts-node versions, but we're stuck
    //   shipping a vendored version of 7.0.1. Users are free to install a more recent version in their
    //   project, and we'll see if we can load that.
    const hasLoaders = process.execArgv.find((arg) => arg === "--loader" || arg === "--import" || arg === "--require");
    let tsNodeESMRequire = "";
    try {
        tsNodeESMRequire = require.resolve("ts-node/esm");
    } catch (err) {
        // ts-node/esm not available, we'll use ts-node instead
    }

    if (isModule && tsVersion && tsVersion.compare(new semver.SemVer("4.7.0")) > 0 && !hasLoaders && tsNodeESMRequire) {
        log.debug("Using automatic ESM mode");
        const options = {
            compiler: typescriptRequire,
            transpileOnly,
            skipProject,
            compilerOptions: {
                target: "ES2022", // TS >= 4.6 and Node >= 20 support this
                module: "nodenext",
                moduleResolution: "nodenext",
                sourceMap: "true",
                ...compilerOptions,
            },
        };
        // The hooks path resolves relative to this file, which sits next to hooks.ts.
        register("./hooks.js", url.pathToFileURL(__filename), {
            data: options,
        });
        return;
    }

    const tsn: typeof tsnode = require(tsnodeRequire);
    // Use nodenext for TS >= 4.7, fall back to commonjs/node for older versions (e.g. vendored TS 3.8.3).
    const useNodeNext = tsVersion && tsVersion.compare(new semver.SemVer("4.7.0")) >= 0;
    tsn.register({
        transpileOnly,
        skipProject,
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

/**
 * Resolves the entry file of an ES module program. `import` does not accept a
 * directory, so for a directory we use the `exports` or `main` field of the
 * package manifest, and fall back to an index file.
 */
async function resolveESMEntrypoint(program: string, packageObject: Record<string, any>): Promise<string> {
    const mainPath = await fspromises.realpath(path.resolve(program));
    const stat = await fspromises.lstat(mainPath);
    if (!stat.isDirectory()) {
        return mainPath;
    }

    let entrypoint: string | undefined;
    const exports = packageObject["exports"];
    if (typeof exports === "string") {
        entrypoint = exports;
    } else if (exports && typeof exports === "object") {
        const mainExport = exports["."];
        if (typeof mainExport === "string") {
            entrypoint = mainExport;
        } else if (mainExport && typeof mainExport === "object") {
            entrypoint = mainExport.default || mainExport.import || mainExport.require;
        }
    } else if (typeof packageObject["main"] === "string") {
        entrypoint = packageObject["main"];
    }
    if (entrypoint) {
        const p = path.join(mainPath, entrypoint);
        if (fs.existsSync(p) && !fs.lstatSync(p).isDirectory()) {
            return p;
        }
        log.warn(`Could not find entry point '${entrypoint}' specified in package.json; using an index file instead`);
    }

    for (const index of ["index.js", "index.mjs", "index.ts", "index.mts"]) {
        if (fs.existsSync(path.join(mainPath, index))) {
            return path.join(mainPath, index);
        }
    }
    throw new Error(`No entrypoint found in ${mainPath}`);
}

/**
 * Loads a plugin or policy pack program and returns its exports. ES modules
 * are loaded with `import`, CommonJS modules with `require`.
 *
 * For an ES module that has only a default export, this returns the default
 * export.
 *
 * @internal
 */
export async function loadProgram(program: string, packageObject: Record<string, any>): Promise<any> {
    if (packageObject["type"] !== "module") {
        return require(program);
    }

    const mainPath = await resolveESMEntrypoint(program, packageObject);
    const programExport = await dynamicImport(url.pathToFileURL(mainPath).href);
    const keys = Object.keys(programExport);
    if (keys.length === 1 && keys[0] === "default") {
        return programExport.default;
    }
    return programExport;
}
