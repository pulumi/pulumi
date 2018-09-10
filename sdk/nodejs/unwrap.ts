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
 * wraps. In practice that should be ok.  Values in an object graph should not wrap Outputs in
 * Promises.  Instead, any code that needs to work Outputs and also be async should either create
 * the Output with the Promise (which will collapse into just an Output).  Or, it should start with
 * an Output and call .Apply on it, passing in an async function.  This will also collapse and just
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

// Note1: unwrap must be async itself.  Otherwise we can't 'peer' through promises to see what
// resources they may be referencing.  i.e. Output<T> needs the set of resources it points at to be
// provided at creation time.  So we could not possibly produce an Output that had all the right
// Resources, unless we deeply (and thus asynchronously) unwrapped the Promises to find all of them.

// Note2: I split this into two functions to keep the flow and return types clearer to me.  The
// helper function is the recursive part and does not talk about Output objects.  It just gives back
// the unwrapped value and any resources it ran into while unwrapping.  Only the topmost function
// deals with them returning that into the final Output to return.

/**
 * [unwrap] takes in a object and creates a deep clone of it with all inner Outputs and Promises
 * unwrapped.  i.e. the object returned will point at the values inside those Outputs and Promises.
 * This process works transitively over the entire value graph and will see inside of Arrays and
 * Objects.
 *
 * The resultant awaited value of this function will be an Output containing the final completely
 * unwrapped object, as well as all [Resource]s that were encountered along the way while unwrapping.
 * With this, the result can then be transformed using [Output.apply] as usual, and the result of
 * that can be passed anywhere that needs such a value and also wants to keep track of dependent
 * [Resource]s.  The expected way to use this function is like:
 *
 * ```ts
 *      var hoisted = await pulumi.unwrap(someVal);
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
export async function unwrap<T>(val: T): Promise<Output<Unwrap<T>>> {
    // Unwrap the value that was passed in.  And also capture all the resources and the isKnown bit.
    // Use all that to create the final output we return.
    const [unwrapped, resources, isKnown] = await unwrapWorker(val);
    return new Output(new Set(resources), Promise.resolve(unwrapped), Promise.resolve(isKnown));
}

async function unwrapWorker<T>(val: T): Promise<[Unwrap<T>, Resource[], boolean]> {
    if (val === null) {
        return [<any>val, [], /*isKnown*/ true];
    }
    else if (typeof val !== "object") {
        // strings, numbers, booleans, functions, symbols, undefineds all are returned as themselves
        return [val, [], /*isKnown*/ true];
    }
    else if (val instanceof Promise) {
        // For a promise, we first await it to get the inner value, then we unwrap that inner value.
        return unwrapWorker(await val);
    }
    else if (Output.isInstance(val)) {
        // Outputs wrap a promise themselves.  So first unwrap that promise.  Then combine any
        // resources we found inside of it with the resources we know are associated with this
        // Output itself.
        const [unwrapped, resources, innerIsKnown] = await unwrapWorker(val.promise());

        const allResources = [...val.resources()];
        allResources.push(...resources);

        const isKnown = (await val.isKnown) && innerIsKnown;

        return [<any>unwrapped, allResources, isKnown];
    }
    else if (val instanceof Array) {
        const unwrappedArray: any[] = [];
        const allResources: Resource[] = [];
        let isKnown = true;

        for (const child of val) {
            // Unwrap each child element and merge all its resources into a combined list of
            // resources.  As long as all elements are known for this array, then this result will
            // be known as well.
            const [unwrapped, resources, innerIsKnown] = await unwrapWorker(child);
            unwrappedArray.push(unwrapped);
            allResources.push(...resources);

            isKnown = isKnown && innerIsKnown;
        }

        return [<any>unwrappedArray, allResources, isKnown];
    }
    else {
        const unwrappedObject: any = {};
        const allResources: Resource[] = [];
        let isKnown = true;

        for (const key of Object.keys(val)) {
            // Unwrap each child property and merge all its resources into a combined list of
            // resources.  As long as all child properties are known for this object, then this
            // result will be known as well.
            const [unwrapped, resources, innerIsKnown] = await unwrapWorker((<any>val)[key]);
            unwrappedObject[key] = unwrapped;
            allResources.push(...resources);

            isKnown = isKnown && innerIsKnown;
        }

        return [<any>unwrappedObject, allResources, isKnown];
    }
}
