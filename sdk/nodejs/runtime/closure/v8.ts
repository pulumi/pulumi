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

// This file provides a low-level interface to a few V8 runtime objects.
// We will use this low-level interface when serializing closures to walk the scope
// chain and find the value of free variables captured by closures, as well as getting
// source-level debug information so that we can present high-quality error messages.
//
// As a side-effect of importing this file, we must enable the --allow-natives-syntax V8
// flag. This is because we are using V8 intrinsics in order to implement this module.
import * as inspector from "inspector";

const inspectorSession = new inspector.Session();
inspectorSession.connect();

// First, register to hear about scripts getting parsed.  This is the only supported way
// to map from a scriptId to a file-name with the inspector API.
const scriptIdToUrlMap = new Map<string, string>();
inspectorSession.addListener("Debugger.scriptParsed", m => {
    const { scriptId, url } = m.params;
    console.log("Mapping: " + scriptId + " to " + url);
    scriptIdToUrlMap.set(scriptId, url);
});

import * as semver from "semver";
import * as v8 from "v8";

console.log("Loading v8 module");
v8.setFlagsFromString("--allow-natives-syntax");

// We depend on V8 intrinsics to inspect JavaScript functions and runtime values. These intrinsics
// are unstable and change radically between versions. These two version checks are used by this module
// to determine whether or not it is safe to use particular intrinsincs.
//
// We were broken especially badly by Node 11, which removed most of the intrinsics that we used.
// We will need to find replacements for them.
const isNodeAtLeastV10 = semver.gte(process.version, "10.0.0");
const isNodeAtLeastV11 = semver.gte(process.version, "11.0.0");

function throwUnsupportedNodeVersion(func: string): never {
    throw new Error(
        `Function serialization with Node version ${process.version} is not supported by Pulumi at this time. ` +
        `Please use Node 10 or older. ${func}`);
}
/** Unique object identifier. */
type RemoteObjectId = string;

/**
 * Primitive value which cannot be JSON-stringified. Includes values `-0`, `NaN`, `Infinity`,
 * `-Infinity`, and bigint literals.
 */
type UnserializableValue = string;

interface Mirror {
    /** Object type. */
    type: "function" | "object" | "number";
    /** Object subtype hint. Specified for `object` type values only. */
    subtype?: string;
    /** Object class (constructor) name. Specified for `object` type values only. */
    className?: string;
    /** Remote object value in case of primitive values or JSON values (if it was requested). */
    value?: any;
    /** Primitive value which can not be JSON-stringified does not have `value`, but gets this property. */
    unserializableValue?: UnserializableValue;
    /** Unique object identifier (for non-primitive values). */
    objectId?: RemoteObjectId;
}

interface FunctionMirror extends Mirror {
    type: "function";
    subtype: never;
    className: "Function";
    value: never;
    unserializableValue: never;
    objectId: RemoteObjectId;
}

let currentFunctionId = 0;

async function getFunctionMirrorAsync(func: Function): Promise<FunctionMirror> {
    const currentFunctionName = "__functionToSerialize" + currentFunctionId++;
    (<any>global)[currentFunctionName] = func;

    try {
        const mirror = await runtimeEvaluateAsync(`global.${currentFunctionName}`);
        if (mirror.type !== "function") {
            throw new Error("Mirror was not 'function': " + mirror.type);
        }

        return <FunctionMirror>mirror;
    }
    finally {
        delete (<any>global)[currentFunctionName];
    }
}

async function runtimeGetPropertiesAsync(
        objectId: inspector.Runtime.RemoteObjectId,
        ownProperties: boolean | undefined) {

    const retType = await new Promise<inspector.Runtime.GetPropertiesReturnType>((resolve, reject) => {
        inspectorSession.post(
            "Runtime.getProperties",
            { objectId, ownProperties },
            (err, ret) => err ? reject(err) : resolve(ret));
    });

    if (retType.exceptionDetails) {
        throw new Error(`Error calling "Runtime.getProperties(${objectId}, ${ownProperties})": `
            + retType.exceptionDetails.text);
    }

    return { internalProperties: retType.internalProperties || [], properties: retType.result };
}

async function runtimeEvaluateAsync(expression: string): Promise<Mirror> {
    const retType = await new Promise<inspector.Runtime.EvaluateReturnType>((resolve, reject) => {
        inspectorSession.post(
            "Runtime.evaluate",
            { expression },
            (err, ret) => err ? reject(err) : resolve(ret));
    });

    if (retType.exceptionDetails) {
        throw new Error(`Error calling "Runtime.evaluate(${expression})": ` + retType.exceptionDetails.text);
    }

    console.log(JSON.stringify(retType.result));

    const remoteObj = retType.result;
    checkRemoteObject(remoteObj);
    return <Mirror>remoteObj;
}

function checkRemoteObject(obj: inspector.Runtime.RemoteObject) {
    switch (obj.type) {
        case "function":
            checkRemoteFunction(obj);
            break;
        case "object":
        case "number":
            break;
        default:
            throw new Error("Unexpected type: " + obj.type);
    }

    if (obj.className) {
        switch (obj.className) {
            case "Function":
            case "Array":
                break;
            default:
                throw new Error("Unexpected className: " + obj.className);
        }
    }
}

function checkRemoteFunction(obj: inspector.Runtime.RemoteObject) {
    if (obj.className !== "Function") {
        throw new Error("Function's className was: " + obj.className);
    }

    if (obj.subtype) {
        throw new Error("Function had subtype: " + obj.subtype);
    }

    if (obj.value) {
        throw new Error("Function had value: " + obj.subtype);
    }

    if (obj.unserializableValue) {
        throw new Error("Function had unserializableValue: " + obj.unserializableValue);
    }

    if (!obj.objectId) {
        throw new Error("Function did not have objectId");
    }
}

// We use four V8 intrinsics in this file. The first, `FunctionGetScript`, gets
// a `Script` object given a JavaScript function. The `Script` object contains metadata
// about the function's source definition.
function getScriptPreV11(func: Function): V8Script | undefined {
    if (isNodeAtLeastV11) {
        throwUnsupportedNodeVersion("getScript_pre_v11");
    }

    // The use of the Function constructor here and elsewhere in this file is because
    // because V8 intrinsics are not valid JavaScript identifiers; they all begin with '%',
    // which means that the TypeScript compiler issues errors for them.
    const scriptFunc = new Function("func", "return %FunctionGetScript(func);") as any;
    return scriptFunc(func);
}

// The V8 script object contains the name of the file that defined a function and a function
// that convert a `V8SourcePosition` into a `V8SourceLocation`. (Conceptually - Positions are offsets
// into a resource stream, while locations are objects with line and column information.)
interface V8Script {
    readonly name: string;
    locationFromPosition(pos: V8SourcePosition): V8SourceLocation;
}


// The second intrinsic is `FunctionGetScriptSourcePosition`, which does about what you'd
// expect. It returns a `V8SourcePosition`, which can be passed to `V8Script::locationFromPosition`
// to produce a `V8SourceLocation`.
const getSourcePosition: (func: Function) => V8SourcePosition =
    new Function("func", "return %FunctionGetScriptSourcePosition(func);") as any;

function scriptPositionInfo(script: V8Script, pos: V8SourcePosition): {line: number, column: number} {
    if (isNodeAtLeastV11) {
        throwUnsupportedNodeVersion("scriptPositionInfo");
    }

    if (isNodeAtLeastV10) {
        const scriptPositionInfoFunc =
            new Function("script", "pos", "return %ScriptPositionInfo(script, pos, false);") as any;

        return scriptPositionInfoFunc(script, pos);
    }

    // Should not be called if running on Node<10.0.0.
    return <any>undefined;
}

// V8SourcePosition is an opaque value that should be passed verbatim to `V8Script.locationFromPosition`
// in order to receive a V8SourceLocation.
interface V8SourcePosition { }

// V8SourceLocation contains metadata about a single location within a Script. For a function, it
// refers to the last character of that function's declaration.
interface V8SourceLocation {
    readonly line: number;
    readonly column: number;
}

// The last two intrinsics are `GetFunctionScopeCount` and `GetFunctionScopeDetails`.
// The former function returns the number of scopes in a given function's scope chain, while
// the latter function returns the i'th entry in a function's scope chain, given a function and
// index i.
function getFunctionScopeDetails(func: Function, index: number): any[] {
    if (isNodeAtLeastV11) {
        throwUnsupportedNodeVersion("getFunctionScopeDetails");
    }

    const getFunctionScopeDetailsFunc =
        new Function("func", "index", "return %GetFunctionScopeDetails(func, index);") as any;

    return getFunctionScopeDetailsFunc(func, index);
}

function getFunctionScopeCount(func: Function): number {
    if (isNodeAtLeastV11) {
        throwUnsupportedNodeVersion("getFunctionScopeCount");
    }

    const getFunctionScopeCountFunc = new Function("func", "return %GetFunctionScopeCount(func);") as any;
    return getFunctionScopeCountFunc(func);
}

// All of these functions contain syntax that is not legal TS/JS (i.e. "%Whatever").  As such,
// we cannot serialize them.  In case they somehow get captured, just block them from closure
// serialization entirely.
(<any>getScriptPreV11).doNotCapture = true;
(<any>getSourcePosition).doNotCapture = true;
(<any>getFunctionScopeDetails).doNotCapture = true;
(<any>getFunctionScopeCount).doNotCapture = true;

// `GetFunctionScopeDetails` returns a raw JavaScript array. This enum enumerates the objects that
// are at specific indices of the array. We only care about one of these.
enum V8ScopeDetailsFields {
    kScopeDetailsTypeIndex = 0,
    kScopeDetailsObjectIndex = 1, // <-- this one
    kScopeDetailsNameIndex = 2,
    kScopeDetailsStartPositionIndex = 3,
    kScopeDetailsEndPositionIndex = 4,
    kScopeDetailsFunctionIndex = 5,
}

// V8ScopeDetails contains a lot of information about a particular scope in the scope chain, but the
// only one we care about is `scopeObject`, which is a mapping of strings to values. The strings are variables
// declared within the given scope, and the values are the value of the captured variables.
interface V8ScopeDetails {
    readonly scopeObject: Record<string, any>;
}

// getScopeForFunction extracts a V8ScopeDetails for the index'th element in the scope chain for the
// given function.
function getScopeForFunction(func: Function, index: number): V8ScopeDetails {
    const scopeDetails = getFunctionScopeDetails(func, index);
    return {
        scopeObject: scopeDetails[V8ScopeDetailsFields.kScopeDetailsObjectIndex] as Record<string, any>,
    };
}

/**
 * Given a function and a free variable name, lookupCapturedVariableValue looks up the value of that free variable
 * in the scope chain of the provided function. If the free variable is not found, `throwOnFailure` indicates
 * whether or not this function should throw or return `undefined.
 *
 * @param func The function whose scope chain is to be analyzed
 * @param freeVariable The name of the free variable to inspect
 * @param throwOnFailure If true, throws if the free variable can't be found.
 * @returns The value of the free variable. If `throwOnFailure` is false, returns `undefined` if not found.
 */
export async function lookupCapturedVariableValueAsync(
        func: Function, freeVariable: string, throwOnFailure: boolean): Promise<any> {

    // The implementation of this function is now very straightforward since the intrinsics do all of the
    // difficult work.
    const count = getFunctionScopeCount(func);
    for (let i = 0; i < count; i++) {
        const scope = getScopeForFunction(func, i);
        if (freeVariable in scope.scopeObject) {
            return scope.scopeObject[freeVariable];
        }
    }

    if (throwOnFailure) {
        throw new Error("Unexpected missing variable in closure environment: " + freeVariable);
    }

    return undefined;
}

/**
 * Given a function, returns the file, line and column number in the file where this function was
 * defined. Returns { "", 0, 0 } if the location cannot be found or if the given function has no Script.
 */
export function getFunctionLocationAsync(func: Function) {

    if (isNodeAtLeastV11) {
        return getFunctionLocationPostV11Async(func);
    }
    else {
        return getFunctionLocationPreV11Async(func);
    }
}

async function getFunctionLocationPreV11Async(func: Function) {
    const script = getScriptPreV11(func);
    const { line, column } = getLineColumn(func, script);

    return { file: script ? script.name : "", line, column };
}

export async function getFunctionLocationPostV11Async(func: Function) {
    const functionMirror = await getFunctionMirrorAsync(func);
    const { properties, internalProperties } = await runtimeGetPropertiesAsync(
        functionMirror.objectId, /*ownProperties:*/ false);

    for (const prop of properties) {
        console.log(JSON.stringify(prop));
    }
    for (const prop of internalProperties) {
        console.log(JSON.stringify(prop));
    }

    const functionLocation = internalProperties.find(p => p.name === "[[FunctionLocation]]");
    if (functionLocation && functionLocation.value && functionLocation.value.value) {
        const value = functionLocation.value.value;
        const scriptId = value.scriptId;
        const line = value.lineNumber;
        const column = value.columnNumber;

        const file = scriptIdToUrlMap.get(scriptId) || "";
        return { file, line, column };
    }

    return { file: "", line: 0, column: 0 };
}

function getLineColumn(func: Function, script: V8Script | undefined) {
    if (script) {
        const pos = getSourcePosition(func);

        try {
            if (isNodeAtLeastV10) {
                return scriptPositionInfo(script, pos);
            } else {
                return script.locationFromPosition(pos);
            }
        } catch (err) {
            // Be resilient to native functions not being available. In this case, we just return
            // '0,0'.  That's not great, but it at least lets us run, and it isn't a terrible
            // experience.
            //
            // Specifically, we only need these locations when we're printing out an error about not
            // being able to serialize something.  In that case, we still print out the names of the
            // functions (as well as the call-tree that got us there), *and* we print out the body
            // of the function.  With both of these, it is generally not too difficult to find out
            // where the code actually lives.
        }
    }

    return { line: 0, column: 0 };
}

