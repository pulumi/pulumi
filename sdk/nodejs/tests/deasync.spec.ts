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
import { asyncTest } from "./util";
import { promiseResult } from "../utils";

describe("deasync", () => {
    it("handles simple promise", () => {
        const actual = 4;
        const promise = new Promise<number>((resolve) => {
            resolve(actual);
        });

        const result = promiseResult(promise);
        assert.equal(result, actual);
    });

    it("handles rejected promise", () => {
        const message = "etc";
        const promise = new Promise<number>((resolve, reject) => {
            reject(new Error(message));
        });

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
        const actual = 4;
        const promise = new Promise<number>((resolve) => {
            setTimeout(resolve, 500 /*ms*/, actual);
        });

        const result = promiseResult(promise);
        assert.equal(result, actual);
    });

    // it("lift properties", asyncTest(async () => {
    //     const actual = { a: "foo", b: 4, c: true, d: [function() {}] };

    //     const promise = new Promise<typeof actual>((resolve) => {
    //         resolve(actual);
    //     });

    //     const combinedResult = liftProperties(promise);

    //     // check that we've lifted the values properly.
    //     for (const key of Object.keys(actual)) {
    //         const value = (<any>actual)[key];
    //         assert.deepStrictEqual(value, (<any>combinedResult)[key]);
    //     }

    //     // also check that we have a proper promise to work with:
    //     const promiseValue = await combinedResult;
    //     for (const key of Object.keys(actual)) {
    //         const value = (<any>actual)[key];
    //         assert.deepStrictEqual(value, (<any>promiseValue)[key]);
    //     }

    //     // also ensure that .then works
    //     await combinedResult.then(v => {
    //         for (const key of Object.keys(actual)) {
    //             const value = (<any>actual)[key];
    //             assert.deepStrictEqual(value, (<any>v)[key]);
    //         }
    //     });
    // }));

    // it("lift properties throws", () => {
    //     const message = "etc";
    //     const promise = new Promise<number>((resolve, reject) => {
    //         reject(new Error(message));
    //     });

    //     try {
    //         const result = liftProperties(promise);
    //         assert.fail("Should not be able to reach here 1.")
    //     }
    //     catch (err) {
    //         assert.equal(err.message, message);
    //         return;
    //     }

    //     assert.fail("Should not be able to reach here 2.")
    // });
});