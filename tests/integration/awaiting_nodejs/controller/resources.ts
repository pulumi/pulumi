import * as pulumi from "@pulumi/pulumi";

export class Awaiting extends pulumi.CustomResource {
    public readonly release!: pulumi.Output<any>;

    constructor(name: string, release: any, ready = true) {
        super("testprovider:index:Awaiting", name, { ready, version: "v1", release });
    }
}

export class RequiresKnown extends pulumi.CustomResource {
    constructor(name: string, source: pulumi.Input<string>) {
        super("testprovider:index:RequiresKnown", name, { source, outputOnly: "member" });
    }
}
