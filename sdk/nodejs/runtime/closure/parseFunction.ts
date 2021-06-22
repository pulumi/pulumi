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
import * as log from "../../log";
import * as utils from "./utils";

/** @internal */
export interface ParsedFunctionCode {
    // The serialized code for the function, usable as an expression. Valid for all functions forms
    // (functions, lambdas, methods, etc.).
    funcExprWithoutName: string;

    // The serialized code for the function, usable as an function-declaration. Valid only for
    // non-lambda function forms.
    funcExprWithName?: string;

    // the name of the function if it was a function-declaration.  This is needed so
    // that we can include an entry in the environment mapping this function name to
    // the actual function we generate for it.  This is needed so that nested recursive calls
    // to the function see the function we're generating.
    functionDeclarationName?: string;

    // Whether or not this was an arrow function.
    isArrowFunction: boolean;
}

/** @internal */
export interface ParsedFunction extends ParsedFunctionCode {
    // The set of variables the function attempts to capture.
    capturedVariables: CapturedVariables;

    // Whether or not the real 'this' (i.e. not a lexically captured this) is used in the function.
    usesNonLexicalThis: boolean;
}

// Information about a captured property.  Both the name and whether or not the property was
// invoked.
/** @internal */
export interface CapturedPropertyInfo {
    name: string;
    invoked: boolean;
}

// Information about a chain of captured properties.  i.e. if you have "foo.bar.baz.quux()", we'll
// say that 'foo' was captured, but that "[bar, baz, quux]" was accessed off of it.  We'll also note
// that 'quux' was invoked.
/** @internal */
export interface CapturedPropertyChain {
    infos: CapturedPropertyInfo[];
}

// A mapping from the names of variables we captured, to information about how those variables were
// used.  For example, if we see "a.b.c()" (and 'a' is not declared in the function), we'll record a
// mapping of { "a": ['b', 'c' (invoked)] }.  i.e. we captured 'a', accessed the properties 'b.c'
// off of it, and we invoked that property access.  With this information we can decide the totality
// of what we need to capture for 'a'.
//
// Note: if we want to capture everything, we just use an empty array for 'CapturedPropertyChain[]'.
// Otherwise, we'll use the chains to determine what portions of the object to serialize.
/** @internal */
export type CapturedVariableMap = Map<string, CapturedPropertyChain[]>;

// The set of variables the function attempts to capture.  There is a required set an an optional
// set. The optional set will not block closure-serialization if we cannot find them, while the
// required set will.  For each variable that is captured we also specify the list of properties of
// that variable we need to serialize.  An empty-list means 'serialize all properties'.
/** @internal */
export interface CapturedVariables {
    required: CapturedVariableMap;
    optional: CapturedVariableMap;
}

// These are the special globals we've marked as ones we do not want to capture by value.
// These values have a dual meaning.  They mean one thing at deployment time and one thing
// at cloud-execution time.  By **not** capturing-by-value we take the view that the user
// wants the cloud-execution time view of things.
const nodeModuleGlobals: {[key: string]: boolean} = {
    "__dirname": true,
    "__filename": true,
    // We definitely should not try to capture/serialize 'require'.  Not only will it bottom
    // out as a native function, but it is definitely something the user intends to run
    // against the right module environment at cloud-execution time and not deployment time.
    "require": true,
};

// Gets the text of the provided function (using .toString()) and massages it so that it is a legal
// function declaration.  Note: this ties us heavily to V8 and its representation for functions.  In
// particular, it has expectations around how functions/lambdas/methods/generators/constructors etc.
// are represented.  If these change, this will likely break us.
/** @internal */
export function parseFunction(funcString: string): [string, ParsedFunction] {
    const [error, functionCode] = parseFunctionCode(funcString);
    if (error) {
        return [error, <any>undefined];
    }

    // In practice it's not guaranteed that a function's toString is parsable by TypeScript.
    // V8 intrinsics are prefixed with a '%' and TypeScript does not consider that to be a valid
    // identifier.
    const [parseError, file] = createSourceFile(functionCode);
    if (parseError) {
        return [parseError, <any>undefined];
    }

    const capturedVariables = computeCapturedVariableNames(file!);

    // if we're looking at an arrow function, the it is always using lexical 'this's
    // so we don't have to bother even examining it.
    const usesNonLexicalThis = !functionCode.isArrowFunction && computeUsesNonLexicalThis(file!);

    const result = <ParsedFunction>functionCode;
    result.capturedVariables = capturedVariables;
    result.usesNonLexicalThis = usesNonLexicalThis;

    if (result.capturedVariables.required.has("this")) {
        return [
            "arrow function captured 'this'. Assign 'this' to another name outside function and capture that.",
            result,
        ];
    }

    return ["", result];
}

function parseFunctionCode(funcString: string): [string, ParsedFunctionCode] {
    if (funcString.startsWith("[Function:")) {
        return [`the function form was not understood.`, <any>undefined];
    }

    // Split this constant out so that if this function *itself* is closure serialized,
    // it will not be thought to be native code itself.
    const nativeCodeString = "[native " + "code]";
    if (funcString.indexOf(nativeCodeString) !== -1) {
        return [`it was a native code function.`, <any>undefined];
    }

    // There are three general forms of node toString'ed Functions we're trying to find out here.
    //
    // 1. `[mods] (...) => ...
    //
    //      i.e. an arrow function.  We need to ensure that arrow-functions stay arrow-functions,
    //      and non-arrow-functions end up looking like normal `function` functions. This will make
    //      it so that we can correctly handle 'this' properly depending on if that should be
    //      treated as the lexical capture of 'this' or the non-lexical 'this'.
    //
    // 2. `class Foo { ... }`
    //
    //      i.e. node uses the entire string of a class when toString'ing the constructor function
    //      for it.
    //
    // 3. `[mods] function ...
    //
    //      i.e. a normal function (maybe async, maybe a get/set function, but def not an arrow
    //      function)

    if (tryParseAsArrowFunction(funcString)) {
        return ["", { funcExprWithoutName: funcString, isArrowFunction: true }];
    }

    // First check to see if this startsWith 'class'.  If so, this is definitely a class.  This
    // works as Node does not place preceding comments on a class/function, allowing us to just
    // directly see if we've started with the right text.
    if (funcString.startsWith("class ")) {
        // class constructor function.  We want to get the actual constructor
        // in the class definition (synthesizing an empty one if one does not)
        // exist.
        const [file, firstDiagnostic] = tryCreateSourceFile(funcString);
        if (firstDiagnostic) {
            return [`the class could not be parsed: ${firstDiagnostic}`, <any>undefined];
        }

        const classDecl = <ts.ClassDeclaration>file!.statements.find(x => ts.isClassDeclaration(x));
        if (!classDecl) {
            return [`the class form was not understood:\n${funcString}`, <any>undefined];
        }

        const constructor = <ts.ConstructorDeclaration>classDecl.members.find(m => ts.isConstructorDeclaration(m));
        if (!constructor) {
            // class without explicit constructor.
            const isSubClass = classDecl.heritageClauses && classDecl.heritageClauses.some(
                c => c.token === ts.SyntaxKind.ExtendsKeyword);
            return isSubClass
                ? makeFunctionDeclaration("constructor() { super(); }", /*isAsync:*/ false, /*isFunctionDeclaration:*/ false)
                : makeFunctionDeclaration("constructor() { }", /*isAsync:*/ false, /*isFunctionDeclaration:*/ false);
        }

        const constructorCode = funcString.substring(constructor.getStart(file, /*includeJsDocComment*/ false), constructor.end).trim();
        return makeFunctionDeclaration(constructorCode, /*isAsync:*/ false, /*isFunctionDeclaration: */ false);
    }

    let isAsync = false;
    if (funcString.startsWith("async ")) {
        isAsync = true;
        funcString = funcString.substr("async".length).trimLeft();
    }

    if (funcString.startsWith("function get ") || funcString.startsWith("function set ")) {
        const trimmed = funcString.substr("function get".length);
        return makeFunctionDeclaration(trimmed, isAsync, /*isFunctionDeclaration: */ false);
    }

    if (funcString.startsWith("get ") || funcString.startsWith("set ")) {
        const trimmed = funcString.substr("get ".length);
        return makeFunctionDeclaration(trimmed, isAsync, /*isFunctionDeclaration: */ false);
    }

    if (funcString.startsWith("function")) {
        const trimmed = funcString.substr("function".length);
        return makeFunctionDeclaration(trimmed, isAsync, /*isFunctionDeclaration: */ true);
    }

    // Add "function" (this will make methods parseable).  i.e.  "foo() { }" becomes
    // "function foo() { }"
    // this also does the right thing for functions with computed names.
    return makeFunctionDeclaration(funcString, isAsync, /*isFunctionDeclaration: */ false);
}

function tryParseAsArrowFunction(toParse: string): boolean {
    const [file] = tryCreateSourceFile(toParse);
    if (!file || file.statements.length !== 1) {
        return false;
    }

    const firstStatement = file.statements[0];
    return ts.isExpressionStatement(firstStatement) &&
           ts.isArrowFunction(firstStatement.expression);
}

function makeFunctionDeclaration(
        v: string, isAsync: boolean, isFunctionDeclaration: boolean): [string, ParsedFunctionCode] {

    let prefix = isAsync ? "async " : "";
    prefix += "function ";

    v = v.trimLeft();

    if (v.startsWith("*")) {
        v = v.substr(1).trimLeft();
        prefix = "function* ";
    }

    const openParenIndex = v.indexOf("(");
    if (openParenIndex < 0) {
        return [`the function form was not understood.`, <any>undefined];
    }

    if (isComputed(v, openParenIndex)) {
        v = v.substr(openParenIndex);
        return ["", {
            funcExprWithoutName: prefix + v,
            funcExprWithName: prefix + "__computed" + v,
            functionDeclarationName: undefined,
            isArrowFunction: false,
        }];
    }

    const nameChunk = v.substr(0, openParenIndex);
    const funcName = utils.isLegalMemberName(nameChunk)
        ? utils.isLegalFunctionName(nameChunk) ? nameChunk : "/*" + nameChunk + "*/"
        : "";
    const commentedName = utils.isLegalMemberName(nameChunk) ? "/*" + nameChunk + "*/" : "";
    v = v.substr(openParenIndex).trimLeft();

    return ["", {
        funcExprWithoutName: prefix + commentedName + v,
        funcExprWithName: prefix + funcName + v,
        functionDeclarationName: isFunctionDeclaration ? nameChunk : undefined,
        isArrowFunction: false,
    }];
}

function isComputed(v: string, openParenIndex: number) {
    if (openParenIndex === 0) {
        // node 8 and lower use no name at all for computed members.
        return true;
    }

    if (v.length > 0 && v.charAt(0) === "[") {
        // node 10 actually has the name as: [expr]
        return true;
    }

    return false;
}

function createSourceFile(serializedFunction: ParsedFunctionCode): [string, ts.SourceFile | null] {
    const funcstr = serializedFunction.funcExprWithName || serializedFunction.funcExprWithoutName;

    // Wrap with parens to make into something parseable.  This is necessary as many
    // types of functions are valid function expressions, but not valid function
    // declarations.  i.e.   "function () { }".  This is not a valid function declaration
    // (it's missing a name).  But it's totally legal as "(function () { })".
    const toParse = "(" + funcstr + ")";

    const [file, firstDiagnostic] = tryCreateSourceFile(toParse);
    if (firstDiagnostic) {
        return [`the function could not be parsed: ${firstDiagnostic}`, null];
    }

    return ["", file!];
}

function tryCreateSourceFile(toParse: string): [ts.SourceFile | undefined, string | undefined] {
    const file = ts.createSourceFile(
        "", toParse, ts.ScriptTarget.Latest, /*setParentNodes:*/ true, ts.ScriptKind.TS);

    const diagnostics: ts.Diagnostic[] = (<any>file).parseDiagnostics;
    if (diagnostics.length) {
        return [undefined, `${diagnostics[0].messageText}`];
    }

    return [file, undefined];
}

function computeUsesNonLexicalThis(file: ts.SourceFile): boolean {
    let inTopmostFunction = false;
    let usesNonLexicalThis = false;

    ts.forEachChild(file, walk);

    return usesNonLexicalThis;

    function walk(node: ts.Node | undefined) {
        if (!node) {
            return;
        }

        switch (node.kind) {
            case ts.SyntaxKind.SuperKeyword:
            case ts.SyntaxKind.ThisKeyword:
                usesNonLexicalThis = true;
                break;

            case ts.SyntaxKind.CallExpression:
                return visitCallExpression(<ts.CallExpression>node);

            case ts.SyntaxKind.MethodDeclaration:
            case ts.SyntaxKind.FunctionDeclaration:
            case ts.SyntaxKind.FunctionExpression:
                return visitBaseFunction(<ts.FunctionLikeDeclarationBase>node);

            // Note: it is intentional that we ignore ArrowFunction.  If we use 'this' inside of it,
            // then that should be considered a use of the non-lexical-this from an outer function.
            // i.e.
            //          function f() { var v = () => console.log(this) }
            //
            // case ts.SyntaxKind.ArrowFunction:
            default:
                break;
        }

        ts.forEachChild(node, walk);
    }

    function visitBaseFunction(node: ts.FunctionLikeDeclarationBase): void {
        if (inTopmostFunction) {
            // we're already in the topmost function.  No need to descend into any
            // further functions.
            return;
        }

        // Entering the topmost function.
        inTopmostFunction = true;

        // Now, visit its body to see if we use 'this/super'.
        walk(node.body);

        inTopmostFunction = false;
    }

    function visitCallExpression(node: ts.CallExpression) {
        // Most call expressions are normal.  But we must special case one kind of function:
        // TypeScript's __awaiter functions.  They are of the form `__awaiter(this, void 0, void 0,
        // function* (){})`,

        // The first 'this' argument is passed along in case the expression awaited uses 'this'.
        // However, doing that can be very bad for us as in many cases the 'this' just refers to the
        // surrounding module, and the awaited expression won't be using that 'this' at all.
        walk(node.expression);

        if (isAwaiterCall(node)) {
            const lastFunction = <ts.FunctionExpression>node.arguments[3];
            walk(lastFunction.body);
            return;
        }

        // For normal calls, just walk all arguments normally.
        for (const arg of node.arguments) {
            walk(arg);
        }
    }
}

/**
 * computeCapturedVariableNames computes the set of free variables in a given function string.  Note that this string is
 * expected to be the usual V8-serialized function expression text.
 */
function computeCapturedVariableNames(file: ts.SourceFile): CapturedVariables {
    // Now that we've parsed the file, compute the free variables, and return them.

    let required: CapturedVariableMap = new Map();
    let optional: CapturedVariableMap = new Map();
    const scopes: Set<string>[] = [];
    let functionVars: Set<string> = new Set();

    // Recurse through the tree.  We use typescript's AST here and generally walk the entire
    // tree. One subtlety to be aware of is that we generally assume that when we hit an
    // identifier that it either introduces a new variable, or it lexically references a
    // variable.  This clearly doesn't make sense for *all* identifiers.  For example, if you
    // have "console.log" then "console" tries to lexically reference a variable, but "log" does
    // not.  So, to avoid that being an issue, we carefully decide when to recurse.  For
    // example, for member access expressions (i.e. A.B) we do not recurse down the right side.

    ts.forEachChild(file, walk);

    // Now just return all variables whose value is true.  Filter out any that are part of the built-in
    // Node.js global object, however, since those are implicitly availble on the other side of serialization.
    const result: CapturedVariables = { required: new Map(), optional: new Map() };

    for (const key of required.keys()) {
        if (!isBuiltIn(key)) {
            result.required.set(key, required.get(key)!.concat(
                optional.has(key) ? optional.get(key)! : []));
        }
    }

    for (const key of optional.keys()) {
        if (!isBuiltIn(key) && !required.has(key)) {
            result.optional.set(key, optional.get(key)!);
        }
    }

    log.debug(`Found free variables: ${JSON.stringify(result)}`);
    return result;

    function isBuiltIn(ident: string): boolean {
        // __awaiter and __rest are never considered built-in.  We do this as async/await code will generate
        // an __awaiter (so we will need it), but some libraries (like tslib) will add this to the 'global'
        // object.  The same is true for __rest when destructuring.
        // If we think these are built-in, we won't serialize them, and the functions may not
        // actually be available if the import that caused it to get attached isn't included in the
        // final serialized code.
        if (ident === "__awaiter" || ident === "__rest") {
            return false;
        }

        // Anything in the global dictionary is a built-in.  So is anything that's a global Node.js object;
        // note that these only exist in the scope of modules, and so are not truly global in the usual sense.
        // See https://nodejs.org/api/globals.html for more details.
        return global.hasOwnProperty(ident) || nodeModuleGlobals[ident];
    }

    function currentScope(): Set<string> {
        return scopes[scopes.length - 1];
    }

    function visitIdentifier(node: ts.Identifier): void {
        // Remember undeclared identifiers during the walk, as they are possibly free.
        const name = node.text;
        for (let i = scopes.length - 1; i >= 0; i--) {
            if (scopes[i].has(name)) {
                // This is currently known in the scope chain, so do not add it as free.
                return;
            }
        }

        // We reached the top of the scope chain and this wasn't found; it's captured.
        const capturedPropertyChain = determineCapturedPropertyChain(node);
        if (node.parent!.kind === ts.SyntaxKind.TypeOfExpression) {
            // "typeof undeclared_id" is legal in JS (and is actually used in libraries). So keep
            // track that we would like to capture this variable, but mark that capture as optional
            // so we will not throw if we aren't able to find it in scope.
            optional.set(name, combineProperties(optional.get(name), capturedPropertyChain));
        } else {
            required.set(name, combineProperties(required.get(name), capturedPropertyChain));
        }
    }

    function walk(node: ts.Node | undefined) {
        if (!node) {
            return;
        }

        switch (node.kind) {
            case ts.SyntaxKind.Identifier:
                return visitIdentifier(<ts.Identifier>node);
            case ts.SyntaxKind.ThisKeyword:
                return visitThisExpression(<ts.ThisExpression>node);
            case ts.SyntaxKind.Block:
                return visitBlockStatement(<ts.Block>node);
            case ts.SyntaxKind.CallExpression:
                return visitCallExpression(<ts.CallExpression>node);
            case ts.SyntaxKind.CatchClause:
                return visitCatchClause(<ts.CatchClause>node);
            case ts.SyntaxKind.MethodDeclaration:
                return visitMethodDeclaration(<ts.MethodDeclaration>node);
            case ts.SyntaxKind.MetaProperty:
                // don't walk down an es6 metaproperty (i.e. "new.target").  It doesn't
                // capture anything.
                return;
            case ts.SyntaxKind.PropertyAssignment:
                return visitPropertyAssignment(<ts.PropertyAssignment>node);
            case ts.SyntaxKind.PropertyAccessExpression:
                return visitPropertyAccessExpression(<ts.PropertyAccessExpression>node);
            case ts.SyntaxKind.FunctionDeclaration:
            case ts.SyntaxKind.FunctionExpression:
                return visitFunctionDeclarationOrExpression(<ts.FunctionDeclaration>node);
            case ts.SyntaxKind.ArrowFunction:
                return visitBaseFunction(<ts.ArrowFunction>node, /*isArrowFunction:*/true, /*name:*/ undefined);
            case ts.SyntaxKind.VariableDeclaration:
                return visitVariableDeclaration(<ts.VariableDeclaration>node);
            default:
                break;
        }

        ts.forEachChild(node, walk);
    }

    function visitThisExpression(node: ts.ThisExpression): void {
        required.set(
            "this", combineProperties(required.get("this"), determineCapturedPropertyChain(node)));
    }

    function combineProperties(existing: CapturedPropertyChain[] | undefined,
                               current: CapturedPropertyChain | undefined) {
        if (existing && existing.length === 0) {
            // We already want to capture everything.  Keep things that way.
            return existing;
        }

        if (current === undefined) {
            // We want to capture everything.  So ignore any properties we've filtered down
            // to and just capture them all.
            return [];
        }

        // We want to capture a specific set of properties.  Add this set of properties
        // into the existing set.
        const combined = existing || [];
        combined.push(current);

        return combined;
    }

    // Finds nodes of the form `(...expr...).PropName` or `(...expr...)["PropName"]`
    // For element access expressions, the argument must be a string literal.
    function isPropertyOrElementAccessExpression(node: ts.Node): node is (ts.PropertyAccessExpression | ts.ElementAccessExpression) {
        if (ts.isPropertyAccessExpression(node)) {
            return true;
        }

        if (ts.isElementAccessExpression(node) && ts.isStringLiteral(node.argumentExpression)) {
            return true;
        }

        return false;
    }

    function determineCapturedPropertyChain(node: ts.Node): CapturedPropertyChain | undefined {
        let infos: CapturedPropertyInfo[] | undefined;

        // Walk up a sequence of property-access'es, recording the names we hit, until we hit
        // something that isn't a property-access.
        while (node &&
               node.parent &&
               isPropertyOrElementAccessExpression(node.parent) &&
               node.parent.expression === node) {

            if (!infos) {
                infos = [];
            }

            const propOrElementAccess = node.parent;

            const name = ts.isPropertyAccessExpression(propOrElementAccess)
                ? propOrElementAccess.name.text
                : (<ts.StringLiteral>propOrElementAccess.argumentExpression).text;

            const invoked = propOrElementAccess.parent !== undefined &&
                            ts.isCallExpression(propOrElementAccess.parent) &&
                            propOrElementAccess.parent.expression === propOrElementAccess;

            // Keep track if this name was invoked.  If so, we'll have to analyze it later
            // to see if it captured 'this'
            infos.push({ name, invoked });
            node = propOrElementAccess;
        }

        if (infos) {
            // Invariant checking.
            if (infos.length === 0) {
                throw new Error("How did we end up with an empty list?");
            }

            for (let i = 0; i < infos.length - 1; i++) {
                if (infos[i].invoked) {
                    throw new Error("Only the last item in the dotted chain is allowed to be invoked.");
                }
            }

            return { infos };
        }

        // For all other cases, capture everything.
        return undefined;
    }

    function visitBlockStatement(node: ts.Block): void {
        // Push new scope, visit all block statements, and then restore the scope.
        scopes.push(new Set());
        ts.forEachChild(node, walk);
        scopes.pop();
    }

    function visitFunctionDeclarationOrExpression(
            node: ts.FunctionDeclaration | ts.FunctionExpression): void {
        // A function declaration is special in one way: its identifier is added to the current function's
        // var-style variables, so that its name is in scope no matter the order of surrounding references to it.

        if (node.name) {
            functionVars.add(node.name.text);
        }

        visitBaseFunction(node, /*isArrowFunction:*/false, node.name);
    }

    function visitBaseFunction(
            node: ts.FunctionLikeDeclarationBase,
            isArrowFunction: boolean,
            functionName: ts.Identifier | undefined): void {
        // First, push new free vars list, scope, and function vars
        const savedRequired = required;
        const savedOptional = optional;
        const savedFunctionVars = functionVars;

        required = new Map();
        optional = new Map();
        functionVars = new Set();
        scopes.push(new Set());

        // If this is a named function, it's name is in scope at the top level of itself.
        if (functionName) {
            functionVars.add(functionName.text);
        }

        // this/arguments are in scope inside any non-arrow function.
        if (!isArrowFunction) {
            functionVars.add("this");
            functionVars.add("arguments");
        }

        // The parameters of any function are in scope at the top level of the function.
        for (const param of node.parameters) {
            nameWalk(param.name, /*isVar:*/ true);

            // Parse default argument expressions
            if (param.initializer) {
                walk(param.initializer);
            }
        }

        // Next, visit the body underneath this new context.
        walk(node.body);

        // Remove any function-scoped variables that we encountered during the walk.
        for (const v of functionVars) {
            required.delete(v);
            optional.delete(v);
        }

        // Restore the prior context and merge our free list with the previous one.
        scopes.pop();

        mergeMaps(savedRequired, required);
        mergeMaps(savedOptional, optional);

        functionVars = savedFunctionVars;
        required = savedRequired;
        optional = savedOptional;
    }

    function mergeMaps(target: CapturedVariableMap, source: CapturedVariableMap) {
        for (const key of source.keys()) {
            const sourcePropInfos = source.get(key)!;
            let targetPropInfos = target.get(key)!;

            if (sourcePropInfos.length === 0) {
                // we want to capture everything.  Make sure that's reflected in the target.
                targetPropInfos = [];
            }
            else {
                // we want to capture a subet of properties.  merge that subset into whatever
                // subset we've recorded so far.
                for (const sourceInfo of sourcePropInfos) {
                    targetPropInfos = combineProperties(targetPropInfos, sourceInfo);
                }
            }

            target.set(key, targetPropInfos);
        }
    }

    function visitCatchClause(node: ts.CatchClause): void {
        scopes.push(new Set());

        // Add the catch pattern to the scope as a variable.  Note that it is scoped to our current
        // fresh scope (so it can't be seen by the rest of the function).
        if (node.variableDeclaration) {
            nameWalk(node.variableDeclaration.name, /*isVar:*/ false);
        }

        // And then visit the block without adding them as free variables.
        walk(node.block);

        // Relinquish the scope so the error patterns aren't available beyond the catch.
        scopes.pop();
    }

    function visitCallExpression(node: ts.CallExpression): void {
        // Most call expressions are normal.  But we must special case one kind of function:
        // TypeScript's __awaiter functions.  They are of the form `__awaiter(this, void 0, void 0, function* (){})`,

        // The first 'this' argument is passed along in case the expression awaited uses 'this'.
        // However, doing that can be very bad for us as in many cases the 'this' just refers to the
        // surrounding module, and the awaited expression won't be using that 'this' at all.
        //
        // However, there are cases where 'this' may be legitimately lexically used in the awaited
        // expression and should be captured properly.  We'll figure this out by actually descending
        // explicitly into the "function*(){}" argument, asking it to be treated as if it was
        // actually a lambda and not a JS function (with the standard js 'this' semantics).  By
        // doing this, if 'this' is used inside the function* we'll act as if it's a real lexical
        // capture so that we pass 'this' along.
        walk(node.expression);

        if (isAwaiterCall(node)) {
            return visitBaseFunction(
                <ts.FunctionLikeDeclarationBase><ts.FunctionExpression>node.arguments[3],
                /*isArrowFunction*/ true,
                /*name*/ undefined);
        }

        // For normal calls, just walk all arguments normally.
        for (const arg of node.arguments) {
            walk(arg);
        }
    }

    function visitMethodDeclaration(node: ts.MethodDeclaration): void {
        if (ts.isComputedPropertyName(node.name)) {
            // Don't walk down the 'name' part of the property assignment if it is an identifier. It
            // does not capture any variables.  However, if it is a computed property name, walk it
            // as it may capture variables.
            walk(node.name);
        }

        // Always walk the method.  Pass 'undefined' for the name as a method's name is not in scope
        // inside itself.
        visitBaseFunction(node, /*isArrowFunction:*/ false, /*name:*/ undefined);
    }

    function visitPropertyAssignment(node: ts.PropertyAssignment): void {
        if (ts.isComputedPropertyName(node.name)) {
            // Don't walk down the 'name' part of the property assignment if it is an identifier. It
            // is not capturing any variables.  However, if it is a computed property name, walk it
            // as it may capture variables.
            walk(node.name);
        }

        // Always walk the property initializer.
        walk(node.initializer);
    }

    function visitPropertyAccessExpression(node: ts.PropertyAccessExpression): void {
        // Don't walk down the 'name' part of the property access.  It could not capture a free variable.
        // i.e. if you have "A.B", we should analyze the "A" part and not the "B" part.
        walk(node.expression);
    }

    function nameWalk(n: ts.BindingName | undefined, isVar: boolean): void {
        if (!n) {
            return;
        }

        switch (n.kind) {
            case ts.SyntaxKind.Identifier:
                return visitVariableDeclarationIdentifier(<ts.Identifier>n, isVar);
            case ts.SyntaxKind.ObjectBindingPattern:
            case ts.SyntaxKind.ArrayBindingPattern:
                const bindingPattern = <ts.BindingPattern>n;
                for (const element of bindingPattern.elements) {
                    if (ts.isBindingElement(element)) {
                        visitBindingElement(element, isVar);
                    }
                }

                return;
            default:
                return;
        }
    }

    function visitVariableDeclaration(node: ts.VariableDeclaration): void {
        // tslint:disable-next-line:max-line-length
        const isLet = node.parent !== undefined && ts.isVariableDeclarationList(node.parent) && (node.parent.flags & ts.NodeFlags.Let) !== 0;
        // tslint:disable-next-line:max-line-length
        const isConst = node.parent !== undefined && ts.isVariableDeclarationList(node.parent) && (node.parent.flags & ts.NodeFlags.Const) !== 0;
        const isVar = !isLet && !isConst;

        // Walk the declaration's `name` property (which may be an Identifier or Pattern) placing
        // any variables we encounter into the right scope.
        nameWalk(node.name, isVar);

        // Also walk into the variable initializer with the original walker to make sure we see any
        // captures on the right hand side.
        walk(node.initializer);
    }

    function visitVariableDeclarationIdentifier(node: ts.Identifier, isVar: boolean): void {
        // If the declaration is an identifier, it isn't a free variable, for whatever scope it
        // pertains to (function-wide for var and scope-wide for let/const).  Track it so we can
        // remove any subseqeunt references to that variable, so we know it isn't free.
        if (isVar) {
            functionVars.add(node.text);
        } else {
            currentScope().add(node.text);
        }
    }

    function visitBindingElement(node: ts.BindingElement, isVar: boolean): void {
        // array and object patterns can be quite complex.  You can have:
        //
        //  var {t} = val;          // lookup a property in 'val' called 't' and place into a variable 't'.
        //  var {t: m} = val;       // lookup a property in 'val' called 't' and place into a variable 'm'.
        //  var {t: <pat>} = val;   // lookup a property in 'val' called 't' and decompose further into the pattern.
        //
        // And, for all of the above, you can have:
        //
        //  var {t = def} = val;
        //  var {t: m = def} = val;
        //  var {t: <pat> = def} = val;
        //
        // These are the same as the above, except that if there is no property 't' in 'val',
        // then the default value will be used.
        //
        // You can also have at the end of the literal: { ...rest}

        // Walk the name portion, looking for names to add.  for
        //
        //       var {t}   // this will be 't'.
        //
        // for
        //
        //      var {t: m} // this will be 'm'
        //
        // and for
        //
        //      var {t: <pat>} // this will recurse into the pattern.
        //
        // and for
        //
        //      ...rest // this will be 'rest'
        nameWalk(node.name, isVar);

        // if there is a default value, walk it as well, looking for captures.
        walk(node.initializer);

        // importantly, we do not walk into node.propertyName
        // This Name defines what property will be retrieved from the value being pattern
        // matched against.  Importantly, it does not define a new name put into scope,
        // nor does it reference a variable in scope.
    }
}

function isAwaiterCall(node: ts.CallExpression) {
    const result =
        ts.isIdentifier(node.expression) &&
        node.expression.text === "__awaiter" &&
        node.arguments.length === 4 &&
        node.arguments[0].kind === ts.SyntaxKind.ThisKeyword &&
        ts.isFunctionLike(node.arguments[3]);

    return result;
}
