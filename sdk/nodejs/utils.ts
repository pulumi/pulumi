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

import * as pulumi from ".";

import * as deasync from "deasync";

import { InvokeOptions } from "./invoke";
import { ComponentResourceOptions } from "./resource";

/**
 * Common code for doing RTTI typechecks.  RTTI is done by having a boolean property on an object
 * with a special name (like "__resource" or "__asset").  This function checks that the object
 * exists, has a **boolean** property with that name, and that that boolean property has the value
 * of 'true'.  Checking that property is 'boolean' helps ensure that this test works even on proxies
 * that synthesize properties dynamically (like Output).  Checking that the property has the 'true'
 * value isn't strictly necessary, but works to make sure that the impls are following a common
 * pattern.
 */
/** @internal */
export function isInstance<T>(obj: any, name: keyof T): obj is T {
    return hasTrueBooleanMember(obj, name);
}

/** @internal */
export function hasTrueBooleanMember(obj: any, memberName: string | number | symbol): boolean {
    if (obj === undefined || obj === null) {
        return false;
    }

    const val = obj[memberName];
    if (typeof val !== "boolean") {
        return false;
    }

    return val === true;
}

/**
 * Synchronously blocks until the result of this promise is computed.  If the promise is rejected,
 * this will throw the error the promise was rejected with.  If this promise does not complete this
 * will block indefinitely.
 *
 * Be very careful with this function.  Only wait on a promise if you are certain it is safe to do
 * so.
 *
 * @internal
 */
export function promiseResult<T>(promise: Promise<T>): T {
    enum State {
        running,
        finishedSuccessfully,
        finishedWithError,
    }

    let result: T;
    let error = undefined;
    let state = <State>State.running;

    promise.then(
        val => {
            result = val;
            state = State.finishedSuccessfully;
        },
        err => {
            error = err;
            state = State.finishedWithError;
        });

    deasync.loopWhile(() => state === State.running);
    if (state === State.finishedWithError) {
        throw error;
    }

    return result!;
}

/**
 * Lifts the properties of the value a promise resolves to and adds them to the promise itself. This
 * can be used to take a function that was previously async (i.e. Promise-returning) and make it
 * synchronous in a backward compatible fashion.  Specifically, because the function now returns a
 * `Promise<T> & T` all properties on it will be available for sync consumers, while async consumers
 * can still use `await` on it or call `.then(...)` on it.
 *
 * This is an advanced compat function for libraries and should not generally be used by normal
 * Pulumi application.
 */
export function liftProperties<T>(promise: Promise<T>, opts: InvokeOptions = {}): Promise<T> & T {
    if (opts.async) {
        // Caller just wants the async side of the result.  That's what we have, so just return it
        // as is.
        //
        // Note: this cast isn't actually safe (since 'promise' doesn't actually provide the T side
        // of things).  That's ok.  By default the return signature will be correct, and users will
        // only get into this code path when specifically trying to force asynchrony.  Given that,
        // it's fine to expect them to have to know what they're doing and that they shoud only use
        // the Promise side of things.
        return <Promise<T> & T>promise;
    }

    // Caller wants the async side and the sync side merged.  Block on getting the underlying
    // promise value, then take all the properties from it and copy over onto the promise itself and
    // return the combined set of each.
    const value = promiseResult(promise);
    return Object.assign(promise, value);
}

/**
 * [mergeOptions] takes two ResourceOptions values and produces a new ResourceOptions with the
 * respective properties of `opts2` merged over the same properties in `opts1`.  The original
 * options objects will be unchanged.
 *
 * Conceptually property merging follows these basic rules:
 *  1. if the property is a collection, the final value will be a collection containing the values
 *     from each options object.
 *  2. Simple scaler values from `opts2` (i.e. strings, numbers, bools) will replace the values of
 *     `opts1`.
 *  3. `opts2` can have properties explicitly provided with `null` or `undefined` as the value. If
 *     explicitly provided, then that will be the final value in the result.
 *  4. For the purposes of merging `dependsOn`, `provider` and `providers` are always treated as
 *     collections, even if only a single value was provided.
 */
export function mergeOptions(opts1: pulumi.CustomResourceOptions | undefined, opts2: pulumi.CustomResourceOptions | undefined): pulumi.CustomResourceOptions;
export function mergeOptions(opts1: pulumi.ComponentResourceOptions | undefined, opts2: pulumi.ComponentResourceOptions | undefined): pulumi.ComponentResourceOptions;
export function mergeOptions(opts1: pulumi.ResourceOptions | undefined, opts2: pulumi.ResourceOptions | undefined): pulumi.ResourceOptions;
export function mergeOptions(
        opts1: pulumi.ResourceOptions | undefined,
        opts2: pulumi.ResourceOptions | undefined): pulumi.ResourceOptions {
    const dest = <any>{ ...opts1 };
    const source = <any>{ ...opts2 };

    // Ensure provider/providers are all expanded into the `{ provName: prov }` form.
    // This makes merging simple.
    expandProviders(dest);
    expandProviders(source);

    // iterate specifically over the supplied properties in [source].  Note: there may not be an
    // corresponding value in [dest].
    for (const key of Object.keys(source)) {
        const destVal = dest[key];
        const sourceVal = source[key];

        if (key === "providers") {
            // Note: this expansion is safe to do as expandProviders will have made sure
            // that both collections are in Record<string, ProviderResource> form.
            dest[key] = { ...destVal, ...sourceVal };
            continue;
        }

        // Due to the possibility of top level and nested Inputs for 'dependsOn' we have to handle
        // that property specially.
        if (key === "dependsOn") {
            dest[key] = mergeDependsOn(destVal, sourceVal);
            continue;
        }

        // we should have no promises/outputs at top level for any other resource option properties.
        if (isPromiseOrOutput(destVal)) {
            throw new Error(`Unexpected promise/output in opts1.${key}`);
        }

        if (isPromiseOrOutput(sourceVal)) {
            throw new Error(`Unexpected promise/output in opts2.${key}`);
        }

        dest[key] = merge(destVal, sourceVal);
    }

    // Now, if we are left with a .providers that is just a single key/value pair, then
    // collapse that down into .provider form.
    collapseProviders(dest);

    return dest;
}

function isPromiseOrOutput(val: any): boolean {
    return val instanceof Promise || pulumi.Output.isInstance(val);
}

function merge(dest: any, source: any): any {
    // if the second options bag contained `prop: null` or `prop: undefined` then that overrides
    // anything in the destination.

    // if there's no destination value, the source value wins.
    if (source === null || source === undefined) {
        return source;
    }

    if (dest === null || dest === undefined) {
        return source;
    }

    // if the source is not an object, then it's a simple scaler (i.e. int/bool/string).  The source
    // overrides the destination in this case.
    if (typeof source !== "object") {
        return source;
    }

    // If either are an array, make a new array and merge the values into it. also do this for the
    // specific "dependsOn" property as we allow options to contain a single value as a shorthand
    // way to represent an array just containing that value.
    if (Array.isArray(source) || Array.isArray(dest)) {
        return mergeArraysAndScalers(dest, source);
    }

    // In any other case, just override the destination with the source value.
    return source;
}

/**
 * @internal For testing purposes only.
 */
export function mergeDependsOn(dest: any, source: any): any {
    if (isPromiseOrOutput(dest)) {
        return pulumi.output(dest).apply(d => mergeDependsOn(d, source));
    }

    if (isPromiseOrOutput(source)) {
        return pulumi.output(source).apply(s => mergeDependsOn(dest, s));
    }

    return mergeArraysAndScalers(dest, source);
}

function expandProviders(options: pulumi.ComponentResourceOptions) {
    // Move 'provider' up to 'providers' if we have it.
    if (options.provider) {
        options.providers = [options.provider];
    }

    // Convert 'providers' array to map form if we have an array.
    if (Array.isArray(options.providers)) {
        const result: Record<string, pulumi.ProviderResource> = {};
        for (const provider of options.providers) {
            result[provider.getPackage()] = provider;
        }

        options.providers = result;
    }

    delete options.provider;
}

function collapseProviders(opts: ComponentResourceOptions) {
    // If we have only 0-1 providers, then merge that back down to the .provider field.
    if (opts.providers) {
        const keys = Object.keys(opts.providers);
        if (keys.length === 0) {
            delete opts.providers;
        }
        else if (keys.length === 1) {
            opts.provider = (<any>opts.providers)[keys[0]];
            delete opts.providers;
        }
    }
}

function mergeArraysAndScalers(dest: any, source: any) {
    const result: any[] = [];
    addToArray(result, dest);
    addToArray(result, source);
    return result;
}

function addToArray(resultArray: any[], value: any) {
    if (Array.isArray(value)) {
        resultArray.push(...value);
    }
    else if (value !== undefined && value !== null) {
        resultArray.push(value);
    }
}
