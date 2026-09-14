import * as pulumi from "@pulumi/pulumi";

export class Awaiting extends pulumi.CustomResource {
    public readonly release!: pulumi.Output<any>;
    public readonly outputOnly!: pulumi.Output<string>;

    constructor(name: string, args: { ready: boolean; version: string; release: any }) {
        super("testprovider:index:Awaiting", name, args);
    }
}

export class RequiresKnown extends pulumi.CustomResource {
    constructor(name: string, args: { source: pulumi.Input<string>; outputOnly: pulumi.Input<string> }) {
        super("testprovider:index:RequiresKnown", name, args);
    }
}
