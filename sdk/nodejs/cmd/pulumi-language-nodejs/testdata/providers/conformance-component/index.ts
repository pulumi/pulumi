import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

export interface SimpleArgs {
    value: pulumi.Input<boolean>;
}

export class Simple extends pulumi.ComponentResource {
    public readonly value: pulumi.Output<boolean>;

    constructor(name: string, args: SimpleArgs, opts?: pulumi.ComponentResourceOptions) {
        super("conformance-component:index:Simple", name, args, opts);

        this.value = pulumi.output(args.value);

        let res = new simple.Resource(`${name}-child`, {
            value: !this.value,
        }, { parent: this });

        this.registerOutputs({
            value: this.value,
        });
    }
}
