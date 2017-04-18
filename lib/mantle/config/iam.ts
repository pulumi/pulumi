// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as aws from "@coconut/aws";

// This file contents various Identity and Access Management (IAM) variables.  Eventually we want it to be highly
// customizable and configurable, but for now it is enormously naive and straightforward.

let awsLambdaRole: aws.iam.Role | undefined;

// getAWSLambdaRole returns a base AWS role suitable Lambda execution, lazily allocating it if necessary.
export function getAWSLambdaRole(): aws.iam.Role {
    if (awsLambdaRole === undefined) {
        awsLambdaRole = new aws.iam.Role("func-exec-role", {
            assumeRolePolicyDocument: {
                Version: "2012-10-17",
                Statement: [{
                    Effect: "Allow",
                    Principal: {
                        Service: [
                            "lambda.amazonaws.com",
                        ],
                    },
                    Action: [
                        "sts:AssumeRole",
                    ],
                }],
            },
        });
    }
    return awsLambdaRole;
}

