// Copyright 2016-2018, Pulumi Corporation.
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

import * as fs from "fs";
import * as minimist from "minimist";
import * as path from "path";
import * as tsnode from "ts-node";
import { ResourceError, RunError } from "../../errors";
import * as log from "../../log";
import * as runtime from "../../runtime";

import * as mod from ".";

/**
 * Attempts to provide a detailed error message for module load failure if the
 * module that failed to load is the top-level module.
 * @param program The name of the program given to `run`, i.e. the top level module
 * @param error The error that occured. Must be a module load error.
 */
function reportModuleLoadFailure(program: string, error: Error): never {
    throwOrPrintModuleLoadError(program, error);

    // Note: from this point on, we've printed something to the user telling them about the
    // problem.  So we can let our langhost know it doesn't need to report any further issues.
    return process.exit(mod.nodeJSProcessExitedAfterLoggingUserActionableMessage);
}


function throwOrPrintModuleLoadError(program: string, error: Error): void {
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
    const stat = fs.lstatSync(program);
    let projectRoot: string;
    if (stat.isDirectory()) {
        projectRoot = program;
    } else {
        projectRoot = path.dirname(program);
    }

    let packageObject: Record<string, any>;
    try {
        const packageJson = path.join(projectRoot, "package.json");
        packageObject = require(packageJson);
    } catch {
        // This is all best-effort so if we can't load the package.json file, that's
        // fine.
        return;
    }

    console.error("Here's what we think went wrong:");

    // The objective here is to emit the best diagnostic we can, starting from the
    // most specific to the least specific.
    const deps = packageObject["dependencies"] || {};
    const devDeps = packageObject["devDependencies"] || {};
    const scripts = packageObject["scripts"] || {};
    const mainProperty  = packageObject["main"] || "index.js";

    // Is there a build script associated with this program? It's a little confusing that the
    // Pulumi CLI doesn't run build scripts before running the program so call that out
    // explicitly.

    if ("build" in scripts) {
        const command = scripts["build"];
        console.error(`  * Your program looks like it has a build script associated with it ('${command}').\n`);
        console.error("Pulumi does not run build scripts before running your program. " +
                        `Please run '${command}', 'yarn build', or 'npm run build' and try again.`);
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
        const mainFile = path.join(projectRoot, mainProperty);
        if (!fs.existsSync(mainFile)) {
            console.error(`  * Your program's 'main' file (${mainFile}) does not exist.`);
            return;
        }
    }

    console.error("  * Yowzas, our sincere apologies, we haven't seen this before!");
    console.error(`    Here is the raw exception message we received: ${error.message}`);
    return;
}

/** @internal */
export function run(argv: minimist.ParsedArgs,
                    programStarted: () => void,
                    reportLoggedError: (err: Error) => void,
                    isErrorReported: (err: Error) => boolean) {
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
    const skipProject = !fs.existsSync("tsconfig.json");

    if (typeScript) {
        tsnode.register({
            typeCheck: true,
            skipProject: skipProject,
            compilerOptions: {
                target: "es6",
                module: "commonjs",
                moduleResolution: "node",
                sourceMap: "true",
            },
        });
    }

    let program: string = argv._[0];
    if (program.indexOf("/") !== 0) {
        // If this isn't an absolute path, make it relative to the working directory.
        program = path.join(process.cwd(), program);
    }

    // Now fake out the process-wide argv, to make the program think it was run normally.
    const programArgs: string[] = argv._.slice(1);
    process.argv = [ process.argv[0], process.argv[1], ...programArgs ];

    // Set up the process uncaught exception, unhandled rejection, and program exit handlers.
    const uncaughtHandler = (err: Error) => {
        // In node, if you throw an error in a chained promise, but the exception is not finally
        // handled, then you can end up getting an unhandledRejection for each exception/promise
        // pair.  Because the exception is the same through all of these, we keep track of it and
        // only report it once so the user doesn't get N messages for the same thing.
        if (isErrorReported(err)) {
            return;
        }

        // Default message should be to include the full stack (which includes the message), or
        // fallback to just the message if we can't get the stack.
        //
        // If both the stack and message are empty, then just stringify the err object itself. This
        // is also necessary as users can throw arbitrary things in JS (including non-Errors).
        const defaultMessage = err.stack || err.message || ("" + err);

        // First, log the error.
        if (RunError.isInstance(err)) {
            // Always hide the stack for RunErrors.
            log.error(err.message);
        }
        else if (ResourceError.isInstance(err)) {
            // Hide the stack if requested to by the ResourceError creator.
            const message = err.hideStack ? err.message : defaultMessage;
            log.error(message, err.resource);
        }
        else {
            log.error(
`Running program '${program}' failed with an unhandled exception:
${defaultMessage}`);
        }

        reportLoggedError(err);
    };

    process.on("uncaughtException", uncaughtHandler);
    // @ts-ignore 'unhandledRejection' will almost always invoke uncaughtHandler with an Error. so
    // just suppress the TS strictness here.
    process.on("unhandledRejection", uncaughtHandler);
    process.on("exit", runtime.disconnectSync);

    programStarted();

    const runProgram = async () => {
        // We run the program inside this context so that it adopts all resources.
        //
        // IDEA: This will miss any resources created on other turns of the event loop.  I think that's a fundamental
        // problem with the current Component design though - not sure what else we could do here.
        //
        // Now go ahead and execute the code. The process will remain alive until the message loop empties.
        log.debug(`Running program '${program}' in pwd '${process.cwd()}' w/ args: ${programArgs}`);
        try {
            // Execute the module and capture any module outputs it exported. If the exported value
            // was itself a Function, then just execute it.  This allows for exported top level
            // async functions that pulumi programs can live in.  Finally, await the value we get
            // back.  That way, if it is async and throws an exception, we properly capture it here
            // and handle it.
            const reqResult = require(program);
            const invokeResult = reqResult instanceof Function
                ? reqResult()
                : reqResult;

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
                reportModuleLoadFailure(program, e);
            }

            throw e;
        }
    };

    // Construct a `Stack` resource to represent the outputs of the program.
    return runtime.runInPulumiStack(runProgram);
}
