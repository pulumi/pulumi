import * as pulumi from "@pulumi/pulumi";
import * as config from "@pulumi/config";

interface ProviderComponentArgs {
    text: pulumi.Input<string>,
}

export class ProviderComponent extends pulumi.ComponentResource {
    public result: pulumi.Output<string>;
    constructor(name: string, args: ProviderComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:ProviderComponent", name, args, opts);
        const prov = new config.Provider(`${name}-prov`, {name: "my config"}, {
            parent: this,
        });

        const res = new config.Resource(`${name}-res`, {text: args.text}, {
            parent: this,
            provider: prov,
        });

        this.result = res.text;
        this.registerOutputs({
            result: res.text,
        });
    }
}
