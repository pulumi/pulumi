// This test case links one resource's property to another.

let assert = require("assert");
let pulumi = require("../../../../../");

class ResourceA extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:ResourceA", name, {
            inprop: 777,
            outprop: undefined,
        });
    }
}

class ResourceB extends pulumi.CustomResource {
    constructor(name, other) {
        super("test:index:ResourceB", name, {
            otherIn: other.inprop,
            otherOut: other.outprop,
        });
    }
}

// First create and validate a simple resource A with an input and output.
let a = new ResourceA("resourceA");
a.urn.apply((urn) => {
    console.log(`A.URN: ${urn}`);
    assert.strictEqual(urn, "test:index:ResourceA::resourceA");
});
a.id.apply((id) => {
    if (id) {
        console.log(`A.ID: ${id}`);
        assert.strictEqual(id, "resourceA");
    }
});
a.inprop.apply((prop) => {
    if (prop) {
        console.log(`A.InProp: ${prop}`);
        assert.strictEqual(prop, 777);
    }
});
a.outprop.apply((prop) => {
    if (prop) {
        console.log(`A.OutProp: ${prop}`);
        assert.strictEqual(prop, "output yeah");
    }
});

// Next, create and validate another resource B which depends upon resource A.
let b = new ResourceB("resourceB", a);
b.urn.apply((urn) => {
    console.log(`B.URN: ${urn}`);
    assert.strictEqual(urn, "test:index:ResourceB::resourceB");
});
b.id.apply((id) => {
    if (id) {
        console.log(`B.ID: ${id}`);
        assert.strictEqual(id, "resourceB");
    }
});
b.otherIn.apply((prop) => {
    if (prop) {
        console.log(`B.OtherIn: ${prop}`);
        assert.strictEqual(prop, 777);
    }
});
b.otherOut.apply((prop) => {
    if (prop) {
        console.log(`B.OtherOut: ${prop}`);
        assert.strictEqual(prop, "output yeah");
    }
});
