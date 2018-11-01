// Copyright 2016-2018, Pulumi Corporation.
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

import * as ts from "typescript";
import * as closure from "./createClosure";
import * as utils from "./utils";

export function rewriteSuperReferences(code: string, isStatic: boolean): string {
    const sourceFile = ts.createSourceFile(
        "", code, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);

    // Transform any usages of "super(...)" into "__super.call(this, ...)", any
    // instance usages of "super.xxx" into "__super.prototype.xxx" and any static
    // usages of "super.xxx" into "__super.xxx"
    const transformed = ts.transform(sourceFile, [rewriteSuperCallsWorker]);
    const printer = ts.createPrinter({ newLine: ts.NewLineKind.LineFeed });
    const output = printer.printNode(ts.EmitHint.Unspecified, transformed.transformed[0], sourceFile).trim();

    return output;

    function rewriteSuperCallsWorker(transformationContext: ts.TransformationContext) {
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

                const text = utils.isLegalMemberName(funcDecl.name!.text)
                    ? "/*" + funcDecl.name!.text + "*/" : "";
                return ts.updateFunctionDeclaration(
                    funcDecl,
                    funcDecl.decorators,
                    funcDecl.modifiers,
                    funcDecl.asteriskToken,
                    ts.createIdentifier(text),
                    funcDecl.typeParameters,
                    funcDecl.parameters,
                    funcDecl.type,
                    funcDecl.body);
            }

            if (node.kind === ts.SyntaxKind.SuperKeyword) {
                const newNode = ts.createIdentifier("__super");
                newNodes.add(newNode);
                return newNode;
            }
            else if (ts.isPropertyAccessExpression(node) &&
                        node.expression.kind === ts.SyntaxKind.SuperKeyword) {

                const expr = isStatic
                    ? ts.createIdentifier("__super")
                    : ts.createPropertyAccess(ts.createIdentifier("__super"), "prototype");
                const newNode = ts.updatePropertyAccess(node, expr, node.name);
                newNodes.add(newNode);
                return newNode;
            }
            else if (ts.isElementAccessExpression(node) &&
                        node.argumentExpression &&
                        node.expression.kind === ts.SyntaxKind.SuperKeyword) {

                const expr = isStatic
                    ? ts.createIdentifier("__super")
                    : ts.createPropertyAccess(ts.createIdentifier("__super"), "prototype");

                const newNode = ts.updateElementAccess(
                    node, expr, node.argumentExpression);
                newNodes.add(newNode);
                return newNode;
            }

            // for all other nodes, recurse first (so we update any usages of 'super')
            // below them
            const rewritten = ts.visitEachChild(node, visitor, transformationContext);

            if (ts.isCallExpression(rewritten) &&
                newNodes.has(rewritten.expression)) {

                // this was a call to super() or super.x() or super["x"]();
                // the super will already have been transformed to __super or
                // __super.prototype.x or __super.prototype["x"].
                //
                // to that, we have to add the .call(this, ...) call.

                const argumentsCopy = rewritten.arguments.slice();
                argumentsCopy.unshift(ts.createThis());

                return ts.updateCall(
                    rewritten,
                    ts.createPropertyAccess(rewritten.expression, "call"),
                    rewritten.typeArguments,
                    argumentsCopy);
            }

            return rewritten;
        }

        return (node: ts.Node) => ts.visitNode(node, visitor);
    }
}
