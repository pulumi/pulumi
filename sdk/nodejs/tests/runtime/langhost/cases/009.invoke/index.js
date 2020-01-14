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

<<<<<<< HEAD
=======
let result1 = pulumi.runtime.invoke("invoke:index:echo", args, { async: false });

// When invoking synchronously: Ensure the properties come back synchronously and are present on the
// result.
for (const key in args) {
    assert.deepEqual(result1[key], args[key]);
}

// When invoking synchronously: Ensure the properties are available asynchronously through normal
// Promise semantics.
result1.then(v => {
    assert.deepEqual(v, args);
});

>>>>>>> asyncDefault
let result2 = pulumi.runtime.invoke("invoke:index:echo", args);

// When invoking asynchronously: Ensure the properties are *not* present on the result.
for (const key in args) {
    assert.notDeepEqual(result2[key], args[key]);
}

// When invoking asynchronously: Ensure the properties are available asynchronously through normal
// Promise semantics.
result2.then(v => {
    assert.deepEqual(v, args);
});
