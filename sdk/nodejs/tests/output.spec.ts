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
import { Output, concat, interpolate, output } from "../output";
import * as runtime from "../runtime";
import { asyncTest } from "./util";

describe("output", () => {
    it("throws on toString in string concat", () => {
        const output = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true));
        try {
            const str = "" + output;
        }
        catch (e) {
            return;
        }

        throw new Error("Should have gotten error above!");
    });

    it("throws on toString in normal interpolation", () => {
        const output = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true));
        try {
            const str = `${output}`;
        }
        catch (e) {
            return;
        }

        throw new Error("Should have gotten error above!");
    });

    it("throws on JSON.stringify", () => {
        const output = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true));
        try {
            const str = JSON.stringify(output);
        }
        catch (e) {
            return;
        }

        throw new Error("Should have gotten error above!");
    });

    it("throws on JSON.stringify even when nested", () => {
        const output = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true));
        try {
            const str = JSON.stringify({ o: output });
        }
        catch (e) {
            return;
        }

        throw new Error("Should have gotten error above!");
    });

    it("propagates true isKnown bit from inner Output", asyncTest(async () => {
        runtime.setIsDryRun(true);

        const output1 = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true));
        const output2 = output1.apply(v => new Output(new Set(), Promise.resolve("inner"), Promise.resolve(true)));

        const isKnown = await output2.isKnown;
        assert.equal(isKnown, true);

        const value = await output2.promise();
        assert.equal(value, "inner");
    }));

    it("propagates false isKnown bit from inner Output", asyncTest(async () => {
        runtime.setIsDryRun(true);

        const output1 = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true));
        const output2 = output1.apply(v => new Output(new Set(), Promise.resolve("inner"), Promise.resolve(false)));

        const isKnown = await output2.isKnown;
        assert.equal(isKnown, false);

        const value = await output2.promise();
        assert.equal(value, "inner");
    }));

    it("can await even when isKnown is a rejected promise.", asyncTest(async () => {
        runtime.setIsDryRun(true);

        const output1 = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true));
        const output2 = output1.apply(v => new Output(new Set(), Promise.resolve("inner"), Promise.reject(new Error())));

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

    describe("concat", () => {
        it ("handles no args", asyncTest(async () => {
            const result = concat();
            assert.equal(await result.promise(), "");
        }));

        it ("handles empty string arg", asyncTest(async () => {
            const result = concat("");
            assert.equal(await result.promise(), "");
        }));

        it ("handles non-empty string arg", asyncTest(async () => {
            const result = concat("a");
            assert.equal(await result.promise(), "a");
        }));

        it ("handles promise string arg", asyncTest(async () => {
            const result = concat(Promise.resolve("a"));
            assert.equal(await result.promise(), "a");
        }));

        it ("handles output string arg", asyncTest(async () => {
            const result = concat(output("a"));
            assert.equal(await result.promise(), "a");
        }));

        it ("handles multiple args", asyncTest(async () => {
            const result = concat("http://", output("a"), ":", 80);
            assert.equal(await result.promise(), "http://a:80");
        }));
    });

    describe("interpolate", () => {
        it ("handles empty interpolation", asyncTest(async () => {
            const result = interpolate ``;
            assert.equal(await result.promise(), "");
        }));

        it ("handles no placeholders arg", asyncTest(async () => {
            const result = interpolate `a`;
            assert.equal(await result.promise(), "a");
        }));

        it ("handles string placeholders arg", asyncTest(async () => {
            const result = interpolate `${"a"}`;
            assert.equal(await result.promise(), "a");
        }));

        it ("handles promise placeholders arg", asyncTest(async () => {
            const result = interpolate `${Promise.resolve("a")}`;
            assert.equal(await result.promise(), "a");
        }));

        it ("handles output placeholders arg", asyncTest(async () => {
            const result = interpolate `${output("a")}`;
            assert.equal(await result.promise(), "a");
        }));

        it ("handles multiple args", asyncTest(async () => {
            const result = interpolate `http://${output("a")}:${80}/`;
            assert.equal(await result.promise(), "http://a:80/");
        }));
    });
});
