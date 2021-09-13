import * as utilities from "./utilities";
import * as pulumi from "@pulumi/pulumi";
/**
 * A list of SSIS object metadata.
 * API Version: 2018-06-01.
 */
export function getIntegrationRuntimeObjectMetadatum(args: GetIntegrationRuntimeObjectMetadatumArgs, opts?: pulumi.InvokeOptions): Promise<GetIntegrationRuntimeObjectMetadatumResult> {
    if (!opts) {
        opts = {}
    }

    if (!opts.version) {
        opts.version = utilities.getVersion();
    }
    return pulumi.runtime.invoke("azure-native:codegentest:getIntegrationRuntimeObjectMetadatum", {
        "factoryName": args.factoryName,
        "integrationRuntimeName": args.integrationRuntimeName,
        "metadataPath": args.metadataPath,
        "resourceGroupName": args.resourceGroupName,
    }, opts);
}

export interface GetIntegrationRuntimeObjectMetadatumArgs {
    /**
     * The factory name.
     */
    factoryName: string;
    /**
     * The integration runtime name.
     */
    integrationRuntimeName: string;
    /**
     * Metadata path.
     */
    metadataPath?: string;
    /**
     * The resource group name.
     */
    resourceGroupName: string;
}

/**
 * A list of SSIS object metadata.
 */
export interface GetIntegrationRuntimeObjectMetadatumResult {
    /**
     * The link to the next page of results, if any remaining results exist.
     */
    readonly nextLink?: string;
    /**
     * List of SSIS object metadata.
     */
    readonly value?: pulumi.types.output.codegentest.SsisEnvironmentResponse | pulumi.types.output.codegentest.SsisFolderResponse | pulumi.types.output.codegentest.SsisPackageResponse | pulumi.types.output.codegentest.SsisProjectResponse[];
}

export function getIntegrationRuntimeObjectMetadatumOutput(args: GetIntegrationRuntimeObjectMetadatumOutputArgs, opts?: pulumi.InvokeOptions): pulumi.Output<GetIntegrationRuntimeObjectMetadatumResult> {
    return pulumi.output(args).apply(a => getIntegrationRuntimeObjectMetadatum(a, opts))
}

export interface GetIntegrationRuntimeObjectMetadatumOutputArgs {
    /**
     * The factory name.
     */
    factoryName: string;
    /**
     * The integration runtime name.
     */
    integrationRuntimeName: string;
    /**
     * Metadata path.
     */
    metadataPath?: string;
    /**
     * The resource group name.
     */
    resourceGroupName: string;
}
