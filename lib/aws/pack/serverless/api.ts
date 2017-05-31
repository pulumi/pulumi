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
    public stage: Stage
    public deployment: Deployment

    constructor(apiName: string, stageName: string, routes?: Route[]) {
        if (stageName === undefined) {
            throw new Error("Missing required stage name");
        }
        if (routes === undefined) {
            routes = [];
        }

        let swaggerSpec = createBaseSpec(apiName);
        for (let i = 0; i < (<any>routes).length; i++) {
            let route = routes[i];
            if (swaggerSpec.paths[route.path] === undefined) {
                swaggerSpec.paths[route.path] = {}
            }
            let swaggerMethod: string;
            switch ((<any>route.method).toLowerCase()) {
                case "get":
                case "put":
                case "post":
                case "delete":
                case "options":
                case "head":
                case "patch":
                    swaggerMethod = (<any>route.method).toLowerCase()
                    break;
                case "any":
                    swaggerMethod = "x-amazon-apigateway-any-method"
                    break;
                default:
                    throw new Error("Method not supported: " + route.method);
            }
            // TODO[pulumi/lumi#90]: Once we suport output properties, we can use `route.lambda.lambda.arn` as input 
            // to constructing this apigateway lambda invocation uri.
            swaggerSpec.paths[route.path][swaggerMethod] = createPathSpec("arn:aws:lambda:us-east-1:490047557317:function:webapi-test-func");
        }

        this.api = new RestAPI(apiName, {
            body: swaggerSpec
        });

        let deploymentId = sha1hash(jsonStringify(swaggerSpec));

        this.deployment = new Deployment(apiName + "_" + deploymentId, {
            restAPI: this.api,
        });

        this.stage = new Stage(apiName + "_prod", {
            stageName: "prod",
            description: "The production deployment of the API.",
            restAPI: this.api,
            deployment: this.deployment
        });
    }
}

