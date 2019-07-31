// This tests the basic creation of a single propertyless resource.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name, opts) {
        super("test:index:MyResource", name, {}, opts);
    }
}

new MyResource("testResource1", { import: "testID" });

