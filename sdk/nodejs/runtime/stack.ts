// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as log from "../log";
import { getProject, getStack } from "../metadata";
import { ComponentResource, ComputedValues, Resource } from "../resource";

let rootPulumiStack: Resource | undefined;

/**
 * getRootPulumiStack returns a root stack that will be used automatically as resource parents.  This ensures that all
 * resources without explicit parents are parented to a common stack type.
 */
export function getRootPulumiStack(): Resource | undefined {
    return rootPulumiStack;
}

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
    constructor(init: () => ComputedValues) {
        super(rootPulumiStackTypeName, `${getProject()}-${getStack()}`);

        if (rootPulumiStack) {
            throw new Error("Only one root Pulumi Stack may be active at once");
        }
        try {
            rootPulumiStack = this;       // install ourselves as the current root.
            const outputs = init();       // run the init code.
            super.recordOutputs(outputs); // save the outputs for this component to whatever the init returned.

            // TODO[pulumi/pulumi#340]: until output properties for components are working again, just print them out.
            for (const key of Object.keys(outputs)) {
                const value = outputs[key];
                (async () => {
                    const v: any | undefined = await value;
                    if (v !== undefined) {
                        log.info(`stack output: ${key}: ${v}`);
                    }
                })();
            }
        }
        finally {
            rootPulumiStack = undefined;
        }
    }
}
