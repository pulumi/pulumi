// Plain invoke calls serialize_properties on the arguments, so even though it
// is supposed to only take plain values, it still happens to work with Outputs.
// In practice, people rely on this behavior, so we test it here to ensure we
// don't break this behavior.

const assert = require("assert");
const pulumi = require("../../../../../");

const dependency = { resolved: false };

class MyResource extends pulumi.CustomResource {
    constructor(name, opts) {
        super("test:index:MyResource", name, {}, opts);
    }
}

const dep = new MyResource("dep");

const arg = new pulumi.Output(
    new Set([dep]), // dependencies, unknown during preview
    Promise.resolve("banana"),
    Promise.resolve(true), // isKnown
    Promise.resolve(false), // isSecret
);

pulumi.runtime.invoke("test:index:echo", { arg }).then((result) => {
    assert.deepStrictEqual({ arg: "banana" }, result);
});
