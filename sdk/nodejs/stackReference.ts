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

import { all, CustomResource, CustomResourceOptions, Input, Output, output } from "./resource";

/**
 * Manages a reference to a Pulumi stack. The referenced stack's outputs are available via the
 * `outputs` property or the `output` method.
 */
export class StackReference extends CustomResource {
    /**
     * The name of the referenced stack.
     */
    public readonly name: Output<string>;

    /**
     * The outputs of the referenced stack.
     */
    public readonly outputs: Output<{[name: string]: any}>;

    /**
     * Create a StackReference resource with the given unique name, arguments, and options.
     *
     * If args is not specified, the name of the referenced stack will be the name of the StackReference resource.
     *
     * @param name The _unique_ name of the stack reference.
     * @param args The arguments to use to populate this resource's properties.
     * @Param opts A bag of options that control this resource's behavior.
     */
    constructor(name: string, args?: StackReferenceArgs, opts?: CustomResourceOptions) {
        args = args || {};

        super("pulumi:pulumi:StackReference", name, {
            name: args.name || name,
            outputs: undefined,
        }, { ...opts, id: args.name || name });
    }

    /**
     * Fetches the value of the named stack output.
     *
     * @param name The name of the stack output to fetch.
     */
    public getOutput(name: Input<string>): Output<any> {
        return all([output(name), this.outputs]).apply(([n, os]) => os[n]);
    }
}

/**
 * The set of arguments for constructing a StackReference resource.
 */
export interface StackReferenceArgs {
    /**
     * The name of the stack to reference.
     */
    readonly name?: Input<string>;
}
