// This test case links one resource's property to another.

let assert = require("assert");
let pulumi = require("../../../../../");

class ResourceA extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:ResourceA", name, {
            "inprop": 777,
            "outprop": undefined,
        });
    }
}

class ResourceB extends pulumi.CustomResource {
    constructor(name, other) {
        super("test:index:ResourceB", name, {
            "otherIn": other.inprop,
            "otherOut": other.outprop,
        });
    }
}

// First create and validate a simple resource A with an input and output.
let a = new ResourceA("resourceA");
a.urn.promise().then(urn => {
    console.log(`A.URN: ${urn}`);
    assert.equal(urn, "test:index:ResourceA::resourceA");
});
a.id.promise().then(id => {
    if (id) {
        console.log(`A.ID: ${id}`);
        assert.equal(id, "resourceA");
    }
});
a.inprop.promise().then(prop => {
    if (prop) {
        console.log(`A.InProp: ${prop}`);
        assert.equal(prop, 777);
    }
});
a.outprop.promise().then(prop => {
    if (prop) {
        console.log(`A.OutProp: ${prop}`);
        assert.equal(prop, "output yeah");
    }
});

// Next, create and validate another resource B which depends upon resource A.
let b = new ResourceB("resourceB", a);
b.urn.promise().then(urn => {
    console.log(`B.URN: ${urn}`);
    assert.equal(urn, "test:index:ResourceB::resourceB");
});
b.id.promise().then(id => {
    if (id) {
        console.log(`B.ID: ${id}`);
        assert.equal(id, "resourceB");
    }
});
b.otherIn.promise().then(prop => {
    if (prop) {
        console.log(`B.OtherIn: ${prop}`);
        assert.equal(prop, 777);
    }
});
b.otherOut.promise().then(prop => {
    if (prop) {
        console.log(`B.OtherOut: ${prop}`);
        assert.equal(prop, "output yeah");
    }
});

