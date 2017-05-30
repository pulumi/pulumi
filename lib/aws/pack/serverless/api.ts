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
    info: {title: string; };
    schemes: string[];
    paths: { [path: string]: SwaggerRoute; }
}

interface SwaggerRoute {
    "x-amazon-apigateway-any-method": {
        produces: string[];
        parameters: SwaggerParameter[];
        "x-amazon-apigateway-integration": {
            responses: { [name: string]: SwaggerResponse; };
            uri: string;
            passthroughBehavior: string;
            httpMethod: string;
            contentHandling: string;
            type: string;
        }
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
        info: { title: apiName },
        schemes: ["https"],
        paths: {}
    }
}

function createPathSpec(lambdaARN: string): SwaggerRoute {
    return {
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
                // TODO: Pass `region`
                "uri": "arn:aws:apigateway:" + "us-east-1" + ":lambda:path/2015-03-31/functions/" + lambdaARN + "/invocations",
                "passthroughBehavior": "when_no_match",
                "httpMethod": "POST",
                "contentHandling": "CONVERT_TO_TEXT",
                "type": "aws_proxy"
            }
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
        for(let i = 0; i < (<any>routes).length; i++) {
            swaggerSpec.paths[routes[i].path] = createPathSpec(/*routes[i].lambda.lambda.arn*/ "arn:aws:lambda:us-east-1:490047557317:function:webapi-test-func");
        }

        this.api = new RestAPI(apiName, {
            body: swaggerSpec
        });

        let deploymentId = sha1hash(jsonStringify(swaggerSpec));

        this.deployment = new Deployment(apiName+ "_" + deploymentId, {
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

