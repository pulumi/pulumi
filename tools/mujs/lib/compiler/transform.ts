// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import {contract, log, object} from "nodejs-ts";
import * as fspath from "path";
import * as ts from "typescript";
import * as ast from "../ast";
import * as diag from "../diag";
import * as pack from "../pack";
import * as tokens from "../tokens";
import {PackageLoader, PackageResult} from "./loader";
import {Script} from "./script";

const defaultExport: string = "default"; // the ES6 default export name.

// A mapping from TypeScript binary operator to Mu AST operator.
let binaryOperators = new Map<ts.SyntaxKind, ast.BinaryOperator>([
    // Arithmetic
    [ ts.SyntaxKind.PlusToken,                                    "+"   ],
    [ ts.SyntaxKind.MinusToken,                                   "-"   ],
    [ ts.SyntaxKind.AsteriskToken,                                "*"   ],
    [ ts.SyntaxKind.SlashToken,                                   "/"   ],
    [ ts.SyntaxKind.PercentToken,                                 "%"   ],
    [ ts.SyntaxKind.AsteriskAsteriskToken,                        "**"  ],

    // Assignment
    [ ts.SyntaxKind.EqualsToken,                                  "="   ],
    [ ts.SyntaxKind.PlusEqualsToken,                              "+="  ],
    [ ts.SyntaxKind.MinusEqualsToken,                             "-="  ],
    [ ts.SyntaxKind.AsteriskEqualsToken,                          "*="  ],
    [ ts.SyntaxKind.SlashEqualsToken,                             "/="  ],
    [ ts.SyntaxKind.PercentEqualsToken,                           "%="  ],
    [ ts.SyntaxKind.AsteriskAsteriskEqualsToken,                  "**=" ],
    [ ts.SyntaxKind.LessThanLessThanEqualsToken,                  "<<=" ],
    [ ts.SyntaxKind.GreaterThanGreaterThanEqualsToken,            ">>=" ],
    [ ts.SyntaxKind.GreaterThanGreaterThanGreaterThanEqualsToken, ">>=" ], // TODO[marapongo/mu#50]: emulate >>>=.
    [ ts.SyntaxKind.AmpersandEqualsToken,                         "&="  ],
    [ ts.SyntaxKind.BarEqualsToken,                               "|="  ],
    [ ts.SyntaxKind.CaretEqualsToken,                             "^="  ],

    // Bitwise
    [ ts.SyntaxKind.LessThanLessThanToken,                        "<<"  ],
    [ ts.SyntaxKind.GreaterThanGreaterThanToken,                  ">>"  ],
    [ ts.SyntaxKind.GreaterThanGreaterThanGreaterThanToken,       ">>"  ], // TODO[marapongo/mu#50]: emulate >>>.
    [ ts.SyntaxKind.BarToken,                                     "|"   ],
    [ ts.SyntaxKind.CaretToken,                                   "^"   ],
    [ ts.SyntaxKind.AmpersandToken,                               "&"   ],

    // Logical
    [ ts.SyntaxKind.AmpersandAmpersandToken,                      "&&"  ],
    [ ts.SyntaxKind.BarBarToken,                                  "||"  ],

    // Relational
    [ ts.SyntaxKind.LessThanToken,                                "<"   ],
    [ ts.SyntaxKind.LessThanEqualsToken,                          "<="  ],
    [ ts.SyntaxKind.GreaterThanToken,                             ">"   ],
    [ ts.SyntaxKind.GreaterThanEqualsToken,                       ">="  ],
    [ ts.SyntaxKind.EqualsEqualsToken,                            "=="  ],
    [ ts.SyntaxKind.EqualsEqualsEqualsToken,                      "=="  ], // TODO[marapongo/mu#50]: emulate ===.
    [ ts.SyntaxKind.ExclamationEqualsToken,                       "!="  ],
    [ ts.SyntaxKind.ExclamationEqualsEqualsToken,                 "!="  ], // TODO[marapongo/mu#50]: emulate !==.

    // Intrinsics
    // TODO: [ ts.SyntaxKind.InKeyword,                           "in" ],
    // TODO: [ ts.SyntaxKind.InstanceOfKeyword,                   "instanceof" ],
]);

// A mapping from TypeScript unary prefix operator to Mu AST operator.
let prefixUnaryOperators = new Map<ts.SyntaxKind, ast.UnaryOperator>([
    [ ts.SyntaxKind.PlusPlusToken,    "++" ],
    [ ts.SyntaxKind.MinusMinusToken,  "--" ],
    [ ts.SyntaxKind.PlusToken,        "+"  ],
    [ ts.SyntaxKind.MinusToken,       "-"  ],
    [ ts.SyntaxKind.TildeToken,       "~"  ],
    [ ts.SyntaxKind.ExclamationToken, "!"  ],
]);

// A mapping from TypeScript unary postfix operator to Mu AST operator.
let postfixUnaryOperators = new Map<ts.SyntaxKind, ast.UnaryOperator>([
    [ ts.SyntaxKind.PlusPlusToken,    "++" ],
    [ ts.SyntaxKind.MinusMinusToken,  "--" ],
]);

// A variable is a MuIL variable with an optional initializer expression.  This is required because MuIL doesn't support
// complex initializers on the Variable AST node -- they must be explicitly placed into an initializer section.
class VariableDeclaration<TVariable extends ast.Variable> {
    constructor(
        public node:         ts.Node,        // the source node.
        public tok:          tokens.Token,   // the qualified token name for this variable.
        public variable:     TVariable,      // the MuIL variable information.
        public legacyVar?:   boolean,        // true to mimick legacy ECMAScript "var" behavior; false for "let".
        public initializer?: ast.Expression, // an optional initialization expression.
    ) { }
}

// A top-level module element is an export, module member (definition), or statement (initializer).
type ModuleElement = ast.ModuleMember | ast.Export | VariableDeclaration<ast.ModuleProperty> | ast.Statement;

// A top-level class element is either a class member (definition) or a statement (initializer).
type ClassElement = ast.ClassMember | VariableDeclaration<ast.ClassProperty>;

function isVariableDeclaration(element: ModuleElement | ClassElement): boolean {
    return !!(element instanceof VariableDeclaration);
}

// PackageInfo contains information about a module's package: both its token and its base path.
interface PackageInfo {
    root:  string;       // the root path from which the package was loaded.
    pkg:   pack.Package; // the package's metadata, including its token, etc.
}

const mujsStdlibPackage: tokens.PackageToken = "mujs"; // the MuJS standard library package.
const typeScriptStdlibPathPrefix: string = "/node_modules/typescript/lib/"; // the TypeScript library path part.
const typeScriptStdlibModulePrefix: string = "lib."; // only modules with this prefix are consiedered "standard".

// isTypeScriptStdlib indicates whether this module reference is to one of the TypeScript standard library headers.
function isTypeScriptStdlib(ref: ModuleReference): boolean {
    return (ref.indexOf(typeScriptStdlibPathPrefix+typeScriptStdlibModulePrefix) !== -1);
}

// getTypeScriptStdlibRoot extracts the root path of a TypeScript standard library module reference.
function getTypeScriptStdlibRoot(ref: ModuleReference): string {
    let stdlibIndex: number = ref.indexOf(typeScriptStdlibPathPrefix);
    contract.assert(stdlibIndex !== -1);
    return ref.substring(0, stdlibIndex+typeScriptStdlibPathPrefix.length);
}


// synthesizeMujsStdlibPackage creates a fake package that can be used to bind to MuJS standard library members.
function synthesizeMujsStdlibPackage(root: string): PackageInfo {
    return <PackageInfo>{
        root: getTypeScriptStdlibRoot(root),
        pkg: <pack.Package>{
            name: mujsStdlibPackage,
        },
    };
}

// ModuleReference represents a reference to an imported module.  It's really just a fancy, strongly typed string-based
// path that can be resolved to a concrete symbol any number of times before serialization.
type ModuleReference = string;

// A variable declaration isn't yet known to be a module or class property, and so it just contains the subset in common
// between them.  This facilitates code reuse in the translation passes.
interface VariableLikeDeclaration {
    name:         ast.Identifier;
    type?:        ast.TypeToken;
    readonly?:    boolean;
    legacyVar?:   boolean;
    initializer?: ast.Expression;
}

// A function declaration isn't yet known to be a module or class method, and so it just contains the subset that is
// common between them.  This facilitates code reuse in the translation passes.
interface FunctionLikeDeclaration {
    name:        ast.Identifier;
    parameters:  ast.LocalVariable[];
    body?:       ast.Block;
    returnType?: ast.TypeToken;
}

// TypeLike is any interface that has a possible TypeNode attached to it and can be queried for binding information.
interface TypeLike extends ts.Node {
    type?: ts.TypeNode;
}

function ident(id: string): ast.Identifier {
    return {
        kind:  ast.identifierKind,
        ident: id,
    };
}

// notYetImplemented simply fail-fasts, but does so in a way where we at least get Node source information.
function notYetImplemented(node: ts.Node | undefined, label?: string): never {
    let msg: string = `${node ? ts.SyntaxKind[node.kind] + " " : ""}Not Yet Implemented`;
    if (label) {
        msg += `[${label}]`;
    }
    if (node) {
        let s: ts.SourceFile = node.getSourceFile();
        let start: ts.LineAndCharacter = s.getLineAndCharacterOfPosition(node.getStart());
        let end: ts.LineAndCharacter = s.getLineAndCharacterOfPosition(node.getEnd());
        msg += `: ${s.fileName}(${start.line+1},${start.character})-(${end.line+1},${end.character})`;
    }
    return contract.fail(msg);
}

// A transpiler is responsible for transforming TypeScript program artifacts into MuPack/MuIL AST forms.
export class Transformer {
    // Immutable elements of the transformer that exist throughout an entire pass:
    private readonly pkg: pack.Manifest;             // the package's manifest.
    private readonly script: Script;                 // the package's compiled TypeScript tree and context.
    private readonly dctx: diag.Context;             // the diagnostics context.
    private readonly diagnostics: diag.Diagnostic[]; // any diagnostics encountered during translation.
    private readonly loader: PackageLoader;          // a loader for resolving dependency packages.

    // Cached symbols required during type checking:
    private readonly builtinObjectType: ts.InterfaceType;          // the ECMA/TypeScript built-in object type.
    private readonly builtinArrayType: ts.InterfaceType;           // the ECMA/TypeScript built-in array type.
    private readonly builtinMapType: ts.InterfaceType | undefined; // the ECMA/TypeScript built-in map type.

    // A lookaside cache of resolved modules to their associated MuPackage metadata:
    private modulePackages: Map<ModuleReference, Promise<PackageInfo>>;

    // Mutable elements of the transformer that are pushed/popped as we perform visitations:
    private currentSourceFile: ts.SourceFile | undefined;
    private currentModuleToken: tokens.ModuleToken | undefined;
    private currentModuleMembers: ast.ModuleMembers | undefined;
    private currentModuleExports: ast.ModuleExports | undefined;
    private currentModuleImports: Map<string, ModuleReference>;
    private currentModuleImportTokens: ast.ModuleToken[];
    private currentClassToken: tokens.TypeToken | undefined;
    private currentSuperClassToken: tokens.TypeToken | undefined;
    private currentPackageDependencies: Set<tokens.PackageToken>;
    private currentTempLocalCounter: number = 0;

    constructor(pkg: pack.Manifest, script: Script, loader: PackageLoader) {
        contract.requires(!!pkg, "pkg", "A package manifest must be supplied");
        contract.requires(!!pkg.name, "pkg.name", "A package must have a valid name");
        contract.requires(!!script.tree, "script", "A valid MuJS AST is required to lower to MuPack/MuIL");
        this.pkg = pkg;
        this.script = script;
        this.dctx = new diag.Context(script.root);
        this.diagnostics = [];
        this.loader = loader;
        this.modulePackages = new Map<ModuleReference, Promise<PackageInfo>>();

        // Cache references to some important global symbols.
        //      - Object, used both for explicit weakly-typed Object references.
        this.builtinObjectType = this.getBuiltinType("Object", 0);
        //      - Array<T>, used both for explicit "Array<T>" references and simple "T[]"s.
        this.builtinArrayType = this.getBuiltinType("Array", 1);
        //      - Map<K, V>, used for ES6-style maps; when targeting pre-ES6, it might be missing.
        this.builtinMapType = this.getOptionalBuiltinType("Map", 2);
    }

    // Translates a TypeScript bound tree into its equivalent MuPack/MuIL AST form, one module per file.  This method is
    // asynchronous because it may need to perform I/O in order to fully resolve dependency packages.
    public async transform(): Promise<TransformResult> {
        let priorPackageDependencies: Set<tokens.PackageToken> | undefined = this.currentPackageDependencies;
        try {
            // Keep track of all transform package dependencies.
            this.currentPackageDependencies = new Set<tokens.PackageToken>();

            // Enumerate all source files (each of which is a module in ECMAScript), and transform it.
            let modules: ast.Modules = {};
            let aliases: pack.ModuleAliases = {};
            for (let sourceFile of this.script.tree!.getSourceFiles()) {
                // TODO[marapongo/mu#52]: to determine whether a SourceFile is part of the current compilation unit, we
                // must rely on a private TypeScript API, isSourceFileFromExternalLibrary.  An alternative would be to
                // check to see if the file was loaded from the node_modules/ directory, which is essentially what the
                // TypeScript compiler does (except that it has logic for nesting and symbolic links that'd be hard to
                // emulate).  Neither approach is great, however, I prefer to use the API and assert that it exists so
                // we match the semantics.  Thankfully, the tsserverlib library will contain these, once it is useable.
                let isSourceFileFromExternalLibrary =
                    <((file: ts.SourceFile) => boolean)>(<any>this.script.tree).isSourceFileFromExternalLibrary;
                contract.assert(!!isSourceFileFromExternalLibrary,
                                "Expected internal Program.isSourceFileFromExternalLibrary function to be non-null");
                if (!isSourceFileFromExternalLibrary(sourceFile) && !sourceFile.isDeclarationFile) {
                    let mod: ast.Module = await this.transformSourceFile(sourceFile);
                    let modname: string = mod.name.ident;
                    modules[modname] = mod;
                    if (modname === "index") {
                        // The special index module is the package's main/default module.
                        // TODO[marapongo/mu#57]: respect the package.json "main" specifier, if it exists.
                        aliases[tokens.defaultModule] = modname;
                    }
                    else if (modname.endsWith("/index")) {
                        // Any module whose name is of the form ".../index" can also be accessed as just "...".
                        aliases[modname.substring(0, modname.lastIndexOf("/index"))] = modname;
                    }
                }
            }

            // Afterwards, ensure that all dependencies encountered were listed in the MuPackage manifest.
            for (let dep of this.currentPackageDependencies) {
                if (dep !== this.pkg.name) {
                    if (!this.pkg.dependencies || !this.pkg.dependencies[dep]) {
                        // If the reference is the MuJS standard library, we will auto-generate one for the user.
                        // TODO: rather than using "*" as the version, take the version we actually compiled against.
                        if (dep === mujsStdlibPackage) {
                            if (!this.pkg.dependencies) {
                                this.pkg.dependencies = {};
                            }
                            this.pkg.dependencies[dep] = "*";
                        }
                        else {
                            this.diagnostics.push(this.dctx.newMissingDependencyError(dep));
                        }
                    }
                }
            }

            // Also warn about dependency packages that weren't actually used.
            if (this.pkg.dependencies) {
                for (let dep of Object.keys(this.pkg.dependencies)) {
                    if (!this.currentPackageDependencies.has(dep)) {
                        this.diagnostics.push(this.dctx.newUnusedDependencyWarning(dep));
                    }
                }
            }

            // Give a warning if this package didn't have a default module; technically, this is fine, but it is most
            // likely a mistake as it will cause complications for consumers of it.
            if (!aliases[tokens.defaultModule]) {
                this.diagnostics.push(this.dctx.newNoDefaultModuleWarning());
            }

            // Now create a new package object.
            return <TransformResult>{
                diagnostics: this.diagnostics,
                pkg:         object.extend(this.pkg, {
                    modules: modules,
                    aliases: aliases,
                }),
            };
        }
        finally {
            this.currentPackageDependencies = priorPackageDependencies;
        }
    }

    /** Helpers **/

    // checker returns the TypeScript type checker object, to inspect semantic bound information on the nodes.
    private checker(): ts.TypeChecker {
        contract.assert(!!this.script.tree);
        return this.script.tree!.getTypeChecker();
    }

    // globals returns the TypeScript globals symbol table.
    private globals(flags: ts.SymbolFlags): Map<string, ts.Symbol> {
        // TODO[marapongo/mu#52]: we are abusing getSymbolsInScope to access the global symbol table, because TypeScript
        //     doesn't expose it.  It is conceivable that the undefined 1st parameter will cause troubles some day.
        let globals = new Map<string, ts.Symbol>();
        for (let sym of this.checker().getSymbolsInScope(<ts.Node><any>undefined, flags)) {
            globals.set(sym.name, sym);
        }
        return globals;
    }

    // getOptionalBuiltinType searches the global symbol table for an interface type with the given name and type
    // parameter count.  It asserts that these properties are true; it is a programming error if they are not.
    private getOptionalBuiltinType(name: string, typeParameterCount: number): ts.InterfaceType | undefined {
        let globals: Map<string, ts.Symbol> = this.globals(ts.SymbolFlags.Interface);
        let builtin: ts.Symbol | undefined = globals.get(name);
        if (builtin) {
            contract.assert(!!(builtin.flags & ts.SymbolFlags.Interface),
                            `Expected built-in '${name}' type to be an interface`);
            let builtinType = <ts.InterfaceType | undefined>this.checker().getDeclaredTypeOfSymbol(builtin);
            contract.assert(!!builtinType,
                            `Expected '${name}' symbol conversion to yield a valid type`);
            let actualTypeParameterCount: number =
                builtinType!.typeParameters ? builtinType!.typeParameters.length : 0;
            contract.assert(actualTypeParameterCount === typeParameterCount,
                            `Expected '${name}' type to have generic arity ${typeParameterCount}; ` +
                            `got ${actualTypeParameterCount} instead`);
            return builtinType!;
        }
        return undefined;
    }

    // getBuiltinType is like getOptionalBuiltinType, but fails if the type is missing.
    private getBuiltinType(name: string, typeParameterCount: number): ts.InterfaceType {
        let builtinType: ts.InterfaceType | undefined = this.getOptionalBuiltinType(name, typeParameterCount);
        contract.assert(!!builtinType, `Expected to find required builtin type '${name}'`);
        return builtinType!;
    }

    // isLocalVariableOrFunction tells us whether a symbol is a local one (versus, say, belonging to a class or module).
    private isLocalVariableOrFunction(symbol: ts.Symbol): boolean {
        if (!symbol.declarations) {
            return false;
        }
        for (let decl of symbol.declarations) {
            // All locals should be one of these kinds.
            if (decl.kind !== ts.SyntaxKind.FunctionExpression &&
                    decl.kind !== ts.SyntaxKind.VariableDeclaration &&
                    decl.kind !== ts.SyntaxKind.Parameter &&
                    decl.kind !== ts.SyntaxKind.FunctionDeclaration) {
                return false;
            }
            // All locals should have functions in the parent tree before modules.
            let parent: ts.Node | undefined = decl.parent;
            while (parent) {
                switch (parent.kind) {
                    // These are function-like and qualify as local parents.
                    case ts.SyntaxKind.Block:
                    case ts.SyntaxKind.Constructor:
                    case ts.SyntaxKind.FunctionExpression:
                    case ts.SyntaxKind.FunctionDeclaration:
                    case ts.SyntaxKind.ArrowFunction:
                    case ts.SyntaxKind.MethodDeclaration:
                    case ts.SyntaxKind.MethodSignature:
                    case ts.SyntaxKind.GetAccessor:
                    case ts.SyntaxKind.SetAccessor:
                    case ts.SyntaxKind.CallSignature:
                    case ts.SyntaxKind.ConstructSignature:
                    case ts.SyntaxKind.IndexSignature:
                    case ts.SyntaxKind.FunctionType:
                    case ts.SyntaxKind.ConstructorType:
                        parent = undefined;
                        break;

                    // These are the top of module definitions and disqualify this as a local.
                    case ts.SyntaxKind.SourceFile:
                    case ts.SyntaxKind.ModuleBlock:
                        return false;

                    // Otherwise, keep on searching.
                    default:
                        parent = parent.parent;
                        break;
                }
            }
        }
        return true;
    }

    // generateTempLocal generates a name that won't conflict with user-authored names.
    private generateTempLocal(meaningful: string): string {
        let index: number = this.currentTempLocalCounter++;
        // Meaningful is a meaningful part of the name, but it can't be used along.  Prefix with `.` so it doesn't
        // conflict w/ user names; suffix it with a counter so it doesn't conflict with other similarly generated names.
        return `.temp_${meaningful}_${index}`;
    }

    // createLoadLocal generates a load of a local variable with the given name.
    private createLoadLocal(name: string, node?: ts.Node): ast.LoadLocationExpression {
        let load = <ast.LoadLocationExpression>{
            kind: ast.loadLocationExpressionKind,
            name: <ast.Token>{
                kind: ast.tokenKind,
                tok:  name,
            },
        };
        if (node) {
            load = this.withLocation(node, load);
        }
        return load;
    }

    // This just carries forward an existing location from another node.
    private copyLocation<T extends ast.Node>(src: ast.Node, dst: T): T {
        dst.loc = src.loc;
        return dst;
    }

    // This just carries forward an existing location range from another node.
    private copyLocationRange<T extends ast.Node>(start: ast.Node, end: ast.Node, dst: T): T {
        let sloc: ast.Location | undefined = start.loc || end.loc;
        let eloc: ast.Location | undefined = end.loc || start.loc;
        if (sloc !== undefined && eloc !== undefined) {
            contract.assert(sloc.file === eloc.file);
            dst.loc = {
                file:  sloc.file,
                start: sloc.start,
                end:   eloc.end,
            };
        }
        return dst;
    }

    // This annotates a given MuPack/MuIL node with another TypeScript node's source position information.
    private withLocation<T extends ast.Node>(src: ts.Node, dst: T): T {
        return this.dctx.withLocation<T>(src, dst);
    }

    // This annotates a given MuPack/MuIL node with a range of TypeScript node source positions.
    private withLocationRange<T extends ast.Node>(start: ts.Node, end: ts.Node, dst: T): T {
        return this.dctx.withLocationRange<T>(start, end, dst);
    }

    /** Transformations **/

    /** Semantics and tokens **/

    // getModulePackageInfo get a package token from the given module reference.  This info contains the package's name,
    // which is required for fully bound tokens, in addition to its base path, which is required to create a module
    // token that is relative to it.  This routine caches lookups since we'll be doing them frequently.
    private async getModulePackageInfo(ref: ModuleReference): Promise<PackageInfo> {
        let pkginfo: Promise<PackageInfo> | undefined = this.modulePackages.get(ref);
        contract.assert(!!pkginfo || ref !== tokens.selfModule); // expect selfrefs to always resolve.
        if (!pkginfo) {
            // For references to the TypeScript library, hijack and redirect them to the MuJS runtime library.
            if (isTypeScriptStdlib(ref)) {
                // TODO[marapongo/mu#87]: at the moment, we unconditionally rewrite references.  This leads to silent
                //     compilation of things that could be missing (either intentional, like `eval`, or simply because
                //     we haven't gotten around to implementing them yet).  Ideally we would reject the MuJS compilation
                //     later on during type token binding so that developers get a better experience.
                let stdlib: PackageInfo = synthesizeMujsStdlibPackage(ref);
                pkginfo = Promise.resolve(stdlib);
                this.currentPackageDependencies.add(stdlib.pkg.name);
            }
            else {
                // Register the promise for loading this package, to ensure interleavings pile up correctly.
                pkginfo = (async () => {
                    let base: string = fspath.dirname(ref);
                    let disc: PackageResult = await this.loader.loadDependency(base);
                    if (disc.diagnostics) {
                        for (let diagnostic of disc.diagnostics) {
                            this.diagnostics.push(diagnostic);
                        }
                    }
                    if (disc.pkg) {
                        // Track this package as a dependency.
                        this.currentPackageDependencies.add(disc.pkg.name);
                    }
                    else {
                        // If there was no package, an error is expected; stick a reference in here so we have a name.
                        contract.assert(disc.diagnostics && disc.diagnostics.length > 0);
                        disc.pkg = { name: tokens.selfModule };
                    }
                    return <PackageInfo>{
                        root: disc.root,
                        pkg:  disc.pkg,
                    };
                })();
            }

            // Memoize this result.  Note that this is the promise, not the actual resolved package, so that we don't
            // attempt to load the same package multiple times when we are waiting for I/Os to complete.
            this.modulePackages.set(ref, pkginfo);
        }

        return pkginfo;
    }

    // createModuleToken turns a module reference -- which encodes a module's fully qualified import path, so that it
    // can be resolved and reresolved any number of times -- into a ModuleToken suitable for long-term serialization.
    private async createModuleToken(ref: ModuleReference): Promise<tokens.ModuleToken> {
        // First figure out which package this reference belongs to.
        let pkginfo: PackageInfo = await this.getModulePackageInfo(ref);
        return this.createModuleTokenFromPackage(ref, pkginfo);
    }

    private createModuleTokenFromPackage(ref: ModuleReference, pkginfo: PackageInfo): tokens.ModuleToken {
        let moduleName: string;
        if (ref === tokens.selfModule) {
            // Simply use the name for the current module.
            contract.assert(!!this.currentModuleToken);
            moduleName = this.getModuleName(this.currentModuleToken!);
        }
        else {
            // To create a module name, make it relative to the package root, and lop off the extension.
            // TODO(joe): this still isn't 100% correct, because we might have ".."s for "up and over" references.
            //     We should consult the dependency list to ensure that it exists, and use that for normalization.
            moduleName = fspath.relative(pkginfo.root, ref);

            // If the module contains a ".js", ".d.ts", or ".ts" on the end of it, strip it off.
            for (let suffix of [ ".js", ".d.ts", ".ts" ]) {
                if (moduleName.endsWith(suffix)) {
                    moduleName = moduleName.substring(0, moduleName.length-suffix.length);
                    break;
                }
            }
        }

        // The module token is the package name, plus a delimiter, plus the module name.
        return `${pkginfo.pkg.name}${tokens.tokenDelimiter}${moduleName}`;
    }

    // getModuleName extracts a name from the given module token as a string.
    private getModuleName(tok: tokens.ModuleToken): string {
        return tok.substring(tok.indexOf(tokens.tokenDelimiter)+1);
    }

    // getPackageFromModule extracts the package name from a given module token.
    private getPackageFromModule(tok: tokens.ModuleToken): tokens.PackageToken {
        return tok.substring(0, tok.indexOf(tokens.tokenDelimiter));
    }

    // createModuleMemberToken binds a string-based exported member name to the associated token that references it.
    private createModuleMemberToken(modtok: tokens.ModuleToken, member: string): tokens.ModuleMemberToken {
        // The concatenated name of the module plus identifier will resolve correctly to an exported definition.
        return `${modtok}${tokens.tokenDelimiter}${member}`;
    }

    // createModuleRefMemberToken binds a string-based exported member name to the associated token that references it.
    private async createModuleRefMemberToken(mod: ModuleReference, member: string): Promise<tokens.ModuleMemberToken> {
        let modtok: tokens.ModuleToken = await this.createModuleToken(mod);
        return this.createModuleMemberToken(modtok, member);
    }

    // createClassMemberToken binds a string-based exported member name to the associated token that references it.
    private createClassMemberToken(classtok: tokens.ModuleMemberToken, member: string): tokens.ClassMemberToken {
        contract.assert(classtok !== tokens.dynamicType);
        // The concatenated name of the class plus identifier will resolve correctly to an exported definition.
        return `${classtok}${tokens.tokenDelimiter}${member}`;
    }

    // createModuleReference turns a ECMAScript import path into a MuIL module token.
    private createModuleReference(sym: ts.Symbol): ModuleReference {
        contract.assert(!!(sym.flags & (ts.SymbolFlags.ValueModule | ts.SymbolFlags.NamespaceModule)),
                        `Symbol is not a module: ${ts.SymbolFlags[sym.flags]}`);
        return this.createModuleReferenceFromPath(sym.name);
    }

    // createModuleReferenceFromPath turns a ECMAScript import path into a MuIL module reference.
    private createModuleReferenceFromPath(path: string): ModuleReference {
        // Module paths can be enclosed in quotes; eliminate them.
        if (path && path[0] === "\"") {
            path = path.substring(1);
        }
        if (path && path[path.length-1] === "\"") {
            path = path.substring(0, path.length-1);
        }
        return path;
    }

    // extractMemberToken returns just the member part of a fully qualified token, leaving off the module part.
    private extractMemberToken(token: tokens.Token): tokens.Token {
        let memberIndex: number = token.lastIndexOf(tokens.tokenDelimiter);
        if (memberIndex !== -1) {
            token = token.substring(memberIndex+1);
        }
        return token;
    }

    // getResolvedModules returns the current SourceFile's known modules inside of a map.
    private getResolvedModules(): ts.Map<ts.ResolvedModuleFull> {
        // TODO[marapongo/mu#52]: we are grabbing the sourceContext's resolvedModules property directly, because
        //     TypeScript doesn't currently offer a convenient way of accessing this information.  The (unexported)
        //     getResolvedModule function almost does this, but not quite, because it doesn't allow us to look up
        //     based on path.  Ideally we can remove this as soon as the tsserverlibrary is consumable as a module.
        let modules = <ts.Map<ts.ResolvedModuleFull>>(<any>this.currentSourceFile).resolvedModules;
        contract.assert(!!modules, "Expected internal SourceFile.resolvedModules property to be non-null");
        return modules;
    }

    // getResolvedModuleSymbol turns a TypeScript module descriptor into a real symbol.
    private getResolvedModuleSymbol(mod: ts.ResolvedModuleFull): ts.Symbol {
        let moduleFile: ts.SourceFile = this.script.tree!.getSourceFile(mod.resolvedFileName);
        let moduleSymbol: ts.Symbol = this.checker().getSymbolAtLocation(moduleFile);
        contract.assert(!!moduleSymbol, `Expected '${mod.resolvedFileName}' module to resolve to a symbol`);
        return moduleSymbol;
    }

    // resolveModuleSymbol binds either a name or a path to an associated module symbol.
    private resolveModuleSymbol(name?: string, path?: string): ts.Symbol {
        // Resolve the module name to a real symbol.
        // TODO(joe): ensure that this dependency exists, to avoid "accidentally" satisfyied name resolution in the
        //     TypeScript compiler; for example, if the package just happens to exist in `node_modules`, etc.
        let candidates: ts.Map<ts.ResolvedModuleFull> = this.getResolvedModules();
        let resolvedModule: ts.ResolvedModuleFull | undefined;
        for (let candidateName of Object.keys(candidates)) {
            let candidate: ts.ResolvedModuleFull = candidates[candidateName];
            if ((name && candidateName === name) ||
                    (path && (candidate.resolvedFileName === path || candidate.resolvedFileName === path+".ts"))) {
                resolvedModule = candidate;
                break;
            }
        }
        contract.assert(!!resolvedModule, `Expected '${name}|${path}' to resolve to a module`);
        return this.getResolvedModuleSymbol(resolvedModule!);
    }

    // resolveModuleSymbolByName binds a string-based module path to the associated symbol.
    private resolveModuleSymbolByName(name: string): ts.Symbol {
        return this.resolveModuleSymbol(name);
    }

    // resolveModuleSymbolByPath binds a string-based module path to the associated symbol.
    private resolveModuleSymbolByPath(path: string): ts.Symbol {
        return this.resolveModuleSymbol(undefined, path);
    }

    // resolveModuleReferenceByName binds a string-based module name to the associated token that references it.
    private resolveModuleReferenceByName(name: string): ModuleReference {
        let moduleSymbol: ts.Symbol = this.resolveModuleSymbol(name);
        return this.createModuleReference(moduleSymbol);
    }

    // resolveModuleReferenceByPath binds a string-based module path to the associated token that references it.
    private resolveModuleReferenceByPath(path: string): ModuleReference {
        let moduleSymbol: ts.Symbol = this.resolveModuleSymbol(undefined, path);
        return this.createModuleReference(moduleSymbol);
    }

    // resolveModuleReferenceByFile binds a TypeScript SourceFile path to the associated token that references it.
    private resolveModuleReferenceByFile(file: ts.SourceFile): ModuleReference {
        let moduleSymbol: ts.Symbol = this.resolveModuleSymbol(undefined, file.fileName);
        return this.createModuleReference(moduleSymbol);
    }

    // resolveModuleExportNames binds a module token to the set of tokens that it exports.
    private async resolveModuleExportNames(mod: ModuleReference): Promise<tokens.Token[]> {
        let exports: tokens.Token[] = [];

        // Resolve the module name to a real symbol.
        let moduleSymbol: ts.Symbol = this.resolveModuleSymbolByPath(mod);
        contract.assert(
            mod === this.createModuleReference(moduleSymbol),
            `Expected discovered module '${this.createModuleReference(moduleSymbol)}' to equal '${mod}'`,
        );
        for (let expsym of this.checker().getExportsOfModule(moduleSymbol)) {
            exports.push(await this.createModuleRefMemberToken(mod, expsym.name));
        }

        return exports;
    }

    // simplifyTypeForToken attempts to simplify a type for purposes of token emission.
    private simplifyTypeForToken(ty: ts.Type): [ ts.Type, ts.TypeFlags ] {
        if (ty.flags & ts.TypeFlags.StringLiteral) {
            return [ ty, ts.TypeFlags.String ]; // string literals just become strings.
        }
        else if (ty.flags & ts.TypeFlags.NumberLiteral) {
            return [ ty, ts.TypeFlags.Number ]; // number literals just become numbers.
        }
        else if (ty.flags & ts.TypeFlags.BooleanLiteral) {
            return [ ty, ts.TypeFlags.Boolean ]; // boolean literals just become booleans.
        }
        else if (ty.flags & ts.TypeFlags.EnumLiteral) {
            // Desugar enum literal types to their base types.
            return this.simplifyTypeForToken((<ts.EnumLiteralType>ty).baseType);
        }
        else if (ty.flags & ts.TypeFlags.Union) {
            // If the type is a union type, see if we can flatten it into a single type:
            //
            //     1) All members of the union that are literal types of the same root type (e.g., all StringLiterals
            //        which can safely compress to Strings) can be compressed just to the shared root type.
            //        TODO[marapongo/mu#82]: eventually, we will support union and literal types natively.
            //
            //     2) Any `undefined` or `null` types can be stripped out.  The reason is that everything in MuIL is
            //        nullable at the moment.  (Note that the special case of `T?` as an interface property is encoded
            //        as `T|undefined`.)  The result is that we just yield just the underlying naked type.
            //        TODO[marapongo/mu#64]: eventually we want to consider supporting nullability in a first class way.
            //
            let union = <ts.UnionOrIntersectionType>ty;
            let result: ts.Type | undefined;
            let resultFlags: ts.TypeFlags | undefined;
            for (let uty of union.types) {
                // Simplify the type first.
                let simple: ts.Type;
                let flags: ts.TypeFlags;
                [ simple, flags ] = this.simplifyTypeForToken(uty);

                if (flags & ts.TypeFlags.Undefined || flags & ts.TypeFlags.Null) {
                    // Skip undefined and null types.
                }
                else {
                    // Now choose this as our result checking for conflicts.  Conflicts around primitives -- string,
                    // number, or boolean -- are permitted because they are harmless and expected due to simplification.
                    if (result && resultFlags &&
                            !(flags & (ts.TypeFlags.String | ts.TypeFlags.Number | ts.TypeFlags.Boolean))) {
                        result = undefined;
                        resultFlags = undefined;
                        break;
                    }
                    result = simple;
                    resultFlags = flags;
                }
            }
            if (result && resultFlags) {
                return [ result, resultFlags ];
            }
        }

        // Otherwise, we fell through, just use the real type and its flags.
        return [ ty, ty.flags ];
    }

    // resolveTypeToken takes a concrete TypeScript Type resolves it to a fully qualified MuIL type token name.
    private async resolveTypeToken(node: ts.Node, ty: ts.Type): Promise<tokens.TypeToken | undefined> {
        // First, simplify the type, if possible, before emitting it.
        let simple: ts.Type;
        let flags: ts.TypeFlags;
        [ simple, flags ] = this.simplifyTypeForToken(ty);

        if (flags & ts.TypeFlags.Any) {
            return tokens.dynamicType;
        }
        else if (flags & ts.TypeFlags.String) {
            return tokens.stringType;
        }
        else if (flags & ts.TypeFlags.Number) {
            return tokens.numberType;
        }
        else if (flags & ts.TypeFlags.Boolean) {
            return tokens.boolType;
        }
        else if (flags & ts.TypeFlags.Void) {
            return undefined; // void is represented as the absence of a type.
        }
        else if (flags & ts.TypeFlags.Null || flags & ts.TypeFlags.Undefined) {
            return tokens.objectType;
        }
        else if (simple.symbol) {
            if (simple.symbol.flags & (ts.SymbolFlags.ObjectLiteral | ts.SymbolFlags.TypeLiteral)) {
                // For object and type literals, simply return the dynamic type.
                // TODO: consider emitting strong types for these and using them anonymously.
                return tokens.objectType;
            }
            else {
                // For object types, we will try to produce the fully qualified symbol name.  If the type is an error,
                // array, or a map, translate it into the appropriate simpler type token.
                if (simple === this.builtinObjectType) {
                    return tokens.objectType;
                }
                else if (!!(flags & ts.TypeFlags.Object) &&
                            !!((<ts.ObjectType>simple).objectFlags & ts.ObjectFlags.Reference) &&
                            !(simple.symbol.flags & ts.SymbolFlags.ObjectLiteral)) {
                    let tyre = <ts.TypeReference>simple;
                    if (tyre.target === this.builtinObjectType) {
                        return tokens.objectType;
                    }
                    else if (tyre.target === this.builtinArrayType) {
                        // Produce a token of the form "[]<elem>".
                        contract.assert(tyre.typeArguments.length === 1);
                        let elem: tokens.TypeToken | undefined =
                            await this.resolveTypeToken(node, tyre.typeArguments[0]);
                        contract.assert(!!elem);
                        return `${tokens.arrayTypePrefix}${elem}`;
                    }
                    else if (tyre.target === this.builtinMapType) {
                        // Produce a token of the form "map[<key>]<elem>".
                        contract.assert(tyre.typeArguments.length === 2);
                        let key: tokens.TypeToken | undefined =
                            await this.resolveTypeToken(node, tyre.typeArguments[0]);
                        contract.assert(!!key);
                        let value: tokens.TypeToken | undefined =
                            await this.resolveTypeToken(node, tyre.typeArguments[1]);
                        contract.assert(!!value);
                        return `${tokens.mapTypePrefix}${key}${tokens.mapTypeSeparator}${value}`;
                    }
                }

                // Otherwise, bottom out on resolving a fully qualified MuPackage type token out of the symbol.
                return await this.resolveTokenFromSymbol(simple.symbol);
            }
        }
        else if (flags & ts.TypeFlags.Object) {
            return tokens.objectType;
        }

        // Finally, if we got here, it's not a type we support yet; issue an error and return `dynamic`.
        this.diagnostics.push(this.dctx.newInvalidTypeError(node, simple));
        return tokens.dynamicType;
    }

    // resolveTokenFromSymbol resolves a symbol to a fully qualified TypeToken that can be used to reference it.
    private async resolveTokenFromSymbol(sym: ts.Symbol): Promise<tokens.Token> {
        // By default, just the type symbol's naked name.
        let token: tokens.TypeToken = sym.name;

        // If the symbol is an aliased symbol, dealias it first.
        if (sym.flags & ts.SymbolFlags.Alias) {
            sym = this.checker().getAliasedSymbol(sym);
        }

        // For member symbols, we must emit the fully qualified name.
        let kinds: ts.SymbolFlags =
            ts.SymbolFlags.Function | ts.SymbolFlags.Property | ts.SymbolFlags.BlockScopedVariable |
            ts.SymbolFlags.Class | ts.SymbolFlags.Interface | ts.SymbolFlags.TypeAlias |
            ts.SymbolFlags.ConstEnum | ts.SymbolFlags.RegularEnum;
        if (sym.flags & kinds) {
            let decls: ts.Declaration[] = sym.getDeclarations();
            contract.assert(decls.length > 0);
            let file: ts.SourceFile = decls[0].getSourceFile();
            let modref: ModuleReference = this.createModuleReferenceFromPath(file.fileName);
            let modtok: tokens.ModuleToken = await this.createModuleToken(modref);
            token = `${modtok}${tokens.tokenDelimiter}${token}`;
        }

        return token;
    }

    // resolveTypeTokenFromTypeLike takes a TypeScript AST node that carries possible typing information and resolves
    // it to fully qualified MuIL type token name.
    private async resolveTypeTokenFromTypeLike(node: TypeLike): Promise<ast.TypeToken | undefined> {
        // Note that we use the getTypeAtLocation API, rather than node's type AST information, so that we can get the
        // fully bound type.  The compiler may have arranged for this to be there through various means, e.g. inference.
        let ty: ts.Type = this.checker().getTypeAtLocation(node);
        contract.assert(!!ty);
        return this.withLocation(node, <ast.TypeToken>{
            kind: ast.typeTokenKind,
            tok:  await this.resolveTypeToken(node, ty),
        });
    }

    // transformIdentifier takes a TypeScript identifier node and yields a true MuIL identifier.
    private transformIdentifier(node: ts.Identifier): ast.Identifier {
        return this.withLocation(node, ident(node.text));
    }

    // createLoadExpression creates an expression that handles all the possible location cases.  It may very well create
    // a dynamic load, rather than a static one, if we are unable to dig through to find an underlying symbol.
    private async createLoadExpression(
            node: ts.Node, objex: ts.Expression | undefined, name: ts.Identifier): Promise<ast.LoadExpression> {
        // Make an identifier out of the name.
        let id: ast.Identifier = this.transformIdentifier(name);

        // Fetch the symbol that this name refers to.
        let idsym: ts.Symbol = this.checker().getSymbolAtLocation(name);
        contract.assert(!!idsym);

        // Fetch information about the object we are loading from, if any.
        let objty: ts.Type | undefined;
        let objsym: ts.Symbol | undefined;
        if (objex) {
            objty = this.checker().getTypeAtLocation(objex);
            contract.assert(!!objty);
            objsym = objty.getSymbol();
            contract.assert(!!objsym);
        }

        // These properties will be initialized and used for the return.
        let tok: tokens.Token | undefined;
        let object: ast.Expression | undefined;
        let isDynamic: boolean = false; // true if the load must be dynamic.

        // In the special case that the object is a value module, we need to perform a special translation.
        if (objsym && !!(objsym.flags & ts.SymbolFlags.ValueModule)) {
            // This is a module property; for instance:
            // 
            //      import * as foo from "foo";
            //      foo.bar();
            //
            // Use this to create a qualified token using the target expression's fully qualified type/module.
            contract.assert(!!(objsym.flags & ts.SymbolFlags.ValueModule));
            let modref: ModuleReference = this.createModuleReference(objsym);
            let modtok: tokens.ModuleToken = await this.createModuleToken(modref);
            tok = this.createModuleMemberToken(modtok, id.ident);
            // note that we intentionally leave object blank, since the token is fully qualified.
        }
        else if (this.isLocalVariableOrFunction(idsym)) {
            // For local variables, just use a simple name load.
            contract.assert(!objex, "Local variables must not have 'this' expressions");
            tok = id.ident;
        }
        else if (objex) {
            // Otherwise, this is a property access, either on an object, or a static through a class.  Create as
            // qualfiied a token we can based on the node's type and symbol; worst case, devolve into a dynamic load.
            let allowed: ts.SymbolFlags =
                ts.SymbolFlags.BlockScopedVariable | ts.SymbolFlags.FunctionScopedVariable |
                ts.SymbolFlags.Function | ts.SymbolFlags.Property | ts.SymbolFlags.Method;
            contract.assert(!!(idsym.flags & allowed),
                            `Unexpected object access symbol: ${ts.SymbolFlags[idsym.flags]}`);
            let ty: ts.Type | undefined = this.checker().getTypeAtLocation(objex);
            contract.assert(!!ty);
            let tytok: tokens.TypeToken | undefined = await this.resolveTypeToken(objex, ty);
            contract.assert(!!tytok);
            if (tytok === tokens.objectType || tytok === tokens.dynamicType) {
                isDynamic = true; // skip the rest; we cannot possibly create a member token.
                object = await this.transformExpression(objex!);
            }
            else {
                // Resolve this member to create a statically bound type token.
                tok = this.createClassMemberToken(tytok!, id.ident);

                // If the property is static, object must be left blank, since the type token will be fully qualified.
                let propIsStatic: boolean = false;
                if (idsym.declarations) {
                    for (let decl of idsym.declarations) {
                        if (ts.getCombinedModifierFlags(decl) & ts.ModifierFlags.Static) {
                            propIsStatic = true;
                            break;
                        }
                    }
                }
                if (!propIsStatic) {
                    contract.assert(!!objex, "Instance methods must have 'this' expressions");
                    object = await this.transformExpression(objex!);
                }
            }
        }
        else {
            // This is a module property load from the current ambient module.
            contract.assert(!!this.currentModuleToken);
            tok = this.createModuleMemberToken(this.currentModuleToken!, id.ident);
            // note that we intentionally leave object blank, since the token is fully qualified.
        }

        if (isDynamic) {
            // If the target type is `dynamic`, we cannot perform static lookups; devolve into a dynamic load.
            contract.assert(!!object);
            return this.withLocation(node, <ast.LoadDynamicExpression>{
                kind:   ast.loadDynamicExpressionKind,
                object: object,
                name:   this.withLocation(name, <ast.StringLiteral>{
                    kind:  ast.stringLiteralKind,
                    value: id.ident,
                }),
            });
        }
        else {
            contract.assert(!!tok);
            return this.withLocation(node, <ast.LoadLocationExpression>{
                kind:   ast.loadLocationExpressionKind,
                object: object,
                name:   this.withLocation(name, <ast.Token>{
                    kind: ast.tokenKind,
                    tok:  tok,
                }),
            });
        }
    }

    /** Modules **/

    // This transforms top-level TypeScript module elements into their corresponding nodes.  This transformation
    // is largely evident in how it works, except that "loose code" (arbitrary statements) is not permitted in
    // MuPack/MuIL.  As such, the appropriate top-level definitions (variables, functions, and classes) are returned as
    // definitions, while any loose code (including variable initializers) is bundled into module inits and entrypoints.
    private async transformSourceFile(node: ts.SourceFile): Promise<ast.Module> {
        if (log.v(7)) {
            log.out(7).info(`Transforming source file: ${node.fileName}`);
        }

        // Each source file is a separate module, and we maintain some amount of context about it.  Push some state.
        let priorSourceFile: ts.SourceFile | undefined = this.currentSourceFile;
        let priorModuleToken: tokens.ModuleToken | undefined = this.currentModuleToken;
        let priorModuleMembers: ast.ModuleMembers | undefined = this.currentModuleMembers;
        let priorModuleExports: ast.ModuleExports | undefined = this.currentModuleExports;
        let priorModuleImports: Map<string, ModuleReference> | undefined = this.currentModuleImports;
        let priorModuleImportTokens: ast.ModuleToken[] | undefined = this.currentModuleImportTokens;
        let priorTempLocalCounter: number = this.currentTempLocalCounter;
        try {
            // Prepare self-referential module information.
            let modref: ModuleReference = this.createModuleReferenceFromPath(node.fileName);
            let pkginfo: PackageInfo = await this.getModulePackageInfo(modref);
            let modtok: tokens.ModuleToken = await this.createModuleTokenFromPackage(modref, pkginfo);
            this.modulePackages.set(tokens.selfModule, Promise.resolve(pkginfo));

            // Now swap out all the information we need during visitation.
            this.currentSourceFile = node;
            this.currentModuleToken = modtok;
            this.currentModuleMembers = {};
            this.currentModuleExports = {};
            this.currentModuleImports = new Map<string, ModuleReference>();
            this.currentModuleImportTokens = []; // to track the imports, in order.
            this.currentTempLocalCounter = 0;

            // Any top-level non-definition statements will pile up into the module initializer.
            let statements: ast.Statement[] = [];

            // Enumerate the module's statements and put them in the respective places.
            for (let statement of node.statements) {
                // Translate the toplevel statement; note that it may produce multiple things, hence the loops below.
                let elements: ModuleElement[] =
                    await this.transformSourceFileStatement(modtok, statement);
                for (let element of elements) {
                    if (isVariableDeclaration(element)) {
                        // This is a module property with a possible initializer.  The property must be registered as a
                        // export in this module's export map, and the initializer must go into the module initializer.
                        // TODO(joe): respect legacyVar to emulate "var"-like scoping.
                        let decl = <VariableDeclaration<ast.ModuleProperty>>element;
                        if (decl.initializer) {
                            statements.push(this.makeVariableInitializer(undefined, decl));
                        }
                        let id: tokens.Name = decl.variable.name.ident;
                        contract.assert(!this.currentModuleMembers[id]);
                        this.currentModuleMembers[id] = decl.variable;
                    }
                    else if (ast.isDefinition(<ast.Node>element)) {
                        let defn = <ast.Definition>element;
                        let id: tokens.Name = defn.name.ident;
                        if (defn.kind === ast.exportKind) {
                            // This is a module export; simply add it to the list.
                            let exp = <ast.Export>defn;
                            contract.assert(!this.currentModuleExports.hasOwnProperty(id),
                                            `Unexpected duplicate export ${this.createModuleMemberToken(modtok, id)}`);
                            this.currentModuleExports[id] = exp;
                        }
                        else {
                            // This is a module member; simply add it to the list.
                            let member = <ast.ModuleMember>element;
                            contract.assert(!this.currentModuleMembers.hasOwnProperty(id),
                                            `Unexpected duplicate member ${this.createModuleMemberToken(modtok, id)}`);
                            this.currentModuleMembers[id] = member;
                        }
                    }
                    else {
                        // This is a top-level module statement; place it into the module initializer.  Note that we
                        // skip empty statements just to avoid superfluously polluting the module with initializers.
                        let stmt = <ast.Statement>element;
                        if (stmt.kind !== ast.emptyStatementKind) {
                            statements.push(stmt);
                        }
                    }
                }
            }

            // If the initialization statements are non-empty, add an initializer method.
            if (statements.length > 0) {
                let initializer: ast.ModuleMethod = {
                    kind:   ast.moduleMethodKind,
                    name:   ident(tokens.initializerFunction),
                    body:   {
                        kind:       ast.blockKind,
                        statements: statements,
                    },
                };
                this.currentModuleMembers[initializer.name.ident] = initializer;
            }

            // Emulate Node.js scripts, where every file is executable.  To do this, simply add an empty main function.
            // Note that because top-level module statements go into the initializer, which will be run before executing
            // any code inside of this module, the main function is actually just empty.
            this.currentModuleMembers[tokens.entryPointFunction] = <ast.ModuleMethod>{
                kind:   ast.moduleMethodKind,
                name:   ident(tokens.entryPointFunction),
                body: {
                    kind:       ast.blockKind,
                    statements: [],
                },
            };

            return this.withLocation(node, <ast.Module>{
                kind:    ast.moduleKind,
                name:    ident(this.getModuleName(modtok)),
                imports: this.currentModuleImportTokens,
                exports: this.currentModuleExports,
                members: this.currentModuleMembers,
            });
        }
        finally {
            this.currentSourceFile = priorSourceFile;
            this.currentModuleToken = priorModuleToken;
            this.currentModuleMembers = priorModuleMembers;
            this.currentModuleExports = priorModuleExports;
            this.currentModuleImports = priorModuleImports;
            this.currentModuleImportTokens = priorModuleImportTokens;
            this.currentTempLocalCounter = priorTempLocalCounter;
        }
    }

    // This transforms a top-level TypeScript module statement.  It might return multiple elements in the rare
    // cases -- like variable initializers -- that expand to multiple elements.
    private async transformSourceFileStatement(
            modtok: tokens.ModuleToken, node: ts.Statement): Promise<ModuleElement[]> {
        if (ts.getCombinedModifierFlags(node) & ts.ModifierFlags.Export) {
            return this.transformExportStatement(modtok, node);
        }
        else {
            switch (node.kind) {
                // Handle module directives; most translate into definitions.
                case ts.SyntaxKind.ExportAssignment:
                    return [ this.transformExportAssignment(<ts.ExportAssignment>node) ];
                case ts.SyntaxKind.ExportDeclaration:
                    return this.transformExportDeclaration(<ts.ExportDeclaration>node);
                case ts.SyntaxKind.ImportDeclaration:
                    return [ await this.transformImportDeclaration(<ts.ImportDeclaration>node) ];

                // Handle declarations; each of these results in a definition.
                case ts.SyntaxKind.ClassDeclaration:
                case ts.SyntaxKind.FunctionDeclaration:
                case ts.SyntaxKind.InterfaceDeclaration:
                case ts.SyntaxKind.ModuleDeclaration:
                case ts.SyntaxKind.TypeAliasDeclaration:
                case ts.SyntaxKind.VariableStatement:
                    return this.transformModuleDeclarationStatement(modtok, node);

                // For any other top-level statements, this.transform them.  They'll be added to the module initializer.
                default:
                    return [ await this.transformStatement(node) ];
            }
        }
    }

    private async transformExportStatement(
            modtok: tokens.ModuleToken, node: ts.Statement): Promise<ModuleElement[]> {
        let elements: ModuleElement[] = await this.transformModuleDeclarationStatement(modtok, node);

        // Default exports get the special name "default"; all others will just reuse the element name.
        contract.assert(!!this.currentModuleToken);
        if (ts.getCombinedModifierFlags(node) & ts.ModifierFlags.Default) {
            contract.assert(elements.length === 1);
            contract.assert(ast.isDefinition(<ast.Node>elements[0]));
            elements.push(this.withLocation(node, <ast.Export>{
                kind:     ast.exportKind,
                name:     ident(defaultExport),
                referent: <ast.Token>{
                    kind: ast.tokenKind,
                    tok:  this.createModuleMemberToken(
                        this.currentModuleToken!, (<ast.ModuleMember>elements[0]).name.ident),
                },
            }));
        }
        else {
            let exports: ast.Export[] = [];
            for (let element of elements) {
                let member: ast.ModuleMember | undefined;
                if (isVariableDeclaration(element)) {
                    member = (<VariableDeclaration<ast.ModuleProperty>>element).variable;
                }
                else if (ast.isDefinition(<ast.Node>element)) {
                    member = <ast.ModuleMember>element;
                }
                if (member) {
                    let id: tokens.Name = member.name.ident;
                    exports.push(this.withLocation(node, <ast.Export>{
                        kind:     ast.exportKind,
                        name:     member.name,
                        referent: this.copyLocation(member.name, <ast.Token>{
                            kind: ast.tokenKind,
                            tok:  this.createModuleMemberToken(this.currentModuleToken!, id),
                        }),
                    }));
                }
            }
            elements = elements.concat(exports);
        }

        return elements;
    }

    private transformExportAssignment(node: ts.ExportAssignment): ast.Definition {
        return notYetImplemented(node);
    }

    private async transformExportDeclaration(node: ts.ExportDeclaration): Promise<ast.ModuleMember[]> {
        let exports: ast.Export[] = [];

        // Otherwise, we are exporting already-imported names from the current module.
        // TODO: in ECMAScript, this is order independent, so we can actually export before declaring something.
        //     To simplify things, we are only allowing you to export things declared lexically before the export.

        // In the case of a module specifier, we are re-exporting elements from another module.
        let sourceModule: ModuleReference | undefined;
        if (node.moduleSpecifier) {
            // The module specifier will be a string literal; fetch and resolve it to a module.
            contract.assert(node.moduleSpecifier.kind === ts.SyntaxKind.StringLiteral);
            let spec: ts.StringLiteral = <ts.StringLiteral>node.moduleSpecifier;
            let source: string = this.transformStringLiteral(spec).value;
            sourceModule = this.resolveModuleReferenceByName(source);
        }

        if (node.exportClause) {
            // This is an export declaration of the form
            //
            //     export { a, b, c }[ from "module"];
            //
            // in which a, b, and c are elements that shall be exported, possibly from another module "module".  If not
            // another module, then these are expected to be definitions within the current module.  Each re-export may
            // optionally rename the symbol being exported.  For example:
            //
            //     export { a as x, b as y, c as z }[ from "module"];
            //
            // For every export clause, we will issue a top-level MuIL re-export AST node.
            for (let exportClause of node.exportClause.elements) {
                let srcname: ast.Identifier = this.transformIdentifier(exportClause.name);
                let dstname: ast.Identifier = srcname;
                if (exportClause.propertyName) {
                    // The export is being renamed (`export <propertyName> as <name>`).  This yields an export node,
                    // even for elements exported from the current module.
                    srcname = this.transformIdentifier(exportClause.propertyName);
                }

                // If this is an export from another module, create an export definition.  Otherwise, for exports
                // referring to ambient elements inside of the current module, we need to do a bitt of investigation.
                let reftok: tokens.Token | undefined;
                if (sourceModule) {
                    reftok = await this.createModuleRefMemberToken(sourceModule, srcname.ident);
                }
                else {
                    let expsym: ts.Symbol | undefined = this.checker().getSymbolAtLocation(exportClause.name);
                    contract.assert(!!expsym);
                    if (expsym.flags & ts.SymbolFlags.Alias) {
                        expsym = this.checker().getAliasedSymbol(expsym);
                    }
                    if (expsym.flags & (ts.SymbolFlags.ValueModule | ts.SymbolFlags.NamespaceModule)) {
                        // If this is a module symbol, then we are rexporting an import, e.g.:
                        //      import * as other from "other";
                        //      export {other};
                        // Create a fully qualified token for that other module using the one we used on import.
                        contract.assert(!!this.currentModuleImports);
                        let modref: ModuleReference | undefined = this.currentModuleImports!.get(srcname.ident);
                        contract.assert(!!modref);
                        reftok = await this.createModuleToken(modref!);
                    }
                    else {
                        // Otherwise, it must be a module member, e.g. an exported class, interface, or variable.
                        contract.assert(!!this.currentModuleToken);
                        contract.assert(!!this.currentModuleMembers);
                        contract.assert(!!this.currentModuleMembers![srcname.ident]);
                        reftok = this.createModuleMemberToken(this.currentModuleToken!, srcname.ident);
                    }
                }

                contract.assert(!!reftok, "Expected either a member or import match for export name");
                exports.push(this.withLocation(exportClause, <ast.Export>{
                    kind:     ast.exportKind,
                    name:     dstname,
                    referent: this.copyLocation(srcname, <ast.Token>{
                        kind: ast.tokenKind,
                        tok:  reftok,
                    }),
                }));
            }
        }
        else {
            // This is an export declaration of the form
            //
            //     export * from "module";
            //
            // For this to work, we simply enumerate all known exports from "module".
            contract.assert(!!sourceModule);
            for (let name of await this.resolveModuleExportNames(sourceModule!)) {
                exports.push(this.withLocation(node, <ast.Export>{
                    kind:  ast.exportKind,
                    name: <ast.Identifier>{
                        kind:  ast.identifierKind,
                        ident: this.extractMemberToken(name),
                    },
                    referent: this.withLocation(node, <ast.Token>{
                        kind: ast.tokenKind,
                        tok:  name,
                    }),
                }));
            }
        }

        return exports;
    }

    private async transformImportDeclaration(node: ts.ImportDeclaration): Promise<ModuleElement> {
        // An import declaration is erased in the output AST, however, we must keep track of the set of known import
        // names so that we can easily look them up by name later on (e.g., in the case of reexporting whole modules).
        if (node.importClause) {
            // First turn the module path into a reference.  The module path may be relative, so we need to consult the
            // current file's module table in order to find its fully resolved path.
            contract.assert(node.moduleSpecifier.kind === ts.SyntaxKind.StringLiteral);
            let importModule: ModuleReference =
                this.resolveModuleReferenceByName((<ts.StringLiteral>node.moduleSpecifier).text);
            let importModuleToken: ast.ModuleToken = this.withLocation(node.moduleSpecifier, <ast.ModuleToken>{
                kind: ast.moduleTokenKind,
                tok:  await this.createModuleToken(importModule),
            });

            // Figure out what kind of import statement this is (there are many, see below).
            let name: ts.Identifier | undefined;
            let namedImports: ts.NamedImports | undefined;
            if (node.importClause.name) {
                name = name;
            }
            else if (node.importClause.namedBindings) {
                if (node.importClause.namedBindings.kind === ts.SyntaxKind.NamespaceImport) {
                    name = (<ts.NamespaceImport>node.importClause.namedBindings).name;
                }
                else {
                    contract.assert(node.importClause.namedBindings.kind === ts.SyntaxKind.NamedImports);
                    namedImports = <ts.NamedImports>node.importClause.namedBindings;
                }
            }

            // Now associate the import names with the module and/or members within it.
            if (name) {
                // This is an import of the form
                //      import * as <name> from "<module>";
                // Just bind the name to an identifier and module to its module reference, and remember the association.
                let importName: ast.Identifier = this.transformIdentifier(name);
                log.out(5).info(`Detected bulk import ${importName.ident}=${importModule}`);
                this.currentModuleImports.set(importName.ident, importModule);
            }
            else if (namedImports) {
                // This is an import of the form
                //      import {a, b, c} from "<module>";
                //  In which case we will need to bind each name and associate it with a fully qualified token.
                for (let importSpec of namedImports.elements) {
                    let member: ast.Identifier = this.transformIdentifier(importSpec.name);
                    let memberToken: tokens.Token = await this.createModuleRefMemberToken(importModule, member.ident);
                    let memberName: string;
                    if (importSpec.propertyName) {
                        // This is of the form
                        //      import {a as x} from "<module>";
                        // in other words, the member is renamed for purposes of this source file.  But we still need to
                        // be able to trace it back to the actual fully qualified exported token name later on.
                        memberName = this.transformIdentifier(importSpec.propertyName).ident;
                    }
                    else {
                        // Otherwise, simply associate the raw member name with the fully qualified member token.
                        memberName = member.ident;
                    }
                    this.currentModuleImports.set(memberName, memberToken);
                    log.out(5).info(`Detected named import ${memberToken} as ${memberName} from ${importModule}`);
                }
            }

            // Now keep track of the import.
            this.currentModuleImportTokens.push(importModuleToken);
        }
        return <ast.EmptyStatement>{ kind: ast.emptyStatementKind };
    }

    /** Statements **/

    private async transformStatement(node: ts.Statement): Promise<ast.Statement> {
        if (log.v(7)) {
            log.out(7).info(`Transforming statement: ${ts.SyntaxKind[node.kind]}`);
        }

        switch (node.kind) {
            // Declaration statements:
            case ts.SyntaxKind.ClassDeclaration:
            case ts.SyntaxKind.FunctionDeclaration:
            case ts.SyntaxKind.InterfaceDeclaration:
            case ts.SyntaxKind.ModuleDeclaration:
            case ts.SyntaxKind.TypeAliasDeclaration:
                // These declarations cannot appear as statements; given an error and return an empty statement.
                this.diagnostics.push(this.dctx.newInvalidDeclarationStatementError(node));
                return { kind: ast.emptyStatementKind };
            case ts.SyntaxKind.VariableStatement:
                return await this.transformLocalVariableStatement(<ts.VariableStatement>node);

            // Control flow statements:
            case ts.SyntaxKind.BreakStatement:
                return this.transformBreakStatement(<ts.BreakStatement>node);
            case ts.SyntaxKind.ContinueStatement:
                return this.transformContinueStatement(<ts.ContinueStatement>node);
            case ts.SyntaxKind.DoStatement:
                return this.transformDoStatement(<ts.DoStatement>node);
            case ts.SyntaxKind.ForStatement:
                return this.transformForStatement(<ts.ForStatement>node);
            case ts.SyntaxKind.ForInStatement:
                return this.transformForInStatement(<ts.ForInStatement>node);
            case ts.SyntaxKind.ForOfStatement:
                return this.transformForOfStatement(<ts.ForOfStatement>node);
            case ts.SyntaxKind.IfStatement:
                return this.transformIfStatement(<ts.IfStatement>node);
            case ts.SyntaxKind.ReturnStatement:
                return this.transformReturnStatement(<ts.ReturnStatement>node);
            case ts.SyntaxKind.SwitchStatement:
                return this.transformSwitchStatement(<ts.SwitchStatement>node);
            case ts.SyntaxKind.ThrowStatement:
                return this.transformThrowStatement(<ts.ThrowStatement>node);
            case ts.SyntaxKind.TryStatement:
                return this.transformTryStatement(<ts.TryStatement>node);
            case ts.SyntaxKind.WhileStatement:
                return this.transformWhileStatement(<ts.WhileStatement>node);

            // Miscellaneous statements:
            case ts.SyntaxKind.Block:
                return this.transformBlock(<ts.Block>node);
            case ts.SyntaxKind.DebuggerStatement:
                return this.transformDebuggerStatement(<ts.DebuggerStatement>node);
            case ts.SyntaxKind.EmptyStatement:
                return this.transformEmptyStatement(<ts.EmptyStatement>node);
            case ts.SyntaxKind.ExpressionStatement:
                return await this.transformExpressionStatement(<ts.ExpressionStatement>node);
            case ts.SyntaxKind.LabeledStatement:
                return this.transformLabeledStatement(<ts.LabeledStatement>node);
            case ts.SyntaxKind.ModuleBlock:
                return this.transformModuleBlock(<ts.ModuleBlock>node);
            case ts.SyntaxKind.WithStatement:
                return this.transformWithStatement(<ts.WithStatement>node);

            // Unrecognized statement:
            default:
                return contract.fail(`Unrecognized statement kind: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    // This routine transforms a declaration statement in TypeScript to a MuIL definition.  Note that definitions in
    // MuIL aren't statements, hence the partitioning between transformDeclaration and transformStatement.  Note that
    // variables do not result in Definitions because they may require higher-level processing to deal with initializer.
    private async transformModuleDeclarationStatement(
            modtok: tokens.ModuleToken, node: ts.Statement): Promise<ModuleElement[]> {
        switch (node.kind) {
            // Declarations:
            case ts.SyntaxKind.ClassDeclaration:
                return [ await this.transformClassDeclaration(modtok, <ts.ClassDeclaration>node) ];
            case ts.SyntaxKind.FunctionDeclaration:
                return [ await this.transformModuleFunctionDeclaration(<ts.FunctionDeclaration>node) ];
            case ts.SyntaxKind.InterfaceDeclaration:
                return [ await this.transformInterfaceDeclaration(modtok, <ts.InterfaceDeclaration>node) ];
            case ts.SyntaxKind.ModuleDeclaration:
                return [ this.transformModuleDeclaration(<ts.ModuleDeclaration>node) ];
            case ts.SyntaxKind.TypeAliasDeclaration:
                return [ await this.transformTypeAliasDeclaration(<ts.TypeAliasDeclaration>node) ];
            case ts.SyntaxKind.VariableStatement:
                return await this.transformModuleVariableStatement(modtok, <ts.VariableStatement>node);
            default:
                return contract.fail(`Node kind is not a module declaration: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    // This transforms a TypeScript Statement, and returns a Block (allocating a new one if needed).
    private async transformStatementAsBlock(node: ts.Statement): Promise<ast.Block> {
        // Transform the statement.  Then, if it is already a block, return it; otherwise, append it to a new one.
        let statement: ast.Statement = await this.transformStatement(node);
        if (statement.kind === ast.blockKind) {
            return <ast.Block>statement;
        }
        return this.withLocation(node, <ast.Block>{
            kind:       ast.blockKind,
            statements: [ statement ],
        });
    }

    /** Declaration statements **/

    private async transformHeritageClauses(
            clauses: ts.HeritageClause[] | undefined):
                Promise<{ extend: ast.TypeToken | undefined, implement: ast.TypeToken[] | undefined }> {
        let extend: ast.TypeToken | undefined;
        let implement: ast.TypeToken[] | undefined;
        if (clauses) {
            for (let heritage of clauses) {
                switch (heritage.token) {
                    case ts.SyntaxKind.ExtendsKeyword:
                        if (!heritage.types) {
                            contract.fail();
                        }
                        else {
                            contract.assert(heritage.types.length === 1);
                            let extsym: ts.Symbol =
                                this.checker().getSymbolAtLocation(heritage.types[0].expression);
                            contract.assert(!!extsym);
                            let exttok: tokens.TypeToken = await this.resolveTokenFromSymbol(extsym);
                            extend = <ast.TypeToken>{
                                kind: ast.typeTokenKind,
                                tok:  exttok,
                            };
                        }
                        break;
                    case ts.SyntaxKind.ImplementsKeyword:
                        if (!heritage.types) {
                            contract.fail();
                        }
                        else {
                            if (!implement) {
                                implement = [];
                            }
                            for (let impltype of heritage.types) {
                                let implsym: ts.Symbol = this.checker().getSymbolAtLocation(impltype.expression);
                                contract.assert(!!implsym);
                                implement.push(<ast.TypeToken>{
                                    kind: ast.typeTokenKind,
                                    tok:  await this.resolveTokenFromSymbol(implsym),
                                });
                            }
                        }
                        break;
                    default:
                        contract.fail(`Unrecognized heritage token kind: ${ts.SyntaxKind[heritage.token]}`);
                }
            }
        }
        return { extend, implement };
    }

    private async transformClassDeclaration(
            modtok: tokens.ModuleToken, node: ts.ClassDeclaration): Promise<ast.Class> {
        // TODO(joe): generics.
        // TODO(joe): decorators.

        // First transform the name into an identifier.  In the absence of a name, we will proceed under the assumption
        // that it is the default export.  This should be verified later on.
        let name: ast.Identifier;
        if (node.name) {
            name = this.transformIdentifier(node.name);
        }
        else {
            name = ident(defaultExport);
        }

        if (log.v(7)) {
            log.out(7).info(`Transforming class declaration: ${name.ident}`);
        }

        // Next, make a class token to use during this class's transformations.
        let classtok: tokens.ModuleMemberToken = this.createModuleMemberToken(modtok, name.ident);
        let priorClassToken: tokens.TypeToken | undefined = this.currentClassToken;
        let priorSuperClassToken: tokens.TypeToken | undefined = this.currentSuperClassToken;
        try {
            this.currentClassToken = classtok;

            // Discover any extends/implements clauses.
            let extend: ast.TypeToken | undefined;
            let implement: ast.TypeToken[] | undefined;
            ({ extend, implement } = await this.transformHeritageClauses(node.heritageClauses));
            if (extend) {
                this.currentSuperClassToken = extend.tok;
            }

            // Transform all non-semicolon members for this declaration into ClassMembers.
            let elements: ClassElement[] = [];
            for (let member of node.members) {
                if (member.kind !== ts.SyntaxKind.SemicolonClassElement) {
                    elements.push(await this.transformClassElement(classtok, member));
                }
            }

            // Now create a member map for this class by translating the ClassMembers created during the translation.
            let members: ast.ClassMembers = {};

            // First do a pass over all methods (including constructor methods).
            for (let element of elements) {
                if (!isVariableDeclaration(element)) {
                    let method = <ast.ClassMethod>element;
                    members[method.name.ident] = method;
                }
            }

            // For all class properties with default values, we need to spill the initializer into either the
            // constructor (for instance initializers) or the class constructor (for static initializers).  This is
            // non-trivial because the class may not have an explicit constructor.  If it doesn't we need to
            // generate one.  In either case, we must be careful to respect initialization order with respect to super
            // calls.  Namely, all property initializers must occur *after* the invocation of `super()`.
            let staticPropertyInitializers: ast.Statement[] = [];
            let instancePropertyInitializers: ast.Statement[] = [];
            for (let element of elements) {
                if (isVariableDeclaration(element)) {
                    let decl = <VariableDeclaration<ast.ClassProperty>>element;
                    if (decl.initializer) {
                        let thisExpression: ast.Expression | undefined;
                        if (decl.variable.static) {
                            staticPropertyInitializers.push(this.makeVariableInitializer(undefined, decl));
                        }
                        else {
                            instancePropertyInitializers.push(
                                this.makeVariableInitializer(
                                    thisExpression = <ast.LoadLocationExpression>{
                                        kind: ast.loadLocationExpressionKind,
                                        name: <ast.Token>{
                                            kind: ast.tokenKind,
                                            tok:  tokens.thisVariable,
                                        },
                                    },
                                    decl,
                                ),
                            );
                        }
                    }
                    members[decl.variable.name.ident] = decl.variable;
                }
            }

            // Create an .init method if there were any static initializers.
            if (staticPropertyInitializers.length > 0) {
                members[tokens.initializerFunction] = <ast.ClassMethod>{
                    kind: ast.classMethodKind,
                    name: ident(tokens.constructorFunction),
                    body: <ast.Block>{
                        kind:       ast.blockKind,
                        statements: staticPropertyInitializers,
                    },
                };
            }

            // Locate the constructor, possibly creating a new one if necessary, if there were instance initializers.
            if (instancePropertyInitializers.length > 0) {
                let ctor: ast.ClassMethod | undefined = <ast.ClassMethod>members[tokens.constructorFunction];
                let insertAt: number | undefined = undefined;
                if (!ctor) {
                    // No explicit constructor was found; create a new one.
                    ctor = <ast.ClassMethod>{
                        kind: ast.classMethodKind,
                        name: ident(tokens.constructorFunction),
                    };
                    insertAt = 0; // add the initializers to the empty block.
                    members[tokens.constructorFunction] = ctor;
                }
                if (ctor.body) {
                    if (extend) {
                        // If there is a superclass, find the insertion point right *after* the explicit call to
                        // `super()`, to achieve the expected initialization order.
                        for (let i = 0; i < ctor.body.statements.length; i++) {
                            if (this.isSuperCall(ctor.body.statements[i], extend.tok)) {
                                insertAt = i+1; // place the initializers right after this call.
                                break;
                            }
                        }
                        contract.assert(insertAt !== undefined);
                    }
                    else {
                        insertAt = 0; // put the initializers before everything else.
                    }
                }
                else {
                    ctor.body = <ast.Block>{
                        kind:       ast.blockKind,
                        statements: [],
                    };
                    if (extend) {
                        // Generate an automatic call to the base class.  Omitting this is only legal if the base class
                        // constructor has zero arguments, so we just generate a simple `super();` call.
                        ctor.body.statements.push(this.createEmptySuperCall(extend.tok));
                        insertAt = 1; // insert the initializers immediately after this call.
                    }
                    else {
                        insertAt = 0; // place the initializers at the start of the (currently empty) block.
                    }
                }

                ctor.body.statements =
                    ctor.body.statements.slice(0, insertAt).concat(
                        instancePropertyInitializers).concat(
                            ctor.body.statements.slice(insertAt, ctor.body.statements.length));
            }

            let mods: ts.ModifierFlags = ts.getCombinedModifierFlags(node);
            return this.withLocation(node, <ast.Class>{
                kind:       ast.classKind,
                name:       name,
                members:    members,
                abstract:   !!(mods & ts.ModifierFlags.Abstract),
                extends:    extend,
                implements: implement,
            });
        }
        finally {
            this.currentClassToken = priorClassToken;
            this.currentSuperClassToken = priorSuperClassToken;
        }
    }

    private async transformDeclarationName(node: ts.DeclarationName): Promise<ast.Expression> {
        switch (node.kind) {
            case ts.SyntaxKind.ArrayBindingPattern:
                return this.transformArrayBindingPattern(node);
            case ts.SyntaxKind.ComputedPropertyName:
                return this.transformComputedPropertyName(node);
            case ts.SyntaxKind.ObjectBindingPattern:
                return this.transformObjectBindingPattern(node);
            case ts.SyntaxKind.Identifier:
                return await this.transformIdentifierExpression(node);
            default:
                return contract.fail(`Unrecognized declaration node: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    private transformDeclarationIdentifier(node: ts.DeclarationName): ast.Identifier {
        switch (node.kind) {
            case ts.SyntaxKind.Identifier:
                return this.transformIdentifier(node);
            default:
                return contract.fail(`Unrecognized declaration identifier: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    private async transformFunctionLikeCommon(node: ts.FunctionLikeDeclaration): Promise<FunctionLikeDeclaration> {
        if (!!node.asteriskToken) {
            this.diagnostics.push(this.dctx.newGeneratorsNotSupportedError(node.asteriskToken));
        }

        // First, visit the body; it can either be a block or a free-standing expression.
        let body: ast.Block | undefined;
        if (node.body) {
            switch (node.body.kind) {
                case ts.SyntaxKind.Block:
                    body = await this.transformBlock(<ts.Block>node.body);
                    break;
                default:
                    // Translate a body of <expr> into
                    //     {
                    //         return <expr>;
                    //     }
                    body = {
                        kind:       ast.blockKind,
                        statements: [
                            <ast.ReturnStatement>{
                                kind:       ast.returnStatementKind,
                                expression: await this.transformExpression(<ts.Expression>node.body),
                            },
                        ],
                    };
                    break;
            }
        }
        return await this.transformFunctionLikeOrSignatureCommon(node, body);
    }

    // A common routine for transforming FunctionLikeDeclarations and MethodSignatures.  The return is specialized per
    // callsite, since differs slightly between module methods, class and interface methods, lambdas, and so on.
    private async transformFunctionLikeOrSignatureCommon(
            node: ts.FunctionLikeDeclaration | ts.MethodSignature, body?: ast.Block): Promise<FunctionLikeDeclaration> {
        // Ensure we are dealing with the supported subset of functions.
        if (ts.getCombinedModifierFlags(node) & ts.ModifierFlags.Async) {
            this.diagnostics.push(this.dctx.newAsyncNotSupportedError(node));
        }

        // First transform the name into an identifier.  In the absence of a name, we will proceed under the assumption
        // that it is the default export.  This should be verified later on.
        let name: ast.Identifier;
        if (node.name) {
            name = this.transformPropertyName(node.name);
        }
        else if (node.kind === ts.SyntaxKind.Constructor) {
            // Constructors have a special name.
            name = ident(tokens.constructorFunction);
        }
        else {
            // All others are assumed to be default exports.
            name = ident(defaultExport);
        }

        // Next transform the parameter variables into locals.
        let parameters: VariableDeclaration<ast.LocalVariable>[] = [];
        for (let param of node.parameters) {
            parameters.push(await this.transformParameterDeclaration(param));
        }

        // If there are any initializers, make sure to prepend them (in order) to the body block.
        if (body) {
            let parameterInits: ast.Statement[] = [];
            for (let parameter of parameters) {
                if (parameter.initializer) {
                    parameterInits.push(this.makeVariableInitializer(undefined, parameter));
                }
            }
            body.statements = parameterInits.concat(body.statements);
        }

        // Get the signature so that we can fetch the return type.
        let returnType: ast.TypeToken | undefined;
        if (node.kind !== ts.SyntaxKind.Constructor) {
            let signature: ts.Signature = this.checker().getSignatureFromDeclaration(node);
            let typeToken: tokens.TypeToken | undefined =
                await this.resolveTypeToken(node, signature.getReturnType());
            if (typeToken) {
                returnType = <ast.TypeToken>{
                    kind: ast.typeTokenKind,
                    tok:  typeToken,
                };
            }
        }

        // Delegate to the factory method to turn this into a real function object.
        return {
            name:       name,
            parameters: parameters.map((p: VariableDeclaration<ast.LocalVariable>) => p.variable),
            body:       body,
            returnType: returnType,
        };
    }

    private async transformModuleFunctionDeclaration(node: ts.FunctionDeclaration): Promise<ast.ModuleMethod> {
        let decl: FunctionLikeDeclaration = await this.transformFunctionLikeCommon(node);
        return this.withLocation(node, <ast.ModuleMethod>{
            kind:       ast.moduleMethodKind,
            name:       decl.name,
            parameters: decl.parameters,
            body:       decl.body,
            returnType: decl.returnType,
        });
    }

    // transformInterfaceDeclaration turns a TypeScript interface into a MuIL interface class.
    private async transformInterfaceDeclaration(
            modtok: tokens.ModuleToken, node: ts.InterfaceDeclaration): Promise<ast.Class> {
        // TODO(joe): generics.
        // TODO(joe): decorators.
        // TODO(joe): extends/implements.

        // Create a name and token for the MuIL class representing this.
        let name: ast.Identifier = this.transformIdentifier(node.name);

        if (log.v(7)) {
            log.out(7).info(`Transforming interface declaration: ${name.ident}`);
        }

        // Next, make a class token to use during this class's transformations.
        let classtok: tokens.ModuleMemberToken = this.createModuleMemberToken(modtok, name.ident);
        let priorClassToken: tokens.TypeToken | undefined = this.currentClassToken;
        let priorSuperClassToken: tokens.TypeToken | undefined = this.currentSuperClassToken;
        try {
            this.currentClassToken = classtok;

            // Discover any extends/implements clauses.
            let extend: ast.TypeToken | undefined;
            let implement: ast.TypeToken[] | undefined;
            ({ extend, implement } = await this.transformHeritageClauses(node.heritageClauses));
            if (extend) {
                this.currentSuperClassToken = extend.tok;
            }

            // Transform all valid members for this declaration into ClassMembers.
            let members: ast.ClassMembers = {};
            for (let member of node.members) {
                if (member.kind !== ts.SyntaxKind.MissingDeclaration) {
                    let decl: ast.ClassMember;
                    let element: ClassElement = await this.transformTypeElement(modtok, member);
                    if (isVariableDeclaration(element)) {
                        let vardecl = <VariableDeclaration<ast.ClassProperty>>element;
                        contract.assert(!vardecl.initializer, "Interface properties do not have initializers");
                        decl = vardecl.variable;
                    }
                    else {
                        decl = <ast.ClassMember>element;
                    }

                    members[decl.name.ident] = decl;
                }
            }

            return this.withLocation(node, <ast.Class>{
                kind:       ast.classKind,
                name:       name,
                members:    members,
                interface:  true,
                extends:    extend,
                implements: implement,
            });
        }
        finally {
            this.currentClassToken = priorClassToken;
            this.currentSuperClassToken = priorSuperClassToken;
        }
    }

    private transformModuleDeclaration(node: ts.ModuleDeclaration): ast.Module {
        return notYetImplemented(node);
    }

    private async transformParameterDeclaration(
            node: ts.ParameterDeclaration): Promise<VariableDeclaration<ast.LocalVariable>> {
        // Validate that we're dealing with the supported subset.
        if (!!node.dotDotDotToken) {
            this.diagnostics.push(this.dctx.newRestParamsNotSupportedError(node.dotDotDotToken));
        }

        // TODO[marapongo/mu#43]: parameters can be any binding name, including destructuring patterns.  For now,
        //     however, we only support the identifier forms.
        let name: ast.Identifier = this.transformBindingIdentifier(node.name);
        let initializer: ast.Expression | undefined;
        if (node.initializer) {
            initializer = await this.transformExpression(node.initializer);
        }
        return {
            node:     node,
            tok:      name.ident,
            variable: {
                kind: ast.localVariableKind,
                name: name,
                type: await this.resolveTypeTokenFromTypeLike(node),
            },
            initializer: initializer,
        };
    }

    // transformTypeAliasDeclaration emits a type whose base is the aliased type.  The MuIL type system permits
    // conversions between such types in a way that is roughly compatible with TypeScript's notion of type aliases.
    private async transformTypeAliasDeclaration(node: ts.TypeAliasDeclaration): Promise<ast.Class> {
        return this.withLocation(node, <ast.Class>{
            kind:    ast.classKind,
            name:    this.transformIdentifier(node.name),
            extends: await this.resolveTypeTokenFromTypeLike(node),
        });
    }

    private makeVariableInitializer(
            object: ast.Expression | undefined, decl: VariableDeclaration<ast.Variable>): ast.Statement {
        contract.requires(!!decl.initializer, "decl", "Expected variable declaration to have an initializer");
        return this.copyLocation(decl.initializer!, <ast.ExpressionStatement>{
            kind:       ast.expressionStatementKind,
            expression: this.copyLocation(decl.initializer!, <ast.BinaryOperatorExpression>{
                kind:     ast.binaryOperatorExpressionKind,
                left:     this.copyLocation(decl.initializer!, <ast.LoadLocationExpression>{
                    kind:   ast.loadLocationExpressionKind,
                    object: object,
                    name:   this.copyLocation(decl.variable.name, <ast.Token>{
                        kind: ast.tokenKind,
                        tok:  decl.tok,
                    }),
                }),
                operator: "=",
                right:    decl.initializer,
            }),
        });
    }

    private async transformVariableStatement(node: ts.VariableStatement): Promise<VariableLikeDeclaration[]> {
        let decls: VariableLikeDeclaration[] = [];
        for (let decl of node.declarationList.declarations) {
            let like: VariableLikeDeclaration = await this.transformVariableDeclaration(decl);

            // If the node is marked "const", tag all variables as readonly.
            if (!!(node.declarationList.flags & ts.NodeFlags.Const)) {
                like.readonly = true;
            }

            // If the node isn't marked "let", we must mark all variables to use legacy "var" behavior.
            if (!(node.declarationList.flags & ts.NodeFlags.Let)) {
                like.legacyVar = true;
            }

            decls.push(like);
        }
        return decls;
    }

    private async transformLocalVariableStatement(node: ts.VariableStatement): Promise<ast.Statement> {
        // For variables, we need to append initializers as assignments if there are any.
        // TODO: emulate "var"-like scoping.
        let statements: ast.Statement[] = [];
        let decls: VariableLikeDeclaration[] = await this.transformVariableStatement(node);
        for (let decl of decls) {
            let local = <ast.LocalVariable>{
                kind:     ast.localVariableKind,
                name:     decl.name,
                type:     decl.type,
                readonly: decl.readonly,
            };

            statements.push(<ast.LocalVariableDeclaration>{
                kind:  ast.localVariableDeclarationKind,
                local: local,
            });

            if (decl.initializer) {
                let vdecl = new VariableDeclaration<ast.LocalVariable>(
                    node, local.name.ident, local, decl.legacyVar, decl.initializer);
                statements.push(this.makeVariableInitializer(undefined, vdecl));
            }
        }

        contract.assert(statements.length > 0);

        if (statements.length > 1) {
            return this.copyLocationRange(
                statements[0],
                statements[statements.length-1],
                <ast.MultiStatement>{
                    kind:       ast.multiStatementKind,
                    statements: statements,
                },
            );
        }
        else {
            return statements[0];
        }
    }

    private async transformModuleVariableStatement(
            modtok: tokens.ModuleToken, node: ts.VariableStatement):
                Promise<VariableDeclaration<ast.ModuleProperty>[]> {
        let decls: VariableLikeDeclaration[] = await this.transformVariableStatement(node);
        return decls.map((decl: VariableLikeDeclaration) =>
            new VariableDeclaration<ast.ModuleProperty>(
                node,
                this.createModuleMemberToken(modtok, decl.name.ident),
                <ast.ModuleProperty>{
                    kind:     ast.modulePropertyKind,
                    name:     decl.name,
                    readonly: decl.readonly,
                    type:     decl.type,
                },
                decl.legacyVar,
                decl.initializer,
            ),
        );
    }

    private async transformVariableDeclaration(node: ts.VariableDeclaration): Promise<VariableLikeDeclaration> {
        // TODO[marapongo/mu#43]: parameters can be any binding name, including destructuring patterns.  For now,
        //     however, we only support the identifier forms.
        let name: ast.Identifier = this.transformDeclarationIdentifier(node.name);
        let initializer: ast.Expression | undefined;
        if (node.initializer) {
            initializer = await this.transformExpression(node.initializer);
        }
        return {
            name:        name,
            type:        await this.resolveTypeTokenFromTypeLike(node),
            initializer: initializer,
        };
    }

    private async transformVariableDeclarationList(
            node: ts.VariableDeclarationList): Promise<VariableLikeDeclaration[]> {
        let decls: VariableLikeDeclaration[] = [];
        for (let decl of node.declarations) {
            decls.push(await this.transformVariableDeclaration(decl));
        }
        return decls;
    }

    /** Class/type elements **/

    private async transformClassElement(
            classtok: tokens.ModuleMemberToken, node: ts.ClassElement): Promise<ClassElement> {
        switch (node.kind) {
            // All the function-like members go here:
            case ts.SyntaxKind.Constructor:
                return this.transformFunctionLikeDeclaration(<ts.ConstructorDeclaration>node);
            case ts.SyntaxKind.MethodDeclaration:
                return this.transformFunctionLikeDeclaration(<ts.MethodDeclaration>node);
            case ts.SyntaxKind.GetAccessor:
                return this.transformFunctionLikeDeclaration(<ts.GetAccessorDeclaration>node);
            case ts.SyntaxKind.SetAccessor:
                return this.transformFunctionLikeDeclaration(<ts.SetAccessorDeclaration>node);

            // Properties are not function-like, so we translate them differently.
            case ts.SyntaxKind.PropertyDeclaration:
                return await this.transformPropertyDeclarationOrSignature(classtok, <ts.PropertyDeclaration>node);

            // Unrecognized cases:
            case ts.SyntaxKind.SemicolonClassElement:
                return contract.fail("Expected all SemiColonClassElements to be filtered out of AST tree");
            default:
                return contract.fail(`Unrecognized ClassElement node kind: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    // transformTypeElement turns a TypeScript type element, typically an interface member, into a MuIL class member.
    private async transformTypeElement(
            classtok: tokens.ModuleMemberToken, node: ts.TypeElement): Promise<ClassElement> {
        switch (node.kind) {
            // Property and method signatures are like their class counterparts, but have no bodies:
            case ts.SyntaxKind.PropertySignature:
                return await this.transformPropertyDeclarationOrSignature(classtok, <ts.PropertySignature>node);
            case ts.SyntaxKind.MethodSignature:
                return await this.transformMethodSignature(<ts.MethodSignature>node);

            default:
                return contract.fail(`Unrecognized TypeElement node kind: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    private getClassAccessibility(node: ts.Node): tokens.Accessibility {
        let mods: ts.ModifierFlags = ts.getCombinedModifierFlags(node);
        if (!!(mods & ts.ModifierFlags.Private)) {
            return tokens.privateAccessibility;
        }
        else if (!!(mods & ts.ModifierFlags.Protected)) {
            return tokens.protectedAccessibility;
        }
        else {
            // All members are public by default in ECMA/TypeScript.
            return tokens.publicAccessibility;
        }
    }

    private async transformFunctionLikeDeclaration(node: ts.FunctionLikeDeclaration): Promise<ast.ClassMethod> {
        // Get/Set accessors aren't yet supported.
        contract.assert(node.kind !== ts.SyntaxKind.GetAccessor, "GetAccessor NYI");
        contract.assert(node.kind !== ts.SyntaxKind.SetAccessor, "SetAccessor NYI");

        let mods: ts.ModifierFlags = ts.getCombinedModifierFlags(node);
        let decl: FunctionLikeDeclaration = await this.transformFunctionLikeCommon(node);
        return this.withLocation(node, <ast.ClassMethod>{
            kind:       ast.classMethodKind,
            name:       decl.name,
            access:     this.getClassAccessibility(node),
            parameters: decl.parameters,
            body:       decl.body,
            returnType: decl.returnType,
            static:     !!(mods & ts.ModifierFlags.Static),
            abstract:   !!(mods & ts.ModifierFlags.Abstract),
        });
    }

    private async transformPropertyDeclarationOrSignature(
            classtok: tokens.ModuleMemberToken,
            node: ts.PropertyDeclaration | ts.PropertySignature): Promise<VariableDeclaration<ast.ClassProperty>> {
        let initializer: ast.Expression | undefined;
        if (node.initializer) {
            initializer = await this.transformExpression(node.initializer);
        }
        let mods: ts.ModifierFlags = ts.getCombinedModifierFlags(node);
        let name: ast.Identifier = this.transformPropertyName(node.name);
        // TODO: primary properties.
        return new VariableDeclaration<ast.ClassProperty>(
            node,
            this.createClassMemberToken(classtok, name.ident),
            {
                kind:     ast.classPropertyKind,
                name:     name,
                access:   this.getClassAccessibility(node),
                readonly: !!(mods & ts.ModifierFlags.Readonly),
                optional: !!(node.questionToken),
                static:   !!(mods & ts.ModifierFlags.Static),
                type:     await this.resolveTypeTokenFromTypeLike(node),
            },
            false,
            initializer,
        );
    }

    private async transformMethodSignature(node: ts.MethodSignature): Promise<ast.ClassMethod> {
        let decl: FunctionLikeDeclaration = await this.transformFunctionLikeOrSignatureCommon(node);
        return this.withLocation(node, <ast.ClassMethod>{
            kind:       ast.classMethodKind,
            name:       decl.name,
            access:     this.getClassAccessibility(node),
            parameters: decl.parameters,
            returnType: decl.returnType,
            abstract:   true,
        });
    }

    /** Control flow statements **/

    private transformBreakStatement(node: ts.BreakStatement): ast.BreakStatement {
        return this.withLocation(node, <ast.BreakStatement>{
            kind:  ast.breakStatementKind,
            label: object.maybeUndefined(node.label, (id: ts.Identifier) => this.transformIdentifier(id)),
        });
    }

    private transformCaseOrDefaultClause(node: ts.CaseOrDefaultClause): ast.Statement {
        return notYetImplemented(node);
    }

    private transformCatchClause(node: ts.CatchClause): ast.Statement {
        return notYetImplemented(node);
    }

    private transformContinueStatement(node: ts.ContinueStatement): ast.ContinueStatement {
        return this.withLocation(node, <ast.ContinueStatement>{
            kind:  ast.continueStatementKind,
            label: object.maybeUndefined(node.label, (id: ts.Identifier) => this.transformIdentifier(id)),
        });
    }

    // This transforms a higher-level TypeScript `do`/`while` statement by expanding into an ordinary `while` statement.
    private async transformDoStatement(node: ts.DoStatement): Promise<ast.WhileStatement> {
        // Now create a new block that simply concatenates the existing one with a test/`break`.
        let body: ast.Block = this.withLocation(node.statement, {
            kind:       ast.blockKind,
            statements: [
                await this.transformStatement(node.statement),
                <ast.IfStatement>{
                    kind:      ast.ifStatementKind,
                    condition: <ast.UnaryOperatorExpression>{
                        kind:     ast.unaryOperatorExpressionKind,
                        operator: "!",
                        operand:  await this.transformExpression(node.expression),
                    },
                    consequent: <ast.BreakStatement>{
                        kind: ast.breakStatementKind,
                    },
                },
            ],
        });

        return this.withLocation(node, <ast.WhileStatement>{
            kind: ast.whileStatementKind,
            test: <ast.BoolLiteral>{
                kind:  ast.boolLiteralKind,
                value: true,
            },
            body: body,
        });
    }

    private transformForStatement(node: ts.ForStatement): ast.Statement {
        return notYetImplemented(node);
    }

    private transformForInitializer(node: ts.ForInitializer): ast.Statement {
        return notYetImplemented(node);
    }

    private transformForInStatement(node: ts.ForInStatement): ast.Statement {
        return notYetImplemented(node);
    }

    private transformForOfStatement(node: ts.ForOfStatement): ast.Statement {
        return notYetImplemented(node);
    }

    private async transformIfStatement(node: ts.IfStatement): Promise<ast.IfStatement> {
        let condition: ast.Expression = await this.transformExpression(node.expression);
        let consequent: ast.Statement = await this.transformStatement(node.thenStatement);
        let alternate: ast.Statement | undefined;
        if (node.elseStatement) {
            alternate = await this.transformStatement(node.elseStatement);
        }
        return this.withLocation(node, <ast.IfStatement>{
            kind:       ast.ifStatementKind,
            condition:  condition,
            consequent: consequent,
            alternate:  alternate,
        });
    }

    private async transformReturnStatement(node: ts.ReturnStatement): Promise<ast.ReturnStatement> {
        let expr: ast.Expression | undefined;
        if (node.expression) {
            expr = await this.transformExpression(node.expression);
        }
        return this.withLocation(node, <ast.ReturnStatement>{
            kind:       ast.returnStatementKind,
            expression: expr,
        });
    }

    // transformSwitchStatement transforms a switch into a series of if/else statements.
    private async transformSwitchStatement(node: ts.SwitchStatement): Promise<ast.Statement> {
        // A switch expands into a block.
        let block = this.withLocation(node, <ast.Block>{
            kind:       ast.blockKind,
            statements: [],
        });

        // Spill the expression an auto-generated local/ variable so that we don't execute it multiple times.
        let exprLocal = this.generateTempLocal("expr");
        block.statements.push(<ast.LocalVariableDeclaration>{
            kind:  ast.localVariableDeclarationKind,
            local: <ast.LocalVariable>{
                kind:  ast.localVariableKind,
                name:  ident(exprLocal),
            },
        });
        block.statements.push(<ast.ExpressionStatement>{
            kind:       ast.expressionStatementKind,
            expression: <ast.BinaryOperatorExpression>{
                kind:     ast.binaryOperatorExpressionKind,
                operator: binaryOperators.get(ts.SyntaxKind.EqualsToken),
                left:     this.createLoadLocal(exprLocal),
                right:    await this.transformExpression(node.expression),
            },
        });

        // Next, generate a list of clauses, including handling fallthrough and defaults; there are three cases:
        //
        //     1) A clause that ends with a break; this gets its own "if" statement.
        //     2) A clause that does dynamically hit a "break"; this automatically falls through to the next case.
        //     3) A default clause; this is executed unconditionally if no prior cases matched.
        //
        //  To illustrate all three cases, consider this example:
        //
        //      switch (<expr>) {¬
        //          case a:¬
        //              foo();¬
        //              break;¬
        //          case b:¬
        //              bar();¬
        //          default:¬
        //              baz();¬
        //              break;¬
        //      }¬
        //
        // The generated code looks roughly like this:¬
        //
        //      let _tmp_expr = <expr>; // spilled expression.
        //      let _tmp_match = false; // initialized to false.
        //      let _tmp_break = true;  // initialized to true.
        //      if (!_tmp_break || (!_tmp_match && _tmp_expr === a)) {¬
        //          _tmp_match = true;
        //          foo();¬
        //          _tmp_break = true;
        //      }¬
        //      if (!_tmp_break || (!_tmp_match && _tmp_expr === b)) {¬
        //          _tmp_match = true;
        //          bar();¬
        //          baz();¬
        //          _tmp_break = false;
        //      }¬
        //      if (!_tmp_break || !tmp_match) {¬
        //          baz();¬
        //      }¬
        //
        // Notice how we track two special variables, _tmp_match and _tmp_break, to indicate whether the prior
        // clause matched or led to a break, respectively.  This informs whether we will dynamically fallthrough.
        let matchLocal = this.generateTempLocal("match");
        let breakLocal = this.generateTempLocal("break");

        // Place all the clauses into a big block and declare and initialize those locals.
        block.statements.push(<ast.LocalVariableDeclaration>{
            kind:  ast.localVariableDeclarationKind,
            local: <ast.LocalVariable>{
                kind:    ast.localVariableKind,
                name:    ident(matchLocal),
                default: false, // no match by default.
            },
        });
        block.statements.push(<ast.LocalVariableDeclaration>{
            kind:  ast.localVariableDeclarationKind,
            local: <ast.LocalVariable>{
                kind:    ast.localVariableKind,
                name:    ident(breakLocal),
                default: true, // break enabled by default (to prevent immediate fallthrough).
            },
        });

        // Now create a proper guard and clause for every switch case.
        for (let clause of node.caseBlock.clauses) {
            // Produce the case body; this is what will execute if the case is taken.
            let body: ast.Block = <ast.Block>{
                kind:       ast.blockKind,
                statements: [],
            };

            // First set _tmp_match to true.
            body.statements.push(<ast.ExpressionStatement>{
                kind:       ast.expressionStatementKind,
                expression: <ast.BinaryOperatorExpression>{
                    kind:     ast.binaryOperatorExpressionKind,
                    operator: binaryOperators.get(ts.SyntaxKind.EqualsToken),
                    left:     this.createLoadLocal(matchLocal),
                    right:    <ast.BoolLiteral>{
                        kind:  ast.boolLiteralKind,
                        value: true,
                    },
                },
            });

            // Now just push each of the clause's transformed statements.
            for (let stmt of clause.statements) {
                let trans: ast.Statement = await this.transformStatement(stmt);
                if (trans.kind === ast.breakStatementKind && !(<ast.BreakStatement>trans).label) {
                    // This is a break; transform it into setting the variable, and skip adding it.
                    body.statements.push(<ast.ExpressionStatement>{
                        kind:       ast.expressionStatementKind,
                        expression: <ast.BinaryOperatorExpression>{
                            kind:     ast.binaryOperatorExpressionKind,
                            operator: binaryOperators.get(ts.SyntaxKind.EqualsToken),
                            left:     this.createLoadLocal(breakLocal),
                            right:    <ast.BoolLiteral>{
                                kind:  ast.boolLiteralKind,
                                value: true,
                            },
                        },
                    });
                }
                else {
                    body.statements.push(trans);
                }
            }


            // Next, generate the if guard; this needs to be sensitive to whether this is the default.
            // First, the `!_tmp_match` part, which is common to all variants.
            let guard: ast.Expression = <ast.UnaryOperatorExpression>{
                kind:     ast.unaryOperatorExpressionKind,
                operator: prefixUnaryOperators.get(ts.SyntaxKind.ExclamationToken),
                operand:  this.createLoadLocal(matchLocal),
            };

            // Now, if this isn't default, we need to check === with the desired expression.  This turns the above guard
            // into `(!_tmp_match && _tmp_expr === <caseExpr>)`.
            if (clause.kind === ts.SyntaxKind.CaseClause) {
                guard = <ast.BinaryOperatorExpression>{
                    kind:     ast.binaryOperatorExpressionKind,
                    operator: binaryOperators.get(ts.SyntaxKind.AmpersandAmpersandToken),
                    left:     guard,
                    right:    <ast.BinaryOperatorExpression>{
                        kind:     ast.binaryOperatorExpressionKind,
                        operator: binaryOperators.get(ts.SyntaxKind.EqualsEqualsEqualsToken),
                        left:     this.createLoadLocal(exprLocal),
                        right: await this.transformExpression((<ts.CaseClause>clause).expression),
                    },
                };
            }

            // Finally turn <guard> into `!tmp_break || <guard>`, which is the final form of the if guard.
            guard = <ast.BinaryOperatorExpression>{
                kind:     ast.binaryOperatorExpressionKind,
                operator: binaryOperators.get(ts.SyntaxKind.BarBarToken),
                left:     this.createLoadLocal(breakLocal),
                right:    guard,
            };

            // Now push the overall thing as an if statement.
            block.statements.push(this.withLocation(clause, <ast.IfStatement>{
               kind:       ast.ifStatementKind,
               condition:  guard,
               consequent: body,
            }));
        }

        return block;
    }

    private async transformThrowStatement(node: ts.ThrowStatement): Promise<ast.ThrowStatement> {
        return this.withLocation(node, <ast.ThrowStatement>{
            kind:       ast.throwStatementKind,
            expression: await this.transformExpression(node.expression),
        });
    }

    private transformTryStatement(node: ts.TryStatement): ast.TryCatchFinally {
        return notYetImplemented(node);
    }

    private async transformWhileStatement(node: ts.WhileStatement): Promise<ast.WhileStatement> {
        return this.withLocation(node, <ast.WhileStatement>{
            kind: ast.whileStatementKind,
            test: await this.transformExpression(node.expression),
            body: await this.transformStatementAsBlock(node.statement),
        });
    }

    /** Miscellaneous statements **/

    private async transformBlock(node: ts.Block): Promise<ast.Block> {
        // TODO(joe): map directives.
        let stmts: ast.Statement[] = [];
        for (let stmt of node.statements) {
            stmts.push(await this.transformStatement(stmt));
        }
        return this.withLocation(node, <ast.Block>{
            kind:       ast.blockKind,
            statements: stmts,
        });
    }

    private transformDebuggerStatement(node: ts.DebuggerStatement): ast.Statement {
        // The debugger statement in ECMAScript can be used to trip a breakpoint.  We don't have the equivalent in Mu at
        // the moment, so we simply produce an empty statement in its place.
        return this.withLocation(node, <ast.Statement>{
            kind: ast.emptyStatementKind,
        });
    }

    private transformEmptyStatement(node: ts.EmptyStatement): ast.EmptyStatement {
        return this.withLocation(node, <ast.EmptyStatement>{
            kind: ast.emptyStatementKind,
        });
    }

    private async transformExpressionStatement(node: ts.ExpressionStatement): Promise<ast.ExpressionStatement> {
        return this.withLocation(node, <ast.ExpressionStatement>{
            kind:       ast.expressionStatementKind,
            expression: await this.transformExpression(node.expression),
        });
    }

    private async transformLabeledStatement(node: ts.LabeledStatement): Promise<ast.LabeledStatement> {
        return this.withLocation(node, <ast.LabeledStatement>{
            kind:      ast.labeledStatementKind,
            label:     this.transformIdentifier(node.label),
            statement: await this.transformStatement(node.statement),
        });
    }

    private transformModuleBlock(node: ts.ModuleBlock): ast.Block {
        return notYetImplemented(node);
    }

    private transformWithStatement(node: ts.WithStatement): ast.Statement {
        return notYetImplemented(node);
    }

    /** Expressions **/

    private async transformExpression(node: ts.Expression): Promise<ast.Expression> {
        if (log.v(7)) {
            log.out(7).info(`Transforming expression: ${ts.SyntaxKind[node.kind]}`);
        }

        switch (node.kind) {
            // Expressions:
            case ts.SyntaxKind.ArrayLiteralExpression:
                return this.transformArrayLiteralExpression(<ts.ArrayLiteralExpression>node);
            case ts.SyntaxKind.ArrowFunction:
                return this.transformArrowFunction(<ts.ArrowFunction>node);
            case ts.SyntaxKind.BinaryExpression:
                return this.transformBinaryExpression(<ts.BinaryExpression>node);
            case ts.SyntaxKind.CallExpression:
                return this.transformCallExpression(<ts.CallExpression>node);
            case ts.SyntaxKind.ClassExpression:
                return this.transformClassExpression(<ts.ClassExpression>node);
            case ts.SyntaxKind.ConditionalExpression:
                return this.transformConditionalExpression(<ts.ConditionalExpression>node);
            case ts.SyntaxKind.DeleteExpression:
                return this.transformDeleteExpression(<ts.DeleteExpression>node);
            case ts.SyntaxKind.ElementAccessExpression:
                return this.transformElementAccessExpression(<ts.ElementAccessExpression>node);
            case ts.SyntaxKind.FunctionExpression:
                return this.transformFunctionExpression(<ts.FunctionExpression>node);
            case ts.SyntaxKind.Identifier:
                return await this.transformIdentifierExpression(<ts.Identifier>node);
            case ts.SyntaxKind.ObjectLiteralExpression:
                return this.transformObjectLiteralExpression(<ts.ObjectLiteralExpression>node);
            case ts.SyntaxKind.PostfixUnaryExpression:
                return this.transformPostfixUnaryExpression(<ts.PostfixUnaryExpression>node);
            case ts.SyntaxKind.PrefixUnaryExpression:
                return this.transformPrefixUnaryExpression(<ts.PrefixUnaryExpression>node);
            case ts.SyntaxKind.PropertyAccessExpression:
                return this.transformPropertyAccessExpression(<ts.PropertyAccessExpression>node);
            case ts.SyntaxKind.NewExpression:
                return await this.transformNewExpression(<ts.NewExpression>node);
            case ts.SyntaxKind.OmittedExpression:
                return this.transformOmittedExpression(<ts.OmittedExpression>node);
            case ts.SyntaxKind.ParenthesizedExpression:
                return await this.transformParenthesizedExpression(<ts.ParenthesizedExpression>node);
            case ts.SyntaxKind.SpreadElement:
                return this.transformSpreadElement(<ts.SpreadElement>node);
            case ts.SyntaxKind.SuperKeyword:
                return this.transformSuperExpression(<ts.SuperExpression>node);
            case ts.SyntaxKind.TaggedTemplateExpression:
                return this.transformTaggedTemplateExpression(<ts.TaggedTemplateExpression>node);
            case ts.SyntaxKind.TemplateExpression:
                return this.transformTemplateExpression(<ts.TemplateExpression>node);
            case ts.SyntaxKind.ThisKeyword:
                return this.transformThisExpression(<ts.ThisExpression>node);
            case ts.SyntaxKind.TypeAssertionExpression:
                return this.transformTypeAssertionExpression(<ts.TypeAssertion>node);
            case ts.SyntaxKind.TypeOfExpression:
                return this.transformTypeOfExpression(<ts.TypeOfExpression>node);
            case ts.SyntaxKind.VoidExpression:
                return this.transformVoidExpression(<ts.VoidExpression>node);
            case ts.SyntaxKind.YieldExpression:
                return this.transformYieldExpression(<ts.YieldExpression>node);

            // Literals:
            case ts.SyntaxKind.FalseKeyword:
            case ts.SyntaxKind.TrueKeyword:
                return this.transformBooleanLiteral(<ts.BooleanLiteral>node);
            case ts.SyntaxKind.NoSubstitutionTemplateLiteral:
                return this.transformNoSubstitutionTemplateLiteral(<ts.NoSubstitutionTemplateLiteral>node);
            case ts.SyntaxKind.NullKeyword:
                return this.transformNullLiteral(<ts.NullLiteral>node);
            case ts.SyntaxKind.NumericLiteral:
                return this.transformNumericLiteral(<ts.NumericLiteral>node);
            case ts.SyntaxKind.RegularExpressionLiteral:
                return this.transformRegularExpressionLiteral(<ts.RegularExpressionLiteral>node);
            case ts.SyntaxKind.StringLiteral:
                return this.transformStringLiteral(<ts.StringLiteral>node);

            // Unrecognized:
            default:
                return contract.fail(`Unrecognized expression kind: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    private async transformArrayLiteralExpression(node: ts.ArrayLiteralExpression): Promise<ast.ArrayLiteral> {
        let elemtok: tokens.TypeToken | undefined;

        // See if TypeScript has assigned a type for this; if it's an array type, extract the element.
        let arrty: ts.Type = this.checker().getTypeAtLocation(node);
        if ((arrty.flags & ts.TypeFlags.Object) &&
                ((<ts.ObjectType>arrty).objectFlags & ts.ObjectFlags.Reference)) {
            let arrobjty: ts.TypeReference = <ts.TypeReference>arrty;
            if (arrobjty.target === this.builtinArrayType) {
                contract.assert(arrobjty.typeArguments.length === 1);
                let elemty: ts.Type = arrobjty.typeArguments[0];
                elemtok = await this.resolveTypeToken(node, elemty);
            }
        }
        if (!elemtok) {
            // If no static type was determined, assign the dynamic type to it.
            elemtok = tokens.dynamicType;
        }

        // If there is an initializer, transform all expressions.
        let elements: ast.Expression[] = [];
        for (let elem of node.elements) {
            elements.push(await this.transformExpression(elem));
        }

        return this.withLocation(node, <ast.ArrayLiteral>{
            kind:     ast.arrayLiteralKind,
            elemType: <ast.TypeToken>{
                kind: ast.typeTokenKind,
                tok:  elemtok,
            },
            elements: elements,
        });
    }

    private transformArrowFunction(node: ts.ArrowFunction): ast.Expression {
        return notYetImplemented(node);
    }

    private async transformBinaryExpression(node: ts.BinaryExpression): Promise<ast.Expression> {
        let op: ts.SyntaxKind = node.operatorToken.kind;
        if (op === ts.SyntaxKind.CommaToken) {
            // Translate this into a sequence expression.
            return await this.transformBinarySequenceExpression(node);
        }
        else {
            // Translate this into an ordinary binary operator.
            return await this.transformBinaryOperatorExpression(node);
        }
    }

    private async transformBinaryOperatorExpression(node: ts.BinaryExpression): Promise<ast.BinaryOperatorExpression> {
        // A few operators aren't faithfully emulated; in those cases, log warnings.
        if (log.v(3)) {
            switch (node.operatorToken.kind) {
                case ts.SyntaxKind.GreaterThanGreaterThanGreaterThanEqualsToken:
                case ts.SyntaxKind.GreaterThanGreaterThanGreaterThanToken:
                case ts.SyntaxKind.EqualsEqualsEqualsToken:
                case ts.SyntaxKind.ExclamationEqualsEqualsToken:
                    log.out(3).info(
                        `ECMAScript operator '${ts.SyntaxKind[node.operatorToken.kind]}' not supported; ` +
                        `until marapongo/mu#50 is implemented, be careful about subtle behavioral differences`,
                    );
                    break;
                default:
                    break;
            }
        }

        let operator: ast.BinaryOperator | undefined = binaryOperators.get(node.operatorToken.kind);
        contract.assert(!!operator, `Expected binary operator for: ${ts.SyntaxKind[node.operatorToken.kind]}`);
        return this.withLocation(node, <ast.BinaryOperatorExpression>{
            kind:     ast.binaryOperatorExpressionKind,
            operator: operator,
            left:     await this.transformExpression(node.left),
            right:    await this.transformExpression(node.right),
        });
    }

    private async transformBinarySequenceExpression(node: ts.BinaryExpression): Promise<ast.SequenceExpression> {
        contract.assert(node.operatorToken.kind === ts.SyntaxKind.CommaToken);

        // Pile up the expressions in the right order.
        let curr: ts.Expression = node;
        let binary: ts.BinaryExpression = node;
        let expressions: ast.Expression[] = [];
        while (curr.kind === ts.SyntaxKind.BinaryExpression &&
                (binary = <ts.BinaryExpression>curr).operatorToken.kind === ts.SyntaxKind.CommaToken) {
            expressions.unshift(await this.transformExpression(binary.right));
            curr = binary.left;
        }
        expressions.unshift(await this.transformExpression(curr));

        contract.assert(expressions.length > 0);
        return this.copyLocationRange(
            expressions[0],
            expressions[expressions.length-1],
            <ast.SequenceExpression>{
                kind:        ast.sequenceExpressionKind,
                expressions: expressions,
            },
        );
    }

    // isSuperCall indicates whether a node represents a canonical `super(..)` base class constructor invocation.
    // This requires digging through a bunch of properties on the given node and reverse engineering the code pattern.
    private isSuperCall(node: ast.Statement, superclass: tokens.TypeToken): boolean {
        if (node.kind !== ast.expressionStatementKind) {
            return false;
        }

        let exprstmt = <ast.ExpressionStatement>node;
        if (exprstmt.expression.kind !== ast.invokeFunctionExpressionKind) {
            return false;
        }

        let invoke = <ast.InvokeFunctionExpression>exprstmt.expression;
        if (invoke.function.kind !== ast.loadLocationExpressionKind) {
            return false;
        }

        let ldloc = <ast.LoadLocationExpression>invoke.function;
        if (!ldloc.object || ldloc.object.kind !== ast.loadLocationExpressionKind) {
            return false;
        }
        if (ldloc.name.tok !== this.createClassMemberToken(superclass, tokens.constructorFunction)) {
            return false;
        }
        let ldobjloc = <ast.LoadLocationExpression>ldloc.object;
        return !ldobjloc.object && ldloc.name.tok === tokens.superVariable;
    }

    // createEmptySuperCall manufactures a synthetic call to a base class, with no arguments (i.e., `super();`).
    private createEmptySuperCall(superclass: tokens.TypeToken): ast.ExpressionStatement {
        return <ast.ExpressionStatement>{
            kind:       ast.expressionStatementKind,
            expression: <ast.InvokeFunctionExpression>{
                kind:     ast.invokeFunctionExpressionKind,
                function: <ast.LoadLocationExpression>{
                    kind: ast.loadLocationExpressionKind,
                    object: <ast.LoadLocationExpression>{
                        kind: ast.loadLocationExpressionKind,
                        name: <ast.Token>{
                            kind: ast.tokenKind,
                            tok:  tokens.superVariable,
                        },
                    },
                    name: <ast.Token>{
                        kind: ast.tokenKind,
                        tok:  this.createClassMemberToken(superclass, tokens.constructorFunction),
                    },
                },
            },
        };
    }

    private async transformCallExpression(node: ts.CallExpression): Promise<ast.InvokeFunctionExpression> {
        let func: ast.Expression = await this.transformExpression(node.expression);

        // In the special case of a `super(<args>)` call, we need to transform the `super` from a plain old variable
        // load of the base object, into an actual invokable constructor function reference.
        if (node.expression.kind === ts.SyntaxKind.SuperKeyword) {
            contract.assert(!!this.currentSuperClassToken);
            func = this.withLocation(node.expression, <ast.LoadLocationExpression>{
                kind:   ast.loadLocationExpressionKind,
                object: func,
                name:   <ast.Token>{
                    kind: ast.tokenKind,
                    tok:  this.createClassMemberToken(this.currentSuperClassToken!, tokens.constructorFunction),
                },
            });
        }

        let args: ast.Expression[] = [];
        for (let argument of node.arguments) {
            args.push(await this.transformExpression(argument));
        }
        return this.withLocation(node, <ast.InvokeFunctionExpression>{
            kind:      ast.invokeFunctionExpressionKind,
            function:  func,
            arguments: args,
        });
    }

    private transformClassExpression(node: ts.ClassExpression): ast.Expression {
        return notYetImplemented(node);
    }

    private async transformConditionalExpression(node: ts.ConditionalExpression): Promise<ast.ConditionalExpression> {
        return this.withLocation(node, <ast.ConditionalExpression>{
            kind:       ast.conditionalExpressionKind,
            condition:  await this.transformExpression(node.condition),
            consequent: await this.transformExpression(node.whenTrue),
            alternate:  await this.transformExpression(node.whenFalse),
        });
    }

    private async transformDeleteExpression(node: ts.DeleteExpression): Promise<ast.Expression> {
        if (log.v(3)) {
            log.out(3).info(
                `ECMAScript operator 'delete' not supported; ` +
                `until marapongo/mu#50 is implemented, be careful about subtle behavioral differences`,
            );
        }
        // TODO[marapongo/mu#50]: we need to decide how to map `delete` into a runtime MuIL operator.  It's possible
        //     this can leverage some dynamic trickery to delete an entry from a map.  But for strong typing reasons,
        //     this is dubious (at best); for now, we will simply `null` the target out, however, this will cause
        //     problems down the road once we properly support nullable types.
        return this.withLocation(node, <ast.BinaryOperatorExpression>{
            kind:     ast.binaryOperatorExpressionKind,
            left:     await this.transformExpression(node.expression),
            operator: "=",
            right:    <ast.NullLiteral>{
                kind: ast.nullLiteralKind,
            },
        });
    }

    private async transformElementAccessExpression(node: ts.ElementAccessExpression): Promise<ast.LoadExpression> {
        // This is an indexer operation; fall back to using a dynamic load operation.
        // TODO: detect array, string constant property loads, and module member loads.
        let object: ast.Expression = await this.transformExpression(node.expression);
        if (node.argumentExpression) {
            return this.withLocation(node, <ast.LoadDynamicExpression>{
                kind:   ast.loadDynamicExpressionKind,
                object: object,
                name:   await this.transformExpression(node.argumentExpression),
            });
        }
        else {
            return object;
        }
    }

    private transformFunctionExpression(node: ts.FunctionExpression): ast.Expression {
        // TODO[marapongo/mu#62]: implement lambdas.
        return notYetImplemented(node);
    }

    private async transformObjectLiteralExpression(node: ts.ObjectLiteralExpression): Promise<ast.ObjectLiteral> {
        // TODO[marapongo/mu#46]: because TypeScript object literals are untyped, it's not clear what MuIL type this
        //     expression should produce.  It's common for a TypeScript literal to be enclosed in a cast, for example,
        //     `<SomeType>{ literal }`, in which case, perhaps we could detect `<SomeType>`.  Alternatively, MuIL could
        //     just automatically dynamically coerce `any` to the target type, similar to TypeScript, when necessary.
        //     I had envisioned requiring explicit dynamic casts for this, in which case, perhaps this expression should
        //     always be encased in something that prepares it for dynamic cast in the consuming expression.
        let properties: ast.ObjectLiteralProperty[] = [];
        for (let prop of node.properties) {
            properties.push(await this.transformObjectLiteralElement(prop));
        }
        return this.withLocation(node, <ast.ObjectLiteral>{
            kind:       ast.objectLiteralKind,
            type:       <ast.TypeToken>{
                kind: ast.typeTokenKind,
                tok:  tokens.dynamicType,
            },
            properties: properties,
        });
    }

    private async transformObjectLiteralElement(node: ts.ObjectLiteralElement): Promise<ast.ObjectLiteralProperty> {
        switch (node.kind) {
            case ts.SyntaxKind.PropertyAssignment:
                return await this.transformObjectLiteralPropertyAssignment(<ts.PropertyAssignment>node);
            case ts.SyntaxKind.ShorthandPropertyAssignment:
                return this.transformObjectLiteralShorthandPropertyAssignment(<ts.ShorthandPropertyAssignment>node);

            case ts.SyntaxKind.GetAccessor:
                return this.transformObjectLiteralFunctionLikeElement(<ts.GetAccessorDeclaration>node);
            case ts.SyntaxKind.SetAccessor:
                return this.transformObjectLiteralFunctionLikeElement(<ts.SetAccessorDeclaration>node);
            case ts.SyntaxKind.MethodDeclaration:
                return this.transformObjectLiteralFunctionLikeElement(<ts.MethodDeclaration>node);

            default:
                return contract.fail(`Unrecognized object literal element kind ${ts.SyntaxKind[node.kind]}`);
        }
    }

    private async transformObjectLiteralPropertyAssignment(
            node: ts.PropertyAssignment): Promise<ast.ObjectLiteralProperty> {
        let pname: ast.Identifier = this.transformPropertyName(node.name);
        return this.withLocation(node, <ast.ObjectLiteralProperty>{
            kind:     ast.objectLiteralPropertyKind,
            property: <ast.Token>{
                kind: ast.tokenKind,
                tok:  pname.ident /*simple name, since this is dynamic*/,
            },
            value:    await this.transformExpression(node.initializer),
        });
    }

    private transformObjectLiteralShorthandPropertyAssignment(
            node: ts.ShorthandPropertyAssignment): ast.ObjectLiteralProperty {
        let name: ast.Identifier = this.transformIdentifier(node.name);
        return this.withLocation(node, <ast.ObjectLiteralProperty>{
            kind:     ast.objectLiteralPropertyKind,
            property: <ast.Token>{
                kind: ast.tokenKind,
                tok:  name.ident /*simple name, since this is dynamic*/,
            },
            value: this.withLocation(node.name, <ast.LoadLocationExpression>{
                kind: ast.loadLocationExpressionKind,
                name: this.copyLocation(name, <ast.Token>{
                    kind: ast.tokenKind,
                    tok:  name.ident,
                }),
            }),
        });
    }

    private transformObjectLiteralFunctionLikeElement(node: ts.FunctionLikeDeclaration): ast.ObjectLiteralProperty {
        // TODO[marapongo/mu#62]: implement lambdas.
        return notYetImplemented(node);
    }

    private async transformPostfixUnaryExpression(
            node: ts.PostfixUnaryExpression): Promise<ast.UnaryOperatorExpression> {
        let operator: ast.UnaryOperator | undefined = postfixUnaryOperators.get(node.operator);
        contract.assert(!!(operator = operator!));
        return this.withLocation(node, <ast.UnaryOperatorExpression>{
            kind:     ast.unaryOperatorExpressionKind,
            postfix:  true,
            operator: operator,
            operand:  await this.transformExpression(node.operand),
        });
    }

    private async transformPrefixUnaryExpression(
            node: ts.PrefixUnaryExpression): Promise<ast.UnaryOperatorExpression> {
        let operator: ast.UnaryOperator | undefined = prefixUnaryOperators.get(node.operator);
        contract.assert(!!(operator = operator!));
        return this.withLocation(node, <ast.UnaryOperatorExpression>{
            kind:     ast.unaryOperatorExpressionKind,
            postfix:  false,
            operator: operator,
            operand:  await this.transformExpression(node.operand),
        });
    }

    private async transformPropertyAccessExpression(
            node: ts.PropertyAccessExpression): Promise<ast.LoadExpression> {
        return await this.createLoadExpression(node, node.expression, node.name);
    }

    private async transformNewExpression(node: ts.NewExpression): Promise<ast.NewExpression> {
        // To transform the new expression, find the signature TypeScript has bound it to.
        let signature: ts.Signature = this.checker().getResolvedSignature(node);
        contract.assert(!!signature);
        let typeToken: tokens.TypeToken | undefined = await this.resolveTypeToken(node, signature.getReturnType());
        contract.assert(!!typeToken);
        let args: ast.Expression[] = [];
        for (let expr of node.arguments) {
            args.push(await this.transformExpression(expr));
        }
        return this.withLocation(node, <ast.NewExpression>{
            kind:      ast.newExpressionKind,
            type:      this.withLocation(node.expression, <ast.TypeToken>{
                kind: ast.typeTokenKind,
                tok:  typeToken!,
            }),
            arguments: args,
        });
    }

    private transformOmittedExpression(node: ts.OmittedExpression): ast.Expression {
        return notYetImplemented(node);
    }

    // transformParenthesizedExpression simply emits the underlying expression.  The TypeScript compiler has already
    // taken care of expression precedence by the time we reach this, and the MuIL AST is blisfully unaware.
    private async transformParenthesizedExpression(node: ts.ParenthesizedExpression): Promise<ast.Expression> {
        return await this.transformExpression(node.expression);
    }

    private transformSpreadElement(node: ts.SpreadElement): ast.Expression {
        return notYetImplemented(node);
    }

    private transformSuperExpression(node: ts.SuperExpression): ast.LoadLocationExpression {
        let id: ast.Identifier = this.withLocation(node, ident(tokens.superVariable));
        return this.withLocation(node, <ast.LoadLocationExpression>{
            kind: ast.loadLocationExpressionKind,
            name: this.copyLocation(id, <ast.Token>{
                kind: ast.tokenKind,
                tok:  id.ident,
            }),
        });
    }

    private transformTaggedTemplateExpression(node: ts.TaggedTemplateExpression): ast.Expression {
        return notYetImplemented(node);
    }

    private transformTemplateExpression(node: ts.TemplateExpression): ast.Expression {
        return notYetImplemented(node);
    }

    private transformThisExpression(node: ts.ThisExpression): ast.LoadLocationExpression {
        let id: ast.Identifier = this.withLocation(node, ident(tokens.thisVariable));
        return this.withLocation(node, <ast.LoadLocationExpression>{
            kind: ast.loadLocationExpressionKind,
            name: this.copyLocation(id, <ast.Token>{
                kind: ast.tokenKind,
                tok:  id.ident,
            }),
        });
    }

    private async transformTypeAssertionExpression(node: ts.TypeAssertion): Promise<ast.CastExpression> {
        let tytok: ast.TypeToken | undefined = await this.resolveTypeTokenFromTypeLike(node);
        contract.assert(!!tytok);
        return this.withLocation(node, <ast.CastExpression>{
            kind:       ast.castExpressionKind,
            expression: await this.transformExpression(node.expression),
            type:       tytok,
        });
    }

    private async transformTypeOfExpression(node: ts.TypeOfExpression): Promise<ast.Expression> {
        return this.withLocation(node, <ast.TypeOfExpression>{
            kind:       ast.typeOfExpressionKind,
            expression: await this.transformExpression(node.expression),
        });
    }

    private transformVoidExpression(node: ts.VoidExpression): ast.Expression {
        return notYetImplemented(node);
    }

    private transformYieldExpression(node: ts.YieldExpression): ast.Expression {
        return notYetImplemented(node);
    }

    /** Literals **/

    private transformBooleanLiteral(node: ts.BooleanLiteral): ast.BoolLiteral {
        contract.assert(node.kind === ts.SyntaxKind.FalseKeyword || node.kind === ts.SyntaxKind.TrueKeyword);
        return this.withLocation(node, <ast.BoolLiteral>{
            kind:  ast.boolLiteralKind,
            raw:   node.getText(),
            value: (node.kind === ts.SyntaxKind.TrueKeyword),
        });
    }

    private transformNoSubstitutionTemplateLiteral(node: ts.NoSubstitutionTemplateLiteral): ast.Expression {
        return notYetImplemented(node);
    }

    private transformNullLiteral(node: ts.NullLiteral): ast.NullLiteral {
        return this.withLocation(node, <ast.NullLiteral>{
            kind: ast.nullLiteralKind,
            raw:  node.getText(),
        });
    }

    private transformNumericLiteral(node: ts.NumericLiteral): ast.NumberLiteral {
        return this.withLocation(node, <ast.NumberLiteral>{
            kind:  ast.numberLiteralKind,
            raw:   node.text,
            value: Number(node.text),
        });
    }

    private transformRegularExpressionLiteral(node: ts.RegularExpressionLiteral): ast.Expression {
        return notYetImplemented(node);
    }

    private transformStringLiteral(node: ts.StringLiteral): ast.StringLiteral {
        // TODO: we need to dynamically populate the resulting object with ECMAScript-style string functions.  It's not
        //     yet clear how to do this in a way that facilitates inter-language interoperability.  This is especially
        //     challenging because most use of such functions will be entirely dynamic.
        return this.withLocation(node, <ast.StringLiteral>{
            kind:  ast.stringLiteralKind,
            raw:   node.text,
            value: node.text,
        });
    }

    /** Patterns **/

    private transformArrayBindingPattern(node: ts.ArrayBindingPattern): ast.Expression {
        return notYetImplemented(node);
    }

    private transformArrayBindingElement(node: ts.ArrayBindingElement): (ast.Expression | null) {
        return notYetImplemented(node);
    }

    private transformBindingName(node: ts.BindingName): ast.Expression {
        return notYetImplemented(node);
    }

    private transformBindingIdentifier(node: ts.BindingName): ast.Identifier {
        contract.assert(node.kind === ts.SyntaxKind.Identifier,
                        "Binding name must be an identifier (TODO[marapongo/mu#34])");
        return this.transformIdentifier(<ts.Identifier>node);
    }

    private transformBindingPattern(node: ts.BindingPattern): ast.Expression {
        return notYetImplemented(node);
    }

    private transformComputedPropertyName(node: ts.ComputedPropertyName): ast.Expression {
        return notYetImplemented(node);
    }

    // transformIdentifierExpression takes a TypeScript identifier node and yields a MuIL expression.  This expression,
    // when evaluated, will load the value of the target identifier, so that it's suitable as an expression node.
    private async transformIdentifierExpression(node: ts.Identifier): Promise<ast.Expression> {
        if (node.text === "null" || node.text === "undefined") {
            // For null and undefined, load a null literal.
            return this.withLocation(node, <ast.NullLiteral>{
                kind: ast.nullLiteralKind,
            });
        }
        else {
            // For other identifiers, transform them into loads.
            return this.createLoadExpression(node, undefined, node);
        }
    }

    private transformObjectBindingPattern(node: ts.ObjectBindingPattern): ast.Expression {
        return notYetImplemented(node);
    }

    private transformObjectBindingElement(node: ts.BindingElement): ast.Expression {
        return notYetImplemented(node);
    }

    private transformPropertyName(node: ts.PropertyName): ast.Identifier {
        switch (node.kind) {
            case ts.SyntaxKind.Identifier:
                return this.transformIdentifier(<ts.Identifier>node);
            case ts.SyntaxKind.StringLiteral:
                return this.withLocation(node, ident((<ts.StringLiteral>node).text));
            default:
                return contract.fail("Property names other than identifiers and string literals not yet supported");
        }
    }
}

// Loads the metadata and transforms a TypeScript program into its equivalent MuPack/MuIL AST form.
export async function transform(script: Script): Promise<TransformResult> {
    let loader: PackageLoader = new PackageLoader();
    let disc: PackageResult = await loader.loadCurrent(script.root);
    let result: TransformResult = {
        diagnostics: disc.diagnostics, // ensure we propagate the diagnostics
        pkg:         undefined,
    };

    if (disc.pkg) {
        // New up a transformer and do it.
        let t = new Transformer(disc.pkg, script, loader);
        let trans: TransformResult = await t.transform();

        // Copy the return to our running result, so we propagate the aggregate of all diagnostics.
        result.diagnostics = result.diagnostics.concat(trans.diagnostics);
        result.pkg = trans.pkg;
    }

    return result;
}

export interface TransformResult {
    diagnostics: diag.Diagnostic[];        // any diagnostics resulting from translation.
    pkg:         pack.Package | undefined; // the resulting MuPack/MuIL AST.
}

