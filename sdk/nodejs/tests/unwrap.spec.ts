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
import { all, output, Output, Resource } from "../index";
import { asyncTest } from "./util";

function test(val: any, expected: any) {
    return asyncTest(async () => {
        const unwrapped = output(val);
        const actual = await unwrapped.promise();
        assert.deepStrictEqual(actual, expected);
    });
}

function testUntouched(val: any) {
    return test(val, val);
}

function testPromise(val: any) {
    return test(Promise.resolve(val), val);
}

function testOutput(val: any) {
    return test(output(val), val);
}

function testResources(val: any, expected: any, resources: TestResource[]) {
    return asyncTest(async () => {
        const unwrapped = output(val);
        const actual = await unwrapped.promise();

        assert.deepStrictEqual(actual, expected);
        assert.deepStrictEqual(await unwrapped.resources(), new Set(resources));

        const unwrappedResources: TestResource[] = <any>[...await unwrapped.resources()];
        unwrappedResources.sort((r1, r2) => r1.name.localeCompare(r2.name));

        resources.sort((r1, r2) => r1.name.localeCompare(r2.name));
        assert.equal(
            JSON.stringify(unwrappedResources),
            JSON.stringify(resources));
    });
}

class TestResource {
    // fake being a pulumi resource.  We can't actually derive from Resource as that then needs an
    // engine and whatnot.  All things we don't want during simple unit tests.
    private readonly __pulumiResource: boolean = true;

    constructor(public name: string) {
    }
}

// Helper type to try to do type asserts.  Note that it's not totally safe.  If TS thinks a type is
// the 'any' type, it will succeed here.  Talking to the TS team, it does not look like there's a
// way to write a totally airtight type assertion.

type EqualsType<X, Y> = X extends Y ? Y extends X ? X : never : never;

describe("unwrap", () => {
    describe("handles simple", () => {
        it("null", testUntouched(null));
        it("undefined", testUntouched(undefined));
        it("true", testUntouched(true));
        it("false", testUntouched(false));
        it("0", testUntouched(0));
        it("numbers", testUntouched(4));
        it("empty string", testUntouched(""));
        it("strings", testUntouched("foo"));
        it("arrays", testUntouched([]));
        it("object", testUntouched({}));
        it("function", testUntouched(() => {}));
    });

    describe("handles promises", () => {
        it("with null", testPromise(null));
        it("with undefined", testPromise(undefined));
        it("with true", testPromise(true));
        it("with false", testPromise(false));
        it("with 0", testPromise(0));
        it("with numbers", testPromise(4));
        it("with empty string", testPromise(""));
        it("with strings", testPromise("foo"));
        it("with array", testPromise([]));
        it("with object", testPromise({}));
        it("with function", testPromise(() => {}));
        it("with nested promise", test(Promise.resolve(Promise.resolve(4)), 4))
    });

    describe("handles outputs", () => {
        it("with null", testOutput(null));
        it("with undefined", testOutput(undefined));
        it("with true", testOutput(true));
        it("with false", testOutput(false));
        it("with 0", testOutput(0));
        it("with numbers", testOutput(4));
        it("with empty string", testOutput(""));
        it("with strings", testOutput("foo"));
        it("with array", testOutput([]));
        it("with object", testOutput({}));
        it("with function", testOutput(() => {}));
        it("with nested output", test(output(output(4)), 4));
        it("with output of promise", test(output(Promise.resolve(4)), 4));
    });

    describe("handles arrays", () => {
        it("empty", testUntouched([]));
        it("with primitives", testUntouched([1, true]));
        it("with inner promise", test([1, true, Promise.resolve("")], [1, true, ""]));
        it("with inner and outer promise", test(Promise.resolve([1, true, Promise.resolve("")]), [1, true, ""]));
        it("recursion", test([1, Promise.resolve(""), [true, Promise.resolve(4)]], [1, "", [true, 4 ]]));
    });

    describe("handles complex object", () => {
        it("empty", testUntouched({}));
        it("with primitives", testUntouched({ a: 1, b: true, c: () => {} }));
        it("with inner promise", test({ a: 1, b: true, c: Promise.resolve("") }, { a: 1, b: true, c: "" }));
        it("with inner and outer promise", test(Promise.resolve({ a: 1, b: true, c: Promise.resolve("") }), { a: 1, b: true, c: "" }));
        it("recursion", test({ a: 1, b: Promise.resolve(""), c: { d: true, e: Promise.resolve(4) } }, { a: 1, b: "", c: { d: true, e: 4 } }));
    });

    function createOutput<T>(cv: T, ...resources: TestResource[]): Output<T> {
        return Output.isInstance<T>(cv)
            ? cv
            : new Output(<any>new Set(resources), Promise.resolve(cv), Promise.resolve(true), Promise.resolve(false))
    }

    describe("preserves resources", () => {
        const r1 = new TestResource("r1");
        const r2 = new TestResource("r2");
        const r3 = new TestResource("r3");
        const r4 = new TestResource("r4");
        const r5 = new TestResource("r5");
        const r6 = new TestResource("r6");

        // assert.deepEqual(r1, r2);

        it("with single output", testResources(
            createOutput(3, r1, r2),
            3,
            [r1, r2]));

        it("inside array", testResources(
            [createOutput(3, r1, r2)],
            [3],
            [r1, r2]));

        it("inside multi array", testResources(
            [createOutput(1, r1, r2),createOutput(2, r2, r3)],
            [1, 2],
            [r1, r2, r3]));

        it("inside nested array", testResources(
            [createOutput(1, r1, r2), createOutput(2, r2, r3), [createOutput(3, r5)]],
            [1, 2, [3]],
            [r1, r2, r3, r5]));

        it("inside object", testResources(
            { a: createOutput(3, r1, r2) },
            { a: 3 },
            [r1, r2]));

        it("inside multi object", testResources(
            { a: createOutput(1, r1, r2), b: createOutput(2, r2, r3) },
            { a: 1, b: 2 },
            [r1, r2, r3]));

        it("inside nested object", testResources(
            { a: createOutput(1, r1, r2), b: createOutput(2, r2, r3), c: { d: createOutput(3, r5) } },
            { a: 1, b: 2, c: { d: 3 } },
            [r1, r2, r3, r5]));

        it("across inner promise", testResources(
            createOutput(Promise.resolve(3), r1, r2),
            3,
            [r1, r2]));
    });

    describe("preserve all resources across promise boundaries", () => {
        const r1 = new TestResource("r1");
        const r2 = new TestResource("r2");
        const r3 = new TestResource("r3");
        const r4 = new TestResource("r4");
        const r5 = new TestResource("r5");
        const r6 = new TestResource("r6");

        // in these tests, not all resources are preserved as they may cross promise boundaries.

        it("inside and outside of array", testResources(
            createOutput([createOutput(3, r1, r2)], r2, r3),
            [3],
            [r1, r2, r3]));

        it("inside and outside of object", testResources(
            createOutput({ a: createOutput(3, r1, r2) }, r2, r3),
            { a: 3 },
            [r1, r2, r3]));

        it("inside nested object and array", testResources(
            { a: createOutput(1, r1, r2), b: createOutput(2, r2, r3), c: { d: createOutput([createOutput(3, r5)], r6)  } },
            { a: 1, b: 2, c: { d: [3] } },
            [r1, r2, r3, r5, r6]));

        it("inside nested array and object", testResources(
            { a: createOutput(1, r1, r2), b: createOutput(2, r2, r3), c: createOutput([{ d: createOutput(3, r5) }], r6) },
            { a: 1, b: 2, c: [{ d: 3 }] },
            [r1, r2, r3, r5, r6]));

        it("across outer promise", testResources(
            Promise.resolve(createOutput(3, r1, r2)),
            3,
            [r1, r2]));

        it("across inner and outer promise", testResources(
            Promise.resolve(createOutput(Promise.resolve(3), r1, r2)),
            3,
            [r1, r2]));

        it("across promise and inner object", testResources(
            Promise.resolve(createOutput(Promise.resolve({ a: createOutput(1, r4, r5)}), r1, r2)),
            { a: 1 },
            [r1, r2, r4, r5]));

        it("across promise and inner array and object", testResources(
            Promise.resolve(createOutput([Promise.resolve({ a: createOutput(1, r4, r5)})], r1, r2)),
            [{ a: 1 }],
            [r1, r2, r4, r5]));

        it("across inner object", testResources(
            createOutput(Promise.resolve({ a: createOutput(1, r4, r5)}), r1, r2),
            { a: 1 },
            [r1, r2, r4, r5]));

        it("across 'all'", testResources(
            all([Promise.resolve({ a: createOutput(1, r1, r2)}), Promise.resolve({ b: createOutput(2, r3, r4)})]),
            [{ a: 1 }, { b: 2 }],
            [r1, r2, r3, r4]));
    });


    describe("type system", () => {
        it ("across promises", asyncTest(async () => {
            var v = { a: 1, b: Promise.resolve(""), c: { d: true, e: Promise.resolve(4) } };
            var xOutput = output(v);
            var x = await xOutput.promise();

            // Ensure that ts thinks that 'e' is a number.
            const z: EqualsType<typeof x.c.e, number> = 1;

            // The runtime value better be a number;
            x.c.e.toExponential();
        }));

        it ("across nested promises", asyncTest(async () => {
            var v = { a: 1, b: Promise.resolve(""), c: Promise.resolve({ d: true, e: Promise.resolve(4) }) };
            var xOutput = output(v);
            var x = await xOutput.promise();

            // Ensure that ts thinks that 'e' is a number.
            const z: EqualsType<typeof x.c.e, number> = 1;

            // The runtime value better be a number;
            x.c.e.toExponential();
        }));

        it ("across outputs", asyncTest(async () => {
            var v = { a: 1, b: Promise.resolve(""), c: output({ d: true, e: [4, 5, 6] }) };
            var xOutput = output(v);
            var x = await xOutput.promise();

            // Ensure that ts thinks that 'e' is an array of numbers;
            const z: EqualsType<typeof x.c.e, number[]> = x.c.e;

            // The runtime value better be a number[]
            x.c.e.push(1);
        }));

        it ("across nested outputs", asyncTest(async () => {
            var v = { a: 1, b: Promise.resolve(""), c: output({ d: true, e: output([4, 5, 6]) }) };
            var xOutput = output(v);
            var x = await xOutput.promise();

            // Ensure that ts thinks that 'e' is an array of numbers;
            const z: EqualsType<typeof x.c.e, number[]> = x.c.e;

            // The runtime value better be a number[]
            x.c.e.push(1);
        }));

        it ("across promise and output", asyncTest(async () => {
            var v = { a: 1, b: Promise.resolve(""), c: Promise.resolve({ d: true, e: output([4, 5, 6]) }) };
            var xOutput = output(v);
            var x = await xOutput.promise();

            // Ensure that ts thinks that 'e' is an array of numbers;
            const z: EqualsType<typeof x.c.e, number[]> = x.c.e;

            // The runtime value better be a number[]
            x.c.e.push(1);
        }));

        it ("across output and promise", asyncTest(async () => {
            var v = { a: 1, b: Promise.resolve(""), c: output({ d: true, e: Promise.resolve([4, 5, 6]) }) };
            var xOutput = output(v);
            var x = await xOutput.promise();

            // Ensure that ts thinks that 'e' is an array of numbers;
            const z: EqualsType<typeof x.c.e, number[]> = x.c.e;

            // The runtime value better be a number[]
            x.c.e.push(1);
        }));

        it ("does not wrap functions", asyncTest(async () => {
            var sentinel = function(_: () => void) {}

            // `v` should be type `() => void` rather than `UnwrappedObject<void>`.
            output(function() {}).apply(v => sentinel(v));
        }));
    });

    it("handles all in one", test(
        Promise.resolve([1, output({ a: [Promise.resolve([1, 2, { b: true, c: null }, undefined])]})]),
        [1, { a: [[1, 2, { b: true, c: null }, undefined]]}]
    ));
});