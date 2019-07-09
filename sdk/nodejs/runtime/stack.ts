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
import { getProject, getStack } from "../metadata";
import { Inputs, Output, output, secret } from "../output";
import { ComponentResource, Resource } from "../resource";
import { getRootResource, isQueryMode, setRootResource } from "./settings";

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
    if (!isQueryMode()) {
        const stack = new Stack(init);
        return stack.outputs.promise();
    } else {
        return Promise.resolve(init());
    }
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

    constructor(init: () => any) {
        super(rootPulumiStackTypeName, `${getProject()}-${getStack()}`);
        this.outputs = output(this.runInit(init));
    }

    /**
     * runInit invokes the given init callback with this resource set as the root resource. The return value of init is
     * used as the stack's output properties.
     *
     * @param init The callback to run in the context of this Pulumi stack
     */
    private async runInit(init: () => any): Promise<any> {
        const parent = await getRootResource();
        if (parent) {
            throw new Error("Only one root Pulumi Stack may be active at once");
        }

        await setRootResource(this);
        let outputs: any;
        try {
            outputs = init();
        } finally {
            // We want to expose stack outputs as simple pojo objects (including Resources).  This
            // helps ensure that outputs can point to resources, and that that is stored and
            // presented as something reasonable, and not as just an id/urn in the case of
            // Resources.
            super.registerOutputs(massage(outputs, new Set()));
        }

        return outputs;
    }
}

async function massage(prop: any, seenObjects: Set<any>): Promise<any> {
    if (prop === undefined ||
        prop === null ||
        typeof prop === "boolean" ||
        typeof prop === "number" ||
        typeof prop === "string") {

        return prop;
    }

    if (prop instanceof Promise) {
        return await massage(await prop, seenObjects);
    }

    if (Output.isInstance(prop)) {
        // If the output itself is a secret, we don't want to lose the secretness by returning the underlying
        // value. So instead, we massage the underlying value and then wrap it back up in an Output which is
        // marked as secret.
        const isSecret = await (prop.isSecret || Promise.resolve(false));
        const value = await massage(await prop.promise(), seenObjects);

        if (isSecret) {
            return secret(value);
        }

        return value;
    }

    // from this point on, we have complex objects.  If we see them again, we don't want to emit
    // them again fully or else we'd loop infinitely.
    if (seenObjects.has(prop)) {
        // Note: for Resources we hit again, emit their urn so cycles can be easily understood
        // in the pojo objects.
        if (Resource.isInstance(prop)) {
            return await massage(prop.urn, seenObjects);
        }

        return undefined;
    }

    seenObjects.add(prop);

    if (asset.Asset.isInstance(prop)) {
        if ((<asset.FileAsset>prop).path !== undefined) {
            return { path: (<asset.FileAsset>prop).path };
        }
        else if ((<asset.RemoteAsset>prop).uri !== undefined) {
            return { uri: (<asset.RemoteAsset>prop).uri };
        }
        else if ((<asset.StringAsset>prop).text !== undefined) {
            return { text: "..." };
        }

        return undefined;
    }

    if (asset.Archive.isInstance(prop)) {
        if ((<asset.AssetArchive>prop).assets) {
            return { assets: massage((<asset.AssetArchive>prop).assets, seenObjects) };
        }
        else if ((<asset.FileArchive>prop).path !== undefined) {
            return { path: (<asset.FileArchive>prop).path };
        }
        else if ((<asset.RemoteArchive>prop).uri !== undefined) {
            return { uri: (<asset.RemoteArchive>prop).uri };
        }

        return undefined;
    }

    if (Resource.isInstance(prop)) {
        // Emit a resource as a normal pojo.  But filter out all our internal properties so that
        // they don't clutter the display/checkpoint with values not relevant to the application.
        return serializeAllKeys(n => !n.startsWith("__"));
    }

    if (prop instanceof Array) {
        const result = [];
        for (let i = 0; i < prop.length; i++) {
            result[i] = await massage(prop[i], seenObjects);
        }

        return result;
    }

    return await serializeAllKeys(n => true);

    async function serializeAllKeys(include: (name: string) => boolean) {
        const obj: Record<string, any> = {};
        for (const k of Object.keys(prop)) {
            if (include(k)) {
                obj[k] = await massage(prop[k], seenObjects);
            }
        }

        return obj;
    }
}
