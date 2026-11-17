import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

interface InnerComponentArgs {
    /**
     * An input passed to the inner component
     */
    input: pulumi.Input<boolean>,
}

export class InnerComponent extends pulumi.ComponentResource {
    public output: pulumi.Output<boolean>;
    constructor(name: string, args: InnerComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:InnerComponent", name, args, opts);
        const res = new simple.Resource(`${name}-res`, {value: !args.input}, {
            parent: this,
        });

        this.output = res.value;
        this.registerOutputs({
            output: res.value,
        });
    }
}
