import * as pulumi from "@pulumi/pulumi";

export class Awaiting extends pulumi.CustomResource {
    public readonly release!: pulumi.Output<any>;
    public readonly outputOnly!: pulumi.Output<string>;

    constructor(name: string, args: { ready: boolean; version: string; release: any }, opts?: pulumi.CustomResourceOptions) {
        super("testprovider:index:Awaiting", name, args, opts);
    }
}

export class RequiresKnown extends pulumi.CustomResource {
    public readonly source!: pulumi.Output<string>;
    public readonly outputOnly!: pulumi.Output<string>;

    constructor(name: string, args: { source: pulumi.Input<string>; outputOnly: pulumi.Input<string> }, opts?: pulumi.CustomResourceOptions) {
        super("testprovider:index:RequiresKnown", name, args, opts);
    }
}
