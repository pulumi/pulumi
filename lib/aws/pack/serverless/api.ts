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

import { jsonStringify, sha1hash, printf } from "@lumi/lumi/runtime"
import { Deployment, RestAPI, Stage } from "../apigateway"
import { Function } from "./function"
import { region } from "../config"

export interface Route {
    method: string;
    path: string;
    lambda: Function;
}

interface SwaggerSpec {
    swagger: string;
    info: SwaggerInfo;
    paths: { [path: string]: { [method: string]: SwaggerOperation; }; };
}

interface SwaggerInfo {
    title: string;
    version: string;
}

interface SwaggerOperation {
    "x-amazon-apigateway-integration": {
        uri: string;
        passthroughBehavior?: string;
        httpMethod: string;
        type: string;
    }
}

interface SwaggerParameter {
    name: string;
    in: string;
    required: boolean;
    type: string;
}

interface SwaggerResponse {
    statusCode: string;
}

function createBaseSpec(apiName: string): SwaggerSpec {
    return {
        swagger: "2.0",
        info: { title: apiName, version: "1.0" },
        paths: {}
    }
}

function createPathSpec(lambdaARN: string): SwaggerOperation {
    return {
        "x-amazon-apigateway-integration": {
            uri: "arn:aws:apigateway:" + region + ":lambda:path/2015-03-31/functions/" + lambdaARN + "/invocations",
            passthroughBehavior: "when_no_match",
            httpMethod: "POST",
            type: "aws_proxy"
        }
    }
}

// API is a higher level abstraction for working with AWS APIGateway reources.
export class API {
    public api: RestAPI
    public deployment: Deployment
    private swaggerSpec: SwaggerSpec
    private apiName: string

    constructor(apiName: string) {
        this.apiName = apiName
        this.swaggerSpec = createBaseSpec(apiName);
        this.api = new RestAPI(apiName, {
            body: this.swaggerSpec
        });
    }

    public route(method: string, path: string, lambda: Function) {
        if (this.swaggerSpec.paths[path] === undefined) {
            this.swaggerSpec.paths[path] = {}
        }
        let swaggerMethod: string;
        switch ((<any>method).toLowerCase()) {
            case "get":
            case "put":
            case "post":
            case "delete":
            case "options":
            case "head":
            case "patch":
                swaggerMethod = (<any>method).toLowerCase()
                break;
            case "any":
                swaggerMethod = "x-amazon-apigateway-any-method"
                break;
            default:
                throw new Error("Method not supported: " + method);
        }
        // TODO[pulumi/lumi#90]: Once we suport output properties, we can use `lambda.lambda.arn` as input 
        //     to constructing this apigateway lambda invocation uri.
        // this.swaggerSpec.paths[path][swaggerMethod] = createPathSpec(lambda.lambda.arn);
        this.swaggerSpec.paths[path][swaggerMethod] = createPathSpec(
            "arn:aws:lambda:us-east-1:490047557317:function:webapi-test-func");
    }

    public publish(stageName?: string): Stage {
        if (stageName === undefined) {
            stageName = "prod"
        }
        let deploymentId = sha1hash(jsonStringify(this.swaggerSpec));
        this.deployment = new Deployment(this.apiName + "_" + deploymentId, {
            restAPI: this.api,
            description: "Deployment of version " + deploymentId,
        });
        let stage = new Stage(this.apiName + "_" + stageName, {
            stageName: stageName,
            description: "The production deployment of the API.",
            restAPI: this.api,
            deployment: this.deployment,
        });
        return stage;
    }
}

