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

import { jsonStringify, objectKeys, printf, sha1hash } from "@lumi/lumirt";
import { Deployment, RestAPI, Stage } from "../apigateway";
import { requireRegion } from "../config";
import { Permission } from "../lambda";
import { Function } from "./function";

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
    };
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
        paths: {},
    };
}

function createPathSpec(lambdaARN: string): SwaggerOperation {
    let region = requireRegion();
    return {
        "x-amazon-apigateway-integration": {
            uri: "arn:aws:apigateway:" + region + ":lambda:path/2015-03-31/functions/" + lambdaARN + "/invocations",
            passthroughBehavior: "when_no_match",
            httpMethod: "POST",
            type: "aws_proxy",
        },
    };
}

// API is a higher level abstraction for working with AWS APIGateway reources.
export class API {
    public api: RestAPI;
    public deployment: Deployment;
    private swaggerSpec: SwaggerSpec;
    private apiName: string;
    private lambdas: { [key: string]: Function};

    constructor(apiName: string) {
        this.apiName = apiName;
        this.swaggerSpec = createBaseSpec(apiName);
        this.lambdas = {};
    }

    public route(method: string, path: string, lambda: Function) {
        if (this.swaggerSpec.paths[path] === undefined) {
            this.swaggerSpec.paths[path] = {};
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
                swaggerMethod = (<any>method).toLowerCase();
                break;
            case "any":
                swaggerMethod = "x-amazon-apigateway-any-method";
                break;
            default:
                throw new Error("Method not supported: " + method);
        }
        this.swaggerSpec.paths[path][swaggerMethod] = createPathSpec(lambda.lambda.arn);
        this.lambdas[swaggerMethod + ":" + path] = lambda;
    }

    public publish(): Stage {
        this.api = new RestAPI(this.apiName, {
            body: this.swaggerSpec,
        });
        let deploymentId = sha1hash(jsonStringify(this.swaggerSpec));
        this.deployment = new Deployment(this.apiName + "_" + deploymentId, {
            restAPI: this.api,
            description: "Deployment of version " + deploymentId,
        });
        let stage = new Stage(this.apiName + "_stage", {
            stageName: "stage",
            description: "The current deployment of the API.",
            restAPI: this.api,
            deployment: this.deployment,
        });

        let pathKeys = objectKeys(this.swaggerSpec.paths);
        for (let i = 0; i < (<any>pathKeys).length; i++) {
            let path = pathKeys[i];
            let methodKeys = objectKeys(this.swaggerSpec.paths[path]);
            for (let j = 0; j < (<any>methodKeys).length; j++) {
                let method = methodKeys[j];
                let lambda = this.lambdas[method + ":" + path];
                if (method === "x-amazon-apigateway-any-method") {
                    method = "*";
                } else {
                    method = (<any>method).toUpperCase();
                }
                let invokePermission = new Permission(this.apiName + "_invoke_" + sha1hash(method + path), {
                    action: "lambda:invokeFunction",
                    function: lambda.lambda,
                    principal: "apigateway.amazonaws.com",
                    sourceARN: stage.executionARN + "/" + method + path,
                });
            }
        }
        return stage;
    }
}

