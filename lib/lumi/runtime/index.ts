// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// printf prints the provided message to standard out.
export function printf(message: any): void {
    // functionality provided by the runtime.
}

// sha1hash generates the SHA-1 hash of the provided string.
export function sha1hash(str: string): string {
    // functionality provided by the runtime.
    return "";
}

// isFunction checks whether the given object is a function (and hence invocable).
export function isFunction(obj: Object): boolean {
    return false; // functionality provided by the runtime.
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
    code: string;                            // a serialization of the function's source code as text.
    language: string;                        // the language runtime required to execute the serialized code.
    signature: string;                       // the function signature type token.
    environment: {[key: string]: EnvEntry}; // the captured lexical environment of variables to values, if any.
}

export interface EnvEntry {
    json?: any;         // a value which can be safely json serialized  
    closure?: Closure   // a closure we are dependent on
}

