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

import { AsyncIterable } from "@pulumi/query/interfaces";

type CloseValue = "7473659d-924c-414d-84e5-b1640b2a6296";
const closeValue: CloseValue = "7473659d-924c-414d-84e5-b1640b2a6296";

// PushableAsyncIterable is an `AsyncIterable` that data can be pushed to. It is useful for turning
// push-based callback APIs into pull-based `AsyncIterable` APIs. For example, a user can write:
//
//     const queue = new PushableAsyncIterable();
//     call.on("data", (thing: any) => queue.push(live));
//
// And then later consume `queue` as any other `AsyncIterable`:
//
//     for await (const l of list) {
//         console.log(l.metadata.name);
//     }
//
// Note that this class implements `AsyncIterable<T | undefined>`. This is for a fundamental reason:
// the user can call `complete` at any time. `AsyncIteratable` would normally know when an element
// is the last, but in this case it can't. Or, another way to look at it is, the last element is
// guaranteed to be `undefined`.
export class PushableAsyncIterable<T> implements AsyncIterable<T | undefined> {
    private bufferedData: T[] = [];
    private nextQueue: ((payload: T | CloseValue) => void)[] = [];
    private completed = false;

    push(payload: T) {
        if (this.nextQueue.length === 0) {
            this.bufferedData.push(payload);
        } else {
            const resolve = this.nextQueue.shift()!;
            resolve(payload);
        }
    }

    complete() {
        this.completed = true;
        if (this.nextQueue.length > 0) {
            const resolve = this.nextQueue.shift()!;
            resolve(closeValue);
        }
    }

    private shift(): Promise<T | CloseValue> {
        return new Promise(resolve => {
            if (this.bufferedData.length === 0) {
                if (this.completed === true) {
                    resolve(closeValue);
                }
                this.nextQueue.push(resolve);
            } else {
                resolve(this.bufferedData.shift());
            }
        });
    }

    [Symbol.asyncIterator]() {
        const t = this;
        return {
            async next(): Promise<{ done: boolean; value: T | undefined; }> {
                const value = await t.shift();
                if (value === closeValue) {
                    return { value: undefined, done: true };
                }
                return { value, done: false };
            },
        };
    }
}
