// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as lumi from "@lumi/lumi";
import * as aws from "@lumi/aws";

///////////////////
// Lambda Function
///////////////////
let policy = {
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}

let role = new aws.iam.Role("mylambdarole", {
  assumeRolePolicyDocument: policy,
  managedPolicyARNs: [aws.iam.AWSLambdaFullAccess],
});

let lambda = new aws.lambda.Function("mylambda", {
  code: new lumi.asset.AssetArchive({
    "index.js": new lumi.asset.String("exports.handler = (ev, ctx, cb) => cb('Hello, world!');"),
  }),
  role: role,
  handler: "index.handler",
  runtime: aws.lambda.NodeJS6d10Runtime,
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
})


///////////////////
// APIGateway RestAPI
///////////////////
let region = aws.config.requireRegion();

let lambdaarn = "arn:aws:lambda:us-east-1:490047557317:function:webapi-test-func"; // should be lambda.arn

let swaggerSpec = {
  swagger: "2.0",
  info: { title: "myrestapi", version: "1.0" },
  paths: {
    "/bambam": {
      "x-amazon-apigateway-any-method": {
        "x-amazon-apigateway-integration": {
          uri: "arn:aws:apigateway:" + region + ":lambda:path/2015-03-31/functions/" + lambdaarn + "/invocations",
          passthroughBehavior: "when_no_match",
          httpMethod: "POST",
          type: "aws_proxy"
        }
      }
    }
  }
}

let restAPI = new aws.apigateway.RestAPI("myrestapi", {
  body: swaggerSpec
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

