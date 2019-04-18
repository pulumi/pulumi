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

//
// NOTE: We choose to be purposefully conservative about what details are exposed through these
// interfaces in case we decide to change the implementation drastically later.
//

import { EnumerablePromiseImpl } from "./enumerablePromise";
import { EnumerablePromise } from "./interfaces";

export { Enumerable, EnumerablePromise } from "./interfaces";
export { Queryable, QueryableCustomResource } from "./queryable";

export function from<T>(source: T[] | Promise<T[]>): EnumerablePromise<T> {
    return EnumerablePromiseImpl.from(source);
}

export function range(start: number, stop?: number): EnumerablePromise<number> {
    return EnumerablePromiseImpl.range(start, stop);
}
