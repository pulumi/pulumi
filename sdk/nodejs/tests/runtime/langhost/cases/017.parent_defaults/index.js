// This tests the creation of ten resources that contains "simple" input and output propeprties.
// In particular, there aren't any fancy dataflow linked properties.

let assert = require("assert");

let pulumi = require("../../../../../");

class Provider extends pulumi.ProviderResource {
	constructor(name, opts) {
		super("test", name, {}, opts);
	}
}

class Resource extends pulumi.CustomResource {
	constructor(name, props, opts) {
		super("test:index:Resource", name, props, opts)
	}
}

class Component extends pulumi.ComponentResource {
	constructor(name, createChildren, opts) {
		super("test:index:Component", name, {}, opts);

		createChildren(name, this);
	}
}

function createResources(name, parent) {
	// Use all parent defaults
	new Resource(`${name}/r0`, {}, { parent: parent });

	// Override protect
	new Resource(`${name}/r1`, {}, { parent: parent, protect: false });
	new Resource(`${name}/r2`, {}, { parent: parent, protect: true });

	// Override provider
	new Resource(`${name}/r3`, {}, { parent: parent, provider: new Provider(`${name}-p`, { parent: parent }) });
}

function createComponents(name, createChildren, parent) {
	// Use all parent defaults
	new Component(`${name}/c0`, createChildren, { parent: parent });

	// Override protect.
	new Component(`${name}/c1`, createChildren, { parent: parent, protect: false });
	new Component(`${name}/c2`, createChildren, { parent: parent, protect: true });

	// Override providers.
	new Component(`${name}/c3`, createChildren, { parent: parent, providers: {} });
	new Component(`${name}/c4`, createChildren, {
		parent: parent,
		providers: { "test": new Provider(`${name}-p`, { parent: parent }) },
	});
}

// Create default (unparented) resources
createResources("unparented");

// Create singly-nested resources
createComponents("single-nest", createResources);

// Create doubly-nested resources
createComponents("double-nest", (name, parent) => {
	createComponents(name, createResources, parent);
});
