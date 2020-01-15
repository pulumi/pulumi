// Test the ability to invoke provider functions via RPC.

let assert = require("assert");
let pulumi = require("../../../../../");
let semver = require("semver");

class Provider extends pulumi.ProviderResource {
	constructor(name, opts) {
		super("test", name, {}, opts);
	}
}

class Resource extends pulumi.CustomResource {
	constructor(name, opts) {
		super("test:index:Resource", name, {}, opts)
	}
}

const provider = new Provider("p");
const parent = new Resource("r", { provider })

let args = {
    a: "hello",
    b: true,
    c: [0.99, 42, { z: "x" }],
    id: "some-id",
    urn: "some-urn",
};

if (semver.lt(process.version, "12.11.0")) {
	// These tests hang on runtimes later than 12.10.x due to their use of deasync.

	let result1 = pulumi.runtime.invoke("test:index:echo", args, { parent });
	for (const key in args) {
		assert.deepEqual(result1[key], args[key]);
	}

	let result2 = pulumi.runtime.invoke("test:index:echo", args, { parent });
	result2.then((v) => {
		assert.deepEqual(v, args);
	});
}

let result3 = pulumi.runtime.invoke("test:index:echo", args, { parent, async: true });
result3.then((v) => {
    assert.deepEqual(v, args);
});
