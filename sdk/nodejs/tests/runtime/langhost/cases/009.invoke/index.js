// Test the ability to invoke provider functions via RPC.

let assert = require("assert");
let pulumi = require("../../../../../");

let args = {
    a: "hello",
    b: true,
    c: [0.99, 42, { z: "x" }],
    id: "some-id",
    urn: "some-urn",
};

let result1 = pulumi.runtime.invoke("invoke:index:echo", args);
for (const key in args) {
    assert.deepEqual(result1[key], args[key]);
}

let result2 = pulumi.runtime.invoke("invoke:index:echo", args);
result2.then((v) => {
    assert.deepEqual(v, args);
});

let result3 = pulumi.runtime.invoke("invoke:index:echo", args, { async: true });
result3.then((v) => {
    assert.deepEqual(v, args);
});
