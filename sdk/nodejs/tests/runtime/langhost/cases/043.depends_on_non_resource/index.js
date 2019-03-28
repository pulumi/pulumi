// This tests the creation of ten propertyless resources.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name, deps) {
        super("test:index:MyResource", name, {}, deps);
    }
}

new MyResource("testResource", { dependsOn: pulumi.output(Promise.resolve([Promise.resolve(1)])) });
