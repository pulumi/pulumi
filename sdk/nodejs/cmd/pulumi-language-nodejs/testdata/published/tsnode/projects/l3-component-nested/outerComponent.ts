import * as pulumi from "@pulumi/pulumi";
import { InnerComponent } from "./innerComponent";

interface OuterComponentArgs {
    /**
     * An input passed to the outer component
     */
    input: pulumi.Input<boolean>,
}

export class OuterComponent extends pulumi.ComponentResource {
    public output: pulumi.Output<boolean>;
    constructor(name: string, args: OuterComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:OuterComponent", name, args, opts);
        const innerComponent = new InnerComponent(`${name}-innerComponent`, {input: !args.input}, {
            parent: this,
        });

        this.output = innerComponent.output;
        this.registerOutputs({
            output: innerComponent.output,
        });
    }
}
