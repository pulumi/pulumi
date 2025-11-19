import * as pulumi from "@pulumi/pulumi";

export interface ComponentArgs {
    echo: pulumi.Input<string>;
}

export class Component extends pulumi.ComponentResource {
    public readonly echo: pulumi.Output<string>;

    constructor(name: string, args: ComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("typescript-b:index:Component", name, args, opts);

        this.echo = pulumi.output(args.echo);

        this.registerOutputs({
            echo: this.echo,
        });
    }
}
