// This test asserts that node_modules can be found when package.json
// is located in a parent directory. We extract a dummy value from an
// installed package. 
import { version as pulumiVersion } from "@pulumi/pulumi/version";
import { strict as assert } from 'assert';

const version = pulumiVersion;
// As long as these values are truthy, we've successfully loaded
// the dep from node_modules
assert(version);
assert(version === pulumiVersion);
