// This test case links one resource's property to another.

let assert = require("assert");
let fabric = require("../../../../lib");

class ResourceA extends fabric.Resource {
    constructor(name) {
        super("test:index:ResourceA", name, {
            "inprop": new fabric.Property(777),
            "outprop": new fabric.Property(),
        });
    }
}

class ResourceB extends fabric.Resource {
    constructor(name, other) {
        super("test:index:ResourceB", name, {
            "otherIn": new fabric.Property(other.inprop),
            "otherOut": new fabric.Property(other.outprop),
        });
    }
}

// First create and validate a simple resource A with an input and output.
let a = new ResourceA("resourceA");
a.id.then(id => {
    console.log(`A.ID: ${id}`);
    assert.equal(id, "tresourceA");
});
a.urn.then(urn => {
    console.log(`A.URN: ${urn}`);
    assert.equal(urn, "test:index:ResourceA::resourceA");
});
a.inprop.then(prop => {
    console.log(`A.InProp: ${prop}`);
    assert.equal(prop, 777);
});
a.outprop.then(prop => {
    console.log(`A.OutProp: ${prop}`);
    assert.equal(prop, "output yeah");
});

// Next, create and validate another resource B which depends upon resource A.
let b = new ResourceB("resourceB", a);
b.id.then(id => {
    console.log(`B.ID: ${id}`);
    assert.equal(id, "tresourceB");
});
b.urn.then(urn => {
    console.log(`B.URN: ${urn}`);
    assert.equal(urn, "test:index:ResourceB::resourceB");
});
b.otherIn.then(prop => {
    console.log(`B.OtherIn: ${prop}`);
    assert.equal(prop, 777);
});
b.otherOut.then(prop => {
    console.log(`B.OtherOut: ${prop}`);
    assert.equal(prop, "output yeah");
});

