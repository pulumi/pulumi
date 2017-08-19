// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// printf prints the provided message to standard out.
export function printf(message: any): void {
    // functionality provided by the runtime.
}

// toString returns a string representation of the given object.
export function toString(obj: Object): string {
    return ""; // functionality provided by the runtime.
}

// sha1hash generates the SHA-1 hash of the provided string.
export function sha1hash(str: string): string {
    return ""; // functionality provided by the runtime.
}

// isFunction checks whether the given object is a function (and hence invocable).
export function isFunction(obj: Object): boolean {
    return false; // functionality provided by the runtime.
}

// defaultIfComputed substitutes a default value if target object is computed.  In the absence of pulumi/pulumi-fabric#170,
// this is required in some cases to avoid conditionalizing code on a computed property.
export function defaultIfComputed(obj: Object, def: Object): Object {
    return <any>undefined; // functionality provided by the runtime.
}

// dynamicInvoke dynamically calls the target function.  If the target is not a function, an error is thrown.
export function dynamicInvoke(obj: Object, thisArg: Object, args: Object[]): Object {
    return <any>undefined; // functionality provided by the runtime.
}

// objectKeys returns the property keys for the target object.
export function objectKeys(obj: any): string[] {
    return <any>undefined; // functionality provided by the runtime.
}

// jsonStringify converts a Lumi value into a JSON string.
export function jsonStringify(val: any): string {
    // functionality provided by the runtime
    return "";
}

// jsonParse converts a JSON string into a Lumi value.
export function jsonParse(json: string): any {
    // functionality provided by the runtime
    return undefined;
}

// serializeClosure serializes a function and its closure environment into a form that is amenable to persistence
// as simple JSON.  Like toString, it includes the full text of the function's source code, suitable for execution.
export function serializeClosure(func: any): Closure | undefined {
    // functionality provided by the runtime
    return undefined;
}

// Closure represents the serialized form of a Lumi function.
export interface Closure {
    code: string;        // a serialization of the function's source code as text.
    language: string;    // the language runtime required to execute the serialized code.
    signature: string;   // the function signature type token.
    environment: EnvObj; // the captured lexical environment of variables to values, if any.
}

export type EnvObj = {[key: string]: EnvEntry};

export interface EnvEntry {
    json?: any;        // a value which can be safely json serialized
    closure?: Closure; // a closure we are dependent on
    obj?: EnvObj;      // an object which may contain nested closures
    arr?: EnvEntry[];  // an array which may contain nested closures 
}

