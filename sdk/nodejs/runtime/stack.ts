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

import { getProject, getStack } from "../metadata";
import { ComponentResource, Inputs, Output, output } from "../resource";
import { getRootResource, setRootResource } from "./settings";

/**
 * rootPulumiStackTypeName is the type name that should be used to construct the root component in the tree of Pulumi
 * resources allocated by a deployment.  This must be kept up to date with
 * `github.com/pulumi/pulumi/pkg/resource/stack.RootPulumiStackTypeName`.
 */
export const rootPulumiStackTypeName = "pulumi:pulumi:Stack";

/**
 * runInPulumiStack creates a new Pulumi stack resource and executes the callback inside of it.  Any outputs
 * returned by the callback will be stored as output properties on this resulting Stack object.
 */
export function runInPulumiStack(init: () => any): Promise<Inputs | undefined> {
    const stack = new Stack(init);
    return stack.outputs.promise();
}

/**
 * Stack is the root resource for a Pulumi stack. Before invoking the `init` callback, it registers itself as the root
 * resource with the Pulumi engine.
 */
class Stack extends ComponentResource {
    /**
     * The outputs of this stack, if the `init` callback exited normally.
     */
    public readonly outputs: Output<Inputs | undefined>;

    constructor(init: () => Inputs) {
        super(rootPulumiStackTypeName, `${getProject()}-${getStack()}`);
        this.outputs = output(this.runInit(init));
    }

    /**
     * runInit invokes the given init callback with this resource set as the root resource. The return value of init is
     * used as the stack's output properties.
     *
     * @param init The callback to run in the context of this Pulumi stack
     */
    private async runInit(init: () => Inputs): Promise<Inputs | undefined> {
        const parent = await getRootResource();
        if (parent) {
            throw new Error("Only one root Pulumi Stack may be active at once");
        }

        await setRootResource(this);
        let outputs: Inputs | undefined;
        try {
            outputs = init();
        } finally {
            super.registerOutputs(outputs);
        }

        return outputs;
    }
}
