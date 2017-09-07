// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as asset from "../asset";
import { Computed, MaybeComputed } from "../computed";
import { ID, Resource, URN } from "../resource";
import { debuggablePromise } from "./debuggable";
import { Log } from "./log";
import { isInsideMapValueCallback, Property } from "./property";
import { getMonitor, isDryRun, rpcKeepAlive } from "./settings";

let langproto = require("../proto/languages_pb.js");
let gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

// registerResource registers a new resource object with a given type t and name.  It returns the auto-generated URN
// and the ID that will resolve after the deployment has completed.  All properties will be initialized to property
// objects that the registration operation will resolve at the right time (or remain unresolved for deployments).
export function registerResource(
    res: Resource, t: string, name: string, props: {[key: string]: MaybeComputed<any> | undefined}): void {
    Log.debug(`Registering resource: t=${t}, name=${name}`);
    if (isInsideMapValueCallback()) {
        throw new Error(
            `Illegal attempt to create a conditional resource '${name}' (type ${t}) inside a mapValue callback`);
    }

    // Ensure the process won't exit until this registerResource call finishes.
    let notAlive: () => void = rpcKeepAlive();

    // Store a URN and ID property, plus any passed in, on the resource object.  Note that we do these using
    // any casts because they are typically readonly and this function is in cahoots with the initialization process.
    let urn = (<any>res).urn = new Property<URN>(undefined, true, false);
    let id = (<any>res).id = new Property<ID>(undefined, true, false);
    let transfer: Promise<any> = debuggablePromise(transferProperties(res, t, name, props));

    // During a real deployment, the transfer operation may take some time to settle (we may need to wait on
    // other in-flight operations.  As a result, we can't launch the RPC request until they are done.  At the same
    // time, we want to give the illusion of non-blocking code, so we return immediately.
    let monitor: any = getMonitor();
    let resourceRegistered: Promise<void> = debuggablePromise(transfer.then(
        async (obj: any) => {
            Log.debug(`Resource RPC prepared: t=${t}, name=${name}, obj=${JSON.stringify(obj)}`);

            // Fetch the monitor; if it doesn't exist, bail right away.
            if (!monitor) {
                Log.debug(`Not sending RPC to monitor -- it doesn't exist: t=${t}, name=${name}`);
                return;
            }

            // Fire off an RPC to the monitor to register the resource.  If/when it resolves, we will blit the properties.
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

            // The resolution will always have a valid URN, even during planning, and it is final (doesn't change).
            urn.setOutput(resp.getUrn(), true, false);

            // If an ID is present, then it's safe to say it's final, because the resource planner wouldn't hand
            // it back to us otherwise (e.g., if the resource was being replaced, it would be missing).
            let idOutput: string | undefined = resp.getId();
            if (idOutput) {
                id.setOutput(idOutput, true, false);
            }

            // Finally propagate any other properties that were given to us as outputs.
            try {
                resolveProperties(res, t, name, resp.getObject(), resp.getStable());
            }
            catch (err) {
                Log.error(`Failed to propagate resource provider properties to '${name}' [${t}]: ${err}`);
                throw err;
            }
        }
    ));

    // If any errors make it this far, ensure we log them.
    resourceRegistered.catch((err: Error) => {
        Log.error(`An unhandled error occurred during resource '${name}' [${t}] creation: ${err}`);
    });

    // Ensure we mark the RPC as done no matter the outcome.
    resourceRegistered.then(() => { notAlive(); }, () => { notAlive(); });
}

// transferProperties stores the properties on the resource object and returns a gRPC serializable
// proto.google.protobuf.Struct out of a resource's properties.
function transferProperties(
    res: Resource, t: string, name: string, props: {[key: string]: MaybeComputed<any> | undefined}): Promise<any> {
    let resbag: any = res;
    let obj: any = {}; // this will eventually hold the serialized object properties.
    let eventuals: Promise<void>[] = []; // this contains all promises outstanding for assignments.
    if (props) {
        for (let k of Object.keys(props)) {
            // Skip "id" and "urn", since we handle those specially.
            if (k === "id" || k === "urn") {
                continue;
            }

            // Create a property to wrap the value and store it on the resource.
            if (resbag[k]) {
                throw new Error(`Property '${k}' is already initialized on resource '${name}' [${t}]`);
            }
            let p = resbag[k] = new Property<any>(props[k], true, false);

            // Now serialize the value and store it in our map.  This operation may return eventuals that resolve
            // after all properties have settled, and we may need to wait for them before this transfer finishes.
            if (props[k] !== undefined) {
                eventuals.push(
                    serializeProperty(p.inputPromise).then((v: any) => {
                        obj[k] = v;
                    })
                );
            }
        }
    }

    // Now return a promise that resolves when all assignments above have settled.  Note that we do not
    // use await here, because we don't actually want to block the above assignments of properties.
    return debuggablePromise(Promise.all(eventuals).then(() => gstruct.Struct.fromJavaScript(obj)));
}

// resolveProperties takes as input a gRPC serialized proto.google.protobuf.Struct and resolves all of the
// resource's matching properties to the values inside.
function resolveProperties(res: Resource, t: string, name: string, propsStruct: any, stable: boolean): void {
    // First set any properties present in the output object.
    let weakres: any = <any>res;
    if (propsStruct) {
        let props: any = propsStruct.toJavaScript();
        for (let k of Object.keys(props)) {
            // Skip "id" and "urn", since we handle those specially.
            if (k === "id" || k === "urn") {
                continue;
            }

            // Otherwise, unmarshal the value, and store it on the resource object.
            let p: Object | undefined = weakres[k];
            if (p === undefined) {
                // If there is no property yet, zero initialize it.  This ensures unexpected properties are
                // still made available on the object.  This isn't ideal, because any code running prior to the actual
                // resource CRUD operation can't hang computations off of it, but it's better than tossing it.
                weakres[k] = p = new Property<any>(undefined, true, false);
            }
            else if (!(p instanceof Property)) {
                throw new Error(
                    `Unable to set property '${k}' on resource '${name}' [${t}] because it is not a Property<T>`);
            }
            try {
                (p as Property<any>).setOutput(
                    deserializeProperty(props[k]), !isDryRun() || stable, false);
            }
            catch (err) {
                throw new Error(
                    `Unable to set property '${k}' on resource '${name}' [${t}]; error: ${err}`);
            }
        }
    }

    // Now latch all properties in case the inputs did not contain any values.  If we're doing a dry-run, we won't
    // actually propagate the provisional state, because we cannot know for sure that it is final yet.
    for (let k of Object.keys(res)) {
        let p: Object = weakres[k];
        if (p instanceof Property) {
            (p as Property<any>).done(isDryRun());
        }
    }
}

// unknownPropertyValue is a special value that the monitor recognizes.
export const unknownPropertyValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";
// specialSigKey is sometimes used to encode type identity inside of a map.  See pkg/resource/properties.go.
export const specialSigKey = "4dabf18193072939515e22adb298388d";
// specialAssetSig is a randomly assigned hash used to identify assets in maps.  See pkg/resource/asset.go.
export const specialAssetSig = "c44067f5952c0a294b673a41bacd8c17";
// specialArchiveSig is a randomly assigned hash used to identify archives in maps.  See pkg/resource/asset.go.
export const specialArchiveSig = "0def7320c3a5731c473e5ecbe6d01bc7";

// serializeProperty serializes properties deeply.  This understands how to wait on any unresolved promises, as
// appropriate, in addition to translating certain "special" values so that they are ready to go on the wire.
async function serializeProperty(prop: any): Promise<any> {
    if (prop === undefined) {
        if (!isDryRun()) {
            throw new Error("Unexpected unknown property during deployment");
        }
        return unknownPropertyValue;
    }
    else if (prop === null || typeof prop === "boolean" ||
            typeof prop === "number" || typeof prop === "string") {
        return prop;
    }
    else if (prop instanceof Array) {
        let elems: any[] = [];
        for (let e of prop) {
            elems.push(await serializeProperty(e));
        }
        return elems;
    }
    else if (prop instanceof Resource) {
        // Resources aren't serializable; instead, we serialize them as references to the ID property.
        return serializeProperty(prop.id);
    }
    else if (prop instanceof asset.Asset || prop instanceof asset.Archive) {
        // Serializing an asset or archive requires the use of a magical signature key, since otherwise it would look
        // like any old weakly typed object/map when received by the other side of the RPC boundary.
        let obj: any = {
            [specialSigKey]: (prop instanceof asset.Asset ? specialAssetSig : specialArchiveSig),
        };
        for (let k of Object.keys(prop)) {
            obj[k] = await serializeProperty((<any>prop)[k]);
        }
        return obj;
    }
    else if (prop instanceof Promise) {
        // For a promise input, await the property and then serialize the result.
        return serializeProperty(await prop);
    }
    else if (prop instanceof Property) {
        // For properties, wait for the output values to become available, and then serialize them.
        return serializeProperty(await prop.outputPromise);
    }
    else if ((prop as Computed<any>).mapValue !== undefined) {
        // For arbitrary computed values, wire up a handler to await their resolution and then serialize the value.
        let value: any = await debuggablePromise(new Promise<any>((resolve) => {
            (prop as Computed<any>).mapValue((v: any) => { resolve(v); });
        }));
        return serializeProperty(value);
    }
    else {
        let obj: any = {};
        for (let k of Object.keys(prop)) {
            obj[k] = await serializeProperty(prop[k]);
        }
        return obj;
    }
}

// deserializeProperty unpacks some special types, reversing the above process.
function deserializeProperty(prop: any): any {
    if (prop === undefined || typeof prop === "boolean" ||
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

