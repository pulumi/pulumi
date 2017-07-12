// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

/* tslint:disable:ordered-imports */

import * as arch from "../../arch";
import * as kubefission from "@lumi/kubefission";

let fissionEnvs: {[langrt: string]: kubefission.Environment} = {};

// getFissionEnvironment returns the Kubernetes Fission environment for the given language runtime.
export function getFissionEnvironment(langrt: arch.Runtime): kubefission.Environment {
    // If the target environment has been created, reuse it.
    let env: kubefission.Environment | undefined = fissionEnvs[langrt];
    if (env !== undefined) {
        return env;
    }

    // If not, create one on-demand, caching it before returning it.
    let image: string;
    switch (langrt) {
        case arch.runtimes.NodeJS:
            image = "fission/node-env";
            break;
        case arch.runtimes.Python:
            image = "fission/python-env";
            break;
        default:
            throw new Error("Unsupported Kubernetes Fission language runtime: " + langrt);
    }

    env = new kubefission.Environment(langrt, {
        runContainerImageURL: image,
    });
    fissionEnvs[langrt] = env;
    return env;
}

