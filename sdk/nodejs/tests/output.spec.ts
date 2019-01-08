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
import * as resource from "../resource";
import * as runtime from "../runtime";
import { asyncTest } from "./util";

describe("output", () => {
    it("propagates true isKnown bit from inner Output", asyncTest(async () => {
        runtime.setIsDryRun(true);

        const output1 = new resource.Output(new Set(), Promise.resolve("outer"), Promise.resolve(true));
        const output2 = output1.apply(v => new resource.Output(new Set(), Promise.resolve("inner"), Promise.resolve(true)));

        const isKnown = await output2.isKnown;
        assert.equal(isKnown, true);

        const value = await output2.promise();
        assert.equal(value, "inner");
    }));

    it("propagates false isKnown bit from inner Output", asyncTest(async () => {
        runtime.setIsDryRun(true);

        const output1 = new resource.Output(new Set(), Promise.resolve("outer"), Promise.resolve(true));
        const output2 = output1.apply(v => new resource.Output(new Set(), Promise.resolve("inner"), Promise.resolve(false)));

        const isKnown = await output2.isKnown;
        assert.equal(isKnown, false);

        const value = await output2.promise();
        assert.equal(value, "inner");
    }));

    it("can await even when isKnown is a rejected promise.", asyncTest(async () => {
        runtime.setIsDryRun(true);

        const output1 = new resource.Output(new Set(), Promise.resolve("outer"), Promise.resolve(true));
        const output2 = output1.apply(v => new resource.Output(new Set(), Promise.resolve("inner"), Promise.reject(new Error())));

        const isKnown = await output2.isKnown;
        assert.equal(isKnown, false);

        try {
            const value = await output2.promise();
        }
        catch (err) {
            return;
        }

        assert.fail("Should not read here");
    }));

    it("toStrings to helpful warnning.", asyncTest(async () => {
        const output = new resource.Output(new Set(), Promise.resolve("outer"), Promise.resolve(true));
        const expected = "<<< Warning: called 'toString' on an Output, use 'apply' instead: https://pulumi.io/help/outputs >>>";
        assert.equal("" + output, expected);
        assert.equal(`${output}`, expected);
    }));
});
