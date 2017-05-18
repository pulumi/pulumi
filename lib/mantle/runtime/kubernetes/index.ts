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

