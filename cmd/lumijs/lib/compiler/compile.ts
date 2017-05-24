// Copyright 2017 Pulumi. All rights reserved.

"use strict";

import {contract, fs} from "nodejs-ts";
import * as fspath from "path";
import * as diag from "../diag";
import * as pack from "../pack";
import {PackageLoader, PackageResult} from "./loader";
import {compileScript, Script, ScriptOutputs} from "./script";
import {transform, TransformResult} from "./transform";

// Compiles a TypeScript program, translates it into LumiPack/LumiIL, and returns the output.
//
// The path can be one of three things: 1) a single TypeScript file (`*.ts`), 2) a Lumi project file (`Lumi.json` or
// `Lumi.yaml`), or 3) a directory containing a Lumi project file.  An optional set of compiler options may also be
// supplied.  In the project file cases, both options and files are read in the from the project file, and will
// override any options passed in the argument form.
//
// If any errors occur, they will be returned in the form of diagnostics.  Unhandled exceptions should not occur unless
// something dramatic has gone wrong.  The resulting tree and pack objects may or may not be undefined, depending on
// what errors occur and during which phase of compilation they happen.
export async function compile(path: string): Promise<CompileResult> {
    let pkg: pack.Package | undefined;
    let definitions: ScriptOutputs | undefined;
    let preferredOut: string | undefined;

    // Detect the project root and load up the project manifest file.
    let loader = new PackageLoader();
    let proj: PackageResult = await detectProject(loader, path);
    let diagnostics: diag.Diagnostic[] = proj.diagnostics;

    // Now go ahead and perform the script compilation and analysis.
    if (proj.pkg && diag.success(diagnostics)) {
        let script: Script = await compileScript(proj.pkg, proj.root, path);
        diagnostics = diagnostics.concat(script.diagnostics);
        preferredOut = script.options.outDir;

        // Next, if there is a tree to transpile into LumiPack/LumiIL, then do it.
        if (script.tree && diag.success(diagnostics)) {
            let result: TransformResult = await transform(proj.pkg, script, loader);
            pkg = result.pkg;
            diagnostics = diagnostics.concat(result.diagnostics);
        }

        // Collect up all of the definition files produced by the compiler, and bring them along.
        if (script.outputs) {
            for (let output of script.outputs) {
                if (output[0].endsWith(".d.ts")) {
                    if (!definitions) {
                        definitions = new Map<string, string>();
                    }
                    definitions.set(output[0], output[1]);
                }
            }
        }
    }

    // Finally, return the overall result of the compilation.
    return new CompileResult(proj.root, diagnostics, pkg, definitions, preferredOut);
}

// detectProject discovers aproject root given a path.  The path can be one of three things: 1) a single TypeScript
// file (`*.ts`), 2) a Lumi project file (`Lumi.json` or `Lumi.yaml`), or 3) a directory containing a Lumi project.
async function detectProject(loader: PackageLoader, path: string): Promise<PackageResult> {
    // If the path refers to a directory, assume we're searching for a project file underneath it.
    let root: string | undefined;
    if ((await fs.lstat(path)).isDirectory()) {
        root = path;
    }
    else {
        root = fspath.dirname(path);
    }
    return await loader.loadCurrent(root);
}

export class CompileResult {
    private readonly dctx: diag.Context;

    constructor(public readonly root:          string,                    // the root path for the compilation.
                public readonly diagnostics:   diag.Diagnostic[],         // any diagnostics resulting from translation.
                public readonly pkg:           pack.Package | undefined,  // the resulting LumiPack/LumiIL AST.
                public readonly definitions:   ScriptOutputs | undefined, // the resulting TypeScript definition files.
                public readonly preferredOut?: string,                    // an optional preferred output location.
    ) {
        this.dctx = new diag.Context(root);
    }

    // Formats a specific diagnostic.
    public formatDiagnostic(index: number, opts?: diag.FormatOptions): string {
        contract.assert(index >= 0 && index < this.diagnostics.length);
        return this.dctx.formatDiagnostic(this.diagnostics[index], opts);
    }

    // Formats all of the diagnostics, separating each by a newline.
    public formatDiagnostics(opts?: diag.FormatOptions): string {
        return this.dctx.formatDiagnostics(this.diagnostics, opts);
    }
}

