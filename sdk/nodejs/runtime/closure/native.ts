// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

// This file provides a low-level interface to a few V8 runtime objects.
// We will use this low-level interface when serializing closures to walk the scope
// chain and find the value of free variables captured by closures, as well as getting
// source-level debug information so that we can present high-quality error messages.
//
// As a side-effect of importing this file, we must enable the --allow-natives-syntax V8
// flag. This is because we are using V8 intrinsics in order to implement this module.
import * as v8 from "v8";
v8.setFlagsFromString("--allow-natives-syntax");

// We use four V8 intrinsics in this file. The first, `FunctionGetScript`, gets
// a `Script` object given a JavaScript function. The `Script` object contains metadata
// about the function's source definition.
const getScript: (func: Function) => V8Script | undefined =
    // The use of the Function constructor here and elsewhere in this file is because
    // because V8 intrinsics are not valid JavaScript identifiers; they all begin with '%',
    // which means that the TypeScript compiler issues errors for them.
    new Function("func", "return %FunctionGetScript(func);") as any;

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

interface V8SourcePosition {}
interface V8SourceLocation {
    readonly line: number;
    readonly column: number;
}

// The last two intrinsics are `GetFunctionScopeCount` and `GetFunctionScopeDetails`.
// The former function returns the number of scopes in a given function's scope chain, while
// the latter function returns the i'th entry in a function's scope chain, given a function and
// index i.
const getFunctionScopeDetails: (func: Function, index: number) => any[] =
    new Function("func", "index", "return %GetFunctionScopeDetails(func, index);") as any;
const getFunctionScopeCount: (func: Function) => number =
    new Function("func", "return %GetFunctionScopeCount(func);") as any;

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

// getScopesForFunction invokes the necessary intrinsics to get a list of `V8ScopeDetails` for a given
// function.
function getScopesForFunction(func: Function): V8ScopeDetails[] {
    const scopes: V8ScopeDetails[] = [];
    const count = getFunctionScopeCount(func);
    for (let scope = 0; scope < count; scope++) {
        const scopeDetails = getFunctionScopeDetails(func, scope);
        scopes.push({
            scopeObject: scopeDetails[V8ScopeDetailsFields.kScopeDetailsObjectIndex] as Record<string, any>,
        });
    }

    return scopes;
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
export function lookupCapturedVariableValue(func: Function, freeVariable: string, throwOnFailure: boolean): any {
    // The implementation of this function is now very straightforward since the intrinsics do all of the
    // difficult work.
    const scopes = getScopesForFunction(func);
    for (const scope of scopes) {
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
 * Given a function, returns the name of the file where this function was defined.
 * Returns the empty string if the given function has no Script. (e.g. a native function)
 */
export function getFunctionFile(func: Function): string {
    const script = getScript(func);
    return script ? script.name : "";
}

/**
 * Given a function, returns the line number in the file where this function was defined.
 * Returns 0 if the given function has no Script.
 */
export function getFunctionLine(func: Function): number {
    const script = getScript(func);
    if (!script) {
        return 0;
    }

    const pos = getSourcePosition(func);
    const location = script.locationFromPosition(pos);
    return location.line;
}

/**
 * Given a function, returns the column in the file where this function was defined.
 * Returns 0 if the given function has no script.
 */
export function getFunctionColumn(func: Function): number {
    const script = getScript(func);
    if (!script) {
        return 0;
    }

    const pos = getSourcePosition(func);
    const location = script.locationFromPosition(pos);
    return location.column;
}
