// package: pulumirpc
// file: pulumi/callback.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

export class Callback extends jspb.Message { 
    getTarget(): string;
    setTarget(value: string): Callback;
    getToken(): string;
    setToken(value: string): Callback;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Callback.AsObject;
    static toObject(includeInstance: boolean, msg: Callback): Callback.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Callback, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Callback;
    static deserializeBinaryFromReader(message: Callback, reader: jspb.BinaryReader): Callback;
}

export namespace Callback {
    export type AsObject = {
        target: string,
        token: string,
    }
}

export class CallbackInvokeRequest extends jspb.Message { 
    getToken(): string;
    setToken(value: string): CallbackInvokeRequest;
    clearArgumentsList(): void;
    getArgumentsList(): Array<google_protobuf_struct_pb.Value>;
    setArgumentsList(value: Array<google_protobuf_struct_pb.Value>): CallbackInvokeRequest;
    addArguments(value?: google_protobuf_struct_pb.Value, index?: number): google_protobuf_struct_pb.Value;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CallbackInvokeRequest.AsObject;
    static toObject(includeInstance: boolean, msg: CallbackInvokeRequest): CallbackInvokeRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CallbackInvokeRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CallbackInvokeRequest;
    static deserializeBinaryFromReader(message: CallbackInvokeRequest, reader: jspb.BinaryReader): CallbackInvokeRequest;
}

export namespace CallbackInvokeRequest {
    export type AsObject = {
        token: string,
        argumentsList: Array<google_protobuf_struct_pb.Value.AsObject>,
    }
}

export class CallbackInvokeResponse extends jspb.Message { 
    clearReturnsList(): void;
    getReturnsList(): Array<google_protobuf_struct_pb.Value>;
    setReturnsList(value: Array<google_protobuf_struct_pb.Value>): CallbackInvokeResponse;
    addReturns(value?: google_protobuf_struct_pb.Value, index?: number): google_protobuf_struct_pb.Value;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CallbackInvokeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: CallbackInvokeResponse): CallbackInvokeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CallbackInvokeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CallbackInvokeResponse;
    static deserializeBinaryFromReader(message: CallbackInvokeResponse, reader: jspb.BinaryReader): CallbackInvokeResponse;
}

export namespace CallbackInvokeResponse {
    export type AsObject = {
        returnsList: Array<google_protobuf_struct_pb.Value.AsObject>,
    }
}
