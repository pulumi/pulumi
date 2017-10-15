// This tests the creation of ten propertyless resources.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name, deps) {
        super("test:index:MyResource", name, {}, deps);
    }
}

let all = [];
for (let i = 0; i < 10; i++) {
    all.push(new MyResource("testResource" + i, all));
}

