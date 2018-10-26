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

// tslint:disable:max-line-length

import * as upath from "upath";
import { ResourceError } from "../../errors";
import * as resource from "../../resource";
import { CapturedPropertyChain, CapturedPropertyInfo, CapturedVariableMap, parseFunction } from "./parseFunction";
import { rewriteSuperReferences } from "./rewriteSuper";
import * as utils from "./utils";

import {
    FunctionMirror,
    getMirrorAsync,
    getMirrorMemberAsync,
    getPrototypeOfMirror,
    isStringMirror,
    isUndefinedOrNullMirror,
    isTruthy,
    isFalsy,
    Mirror } from "./mirrors";
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

    // An RegExp. Will be serialized as 'new RegExp(re.source, re.flags)'
    regexp?: RegExp;

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
    cache: Map<Mirror, Entry>;

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
    classInstanceMemberToSuperEntry: Map<FunctionMirror, Entry>;
    classStaticMemberToSuperEntry: Map<FunctionMirror, Entry>;

    // // The set of async jobs we have to complete after serializing the object graph. This happens
    // // when we encounter Promises/Outputs while walking the graph.  We'll add that work here and
    // // then process it at the end of the graph.  Note: as we hit those promises we may discover more
    // // work to be done.  So we'll just keep processing this this queue until there is nothing left
    // // in it.
    // asyncWorkQueue: (() => Promise<void>)[];

    // A list of 'simple' functions.  Simple functions do not capture anything, do not have any
    // special properties on them, and do not have a custom prototype.  If we run into multiple
    // functions that are simple, and share the same code, then we can just emit the function once
    // for them.  A good example of this is the __awaiter function.  Normally, there will be one
    // __awaiter per .js file that uses 'async/await'.  Instead of needing to generate serialized
    // functions for each of those, we can just serialize out the function once.
    simpleFunctions: FunctionInfo[];

    /**
     * The resource to log any errors we encounter against.
     */
    logResource: resource.Resource | undefined;
}

interface FunctionLocation {
    mirror: FunctionMirror;
    isArrowFunction?: boolean;
}

interface ContextFrame {
    functionLocation?: FunctionLocation;
    capturedFunctionName?: string;
    capturedVariableName?: string;
    capturedModule?: { name: string, mirror: Mirror };
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
    func: Function, serialize: (o: any) => boolean, logResource: resource.Resource | undefined): Promise<FunctionInfo> {

    const context: Context = {
        cache: new Map(),
        classInstanceMemberToSuperEntry: new Map(),
        classStaticMemberToSuperEntry: new Map(),
        frames: [],
        // asyncWorkQueue: [],
        simpleFunctions: [],
        logResource,
    };

    // Pre-populate our context's cache with global well-known values.  These are values for things
    // like global.Number, or Function.prototype.  Actually trying to serialize/deserialize these
    // would be a bad idea as that would mean once deserialized the objects wouldn't point to the
    // well known globals that were expected.  Furthermore, most of these are almost certain to fail
    // to serialize due to hitting things like native-builtins.
    const mirrorToEmitExprMap = await v8.getMirrorToEmitExprMap();
    for (const [mirror, expr] of mirrorToEmitExprMap) {
        context.cache.set(mirror, { expr });
    }

    const funcMirror = await getMirrorAsync(func);

    // Make sure this func is in the cache itself as we may hit it again while recursing.
    const entry: Entry = {};
    context.cache.set(funcMirror, entry);

    entry.function = await analyzeFunctionMirrorAsync(funcMirror, context, serialize);

    // await processAsyncWorkQueue();

    return entry.function;

    // async function processAsyncWorkQueue() {
    //     while (context.asyncWorkQueue.length > 0) {
    //         const queue = context.asyncWorkQueue;
    //         context.asyncWorkQueue = [];

    //         for (const work of queue) {
    //             await work();
    //         }
    //     }
    // }
}

// This function ends up capturing many external modules that cannot themselves be serialized.
// Do not allow it to be captured.
(<any>createFunctionInfoAsync).doNotCapture = true;

/**
 * analyzeFunctionInfoAsync does the work to create an asynchronous dataflow graph that resolves to a
 * final FunctionInfo.
 */
async function analyzeFunctionMirrorAsync(
        funcMirror: FunctionMirror, context: Context,
        serialize: (o: any) => boolean, logInfo?: boolean): Promise<FunctionInfo> {

    // logInfo = logInfo || func.name === "addHandler";

    // const { file, line, column } = await v8.getFunctionLocationAsync(funcMirror);
    const frame = { functionLocation: { mirror: funcMirror, isArrowFunction: false } };

    context.frames.push(frame);
    const result = await serializeWorkerAsync();
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

    async function serializeWorkerAsync(): Promise<FunctionInfo> {
        const funcEntry = context.cache.get(funcMirror);
        if (!funcEntry) {
            throw new Error("Entry for this this function was not created by caller");
        }

        // First, convert the js func object to a reasonable stringified version that we can operate on.
        // Importantly, this function helps massage all the different forms that V8 can produce to
        // either a "function (...) { ... }" form, or a "(...) => ..." form.  In other words, all
        // 'funky' functions (like classes and whatnot) will be transformed to reasonable forms we can
        // process down the pipeline.
        const [error, parsedFunction] = parseFunction(funcMirror.description);
        if (error) {
            throwSerializationError(funcMirror, context, error);
        }

        const funcExprWithName = parsedFunction.funcExprWithName;
        const functionDeclarationName = parsedFunction.functionDeclarationName;
        frame.functionLocation.isArrowFunction = parsedFunction.isArrowFunction;

        const capturedValues: PropertyMap = new Map();
        await processCapturedVariablesAsync(parsedFunction.capturedVariables.required, /*throwOnFailure:*/ true);
        await processCapturedVariablesAsync(parsedFunction.capturedVariables.optional, /*throwOnFailure:*/ false);

        const functionInfo: FunctionInfo = {
            code: parsedFunction.funcExprWithoutName,
            capturedValues: capturedValues,
            env: new Map(),
            usesNonLexicalThis: parsedFunction.usesNonLexicalThis,
            name: functionDeclarationName,
        };

        const protoMirror = await getPrototypeOfMirror(funcMirror);
        const isAsyncFunction = await computeIsAsyncFunction(funcMirror);

        // Ensure that the prototype of this function is properly serialized as well. We only need to do
        // this for functions with a custom prototype (like a derived class constructor, or a function
        // that a user has explicit set the prototype for). Normal functions will pick up
        // Function.prototype by default, so we don't need to do anything for them.
        if (protoMirror !== await getMirrorAsync(Function.prototype) &&
            !isAsyncFunction &&
            !await isDerivedNoCaptureConstructorAsync(funcMirror)) {

            const protoEntry = await getOrCreateEntryAsync(protoMirror, undefined, context, serialize, logInfo);
            functionInfo.proto = protoEntry;

            if (funcMirror.description.startsWith("class ")) {
                // This was a class (which is effectively synonymous with a constructor-function).
                // We also know that it's a derived class because of the `proto !==
                // Function.prototype` check above.  (The prototype of a non-derived class points at
                // Function.prototype).
                //
                // they're a bit trickier to serialize than just a straight function. Specifically,
                // we have to keep track of the inheritance relationship between classes.  That way
                // if any of the class members references 'super' we'll be able to rewrite it
                // accordingly (since we emit classes as Functions)
                processDerivedClassConstructor(protoEntry);

                // Because this was was class constructor function, rewrite any 'super' references
                // in it do its derived type if it has one.
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
                await getOrCreateEntryAsync(keyOrSymbol, undefined, context, serialize, logInfo),
                { entry: await getOrCreateEntryAsync(funcProp, undefined, context, serialize, logInfo) });
        }

        const superEntry = context.classInstanceMemberToSuperEntry.get(funcMirror) ||
                           context.classStaticMemberToSuperEntry.get(funcMirror);
        if (superEntry) {
            // this was a class constructor or method.  We need to put a special __super
            // entry into scope, and then rewrite any calls to super() to refer to it.
            capturedValues.set(
                await getOrCreateNameEntryAsync("__super", undefined, context, serialize, logInfo),
                { entry: superEntry });

            functionInfo.code = rewriteSuperReferences(
                funcExprWithName!, context.classStaticMemberToSuperEntry.has(funcMirror));
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
                await getOrCreateNameEntryAsync(functionDeclarationName, undefined, context, serialize, logInfo),
                { entry: funcEntry });
        }

        return functionInfo;

        async function processCapturedVariablesAsync(
            capturedVariables: CapturedVariableMap, throwOnFailure: boolean): Promise<void> {

            for (const name of capturedVariables.keys()) {
                let valueMirror: Mirror;
                try {
                    valueMirror = await v8.lookupCapturedVariableAsync(funcMirror, name, throwOnFailure);
                }
                catch (err) {
                    return throwSerializationError(funcMirror, context, err.message);
                    // TODO(cyrusn): should be able to remove this.
                    // throw err;
                }

                const moduleName = await findNormalizedModuleNameAsync(valueMirror);
                const frameLength = context.frames.length;
                if (moduleName) {
                    context.frames.push({ capturedModule: { name: moduleName, value: value } });
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

                await processCapturedVariableAsync(capturedVariables, name, value);

                // Only if we pushed a frame on should we pop it off.
                if (context.frames.length !== frameLength) {
                    context.frames.pop();
                }
            }
        }

        async function processCapturedVariableAsync(
            capturedVariables: CapturedVariableMap, name: string, value: any) {

            const properties = capturedVariables.get(name);
            const serializedName = await getOrCreateNameEntryAsync(name, undefined, context, serialize, logInfo);

            // try to only serialize out the properties that were used by the user's code.
            const serializedValue = await getOrCreateEntryAsync(value, properties, context, serialize, logInfo);

            capturedValues.set(serializedName, { entry: serializedValue });
        }
    }

    function processDerivedClassConstructor(protoEntry: Entry) {
        // Map from derived class' constructor and members, to the entry for the base class (i.e.
        // the base class' constructor function). We'll use this when serializing out those members
        // to rewrite any usages of 'super' appropriately.

        // We're processing the derived class constructor itself.  Just map it directly to the base
        // class function.
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

async function computeIsAsyncFunction(funcMirror: Mirror): Promise<boolean> {
    // Note, i can't think of a better way to determine this.  This is particularly hard because
    // we can't even necessary refer to async function objects here as this code is rewritten by
    // TS, converting all async functions to non async functions.
    const funcConstructorMirror = await getMirrorMemberAsync(funcMirror, "constructor");
    if (isFalsy(funcConstructorMirror)) {
        return false;
    }

    const constructorNameMirror = await getMirrorMemberAsync(funcConstructorMirror, "name");
    if (!isStringMirror(constructorNameMirror)) {
        return false;
    }

    return constructorNameMirror.value === "AsyncFunction";
}

function getOwnPropertyNamesAndSymbols(obj: any): (string | symbol)[] {
    const names: (string | symbol)[] = Object.getOwnPropertyNames(obj);
    return names.concat(Object.getOwnPropertySymbols(obj));
}

function throwSerializationError(
    funcMirror: FunctionMirror, context: Context, info: string): never {

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
        else if (frame.capturedModule) {
            if (i === n - 1) {
                message += `module '${frame.capturedModule.name}'\n`;
            }
            else {
                message += `module '${frame.capturedModule.name}' which indirectly referenced\n`;
            }
        }
        else if (frame.capturedVariableName) {
            message += `variable '${frame.capturedVariableName}' which indirectly referenced\n`;
        }
    }

    message += "  ".repeat(i) + info + "\n\n";
    message += getTrimmedFunctionCode(funcMirror);

    const moduleIndex = context.frames.findIndex(
            f => f.capturedModule !== undefined);

    if (moduleIndex >= 0) {
        const module = context.frames[moduleIndex].capturedModule!;
        const moduleName = module.name;
        message += "\n";

        if (module.value.deploymentOnlyModule) {
            message += `Module '${moduleName}' is a 'deployment only' module. In general these cannot be captured inside a 'run time' function.`;
        }
        else {
            const functionLocation = context.frames[moduleIndex - 1].functionLocation!;
            const location = getFunctionLocation(functionLocation);
            message += `Capturing modules can sometimes cause problems.
Consider using import('${moduleName}') or require('${moduleName}') inside ${location}`;
        }
    }

    // Hide the stack when printing out the closure serialization error.  We don't want both the
    // closure serialization object stack *and* the function execution stack.  Furthermore, there
    // is enough information about the Function printed (both line/col, and a few lines of its
    // text) to give the user the appropriate info for fixing.
    throw new ResourceError(message, context.logResource, /*hideStack:*/true);
}

function getTrimmedFunctionCode(funcMirror: FunctionMirror): string {
    const funcString = funcMirror.description;

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
        name += `: ${upath.basename(loc.file)}(${loc.line + 1},${loc.column})`;
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

    if (loc.funcMirror.name) {
        return loc.funcMirror.name;
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

async function getOrCreateNameEntryAsync(
    name: string, capturedObjectProperties: CapturedPropertyChain[] | undefined,
    context: Context,
    serialize: (o: any) => boolean,
    logInfo: boolean | undefined): Promise<Entry> {

    const mirror = await getMirrorAsync(name);
    return await getOrCreateEntryAsync(mirror, capturedObjectProperties, context, serialize, logInfo);
}

/**
 * serializeAsync serializes an object, deeply, into something appropriate for an environment
 * entry.  If propNames is provided, and is non-empty, then only attempt to serialize out those
 * specific properties.  If propNames is not provided, or is empty, serialize out all properties.
 */
async function getOrCreateEntryAsync(
        mirror: Mirror, capturedObjectProperties: CapturedPropertyChain[] | undefined,
        context: Context,
        serialize: (o: any) => boolean,
        logInfo: boolean | undefined): Promise<Entry> {

    // See if we have a cache hit.  If yes, use the object as-is.
    let entry = context.cache.get(mirror)!;
    if (entry) {
        // Even though we've already serialized out this object, it might be the case
        // that we serialized out a different set of properties than the current set
        // we're being asked to serialize.  So we have to make sure that all these props
        // are actually serialized.
        if (entry.object) {
            await serializeObjectAsync();
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
    context.cache.set(mirror, entry);
    await dispatchAnyAsync();
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

    async function dispatchAnyAsync(): Promise<void> {
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
        else if (obj instanceof RegExp) {
            entry.regexp = obj;
            return;
        }

        const normalizedModuleName = findNormalizedModuleName(obj);
        if (normalizedModuleName) {
            await captureModuleAsync(normalizedModuleName);
        }
        else if (obj instanceof Function) {
            // Serialize functions recursively, and store them in a closure property.
            entry.function = await analyzeFunctionInfoAsync(obj, context, serialize, logInfo);
        }
        else if (resource.Output.isInstance(obj)) {
            const val = await obj.promise();
            entry.output = await getOrCreateEntryAsync(new SerializedOutput(val), undefined, context, serialize, logInfo);

            // // captures the frames up to this point. so we can give a good message if we
            // // fail when we resume serializing this promise.
            // const framesCopy = context.frames.slice();

            // // Push handling this output to the async work queue.  It will be processed in batches
            // // after we've walked as much of the graph synchronously as possible.
            // context.asyncWorkQueue.push(async () => {
            //     const val = await obj.promise();
            //     const oldFrames = context.frames;
            //     context.frames = framesCopy;
            //     entry.output = getOrCreateEntry(new SerializedOutput(val), undefined, context, serialize, logInfo);
            //     context.frames = oldFrames;
            // });
        }
        else if (obj instanceof Promise) {
            // // captures the frames up to this point. so we can give a good message if we
            // // fail when we resume serializing this promise.
            // const framesCopy = context.frames.slice();

            // // Push handling this promise to the async work queue.  It will be processed in batches
            // // after we've walked as much of the graph synchronously as possible.
            // context.asyncWorkQueue.push(async () => {
            //     const val = await obj;
            //     const oldFrames = context.frames;
            //     context.frames = framesCopy;
            //     entry.promise = getOrCreateEntry(val, undefined, context, serialize, logInfo);
            //     context.frames = oldFrames;
            // });

            const val = await obj;
            entry.promise = await getOrCreateEntryAsync(val, undefined, context, serialize, logInfo);
        }
        else if (obj instanceof Array) {
            // Recursively serialize elements of an array. Note: we use getOwnPropertyNames as the array
            // may be sparse and we want to properly respect that when serializing.
            entry.array = [];
            for (const key of Object.getOwnPropertyNames(obj)) {
                if (key !== "length" && obj.hasOwnProperty(key)) {
                    entry.array[<any>key] = await getOrCreateEntryAsync(
                        obj[<any>key], undefined, context, serialize, logInfo);
                }
            }
        }
        else if (Object.prototype.toString.call(obj) === "[object Arguments]") {
            // From: https://stackoverflow.com/questions/7656280/how-do-i-check-whether-an-object-is-an-arguments-object-in-javascript
            entry.array = [];
            for (const elem of obj) {
                entry.array.push(await getOrCreateEntryAsync(elem, undefined, context, serialize, logInfo));
            }
        }
        else {
            // For all other objects, serialize out the properties we've been asked to serialize
            // out.
            await serializeObjectAsync();
        }
    }

    async function serializeObjectAsync() {
        // Serialize the set of property names asked for.  If we discover that any of them
        // use this/super, then go and reserialize all the properties.
        const serializeAll = await serializeObjectWorkerAsync(capturedObjectProperties || []);
        if (serializeAll) {
            await serializeObjectWorkerAsync([]);
        }
    }

    // Returns 'true' if the caller (serializeObjectAsync) should call this again, but without any
    // property filtering.
    async function serializeObjectWorkerAsync(localCapturedPropertyChains: CapturedPropertyChain[]): Promise<boolean> {
        const objectInfo: ObjectInfo = entry.object || { env: new Map() };
        entry.object = objectInfo;
        const environment = entry.object.env;

        if (localCapturedPropertyChains.length === 0) {
            await serializeAllObjectPropertiesAsync(environment);
            return false;
        } else {
            return await serializeSomeObjectPropertiesAsync(environment, localCapturedPropertyChains);
        }
    }

    // Serializes out all the properties of this object.  Used when we can't prove that
    // only a subset of properties are used on this object.
    async function serializeAllObjectPropertiesAsync(environment: PropertyMap) {
        // we wanted to capture everything (including the prototype chain)
        const ownPropertyNamesAndSymbols = getOwnPropertyNamesAndSymbols(obj);

        for (const keyOrSymbol of ownPropertyNamesAndSymbols) {
            // we're about to recurse inside this object.  In order to prever infinite
            // loops, put a dummy entry in the environment map.  That way, if we hit
            // this object again while recursing we won't try to generate this property.
            const keyEntry = await getOrCreateEntryAsync(keyOrSymbol, undefined, context, serialize, logInfo);
            if (!environment.has(keyEntry)) {
                environment.set(keyEntry, <any>undefined);

                const propertyInfo = await getPropertyInfoAsync(obj, keyOrSymbol);
                if (!propertyInfo) {
                    throw new Error("Could not find property info for 'own' property.");
                }

                const valEntry = await getOrCreateEntryAsync(
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
                entry.object!.proto = await getOrCreateEntryAsync(
                    proto, undefined, context, serialize, logInfo);
            }
        }
    }

    // Serializes out only the subset of properties of this object that we have seen used
    // and have recorded in localCapturedPropertyChains
    async function serializeSomeObjectPropertiesAsync(
            environment: PropertyMap, localCapturedPropertyChains: CapturedPropertyChain[]): Promise<boolean> {

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
            const keyEntry = await getOrCreateNameEntryAsync(propName, undefined, context, serialize, logInfo);

            if (environment.has(keyEntry)) {
                continue;
            }

            // we're about to recurse inside this object.  In order to prevent infinite
            // loops, put a dummy entry in the environment map.  That way, if we hit
            // this object again while recursing we won't try to generate this property.
            environment.set(keyEntry, <any>undefined);
            const objPropValue = obj[propName];

            const propertyInfo = await getPropertyInfoAsync(obj, propName);
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
                const valEntry = await getOrCreateEntryAsync(
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

    async function getPropertyInfoAsync(on: any, key: PropertyKey): Promise<PropertyInfo | undefined> {
        for (let current = on; current; current = Object.getPrototypeOf(current)) {
            const desc = Object.getOwnPropertyDescriptor(current, key);
            if (desc) {
                const propertyInfo = <PropertyInfo>{ hasValue: desc.value !== undefined };
                propertyInfo.configurable = desc.configurable;
                propertyInfo.enumerable = desc.enumerable;
                propertyInfo.writable = desc.writable;
                if (desc.get) {
                    propertyInfo.get = await getOrCreateEntryAsync(
                        desc.get, undefined, context, serialize, logInfo);
                }
                if (desc.set) {
                    propertyInfo.set = await getOrCreateEntryAsync(
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

    async function captureModuleAsync(normalizedModuleName: string): Promise<void> {
        // Splitting on "/" is safe to do as this module name is already in a normalized form.
        const moduleParts = normalizedModuleName.split("/");

        const nodeModulesSegment = "node_modules";
        const nodeModulesSegmentIndex = moduleParts.findIndex(v => v === nodeModulesSegment);
        const isInNodeModules = nodeModulesSegmentIndex >= 0;

        const isLocalModule = normalizedModuleName.startsWith(".") && !isInNodeModules;

        if (obj.deploymentOnlyModule || isLocalModule) {
            // Try to serialize deployment-time and local-modules by-value.
            //
            // A deployment-only modules can't ever be successfully 'required' on the 'inside'. But
            // parts of it may be serializable on the inside (i.e. pulumi.Config).  So just try to
            // capture this as a value.  If it fails, we will give the user a good message.
            // Otherwise, it may succeed if the user is only using a small part of the API that is
            // serializable (like pulumi.Config)
            //
            // Or this is a reference to a local module (i.e. starts with '.', but isn't in
            // /node_modules/ somewhere). Always capture the local module as a value.  We do this
            // because capturing as a reference (i.e. 'require(...)') has the following problems:
            //
            // 1. 'require(...)' will not work at run-time, because the user's code will not be
            //    serialized in a way that can actually be require'd (i.e. it is not ) serialized
            //    into any sort of appropriate file/folder structure for those 'require's to work.
            //
            // 2. if we stop here and capture as a reference, then we won't actually see and walk
            //    the code that exists in those local modules (direct or transitive). So we won't
            //    actually generate the serialized code for the functions or values in that module.
            //    This will also lead to code that simply will not work at run-time.
            await serializeObjectAsync();
        }
        else  {
            // If the path goes into node_modules, strip off the node_modules part. This will help
            // ensure that lookup of those modules will work on the cloud-side even if the module
            // isn't in a relative node_modules directory.  For example, this happens with aws-sdk.
            // It ends up actually being in /var/runtime/node_modules inside aws lambda.
            //
            // This also helps ensure that modules that are 'yarn link'ed are found properly. The
            // module path we have may be on some non-local path due to the linking, however this
            // will ensure that the module-name we load is a simple path that can be found off the
            // node_modules that we actually upload with our serialized functions.
            entry.module = isInNodeModules
                ? upath.join(...moduleParts.slice(nodeModulesSegmentIndex + 1))
                : normalizedModuleName;
        }
    }
}

// Is this a constructor derived from a noCapture constructor.  if so, we don't want to
// emit it.  We would be unable to actually hook up the "super()" call as one of the base
// constructors was set to not be captured.
async function isDerivedNoCaptureConstructorAsync(func: FunctionMirror) {
    for (let current: Mirror = func; isTruthy(current); current = await getPrototypeOfMirror(current)) {
        if (isTruthy(await getMirrorMemberAsync(current, "doNotCapture"))) {
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

// findNormalizedModuleName attempts to find a global name bound to the object, which can be used as
// a stable reference across serialization.  For built-in modules (i.e. "os", "fs", etc.) this will
// return that exact name of the module.  Otherwise, this will return the relative path to the
// module from the current working directory of the process.  This will normally be something of the
// form ./node_modules/<package_name>...
//
// This function will also always return modules in a normalized form (i.e. all path components will
// be '/').
async function findNormalizedModuleNameAsync(obj: Mirror): Promise<string | undefined> {
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
        const exportsMirror = await getMirrorAsync(require.cache[path].exports);
        if (exportsMirror === obj) {
            // Rewrite the path to be a local module reference relative to the current working
            // directory.
            const modPath = upath.relative(process.cwd(), path);
            return "./" + modPath;
        }
    }

    // Else, return that no global name is available for this object.
    return undefined;
}
