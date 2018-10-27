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

import { Input, Output } from "../resource";

/**
 * toMap takes an array of T values, and a selector that produces key/value pairs from those inputs,
 * and converts this array into an output map with those keys and values.
 *
 * For instance, given an array as follows
 *
 *     [{ s: "a", n: 1 }, { s: "b", n: 2 }, { s: "c", n: 3 }]
 *
 * and whose selector is roughly `(e) => [e.s, e.n]`, the resulting map will contain
 *
 *     [[ "a", 1 ], [ "b", 2 ], [ "c", 3 ]]
 *
 */
export function toMap<T, K, V>(
        iter: Input<Input<T>[]>, selector: (t: T) => [Input<K>, Input<V>]): Output<Map<K, V>> {
    return Output.create(iter).apply(elems => {
        const array: [Input<K>, Input<V>][] = [];
        for (const e of elems) {
            array.push(selector(<any>e));
        }
        return Output.create(array).apply(a => new Map<K, V>(<any>a));
    });
}

/**
 * groupBy takes an array of T values, and a selector that prduces key/value pairs from those inputs,
 * and converts this array into an output map, with those keys, and where each entry is an array of values,
 * in the case that the same key shows up multiple times in the input.
 *
 * For instance, given an array as follows
 *
 *     [{ s: "a", n: 1 }, { s: "a", n: 2 }, { s: "b", n: 1 }]
 *
 * and whose selector is roughly `(e) => [e.s, e.n]`, the resulting map will contain
 *
 *     [[ "a", [1, 2] ], [ "b", [1] ]]
 *
 */
export function groupBy<T, K, V>(
        iter: Input<Input<T>[]>, selector: (t: T) => [Input<K>, Input<V>]): Output<Map<K, V[]>> {
    return Output.create(iter).apply(elems => {
        const array: [Input<K>, Input<V>][] = [];
        for (const e of elems) {
            array.push(selector(<any>e));
        }
        return Output.create(array).apply(kvps => {
            const m = new Map<K, V[]>();
            for (let kvp of kvps) {
                let r = m.get(<any>kvp[0]);
                if (!r) {
                    r = [];
                    m.set(<any>kvp[0], r);
                }
                r.push(<any>kvp[1]);
            }
            return m;
        });
    });
}
