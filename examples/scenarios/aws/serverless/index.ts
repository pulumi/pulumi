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

let music = new aws.dynamodb.Table("music", {
  attributes: [
    { name: "Album", type: "S" },
    { name: "Artist", type: "S" },
    { name: "NumberOfSongs", type: "N" },
    { name: "Sales", type: "N" },
  ],
  hashKey: "Album",
  rangeKey: "Artist",
  readCapacity: 1,
  writeCapacity: 1,
  globalSecondaryIndexes: [
    {
      indexName: "myGSI",
      hashKey: "Sales",
      rangeKey: "Artist",
      readCapacity: 1,
      writeCapacity: 1,
      nonKeyAttributes: ["Album", "NumberOfSongs"],
      projectionType: "INCLUDE",
    },
    {
      indexName: "myGSI2",
      hashKey: "NumberOfSongs",
      rangeKey: "Sales",
      nonKeyAttributes: ["Album", "Artist"],
      projectionType: "INCLUDE",
      readCapacity: 2,
      writeCapacity: 2,
    },
  ],
})

// TODO[pulumi/lumi#174] Until we have global definitions available in Lumi for these APIs that are expected 
// by runtime code, we'll declare variables that should be available on the global scope of the lambda to keep
// TypeScript type checking happy.
let console: any

function createLambda() {
  // TODO[pulumi/lumi#175] Currently, we can only capture local variables, not module scope variables,
  // so we keep this inside a helper function.
  let hello = "Hello, world!"
  let num = 3
  let obj = { x: 42 }
  let mus = music

  let lambda = new aws.lambda.FunctionX(
    "mylambda",
    [aws.iam.AWSLambdaFullAccess],
    (event, context, callback) => {
      console.log(hello);
      console.log(obj.x);
      console.log("Music table hash key is: " + mus.hashKey);
      console.log("Invoked function: " + context.invokedFunctionArn);
      callback(null, "Succeeed with " + context.getRemainingTimeInMillis() + "ms remaining.");
    }
  );
  return lambda;
}

let lambda = createLambda();

let swaggerSpec = {
    swagger: "2.0",
    info: { title: "webapi" },
    schemes: ["https"],
    paths: {
        "/bar": {
            "x-amazon-apigateway-any-method": {
                "produces": [
                    "application/json"
                ],
                "parameters": [
                    {
                        "name": "proxy",
                        "in": "path",
                        "required": true,
                        "type": "string"
                    }
                ],
                "x-amazon-apigateway-integration": {
                    "responses": {
                        "default": {
                            "statusCode": "200"
                        }
                    },
                    // TODO[pulumi/lumi#90]: Once we suport output properties, we can use `lambda.ARN` as input 
                    // to constructing this apigateway lambda invocation uri.
                    "uri": "arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:490047557317:function:webapi-test-func/invocations",
                    "passthroughBehavior": "when_no_match",
                    "httpMethod": "POST",
                    "contentHandling": "CONVERT_TO_TEXT",
                    "type": "aws_proxy"
                }
            }
        }
    }
}

let api = new aws.apigateway.RestAPI("web", {
    body: swaggerSpec
})

let deployment = new aws.apigateway.Deployment("v1", {
    restAPI: api,
})

let stage = new aws.apigateway.Stage("prod", {
    stageName: "prod",
    description: "The production deployment of the API.",
    restAPI: api,
    deployment: deployment
})