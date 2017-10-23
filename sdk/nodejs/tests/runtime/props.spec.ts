// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { ComputedValues, runtime } from "../../index";
import { asyncTest } from "../util";

describe("runtime", () => {
    describe("transferProperties", () => {
        it("marshals basic properties correctly", asyncTest(async () => {
            const inputs: ComputedValues = {
                "aNum": 42,
                "bStr": "a string",
                "cUnd": undefined,
                "dArr": [ "x", 42, true, undefined ],
            };
            // Serialize and then deserialize all the properties, checking that they round-trip as expected.
            const transfer: runtime.PropertyTransfer =
                await runtime.transferProperties(undefined, "test", inputs, undefined);
            const result: any = runtime.deserializeProperties(transfer.obj);
            assert.equal(result.aNum, 42);
            assert.equal(result.bStr, "a string");
            assert.equal(result.cUnd, undefined);
            assert.deepEqual(result.dArr, [ "x", 42, true, undefined ]);
        }));
    });
});

