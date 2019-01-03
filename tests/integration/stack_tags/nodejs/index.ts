// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { Config, getStackTag, getStackTags } from "@pulumi/pulumi";

const config = new Config();
const customtag = config.requireBoolean("customtag");

const expected = {
    "pulumi:project": "stack_tags_js",
    "pulumi:runtime": "nodejs",
    "pulumi:description": "A simple Node.js program that uses stack tags",
};
if (customtag) {
    expected["foo"] = "bar";
}

for (const name of Object.keys(expected)) {
    assert.equal(getStackTag(name), expected[name], `${name} not the expected value from getStackTag`);
}

const tags = getStackTags();
for (const name of Object.keys(expected)) {
    assert.equal(tags[name], expected[name], `${name} not the expected value from getStackTags`);
}
