// This test checks for unusually named pulumi resources with aliases

let pulumi = require("../../../../..");
let assert = require("assert");

class MyResource extends pulumi.CustomResource {
    constructor(name, aliases) {
        super(
            "test:index:MyResource",
            name,
            {},
            {
                aliases: aliases,
            },
        );
    }
}

const resource1 = new MyResource("testResource1", []);
assert.equal(resource1.__aliases[0], undefined);
const resource2 = new MyResource("some-random-resource-name", ["test-alias-name"]);
resource2.__aliases[0].apply((alias) => assert.equal(alias, ["test-alias-name"]));
const resource3 = new MyResource("other:random:resource:name", ["other:test:alias:name"]);
resource3.__aliases[0].apply((alias) => assert.equal(alias, ["other:test:alias:name"]));
const resource4 = new MyResource("-other@random:resource!name", ["other!test@alias+name"]);
resource4.__aliases[0].apply((alias) => assert.equal(alias, ["other!test@alias+name"]));
