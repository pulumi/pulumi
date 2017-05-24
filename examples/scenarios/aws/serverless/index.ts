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
  ],
  hashKey: "Album",
  rangeKey: "Artist",
  readCapacity: 1,
  writeCapacity: 1
})

// [Workaround] Declare variables that should be available on the global scope of the lambda
let console: any

function createLambda() {
  let hello = "Hello, world!"
  let num = 3
  let obj = { x: 42 }
  let mus = music

  let lambda = new aws.lambda.FunctionX(
    "mylambda",
    [aws.iam.AWSLambdaFullAccess],
    (event: any, context: aws.lambda.Context) => {
      console.log(hello);
      console.log("Invoked function: " + context.invokedFunctionArn);
      console.log("Time remaining : " + context.getRemainingTimeInMillis());
    }
  );
}

createLambda();
