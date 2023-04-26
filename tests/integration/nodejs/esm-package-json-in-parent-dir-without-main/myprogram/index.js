// This test asserts that node_modules can be found when package.json
// is located in a parent directory. We extract a dummy value from an
// installed package. 
import { version } from "@pulumi/pulumi";
import assert from "node:assert";

const myVersion = version;
// As long as these values are truthy, we've successfully loaded
// the dep from node_modules
assert.ok(myVersion);
assert.strictEquals(myVersion, version);