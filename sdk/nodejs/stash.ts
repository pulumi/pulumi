// Copyright 2025-2025, Pulumi Corporation.
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

import { Input, Output } from "./output";
import { CustomResource, CustomResourceOptions } from "./resource";

/**
 * Manages a reference to a Pulumi stash value.
 */
export class Stash extends CustomResource {
    /**
     * Output is any value stored in the stash resource.
     */
    public readonly output!: Output<any>;

    /**
     * Input is the value stored in the stash resource.
     */
    public readonly input!: Output<any>;

    /**
     * Create a {@link Stash} resource with the given arguments, and options.
     *
     * @param args The arguments to use to populate this resource's properties.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(name: string, args: StashArgs, opts?: CustomResourceOptions) {
        super(
            "pulumi:index:Stash",
            name,
            {
                input: args.input,
                output: undefined,
            },
            opts,
        );
    }
}

/**
 * The set of arguments for constructing a {@link Stash} resource.
 */
export interface StashArgs {
    /**
     * The value to store in the stash resource.
     */
    readonly input: Input<any>;
}
