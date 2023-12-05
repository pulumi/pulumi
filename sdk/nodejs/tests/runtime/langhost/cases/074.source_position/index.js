let assert = require("assert");
let pulumi = require("../../../../../");

class MyCustomResource extends pulumi.CustomResource {
    constructor(name, opts) {
        super("test:index:MyCustomResource", name, {}, opts);
    }
}

class MyComponentResource extends pulumi.ComponentResource {
    constructor(name, opts) {
        super("test:index:MyComponentResource", name, {}, opts);
    }
}

const custom = new MyCustomResource("custom");
const component = new MyComponentResource("component");
