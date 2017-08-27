// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

let gstruct = require("google-protobuf/google/protobuf/struct_pb.js");

// marshalJSONObjectToGRPCStruct marshals a JSON object into its google.protobuf.Struct equivalent.
export function marshalJSONObjectToGRPCStruct(obj: any): any {
    if (obj === undefined) {
        return undefined;
    }
    let struct = new gstruct.Struct();
    let fields = struct.getFieldsMap();
    for (let k of Object.keys(obj)) {
        fields[k] = marshalJSONValueToGRPCValue(obj[k]);
    }
    return struct;
}

// marshalJSONValueToGRPCValue marshals a JSON value into its google.protobuf.Value equivalent.
export function marshalJSONValueToGRPCValue(v: any): any {
    if (v === undefined) {
        return undefined;
    }
    let val = new gstruct.Value();
    if (v === null) {
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
            a.push(marshalJSONValueToGRPCValue(e));
        }
        val.setListValue(a);
        val.setKindCase(gstruct.LIST_VALUE);
    }
    else {
        val.setStructValue(marshalJSONObjectToGRPCStruct(v));
        val.setKindCase(gstruct.STRUCT_VALUE);
    }
}

// unmarshalGRPCStructToJSONObject marshals a google.protobuf.Struct into its regular JSON equivalent.
export function unmarshalGRPCStructToJSONObject(struct: any): any {
    if (struct === undefined) {
        return undefined;
    }
    let obj: any = {};
    let fields = struct.getFieldsMap();
    for (let entry of fields.entries()) {
        obj[entry[0]] = unmarshalGRPCValueToJSONValue(entry[1]);
    }
    return obj;
}

// unmarshalGRPCValueToJSONValue marshals a google.protobuf.Value into its regular JSON equivalent.
export function unmarshalGRPCValueToJSONValue(v: any): any {
    if (v === undefined) {
        return undefined;
    }
    switch (v.getKindCase()) {
        case gstruct.NULL_VALUE:
            return null;
        case gstruct.NUMBER_VALUE:
            return v.getNumberValue();
        case gstruct.STRING_VALUE:
            return v.getStringValue();
        case gstruct.BOOL_VALUE:
            return v.getBoolValue();
        case gstruct.STRUCT_VALUE:
            return unmarshalGRPCStructToJSONObject(v.getStructValue());
        case gstruct.LIST_VALUE:
            let a = [];
            for (let e of v.getListValue()) {
                a.push(unmarshalGRPCValueToJSONValue(e));
            }
            return a;
        default:
            throw new Error(`Unrecognized gRPC struct value kind: ${v.getKindCase()}`);
    }
}

