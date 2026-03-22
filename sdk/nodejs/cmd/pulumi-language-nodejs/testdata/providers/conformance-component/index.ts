import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

export interface SimpleArgs {
    value: pulumi.Input<boolean>;
}

export class Simple extends pulumi.ComponentResource {
    public readonly value: pulumi.Output<boolean>;
    public readonly context: pulumi.Output<string>;

    constructor(name: string, args: SimpleArgs, opts?: pulumi.ComponentResourceOptions) {
        super("conformance-component:index:Simple", name, args, opts);

        this.value = pulumi.output(args.value);
        this.context = pulumi.output(
            `${pulumi.getOrganization()}-${pulumi.getProject()}-${pulumi.getStack()}`,
        );

        let res = new simple.Resource(`${name}-child`, {
            value: !this.value,
        }, { parent: this });

        this.registerOutputs({
            context: this.context,
            value: this.value,
        });
    }
}
