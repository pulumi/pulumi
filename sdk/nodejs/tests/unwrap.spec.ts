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
import { unwrap } from "../unwrap";
import { asyncTest } from "./util";

function test(val: any, expected: any) {
    return asyncTest(async () => {
        const actual = await unwrap(val).promise();
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
        const unwrapped = unwrap(val);
        const actual = await unwrapped.promise();

        assert.deepStrictEqual(actual, expected);
        // console.log(JSON.stringify([...unwrapped.resources()]));
        // console.log(JSON.stringify(resources));
        assert.equal(unwrapped.resources(), new Set(resources));
    });
}

class TestResource {
    // fake being a pulumi resource.  We can't actually derive from Resource as that then needs an
    // engine and whatnot.  All things we don't want during simple unit tests.
    private readonly __pulumiResource: boolean = true;

    constructor(public name: string) {

    }
}

describe("unwrap", () => {
    // describe("handles simple", () => {
    //     it("null", testUntouched(null));
    //     it("undefined", testUntouched(undefined));
    //     it("true", testUntouched(true));
    //     it("false", testUntouched(false));
    //     it("0", testUntouched(0));
    //     it("numbers", testUntouched(4));
    //     it("empty string", testUntouched(""));
    //     it("strings", testUntouched("foo"));
    //     it("arrays", testUntouched([]));
    //     it("object", testUntouched({}));
    // });

    // describe("handles promises", () => {
    //     it("with null", testPromise(null));
    //     it("with undefined", testPromise(undefined));
    //     it("with true", testPromise(true));
    //     it("with false", testPromise(false));
    //     it("with 0", testPromise(0));
    //     it("with numbers", testPromise(4));
    //     it("with empty string", testPromise(""));
    //     it("with strings", testPromise("foo"));
    //     it("with array", testPromise([]));
    //     it("with object", testPromise({}));
    //     it("with nested promise", test(Promise.resolve(Promise.resolve(4)), 4))
    // });

    // describe("handles outputs", () => {
    //     it("with null", testOutput(null));
    //     it("with undefined", testOutput(undefined));
    //     it("with true", testOutput(true));
    //     it("with false", testOutput(false));
    //     it("with 0", testOutput(0));
    //     it("with numbers", testOutput(4));
    //     it("with empty string", testOutput(""));
    //     it("with strings", testOutput("foo"));
    //     it("with array", testOutput([]));
    //     it("with object", testOutput({}));
    //     it("with nested output", test(output(output(4)), 4));
    //     it("with output of promise", test(output(Promise.resolve(4)), 4));
    // });

    // describe("handles arrays", () => {
    //     it("empty", testUntouched([]));
    //     it("with primitives", testUntouched([1, true]));
    //     it("with inner promise", test([1, true, Promise.resolve("")], [1, true, ""]));
    //     it("with inner and outer promise", test(Promise.resolve([1, true, Promise.resolve("")]), [1, true, ""]));
    //     it("recursion", test([1, Promise.resolve(""), [true, Promise.resolve(4)]], [1, "", [true, 4 ]]));
    // });

    // describe("handles complex object", () => {
    //     it("empty", testUntouched({}));
    //     it("with primitives", testUntouched({ a: 1, b: true }));
    //     it("with inner promise", test({ a: 1, b: true, c: Promise.resolve("") }, { a: 1, b: true, c: "" }));
    //     it("with inner and outer promise", test(Promise.resolve({ a: 1, b: true, c: Promise.resolve("") }), { a: 1, b: true, c: "" }));
    //     it("recursion", test({ a: 1, b: Promise.resolve(""), c: { d: true, e: Promise.resolve(4) } }, { a: 1, b: "", c: { d: true, e: 4 } }));
    // });

    describe("handles resources", () => {
        const r1 = new TestResource("r1");
        const r2 = new TestResource("r2");
        const r3 = new TestResource("r3");
        const r4 = new TestResource("r4");
        const r5 = new TestResource("r5");
        const r6 = new TestResource("r6");

        // assert.deepEqual(r1, r2);

        function createOutput<T>(val: T, ...resources: TestResource[]): Output<T> {
            return new Output(<Set<Resource>><any>new Set(resources), Promise.resolve(val), Promise.resolve(true));
        }

        it("inside and outside of array", testResources(
            createOutput([createOutput(1, r1), createOutput(2, r2, r3), [createOutput(3, r3, r4)]], r5),
            [1, 2, [3]],
            [r1, r2, r4]));
    });

    // it("handles all in one", test(
    //     Promise.resolve([1, output({ a: [Promise.resolve([1, 2, { b: true, c: null }, undefined])]})]),
    //     [1, { a: [[1, 2, { b: true, c: null }, undefined]]}]
    // ));
});