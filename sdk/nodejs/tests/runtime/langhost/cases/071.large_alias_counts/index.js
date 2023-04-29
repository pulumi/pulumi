// This test creates a resource with many aliases

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
                parent,
            },
        );
    }
}

const resource1Aliases = Array.from(new Array(10000).keys()).map((key) => `my-alias-name-${key}`);
const resource1 = new MyResource("testResource1", resource1Aliases);
resource1.__aliases.map((alias) => alias.apply((aliasName) => assert(resource1Aliases.includes(aliasName))));
