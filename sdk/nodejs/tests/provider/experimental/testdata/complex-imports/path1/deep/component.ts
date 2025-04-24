import * as pulumi from "@pulumi/pulumi";

export interface DeepComponentArgs {
    level: number;
    name?: string;
}

export class DeepComponent extends pulumi.ComponentResource {
    public readonly path: pulumi.Output<string>;

    constructor(name: string, args: DeepComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("test:component:DeepComponent", name, {}, opts);

        this.path = pulumi.output(`level-${args.level}/${args.name || "default"}`);

        this.registerOutputs({
            path: this.path,
        });
    }
}
