let assert = require("assert");
let pulumi = require("../../../../../");

class MyCustomResource extends pulumi.CustomResource {
    constructor(name, args, opts) {
        super("test:index:MyCustomResource", name, args, opts);
    }
}

class MyComponentResource extends pulumi.ComponentResource {
    constructor(name, args, opts) {
        super("test:index:MyComponentResource", name, args, opts);
    }
}

//            comp1
//            /   \
//       cust1    cust2

let comp1 = new MyComponentResource("comp1");

// this represents a nonsensical program where a custom resource references the urn
// of a component.  There is no good need for the urn to be used there.  To represent
// a component dependency, 'dependsOn' should be used instead.
//
// This test just documents our behavior here (which is that we deadlock).
let cust1 = new MyCustomResource("cust1", { parentId: comp1.urn }, { parent: comp1 });
let cust2 = new MyCustomResource("cust2", { parentId: comp1.urn }, { parent: comp1 });
