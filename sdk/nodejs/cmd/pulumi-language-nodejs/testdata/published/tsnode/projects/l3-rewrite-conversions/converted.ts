import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";

interface ConvertedArgs {
    boolean: pulumi.Input<boolean>,
    float: pulumi.Input<number>,
    integer: pulumi.Input<number>,
    string: pulumi.Input<string>,
    numberArray: pulumi.Input<number[]>,
    booleanMap: pulumi.Input<Record<string, pulumi.Input<boolean>>>,
}

export class Converted extends pulumi.ComponentResource {
    constructor(name: string, args: ConvertedArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:Converted", name, args, opts);
        const res = new primitive.Resource(`${name}-res`, {
            boolean: args.boolean,
            float: args.float,
            integer: args.integer,
            string: args.string,
            numberArray: args.numberArray,
            booleanMap: args.booleanMap,
        }, {
            parent: this,
        });

        this.registerOutputs();
    }
}
