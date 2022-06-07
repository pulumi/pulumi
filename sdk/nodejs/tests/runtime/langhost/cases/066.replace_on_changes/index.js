// This tests the replaceOnChanges ResourceOption.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name, opts) {
        super("test:index:MyResource", name, {}, opts);
    }
}

new MyResource("testResource", { replaceOnChanges: ["foo"] });
