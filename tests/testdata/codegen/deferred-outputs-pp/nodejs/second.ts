import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

interface SecondArgs {
    petName: pulumi.Input<string>,
}

export class Second extends pulumi.ComponentResource {
    public passwordLength: pulumi.Output<number>;
    constructor(name: string, args: SecondArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:Second", name, args, opts);
        const randomPet = new random.RandomPet(`${name}-randomPet`, {length: args.petName.length}, {
            parent: this,
        });

        const password = new random.RandomPassword(`${name}-password`, {
            length: 16,
            special: true,
            numeric: false,
        }, {
            parent: this,
        });

        this.passwordLength = password.length;
        this.registerOutputs({
            passwordLength: password.length,
        });
    }
}
