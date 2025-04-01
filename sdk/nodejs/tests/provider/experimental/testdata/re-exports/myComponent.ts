import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
    message: string;
}

export class MyComponent extends pulumi.ComponentResource {
    public readonly output: pulumi.Output<string>;

    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("test:index:MyComponent", name, {}, opts);
        this.output = pulumi.output(args.message).apply((m) => `Hello ${m}!`);
    }
}
