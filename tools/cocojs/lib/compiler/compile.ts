// Copyright 2016 Pulumi. All rights reserved.

"use strict";

import {contract} from "nodejs-ts";
import * as diag from "../diag";
import * as pack from "../pack";
import {compileScript, Script, ScriptOutputs} from "./script";
import {transform, TransformResult} from "./transform";

// Compiles a TypeScript program, translates it into NutPack/NutIL, and returns the output.
//
// The path can be one of three things: 1) a single TypeScript file (`*.ts`), 2) a TypeScript project file
// (`tsconfig.json`), or 3) a directory containing a TypeScript project file.  An optional set of compiler options may
// also be supplied.  In the project file cases, both options and files are read in the from the project file, and will
// override any options passed in the argument form.
//
// If any errors occur, they will be returned in the form of diagnostics.  Unhandled exceptions should not occur unless
// something dramatic has gone wrong.  The resulting tree and pack objects may or may not be undefined, depending on
// what errors occur and during which phase of compilation they happen.
export async function compile(path: string): Promise<CompileResult> {
    // First perform the script compilation and analysis.
    let script: Script = await compileScript(path);
    let diagnostics: diag.Diagnostic[] = script.diagnostics;

    // Next, if there is a tree to transpile into NutPack/NutIL, then do it.
    let pkg: pack.Package | undefined;
    if (script.tree && diag.success(diagnostics)) {
        let result: TransformResult = await transform(script);
        pkg = result.pkg;
        diagnostics = diagnostics.concat(result.diagnostics);
    }

    // Collect up all of the definition files produced by the compiler, and bring them along.
    let definitions: ScriptOutputs | undefined;
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

    // Finally, return the overall result of the compilation.
    return new CompileResult(script.root, diagnostics, pkg, definitions, script.options.outDir);
}

export class CompileResult {
    private readonly dctx: diag.Context;

    constructor(public readonly root:          string,                    // the root path for the compilation.
                public readonly diagnostics:   diag.Diagnostic[],         // any diagnostics resulting from translation.
                public readonly pkg:           pack.Package | undefined,  // the resulting NutPack/NutIL AST.
                public readonly definitions:   ScriptOutputs | undefined, // the resulting TypeScript definition files.
                public readonly preferredOut?: string,                    // an optional preferred output location.
    ) {
        this.dctx = new diag.Context(root);
    }

    // Formats a specific diagnostic 
    public formatDiagnostic(index: number, opts?: diag.FormatOptions): string {
        contract.assert(index >= 0 && index < this.diagnostics.length);
        return this.dctx.formatDiagnostic(this.diagnostics[index], opts);
    }

    // Formats all of the diagnostics, separating each by a newline.
    public formatDiagnostics(opts?: diag.FormatOptions): string {
        return this.dctx.formatDiagnostics(this.diagnostics, opts);
    }
}

