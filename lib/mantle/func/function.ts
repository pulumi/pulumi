// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as aws from "@lumi/aws";
import * as kubefission from "@lumi/kubefission";
import {asset} from "@lumi/lumi";
import * as arch from "../arch";
import * as config from "../config";
import * as runtime from "../runtime";

// Function is a cross-cloud function abstraction whose source code is taken from a string, file, or network asset.
// For example, `https://gist.github.com/pulumi/fe8a5ae322ffe63fac90535eb554237f` will use a Gist published on GitHub,
// while `file://./hello.js` will load the code from a file named hello.js in the current working directory.  The
// default protocol is file://, so `hello.js` alone will likewise load a file named hello.js at deployment time.
export class Function {
    private readonly name: string;          // the function name.
    private readonly runtime: arch.Runtime; // the function's language runtime.
    private readonly code: asset.Asset;     // the function's code.
    private readonly handler: string;       // the function's entrypoint/handler.
    private readonly resource: any;         // the cloud-specific function object.

    constructor(name: string, code: asset.Asset, runtime: arch.Runtime) {
        this.name = name;
        this.code = code;
        this.runtime = runtime;
        this.resource = this.initCloudResources();
    }

    // getResource returns the underlying cloud resource.
    public getResource(): any {
        return this.resource;
    }

    // initCloudResources sets up the right resources for the given cloud and scheduler target.
    private initCloudResources(): any {
        let target: arch.Arch = config.requireArch();
        if (target.scheduler === arch.schedulers.kubernetes) {
            return this.initKubernetesResources();
        }
        else {
            switch (target.cloud) {
                case arch.clouds.aws:
                    return this.initAWSResources();
                case arch.clouds.gcp:
                    return this.initGCPResources();
                case arch.clouds.azure:
                    return this.initAzureResources();
                default:
                    throw new Error("Unsupported target cloud: " + target.cloud);
            }
        }
    }

    private initKubernetesResources(): any {
        return new kubefission.Function(this.name, {
            code: this.code,
            environment: runtime.kubernetes.getFissionEnvironment(this.runtime),
        });
    }

    private initAWSResources(): any {
        // Generate a thunk that invokes the callback with the right arguments.
        return new aws.lambda.Function(this.name, {
            code: new asset.AssetArchive({
                ["index" + arch.runtimes.ext[this.runtime]]: this.code,
            }),
            handler: "index.handler",
            runtime: runtime.aws.getLambdaRuntime(this.runtime),
            role:    runtime.aws.getLambdaRole(),
        });
    }

    private initGCPResources(): void {
        throw new Error("Google Cloud Functions not yet implemented");
    }

    private initAzureResources(): void {
        throw new Error("Azure Functions not yet implemented");
    }
}

