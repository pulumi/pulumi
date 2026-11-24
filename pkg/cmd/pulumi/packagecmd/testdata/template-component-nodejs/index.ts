import * as pulumi from "@pulumi/pulumi";

export class MyComponent extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("${PROJECT}:index:MyComponent", name, {}, opts);
    }
}
