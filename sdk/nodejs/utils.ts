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

import * as deasync from "deasync";

import { InvokeOptions } from "./invoke";

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
