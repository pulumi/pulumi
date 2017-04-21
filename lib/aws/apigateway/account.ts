// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as cloudformation from "../cloudformation";
import * as iam from "../iam";

// The Account resource specifies the AWS Identity and Access Management (IAM) role that Amazon API
// Gateway (API Gateway) uses to write API logs to Amazon CloudWatch Logs (CloudWatch Logs).
export class Account extends cloudformation.Resource implements AccountProperties {
    public cloudWatchRole?: iam.Role;

    constructor(name: string, args: AccountProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::Account",
        });
        this.cloudWatchRole = args.cloudWatchRole;
    }
}

export interface AccountProperties {
    // cloudWatchRole is the IAM role that has write access to CloudWatch Logs in your account.
    cloudWatchRole?: iam.Role;
}

