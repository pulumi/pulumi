// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as arch from "../arch";
import * as config from "../config";
import * as func from "../func";
import * as runtime from "../runtime";
import * as aws from "@coconut/aws";
import * as kubefission from "@coconut/kube-fission";
import {asset} from "@coconut/coconut";

// API is a cross-cloud API gateway endpoint.
export class API {
    private readonly path: string;            // the URL path part.
    private readonly method: APIMethod;       // the HTTP method triggering this endpoint.
    private readonly function: func.Function; // the function to run when the API is called.
    private readonly resource: any;           // the underlying API resource.

    constructor(path: string, method: APIMethod, fnc: func.Function) {
        this.path = path;
        this.method = method;
        this.function = fnc;
        this.resource = this.initCloudResources();
    }

    // initCloudResources sets up the right resources for the given cloud and scheduler target.
    private initCloudResources(): any {
        let target: arch.Arch = config.requireArch();
        if (target.scheduler === arch.schedulers.Kubernetes) {
            return this.initKubernetesResources();
        }
        else {
            switch (target.cloud) {
                case arch.clouds.AWS:
                    return this.initAWSResources();
                case arch.clouds.GCP:
                    return this.initGCPResources();
                case arch.clouds.Azure:
                    return this.initAzureResources();
                default:
                    throw new Error("Unsupported target cloud: " + target.cloud);
            }
        }
    }

    private initKubernetesResources(): any {
        // Ensure that we're dealing with a Kubernetes Fission function.
        let funcres: any = this.function.getResource();
        if (!(funcres instanceof kubefission.Function)) {
            throw new Error("Kubernetes API Gateway can only use Kubernetes Fission functions");
        }
        let kubeFunc = <kubefission.Function>funcres;

        // Simply wire up the function to an HTTP trigger.
        // TODO: think about multi-instancing routers, rather than assuming there is a global one.  Ideally we would
        //     be able to instance a parallel Fission provider, rather than relying on a single shared global instance.
        return new kubefission.HTTPTrigger({
            name: this.path, // TODO: replace("/", "_")
            urlPattern: this.path,
            method: this.method,
            function: kubeFunc,
        });
    }

    private initAWSResources(): any {
        let funcres: any = this.function.getResource();
        if (!(funcres instanceof aws.lambda.Function)) {
            throw new Error("AWS API Gateway can only use AWS Lambda functions");
        }
        let lambdaFunc = <aws.lambda.Function>funcres;

        // Create a prefix that all resources will use.
        // TODO: replace(this.path, "/", "_") and use it as part of the name (else multi-subscription won't work).
        let prefix: string = lambdaFunc.name + "-api";

        // Create a Rest API at the desired path with a single deployment / stage.
        let restAPI = new aws.apigateway.RestAPI(prefix, {
            // The body is an OpenAPI specification for the API that we're creating.
            body: {
                info: {
                    title: prefix,
                    version: "1.0",
                },
                paths: {
                    [this.path]: {
                        "x-amazon-apigateway-any-method": {
                            responses: {},
                            "x-amazon-apigateway-integration": {
                                httpMethod: "POST",
                                type: "aws_proxy",
                                uri: runtime.aws.getLambdaAPIInvokeURI(lambdaFunc),
                            },
                        },
                    },
                },
            },
        });
        let deployment = new aws.apigateway.Deployment(prefix + "-deployment", {
            restAPI: restAPI,
            stageName: "Stage",
        });
        let stage = new aws.apigateway.Stage(prefix + "-primary-stage", {
            deployment: deployment,
            restAPI: restAPI,
            stageName: "Primary", // TODO: consider using the Coconut environment name.
        });

        // Grant permissions for the API gateway to invoke the target function.
        let invokePermission = new aws.lambda.Permission(prefix + "-invoke-perm", {
            action: "lambda:invokeFunction",
            function: lambdaFunc,
            principal: "apigateway.amazonaws.com",
            sourceARN: runtime.aws.getAPIExecuteSourceURI(restAPI, stage, this.path),
        });

        return restAPI;
    }

    private initGCPResources(): any {
        throw new Error("Google Cloud API Gateways not yet implemented");
    }

    private initAzureResources(): any {
        throw new Error("Azure API Gateways not yet implemented");
    }
}

// APIMethod is the set of HTTP(S) methods supported by API gateways.
export type APIMethod = "DELETE" | "GET" | "HEAD" | "OPTIONS" | "PATCH" | "POST" | "PUT";

