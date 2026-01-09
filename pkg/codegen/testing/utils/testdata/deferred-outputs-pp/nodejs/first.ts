import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

interface FirstArgs {
    passwordLength: pulumi.Input<number>,
}

export class First extends pulumi.ComponentResource {
    public petName: pulumi.Output<string>;
    public password: pulumi.Output<string>;
    constructor(name: string, args: FirstArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:First", name, args, opts);
        const randomPet = new random.RandomPet(`${name}-randomPet`, {}, {
            parent: this,
        });

        const randomPassword = new random.RandomPassword(`${name}-randomPassword`, {length: args.passwordLength}, {
            parent: this,
        });

        this.petName = randomPet.id;
        this.password = randomPassword.result;
        this.registerOutputs({
            petName: randomPet.id,
            password: randomPassword.result,
        });
    }
}
