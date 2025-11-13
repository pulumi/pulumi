import * as pulumi from "@pulumi/pulumi";

export interface SimpleComponentArgs {
    message?: pulumi.Input<string>;
}

export class SimpleComponent extends pulumi.ComponentResource {
    public readonly message: pulumi.Output<string>;

    constructor(name: string, args?: SimpleComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("pkg-a:index:SimpleComponent", name, {}, opts);

        this.message = pulumi.output(args?.message || "Hello from pkg-a");

        this.registerOutputs({
            message: this.message,
        });
    }
}
