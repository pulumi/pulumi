// This tests the creation of a resource that contains lots of input properties.

let fabric = require("../../../../lib");

class MyResource extends fabric.Resource {
    constructor(name) {
        super("test:index:MyResource", name);
    }
}

new MyResource("testResource1");

