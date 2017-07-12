// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

/* tslint:disable:ordered-imports */

import * as arch from "../../arch";
import * as aws from "@lumi/aws";

// This file contains various AWS "runtime" helper methods, including various URIs, ARN management utilities, and
// Identity and Access Management (IAM) variables.  Eventually we want it to be highly customizable and configurable,
// but for now it is enormously naive and straightforward, and acts as a placeholder for boilerplate accumulation.

// getAccountID fetches the current AWS account ID.
function getAccountID(): string {
    throw new Error("TODO: getAccountID not yet implemented");
}

// getAPIExecuteSourceURI retrieves the source URI for a given API, stage name, and path combination.
export function getAPIExecuteSourceURI(
        api: aws.apigateway.RestAPI, stage: aws.apigateway.Stage, path: string): string {
    let region: aws.Region = aws.config.requireRegion();
    return "arn:aws:execute-api:" +
        region + ":" + getAccountID() + ":" +
        api.name + "/" + stage.stageName + "/ANY/" + path;
}

// getLambdaAPIInvokeURI returns the standard API Gateway invocation URI for a lambda.
export function getLambdaAPIInvokeURI(lambda: aws.lambda.Function): string {
    let region: aws.Region = aws.config.requireRegion();
    return "arn:aws:apigateway:" + region + ":lambda:path/2015-03-31/functions/" + lambda.arn + "/invocations";
}

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
            managedPolicyARNs: [
                "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole",
            ],
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

