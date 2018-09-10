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

import { all, Output, output, Resource } from ".";

export type primitive = string | number | boolean | undefined | null;

/**
 * The 'Unwrap' type allows us to express the operation of taking a type, with potentially deeply
 * nested Promises and Outputs and to then get that same type with all the Promises and Outputs
 * replaced with their wrapped type.  Note that this Unwrapping is 'deep'.  So if you had:
 *
 *      `type X = { A: Promise<{ B: Output<{ c: Input<boolean> }> }> }`
 *
 * Then `Unwrap<X>` would be equivalent to:
 *
 *      `type ... = { A: { B: { C: boolean } } }`
 *
 * Unwrapping sees through Promises, Outputs, Arrays and Objects.
 *
 * Note: due to TypeScript limitations there are some things that cannot be expressed. Specifically,
 * if you had a `Promise<Output<T>>` then the Unwrap type would not be able to undo both of those
 * wraps. In practice that should be ok.  Code should not wrap Outputs in Promises. Instead, any
 * code that needs to work Outputs and also be async should either create the Output with the
 * Promise (which will collapse into just an Output).  Or, it should start with an Output and call
 * .Apply on it, passing in an async function.  This will also collapse and just produce an Output.
 */
export type Unwrap<T> =
    // If we have a promise, just get the type it itself is wrapping and recursively unwrap that.
    T extends Promise<infer U> ? UnwrapSimple<U> :
    // If we have an output, do the same as a promise.
    T extends Output<infer U> ? UnwrapSimple<U> :
    // Otherwise, we have a basic type.  Just unwrap that.
    UnwrapSimple<T>;

/**
 * Handles encountering basic types when unwrapping.
 */
type UnwrapSimple<T> =
    // Any of the primitive types just unwrap to themselves.
    T extends primitive ? T :
    // An array of some types unwraps to an array of that type itself unwrapped. Note, due to a TS
    // limitation we cannot express that as Array<Unwrap<U>> due to how it handles recursive types.
    // We work around that by introducing an structurally equivalent interface that then helps
    // make typescript defer type-evaluation instead of doing it eagerly.
    T extends Array<infer U> ? UnwrappedArray<U> :
    // An object unwraps to an object with properties of the same name, but where the property types
    // have been unwrapped.
    T extends object ? UnwrappedObject<T> :
    // return 'never' at the end so that if we've missed something we'll discover it.
    never;

interface UnwrappedArray<T> extends Array<Unwrap<T>> {}

type UnwrappedObject<T> = {
    [P in keyof T]: Unwrap<T[P]>;
};

export async function unwrap<T>(val: T): Promise<Output<Unwrap<T>>> {
    if (val === null) {
        return <any>output(val);
    }

    // strings, numbers, booleans, functions, symbols, undefineds all are returned as themselves
    if (typeof val !== "object") {
        // console.log("Returning simple value: " + val)
        return output(val);
    }

    if (val instanceof Promise) {
        // console.log("Unwrapping promise");
        const unwrappedPromise = await val;
        // console.log("Unwrapped promise: " + JSON.stringify(unwrappedPromise));
        return unwrap(unwrappedPromise);
    }

    if (Output.isInstance(val)) {
        // console.log("Unwrapping output");
        const unwrapped = await unwrap(val.promise());
        // console.log("Unwrapped output: " + JSON.stringify(unwrapped));
        const allResources = new Set<Resource>();

        val.resources().forEach(r => allResources.add(r));
        unwrapped.resources().forEach(r => allResources.add(r));

        return <any>new Output(allResources, unwrapped.promise(), unwrapped.isKnown);
    }

    if (val instanceof Array) {
        // console.log("Unwrapping array.");
        const unwrappedArray: any[] = [];
        for (const child of val) {
            unwrappedArray.push(await unwrap(child));
        }
        // console.log("Unwrapped array: " + JSON.stringify(unwrappedArray));

        const allOutput = all(unwrappedArray);
        // console.log("all.resources: " + JSON.stringify([...allOutput.resources()]));

        return <any>allOutput;
    }

    const unwrappedObject: any = {};
    for (const key of Object.keys(val)) {
        unwrappedObject[key] = await unwrap((<any>val)[key]);
    }

    return <any>all(unwrappedObject);
}
