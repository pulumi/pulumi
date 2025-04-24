// Test the dependsOn invoke option with components

const assert = require("assert");
const pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name, opts) {
        super("test:index:MyResource", name, {}, opts);
    }
}

const dep = new MyResource("dep");

const argWithResourceDependency = new pulumi.Output(
    new Set([dep]),
    Promise.resolve(0),
    Promise.resolve(true), // isKnown
    Promise.resolve(false), // isSecret
);

pulumi.runtime.invokeOutput("test:index:echo", { argWithResourceDependency });
