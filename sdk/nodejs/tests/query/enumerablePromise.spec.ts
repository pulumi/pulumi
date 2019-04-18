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

import * as assert from "assert";

import { EnumerablePromiseImpl } from "../../query/enumerablePromise";

describe("EnumerablePromise", () => {
    describe("range", () => {
        it("produces an empty array for overlapping ranges", async () => {
            let xs = await EnumerablePromiseImpl.range(0, 0).toArray();
            assert.deepEqual(xs, []);

            xs = await EnumerablePromiseImpl.range(0, -1).toArray();
            assert.deepEqual(xs, []);
        });

        it("produces an array of one for boundary case", async () => {
            const xs = await EnumerablePromiseImpl.range(0, 1).toArray();
            assert.deepEqual(xs, [0]);
        });

        it("can produce a range including negative numbers", async () => {
            const xs = await EnumerablePromiseImpl.range(-3, 2).toArray();
            assert.deepEqual(xs, [-3, -2, -1, 0, 1]);
        });

        it("is lazily evaluated by take when range is infinite", async () => {
            const xs = await EnumerablePromiseImpl.range(0)
                .take(5)
                .toArray();
            assert.deepEqual(xs, [0, 1, 2, 3, 4]);
        });

        it("is lazily transformed and filtered when range is infinite", async () => {
            const xs = await EnumerablePromiseImpl.range(0)
                .map(x => x + 2)
                // If filter is bigger than the take window, we enumerate all numbers and hang
                // forever.
                .filter(x => x <= 10)
                .take(7)
                .map(x => x - 2)
                .filter(x => x > 3)
                .toArray();
            assert.deepEqual(xs, [4, 5, 6]);
        });

        it("is lazily flatMap'd when range is infinite", async () => {
            const xs = await EnumerablePromiseImpl.range(0)
                // If filter is bigger than the take window, we enumerate all numbers and hang
                // forever.
                .flatMap(x => (x <= 10 ? [x, x] : []))
                .take(5)
                .toArray();
            assert.deepEqual(xs, [0, 0, 1, 1, 2]);
        });
    });

    describe("filter", () => {
        it("produces [] when all elements are filtered out", async () => {
            const xs = await EnumerablePromiseImpl.from([1, 2, 3, 4])
                .filter(x => x < 0)
                .toArray();
            assert.deepEqual(xs, []);
        });

        it("produces an non-empty array when some elements are filtered out", async () => {
            const xs = await EnumerablePromiseImpl.from([1, 2, 3, 4])
                .filter(x => x >= 3)
                .toArray();
            assert.deepEqual(xs, [3, 4]);
        });
    });

    describe("flatMap", () => {
        it("produces [] when all elements are filtered out", async () => {
            const xs = await EnumerablePromiseImpl.from([1, 2, 3, 4])
                .flatMap(x => [])
                .toArray();
            assert.deepEqual(xs, []);
        });

        it("can add elements to an enumerable", async () => {
            const xs = await EnumerablePromiseImpl.from([1, 2, 3, 4])
                .flatMap(x => [x, x])
                .toArray();
            assert.deepEqual(xs, [1, 1, 2, 2, 3, 3, 4, 4]);
        });
    });

    describe("map", () => {
        it("x => x does identity transformation", async () => {
            const xs = await EnumerablePromiseImpl.from([1, 2, 3, 4])
                .map(x => x)
                .toArray();
            assert.deepEqual(xs, [1, 2, 3, 4]);
        });

        it("x => x+1 adds one to every element", async () => {
            const xs = await EnumerablePromiseImpl.from([1, 2, 3, 4])
                .map(x => x + 1)
                .toArray();
            assert.deepEqual(xs, [2, 3, 4, 5]);
        });
    });

    describe("toArray", () => {
        it("returns empty array for empty enumerable", async () => {
            const xs = await EnumerablePromiseImpl.from([]).toArray();
            assert.deepEqual(xs, []);
        });
    });
});
