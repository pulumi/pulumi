// Copyright 2017 Pulumi, Inc. All rights reserved.

import {Function} from "./function";
import * as cloudformation from "../cloudformation";
import {ARN} from "../types";

// The Permission resource associates a policy statement with a specific AWS Lambda function's access policy.  The
// function policy grants a specific AWS service or application permission to invoke the function.  For more
// information, see http://docs.aws.amazon.com/lambda/latest/dg/API_AddPermission.html.
export class Permission extends cloudformation.Resource implements PermissionProperties {
    public readonly action: string;
    public readonly function: Function;
    public readonly principal: string;
    public readonly sourceAccount?: string;
    public readonly sourceARN?: ARN;

    constructor(name: string, args: PermissionProperties) {
        super({
            name: name,
            resource: "AWS::Lambda::Permission",
        });
        this.action = args.action;
        this.function = args.function;
        this.principal = args.principal;
        this.sourceAccount = args.sourceAccount;
        this.sourceARN = args.sourceARN;
    }
}

export interface PermissionProperties extends cloudformation.TagArgs {
    // The Lambda actions that you want to allow in this statement.  For example, you can specify lambda:CreateFunction
    // to specify a certain action, or use a wildcard (lambda:*) to grant permission to all Lambda actions.  For a list
    // of actions, see http://docs.aws.amazon.com/IAM/latest/UserGuide/list_lambda.html.
    readonly action: string;
    // The Lambda function that you want to associate with this statement.
    readonly function: Function;
    // The entity for which you are granting permission to invoke the Lambda function.  This entity can be any valid AWS
    // service principal, such as `s3.amazonaws.com` or `sns.amazonaws.com`, or, if you are granting cross-account
    // permission, an AWS account ID.  For example, you might want to allow a custom application in another AWS account
    // to push events to Lambda by invoking your function.
    readonly principal: string;
    // The AWS account ID (without hyphens) of the source owner.  For example, if you specify an S3 bucket in the
    // sourceARN property, this value is the bucket owner's account ID.  You can use this property to ensure that all
    // source principals are owned by a specific account.
    readonly sourceAccount?: string;
    // The ARN of a resource that is invoking your function.  When granting Amazon Simple Storage Service (Amazon S3)
    // permission to invoke your function, specify this property with the bucket ARN as its value.  This ensures that
    // events generated only from the specified bucket, not just any bucket from any AWS account that creates a mapping
    // to your function, can invoke the function.
    readonly sourceARN?: string;
}

