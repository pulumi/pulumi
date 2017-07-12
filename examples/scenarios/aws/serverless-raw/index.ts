// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as aws from "@lumi/aws";
import * as lumi from "@lumi/lumi";
let region = aws.config.requireRegion();

///////////////////
// Lambda Function
///////////////////
let policy = {
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com",
      },
      "Effect": "Allow",
      "Sid": "",
    },
  ],
};

let role = new aws.iam.Role("mylambdarole", {
  assumeRolePolicyDocument: policy,
  managedPolicyARNs: [aws.iam.AWSLambdaFullAccess],
});

let lambda = new aws.lambda.Function("mylambda", {
  code: new lumi.asset.AssetArchive({
    "index.js": new lumi.asset.String(
        "exports.handler = (e, c, cb) => cb(null, {statusCode: 200, body: 'Hello, world!'});",
    ),
  }),
  role: role,
  handler: "index.handler",
  runtime: aws.lambda.NodeJS6d10Runtime,
});

///////////////////
// Logging
///////////////////
let logGroup = new aws.cloudwatch.LogGroup("mylambda-logs", {
  logGroupName: "/aws/lambda/" + lambda.functionName,
  retentionInDays: 7,
});

let logcollector = new aws.lambda.Function("mylambda-logcollector", {
  code: new lumi.asset.AssetArchive({
    "index.js": new lumi.asset.String(
        "exports.handler = (e, c, cb) => console.log(e);",
    ),
  }),
  role: role,
  handler: "index.handler",
  runtime: aws.lambda.NodeJS6d10Runtime,
});

let permission = new aws.lambda.Permission("logcollector-permission", {
  action: "lambda:InvokeFunction",
  principal: "logs." + region + ".amazonaws.com",
  sourceARN: logGroup.id + ":*",
  function: logcollector,
});

let logSubscription = new aws.cloudwatch.LogSubscriptionFilter("logsubscription", {
  destinationArn: logcollector.id,
  logGroupName: logGroup.logGroupName!,
  filterPattern: "",
});

///////////////////
// DynamoDB Table
///////////////////
let music = new aws.dynamodb.Table("music", {
  attributes: [
    { name: "Album", type: "S" },
    { name: "Artist", type: "S" },
  ],
  hashKey: "Album",
  rangeKey: "Artist",
  readCapacity: 1,
  writeCapacity: 1,
});

///////////////////
// APIGateway RestAPI
///////////////////

let swaggerSpec = {
  swagger: "2.0",
  info: { title: "myrestapi", version: "1.0" },
  paths: {
    "/bambam": {
      "x-amazon-apigateway-any-method": {
        "x-amazon-apigateway-integration": {
          uri: "arn:aws:apigateway:" + region + ":lambda:path/2015-03-31/functions/" + lambda.arn + "/invocations",
          passthroughBehavior: "when_no_match",
          httpMethod: "POST",
          type: "aws_proxy",
        },
      },
    },
  },
};

let restAPI = new aws.apigateway.RestAPI("myrestapi", {
  body: swaggerSpec,
});

let deployment = new aws.apigateway.Deployment("myrestapi_deployment", {
  restAPI: restAPI,
  description: "my deployment",
});

let stage = new aws.apigateway.Stage("myrestapi-prod", {
  restAPI: restAPI,
  deployment: deployment,
  stageName: "prod",
});

