// This tests the creation of ten propertyless resources.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:MyResource", name);
    }
}

for (let i = 0; i < 10; i++) {
    new MyResource("testResource" + i);
}
