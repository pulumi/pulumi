import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

interface SubmoduleArgs {
    name: pulumi.Input<string>,
}

export class Submodule extends pulumi.ComponentResource {
    public someOutput: pulumi.Output<string>;
    constructor(name: string, args: SubmoduleArgs, opts?: pulumi.ComponentResourceOptions) {
        super("components:index:Submodule", name, args, opts);
        const path = "fake/path";

        const forceDestroy = true;

        const pgpKey = "fakekey";

        const passwordLength = 16;

        const passwordResetRequired = true;

        const iamAccessKeyStatus = "Active";

        const _this: aws.iam.User[] = [];
        for (const range = {value: 0}; range.value < 1; range.value++) {
            _this.push(new aws.iam.User(`${name}-this-${range.value}`, {
                name: args.name,
                path: path,
                forceDestroy: forceDestroy,
            }, {
            parent: this,
        }));
        }

        const thisUserLoginProfile: aws.iam.UserLoginProfile[] = [];
        for (const range = {value: 0}; range.value < 1; range.value++) {
            thisUserLoginProfile.push(new aws.iam.UserLoginProfile(`${name}-this-${range.value}`, {
                user: _this[0].name,
                pgpKey: pgpKey,
                passwordLength: passwordLength,
                passwordResetRequired: passwordResetRequired,
            }, {
            parent: this,
        }));
        }

        const thisAccessKey: aws.iam.AccessKey[] = [];
        for (const range = {value: 0}; range.value < 1; range.value++) {
            thisAccessKey.push(new aws.iam.AccessKey(`${name}-this-${range.value}`, {
                user: _this[0].name,
                pgpKey: pgpKey,
                status: iamAccessKeyStatus,
            }, {
            parent: this,
        }));
        }

        this.someOutput = thisAccessKey[0].id;
        this.registerOutputs({
            someOutput: thisAccessKey[0].id,
        });
    }
}
