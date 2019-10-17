// This tests the creation of ten propertyless resources.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name, opts) {
        super("test:index:MyResource", name, {}, opts);
    }
}

new MyResource("testResource", { ignoreChanges: ["ignoredProperty"] });
