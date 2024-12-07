// package: codegen
// file: pulumi/codegen/loader.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as pulumi_codegen_schema_schema_pb from "../codegen/schema/schema_pb";

export class Parameterization extends jspb.Message { 
    getName(): string;
    setName(value: string): Parameterization;
    getVersion(): string;
    setVersion(value: string): Parameterization;
    getValue(): Uint8Array | string;
    getValue_asU8(): Uint8Array;
    getValue_asB64(): string;
    setValue(value: Uint8Array | string): Parameterization;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Parameterization.AsObject;
    static toObject(includeInstance: boolean, msg: Parameterization): Parameterization.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Parameterization, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Parameterization;
    static deserializeBinaryFromReader(message: Parameterization, reader: jspb.BinaryReader): Parameterization;
}

export namespace Parameterization {
    export type AsObject = {
        name: string,
        version: string,
        value: Uint8Array | string,
    }
}

export class GetSchemaRequest extends jspb.Message { 
    getPackage(): string;
    setPackage(value: string): GetSchemaRequest;
    getVersion(): string;
    setVersion(value: string): GetSchemaRequest;
    getDownloadUrl(): string;
    setDownloadUrl(value: string): GetSchemaRequest;

    hasParameterization(): boolean;
    clearParameterization(): void;
    getParameterization(): Parameterization | undefined;
    setParameterization(value?: Parameterization): GetSchemaRequest;

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
        downloadUrl: string,
        parameterization?: Parameterization.AsObject,
    }
}

export class GetSchemaPartRequest extends jspb.Message { 

    hasRequest(): boolean;
    clearRequest(): void;
    getRequest(): GetSchemaRequest | undefined;
    setRequest(value?: GetSchemaRequest): GetSchemaPartRequest;
    getItem(): string;
    setItem(value: string): GetSchemaPartRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetSchemaPartRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetSchemaPartRequest): GetSchemaPartRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetSchemaPartRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetSchemaPartRequest;
    static deserializeBinaryFromReader(message: GetSchemaPartRequest, reader: jspb.BinaryReader): GetSchemaPartRequest;
}

export namespace GetSchemaPartRequest {
    export type AsObject = {
        request?: GetSchemaRequest.AsObject,
        item: string,
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

export class GetPartialSchemaRequest extends jspb.Message { 
    getPackage(): string;
    setPackage(value: string): GetPartialSchemaRequest;
    getVersion(): string;
    setVersion(value: string): GetPartialSchemaRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetPartialSchemaRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetPartialSchemaRequest): GetPartialSchemaRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetPartialSchemaRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetPartialSchemaRequest;
    static deserializeBinaryFromReader(message: GetPartialSchemaRequest, reader: jspb.BinaryReader): GetPartialSchemaRequest;
}

export namespace GetPartialSchemaRequest {
    export type AsObject = {
        pb_package: string,
        version: string,
    }
}

export class GetPartialSchemaPartRequest extends jspb.Message { 

    hasRequest(): boolean;
    clearRequest(): void;
    getRequest(): GetPartialSchemaRequest | undefined;
    setRequest(value?: GetPartialSchemaRequest): GetPartialSchemaPartRequest;
    getItem(): string;
    setItem(value: string): GetPartialSchemaPartRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetPartialSchemaPartRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetPartialSchemaPartRequest): GetPartialSchemaPartRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetPartialSchemaPartRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetPartialSchemaPartRequest;
    static deserializeBinaryFromReader(message: GetPartialSchemaPartRequest, reader: jspb.BinaryReader): GetPartialSchemaPartRequest;
}

export namespace GetPartialSchemaPartRequest {
    export type AsObject = {
        request?: GetPartialSchemaRequest.AsObject,
        item: string,
    }
}
