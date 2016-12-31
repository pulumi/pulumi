// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import { fs, log } from "nodets";
import * as os from "os";
import * as fspath from "path";
import * as ts from "typescript";

const TS_PROJECT_FILE = "tsconfig.json";

export interface ICompilation {
    root:        string;                 // the root directory for the compilation.
    tree:        ts.Program | undefined; // the resulting TypeScript program object.
    diagnostics: ts.Diagnostic[];        // any diagnostics resulting from compilation.
}

// Compiles a TypeScript program and returns its output.  The path can be one of three things: 1) a single TypeScript
// file (`*.ts`), 2) a TypeScript project file (`tsconfig.json`), or 3) a directory containing a TypeScript project
// file.  An optional set of compiler options may also be supplied.  In the project file cases, both options and files
// are read in the from the project file, and will override any options passed in the argument form.
export async function compile(path: string, options?: ts.CompilerOptions): Promise<ICompilation> {
    // Default the options to TypeScript's usual defaults if not provided.
    options = options || ts.getDefaultCompilerOptions();

    // See if we"re dealing with a tsproject.json file.  This happens when path directly points to one, or when
    // path refers to a directory, in which case we will assume we"re searching for a config file underneath it.
    let root: string | undefined;
    let configPath: string | undefined;
    if (fspath.basename(path) === TS_PROJECT_FILE) {
        configPath = path;
        root = fspath.dirname(path);
    }
    else if ((await fs.lstat(path)).isDirectory()) {
        configPath = fspath.join(path, TS_PROJECT_FILE);
        root = path;
    }
    else {
        root = fspath.dirname(path);
    }

    let files: string[] = [];
    let diagnostics: ts.Diagnostic[] = [];
    if (configPath) {
        // A config file is suspected; try to load it up and parse its contents.
        let config: any = JSON.parse(await fs.readFile(configPath));
        if (!config) {
            throw new Error(`No ${TS_PROJECT_FILE} found underneath the path ${path}`);
        }

        const parseConfigHost: ts.ParseConfigHost = new ParseConfigHost();
        const parsedConfig: ts.ParsedCommandLine = ts.parseJsonConfigFileContent(
            config, parseConfigHost, root, options);
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

    // Many options can be supplied, however, we want to hook the outputs to translate them on the fly.
    options.rootDir = root;
    options.outDir = undefined;
    options.declaration = false;

    if (log.v(5)) {
        log.out(5).infof(`files: ${JSON.stringify(files)}`);
        log.out(5).infof(`options: ${JSON.stringify(options, null, 4)}`);
    }
    if (log.v(7)) {
        options.traceResolution = true;
    }

    let program: ts.Program | undefined;
    if (diagnostics.length === 0) {
        // Create a compiler host and perform the compilation.
        const host: ts.CompilerHost = ts.createCompilerHost(options);
        host.writeFile = (_: string, __: string, ___: boolean) => { /*ignore outputs*/ };
        program = ts.createProgram(files, options, host);

        // Concatenate all of the diagnostics into a single array.
        diagnostics = diagnostics.concat(program.getSyntacticDiagnostics());
        if (diagnostics.length === 0) {
            diagnostics = diagnostics.concat(program.getOptionsDiagnostics());
            diagnostics = diagnostics.concat(program.getGlobalDiagnostics());
            if (diagnostics.length === 0) {
                diagnostics = diagnostics.concat(program.getSemanticDiagnostics());
                diagnostics = diagnostics.concat(program.getDeclarationDiagnostics());
            }
        }

        // Now perform the creation of the AST data structures.
        const emitOutput: ts.EmitResult = program.emit();
        diagnostics = diagnostics.concat(emitOutput.diagnostics);
    }

    return {
        root:        root,
        tree:        program,
        diagnostics: diagnostics,
    };
}

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

export function formatDiagnostics(comp: ICompilation): string {
    // TODO: implement colorization and fancy source context pretty-printing.
    return ts.formatDiagnostics(comp.diagnostics, new FormatDiagnosticsHost(comp.root));
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

