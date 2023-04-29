// This tests the creation of ten resources that contains "simple" input and output propeprties.
// In particular, there aren't any fancy dataflow linked properties.

let assert = require("assert");
let MyResource = require("./resource").MyResource;

for (let i = 0; i < 10; i++) {
    let name = "testResource" + i;
    let res = new MyResource(name, i);
    res.urn.apply((urn) => {
        console.log(`${name}.URN: ${urn}`);
        assert.strictEqual(urn, "test:index:MyResource::" + name);
    });
    res.id.apply((id) => {
        if (id) {
            console.log(`${name}.ID: ${id}`);
            assert.strictEqual(id, name);
        }
    });
    res.outseq.apply((seq) => {
        if (seq) {
            console.log(`${name}.Seq: ${seq}`);
            assert.strictEqual(seq, i);
        }
    });
    res.outprop1.apply((prop) => {
        if (prop) {
            console.log(`${name}.OutProp1: ${prop}`);
            assert.strictEqual(prop, "output properties ftw");
        }
    });
    res.outprop2.apply((prop) => {
        if (prop) {
            console.log(`${name}.OutProp2: ${prop}`);
            assert.strictEqual(prop, 998.6);
        }
    });
}
