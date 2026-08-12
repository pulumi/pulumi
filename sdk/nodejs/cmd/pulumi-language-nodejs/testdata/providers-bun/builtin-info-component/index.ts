import * as pulumi from "@pulumi/pulumi";

export interface BuiltinInfoArgs {}

export class BuiltinInfo extends pulumi.ComponentResource {
    public readonly organization: pulumi.Output<string>;
    public readonly project: pulumi.Output<string>;
    public readonly stack: pulumi.Output<string>;

    constructor(name: string, args: BuiltinInfoArgs, opts?: pulumi.ComponentResourceOptions) {
        super("builtin-info-component:index:BuiltinInfo", name, args, opts);

        this.organization = pulumi.output(pulumi.getOrganization());
        this.project = pulumi.output(pulumi.getProject());
        this.stack = pulumi.output(pulumi.getStack());

        this.registerOutputs({
            organization: this.organization,
            project: this.project,
            stack: this.stack,
        });
    }
}
