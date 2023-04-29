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
    constructor(name, createChildren, opts) {
        super("test:index:Resource", name, {}, opts);

        if (createChildren) {
            createChildren(name, this);
        }
    }
}

class Component extends pulumi.ComponentResource {
    constructor(name, createChildren, opts) {
        super("test:index:Component", name, {}, opts);

        createChildren(name, this);
    }
}

function createResources(name, createChildren, parent) {
    // Use all parent defaults
    new Resource(`${name}/r0`, createChildren, { parent: parent });

    // Override protect
    new Resource(`${name}/r1`, createChildren, { parent: parent, protect: false });
    new Resource(`${name}/r2`, createChildren, { parent: parent, protect: true });

    // Override provider
    const provider = new Provider(`${name}-p`, { parent });
    const r3 = new Resource(`${name}/r3`, createChildren, { parent, provider });
    assert.strictEqual(r3.getProvider("test:index:Resource"), provider);
}

function createComponents(name, createChildren, parent) {
    // Use all parent defaults
    new Component(`${name}/c0`, createChildren, { parent: parent });

    // Override protect.
    new Component(`${name}/c1`, createChildren, { parent: parent, protect: false });
    new Component(`${name}/c2`, createChildren, { parent: parent, protect: true });

    // Override providers.
    new Component(`${name}/c3`, createChildren, {
        parent: parent,
        providers: { test: new Provider(`${name}-p`, { parent: parent }) },
    });
}

// Create default (unparented) resources
createResources("unparented");

// Create singly-nested resources
createComponents("single-nest", (name, parent) => {
    createResources(name, undefined, parent);
});

// Create doubly-nested resources
createComponents("double-nest", (name, parent) => {
    createComponents(
        name,
        (name, parent) => {
            createResources(name, undefined, parent);
        },
        parent,
    );
});

// Create doubly-nested resources parented to other resources
createComponents("double-nest-2", (name, parent) => {
    createResources(
        name,
        (name, parent) => {
            createResources(name, undefined, parent);
        },
        parent,
    );
});
