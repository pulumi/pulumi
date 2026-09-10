import * as pulumi from "@pulumi/pulumi";
import * as primitive from "@pulumi/primitive";

interface MyComponentArgs {
    booleanMap: pulumi.Input<Record<string, pulumi.Input<boolean>>>,
}

export class MyComponent extends pulumi.ComponentResource {
    public booleanMap: pulumi.Output<Record<string, pulumi.Input<boolean>>>;
    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:MyComponent", name, args, opts);
        const res = new primitive.Resource(`${name}-res`, {
            boolean: false,
            float: 2.17,
            integer: -12,
            string: "adversarial",
            numberArray: [
                0,
                1,
            ],
            booleanMap: args.booleanMap,
        }, {
            parent: this,
        });

        this.booleanMap = res.booleanMap;
        this.registerOutputs({
            booleanMap: res.booleanMap,
        });
    }
}
