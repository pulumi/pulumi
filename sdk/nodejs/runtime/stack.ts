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

import * as asset from "../asset";
import { warn } from "../log";
import { getProject, getStack } from "../metadata";
import { Inputs, Output, output } from "../output";
import { ComponentResource, Resource, ResourceTransform, ResourceTransformation } from "../resource";
import { InvokeTransform } from "../invoke";
import { getCallbacks, isDryRun, setRootResource } from "./settings";
import { getStore, setStackResource, getStackResource as stateGetStackResource } from "./state";

/**
 * The type name that should be used to construct the root component in the tree
 * of Pulumi resources allocated by a deployment. This must be kept up to date
 * with `github.com/pulumi/pulumi/sdk/v3/go/common/resource/stack.RootStackType`.
 */
export const rootPulumiStackTypeName = "pulumi:pulumi:Stack";

/**
 * Creates a new Pulumi stack resource and executes the callback inside of it.
 * Any outputs returned by the callback will be stored as output properties on
 * this resulting Stack object.
 */
export function runInPulumiStack(init: () => Promise<any>): Promise<Inputs | undefined> {
    const stack = new Stack(init);
    return stack.outputs.promise();
}

/**
 * {@link Stack} is the root resource for a Pulumi stack. Before invoking the
 * `init` callback, it registers itself as the root resource with the Pulumi
 * engine.
 */
export class Stack extends ComponentResource<Inputs> {
    /**
     * The outputs of this stack, if the `init` callback exited normally.
     */
    public readonly outputs: Output<Inputs>;

    constructor(init: () => Promise<Inputs>) {
        // Clear the stackResource so that when the Resource constructor runs it gives us no parent, we'll
        // then set this to ourselves in init before calling the user code that will then create other
        // resources.
        setStackResource(undefined);
        super(rootPulumiStackTypeName, `${getProject()}-${getStack()}`, { init });
        const data = this.getData();
        this.outputs = output(data);
    }

    /**
     * Invokes the given `init` callback with this resource set as the root
     * resource. The return value of init is used as the stack's output
     * properties.
     *
     * @param args.init
     *  The callback to run in the context of this Pulumi stack
     */
    async initialize(args: { init: () => Promise<Inputs> }): Promise<Inputs> {
        await setRootResource(this);

        // Set the global reference to the stack resource before invoking this init() function
        setStackResource(this);

        let outputs: Inputs | undefined;
        try {
            const inputs = await args.init();
            outputs = await massage(undefined, inputs, []);
        } finally {
            // We want to expose stack outputs as simple pojo objects (including Resources).  This
            // helps ensure that outputs can point to resources, and that that is stored and
            // presented as something reasonable, and not as just an id/urn in the case of
            // Resources.
            super.registerOutputs(outputs);
        }

        return outputs!;
    }
}

async function massage(key: string | undefined, prop: any, objectStack: any[]): Promise<any> {
    if (prop === undefined && objectStack.length === 1) {
        // This is a top level undefined value, it will not show up in stack outputs, warn the user about
        // this.
        warn(`Undefined value (${key}) will not show as a stack output.`);
        return undefined;
    }

    if (
        prop === undefined ||
        prop === null ||
        typeof prop === "boolean" ||
        typeof prop === "number" ||
        typeof prop === "string"
    ) {
        return prop;
    }

    if (prop instanceof Promise) {
        return await massage(key, await prop, objectStack);
    }

    if (Output.isInstance(prop)) {
        const result = prop.apply((v) => massage(key, v, objectStack));
        // explicitly await the underlying promise of the output here.  This is necessary to get a
        // deterministic walk of the object graph.  We need that deterministic walk, otherwise our
        // actual cycle detection logic (using 'objectStack') doesn't work.  i.e. if we don't do
        // this then the main walking logic will be interleaved with the async function this output
        // is executing.  This interleaving breaks out assumption about pushing/popping values onto
        // objectStack'
        await result.promise();
        return result;
    }

    // from this point on, we have complex objects.  If we see them again, we don't want to emit
    // them again fully or else we'd loop infinitely.
    if (objectStack.indexOf(prop) >= 0) {
        // Note: for Resources we hit again, emit their urn so cycles can be easily understood
        // in the pojo objects.
        if (Resource.isInstance(prop)) {
            return await massage(key, prop.urn, objectStack);
        }

        return undefined;
    }

    try {
        // push and pop what we see into a stack.  That way if we see the same object through
        // different paths, we will still print it out.  We only skip it if it would truly cause
        // recursion.
        objectStack.push(prop);
        return await massageComplex(prop, objectStack);
    } finally {
        const popped = objectStack.pop();
        if (popped !== prop) {
            throw new Error("Invariant broken when processing stack outputs");
        }
    }
}

async function massageComplex(prop: any, objectStack: any[]): Promise<any> {
    if (asset.Asset.isInstance(prop)) {
        if ((<asset.FileAsset>prop).path !== undefined) {
            return { path: (<asset.FileAsset>prop).path };
        } else if ((<asset.RemoteAsset>prop).uri !== undefined) {
            return { uri: (<asset.RemoteAsset>prop).uri };
        } else if ((<asset.StringAsset>prop).text !== undefined) {
            return { text: "..." };
        }

        return undefined;
    }

    if (asset.Archive.isInstance(prop)) {
        if ((<asset.AssetArchive>prop).assets) {
            return { assets: await massage("assets", (<asset.AssetArchive>prop).assets, objectStack) };
        } else if ((<asset.FileArchive>prop).path !== undefined) {
            return { path: (<asset.FileArchive>prop).path };
        } else if ((<asset.RemoteArchive>prop).uri !== undefined) {
            return { uri: (<asset.RemoteArchive>prop).uri };
        }

        return undefined;
    }

    if (Resource.isInstance(prop)) {
        // Emit a resource as a normal pojo.  But filter out all our internal properties so that
        // they don't clutter the display/checkpoint with values not relevant to the application.
        //
        // In preview only, we mark the POJO with "@isPulumiResource" to indicate that it is derived
        // from a resource. This allows the engine to perform resource-specific filtering of unknowns
        // from output diffs during a preview. This filtering is not necessary during an update because
        // all property values are known.
        const pojo = await serializeAllKeys((n) => !n.startsWith("__"));
        return !isDryRun() ? pojo : { ...pojo, "@isPulumiResource": true };
    }

    if (prop instanceof Array) {
        const result = [];
        for (let i = 0; i < prop.length; i++) {
            result[i] = await massage(undefined, prop[i], objectStack);
        }

        return result;
    }

    return await serializeAllKeys((n) => true);

    async function serializeAllKeys(include: (name: string) => boolean) {
        const obj: Record<string, any> = {};
        for (const k of Object.keys(prop)) {
            if (include(k)) {
                obj[k] = await massage(k, prop[k], objectStack);
            }
        }

        return obj;
    }
}

/**
 * Add a transformation to all future resources constructed in this Pulumi
 * stack.
 */
export function registerStackTransformation(t: ResourceTransformation) {
    const stackResource = getStackResource();
    if (!stackResource) {
        throw new Error("The root stack resource was referenced before it was initialized.");
    }
    stackResource.__transformations = [...(stackResource.__transformations || []), t];
}

/**
 * Add a transformation to all future resources constructed in this Pulumi
 * stack.
 */
export function registerResourceTransform(t: ResourceTransform): void {
    if (!getStore().supportsTransforms) {
        throw new Error("The Pulumi CLI does not support transforms. Please update the Pulumi CLI");
    }
    const callbacks = getCallbacks();
    if (!callbacks) {
        throw new Error("No callback server registered.");
    }
    callbacks.registerStackTransform(t);
}

/**
 * Add a transformation to all future resources constructed in this Pulumi
 * stack.
 *
 * @deprecated
 *  Use `registerResourceTransform` instead.
 */
export function registerStackTransform(t: ResourceTransform) {
    registerResourceTransform(t);
}

/**
 * Add a transformation to all future invoke calls in this Pulumi stack.
 */
export function registerInvokeTransform(t: InvokeTransform): void {
    if (!getStore().supportsInvokeTransforms) {
        throw new Error("The Pulumi CLI does not support transforms. Please update the Pulumi CLI");
    }
    const callbacks = getCallbacks();
    if (!callbacks) {
        throw new Error("No callback server registered.");
    }
    callbacks.registerStackInvokeTransform(t);
}

export function getStackResource(): Stack | undefined {
    return stateGetStackResource();
}
