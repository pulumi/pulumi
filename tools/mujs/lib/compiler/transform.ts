// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import {contract, log, object} from "nodets";
import * as fspath from "path";
import * as ts from "typescript";
import * as ast from "../ast";
import * as diag from "../diag";
import * as pack from "../pack";
import * as symbols from "../symbols";
import {discover, DiscoverResult} from "./discover";
import {Script} from "./script";

const defaultExport: string = "default"; // the ES6 default export name.

// A mapping from TypeScript operator to Mu AST operator.
let binaryOperators = new Map<ts.SyntaxKind, ast.BinaryOperator>([
    // Arithmetic
    [ ts.SyntaxKind.PlusToken,                              "+" ],
    [ ts.SyntaxKind.MinusToken,                             "-" ],
    [ ts.SyntaxKind.AsteriskToken,                          "*" ],
    [ ts.SyntaxKind.SlashToken,                             "/" ],
    [ ts.SyntaxKind.PercentToken,                           "%" ],
    [ ts.SyntaxKind.AsteriskAsteriskToken,                  "**" ],

    // Assignment
    [ ts.SyntaxKind.EqualsToken,                                  "=" ],
    [ ts.SyntaxKind.PlusEqualsToken,                              "+=" ],
    [ ts.SyntaxKind.MinusEqualsToken,                             "-=" ],
    [ ts.SyntaxKind.AsteriskEqualsToken,                          "*=" ],
    [ ts.SyntaxKind.SlashEqualsToken,                             "/=" ],
    [ ts.SyntaxKind.PercentEqualsToken,                           "%=" ],
    [ ts.SyntaxKind.AsteriskAsteriskEqualsToken,                  "**=" ],
    [ ts.SyntaxKind.LessThanLessThanEqualsToken,                  "<<=" ],
    [ ts.SyntaxKind.GreaterThanGreaterThanEqualsToken,            ">>=" ],
    [ ts.SyntaxKind.GreaterThanGreaterThanGreaterThanEqualsToken, ">>=" ], // TODO[marapongo/mu#50]: emulate >>>=.
    [ ts.SyntaxKind.AmpersandEqualsToken,                         "&=" ],
    [ ts.SyntaxKind.BarEqualsToken,                               "|=" ],
    [ ts.SyntaxKind.CaretEqualsToken,                             "^=" ],

    // Bitwise
    [ ts.SyntaxKind.LessThanLessThanToken,                  "<<" ],
    [ ts.SyntaxKind.GreaterThanGreaterThanToken,            ">>" ],
    [ ts.SyntaxKind.GreaterThanGreaterThanGreaterThanToken, ">>" ], // TODO[marapongo/mu#50]: emulate >>>.
    [ ts.SyntaxKind.BarToken,                               "|" ],
    [ ts.SyntaxKind.CaretToken,                             "^" ],
    [ ts.SyntaxKind.AmpersandToken,                         "&" ],

    // Logical
    [ ts.SyntaxKind.AmpersandAmpersandToken, "&&" ],
    [ ts.SyntaxKind.BarBarToken,             "||" ],

    // Relational
    [ ts.SyntaxKind.LessThanToken,                          "<" ],
    [ ts.SyntaxKind.LessThanEqualsToken,                    "<=" ],
    [ ts.SyntaxKind.GreaterThanToken,                       ">" ],
    [ ts.SyntaxKind.GreaterThanEqualsToken,                 ">=" ],
    [ ts.SyntaxKind.EqualsEqualsToken,                      "==" ],
    [ ts.SyntaxKind.EqualsEqualsEqualsToken,                "==" ], // TODO[marapongo/mu#50]: emulate ===.
    [ ts.SyntaxKind.ExclamationEqualsToken,                 "!=" ],
    [ ts.SyntaxKind.ExclamationEqualsEqualsToken,           "!=" ], // TODO[marapongo/mu#50]: emulate !==.

    // Intrinsics
    // TODO: [ ts.SyntaxKind.InKeyword,                              "in" ],
    // TODO: [ ts.SyntaxKind.InstanceOfKeyword,                      "instanceof" ],
]);

let prefixUnaryOperators = new Map<ts.SyntaxKind, ast.UnaryOperator>([
    [ ts.SyntaxKind.PlusPlusToken,    "++" ],
    [ ts.SyntaxKind.MinusMinusToken,  "--" ],
    [ ts.SyntaxKind.PlusToken,        "+" ],
    [ ts.SyntaxKind.MinusToken,       "-" ],
    [ ts.SyntaxKind.TildeToken,       "~" ],
    [ ts.SyntaxKind.ExclamationToken, "!" ],
]);

let postfixUnaryOperators = new Map<ts.SyntaxKind, ast.UnaryOperator>([
    [ ts.SyntaxKind.PlusPlusToken,   "++" ],
    [ ts.SyntaxKind.MinusMinusToken, "--" ],
]);

// A variable is a MuIL variable with an optional initializer expression.  This is required because MuIL doesn't support
// complex initializers on the Variable AST node -- they must be explicitly placed into an initializer section.
class VariableDeclaration<TVariable extends ast.Variable> {
    constructor(
        public node:         ts.Node,        // the source node.
        public variable:     TVariable,      // the MuIL variable information.
        public legacyVar?:   boolean,        // true to mimick legacy ECMAScript "var" behavior; false for "let".
        public initializer?: ast.Expression, // an optional initialization expression.
    ) { }
}

// A top-level module element is either a module member (definition) or a statement (initializer).
type ModuleElement = ast.ModuleMember | VariableDeclaration<ast.ModuleProperty> | ast.Statement;

// A top-level class element is either a class member (definition) or a statement (initializer).
type ClassElement = ast.ClassMember | VariableDeclaration<ast.ClassProperty>;

function isVariableDeclaration(element: ModuleElement | ClassElement): boolean {
    return !!(element instanceof VariableDeclaration);
}

// A variable declaration isn't yet known to be a module or class property, and so it just contains the subset in common
// between them.  This facilitates code reuse in the translation passes.
interface VariableLikeDeclaration {
    name:         ast.Identifier;
    type:         symbols.TypeToken;
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
    returnType?: symbols.TypeToken;
}

function ident(id: string): ast.Identifier {
    return {
        kind:  ast.identifierKind,
        ident: id,
    };
}

function isComputed(name: ts.Node | undefined): boolean {
    if (name) {
        return (name.kind === ts.SyntaxKind.ComputedPropertyName);
    }
    return false;
}

// notYetImplemented simply fail-fasts, but does so in a way where we at least get Node source information.
function notYetImplemented(node: ts.Node | undefined): never {
    let msg: string = "Not Yet Implemented";
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
    private meta: pack.Metadata;            // the package's metadata.
    private script: Script;                 // the package's compiled TypeScript tree and context.
    private dctx: diag.Context;             // the diagnostics context.
    private diagnostics: diag.Diagnostic[]; // any diagnostics encountered during translation.

    constructor(meta: pack.Metadata, script: Script) {
        contract.requires(!!script.tree, "script", "A valid MuJS AST is required to lower to MuPack/MuIL");
        this.meta = meta;
        this.script = script;
        this.dctx = new diag.Context(script.root);
        this.diagnostics = [];
    }

    // Translates a TypeScript bound tree into its equivalent MuPack/MuIL AST form, one module per file.
    public transform(): TransformResult {
        // Enumerate all source files (each of which is a module in ECMAScript), and transform it.
        let modules: ast.Modules = {};
        for (let sourceFile of this.script.tree!.getSourceFiles()) {
            // By default, skip declaration files, since they are "dependencies."
            // TODO(joe): how to handle re-exports in ECMAScript, such as index aggregation.
            // TODO(joe): this isn't a perfect heuristic.  But ECMAScript is all source dependencies, so there isn't a
            //     true notion of source versus binary dependency.  We could crack open the dependencies to see if they
            //     exist within an otherwise known package, but that seems a little hokey.
            if (!sourceFile.isDeclarationFile) {
                let mod: ast.Module = this.transformSourceFile(sourceFile, this.script.root);
                modules[mod.name.ident] = mod;
            }
        }

        // Now create a new package object.
        // TODO: create a list of dependencies, partly from the metadata, partly from the TypeScript compilation.
        return <TransformResult>{
            diagnostics: this.diagnostics,
            pack:        object.extend(this.meta, {
                modules: modules,
            }),
        };
    }

    /** Helpers **/

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

    /** Symbols **/

    private transformIdentifier(node: ts.Identifier): ast.Identifier {
        return this.withLocation(node, ident(node.text));
    }

    private transformTypeNode(node: ts.TypeNode | undefined): symbols.TypeToken {
        // TODO[marapongo/mu#46]: emit strong typing information.
        return "any";
    }

    /** Modules **/

    // This transforms top-level TypeScript module elements into their corresponding nodes.  This transformation
    // is largely evident in how it works, except that "loose code" (arbitrary statements) is not permitted in
    // MuPack/MuIL.  As such, the appropriate top-level definitions (variables, functions, and classes) are returned as
    // definitions, while any loose code (including variable initializers) is bundled into module inits and entrypoints.
    private transformSourceFile(node: ts.SourceFile, root: string): ast.Module {
        // All definitions will go into a map keyed by their identifier.
        let members: ast.ModuleMembers = {};

        // Any top-level non-definition statements will pile up into the module initializer.
        let statements: ast.Statement[] = [];

        // Enumerate the module's statements and put them in the respective places.
        for (let statement of node.statements) {
            let elements: ModuleElement[] = this.transformSourceFileStatement(statement);
            for (let element of elements) {
                if (isVariableDeclaration(element)) {
                    // This is a module property with a possible initializer.  The property should get registered as a
                    // member in this module's member map, and the initializer must happen in the module initializer.
                    // TODO(joe): respect legacyVar to emulate "var"-like scoping.
                    let decl = <VariableDeclaration<ast.ModuleProperty>>element;
                    if (decl.initializer) {
                        statements.push(this.makeVariableInitializer(decl));
                    }
                    members[decl.variable.name.ident] = decl.variable;
                }
                else if (ast.isDefinition(<ast.Node>element)) {
                    // This is a module member; simply add it to the list.
                    let member = <ast.ModuleMember>element;
                    members[member.name.ident] = member;
                }
                else {
                    // This is a top-level module statement; place it into the module initializer.
                    statements.push(<ast.Statement>element);
                }
            }
         }

        // If the initialization statements are non-empty, add an initializer method.
        if (statements.length > 0) {
            let initializer: ast.ModuleMethod = {
                kind:   ast.moduleMethodKind,
                name:   ident(symbols.specialFunctionInitializer),
                access: symbols.publicAccessibility,
                body:   {
                    kind:       ast.blockKind,
                    statements: statements,
                },
            };
            members[initializer.name.ident] = initializer;
        }

        // To create a module name, make it relative to the current root directory, and lop off the extension.
        // TODO(joe): this still isn't 100% correct, because we might have ".."s for "up and over" module references.
        let moduleName: string = fspath.relative(root, node.fileName);
        let moduleExtIndex: number = moduleName.lastIndexOf(".");
        if (moduleExtIndex !== -1) {
            moduleName = moduleName.substring(0, moduleExtIndex);
        }

        return this.withLocation(node, <ast.Module>{
            kind:    ast.moduleKind,
            name:    ident(moduleName),
            members: members,
        });
    }

    // This transforms a top-level TypeScript module statement.  It might return multiple elements in the rare
    // cases -- like variable initializers -- that expand to multiple elements.
    private transformSourceFileStatement(node: ts.Statement): ModuleElement[] {
        if (ts.getCombinedModifierFlags(node) & ts.ModifierFlags.Export) {
            return this.transformExportStatement(node);
        }
        else {
            switch (node.kind) {
                // Handle module directives; most translate into definitions.
                case ts.SyntaxKind.ExportAssignment:
                    return [ this.transformExportAssignment(<ts.ExportAssignment>node) ];
                case ts.SyntaxKind.ExportDeclaration:
                    return [ this.transformExportDeclaration(<ts.ExportDeclaration>node) ];
                case ts.SyntaxKind.ImportDeclaration:
                    return [ this.transformImportDeclaration(<ts.ImportDeclaration>node) ];

                // Handle declarations; each of these results in a definition.
                case ts.SyntaxKind.ClassDeclaration:
                case ts.SyntaxKind.FunctionDeclaration:
                case ts.SyntaxKind.InterfaceDeclaration:
                case ts.SyntaxKind.ModuleDeclaration:
                case ts.SyntaxKind.TypeAliasDeclaration:
                case ts.SyntaxKind.VariableStatement:
                    return this.transformModuleDeclarationStatement(node, symbols.privateAccessibility);

                // For any other top-level statements, this.transform them.  They'll be added to the module initializer.
                default:
                    return [ this.transformStatement(node) ];
            }
        }
    }

    private transformExportStatement(node: ts.Statement): ModuleElement[] {
        let elements: ModuleElement[] = this.transformModuleDeclarationStatement(node, symbols.publicAccessibility);

        // If this is a default export, first ensure that it is one of the legal default export kinds; namely, only
        // function or class is permitted, and specifically not interface or let.  Then smash the name with "default".
        if (ts.getCombinedModifierFlags(node) & ts.ModifierFlags.Default) {
            contract.assert(elements.length === 1);
            let defn = <ast.Definition>elements[0];
            contract.assert(defn.kind === ast.moduleMethodKind || defn.kind === ast.classKind);
            defn.name = ident(defaultExport);
        }

        return elements;
    }

    private transformExportAssignment(node: ts.ExportAssignment): ast.Definition {
        return notYetImplemented(node);
    }

    private transformExportDeclaration(node: ts.ExportDeclaration): ast.Definition {
        return notYetImplemented(node);
    }

    private transformExportSpecifier(node: ts.ExportSpecifier): ast.Definition {
        return notYetImplemented(node);
    }

    private transformImportDeclaration(node: ts.ImportDeclaration): ModuleElement {
        // TODO[marapongo/mu#46]: we are ignoring import declarations for the time being.  Eventually we need to
        //     transform all dependency symbols into real MuIL references.  (Today, bound node information is
        //     discarded.)  When that day comes (soon), import declarations will most likely still be ignored, however,
        //     I am leaving this comment in here so that we can make an explicit decision about this.
        return <ast.EmptyStatement>{ kind: ast.emptyStatementKind };
    }

    /** Statements **/

    private transformStatement(node: ts.Statement): ast.Statement {
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
                return this.transformLocalVariableStatement(<ts.VariableStatement>node);

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
                return this.transformExpressionStatement(<ts.ExpressionStatement>node);
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
    private transformModuleDeclarationStatement(node: ts.Statement, access: symbols.Accessibility): ModuleElement[] {
        switch (node.kind) {
            // Declarations:
            case ts.SyntaxKind.ClassDeclaration:
                return [ this.transformClassDeclaration(<ts.ClassDeclaration>node, access) ];
            case ts.SyntaxKind.FunctionDeclaration:
                return [ this.transformModuleFunctionDeclaration(<ts.FunctionDeclaration>node, access) ];
            case ts.SyntaxKind.InterfaceDeclaration:
                return [ this.transformInterfaceDeclaration(<ts.InterfaceDeclaration>node, access) ];
            case ts.SyntaxKind.ModuleDeclaration:
                return [ this.transformModuleDeclaration(<ts.ModuleDeclaration>node, access) ];
            case ts.SyntaxKind.TypeAliasDeclaration:
                return [ this.transformTypeAliasDeclaration(<ts.TypeAliasDeclaration>node, access) ];
            case ts.SyntaxKind.VariableStatement:
                return this.transformModuleVariableStatement(<ts.VariableStatement>node, access);
            default:
                return contract.fail(`Node kind is not a module declaration: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    // This transforms a TypeScript Statement, and returns a Block (allocating a new one if needed).
    private transformStatementAsBlock(node: ts.Statement): ast.Block {
        // Transform the statement.  Then, if it is already a block, return it; otherwise, append it to a new one.
        let statement: ast.Statement = this.transformStatement(node);
        if (statement.kind === ast.blockKind) {
            return <ast.Block>statement;
        }
        return this.withLocation(node, <ast.Block>{
            kind:       ast.blockKind,
            statements: [ statement ],
        });
    }

    /** Declaration statements **/

    private transformClassDeclaration(node: ts.ClassDeclaration, access: symbols.Accessibility): ast.Class {
        // TODO(joe): generics.
        // TODO(joe): decorators.
        // TODO(joe): extends/implements.

        // First transform the name into an identifier.  In the absence of a name, we will proceed under the assumption
        // that it is the default export.  This should be verified later on.
        let name: ast.Identifier;
        if (node.name) {
            name = this.transformIdentifier(node.name);
        }
        else {
            name = ident(defaultExport);
        }

        // Transform all non-semicolon members for this declaration into ClassMembers.
        let elements: ClassElement[] = [];
        for (let member of node.members) {
            if (member.kind !== ts.SyntaxKind.SemicolonClassElement) {
                elements.push(this.transformClassElement(member));
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

        // For all class properties with default values, we need to spill the initializer into the constructor.  This
        // is non-trivial, because the class may not have an explicit constructor.  If it doesn't we need to generate
        // one.  In either case, we must be careful to respect initialization order with respect to super calls.
        // Namely, all property initializers must occur *after* the invocation of `super()`.
        let propertyInitializers: ast.Statement[] = [];
        for (let element of elements) {
            if (isVariableDeclaration(element)) {
                let decl = <VariableDeclaration<ast.ClassProperty>>element;
                if (decl.initializer) {
                    propertyInitializers.push(this.makeVariableInitializer(decl));
                }
                members[decl.variable.name.ident] = decl.variable;
            }
        }
        if (propertyInitializers.length > 0) {
            // Locate the constructor, possibly fabricating one if necessary.
            let ctor: ast.ClassMethod | undefined =
                <ast.ClassMethod>members[symbols.specialFunctionConstructor];
            if (!ctor) {
                // TODO: once we support base classes, inject a call to super() at the front.
                ctor = members[symbols.specialFunctionConstructor] = <ast.ClassMethod>{
                    kind: ast.classMethodKind,
                    name: ident(symbols.specialFunctionConstructor),
                };
            }
            if (!ctor.body) {
                ctor.body = <ast.Block>{
                    kind:       ast.blockKind,
                    statements: [],
                };
            }
            // TODO: once we support base classes, search for the super() call and append afterwards.
            ctor.body.statements = propertyInitializers.concat(ctor.body.statements);
        }

        let mods: ts.ModifierFlags = ts.getCombinedModifierFlags(node);
        return this.withLocation(node, <ast.Class>{
            kind:     ast.classKind,
            name:     name,
            access:   access,
            members:  members,
            abstract: !!(mods & ts.ModifierFlags.Abstract),
        });
    }

    private transformDeclarationName(node: ts.DeclarationName): ast.Expression {
        switch (node.kind) {
            case ts.SyntaxKind.ArrayBindingPattern:
                return this.transformArrayBindingPattern(node);
            case ts.SyntaxKind.ComputedPropertyName:
                return this.transformComputedPropertyName(node);
            case ts.SyntaxKind.ObjectBindingPattern:
                return this.transformObjectBindingPattern(node);
            case ts.SyntaxKind.Identifier:
                return this.transformIdentifierExpression(node);
            default:
                return contract.fail(`Unrecognized declaration node: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    private transformDeclarationIdentifier(node: ts.DeclarationName): ast.Identifier {
        switch (node.kind) {
            case ts.SyntaxKind.Identifier:
                return this.transformIdentifierExpression(node);
            default:
                return contract.fail(`Unrecognized declaration identifier: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    private transformFunctionLikeCommon(node: ts.FunctionLikeDeclaration): FunctionLikeDeclaration {
        if (!!node.asteriskToken) {
            this.diagnostics.push(this.dctx.newGeneratorsNotSupportedError(node.asteriskToken));
        }

        // First, visit the body; it can either be a block or a free-standing expression.
        let body: ast.Block | undefined;
        if (node.body) {
            switch (node.body.kind) {
                case ts.SyntaxKind.Block:
                    body = this.transformBlock(<ts.Block>node.body);
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
                                expression: this.transformExpression(<ts.Expression>node.body),
                            },
                        ],
                    };
                    break;
            }
        }
        return this.transformFunctionLikeOrSignatureCommon(node, body);
    }

    // A common routine for transforming FunctionLikeDeclarations and MethodSignatures.  The return is specialized per
    // callsite, since differs slightly between module methods, class and interface methods, lambdas, and so on.
    private transformFunctionLikeOrSignatureCommon(
            node: ts.FunctionLikeDeclaration | ts.MethodSignature, body?: ast.Block): FunctionLikeDeclaration {
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
            name = ident(symbols.specialFunctionConstructor);
        }
        else {
            // All others are assumed to be default exports.
            name = ident(defaultExport);
        }

        // Next transform the parameter variables into locals.
        let parameters: VariableDeclaration<ast.LocalVariable>[] = node.parameters.map(
            (param: ts.ParameterDeclaration) => this.transformParameterDeclaration(param));

        // If there are any initializers, make sure to prepend them (in order) to the body block.
        for (let parameter of parameters) {
            if (parameter.initializer && body) {
                body.statements = [ this.makeVariableInitializer(parameter) ].concat(body.statements);
            }
        }

        // Delegate to the factory method to turn this into a real function object.
        return {
            name:       name,
            parameters: parameters.map((p: VariableDeclaration<ast.LocalVariable>) => p.variable),
            body:       body,
            returnType: this.transformTypeNode(node.type),
        };
    }

    private transformModuleFunctionDeclaration(
            node: ts.FunctionDeclaration, access: symbols.Accessibility): ast.ModuleMethod {
        let decl: FunctionLikeDeclaration = this.transformFunctionLikeCommon(node);
        return this.withLocation(node, <ast.ModuleMethod>{
            kind:       ast.moduleMethodKind,
            name:       decl.name,
            access:     access,
            parameters: decl.parameters,
            body:       decl.body,
            returnType: decl.returnType,
        });
    }

    // transformInterfaceDeclaration turns a TypeScript interface into a MuIL interface class.
    private transformInterfaceDeclaration(node: ts.InterfaceDeclaration, access: symbols.Accessibility): ast.Class {
        // TODO(joe): generics.
        // TODO(joe): decorators.
        // TODO(joe): extends/implements.

        // Transform all valid members for this declaration into ClassMembers.
        let members: ast.ClassMembers = {};
        for (let member of node.members) {
            if (member.kind !== ts.SyntaxKind.MissingDeclaration) {
                let decl: ast.ClassMember;
                let element: ClassElement = this.transformTypeElement(member);
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
            kind:      ast.classKind,
            name:      this.transformIdentifier(node.name),
            access:    access,
            members:   members,
            interface: true,
        });
    }

    private transformModuleDeclaration(node: ts.ModuleDeclaration, access: symbols.Accessibility): ast.Module {
        return notYetImplemented(node);
    }

    private transformParameterDeclaration(node: ts.ParameterDeclaration): VariableDeclaration<ast.LocalVariable> {
        // Validate that we're dealing with the supported subset.
        if (!!node.dotDotDotToken) {
            this.diagnostics.push(this.dctx.newRestParamsNotSupportedError(node.dotDotDotToken));
        }

        // TODO[marapongo/mu#43]: parameters can be any binding name, including destructuring patterns.  For now,
        //     however, we only support the identifier forms.
        let name: ast.Identifier = this.transformBindingIdentifier(node.name);
        let initializer: ast.Expression | undefined;
        if (node.initializer) {
            initializer = this.transformExpression(node.initializer);
        }
        return {
            node:     node,
            variable: {
                kind: ast.localVariableKind,
                name: name,
                type: this.transformTypeNode(node.type),
            },
            initializer: initializer,
        };
    }

    // transformTypeAliasDeclaration emits a type whose base is the aliased type.  The MuIL type system permits
    // conversions between such types in a way that is roughly compatible with TypeScript's notion of type aliases.
    private transformTypeAliasDeclaration(node: ts.TypeAliasDeclaration, access: symbols.Accessibility): ast.Class {
        return this.withLocation(node, <ast.Class>{
            kind:    ast.classKind,
            name:    this.transformIdentifier(node.name),
            access:  access,
            extends: this.transformTypeNode(node.type),
        });
    }

    private makeVariableInitializer(decl: VariableDeclaration<ast.Variable>): ast.Statement {
        contract.requires(!!decl.initializer, "decl", "Expected variable declaration to have an initializer");
        return this.withLocation(decl.node, {
            kind:     ast.binaryOperatorExpressionKind,
            left:     <ast.LoadLocationExpression>{
                kind: ast.loadLocationExpressionKind,
                name: decl.variable.name,
            },
            operator: "=",
            right:    decl.initializer,
        });
    }

    private transformVariableStatement(node: ts.VariableStatement): VariableLikeDeclaration[] {
        return node.declarationList.declarations.map(
            (decl: ts.VariableDeclaration) => {
                let like: VariableLikeDeclaration = this.transformVariableDeclaration(decl);
                // If the node is marked "const", tag all variables as readonly.
                if (!!(node.declarationList.flags & ts.NodeFlags.Const)) {
                    like.readonly = true;
                }
                // If the node isn't marked "let", we must mark all variables to use legacy "var" behavior.
                if (!(node.declarationList.flags & ts.NodeFlags.Let)) {
                    like.legacyVar = true;
                }
                return like;
            },
        );
    }

    private transformLocalVariableStatement(node: ts.VariableStatement): ast.Statement {
        // For variables, we need to append initializers as assignments if there are any.
        // TODO: emulate "var"-like scoping.
        let statements: ast.Statement[] = [];
        let decls: VariableLikeDeclaration[] = this.transformVariableStatement(node);
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
                    node, local, decl.legacyVar, decl.initializer);
                statements.push(this.makeVariableInitializer(vdecl));
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

    private transformModuleVariableStatement(
            node: ts.VariableStatement, access: symbols.Accessibility): VariableDeclaration<ast.ModuleProperty>[] {
        let decls: VariableLikeDeclaration[] = this.transformVariableStatement(node);
        return decls.map((decl: VariableLikeDeclaration) =>
            new VariableDeclaration<ast.ModuleProperty>(
                node,
                <ast.ModuleProperty>{
                    kind:     ast.modulePropertyKind,
                    name:     decl.name,
                    access:   access,
                    readonly: decl.readonly,
                    type:     decl.type,
                },
                decl.legacyVar,
                decl.initializer,
            ),
        );
    }

    private transformVariableDeclaration(node: ts.VariableDeclaration): VariableLikeDeclaration {
        // TODO[marapongo/mu#43]: parameters can be any binding name, including destructuring patterns.  For now,
        //     however, we only support the identifier forms.
        let name: ast.Identifier = this.transformDeclarationIdentifier(node.name);
        let initializer: ast.Expression | undefined;
        if (node.initializer) {
            initializer = this.transformExpression(node.initializer);
        }
        return {
            name:        name,
            type:        this.transformTypeNode(node.type),
            initializer: initializer,
        };
    }

    private transformVariableDeclarationList(node: ts.VariableDeclarationList): VariableLikeDeclaration[] {
        return node.declarations.map((decl: ts.VariableDeclaration) => this.transformVariableDeclaration(decl));
    }

    /** Class/type elements **/

    private transformClassElement(node: ts.ClassElement): ClassElement {
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
                return this.transformPropertyDeclarationOrSignature(<ts.PropertyDeclaration>node);

            // Unrecognized cases:
            case ts.SyntaxKind.SemicolonClassElement:
                return contract.fail("Expected all SemiColonClassElements to be filtered out of AST tree");
            default:
                return contract.fail(`Unrecognized ClassElement node kind: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    // transformTypeElement turns a TypeScript type element, typically an interface member, into a MuIL class member.
    private transformTypeElement(node: ts.TypeElement): ClassElement {
        switch (node.kind) {
            // Property and method signatures are like their class counterparts, but have no bodies:
            case ts.SyntaxKind.PropertySignature:
                return this.transformPropertyDeclarationOrSignature(<ts.PropertySignature>node);
            case ts.SyntaxKind.MethodSignature:
                return this.transformMethodSignature(<ts.MethodSignature>node);

            default:
                return contract.fail(`Unrecognized TypeElement node kind: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    private getClassAccessibility(node: ts.Node): symbols.ClassMemberAccessibility {
        let mods: ts.ModifierFlags = ts.getCombinedModifierFlags(node);
        if (!!(mods & ts.ModifierFlags.Private)) {
            return symbols.privateAccessibility;
        }
        else if (!!(mods & ts.ModifierFlags.Protected)) {
            return symbols.protectedAccessibility;
        }
        else {
            // All members are public by default in ECMA/TypeScript.
            return symbols.publicAccessibility;
        }
    }

    private transformFunctionLikeDeclaration(node: ts.FunctionLikeDeclaration): ast.ClassMethod {
        // Get/Set accessors aren't yet supported.
        contract.assert(node.kind !== ts.SyntaxKind.GetAccessor, "GetAccessor NYI");
        contract.assert(node.kind !== ts.SyntaxKind.SetAccessor, "SetAccessor NYI");

        let mods: ts.ModifierFlags = ts.getCombinedModifierFlags(node);
        let decl: FunctionLikeDeclaration = this.transformFunctionLikeCommon(node);
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

    private transformPropertyDeclarationOrSignature(
            node: ts.PropertyDeclaration | ts.PropertySignature): VariableDeclaration<ast.ClassProperty> {
        let initializer: ast.Expression | undefined;
        if (node.initializer) {
            initializer = this.transformExpression(node.initializer);
        }

        let mods: ts.ModifierFlags = ts.getCombinedModifierFlags(node);
        // TODO: primary properties.
        return new VariableDeclaration<ast.ClassProperty>(
            node,
            {
                kind:     ast.classPropertyKind,
                name:     this.transformPropertyName(node.name),
                access:   this.getClassAccessibility(node),
                readonly: !!(mods & ts.ModifierFlags.Readonly),
                static:   !!(mods & ts.ModifierFlags.Static),
                type:     this.transformTypeNode(node.type),
            },
            false,
            initializer,
        );
    }

    private transformMethodSignature(node: ts.MethodSignature): ast.ClassMethod {
        let decl: FunctionLikeDeclaration = this.transformFunctionLikeOrSignatureCommon(node);
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
    private transformDoStatement(node: ts.DoStatement): ast.WhileStatement {
        // Now create a new block that simply concatenates the existing one with a test/`break`.
        let body: ast.Block = this.withLocation(node.statement, {
            kind:       ast.blockKind,
            statements: [
                this.transformStatement(node.statement),
                <ast.IfStatement>{
                    kind:      ast.ifStatementKind,
                    condition: <ast.UnaryOperatorExpression>{
                        kind:     ast.unaryOperatorExpressionKind,
                        operator: "!",
                        operand:  this.transformExpression(node.expression),
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

    private transformIfStatement(node: ts.IfStatement): ast.IfStatement {
        return this.withLocation(node, <ast.IfStatement>{
            kind:       ast.ifStatementKind,
            condition:  this.transformExpression(node.expression),
            consequent: this.transformStatement(node.thenStatement),
            alternate:  object.maybeUndefined(
                node.elseStatement, (stmt: ts.Statement) => this.transformStatement(stmt)),
        });
    }

    private transformReturnStatement(node: ts.ReturnStatement): ast.ReturnStatement {
        return this.withLocation(node, <ast.ReturnStatement>{
            kind:       ast.returnStatementKind,
            expression: object.maybeUndefined(
                node.expression, (expr: ts.Expression) => this.transformExpression(expr)),
        });
    }

    private transformSwitchStatement(node: ts.SwitchStatement): ast.Statement {
        return notYetImplemented(node);
    }

    private transformThrowStatement(node: ts.ThrowStatement): ast.ThrowStatement {
        return this.withLocation(node, <ast.ThrowStatement>{
            kind:       ast.throwStatementKind,
            expression: this.transformExpression(node.expression),
        });
    }

    private transformTryStatement(node: ts.TryStatement): ast.TryCatchFinally {
        return notYetImplemented(node);
    }

    private transformWhileStatement(node: ts.WhileStatement): ast.WhileStatement {
        return this.withLocation(node, <ast.WhileStatement>{
            kind: ast.whileStatementKind,
            test: this.transformExpression(node.expression),
            body: this.transformStatementAsBlock(node.statement),
        });
    }

    /** Miscellaneous statements **/

    private transformBlock(node: ts.Block): ast.Block {
        // TODO(joe): map directives.
        return this.withLocation(node, <ast.Block>{
            kind:       ast.blockKind,
            statements: node.statements.map((stmt: ts.Statement) => this.transformStatement(stmt)),
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

    private transformExpressionStatement(node: ts.ExpressionStatement): ast.ExpressionStatement {
        return this.withLocation(node, <ast.ExpressionStatement>{
            kind:       ast.expressionStatementKind,
            expression: this.transformExpression(node.expression),
        });
    }

    private transformLabeledStatement(node: ts.LabeledStatement): ast.LabeledStatement {
        return this.withLocation(node, <ast.LabeledStatement>{
            kind:      ast.labeledStatementKind,
            label:     this.transformIdentifier(node.label),
            statement: this.transformStatement(node.statement),
        });
    }

    private transformModuleBlock(node: ts.ModuleBlock): ast.Block {
        return notYetImplemented(node);
    }

    private transformWithStatement(node: ts.WithStatement): ast.Statement {
        return notYetImplemented(node);
    }

    /** Expressions **/

    private transformExpression(node: ts.Expression): ast.Expression {
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
                return this.transformIdentifierExpression(<ts.Identifier>node);
            case ts.SyntaxKind.ObjectLiteralExpression:
                return this.transformObjectLiteralExpression(<ts.ObjectLiteralExpression>node);
            case ts.SyntaxKind.PostfixUnaryExpression:
                return this.transformPostfixUnaryExpression(<ts.PostfixUnaryExpression>node);
            case ts.SyntaxKind.PrefixUnaryExpression:
                return this.transformPrefixUnaryExpression(<ts.PrefixUnaryExpression>node);
            case ts.SyntaxKind.PropertyAccessExpression:
                return this.transformPropertyAccessExpression(<ts.PropertyAccessExpression>node);
            case ts.SyntaxKind.NewExpression:
                return this.transformNewExpression(<ts.NewExpression>node);
            case ts.SyntaxKind.OmittedExpression:
                return this.transformOmittedExpression(<ts.OmittedExpression>node);
            case ts.SyntaxKind.ParenthesizedExpression:
                return this.transformParenthesizedExpression(<ts.ParenthesizedExpression>node);
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

    private transformArrayLiteralExpression(node: ts.ArrayLiteralExpression): ast.ArrayLiteral {
        return this.withLocation(node, <ast.ArrayLiteral>{
            kind:     ast.arrayLiteralKind,
            type:     this.transformTypeNode(undefined),
            elements: node.elements.map((expr: ts.Expression) => this.transformExpression(expr)),
        });
    }

    private transformArrowFunction(node: ts.ArrowFunction): ast.Expression {
        return notYetImplemented(node);
    }

    private transformBinaryExpression(node: ts.BinaryExpression): ast.Expression {
        let op: ts.SyntaxKind = node.operatorToken.kind;
        if (op === ts.SyntaxKind.CommaToken) {
            // Translate this into a sequence expression.
            return this.transformBinarySequenceExpression(node);
        }
        else {
            // Translate this into an ordinary binary operator.
            return this.transformBinaryOperatorExpression(node);
        }
    }

    private transformBinaryOperatorExpression(node: ts.BinaryExpression): ast.BinaryOperatorExpression {
        // A few operators aren't faithfully emulated; in those cases, log warnings.
        if (log.v(3)) {
            switch (node.operatorToken.kind) {
                case ts.SyntaxKind.GreaterThanGreaterThanGreaterThanEqualsToken:
                case ts.SyntaxKind.GreaterThanGreaterThanGreaterThanToken:
                case ts.SyntaxKind.EqualsEqualsEqualsToken:
                case ts.SyntaxKind.ExclamationEqualsEqualsToken:
                    log.out(3).info(
                        `ECMAScript operator '${ts.SyntaxKind[node.operatorToken.kind]}' not supported; ` +
                        `until marapongo/mu#50 is implemented, be careful about subtle behavioral differences`
                    );
                    break;
            }
        }

        let operator: ast.BinaryOperator | undefined = binaryOperators.get(node.operatorToken.kind);
        contract.assert(!!operator, `Expected binary operator for: ${ts.SyntaxKind[node.operatorToken.kind]}`);
        return this.withLocation(node, <ast.BinaryOperatorExpression>{
            kind:     ast.binaryOperatorExpressionKind,
            operator: operator,
            left:     this.transformExpression(node.left),
            right:    this.transformExpression(node.right),
        });
    }

    private transformBinarySequenceExpression(node: ts.BinaryExpression): ast.SequenceExpression {
        contract.assert(node.operatorToken.kind === ts.SyntaxKind.CommaToken);

        // Pile up the expressions in the right order.
        let curr: ts.Expression = node;
        let binary: ts.BinaryExpression = node;
        let expressions: ast.Expression[] = [];
        while (curr.kind === ts.SyntaxKind.BinaryExpression &&
                (binary = <ts.BinaryExpression>curr).operatorToken.kind === ts.SyntaxKind.CommaToken) {
            expressions.unshift(this.transformExpression(binary.right));
            curr = binary.left;
        }
        expressions.unshift(this.transformExpression(curr));

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

    private transformCallExpression(node: ts.CallExpression): ast.InvokeFunctionExpression {
        return this.withLocation(node, <ast.InvokeFunctionExpression>{
            kind:      ast.invokeFunctionExpressionKind,
            function:  this.transformExpression(node.expression),
            arguments: node.arguments.map((expr: ts.Expression) => this.transformExpression(expr)),
        });
    }

    private transformClassExpression(node: ts.ClassExpression): ast.Expression {
        return notYetImplemented(node);
    }

    private transformConditionalExpression(node: ts.ConditionalExpression): ast.ConditionalExpression {
        return this.withLocation(node, <ast.ConditionalExpression>{
            kind:       ast.conditionalExpressionKind,
            condition:  this.transformExpression(node.condition),
            consequent: this.transformExpression(node.whenTrue),
            alternate:  this.transformExpression(node.whenFalse),
        });
    }

    private transformDeleteExpression(node: ts.DeleteExpression): ast.Expression {
        if (log.v(3)) {
            log.out(3).info(
                `ECMAScript operator 'delete' not supported; ` +
                `until marapongo/mu#50 is implemented, be careful about subtle behavioral differences`
            );
        }
        // TODO[marapongo/mu#50]: we need to decide how to map `delete` into a runtime MuIL operator.  It's possible
        //     this can leverage some dynamic trickery to delete an entry from a map.  But for strong typing reasons,
        //     this is dubious (at best); for now, we will simply `null` the target out, however, this will cause
        //     problems down the road once we properly support nullable types.
        return this.withLocation(node, <ast.BinaryOperatorExpression>{
            kind:     ast.binaryOperatorExpressionKind,
            left:     this.transformExpression(node.expression),
            operator: "=",
            right:    <ast.NullLiteral>{
                kind: ast.nullLiteralKind,
            },
        });
    }

    private transformElementAccessExpression(node: ts.ElementAccessExpression): ast.LoadExpression {
        let object: ast.Expression = this.transformExpression(node.expression);
        if (node.argumentExpression) {
            switch (node.argumentExpression.kind) {
                case ts.SyntaxKind.Identifier:
                    return this.withLocation(node, <ast.LoadLocationExpression>{
                        kind:   ast.loadLocationExpressionKind,
                        object: object,
                        name:   this.transformIdentifier(<ts.Identifier>node.argumentExpression),
                    });
                default:
                    return this.withLocation(node, <ast.LoadDynamicExpression>{
                        kind:   ast.loadDynamicExpressionKind,
                        object: object,
                        name:   this.transformExpression(<ts.Expression>node.argumentExpression),
                    });
            }
        }
        else {
            return object;
        }
    }

    private transformFunctionExpression(node: ts.FunctionExpression): ast.Expression {
        return notYetImplemented(node);
    }

    private transformObjectLiteralExpression(node: ts.ObjectLiteralExpression): ast.ObjectLiteral {
        // TODO[marapongo/mu#46]: because TypeScript object literals are untyped, it's not clear what MuIL type this
        //     expression should produce.  It's common for a TypeScript literal to be enclosed in a cast, for example,
        //     `<SomeType>{ literal }`, in which case, perhaps we could detect `<SomeType>`.  Alternatively, MuIL could
        //     just automatically dynamically coerce `any` to the target type, similar to TypeScript, when necessary.
        //     I had envisioned requiring explicit dynamic casts for this, in which case, perhaps this expression should
        //     always be encased in something that prepares it for dynamic cast in the consuming expression.
        return this.withLocation(node, <ast.ObjectLiteral>{
            kind:       ast.objectLiteralKind,
            type:       this.transformTypeNode(undefined),
            properties: node.properties.map(
                (prop: ts.ObjectLiteralElement) => this.transformObjectLiteralElement(prop)),
        });
    }

    private transformObjectLiteralElement(node: ts.ObjectLiteralElement): ast.ObjectLiteralProperty {
        switch (node.kind) {
            case ts.SyntaxKind.PropertyAssignment:
                return this.transformObjectLiteralPropertyAssignment(<ts.PropertyAssignment>node);
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

    private transformObjectLiteralPropertyAssignment(node: ts.PropertyAssignment): ast.ObjectLiteralProperty {
        return this.withLocation(node, <ast.ObjectLiteralProperty>{
            kind:  ast.objectLiteralPropertyKind,
            name:  this.transformPropertyName(node.name),
            value: this.transformExpression(node.initializer),
        });
    }

    private transformObjectLiteralShorthandPropertyAssignment(
            node: ts.ShorthandPropertyAssignment): ast.ObjectLiteralProperty {
        let name: ast.Identifier = this.transformIdentifier(node.name);
        return this.withLocation(node, <ast.ObjectLiteralProperty>{
            kind:  ast.objectLiteralPropertyKind,
            name:  name,
            value: this.withLocation(node.name, <ast.LoadLocationExpression>{
                kind: ast.loadLocationExpressionKind,
                name: name,
            }),
        });
    }

    private transformObjectLiteralFunctionLikeElement(node: ts.FunctionLikeDeclaration): ast.ObjectLiteralProperty {
        // TODO: turn these into lambdas.
        return notYetImplemented(node);
    }

    private transformPostfixUnaryExpression(node: ts.PostfixUnaryExpression): ast.UnaryOperatorExpression {
        let operator: ast.UnaryOperator | undefined = postfixUnaryOperators.get(node.operator);
        contract.assert(!!(operator = operator!));
        return this.withLocation(node, <ast.UnaryOperatorExpression>{
            kind:     ast.unaryOperatorExpressionKind,
            postfix:  true,
            operator: operator,
            operand:  this.transformExpression(node.operand),
        });
    }

    private transformPrefixUnaryExpression(node: ts.PrefixUnaryExpression): ast.UnaryOperatorExpression {
        let operator: ast.UnaryOperator | undefined = prefixUnaryOperators.get(node.operator);
        contract.assert(!!(operator = operator!));
        return this.withLocation(node, <ast.UnaryOperatorExpression>{
            kind:     ast.unaryOperatorExpressionKind,
            postfix:  false,
            operator: operator,
            operand:  this.transformExpression(node.operand),
        });
    }

    private transformPropertyAccessExpression(node: ts.PropertyAccessExpression): ast.LoadLocationExpression {
        return this.withLocation(node, <ast.LoadLocationExpression>{
            kind:   ast.loadLocationExpressionKind,
            object: this.transformExpression(node.expression),
            name:   this.transformIdentifier(node.name),
        });
    }

    private transformNewExpression(node: ts.NewExpression): ast.NewExpression {
        // Only "new T(..)" constructors, where T is an identifier referring to a type, are permitted.
        let ty: ast.Identifier;
        if (node.expression.kind === ts.SyntaxKind.Identifier) {
            ty = this.transformIdentifier(<ts.Identifier>node.expression);
        }
        else {
            return contract.fail("New T(..) expression must have an identifier T");
        }

        return this.withLocation(node, <ast.NewExpression>{
            kind:      ast.newExpressionKind,
            type:      ty,
            arguments: node.arguments.map((expr: ts.Expression) => this.transformExpression(expr)),
        });
    }

    private transformOmittedExpression(node: ts.OmittedExpression): ast.Expression {
        return notYetImplemented(node);
    }

    private transformParenthesizedExpression(node: ts.ParenthesizedExpression): ast.Expression {
        return notYetImplemented(node);
    }

    private transformSpreadElement(node: ts.SpreadElement): ast.Expression {
        return notYetImplemented(node);
    }

    private transformSuperExpression(node: ts.SuperExpression): ast.LoadLocationExpression {
        return this.withLocation(node, <ast.LoadLocationExpression>{
            kind: ast.loadLocationExpressionKind,
            name: ident(symbols.specialVariableSuper),
        });
    }

    private transformTaggedTemplateExpression(node: ts.TaggedTemplateExpression): ast.Expression {
        return notYetImplemented(node);
    }

    private transformTemplateExpression(node: ts.TemplateExpression): ast.Expression {
        return notYetImplemented(node);
    }

    private transformThisExpression(node: ts.ThisExpression): ast.LoadLocationExpression {
        return this.withLocation(node, <ast.LoadLocationExpression>{
            kind: ast.loadLocationExpressionKind,
            name: ident(symbols.specialVariableThis),
        });
    }

    private transformTypeOfExpression(node: ts.TypeOfExpression): ast.Expression {
        return notYetImplemented(node);
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

    private transformIdentifierExpression(node: ts.Identifier): ast.Identifier {
        return this.withLocation(node, ident(node.text));
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
            default:
                return contract.fail("Property names other than identifiers not yet supported");
        }
    }
}

// Loads the metadata and transforms a TypeScript program into its equivalent MuPack/MuIL AST form.
export async function transform(script: Script): Promise<TransformResult> {
    let disc: DiscoverResult = await discover(script.root);
    let result: TransformResult = {
        diagnostics: disc.diagnostics, // ensure we propagate the diagnostics
        pack:        undefined,
    };

    if (disc.meta) {
        // New up a transformer and do it.
        let t = new Transformer(disc.meta, script);
        let trans: TransformResult = t.transform();

        // Copy the return to our running result, so we propagate the aggregate of all diagnostics.
        result.diagnostics = result.diagnostics.concat(trans.diagnostics);
        result.pack = trans.pack;
    }

    return result;
}

export interface TransformResult {
    diagnostics: diag.Diagnostic[];        // any diagnostics resulting from translation.
    pack:        pack.Package | undefined; // the resulting MuPack/MuIL AST.
}

