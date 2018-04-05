// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import * as asset from "../asset";
import * as log from "../log";
import { CustomResource, Input, Inputs, Output, Resource } from "../resource";
import { debuggablePromise, errorString } from "./debuggable";
import { excessiveDebugOutput, options } from "./settings";

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
        let resolvePerformApply: (v: boolean) => void;

        resolvers[k] = (v: any, performApply: boolean) => {
            resolveValue(v);
            resolvePerformApply(performApply);
        };

        (<any>onto)[k] = Output.create(
            onto,
            debuggablePromise(
                new Promise<any>(resolve => resolveValue = resolve),
                `transferProperty(${label}, ${k}, ${props[k]})`),
            debuggablePromise(
                new Promise<boolean>(resolve => resolvePerformApply = resolve),
                `transferIsStable(${label}, ${k}, ${props[k]})`));
    }

    return resolvers;
}

/**
 * serializeAllProperties walks the props object passed in, awaiting all interior promises,
 * creating a reaosnable POJO object that can be remoted over to registerResource.
 */
export async function serializeProperties(
        label: string, props: Inputs, dependentResources: Resource[] = []): Promise<Record<string, any>> {
    const result: Record<string, any> = {};
    for (const k of Object.keys(props)) {
        if (k !== "id" && k !== "urn" && props[k] !== undefined) {
            result[k] = await serializeProperty(`${label}.${k}`, props[k], dependentResources);
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
    res: Resource, resolvers: Record<string, (v: any, performApply: boolean) => void>,
    t: string, name: string, allProps: any): void {

    // Now go ahead and resolve all properties present in the inputs and outputs set.
    for (const k of Object.keys(allProps)) {
        // Skip "id" and "urn", since we handle those specially.
        if (k === "id" || k === "urn") {
            continue;
        }

        // Otherwise, unmarshal the value, and store it on the resource object.
        let resolve = resolvers[k];

        if (resolve === undefined) {
            let resolveValue: (v: any) => void;
            let resolvePerformApply: (v: boolean) => void;

            resolve = (v, performApply) => {
                resolveValue(v);
                resolvePerformApply(performApply);
            };

            // If there is no property yet, zero initialize it.  This ensures unexpected properties
            // are still made available on the object.  This isn't ideal, because any code running
            // prior to the actual resource CRUD operation can't hang computations off of it, but
            // it's better than tossing it.
            (res as any)[k] = Output.create(
                res,
                debuggablePromise(new Promise<any>(r => resolveValue = r)),
                debuggablePromise(new Promise<boolean>(r => resolvePerformApply = r)));
        }

        try {
            // If either we are performing a real deployment, or this is a stable property value, we
            // can propagate its final value.  Otherwise, it must be undefined, since we don't know
            // if it's final.
            if (!options.dryRun) {
                // normal 'pulumi update'.  resolve the output with the value we got back
                // from the engine.  That output can always run its .apply calls.
                resolve(allProps[k], true);
            }
            else {
                // We're previewing.  If the engine was able to give us a reasonable value back,
                // then use it.  Otherwise, let the Output know that the value isn't known and it
                // shoudl not use it when performing .apply calls.

                const value = allProps[k];
                const performApply = value !== undefined;
                resolve(value, performApply);
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
        if (!allProps.hasOwnProperty(k)) {
            if (!options.dryRun) {
                throw new Error(
                    `Unexpected missing property '${k}' on resource '${name}' [${t}] during final deployment`);
            }

            const resolve = resolvers[k];
            resolve(undefined, false);
        }
    }
}

/**
 * Protobuf js doesn't like undefined values.  so we just encode as a string
 */
export const undefinedValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";
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
export async function serializeProperty(ctx: string, prop: Input<any>, dependentResources: Resource[]): Promise<any> {
    if (prop === undefined) {
        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: undefined`);
        }
        return undefinedValue;
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
            elems.push(await serializeProperty(`${ctx}[${i}]`, prop[i], dependentResources));
        }
        return elems;
    }
    else if (prop instanceof CustomResource) {
        // Resources aren't serializable; instead, we serialize them as references to the ID property.
        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: resource ID`);
        }

        dependentResources.push(prop);
        return serializeProperty(`${ctx}.id`, prop.id, dependentResources);
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
            log.debug(`Serialize property [${ctx}]: Promise<T>`);
        }
        const subctx = `Promise<${ctx}>`;
        return serializeProperty(subctx,
            await debuggablePromise(prop, `serializeProperty.await(${subctx})`), dependentResources);
    }
    else if (prop instanceof Output) {
        if (excessiveDebugOutput) {
            log.debug(`Serialize property [${ctx}]: Dependency<T>`);
        }

        dependentResources.push(...prop.resources());
        return await serializeProperty(`${ctx}.id`, prop.promise(), dependentResources);
    } else {
        return await serializeAllKeys(prop, {});
    }

    async function serializeAllKeys(innerProp: any, obj: any) {
        for (const k of Object.keys(innerProp)) {
            if (excessiveDebugOutput) {
                log.debug(`Serialize property [${ctx}]: object.${k}`);
            }
            obj[k] = await serializeProperty(`${ctx}.${k}`, innerProp[k], dependentResources);
        }

        return obj;
    }
}

/**
 * deserializeProperty unpacks some special types, reversing the above process.
 */
export function deserializeProperty(prop: any): any {
    if (prop === undefined || prop === undefinedValue) {
        return undefined;
    }
    else if (prop === null || typeof prop === "boolean" || typeof prop === "number") {
        return prop;
    }
    else if (typeof prop === "string") {
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
