import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

interface FirstArgs {
    input: pulumi.Input<boolean>,
}

export class First extends pulumi.ComponentResource {
    public untainted: pulumi.Output<boolean>;
    public tainted: pulumi.Output<boolean>;
    constructor(name: string, args: FirstArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:First", name, args, opts);
        const first_untainted = new simple.Resource(`${name}-first-untainted`, {value: true}, {
            parent: this,
        });

        const first_tainted = new simple.Resource(`${name}-first-tainted`, {value: !args.input}, {
            parent: this,
        });

        this.untainted = first_untainted.value;
        this.tainted = first_tainted.value;
        this.registerOutputs({
            untainted: first_untainted.value,
            tainted: first_tainted.value,
        });
    }
}
