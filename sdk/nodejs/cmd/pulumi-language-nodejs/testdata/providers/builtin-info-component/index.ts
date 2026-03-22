import * as pulumi from "@pulumi/pulumi";

export interface BuiltinInfoArgs {}

export class BuiltinInfo extends pulumi.ComponentResource {
    public readonly context: pulumi.Output<string>;

    constructor(name: string, args: BuiltinInfoArgs, opts?: pulumi.ComponentResourceOptions) {
        super("builtin-info-component:index:BuiltinInfo", name, args, opts);

        this.context = pulumi.output(
            `${pulumi.getOrganization()}-${pulumi.getProject()}-${pulumi.getStack()}`,
        );

        this.registerOutputs({
            context: this.context,
        });
    }
}
