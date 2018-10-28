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
    callAccessorOn,
    callFunctionOn,
    FunctionDetails,
    FunctionMirror,
    getFunctionDetailsAsync,
    getMirrorAsync,
    getNameOrSymbol,
    getOwnPropertyAsync,
    getOwnPropertyDescriptorsAsync,
    getPromiseMirrorValueAsync,
    getPrototypeOfMirrorAsync,
    isArrayMirror,
    isBooleanMirror,
    isFalsy,
    isFunctionMirror,
    isMirror,
    isNullMirror,
    isNumberMirror,
    isPromiseMirror,
    isRegExpMirror,
    isStringMirror,
    isStringValue,
    isTruthy,
    isUndefinedMirror,
    isUndefinedOrNullMirror,
    lookupCapturedVariableAsync,
    Mirror,
    MirrorPropertyDescriptor,
} from "./mirrors";

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
    regexp?: { source: string, flags: string };

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
    details: FunctionDetails;
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
 *
 * IMPORTANT: Do not change the structure of this type.  Closure serialization code takes a
 * dependency on the actual shape (including the names of properties like 'value').
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

    // Initialize our Context object.  It is effectively used to keep track of the work we're doing
    // as well as to keep track of the graph as we're walking it so we don't infinitely recurse.
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
    await addEntriesForWellKnownGlobalObjectsAsync();

    const funcMirror = await getMirrorAsync(func);

    // Make sure this func is in the cache itself as we may hit it again while recursing.
    const entry: Entry = {};
    context.cache.set(funcMirror, entry);

    entry.function = await analyzeFunctionMirrorAsync(funcMirror, context, serialize);

    // await processAsyncWorkQueue();

    return entry.function;

    async function addEntriesForWellKnownGlobalObjectsAsync() {
        const seenGlobalObjects = new Set<any>();

        // Front load these guys so we prefer emitting code that references them directly,
        // instead of in unexpected ways.  i.e. we'd prefer to have Number.prototype vs
        // Object.getPrototypeOf(Infinity) (even though they're the same thing.)

        await addGlobalInfoAsync("Object");
        await addGlobalInfoAsync("Function");
        await addGlobalInfoAsync("Array");
        await addGlobalInfoAsync("Number");
        await addGlobalInfoAsync("String");

        for (let current = global; current; current = Object.getPrototypeOf(current)) {
            for (const key of Object.getOwnPropertyNames(current)) {
                // "GLOBAL" and "root" are deprecated and give warnings if you try to access them.  So
                // just skip them.
                if (key !== "GLOBAL" && key !== "root") {
                    await addGlobalInfoAsync(key);
                }
            }
        }

        // Add information so that we can properly serialize over generators/iterators.
        await addGeneratorEntriesAsync();
        await addEntriesAsync(Symbol.iterator, "Symbol.iterator");

        return;

        async function addEntriesAsync(val: any, emitExpr: string) {
            if (val === undefined || val === null) {
                return;
            }

            // No need to add values twice.  Ths can happen as we walk the global namespace and
            // sometimes run into multiple names aliasing to the same value.
            if (seenGlobalObjects.has(val)) {
                return;
            }

            seenGlobalObjects.add(val);
            context.cache.set(await getMirrorAsync(val), { expr: emitExpr });
        }

        async function addGlobalInfoAsync(key: string) {
            const globalObj = (<any>global)[key];
            const text = utils.isLegalMemberName(key) ? `global.${key}` : `global["${key}"]`;

            if (globalObj !== undefined && globalObj !== null) {
                await addEntriesAsync(globalObj, text);
                await addEntriesAsync(Object.getPrototypeOf(globalObj), `Object.getPrototypeOf(${text})`);
                await addEntriesAsync(globalObj.prototype, `${text}.prototype`);
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
        async function addGeneratorEntriesAsync() {
            // tslint:disable-next-line:no-empty
            const emptyGenerator = function* (): any { };

            await addEntriesAsync(
                Object.getPrototypeOf(emptyGenerator),
                "Object.getPrototypeOf(function*(){})");

            await addEntriesAsync(
                Object.getPrototypeOf(emptyGenerator.prototype),
                "Object.getPrototypeOf((function*(){}).prototype)");
        }
    }
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

    if (!isMirror(funcMirror)) {
        throw new Error("Was not passed a mirror to analyzeFunctionMirrorAsync: " + JSON.stringify(funcMirror));
    }
    if (!isFunctionMirror(funcMirror)) {
        throw new Error("Was not passed a function mirror to analyzeFunctionMirrorAsync: " + JSON.stringify(funcMirror));
    }

    const funcDetails = await getFunctionDetailsAsync(funcMirror);
    const frame = { functionLocation: { mirror: funcMirror, details: funcDetails, isArrowFunction: false } };

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
        const [error, parsedFunction] = parseFunction(funcDetails.code);
        if (error) {
            await throwSerializationErrorAsync(funcDetails, context, error);
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

        const protoMirror = await getPrototypeOfMirrorAsync(funcMirror);
        const isAsyncFunction = await computeIsAsyncFunction(funcMirror);

        const globalFunctionPrototypeMirror = await getMirrorAsync(Function.prototype);

        // Ensure that the prototype of this function is properly serialized as well. We only need to do
        // this for functions with a custom prototype (like a derived class constructor, or a function
        // that a user has explicit set the prototype for). Normal functions will pick up
        // Function.prototype by default, so we don't need to do anything for them.
        if (protoMirror !== globalFunctionPrototypeMirror &&
            !isAsyncFunction &&
            !await isDerivedNoCaptureConstructorAsync(funcMirror)) {

            console.log(`Object.getPrototypeOf(${JSON.stringify(funcMirror)}) is ${JSON.stringify(protoMirror)} which wasn't ${JSON.stringify(globalFunctionPrototypeMirror)}`);

            const protoEntry = await getOrCreateEntryAsync(protoMirror, undefined, context, serialize, logInfo);
            functionInfo.proto = protoEntry;

            if (funcDetails.code.startsWith("class ")) {
                // console.log("Processing class: " + funcMirror.description);

                // This was a class (which is effectively synonymous with a constructor-function).
                // We also know that it's a derived class because of the `proto !==
                // Function.prototype` check above.  (The prototype of a non-derived class points at
                // Function.prototype).
                //
                // they're a bit trickier to serialize than just a straight function. Specifically,
                // we have to keep track of the inheritance relationship between classes.  That way
                // if any of the class members references 'super' we'll be able to rewrite it
                // accordingly (since we emit classes as Functions)
                await processDerivedClassConstructorAsync(protoEntry);

                // Because this was was class constructor function, rewrite any 'super' references
                // in it do its derived type if it has one.
                functionInfo.code = rewriteSuperReferences(funcExprWithName!, /*isStatic*/ false);
            }
        }

        // capture any properties placed on the function itself.  Don't bother with
        // "length/name" as those are not things we can actually change.
        for (const descriptor of await getOwnPropertyDescriptorsAsync(funcMirror)) {
            if (isStringValue(descriptor.name, "length") || isStringValue(descriptor.name, "name")) {
                continue;
            }

            const funcPropMirror = await getOwnPropertyAsync(funcMirror, descriptor);

            // We don't need to emit code to serialize this function's .prototype object
            // unless that .prototype object was actually changed.
            //
            // In other words, in general, we will not emit the prototype for a normal
            // 'function foo() {}' declaration.  but we will emit the prototype for the
            // constructor function of a class.
            if (isStringValue(descriptor.name, "prototype") &&
                await isDefaultFunctionPrototypeAsync(funcMirror, funcPropMirror)) {

                continue;
            }

            functionInfo.env.set(
                await getOrCreateEntryAsync(getNameOrSymbol(descriptor), undefined, context, serialize, logInfo),
                { entry: await getOrCreateEntryAsync(funcPropMirror, undefined, context, serialize, logInfo) });
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
                    valueMirror = await lookupCapturedVariableAsync(funcMirror, name, throwOnFailure);
                }
                catch (err) {
                    return await throwSerializationErrorAsync(funcDetails, context, err.message);
                    // TODO(cyrusn): should be able to remove this.
                    // throw err;
                }

                const moduleName = await findNormalizedModuleNameAsync(valueMirror);
                const frameLength = context.frames.length;
                if (moduleName) {
                    context.frames.push({ capturedModule: { name: moduleName, mirror: valueMirror } });
                }
                else if (isFunctionMirror(valueMirror)) {
                    // Only bother pushing on context frame if the name of the variable
                    // we captured is different from the name of the function.  If the
                    // names are the same, this is a direct reference, and we don't have
                    // to list both the name of the capture and of the function.  if they
                    // are different, it's an indirect reference, and the name should be
                    // included for clarity.
                    const funcNameMirror = await callAccessorOn(valueMirror, "name");
                    if (isStringValue(funcNameMirror, name)) {
                        context.frames.push({ capturedFunctionName: name });
                    }
                }
                else {
                    context.frames.push({ capturedVariableName: name });
                }

                await processCapturedVariableAsync(capturedVariables, name, valueMirror);

                // Only if we pushed a frame on should we pop it off.
                if (context.frames.length !== frameLength) {
                    context.frames.pop();
                }
            }
        }

        async function processCapturedVariableAsync(
            capturedVariables: CapturedVariableMap, name: string, valueMirror: Mirror) {

            const properties = capturedVariables.get(name);
            const serializedName = await getOrCreateNameEntryAsync(name, undefined, context, serialize, logInfo);

            // try to only serialize out the properties that were used by the user's code.
            const serializedValue = await getOrCreateEntryAsync(valueMirror, properties, context, serialize, logInfo);

            capturedValues.set(serializedName, { entry: serializedValue });
        }
    }

    async function processDerivedClassConstructorAsync(protoEntry: Entry) {
        // Map from derived class' constructor and members, to the entry for the base class (i.e.
        // the base class' constructor function). We'll use this when serializing out those members
        // to rewrite any usages of 'super' appropriately.

        // We're processing the derived class constructor itself.  Just map it directly to the base
        // class function.
        context.classInstanceMemberToSuperEntry.set(funcMirror, protoEntry);

        // Also, make sure our methods can also find this entry so they too can refer to
        // 'super'.
        for (const descriptor of await getOwnPropertyDescriptorsAsync(funcMirror)) {
            if (isStringValue(descriptor.name, "length") ||
                isStringValue(descriptor.name, "name") ||
                isStringValue(descriptor.name, "prototype")) {

                continue;
            }

            // static method.
            const classProp = await getOwnPropertyAsync(funcMirror, descriptor);
            addIfFunction(classProp, /*isStatic*/ true);
        }

        const funcPrototypeMirror = await callAccessorOn(funcMirror, "prototype");
        for (const descriptor of await getOwnPropertyDescriptorsAsync(funcPrototypeMirror)) {
            // instance method.
            const classProp = await getOwnPropertyAsync(funcPrototypeMirror, descriptor);
            addIfFunction(classProp, /*isStatic*/ false);
        }

        return;

        function addIfFunction(prop: Mirror, isStatic: boolean) {
            if (isFunctionMirror(prop)) {
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
    const funcConstructorMirror = await callAccessorOn(funcMirror, "constructor");
    if (isFalsy(funcConstructorMirror)) {
        return false;
    }

    const constructorNameMirror = await callAccessorOn(funcConstructorMirror, "name");
    if (!isStringMirror(constructorNameMirror)) {
        return false;
    }

    return constructorNameMirror.value === "AsyncFunction";
}

async function throwSerializationErrorAsync(
    funcDetails: FunctionDetails, context: Context, info: string): Promise<never> {

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
    message += getTrimmedFunctionCode(funcDetails);

    const moduleIndex = context.frames.findIndex(
            f => f.capturedModule !== undefined);

    if (moduleIndex >= 0) {
        const module = context.frames[moduleIndex].capturedModule!;
        const moduleName = module.name;
        message += "\n";

        if (await hasTruthyMemberAsync(module.mirror, "deploymentOnlyModule")) {
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

function getTrimmedFunctionCode(funcDetails: FunctionDetails): string {
    const funcString = funcDetails.code;

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

function getFunctionLocation(funcLoc: FunctionLocation): string {
    const loc = funcLoc.details.location;
    let name = "'" + getFunctionName(funcLoc) + "'";
    if (loc.file) {
        name += `: ${upath.basename(loc.file)}(${loc.line + 1},${loc.column})`;
    }

    const prefix = funcLoc.isArrowFunction ? "" : "function ";
    return prefix + name;
}

function getFunctionName(loc: FunctionLocation): string {
    if (loc.isArrowFunction) {
        let funcString = loc.details.code;

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

    if (loc.details.name) {
        return loc.details.name;
    }

    return "<anonymous>";
}

async function isDefaultFunctionPrototypeAsync(funcMirror: FunctionMirror, prototypePropMirror: Mirror) {
    // The initial value of prototype on any newly-created Function instance is a new instance of
    // Object, but with the own-property 'constructor' set to point back to the new function.
    if (isTruthy(prototypePropMirror) &&
        await callAccessorOn(prototypePropMirror, "constructor") === funcMirror) {

        const descriptors = await getOwnPropertyDescriptorsAsync(prototypePropMirror);
        return descriptors.length === 1 &&
               descriptors[0].name &&
               descriptors[0].name.value === "constructor";
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

    if (!isMirror(mirror)) {
        throw new Error("Was not passed a mirror to getOrCreateEntryAsync: " + JSON.stringify(mirror));
    }

    // Check if this is a special number that we cannot json serialize.  Instead, we'll just inject
    // the code necessary to represent the number on the other side.  Note: we have to do this
    // before we do *anything* else.  This is because these special numbers don't even work in maps
    // properly.  So, if we lookup the value in a map, we may get the cached value for something
    // else *entirely*.  For example, 0 and -0 will map to the same entry.
    if (isNumberMirror(mirror)) {
        if (mirror.unserializableValue) {
            return { expr: mirror.unserializableValue };
        }

        // Not special, just use normal json serialization.
        return { json: mirror.value };
    }

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

    if (isFunctionMirror(mirror) && await hasTruthyMemberAsync(mirror, "doNotCapture")) {
        // If we get a function we're not supposed to capture, then actually just serialize
        // out a function that will throw at runtime so the user can understand the problem
        // better.
        const details = await getFunctionDetailsAsync(mirror);
        const funcName = details.name || "anonymous";
        const funcCode = getTrimmedFunctionCode(details);

        const message =
            `Function '${funcName}' cannot be called at runtime. ` +
            `It can only be used at deployment time.\n\n${funcCode}`;
        const errorFunc = () => { throw new Error(message); };

        mirror = await getMirrorAsync(errorFunc);
    }

    // We may be processing recursive objects.  Because of that, we preemptively put a placeholder
    // entry in the cache.  That way, if we encounter this obj again while recursing we can just
    // return that placeholder.
    entry = {};
    context.cache.set(mirror, entry);
    await dispatchAnyAsync();
    return entry;

    async function doNotCaptureAsync(): Promise<boolean> {
        if (!serialize(mirror)) {
            // caller explicitly does not want us to capture this value.
            return true;
        }

        if (await hasTruthyMemberAsync(mirror, "doNotCapture")) {
            // object has set itself as something that should not be captured.
            return true;
        }

        if (isFunctionMirror(mirror) &&
            await isDerivedNoCaptureConstructorAsync(mirror)) {

            // this was a constructor that derived from something that should not be captured.
            return true;
        }

        return false;
    }

    async function dispatchAnyAsync(): Promise<void> {
        if (await doNotCaptureAsync()) {
            // We do not want to capture this object.  Explicit set .json to undefined so
            // that we will see that the property is set and we will simply roundtrip this
            // as the 'undefined value.
            entry.json = undefined;
            return;
        }

        if (isUndefinedMirror(mirror) ||
            isNullMirror(mirror) ||
            isBooleanMirror(mirror) ||
            isStringMirror(mirror)) {

                // Serialize primitives as-is.
            entry.json = mirror.value;
            return;
        }
        else if (isRegExpMirror(mirror)) {
            const sourceMirror = await callAccessorOn(mirror, "source");
            const flagsMirror = await callAccessorOn(mirror, "flags");
            if (!isStringMirror(sourceMirror) || !isStringMirror(flagsMirror)) {
                throw new Error("Got a regexp without 'source' or 'flags': " + JSON.stringify(mirror));
            }

            entry.regexp = { source: sourceMirror.value, flags: flagsMirror.value };
            return;
        }

        const normalizedModuleName = await findNormalizedModuleNameAsync(mirror);
        if (normalizedModuleName) {
            await captureModuleAsync(normalizedModuleName);
        }
        else if (isFunctionMirror(mirror)) {
            // Serialize functions recursively, and store them in a closure property.
            entry.function = await analyzeFunctionMirrorAsync(mirror, context, serialize, logInfo);
        }
        else if (await isOutputAsync(mirror)) {
            entry.output = await createOutputEntryAsync(mirror);
        }
        else if (isPromiseMirror(mirror)) {
            const underlyingValueMirror = await getPromiseMirrorValueAsync(mirror);
            entry.promise = await getOrCreateEntryAsync(underlyingValueMirror, undefined, context, serialize, logInfo);
        }
        else if (isArrayMirror(mirror)) {
            // Recursively serialize elements of an array. Note: we use getOwnPropertyDescriptors as the array
            // may be sparse and we want to properly respect that when serializing.
            entry.array = [];
            for (const descriptor of await getOwnPropertyDescriptorsAsync(mirror)) {
                if (descriptor.name !== undefined &&
                    !isStringValue(descriptor.name, "length")) {

                    entry.array[<any>descriptor.name.value] = await getOrCreateEntryAsync(
                        await getOwnPropertyAsync(mirror, descriptor), undefined, context, serialize, logInfo);
                }
            }

            // TODO(cyrusn): It feels weird that we're not examining any other descriptors of an
            // array.  For example, if someone put on a property with a symbolic name, we'd lose
            // that here. Unlikely, but something we may need to handle in the future.
        }
        // TODO(cyrusn): Get 'Arguments' working again.
        // else if (Object.prototype.toString.call(obj) === "[object Arguments]") {
        //     // From: https://stackoverflow.com/questions/7656280/how-do-i-check-whether-an-object-is-an-arguments-object-in-javascript
        //     entry.array = [];
        //     for (const elem of obj) {
        //         entry.array.push(await getOrCreateEntryAsync(elem, undefined, context, serialize, logInfo));
        //     }
        // }
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
        const descriptors = await getOwnPropertyDescriptorsAsync(mirror);

        for (const descriptor of descriptors) {
            // we're about to recurse inside this object.  In order to prever infinite
            // loops, put a dummy entry in the environment map.  That way, if we hit
            // this object again while recursing we won't try to generate this property.
            const keyEntry = await getOrCreateEntryAsync(getNameOrSymbol(descriptor), undefined, context, serialize, logInfo);
            if (!environment.has(keyEntry)) {
                environment.set(keyEntry, <any>undefined);

                const propertyInfo = await createPropertyInfoAsync(descriptor);
                const prop = await getOwnPropertyAsync(mirror, descriptor);
                const valEntry = await getOrCreateEntryAsync(
                    prop, undefined, context, serialize, logInfo);

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
            const mirrorProto = await getPrototypeOfMirrorAsync(mirror);
            if (mirrorProto !== await getMirrorAsync(Object.prototype)) {
                entry.object!.proto = await getOrCreateEntryAsync(
                    mirrorProto, undefined, context, serialize, logInfo);
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
            const objPropValue = await callAccessorOn(mirror, propName);

            const propertyInfo = await getPropertyInfoAsync(mirror, propName);
            if (!propertyInfo) {
                if (!isUndefinedMirror(objPropValue)) {
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

    async function getPropertyInfoAsync(on: Mirror, key: string): Promise<PropertyInfo | undefined> {
        for (let current = on; isTruthy(current); current = await getPrototypeOfMirrorAsync(current)) {
            const descriptors = await getOwnPropertyDescriptorsAsync(current);
            const descriptor = descriptors.find(d => d.name !== undefined && d.name.value === key);
            if (descriptor) {
                const propertyInfo = await createPropertyInfoAsync(descriptor);
                return propertyInfo;
            }
        }

        return undefined;
    }

    async function createPropertyInfoAsync(descriptor: MirrorPropertyDescriptor): Promise<PropertyInfo> {
        const propertyInfo = <PropertyInfo>{ hasValue: descriptor.value !== undefined };
        propertyInfo.configurable = descriptor.configurable;
        propertyInfo.enumerable = descriptor.enumerable;
        propertyInfo.writable = descriptor.writable;
        if (descriptor.get) {
            propertyInfo.get = await getOrCreateEntryAsync(
                descriptor.get, undefined, context, serialize, logInfo);
        }
        if (descriptor.set) {
            propertyInfo.set = await getOrCreateEntryAsync(
                descriptor.set, undefined, context, serialize, logInfo);
        }

        return propertyInfo;
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

        if (await hasTruthyMemberAsync(mirror, "deploymentOnlyModule") || isLocalModule) {
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

    async function createOutputEntryAsync(outputMirror: Mirror): Promise<Entry> {
        // We have an Output<T>.  This is effectively just a wrapped value 'V' at deployment-time.
        // We want to effectively generate a value post serialization effectively equivalent to `new
        // SerializedOutput(V)`.  It is tempting to want to just do the following:
        //
        //      const val = await output.promise();
        //      return await getOrCreateEntryAsync(new SerializedOutput(val), undefined, context, serialize, logInfo);
        //
        // That seems like it would work.  We're instantiating a SerializedOutput that will point at
        // the underlying 'val' instance, and we're then serializing that entire object to be
        // rehydrated on the other side.
        //
        // However, there's a subtlety here that we need to avoid.  Specifically, in a world where
        // we are never actually looking at real values, but are instead looking at 'Mirrors' of
        // values, we never want to serialize something that actually points at a Mirror (like the
        // SerializedOutput instance would).  The reason for this is that if we then go to serialize
        // the SerializedOutput, our Inspector APIs will hit the Mirror value and then get a Mirror
        // for *that* Mirror. I.e. a Mirror<Mirror>.  This is not what we want and will cause us to
        // generate code that actually produces a Mirror object at cloud-runtime time instead of
        // producing the real value.
        //
        // To avoid this, do something tricky.  We first create an 'empty' SerializedObject.  i.e.
        //
        //      new SerializedOutput(undefined)
        //
        // We then serialize that instance (which we know must be an 'Object-Entry').  We then
        // serialize out 'V', getting back the Entry for it.  We then manually jam in that Entry
        // into the Object-Entry for the SerializedOutput instance.

        // First, extract out the inner value this Output wraps.
        const promiseMirror = await callFunctionOn(mirror, "promise", []);
        if (!isPromiseMirror(promiseMirror)) {
            throw new Error("output.promise() did not return a promise: " + JSON.stringify(promiseMirror));
        }

        // Then, get the underlying value of the out, and create the environment entry for it.
        const valMirror = await getPromiseMirrorValueAsync(promiseMirror);
        const valEntry = await getOrCreateEntryAsync(valMirror, undefined, context, serialize, logInfo);

        // Now, create an empty-serialized output and create an environment entry for it.  It
        // will have a property 'value' that points to an Entry for 'undefined'.
        const emptyOutputMirror = await getMirrorAsync(new SerializedOutput(undefined));
        const emptyOutputEntry = await getOrCreateEntryAsync(emptyOutputMirror, undefined, context, serialize, logInfo);

        // validate that we created the right sort of entry.  It should be an Object-Entry with
        // a single property called 'value' in it.
        if (!emptyOutputEntry.object) {
            throw new Error("Did not get an 'object' in the entry for a serialized output");
        }

        const envEntries = [...emptyOutputEntry.object.env.entries()];
        if (envEntries.length !== 1) {
            throw new Error("Expected SerializedOutput object to only have one property: " + envEntries.length);
        }

        const [envEntry] = envEntries[0];
        if (envEntry.json !== "value") {
            throw new Error("Expected SerializedOutput object sole property to be called 'value': " + envEntry.json);
        }

        // Everything looked good.  Replace the `"value" -> undefined-Entry` mapping in this entry
        // with `"value" -> V-Entry`
        emptyOutputEntry.object.env.set(envEntry, { entry: valEntry });
        return emptyOutputEntry;
    }
}

async function isOutputAsync(mirror: Mirror): Promise<boolean> {
    // Equivalent to calling: resource.Output.isInstance(mirror_value)
    const outputClassMirror = await getMirrorAsync(resource.Output);
    const isInstanceMirror = await callFunctionOn(outputClassMirror, "isInstance", [mirror]);
    if (!isBooleanMirror(isInstanceMirror)) {
        throw new Error("Calling isInstance did not return a boolean: " + JSON.stringify(isInstanceMirror));
    }

    return isInstanceMirror.value;
}

// Is this a constructor derived from a noCapture constructor.  if so, we don't want to
// emit it.  We would be unable to actually hook up the "super()" call as one of the base
// constructors was set to not be captured.
async function isDerivedNoCaptureConstructorAsync(func: FunctionMirror) {
    for (let current: Mirror = func;
         isTruthy(current);
         current = await getPrototypeOfMirrorAsync(current)) {

        if (await hasTruthyMemberAsync(current, "doNotCapture")) {
            return true;
        }
    }

    return false;
}

let builtInModules: Promise<Map<Mirror, string>> | undefined;
function getBuiltInModules(): Promise<Map<Mirror, string>> {
    if (!builtInModules) {
        builtInModules = computeBuiltInModules();
    }

    return builtInModules;

    async function computeBuiltInModules() {
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

        const map = new Map<Mirror, string>();
        for (const name of builtInModuleNames) {
            map.set(await getMirrorAsync(require(name)), name);
        }

        return map;
    }
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
    const modules = await getBuiltInModules();
    const key = modules.get(obj);
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

async function hasTruthyMemberAsync(mirror: Mirror, memberName: string): Promise<boolean> {
    if (isUndefinedOrNullMirror(mirror)) {
        return false;
    }

    return isTruthy(await callAccessorOn(mirror, memberName));
}
