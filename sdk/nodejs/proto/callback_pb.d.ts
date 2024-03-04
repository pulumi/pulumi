// package: pulumirpc
// file: pulumi/callback.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

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
    getRequest(): Uint8Array | string;
    getRequest_asU8(): Uint8Array;
    getRequest_asB64(): string;
    setRequest(value: Uint8Array | string): CallbackInvokeRequest;

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
        request: Uint8Array | string,
    }
}

export class CallbackInvokeResponse extends jspb.Message { 
    getResponse(): Uint8Array | string;
    getResponse_asU8(): Uint8Array;
    getResponse_asB64(): string;
    setResponse(value: Uint8Array | string): CallbackInvokeResponse;

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
        response: Uint8Array | string,
    }
}
