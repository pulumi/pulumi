// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { Property } from "../property";
import { Resource, URN } from "../resource";
import { getMonitor, isDryRun } from "./monitor";

let langproto = require("../proto/nodejs/languages_pb");
let gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

// registerResource registers a new resource object with a given type t and name.  It returns the auto-generated URN
// and the ID that will resolve after the deployment has completed.  All properties will be initialized to property
// objects that the registration operation will resolve at the right time (or remain unresolved for deployments).
export function registerResource(
        res: Resource, t: string, name: string, props?: {[key: string]: Property<any>}): void {
    let monitor: any = getMonitor();

    // Create a resource URN and an ID that will get populated after deployment.
    let urn = new Property<URN>();
    let id = new Property<string>();

    // Store these properties, plus all of those passed in, on the resource object.  Note that we do these using
    // any casts because they are typically readonly and this function is in cahoots with the initialization process.
    let transfer: Promise<any> = transferProperties(res, props);
    (<any>res).urn = urn;
    (<any>res).id = id;

    // During a real deployment, the transfer operation may take some time to settle (we may need to wait on
    // other in-flight operations.  As a result, we can't launch the RPC request until they are done.  At the same
    // time, we want to give the illusion of non-blocking code, so we return immediately.
    transfer.then((obj: any) => {
        // Fire off an RPC to the monitor to register the resource.  If/when it resolves, we will blit the properties.
        let req = new langproto.NewResourceRequest();
        req.setType(t);
        req.setName(name);
        req.setObject(obj);
        monitor.newResource(req, (err: Error, resp: any) => {
            if (err) {
                throw new Error(`Failed to register new resource with monitor: ${err}`);
            }
            else {
                // The resolution will always have a valid URN, even during planning.
                urn.resolve(resp.getUrn());

                // The ID and properties, on the other hand, are only resolved to values during deployments.
                let newID: any = resp.getId();
                if (newID) {
                    id.resolve(newID);
                }
                let newProperties: any = resp.getObject();
                if (newProperties) {
                    resolveProperties(newProperties, res);
                }
            }
        });
    });
}

// unknownPropertyValue is a special value that the monitor recognizes.
export const unknownPropertyValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";

// transferProperties stores the properties on the resource object and returns a gRPC serializable
// proto.google.protobuf.Struct out of a resource's properties.
function transferProperties(res: Resource, props?: {[key: string]: Property<any>}): Promise<any> {
    let obj: any = {}; // this will eventually hold the serialized object properties.
    let eventuals: Promise<void>[] = []; // this may end up containing promises for linked properties.
    if (props) {
        for (let k of Object.keys(props)) {
            // Store the property on the resource.
            let v: Property<any> = props[k];
            if (v === undefined) {
                throw new Error(`Property '${k}' is undefined`);
            }
            else if (!(v instanceof Property)) {
                throw new Error(`Property '${k}' is not a fabric Property object`);
            }
            else if ((<any>res)[k]) {
                throw new Error(`Property '${k}' is already initialized on this resource object`);
            }
            (<any>res)[k] = v;

            // If this is a property with a known value, serialize it; skip outputs for now.
            // TODO: we need to serialize assets/archives using sig keys.
            if (v.has()) {
                // If this is a property, and it is a concrete value, propagate it.
                obj[k] = v.require();
            }
            else {
                let link: Promise<any> | undefined = v.linked();
                if (link) {
                    // If this is a property linked to the completion of another one, it's computed.  In the case
                    // of a dry run, we cannot know its value, so we say so.  For other cases, we must wait.
                    if (isDryRun()) {
                        obj[k] = unknownPropertyValue;
                    }
                    else {
                        eventuals.push(link.then((ev) => {
                            obj[k] = ev;
                        }));
                    }
                }
            }
        }
    }
    return Promise.all(eventuals).then(() => {
        return gstruct.Struct.fromJavaScript(obj);
    });
}

// resolveProperties takes as input a gRPC serialized proto.google.protobuf.Struct and resolves all of the
// resource's matching properties to the values inside.
function resolveProperties(struct: any, res: Resource): void {
    // First set any properties present in the output object.
    let props: any = struct.toJavaScript();
    for (let k of Object.keys(props)) {
        let v: any = (<any>res)[k];
        if (!(v instanceof Property)) {
            throw new Error(`Unable to set resource property '${k}' because it is not a Property<T>`);
        }
        v.resolve(props[k]);
    }
    // Now latch any other properties to their final values, in case they aren't set.
    for (let k of Object.keys(res)) {
        let v: any = (<any>res)[k];
        if (v instanceof Property) {
            (<any>res)[k].done();
        }
    }
}

