// Copyright 2017 Pulumi, Inc. All rights reserved.

import {ClientCertificate} from "./clientCertificate";
import {Deployment} from "./deployment";
import {MethodSetting} from "./method";
import {RestAPI} from "./restAPI";
import * as cloudformation from "../cloudformation";

// The Stage resource specifies the AWS Identity and Access Management (IAM) role that Amazon API
// Gateway (API Gateway) uses to write API logs to Amazon CloudWatch Logs (CloudWatch Logs).
export class Stage extends cloudformation.Resource implements StageProperties {
    public readonly restAPI: RestAPI;
    public readonly stageName: string;
    public deployment: Deployment;
    public cacheClusterEnabled?: boolean;
    public cacheClusterSize?: string;
    public clientCertificate?: ClientCertificate;
    public description?: string;
    public methodSettings?: MethodSetting[];
    public variables?: {[key: string]: string};

    constructor(name: string, args: StageProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::Stage",
        });
        this.restAPI = args.restAPI;
        this.stageName = args.stageName;
        this.deployment = args.deployment;
        this.cacheClusterEnabled = args.cacheClusterEnabled;
        this.cacheClusterSize = args.cacheClusterSize;
        this.clientCertificate = args.clientCertificate;
        this.description = args.description;
        this.methodSettings = args.methodSettings;
        this.variables = args.variables;
    }
}

export interface StageProperties {
    // The RestAPI resource that you're deploying with this stage.
    readonly restAPI: RestAPI;
    // The name of the stage, which API Gateway uses as the first path segment in the invoke URI.
    readonly stageName: string;
    // The deployment that the stage points to.
    deployment: Deployment;
    // Indicates whether cache clustering is enabled for the stage.
    cacheClusterEnabled?: boolean;
    // The stage's cache cluster size.
    cacheClusterSize?: string;
    // The identifier of the client certificate that API Gateway uses to call your integration endpoints in the stage.
    clientCertificate?: ClientCertificate;
    // A description of the stage's purpose.
    description?: string;
    // Settings for all methods in the stage.
    methodSettings?: MethodSetting[];
    // A map (string to string map) that defines the stage variables, where the variable name is the key and the
    // variable value is the value. Variable names are limited to alphanumeric characters. Values must match the
    // following regular expression: `[A-Za-z0-9-._~:/?#&=,]+`.
    variables?: {[key: string]: string};
}

