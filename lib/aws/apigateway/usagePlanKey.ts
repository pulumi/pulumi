// Copyright 2017 Pulumi, Inc. All rights reserved.

import {APIKey} from "./apiKey";
import {UsagePlan} from "./usagePlan";
import * as cloudformation from "../cloudformation";

// The UsagePlanKey resource associates an Amazon API Gateway API key with an API Gateway usage plan.  This association
// determines which user the usage plan is applied to.
export class UsagePlanKey extends cloudformation.Resource implements UsagePlanKeyProperties {
    public readonly key: APIKey;
    public readonly usagePlan: UsagePlan;

    constructor(name: string, args: UsagePlanKeyProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::UsagePlanKey",
        });
        this.key = args.key;
        this.usagePlan = args.usagePlan;
    }
}

export interface UsagePlanKeyProperties {
    // The API key for the API resource to associate with a usage plan.
    readonly key: APIKey;
    // The usage plan.
    readonly usagePlan: UsagePlan;
}

