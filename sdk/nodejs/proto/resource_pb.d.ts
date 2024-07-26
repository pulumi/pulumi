// package: pulumirpc
// file: pulumi/resource.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as pulumi_provider_pb from "./provider_pb";
import * as pulumi_alias_pb from "./alias_pb";
import * as pulumi_source_pb from "./source_pb";
import * as pulumi_callback_pb from "./callback_pb";

export class SupportsFeatureRequest extends jspb.Message { 
    getId(): string;
    setId(value: string): SupportsFeatureRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SupportsFeatureRequest.AsObject;
    static toObject(includeInstance: boolean, msg: SupportsFeatureRequest): SupportsFeatureRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SupportsFeatureRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SupportsFeatureRequest;
    static deserializeBinaryFromReader(message: SupportsFeatureRequest, reader: jspb.BinaryReader): SupportsFeatureRequest;
}

export namespace SupportsFeatureRequest {
    export type AsObject = {
        id: string,
    }
}

export class SupportsFeatureResponse extends jspb.Message { 
    getHassupport(): boolean;
    setHassupport(value: boolean): SupportsFeatureResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SupportsFeatureResponse.AsObject;
    static toObject(includeInstance: boolean, msg: SupportsFeatureResponse): SupportsFeatureResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SupportsFeatureResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SupportsFeatureResponse;
    static deserializeBinaryFromReader(message: SupportsFeatureResponse, reader: jspb.BinaryReader): SupportsFeatureResponse;
}

export namespace SupportsFeatureResponse {
    export type AsObject = {
        hassupport: boolean,
    }
}

export class ReadResourceRequest extends jspb.Message { 
    getId(): string;
    setId(value: string): ReadResourceRequest;
    getType(): string;
    setType(value: string): ReadResourceRequest;
    getName(): string;
    setName(value: string): ReadResourceRequest;
    getParent(): string;
    setParent(value: string): ReadResourceRequest;

    hasProperties(): boolean;
    clearProperties(): void;
    getProperties(): google_protobuf_struct_pb.Struct | undefined;
    setProperties(value?: google_protobuf_struct_pb.Struct): ReadResourceRequest;
    clearDependenciesList(): void;
    getDependenciesList(): Array<string>;
    setDependenciesList(value: Array<string>): ReadResourceRequest;
    addDependencies(value: string, index?: number): string;
    getProvider(): string;
    setProvider(value: string): ReadResourceRequest;
    getVersion(): string;
    setVersion(value: string): ReadResourceRequest;
    getAcceptsecrets(): boolean;
    setAcceptsecrets(value: boolean): ReadResourceRequest;
    clearAdditionalsecretoutputsList(): void;
    getAdditionalsecretoutputsList(): Array<string>;
    setAdditionalsecretoutputsList(value: Array<string>): ReadResourceRequest;
    addAdditionalsecretoutputs(value: string, index?: number): string;
    getAcceptresources(): boolean;
    setAcceptresources(value: boolean): ReadResourceRequest;
    getPlugindownloadurl(): string;
    setPlugindownloadurl(value: string): ReadResourceRequest;

    getPluginchecksumsMap(): jspb.Map<string, Uint8Array | string>;
    clearPluginchecksumsMap(): void;

    hasSourceposition(): boolean;
    clearSourceposition(): void;
    getSourceposition(): pulumi_source_pb.SourcePosition | undefined;
    setSourceposition(value?: pulumi_source_pb.SourcePosition): ReadResourceRequest;
    getPackageref(): string;
    setPackageref(value: string): ReadResourceRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ReadResourceRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ReadResourceRequest): ReadResourceRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ReadResourceRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ReadResourceRequest;
    static deserializeBinaryFromReader(message: ReadResourceRequest, reader: jspb.BinaryReader): ReadResourceRequest;
}

export namespace ReadResourceRequest {
    export type AsObject = {
        id: string,
        type: string,
        name: string,
        parent: string,
        properties?: google_protobuf_struct_pb.Struct.AsObject,
        dependenciesList: Array<string>,
        provider: string,
        version: string,
        acceptsecrets: boolean,
        additionalsecretoutputsList: Array<string>,
        acceptresources: boolean,
        plugindownloadurl: string,

        pluginchecksumsMap: Array<[string, Uint8Array | string]>,
        sourceposition?: pulumi_source_pb.SourcePosition.AsObject,
        packageref: string,
    }
}

export class ReadResourceResponse extends jspb.Message { 
    getUrn(): string;
    setUrn(value: string): ReadResourceResponse;

    hasProperties(): boolean;
    clearProperties(): void;
    getProperties(): google_protobuf_struct_pb.Struct | undefined;
    setProperties(value?: google_protobuf_struct_pb.Struct): ReadResourceResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ReadResourceResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ReadResourceResponse): ReadResourceResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ReadResourceResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ReadResourceResponse;
    static deserializeBinaryFromReader(message: ReadResourceResponse, reader: jspb.BinaryReader): ReadResourceResponse;
}

export namespace ReadResourceResponse {
    export type AsObject = {
        urn: string,
        properties?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class RegisterResourceRequest extends jspb.Message { 
    getType(): string;
    setType(value: string): RegisterResourceRequest;
    getName(): string;
    setName(value: string): RegisterResourceRequest;
    getParent(): string;
    setParent(value: string): RegisterResourceRequest;
    getCustom(): boolean;
    setCustom(value: boolean): RegisterResourceRequest;

    hasObject(): boolean;
    clearObject(): void;
    getObject(): google_protobuf_struct_pb.Struct | undefined;
    setObject(value?: google_protobuf_struct_pb.Struct): RegisterResourceRequest;
    getProtect(): boolean;
    setProtect(value: boolean): RegisterResourceRequest;
    clearDependenciesList(): void;
    getDependenciesList(): Array<string>;
    setDependenciesList(value: Array<string>): RegisterResourceRequest;
    addDependencies(value: string, index?: number): string;
    getProvider(): string;
    setProvider(value: string): RegisterResourceRequest;

    getPropertydependenciesMap(): jspb.Map<string, RegisterResourceRequest.PropertyDependencies>;
    clearPropertydependenciesMap(): void;
    getDeletebeforereplace(): boolean;
    setDeletebeforereplace(value: boolean): RegisterResourceRequest;
    getVersion(): string;
    setVersion(value: string): RegisterResourceRequest;
    clearIgnorechangesList(): void;
    getIgnorechangesList(): Array<string>;
    setIgnorechangesList(value: Array<string>): RegisterResourceRequest;
    addIgnorechanges(value: string, index?: number): string;
    getAcceptsecrets(): boolean;
    setAcceptsecrets(value: boolean): RegisterResourceRequest;
    clearAdditionalsecretoutputsList(): void;
    getAdditionalsecretoutputsList(): Array<string>;
    setAdditionalsecretoutputsList(value: Array<string>): RegisterResourceRequest;
    addAdditionalsecretoutputs(value: string, index?: number): string;
    clearAliasurnsList(): void;
    getAliasurnsList(): Array<string>;
    setAliasurnsList(value: Array<string>): RegisterResourceRequest;
    addAliasurns(value: string, index?: number): string;
    getImportid(): string;
    setImportid(value: string): RegisterResourceRequest;

    hasCustomtimeouts(): boolean;
    clearCustomtimeouts(): void;
    getCustomtimeouts(): RegisterResourceRequest.CustomTimeouts | undefined;
    setCustomtimeouts(value?: RegisterResourceRequest.CustomTimeouts): RegisterResourceRequest;
    getDeletebeforereplacedefined(): boolean;
    setDeletebeforereplacedefined(value: boolean): RegisterResourceRequest;
    getSupportspartialvalues(): boolean;
    setSupportspartialvalues(value: boolean): RegisterResourceRequest;
    getRemote(): boolean;
    setRemote(value: boolean): RegisterResourceRequest;
    getAcceptresources(): boolean;
    setAcceptresources(value: boolean): RegisterResourceRequest;

    getProvidersMap(): jspb.Map<string, string>;
    clearProvidersMap(): void;
    clearReplaceonchangesList(): void;
    getReplaceonchangesList(): Array<string>;
    setReplaceonchangesList(value: Array<string>): RegisterResourceRequest;
    addReplaceonchanges(value: string, index?: number): string;
    getPlugindownloadurl(): string;
    setPlugindownloadurl(value: string): RegisterResourceRequest;

    getPluginchecksumsMap(): jspb.Map<string, Uint8Array | string>;
    clearPluginchecksumsMap(): void;
    getRetainondelete(): boolean;
    setRetainondelete(value: boolean): RegisterResourceRequest;
    clearAliasesList(): void;
    getAliasesList(): Array<pulumi_alias_pb.Alias>;
    setAliasesList(value: Array<pulumi_alias_pb.Alias>): RegisterResourceRequest;
    addAliases(value?: pulumi_alias_pb.Alias, index?: number): pulumi_alias_pb.Alias;
    getDeletedwith(): string;
    setDeletedwith(value: string): RegisterResourceRequest;
    getAliasspecs(): boolean;
    setAliasspecs(value: boolean): RegisterResourceRequest;

    hasSourceposition(): boolean;
    clearSourceposition(): void;
    getSourceposition(): pulumi_source_pb.SourcePosition | undefined;
    setSourceposition(value?: pulumi_source_pb.SourcePosition): RegisterResourceRequest;
    clearTransformsList(): void;
    getTransformsList(): Array<pulumi_callback_pb.Callback>;
    setTransformsList(value: Array<pulumi_callback_pb.Callback>): RegisterResourceRequest;
    addTransforms(value?: pulumi_callback_pb.Callback, index?: number): pulumi_callback_pb.Callback;
    getSupportsresultreporting(): boolean;
    setSupportsresultreporting(value: boolean): RegisterResourceRequest;
    getPackageref(): string;
    setPackageref(value: string): RegisterResourceRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterResourceRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterResourceRequest): RegisterResourceRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterResourceRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterResourceRequest;
    static deserializeBinaryFromReader(message: RegisterResourceRequest, reader: jspb.BinaryReader): RegisterResourceRequest;
}

export namespace RegisterResourceRequest {
    export type AsObject = {
        type: string,
        name: string,
        parent: string,
        custom: boolean,
        object?: google_protobuf_struct_pb.Struct.AsObject,
        protect: boolean,
        dependenciesList: Array<string>,
        provider: string,

        propertydependenciesMap: Array<[string, RegisterResourceRequest.PropertyDependencies.AsObject]>,
        deletebeforereplace: boolean,
        version: string,
        ignorechangesList: Array<string>,
        acceptsecrets: boolean,
        additionalsecretoutputsList: Array<string>,
        aliasurnsList: Array<string>,
        importid: string,
        customtimeouts?: RegisterResourceRequest.CustomTimeouts.AsObject,
        deletebeforereplacedefined: boolean,
        supportspartialvalues: boolean,
        remote: boolean,
        acceptresources: boolean,

        providersMap: Array<[string, string]>,
        replaceonchangesList: Array<string>,
        plugindownloadurl: string,

        pluginchecksumsMap: Array<[string, Uint8Array | string]>,
        retainondelete: boolean,
        aliasesList: Array<pulumi_alias_pb.Alias.AsObject>,
        deletedwith: string,
        aliasspecs: boolean,
        sourceposition?: pulumi_source_pb.SourcePosition.AsObject,
        transformsList: Array<pulumi_callback_pb.Callback.AsObject>,
        supportsresultreporting: boolean,
        packageref: string,
    }


    export class PropertyDependencies extends jspb.Message { 
        clearUrnsList(): void;
        getUrnsList(): Array<string>;
        setUrnsList(value: Array<string>): PropertyDependencies;
        addUrns(value: string, index?: number): string;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): PropertyDependencies.AsObject;
        static toObject(includeInstance: boolean, msg: PropertyDependencies): PropertyDependencies.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: PropertyDependencies, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): PropertyDependencies;
        static deserializeBinaryFromReader(message: PropertyDependencies, reader: jspb.BinaryReader): PropertyDependencies;
    }

    export namespace PropertyDependencies {
        export type AsObject = {
            urnsList: Array<string>,
        }
    }

    export class CustomTimeouts extends jspb.Message { 
        getCreate(): string;
        setCreate(value: string): CustomTimeouts;
        getUpdate(): string;
        setUpdate(value: string): CustomTimeouts;
        getDelete(): string;
        setDelete(value: string): CustomTimeouts;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): CustomTimeouts.AsObject;
        static toObject(includeInstance: boolean, msg: CustomTimeouts): CustomTimeouts.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: CustomTimeouts, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): CustomTimeouts;
        static deserializeBinaryFromReader(message: CustomTimeouts, reader: jspb.BinaryReader): CustomTimeouts;
    }

    export namespace CustomTimeouts {
        export type AsObject = {
            create: string,
            update: string,
            pb_delete: string,
        }
    }

}

export class RegisterResourceResponse extends jspb.Message { 
    getUrn(): string;
    setUrn(value: string): RegisterResourceResponse;
    getId(): string;
    setId(value: string): RegisterResourceResponse;

    hasObject(): boolean;
    clearObject(): void;
    getObject(): google_protobuf_struct_pb.Struct | undefined;
    setObject(value?: google_protobuf_struct_pb.Struct): RegisterResourceResponse;
    getStable(): boolean;
    setStable(value: boolean): RegisterResourceResponse;
    clearStablesList(): void;
    getStablesList(): Array<string>;
    setStablesList(value: Array<string>): RegisterResourceResponse;
    addStables(value: string, index?: number): string;

    getPropertydependenciesMap(): jspb.Map<string, RegisterResourceResponse.PropertyDependencies>;
    clearPropertydependenciesMap(): void;
    getResult(): Result;
    setResult(value: Result): RegisterResourceResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterResourceResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterResourceResponse): RegisterResourceResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterResourceResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterResourceResponse;
    static deserializeBinaryFromReader(message: RegisterResourceResponse, reader: jspb.BinaryReader): RegisterResourceResponse;
}

export namespace RegisterResourceResponse {
    export type AsObject = {
        urn: string,
        id: string,
        object?: google_protobuf_struct_pb.Struct.AsObject,
        stable: boolean,
        stablesList: Array<string>,

        propertydependenciesMap: Array<[string, RegisterResourceResponse.PropertyDependencies.AsObject]>,
        result: Result,
    }


    export class PropertyDependencies extends jspb.Message { 
        clearUrnsList(): void;
        getUrnsList(): Array<string>;
        setUrnsList(value: Array<string>): PropertyDependencies;
        addUrns(value: string, index?: number): string;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): PropertyDependencies.AsObject;
        static toObject(includeInstance: boolean, msg: PropertyDependencies): PropertyDependencies.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: PropertyDependencies, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): PropertyDependencies;
        static deserializeBinaryFromReader(message: PropertyDependencies, reader: jspb.BinaryReader): PropertyDependencies;
    }

    export namespace PropertyDependencies {
        export type AsObject = {
            urnsList: Array<string>,
        }
    }

}

export class RegisterResourceOutputsRequest extends jspb.Message { 
    getUrn(): string;
    setUrn(value: string): RegisterResourceOutputsRequest;

    hasOutputs(): boolean;
    clearOutputs(): void;
    getOutputs(): google_protobuf_struct_pb.Struct | undefined;
    setOutputs(value?: google_protobuf_struct_pb.Struct): RegisterResourceOutputsRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterResourceOutputsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterResourceOutputsRequest): RegisterResourceOutputsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterResourceOutputsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterResourceOutputsRequest;
    static deserializeBinaryFromReader(message: RegisterResourceOutputsRequest, reader: jspb.BinaryReader): RegisterResourceOutputsRequest;
}

export namespace RegisterResourceOutputsRequest {
    export type AsObject = {
        urn: string,
        outputs?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class ResourceInvokeRequest extends jspb.Message { 
    getTok(): string;
    setTok(value: string): ResourceInvokeRequest;

    hasArgs(): boolean;
    clearArgs(): void;
    getArgs(): google_protobuf_struct_pb.Struct | undefined;
    setArgs(value?: google_protobuf_struct_pb.Struct): ResourceInvokeRequest;
    getProvider(): string;
    setProvider(value: string): ResourceInvokeRequest;
    getVersion(): string;
    setVersion(value: string): ResourceInvokeRequest;
    getAcceptresources(): boolean;
    setAcceptresources(value: boolean): ResourceInvokeRequest;
    getPlugindownloadurl(): string;
    setPlugindownloadurl(value: string): ResourceInvokeRequest;

    getPluginchecksumsMap(): jspb.Map<string, Uint8Array | string>;
    clearPluginchecksumsMap(): void;

    hasSourceposition(): boolean;
    clearSourceposition(): void;
    getSourceposition(): pulumi_source_pb.SourcePosition | undefined;
    setSourceposition(value?: pulumi_source_pb.SourcePosition): ResourceInvokeRequest;
    getPackageref(): string;
    setPackageref(value: string): ResourceInvokeRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceInvokeRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceInvokeRequest): ResourceInvokeRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceInvokeRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceInvokeRequest;
    static deserializeBinaryFromReader(message: ResourceInvokeRequest, reader: jspb.BinaryReader): ResourceInvokeRequest;
}

export namespace ResourceInvokeRequest {
    export type AsObject = {
        tok: string,
        args?: google_protobuf_struct_pb.Struct.AsObject,
        provider: string,
        version: string,
        acceptresources: boolean,
        plugindownloadurl: string,

        pluginchecksumsMap: Array<[string, Uint8Array | string]>,
        sourceposition?: pulumi_source_pb.SourcePosition.AsObject,
        packageref: string,
    }
}

export class ResourceCallRequest extends jspb.Message { 
    getTok(): string;
    setTok(value: string): ResourceCallRequest;

    hasArgs(): boolean;
    clearArgs(): void;
    getArgs(): google_protobuf_struct_pb.Struct | undefined;
    setArgs(value?: google_protobuf_struct_pb.Struct): ResourceCallRequest;

    getArgdependenciesMap(): jspb.Map<string, ResourceCallRequest.ArgumentDependencies>;
    clearArgdependenciesMap(): void;
    getProvider(): string;
    setProvider(value: string): ResourceCallRequest;
    getVersion(): string;
    setVersion(value: string): ResourceCallRequest;
    getPlugindownloadurl(): string;
    setPlugindownloadurl(value: string): ResourceCallRequest;

    getPluginchecksumsMap(): jspb.Map<string, Uint8Array | string>;
    clearPluginchecksumsMap(): void;

    hasSourceposition(): boolean;
    clearSourceposition(): void;
    getSourceposition(): pulumi_source_pb.SourcePosition | undefined;
    setSourceposition(value?: pulumi_source_pb.SourcePosition): ResourceCallRequest;
    getPackageref(): string;
    setPackageref(value: string): ResourceCallRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceCallRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceCallRequest): ResourceCallRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceCallRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceCallRequest;
    static deserializeBinaryFromReader(message: ResourceCallRequest, reader: jspb.BinaryReader): ResourceCallRequest;
}

export namespace ResourceCallRequest {
    export type AsObject = {
        tok: string,
        args?: google_protobuf_struct_pb.Struct.AsObject,

        argdependenciesMap: Array<[string, ResourceCallRequest.ArgumentDependencies.AsObject]>,
        provider: string,
        version: string,
        plugindownloadurl: string,

        pluginchecksumsMap: Array<[string, Uint8Array | string]>,
        sourceposition?: pulumi_source_pb.SourcePosition.AsObject,
        packageref: string,
    }


    export class ArgumentDependencies extends jspb.Message { 
        clearUrnsList(): void;
        getUrnsList(): Array<string>;
        setUrnsList(value: Array<string>): ArgumentDependencies;
        addUrns(value: string, index?: number): string;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): ArgumentDependencies.AsObject;
        static toObject(includeInstance: boolean, msg: ArgumentDependencies): ArgumentDependencies.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: ArgumentDependencies, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): ArgumentDependencies;
        static deserializeBinaryFromReader(message: ArgumentDependencies, reader: jspb.BinaryReader): ArgumentDependencies;
    }

    export namespace ArgumentDependencies {
        export type AsObject = {
            urnsList: Array<string>,
        }
    }

}

export class TransformResourceOptions extends jspb.Message { 
    clearDependsOnList(): void;
    getDependsOnList(): Array<string>;
    setDependsOnList(value: Array<string>): TransformResourceOptions;
    addDependsOn(value: string, index?: number): string;
    getProtect(): boolean;
    setProtect(value: boolean): TransformResourceOptions;
    clearIgnoreChangesList(): void;
    getIgnoreChangesList(): Array<string>;
    setIgnoreChangesList(value: Array<string>): TransformResourceOptions;
    addIgnoreChanges(value: string, index?: number): string;
    clearReplaceOnChangesList(): void;
    getReplaceOnChangesList(): Array<string>;
    setReplaceOnChangesList(value: Array<string>): TransformResourceOptions;
    addReplaceOnChanges(value: string, index?: number): string;
    getVersion(): string;
    setVersion(value: string): TransformResourceOptions;
    clearAliasesList(): void;
    getAliasesList(): Array<pulumi_alias_pb.Alias>;
    setAliasesList(value: Array<pulumi_alias_pb.Alias>): TransformResourceOptions;
    addAliases(value?: pulumi_alias_pb.Alias, index?: number): pulumi_alias_pb.Alias;
    getProvider(): string;
    setProvider(value: string): TransformResourceOptions;

    hasCustomTimeouts(): boolean;
    clearCustomTimeouts(): void;
    getCustomTimeouts(): RegisterResourceRequest.CustomTimeouts | undefined;
    setCustomTimeouts(value?: RegisterResourceRequest.CustomTimeouts): TransformResourceOptions;
    getPluginDownloadUrl(): string;
    setPluginDownloadUrl(value: string): TransformResourceOptions;
    getRetainOnDelete(): boolean;
    setRetainOnDelete(value: boolean): TransformResourceOptions;
    getDeletedWith(): string;
    setDeletedWith(value: string): TransformResourceOptions;

    hasDeleteBeforeReplace(): boolean;
    clearDeleteBeforeReplace(): void;
    getDeleteBeforeReplace(): boolean | undefined;
    setDeleteBeforeReplace(value: boolean): TransformResourceOptions;
    clearAdditionalSecretOutputsList(): void;
    getAdditionalSecretOutputsList(): Array<string>;
    setAdditionalSecretOutputsList(value: Array<string>): TransformResourceOptions;
    addAdditionalSecretOutputs(value: string, index?: number): string;

    getProvidersMap(): jspb.Map<string, string>;
    clearProvidersMap(): void;

    getPluginChecksumsMap(): jspb.Map<string, Uint8Array | string>;
    clearPluginChecksumsMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TransformResourceOptions.AsObject;
    static toObject(includeInstance: boolean, msg: TransformResourceOptions): TransformResourceOptions.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TransformResourceOptions, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TransformResourceOptions;
    static deserializeBinaryFromReader(message: TransformResourceOptions, reader: jspb.BinaryReader): TransformResourceOptions;
}

export namespace TransformResourceOptions {
    export type AsObject = {
        dependsOnList: Array<string>,
        protect: boolean,
        ignoreChangesList: Array<string>,
        replaceOnChangesList: Array<string>,
        version: string,
        aliasesList: Array<pulumi_alias_pb.Alias.AsObject>,
        provider: string,
        customTimeouts?: RegisterResourceRequest.CustomTimeouts.AsObject,
        pluginDownloadUrl: string,
        retainOnDelete: boolean,
        deletedWith: string,
        deleteBeforeReplace?: boolean,
        additionalSecretOutputsList: Array<string>,

        providersMap: Array<[string, string]>,

        pluginChecksumsMap: Array<[string, Uint8Array | string]>,
    }
}

export class TransformRequest extends jspb.Message { 
    getType(): string;
    setType(value: string): TransformRequest;
    getName(): string;
    setName(value: string): TransformRequest;
    getCustom(): boolean;
    setCustom(value: boolean): TransformRequest;
    getParent(): string;
    setParent(value: string): TransformRequest;

    hasProperties(): boolean;
    clearProperties(): void;
    getProperties(): google_protobuf_struct_pb.Struct | undefined;
    setProperties(value?: google_protobuf_struct_pb.Struct): TransformRequest;

    hasOptions(): boolean;
    clearOptions(): void;
    getOptions(): TransformResourceOptions | undefined;
    setOptions(value?: TransformResourceOptions): TransformRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TransformRequest.AsObject;
    static toObject(includeInstance: boolean, msg: TransformRequest): TransformRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TransformRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TransformRequest;
    static deserializeBinaryFromReader(message: TransformRequest, reader: jspb.BinaryReader): TransformRequest;
}

export namespace TransformRequest {
    export type AsObject = {
        type: string,
        name: string,
        custom: boolean,
        parent: string,
        properties?: google_protobuf_struct_pb.Struct.AsObject,
        options?: TransformResourceOptions.AsObject,
    }
}

export class TransformResponse extends jspb.Message { 

    hasProperties(): boolean;
    clearProperties(): void;
    getProperties(): google_protobuf_struct_pb.Struct | undefined;
    setProperties(value?: google_protobuf_struct_pb.Struct): TransformResponse;

    hasOptions(): boolean;
    clearOptions(): void;
    getOptions(): TransformResourceOptions | undefined;
    setOptions(value?: TransformResourceOptions): TransformResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TransformResponse.AsObject;
    static toObject(includeInstance: boolean, msg: TransformResponse): TransformResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TransformResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TransformResponse;
    static deserializeBinaryFromReader(message: TransformResponse, reader: jspb.BinaryReader): TransformResponse;
}

export namespace TransformResponse {
    export type AsObject = {
        properties?: google_protobuf_struct_pb.Struct.AsObject,
        options?: TransformResourceOptions.AsObject,
    }
}

export class TransformInvokeRequest extends jspb.Message { 
    getToken(): string;
    setToken(value: string): TransformInvokeRequest;

    hasArgs(): boolean;
    clearArgs(): void;
    getArgs(): google_protobuf_struct_pb.Struct | undefined;
    setArgs(value?: google_protobuf_struct_pb.Struct): TransformInvokeRequest;

    hasOptions(): boolean;
    clearOptions(): void;
    getOptions(): TransformInvokeOptions | undefined;
    setOptions(value?: TransformInvokeOptions): TransformInvokeRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TransformInvokeRequest.AsObject;
    static toObject(includeInstance: boolean, msg: TransformInvokeRequest): TransformInvokeRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TransformInvokeRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TransformInvokeRequest;
    static deserializeBinaryFromReader(message: TransformInvokeRequest, reader: jspb.BinaryReader): TransformInvokeRequest;
}

export namespace TransformInvokeRequest {
    export type AsObject = {
        token: string,
        args?: google_protobuf_struct_pb.Struct.AsObject,
        options?: TransformInvokeOptions.AsObject,
    }
}

export class TransformInvokeResponse extends jspb.Message { 

    hasArgs(): boolean;
    clearArgs(): void;
    getArgs(): google_protobuf_struct_pb.Struct | undefined;
    setArgs(value?: google_protobuf_struct_pb.Struct): TransformInvokeResponse;

    hasOptions(): boolean;
    clearOptions(): void;
    getOptions(): TransformInvokeOptions | undefined;
    setOptions(value?: TransformInvokeOptions): TransformInvokeResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TransformInvokeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: TransformInvokeResponse): TransformInvokeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TransformInvokeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TransformInvokeResponse;
    static deserializeBinaryFromReader(message: TransformInvokeResponse, reader: jspb.BinaryReader): TransformInvokeResponse;
}

export namespace TransformInvokeResponse {
    export type AsObject = {
        args?: google_protobuf_struct_pb.Struct.AsObject,
        options?: TransformInvokeOptions.AsObject,
    }
}

export class TransformInvokeOptions extends jspb.Message { 
    getProvider(): string;
    setProvider(value: string): TransformInvokeOptions;
    getPluginDownloadUrl(): string;
    setPluginDownloadUrl(value: string): TransformInvokeOptions;
    getVersion(): string;
    setVersion(value: string): TransformInvokeOptions;

    getPluginChecksumsMap(): jspb.Map<string, Uint8Array | string>;
    clearPluginChecksumsMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TransformInvokeOptions.AsObject;
    static toObject(includeInstance: boolean, msg: TransformInvokeOptions): TransformInvokeOptions.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TransformInvokeOptions, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TransformInvokeOptions;
    static deserializeBinaryFromReader(message: TransformInvokeOptions, reader: jspb.BinaryReader): TransformInvokeOptions;
}

export namespace TransformInvokeOptions {
    export type AsObject = {
        provider: string,
        pluginDownloadUrl: string,
        version: string,

        pluginChecksumsMap: Array<[string, Uint8Array | string]>,
    }
}

export class RegisterPackageRequest extends jspb.Message { 
    getName(): string;
    setName(value: string): RegisterPackageRequest;
    getVersion(): string;
    setVersion(value: string): RegisterPackageRequest;
    getDownloadUrl(): string;
    setDownloadUrl(value: string): RegisterPackageRequest;

    getChecksumsMap(): jspb.Map<string, Uint8Array | string>;
    clearChecksumsMap(): void;

    hasParameterization(): boolean;
    clearParameterization(): void;
    getParameterization(): Parameterization | undefined;
    setParameterization(value?: Parameterization): RegisterPackageRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterPackageRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterPackageRequest): RegisterPackageRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterPackageRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterPackageRequest;
    static deserializeBinaryFromReader(message: RegisterPackageRequest, reader: jspb.BinaryReader): RegisterPackageRequest;
}

export namespace RegisterPackageRequest {
    export type AsObject = {
        name: string,
        version: string,
        downloadUrl: string,

        checksumsMap: Array<[string, Uint8Array | string]>,
        parameterization?: Parameterization.AsObject,
    }
}

export class RegisterPackageResponse extends jspb.Message { 
    getRef(): string;
    setRef(value: string): RegisterPackageResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterPackageResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterPackageResponse): RegisterPackageResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterPackageResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterPackageResponse;
    static deserializeBinaryFromReader(message: RegisterPackageResponse, reader: jspb.BinaryReader): RegisterPackageResponse;
}

export namespace RegisterPackageResponse {
    export type AsObject = {
        ref: string,
    }
}

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

export enum Result {
    SUCCESS = 0,
    FAIL = 1,
    SKIP = 2,
}
