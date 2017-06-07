// Copyright 2017 Pulumi. All rights reserved.

"use strict";

import {contract, fs, log} from "nodejs-ts";
import * as os from "os";
import * as fspath from "path";
import * as ts from "typescript";
import * as diag from "../diag";
import * as pack from "../pack";

// Compiles a TypeScript program and returns its output.  The compiler settings are taken from the project file.
export async function compileScript(proj: pack.Manifest, root: string, path: string): Promise<Script> {
    let files: string[] = [];
    let diagnostics: ts.Diagnostic[] = [];

    // If there are custom language settings, use them.
    let options: ts.CompilerOptions | undefined;
    if ((proj.language && proj.language.settings) || proj.files) {
        let compilerConfig: any = {};
        if (proj.language && proj.language.settings) {
            compilerConfig["compilerOptions"] = proj.language.settings;
        }
        if (proj.files) {
            compilerConfig["files"] = proj.files;
        }

        const parseConfigHost: ts.ParseConfigHost = new ParseConfigHost();
        const parsedConfig: ts.ParsedCommandLine = ts.parseJsonConfigFileContent(
            compilerConfig, parseConfigHost, root, options);
        if (parsedConfig.errors.length > 0) {
            diagnostics = diagnostics.concat(parsedConfig.errors);
        }

        if (parsedConfig.options) {
            options = parsedConfig.options;
        }

        if (parsedConfig.fileNames) {
            files = files.concat(parsedConfig.fileNames);
        }
    } else {
        // Otherwise, assume it's a single file, and populate the paths with it.
        files.push(path);
    }

    // Default the options to TypeScript's usual defaults if not provided or discovered.
    options = options || ts.getDefaultCompilerOptions();

    if (log.v(5)) {
        log.out(5).info(`files: ${JSON.stringify(files)}`);
        log.out(5).info(`options: ${JSON.stringify(options, null, 4)}`);
    }
    if (log.v(7)) {
        options.traceResolution = true;
    }

    let tree: ts.Program | undefined;
    let outputs: ScriptOutputs | undefined;
    if (diagnostics.length === 0) {
        // Create a compiler host and perform the compilation.
        const host: ts.CompilerHost = ts.createCompilerHost(options);
        host.writeFile = (filename: string, data: string, writeBOM: boolean) => {
            contract.ignore(writeBOM); // TODO[pulumi/lumi#208]: consider respecting the BOM (for Windows).

            if (!outputs) {
                outputs = new Map<string, string>();
            }
            // Instead of writing the file, simply enter the it into the map entry, indexed by relative filename.  This
            // filename is relative to the root directory, by default, and the outDir directory otherwise.  We expect
            // callers to reconsutruct the absolute output target based on custom logic (and the compiler options).
            contract.assert(!outputs.has(filename));
            if (options && options.outDir) {
                filename = fspath.relative(options.outDir, filename);
            }
            else if (root) {
                filename = fspath.relative(root, filename);
            }
            outputs.set(filename, data);
        };
        tree = ts.createProgram(files, options, host);

        // Concatenate all of the diagnostics into a single array.
        diagnostics = diagnostics.concat(tree.getSyntacticDiagnostics());
        if (diagnostics.length === 0) {
            diagnostics = diagnostics.concat(tree.getOptionsDiagnostics());
            diagnostics = diagnostics.concat(tree.getGlobalDiagnostics());
            if (diagnostics.length === 0) {
                diagnostics = diagnostics.concat(tree.getSemanticDiagnostics());
                diagnostics = diagnostics.concat(tree.getDeclarationDiagnostics());
            }
        }

        // Now perform the creation of the AST data structures.
        const emitOutput: ts.EmitResult = tree.emit();
        diagnostics = diagnostics.concat(emitOutput.diagnostics);
    }

    return <Script>{
        proj:        proj,
        root:        root,
        files:       files,
        options:     options,
        diagnostics: transformDiagnostics(root, diagnostics),
        tree:        tree,
        outputs:     outputs,
    };
}

function transformDiagnostics(root: string, diagnostics: ts.Diagnostic[]): diag.Diagnostic[] {
    let result: diag.Diagnostic[] = [];
    let diagnosticsHost = new FormatDiagnosticsHost(root);
    for (let diagnostic of diagnostics) {
        // IDEA: implement colorization and fancy source context pretty-printing.
        result.push(<diag.Diagnostic>{
            category:     (diagnostic.category === ts.DiagnosticCategory.Warning) ?
                              diag.DiagnosticCategory.Warning : diag.DiagnosticCategory.Error,
            code:         diagnostic.code,
            message:      ts.formatDiagnostics([ diagnostic ], diagnosticsHost),
            preformatted: true, // prevent the usual formatting, just use TypeScript's.
        });
    }
    return result;
}

// The result of script compilation.
export interface Script {
    proj:        pack.Manifest;             // the project manifest.
    root:        string;                    // the root directory for the compilation.
    files:       string[];                  // the files that are considered part of this script's package.
    options:     ts.CompilerOptions;        // the compiler options used to compile this project.
    diagnostics: diag.Diagnostic[];         // any diagnostics resulting from compilation.
    tree:        ts.Program | undefined;    // the resulting TypeScript program object.
    outputs:     ScriptOutputs | undefined; // a collection of the resulting file outputs.
}

export type ScriptOutputs = Map<string, string>;

class ParseConfigHost implements ts.ParseConfigHost {
    public readonly useCaseSensitiveFileNames = isFilesystemCaseSensitive();

    public readDirectory(path: string, extensions: string[], exclude: string[], include: string[]): string[] {
        return ts.sys.readDirectory(path, extensions, exclude, include);
    }

    public fileExists(path: string): boolean {
        return ts.sys.fileExists(path);
    }

    public readFile(path: string): string {
        return ts.sys.readFile(path);
    }
}

class FormatDiagnosticsHost implements ts.FormatDiagnosticsHost {
    private readonly cwd: string;

    constructor(cwd: string) {
        this.cwd = cwd;
    }

    public getCurrentDirectory(): string {
        return this.cwd;
    }

    public getNewLine(): string {
        return os.EOL;
    }

    public getCanonicalFileName(filename: string): string {
        if (isFilesystemCaseSensitive()) {
            return filename;
        }
        else {
            return filename.toLowerCase();
        }
    }
}

function isFilesystemCaseSensitive(): boolean {
    let platform: string = os.platform();
    return platform === "win32" || platform === "win64";
}

