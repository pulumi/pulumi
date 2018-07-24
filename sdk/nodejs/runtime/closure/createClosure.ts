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

import { relative as pathRelative } from "path";
import { basename } from "path";
import * as ts from "typescript";
import { RunError } from "../../errors";
import * as resource from "../../resource";
import { CapturedPropertyChain, CapturedPropertyInfo, CapturedVariableMap, parseFunction } from "./parseFunction";
import { rewriteSuperReferences } from "./rewriteSuper";
import * as v8 from "./v8";

export interface ObjectInfo {
    // information about the prototype of this object/function.  If this is an object, we only store
    // this if the object's prototype is not Object.prototype.  If this is a function, we only store
    // this if the function's prototype is not Function.prototype.
    proto?: Entry;

    // information about the properties of the object.  We store all properties of the object,
    // regardless of whether they have string or symbol names.
    env: PropertyMap;
}

// Information about a javascript function.  Note that this derives from ObjectInfo as all functions
// are objects in JS, and thus can have their own proto and properties.
export interface FunctionInfo extends ObjectInfo {
    // a serialization of the function's source code as text.
    code: string;

    // the captured lexical environment of names to values, if any.
    capturedValues: PropertyMap;

    // Whether or not the real 'this' (i.e. not a lexically captured this) is used in the function.
    usesNonLexicalThis: boolean;

    // name that the function was declared with.  used only for trying to emit a better
    // name into the serialized code for it.
    name: string | undefined;

    // the set of package 'requires' seen in the function body.
    requiredPackages: Set<string>;
}

// Similar to PropertyDescriptor.  Helps describe an Entry in the case where it is not
// simple.
export interface PropertyInfo {
    // If the property has a value we should directly provide when calling .defineProperty
    hasValue: boolean;

    // same as PropertyDescriptor
    configurable?: boolean;
    enumerable?: boolean;
    writable?: boolean;

    // The entries we've made for custom getters/setters if the property is defined that
    // way.
    get?: Entry;
    set?: Entry;
}

// Information about a property.  Specifically the actual entry containing the data about it and
// then an optional PropertyInfo in the case that this isn't just a common property.
export interface PropertyInfoAndValue {
    info?: PropertyInfo;
    entry: Entry;
}

// A mapping between the name of a property (symbolic or string) to information about the
// value for that property.
export interface PropertyMap extends Map<Entry, PropertyInfoAndValue> {
}

/**
 * Entry is the environment slot for a named lexically captured variable.
 */
export interface Entry {
    // a value which can be safely json serialized.
    json?: any;

    // a closure we are dependent on.
    function?: FunctionInfo;

    // An object which may contain nested closures.
    // Can include an optional proto if the user is not using the default Object.prototype.
    object?: ObjectInfo;

    // an array which may contain nested closures.
    array?: Entry[];

    // a reference to a requirable module name.
    module?: string;

    // A promise value.  this will be serialized as the underlyign value the promise
    // points to.  And deserialized as Promise.resolve(<underlying_value>)
    promise?: Entry;

    // an Output<T> property.  It will be serialized over as a get() method that
    // returns the raw underlying value.
    output?: Entry;

    // a simple expression to use to represent this instance.  For example "global.Number";
    expr?: string;
}

interface Context {
    // The cache stores a map of objects to the entries we've created for them.  It's used so that
    // we only ever create a single environemnt entry for a single object. i.e. if we hit the same
    // object multiple times while walking the memory graph, we only emit it once.
    cache: Map<Object, Entry>;

    // The 'frames' we push/pop as we're walking the object graph serializing things.
    // These frames allow us to present a useful error message to the user in the context
    // of their code as opposed the async callstack we have while serializing.
    frames: ContextFrame[];

    // A mapping from a class method/constructor to the environment entry corresponding to the
    // __super value.  When we emit the code for any class member we will end up adding
    //
    //  with ( { __super: <...> })
    //
    // We will also rewrite usages of "super" in the methods to refer to __super.  This way we can
    // accurately serialize out the class members, while preserving functionality.
    classInstanceMemberToSuperEntry: Map<Function, Entry>;
    classStaticMemberToSuperEntry: Map<Function, Entry>;

    // The set of async jobs we have to complete after serializing the object graph. This happens
    // when we encounter Promises/Outputs while walking the graph.  We'll add that work here and
    // then process it at the end of the graph.  Note: as we hit those promises we may discover more
    // work to be done.  So we'll just keep processing this this queue until there is nothing left
    // in it.
    asyncWorkQueue: (() => Promise<void>)[];

    // A list of 'simple' functions.  Simple functions do not capture anything, do not have any
    // special properties on them, and do not have a custom prototype.  If we run into multiple
    // functions that are simple, and share the same code, then we can just emit the function once
    // for them.  A good example of this is the __awaiter function.  Normally, there will be one
    // __awaiter per .js file that uses 'async/await'.  Instead of needing to generate serialized
    // functions for each of those, we can just serialize out the function once.
    simpleFunctions: FunctionInfo[];
}

interface FunctionLocation {
    func: Function;
    file: string;
    line: number;
    column: number;
    functionString: string;
    isArrowFunction?: boolean;
}

interface ContextFrame {
    functionLocation?: FunctionLocation;
    capturedFunctionName?: string;
    capturedVariableName?: string;
    capturedModuleName?: string;
}

/*
 * SerializedOutput is the type we convert real deployment time outputs to when we serialize them
 * into the environment for a closure.  The output will go from something you call 'apply' on to
 * transform during deployment, to something you call .get on to get the raw underlying value from
 * inside a cloud callback.
 */
class SerializedOutput<T> implements resource.Output<T> {
    /* @internal */ public isKnown: Promise<boolean>;
    /* @internal */ public readonly promise: () => Promise<T>;
    /* @internal */ public readonly resources: () => Set<resource.Resource>;
    /* @internal */ private readonly value: T;

    public constructor(value: T) {
        this.value = value;
    }

    public apply<U>(func: (t: T) => resource.Input<U>): resource.Output<U> {
        throw new Error(
"'apply' is not allowed from inside a cloud-callback. Use 'get' to retrieve the value of this Output directly.");
    }

     public get(): T {
         return this.value;
     }
}

/**
 * createFunctionInfo serializes a function and its closure environment into a form that is
 * amenable to persistence as simple JSON.  Like toString, it includes the full text of the
 * function's source code, suitable for execution. Unlike toString, it actually includes information
 * about the captured environment.
 */
export async function createFunctionInfoAsync(
    func: Function, serialize: (o: any) => boolean): Promise<FunctionInfo> {

    const context: Context = {
        cache: new Map(),
        classInstanceMemberToSuperEntry: new Map(),
        classStaticMemberToSuperEntry: new Map(),
        frames: [],
        asyncWorkQueue: [],
        simpleFunctions: [],
     };

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
    context.cache.set(Symbol.iterator, { expr: "Symbol.iterator" });

    // Make sure this func is in the cache itself as we may hit it again while recursing.
    const entry: Entry = {};
    context.cache.set(func, entry);

    entry.function = createFunctionInfo(func, context, serialize);

    await processAsyncWorkQueue();

    return entry.function;

    function addGlobalInfo(key: string) {
        const globalObj = (<any>global)[key];
        const text = isLegalMemberName(key) ? `global.${key}` :  `global["${key}"]`;

        if (!context.cache.has(globalObj)) {
            context.cache.set(globalObj, { expr: text });
        }

        const proto1 = Object.getPrototypeOf(globalObj);
        if (proto1 && !context.cache.has(proto1)) {
            context.cache.set(proto1, { expr: `Object.getPrototypeOf(${text})`});
        }

        const proto2 = globalObj.prototype;
        if (proto2 && !context.cache.has(proto2)) {
            context.cache.set(proto2, { expr: `${text}.prototype`});
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

        context.cache.set(Object.getPrototypeOf(emptyGenerator),
            { expr: "Object.getPrototypeOf(function*(){})" });

        context.cache.set(Object.getPrototypeOf(emptyGenerator.prototype),
            { expr: "Object.getPrototypeOf((function*(){}).prototype)" });
    }

    async function processAsyncWorkQueue() {
        while (context.asyncWorkQueue.length > 0) {
            const queue = context.asyncWorkQueue;
            context.asyncWorkQueue = [];

            for (const work of queue) {
                await work();
            }
        }
    }
}

// This function ends up capturing many external modules that cannot themselves be serialized.
// Do not allow it to be captured.
(<any>createFunctionInfoAsync).doNotCapture = true;

/**
 * createFunctionInfo does the work to create an asynchronous dataflow graph that resolves to a
 * final FunctionInfo.
 */
function createFunctionInfo(
        func: Function, context: Context,
        serialize: (o: any) => boolean, logInfo?: boolean): FunctionInfo {

    // logInfo = logInfo || func.name === "addHandler";

    const file: string =  v8.getFunctionFile(func);
    const line: number = v8.getFunctionLine(func);
    const column: number = v8.getFunctionColumn(func);
    const functionString = func.toString();
    const frame = { functionLocation: { func, file, line, column, functionString, isArrowFunction: false } };

    context.frames.push(frame);
    const result = serializeWorker();
    context.frames.pop();

    if (isSimple(result)) {
        const existingSimpleFunction = findSimpleFunction(result);
        if (existingSimpleFunction) {
            return existingSimpleFunction;
        }

        context.simpleFunctions.push(result);
    }

    return result;

    function isSimple(info: FunctionInfo) {
        return info.capturedValues.size === 0 && info.env.size === 0 && !info.proto;
    }

    function findSimpleFunction(info: FunctionInfo) {
        for (const other of context.simpleFunctions) {
            if (other.code === info.code && other.usesNonLexicalThis === info.usesNonLexicalThis) {
                return other;
            }
        }

        return undefined;
    }

    function serializeWorker(): FunctionInfo {
        const funcEntry = context.cache.get(func);
        if (!funcEntry) {
            throw new Error("Entry for this this function was not created by caller");
        }

        // First, convert the js func object to a reasonable stringified version that we can operate on.
        // Importantly, this function helps massage all the different forms that V8 can produce to
        // either a "function (...) { ... }" form, or a "(...) => ..." form.  In other words, all
        // 'funky' functions (like classes and whatnot) will be transformed to reasonable forms we can
        // process down the pipeline.
        const [error, parsedFunction] = parseFunction(functionString);
        if (error) {
            throwSerializationError(func, context, error);
        }

        const funcExprWithName = parsedFunction.funcExprWithName;
        const functionDeclarationName = parsedFunction.functionDeclarationName;
        frame.functionLocation.isArrowFunction = parsedFunction.isArrowFunction;

        const capturedValues: PropertyMap = new Map();
        processCapturedVariables(parsedFunction.capturedVariables.required, /*throwOnFailure:*/ true);
        processCapturedVariables(parsedFunction.capturedVariables.optional, /*throwOnFailure:*/ false);

        const functionInfo: FunctionInfo = {
            code: parsedFunction.funcExprWithoutName,
            capturedValues: capturedValues,
            env: new Map(),
            usesNonLexicalThis: parsedFunction.usesNonLexicalThis,
            name: functionDeclarationName,
            requiredPackages: parsedFunction.requiredPackages,
        };

        const proto = Object.getPrototypeOf(func);

        const isDerivedClassConstructor =
            func.toString().startsWith("class ") &&
            proto !== Function.prototype(func);

        // Note, i can't think of a better way to determine this.  This is particularly hard because
        // we can't even necessary refer to async function objects here as this code is rewritten by
        // TS, converting all async functions to non async functions.
        const isAsyncFunction = func.constructor && func.constructor.name === "AsyncFunction";

        // Ensure that the prototype of this function is properly serialized as well. We only need to do
        // this for functions with a custom prototype (like a derived class constructor, or a function
        // that a user has explicit set the prototype for). Normal functions will pick up
        // Function.prototype by default, so we don't need to do anything for them.
        if (proto !== Function.prototype &&
            !isAsyncFunction &&
            !isDerivedNoCaptureConstructor(func)) {

            const protoEntry = getOrCreateEntry(proto, undefined, context, serialize, logInfo);
            functionInfo.proto = protoEntry;

            if (isDerivedClassConstructor) {
                processDerivedClassConstructor(protoEntry);
                functionInfo.code = rewriteSuperReferences(funcExprWithName!, /*isStatic*/ false);
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

            functionInfo.env.set(
                getOrCreateEntry(keyOrSymbol, undefined, context, serialize, logInfo),
                { entry: getOrCreateEntry(funcProp, undefined, context, serialize, logInfo) });
        }

        const superEntry = context.classInstanceMemberToSuperEntry.get(func) ||
                           context.classStaticMemberToSuperEntry.get(func);
        if (superEntry) {
            // this was a class constructor or method.  We need to put a special __super
            // entry into scope, and then rewrite any calls to super() to refer to it.
            capturedValues.set(
                getOrCreateEntry("__super", undefined, context, serialize, logInfo),
                { entry: superEntry });

            functionInfo.code = rewriteSuperReferences(
                funcExprWithName!, context.classStaticMemberToSuperEntry.has(func));
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
            capturedValues.set(
                getOrCreateEntry(functionDeclarationName, undefined, context, serialize, logInfo),
                { entry: funcEntry });
        }

        return functionInfo;

        function processCapturedVariables(
                capturedVariables: CapturedVariableMap, throwOnFailure: boolean): void {

            for (const name of capturedVariables.keys()) {
                let value: any;
                try {
                    value = v8.lookupCapturedVariableValue(func, name, throwOnFailure);
                }
                catch (err) {
                    throwSerializationError(func, context, err.message);
                }

                const moduleName = findModuleName(value);
                const frameLength = context.frames.length;
                if (moduleName) {
                    context.frames.push({ capturedModuleName: moduleName });
                }
                else if (value instanceof Function) {
                    // Only bother pushing on context frame if the name of the variable
                    // we captured is different from the name of the function.  If the
                    // names are the same, this is a direct reference, and we don't have
                    // to list both the name of the capture and of the function.  if they
                    // are different, it's an indirect reference, and the name should be
                    // included for clarity.
                    if (name !== value.name) {
                        context.frames.push({ capturedFunctionName: name });
                    }
                }
                else {
                    context.frames.push({ capturedVariableName: name });
                }

                processCapturedVariable(capturedVariables, name, value);

                // Only if we pushed a frame on should we pop it off.
                if (context.frames.length !== frameLength) {
                    context.frames.pop();
                }
            }
        }

        function processCapturedVariable(
            capturedVariables: CapturedVariableMap, name: string, value: any) {

            const properties = capturedVariables.get(name);
            const serializedName = getOrCreateEntry(name, undefined, context, serialize, logInfo);

            // try to only serialize out the properties that were used by the user's code.
            const serializedValue = getOrCreateEntry(value, properties, context, serialize, logInfo);

            capturedValues.set(serializedName, { entry: serializedValue });
        }
    }

    function processDerivedClassConstructor(protoEntry: Entry) {
        // A reference to the base constructor function.  Used so that the derived constructor and
        // class-methods can refer to the base class for "super" calls.

        // constructor
        context.classInstanceMemberToSuperEntry.set(func, protoEntry);

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
                const set = isStatic
                    ? context.classStaticMemberToSuperEntry
                    : context.classInstanceMemberToSuperEntry;
                set.set(prop, protoEntry);
            }
        }
    }
}

function getOwnPropertyNamesAndSymbols(obj: any): (string | symbol)[] {
    const names: (string | symbol)[] = Object.getOwnPropertyNames(obj);
    return names.concat(Object.getOwnPropertySymbols(obj));
}

function throwSerializationError(func: Function, context: Context, info: string): never {
    let message = "";

    const initialFuncLocation = getFunctionLocation(context.frames[0].functionLocation!);
    message += `Error serializing ${initialFuncLocation}\n\n`;

    let i = 0;
    const n = context.frames.length;
    for (; i < n; i++) {
        const frame = context.frames[i];

        const indentString = "  ".repeat(i);
        message += indentString;

        if (frame.functionLocation) {
            const funcLocation = getFunctionLocation(frame.functionLocation);
            const nextFrameIsFunction = i < n - 1 && context.frames[i + 1].functionLocation !== undefined;

            if (nextFrameIsFunction) {
                if (i === 0) {
                    message += `${funcLocation}: referenced\n`;
                }
                else {
                    message += `${funcLocation}: which referenced\n`;
                }
            }
            else {
                if (i === n - 1) {
                    message += `${funcLocation}: which could not be serialized because\n`;
                }
                else if (i === 0) {
                    message += `${funcLocation}: captured\n`;
                }
                else {
                    message += `${funcLocation}: which captured\n`;
                }
            }
        }
        else if (frame.capturedFunctionName) {
            message += `'${frame.capturedFunctionName}', a function defined at\n`;
        }
        else if (frame.capturedModuleName) {
            message += `module '${frame.capturedModuleName}' which indirectly referenced\n`;
        }
        else if (frame.capturedVariableName) {
            message += `variable '${frame.capturedVariableName}' which indirectly referenced\n`;
        }
    }

    message += "  ".repeat(i) + info + "\n\n";
    message += getTrimmedFunctionCode(func);

    const moduleIndex = context.frames.findIndex(f => f.capturedModuleName !== undefined);
    if (moduleIndex >= 0) {
        const moduleName = context.frames[moduleIndex].capturedModuleName;
        message += "\n";

        const functionLocation = context.frames[moduleIndex - 1].functionLocation!;
        const location = getFunctionLocation(functionLocation);
        message += `Capturing modules can sometimes cause problems.
Consider using import('${moduleName}') or require('${moduleName}') inside ${location}`;
    }

    throw new RunError(message);
}

function getTrimmedFunctionCode(func: Function): string {
    const funcString = func.toString();

    // include up to the first 5 lines of the function to help show what is wrong with it.
    let split = funcString.split(/\r?\n/);
    if (split.length > 5) {
        split = split.slice(0, 5);
        split.push("...");
    }

    let code = "Function code:\n";
    for (const line of split) {
        code += "  " + line + "\n";
    }

    return code;
}

function getFunctionLocation(loc: FunctionLocation): string {
    let name = "'" + getFunctionName(loc) + "'";
    if (loc.file) {
        name += `: ${basename(loc.file)}(${loc.line + 1},${loc.column})`;
    }

    const prefix = loc.isArrowFunction ? "" : "function ";
    return prefix + name;
}

function getFunctionName(loc: FunctionLocation): string {
    if (loc.isArrowFunction) {
        let funcString = loc.functionString;

        // If there's a semicolon in the text, only include up to that.  we don't want to pull in
        // the entire lambda if it's lots of statements.
        const semicolonIndex = funcString.indexOf(";");
        if (semicolonIndex >= 0) {
            funcString = funcString.substr(0, semicolonIndex + 1) + " ...";
        }

        // squash all whitespace to single spaces.
        funcString = funcString.replace(/\s\s+/g, " ");

        const lengthLimit = 40;
        if (funcString.length > lengthLimit) {
            // Trim the header if its very long.
            funcString = funcString.substring(0, lengthLimit - " ...".length) + " ...";
        }

        return funcString;
    }

    if (loc.func.name) {
        return loc.func.name;
    }

    return "<anonymous>";
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
 * serializeAsync serializes an object, deeply, into something appropriate for an environment
 * entry.  If propNames is provided, and is non-empty, then only attempt to serialize out those
 * specific properties.  If propNames is not provided, or is empty, serialize out all properties.
 */
function getOrCreateEntry(
        obj: any, capturedObjectProperties: CapturedPropertyChain[] | undefined,
        context: Context,
        serialize: (o: any) => boolean,
        logInfo: boolean | undefined): Entry {

    // See if we have a cache hit.  If yes, use the object as-is.
    let entry = context.cache.get(obj)!;
    if (entry) {
        // Even though we've already serialized out this object, it might be the case
        // that we serialized out a different set of properties than the current set
        // we're being asked to serialize.  So we have to make sure that all these props
        // are actually serialized.
        if (entry.object) {
            serializeObject();
        }

        return entry;
    }

    if (obj instanceof Function && obj.doNotCapture) {
        // If we get a function we're not supposed to capture, then actually just serialize
        // out a function that will throw at runtime so the user can understand the problem
        // better.
        const funcName = obj.name || "anonymous";
        const funcCode = getTrimmedFunctionCode(obj);

        const message =
            `Function '${funcName}' cannot be called at runtime. ` +
            `It can only be used at deployment time.\n\n${funcCode}`;
        const errorFunc = () => { throw new Error(message); };

        obj = errorFunc;
    }

    // We may be processing recursive objects.  Because of that, we preemptively put a placeholder
    // entry in the cache.  That way, if we encounter this obj again while recursing we can just
    // return that placeholder.
    entry = {};
    context.cache.set(obj, entry);
    dispatchAny();
    return entry;

    function doNotCapture() {
        if (!serialize(obj)) {
            // caller explicitly does not want us to capture this value.
            return true;
        }

        if (obj && obj.doNotCapture) {
            // object has set itself as something that should not be captured.
            return true;
        }

        if (isDerivedNoCaptureConstructor(obj)) {
            // this was a constructor that derived from something that should not be captured.
            return true;
        }

        return false;
    }

    function dispatchAny(): void {
        if (doNotCapture()) {
            // We do not want to capture this object.  Explicit set .json to undefined so
            // that we will see that the property is set and we will simply roundtrip this
            // as the 'undefined value.
            entry.json = undefined;
            return;
        }

        if (obj === undefined || obj === null ||
            typeof obj === "boolean" || typeof obj === "number" || typeof obj === "string") {
            // Serialize primitives as-is.
            entry.json = obj;
            return;
        }

        const moduleName = findModuleName(obj);
        if (moduleName) {
            console.log("Detected use of : " + moduleName + "\n");
        }
        if (moduleName && !obj.captureAsValue && moduleName[0] !== ".") {
            // This name bound to a module, and the module did not opt into having itself captured
            // by value. In this case serialize it out as a direct 'require' call
            //
            // Also, importantly, if this is a reference to a local module (i.e. starts with '.'),
            // then always capture it as a value.  The reason for this is that otherwise we won't
            // actually know or walk into the rest of the user's code. This means we won't properly
            // serialize it into the index.js handler and things will not work in the
            // cloud-callback.
            entry.module = moduleName;
        }
        else if (obj instanceof Function) {
            // Serialize functions recursively, and store them in a closure property.
            entry.function = createFunctionInfo(obj, context, serialize, logInfo);
        }
        else if (resource.Output.isInstance(obj)) {
            // captures the frames up to this point. so we can give a good message if we
            // fail when we resume serializing this promise.
            const framesCopy = context.frames.slice();

            // Push handling this output to the async work queue.  It will be processed in batches
            // after we've walked as much of the graph synchronously as possible.
            context.asyncWorkQueue.push(async () => {
                const val = await obj.promise();
                const oldFrames = context.frames;
                context.frames = framesCopy;
                entry.output = getOrCreateEntry(new SerializedOutput(val), undefined, context, serialize, logInfo);
                context.frames = oldFrames;
            });
        }
        else if (obj instanceof Promise) {
            // captures the frames up to this point. so we can give a good message if we
            // fail when we resume serializing this promise.
            const framesCopy = context.frames.slice();

            // Push handling this promise to the async work queue.  It will be processed in batches
            // after we've walked as much of the graph synchronously as possible.
            context.asyncWorkQueue.push(async () => {
                const val = await obj;
                const oldFrames = context.frames;
                context.frames = framesCopy;
                entry.promise = getOrCreateEntry(val, undefined, context, serialize, logInfo);
                context.frames = oldFrames;
            });
        }
        else if (obj instanceof Array) {
            // Recursively serialize elements of an array. Note: we use getOwnPropertyNames as the array
            // may be sparse and we want to properly respect that when serializing.
            entry.array = [];
            for (const key of Object.getOwnPropertyNames(obj)) {
                if (key !== "length" && obj.hasOwnProperty(key)) {
                    entry.array[<any>key] = getOrCreateEntry(
                        obj[<any>key], undefined, context, serialize, logInfo);
                }
            }
        }
        else if (Object.prototype.toString.call(obj) === "[object Arguments]") {
            // tslint:disable-next-line:max-line-length
            // From: https://stackoverflow.com/questions/7656280/how-do-i-check-whether-an-object-is-an-arguments-object-in-javascript
            entry.array = [];
            for (const elem of obj) {
                entry.array.push(getOrCreateEntry(elem, undefined, context, serialize, logInfo));
            }
        }
        else {
            // For all other objects, serialize out the properties we've been asked to serialize
            // out.
            serializeObject();
        }
    }

    function serializeObject() {
        // Serialize the set of property names asked for.  If we discover that any of them
        // use this/super, then go and reserialize all the properties.
        const serializeAll = serializeObjectWorker(capturedObjectProperties || []);
        if (serializeAll) {
            serializeObjectWorker([]);
        }
    }

    // Returns 'true' if the caller (serializeObjectAsync) should call this again, but without any
    // property filtering.
    function serializeObjectWorker(localCapturedPropertyChains: CapturedPropertyChain[]): boolean {
        const objectInfo: ObjectInfo = entry.object || { env: new Map() };
        entry.object = objectInfo;
        const environment = entry.object.env;

        if (localCapturedPropertyChains.length === 0) {
            serializeAllObjectProperties(environment);
            return false;
        } else {
            return serializeSomeObjectProperties(environment, localCapturedPropertyChains);
        }
    }

    // Serializes out all the properties of this object.  Used when we can't prove that
    // only a subset of properties are used on this object.
    function serializeAllObjectProperties(environment: PropertyMap) {
        // we wanted to capture everything (including the prototype chain)
        const ownPropertyNamesAndSymbols = getOwnPropertyNamesAndSymbols(obj);

        for (const keyOrSymbol of ownPropertyNamesAndSymbols) {
            // we're about to recurse inside this object.  In order to prever infinite
            // loops, put a dummy entry in the environment map.  That way, if we hit
            // this object again while recursing we won't try to generate this property.
            const keyEntry = getOrCreateEntry(keyOrSymbol, undefined, context, serialize, logInfo);
            if (!environment.has(keyEntry)) {
                environment.set(keyEntry, <any>undefined);

                const propertyInfo = getPropertyInfo(obj, keyOrSymbol);
                if (!propertyInfo) {
                    throw new Error("Could not find property info for 'own' property.");
                }

                const valEntry = getOrCreateEntry(
                    obj[keyOrSymbol], undefined, context, serialize, logInfo);

                // Now, replace the dummy entry with the actual one we want.
                environment.set(keyEntry, { info: propertyInfo, entry: valEntry });
            }
        }

        // If the object's __proto__ is not Object.prototype, then we have to capture what it
        // actually is.  On the other end, we'll use Object.create(deserializedProto) to set
        // things up properly.
        //
        // We don't need to capture the prototype if the user is not capturing 'this' either.
        if (!entry.object!.proto) {
            const proto = Object.getPrototypeOf(obj);
            if (proto !== Object.prototype) {
                entry.object!.proto = getOrCreateEntry(proto, undefined, context, serialize, logInfo);
            }
        }
    }

    // Serializes out only the subset of properties of this object that we have seen used
    // and have recorded in localCapturedPropertyChains
    function serializeSomeObjectProperties(
            environment: PropertyMap, localCapturedPropertyChains: CapturedPropertyChain[]): boolean {

        // validate our invariants.
        for (const chain of localCapturedPropertyChains) {
            if (chain.infos.length === 0) {
                throw new Error("Expected a non-empty chain.");
            }
        }

        // we only want to capture a subset of properties.  We can do this as long those
        // properties don't somehow end up involving referencing "this" in an 'invoked'
        // capacity (in which case we need to completely realize the object.
        //
        // this is slightly tricky as it's not obvious if a property is a getter/setter
        // and this is implicitly invoked just by access it.

        // Find the list of property names *directly* accessed off this object.
        const propChainFirstNames = new Set(localCapturedPropertyChains.map(
            chain => chain.infos[0].name));

        // Now process each top level property name accessed off of this object in turn. For
        // example, if we say "foo.bar.baz", "foo.bar.quux", "foo.ztesch", this would "bar" and
        // "ztesch".
        for (const propName of propChainFirstNames) {
            // Get the named chains starting with this prop name.  In the above example, if
            // this was "bar", then we would get "[bar, baz]" and [bar, quux].
            const propChains = localCapturedPropertyChains.filter(chain => chain.infos[0].name === propName);

            // Now, make an entry just for this name.
            const keyEntry = getOrCreateEntry(propName, undefined, context, serialize, logInfo);

            if (environment.has(keyEntry)) {
                continue;
            }

            // we're about to recurse inside this object.  In order to prevent infinite
            // loops, put a dummy entry in the environment map.  That way, if we hit
            // this object again while recursing we won't try to generate this property.
            environment.set(keyEntry, <any>undefined);
            const objPropValue = obj[propName];

            const propertyInfo = getPropertyInfo(obj, propName);
            if (!propertyInfo) {
                if (objPropValue !== undefined) {
                    throw new Error("Could not find property info for real property on object: " + propName);
                }

                // User code referenced a property not actually on the object at all.
                // So to properly represent that, we don't place any information about
                // this property on the object.
                environment.delete(keyEntry);
            } else {
                // Determine what chained property names we're accessing off of this sub-property.
                // if we have no sub property name chain, then indicate that with an empty array
                // so that we capture the entire object.
                //
                // i.e.: if we started with a.b.c.d, and we've finally gotten to the point where
                // we're serializing out the 'd' property, then we need to serialize it out fully
                // since there are no more accesses off of it.
                let nestedPropChains = propChains.map(chain => ({ infos: chain.infos.slice(1) }));
                if (nestedPropChains.some(chain => chain.infos.length === 0)) {
                    nestedPropChains = [];
                }

                // Note: objPropValue can be undefined here.  That's the case where the
                // object does have the property, but the property is just set to the
                // undefined value.
                const valEntry = getOrCreateEntry(
                    objPropValue, nestedPropChains, context, serialize, logInfo);

                const infos = propChains.map(chain => chain.infos[0]);
                if (propInfoUsesNonLexicalThis(infos, propertyInfo, valEntry)) {
                    // the referenced function captured 'this'.  Have to serialize out
                    // this entire object.  Undo the work we did to just serialize out a
                    // few properties.
                    environment.clear();

                    // Signal our caller to serialize the entire object.
                    return true;
                }

                // Now, replace the dummy entry with the actual one we want.
                environment.set(keyEntry, { info: propertyInfo, entry: valEntry });
            }
        }

        return false;
    }

    function propInfoUsesNonLexicalThis(
            capturedInfos: CapturedPropertyInfo[], propertyInfo: PropertyInfo | undefined, valEntry: Entry) {
        if (capturedInfos.some(info => info.invoked)) {
            // If the property was invoked, then we have to check if that property ends
            // up using this/super.  if so, then we actually have to serialize out this
            // object entirely.
            if (usesNonLexicalThis(valEntry)) {
                return true;
            }
        }

        // if we're accessing a getter/setter, and that getter/setter uses
        // 'this', then we need to serialize out this object entirely.

        if (usesNonLexicalThis(propertyInfo ? propertyInfo.get : undefined) ||
            usesNonLexicalThis(propertyInfo ? propertyInfo.set : undefined)) {

            return true;
        }

        return false;
    }

    function getPropertyInfo(on: any, key: PropertyKey): PropertyInfo | undefined {
        for (let current = on; current; current = Object.getPrototypeOf(current)) {
            const desc = Object.getOwnPropertyDescriptor(current, key);
            if (desc) {
                const propertyInfo = <PropertyInfo>{ hasValue: desc.value !== undefined };
                propertyInfo.configurable = desc.configurable;
                propertyInfo.enumerable = desc.enumerable;
                propertyInfo.writable = desc.writable;
                if (desc.get) {
                    propertyInfo.get = getOrCreateEntry(
                        desc.get, undefined, context, serialize, logInfo);
                }
                if (desc.set) {
                    propertyInfo.set = getOrCreateEntry(
                        desc.set, undefined, context, serialize, logInfo);
                }

                return propertyInfo;
            }
        }

        return undefined;
    }

    function usesNonLexicalThis(localEntry: Entry | undefined) {
        return localEntry && localEntry.function && localEntry.function.usesNonLexicalThis;
    }
}

// Is this a constructor derived from a noCapture constructor.  if so, we don't want to
// emit it.  We would be unable to actually hook up the "super()" call as one of the base
// constructors was set to not be captured.
function isDerivedNoCaptureConstructor(func: Function) {
    for (let current: any = func; current; current = Object.getPrototypeOf(current)) {
        if (current && current.doNotCapture) {
            return true;
        }
    }

    return false;
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
function findModuleName(obj: any): string | undefined  {
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

const legalNameRegex = /^[a-zA-Z_][0-9a-zA-Z_]*$/;
export function isLegalMemberName(n: string) {
    return legalNameRegex.test(n);
}

export function isLegalFunctionName(n: string) {
    if (!isLegalMemberName(n)) {
        return false;
    }

    const scanner = ts.createScanner(
        ts.ScriptTarget.Latest, /*skipTrivia:*/false, ts.LanguageVariant.Standard, n);
    const tokenKind = scanner.scan();
    if (tokenKind !== ts.SyntaxKind.Identifier &&
        tokenKind !== ts.SyntaxKind.ConstructorKeyword) {
        return false;
    }

    return true;
}
