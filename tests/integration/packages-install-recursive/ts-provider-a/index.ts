import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

export interface ComponentArgs {
    echo: pulumi.Input<any>;
}

export class Component extends pulumi.ComponentResource {
    public readonly childId: pulumi.Output<string>;
    public readonly echo: pulumi.Output<any>;

    constructor(name: string, args: ComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("typescript-a:index:Component", name, args, opts);

        this.echo = pulumi.output(args.echo);

        // Create a child Echo resource
        const child = new random.RandomString(`${name}-child`, { length: 7 }, { parent: this });

        this.childId = child.id;

        this.registerOutputs({
            childId: this.childId,
            echo: this.echo,
        });
    }
}
