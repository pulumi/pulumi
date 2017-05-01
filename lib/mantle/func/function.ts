// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as arch from "../arch";
import * as config from "../config";
import * as runtime from "../runtime";
import * as aws from "@coconut/aws";
import * as kubefission from "@coconut/kubefission";
import {asset} from "@coconut/coconut";

// Function is a cross-cloud function abstraction whose source code is taken from a string, file, or network asset.
// For example, `https://gist.github.com/pulumi/fe8a5ae322ffe63fac90535eb554237f` will use a Gist published on GitHub,
// while `file://./hello.js` will load the code from a file named hello.js in the current working directory.  The
// default protocol is file://, so `hello.js` alone will likewise load a file named hello.js at deployment time.
export class Function {
    private readonly name: string;          // the function name.
    private readonly runtime: arch.Runtime; // the function's language runtime.
    private readonly code: asset.Archive;   // the function's code archive.
    private readonly resource: any;         // the cloud-specific function object.

    constructor(name: string, code: asset.Archive, runtime: arch.Runtime) {
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
        return new kubefission.Function(this.name, {
            code: this.code,
            environment: runtime.kubernetes.getFissionEnvironment(this.runtime),
        });
    }

    private initAWSResources(): any {
        return new aws.lambda.Function(this.name, {
            code:    this.code,
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

