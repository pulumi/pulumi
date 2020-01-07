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
import { Output, all, concat, interpolate, output, unknown } from "../output";
import { Resource } from "../resource";
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

// mockOutput returns a value that looks like an Output, but allows for greater control over its behavior. This can be
// used to simulate outputs from downlevel SDKs.
function mockOutput(isKnown: boolean | Promise<boolean>, value: any | Promise<any>): any {
    isKnown = isKnown instanceof Promise ? isKnown : Promise.resolve(isKnown);
    value = value instanceof Promise ? value : Promise.resolve(value);

    return {
        __pulumiOutput: true,
        isKnown: isKnown,
        isSecret: Promise.resolve(false),
        promise: () => value,
        resources: () => new Set<Resource>(),

        apply(callback: any): any {
            return mockOutput(isKnown, value.then(async (v: any) => {
                if (!isKnown) {
                    return undefined;
                }
                return callback(v);
            }));
        }
    }
}

describe("output", () => {
    it("propagates true isKnown bit from inner Output", asyncTest(async () => {
        runtime._setIsDryRun(true);

        const output1 = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));
        const output2 = output1.apply(v => new Output(new Set(), Promise.resolve("inner"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set())));

        const isKnown = await output2.isKnown;
        assert.equal(isKnown, true);

        const value = await output2.promise();
        assert.equal(value, "inner");
    }));

    it("propagates false isKnown bit from inner Output", asyncTest(async () => {
        runtime._setIsDryRun(true);

        const output1 = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));
        const output2 = output1.apply(v => new Output(new Set(), Promise.resolve("inner"), Promise.resolve(false), Promise.resolve(false), Promise.resolve(new Set())));

        const isKnown = await output2.isKnown;
        assert.equal(isKnown, false);

        const value = await output2.promise();
        assert.equal(value, "inner");
    }));

    it("can not await if isKnown is a rejected promise.", asyncTest(async () => {
        runtime._setIsDryRun(true);

        const output1 = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));
        const output2 = output1.apply(v => new Output(new Set(), Promise.resolve("inner"), Promise.reject(new Error("foo")), Promise.resolve(false), Promise.resolve(new Set())));

        try {
            const isKnown = await output2.isKnown;
            assert.fail("Should not reach here");
        }
        catch (err) {
        }

        try {
            const value = await output2.promise();
            assert.fail("Should not reach here");
        }
        catch (err) {
        }
    }));

    it("propagates true isSecret bit from inner Output", asyncTest(async () => {
        runtime._setIsDryRun(true);

        const output1 = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));
        const output2 = output1.apply(v => new Output(new Set(), Promise.resolve("inner"), Promise.resolve(true), Promise.resolve(true), Promise.resolve(new Set())));

        const isSecret = await output2.isSecret;
        assert.equal(isSecret, true);

        const value = await output2.promise();
        assert.equal(value, "inner");
    }));

     it("retains true isSecret bit from outer Output", asyncTest(async () => {
        runtime._setIsDryRun(true);

        const output1 = new Output(new Set(), Promise.resolve("outer"), Promise.resolve(true), Promise.resolve(true), Promise.resolve(new Set()));
        const output2 = output1.apply(v => new Output(new Set(), Promise.resolve("inner"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set())));

        const isSecret = await output2.isSecret;
        assert.equal(isSecret, true);

        const value = await output2.promise();
        assert.equal(value, "inner");
    }));

    describe("isKnown", () => {
        function or<T>(output1: Output<T>, output2: Output<T>): Output<T> {
            const val1 = output1.promise();
            const val2 = output2.promise();
            return new Output<T>(
                new Set([...output1.resources(), ...output2.resources()]),
                Promise.all([val1, val2])
                       .then(([val1, val2]) => val1 || val2),
                Promise.all([val1, output1.isKnown, output2.isKnown])
                       .then(([val1, isKnown1, isKnown2]) => val1 ? isKnown1 : isKnown2),
                Promise.all([val1, output1.isSecret, output2.isSecret])
                       .then(([val1, isSecret1, isSecret2]) => val1 ? isSecret1 : isSecret2),
                Promise.all([output1.allResources!(), output2.allResources!()])
                       .then(([r1, r2]) => new Set([...r1, ...r2])));
        }

        it("choose between known and known output, non-secret", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve("foo"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));
            const o2 = new Output(new Set(), Promise.resolve("bar"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));

            const result = or(o1, o2);

            const isKnown = await result.isKnown;
            assert.equal(isKnown, true);

            const value = await result.promise();
            assert.equal(value, "foo");

            const secret = await result.isSecret;
            assert.equal(secret, false);
        }));

        it("choose between known and known output, secret", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve("foo"), Promise.resolve(true), Promise.resolve(true), Promise.resolve(new Set()));
            const o2 = new Output(new Set(), Promise.resolve("bar"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));

            const result = or(o1, o2);

            const isKnown = await result.isKnown;
            assert.equal(isKnown, true);

            const value = await result.promise();
            assert.equal(value, "foo");

            const secret = await result.isSecret;
            assert.equal(secret, true);
        }));

        it("choose between known and unknown output, non-secret", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve("foo"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));
            const o2 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(false), Promise.resolve(new Set()));

            const result = or(o1, o2);

            const isKnown = await result.isKnown;
            assert.equal(isKnown, true);

            const value = await result.promise();
            assert.equal(value, "foo");

            const secret = await result.isSecret;
            assert.equal(secret, false);
        }));

        it("choose between known and unknown output, secret", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve("foo"), Promise.resolve(true), Promise.resolve(true), Promise.resolve(new Set()));
            const o2 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(false), Promise.resolve(new Set()));

            const result = or(o1, o2);

            const isKnown = await result.isKnown;
            assert.equal(isKnown, true);

            const value = await result.promise();
            assert.equal(value, "foo");

            const secret = await result.isSecret;
            assert.equal(secret, true);
        }));

        it("choose between unknown and known output, non-secret", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(false), Promise.resolve(new Set()));
            const o2 = new Output(new Set(), Promise.resolve("bar"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));

            const result = or(o1, o2);

            const isKnown = await result.isKnown;
            assert.equal(isKnown, true);

            const value = await result.promise();
            assert.equal(value, "bar");

            const secret = await result.isSecret;
            assert.equal(secret, false);
        }));

        it("choose between unknown and known output, secret", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(false), Promise.resolve(new Set()));
            const o2 = new Output(new Set(), Promise.resolve("bar"), Promise.resolve(true), Promise.resolve(true), Promise.resolve(new Set()));

            const result = or(o1, o2);

            const isKnown = await result.isKnown;
            assert.equal(isKnown, true);

            const value = await result.promise();
            assert.equal(value, "bar");

            const secret = await result.isSecret;
            assert.equal(secret, true);
        }));

        it("choose between unknown and unknown output, non-secret", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(false), Promise.resolve(new Set()));
            const o2 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(false), Promise.resolve(new Set()));

            const result = or(o1, o2);

            const isKnown = await result.isKnown;
            assert.equal(isKnown, false);

            const value = await result.promise();
            assert.equal(value, undefined);

            const secret = await result.isSecret;
            assert.equal(secret, false);
        }));

        it("choose between unknown and unknown output, secret1", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(true), Promise.resolve(new Set()));
            const o2 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(false), Promise.resolve(new Set()));

            const result = or(o1, o2);

            const isKnown = await result.isKnown;
            assert.equal(isKnown, false);

            const value = await result.promise();
            assert.equal(value, undefined);

            const secret = await result.isSecret;
            assert.equal(secret, false);
        }));

        it("choose between unknown and unknown output, secret2", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(false), Promise.resolve(new Set()));
            const o2 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(true), Promise.resolve(new Set()));

            const result = or(o1, o2);

            const isKnown = await result.isKnown;
            assert.equal(isKnown, false);

            const value = await result.promise();
            assert.equal(value, undefined);

            const secret = await result.isSecret;
            assert.equal(secret, true);
        }));

        it("choose between unknown and unknown output, secret3", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(true), Promise.resolve(new Set()));
            const o2 = new Output(new Set(), Promise.resolve(undefined), Promise.resolve(false), Promise.resolve(true), Promise.resolve(new Set()));

            const result = or(o1, o2);

            const isKnown = await result.isKnown;
            assert.equal(isKnown, false);

            const value = await result.promise();
            assert.equal(value, undefined);

            const secret = await result.isSecret;
            assert.equal(secret, true);
        }));

        it("is unknown if the value is or contains unknowns", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve(unknown), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));
            const o2 = new Output(new Set(), Promise.resolve(["foo", unknown]), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));
            const o3 = new Output(new Set(), Promise.resolve({"foo": "foo", unknown}), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));

            assert.equal(await o1.isKnown, false);
            assert.equal(await o2.isKnown, false);
            assert.equal(await o3.isKnown, false);
        }));

        it("is unknown if the result after apply is unknown or contains unknowns", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const o1 = new Output(new Set(), Promise.resolve("foo"), Promise.resolve(true), Promise.resolve(false), Promise.resolve(new Set()));
            const r1 = o1.apply(v => unknown);
            const r2 = o1.apply(v => [v, unknown]);
            const r3 = o1.apply(v => <any>{v, unknown});
            const r4 = (<any>o1.apply(v => unknown)).apply((v: any) => v, true);
            const r5 = (<any>o1.apply(v => [v, unknown])).apply((v: any) => v, true);
            const r6 = (<any>o1.apply(v => <any>{v, unknown})).apply((v: any) => v, true);

            assert.equal(await r1.isKnown, false);
            assert.equal(await r2.isKnown, false);
            assert.equal(await r3.isKnown, false);
            assert.equal(await r4.isKnown, false);
            assert.equal(await r5.isKnown, false);
            assert.equal(await r6.isKnown, false);
        }));
    });

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

        it("lifts properties from values with nested unknowns", asyncTest(async () => {
            runtime._setIsDryRun(true);

            const output1 = output({
                foo: "foo",
                bar: unknown,
                baz: Promise.resolve(unknown),
                qux: mockOutput(false, undefined),
            });
            assert.equal(await output1.isKnown, false);

            const result1 = output1.foo;
            assert.equal(await result1.isKnown, true);
            assert.equal(await (<any>result1).promise(/*withUnknowns*/ true), "foo");

            const result2 = output1.bar;
            assert.equal(await result2.isKnown, false);
            assert.equal(await (<any>result2).promise(/*withUnknowns*/ true), unknown);

            const result3 = output1.baz;
            assert.equal(await result3.isKnown, false);
            assert.equal(await (<any>result3).promise(/*withUnknowns*/ true), unknown);

            const result4 = output1.qux;
            assert.equal(await result4.isKnown, false);
            assert.equal(await (<any>result4).promise(/*withUnknowns*/ true), unknown);

            const result5 = (<any>output1.baz).qux;
            assert.equal(await result5.isKnown, false);
            assert.equal(await (<any>result5).promise(/*withUnknowns*/ true), unknown);

            const output2 = output([ "foo", unknown, mockOutput(false, undefined) ]);
            assert.equal(await output2.isKnown, false);

            const result6 = output2[0];
            assert.equal(await result6.isKnown, true);
            assert.equal(await (<any>result6).promise(/*withUnknowns*/ true), "foo");

            const result7 = output2[1];
            assert.equal(await result7.isKnown, false);
            assert.equal(await (<any>result7).promise(/*withUnknowns*/ true), unknown);

            const result8 = output2[2];
            assert.equal(await result8.isKnown, false);
            assert.equal(await (<any>result8).promise(/*withUnknowns*/ true), unknown);

            const output3 = all([ unknown, mockOutput(false, undefined), output([ "foo", unknown ])]);
            assert.equal(await output3.isKnown, false);

            const result9 = output3[0];
            assert.equal(await result9.isKnown, false);
            assert.equal(await (<any>result9).promise(/*withUnknowns*/ true), unknown);

            const result10 = output3[1];
            assert.equal(await result10.isKnown, false);
            assert.equal(await (<any>result10).promise(/*withUnknowns*/ true), unknown);

            const result11 = output3[2];
            assert.equal(await result11.isKnown, false);

            const result12 = (<any>result11)[0];
            assert.equal(await result12.isKnown, true);
            assert.equal(await (<any>result12).promise(/*withUnknowns*/ true), "foo");

            const result13 = (<any>result11)[1];
            assert.equal(await result13.isKnown, false);
            assert.equal(await (<any>result13).promise(/*withUnknowns*/ true), unknown);

            const output4 = all({
                foo: unknown,
                bar: mockOutput(false, undefined),
                baz: output({ foo: "foo", qux: unknown }),
            });
            assert.equal(await output4.isKnown, false);

            const result14 = output4.foo;
            assert.equal(await result14.isKnown, false);
            assert.equal(await (<any>result14).promise(/*withUnknowns*/ true), unknown);

            const result15 = output4.bar;
            assert.equal(await result15.isKnown, false);
            assert.equal(await (<any>result15).promise(/*withUnknowns*/ true), unknown);

            const result16 = output4.baz;
            assert.equal(await result16.isKnown, false);

            const result17 = (<any>result16).foo;
            assert.equal(await result17.isKnown, true);
            assert.equal(await (<any>result17).promise(/*withUnknowns*/ true), "foo");

            const result18 = (<any>result16).qux;
            assert.equal(await result18.isKnown, false);
            assert.equal(await (<any>result18).promise(/*withUnknowns*/ true), unknown);
        }));
    });
});
