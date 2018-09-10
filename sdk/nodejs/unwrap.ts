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
 *      `...    = { A: { B: { C: boolean } } }`
 *
 * Unwrapping sees through Promises, Outputs, Arrays and Objects.
 *
 * Note: due to TypeScript limitations there are some things that cannot be expressed. Specifically,
 * if you had a `Promise<Output<T>>` then the Unwrap type would not be able to undo both of those
 * wraps. In practice that should be ok.  Values in an object graph should not wrap Outputs in
 * Promises.  Instead, any code that needs to work Outputs and also be async should either create
 * the Output with the Promise (which will collapse into just an Output).  Or, it should start with
 * an Output and call [apply] on it, passing in an async function.  This will also collapse and just
 * produce an Output.
 *
 * In other words, this should not be used as the shape of an object: `{ a: Promise<Output<...>> }`.
 * It should always either be `{ a: Promise<NonOutput> }` or just `{ a: Output<...> }`.
 */
export type Unwrap<T> =
    // 1. If we have a promise, just get the type it itself is wrapping and recursively unwrap that.
    // 2. Otherwise, if we have an output, do the same as a promise and just unwrap the inner type.
    // 3. Otherwise, we have a basic type.  Just unwrap that.
    T extends Promise<infer U> ? UnwrapSimple<U> :
    T extends Output<infer U> ? UnwrapSimple<U> :
    UnwrapSimple<T>;

/**
 * Handles encountering basic types when unwrapping.
 */
type UnwrapSimple<T> =
    // 1. Any of the primitive types just unwrap to themselves.
    // 2. An array of some types unwraps to an array of that type itself unwrapped. Note, due to a
    //    TS limitation we cannot express that as Array<Unwrap<U>> due to how it handles recursive
    //    types. We work around that by introducing an structurally equivalent interface that then
    //    helps make typescript defer type-evaluation instead of doing it eagerly.
    // 3. An object unwraps to an object with properties of the same name, but where the property
    //    types have been unwrapped.
    // 4. return 'never' at the end so that if we've missed something we'll discover it.
    T extends primitive ? T :
    T extends Array<infer U> ? UnwrappedArray<U> :
    T extends object ? UnwrappedObject<T> :
    never;

interface UnwrappedArray<T> extends Array<Unwrap<T>> {}

type UnwrappedObject<T> = {
    [P in keyof T]: Unwrap<T[P]>;
};

/**
 * [unwrap] takes in a object and creates a deep clone of it with all inner Outputs and Promises
 * unwrapped.  i.e. the object returned will point at the values inside those Outputs and Promises.
 * This process works transitively over the entire value graph and will see inside of Arrays and
 * Objects.
 *
 * The resultant awaited value of this function will be an Output containing the final completely
 * unwrapped object, as well as all [Resource]s that were encountered along the way while unwrapping
 * (not including Promise boundaries). With this, the result can then be transformed using
 * [Output.apply] as usual, and the result of that can be passed anywhere that needs such a value
 * and also wants to keep track of dependent [Resource]s.  The expected way to use this function is
 * like:
 *
 * ```ts
 *      var hoisted = pulumi.unwrap(someVal);
 *
 *      var transformed = hoisted.apply(unwrapped => {
 *          // Do whatever you want now.  'unwrapped' will contain no outputs/promises inside
 *          // here, so you can easily do whatever sort of transformation is most convenient.
 *      });
 *
 *      // Result can be passed to another Resource.  The dependency information will be
 *      // propertly maintained.
 *      var someResource = new SomeResource(name, { data: transformed ... });
 * ```
 */
export function unwrap<T>(val: T): Output<Unwrap<T>> {
    if (val === null) {
        return output(val);
    }
    else if (typeof val !== "object") {
        // strings, numbers, booleans, functions, symbols, undefineds all are returned as themselves
        return output(val);
    }
    else if (val instanceof Promise) {
        // For a promise, we need to peek into the value the promise wraps and 'unwrap' that.
        // However, unwrapping that inner value will itself return an Output.  So we get the promise
        // off of *that* inner Output, and we create hte Outer output from that.
        //
        // This has the consequence of losing the resources the inner Promise/Output pointed at.
        // However, that fits in line with our general pulumi model that Outputs should themselves
        // not be wrapped in Promises (as those promises may not be executed, and may not pass their
        // dependency information along.
        return <any>output(val.then(v => unwrap(v).promise()));
    }
    else if (Output.isInstance(val)) {
        return <any>val.apply(unwrap);
    }
    else if (val instanceof Array) {
        return <any>all(val.map(unwrap));
    }
    else {
        const unwrappedObject: any = {};
        Object.keys(val).forEach(k => {
            unwrappedObject[k] = unwrap((<any>val)[k]);
        });

        return <any>all(unwrappedObject);
    }
}
