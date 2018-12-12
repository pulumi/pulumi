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

import * as assert from "assert";
import * as asset from "../asset";
import * as log from "../log";
import { ComponentResource, CustomResource, Input, Inputs, Output, Resource } from "../resource";
import { debuggablePromise, errorString } from "./debuggable";
import { excessiveDebugOutput, isDryRun } from "./settings";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

export type OutputResolvers = Record<string, (value: any, isStable: boolean) => void>;

/**
 * transferProperties mutates the 'onto' resource so that it has Promise-valued properties for all
 * the 'props' input/output props.  *Importantly* all these promises are completely unresolved. This
 * is because we don't want anyone to observe the values of these properties until the rpc call to
 * registerResource actually returns.  This is because the registerResource call may actually
 * override input values, and we only want people to see the final value.
 *
 * The result of this call (beyond the stateful changes to 'onto') is the set of Promise resolvers
 * that will be called post-RPC call.  When the registerResource RPC call comes back, the values
 * that the engine actualy produced will be used to resolve all the unresolved promised placed on
 * 'onto'.
 */
export function transferProperties(onto: Resource, label: string, props: Inputs): OutputResolvers {
    const resolvers: OutputResolvers = {};
    for (const k of Object.keys(props)) {
        // Skip "id" and "urn", since we handle those specially.
        if (k === "id" || k === "urn") {
            continue;
        }

        // Create a property to wrap the value and store it on the resource.
        if (onto.hasOwnProperty(k)) {
            throw new Error(`Property '${k}' is already initialized on target '${label}`);
        }

        let resolveValue: (v: any) => void;
        let resolveIsKnown: (v: boolean) => void;

        resolvers[k] = (v: any, isKnown: boolean) => {
            resolveValue(v);
            resolveIsKnown(isKnown);
        };

        (<any>onto)[k] = new Output(
            onto,
            debuggablePromise(
                new Promise<any>(resolve => resolveValue = resolve),
                `transferProperty(${label}, ${k}, ${props[k]})`),
            debuggablePromise(
                new Promise<boolean>(resolve => resolveIsKnown = resolve),
                `transferIsStable(${label}, ${k}, ${props[k]})`));
    }

    return resolvers;
}

/**
 * serializeFilteredProperties walks the props object passed in, awaiting all interior promises for properties with
 * keys that match the provided filter, creating a reasonable POJO object that can be remoted over to
 * registerResource.
 */
async function serializeFilteredProperties(
        label: string, props: Inputs, acceptKey: (k: string) => boolean,
        dependentResources: Resource[] = []): Promise<Record<string, any>> {
    const result: Record<string, any> = {};
    for (const k of Object.keys(props)) {
        if (acceptKey(k)) {
            // We treat properties with undefined values as if they do not exist.
            const v = await serializeProperty(`${label}.${k}`, props[k], dependentResources);
            if (v !== undefined) {
                result[k] = v;
            }
        }
    }

    return result;
}

/**
 * serializeResourceProperties walks the props object passed in, awaiting all interior promises besides those for `id`
 * and `urn`, creating a reasonable POJO object that can be remoted over to registerResource.
 */
export async function serializeResourceProperties(
        label: string, props: Inputs, dependentResources: Resource[] = []): Promise<Record<string, any>> {
    return serializeFilteredProperties(label, props, key => key !== "id" && key !== "urn", dependentResources);
}

/**
 * serializeProperties walks the props object passed in, awaiting all interior promises, creating a reasonable
 * POJO object that can be remoted over to registerResource.
 */
export async function serializeProperties(
        label: string, props: Inputs, dependentResources: Resource[] = []): Promise<Record<string, any>> {
    return serializeFilteredProperties(label, props, key => true, dependentResources);
}

/**
 * deserializeProperties fetches the raw outputs and deserializes them from a gRPC call result.
 */
export function deserializeProperties(outputsStruct: any): any {
    const props: any = {};
    const outputs: any = outputsStruct.toJavaScript();
    for (const k of Object.keys(outputs)) {
        // We treat properties with undefined values as if they do not exist.
        if (outputs[k] !== undefined) {
            props[k] = deserializeProperty(outputs[k]);
        }
    }
    return props;
}

/**
 * resolveProperties takes as input a gRPC serialized proto.google.protobuf.Struct and resolves all
 * of the resource's matching properties to the values inside.
 *
 * NOTE: it is imperative that the properties in `allProps` were produced by `deserializeProperties` in order for
 * output properties to work correctly w.r.t. knowns/unknowns: this function assumes that any undefined value in
 * `allProps`represents an unknown value that was returned by an engine operation.
 */
export function resolveProperties(
    res: Resource, resolvers: Record<string, (v: any, isKnown: boolean) => void>,
    t: string, name: string, allProps: any): void {

    // Now go ahead and resolve all properties present in the inputs and outputs set.
    for (const k of Object.keys(allProps)) {
        // Skip "id" and "urn", since we handle those specially.
        if (k === "id" || k === "urn") {
            continue;
        }

        // Otherwise, unmarshal the value, and store it on the resource object.
        const resolve = resolvers[k];

        if (resolve === undefined) {
            // engine returned a property that was not in our initial property-map.  This can happen
            // for outputs that were registered through direct calls to 'registerOutputs'. We do
            // *not* want to do anything with these returned properties.  First, the component
            // resources that were calling 'registerOutputs' will have already assigned these fields
            // directly on them themselves.  Second, if we were to try to assign here we would have
            // an incredibly bad race condition for two reasons:
            //
            //  1. This call to 'resolveProperties' happens asynchronously at some point far after
            //     the resource was constructed.  So the user will have been able to observe the
            //     initial value up until we get to this point.
            //
            //  2. The component resource will have often assigned a value of some arbitrary type
            //     (say, a 'string').  If we overwrite this with an `Output<string>` we'll be changing
            //     the type at some non-deterministic point in the future.
            continue;
        }

        try {
            // If either we are performing a real deployment, or this is a stable property value, we
            // can propagate its final value.  Otherwise, it must be undefined, since we don't know
            // if it's final.
            if (!isDryRun()) {
                // normal 'pulumi update'.  resolve the output with the value we got back
                // from the engine.  That output can always run its .apply calls.
                resolve(allProps[k], true);
            }
            else {
                // We're previewing. If the engine was able to give us a reasonable value back,
                // then use it. Otherwise, inform the Output that the value isn't known.
                const value = allProps[k];
                const isKnown = value !== undefined;
                resolve(value, isKnown);
            }
        }
        catch (err) {
            throw new Error(
                `Unable to set property '${k}' on resource '${name}' [${t}]; error: ${errorString(err)}`);
        }
    }

    // `allProps` may not have contained a value for every resolver: for example, optional outputs may not be present.
    // We will resolve all of these values as `undefined`, and will mark the value as known if we are not running a
    // preview.
    for (const k of Object.keys(resolvers)) {
        if (!allProps.hasOwnProperty(k)) {
            const resolve = resolvers[k];
            resolve(undefined, !isDryRun());
        }
    }
}

/**
 * Unknown values are encoded as a distinguished string value.
 */
export const unknownValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";
/**
 * specialSigKey is sometimes used to encode type identity inside of a map.  See pkg/resource/properties.go.
 */
export const specialSigKey = "4dabf18193072939515e22adb298388d";
/**
 * specialAssetSig is a randomly assigned hash used to identify assets in maps.  See pkg/resource/asset.go.
 */
export const specialAssetSig = "c44067f5952c0a294b673a41bacd8c17";
/**
 * specialArchiveSig is a randomly assigned hash used to identify archives in maps.  See pkg/resource/asset.go.
 */
export const specialArchiveSig = "0def7320c3a5731c473e5ecbe6d01bc7";

/**
 * serializeProperty serializes properties deeply.  This understands how to wait on any unresolved promises, as
 * appropriate, in addition to translating certain "special" values so that they are ready to go on the wire.
 */
export function serializeProperty(ctx: string, prop: Input<any>, dependentResources: Resource[]): Promise<any> {
    return serializePropertyWorker(ctx, prop, dependentResources, new Set());
}

async function serializePropertyWorker(
    ctx: string, prop: Input<any>,
    dependentResources: Resource[],
    seenObjects: Set<any>): Promise<any> {

    // Simple values, always serialize fully.
    if (prop === undefined ||
        prop === null ||
        typeof prop === "boolean" ||
        typeof prop === "number" ||
        typeof prop === "string") {

        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: primitive=${prop}`);
        }
        return prop;
    }

    if (asset.Asset.isInstance(prop) || asset.Archive.isInstance(prop)) {
        // Serializing an asset or archive requires the use of a magical signature key, since otherwise it would look
        // like any old weakly typed object/map when received by the other side of the RPC boundary.
        const obj: any = {
            [specialSigKey]: asset.Asset.isInstance(prop) ? specialAssetSig : specialArchiveSig,
        };

        return await serializeAllKeys(prop, obj);
    }

    if (prop instanceof Promise) {
        // For a promise input, await the property and then serialize the result.
        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: Promise<T>`);
        }

        const subctx = `Promise<${ctx}>`;
        return serializePropertyWorker(subctx,
            await debuggablePromise(prop, `serializeProperty.await(${subctx})`),
            dependentResources, seenObjects);
    }

    if (Output.isInstance(prop)) {
        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: Output<T>`);
        }

        dependentResources.push(...(await prop.__resources));

        // When serializing an Output, we will either serialize it as its resolved value or the "unknown value"
        // sentinel. We will do the former for all outputs created directly by user code (such outputs always
        // resolve isKnown to true) and for any resource outputs that were resolved with known values.
        const isKnown = await prop.isKnown;
        const value = await serializePropertyWorker(
            `${ctx}.id`, prop.promise(), dependentResources, seenObjects);
        return isKnown ? value : unknownValue;
    }

    if (CustomResource.isInstance(prop)) {
        // Resources aren't serializable; instead, we serialize them as references to the ID property.
        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: custom resource id`);
        }

        dependentResources.push(prop);
        return serializePropertyWorker(`${ctx}.id`, prop.id, dependentResources, seenObjects);
    }

    if (ComponentResource.isInstance(prop)) {
        // Component resources often can contain cycles in them.  For example, an awsinfra
        // SecurityGroupRule can point a the awsinfra SecurityGroup, which in turn can point back to
        // its rules through its `egressRules` and `ingressRules` properties.  If serializing out
        // the `SecurityGroup` resource ends up trying to serialize out those properties, a deadlock
        // will happen, due to waiting on the child, which is waiting on the parent.
        //
        // Practically, there is no need to actually serialize out a component.  It doesn't represent
        // a real resource, nor does it have normal properties that need to be tracked for differences
        // (since changes to its properties don't represent changes to resources in the real world).
        //
        // So, to avoid these problems, while allowing a flexible and simple programming model, we
        // just serialize out the component as its urn.  This allows the component to be identified
        // and tracked in a reasonable manner, while not causing us to compute or embed information
        // about it that is not needed, and which can lead to deadlocks.
        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: component resource urnid`);
        }

        return serializePropertyWorker(`${ctx}.urn`, prop.urn, dependentResources, seenObjects);
    }

    // We're now getting to complex objects where we are recursing into them.  Prevent infinite
    // recursion if we've already seen this object before.
    if (seenObjects.has(prop)) {
        return undefined;
    }

    seenObjects.add(prop);

    if (prop instanceof Array) {
        const result: any[] = [];
        for (let i = 0; i < prop.length; i++) {
            if (excessiveDebugOutput) {
                log.debug(`Serialize property [${ctx}]: array[${i}] element`);
            }

            // When serializing arrays, we serialize any undefined values as `null`. This matches
            // JSON semantics.
            const elem = await serializePropertyWorker(
                `${ctx}[${i}]`, prop[i], dependentResources, seenObjects);
            result.push(elem === undefined ? null : elem);
        }
        return result;
    }

    return await serializeAllKeys(prop, {});

    async function serializeAllKeys(innerProp: any, obj: any) {
        for (const k of Object.keys(innerProp)) {
            if (excessiveDebugOutput) {
                log.debug(`Serialize property [${ctx}]: object.${k}`);
            }

            // When serializing an object, we omit any keys with undefined values. This matches JSON semantics.
            const v = await serializePropertyWorker(
                `${ctx}.${k}`, innerProp[k], dependentResources, seenObjects);
            if (v !== undefined) {
                obj[k] = v;
            }
        }

        return obj;
    }
}

/**
 * deserializeProperty unpacks some special types, reversing the above process.
 */
export function deserializeProperty(prop: any): any {
    if (prop === undefined) {
        throw new Error("unexpected undefined property value during deserialization");
    }
    else if (prop === unknownValue) {
        return undefined;
    }
    else if (prop === null || typeof prop === "boolean" || typeof prop === "number" || typeof prop === "string") {
        return prop;
    }
    else if (prop instanceof Array) {
        const elems: any[] = [];
        for (const e of prop) {
            elems.push(deserializeProperty(e));
        }
        return elems;
    }
    else {
        // We need to recognize assets and archives specially, so we can produce the right runtime objects.
        const sig: any = prop[specialSigKey];
        if (sig) {
            switch (sig) {
                case specialAssetSig:
                    if (prop["path"]) {
                        return new asset.FileAsset(<string>prop["path"]);
                    }
                    else if (prop["text"]) {
                        return new asset.StringAsset(<string>prop["text"]);
                    }
                    else if (prop["uri"]) {
                        return new asset.RemoteAsset(<string>prop["uri"]);
                    }
                    else {
                        throw new Error("Invalid asset encountered when unmarshaling resource property");
                    }
                case specialArchiveSig:
                    if (prop["assets"]) {
                        const assets: asset.AssetMap = {};
                        for (const name of Object.keys(prop["assets"])) {
                            const a = deserializeProperty(prop["assets"][name]);
                            if (!(asset.Asset.isInstance(a)) && !(asset.Archive.isInstance(a))) {
                                throw new Error(
                                    "Expected an AssetArchive's assets to be unmarshaled Asset or Archive objects");
                            }
                            assets[name] = a;
                        }
                        return new asset.AssetArchive(assets);
                    }
                    else if (prop["path"]) {
                        return new asset.FileArchive(<string>prop["path"]);
                    }
                    else if (prop["uri"]) {
                        return new asset.RemoteArchive(<string>prop["uri"]);
                    }
                    else {
                        throw new Error("Invalid archive encountered when unmarshaling resource property");
                    }
                default:
                    throw new Error(`Unrecognized signature '${sig}' when unmarshaling resource property`);
            }
        }

        // If there isn't a signature, it's not a special type, and we can simply return the object as a map.
        const obj: any = {};
        for (const k of Object.keys(prop)) {
            obj[k] = deserializeProperty(prop[k]);
        }
        return obj;
    }
}
