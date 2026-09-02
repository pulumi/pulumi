// Copyright 2025, Pulumi Corporation.
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

import * as log from "./log";
import { Input, Output } from "./output";
import { CustomResource, CustomResourceOptions } from "./resource";
import { getCallbacks } from "./runtime/settings";

/**
 * A reducer combines the previously stashed input and output with the current program input
 * to produce a new output value. It is invoked by the engine on update; on create the initial
 * output is just the current input.
 *
 * `oldInput` and `oldOutput` are `undefined` on create.
 */
export type StashReducer = (oldInput: any, oldOutput: any, newInput: any) => any | Promise<any>;

/**
 * Stash stores an arbitrary value in the state.
 */
export class Stash extends CustomResource {
    /**
     * The value saved in the state for the stash.
     */
    public readonly output!: Output<any>;

    /**
     * The most recent value passed to the stash resource.
     */
    public readonly input!: Output<any>;

    /**
     * Create a {@link Stash} resource with the given arguments, and options.
     *
     * @param args The arguments to use to populate this resource's properties.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(name: string, args: StashArgs, opts?: CustomResourceOptions) {
        // If a reducer callback was supplied, register it with the callback server and pass a
        // `{ target, token }` `reducer` input through to the engine. The builtin provider will
        // invoke this callback during Check on update to combine the previously persisted
        // output with the current input; the reducer object itself is stripped from state.
        let reducer: Promise<{ target: string; token: string }> | undefined;
        if (args.reduce !== undefined) {
            const callbacks = getCallbacks();
            if (callbacks === undefined) {
                // Should only happen if running outside a Pulumi program.
                log.warn("Stash reducer requires an active Pulumi resource monitor; ignoring reducer");
            } else {
                const reduce = args.reduce;
                reducer = callbacks.registerStashReducer(reduce).then((cb) => ({
                    target: cb.getTarget(),
                    token: cb.getToken(),
                }));
            }
        }

        super(
            "pulumi:index:Stash",
            name,
            {
                input: args.input,
                output: undefined,
                reducer: reducer,
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

    /**
     * An optional reducer function. When supplied, the engine invokes it during Check on
     * update with `(oldInput, oldOutput, newInput)` and persists the return value as the new
     * `output`. On create the reducer is skipped and `output` is just the current `input`.
     */
    readonly reduce?: StashReducer;
}
