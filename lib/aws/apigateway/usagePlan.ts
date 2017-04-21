// Copyright 2017 Pulumi, Inc. All rights reserved.

import {RestAPI} from "./restAPI";
import {Stage} from "./stage";
import * as cloudformation from "../cloudformation";

// The UsagePlan resource specifies a usage plan for deployed Amazon API Gateway (API Gateway) APIs.  A
// usage plan enforces throttling and quota limits on individual client API keys. For more information, see
// http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-api-usage-plans.html.
export class UsagePlan extends cloudformation.Resource implements UsagePlanProperties {
    public apiStages?: APIStage[];
    public description?: string;
    public quota?: QuotaSettings;
    public throttle?: ThrottleSettings;
    public usagePlanName?: string;

    constructor(name: string, args: UsagePlanProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::UsagePlan",
        });
        this.apiStages = args.apiStages;
        this.description = args.description;
        this.quota = args.quota;
        this.throttle = args.throttle;
        this.usagePlanName = args.usagePlanName;
    }
}

export interface UsagePlanProperties {
    apiStages?: APIStage[];
    description?: string;
    quota?: QuotaSettings;
    throttle?: ThrottleSettings;
    usagePlanName?: string;
}

// APIStage specifies which Amazon API Gateway (API Gateway) stage and API to associate with a usage plan.
export interface APIStage {
    // The API you want to associate with the usage plan.
    api?: RestAPI;
    // The Stage you want to associate with the usage plan.
    stage?: Stage;
}

// QuotaSettings specifies the maximum number of requests users can make to your Amazon API Gateway (API Gateway) APIs.
export interface QuotaSettings {
    // The maximum number of requests that users can make within the specified time period.
    limit?: number;
    // For the initial time period, the number of requests to subtract from the specified limit.  When you first
    // implement a usage plan, the plan might start in the middle of the week or month.  With this property, you can
    // decrease the limit for this initial time period.
    offset?: number;
    // The time period for which the maximum limit of requests applies.
    period?: QuotaPeriod;
}

// The time period in which a quota limit applies.
export type QuotaPeriod = "DAY" | "WEEK" | "MONTH";

// ThrottleSettings specifies the overall request rate (average requests per second) and burst capacity when users call
// your Amazon API Gateway (API Gateway) APIs.
export interface ThrottleSettings {
    // The maximum API request rate limit over a time ranging from one to a few seconds.  The maximum API request rate
    // limit depends on whether the underlying token bucket is at its full capacity.  For more information about request
    // throttling, see http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html.
    burstRateLimit?: number;
    // The API request steady-state rate limit (average requests per second over an extended period of time). For more
    // information about request throttling, see
    // http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html.
    rateLimit?: number;
}

