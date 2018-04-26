// Test the ability to invoke provider functions via RPC.

let assert = require("assert");
let pulumi = require("../../../../../");

let args = {
    a: "hello",
    b: true,
    c: [ 0.99, 42, { z: "x" } ],
    id: "some-id",
    urn: "some-urn",
};
let result = pulumi.runtime.invoke("invoke:index:echo", args);
result.then((v) => {
    assert.deepEqual(v, args);
});

