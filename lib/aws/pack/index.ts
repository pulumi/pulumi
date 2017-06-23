// Copyright 2016-2017, Pulumi Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

export * from "./types";

import * as apigateway from "./apigateway";
import * as cloudwatch from "./cloudwatch";
import * as config from "./config";
import * as dynamodb from "./dynamodb";
import * as ec2 from "./ec2";
import * as elasticbeanstalk from "./elasticbeanstalk";
import * as iam from "./iam";
import * as kms from "./kms";
import * as lambda from "./lambda";
import * as s3 from "./s3";
import * as serverless from "./serverless";
import * as sns from "./sns";
import * as sqs from "./sqs";
export {apigateway, cloudwatch, config, dynamodb, ec2, elasticbeanstalk, iam, kms, lambda, s3, serverless, sns, sqs};

