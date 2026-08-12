import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

interface MyComponentArgs {
    /**
     * A simple input
     */
    input: pulumi.Input<boolean>,
}

export class MyComponent extends pulumi.ComponentResource {
    public output: pulumi.Output<boolean>;
    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:MyComponent", name, args, opts);
        const res = new simple.Resource(`${name}-res`, {value: args.input}, {
            parent: this,
        });

        this.output = res.value;
        this.registerOutputs({
            output: res.value,
        });
    }
}
