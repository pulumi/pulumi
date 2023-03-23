import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";
import { SimpleComponent } from "./simpleComponent";

interface ExampleComponentArgs {
    /**
     * A simple input
     */
    input: pulumi.Input<string>,
    /**
     * The main CIDR blocks for the VPC
     * It is a map of strings
     */
    cidrBlocks: pulumi.Input<Record<string, pulumi.Input<string>>>,
    /**
     * GitHub app parameters, see your github app. Ensure the key is the base64-encoded `.pem` file (the output of `base64 app.private-key.pem`, not the content of `private-key.pem`).
     */
    githubApp: {
        id?: pulumi.Input<string>,
        keyBase64?: pulumi.Input<string>,
        webhookSecret?: pulumi.Input<string>,
    },
    /**
     * A list of servers
     */
    servers: {
        name?: pulumi.Input<string>,
    }[],
    /**
     * A map between for zones
     */
    deploymentZones: Record<string, {
        zone?: pulumi.Input<string>,
    }>,
    ipAddress: pulumi.Input<number[]>,
}

export class ExampleComponent extends pulumi.ComponentResource {
    public result: pulumi.Output<string>;
    constructor(name: string, args: ExampleComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:ExampleComponent", name, args, opts);
        const password = new random.RandomPassword(`${name}-password`, {
            length: 16,
            special: true,
            overrideSpecial: args.input,
        }, {
            parent: this,
        });

        const githubPassword = new random.RandomPassword(`${name}-githubPassword`, {
            length: 16,
            special: true,
            overrideSpecial: args.githubApp.webhookSecret,
        }, {
            parent: this,
        });

        const simpleComponent = new SimpleComponent(`${name}-simpleComponent`, {
            parent: this,
        });

        this.result = password.result;
        this.registerOutputs({
            result: password.result,
        });
    }
}
