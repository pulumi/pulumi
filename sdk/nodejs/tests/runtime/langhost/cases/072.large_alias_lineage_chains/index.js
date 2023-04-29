// This test creates two resources - the first a parent of the second, both of which have many aliases

let pulumi = require("../../../../..");
let assert = require("assert");

class MyResource extends pulumi.CustomResource {
    constructor(name, aliases, parent) {
        super(
            "test:index:MyResource",
            name,
            {},
            {
                aliases,
                parent,
            },
        );
    }
}

class MyOtherResource extends pulumi.CustomResource {
    constructor(name, aliases, parent) {
        super(
            "test:index:MyOtherResource",
            name,
            {},
            {
                aliases,
                parent,
            },
        );
    }
}

const resource1Aliases = Array.from(new Array(1000).keys()).map((key) => `my-alias-name-${key}`);
const resource1 = new MyResource("testResource1", resource1Aliases);
resource1.__aliases.map((alias) => alias.apply((aliasName) => assert(resource1Aliases.includes(aliasName))));
assert.equal(resource1.__aliases.length, 1000);

const resource2Aliases = Array.from(new Array(1000).keys()).map((key) => `my-other-alias-name-${key}`);
const resource2 = new MyResource("testResource2", resource2Aliases, resource1);
resource2.__aliases.map((alias) => alias.apply((aliasName) => assert(resource2Aliases.includes(aliasName))));
assert.equal(resource2.__aliases.length, 1000);

const resource3Aliases = Array.from(new Array(1000).keys()).map((key) => {
    return {
        name: `my-alias-${key}`,
        stack: "my-stack",
        project: "my-project",
        type: "test:index:MyOtherResource",
    };
});
const resource3 = new MyOtherResource("testResource2", resource3Aliases, resource2);
assert.equal(resource3.__aliases.length, 1000);
// We want to ensure that the parent's type is included in computed aliases from the engine
resource3.__aliases[0].apply((aliasName) => assert(aliasName.includes("test:index:MyResource")));
