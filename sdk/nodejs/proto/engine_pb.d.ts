// package: pulumirpc
// file: pulumi/engine.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

export class LogRequest extends jspb.Message { 
    getSeverity(): LogSeverity;
    setSeverity(value: LogSeverity): LogRequest;
    getMessage(): string;
    setMessage(value: string): LogRequest;
    getUrn(): string;
    setUrn(value: string): LogRequest;
    getStreamid(): number;
    setStreamid(value: number): LogRequest;
    getEphemeral(): boolean;
    setEphemeral(value: boolean): LogRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): LogRequest.AsObject;
    static toObject(includeInstance: boolean, msg: LogRequest): LogRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: LogRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): LogRequest;
    static deserializeBinaryFromReader(message: LogRequest, reader: jspb.BinaryReader): LogRequest;
}

export namespace LogRequest {
    export type AsObject = {
        severity: LogSeverity,
        message: string,
        urn: string,
        streamid: number,
        ephemeral: boolean,
    }
}

export class GetRootResourceRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetRootResourceRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetRootResourceRequest): GetRootResourceRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetRootResourceRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetRootResourceRequest;
    static deserializeBinaryFromReader(message: GetRootResourceRequest, reader: jspb.BinaryReader): GetRootResourceRequest;
}

export namespace GetRootResourceRequest {
    export type AsObject = {
    }
}

export class GetRootResourceResponse extends jspb.Message { 
    getUrn(): string;
    setUrn(value: string): GetRootResourceResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetRootResourceResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetRootResourceResponse): GetRootResourceResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetRootResourceResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetRootResourceResponse;
    static deserializeBinaryFromReader(message: GetRootResourceResponse, reader: jspb.BinaryReader): GetRootResourceResponse;
}

export namespace GetRootResourceResponse {
    export type AsObject = {
        urn: string,
    }
}

export class SetRootResourceRequest extends jspb.Message { 
    getUrn(): string;
    setUrn(value: string): SetRootResourceRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SetRootResourceRequest.AsObject;
    static toObject(includeInstance: boolean, msg: SetRootResourceRequest): SetRootResourceRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SetRootResourceRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SetRootResourceRequest;
    static deserializeBinaryFromReader(message: SetRootResourceRequest, reader: jspb.BinaryReader): SetRootResourceRequest;
}

export namespace SetRootResourceRequest {
    export type AsObject = {
        urn: string,
    }
}

export class SetRootResourceResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SetRootResourceResponse.AsObject;
    static toObject(includeInstance: boolean, msg: SetRootResourceResponse): SetRootResourceResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SetRootResourceResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SetRootResourceResponse;
    static deserializeBinaryFromReader(message: SetRootResourceResponse, reader: jspb.BinaryReader): SetRootResourceResponse;
}

export namespace SetRootResourceResponse {
    export type AsObject = {
    }
}

export class StartDebuggingRequest extends jspb.Message { 

    hasConfig(): boolean;
    clearConfig(): void;
    getConfig(): google_protobuf_struct_pb.Struct | undefined;
    setConfig(value?: google_protobuf_struct_pb.Struct): StartDebuggingRequest;
    getMessage(): string;
    setMessage(value: string): StartDebuggingRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): StartDebuggingRequest.AsObject;
    static toObject(includeInstance: boolean, msg: StartDebuggingRequest): StartDebuggingRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: StartDebuggingRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): StartDebuggingRequest;
    static deserializeBinaryFromReader(message: StartDebuggingRequest, reader: jspb.BinaryReader): StartDebuggingRequest;
}

export namespace StartDebuggingRequest {
    export type AsObject = {
        config?: google_protobuf_struct_pb.Struct.AsObject,
        message: string,
    }
}

export enum LogSeverity {
    DEBUG = 0,
    INFO = 1,
    WARNING = 2,
    ERROR = 3,
}
