// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// The aws.serverless module provides abstractions similar to those available in the Serverless Application Model.
// See: https://github.com/awslabs/serverless-application-model/blob/master/versions/2016-10-31.md#awsserverlessapi
//
// In particular, these are similar to the following AWS CloudFormation resource types:
// * AWS::Serverless::Function
// * AWS::Serverless::Api
// * AWS::Serverless::SimpleTable

export * from "./api";
export * from "./function";
