// Copyright 2016-2018, Pulumi Corporation.
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

import { Inputs, Output } from "../output";
import { ComponentResource, ComponentResourceOptions } from "../resource";

/**
 * rootPulumiSubStackTypeName is the type name that should be used to construct the root component in the tree of Pulumi
 * resources allocated by a deployment.  This must be kept up to date with
 * `github.com/pulumi/pulumi/sdk/v3/go/common/resource/stack.RootStackType`.
 */
export const rootPulumiSubStackTypeName = "pulumi:pulumi:Stack";

/**
 * Stack is the root resource for a Pulumi stack. Before invoking the `init` callback, it registers itself as the root
 * resource with the Pulumi engine.
 */
export class SubStack extends ComponentResource {
    /**
     * The outputs of this stack, if the `init` callback exited normally.
     */
    public readonly outputs!: Output<Inputs>;

    constructor(name: string, args: SubStackArgs, opts?: ComponentResourceOptions) {
        let resourceInputs: Inputs = {};
        opts = opts || {};
        if (!opts.id) {
            resourceInputs["source"] = args ? args.source : undefined;
            resourceInputs["inputs"] = args ? args.inputs : undefined;
            resourceInputs["prefixResourceNames"] = args ? args.prefixResourceNames : false;
        } else {
            resourceInputs["source"] = undefined;
            resourceInputs["inputs"] = undefined;
            resourceInputs["prefixResourceNames"] = false;
        }
        resourceInputs["outputs"] = undefined;
        super(rootPulumiSubStackTypeName, name, resourceInputs, opts, true);
    }
}

export interface SubStackArgs {
    /**
     * The source of the stack. This can be a local filepath or a remote URI (e.g. GitHub URL).
     */
    source: string;

    /**
     * The input configure the stack.
     */
    inputs?: Inputs;

    /**
     * Whether the resource name of the substack should be prefixed by the substack name.
     */
    prefixResourceNames?: boolean;
}
