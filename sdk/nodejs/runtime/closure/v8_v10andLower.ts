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

import * as semver from "semver";

const isNodeAtLeastV10 = semver.gte(process.version, "10.0.0");

// The V8 script object contains the name of the file that defined a function and a function
// that convert a `V8SourcePosition` into a `V8SourceLocation`. (Conceptually - Positions are offsets
// into a resource stream, while locations are objects with line and column information.)
interface V8Script {
    readonly name: string;
    locationFromPosition(pos: V8SourcePosition): V8SourceLocation;
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

export async function getFunctionLocationAsync(func: Function) {
    const script = getScript(func);
    const { line, column } = getLineColumn();

    return { file: script ? script.name : "", line, column };

    function getLineColumn() {
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
}

function getScript(func: Function): V8Script | undefined {
    // The use of the Function constructor here and elsewhere in this file is because
    // because V8 intrinsics are not valid JavaScript identifiers; they all begin with '%',
    // which means that the TypeScript compiler issues errors for them.
    const scriptFunc = new Function("func", "return %FunctionGetScript(func);") as any;
    return scriptFunc(func);
}


// The second intrinsic is `FunctionGetScriptSourcePosition`, which does about what you'd
// expect. It returns a `V8SourcePosition`, which can be passed to `V8Script::locationFromPosition`
// to produce a `V8SourceLocation`.
const getSourcePosition: (func: Function) => V8SourcePosition =
    new Function("func", "return %FunctionGetScriptSourcePosition(func);") as any;

function scriptPositionInfo(script: V8Script, pos: V8SourcePosition): {line: number, column: number} {
    if (isNodeAtLeastV10) {
        const scriptPositionInfoFunc =
            new Function("script", "pos", "return %ScriptPositionInfo(script, pos, false);") as any;

        return scriptPositionInfoFunc(script, pos);
    }

    // Should not be called if running on Node<10.0.0.
    return <any>undefined;
}

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

// The last two intrinsics are `GetFunctionScopeCount` and `GetFunctionScopeDetails`.
// The former function returns the number of scopes in a given function's scope chain, while
// the latter function returns the i'th entry in a function's scope chain, given a function and
// index i.
function getFunctionScopeDetails(func: Function, index: number): any[] {
    const getFunctionScopeDetailsFunc =
        new Function("func", "index", "return %GetFunctionScopeDetails(func, index);") as any;

    return getFunctionScopeDetailsFunc(func, index);
}

function getFunctionScopeCount(func: Function): number {
    const getFunctionScopeCountFunc = new Function("func", "return %GetFunctionScopeCount(func);") as any;
    return getFunctionScopeCountFunc(func);
}

// getScopeForFunction extracts a V8ScopeDetails for the index'th element in the scope chain for the
// given function.
function getScopeForFunction(func: Function, index: number): V8ScopeDetails {
    const scopeDetails = getFunctionScopeDetails(func, index);
    return {
        scopeObject: scopeDetails[V8ScopeDetailsFields.kScopeDetailsObjectIndex] as Record<string, any>,
    };
}

// All of these functions contain syntax that is not legal TS/JS (i.e. "%Whatever").  As such,
// we cannot serialize them.  In case they somehow get captured, just block them from closure
// serialization entirely.
(<any>getScript).doNotCapture = true;
(<any>getSourcePosition).doNotCapture = true;
(<any>getFunctionScopeDetails).doNotCapture = true;
(<any>getFunctionScopeCount).doNotCapture = true;
