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

import * as log from "../log";
import { getProject, getStack } from "../metadata";
import { ComponentResource, Inputs, Resource } from "../resource";
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
export function runInPulumiStack(init: () => any): void {
    const _ = new Stack(init);
}

class Stack extends ComponentResource {
    constructor(init: () => Inputs) {
        super(rootPulumiStackTypeName, `${getProject()}-${getStack()}`);

        if (getRootResource()) {
            throw new Error("Only one root Pulumi Stack may be active at once");
        }
        let outputs: Inputs | undefined;
        try {
            setRootResource(this);      // install ourselves as the current root.
            outputs = init();           // run the init code.
        }
        finally {
            super.registerOutputs(outputs); // save the outputs for this component to whatever the init returned.
            // intentionally not removing the root resource because we want subsequent async turns to parent to it.
        }
    }
}
