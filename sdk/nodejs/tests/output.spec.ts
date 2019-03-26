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

interface Widget {
    type: string;  // metric | text
    x?: number;
    y?: number;
    properties: Object;
}

// This ensures that the optionality of 'x' and 'y' are preserved when describing an Output<Widget>.
// Subtle changes in the definitions of Lifted<T> can make TS think these are required, which can
// break downstream consumers.
function mustCompile(): Output<Widget> {
    return output({
        type: "foo",
        properties: {
            whatever: 1,
        }
    })
}

describe("output", () => {
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

    describe("lifted operations", () => {
        it("lifts properties from inner object", asyncTest(async () => {
            const output1 = output({ a: 1, b: true, c: "str", d: [2], e: { f: 3 }, g: undefined, h: null });

            assert.equal(await output1.a.promise(), 1);
            assert.equal(await output1.b.promise(), true);
            assert.equal(await output1.c.promise(), "str");

            // Can lift both outer arrays as well as array accesses
            assert.deepEqual(await output1.d.promise(), [2]);
            assert.equal(await output1.d[0].promise(), 2);

            // Can lift nested objects as well as their properties.
            assert.deepEqual(await output1.e.promise(), { f: 3 });
            assert.equal(await output1.e.f.promise(), 3);

            assert.strictEqual(await output1.g.promise(), undefined);
            assert.strictEqual(await output1.h.promise(), null);

            // Unspecified things can be lifted, but produce 'undefined'.
            assert.notEqual((<any>output1).z, undefined);
            assert.equal(await (<any>output1).z.promise(), undefined);
        }));

        it("prefers Output members over lifted members", asyncTest(async () => {
            const output1 = output({ apply: 1, promise: 2 });
            assert.ok(output1.apply instanceof Function);
            assert.ok(output1.isKnown instanceof Promise);
        }));

        it("does not lift symbols", asyncTest(async () => {
            const output1 = output({ apply: 1, promise: 2 });
            assert.strictEqual((<any>output1)[Symbol.toPrimitive], undefined);
        }));

        it("does not lift __ properties", asyncTest(async () => {
            const output1 = output({ a: 1, b: 2 });
            assert.strictEqual((<any>output1).__pulumiResource, undefined);
        }));
    });
});
