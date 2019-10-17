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
});