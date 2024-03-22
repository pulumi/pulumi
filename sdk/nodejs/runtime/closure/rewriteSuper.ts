// Copyright 2016-2022, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as semver from "semver";
import * as ts from "typescript";
import * as utils from "./utils";

type Factory = {
    createIdentifier: typeof ts.createIdentifier;
    createThis: typeof ts.createThis;
    createPropertyAccessExpression: typeof ts.createPropertyAccess;
    updatePropertyAccessExpression: typeof ts.updatePropertyAccess;
    updateFunctionDeclaration: typeof ts.updateFunctionDeclaration;
    updateElementAccessExpression: typeof ts.updateElementAccess;
    updateCallExpression: typeof ts.updateCall;
};

// TypeScript 4.0 moved the factory functions to the transformationContext
// with deprecation. TypeScript 5.0 removed the deprecated functions.
// https://github.com/microsoft/TypeScript/wiki/API-Breaking-Changes#typescript-40
// Use a shim factory that calls the correct function based on the TypeScript version.
function getFactory(transformationContext: ts.TransformationContext): Factory {
    const tsVersion = semver.parse(ts.version)!;
    const tsLessThan4 = semver.satisfies(tsVersion, "<4.0.0");
    const tsLessThan48 = semver.satisfies(tsVersion, "<4.8.0");
    const transformationContextFactory = (<any>transformationContext).factory;

    // In 4.8 the signature of updateFunctionDeclaration changed to remove the decorators parameter.
    function updateFunctionDeclaration(
        node: ts.FunctionDeclaration,
        decorators: readonly ts.Decorator[] | undefined,
        modifiers: readonly ts.Modifier[] | undefined,
        asteriskToken: ts.AsteriskToken | undefined,
        name: ts.Identifier | undefined,
        typeParameters: readonly ts.TypeParameterDeclaration[] | undefined,
        parameters: readonly ts.ParameterDeclaration[],
        type: ts.TypeNode | undefined,
        body: ts.Block | undefined,
    ): ts.FunctionDeclaration {
        if (tsLessThan4) {
            return ts.updateFunctionDeclaration(
                node,
                decorators,
                modifiers,
                asteriskToken,
                name,
                typeParameters,
                parameters,
                type,
                body,
            );
        } else if (tsLessThan48) {
            return transformationContextFactory.updateFunctionDeclaration(
                node,
                decorators,
                modifiers,
                asteriskToken,
                name,
                typeParameters,
                parameters,
                type,
                body,
            );
        } else {
            return transformationContextFactory.updateFunctionDeclaration(
                node,
                modifiers,
                asteriskToken,
                name,
                typeParameters,
                parameters,
                type,
                body,
            );
        }
    }

    return {
        createIdentifier: tsLessThan4 ? ts.createIdentifier : transformationContextFactory.createIdentifier,
        createThis: tsLessThan4 ? ts.createThis : transformationContextFactory.createThis,
        createPropertyAccessExpression: tsLessThan4
            ? ts.createPropertyAccess
            : transformationContextFactory.createPropertyAccessExpression,
        updatePropertyAccessExpression: tsLessThan4
            ? ts.updatePropertyAccess
            : transformationContextFactory.updatePropertyAccessExpression,
        updateFunctionDeclaration,
        updateElementAccessExpression: tsLessThan4
            ? ts.updateElementAccess
            : transformationContextFactory.updateElementAccessExpression,
        updateCallExpression: tsLessThan4 ? ts.updateCall : transformationContextFactory.updateCallExpression,
    };
}

/** @internal */
export function rewriteSuperReferences(code: string, isStatic: boolean): string {
    const sourceFile = ts.createSourceFile("", code, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);

    // Transform any usages of "super(...)" into "__super.call(this, ...)", any
    // instance usages of "super.xxx" into "__super.prototype.xxx" and any static
    // usages of "super.xxx" into "__super.xxx"
    const transformed = ts.transform(sourceFile, [rewriteSuperCallsWorker]);
    const printer = ts.createPrinter({ newLine: ts.NewLineKind.LineFeed });
    const output = printer.printNode(ts.EmitHint.Unspecified, transformed.transformed[0], sourceFile).trim();

    return output;

    function rewriteSuperCallsWorker(transformationContext: ts.TransformationContext) {
        const factory = getFactory(transformationContext);
        const newNodes = new Set<ts.Node>();
        let firstFunctionDeclaration = true;

        function visitor(node: ts.Node): ts.Node {
            // Convert the top level function so it doesn't have a name. We want to convert the user
            // function to an anonymous function so that interior references to the same function
            // bind properly.  i.e. if we start with "function f() { f(); }" then this gets converted to
            //
            //  function __f() {
            //      with ({ f: __f }) {
            //          return /*f*/() { f(); }
            //
            // This means the inner call properly binds to the *outer* function we create.
            if (firstFunctionDeclaration && ts.isFunctionDeclaration(node)) {
                firstFunctionDeclaration = false;
                const funcDecl = ts.visitEachChild(node, visitor, transformationContext);

                const text = utils.isLegalMemberName(funcDecl.name!.text) ? "/*" + funcDecl.name!.text + "*/" : "";
                return factory.updateFunctionDeclaration(
                    funcDecl,
                    funcDecl.decorators,
                    funcDecl.modifiers,
                    funcDecl.asteriskToken,
                    factory.createIdentifier(text),
                    funcDecl.typeParameters,
                    funcDecl.parameters,
                    funcDecl.type,
                    funcDecl.body,
                );
            }

            if (node.kind === ts.SyntaxKind.SuperKeyword) {
                const newNode = factory.createIdentifier("__super");
                newNodes.add(newNode);
                return newNode;
            } else if (ts.isPropertyAccessExpression(node) && node.expression.kind === ts.SyntaxKind.SuperKeyword) {
                const expr = isStatic
                    ? factory.createIdentifier("__super")
                    : factory.createPropertyAccessExpression(factory.createIdentifier("__super"), "prototype");
                const newNode = factory.updatePropertyAccessExpression(node, expr, node.name);
                newNodes.add(newNode);
                return newNode;
            } else if (
                ts.isElementAccessExpression(node) &&
                node.argumentExpression &&
                node.expression.kind === ts.SyntaxKind.SuperKeyword
            ) {
                const expr = isStatic
                    ? factory.createIdentifier("__super")
                    : factory.createPropertyAccessExpression(factory.createIdentifier("__super"), "prototype");
                const newNode = factory.updateElementAccessExpression(node, expr, node.argumentExpression);
                newNodes.add(newNode);
                return newNode;
            }

            // for all other nodes, recurse first (so we update any usages of 'super')
            // below them
            const rewritten = ts.visitEachChild(node, visitor, transformationContext);

            if (ts.isCallExpression(rewritten) && newNodes.has(rewritten.expression)) {
                // this was a call to super() or super.x() or super["x"]();
                // the super will already have been transformed to __super or
                // __super.prototype.x or __super.prototype["x"].
                //
                // to that, we have to add the .call(this, ...) call.

                const argumentsCopy = rewritten.arguments.slice();
                argumentsCopy.unshift(factory.createThis());

                return factory.updateCallExpression(
                    rewritten,
                    factory.createPropertyAccessExpression(rewritten.expression, "call"),
                    rewritten.typeArguments,
                    argumentsCopy,
                );
            }

            return rewritten;
        }

        return (node: ts.Node) => ts.visitNode(node, visitor);
    }
}
