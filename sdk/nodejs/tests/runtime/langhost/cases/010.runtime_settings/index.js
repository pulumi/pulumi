// Ensure that certain runtime settings are available.

const assert = require("assert");
const pulumi = require("../../../../../");

assert.strictEqual(pulumi.getProject(), "runtimeSettingsProject");
assert.strictEqual(pulumi.getStack(), "runtimeSettingsStack");

const config = new pulumi.Config("myBag");
assert.strictEqual(config.getNumber("A"), 42);
assert.strictEqual(config.requireNumber("A"), 42);
assert.strictEqual(config.get("bbbb"), "a string o' b's");
assert.strictEqual(config.require("bbbb"), "a string o' b's");
assert.strictEqual(config.get("missingC"), undefined);

// ensure the older format works as well
const configOld = new pulumi.Config("myBag:config");
assert.strictEqual(configOld.getNumber("A"), 42);
assert.strictEqual(configOld.requireNumber("A"), 42);
assert.strictEqual(configOld.get("bbbb"), "a string o' b's");
assert.strictEqual(configOld.require("bbbb"), "a string o' b's");
assert.strictEqual(configOld.get("missingC"), undefined);
