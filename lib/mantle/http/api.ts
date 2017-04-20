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
        let fnc: any = this.function.getResource();
        if (!(fnc instanceof kubefission.Function)) {
            throw new Error("Kubernetes API Gateway cannot target a non-Kubernetes function object");
        }

        return new kubefission.HTTPTrigger({
            name: this.path, // TODO: replace("/", "_")
            urlPattern: this.path,
            method: this.method,
            function: <kubefission.Function>fnc,
        });
    }

    private initAWSResources(): void {
        throw new Error("AWS API Gateways not yet implemented");
    }

    private initGCPResources(): void {
        throw new Error("Google Cloud API Gateways not yet implemented");
    }

    private initAzureResources(): void {
        throw new Error("Azure API Gateways not yet implemented");
    }
}

// APIMethod is the set of HTTP(S) methods supported by API gateways.
export type APIMethod = "DELETE" | "GET" | "HEAD" | "OPTIONS" | "PATCH" | "POST" | "PUT";

