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

export class PackageDescriptor extends jspb.Message { 
    getPackage(): string;
    setPackage(value: string): PackageDescriptor;
    getVersion(): string;
    setVersion(value: string): PackageDescriptor;
    getDownloadUrl(): string;
    setDownloadUrl(value: string): PackageDescriptor;

    hasParameterization(): boolean;
    clearParameterization(): void;
    getParameterization(): Parameterization | undefined;
    setParameterization(value?: Parameterization): PackageDescriptor;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PackageDescriptor.AsObject;
    static toObject(includeInstance: boolean, msg: PackageDescriptor): PackageDescriptor.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PackageDescriptor, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PackageDescriptor;
    static deserializeBinaryFromReader(message: PackageDescriptor, reader: jspb.BinaryReader): PackageDescriptor;
}

export namespace PackageDescriptor {
    export type AsObject = {
        pb_package: string,
        version: string,
        downloadUrl: string,
        parameterization?: Parameterization.AsObject,
    }
}

export class PackageDescriptorMember extends jspb.Message { 

    hasSchema(): boolean;
    clearSchema(): void;
    getSchema(): PackageDescriptor | undefined;
    setSchema(value?: PackageDescriptor): PackageDescriptorMember;
    getMember(): string;
    setMember(value: string): PackageDescriptorMember;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PackageDescriptorMember.AsObject;
    static toObject(includeInstance: boolean, msg: PackageDescriptorMember): PackageDescriptorMember.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PackageDescriptorMember, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PackageDescriptorMember;
    static deserializeBinaryFromReader(message: PackageDescriptorMember, reader: jspb.BinaryReader): PackageDescriptorMember;
}

export namespace PackageDescriptorMember {
    export type AsObject = {
        schema?: PackageDescriptor.AsObject,
        member: string,
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

export class MetaSpec extends jspb.Message { 
    getModuleFormat(): string;
    setModuleFormat(value: string): MetaSpec;
    getSupportPack(): boolean;
    setSupportPack(value: boolean): MetaSpec;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): MetaSpec.AsObject;
    static toObject(includeInstance: boolean, msg: MetaSpec): MetaSpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: MetaSpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): MetaSpec;
    static deserializeBinaryFromReader(message: MetaSpec, reader: jspb.BinaryReader): MetaSpec;
}

export namespace MetaSpec {
    export type AsObject = {
        moduleFormat: string,
        supportPack: boolean,
    }
}

export class ParameterizationSpec extends jspb.Message { 
    getBaseProviderName(): string;
    setBaseProviderName(value: string): ParameterizationSpec;
    getBaseProviderVersion(): string;
    setBaseProviderVersion(value: string): ParameterizationSpec;
    getParameter(): Uint8Array | string;
    getParameter_asU8(): Uint8Array;
    getParameter_asB64(): string;
    setParameter(value: Uint8Array | string): ParameterizationSpec;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ParameterizationSpec.AsObject;
    static toObject(includeInstance: boolean, msg: ParameterizationSpec): ParameterizationSpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ParameterizationSpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ParameterizationSpec;
    static deserializeBinaryFromReader(message: ParameterizationSpec, reader: jspb.BinaryReader): ParameterizationSpec;
}

export namespace ParameterizationSpec {
    export type AsObject = {
        baseProviderName: string,
        baseProviderVersion: string,
        parameter: Uint8Array | string,
    }
}

export class PackageSpec extends jspb.Message { 
    getName(): string;
    setName(value: string): PackageSpec;
    getDisplayName(): string;
    setDisplayName(value: string): PackageSpec;

    hasVersion(): boolean;
    clearVersion(): void;
    getVersion(): string | undefined;
    setVersion(value: string): PackageSpec;
    getDescription(): string;
    setDescription(value: string): PackageSpec;
    clearKeywordsList(): void;
    getKeywordsList(): Array<string>;
    setKeywordsList(value: Array<string>): PackageSpec;
    addKeywords(value: string, index?: number): string;
    getHomepage(): string;
    setHomepage(value: string): PackageSpec;
    getLicense(): string;
    setLicense(value: string): PackageSpec;
    getAttribution(): string;
    setAttribution(value: string): PackageSpec;
    getRepository(): string;
    setRepository(value: string): PackageSpec;
    getLogoUrl(): string;
    setLogoUrl(value: string): PackageSpec;
    getPluginDownloadUrl(): string;
    setPluginDownloadUrl(value: string): PackageSpec;
    getPublisher(): string;
    setPublisher(value: string): PackageSpec;
    getNamespace(): string;
    setNamespace(value: string): PackageSpec;

    getDependenciesMap(): jspb.Map<string, PackageDescriptor>;
    clearDependenciesMap(): void;

    hasMeta(): boolean;
    clearMeta(): void;
    getMeta(): MetaSpec | undefined;
    setMeta(value?: MetaSpec): PackageSpec;
    clearAllowedPackageNamesList(): void;
    getAllowedPackageNamesList(): Array<string>;
    setAllowedPackageNamesList(value: Array<string>): PackageSpec;
    addAllowedPackageNames(value: string, index?: number): string;

    hasParameterization(): boolean;
    clearParameterization(): void;
    getParameterization(): ParameterizationSpec | undefined;
    setParameterization(value?: ParameterizationSpec): PackageSpec;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PackageSpec.AsObject;
    static toObject(includeInstance: boolean, msg: PackageSpec): PackageSpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PackageSpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PackageSpec;
    static deserializeBinaryFromReader(message: PackageSpec, reader: jspb.BinaryReader): PackageSpec;
}

export namespace PackageSpec {
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
        namespace: string,

        dependenciesMap: Array<[string, PackageDescriptor.AsObject]>,
        meta?: MetaSpec.AsObject,
        allowedPackageNamesList: Array<string>,
        parameterization?: ParameterizationSpec.AsObject,
    }
}

export class ResourceSpec extends jspb.Message { 
    getName(): string;
    setName(value: string): ResourceSpec;
    getDescription(): string;
    setDescription(value: string): ResourceSpec;
    clearAliasesList(): void;
    getAliasesList(): Array<string>;
    setAliasesList(value: Array<string>): ResourceSpec;
    addAliases(value: string, index?: number): string;

    getPropertiesMap(): jspb.Map<string, string>;
    clearPropertiesMap(): void;

    getInputsMap(): jspb.Map<string, string>;
    clearInputsMap(): void;

    getOutputsMap(): jspb.Map<string, string>;
    clearOutputsMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceSpec.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceSpec): ResourceSpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceSpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceSpec;
    static deserializeBinaryFromReader(message: ResourceSpec, reader: jspb.BinaryReader): ResourceSpec;
}

export namespace ResourceSpec {
    export type AsObject = {
        name: string,
        description: string,
        aliasesList: Array<string>,

        propertiesMap: Array<[string, string]>,

        inputsMap: Array<[string, string]>,

        outputsMap: Array<[string, string]>,
    }
}
