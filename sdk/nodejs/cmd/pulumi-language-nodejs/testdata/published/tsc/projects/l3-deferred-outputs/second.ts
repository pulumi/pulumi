import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

interface SecondArgs {
    input: pulumi.Input<boolean>,
}

export class Second extends pulumi.ComponentResource {
    public untainted: pulumi.Output<boolean>;
    public tainted: pulumi.Output<boolean>;
    constructor(name: string, args: SecondArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:Second", name, args, opts);
        const second_untainted = new simple.Resource(`${name}-second-untainted`, {value: true}, {
            parent: this,
        });

        const second_tainted = new simple.Resource(`${name}-second-tainted`, {value: !args.input}, {
            parent: this,
        });

        this.untainted = second_untainted.value;
        this.tainted = second_tainted.value;
        this.registerOutputs({
            untainted: second_untainted.value,
            tainted: second_tainted.value,
        });
    }
}
