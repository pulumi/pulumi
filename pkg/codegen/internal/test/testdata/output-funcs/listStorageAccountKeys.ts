import * as pulumi from "@pulumi/pulumi";
/**
 * The response from the ListKeys operation.
 */
export function listStorageAccountKeys(args: ListStorageAccountKeysArgs, opts?: pulumi.InvokeOptions): Promise<ListStorageAccountKeysResult> {
    if (!opts) {
        opts = {}
    }

    if (!opts.version) {
        opts.version = utilities.getVersion();
    }
    return pulumi.runtime.invoke("azure-native:codegentest:listStorageAccountKeys", {
        "accountName": args.accountName,
        "expand": args.expand,
        "resourceGroupName": args.resourceGroupName,
    }, opts);
}

export interface ListStorageAccountKeysArgs {
    /**
     * The name of the storage account within the specified resource group. Storage account names must be between 3 and 24 characters in length and use numbers and lower-case letters only.
     */
    accountName: string;
    /**
     * Specifies type of the key to be listed. Possible value is kerb.
     */
    expand?: string;
    /**
     * The name of the resource group within the user's subscription. The name is case insensitive.
     */
    resourceGroupName: string;
}

/**
 * The response from the ListKeys operation.
 */
export interface ListStorageAccountKeysResult {
    /**
     * Gets the list of storage account keys and their properties for the specified storage account.
     */
    readonly keys: StorageAccountKeyResponse[];
}

export function listStorageAccountKeysApply(args: ListStorageAccountKeysApplyArgs, opts?: pulumi.InvokeOptions): pulumi.Output<ListStorageAccountKeysResult> {
    return pulumi.output(args).apply(a => listStorageAccountKeys(a, opts))
}

export interface ListStorageAccountKeysApplyArgs {
    /**
     * The name of the storage account within the specified resource group. Storage account names must be between 3 and 24 characters in length and use numbers and lower-case letters only.
     */
    accountName: pulumi.Input<string>;
    /**
     * Specifies type of the key to be listed. Possible value is kerb.
     */
    expand?: pulumi.Input<string>;
    /**
     * The name of the resource group within the user's subscription. The name is case insensitive.
     */
    resourceGroupName: pulumi.Input<string>;
}
