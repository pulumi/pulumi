// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as asset from "../asset";
import { ID, ComputedValue, ComputedValues, Resource, URN } from "../resource";
import { errorString, debuggablePromise } from "./debuggable";
import { Log } from "./log";
import { getMonitor, options, rpcKeepAlive, serialize } from "./settings";

let langproto = require("../proto/languages_pb.js");
let gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

/**
 * excessiveDebugOutput enables, well, pretty excessive debug output pertaining to resources and properties.
 */
let excessiveDebugOutput: boolean = false;

/**
 * resourceChain is used to serialize all resource requests.  If we don't do this, all resource operations will be
 * entirely asynchronous, meaning the dataflow graph that results will determine ordering of operations.  This
 * causes problems with some resource providers, so for now we will serialize all of them.  The issue
 * pulumi/pulumi#335 tracks coming up with a long-term solution here.
 */
let resourceChain: Promise<void> = Promise.resolve();

/**
 * registerResource registers a new resource object with a given type t and name.  It returns the auto-generated URN
 * and the ID that will resolve after the deployment has completed.  All properties will be initialized to property
 * objects that the registration operation will resolve at the right time (or remain unresolved for deployments).
 */
export function registerResource(res: Resource, t: string, name: string, props: ComputedValues | undefined,
    dependsOn: Resource[] | undefined): void {
    Log.debug(
        `Registering resource: t=${t}, name=${name}` +
        excessiveDebugOutput ? `, props=${JSON.stringify(props)}` : ``);

    // Pre-allocate an error so we have a clean stack to print even if an asynchronous operation occurs.
    let createError: Error = new Error(`Resouce '${name}' [${t}] could not be created`);

    // Store a URN and ID property, plus any passed in, on the resource object.  Note that we do these using any
    // casts because they are typically readonly and this function is in cahoots with the initialization process.
    let resolveURN: (v: URN | undefined) => void;
    (<any>res).urn = debuggablePromise(new Promise<URN | undefined>((resolve) => { resolveURN = resolve; }));
    let resolveID: (v: ID | undefined) => void;
    (<any>res).id = debuggablePromise(new Promise<ID | undefined>((resolve) => { resolveID = resolve; }));

    // Now "transfer" all input properties; this simply awaits any promises and resolves when they all do.
    let transfer: Promise<PropertyTransfer> = debuggablePromise(transferProperties(res, t, name, props, dependsOn));

    // Serialize the invocation if necessary.
    let resourceOp: Promise<void> = debuggablePromise(resourceChain.then(async () => {
        // Make sure to propagate these no matter what.
        let urn: URN | undefined = undefined;
        let id: ID | undefined = undefined;
        let propsStruct: any | undefined = undefined;
        let stable: boolean = false;

        // During a real deployment, the transfer operation may take some time to settle (we may need to wait on
        // other in-flight operations.  As a result, we can't launch the RPC request until they are done.  At the same
        // time, we want to give the illusion of non-blocking code, so we return immediately.
        let result: PropertyTransfer = await transfer;
        try {
            let obj: any = result.obj;
            Log.debug(`Resource RPC prepared: t=${t}, name=${name}, obj=${JSON.stringify(obj)}`);

            // Fetch the monitor and make an RPC request.
            let monitor: any = getMonitor();
            if (monitor) {
                let req = new langproto.NewResourceRequest();
                req.setType(t);
                req.setName(name);
                req.setObject(obj);

                let resp: any = await debuggablePromise(new Promise((resolve, reject) => {
                    monitor.newResource(req, (err: Error, resp: any) => {
                        Log.debug(`Resource RPC finished: t=${t}, name=${name}; err: ${err}, resp: ${resp}`);
                        if (err) {
                            Log.error(`Failed to register new resource '${name}' [${t}]: ${err}`);
                            reject(err);
                        }
                        else {
                            resolve(resp);
                        }
                    });
                }));

                urn = resp.getUrn();
                id = resp.getId();
                propsStruct = resp.getObject();
                stable = resp.getStable();
            }
            else {
                // If the monitor doesn't exist, still make sure to resolve all properties to undefined.
                Log.debug(`Not sending RPC to monitor -- it doesn't exist: t=${t}, name=${name}`);
            }
        }
        finally {
            // The resolution will always have a valid URN, even during planning, and it is final (doesn't change).
            resolveURN(urn);

            // If an ID is present, then it's safe to say it's final, because the resource planner wouldn't hand
            // it back to us otherwise (e.g., if the resource was being replaced, it would be missing).  If it isn't
            // available, ensure the ID gets resolved, just resolve it to undefined (indicating it isn't known).
            resolveID(id || undefined);

            // Finally propagate any other properties that were given to us as outputs.
            resolveProperties(res, result, t, name, props, propsStruct, stable);
        }
    }));

    // If any errors make it this far, ensure we log them.
    let finalOp: Promise<void> = debuggablePromise(resourceOp.catch((err: Error) => {
        // At this point, we've gone fully asynchronous, and the stack is missing.  To make it easier
        // to debug which resource this came from, we will emit the original stack trace too.
        Log.error(errorString(createError));
        Log.error(`Failed to create resource '${name}' [${t}]: ${errorString(err)}`);
    }));

    // Ensure the process won't exit until this registerResource call finishes and resolve it when appropriate.
    let done: () => void = rpcKeepAlive();
    finalOp.then(() => { done(); }, () => { done(); });

    // If serialization is requested, wait for the prior resource operation to finish before we proceed, serializing
    // them, and make this the current resource operation so that everybody piles up on it.
    if (serialize()) {
        resourceChain = finalOp;
    }
}

/**
 * PropertyTransfer is the result of transferring all properties.
 */
interface PropertyTransfer {
    obj: any; // the bag of input properties after awaiting them.
    resolvers: {[key: string]: ((v: any) => void)}; // a map of resolvers for output properties that will resolve.
}

/**
 * transferProperties stores the properties on the resource object and returns a gRPC serializable
 * proto.google.protobuf.Struct out of a resource's properties.
 */
function transferProperties(res: Resource, t: string, name: string, props: ComputedValues | undefined,
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
            if (res.hasOwnProperty(k)) {
                throw new Error(`Property '${k}' is already initialized on resource '${name}' [${t}]`);
            }
            (res as any)[k] =
                debuggablePromise(new Promise<any>((resolve) => { resolvers[k] = resolve; }));

            // Now serialize the value and store it in our map.  This operation may return eventuals that resolve
            // after all properties have settled, and we may need to wait for them before this transfer finishes.
            if (props[k] !== undefined) {
                eventuals.push(
                    serializeProperty(props[k], `${t}[${name}]`).then(
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
 * resolveProperties takes as input a gRPC serialized proto.google.protobuf.Struct and resolves all of the
 * resource's matching properties to the values inside.
 */
function resolveProperties(res: Resource, transfer: PropertyTransfer,
    t: string, name: string, inputs: ComputedValues | undefined, outputsStruct: any, stable: boolean): void {

    // Produce a combined set of property states, starting with inputs and then applying outputs.  If the same
    // property exists in the inputs and outputs states, the output wins.
    let props: any = inputs || {};
    if (outputsStruct) {
        let outputs: any = outputsStruct.toJavaScript();
        for (let k of Object.keys(outputs)) {
            props[k] = deserializeProperty(outputs[k]);
        }
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
            if (!options.dryRun || stable) {
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

