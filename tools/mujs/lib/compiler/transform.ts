// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import {contract, object} from "nodets";
import * as ts from "typescript";

import * as ast from "../ast";
import * as pack from "../pack";
import * as symbols from "../symbols";

// Translates a TypeScript bound tree into its equivalent MuPack/MuIL AST form, one tree per file.
export function transform(program: ts.Program): pack.Package {
    // Enumerate all source files (each of which is a module in ECMAScript), and transform it.
    let modules: ast.Modules = {};
    for (let sourceFile of program.getSourceFiles()) {
        // By default, skip declaration files, since they are "dependencies."
        // TODO(joe): how to handle re-exports in ECMAScript, such as index aggregation.
        // TODO(joe): this isn't a perfect heuristic.  But ECMAScript is all source dependencies, so there isn't a
        //     true notion of source versus binary dependency.  We could crack open the dependencies to see if they
        //     exist within an otherwise known package, but that seems a little hokey.
        if (!sourceFile.isDeclarationFile) {
            let mod: ast.Module = transformSourceFile(sourceFile);
            modules[mod.name.ident] = mod;
        }
    }

    // Now create a new package object.
    // TODO(joe): discover dependencies, name, etc. from Mu.json|yaml metadata.
    return {
        name:    "TODO",
        modules: modules,
    };
}

/** Constants **/

const defaultExport: string = "default"; // the ES6 default export name.

/** Helpers **/

// This function annotates a given MuPack/MuIL node with another TypeScript node's source position information.
function copyLocation<T extends ast.Node>(src: ts.Node, dst: T): T {
    let pos = (s: ts.SourceFile, p: number) => {
        // Translate a TypeScript position into a MuIL position (0 to 1 based lines).
        let lc = s.getLineAndCharacterOfPosition(p);
        return {
            line:   lc.line + 1,  // transform to 1-based line number
            column: lc.character,
        };
    };

    let s: ts.SourceFile = src.getSourceFile();
    dst.loc = {
        file:  s.fileName,
        start: pos(s, src.getStart()),
        end:   pos(s, src.getEnd()),
    };

    // Despite mutating in place, we return the node to facilitate a more fluent style.
    return dst;
}

/** AST queries **/

function isComputed(name: ts.Node | undefined): boolean {
    if (name) {
        return (name.kind === ts.SyntaxKind.ComputedPropertyName);
    }
    return false;
}

/** Transformations **/

/** Symbols **/

function transformIdentifier(node: ts.Identifier): ast.Identifier {
    return copyLocation(node, {
        kind:  ast.identifierKind,
        ident: node.text,
    });
}

/** Modules **/

// A top-level module element is either a definition or a statement.
type ModuleElement = ast.Definition | ast.Statement;

// This function transforms top-level TypeScript module elements into their corresponding nodes.  This transformation
// is largely evident in how it works, except that "loose code" in the form of arbitrary statements is not permitted in
// MuPack/MuIL.  As such, the appropriate top-level definitions (variables, functions, and classes) are returned as
// definitions, while any loose code (including variable initializers) is bundled into module inits and entrypoints.
function transformSourceFile(node: ts.SourceFile): ast.Module {
    // All definitions will go into a map keyed by their identifier.
    let members: ast.ModuleMembers = {};

    // Any top-level non-definition statements will pile up into the module initializer.
    let statements: ast.Statement[] = [];

    // Enumerate the module's statements and put them in the respective places.
    for (let statement of node.statements) {
        let elements: ModuleElement[] = transformSourceFileStatement(statement);
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

    return copyLocation(node, {
        kind:    ast.moduleKind,
        name:    <ast.Identifier>{
            kind:  ast.identifierKind,
            ident: node.moduleName,
        },
        members: members,
    });
}

// This function transforms a top-level TypeScript module statement.  It might return multiple elements in the rare
// cases -- like variable initializers -- that expand to multiple elements.
function transformSourceFileStatement(node: ts.Statement): ModuleElement[] {
    if (ts.getCombinedModifierFlags(node) & ts.ModifierFlags.Export) {
        return transformExportStatement(node);
    }
    else {
        switch (node.kind) {
            // Handle module directives; most translate into definitions.
            case ts.SyntaxKind.ExportAssignment:
                return [ transformExportAssignment(<ts.ExportAssignment>node) ];
            case ts.SyntaxKind.ExportDeclaration:
                return [ transformExportDeclaration(<ts.ExportDeclaration>node) ];
            case ts.SyntaxKind.ImportDeclaration:
                // TODO: register the import name so we can "mangle" any references to it accordingly later on.
                return contract.failf("NYI");

            // Handle declarations; each of these results in a definition.
            case ts.SyntaxKind.ClassDeclaration:
            case ts.SyntaxKind.FunctionDeclaration:
            case ts.SyntaxKind.InterfaceDeclaration:
            case ts.SyntaxKind.ModuleDeclaration:
            case ts.SyntaxKind.TypeAliasDeclaration:
            case ts.SyntaxKind.VariableStatement:
                return transformModuleDeclarationStatement(node, symbols.privateAccessibility);

            // For any other top-level statements, transform them.  They'll be added to the module initializer.
            default:
                return [ transformStatement(node) ];
        }
    }
}

function transformExportStatement(node: ts.Statement): ModuleElement[] {
    let elements: ModuleElement[] = transformModuleDeclarationStatement(node, symbols.publicAccessibility);

    // If this is a default export, first ensure that it is one of the legal default export kinds; namely, only function
    // or class is permitted, and specifically not interface or let.  Then smash the name with "default".
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

function transformExportAssignment(node: ts.ExportAssignment): ast.Definition {
    return contract.failf("NYI");
}

function transformExportDeclaration(node: ts.ExportDeclaration): ast.Definition {
    return contract.failf("NYI");
}

function transformExportSpecifier(node: ts.ExportSpecifier): ast.Definition {
    return contract.failf("NYI");
}

function transformImportDeclaration(node: ts.ImportDeclaration): ast.Definition {
    return contract.failf("NYI");
}

function transformImportClause(node: ts.ImportClause | undefined): ast.Definition[] {
    return contract.failf("NYI");
}

function transformImportSpecifier(node: ts.ImportSpecifier): ast.Definition {
    return contract.failf("NYI");
}

/** Statements **/

function transformStatement(node: ts.Statement): ast.Statement {
    switch (node.kind) {
        // Declaration statements:
        case ts.SyntaxKind.ClassDeclaration:
        case ts.SyntaxKind.FunctionDeclaration:
        case ts.SyntaxKind.InterfaceDeclaration:
        case ts.SyntaxKind.ModuleDeclaration:
        case ts.SyntaxKind.TypeAliasDeclaration:
            // TODO: issue a proper error; these sorts of declarations aren't valid statements in MuIL.
            return contract.failf(`Declaration node ${ts.SyntaxKind[node.kind]} isn't a valid statement in MuIL`);
        case ts.SyntaxKind.VariableStatement:
            return transformLocalVariableStatement(<ts.VariableStatement>node);

        // Control flow statements:
        case ts.SyntaxKind.BreakStatement:
            return transformBreakStatement(<ts.BreakStatement>node);
        case ts.SyntaxKind.ContinueStatement:
            return transformContinueStatement(<ts.ContinueStatement>node);
        case ts.SyntaxKind.DoStatement:
            return transformDoStatement(<ts.DoStatement>node);
        case ts.SyntaxKind.ForStatement:
            return transformForStatement(<ts.ForStatement>node);
        case ts.SyntaxKind.ForInStatement:
            return transformForInStatement(<ts.ForInStatement>node);
        case ts.SyntaxKind.ForOfStatement:
            return transformForOfStatement(<ts.ForOfStatement>node);
        case ts.SyntaxKind.IfStatement:
            return transformIfStatement(<ts.IfStatement>node);
        case ts.SyntaxKind.ReturnStatement:
            return transformReturnStatement(<ts.ReturnStatement>node);
        case ts.SyntaxKind.SwitchStatement:
            return transformSwitchStatement(<ts.SwitchStatement>node);
        case ts.SyntaxKind.ThrowStatement:
            return transformThrowStatement(<ts.ThrowStatement>node);
        case ts.SyntaxKind.TryStatement:
            return transformTryStatement(<ts.TryStatement>node);
        case ts.SyntaxKind.WhileStatement:
            return transformWhileStatement(<ts.WhileStatement>node);

        // Miscellaneous statements:
        case ts.SyntaxKind.Block:
            return transformBlock(<ts.Block>node);
        case ts.SyntaxKind.DebuggerStatement:
            return transformDebuggerStatement(<ts.DebuggerStatement>node);
        case ts.SyntaxKind.EmptyStatement:
            return transformEmptyStatement(<ts.EmptyStatement>node);
        case ts.SyntaxKind.ExpressionStatement:
            return transformExpressionStatement(<ts.ExpressionStatement>node);
        case ts.SyntaxKind.LabeledStatement:
            return transformLabeledStatement(<ts.LabeledStatement>node);
        case ts.SyntaxKind.ModuleBlock:
            return transformModuleBlock(<ts.ModuleBlock>node);
        case ts.SyntaxKind.WithStatement:
            return transformWithStatement(<ts.WithStatement>node);

        // Unrecognized statement:
        default:
            return contract.failf(`Unrecognized statement kind: ${ts.SyntaxKind[node.kind]}`);
    }
}

// This routine transforms a declaration statement in TypeScript to a MuIL definition.  Note that definitions in MuIL
// aren't statements, hence the partitioning between transformDeclaration and transformStatement.  Note that variables
// do not result in Definitions because they may require higher-level processing to deal with initializer.
function transformModuleDeclarationStatement(node: ts.Statement, access: symbols.Accessibility): ModuleElement[] {
    switch (node.kind) {
        // Declarations:
        case ts.SyntaxKind.ClassDeclaration:
            return [ transformClassDeclaration(<ts.ClassDeclaration>node, access) ];
        case ts.SyntaxKind.FunctionDeclaration:
            return [ transformFunctionDeclaration(<ts.FunctionDeclaration>node, access) ];
        case ts.SyntaxKind.InterfaceDeclaration:
            return [ transformInterfaceDeclaration(<ts.InterfaceDeclaration>node, access) ];
        case ts.SyntaxKind.ModuleDeclaration:
            return [ transformModuleDeclaration(<ts.ModuleDeclaration>node, access) ];
        case ts.SyntaxKind.TypeAliasDeclaration:
            return [ transformTypeAliasDeclaration(<ts.TypeAliasDeclaration>node, access) ];
        case ts.SyntaxKind.VariableStatement:
            return transformModuleVariableStatement(<ts.VariableStatement>node, access);
        default:
            return contract.failf(`Node kind is not a module declaration: ${ts.SyntaxKind[node.kind]}`);
    }
}

// This function transforms a TypeScript Statement, and returns a Block (allocating a new one if needed).
function transformStatementAsBlock(node: ts.Statement): ast.Block {
    // Transform the statement.  Then, if it is already a block, return it; otherwise, append it to a new one.
    let statement: ast.Statement = transformStatement(node);
    if (statement.kind === ast.blockKind) {
        return <ast.Block>statement;
    }
    return copyLocation(node, {
        kind:       ast.blockKind,
        statements: [ statement ],
    });
}

/** Declaration statements **/

function transformClassDeclaration(node: ts.ClassDeclaration, access: symbols.Accessibility): ast.Class {
    return contract.failf("NYI");
}

function transformDeclarationName(node: ts.DeclarationName): ast.Expression {
    switch (node.kind) {
        case ts.SyntaxKind.ArrayBindingPattern:
            return transformArrayBindingPattern(node);
        case ts.SyntaxKind.ComputedPropertyName:
            return transformComputedPropertyName(node);
        case ts.SyntaxKind.ObjectBindingPattern:
            return transformObjectBindingPattern(node);
        case ts.SyntaxKind.Identifier:
            return transformIdentifierExpression(node);
        default:
            return contract.failf(`Unrecognized declaration node: ${ts.SyntaxKind[node.kind]}`);
    }
}

function transformDeclarationIdentifier(node: ts.DeclarationName): ast.Identifier {
    switch (node.kind) {
        case ts.SyntaxKind.Identifier:
            return transformIdentifierExpression(node);
        default:
            return contract.failf(`Unrecognized declaration identifier: ${ts.SyntaxKind[node.kind]}`);
    }
}

function transformFunctionDeclaration(node: ts.FunctionDeclaration, access: symbols.Accessibility): ast.Function {
    return contract.failf("NYI");
}

function transformFunctionLikeDeclaration(node: ts.FunctionLikeDeclaration): ast.Function {
    return contract.failf("NYI");
}

function transformInterfaceDeclaration(node: ts.InterfaceDeclaration, access: symbols.Accessibility): ast.Class {
    return contract.failf("NYI");
}

function transformModuleDeclaration(node: ts.ModuleDeclaration, access: symbols.Accessibility): ast.Module {
    return contract.failf("NYI");
}

function transformParameterDeclaration(node: ts.ParameterDeclaration): ast.LocalVariable {
    return contract.failf("NYI");
}

function transformTypeAliasDeclaration(node: ts.TypeAliasDeclaration, access: symbols.Accessibility): ast.Class {
    return contract.failf("NYI");
}

// A variable is a MuIL variable with an optional initializer expression.  This is required because MuIL doesn't support
// complex initializers on the Variable AST node -- they must be explicitly placed into an initializer section.
interface VariableDeclaration {
    node:         ts.Node;           // the source node.
    local:        ast.LocalVariable; // the MuIL variable information.
    legacyVar?:   boolean;           // true if we should mimick legacy ECMAScript "var" behavior; false for "let".
    initializer?: ast.Expression;    // an optional initialization expression.
}

function makeVariableInitializer(variable: VariableDeclaration): ast.Statement {
    return copyLocation(variable.node, {
        kind:     ast.binaryOperatorExpressionKind,
        left:     <ast.LoadVariableExpression>{
            kind:     ast.loadVariableExpressionKind,
            variable: variable.local.name.ident,
        },
        operator: "=",
        right:    variable.initializer,
    });
}

function transformVariableStatement(node: ts.VariableStatement): VariableDeclaration[] {
    let variables: VariableDeclaration[] = node.declarationList.declarations.map(transformVariableDeclaration);

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

function transformLocalVariableStatement(node: ts.VariableStatement): ast.Statement {
    // For variables, we need to append initializers as assignments if there are any.
    // TODO: emulate "var"-like scoping.
    let statements: ast.Statement[] = [];
    let variables: VariableDeclaration[] = transformVariableStatement(node);
    for (let variable of variables) {
        statements.push(<ast.LocalVariableDeclaration>{
            kind:  ast.localVariableDeclarationKind,
            local: variable.local,
        });
        if (variable.initializer) {
            statements.push(makeVariableInitializer(variable));
        }
    };
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

function transformModuleVariableStatement(node: ts.VariableStatement, access: symbols.Accessibility): ModuleElement[] {
    let elements: ModuleElement[] = [];
    let variables: VariableDeclaration[] = transformVariableStatement(node);
    for (let variable of variables) {
        // First transform the local varaible into a module property.
        // TODO(joe): emulate "var"-like scoping.
        elements.push(<ast.ModuleProperty>{
            kind:        ast.modulePropertyKind,
            name:        variable.local.name,
            type:        variable.local.type,
            description: variable.local.description,
            default:     variable.local.default,
            readonly:    variable.local.readonly,
        });

        // Next, if there is an initializer, use it to initialize the variable in the module initializer.
        if (variable.initializer) {
            elements.push(makeVariableInitializer(variable));
        }
    }
    return elements;
}

function transformVariableDeclaration(node: ts.VariableDeclaration): VariableDeclaration {
    let initializer: ast.Expression | undefined;
    if (node.initializer) {
        initializer = transformExpression(node.initializer);
    }
    return {
        node:  node,
        local: {
            kind: ast.localVariableKind,
            name: transformDeclarationIdentifier(node.name),
            type: "TODO",
        },
        initializer: initializer,
    };
}

function transformVariableDeclarationList(node: ts.VariableDeclarationList): VariableDeclaration[] {
    return node.declarations.map(transformVariableDeclaration);
}

/** Classes **/

function transformClassElement(node: ts.ClassElement): ast.ClassMember {
    switch (node.kind) {
        // All the function-like members go here:
        case ts.SyntaxKind.Constructor:
            return transformClassElementFunctionLike(<ts.ConstructorDeclaration>node);
        case ts.SyntaxKind.MethodDeclaration:
            return transformClassElementFunctionLike(<ts.MethodDeclaration>node);
        case ts.SyntaxKind.GetAccessor:
            return transformClassElementFunctionLike(<ts.GetAccessorDeclaration>node);
        case ts.SyntaxKind.SetAccessor:
            return transformClassElementFunctionLike(<ts.SetAccessorDeclaration>node);

        // Properties are not function-like, so we translate them differently.
        case ts.SyntaxKind.PropertyDeclaration:
            return transformClassElementProperty(<ts.PropertyDeclaration>node);

        // Unrecognized cases:
        case ts.SyntaxKind.SemicolonClassElement:
            return contract.failf("Expected all SemiColonClassElements to be filtered out of AST tree");
        default:
            return contract.failf(`Unrecognized TypeElement node kind: ${ts.SyntaxKind[node.kind]}`);
    }
}

function transformClassElementFunctionLike(node: ts.FunctionLikeDeclaration): ast.Definition {
    return contract.failf("NYI");
}

function transformClassElementProperty(node: ts.PropertyDeclaration): ast.ClassProperty {
    return contract.failf("NYI");
}

/** Control flow statements **/

function transformBreakStatement(node: ts.BreakStatement): ast.BreakStatement {
    return copyLocation(node, {
        kind:  ast.breakStatementKind,
        label: object.maybeUndefined(node.label, transformIdentifier),
    });
}

function transformCaseOrDefaultClause(node: ts.CaseOrDefaultClause): ast.Statement {
    return contract.failf("NYI");
}

function transformCatchClause(node: ts.CatchClause): ast.Statement {
    return contract.failf("NYI");
}

function transformContinueStatement(node: ts.ContinueStatement): ast.ContinueStatement {
    return copyLocation(node, {
        kind:  ast.continueStatementKind,
        label: object.maybeUndefined(node.label, transformIdentifier),
    });
}

// This transforms a higher-level TypeScript `do`/`while` statement by expanding into an ordinary `while` statement.
function transformDoStatement(node: ts.DoStatement): ast.WhileStatement {
    // Now create a new block that simply concatenates the existing one with a test/`break`.
    let body: ast.Block = copyLocation(node.statement, {
        kind:       ast.blockKind,
        statements: [
            transformStatement(node.statement),
            <ast.IfStatement>{
                kind:      ast.ifStatementKind,
                condition: <ast.UnaryOperatorExpression>{
                    kind:     ast.unaryOperatorExpressionKind,
                    operator: "!",
                    operand:  transformExpression(node.expression),
                },
                consequent: <ast.BreakStatement>{
                    kind: ast.breakStatementKind,
                },
            },
        ],
    });

    return copyLocation(node, {
        kind: ast.whileStatementKind,
        test: <ast.BoolLiteral>{
            kind:  ast.boolLiteralKind,
            value: true,
        },
        body: body,
    });
}

function transformForStatement(node: ts.ForStatement): ast.Statement {
    return contract.failf("NYI");
}

function transformForInitializer(node: ts.ForInitializer): ast.Statement {
    return contract.failf("NYI");
}

function transformForInStatement(node: ts.ForInStatement): ast.Statement {
    return contract.failf("NYI");
}

function transformForOfStatement(node: ts.ForOfStatement): ast.Statement {
    return contract.failf("NYI");
}

function transformIfStatement(node: ts.IfStatement): ast.IfStatement {
    return copyLocation(node, {
        kind:       ast.ifStatementKind,
        condition:  transformExpression(node.expression),
        consequent: transformStatement(node.thenStatement),
        alternate:  object.maybeUndefined(node.elseStatement, transformStatement),
    });
}

function transformReturnStatement(node: ts.ReturnStatement): ast.ReturnStatement {
    return copyLocation(node, {
        kind:       ast.returnStatementKind,
        expression: object.maybeUndefined(node.expression, transformExpression),
    });
}

function transformSwitchStatement(node: ts.SwitchStatement): ast.Statement {
    return contract.failf("NYI");
}

function transformThrowStatement(node: ts.ThrowStatement): ast.ThrowStatement {
    return copyLocation(node, {
        kind:       ast.throwStatementKind,
        expression: transformExpression(node.expression),
    });
}

function transformTryStatement(node: ts.TryStatement): ast.TryCatchFinally {
    return contract.failf("NYI");
}

function transformWhileStatement(node: ts.WhileStatement): ast.Statement {
    return copyLocation(node, {
        kind: ast.whileStatementKind,
        test: transformExpression(node.expression),
        body: transformStatementAsBlock(node.statement),
    });
}

/** Miscellaneous statements **/

function transformBlock(node: ts.Block): ast.Block {
    // TODO(joe): map directives.
    return copyLocation(node, {
        kind:       ast.blockKind,
        statements: node.statements.map(transformStatement),
    });
}

function transformDebuggerStatement(node: ts.DebuggerStatement): ast.Statement {
    // The debugger statement in ECMAScript can be used to trip a breakpoint.  We don't have the equivalent in Mu at
    // the moment, so we simply produce an empty statement in its place.
    return copyLocation(node, {
        kind: ast.emptyStatementKind,
    });
}

function transformEmptyStatement(node: ts.EmptyStatement): ast.EmptyStatement {
    return copyLocation(node, {
        kind: ast.emptyStatementKind,
    });
}

function transformExpressionStatement(node: ts.ExpressionStatement): ast.ExpressionStatement {
    return copyLocation(node, {
        kind:       ast.expressionStatementKind,
        expression: transformExpression(node.expression),
    });
}

function transformLabeledStatement(node: ts.LabeledStatement): ast.LabeledStatement {
    return copyLocation(node, {
        kind:      ast.labeledStatementKind,
        label:     transformIdentifier(node.label),
        statement: transformStatement(node.statement),
    });
}

function transformModuleBlock(node: ts.ModuleBlock): ast.Block {
    return contract.failf("NYI");
}

function transformWithStatement(node: ts.WithStatement): ast.Statement {
    return contract.failf("NYI");
}

/** Expressions **/

function transformExpression(node: ts.Expression): ast.Expression {
    switch (node.kind) {
        // Expressions:
        case ts.SyntaxKind.ArrayLiteralExpression:
            return transformArrayLiteralExpression(<ts.ArrayLiteralExpression>node);
        case ts.SyntaxKind.ArrowFunction:
            return transformArrowFunction(<ts.ArrowFunction>node);
        case ts.SyntaxKind.BinaryExpression:
            return transformBinaryExpression(<ts.BinaryExpression>node);
        case ts.SyntaxKind.CallExpression:
            return transformCallExpression(<ts.CallExpression>node);
        case ts.SyntaxKind.ClassExpression:
            return transformClassExpression(<ts.ClassExpression>node);
        case ts.SyntaxKind.ConditionalExpression:
            return transformConditionalExpression(<ts.ConditionalExpression>node);
        case ts.SyntaxKind.DeleteExpression:
            return transformDeleteExpression(<ts.DeleteExpression>node);
        case ts.SyntaxKind.ElementAccessExpression:
            return transformElementAccessExpression(<ts.ElementAccessExpression>node);
        case ts.SyntaxKind.FunctionExpression:
            return transformFunctionExpression(<ts.FunctionExpression>node);
        case ts.SyntaxKind.Identifier:
            return transformIdentifierExpression(<ts.Identifier>node);
        case ts.SyntaxKind.ObjectLiteralExpression:
            return transformObjectLiteralExpression(<ts.ObjectLiteralExpression>node);
        case ts.SyntaxKind.PostfixUnaryExpression:
            return transformPostfixUnaryExpression(<ts.PostfixUnaryExpression>node);
        case ts.SyntaxKind.PrefixUnaryExpression:
            return transformPrefixUnaryExpression(<ts.PrefixUnaryExpression>node);
        case ts.SyntaxKind.PropertyAccessExpression:
            return transformPropertyAccessExpression(<ts.PropertyAccessExpression>node);
        case ts.SyntaxKind.NewExpression:
            return transformNewExpression(<ts.NewExpression>node);
        case ts.SyntaxKind.OmittedExpression:
            return transformOmittedExpression(<ts.OmittedExpression>node);
        case ts.SyntaxKind.ParenthesizedExpression:
            return transformParenthesizedExpression(<ts.ParenthesizedExpression>node);
        case ts.SyntaxKind.SpreadElement:
            return transformSpreadElement(<ts.SpreadElement>node);
        case ts.SyntaxKind.SuperKeyword:
            return transformSuperExpression(<ts.SuperExpression>node);
        case ts.SyntaxKind.TaggedTemplateExpression:
            return transformTaggedTemplateExpression(<ts.TaggedTemplateExpression>node);
        case ts.SyntaxKind.TemplateExpression:
            return transformTemplateExpression(<ts.TemplateExpression>node);
        case ts.SyntaxKind.ThisKeyword:
            return transformThisExpression(<ts.ThisExpression>node);
        case ts.SyntaxKind.TypeOfExpression:
            return transformTypeOfExpression(<ts.TypeOfExpression>node);
        case ts.SyntaxKind.VoidExpression:
            return transformVoidExpression(<ts.VoidExpression>node);
        case ts.SyntaxKind.YieldExpression:
            return transformYieldExpression(<ts.YieldExpression>node);

        // Literals:
        case ts.SyntaxKind.FalseKeyword:
        case ts.SyntaxKind.TrueKeyword:
            return transformBooleanLiteral(<ts.BooleanLiteral>node);
        case ts.SyntaxKind.NoSubstitutionTemplateLiteral:
            return transformNoSubstitutionTemplateLiteral(<ts.NoSubstitutionTemplateLiteral>node);
        case ts.SyntaxKind.NullKeyword:
            return transformNullLiteral(<ts.NullLiteral>node);
        case ts.SyntaxKind.NumericLiteral:
            return transformNumericLiteral(<ts.NumericLiteral>node);
        case ts.SyntaxKind.RegularExpressionLiteral:
            return transformRegularExpressionLiteral(<ts.RegularExpressionLiteral>node);
        case ts.SyntaxKind.StringLiteral:
            return transformStringLiteral(<ts.StringLiteral>node);

        // Unrecognized:
        default:
            return contract.failf(`Unrecognized expression kind: ${ts.SyntaxKind[node.kind]}`);
    }
}

function transformArrayLiteralExpression(node: ts.ArrayLiteralExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformArrowFunction(node: ts.ArrowFunction): ast.Expression {
    return contract.failf("NYI");
}

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

function transformBinaryExpression(node: ts.BinaryExpression): ast.Expression {
    let op: ts.SyntaxKind = node.operatorToken.kind;
    if (op === ts.SyntaxKind.CommaToken) {
        // Translate this into a sequence expression.
        return transformBinarySequenceExpression(node);
    }
    else {
        // Translate this into an ordinary binary operator.
        return transformBinaryOperatorExpression(node);
    }
}

function transformBinaryOperatorExpression(node: ts.BinaryExpression): ast.BinaryOperatorExpression {
    let operator: ast.BinaryOperator | undefined = binaryOperators.get(node.operatorToken.kind);
    if (!operator) {
        // TODO: finish binary operator mapping; for any left that are unsupported, introduce a real error message.
        return contract.failf(`Unsupported binary operator: ${ts.SyntaxKind[node.operatorToken.kind]}`);
    }
    return copyLocation(node, {
        kind:     ast.binaryOperatorExpressionKind,
        operator: operator,
        left:     transformExpression(node.left),
        right:    transformExpression(node.right),
    });
}

function transformBinarySequenceExpression(node: ts.BinaryExpression): ast.SequenceExpression {
    contract.assert(node.operatorToken.kind === ts.SyntaxKind.CommaToken);
    let curr: ts.Expression = node;
    let binary: ts.BinaryExpression = node;
    let expressions: ast.Expression[] = [];
    while (curr.kind === ts.SyntaxKind.BinaryExpression &&
            (binary = <ts.BinaryExpression>curr).operatorToken.kind === ts.SyntaxKind.CommaToken) {
        expressions.unshift(transformExpression(binary.right));
        curr = binary.left;
    }
    expressions.unshift(transformExpression(curr));
    return {
        kind:        ast.sequenceExpressionKind,
        expressions: expressions,
    };
}

function transformCallExpression(node: ts.CallExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformClassExpression(node: ts.ClassExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformConditionalExpression(node: ts.ConditionalExpression): ast.ConditionalExpression {
    return copyLocation(node, {
        kind:       ast.conditionalExpressionKind,
        condition:  transformExpression(node.condition),
        consequent: transformExpression(node.whenTrue),
        alternate:  transformExpression(node.whenFalse),
    });
}

function transformDeleteExpression(node: ts.DeleteExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformElementAccessExpression(node: ts.ElementAccessExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformFunctionExpression(node: ts.FunctionExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformObjectLiteralExpression(node: ts.ObjectLiteralExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformObjectLiteralElement(node: ts.ObjectLiteralElement): ast.Expression {
    return contract.failf("NYI");
}

function transformObjectLiteralPropertyElement(
        node: ts.PropertyAssignment | ts.ShorthandPropertyAssignment): ast.Expression {
    return contract.failf("NYI");
}

function transformObjectLiteralFunctionLikeElement(
        node: ts.AccessorDeclaration | ts.MethodDeclaration): ast.Expression {
    return contract.failf("NYI");
}

let postfixUnaryOperators = new Map<ts.SyntaxKind, ast.UnaryOperator>([
    [ ts.SyntaxKind.PlusPlusToken,   "++" ],
    [ ts.SyntaxKind.MinusMinusToken, "--" ],
]);

function transformPostfixUnaryExpression(node: ts.PostfixUnaryExpression): ast.UnaryOperatorExpression {
    let operator: ast.UnaryOperator | undefined = postfixUnaryOperators.get(node.operator);
    contract.assert(!!(operator = operator!));
    return copyLocation(node, {
        kind:     ast.unaryOperatorExpressionKind,
        postfix:  true,
        operator: operator,
        operand:  transformExpression(node.operand),
    });
}

let prefixUnaryOperators = new Map<ts.SyntaxKind, ast.UnaryOperator>([
    [ ts.SyntaxKind.PlusPlusToken,    "++" ],
    [ ts.SyntaxKind.MinusMinusToken,  "--" ],
    [ ts.SyntaxKind.PlusToken,        "+" ],
    [ ts.SyntaxKind.MinusToken,       "-" ],
    [ ts.SyntaxKind.TildeToken,       "~" ],
    [ ts.SyntaxKind.ExclamationToken, "!" ],
]);

function transformPrefixUnaryExpression(node: ts.PrefixUnaryExpression): ast.UnaryOperatorExpression {
    let operator: ast.UnaryOperator | undefined = prefixUnaryOperators.get(node.operator);
    contract.assert(!!(operator = operator!));
    return copyLocation(node, {
        kind:     ast.unaryOperatorExpressionKind,
        postfix:  false,
        operator: operator,
        operand:  transformExpression(node.operand),
    });
}

function transformPropertyAccessExpression(node: ts.PropertyAccessExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformNewExpression(node: ts.NewExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformOmittedExpression(node: ts.OmittedExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformParenthesizedExpression(node: ts.ParenthesizedExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformSpreadElement(node: ts.SpreadElement): ast.Expression {
    return contract.failf("NYI");
}

function transformSuperExpression(node: ts.SuperExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformTaggedTemplateExpression(node: ts.TaggedTemplateExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformTemplateExpression(node: ts.TemplateExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformThisExpression(node: ts.ThisExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformTypeOfExpression(node: ts.TypeOfExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformVoidExpression(node: ts.VoidExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformYieldExpression(node: ts.YieldExpression): ast.Expression {
    return contract.failf("NYI");
}

/** Literals **/

function transformBooleanLiteral(node: ts.BooleanLiteral): ast.BoolLiteral {
    contract.assert(node.kind === ts.SyntaxKind.FalseKeyword || node.kind === ts.SyntaxKind.TrueKeyword);
    return copyLocation(node, {
        kind:  ast.boolLiteralKind,
        raw:   node.getText(),
        value: (node.kind === ts.SyntaxKind.TrueKeyword),
    });
}

function transformNoSubstitutionTemplateLiteral(node: ts.NoSubstitutionTemplateLiteral): ast.Expression {
    return contract.failf("NYI");
}

function transformNullLiteral(node: ts.NullLiteral): ast.NullLiteral {
    return copyLocation(node, {
        kind: ast.nullLiteralKind,
        raw:  node.getText(),
    });
}

function transformNumericLiteral(node: ts.NumericLiteral): ast.NumberLiteral {
    return copyLocation(node, {
        kind:  ast.numberLiteralKind,
        raw:   node.text,
        value: Number(node.text),
    });
}

function transformRegularExpressionLiteral(node: ts.RegularExpressionLiteral): ast.Expression {
    return contract.failf("NYI");
}

function transformStringLiteral(node: ts.StringLiteral): ast.StringLiteral {
    // TODO: we need to dynamically populate the resulting object with ECMAScript-style string functions.  It's not
    //     yet clear how to do this in a way that facilitates inter-language interoperability.  This is especially
    //     challenging because most use of such functions will be entirely dynamic.
    return copyLocation(node, {
        kind:  ast.stringLiteralKind,
        raw:   node.text,
        value: node.text,
    });
}

/** Patterns **/

function transformArrayBindingPattern(node: ts.ArrayBindingPattern): ast.Expression {
    return contract.failf("NYI");
}

function transformArrayBindingElement(node: ts.ArrayBindingElement): (ast.Expression | null) {
    return contract.failf("NYI");
}

function transformBindingName(node: ts.BindingName): ast.Expression {
    return contract.failf("NYI");
}

function transformBindingPattern(node: ts.BindingPattern): ast.Expression {
    return contract.failf("NYI");
}

function transformComputedPropertyName(node: ts.ComputedPropertyName): ast.Expression {
    return contract.failf("NYI");
}

function transformIdentifierExpression(node: ts.Identifier): ast.Identifier {
    return copyLocation(node, {
        kind:  ast.identifierKind,
        ident: node.text,
    });
}

function transformObjectBindingPattern(node: ts.ObjectBindingPattern): ast.Expression {
    return contract.failf("NYI");
}

function transformObjectBindingElement(node: ts.BindingElement): ast.Expression {
    return contract.failf("NYI");
}

function transformPropertyName(node: ts.PropertyName): ast.Expression {
    return contract.failf("NYI");
}

