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
import { PushableAsyncIterable } from "../../runtime/asyncIterableUtil";

async function enumerate<T>(ts: AsyncIterable<T>): Promise<T[]> {
    const tss: T[] = [];
    for await (const n of ts) {
        tss.push(n);
    }
    return tss;
}

describe("PushableAsyncIterable", () => {
    it("correctly produces empty sequence", async () => {
        const queue = new PushableAsyncIterable<number>();
        queue.complete();
        assert.deepStrictEqual(await enumerate(queue), []);
    });

    it("correctly produces singleton sequence", async () => {
        const queue = new PushableAsyncIterable<number>();
        queue.push(1);
        queue.complete();
        assert.deepStrictEqual(await enumerate(queue), [1]);
    });

    it("correctly produces multiple sequence", async () => {
        const queue = new PushableAsyncIterable<number>();
        queue.push(1);
        queue.push(2);
        queue.push(3);
        queue.complete();
        assert.deepStrictEqual(await enumerate(queue), [1, 2, 3]);
    });

    it("correctly terminates outstanding operations afte complete", async () => {
        const queue = new PushableAsyncIterable<number>();
        const queueIter = queue[Symbol.asyncIterator]();
        const terminates = async () => {
            assert.deepStrictEqual(await queueIter.next(), { value: undefined, done: true });
            assert.deepStrictEqual(await queueIter.next(), { value: undefined, done: true });
            assert.deepStrictEqual(await queueIter.next(), { value: undefined, done: true });
        };
        queue.complete();
        await terminates;
        assert.deepStrictEqual(await queueIter.next(), { value: undefined, done: true });
    });

    it("correctly interleaves operations", async () => {
        const queue = new PushableAsyncIterable<number>();
        const queueIter = queue[Symbol.asyncIterator]();
        queue.push(1);
        queue.push(2);
        assert.deepStrictEqual(await queueIter.next(), { value: 1, done: false });
        queue.push(3);
        assert.deepStrictEqual(await queueIter.next(), { value: 2, done: false });
        assert.deepStrictEqual(await queueIter.next(), { value: 3, done: false });
        queue.push(4);
        queue.push(5);
        queue.push(6);
        queue.push(7);
        assert.deepStrictEqual(await queueIter.next(), { value: 4, done: false });
        assert.deepStrictEqual(await queueIter.next(), { value: 5, done: false });
        assert.deepStrictEqual(await queueIter.next(), { value: 6, done: false });
        assert.deepStrictEqual(await queueIter.next(), { value: 7, done: false });
        queue.complete();
        assert.deepStrictEqual(await queueIter.next(), { value: undefined, done: true });
    });
});
