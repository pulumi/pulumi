// *** WARNING: this file was generated by test. ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

import * as pulumi from "@pulumi/pulumi";
import * as inputs from "./types/input";
import * as outputs from "./types/output";
import * as enums from "./types/enums";
import * as utilities from "./utilities";

export class Provider extends pulumi.ProviderResource {
    /** @internal */
    public static readonly __pulumiType = 'credentials';

    /**
     * Returns true if the given object is an instance of Provider.  This is designed to work even
     * when multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is Provider {
        if (obj === undefined || obj === null) {
            return false;
        }
        return obj['__pulumiType'] === "pulumi:providers:" + Provider.__pulumiType;
    }

    /**
     * The password. It is very secret.
     */
    public readonly password!: pulumi.Output<string | undefined>;
    /**
     * The username. Its important but not secret.
     */
    public readonly user!: pulumi.Output<string>;

    /**
     * Create a Provider resource with the given unique name, arguments, and options.
     *
     * @param name The _unique_ name of the resource.
     * @param args The arguments to use to populate this resource's properties.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(name: string, args: ProviderArgs, opts?: pulumi.ResourceOptions) {
        let resourceInputs: pulumi.Inputs = {};
        opts = opts || {};
        {
            if ((!args || args.hash === undefined) && !opts.urn) {
                throw new Error("Missing required property 'hash'");
            }
            if ((!args || args.shared === undefined) && !opts.urn) {
                throw new Error("Missing required property 'shared'");
            }
            if ((!args || args.user === undefined) && !opts.urn) {
                throw new Error("Missing required property 'user'");
            }
            resourceInputs["hash"] = args ? args.hash : undefined;
            resourceInputs["password"] = (args?.password ? pulumi.secret(args.password) : undefined) ?? (utilities.getEnv("FOO") || "");
            resourceInputs["shared"] = pulumi.output(args ? args.shared : undefined).apply(JSON.stringify);
            resourceInputs["user"] = args ? args.user : undefined;
        }
        opts = pulumi.mergeOptions(utilities.resourceOptsDefaults(), opts);
        const secretOpts = { additionalSecretOutputs: ["password"] };
        opts = pulumi.mergeOptions(opts, secretOpts);
        super(Provider.__pulumiType, name, resourceInputs, opts);
    }
}

/**
 * The set of arguments for constructing a Provider resource.
 */
export interface ProviderArgs {
    /**
     * The (entirely uncryptographic) hash function used to encode the "password".
     */
    hash: pulumi.Input<enums.HashKind>;
    /**
     * The password. It is very secret.
     */
    password?: pulumi.Input<string>;
    shared: pulumi.Input<inputs.SharedArgs>;
    /**
     * The username. Its important but not secret.
     */
    user: pulumi.Input<string>;
}
