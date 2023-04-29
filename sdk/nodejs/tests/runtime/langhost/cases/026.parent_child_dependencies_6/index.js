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
//        cust1   comp2
//                /   \
//            cust2    cust3

let comp1 = new MyComponentResource("comp1");
let cust1 = new MyCustomResource("cust1", {}, { parent: comp1 });

let comp2 = new MyComponentResource("comp2", {}, { parent: comp1 });
let cust2 = new MyCustomResource("cust2", {}, { parent: comp2 });
let cust3 = new MyCustomResource("cust3", {}, { parent: comp2 });

let res1 = new MyCustomResource("res1", {}, { dependsOn: comp1 });
