// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import {contract, object} from "nodets";
import * as fspath from "path";
import * as ts from "typescript";
import * as ast from "../ast";
import * as pack from "../pack";
import * as symbols from "../symbols";
import {Compilation} from "./compile";
import {discover} from "./discover";

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
    // TODO: [ ts.SyntaxKind.GreaterThanGreaterThanGreaterThanEqualsToken, ">>>=" ],
    [ ts.SyntaxKind.AmpersandEqualsToken,                         "&=" ],
    [ ts.SyntaxKind.BarEqualsToken,                               "|=" ],
    [ ts.SyntaxKind.CaretEqualsToken,                             "^=" ],

    // Bitwise
    [ ts.SyntaxKind.LessThanLessThanToken,                  "<<" ],
    [ ts.SyntaxKind.GreaterThanGreaterThanToken,            ">>" ],
    [ ts.SyntaxKind.BarToken,                               "|" ],
    [ ts.SyntaxKind.CaretToken,                             "^" ],
    [ ts.SyntaxKind.AmpersandToken,                         "&" ],
    // TODO: [ ts.SyntaxKind.GreaterThanGreaterThanGreaterThanToken, ">>>" ],

    // Logical
    [ ts.SyntaxKind.AmpersandAmpersandToken, "&&" ],
    [ ts.SyntaxKind.BarBarToken,             "||" ],

    // Relational
    [ ts.SyntaxKind.LessThanToken,                          "<" ],
    [ ts.SyntaxKind.LessThanEqualsToken,                    "<=" ],
    [ ts.SyntaxKind.GreaterThanToken,                       ">" ],
    [ ts.SyntaxKind.GreaterThanEqualsToken,                 ">=" ],
    [ ts.SyntaxKind.EqualsEqualsToken,                      "==" ],
    [ ts.SyntaxKind.ExclamationEqualsToken,                 "!=" ],
    // TODO: [ ts.SyntaxKind.EqualsEqualsEqualsToken,                "===" ],
    // TODO: [ ts.SyntaxKind.ExclamationEqualsEqualsToken,           "!==" ],

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

// A top-level module element is either a definition or a statement.
type ModuleElement = ast.Definition | ast.Statement;

// A variable is a MuIL variable with an optional initializer expression.  This is required because MuIL doesn't support
// complex initializers on the Variable AST node -- they must be explicitly placed into an initializer section.
interface VariableDeclaration {
    node:         ts.Node;           // the source node.
    local:        ast.LocalVariable; // the MuIL variable information.
    legacyVar?:   boolean;           // true if we should mimick legacy ECMAScript "var" behavior; false for "let".
    initializer?: ast.Expression;    // an optional initialization expression.
}

// A function declaration isn't yet known to be a module or class method, and so it just contains the subset that is
// common between them.  This facilitates code reuse in the translation passes.
interface FunctionDeclaration {
    node:        ts.Node;
    name:        ast.Identifier;
    parameters:  ast.LocalVariable[];
    body?:       ast.Block;
    returnType?: symbols.TypeToken;
}

// A transpiler is responsible for transforming TypeScript program artifacts into MuPack/MuIL AST forms.
export class Transpiler {
    private meta: pack.Metadata; // the package's metadata.
    private comp: Compilation;   // the package's compiled TypeScript tree and context.

    // Loads up Mu metadata and then creates a new Transpiler object.
    public static async createFrom(comp: Compilation): Promise<Transpiler> {
        return new Transpiler(await discover(comp.root), comp);
    }

    constructor(meta: pack.Metadata, comp: Compilation) {
        contract.requires(!!comp.tree, "comp", "A valid MuJS AST is required to lower to MuPack/MuIL");
        this.meta = meta;
        this.comp = comp;
    }

    // Translates a TypeScript bound tree into its equivalent MuPack/MuIL AST form, one module per file.
    public transform(): pack.Package {
        // Enumerate all source files (each of which is a module in ECMAScript), and transform it.
        let modules: ast.Modules = {};
        for (let sourceFile of this.comp.tree!.getSourceFiles()) {
            // By default, skip declaration files, since they are "dependencies."
            // TODO(joe): how to handle re-exports in ECMAScript, such as index aggregation.
            // TODO(joe): this isn't a perfect heuristic.  But ECMAScript is all source dependencies, so there isn't a
            //     true notion of source versus binary dependency.  We could crack open the dependencies to see if they
            //     exist within an otherwise known package, but that seems a little hokey.
            if (!sourceFile.isDeclarationFile) {
                let mod: ast.Module = this.transformSourceFile(sourceFile, this.comp.root);
                modules[mod.name.ident] = mod;
            }
        }

        // Now create a new package object.
        // TODO: create a list of dependencies, partly from the metadata, partly from the TypeScript compilation.
        return object.extend(this.meta, {
            modules: modules,
        });
    }

    /** Helpers **/

    // This annotates a given MuPack/MuIL node with another TypeScript node's source position information.
    private copyLocation<T extends ast.Node>(src: ts.Node, dst: T): T {
        let pos = (s: ts.SourceFile, p: number) => {
            // Translate a TypeScript position into a MuIL position (0 to 1 based lines).
            let lc = s.getLineAndCharacterOfPosition(p);
            return {
                line:   lc.line + 1,  // transform to 1-based line number
                column: lc.character,
            };
        };

        // Turn the source file name into one relative to the current root path.
        let s: ts.SourceFile = src.getSourceFile();
        let relativePath: string = fspath.relative(this.comp.root, s.fileName);

        dst.loc = {
            file:  relativePath,
            start: pos(s, src.getStart()),
            end:   pos(s, src.getEnd()),
        };

        // Despite mutating in place, we return the node to facilitate a more fluent style.
        return dst;
    }

    /** AST queries **/

    private isComputed(name: ts.Node | undefined): boolean {
        if (name) {
            return (name.kind === ts.SyntaxKind.ComputedPropertyName);
        }
        return false;
    }

    /** Transformations **/

    /** Symbols **/

    private transformIdentifier(node: ts.Identifier): ast.Identifier {
        return this.copyLocation(node, {
            kind:  ast.identifierKind,
            ident: node.text,
        });
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
                if (ast.isDefinition(element)) {
                    let defn: ast.Definition = <ast.Definition>element;
                    members[defn.name.ident] = defn;
                }
                else {
                    statements.push(<ast.Statement>element);
                }
            }
         }

        // If the initialization statements are non-empty, add an initializer method.
        if (statements.length > 0) {
            let initializer: ast.ModuleMethod = {
                kind:   ast.moduleMethodKind,
                name:   {
                    kind:  ast.identifierKind,
                    ident: symbols.specialFunctionInitializer,
                },
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

        return this.copyLocation(node, {
            kind:    ast.moduleKind,
            name:    <ast.Identifier>{
                kind:  ast.identifierKind,
                ident: moduleName,
            },
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
                    // TODO: register the import name so we can "mangle" any references to it accordingly later on.
                    return contract.fail("NYI");

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
            contract.assert(elements[0].kind === ast.moduleMethodKind || elements[0].kind === ast.classKind);
            (<ast.Definition>elements[0]).name = {
                kind:  ast.identifierKind,
                ident: defaultExport,
            };
        }

        return elements;
    }

    private transformExportAssignment(node: ts.ExportAssignment): ast.Definition {
        return contract.fail("NYI");
    }

    private transformExportDeclaration(node: ts.ExportDeclaration): ast.Definition {
        return contract.fail("NYI");
    }

    private transformExportSpecifier(node: ts.ExportSpecifier): ast.Definition {
        return contract.fail("NYI");
    }

    private transformImportDeclaration(node: ts.ImportDeclaration): ast.Definition {
        return contract.fail("NYI");
    }

    private transformImportClause(node: ts.ImportClause | undefined): ast.Definition[] {
        return contract.fail("NYI");
    }

    private transformImportSpecifier(node: ts.ImportSpecifier): ast.Definition {
        return contract.fail("NYI");
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
                // TODO: issue a proper error; these sorts of declarations aren't valid statements in MuIL.
                return contract.fail(`Declaration node ${ts.SyntaxKind[node.kind]} isn't a valid statement in MuIL`);
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
                return [ this.transformFunctionDeclaration<ast.ModuleMethod>(
                    <ts.FunctionDeclaration>node, ast.moduleMethodKind, access) ];
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
        return this.copyLocation(node, {
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
            name = {
                kind:  ast.identifierKind,
                ident: defaultExport,
            };
        }

        // Transform all non-semicolon members for this declaration.
        let members: ast.ClassMembers = {};
        for (let member of node.members) {
            if (member.kind !== ts.SyntaxKind.SemicolonClassElement) {
                let result: ast.ClassMember = this.transformClassElement(member);
                members[result.name.ident] = result;
            }
        }

        let mods: ts.ModifierFlags = ts.getCombinedModifierFlags(node);
        return this.copyLocation(node, {
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

    private transformFunctionLikeDeclaration(node: ts.FunctionLikeDeclaration): FunctionDeclaration {
        // Ensure we are dealing with the supported subset of functions.
        // TODO: turn these into real errors.
        if (ts.getCombinedModifierFlags(node) & ts.ModifierFlags.Async) {
            throw new Error("Async functions are not supported in MuJS");
        }
        if (!!node.asteriskToken) {
            throw new Error("Generator functions are not supported in MuJS");
        }

        // First transform the name into an identifier.  In the absence of a name, we will proceed under the assumption
        // that it is the default export.  This should be verified later on.
        let name: ast.Identifier;
        if (node.name) {
            name = this.transformPropertyName(node.name);
        }
        else {
            // Create a default identifier name.
            let ident: string;
            if (node.kind === ts.SyntaxKind.Constructor) {
                // Constructors have a special name.
                ident = symbols.specialFunctionConstructor;
            }
            else {
                // All others are assumed to be default exports.
                ident = defaultExport;
            }
            name = {
                kind:  ast.identifierKind,
                ident: ident,
            };
        }

        // Now visit the body; it can either be a block or a free-standing expression.
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

        // Next transform the parameter variables into locals.
        let parameters: VariableDeclaration[] = node.parameters.map(
            (param: ts.ParameterDeclaration) => this.transformParameterDeclaration(param));

        // If there are any initializers, make sure to prepend them (in order) to the body block.
        for (let parameter of parameters) {
            if (parameter.initializer && body) {
                body.statements = [ this.makeVariableInitializer(parameter) ].concat(body.statements);
            }
        }

        return {
            node:       node,
            name:       name,
            parameters: parameters.map((p: VariableDeclaration) => p.local),
            body:       body,
            returnType: "TODO",
        };
    }

    private transformFunctionDeclaration<TFunction extends ast.Function>(
            node: ts.FunctionDeclaration, kind: ast.NodeKind, access: symbols.Accessibility): TFunction {
        let decl: FunctionDeclaration = this.transformFunctionLikeDeclaration(node);
        return this.copyLocation(node, <TFunction><any>{
            kind:       kind,
            name:       decl.name,
            access:     access,
            parameters: decl.parameters,
            body:       decl.body,
            returnType: decl.returnType,
        });
    }

    private transformInterfaceDeclaration(node: ts.InterfaceDeclaration, access: symbols.Accessibility): ast.Class {
        return contract.fail("NYI");
    }

    private transformModuleDeclaration(node: ts.ModuleDeclaration, access: symbols.Accessibility): ast.Module {
        return contract.fail("NYI");
    }

    private transformParameterDeclaration(node: ts.ParameterDeclaration): VariableDeclaration {
        // Validate that we're dealing with the supported subset.
        // TODO(joe): turn these into real error messages.
        if (!!node.dotDotDotToken) {
            throw new Error("Rest-style arguments not supported by MuJS");
        }

        // TODO[marapongo/mu#43]: parameters can be any binding name, including destructuring patterns.  For now,
        //     however, we only support the identifier forms.
        let name: ast.Identifier = this.transformBindingIdentifier(node.name);
        let initializer: ast.Expression | undefined;
        if (node.initializer) {
            initializer = this.transformExpression(node.initializer);
        }
        return {
            node:  node,
            local: {
                kind: ast.localVariableKind,
                name: name,
                type: "TODO",
            },
            initializer: initializer,
        };
    }

    private transformTypeAliasDeclaration(node: ts.TypeAliasDeclaration, access: symbols.Accessibility): ast.Class {
        return contract.fail("NYI");
    }

    private makeVariableInitializer(variable: VariableDeclaration): ast.Statement {
        contract.requires(!!variable.initializer, "variable", "Expected variable to have an initializer");
        return this.copyLocation(variable.node, {
            kind:     ast.binaryOperatorExpressionKind,
            left:     <ast.LoadVariableExpression>{
                kind:     ast.loadVariableExpressionKind,
                variable: variable.local.name.ident,
            },
            operator: "=",
            right:    variable.initializer,
        });
    }

    private transformVariableStatement(node: ts.VariableStatement): VariableDeclaration[] {
        let variables: VariableDeclaration[] = node.declarationList.declarations.map(
            (decl: ts.VariableDeclaration) => this.transformVariableDeclaration(decl));

        // If the node is marked "const", tag all variables as readonly.
        if (!!(node.declarationList.flags & ts.NodeFlags.Const)) {
            for (let variable of variables) {
                variable.local.readonly = true;
            }
        }

        // If the node isn't marked "let", we must mark all variables to use legacy "var" behavior.
        if (!(node.declarationList.flags & ts.NodeFlags.Let)) {
            for (let variable of variables) {
                variable.legacyVar = true;
            }
        }

        return variables;
    }

    private transformLocalVariableStatement(node: ts.VariableStatement): ast.Statement {
        // For variables, we need to append initializers as assignments if there are any.
        // TODO: emulate "var"-like scoping.
        let statements: ast.Statement[] = [];
        let variables: VariableDeclaration[] = this.transformVariableStatement(node);
        for (let variable of variables) {
            statements.push(<ast.LocalVariableDeclaration>{
                kind:  ast.localVariableDeclarationKind,
                local: variable.local,
            });
            if (variable.initializer) {
                statements.push(this.makeVariableInitializer(variable));
            }
        }
        if (statements.length === 1) {
            return statements[0];
        }
        else {
            return <ast.MultiStatement>{
                kind:       ast.multiStatementKind,
                statements: statements,
            };
        }
    }

    private transformModuleVariableStatement(
            node: ts.VariableStatement, access: symbols.Accessibility): ModuleElement[] {
        let elements: ModuleElement[] = [];
        let variables: VariableDeclaration[] = this.transformVariableStatement(node);
        for (let variable of variables) {
            // First transform the local varaible into a module property.
            // TODO(joe): emulate "var"-like scoping.
            elements.push(object.extend(variable.local, {
                kind:   ast.modulePropertyKind,
                access: access,
            }));

            // Next, if there is an initializer, use it to initialize the variable in the module initializer.
            if (variable.initializer) {
                elements.push(this.makeVariableInitializer(variable));
            }
        }
        return elements;
    }

    private transformVariableDeclaration(node: ts.VariableDeclaration): VariableDeclaration {
        // TODO[marapongo/mu#43]: parameters can be any binding name, including destructuring patterns.  For now,
        //     however, we only support the identifier forms.
        let name: ast.Identifier = this.transformDeclarationIdentifier(node.name);
        let initializer: ast.Expression | undefined;
        if (node.initializer) {
            initializer = this.transformExpression(node.initializer);
        }
        return {
            node:  node,
            local: {
                kind: ast.localVariableKind,
                name: name,
                type: "TODO",
            },
            initializer: initializer,
        };
    }

    private transformVariableDeclarationList(node: ts.VariableDeclarationList): VariableDeclaration[] {
        return node.declarations.map((decl: ts.VariableDeclaration) => this.transformVariableDeclaration(decl));
    }

    /** Classes **/

    private transformClassElement(node: ts.ClassElement): ast.ClassMember {
        switch (node.kind) {
            // All the function-like members go here:
            case ts.SyntaxKind.Constructor:
                return this.transformClassElementFunctionLike(<ts.ConstructorDeclaration>node);
            case ts.SyntaxKind.MethodDeclaration:
                return this.transformClassElementFunctionLike(<ts.MethodDeclaration>node);
            case ts.SyntaxKind.GetAccessor:
                return this.transformClassElementFunctionLike(<ts.GetAccessorDeclaration>node);
            case ts.SyntaxKind.SetAccessor:
                return this.transformClassElementFunctionLike(<ts.SetAccessorDeclaration>node);

            // Properties are not function-like, so we translate them differently.
            case ts.SyntaxKind.PropertyDeclaration:
                return this.transformClassElementProperty(<ts.PropertyDeclaration>node);

            // Unrecognized cases:
            case ts.SyntaxKind.SemicolonClassElement:
                return contract.fail("Expected all SemiColonClassElements to be filtered out of AST tree");
            default:
                return contract.fail(`Unrecognized TypeElement node kind: ${ts.SyntaxKind[node.kind]}`);
        }
    }

    private transformClassElementFunctionLike(node: ts.FunctionLikeDeclaration): ast.Definition {
        // Get/Set accessors aren't yet supported.
        contract.assert(node.kind !== ts.SyntaxKind.GetAccessor, "GetAccessor NYI");
        contract.assert(node.kind !== ts.SyntaxKind.SetAccessor, "SetAccessor NYI");

        let mods: ts.ModifierFlags = ts.getCombinedModifierFlags(node);
        let decl: FunctionDeclaration = this.transformFunctionLikeDeclaration(node);
        let access: symbols.ClassMemberAccessibility;
        if (!!(mods & ts.ModifierFlags.Private)) {
            access = symbols.privateAccessibility;
        }
        else if (!!(mods & ts.ModifierFlags.Protected)) {
            access = symbols.protectedAccessibility;
        }
        else {
            // All members are public by default in ECMA/TypeScript.
            access = symbols.publicAccessibility;
        }

        return this.copyLocation(node, {
            kind:       ast.classMethodKind,
            name:       decl.name,
            access:     access,
            parameters: decl.parameters,
            body:       decl.body,
            returnType: decl.returnType,
            static:     !!(mods & ts.ModifierFlags.Static),
            abstract:   !!(mods & ts.ModifierFlags.Abstract),
        });
    }

    private transformClassElementProperty(node: ts.PropertyDeclaration): ast.ClassProperty {
        return contract.fail("NYI");
    }

    /** Control flow statements **/

    private transformBreakStatement(node: ts.BreakStatement): ast.BreakStatement {
        return this.copyLocation(node, {
            kind:  ast.breakStatementKind,
            label: object.maybeUndefined(node.label, this.transformIdentifier),
        });
    }

    private transformCaseOrDefaultClause(node: ts.CaseOrDefaultClause): ast.Statement {
        return contract.fail("NYI");
    }

    private transformCatchClause(node: ts.CatchClause): ast.Statement {
        return contract.fail("NYI");
    }

    private transformContinueStatement(node: ts.ContinueStatement): ast.ContinueStatement {
        return this.copyLocation(node, {
            kind:  ast.continueStatementKind,
            label: object.maybeUndefined(node.label, this.transformIdentifier),
        });
    }

    // This transforms a higher-level TypeScript `do`/`while` statement by expanding into an ordinary `while` statement.
    private transformDoStatement(node: ts.DoStatement): ast.WhileStatement {
        // Now create a new block that simply concatenates the existing one with a test/`break`.
        let body: ast.Block = this.copyLocation(node.statement, {
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

        return this.copyLocation(node, {
            kind: ast.whileStatementKind,
            test: <ast.BoolLiteral>{
                kind:  ast.boolLiteralKind,
                value: true,
            },
            body: body,
        });
    }

    private transformForStatement(node: ts.ForStatement): ast.Statement {
        return contract.fail("NYI");
    }

    private transformForInitializer(node: ts.ForInitializer): ast.Statement {
        return contract.fail("NYI");
    }

    private transformForInStatement(node: ts.ForInStatement): ast.Statement {
        return contract.fail("NYI");
    }

    private transformForOfStatement(node: ts.ForOfStatement): ast.Statement {
        return contract.fail("NYI");
    }

    private transformIfStatement(node: ts.IfStatement): ast.IfStatement {
        return this.copyLocation(node, {
            kind:       ast.ifStatementKind,
            condition:  this.transformExpression(node.expression),
            consequent: this.transformStatement(node.thenStatement),
            alternate:  object.maybeUndefined(node.elseStatement, this.transformStatement),
        });
    }

    private transformReturnStatement(node: ts.ReturnStatement): ast.ReturnStatement {
        return this.copyLocation(node, {
            kind:       ast.returnStatementKind,
            expression: object.maybeUndefined(node.expression, this.transformExpression),
        });
    }

    private transformSwitchStatement(node: ts.SwitchStatement): ast.Statement {
        return contract.fail("NYI");
    }

    private transformThrowStatement(node: ts.ThrowStatement): ast.ThrowStatement {
        return this.copyLocation(node, {
            kind:       ast.throwStatementKind,
            expression: this.transformExpression(node.expression),
        });
    }

    private transformTryStatement(node: ts.TryStatement): ast.TryCatchFinally {
        return contract.fail("NYI");
    }

    private transformWhileStatement(node: ts.WhileStatement): ast.Statement {
        return this.copyLocation(node, {
            kind: ast.whileStatementKind,
            test: this.transformExpression(node.expression),
            body: this.transformStatementAsBlock(node.statement),
        });
    }

    /** Miscellaneous statements **/

    private transformBlock(node: ts.Block): ast.Block {
        // TODO(joe): map directives.
        return this.copyLocation(node, {
            kind:       ast.blockKind,
            statements: node.statements.map((stmt: ts.Statement) => this.transformStatement(stmt)),
        });
    }

    private transformDebuggerStatement(node: ts.DebuggerStatement): ast.Statement {
        // The debugger statement in ECMAScript can be used to trip a breakpoint.  We don't have the equivalent in Mu at
        // the moment, so we simply produce an empty statement in its place.
        return this.copyLocation(node, {
            kind: ast.emptyStatementKind,
        });
    }

    private transformEmptyStatement(node: ts.EmptyStatement): ast.EmptyStatement {
        return this.copyLocation(node, {
            kind: ast.emptyStatementKind,
        });
    }

    private transformExpressionStatement(node: ts.ExpressionStatement): ast.ExpressionStatement {
        return this.copyLocation(node, {
            kind:       ast.expressionStatementKind,
            expression: this.transformExpression(node.expression),
        });
    }

    private transformLabeledStatement(node: ts.LabeledStatement): ast.LabeledStatement {
        return this.copyLocation(node, {
            kind:      ast.labeledStatementKind,
            label:     this.transformIdentifier(node.label),
            statement: this.transformStatement(node.statement),
        });
    }

    private transformModuleBlock(node: ts.ModuleBlock): ast.Block {
        return contract.fail("NYI");
    }

    private transformWithStatement(node: ts.WithStatement): ast.Statement {
        return contract.fail("NYI");
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

    private transformArrayLiteralExpression(node: ts.ArrayLiteralExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformArrowFunction(node: ts.ArrowFunction): ast.Expression {
        return contract.fail("NYI");
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
        let operator: ast.BinaryOperator | undefined = binaryOperators.get(node.operatorToken.kind);
        if (!operator) {
            // TODO: finish binary operator mapping; for any left that are unsupported, introduce a real error message.
            return contract.fail(`Unsupported binary operator: ${ts.SyntaxKind[node.operatorToken.kind]}`);
        }
        return this.copyLocation(node, {
            kind:     ast.binaryOperatorExpressionKind,
            operator: operator,
            left:     this.transformExpression(node.left),
            right:    this.transformExpression(node.right),
        });
    }

    private transformBinarySequenceExpression(node: ts.BinaryExpression): ast.SequenceExpression {
        contract.assert(node.operatorToken.kind === ts.SyntaxKind.CommaToken);
        let curr: ts.Expression = node;
        let binary: ts.BinaryExpression = node;
        let expressions: ast.Expression[] = [];
        while (curr.kind === ts.SyntaxKind.BinaryExpression &&
                (binary = <ts.BinaryExpression>curr).operatorToken.kind === ts.SyntaxKind.CommaToken) {
            expressions.unshift(this.transformExpression(binary.right));
            curr = binary.left;
        }
        expressions.unshift(this.transformExpression(curr));
        return {
            kind:        ast.sequenceExpressionKind,
            expressions: expressions,
        };
    }

    private transformCallExpression(node: ts.CallExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformClassExpression(node: ts.ClassExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformConditionalExpression(node: ts.ConditionalExpression): ast.ConditionalExpression {
        return this.copyLocation(node, {
            kind:       ast.conditionalExpressionKind,
            condition:  this.transformExpression(node.condition),
            consequent: this.transformExpression(node.whenTrue),
            alternate:  this.transformExpression(node.whenFalse),
        });
    }

    private transformDeleteExpression(node: ts.DeleteExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformElementAccessExpression(node: ts.ElementAccessExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformFunctionExpression(node: ts.FunctionExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformObjectLiteralExpression(node: ts.ObjectLiteralExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformObjectLiteralElement(node: ts.ObjectLiteralElement): ast.Expression {
        return contract.fail("NYI");
    }

    private transformObjectLiteralPropertyElement(
            node: ts.PropertyAssignment | ts.ShorthandPropertyAssignment): ast.Expression {
        return contract.fail("NYI");
    }

    private transformObjectLiteralFunctionLikeElement(
            node: ts.AccessorDeclaration | ts.MethodDeclaration): ast.Expression {
        return contract.fail("NYI");
    }

    private transformPostfixUnaryExpression(node: ts.PostfixUnaryExpression): ast.UnaryOperatorExpression {
        let operator: ast.UnaryOperator | undefined = postfixUnaryOperators.get(node.operator);
        contract.assert(!!(operator = operator!));
        return this.copyLocation(node, {
            kind:     ast.unaryOperatorExpressionKind,
            postfix:  true,
            operator: operator,
            operand:  this.transformExpression(node.operand),
        });
    }

    private transformPrefixUnaryExpression(node: ts.PrefixUnaryExpression): ast.UnaryOperatorExpression {
        let operator: ast.UnaryOperator | undefined = prefixUnaryOperators.get(node.operator);
        contract.assert(!!(operator = operator!));
        return this.copyLocation(node, {
            kind:     ast.unaryOperatorExpressionKind,
            postfix:  false,
            operator: operator,
            operand:  this.transformExpression(node.operand),
        });
    }

    private transformPropertyAccessExpression(node: ts.PropertyAccessExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformNewExpression(node: ts.NewExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformOmittedExpression(node: ts.OmittedExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformParenthesizedExpression(node: ts.ParenthesizedExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformSpreadElement(node: ts.SpreadElement): ast.Expression {
        return contract.fail("NYI");
    }

    private transformSuperExpression(node: ts.SuperExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformTaggedTemplateExpression(node: ts.TaggedTemplateExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformTemplateExpression(node: ts.TemplateExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformThisExpression(node: ts.ThisExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformTypeOfExpression(node: ts.TypeOfExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformVoidExpression(node: ts.VoidExpression): ast.Expression {
        return contract.fail("NYI");
    }

    private transformYieldExpression(node: ts.YieldExpression): ast.Expression {
        return contract.fail("NYI");
    }

    /** Literals **/

    private transformBooleanLiteral(node: ts.BooleanLiteral): ast.BoolLiteral {
        contract.assert(node.kind === ts.SyntaxKind.FalseKeyword || node.kind === ts.SyntaxKind.TrueKeyword);
        return this.copyLocation(node, {
            kind:  ast.boolLiteralKind,
            raw:   node.getText(),
            value: (node.kind === ts.SyntaxKind.TrueKeyword),
        });
    }

    private transformNoSubstitutionTemplateLiteral(node: ts.NoSubstitutionTemplateLiteral): ast.Expression {
        return contract.fail("NYI");
    }

    private transformNullLiteral(node: ts.NullLiteral): ast.NullLiteral {
        return this.copyLocation(node, {
            kind: ast.nullLiteralKind,
            raw:  node.getText(),
        });
    }

    private transformNumericLiteral(node: ts.NumericLiteral): ast.NumberLiteral {
        return this.copyLocation(node, {
            kind:  ast.numberLiteralKind,
            raw:   node.text,
            value: Number(node.text),
        });
    }

    private transformRegularExpressionLiteral(node: ts.RegularExpressionLiteral): ast.Expression {
        return contract.fail("NYI");
    }

    private transformStringLiteral(node: ts.StringLiteral): ast.StringLiteral {
        // TODO: we need to dynamically populate the resulting object with ECMAScript-style string functions.  It's not
        //     yet clear how to do this in a way that facilitates inter-language interoperability.  This is especially
        //     challenging because most use of such functions will be entirely dynamic.
        return this.copyLocation(node, {
            kind:  ast.stringLiteralKind,
            raw:   node.text,
            value: node.text,
        });
    }

    /** Patterns **/

    private transformArrayBindingPattern(node: ts.ArrayBindingPattern): ast.Expression {
        return contract.fail("NYI");
    }

    private transformArrayBindingElement(node: ts.ArrayBindingElement): (ast.Expression | null) {
        return contract.fail("NYI");
    }

    private transformBindingName(node: ts.BindingName): ast.Expression {
        return contract.fail("NYI");
    }

    private transformBindingIdentifier(node: ts.BindingName): ast.Identifier {
        contract.assert(node.kind === ts.SyntaxKind.Identifier,
                        "Binding name must be an identifier (TODO[marapongo/mu#34])");
        return this.transformIdentifier(<ts.Identifier>node);
    }

    private transformBindingPattern(node: ts.BindingPattern): ast.Expression {
        return contract.fail("NYI");
    }

    private transformComputedPropertyName(node: ts.ComputedPropertyName): ast.Expression {
        return contract.fail("NYI");
    }

    private transformIdentifierExpression(node: ts.Identifier): ast.Identifier {
        return this.copyLocation(node, {
            kind:  ast.identifierKind,
            ident: node.text,
        });
    }

    private transformObjectBindingPattern(node: ts.ObjectBindingPattern): ast.Expression {
        return contract.fail("NYI");
    }

    private transformObjectBindingElement(node: ts.BindingElement): ast.Expression {
        return contract.fail("NYI");
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
export async function transpile(comp: Compilation): Promise<pack.Package> {
    let t = await Transpiler.createFrom(comp);
    return t.transform();
}

