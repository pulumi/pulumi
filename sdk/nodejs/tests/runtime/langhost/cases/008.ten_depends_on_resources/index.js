// This tests the creation of ten propertyless resources.

let fabric = require("../../../../../");

class MyResource extends fabric.Resource {
    constructor(name, deps) {
        super("test:index:MyResource", name, {}, deps);
    }
}

let all = [];
for (let i = 0; i < 10; i++) {
    all.push(new MyResource("testResource" + i, all));
}

