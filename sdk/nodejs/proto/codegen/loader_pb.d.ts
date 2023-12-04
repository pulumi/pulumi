// package: codegen
// file: pulumi/codegen/loader.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

export class GetSchemaRequest extends jspb.Message { 
    getPackage(): string;
    setPackage(value: string): GetSchemaRequest;
    getVersion(): string;
    setVersion(value: string): GetSchemaRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetSchemaRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetSchemaRequest): GetSchemaRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetSchemaRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetSchemaRequest;
    static deserializeBinaryFromReader(message: GetSchemaRequest, reader: jspb.BinaryReader): GetSchemaRequest;
}

export namespace GetSchemaRequest {
    export type AsObject = {
        pb_package: string,
        version: string,
    }
}

export class GetSchemaResponse extends jspb.Message { 
    getSchema(): Uint8Array | string;
    getSchema_asU8(): Uint8Array;
    getSchema_asB64(): string;
    setSchema(value: Uint8Array | string): GetSchemaResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetSchemaResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetSchemaResponse): GetSchemaResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetSchemaResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetSchemaResponse;
    static deserializeBinaryFromReader(message: GetSchemaResponse, reader: jspb.BinaryReader): GetSchemaResponse;
}

export namespace GetSchemaResponse {
    export type AsObject = {
        schema: Uint8Array | string,
    }
}
