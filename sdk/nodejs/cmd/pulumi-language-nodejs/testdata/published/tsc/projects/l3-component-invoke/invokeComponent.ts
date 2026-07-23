import * as pulumi from "@pulumi/pulumi";
import * as config from "@pulumi/config";
import * as multi_argument_invoke from "@pulumi/multi-argument-invoke";

export class InvokeComponent extends pulumi.ComponentResource {
    public result: pulumi.Output<string>;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:InvokeComponent", name, {}, opts);
        // A multi-argument invoke passes its arguments positionally and omits the ones the program leaves
        // out, so parenting it must not displace the options bag into an argument slot.
        const greeting = multi_argument_invoke.multiArgumentInvokeOutput("hello", undefined, {
            parent: this,
        });

        const providerConfig = config.getConfigOutput({
            text: greeting.result,
        }, {
            parent: this,
        });

        this.result = providerConfig.text;
        this.registerOutputs({
            result: providerConfig.text,
        });
    }
}
