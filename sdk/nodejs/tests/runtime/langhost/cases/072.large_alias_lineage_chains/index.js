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
                aliases: aliases,
                parent
            }
        );
    }
}

const resource1Aliases = Array.from(new Array(1000).keys()).map(key => `my-alias-name-${key}`);
const resource1 = new MyResource("testResource1", resource1Aliases);
resource1.__aliases.map(alias => alias.apply(aliasName => assert(resource1Aliases.includes(aliasName))));

const resource2Aliases = Array.from(new Array(1000).keys()).map(key => `my-other-alias-name-${key}`);
const resource2 = new MyResource("testResource2", resource2Aliases, resource1)
resource2.__aliases.map(alias => alias.apply(aliasName => assert(resource2Aliases.includes(aliasName))));
