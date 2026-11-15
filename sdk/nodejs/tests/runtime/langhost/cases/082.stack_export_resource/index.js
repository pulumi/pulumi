// This test exports a whole resource as a stack output.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:MyResource", name, { foo: "bar" });
    }
}

module.exports.res = new MyResource("testResource");
