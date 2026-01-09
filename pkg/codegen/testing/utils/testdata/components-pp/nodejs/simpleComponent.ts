import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

export class SimpleComponent extends pulumi.ComponentResource {
    constructor(name: string, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:SimpleComponent", name, {}, opts);
        const firstPassword = new random.RandomPassword(`${name}-firstPassword`, {
            length: 16,
            special: true,
        }, {
            parent: this,
        });

        const secondPassword = new random.RandomPassword(`${name}-secondPassword`, {
            length: 16,
            special: true,
        }, {
            parent: this,
        });

        this.registerOutputs();
    }
}
