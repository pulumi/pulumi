// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import * as asset from "../asset";
import * as log from "../log";
import { ComputedValue, ComputedValues, CustomResource, Resource } from "../resource";
import { debuggablePromise, errorString } from "./debuggable";
import { excessiveDebugOutput, options } from "./settings";

const gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

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
export function transferProperties(
        onto: Resource, label: string, props: ComputedValues): Record<string, (v: any) => void> {

    const resolvers: Record<string, (v: any) => void> = {};
    for (const k of Object.keys(props)) {
        // Skip "id" and "urn", since we handle those specially.
        if (k === "id" || k === "urn") {
            continue;
        }

        // Create a property to wrap the value and store it on the resource.
        if (onto.hasOwnProperty(k)) {
            throw new Error(`Property '${k}' is already initialized on target '${label}`);
        }
        (<any>onto)[k] =
            debuggablePromise(
                new Promise<any>(resolve => resolvers[k] = resolve),
                `transferProperty(${label}, ${k}, ${props[k]})`);
    }

    return resolvers;
}

/**
 * serializeAllProperties walks the props object passed in, awaiting all interior promises,
 * creating a reaosnable POJO object that can be remoted over to registerResource.
 */
export async function serializeProperties(label: string, props: ComputedValues): Promise<Record<string, any>> {
    const result: Record<string, any> = {};
    for (const k of Object.keys(props)) {
        if (k !== "id" && k !== "urn" && props[k] !== undefined) {
            result[k] = await serializeProperty(props[k], `${label}.${k}`);
        }
    }

    return result;
}

/**
 * deserializeProperties fetches the raw outputs and deserializes them from a gRPC call result.
 */
export function deserializeProperties(outputsStruct: any): any {
    const props: any = {};
    const outputs: any = outputsStruct.toJavaScript();
    for (const k of Object.keys(outputs)) {
        props[k] = deserializeProperty(outputs[k]);
    }
    return props;
}

/**
 * resolveProperties takes as input a gRPC serialized proto.google.protobuf.Struct and resolves all
 * of the resource's matching properties to the values inside.
 */
export function resolveProperties(
    res: Resource, resolvers: Record<string, (v: any) => void>,
    t: string, name: string, inputs: ComputedValues, outputsStruct: any,
    stable: boolean, stables: Set<string>): void {

    // Produce a combined set of property states, starting with inputs and then applying outputs.  If the same
    // property exists in the inputs and outputs states, the output wins.
    const props: any = inputs || {};
    if (outputsStruct) {
        Object.assign(props, deserializeProperties(outputsStruct));
    }

    // Now go ahead and resolve all properties present in the inputs and outputs set.
    for (const k of Object.keys(props)) {
        // Skip "id" and "urn", since we handle those specially.
        if (k === "id" || k === "urn") {
            continue;
        }

        // Otherwise, unmarshal the value, and store it on the resource object.
        let resolve = resolvers[k];
        if (resolve === undefined) {
            // If there is no property yet, zero initialize it.  This ensures unexpected properties
            // are still made available on the object.  This isn't ideal, because any code running
            // prior to the actual resource CRUD operation can't hang computations off of it, but
            // it's better than tossing it.
            (res as any)[k] = debuggablePromise(new Promise<any>(r => resolve = r));
        }
        try {
            // If either we are performing a real deployment, or this is a stable property value, we
            // can propagate its final value.  Otherwise, it must be undefined, since we don't know
            // if it's final.
            if (!options.dryRun || stable || stables.has(k)) {
                resolve(props[k]);
            }
            else {
                resolve(undefined);
            }
        }
        catch (err) {
            throw new Error(
                `Unable to set property '${k}' on resource '${name}' [${t}]; error: ${errorString(err)}`);
        }
    }

    // Now latch all properties in case the inputs did not contain any values.  If we're doing a dry-run, we won't
    // actually propagate the provisional state, because we cannot know for sure that it is final yet.
    for (const k of Object.keys(resolvers)) {
        if (!props.hasOwnProperty(k)) {
            if (!options.dryRun) {
                throw new Error(
                    `Unexpected missing property '${k}' on resource '${name}' [${t}] during final deployment`);
            }
            resolvers[k](undefined);
        }
    }
}

/**
 * unknownComputedValue is a special value that the monitor recognizes.
 */
export const unknownComputedValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";
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
async function serializeProperty(prop: ComputedValue<any>, ctx?: string): Promise<any> {
    if (prop === undefined) {
        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: undefined`);
        }
        return unknownComputedValue;
    }
    else if (prop === null ||
             typeof prop === "boolean" ||
             typeof prop === "number" ||
             typeof prop === "string") {
        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: primitive=${prop}`);
        }
        return prop;
    }
    else if (prop instanceof Array) {
        const elems: any[] = [];
        for (let i = 0; i < prop.length; i++) {
            if (excessiveDebugOutput) {
                log.debug(`Serialize property [${ctx}]: array[${i}] element`);
            }
            elems.push(await serializeProperty(prop[i], `${ctx}[${i}]`));
        }
        return elems;
    }
    else if (prop instanceof CustomResource) {
        // Resources aren't serializable; instead, we serialize them as references to the ID property.
        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: resource ID`);
        }
        return serializeProperty(prop.id, `${ctx}.id`);
    }
    else if (prop instanceof asset.Asset || prop instanceof asset.Archive) {
        // Serializing an asset or archive requires the use of a magical signature key, since otherwise it would look
        // like any old weakly typed object/map when received by the other side of the RPC boundary.
        const obj: any = {
            [specialSigKey]: prop instanceof asset.Asset ? specialAssetSig : specialArchiveSig,
        };

        return await serializeAllKeys(prop, obj);
    }
    else if (prop instanceof Promise) {
        // For a promise input, await the property and then serialize the result.
        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: promise<T>`);
        }
        const subctx = `promise<${ctx}>`;
        return serializeProperty(
            await debuggablePromise(prop, `serializeProperty.await(${subctx})`), subctx);
    }
    else {
        return await serializeAllKeys(prop, {});
    }

    async function serializeAllKeys(innerProp: any, obj: any) {
        for (const k of Object.keys(innerProp)) {
            if (excessiveDebugOutput) {
                log.debug(`Serialize property [${ctx}]: object.${k}`);
            }
            obj[k] = await serializeProperty(innerProp[k], `${ctx}.${k}`);
        }

        return obj;
    }
}

/**
 * deserializeProperty unpacks some special types, reversing the above process.
 */
function deserializeProperty(prop: any): any {
    if (prop === undefined) {
        return undefined;
    }
    else if (prop === null || typeof prop === "boolean" || typeof prop === "number") {
        return prop;
    }
    else if (typeof prop === "string") {
        if (prop === unknownComputedValue) {
            return undefined;
        }
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
                        const assets: Record<string, asset.Asset> = {};
                        for (const name of Object.keys(prop["assets"])) {
                            const a = deserializeProperty(prop["assets"][name]);
                            if (!(a instanceof asset.Asset) && !(a instanceof asset.Archive)) {
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
