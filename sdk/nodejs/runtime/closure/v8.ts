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
import * as semver from "semver";
import * as v8 from "v8";
v8.setFlagsFromString("--allow-natives-syntax");

// We depend on V8 intrinsics to inspect JavaScript functions and runtime values. These intrinsics
// are unstable and change radically between versions. These two version checks are used by this module
// to determine whether or not it is safe to use particular intrinsincs.
//
// We were broken especially badly by Node 11, which removed most of the intrinsics that we used.
// We will need to find replacements for them.
const isNodeAtLeastV10 = semver.gte(process.version, "10.0.0");
export const isNodeAtLeastV11 = semver.gte(process.version, "11.0.0");

function throwUnsupportedNodeVersion(): never {
    throw new Error(
        `Function serialization with Node version ${process.version} is not supported by Pulumi at this time. ` +
        "Please use Node 10 or older.");
}

// We use four V8 intrinsics in this file. The first, `FunctionGetScript`, gets
// a `Script` object given a JavaScript function. The `Script` object contains metadata
// about the function's source definition.
function getScript(func: Function): V8Script | undefined {
    if (isNodeAtLeastV11) {
        throwUnsupportedNodeVersion();
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
        throwUnsupportedNodeVersion();
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
        throwUnsupportedNodeVersion();
    }

    const getFunctionScopeDetailsFunc =
        new Function("func", "index", "return %GetFunctionScopeDetails(func, index);") as any;

    return getFunctionScopeDetailsFunc(func, index);
}

function getFunctionScopeCount(func: Function): number {
    if (isNodeAtLeastV11) {
        throwUnsupportedNodeVersion();
    }

    const getFunctionScopeCountFunc = new Function("func", "return %GetFunctionScopeCount(func);") as any;
    return getFunctionScopeCountFunc(func);
}

// All of these functions contain syntax that is not legal TS/JS (i.e. "%Whatever").  As such,
// we cannot serialize them.  In case they somehow get captured, just block them from closure
// serialization entirely.
(<any>getScript).doNotCapture = true;
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
export function lookupCapturedVariable(func: Function, freeVariable: string, throwOnFailure: boolean): any {
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

// import * as inspector from "inspector";
// var session = new inspector.Session();
// session.post("Runtime.callFunctionOn");

/**
 * Given a function, returns the file, line and column number in the file where this function was
 * defined. Returns { "", 0, 0 } if the location cannot be found or if the given function has no Script.
 */
export function getFunctionLocation(func: Function): { file: string, line: number, column: number } {

    const script = getScript(func);
    const { line, column } = getLineColumn(func, script);

    return { file: script ? script.name : "", line, column };
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
