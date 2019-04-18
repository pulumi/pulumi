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
// NOTE: We don't need `Disposable` or `Enumerator#reset()` yet, but it's worth noting here that we
// didn't include them, and this causes us to diverge slightly from the "canonical" `Enumerable`
// model.
//

export interface Enumerable<T> {
    filter(f: (t: T) => boolean): Enumerable<T>;
    flatMap(f: (t: T, index?: number) => T[]): Enumerable<T>;
    map<U>(f: (t: T) => U): Enumerable<U>;
    take(n: number): Enumerable<T>;

    toArray(): Promise<T[]>;
    forEach(f: (t: T) => void): void;
}

export interface Enumerator<T> {
    current(): T;
    moveNext(): boolean;
}

export interface EnumerablePromise<T> extends Enumerable<T>, Promise<Enumerator<T>> {
    filter(f: (t: T) => boolean): EnumerablePromise<T>;
    flatMap(f: (t: T, index?: number) => T[]): EnumerablePromise<T>;
    map<U>(f: (t: T) => U): EnumerablePromise<U>;
    take(n: number): EnumerablePromise<T>;

    toArray(): Promise<T[]>;
    forEach(f: (t: T) => void): void;
}
