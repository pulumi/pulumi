// package: codegen
// file: pulumi/codegen/mapper.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class GetMappingRequest extends jspb.Message { 
    getProvider(): string;
    setProvider(value: string): GetMappingRequest;
    getPulumiProvider(): string;
    setPulumiProvider(value: string): GetMappingRequest;

    hasParameterizationHint(): boolean;
    clearParameterizationHint(): void;
    getParameterizationHint(): MapperParameterizationHint | undefined;
    setParameterizationHint(value?: MapperParameterizationHint): GetMappingRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetMappingRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetMappingRequest): GetMappingRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetMappingRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetMappingRequest;
    static deserializeBinaryFromReader(message: GetMappingRequest, reader: jspb.BinaryReader): GetMappingRequest;
}

export namespace GetMappingRequest {
    export type AsObject = {
        provider: string,
        pulumiProvider: string,
        parameterizationHint?: MapperParameterizationHint.AsObject,
    }
}

export class MapperParameterizationHint extends jspb.Message { 
    getName(): string;
    setName(value: string): MapperParameterizationHint;
    getVersion(): string;
    setVersion(value: string): MapperParameterizationHint;
    getValue(): Uint8Array | string;
    getValue_asU8(): Uint8Array;
    getValue_asB64(): string;
    setValue(value: Uint8Array | string): MapperParameterizationHint;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): MapperParameterizationHint.AsObject;
    static toObject(includeInstance: boolean, msg: MapperParameterizationHint): MapperParameterizationHint.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: MapperParameterizationHint, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): MapperParameterizationHint;
    static deserializeBinaryFromReader(message: MapperParameterizationHint, reader: jspb.BinaryReader): MapperParameterizationHint;
}

export namespace MapperParameterizationHint {
    export type AsObject = {
        name: string,
        version: string,
        value: Uint8Array | string,
    }
}

export class GetMappingResponse extends jspb.Message { 
    getData(): Uint8Array | string;
    getData_asU8(): Uint8Array;
    getData_asB64(): string;
    setData(value: Uint8Array | string): GetMappingResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetMappingResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetMappingResponse): GetMappingResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetMappingResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetMappingResponse;
    static deserializeBinaryFromReader(message: GetMappingResponse, reader: jspb.BinaryReader): GetMappingResponse;
}

export namespace GetMappingResponse {
    export type AsObject = {
        data: Uint8Array | string,
    }
}
