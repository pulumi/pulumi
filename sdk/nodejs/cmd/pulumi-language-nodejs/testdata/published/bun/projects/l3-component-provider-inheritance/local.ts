import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";

export class Local extends pulumi.ComponentResource {
    public result: pulumi.Output<boolean>;
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:Local", name, {}, opts);
        // No provider options here: the providers map must be inherited from the
        // enclosing local component and flow through the remote component's
        // registration into its construct call.
        const mlc = new component.ComponentForeignChild(`${name}-mlc`, {value: true}, {
            parent: this,
        });

        this.result = mlc.value;
        this.registerOutputs({
            result: mlc.value,
        });
    }
}
