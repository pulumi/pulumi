// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import { Config } from "@pulumi/pulumi";
import * as assert from "assert";

// Just test that basic config works.
const config = new Config("config_basic_js");

// This value is plaintext and doesn't require encryption.
const value = config.require("aConfigValue");
assert.strictEqual(value, "this value is a value", "'aConfigValue' not the expected value");

const phonenumber = config.require("phonenumber");
assert.strictEqual(phonenumber, "+441234567890", "'phonenumber' not the expected value");

// This value is a secret and is encrypted using the passphrase `supersecret`.
const secret = config.require("bEncryptedSecret");
assert.strictEqual(secret, "this super secret is encrypted", "'bEncryptedSecret' not the expected value");

const testData: {
    key: string;
    expectedJSON: string;
    expectedObject: any;
}[] = [
    {
        key: "outer",
        expectedJSON: `{"inner":"value"}`,
        expectedObject: { inner: "value" },
    },
    {
        key: "names",
        expectedJSON: `["a","b","c","super secret name"]`,
        expectedObject: ["a", "b", "c", "super secret name"],
    },
    {
        key: "servers",
        expectedJSON: `[{"host":"example","port":80}]`,
        expectedObject: [{ host: "example", port: 80 }],
    },
    {
        key: "a",
        expectedJSON: `{"b":[{"c":true},{"c":false}]}`,
        expectedObject: { b: [{ c: true }, { c: false }] },
    },
    {
        key: "tokens",
        expectedJSON: `["shh"]`,
        expectedObject: ["shh"],
    },
    {
        key: "foo",
        expectedJSON: `{"bar":"don't tell"}`,
        expectedObject: { bar: "don't tell" },
    },
];

for (const test of testData) {
    const json = config.require(test.key);
    const obj = config.requireObject(test.key);
    assert.strictEqual(json, test.expectedJSON, `'${test.key}' not the expected JSON`);
    assert.deepStrictEqual(obj, test.expectedObject, `'${test.key}' not the expected object`);
}
