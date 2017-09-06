// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { asyncTest, assertAsyncThrows } from "../util";
import { Computed, runtime } from "../../index";

function computedToPromise<T>(computed: Computed<T>): Promise<T> {
    return new Promise((resolve: any) => {
        computed.mapValue((res: T) => resolve(res));
    });
}

// Some basic computed tests.
describe("computed", () => {
    it("resolves mapValues correctly", asyncTest(async () => {
        let v1: Computed<string> = new runtime.Property<string>("x");
        let v11: Computed<string> = v1.mapValue((x: string) => x);
        assert.strictEqual(await computedToPromise(v11), "x");
        let v12: Computed<string> = v1.mapValue((x: string) => v1.mapValue((y: string) => x + "*" + y));
        assert.strictEqual(await computedToPromise(v12), "x*x");

        let v2res: any;
        let v2 = new runtime.Property<string>(new Promise<string>((resolve: any) => { v2res = resolve; }));
        v2res("y");
        let v21: Computed<string> = v2.mapValue((x: string) => x);
        assert.strictEqual(await computedToPromise(v21), "y");
        let v22: Computed<string> = v2.mapValue((x: string) => v2.mapValue((y: string) => x + "|" + y));
        assert.strictEqual(await computedToPromise(v22), "y|y");
    }));
});

