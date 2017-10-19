// Ensure that certain runtime settings are available.

const assert = require("assert");
const pulumi = require("../../../../../");

assert.equal(pulumi.getProject(), "runtimeSettingsProject");
assert.equal(pulumi.getStack(), "runtimeSettingsStack");

const config = new pulumi.Config("myBag");
assert.equal(config.getNumber("A"), 42);
assert.equal(config.requireNumber("A"), 42);
assert.equal(config.get("bbbb"), "a string o' b's");
assert.equal(config.require("bbbb"), "a string o' b's");
assert.equal(config.get("missingC"), undefined);

