// This tests the creation of a resource that contains "simple" input and output propeprties.
// In particular, there aren't any fancy dataflow linked properties.

let assert = require("assert");
let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:MyResource", name, {
            prop: [pulumi.output(1), 2],
        });
    }
}

let res = new MyResource("testResource1");
res.urn.apply((urn) => {
    console.log(`URN: ${urn}`);
    assert.strictEqual(urn, "test:index:MyResource::testResource1");
});
res.id.apply((id) => {
    console.log(`ID: ${id}`);
    assert.strictEqual(id, "testResource1");
});
res.prop.apply((prop) => {
    console.log(`prop: ${prop}`);
    assert.deepStrictEqual(prop, [1, 2]);
});
