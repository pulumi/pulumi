// package: codegen
// file: pulumi/codegen/loader.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

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

export class PackageMeta extends jspb.Message { 
    getModuleFormat(): string;
    setModuleFormat(value: string): PackageMeta;
    getSupportPack(): boolean;
    setSupportPack(value: boolean): PackageMeta;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PackageMeta.AsObject;
    static toObject(includeInstance: boolean, msg: PackageMeta): PackageMeta.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PackageMeta, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PackageMeta;
    static deserializeBinaryFromReader(message: PackageMeta, reader: jspb.BinaryReader): PackageMeta;
}

export namespace PackageMeta {
    export type AsObject = {
        moduleFormat: string,
        supportPack: boolean,
    }
}

export class PackageInfo extends jspb.Message { 
    getName(): string;
    setName(value: string): PackageInfo;
    getDisplayName(): string;
    setDisplayName(value: string): PackageInfo;

    hasVersion(): boolean;
    clearVersion(): void;
    getVersion(): string | undefined;
    setVersion(value: string): PackageInfo;
    getDescription(): string;
    setDescription(value: string): PackageInfo;
    clearKeywordsList(): void;
    getKeywordsList(): Array<string>;
    setKeywordsList(value: Array<string>): PackageInfo;
    addKeywords(value: string, index?: number): string;
    getHomepage(): string;
    setHomepage(value: string): PackageInfo;
    getLicense(): string;
    setLicense(value: string): PackageInfo;
    getAttribution(): string;
    setAttribution(value: string): PackageInfo;
    getRepository(): string;
    setRepository(value: string): PackageInfo;
    getLogoUrl(): string;
    setLogoUrl(value: string): PackageInfo;
    getPluginDownloadUrl(): string;
    setPluginDownloadUrl(value: string): PackageInfo;
    getPublisher(): string;
    setPublisher(value: string): PackageInfo;

    hasNamespace(): boolean;
    clearNamespace(): void;
    getNamespace(): string | undefined;
    setNamespace(value: string): PackageInfo;

    hasMeta(): boolean;
    clearMeta(): void;
    getMeta(): PackageMeta | undefined;
    setMeta(value?: PackageMeta): PackageInfo;
    clearAllowedPackageNamesList(): void;
    getAllowedPackageNamesList(): Array<string>;
    setAllowedPackageNamesList(value: Array<string>): PackageInfo;
    addAllowedPackageNames(value: string, index?: number): string;

    getLanguagesMap(): jspb.Map<string, string>;
    clearLanguagesMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PackageInfo.AsObject;
    static toObject(includeInstance: boolean, msg: PackageInfo): PackageInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PackageInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PackageInfo;
    static deserializeBinaryFromReader(message: PackageInfo, reader: jspb.BinaryReader): PackageInfo;
}

export namespace PackageInfo {
    export type AsObject = {
        name: string,
        displayName: string,
        version?: string,
        description: string,
        keywordsList: Array<string>,
        homepage: string,
        license: string,
        attribution: string,
        repository: string,
        logoUrl: string,
        pluginDownloadUrl: string,
        publisher: string,
        namespace?: string,
        meta?: PackageMeta.AsObject,
        allowedPackageNamesList: Array<string>,

        languagesMap: Array<[string, string]>,
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
