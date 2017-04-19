// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as arch from "../../arch";
import * as aws from "@coconut/aws";

// This file contents various Identity and Access Management (IAM) variables.  Eventually we want it to be highly
// customizable and configurable, but for now it is enormously naive and straightforward.

let lambdaRole: aws.iam.Role | undefined;

// getLambdaRole returns a base AWS role suitable Lambda execution, lazily allocating it if necessary.
export function getLambdaRole(): aws.iam.Role {
    if (lambdaRole === undefined) {
        lambdaRole = new aws.iam.Role("func-exec-role", {
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
    return lambdaRole;
}

// getLambdaRuntime returns an AWS Lambda runtime for the given generic language runtime architecture.
export function getLambdaRuntime(langrt: arch.Runtime): aws.lambda.Runtime {
    switch (langrt) {
        case arch.runtimes.NodeJS:
            return "nodejs6.10";
        case arch.runtimes.Python:
            return "python2.7";
        default:
            throw new Error("Unsupported AWS language runtime: " + langrt);
    }
}

