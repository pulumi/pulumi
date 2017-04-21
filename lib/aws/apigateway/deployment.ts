// Copyright 2017 Pulumi, Inc. All rights reserved.

import {ClientCertificate} from "./clientCertificate";
import {RestAPI} from "./restAPI";
import {LoggingLevel, MethodSetting} from "./method";
import * as cloudformation from "../cloudformation";

// The Deployment resource deploys an Amazon API Gateway (API Gateway) RestAPI resource to a stage so
// that clients can call the API over the Internet.  The stage acts as an environment.
export class Deployment extends cloudformation.Resource implements DeploymentProperties {
    public restAPI: RestAPI;
    public description?: string;
    public stageDescription?: StageDescription;
    public stageName?: string;

    constructor(name: string, args: DeploymentProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::Deployment",
        });
        this.restAPI = args.restAPI;
        this.description = args.description;
        this.stageDescription = args.stageDescription;
        this.stageName = args.stageName;
    }
}

export interface DeploymentProperties {
    // restAPI is the RestAPI resource to deploy.
    restAPI: RestAPI;
    // description is a description of the purpose of the API Gateway deployment.
    description?: string;
    // stageDescription configures the stage that API Gateway creates with this deployment.
    stageDescription?: StageDescription;
    // stageName is a name for the stage that API Gateway creates with this deployment.  Use only alphanumeric
    // characters.
    stageName?: string;
}

export interface StageDescription {
    // Indicates whether cache clustering is enabled for the stage.
    cacheClusterEnabled?: boolean;
    // The size of the stage's cache cluster.
    cacheClusterSize?: string;
    // Indicates whether the cached responses are encrypted.
    cacheDataEncrypted?: boolean;
    // The time-to-live (TTL) period, in seconds, that specifies how long API Gateway caches responses.
    cacheTTLInSeconds?: number;
    // Indicates whether responses are cached and returned for requests. You must enable a cache cluster on the stage
    // to cache responses. For more information, see
    // http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-caching.html.
    cachingEnabled?: boolean;
    // The client certificate that API Gateway uses to call your integration endpoints in the stage.
    clientCertificate?: ClientCertificate;
    // Indicates whether data trace logging is enabled for methods in the stage. API Gateway pushes these logs to Amazon
    // CloudWatch Logs.
    dataTraceEnabled?: boolean;
    // A description of the purpose of the stage.
    description?: string;
    // The logging level for this method.
    loggingLevel?: LoggingLevel;
    // Configures settings for all of the stage's methods.
    methodSettings?: MethodSetting[];
    // Indicates whether Amazon CloudWatch metrics are enabled for methods in the stage.
    metricsEnabled?: boolean;
    // The name of the stage, which API Gateway uses as the first path segment in the invoke URI.
    stageName?: string;
    // The number of burst requests per second that API Gateway permits across all APIs, stages, and methods in your
    // AWS account. For more information, see
    // http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html.
    throttlingBurstLimit?: number;
    // The number of steady-state requests per second that API Gateway permits across all APIs, stages, and methods in
    // your AWS account. For more information, see
    // http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html.
    throttlingRateLimit?: number;
    // A map that defines the stage variables.  Variable names must consist of alphanumeric characters, and the values
    // must match the following regular expression: `[A-Za-z0-9-._~:/?#&=,]+`.
    variables?: {[key: string]: string};
}

