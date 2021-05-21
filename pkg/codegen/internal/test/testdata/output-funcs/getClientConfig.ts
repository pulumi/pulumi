import * as pulumi from "@pulumi/pulumi";
/**
 * Use this function to access the current configuration of the native Azure provider.
 */
export function getClientConfig(opts?: pulumi.InvokeOptions): Promise<GetClientConfigResult> {
    if (!opts) {
        opts = {}
    }

    if (!opts.version) {
        opts.version = utilities.getVersion();
    }
    return pulumi.runtime.invoke("azure-native:codegentest:getClientConfig", {
    }, opts);
}

/**
 * Configuration values returned by getClientConfig.
 */
export interface GetClientConfigResult {
    /**
     * Azure Client ID (Application Object ID).
     */
    readonly clientId: string;
    /**
     * Azure Object ID of the current user or service principal.
     */
    readonly objectId: string;
    /**
     * Azure Subscription ID
     */
    readonly subscriptionId: string;
    /**
     * Azure Tenant ID
     */
    readonly tenantId: string;
}

export function getClientConfigApply(opts?: pulumi.InvokeOptions): pulumi.Output<GetClientConfigResult> {
    return pulumi.output(getClientConfig(opts))
}
