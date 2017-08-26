// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { Property } from "../property";
import { Resource, URN } from "../resource";
import { getMonitor } from "./monitor";

let gstruct = require("google-protobuf/google/protobuf/struct_pb.js");
let langproto = require("../proto/nodejs/languages_pb");

// registerResource registers a new resource object with a given type t and name.  It returns the auto-generated URN
// and the ID that will resolve after the deployment has completed.  All properties will be initialized to property
// objects that the registration operation will resolve at the right time (or remain unresolved for deployments).
export function registerResource(res: Resource, t: string, name: string): { urn: Property<URN>, id: Property<string> } {
    let monitor: any = getMonitor();

    // Create a resource URN and an ID that will get populated after deployment.
    let urn = new Property<URN>();
    let id = new Property<string>();

    // Fire off an RPC to the monitor to register the resource.  If/when it resolves, we will blit the properties.
    let req = new langproto.NewResourceRequest();
    req.setType(t);
    req.setName(name);
    req.setObject(encodeProperties(res));
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

    return { urn: urn, id: id };
}

// unknownValueSentinel is a special value that the monitor recognizes.
const unknownValueSentinel = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";

// encodeProperties creates a gRPC serializable proto.google.protobuf.Struct out of a resource's properties.
function encodeProperties(res: Resource): any {
    let obj: any = {};
    for (let k of Object.keys(res)) {
        let v: any = (<any>res)[k];
        if (v instanceof Property) {
            // If this is a property with a known value, serialize it; skip outputs for now.
            // TODO: we need to serialize assets/archives using sig keys.
            // TODO: if any are waiting, we need to also wait for them.
            if (v.has()) {
                // If this is a property, and it is a concrete value, propagate it.
                obj[k] = v.require();
            }
            else if (v.linked()) {
                // If this is a property linked to the completion of another one, it's computed.
                obj[k] = unknownValueSentinel;
            }
        }
    }
    return marshalStruct(obj);
}

// resolveProperties takes as input a gRPC serialized proto.google.protobuf.Struct and resolves all of the
// resource's matching properties to the values inside.
function resolveProperties(obj: any, res: Resource): void {
    // First set any properties present in the output object.
    let props: any = unmarshalStruct(obj);
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

function marshalStruct(obj: any): any {
    let struct = new gstruct.Struct();
    let fields = struct.getFieldsMap();
    for (let k of Object.keys(obj)) {
        fields[k] = marshalValue(obj[k]);
    }
    return struct;
}

function marshalValue(v: any): any {
    let val = new gstruct.Value();
    if (v === undefined || v === null) {
        val.setNullValue(gstruct.NULL_VALUE);
        val.setKindCase(gstruct.NULL_VALUE);
    }
    else if (typeof v === "number") {
        val.setNumberValue(v);
        val.setKindCase(gstruct.NUMBER_VALUE);
    }
    else if (typeof v === "string") {
        val.setStringValue(v);
        val.setKindCase(gstruct.STRING_VALUE);
    }
    else if (typeof v === "boolean") {
        val.setBoolValue(v);
        val.setKindCase(gstruct.BOOL_VALUE);
    }
    else if (v instanceof Array) {
        let a = [];
        for (let e of v) {
            a.push(marshalValue(e));
        }
        val.setListValue(a);
        val.setKindCase(gstruct.LIST_VALUE);
    }
    else {
        val.setStructValue(marshalStruct(v));
        val.setKindCase(gstruct.STRUCT_VALUE);
    }
}

function unmarshalStruct(struct: any): any {
    let obj: any = {};
    let fields = struct.getFieldsMap();
    for (let k of Object.keys(fields)) {
        obj[k] = unmarshalValue(fields[k]);
    }
    return obj;
}

function unmarshalValue(v: any): any {
    switch (v.getKindCase()) {
        case gstruct.NULL_VALUE:
            return undefined;
        case gstruct.NUMBER_VALUE:
            return v.getNumberValue();
        case gstruct.STRING_VALUE:
            return v.getStringValue();
        case gstruct.BOOL_VALUE:
            return v.getBoolValue();
        case gstruct.STRUCT_VALUE:
            return unmarshalStruct(v.getStructValue());
        case gstruct.LIST_VALUE:
            let a = [];
            for (let e of v.getListValue()) {
                a.push(unmarshalValue(e));
            }
            return a;
        default:
            throw new Error(`Unrecognized gRPC struct value kind: ${v.getKindCase()}`);
    }
}

