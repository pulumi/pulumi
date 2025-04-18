// *** WARNING: this file was generated by pulumi-language-nodejs. ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

import * as pulumi from "@pulumi/pulumi";
import * as utilities from "./utilities";

/**
 * The `call` package's provider resource
 */
export class Provider extends pulumi.ProviderResource {
    /** @internal */
    public static readonly __pulumiType = 'call';

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

    public readonly value!: pulumi.Output<string>;

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
            if ((!args || args.value === undefined) && !opts.urn) {
                throw new Error("Missing required property 'value'");
            }
            resourceInputs["value"] = args ? args.value : undefined;
        }
        opts = pulumi.mergeOptions(utilities.resourceOptsDefaults(), opts);
        super(Provider.__pulumiType, name, resourceInputs, opts);
    }

    /**
     * The `identity` method of the `call` package's provider. Returns the provider's `value` configuration unaltered.
     */
    identity(): pulumi.Output<Provider.IdentityResult> {
        return pulumi.runtime.call("pulumi:providers:call/identity", {
            "__self__": this,
        }, this);
    }

    /**
     * The `prefixed` method of the `call` package's provider. Accepts a string and returns the provider's `value` configuration prefixed with that string.
     */
    prefixed(args: Provider.PrefixedArgs): pulumi.Output<Provider.PrefixedResult> {
        return pulumi.runtime.call("pulumi:providers:call/prefixed", {
            "__self__": this,
            "prefix": args.prefix,
        }, this);
    }
}

/**
 * The set of arguments for constructing a Provider resource.
 */
export interface ProviderArgs {
    value: pulumi.Input<string>;
}

export namespace Provider {
    /**
     * The results of the Provider.identity method.
     */
    export interface IdentityResult {
        readonly result: string;
    }

    /**
     * The set of arguments for the Provider.prefixed method.
     */
    export interface PrefixedArgs {
        prefix: pulumi.Input<string>;
    }

    /**
     * The results of the Provider.prefixed method.
     */
    export interface PrefixedResult {
        readonly result: string;
    }

}
