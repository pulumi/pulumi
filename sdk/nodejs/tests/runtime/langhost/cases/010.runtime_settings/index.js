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

// ensure the older format works as well
const configOld = new pulumi.Config("myBag:config");
assert.equal(configOld.getNumber("A"), 42);
assert.equal(configOld.requireNumber("A"), 42);
assert.equal(configOld.get("bbbb"), "a string o' b's");
assert.equal(configOld.require("bbbb"), "a string o' b's");
assert.equal(configOld.get("missingC"), undefined);
