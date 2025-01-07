// Test the dependsOn invoke option with components

const assert = require("assert");
const pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name, opts) {
        super("test:index:MyResource", name, {}, opts);
    }
}

class MyComponent extends pulumi.ComponentResource {
    constructor(name) {
        super("test:index:MyComponent", name);
        const dep = new MyResource("dep", { parent: this });
    }
}

const comp = new MyComponent("comp");
const dependsOn = [comp];

pulumi.runtime.invokeOutput("test:index:echo", {}, { dependsOn });
