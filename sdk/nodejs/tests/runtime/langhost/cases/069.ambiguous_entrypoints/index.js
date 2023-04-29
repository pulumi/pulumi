// This tests the basic creation of a single propertyless resource.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:MyResource", name);
    }
}

new MyResource("testResource1");
