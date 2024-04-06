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

import { Inputs, Output } from "./output";
import { ComponentResource, ComponentResourceOptions } from "./resource";

export class Program extends ComponentResource {
    /**
     * The outputs of this stack, if the `init` callback exited normally.
     */
    public readonly outputs!: Output<Inputs>;

    constructor(name: string, args: StackArgs, opts?: ComponentResourceOptions) {
        const resourceInputs: Inputs = {};
        opts = opts || {};
        if (!opts.id) {
            resourceInputs["source"] = args ? args.source : undefined;
            resourceInputs["config"] = args ? args.config : undefined;
        } else {
            resourceInputs["source"] = undefined;
            resourceInputs["config"] = undefined;
        }
        resourceInputs["outputs"] = undefined;
        super("pulumi:pulumi:Stack", name, resourceInputs, opts, true);
    }
}

export interface StackArgs {
    /**
     * The source of the stack. This can be a local filepath or a remote URI (e.g. GitHub URL).
     */
    source: string;

    /**
     * The input configure the stack.
     */
    config?: Inputs;
}
