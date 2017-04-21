// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as cloudformation from "../cloudformation";
import * as ec2 from "../ec2";
import * as iam from "../iam";
import * as kms from "../kms";
import * as sns from "../sns";
import * as sqs from "../sqs";
import {ARN} from "../types";
import {asset} from "@coconut/coconut";

// The Function resource creates an AWS Lambda function that can run code in response to events.
export class Function extends cloudformation.Resource implements FunctionProperties {
    public code: asset.Asset;
    public handler: string;
    public role: iam.Role;
    public runtime: Runtime;
    public readonly functionName?: string;
    public deadLetterConfig?: DeadLetterConfig;
    public description?: string;
    public environment?: Environment;
    public kmsKey?: kms.Key;
    public memorySize?: number;
    public timeout?: number;
    public vpcConfig?: VPCConfig;

    // Output properties:

    // The ARN of the Lambda function, such as `arn:aws:lambda:us-west-2:123456789012:MyStack-AMILookUp-NT5EUXTNTXXD`.
    public arn?: ARN;

    constructor(name: string, args: FunctionProperties) {
        super({
            name: name,
            resource: "AWS::Lambda::Function",
        });
        this.code = args.code;
        this.handler = args.handler;
        this.role = args.role;
        this.runtime = args.runtime;
        this.functionName = args.functionName;
        this.deadLetterConfig = args.deadLetterConfig;
        this.description = args.description;
        this.environment = args.environment;
        this.kmsKey = args.kmsKey;
        this.memorySize = args.memorySize;
        this.timeout = args.timeout;
        this.vpcConfig = args.vpcConfig;
    }
}

export interface FunctionProperties extends cloudformation.TagArgs {
    // code is the source code of your Lambda function.  This supports all the usual Coconut asset schemes, in addition
    // to Amazon Simple Storage Service (S3) bucket locations, indicating with a URI scheme of s3://<bucket>/<object>.
    code: asset.Asset;
    // handler is the name of the function (within your source code) that Lambda calls to start running your code.
    handler: string;
    // role is the AWS Identity and Access Management (IAM) execution role that Lambda assumes when it runs your code
    // to access AWS services.
    role: iam.Role;
    // runtime is the runtime environment for the Lambda function that you are uploading.
    runtime: Runtime;
    // functionName is a name for the function.  If you don't specify a name, a unique physical ID is used instead.
    readonly functionName?: string;
    // deadLetterConfig configures how Lambda handles events that it can't process.  If you don't specify a Dead Letter
    // Queue (DLQ) configuration, Lambda discards events after the maximum number of retries.
    deadLetterConfig?: DeadLetterConfig;
    // description is an optional description of the function.
    description?: string;
    // environment contains key-value pairs that Lambda caches and makes available for your Lambda functions.  Use
    // environment variables to apply configuration changes, such as test and production environment configurations,
    // without changing your Lambda function source code.
    environment?: Environment;
    // kmsKey is a AWS Key Management Service (AMS KMS) key that Lambda uses to encrypt and decrypt environment
    // variables.
    kmsKey?: kms.Key;
    // memorySize is the amount of memory, in MB, that is allocated to your Lambda function.  Lambda uses this value to
    // proportionally allocate the amount of CPU power.  Your function use case determines your CPU and memory
    // requirements.  For example, a database operation might need less memory than an image processing function.  You
    // must specify a value that is greater than or equal to `128` and it must be a multiple of `64`.  You cannot
    // specify a size larger than `1536`.  The default value is `128` MB.
    memorySize?: number;
    // timeout is the function execution time (in seconds) after which Lambda terminates the function.  Because the
    // execution time affects cost, set this value based on the function's expected execution time.  By default, timeout
    // is set to `3` seconds.
    timeout?: number;
    // vpcConfig specifies a VPC configuration that Lambda uses to set up an elastic network interface (ENI).  The ENI
    // enables your function to connect to other resources in your VPC, but it doesn't provide public Internet access.
    // If your function requires Internet access (for example, to access AWS services that don't have VPC endpoints),
    // configure a Network Address Translation (NAT) instance inside your VPC or use an Amazon Virtual Private Cloud
    // (Amazon VPC) NAT gateway.
    vpcConfig?: VPCConfig;
}

// Runtime represents the legal runtime environments for Lambdas.
export type Runtime =
    "nodejs" |
    "nodejs4.3" |
    "nodejs4.3-edge" |
    "nodejs6.10" |
    "java8" |
    "python2.7" |
    "dotnetcore1.0";

// DeadLetterConfig is a property of an AWS Lambda Function resource that specifies a Dead Letter Queue (DLQ) that
// events are sent to when functions cannot be processed.  For example, you can send unprocessed events to an Amazon
// Simple Notification Service (Amazon SQS) topic, where you can take further action.
export interface DeadLetterConfig {
    // target is the target resource where Lambda delivers unprocessed events.  It may be an Amazon SNS topic or Amazon
    // Simple Queue Service (SQS) queue.  For the Lambda function-execution role, you must explicitly provide the
    // relevant permissions so that access to your DLQ resource is part of the execution role for your Lambda function.
    target: any; // TODO: sns.Topic | sqs.Queue;
}

// Environment is a property of an AWS Lambda Function resource that specifies key-value pairs that the function can
// access so that you can apply configuration changes, such as test and production environment configurations, without
// changing the function code.
export type Environment = {[key: string]: string};

// VPCConfig is a property of an AWS Lambda Function resource that enables it to access resources in a VPC.  For more
// information, see http://docs.aws.amazon.com/lambda/latest/dg/vpc.html.
export interface VPCConfig {
    // securityGroups is a list of one or more security groups in the VPC that include the resources to which your
    // Lambda function requires access.
    securityGroups: ec2.SecurityGroup[];
    // subnets is a list of one or more subnet IDs in the VPC that includes the resources to which your Lambda function
    // requires access.
    subnets: ec2.Subnet[];
}

