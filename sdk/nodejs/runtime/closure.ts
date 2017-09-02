// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

const nativeruntime = require("./native/build/Release/nativeruntime.node");

// Closure represents the serialized form of a JavaScript serverless function.
export interface Closure {
    code: string;        // a serialization of the function's source code as text.
    runtime: string;     // the language runtime required to execute the serialized code.
    environment: EnvObj; // the captured lexical environment of variables to values, if any.
}

// EnvObj is the captured lexical environment for a closure.
export type EnvObj = {[key: string]: EnvEntry};

// EnvEntry is the environment slot for a named lexically captured variable.
export interface EnvEntry {
    json?: any;        // a value which can be safely json serialized.
    closure?: Closure; // a closure we are dependent on.
    obj?: EnvObj;      // an object which may contain nested closures.
    arr?: EnvEntry[];  // an array which may contain nested closures.
}

// serializeClosure serializes a function and its closure environment into a form that is amenable to persistence
// as simple JSON.  Like toString, it includes the full text of the function's source code, suitable for execution.
// Unlike toString, it actually includes information about the captured environment.
export function serializeClosure(func: Function): Closure {
    // Ensure this is an expression function.
    let funcstr: string = func.toString();
    if (funcstr.indexOf("[Function:") === 0) {
        throw new Error("Cannot serialize non-expression functions (such as definitions and generators)");
    }

    // Produce the free variables and then pass them into the native runtime.
    return <Closure><any>nativeruntime.serializeClosure(func);
}

