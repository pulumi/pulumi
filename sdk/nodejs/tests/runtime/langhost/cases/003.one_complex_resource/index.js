// This tests the creation of a resource that contains "simple" input and output propeprties.
// In particular, there aren't any fancy dataflow linked properties.

let assert = require("assert");
let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:MyResource", name, {
            // First a few basic properties that are resolved to values.
            inpropB1: false,
            inpropB2: true,
            inpropN: 42,
            inpropS: "a string",
            inpropA: [true, 99, "what a great property"],
            inpropO: {
                b1: false,
                b2: true,
                n: 42,
                s: "another string",
                a: [66, false, "strings galore"],
                o: { z: "x" },
            },

            // Next some properties that are completely unresolved (outputs) but will be provided after creation.
            outprop1: undefined,
            outprop2: undefined,

            // Finally define an output property that will not be provided after creation.
            outprop3: undefined,
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
res.outprop1.apply((prop) => {
    console.log(`OutProp1: ${prop}`);
    assert.strictEqual(prop, "output properties ftw");
});
res.outprop2.apply((prop) => {
    console.log(`OutProp2: ${prop}`);
    assert.strictEqual(prop, 998.6);
});
res.outprop3.apply((prop) => {
    console.log(`OutProp3: ${prop}`);
    assert.strictEqual(prop, undefined);
});

let resOutput = pulumi.output(res);
resOutput.urn.apply((urn) => {
    console.log(`URN: ${urn}`);
    assert.strictEqual(urn, "test:index:MyResource::testResource1");
});
resOutput.id.apply((id) => {
    console.log(`ID: ${id}`);
    assert.strictEqual(id, "testResource1");
});
resOutput.outprop1.apply((prop) => {
    console.log(`OutProp1: ${prop}`);
    assert.strictEqual(prop, "output properties ftw");
});
resOutput.outprop2.apply((prop) => {
    console.log(`OutProp2: ${prop}`);
    assert.strictEqual(prop, 998.6);
});
resOutput.outprop3.apply((prop) => {
    console.log(`OutProp3: ${prop}`);
    assert.strictEqual(prop, undefined);
});
