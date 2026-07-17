// package: pulumirpc
// file: pulumi/converter.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as pulumi_codegen_hcl_pb from "./codegen/hcl_pb";
import * as pulumi_codegen_loader_pb from "./codegen/loader_pb";

export class ConvertStateRequest extends jspb.Message { 
    getMapperTarget(): string;
    setMapperTarget(value: string): ConvertStateRequest;
    clearArgsList(): void;
    getArgsList(): Array<string>;
    setArgsList(value: Array<string>): ConvertStateRequest;
    addArgs(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConvertStateRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ConvertStateRequest): ConvertStateRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConvertStateRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConvertStateRequest;
    static deserializeBinaryFromReader(message: ConvertStateRequest, reader: jspb.BinaryReader): ConvertStateRequest;
}

export namespace ConvertStateRequest {
    export type AsObject = {
        mapperTarget: string,
        argsList: Array<string>,
    }
}

export class ResourceImport extends jspb.Message { 
    getType(): string;
    setType(value: string): ResourceImport;
    getName(): string;
    setName(value: string): ResourceImport;
    getId(): string;
    setId(value: string): ResourceImport;
    getVersion(): string;
    setVersion(value: string): ResourceImport;
    getPlugindownloadurl(): string;
    setPlugindownloadurl(value: string): ResourceImport;
    getLogicalName(): string;
    setLogicalName(value: string): ResourceImport;
    getIsComponent(): boolean;
    setIsComponent(value: boolean): ResourceImport;
    getIsRemote(): boolean;
    setIsRemote(value: boolean): ResourceImport;

    hasParameterization(): boolean;
    clearParameterization(): void;
    getParameterization(): ResourceParameterization | undefined;
    setParameterization(value?: ResourceParameterization): ResourceImport;

    hasExtension$(): boolean;
    clearExtension$(): void;
    getExtension$(): ResourceExtension | undefined;
    setExtension$(value?: ResourceExtension): ResourceImport;
    getParent(): string;
    setParent(value: string): ResourceImport;
    clearPropertiesList(): void;
    getPropertiesList(): Array<string>;
    setPropertiesList(value: Array<string>): ResourceImport;
    addProperties(value: string, index?: number): string;
    getProvider(): string;
    setProvider(value: string): ResourceImport;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceImport.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceImport): ResourceImport.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceImport, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceImport;
    static deserializeBinaryFromReader(message: ResourceImport, reader: jspb.BinaryReader): ResourceImport;
}

export namespace ResourceImport {
    export type AsObject = {
        type: string,
        name: string,
        id: string,
        version: string,
        plugindownloadurl: string,
        logicalName: string,
        isComponent: boolean,
        isRemote: boolean,
        parameterization?: ResourceParameterization.AsObject,
        extension?: ResourceExtension.AsObject,
        parent: string,
        propertiesList: Array<string>,
        provider: string,
    }
}

export class ResourceParameterization extends jspb.Message { 
    getPluginName(): string;
    setPluginName(value: string): ResourceParameterization;
    getPluginVersion(): string;
    setPluginVersion(value: string): ResourceParameterization;
    getValue(): Uint8Array | string;
    getValue_asU8(): Uint8Array;
    getValue_asB64(): string;
    setValue(value: Uint8Array | string): ResourceParameterization;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceParameterization.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceParameterization): ResourceParameterization.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceParameterization, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceParameterization;
    static deserializeBinaryFromReader(message: ResourceParameterization, reader: jspb.BinaryReader): ResourceParameterization;
}

export namespace ResourceParameterization {
    export type AsObject = {
        pluginName: string,
        pluginVersion: string,
        value: Uint8Array | string,
    }
}

export class ResourceExtension extends jspb.Message { 
    getName(): string;
    setName(value: string): ResourceExtension;
    getVersion(): string;
    setVersion(value: string): ResourceExtension;
    getValue(): Uint8Array | string;
    getValue_asU8(): Uint8Array;
    getValue_asB64(): string;
    setValue(value: Uint8Array | string): ResourceExtension;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceExtension.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceExtension): ResourceExtension.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceExtension, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceExtension;
    static deserializeBinaryFromReader(message: ResourceExtension, reader: jspb.BinaryReader): ResourceExtension;
}

export namespace ResourceExtension {
    export type AsObject = {
        name: string,
        version: string,
        value: Uint8Array | string,
    }
}

export class ConvertStateResponse extends jspb.Message { 
    clearResourcesList(): void;
    getResourcesList(): Array<ResourceImport>;
    setResourcesList(value: Array<ResourceImport>): ConvertStateResponse;
    addResources(value?: ResourceImport, index?: number): ResourceImport;
    clearDiagnosticsList(): void;
    getDiagnosticsList(): Array<pulumi_codegen_hcl_pb.Diagnostic>;
    setDiagnosticsList(value: Array<pulumi_codegen_hcl_pb.Diagnostic>): ConvertStateResponse;
    addDiagnostics(value?: pulumi_codegen_hcl_pb.Diagnostic, index?: number): pulumi_codegen_hcl_pb.Diagnostic;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConvertStateResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ConvertStateResponse): ConvertStateResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConvertStateResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConvertStateResponse;
    static deserializeBinaryFromReader(message: ConvertStateResponse, reader: jspb.BinaryReader): ConvertStateResponse;
}

export namespace ConvertStateResponse {
    export type AsObject = {
        resourcesList: Array<ResourceImport.AsObject>,
        diagnosticsList: Array<pulumi_codegen_hcl_pb.Diagnostic.AsObject>,
    }
}

export class ConvertProgramRequest extends jspb.Message { 
    getSourceDirectory(): string;
    setSourceDirectory(value: string): ConvertProgramRequest;
    getTargetDirectory(): string;
    setTargetDirectory(value: string): ConvertProgramRequest;
    getMapperTarget(): string;
    setMapperTarget(value: string): ConvertProgramRequest;
    getLoaderTarget(): string;
    setLoaderTarget(value: string): ConvertProgramRequest;
    clearArgsList(): void;
    getArgsList(): Array<string>;
    setArgsList(value: Array<string>): ConvertProgramRequest;
    addArgs(value: string, index?: number): string;
    getGeneratedProjectDirectory(): string;
    setGeneratedProjectDirectory(value: string): ConvertProgramRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConvertProgramRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ConvertProgramRequest): ConvertProgramRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConvertProgramRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConvertProgramRequest;
    static deserializeBinaryFromReader(message: ConvertProgramRequest, reader: jspb.BinaryReader): ConvertProgramRequest;
}

export namespace ConvertProgramRequest {
    export type AsObject = {
        sourceDirectory: string,
        targetDirectory: string,
        mapperTarget: string,
        loaderTarget: string,
        argsList: Array<string>,
        generatedProjectDirectory: string,
    }
}

export class ConvertProgramResponse extends jspb.Message { 
    clearDiagnosticsList(): void;
    getDiagnosticsList(): Array<pulumi_codegen_hcl_pb.Diagnostic>;
    setDiagnosticsList(value: Array<pulumi_codegen_hcl_pb.Diagnostic>): ConvertProgramResponse;
    addDiagnostics(value?: pulumi_codegen_hcl_pb.Diagnostic, index?: number): pulumi_codegen_hcl_pb.Diagnostic;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConvertProgramResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ConvertProgramResponse): ConvertProgramResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConvertProgramResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConvertProgramResponse;
    static deserializeBinaryFromReader(message: ConvertProgramResponse, reader: jspb.BinaryReader): ConvertProgramResponse;
}

export namespace ConvertProgramResponse {
    export type AsObject = {
        diagnosticsList: Array<pulumi_codegen_hcl_pb.Diagnostic.AsObject>,
    }
}

export class ConvertSnippetRequest extends jspb.Message { 
    getFilename(): string;
    setFilename(value: string): ConvertSnippetRequest;
    getSource(): Uint8Array | string;
    getSource_asU8(): Uint8Array;
    getSource_asB64(): string;
    setSource(value: Uint8Array | string): ConvertSnippetRequest;
    getTargetLoader(): string;
    setTargetLoader(value: string): ConvertSnippetRequest;

    hasPackage(): boolean;
    clearPackage(): void;
    getPackage(): pulumi_codegen_loader_pb.GetSchemaRequest | undefined;
    setPackage(value?: pulumi_codegen_loader_pb.GetSchemaRequest): ConvertSnippetRequest;
    getToken(): string;
    setToken(value: string): ConvertSnippetRequest;

    getAttributesMap(): jspb.Map<string, string>;
    clearAttributesMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConvertSnippetRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ConvertSnippetRequest): ConvertSnippetRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConvertSnippetRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConvertSnippetRequest;
    static deserializeBinaryFromReader(message: ConvertSnippetRequest, reader: jspb.BinaryReader): ConvertSnippetRequest;
}

export namespace ConvertSnippetRequest {
    export type AsObject = {
        filename: string,
        source: Uint8Array | string,
        targetLoader: string,
        pb_package?: pulumi_codegen_loader_pb.GetSchemaRequest.AsObject,
        token: string,

        attributesMap: Array<[string, string]>,
    }
}

export class ConvertSnippetResponse extends jspb.Message { 
    clearDiagnosticsList(): void;
    getDiagnosticsList(): Array<pulumi_codegen_hcl_pb.Diagnostic>;
    setDiagnosticsList(value: Array<pulumi_codegen_hcl_pb.Diagnostic>): ConvertSnippetResponse;
    addDiagnostics(value?: pulumi_codegen_hcl_pb.Diagnostic, index?: number): pulumi_codegen_hcl_pb.Diagnostic;
    getFilename(): string;
    setFilename(value: string): ConvertSnippetResponse;
    getSource(): Uint8Array | string;
    getSource_asU8(): Uint8Array;
    getSource_asB64(): string;
    setSource(value: Uint8Array | string): ConvertSnippetResponse;

    getAttributesMap(): jspb.Map<string, string>;
    clearAttributesMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConvertSnippetResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ConvertSnippetResponse): ConvertSnippetResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConvertSnippetResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConvertSnippetResponse;
    static deserializeBinaryFromReader(message: ConvertSnippetResponse, reader: jspb.BinaryReader): ConvertSnippetResponse;
}

export namespace ConvertSnippetResponse {
    export type AsObject = {
        diagnosticsList: Array<pulumi_codegen_hcl_pb.Diagnostic.AsObject>,
        filename: string,
        source: Uint8Array | string,

        attributesMap: Array<[string, string]>,
    }
}
