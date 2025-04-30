import * as pulumi from "@pulumi/pulumi";

export interface EchoArgs {
    echo?: pulumi.Input<string>;
}

export class Echo extends pulumi.CustomResource {
    public readonly echo?: pulumi.Output<string>;

    constructor(name: string, args: EchoArgs, opts?: pulumi.CustomResourceOptions) {
        super("testprovider:index:Echo", name, args, opts);
    }
}
