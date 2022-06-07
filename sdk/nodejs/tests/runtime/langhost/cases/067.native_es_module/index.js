// This tests the basic creation of a single propertyless resource using a native ES module

import pulumi from "../../../../../index.js";

class MyResource extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:MyResource", name);
    }
}

new MyResource("testResource1");

