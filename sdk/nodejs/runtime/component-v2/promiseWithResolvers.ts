// Copyright 2016-2024, Pulumi Corporation.
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

/**
 * @internal
 */
export interface PromiseResolvers<T> {
    promise: Promise<T>;
    reject: (err: any) => void;
    resolve: (value: T) => void;
}

/**
 * The Promise.withResolvers() static method returns an object containing a new Promise object and
 * two functions to resolve or reject it, corresponding to the two parameters passed to the executor
 * of the Promise() constructor.
 *
 * Exactly
 * [Promise#withResolvers](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Promise/withResolvers)
 *
 * @internal
 */

export function promiseWithResolvers<T>(): PromiseResolvers<T> {
    let resolve!: (value: T) => void;
    let reject!: (err: any) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return { promise, reject, resolve };
}
