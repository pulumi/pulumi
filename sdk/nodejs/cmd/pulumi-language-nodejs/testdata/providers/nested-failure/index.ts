import * as pulumi from "@pulumi/pulumi";

interface WrapperArgs {}

export class Wrapper extends pulumi.ComponentResource {
    public readonly innerUrn: pulumi.Output<pulumi.URN>;

    constructor(name: string, args: WrapperArgs, opts?: pulumi.ComponentResourceOptions) {
        super("nested-failure:index:Wrapper", name, args, opts);

        const inner = new pulumi.ComponentResource("failingcomponent:index:Component", `${name}-inner`, {}, {
            parent: this,
            version: "59.0.0",
        }, true);
        this.innerUrn = inner.urn;
        this.registerOutputs({ innerUrn: this.innerUrn });
    }
}
