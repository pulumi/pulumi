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

import { all, Input, Output } from "../output";

/**
 * toObject takes an array of T values, and a selector that produces key/value pairs from those inputs,
 * and converts this array into an output object with those keys and values.
 *
 * For instance, given an array as follows
 *
 *     [{ s: "a", n: 1 }, { s: "b", n: 2 }, { s: "c", n: 3 }]
 *
 * and whose selector is roughly `(e) => [e.s, e.n]`, the resulting object will be
 *
 *     { "a": 1, "b": 2, "c": 3 }
 *
 */
export function toObject<T, V>(
        iter: Input<T[]>, selector: (t: T) => Input<[string, V]>): Output<{[key: string]: V}> {
    return Output.create<T[]>(iter).apply(elems => {
        const array: Input<[string, V]>[] = [];
        for (const e of elems) {
            array.push(selector(e));
        }
        return all<[string, V]>(array).apply(kvps => {
            const obj: {[key: string]: V} = {};
            for (const kvp of kvps) {
                obj[kvp[0]] = kvp[1];
            }
            return obj;
        });
    });
}

/**
 * groupBy takes an array of T values, and a selector that prduces key/value pairs from those inputs,
 * and converts this array into an output object, with those keys, and where each property is an array of values,
 * in the case that the same key shows up multiple times in the input.
 *
 * For instance, given an array as follows
 *
 *     [{ s: "a", n: 1 }, { s: "a", n: 2 }, { s: "b", n: 1 }]
 *
 * and whose selector is roughly `(e) => [e.s, e.n]`, the resulting object will be
 *
 *     { "a": [1, 2], "b": [1] }
 *
 */
export function groupBy<T, V>(
        iter: Input<T[]>, selector: (t: T) => Input<[string, V]>): Output<{[key: string]: V[]}> {
    return Output.create<T[]>(iter).apply(elems => {
        const array: Input<[string, V]>[] = [];
        for (const e of elems) {
            array.push(selector(<any>e));
        }
        return all<[string, V]>(array).apply(kvps => {
            const obj: {[key: string]: V[]} = {};
            for (const kvp of kvps) {
                let r = obj[kvp[0]];
                if (!r) {
                    r = [];
                    obj[kvp[0]] = r;
                }
                r.push(kvp[1]);
            }
            return obj;
        });
    });
}
