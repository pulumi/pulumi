// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { Inputs, runtime } from "../../index";
import { asyncTest } from "../util";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

describe("runtime", () => {
    describe("transferProperties", () => {
        it("marshals basic properties correctly", asyncTest(async () => {
            const inputs: Inputs = {
                "aNum": 42,
                "bStr": "a string",
                "cUnd": undefined,
                "dArr": Promise.resolve([ "x", 42, Promise.resolve(true), Promise.resolve(undefined) ]),
            };
            // Serialize and then deserialize all the properties, checking that they round-trip as expected.
            const transfer = gstruct.Struct.fromJavaScript(
                await runtime.serializeProperties("test", inputs));
            const result = runtime.deserializeProperties(transfer);
            assert.equal(result.aNum, 42);
            assert.equal(result.bStr, "a string");
            assert.equal(result.cUnd, undefined);
            assert.deepEqual(result.dArr, [ "x", 42, true, undefined ]);
        }));
    });
});

