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

// tslint:disable

import * as assert from "assert";
import { output, Output, Resource } from "../index";
import { asyncTest } from "./util";
import { promiseResult, liftProperties } from "../utils";

// function test(val: any, expected: any) {
//     return asyncTest(async () => {
//         const unwrapped = output(val);
//         const actual = await unwrapped.promise();
//         assert.deepStrictEqual(actual, expected);
//     });
// }

// function testUntouched(val: any) {
//     return test(val, val);
// }

// function testPromise(val: any) {
//     return test(Promise.resolve(val), val);
// }

// function testOutput(val: any) {
//     return test(output(val), val);
// }

// function testResources(val: any, expected: any, resources: TestResource[]) {
//     return asyncTest(async () => {
//         const unwrapped = output(val);
//         const actual = await unwrapped.promise();

//         assert.deepStrictEqual(actual, expected);
//         assert.deepStrictEqual(unwrapped.resources(), new Set(resources));

//         const unwrappedResources: TestResource[] = <any>[...unwrapped.resources()];
//         unwrappedResources.sort((r1, r2) => r1.name.localeCompare(r2.name));

//         resources.sort((r1, r2) => r1.name.localeCompare(r2.name));
//         assert.equal(
//             JSON.stringify(unwrappedResources),
//             JSON.stringify(resources));
//     });
// }

// class TestResource {
//     // fake being a pulumi resource.  We can't actually derive from Resource as that then needs an
//     // engine and whatnot.  All things we don't want during simple unit tests.
//     private readonly __pulumiResource: boolean = true;

//     constructor(public name: string) {
//     }
// }

// // Helper type to try to do type asserts.  Note that it's not totally safe.  If TS thinks a type is
// // the 'any' type, it will succeed here.  Talking to the TS team, it does not look like there's a
// // way to write a totally airtight type assertion.

// type EqualsType<X, Y> = X extends Y ? Y extends X ? X : never : never;

describe("deasync", () => {
    it("handles simple promise", () => {
        let res: (value: number) => void;
        const promise = new Promise<number>((resolve) => {
            res = resolve;
        })

        const actual = 4;
        res!(actual);

        const result = promiseResult(promise);
        assert.equal(result, actual);
    });

    it("handles rejected promise", () => {
        let rej: (reason: any) => void;
        const promise = new Promise<number>((resolve, reject) => {
            rej = reject;
        })

        const message = "etc";
        rej!(new Error(message));

        try {
            const result = promiseResult(promise);
            assert.fail("Should not be able to reach here 1.")
        }
        catch (err) {
            assert.equal(err.message, message);
            return;
        }

        assert.fail("Should not be able to reach here 2.")
    });

    it("handles pumping", () => {
        let res: (value: number) => void;
        const promise = new Promise<number>((resolve) => {
            res = resolve;
        })

        const actual = 4;
        setTimeout(res!, 500 /*ms*/, actual);

        const result = promiseResult(promise);
        assert.equal(result, actual);
    });

    it("lift properties", asyncTest(async () => {
        const actual = { a: "foo", b: 4, c: true, d: [function() {}] };

        let res: (value: typeof actual) => void;
        const promise = new Promise<typeof actual>((resolve) => {
            res = resolve;
        })

        res!(actual);

        const combinedResult = liftProperties(promise);

        // check that we've lifted the values properly.
        for (const key of Object.keys(actual)) {
            const value = (<any>actual)[key];
            assert.deepStrictEqual(value, (<any>combinedResult)[key]);
        }

        // also check that we have a proper promise to work with:
        const promiseValue = await combinedResult;
        for (const key of Object.keys(actual)) {
            const value = (<any>actual)[key];
            assert.deepStrictEqual(value, (<any>promiseValue)[key]);
        }

        // also ensure that .then works
        await combinedResult.then(v => {
            for (const key of Object.keys(actual)) {
                const value = (<any>actual)[key];
                assert.deepStrictEqual(value, (<any>v)[key]);
            }
        });
    }));
});