// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as colors from "colors";
import {contract} from "nodejs-ts";
import * as os from "os";
import * as fspath from "path";
import * as ts from "typescript";
import * as ast from "../ast";
import * as tokens from "../tokens";
import {Diagnostic, DiagnosticCategory} from "./diagnostic";

// A diagnostics context understands how to manipulate diagnostics using the location.
export class Context {
    private readonly root: string; // the root path that all diagnostics should be relative to.

    constructor(root: string) {
        this.root = root;
    }

    /** Formatting **/

    // Formats a specific diagnostic 
    public formatDiagnostic(d: Diagnostic, opts?: FormatOptions): string {
        // If the message is already formatted, return it as-is.
        // TODO: unify this formatting w/ TypeScript so they are uniform.
        if (d.preformatted) {
            let msg: string = d.message;
            // Strip off any trailing newline characters.
            while (msg.length >= os.EOL.length &&
                   msg.substring(msg.length - os.EOL.length) === os.EOL) {
                msg = d.message.substring(0, msg.length - os.EOL.length);
            }

            // Until we've unified formatting w/ TypeScript, we must retroactively parse and apply colors.
            if (opts && opts.colors) {
                let colorized: string;
                let catix: number | undefined;
                let category: DiagnosticCategory | undefined;
                for (let cat of [ DiagnosticCategory.Error, DiagnosticCategory.Warning ]) {
                    catix = msg.indexOf(`${DiagnosticCategory[cat].toLowerCase()} TS`);
                    if (catix !== -1) {
                        category = cat;
                        break;
                    }
                }
                if (catix === undefined || catix === -1) {
                    colorized = msg; // unrecognized format, just emit it as-is.
                }
                else {
                    colorized = "";

                    // See if there's a location part; if so, make it cyan.
                    if (catix !== 0) {
                        let pre: number = msg.indexOf(":");
                        contract.assert(pre !== -1);
                        colorized += colors.cyan(msg.substring(0, pre+1));
                        colorized += msg.substring(pre+1, catix);
                        msg = msg.substring(catix);
                    }

                    // Now highlight the error/warning accordingly.
                    let post: number = msg.indexOf(":");
                    contract.assert(post !== -1);
                    switch (category) {
                        case DiagnosticCategory.Error:
                            colorized += colors.red(msg.substring(0, post+1));
                            break;
                        case DiagnosticCategory.Warning:
                            colorized += colors.yellow(msg.substring(0, post+1));
                            break;
                        default:
                            contract.fail(`Unrecognized diagnostic category: ${category}`);
                    }
                    colorized += colors.white(msg.substring(post+1));

                    msg = colorized;
                }
            }

            return msg;
        }

        // Otherwise, format it in the usual ways.
        let s = d.message;
        if (opts && opts.colors) {
            s = colors.white(s);
        }

        // Now prepend both the category and the optional error number to the message.
        let category = DiagnosticCategory[d.category].toLowerCase();
        if (d.code) {
            category = `${category} MU${d.code}`;
        }

        // Append a delimiter (we want this colorized if enabled).
        category += ":";

        if (opts && opts.colors) {
            switch (d.category) {
                case DiagnosticCategory.Error:
                    category = colors.red(category);
                    break;
                case DiagnosticCategory.Warning:
                    category = colors.yellow(category);
                    break;
                default:
                    contract.fail(`Unexpected diagnostic category: ${d.category}`);
            }
        }

        s = `${category} ${s}`;

        // If there is a location part, prepend that to the whole thing (to come before the category/code).
        if (d.loc) {
            // TODO: implementfancy source context, range-based pretty-printing.
            let loc: string = `${d.loc.file}(${d.loc.start.line},${d.loc.start.column}):`;
            if (opts && opts.colors) {
                loc = colors.cyan(loc);
            }
            s = `${loc} ${s}`;
        }

        return s;
    }

    // Formats all of the diagnostics, separating each by a newline.
    public formatDiagnostics(ds: Diagnostic[], opts?: FormatOptions): string {
        let s: string = "";
        for (let d of ds) {
            if (s !== "") {
                s += os.EOL;
            }
            s += this.formatDiagnostic(d, opts);
        }
        return s;
    }

    /** General helper methods **/

    // This annotates a given MuPack/MuIL node with another TypeScript node's source position information.
    public withLocation<T extends ast.Node>(src: ts.Node, dst: T): T {
        dst.loc = this.locationFrom(src);
        // Despite mutating in place, we return the node to facilitate a more fluent style.
        return dst;
    }

    // This annotates a given MuPack/MuIL node with a range of TypeScript node source positions.
    public withLocationRange<T extends ast.Node>(start: ts.Node, end: ts.Node, dst: T): T {
        contract.assert(start.getSourceFile() === end.getSourceFile());
        // Turn the source file name into one relative to the current root path.
        let s: ts.SourceFile = start.getSourceFile();
        let relativePath: string = fspath.relative(this.root, s.fileName);
        dst.loc = {
            file:  relativePath,
            start: this.positionFrom(s, start.getStart()),
            end:   this.positionFrom(s, end.getEnd()),
        };

        // Despite mutating in place, we return the node to facilitate a more fluent style.
        return dst;
    }

    // Translates a TypeScript location into a MuIL location.
    private locationFrom(src: ts.Node): ast.Location {
        // Turn the source file name into one relative to the current root path.
        let s: ts.SourceFile = src.getSourceFile();
        let relativePath: string = fspath.relative(this.root, s.fileName);
        return <ast.Location>{
            file:  relativePath,
            start: this.positionFrom(s, src.getStart()),
            end:   this.positionFrom(s, src.getEnd()),
        };
    }

    // Translates a TypeScript position into a MuIL position (0 to 1 based lines).
    private positionFrom(s: ts.SourceFile, p: number): ast.Position {
        let lc = s.getLineAndCharacterOfPosition(p);
        return <ast.Position>{
            line:   lc.line + 1,  // transform to 1-based line number
            column: lc.character,
        };
    }

    /** Error factories **/

    public newMissingMufileError(path: string, exts: string[]): Diagnostic {
        let altExts: string = "";
        if (exts.length > 0) {
            path = path + exts[0];
            if (exts.length > 1) {
                altExts = " (or the alternative extensions: ";
                for (let i = 1; i < exts.length; i++) {
                    if (i > 1) {
                        altExts += ", ";
                    }
                    altExts += `'${exts[i]}'`;
                }
                altExts += ")";
            }
        }
        return {
            category: DiagnosticCategory.Error,
            code:     1,
            message:  `No Mufile was found at '${path}'${altExts}`,
        };
    }

    public newMissingMupackNameError(path: string): Diagnostic {
        return {
            category: DiagnosticCategory.Error,
            code:     2,
            message:  `Mufile '${path}' is missing a name`,
        };
    }

    public newMalformedMufileError(path: string, err: Error): Diagnostic {
        return {
            category: DiagnosticCategory.Error,
            code:     3,
            message:  `Mufile '${path}' is malformed: ${err}`,
        };
    }

    public newUnusedDependencyWarning(pkg: tokens.PackageToken): Diagnostic {
        return {
            category: DiagnosticCategory.Warning,
            code:     4,
            message:  `Package '${pkg}' was declared as a dependency but not used`,
        };
    }

    public newNoDefaultModuleWarning(): Diagnostic {
        return {
            category: DiagnosticCategory.Warning,
            code:     5,
            message:  "Package is missing a default module (either named 'index' or explicitly specified)",
        };
    }

    public newAsyncNotSupportedError(node: ts.Node): Diagnostic {
        return {
            category: DiagnosticCategory.Error,
            code:     100,
            message:  "Async functions are not supported in the MuJS subset",
            loc:      this.locationFrom(node),
        };
    }

    public newGeneratorsNotSupportedError(node: ts.Node): Diagnostic {
        return {
            category: DiagnosticCategory.Error,
            code:     101,
            message:  "Generator functions are not supported in the MuJS subset",
            loc:      this.locationFrom(node),
        };
    }

    public newRestParamsNotSupportedError(node: ts.Node): Diagnostic {
        return {
            category: DiagnosticCategory.Error,
            code:     102,
            message:  "Rest-style parameters are not supported in the MuJS subset",
            loc:      this.locationFrom(node),
        };
    }

    public newInvalidDeclarationStatementError(node: ts.Node): Diagnostic {
        return {
            category: DiagnosticCategory.Error,
            code:     500,
            message:  `Declaration node ${ts.SyntaxKind[node.kind]} isn't a valid MuJS statement`,
            loc:      this.locationFrom(node),
        };
    }

    public newInvalidTypeError(node: ts.Node, ty: ts.Type): Diagnostic {
        let name: string;
        if (ty.symbol) {
            name = `'${ty.symbol.name}' `;
        }
        else {
            name = "";
        }
        return {
            category: DiagnosticCategory.Error,
            code:     501,
            message:  `Type ${name}(kind ${ts.TypeFlags[ty.flags]}) is not supported in MuJS`,
            loc:      this.locationFrom(node),
        };
    }

    public newMissingDependencyError(pkg: tokens.PackageToken): Diagnostic {
        return {
            category: DiagnosticCategory.Error,
            code:     502,
            message:  `Package '${pkg}' was encountered during compilation, ` +
                `but wasn't listed as a dependency in the Mufile`,
        };
    }
}

// FormatOptions controls the kind of formatting to apply, such as colorization, extended diagnostics, etc.
export interface FormatOptions {
    colors?: boolean; // If true, the output will be colorized.
}

