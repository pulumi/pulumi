import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";

interface PrimitiveComponentArgs {
    boolean: pulumi.Input<boolean>,
    float: pulumi.Input<number>,
    integer: pulumi.Input<number>,
    string: pulumi.Input<string>,
}

export class PrimitiveComponent extends pulumi.ComponentResource {
    constructor(name: string, args: PrimitiveComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:PrimitiveComponent", name, args, opts);
        const res = new primitive.Resource(`${name}-res`, {
            boolean: args.boolean,
            float: args.float,
            integer: args.integer,
            string: args.string,
            numberArray: [
                -1,
                0,
                1,
            ],
            booleanMap: {
                t: true,
                f: false,
            },
        }, {
            parent: this,
        });

        this.registerOutputs();
    }
}
