import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";
import { SimpleComponent } from "./simpleComponent";

interface ExampleComponentArgs {
    /**
     * A simple input
     */
    input: pulumi.Input<string>,
    /**
     * The main CIDR blocks for the VPC
     */
    cidrBlocks: pulumi.Input<Record<string, pulumi.Input<string>>>,
    ipAddress: pulumi.Input<number[]>,
}

export class ExampleComponent extends pulumi.ComponentResource {
    public result: pulumi.Output<string>;
    constructor(name: string, args: ExampleComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:ExampleComponent", name, args, opts);
        const password = new random.RandomPassword(`${name}-password`, {
            length: 16,
            special: true,
            overrideSpecial: args.input,
        }, {
            parent: this,
        });

        const simpleComponent = new SimpleComponent(`${name}-simpleComponent`, {
            parent: this,
        });

        this.result = password.result;
        this.registerOutputs({
            result: password.result,
        });
    }
}
