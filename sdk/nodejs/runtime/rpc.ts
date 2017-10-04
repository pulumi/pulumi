// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as asset from "../asset";
import { ComputedValue, ComputedValues, Resource } from "../resource";
import { errorString, debuggablePromise } from "./debuggable";
import { Log } from "./log";
import { excessiveDebugOutput, options } from "./settings";

let gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

/**
 * PropertyTransfer is the result of transferring all properties.
 */
export interface PropertyTransfer {
    obj: any; // the bag of input properties after awaiting them.
    resolvers: {[key: string]: ((v: any) => void)}; // a map of resolvers for output properties that will resolve.
}

/**
 * transferProperties stores the properties on the resource object and returns a gRPC serializable
 * proto.google.protobuf.Struct out of a resource's properties.
 */
export function transferProperties(onto: any | undefined, label: string, props: ComputedValues | undefined,
    dependsOn: Resource[] | undefined): Promise<PropertyTransfer> {
    // First set up an array of all promises that we will await on before completing the transfer.
    let eventuals: Promise<any>[] = [];

    // If the dependsOn array is present, make sure we wait on those.
    if (dependsOn) {
        for (let dep of dependsOn) {
            eventuals.push(dep.id);
        }
    }

    // Set up an object that will hold the serialized object properties and then serialize them.
    let obj: any = {};
    let resolvers: {[key: string]: ((v: any) => void)} = {};
    if (props) {
        for (let k of Object.keys(props)) {
            // Skip "id" and "urn", since we handle those specially.
            if (k === "id" || k === "urn") {
                continue;
            }

            // Create a property to wrap the value and store it on the resource.
            if (onto) {
                if (onto.hasOwnProperty(k)) {
                    throw new Error(`Property '${k}' is already initialized on target '${label}`);
                }
                onto[k] =
                    debuggablePromise(new Promise<any>((resolve) => { resolvers[k] = resolve; }));
            }

            // Now serialize the value and store it in our map.  This operation may return eventuals that resolve
            // after all properties have settled, and we may need to wait for them before this transfer finishes.
            if (props[k] !== undefined) {
                eventuals.push(
                    serializeProperty(props[k], label).then(
                        (v: any) => {
                            obj[k] = v;
                        },
                        (err: Error) => {
                            throw new Error(`Property '${k}' could not be serialized: ${errorString(err)}`);
                        },
                    )
                );
            }
        }
    }

    // Now return a promise that resolves when all assignments above have settled.  Note that we do not
    // use await here, because we don't actually want to block the above assignments of properties.
    return debuggablePromise(Promise.all(eventuals).then(() => {
        return {
            obj: gstruct.Struct.fromJavaScript(obj),
            resolvers: resolvers,
        };
    }));
}

/**
 * deserializeProperties fetches the raw outputs and deserializes them from a gRPC call result.
 */
export function deserializeProperties(outputsStruct: any): any {
    let props: any = {};
    let outputs: any = outputsStruct.toJavaScript();
    for (let k of Object.keys(outputs)) {
        props[k] = deserializeProperty(outputs[k]);
    }
    return props;
}

/**
 * resolveProperties takes as input a gRPC serialized proto.google.protobuf.Struct and resolves all of the
 * resource's matching properties to the values inside.
 */
export function resolveProperties(res: Resource, transfer: PropertyTransfer,
    t: string, name: string, inputs: ComputedValues | undefined, outputsStruct: any,
    stable: boolean, stables: Set<string> | undefined): void {

    // Produce a combined set of property states, starting with inputs and then applying outputs.  If the same
    // property exists in the inputs and outputs states, the output wins.
    let props: any = inputs || {};
    if (outputsStruct) {
        Object.assign(props, deserializeProperties(outputsStruct));
    }

    // Now go ahead and resolve all properties present in the inputs and outputs set.
    for (let k of Object.keys(props)) {
        // Skip "id" and "urn", since we handle those specially.
        if (k === "id" || k === "urn") {
            continue;
        }

        // Otherwise, unmarshal the value, and store it on the resource object.
        let resolve: (v: any) => void | undefined = transfer.resolvers[k];
        if (resolve === undefined) {
            // If there is no property yet, zero initialize it.  This ensures unexpected properties are
            // still made available on the object.  This isn't ideal, because any code running prior to the actual
            // resource CRUD operation can't hang computations off of it, but it's better than tossing it.
            (res as any)[k] = debuggablePromise(new Promise<any>((r) => { resolve = r; }));
        }
        try {
            // If either we are performing a real deployment, or this is a stable property value, we
            // can propagate its final value.  Otherwise, it must be undefined, since we don't know if it's final.
            if (!options.dryRun || stable || (stables && stables.has(k))) {
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
    for (let k of Object.keys(transfer.resolvers)) {
        if (!props.hasOwnProperty(k)) {
            if (!options.dryRun) {
                throw new Error(
                    `Unexpected missing property '${k}' on resource '${name}' [${t}] during final deployment`);
            }
            transfer.resolvers[k](undefined);
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
async function serializeProperty(prop: any, ctx?: string): Promise<any> {
    if (prop === undefined) {
        if (excessiveDebugOutput) {
            Log.debug(`Serialize property [${ctx}]: undefined`);
        }
        if (!options.dryRun) {
            Log.error(`Unexpected unknown property during deployment`);
        }
        return unknownComputedValue;
    }
    else if (prop === null || typeof prop === "boolean" ||
            typeof prop === "number" || typeof prop === "string") {
        if (excessiveDebugOutput) {
            Log.debug(`Serialize property [${ctx}]: primitive=${prop}`);
        }
        return prop;
    }
    else if (prop instanceof Array) {
        let elems: any[] = [];
        for (let i = 0; i < prop.length; i++) {
            if (excessiveDebugOutput) {
                Log.debug(`Serialize property [${ctx}]: array[${i}] element`);
            }
            elems.push(await serializeProperty(prop[i], `${ctx}[${i}]`));
        }
        return elems;
    }
    else if (prop instanceof Resource) {
        // Resources aren't serializable; instead, we serialize them as references to the ID property.
        if (excessiveDebugOutput) {
            Log.debug(`Serialize property [${ctx}]: resource ID`);
        }
        return serializeProperty(prop.id, `${ctx}.id`);
    }
    else if (prop instanceof asset.Asset || prop instanceof asset.Archive) {
        // Serializing an asset or archive requires the use of a magical signature key, since otherwise it would look
        // like any old weakly typed object/map when received by the other side of the RPC boundary.
        let obj: any = {
            [specialSigKey]: (prop instanceof asset.Asset ? specialAssetSig : specialArchiveSig),
        };
        for (let k of Object.keys(prop)) {
            if (excessiveDebugOutput) {
                Log.debug(`Serialize property [${ctx}]: asset.${k}`);
            }
            obj[k] = await serializeProperty((<any>prop)[k], `asset<${ctx}>.${k}`);
        }
        return obj;
    }
    else if (prop instanceof Promise) {
        // For a promise input, await the property and then serialize the result.
        if (excessiveDebugOutput) {
            Log.debug(`Serialize property [${ctx}]: promise<T>`);
        }
        return serializeProperty(await prop, `promise<${ctx}>`);
    }
    else {
        let obj: any = {};
        for (let k of Object.keys(prop)) {
            if (excessiveDebugOutput) {
                Log.debug(`Serialize property [${ctx}]: object.${k}`);
            }
            obj[k] = await serializeProperty(prop[k], `${ctx}.${k}`);
        }
        return obj;
    }
}

/**
 * deserializeProperty unpacks some special types, reversing the above process.
 */
function deserializeProperty(prop: any): any {
    if (prop === undefined) {
        throw new Error("Unexpected unknown property value");
    }
    else if (prop === null || typeof prop === "boolean" ||
            typeof prop === "number" || typeof prop === "string") {
        return prop;
    }
    else if (prop instanceof Array) {
        let elems: any[] = [];
        for (let e of prop) {
            elems.push(deserializeProperty(e));
        }
        return elems;
    }
    else {
        // We need to recognize assets and archives specially, so we can produce the right runtime objects.
        let sig: any = prop[specialSigKey];
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
                        let assets: {[name: string]: asset.Asset} = {};
                        for (let name of Object.keys(prop["assets"])) {
                            let a = deserializeProperty(prop["assets"][name]);
                            if (!(a instanceof asset.Asset)) {
                                throw new Error("Expected an AssetArchive's assets to be unmarshaled Asset objects");
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
        let obj: any = {};
        for (let k of Object.keys(prop)) {
            obj[k] = deserializeProperty(prop[k]);
        }
        return obj;
    }
}

