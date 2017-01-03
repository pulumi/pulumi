// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import {contract, object} from "nodets";
import * as ts from "typescript";

import * as ast from "../ast";
import * as pack from "../pack";
import * as symbols from "../symbols";

// Translates a TypeScript bound tree into its equivalent MuPack/MuIL AST form, one tree per file.
export function transform(program: ts.Program): pack.Package {
    return contract.failf("NYI");
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

function transformSourceFile(node: ts.SourceFile): ast.Module {
    return contract.failf("NYI");
}

// This function transforms a top-level TypeScript module element into its corresponding definition node.  This
// transformation is largely evident in how it works, except that "loose code" in the form of arbitrary statements is
// not permitted in MuIL.  As such, the appropriate top-level definitions (modules, variables, functions, and classes)
// are returned as definition nodes, while any loose code (other than variable initializers) is rejected outright.
// MuIL supports module initializers and entrypoint functions for executable MuPacks, where such code can go instead.
function transformSourceFileElement(node: ts.Statement): ast.Definition {
    return contract.failf("NYI");
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
            return transformClassDeclaration(<ts.ClassDeclaration>node);
        case ts.SyntaxKind.FunctionDeclaration:
            return transformFunctionDeclaration(<ts.FunctionDeclaration>node);
        case ts.SyntaxKind.InterfaceDeclaration:
            return transformInterfaceDeclaration(<ts.InterfaceDeclaration>node);
        case ts.SyntaxKind.ModuleDeclaration:
            return transformModuleDeclaration(<ts.ModuleDeclaration>node);
        case ts.SyntaxKind.TypeAliasDeclaration:
            return transformTypeAliasDeclaration(<ts.TypeAliasDeclaration>node);
        case ts.SyntaxKind.VariableStatement:
            return transformVariableStatement(<ts.VariableStatement>node);

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

function transformClassDeclaration(node: ts.ClassDeclaration): ast.Class {
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

function transformFunctionDeclaration(node: ts.FunctionDeclaration): ast.Function {
    return contract.failf("NYI");
}

function transformFunctionLikeDeclaration(node: ts.FunctionLikeDeclaration): ast.Function {
    return contract.failf("NYI");
}

function transformInterfaceDeclaration(node: ts.InterfaceDeclaration): ast.Class {
    return contract.failf("NYI");
}

function transformModuleDeclaration(node: ts.ModuleDeclaration): ast.Statement {
    return contract.failf("NYI");
}

function transformParameterDeclaration(node: ts.ParameterDeclaration): ast.Parameter {
    return contract.failf("NYI");
}

function transformTypeAliasDeclaration(node: ts.TypeAliasDeclaration): ast.Statement {
    return contract.failf("NYI");
}

function transformVariableStatement(node: ts.VariableStatement): ast.LocalVariableDeclaration {
    return contract.failf("NYI");
}

function transformVariableDeclaration(node: ts.VariableDeclaration): ast.LocalVariableDeclaration {
    return contract.failf("NYI");
}

function transformVariableDeclarationList(node: ts.VariableDeclarationList): ast.LocalVariableDeclaration[] {
    return contract.failf("NYI");
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
        test: <ast.BoolLiteralExpression>{
            kind:  ast.boolLiteralExpressionKind,
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
    return contract.failf("NYI");
}

function transformEmptyStatement(node: ts.EmptyStatement): ast.EmptyStatement {
    return copyLocation(node, {
        kind: ast.emptyStatementKind
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

function transformBinaryExpression(node: ts.BinaryExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformBinaryAssignmentExpression(node: ts.BinaryExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformBinaryLogicalExpression(node: ts.BinaryExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformBinaryOperatorExpression(node: ts.BinaryExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformBinarySequenceExpression(node: ts.BinaryExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformCallExpression(node: ts.CallExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformClassExpression(node: ts.ClassExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformConditionalExpression(node: ts.ConditionalExpression): ast.Expression {
    return contract.failf("NYI");
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

function transformPostfixUnaryExpression(node: ts.PostfixUnaryExpression): ast.Expression {
    return contract.failf("NYI");
}

function transformPrefixUnaryExpression(node: ts.PrefixUnaryExpression): ast.Expression {
    return contract.failf("NYI");
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

function transformBooleanLiteral(node: ts.BooleanLiteral): ast.BoolLiteralExpression {
    contract.assert(node.kind === ts.SyntaxKind.FalseKeyword || node.kind === ts.SyntaxKind.TrueKeyword);
    return copyLocation(node, {
        kind:  ast.boolLiteralExpressionKind,
        raw:   node.getText(),
        value: (node.kind === ts.SyntaxKind.TrueKeyword),
    });
}

function transformNoSubstitutionTemplateLiteral(node: ts.NoSubstitutionTemplateLiteral): ast.Expression {
    return contract.failf("NYI");
}

function transformNullLiteral(node: ts.NullLiteral): ast.NullLiteralExpression {
    return copyLocation(node, {
        kind: ast.nullLiteralExpressionKind,
        raw:  node.getText(),
    });
}

function transformNumericLiteral(node: ts.NumericLiteral): ast.NumberLiteralExpression {
    return copyLocation(node, {
        kind:  ast.numberLiteralExpressionKind,
        raw:   node.text,
        value: Number(node.text),
    });
}

function transformRegularExpressionLiteral(node: ts.RegularExpressionLiteral): ast.Expression {
    return contract.failf("NYI");
}

function transformStringLiteral(node: ts.StringLiteral): ast.StringLiteralExpression {
    return copyLocation(node, {
        kind:  ast.stringLiteralExpressionKind,
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

function transformIdentifierExpression(node: ts.Identifier): ast.Expression {
    return contract.failf("NYI");
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

