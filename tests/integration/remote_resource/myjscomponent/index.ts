import * as pulumi from "@pulumi/pulumi";

class MyComponent extends pulumi.ComponentResource {
    constructor(name, opts?) {
        super("my:module:Resource", name, {}, opts);
    }
}

const comp = new MyComponent("foo");
export const compUrn = comp.urn;
