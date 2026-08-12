import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";

interface PrimitiveComponentArgs {
    numberArray: pulumi.Input<number[]>,
    booleanMap: pulumi.Input<Record<string, pulumi.Input<boolean>>>,
}

export class PrimitiveComponent extends pulumi.ComponentResource {
    constructor(name: string, args: PrimitiveComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:PrimitiveComponent", name, args, opts);
        const res = new primitive.Resource(`${name}-res`, {
            boolean: true,
            float: 3.5,
            integer: 3,
            string: "plain",
            numberArray: args.numberArray,
            booleanMap: args.booleanMap,
        }, {
            parent: this,
        });

        this.registerOutputs();
    }
}
