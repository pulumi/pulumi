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

// The tsnode import is used for type-checking only. Do not reference it in the emitted code.
import * as tsnode from "ts-node";
import * as fs from "fs";
import * as fspromises from "fs/promises";
import * as ini from "ini";
import * as minimist from "minimist";
import * as path from "path";
import * as semver from "semver";
import * as url from "url";
import * as util from "util";
import { ResourceError, RunError } from "../../errors";
import * as log from "../../log";
import { Inputs } from "../../output";
import * as settings from "../../runtime/settings";
import * as stack from "../../runtime/stack";
import * as tsutils from "../../tsutils";
import * as tracing from "./tracing";
import { defaultErrorMessage } from "./error";

import * as mod from ".";

// Workaround for typescript transpiling dynamic import into `Promise.resolve().then(() => require`
// Follow this issue for progress on when we can remove this:
// https://github.com/microsoft/TypeScript/issues/43329
//
// Workaround inspired by es-module-shims:
// https://github.com/guybedford/es-module-shims/blob/main/src/common.js#L21
/** @internal */
// eslint-disable-next-line no-eval
const dynamicImport = (0, eval)("u=>import(u)");

/**
 * Attempts to provide a detailed error message for module load failure if the
 * module that failed to load is the top-level module.
 * @param program The name of the program given to `run`, i.e. the top level module
 * @param error The error that occured. Must be a module load error.
 */
async function reportModuleLoadFailure(program: string, error: Error): Promise<void> {
    await throwOrPrintModuleLoadError(program, error);

    // Note: from this point on, we've printed something to the user telling them about the
    // problem.  So we can let our langhost know it doesn't need to report any further issues.
    return process.exit(mod.nodeJSProcessExitedAfterLoggingUserActionableMessage);
}

/**
 * @internal
 * This function searches for the nearest package.json file, scanning up from the
 * program path until it finds one. If it does not find a package.json file, it
 * it returns the folder enclosing the program.
 * @param programPath the path to the Pulumi program; this is the project "main" directory,
 * which defaults to the project "root" directory.
 */
async function npmPackageRootFromProgramPath(programPath: string): Promise<string> {
    // pkg-dir is an ESM module which we use to find the location of package.json
    // Because it's an ESM module, we cannot import it directly.
    const { packageDirectory } = await dynamicImport("pkg-dir");
    // Check if programPath is a directory. If not, then we
    // look at it's parent dir for the package root.
    let isDirectory = false;
    try {
        const fileStat = await fspromises.lstat(programPath);
        isDirectory = fileStat.isDirectory();
    } catch {
        // Since an exception was thrown, the program path doesn't exist.
        // Do nothing, because isDirectory is already false.
    }
    const programDirectory = isDirectory ? programPath : path.dirname(programPath);
    const pkgDir = await packageDirectory({
        cwd: programDirectory,
    });
    if (pkgDir === undefined) {
        log.warn(
            "Could not find a package.json file for the program. Using the Pulumi program directory as the project root.",
        );
        return programDirectory;
    }
    return pkgDir;
}

function packageObjectFromProjectRoot(projectRoot: string): Record<string, any> {
    const packageJson = path.join(projectRoot, "package.json");
    try {
        return require(packageJson);
    } catch {
        // This is all best-effort so if we can't load the package.json file, that's
        // fine.
        return {};
    }
}

// Reads and parses the contents of .npmrc file if it exists under the project root
// This assumes that .npmrc is a sibling to package.json
function npmRcFromProjectRoot(projectRoot: string): Record<string, any> {
    const rcSpan = tracing.newSpan("language-runtime.reading-npm-rc");
    const emptyConfig = {};
    try {
        const npmRcPath = path.join(projectRoot, ".npmrc");
        if (!fs.existsSync(npmRcPath)) {
            return emptyConfig;
        }
        // file .npmrc exists, read its contents
        const npmRc = fs.readFileSync(npmRcPath, "utf-8");
        // Use ini to parse the contents of the .npmrc file
        // This is what node does as described in the npm docs
        // https://docs.npmjs.com/cli/v8/configuring-npm/npmrc#comments
        const parseResult = ini.parse(npmRc);
        rcSpan.end();
        return parseResult;
    } catch {
        // .npmrc file exists but we couldn't read or parse it
        // user out of luck here
        rcSpan.end();
        return emptyConfig;
    }
}

async function throwOrPrintModuleLoadError(program: string, error: Error): Promise<void> {
    // error is guaranteed to be a Node module load error. Node emits a very
    // specific string in its error message for module load errors, which includes
    // the module it was trying to load.
    const errorRegex = /Cannot find module '(.*)'/;

    // If there's no match, who knows what this exception is; it's not something
    // we can provide an intelligent diagnostic for.
    const moduleNameMatches = errorRegex.exec(error.message);
    if (moduleNameMatches === null) {
        throw error;
    }

    // Is the module that failed to load exactly the one that this script considered to
    // be the top-level module for this program?
    //
    // We are only interested in producing good diagnostics for top-level module loads,
    // since anything else are probably user code issues.
    const moduleName = moduleNameMatches[1];
    if (moduleName !== program) {
        throw error;
    }

    // Note: from this point on, we've printed something to the user telling them about the
    // problem.  So we can let our langhost know it doesn't need to report any further issues.
    console.error(`We failed to locate the entry point for your program: ${program}`);

    // From here on out, we're going to try to inspect the program we're being asked to run
    // a little to see what sort of details we can glean from it, in the hopes of producing
    // a better error message.
    //
    // The first step of this is trying to slurp up a package.json for this program, if
    // one exists.
    const packageRoot = await npmPackageRootFromProgramPath(program);
    const packageObject = packageObjectFromProjectRoot(packageRoot);

    console.error("Here's what we think went wrong:");

    // The objective here is to emit the best diagnostic we can, starting from the
    // most specific to the least specific.
    const deps = packageObject["dependencies"] || {};
    const devDeps = packageObject["devDependencies"] || {};
    const scripts = packageObject["scripts"] || {};
    const mainProperty = packageObject["main"] || "index.js";

    // Is there a build script associated with this program? It's a little confusing that the
    // Pulumi CLI doesn't run build scripts before running the program so call that out
    // explicitly.

    if ("build" in scripts) {
        const command = scripts["build"];
        console.error(`  * Your program looks like it has a build script associated with it ('${command}').\n`);
        console.error(
            "Pulumi does not run build scripts before running your program. " +
                `Please run '${command}', 'yarn build', or 'npm run build' and try again.`,
        );
        return;
    }

    // Not all typescript programs have build scripts. If we think it's a typescript program,
    // tell the user to run tsc.
    if ("typescript" in deps || "typescript" in devDeps) {
        console.error("  * Your program looks like a TypeScript program. Have you run 'tsc'?");
        return;
    }

    // Not all projects are typescript. If there's a main property, check that the file exists.
    if (mainProperty !== undefined && typeof mainProperty === "string") {
        const mainFile = path.join(packageRoot, mainProperty);
        if (!fs.existsSync(mainFile)) {
            console.error(`  * Your program's 'main' file (${mainFile}) does not exist.`);
            return;
        }
    }

    console.error("  * Pulumi encountered an unexpected error.");
    console.error(`    Raw exception message: ${error.message}`);
    return;
}

function tracingIsEnabled(tracingUrl: string | boolean): boolean {
    if (typeof tracingUrl !== "string") {
        return false;
    }
    const experimental = process.env["PULUMI_EXPERIMENTAL"] ?? "";
    const nonzeroLength = tracingUrl.length > 0;
    const experimentalEnabled = experimental.length > 0;
    return nonzeroLength && experimentalEnabled;
}

/** @internal */
export async function run(
    argv: minimist.ParsedArgs,
    programStarted: () => void,
    reportLoggedError: (err: Error) => void,
    isErrorReported: (err: Error) => boolean,
): Promise<Inputs | undefined> {
    const tracingUrl: string | boolean = argv["tracing"];
    // Start tracing. Before exiting, gracefully shutdown tracing, exporting
    // all remaining spans in the batch.
    if (tracingIsEnabled(tracingUrl)) {
        tracing.start(tracingUrl as string); // safe cast, since tracingIsEnable confirmed the type
        process.on("exit", tracing.stop);
    }
    // Start a new span, which we shutdown at the bottom of this method.
    const span = tracing.newSpan("language-runtime.run");

    // If there is a --pwd directive, switch directories.
    const pwd: string | undefined = argv["pwd"];
    if (pwd) {
        process.chdir(pwd);
    }

    // If this is a typescript project, we'll want to load node-ts.
    const typeScript: boolean = process.env["PULUMI_NODEJS_TYPESCRIPT"] === "true";

    // We provide reasonable defaults for many ts options, meaning you don't need to have a tsconfig.json present
    // if you want to use TypeScript with Pulumi. However, ts-node's default behavior is to walk up from the cwd to
    // find a tsconfig.json. For us, it's reasonable to say that the "root" of the project is the cwd,
    // if there's a tsconfig.json file here. Otherwise, just tell ts-node to not load project options at all.
    // This helps with cases like pulumi/pulumi#1772.
    const defaultTsConfigPath = "tsconfig.json";
    const tsConfigPath: string = process.env["PULUMI_NODEJS_TSCONFIG_PATH"] ? path.resolve(process.env["PULUMI_NODEJS_TSCONFIG_PATH"]) : defaultTsConfigPath;
    const skipProject = !fs.existsSync(tsConfigPath);

    span.setAttribute("typescript-enabled", typeScript);
    if (typeScript) {
        const compilerOptions = tsutils.loadTypeScriptCompilerOptions(tsConfigPath);

        // tanspileOnly controls wether ts-node should do type checking or not.
        // Users might have a separate build step that runs tsc for type
        // checking, and don't want to pay the performance cost of type checking
        // twice. This also enables using swc, which doesn' support type
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

        const { tsnodeRequire, typescriptRequire } = tsutils.typeScriptRequireStrings();
        const tsn: typeof tsnode = require(tsnodeRequire);
        tsn.register({
            compiler: typescriptRequire,
            transpileOnly,
            // PULUMI_NODEJS_TSCONFIG_PATH might be set to a config file such as "tsconfig.pulumi.yaml" which
            // would not get picked up by tsnode by default, so we explicitly tell tsnode which config file to
            // use (Which might just be ./tsconfig.yaml)
            project: tsConfigPath,
            skipProject: skipProject,
            compilerOptions: {
                target: "es6",
                module: "commonjs",
                moduleResolution: "node",
                sourceMap: "true",
                ...compilerOptions,
            },
        });
    }

    const hasEntrypoint = argv._[0] !== ".";
    let program: string = argv._[0];
    if (!path.isAbsolute(program)) {
        // If this isn't an absolute path, make it relative to the working directory.
        program = path.join(process.cwd(), program);
    }

    // Now fake out the process-wide argv, to make the program think it was run normally.
    const programArgs: string[] = argv._.slice(1);
    process.argv = [process.argv[0], process.argv[1], ...programArgs];

    // Set up the process uncaught exception, unhandled rejection, and program exit handlers.
    const uncaughtHandler = (err: Error) => {
        // In node, if you throw an error in a chained promise, but the exception is not finally
        // handled, then you can end up getting an unhandledRejection for each exception/promise
        // pair.  Because the exception is the same through all of these, we keep track of it and
        // only report it once so the user doesn't get N messages for the same thing.
        if (isErrorReported(err)) {
            return;
        }

        // First, log the error.
        if (RunError.isInstance(err)) {
            // Always hide the stack for RunErrors.
            log.error(err.message);
        } else if (err.name === "TSError" || err.name === SyntaxError.name) {
            // Hide stack frames as TSError/SyntaxError have messages containing
            // where the error is located
            const errOut = err.stack?.toString() || "";
            let errMsg = err.message;

            const errParts = errOut.split(err.message);
            if (errParts.length === 2) {
                errMsg = errParts[0] + err.message;
            }

            log.error(
                `Running program '${program}' failed with an unhandled exception:
${errMsg}`,
            );
        } else if (ResourceError.isInstance(err)) {
            // Hide the stack if requested to by the ResourceError creator.
            const message = err.hideStack ? err.message : defaultErrorMessage(err);
            log.error(message, err.resource);
        } else {
            log.error(
                `Running program '${program}' failed with an unhandled exception:
${defaultErrorMessage(err)}`,
            );
        }

        span.addEvent(`uncaughtError: ${err}`);
        reportLoggedError(err);
    };

    process.on("uncaughtException", uncaughtHandler);
    // @ts-ignore 'unhandledRejection' will almost always invoke uncaughtHandler with an Error. so
    // just suppress the TS strictness here.
    process.on("unhandledRejection", uncaughtHandler);
    process.on("exit", settings.disconnectSync);

    // Trigger callback to update a sentinel variable tracking
    // whether the program is running.
    programStarted();

    // This needs to occur after `programStarted` to ensure execution of the parent process stops.
    if (skipProject && tsConfigPath !== defaultTsConfigPath) {
        span.addEvent("Missing tsconfig file");
        return new Promise(() => {
            const e = new Error(`tsconfig path was set to ${tsConfigPath} but the file was not found`);
            e.stack = undefined;
            throw e;
        });
    }

    const containsTSAndJSModules = async (programPath: string) => {
        const programStats = await fs.promises.lstat(programPath);
        if (programStats.isDirectory()) {
            const programDirFiles = await fs.promises.readdir(programPath);
            return programDirFiles.includes("index.js") && programDirFiles.includes("index.ts");
        } else {
            return false;
        }
    };

    const runProgram = async () => {
        // We run the program inside this context so that it adopts all resources.
        //
        // IDEA: This will miss any resources created on other turns of the event loop.  I think that's a fundamental
        // problem with the current Component design though - not sure what else we could do here.
        //
        // Now go ahead and execute the code. The process will remain alive until the message loop empties.
        log.debug(`Running program '${program}' in pwd '${process.cwd()}' w/ args: ${programArgs}`);

        // Create a new span for the execution of the user program.
        const runProgramSpan = tracing.newSpan("language-runtime.runProgram");

        try {
            const packageRoot = await npmPackageRootFromProgramPath(program);
            const packageObject = packageObjectFromProjectRoot(packageRoot);
            let programExport: any;

            // If there is no entrypoint set in Pulumi.yaml via the main
            // option, look for an entrypoint defined in package.json
            if (!hasEntrypoint && packageObject["main"]) {
                const packageMainPath = path.join(packageRoot, packageObject["main"]);
                if (fs.existsSync(packageMainPath)) {
                    program = packageMainPath;
                } else {
                    log.warn(
                        `Could not find entry point '${packageMainPath}' specified in package.json; ` +
                            `using '${program}' instead`,
                    );
                }
            }

            // We use dynamic import instead of require for projects using native ES modules instead of commonjs
            if (packageObject["type"] === "module") {
                // Use the same behavior for loading the main entrypoint as `node <program>`.
                // See https://github.com/nodejs/node/blob/master/lib/internal/modules/run_main.js#L74.
                const mainPath: string =
                    require("module").Module._findPath(path.resolve(program), null, true) || program;
                const main = path.isAbsolute(mainPath) ? url.pathToFileURL(mainPath).href : mainPath;
                // Import the module and capture any module outputs it exported. Finally, await the value we get
                // back.  That way, if it is async and throws an exception, we properly capture it here
                // and handle it.
                programExport = await dynamicImport(main);
                // If there is a default export, use that instead of the named exports (and error if there are both).
                if (Object.getOwnPropertyDescriptor(programExport, "default") !== undefined) {
                    if (Object.keys(programExport).length !== 1) {
                        throw new Error(
                            "expected entrypoint module to have either a default export or named exports but not both",
                        );
                    }
                    programExport = programExport.default;
                }
            } else {
                // It's a CommonJS module, so require the module and capture any module outputs it exported.

                // If this is a folder ensure it ends with a "/" so we require the folder, not any adjacent .json file
                const programStats = await fs.promises.lstat(program);
                if (programStats.isDirectory() && !program.endsWith("/")) {
                    program = program + "/";
                }
                programExport = require(program);
            }

            if (await containsTSAndJSModules(program)) {
                log.warn(
                    "Found a TypeScript project containing an index.js file and no explicit entrypoint in Pulumi.yaml - Pulumi will use index.js",
                );
            }

            // Check compatible engines before running the program:
            const npmRc = npmRcFromProjectRoot(packageRoot);
            if (npmRc["engine-strict"] && packageObject.engines && packageObject.engines.node) {
                // found:
                //   - { engines: { node: "<version>" } } in package.json
                //   - engine-strict=true in .npmrc
                //
                // Check that current node version satistfies the required version
                const requiredNodeVersion = packageObject.engines.node;
                const currentNodeVersion = process.versions.node;
                if (!semver.satisfies(currentNodeVersion, requiredNodeVersion)) {
                    const errorMessage = [
                        `Your current Node version is incompatible to run ${packageRoot}`,
                        `Expected version: ${requiredNodeVersion} as found in package.json > engines > node`,
                        `Actual Node version: ${currentNodeVersion}`,
                        `To fix issue, install a Node version that is compatible with ${requiredNodeVersion}`,
                    ];

                    runProgramSpan.addEvent("Incompatible Node version");
                    throw new Error(errorMessage.join("\n"));
                }
            }

            // If the exported value was itself a Function, then just execute it.  This allows for
            // exported top level async functions that pulumi programs can live in.  Finally, await
            // the value we get back.  That way, if it is async and throws an exception, we properly
            // capture it here and handle it.
            const invokeResult = programExport instanceof Function ? programExport() : programExport;
            runProgramSpan.end();
            return await invokeResult;
        } catch (e) {
            // User JavaScript can throw anything, so if it's not an Error it's definitely
            // not something we want to catch up here.
            if (!(e instanceof Error)) {
                throw e;
            }

            // Give a better error message, if we can.
            const errorCode = (<any>e).code;
            if (errorCode === "MODULE_NOT_FOUND") {
                runProgramSpan.addEvent("Module Load Failure.");
                await reportModuleLoadFailure(program, e);
            }

            throw e;
        } finally {
            runProgramSpan.end();
        }
    };

    // Construct a `Stack` resource to represent the outputs of the program.
    const stackOutputs = await stack.runInPulumiStack(runProgram);
    await settings.disconnect();
    span.end();
    return stackOutputs;
}
