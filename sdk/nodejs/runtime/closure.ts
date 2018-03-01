// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as crypto from "crypto";
import { relative as pathRelative } from "path";
import * as ts from "typescript";
import { RunError } from "../errors";
import * as log from "../log";
import * as resource from "../resource";
import { debuggablePromise } from "./debuggable";

// Our closure serialization code links against v8 internals. On Windows,
// we can't dynamically link against v8 internals because their symbols are
// unexported. In order to address this problem, Pulumi programs run on a
// custom build of Node.
//
// On Linux and OSX, we can dynamically link against v8 internals, so we can run
// on stock Node. However, we only build nativeruntime.node against specific versions
// of Node, users running Pulumi programs must explicitly use a supported version
// of Node.
const supportedNodeVersions = ["v6.10.2"];
let nativeruntime: any;
try {
    nativeruntime = require("nativeruntime-v0.11.0.node");
}
catch (err) {
    // There are two reasons why this can happen:
    //   1. We messed up when packaging Pulumi and failed to include nativeruntime.node,
    //   2. A user is running their Pulumi program with a version of Node that we do not explicitly support.
    const thisNodeVersion = process.version;
    if (supportedNodeVersions.indexOf(thisNodeVersion) > -1) {
        // This node version is explicitly supported, but the load still failed.
        // This means that Pulumi messed up when installing itself.
        throw new RunError(`Failed to locate custom Pulumi SDK Node.js extension. This is a bug! (${err.message})`);
    }

    throw new RunError(
        `Failed to load custom Pulumi SDK Node.js extension; The version of Node.js that you are
         using (${thisNodeVersion}) is not explicitly supported, you must use one of these
         supported versions of Node.js: ${supportedNodeVersions}`);
}

/**
 * Closure represents the serialized form of a JavaScript serverless function.
 */
interface Closure {
    code: string;             // a serialization of the function's source code as text.
    runtime: string;          // the language runtime required to execute the serialized code.
    environment: Environment; // the captured lexical environment of variables to values, if any.
    obj: ObjectEntry;         // The object-side of the function.  i.e. it's proto, properties, symbols (if any)
}

// Similar to PropertyDescriptor.  Helps describe an EnvironmentEntry in the case where it is not
// simple.
interface EntryDescriptor {
    // If the property has a value we should directly provide when calling .defineProperty
    hasValue?: boolean;

    // same as PropertyDescriptor
    configurable?: boolean;
    enumerable?: boolean;
    writable?: boolean;

    // The entries we've made for custom getters/setters if the property is defined that
    // way.
    get?: EnvironmentEntry;
    set?: EnvironmentEntry;
}

// Information about a property.  Specifically the actual entry containing the data about it and
// then an optional descriptor in the case that this isn't just a common property.
type EnvironmentEntryAndDescriptor = { descriptor?: EntryDescriptor, entry: EnvironmentEntry };

/**
 * Environment is the captured lexical environment for a closure.
 */
type Environment = Map<EnvironmentEntry, EnvironmentEntryAndDescriptor>;

type ObjectEntry = {
    // information about the prototype of this object.  only stored if the prototype is
    // not Object.prototype.
    proto?: EnvironmentEntry,

    // information about the normal string-named properties of the object.
    env: Environment,
};

/**
 * EnvironmentEntry is the environment slot for a named lexically captured variable.
 */
interface EnvironmentEntry {
    // a value which can be safely json serialized.
    json?: any;

    // a closure we are dependent on.
    closure?: Closure;

    // An object which may contain nested closures.
    // Can include an optional proto if the user is not using the default Object.prototype.
    obj?: ObjectEntry;

    // an array which may contain nested closures.
    arr?: EnvironmentEntry[];

    // a reference to a requirable module name.
    module?: string;

    // a Dependency<T> property.  It will be serialized over as a get() method that
    // returns the raw underlying value.
    dep?: EnvironmentEntry;

    // a simple expression to use to represent this instance.  For example "global.Number";
    expr?: string;

    // A promise value.  this will be serialized as the underlyign value the promise
    // points to.  And deserialized as Promise.resolve(<underlying_value>)
    promise?: EnvironmentEntry;
}

export async function serializeFunctionAsync(func: Function, doNotSerialize?: Set<any>): Promise<string> {
    doNotSerialize = doNotSerialize || new Set();
    const closure = await serializeClosureAsync(func, doNotSerialize);
    return serializeJavaScriptTextAsync(closure);
}

/**
 * serializeClosureAsync serializes a function and its closure environment into a form that is
 * amenable to persistence as simple JSON.  Like toString, it includes the full text of the
 * function's source code, suitable for execution. Unlike toString, it actually includes information
 * about the captured environment.
 */
async function serializeClosureAsync(func: Function, doNotSerialize: Set<any>): Promise<Closure> {
    // entryCache stores a map of objects to the entries we've created for them.  It's
    // used so that we only ever create a single environemnt entry for a single object.
    // i.e. if we hit the same object multiple times while walking the memory graph,
    // we only emit it once.
    const entryCache = new Map<Object, EnvironmentEntry>();

    // Add well-known javascript global variables into our cache.  This way, if there
    // is any code that references them, we can just emit that as simple expressions
    // (like "new Array"), instead of trying to actually serialize out these core types.

    // Front load these guys so we prefer emitting code that references them directly,
    // instead of in unexpected ways.  i.e. we'd prefer to have Number.prototype vs
    // Object.getPrototypeOf(Infinity) (even though they're the same thing.)

    addGlobalInfo("Object");
    addGlobalInfo("Function");
    addGlobalInfo("Array");
    addGlobalInfo("Number");
    addGlobalInfo("String");

    for (let current = global; current; current = Object.getPrototypeOf(current)) {
        for (const key of Object.getOwnPropertyNames(current)) {
            // "GLOBAL" and "root" are deprecated and give warnings if you try to access them.  So
            // just skip them.
            if (key !== "GLOBAL" && key !== "root") {
                if ((<any>global)[key]) {
                    addGlobalInfo(key);
                }
            }
        }
    }

    // Add information so that we can properly serialize over generators/iterators.
    addGeneratorEntries();
    entryCache.set(Symbol.iterator, { expr: "Symbol.iterator" });

    // Make sure this func is in the cache itself as we may hit it again while recursing.
    const entry: EnvironmentEntry = {};
    entryCache.set(func, entry);

    entry.closure = await serializeFunctionRecursiveAsync(func, entryCache, doNotSerialize);

    return entry.closure;

    function addGlobalInfo(key: string) {
        const globalObj = (<any>global)[key];
        const text = isLegalName(key) ? `global.${key}` :  `global["${key}"]`;

        if (!entryCache.has(globalObj)) {
            entryCache.set(globalObj, { expr: text });
        }

        const proto1 = Object.getPrototypeOf(globalObj);
        if (proto1 && !entryCache.has(proto1)) {
            entryCache.set(proto1, { expr: `Object.getPrototypeOf(${text}})`});
        }

        const proto2 = globalObj.prototype;
        if (proto2 && !entryCache.has(proto2)) {
            entryCache.set(proto2, { expr: `${text}.prototype`});
        }
    }

    // A generator function ('f') has ends up creating two interesting objects in the js
    // environment:
    //
    // 1. the generator function itself ('f').  This generator function has an __proto__ that is
    //    shared will all other generator functions.
    //
    // 2. a property 'prototype' on 'f'.  This property's __proto__ will be shared will all other
    //    'prototype' properties of other generator functions.
    //
    // So, to properly serialize a generator, we stash these special objects away so that we can
    // refer to the well known instance on the other side when we desirialize. Otherwise, if we
    // actually tried to deserialize the instances/prototypes we have we would end up failing when
    // we hit native functions.
    //
    // see http://www.ecma-international.org/ecma-262/6.0/#sec-generatorfunction-objects and
    // http://www.ecma-international.org/ecma-262/6.0/figure-2.png
    function addGeneratorEntries() {
        // tslint:disable-next-line:no-empty
        const emptyGenerator = function*(): any {};

        entryCache.set(Object.getPrototypeOf(emptyGenerator),
            { expr: "Object.getPrototypeOf(function*(){})" });

        entryCache.set(Object.getPrototypeOf(emptyGenerator.prototype),
            { expr: "Object.getPrototypeOf((function*(){}).prototype)" });
    }
}

// A mapping from a class method/constructor to the environment entry corresponding to
// the __super value.  When we emit the code for any class member we will end up adding
//
//  with ( { __super: <...> })
//
// We will also rewrite usages of "super" in the methods to refer to __super.  This way
// we can accurately serialize out the class members, while preserving functionality.
const classInstanceMemberToSuperEntry = new Map<Function, EnvironmentEntryAndDescriptor>();
const classStaticMemberToSuperEntry = new Map<Function, EnvironmentEntryAndDescriptor>();

/**
 * serializeClosureAsync does the work to create an asynchronous dataflow graph that resolves to a
 * final closure.
 */
async function serializeFunctionRecursiveAsync(
        func: Function, entryCache: Map<Object, EnvironmentEntry>, doNotSerialize: Set<any>): Promise<Closure> {

    const funcEntry = entryCache.get(func);
    if (!funcEntry) {
        throw new Error("EnvironmentEntry for this this function was not created by caller");
    }

    // First, convert the js func object to a reasonable stringified version that we can operate on.
    // Importantly, this function helps massage all the different forms that V8 can produce to
    // either a "function (...) { ... }" form, or a "(...) => ..." form.  In other words, all
    // 'funky' functions (like classes and whatnot) will be transformed to reasonable forms we can
    // process down the pipeline.
    const serializedFunction = serializeFunctionCode(func);

    const funcExprWithoutName = serializedFunction.funcExprWithoutName;
    const funcExprWithName = serializedFunction.funcExprWithName;
    const functionDeclarationName = serializedFunction.functionDeclarationName;

    const freeVariableNames = computeCapturedVariableNames(funcExprWithName || funcExprWithoutName);
    const freeVariableValues: any[] = [];

    for (const name of freeVariableNames) {
        freeVariableValues.push(nativeruntime.lookupCapturedVariableValue(func, name));
    }

    const closure: Closure = {
        code: funcExprWithoutName,
        runtime: "nodejs",
        environment: await convertFreeVariablesAsync(),
        obj: { env: new Map() },
    };

    const proto = Object.getPrototypeOf(func);

    const isDerivedClassConstructor = func.toString().startsWith("class ") &&
        proto !== Function.prototype;

    // Ensure that the prototype of this function is properly serialized as well. We only need to do
    // this for functions with a custom prototype (like a derived class constructor, or a functoin
    // that a user has explicit set the prototype for). Normal functions will pick up
    // Function.prototype by default, so we don't need to do anything for them.
    if (proto !== Function.prototype) {
        const protoEntry = await serializeObjectAsync(proto, entryCache, doNotSerialize);
        closure.obj.proto = protoEntry;

        if (isDerivedClassConstructor) {
            processDerivedClassConstructor(protoEntry);
            closure.code = rewriteSuperReferences(funcExprWithName!, /*isStatic*/ false);
        }
    }

    // capture any properties placed on the function itself.  Don't bother with
    // "length/name" as those are not things we can actually change.
    for (const keyOrSymbol of getOwnPropertyNamesAndSymbols(func)) {
        if (keyOrSymbol === "length" || keyOrSymbol === "name") {
            continue;
        }

        const funcProp = (<any>func)[keyOrSymbol];

        // We don't need to emit code to serialize this function's .prototype object
        // unless that .prototype object was actually changed.
        //
        // In other words, in general, we will not emit the prototype for a normal
        // 'function foo() {}' declaration.  but we will emit the prototype for the
        // constructor function of a class.
        if (keyOrSymbol === "prototype" && isDefaultFunctionPrototype(func, funcProp)) {
            continue;
        }

        closure.obj.env.set(
            await serializeObjectAsync(keyOrSymbol, entryCache, doNotSerialize),
            { entry: await serializeObjectAsync(funcProp, entryCache, doNotSerialize) });
    }

    const superEntry = classInstanceMemberToSuperEntry.get(func) || classStaticMemberToSuperEntry.get(func);
    if (superEntry) {
        // this was a class constructor or method.  We need to put a special __super
        // entry into scope, and then rewrite any calls to super() to refer to it.
        closure.environment.set(
            await serializeObjectAsync("__super", entryCache, doNotSerialize),
            superEntry);

        closure.code = rewriteSuperReferences(
            funcExprWithName!, classStaticMemberToSuperEntry.has(func));
    }

    // If this was a named function (literally, only a named function-expr or function-decl), then
    // place an entry in the environment that maps from this function name to the serialized
    // function we're creating.  This ensures that recursive functions will call the right method.
    // i.e if we have "function f() { f(); }" this will get rewritten to:
    //
    //      function __f() {
    //          with ({ f: __f }) {
    //              return function () { f(); }
    //
    // i.e. the inner call to "f();" will actually call the *outer* __f function, and not
    // itself.
    if (functionDeclarationName !== undefined) {
        closure.environment.set(
            await serializeObjectAsync(functionDeclarationName, entryCache, doNotSerialize),
            { entry: funcEntry });
    }

    return closure;

    async function convertFreeVariablesAsync(): Promise<Environment> {
        const env: Environment = new Map();

        for (let i = 0, n = freeVariableNames.length; i < n; i++) {
            const name = freeVariableNames[i];
            const value = freeVariableValues[i];

            const serializedName = await serializeObjectAsync(name, entryCache, doNotSerialize);
            const serializedValue = await serializeObjectAsync(value, entryCache, doNotSerialize);
            env.set(serializedName, { entry: serializedValue });
        }

        return env;
    }

    function processDerivedClassConstructor(protoEntry: EnvironmentEntry) {
        // A reference to the base constructor function.  Used so that the derived constructor and
        // class-methods can refer to the base class for "super" calls.
        const superEntryAndDescriptor: EnvironmentEntryAndDescriptor = { entry: protoEntry };

        // constructor
        classInstanceMemberToSuperEntry.set(func, superEntryAndDescriptor);

        // Also, make sure our methods can also find this entry so they too can refer to
        // 'super'.
        for (const keyOrSymbol of getOwnPropertyNamesAndSymbols(func)) {
            if (keyOrSymbol !== "length" && keyOrSymbol !== "name" && keyOrSymbol !== "prototype") {
                // static method.
                const classProp = (<any>func)[keyOrSymbol];
                addIfFunction(classProp, /*isStatic*/ true);
            }
        }

        for (const keyOrSymbol of getOwnPropertyNamesAndSymbols(func.prototype)) {
            // instance method.
            const classProp = func.prototype[keyOrSymbol];
            addIfFunction(classProp, /*isStatic*/ false);
        }

        return;

        function addIfFunction(prop: any, isStatic: boolean) {
            if (prop instanceof Function) {
                const set = isStatic ? classStaticMemberToSuperEntry : classInstanceMemberToSuperEntry;
                set.set(prop, superEntryAndDescriptor);
            }
        }
    }

    function rewriteSuperReferences(code: string, isStatic: boolean): string {
        const file = ts.createSourceFile(
            "", code, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);

        // Transform any usages of "super(...)" into "__super.call(this, ...)", any
        // instance usages of "super.xxx" into "__super.prototype.xxx" and any static
        // usages of "super.xxx" into "__super.xxx"
        const transformed = ts.transform(file, [rewriteSuperCallsWorker]);
        const printer = ts.createPrinter({ newLine: ts.NewLineKind.LineFeed });
        const output = printer.printNode(ts.EmitHint.Unspecified, transformed.transformed[0], file).trim();

        return output;

        function rewriteSuperCallsWorker(context: ts.TransformationContext) {
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
                    const funcDecl = ts.visitEachChild(node, visitor, context);

                    const text = isLegalName(funcDecl.name!.text)
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
                const rewritten = ts.visitEachChild(node, visitor, context);

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
}

function getOwnPropertyNamesAndSymbols(obj: any): (string | symbol)[] {
    const names: (string | symbol)[] = Object.getOwnPropertyNames(obj);
    return names.concat(Object.getOwnPropertySymbols(obj));
}

interface SerializedFunction {
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
}

// Gets the text of the provided function (using .toString()) and massages it so that it is a legal
// function declaration.  Note: this ties us heavily to V8 and its representation for functions.  In
// particular, it has expectations around how functions/lambdas/methods/generators/constructors etc.
// are represented.  If these change, this will likely break us.zs
function serializeFunctionCode(func: Function): SerializedFunction {
    const funcString = func.toString();
    if (funcString.startsWith("[Function:")) {
        throw new Error(
            `Cannot serialize non-expression functions (such as definitions and generators):\n${funcString}`);
    }

    if (funcString.indexOf("[native code]") !== -1) {
        throw new Error(`Cannot serialize native code function:\n${funcString}`);
    }

    const openCurlyIndex = funcString.indexOf("{");
    if (openCurlyIndex < 0) {
        // No block body.  Can happen if this is an arrow function with an expression body.
        const arrowIndex = funcString.indexOf("=>");
        if (arrowIndex >= 0) {
            // (...) => expr
            return { funcExprWithoutName: funcString };
        }

        throw new Error(`Could not understand function:\n${funcString}`);
    }

    const signature = funcString.substr(0, openCurlyIndex);
    if (signature.indexOf("=>") >= 0) {
        // (...) => { ... }
        return { funcExprWithoutName: funcString };
    }

    if (funcString.startsWith("function get ") || funcString.startsWith("function set ")) {
        const trimmed = funcString.substr("function get".length);
        return makeFunctionDeclaration(trimmed, /*isFunctionDeclaration: */ false);
    }

    if (funcString.startsWith("function")) {
        const trimmed = funcString.substr("function".length);
        return makeFunctionDeclaration(trimmed, /*isFunctionDeclaration: */ true);
    }

    if (funcString.startsWith("class ")) {
        // class constructor function.  We want to get the actual constructor
        // in the class definition (synthesizing an empty one if one does not)
        // exist.
        const file = ts.createSourceFile("", funcString, ts.ScriptTarget.Latest);
        const diagnostics: ts.Diagnostic[] = (<any>file).parseDiagnostics;
        if (diagnostics.length) {
            throw new Error(`Could not parse class: ${diagnostics[0].messageText}\n${funcString}`);
        }

        const classDecl = <ts.ClassDeclaration>file.statements.find(x => ts.isClassDeclaration(x));
        if (!classDecl) {
            throw new Error(`Could not understand class:\n${funcString}`);
        }

        const constructor = <ts.ConstructorDeclaration>classDecl.members.find(m => ts.isConstructorDeclaration(m));
        if (!constructor) {
            // class without explicit constructor.
            const isSubClass = classDecl.heritageClauses && classDecl.heritageClauses.some(
                c => c.token === ts.SyntaxKind.ExtendsKeyword);
            return isSubClass
                ? makeFunctionDeclaration("constructor() { super(); }", /*isFunctionDeclaration: */ false)
                : makeFunctionDeclaration("constructor() { }", /*isFunctionDeclaration: */ false);
        }

        const constructorCode = funcString.substring(constructor.pos, constructor.end).trim();
        return makeFunctionDeclaration(constructorCode, /*isFunctionDeclaration: */ false);
    }

    // Add "function" (this will make methods parseable).  i.e.  "foo() { }" becomes
    // "function foo() { }"
    // this also does the right thing for functions with computed names.
    return makeFunctionDeclaration(funcString, /*isFunctionDeclaration: */ false);

    function makeFunctionDeclaration(v: string, isFunctionDeclaration: boolean): SerializedFunction {
        let prefix = "function ";
        v = v.trimLeft();

        if (v.startsWith("*")) {
            v = v.substr(1).trimLeft();
            prefix = "function* ";
        }

        const openParenIndex = v.indexOf("(");
        if (openParenIndex < 0) {
            throw new Error(`Could not understand function:\n${funcString}`);
        }

        if (openParenIndex === 0) {
            return {
                funcExprWithoutName: prefix + v,
                funcExprWithName: prefix + "__computed" + v,
                functionDeclarationName: undefined,
            };
        }

        const funcName = v.substr(0, openParenIndex);
        const commentedName = isLegalName(funcName) ? "/*" + funcName + "*/" : "";
        v = v.substr(openParenIndex).trimLeft();

        return {
            funcExprWithoutName: prefix + commentedName + v,
            funcExprWithName: prefix + funcName + v,
            functionDeclarationName: isFunctionDeclaration ? funcName : undefined,
        };
    }
}

function isDefaultFunctionPrototype(func: Function, prototypeProp: any) {
    // The initial value of prototype on any newly-created Function instance is a new instance of
    // Object, but with the own-property 'constructor' set to point back to the new function.
    if (prototypeProp && prototypeProp.constructor === func) {
        const keysAndSymbols = getOwnPropertyNamesAndSymbols(prototypeProp);
        return keysAndSymbols.length === 1 && keysAndSymbols[0] === "constructor";
    }

    return false;
}

/**
 * serializeObjectAsync serializes an object, deeply, into something appropriate for an environment entry.
 */
function serializeObjectAsync(
        obj: any, entryCache: Map<Object, EnvironmentEntry>, doNotSerialize: Set<any>): Promise<EnvironmentEntry> {
    // See if we have a cache hit.  If yes, use the object as-is.
    const result = entryCache.get(obj);
    if (result) {
        return Promise.resolve(result);
    }

    return serializeObjectWorkerAsync(obj, entryCache, doNotSerialize);
}

/**
 * serializeObjectWorkerAsync is the work-horse that actually performs object serialization.
 */
async function serializeObjectWorkerAsync(
        obj: any, entryCache: Map<Object, EnvironmentEntry>, doNotSerialize: Set<any>): Promise<EnvironmentEntry> {

    // We may be processing recursive objects.  Because of that, we preemptively put a placeholder
    // AsyncEnvironmentEntry in the cache.  That way, if we encounter this obj again while recursing
    // we can just return that placeholder.
    const entry: EnvironmentEntry = {};
    entryCache.set(obj, entry);

    if (doNotSerialize.has(obj)) {
        entry.json = undefined;
        return entry;
    }

    const moduleName = findRequirableModuleName(obj);

    if (obj === undefined || obj === null ||
        typeof obj === "boolean" || typeof obj === "number" || typeof obj === "string") {
        // Serialize primitives as-is.
        entry.json = obj;
    }
    else if (moduleName) {
        // Serialize any value which was found as a requirable module name as a reference to the module
        entry.module = moduleName;
    }
    else if (obj instanceof Function) {
        // Serialize functions recursively, and store them in a closure property.
        entry.closure = await serializeFunctionRecursiveAsync(obj, entryCache, doNotSerialize);
    }
    else if (obj instanceof resource.Output) {
        entry.dep = await serializeObjectAsync(await obj.promise(), entryCache, doNotSerialize);
    }
    else if (obj instanceof Promise) {
        // If this is a promise, we will await it and serialize the result instead.
        entry.promise = await serializeObjectAsync(await obj, entryCache, doNotSerialize);
    }
    else if (obj instanceof Array) {
        // Recursively serialize elements of an array. Note: we use getOwnPropertyNames as the array
        // may be sparse and we want to properly respect that when serializing.
        entry.arr = [];
        for (const key of Object.getOwnPropertyNames(obj)) {
            if (key !== "length" && obj.hasOwnProperty(key)) {
                entry.arr[<any>key] = await serializeObjectAsync(obj[<any>key], entryCache, doNotSerialize);
            }
        }
    }
    else if (Object.prototype.toString.call(obj) === "[object Arguments]") {
        // tslint:disable-next-line:max-line-length
        // From: https://stackoverflow.com/questions/7656280/how-do-i-check-whether-an-object-is-an-arguments-object-in-javascript
        entry.arr = [];
        for (const elem of obj) {
            entry.arr.push(await serializeObjectAsync(elem, entryCache, doNotSerialize));
        }
    }
    else {
        // For all other objects, serialize all of their properties.
        const environment: Environment = new Map();
        entry.obj = { env: environment };

        for (const keyOrSymbol of getOwnPropertyNamesAndSymbols(obj)) {
            const descriptor = await getEntryDescriptorAsync(keyOrSymbol);

            const keyEntry = await serializeObjectAsync(keyOrSymbol, entryCache, doNotSerialize);
            const valEntry = await serializeObjectAsync(obj[keyOrSymbol], entryCache, doNotSerialize);
            environment.set(keyEntry, { descriptor: descriptor, entry: valEntry });
        }

        // If the object's __proto__ is not Object.prototype, then we have to capture what it
        // actually is.  On the other end, we'll use Object.create(deserializedProto) to set things
        // up properly.
        const proto = Object.getPrototypeOf(obj);
        if (proto !== Object.prototype) {
            entry.obj.proto = await serializeObjectAsync(proto, entryCache, doNotSerialize);
        }
    }

    return entry;

    async function getEntryDescriptorAsync(key: PropertyKey) {
        const desc = Object.getOwnPropertyDescriptor(obj, key);
        let entryDescriptor: EntryDescriptor | undefined;
        if (desc) {
            if (!desc.enumerable || !desc.writable || !desc.configurable || desc.get || desc.set) {
                // Complex property.  Copy over the relevant flags.  (We copy to make
                // testing easier).
                entryDescriptor = { hasValue: desc.value !== undefined };
                if (desc.configurable) {
                    entryDescriptor.configurable = desc.configurable;
                }
                if (desc.enumerable) {
                    entryDescriptor.enumerable = desc.enumerable;
                }
                if (desc.writable) {
                    entryDescriptor.writable = desc.writable;
                }
                if (desc.get) {
                    entryDescriptor.get = await serializeObjectAsync(desc.get, entryCache, doNotSerialize);
                }
                if (desc.set) {
                    entryDescriptor.set = await serializeObjectAsync(desc.set, entryCache, doNotSerialize);
                }
            }
        }

        return entryDescriptor;
    }
}

// These modules are built-in to Node.js, and are available via `require(...)`
// but are not stored in the `require.cache`.  They are guaranteed to be
// available at the unqualified names listed below. _Note_: This list is derived
// based on Node.js 6.x tree at: https://github.com/nodejs/node/tree/v6.x/lib
const builtInModuleNames = [
    "assert", "buffer", "child_process", "cluster", "console", "constants", "crypto",
    "dgram", "dns", "domain", "events", "fs", "http", "https", "module", "net", "os",
    "path", "process", "punycode", "querystring", "readline", "repl", "stream", "string_decoder",
    /* "sys" deprecated ,*/ "timers", "tls", "tty", "url", "util", "v8", "vm", "zlib",
];
const builtInModules = new Map<any, string>();
for (const name of builtInModuleNames) {
    builtInModules.set(require(name), name);
}

// findRequirableModuleName attempts to find a global name bound to the object, which can
// be used as a stable reference across serialization.
function findRequirableModuleName(obj: any): string | undefined  {
    // First, check the built-in modules
    const key = builtInModules.get(obj);
    if (key) {
        return key;
    }
    // Next, check the Node module require cache, which will store cached values
    // of all non-built-in Node modules loaded by the program so far. _Note_: We
    // don't pre-compute this because the require cache will get populated
    // dynamically during execution.
    for (const path of Object.keys(require.cache)) {
        if (require.cache[path].exports === obj) {
            // Rewrite the path to be a local module reference relative to the
            // current working directory
            const modPath = pathRelative(process.cwd(), path).replace(/\\/g, "\\\\");
            return "./" + modPath;
        }
    }

    // Else, return that no global name is available for this object.
    return undefined;
}

const nodeModuleGlobals: {[key: string]: boolean} = {
    "__dirname": true,
    "__filename": true,
    "exports": true,
    "module": true,
    "require": true,
};

/**
 * computeCapturedVariableNames computes the set of free variables in a given function string.  Note that this string is
 * expected to be the usual V8-serialized function expression text.
 */
function computeCapturedVariableNames(funcstr: string): string[] {
    log.debug(`Computing free variables for function: ${funcstr}`);

    // Wrap with parens to make into something parseable.  This is necessary as many
    // types of functions are valid function expressions, but not valid function
    // declarations.  i.e.   "function () { }".  This is not a valid function declaration
    // (it's missing a name).  But it's totally legal as "(function () { })".
    const toParse = "(" + funcstr + ")";

    const file = ts.createSourceFile(
        "", toParse, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
    const diagnostics: ts.Diagnostic[] = (<any>file).parseDiagnostics;
    if (diagnostics.length) {
        throw new Error(`Could not parse function: ${diagnostics[0].messageText}\n${toParse}`);
    }

    // Now that we've parsed the file, compute the free variables, and return them.

    let captures: {[key: string]: boolean} = {};
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
    const freeVars: string[] = [];
    for (const key of Object.keys(captures)) {
        if (captures[key] && !isBuiltIn(key)) {
            freeVars.push(key);
        }
    }

    log.debug(`Found free variables: ${freeVars}`);
    return freeVars;

    function isBuiltIn(ident: string): boolean {
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

        // We reached the top of the scope chain and this wasn't found; it's free.
        captures[name] = true;
    }

    function walk(node: ts.Node | undefined) {
        if (!node) {
            return;
        }

        switch (node.kind) {
            case ts.SyntaxKind.Identifier:
                return visitIdentifier(<ts.Identifier>node);
            case ts.SyntaxKind.ThisKeyword:
                return visitThisExpression(<ts.PrimaryExpression>node);
            case ts.SyntaxKind.Block:
                return visitBlockStatement(<ts.Block>node);
            case ts.SyntaxKind.CallExpression:
                return visitCallExpression(<ts.CallExpression>node);
            case ts.SyntaxKind.CatchClause:
                return visitCatchClause(<ts.CatchClause>node);
            case ts.SyntaxKind.MethodDeclaration:
                return visitMethodDeclaration(<ts.MethodDeclaration>node);
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

    function visitThisExpression(node: ts.PrimaryExpression): void {
        // Mark references to the built-in 'this' variable as free.
        captures["this"] = true;
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
        const savedCaptures = captures;
        const savedFunctionVars = functionVars;
        captures = {};
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
        }

        // Next, visit the body underneath this new context.
        walk(node.body);

        // Remove any function-scoped variables that we encountered during the walk.
        for (const v of functionVars) {
            delete captures[v];
        }

        // Restore the prior context and merge our free list with the previous one.
        scopes.pop();

        functionVars = savedFunctionVars;
        for (const free of Object.keys(captures)) {
            if (captures[free]) {
                savedCaptures[free] = true;
            }
        }
        captures = savedCaptures;
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

        const isAwaiterCall =
            ts.isIdentifier(node.expression) &&
            node.expression.text === "__awaiter" &&
            node.arguments.length === 4 &&
            node.arguments[0].kind === ts.SyntaxKind.ThisKeyword &&
            ts.isFunctionLike(node.arguments[3]);

        if (isAwaiterCall) {
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

/**
 * serializeJavaScriptText converts a Closure object into a string representation of a Node.js module body which
 * exposes a single function `exports.handler` representing the serialized function.
 *
 * @param c The Closure to be serialized into a module string.
 */
async function serializeJavaScriptTextAsync(outerClosure: Closure): Promise<string> {
    // Ensure the closure is targeting a supported runtime.
    if (outerClosure.runtime !== "nodejs") {
        throw new Error(`Runtime '${outerClosure.runtime}' not yet supported (currently only 'nodejs')`);
    }

    // Now produce a textual representation of the closure and its serialized captured environment.

    // State used to build up the environment variables for all the funcs we generate.
    // In general, we try to create idiomatic code to make the generated code not too
    // hideous.  For example, we will try to generate code like:
    //
    //      var __e1 = [1, 2, 3] // or
    //      var __e2 = { a: 1, b: 2, c: 3 }
    //
    // However, for non-common cases (i.e. sparse arrays, objects with configured properties,
    // etc. etc.) we will spit things out in a much more verbose fashion that eschews
    // prettyness for correct semantics.
    let currentClosureIndex = 0;
    let currentEnvIndex = 0;
    const envEntryToEnvVar = new Map<EnvironmentEntry, string>();
    const closureToEnvVar = new Map<Closure, string>();

    let environmentText = "";
    let functionText = "";

    const outerClosureName = await emitClosureAndGetNameAsync(outerClosure);

    if (environmentText) {
        environmentText = "\n" + environmentText;
    }

    const text = "exports.handler = " + outerClosureName + ";\n"
        + environmentText + functionText;

    return text;

    async function emitClosureAndGetNameAsync(closure: Closure): Promise<string> {
        // If this is the first time seeing this closure, then actually emit the function code for
        // it.  Otherwise, just return the name of the emitted function for anyone that wants to
        // reference it from their own code.
        let closureName = closureToEnvVar.get(closure);
        if (!closureName) {
            closureName = `__f${currentClosureIndex++}`;
            closureToEnvVar.set(closure, closureName);

            await emitClosureWorkerAsync(closure, closureName);
        }

        return closureName;
    }

    async function emitClosureWorkerAsync(closure: Closure, varName: string) {
        const environment = await envFromEnvObjAsync(closure.environment);

        const thisCapture = environment.this;
        const argumentsCapture = environment.arguments;

        delete environment.this;
        delete environment.arguments;

        functionText += "\n" +
            "function " + varName + "() {\n" +
            "  return (function() {\n" +
            "    with(" + envObjToString(environment) + ") {\n\n" +
            "return " + closure.code + ";\n\n" +
            "    }\n" +
            "  }).apply(" + thisCapture + ", " + argumentsCapture + ").apply(this, arguments);\n" +
            "}\n";

        // If this function is complex (i.e. non-default __proto__, or has properties, etc.)
        // then emit those as well.
        if (closure.obj !== undefined) {
            await emitComplexObjectPropertiesAsync(varName, varName, closure.obj);

            if (closure.obj.proto !== undefined) {
                const protoVar = await envEntryToStringAsync(
                    await closure.obj.proto, `${varName}_proto`);
                environmentText += `Object.setPrototypeOf(${varName}, ${protoVar});\n`;
            }
        }
    }

    async function envFromEnvObjAsync(env: Environment): Promise<Record<string, string>> {
        const envObj: Record<string, string> = {};
        for (const [keyEntry, { entry: valEntry }] of env) {
            if (typeof keyEntry.json !== "string") {
                throw new Error("Environment key was not a string.");
            }

            const key = keyEntry.json;
            const val = await envEntryToStringAsync(valEntry, key);
            envObj[key] = val;
        }
        return envObj;
    }

    async function envEntryToStringAsync(
            envEntry: EnvironmentEntry, varName: string): Promise<string> {

        const envVar = envEntryToEnvVar.get(envEntry);
        if (envVar !== undefined) {
            return envVar;
        }

        // Objects any arrays may have cycles in them.  They may also be referenced from multiple
        // closures.  As such, we have to create variables for them in the environment so that all
        // references to them unify to the same reference to the env variable.
        if (isObjOrArray(envEntry)) {
            return complexEnvEntryToString(envEntry, varName);
        }
        else {
            // Other values (like strings, bools, etc.) can just be emitted inline.
            return await simpleEnvEntryToStringAsync(envEntry, varName);
        }
    }

    async function simpleEnvEntryToStringAsync(
            envEntry: EnvironmentEntry, varName: string): Promise<string> {

        if (envEntry.hasOwnProperty("json")) {
            return JSON.stringify(envEntry.json);
        }
        else if (envEntry.closure !== undefined) {
            const closureName = await emitClosureAndGetNameAsync(envEntry.closure);
            return closureName;
        }
        else if (envEntry.module !== undefined) {
            return `require("${envEntry.module}")`;
        }
        else if (envEntry.dep !== undefined) {
            // get: () => { ... }
            // parses as a lambda with a block body, not as a lambda returning an object
            // initializer.  If we have a block body, wrap it with parens.
            let value = await envEntryToStringAsync(envEntry.dep, varName);
            if (value && value.charAt(0) === "{") {
                value = `(${value})`;
            }

            return `{ get: () => ${value} }`;
        }
        else if (envEntry.expr) {
            // Entry specifies exactly how it should be emitted.  So just use whatever
            // it wanted.
            return envEntry.expr;
        }
        else if (envEntry.promise) {
            return `Promise.resolve(${await envEntryToStringAsync(envEntry.promise, varName)})`;
        }
        else {
            throw new Error("Malformed: " + JSON.stringify(envEntry));
        }
    }

    async function complexEnvEntryToString(
            envEntry: EnvironmentEntry, varName: string): Promise<string> {
        const index = currentEnvIndex++;

        // Call all environment variables __e<num> to make them unique.  But suffix
        // them with the original name of the property to help provide context when
        // looking at the source.
        const envVar = `__e${index}_${makeLegalJSName(varName)}`;
        envEntryToEnvVar.set(envEntry, envVar);

        if (envEntry.obj) {
            await emitObjectAsync(envVar, envEntry.obj, varName);
        } else if (envEntry.arr) {
            await emitArrayAsync(envVar, envEntry.arr, varName);
        }

        return envVar;
    }

    async function emitObjectAsync(envVar: string, obj: ObjectEntry, varName: string): Promise<void> {
        const complex = await isComplex(obj);

        if (complex) {
            // we have a complex child.  Because of the possibility of recursion in
            // the object graph, we have to spit out this variable uninitialized first.
            // Then we can walk our children, creating a single assignment per child.
            // This way, if the child ends up referencing us, we'll have already emitted
            // the **initialized** variable for them to reference.
            if (obj.proto) {
                const protoVar = await envEntryToStringAsync(await obj.proto, `${varName}_proto`);
                environmentText += `var ${envVar} = Object.create(${protoVar});\n`;
            }
            else {
                environmentText += `var ${envVar} = {};\n`;
            }

            await emitComplexObjectPropertiesAsync(envVar, varName, obj);
        }
        else {
            // All values inside this obj are simple.  We can just emit the object
            // directly as an object literal with all children embedded in the literal.
            const props: string[] = [];

            for (const [keyEntry, { entry: valEntry }] of obj.env) {
                const keyName = typeof keyEntry.json === "string" ? keyEntry.json : "sym";
                const propName = await envEntryToStringAsync(keyEntry, keyName);
                const propVal = await simpleEnvEntryToStringAsync(await valEntry, keyName);

                if (typeof keyEntry.json === "string" && isLegalName(keyEntry.json)) {
                    props.push(`${keyEntry.json}: ${propVal}`);
                }
                else {
                    props.push(`[${propName}]: ${propVal}`);
                }
            }

            const allProps = props.join(", ");
            const entryString = `var ${envVar} = {${allProps}};\n`;
            environmentText += entryString;
        }

        async function isComplex(o: ObjectEntry) {
            if (obj.proto !== undefined) {
                return true;
            }

            for (const v of o.env.values()) {
                if (entryIsComplex(v)) {
                    return true;
                }
            }

            return false;
        }

        function entryIsComplex(v: EnvironmentEntryAndDescriptor) {
            return v.descriptor !== undefined || deepContainsObjOrArray(v.entry);
        }
    }

    async function emitComplexObjectPropertiesAsync(
            envVar: string, varName: string, objEntry: ObjectEntry): Promise<void> {

        for (const [keyEntry, { descriptor, entry: valEntry }] of objEntry.env) {
            const subName = typeof keyEntry.json === "string" ? keyEntry.json : "sym";
            const keyString = await envEntryToStringAsync(keyEntry, subName);
            const valString = await envEntryToStringAsync(valEntry, subName);

            if (!descriptor) {
                // normal property.  Just emit simply as a direct assignment.
                if (typeof keyEntry.json === "string" && isLegalName(keyEntry.json)) {
                    environmentText += `${envVar}.${keyEntry.json} = ${valString};\n`;
                }
                else {
                    environmentText += `${envVar}${`[${keyString}]`} = ${valString};\n`;
                }
            }
            else {
                // complex property.  emit as Object.defineProperty
                await emitDefinePropertyAsync(descriptor, valString, keyString);
            }
        }

        async function emitDefinePropertyAsync(
            desc: EntryDescriptor, entryValue: string, propName: string) {

            const copy: any = {};
            if (desc.configurable !== undefined) {
                copy.configurable = desc.configurable;
            }
            if (desc.enumerable !== undefined) {
                copy.enumerable = desc.enumerable;
            }
            if (desc.writable !== undefined) {
                copy.writable = desc.writable;
            }
            if (desc.get) {
                copy.get = await envEntryToStringAsync(desc.get, `${varName}_get`);
            }
            if (desc.set) {
                copy.set = await envEntryToStringAsync(desc.set, `${varName}_set`);
            }
            if (desc.hasValue) {
                copy.value = entryValue;
            }
            environmentText += `Object.defineProperty(${envVar}, ${propName}, ` +
                `${ envObjToString(copy) });\n`;
        }
    }

    async function emitArrayAsync(
            envVar: string, arr: EnvironmentEntry[], varName: string): Promise<void> {
        if (arr.some(deepContainsObjOrArray) || isSparse(arr) || hasNonNumericIndices(arr)) {
            // we have a complex child.  Because of the possibility of recursion in the object
            // graph, we have to spit out this variable initialized (but empty) first. Then we can
            // walk our children, knowing we'll be able to find this variable if they reference it.
            environmentText += `var ${envVar} = [];\n`;

            // Walk the names of the array properties directly. This ensures we work efficiently
            // with sparse arrays.  i.e. if the array has length 1k, but only has one value in it
            // set, we can just set htat value, instead of setting 999 undefineds.
            let length = 0;
            for (const key of Object.getOwnPropertyNames(arr)) {
                if (key !== "length") {
                    const entryString = await envEntryToStringAsync(arr[<any>key], `${varName}_${key}`);
                    environmentText += `${envVar}${
                        isNumeric(key) ? `[${key}]` : `.${key}`} = ${entryString};\n`;
                    length++;
                }
            }
        }
        else {
            // All values inside this array are simple.  We can just emit the array elements in
            // place.  i.e. we can emit as ``var arr = [1, 2, 3]`` as that's far more preferred than
            // having four individual statements to do the same.
            const strings: string[] = [];
            for (let i = 0, n = arr.length; i < n; i++) {
                strings.push(await simpleEnvEntryToStringAsync(arr[i], `${varName}_${i}`));
            }

            const entryString = `var ${envVar} = [${strings.join(", ")}];\n`;
            environmentText += entryString;
        }
    }
}

const regex = /[^0-9a-zA-Z_]/g;
function makeLegalJSName(n: string) {
    return n.replace(regex, x => "");
}

function isLegalName(n: string) {
    return makeLegalJSName(n) === n;
}

function isSparse<T>(arr: Array<T>) {
    // getOwnPropertyNames for an array returns all the indices as well as 'length'.
    // so we subtract one to get all the real indices.  If that's not the same as
    // the array length, then we must have missing properties and are thus sparse.
    return arr.length !== (Object.getOwnPropertyNames(arr).length - 1);
}

function hasNonNumericIndices<T>(arr: Array<T>) {
    return Object.keys(arr).some(k => k !== "length" && !isNumeric(k));
}

function isNumeric(n: string) {
    return !isNaN(parseFloat(n)) && isFinite(+n);
}

function isObjOrArray(env: EnvironmentEntry): boolean {
    return env.obj !== undefined || env.arr !== undefined;
}

function deepContainsObjOrArray(env: EnvironmentEntry): boolean {
    return isObjOrArray(env) ||
        (env.dep !== undefined && deepContainsObjOrArray(env.dep)) ||
        (env.promise !== undefined && deepContainsObjOrArray(env.promise));
}

/**
 * Converts an environment object into a string which can be embedded into a serialized function
 * body.  Note that this is not JSON serialization, as we may have property values which are
 * variable references to other global functions. In other words, there can be free variables in the
 * resulting object literal.
 *
 * @param envObj The environment object to convert to a string.
 */
function envObjToString(envObj: Record<string, string>): string {
    let result = "";
    let first = true;
    for (const key of Object.keys(envObj)) {
        const val = envObj[key];

        if (!first) {
            result += ", ";
        }

        result += key + ": " + val;
        first = false;
    }
    return "{ " + result + " }";
}
