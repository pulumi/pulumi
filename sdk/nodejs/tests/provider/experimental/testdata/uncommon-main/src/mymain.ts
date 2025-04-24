import * as pulumi from "@pulumi/pulumi";

export interface MainComponentArgs {
    message: string;
}

export class MainComponent extends pulumi.ComponentResource {
    public readonly formattedMessage: pulumi.Output<string>;

    constructor(name: string, args: MainComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("pkg:index:MainComponent", name, {}, opts);

        this.formattedMessage = pulumi.output(args.message).apply((msg) => `Formatted: ${msg}`);

        this.registerOutputs({
            formattedMessage: this.formattedMessage,
        });
    }
}
