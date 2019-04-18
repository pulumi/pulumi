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

import { EnumerablePromise, Enumerator } from "./interfaces";
import { Filter, FlatMap, Map, Take } from "./operators";
import { ListEnumerator, RangeEnumerator } from "./sources";

export class EnumerablePromiseImpl<T> extends Promise<Enumerator<T>>
    implements EnumerablePromise<T> {
    public static from<T>(source: T[] | Promise<T[]>): EnumerablePromiseImpl<T> {
        if (Array.isArray(source)) {
            return new EnumerablePromiseImpl(resolve => resolve(ListEnumerator.from(source)));
        } else {
            return new EnumerablePromiseImpl(resolve =>
                source.then(ts => resolve(ListEnumerator.from(ts))),
            );
        }
    }

    public static range(start: number, stop?: number): EnumerablePromise<number> {
        return new EnumerablePromiseImpl(resolve => resolve(new RangeEnumerator(start, stop)));
    }

    private constructor(executor: (resolve: (value?: Enumerator<T>) => void) => void) {
        super(executor);
    }

    public filter(f: (t: T) => boolean): EnumerablePromise<T> {
        return new EnumerablePromiseImpl(resolve => this.then(ts => resolve(new Filter(ts, f))));
    }

    public flatMap<U>(f: (t: T) => U[]): EnumerablePromise<U> {
        return new EnumerablePromiseImpl(resolve => this.then(ts => resolve(new FlatMap(ts, f))));
    }

    public map<U>(f: (t: T) => U): EnumerablePromise<U> {
        return new EnumerablePromiseImpl(resolve => this.then(ts => resolve(new Map(ts, f))));
    }

    public take(n: number): EnumerablePromise<T> {
        return new EnumerablePromiseImpl(resolve => this.then(ts => resolve(new Take(ts, n))));
    }

    public toArray(): Promise<T[]> {
        return this.then(ts => {
            const tss: T[] = [];
            while (ts.moveNext()) {
                tss.push(ts.current());
            }
            return tss;
        });
    }

    public forEach(f: (t: T) => void): void {
        this.then(ts => {
            while (ts.moveNext()) {
                f(ts.current());
            }
        });
    }
}
