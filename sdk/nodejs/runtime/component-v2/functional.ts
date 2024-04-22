// Copyright 2016-2024, Pulumi Corporation.
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

import { ComponentResource, ComponentResourceOptions, Inputs, Output, merge, mergeOptions } from "../..";
import { enterForkedOutputlessState } from "../outputless";
import { registerResourceOutputs } from "../resource";
import { AsyncComponentContext } from "./context";
import { defaultNamingConvention } from "./defaultNamingConvention";

type ComponentOutputs = Inputs | Promise<Inputs> | Output<Inputs>;

/**
 * Declare a functional component. This is a component that is defined by a factory function that
 * takes inputs and returns outputs. The component is created and managed by the Pulumi engine.
 */

export function FunctionalComponent<I extends Inputs, Outputs extends ComponentOutputs>(
    type: string,
    factory: (name: string, inputs: I, opts?: ComponentResourceOptions) => Outputs,
    defaultOptions?: ComponentResourceOptions,
): (name: string, inputs: I, opts?: ComponentResourceOptions) => Outputs {
    return (name: string, inputs: I, opts?: ComponentResourceOptions) => {
        enterForkedOutputlessState();
        opts = mergeOptions({ namingConvention: defaultNamingConvention}, opts);

        const parent = new ComponentResource(type, name, {}, opts);

        opts = mergeOptions({ parent }, opts);

        const context = new AsyncComponentContext(parent);
        return context.run(() => {
            // Clone the inputs to the factory to mitigate mutation on shared defaults.
            // Could Object.freeze instead, though?
            const outputs = factory(name, { ... inputs}, { ... opts });

            // Await completion of the outputs
            Promise.all([outputs, context.asyncCompletion]).then(() => {
                registerResourceOutputs(parent, outputs);
            });
            return outputs;
        });
    };
}
