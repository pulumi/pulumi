// Test the ability to invoke provider functions via RPC.

let assert = require("assert");
let pulumi = require("../../../../../");

let inputs = {
    a: "fizzz",
    b: false,
    c: [ 0.73, "x", { zed: 923 } ],
    d: undefined,
};

let res = new pulumi.CustomResource("test:read:resource", "foo", inputs, { id: "abc123" });
res.id.apply(id => assert.strictEqual(id, "abc123"));
res.urn.apply(urn => assert.strictEqual(urn, "test:read:resource::foo"));
res.a.apply(a => assert.strictEqual(a, inputs.a)); // same as input
res.b.apply(b => assert.strictEqual(b, true)); // output changed to true
res.c.apply(c => assert.deepStrictEqual(c, inputs.c)); // same as input
res.d.apply(d => assert.strictEqual(d, "and then, out of nowhere ...")); // from the inputs
