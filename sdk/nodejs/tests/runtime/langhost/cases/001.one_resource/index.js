// This tests the basic creation of a single propertyless resource.

let fabric = require("../../../../../");

class MyResource extends fabric.Resource {
    constructor(name) {
        super("test:index:MyResource", name);
    }
}

new MyResource("testResource1");

