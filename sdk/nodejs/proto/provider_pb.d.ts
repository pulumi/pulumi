// package: pulumirpc
// file: pulumi/provider.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as pulumi_plugin_pb from "./plugin_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

export class ParameterizeRequest extends jspb.Message { 

    hasArgs(): boolean;
    clearArgs(): void;
    getArgs(): ParameterizeRequest.ParametersArgs | undefined;
    setArgs(value?: ParameterizeRequest.ParametersArgs): ParameterizeRequest;

    hasValue(): boolean;
    clearValue(): void;
    getValue(): ParameterizeRequest.ParametersValue | undefined;
    setValue(value?: ParameterizeRequest.ParametersValue): ParameterizeRequest;

    getParametersCase(): ParameterizeRequest.ParametersCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ParameterizeRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ParameterizeRequest): ParameterizeRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ParameterizeRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ParameterizeRequest;
    static deserializeBinaryFromReader(message: ParameterizeRequest, reader: jspb.BinaryReader): ParameterizeRequest;
}

export namespace ParameterizeRequest {
    export type AsObject = {
        args?: ParameterizeRequest.ParametersArgs.AsObject,
        value?: ParameterizeRequest.ParametersValue.AsObject,
    }


    export class ParametersArgs extends jspb.Message { 
        clearArgsList(): void;
        getArgsList(): Array<string>;
        setArgsList(value: Array<string>): ParametersArgs;
        addArgs(value: string, index?: number): string;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): ParametersArgs.AsObject;
        static toObject(includeInstance: boolean, msg: ParametersArgs): ParametersArgs.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: ParametersArgs, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): ParametersArgs;
        static deserializeBinaryFromReader(message: ParametersArgs, reader: jspb.BinaryReader): ParametersArgs;
    }

    export namespace ParametersArgs {
        export type AsObject = {
            argsList: Array<string>,
        }
    }

    export class ParametersValue extends jspb.Message { 
        getName(): string;
        setName(value: string): ParametersValue;
        getVersion(): string;
        setVersion(value: string): ParametersValue;
        getValue(): Uint8Array | string;
        getValue_asU8(): Uint8Array;
        getValue_asB64(): string;
        setValue(value: Uint8Array | string): ParametersValue;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): ParametersValue.AsObject;
        static toObject(includeInstance: boolean, msg: ParametersValue): ParametersValue.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: ParametersValue, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): ParametersValue;
        static deserializeBinaryFromReader(message: ParametersValue, reader: jspb.BinaryReader): ParametersValue;
    }

    export namespace ParametersValue {
        export type AsObject = {
            name: string,
            version: string,
            value: Uint8Array | string,
        }
    }


    export enum ParametersCase {
        PARAMETERS_NOT_SET = 0,
        ARGS = 1,
        VALUE = 2,
    }

}

export class ParameterizeResponse extends jspb.Message { 
    getName(): string;
    setName(value: string): ParameterizeResponse;
    getVersion(): string;
    setVersion(value: string): ParameterizeResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ParameterizeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ParameterizeResponse): ParameterizeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ParameterizeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ParameterizeResponse;
    static deserializeBinaryFromReader(message: ParameterizeResponse, reader: jspb.BinaryReader): ParameterizeResponse;
}

export namespace ParameterizeResponse {
    export type AsObject = {
        name: string,
        version: string,
    }
}

export class GetSchemaRequest extends jspb.Message { 
    getVersion(): number;
    setVersion(value: number): GetSchemaRequest;
    getSubpackageName(): string;
    setSubpackageName(value: string): GetSchemaRequest;
    getSubpackageVersion(): string;
    setSubpackageVersion(value: string): GetSchemaRequest;

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
        version: number,
        subpackageName: string,
        subpackageVersion: string,
    }
}

export class GetSchemaResponse extends jspb.Message { 
    getSchema(): string;
    setSchema(value: string): GetSchemaResponse;

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
        schema: string,
    }
}

export class ConfigureRequest extends jspb.Message { 

    getVariablesMap(): jspb.Map<string, string>;
    clearVariablesMap(): void;

    hasArgs(): boolean;
    clearArgs(): void;
    getArgs(): google_protobuf_struct_pb.Struct | undefined;
    setArgs(value?: google_protobuf_struct_pb.Struct): ConfigureRequest;
    getAcceptsecrets(): boolean;
    setAcceptsecrets(value: boolean): ConfigureRequest;
    getAcceptresources(): boolean;
    setAcceptresources(value: boolean): ConfigureRequest;
    getSendsOldInputs(): boolean;
    setSendsOldInputs(value: boolean): ConfigureRequest;
    getSendsOldInputsToDelete(): boolean;
    setSendsOldInputsToDelete(value: boolean): ConfigureRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConfigureRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ConfigureRequest): ConfigureRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConfigureRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConfigureRequest;
    static deserializeBinaryFromReader(message: ConfigureRequest, reader: jspb.BinaryReader): ConfigureRequest;
}

export namespace ConfigureRequest {
    export type AsObject = {

        variablesMap: Array<[string, string]>,
        args?: google_protobuf_struct_pb.Struct.AsObject,
        acceptsecrets: boolean,
        acceptresources: boolean,
        sendsOldInputs: boolean,
        sendsOldInputsToDelete: boolean,
    }
}

export class ConfigureResponse extends jspb.Message { 
    getAcceptsecrets(): boolean;
    setAcceptsecrets(value: boolean): ConfigureResponse;
    getSupportspreview(): boolean;
    setSupportspreview(value: boolean): ConfigureResponse;
    getAcceptresources(): boolean;
    setAcceptresources(value: boolean): ConfigureResponse;
    getAcceptoutputs(): boolean;
    setAcceptoutputs(value: boolean): ConfigureResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConfigureResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ConfigureResponse): ConfigureResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConfigureResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConfigureResponse;
    static deserializeBinaryFromReader(message: ConfigureResponse, reader: jspb.BinaryReader): ConfigureResponse;
}

export namespace ConfigureResponse {
    export type AsObject = {
        acceptsecrets: boolean,
        supportspreview: boolean,
        acceptresources: boolean,
        acceptoutputs: boolean,
    }
}

export class ConfigureErrorMissingKeys extends jspb.Message { 
    clearMissingkeysList(): void;
    getMissingkeysList(): Array<ConfigureErrorMissingKeys.MissingKey>;
    setMissingkeysList(value: Array<ConfigureErrorMissingKeys.MissingKey>): ConfigureErrorMissingKeys;
    addMissingkeys(value?: ConfigureErrorMissingKeys.MissingKey, index?: number): ConfigureErrorMissingKeys.MissingKey;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConfigureErrorMissingKeys.AsObject;
    static toObject(includeInstance: boolean, msg: ConfigureErrorMissingKeys): ConfigureErrorMissingKeys.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConfigureErrorMissingKeys, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConfigureErrorMissingKeys;
    static deserializeBinaryFromReader(message: ConfigureErrorMissingKeys, reader: jspb.BinaryReader): ConfigureErrorMissingKeys;
}

export namespace ConfigureErrorMissingKeys {
    export type AsObject = {
        missingkeysList: Array<ConfigureErrorMissingKeys.MissingKey.AsObject>,
    }


    export class MissingKey extends jspb.Message { 
        getName(): string;
        setName(value: string): MissingKey;
        getDescription(): string;
        setDescription(value: string): MissingKey;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): MissingKey.AsObject;
        static toObject(includeInstance: boolean, msg: MissingKey): MissingKey.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: MissingKey, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): MissingKey;
        static deserializeBinaryFromReader(message: MissingKey, reader: jspb.BinaryReader): MissingKey;
    }

    export namespace MissingKey {
        export type AsObject = {
            name: string,
            description: string,
        }
    }

}

export class InvokeRequest extends jspb.Message { 
    getTok(): string;
    setTok(value: string): InvokeRequest;

    hasArgs(): boolean;
    clearArgs(): void;
    getArgs(): google_protobuf_struct_pb.Struct | undefined;
    setArgs(value?: google_protobuf_struct_pb.Struct): InvokeRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): InvokeRequest.AsObject;
    static toObject(includeInstance: boolean, msg: InvokeRequest): InvokeRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: InvokeRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): InvokeRequest;
    static deserializeBinaryFromReader(message: InvokeRequest, reader: jspb.BinaryReader): InvokeRequest;
}

export namespace InvokeRequest {
    export type AsObject = {
        tok: string,
        args?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class InvokeResponse extends jspb.Message { 

    hasReturn(): boolean;
    clearReturn(): void;
    getReturn(): google_protobuf_struct_pb.Struct | undefined;
    setReturn(value?: google_protobuf_struct_pb.Struct): InvokeResponse;
    clearFailuresList(): void;
    getFailuresList(): Array<CheckFailure>;
    setFailuresList(value: Array<CheckFailure>): InvokeResponse;
    addFailures(value?: CheckFailure, index?: number): CheckFailure;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): InvokeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: InvokeResponse): InvokeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: InvokeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): InvokeResponse;
    static deserializeBinaryFromReader(message: InvokeResponse, reader: jspb.BinaryReader): InvokeResponse;
}

export namespace InvokeResponse {
    export type AsObject = {
        pb_return?: google_protobuf_struct_pb.Struct.AsObject,
        failuresList: Array<CheckFailure.AsObject>,
    }
}

export class CallRequest extends jspb.Message { 
    getTok(): string;
    setTok(value: string): CallRequest;

    hasArgs(): boolean;
    clearArgs(): void;
    getArgs(): google_protobuf_struct_pb.Struct | undefined;
    setArgs(value?: google_protobuf_struct_pb.Struct): CallRequest;

    getArgdependenciesMap(): jspb.Map<string, CallRequest.ArgumentDependencies>;
    clearArgdependenciesMap(): void;
    getProject(): string;
    setProject(value: string): CallRequest;
    getStack(): string;
    setStack(value: string): CallRequest;

    getConfigMap(): jspb.Map<string, string>;
    clearConfigMap(): void;
    clearConfigsecretkeysList(): void;
    getConfigsecretkeysList(): Array<string>;
    setConfigsecretkeysList(value: Array<string>): CallRequest;
    addConfigsecretkeys(value: string, index?: number): string;
    getDryrun(): boolean;
    setDryrun(value: boolean): CallRequest;
    getParallel(): number;
    setParallel(value: number): CallRequest;
    getMonitorendpoint(): string;
    setMonitorendpoint(value: string): CallRequest;
    getOrganization(): string;
    setOrganization(value: string): CallRequest;
    getAcceptsOutputValues(): boolean;
    setAcceptsOutputValues(value: boolean): CallRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CallRequest.AsObject;
    static toObject(includeInstance: boolean, msg: CallRequest): CallRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CallRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CallRequest;
    static deserializeBinaryFromReader(message: CallRequest, reader: jspb.BinaryReader): CallRequest;
}

export namespace CallRequest {
    export type AsObject = {
        tok: string,
        args?: google_protobuf_struct_pb.Struct.AsObject,

        argdependenciesMap: Array<[string, CallRequest.ArgumentDependencies.AsObject]>,
        project: string,
        stack: string,

        configMap: Array<[string, string]>,
        configsecretkeysList: Array<string>,
        dryrun: boolean,
        parallel: number,
        monitorendpoint: string,
        organization: string,
        acceptsOutputValues: boolean,
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

export class CallResponse extends jspb.Message { 

    hasReturn(): boolean;
    clearReturn(): void;
    getReturn(): google_protobuf_struct_pb.Struct | undefined;
    setReturn(value?: google_protobuf_struct_pb.Struct): CallResponse;
    clearFailuresList(): void;
    getFailuresList(): Array<CheckFailure>;
    setFailuresList(value: Array<CheckFailure>): CallResponse;
    addFailures(value?: CheckFailure, index?: number): CheckFailure;

    getReturndependenciesMap(): jspb.Map<string, CallResponse.ReturnDependencies>;
    clearReturndependenciesMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CallResponse.AsObject;
    static toObject(includeInstance: boolean, msg: CallResponse): CallResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CallResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CallResponse;
    static deserializeBinaryFromReader(message: CallResponse, reader: jspb.BinaryReader): CallResponse;
}

export namespace CallResponse {
    export type AsObject = {
        pb_return?: google_protobuf_struct_pb.Struct.AsObject,
        failuresList: Array<CheckFailure.AsObject>,

        returndependenciesMap: Array<[string, CallResponse.ReturnDependencies.AsObject]>,
    }


    export class ReturnDependencies extends jspb.Message { 
        clearUrnsList(): void;
        getUrnsList(): Array<string>;
        setUrnsList(value: Array<string>): ReturnDependencies;
        addUrns(value: string, index?: number): string;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): ReturnDependencies.AsObject;
        static toObject(includeInstance: boolean, msg: ReturnDependencies): ReturnDependencies.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: ReturnDependencies, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): ReturnDependencies;
        static deserializeBinaryFromReader(message: ReturnDependencies, reader: jspb.BinaryReader): ReturnDependencies;
    }

    export namespace ReturnDependencies {
        export type AsObject = {
            urnsList: Array<string>,
        }
    }

}

export class CheckRequest extends jspb.Message { 
    getUrn(): string;
    setUrn(value: string): CheckRequest;

    hasOlds(): boolean;
    clearOlds(): void;
    getOlds(): google_protobuf_struct_pb.Struct | undefined;
    setOlds(value?: google_protobuf_struct_pb.Struct): CheckRequest;

    hasNews(): boolean;
    clearNews(): void;
    getNews(): google_protobuf_struct_pb.Struct | undefined;
    setNews(value?: google_protobuf_struct_pb.Struct): CheckRequest;
    getRandomseed(): Uint8Array | string;
    getRandomseed_asU8(): Uint8Array;
    getRandomseed_asB64(): string;
    setRandomseed(value: Uint8Array | string): CheckRequest;
    getName(): string;
    setName(value: string): CheckRequest;
    getType(): string;
    setType(value: string): CheckRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CheckRequest.AsObject;
    static toObject(includeInstance: boolean, msg: CheckRequest): CheckRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CheckRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CheckRequest;
    static deserializeBinaryFromReader(message: CheckRequest, reader: jspb.BinaryReader): CheckRequest;
}

export namespace CheckRequest {
    export type AsObject = {
        urn: string,
        olds?: google_protobuf_struct_pb.Struct.AsObject,
        news?: google_protobuf_struct_pb.Struct.AsObject,
        randomseed: Uint8Array | string,
        name: string,
        type: string,
    }
}

export class CheckResponse extends jspb.Message { 

    hasInputs(): boolean;
    clearInputs(): void;
    getInputs(): google_protobuf_struct_pb.Struct | undefined;
    setInputs(value?: google_protobuf_struct_pb.Struct): CheckResponse;
    clearFailuresList(): void;
    getFailuresList(): Array<CheckFailure>;
    setFailuresList(value: Array<CheckFailure>): CheckResponse;
    addFailures(value?: CheckFailure, index?: number): CheckFailure;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CheckResponse.AsObject;
    static toObject(includeInstance: boolean, msg: CheckResponse): CheckResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CheckResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CheckResponse;
    static deserializeBinaryFromReader(message: CheckResponse, reader: jspb.BinaryReader): CheckResponse;
}

export namespace CheckResponse {
    export type AsObject = {
        inputs?: google_protobuf_struct_pb.Struct.AsObject,
        failuresList: Array<CheckFailure.AsObject>,
    }
}

export class CheckFailure extends jspb.Message { 
    getProperty(): string;
    setProperty(value: string): CheckFailure;
    getReason(): string;
    setReason(value: string): CheckFailure;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CheckFailure.AsObject;
    static toObject(includeInstance: boolean, msg: CheckFailure): CheckFailure.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CheckFailure, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CheckFailure;
    static deserializeBinaryFromReader(message: CheckFailure, reader: jspb.BinaryReader): CheckFailure;
}

export namespace CheckFailure {
    export type AsObject = {
        property: string,
        reason: string,
    }
}

export class DiffRequest extends jspb.Message { 
    getId(): string;
    setId(value: string): DiffRequest;
    getUrn(): string;
    setUrn(value: string): DiffRequest;

    hasOlds(): boolean;
    clearOlds(): void;
    getOlds(): google_protobuf_struct_pb.Struct | undefined;
    setOlds(value?: google_protobuf_struct_pb.Struct): DiffRequest;

    hasNews(): boolean;
    clearNews(): void;
    getNews(): google_protobuf_struct_pb.Struct | undefined;
    setNews(value?: google_protobuf_struct_pb.Struct): DiffRequest;
    clearIgnorechangesList(): void;
    getIgnorechangesList(): Array<string>;
    setIgnorechangesList(value: Array<string>): DiffRequest;
    addIgnorechanges(value: string, index?: number): string;

    hasOldInputs(): boolean;
    clearOldInputs(): void;
    getOldInputs(): google_protobuf_struct_pb.Struct | undefined;
    setOldInputs(value?: google_protobuf_struct_pb.Struct): DiffRequest;
    getName(): string;
    setName(value: string): DiffRequest;
    getType(): string;
    setType(value: string): DiffRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DiffRequest.AsObject;
    static toObject(includeInstance: boolean, msg: DiffRequest): DiffRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DiffRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DiffRequest;
    static deserializeBinaryFromReader(message: DiffRequest, reader: jspb.BinaryReader): DiffRequest;
}

export namespace DiffRequest {
    export type AsObject = {
        id: string,
        urn: string,
        olds?: google_protobuf_struct_pb.Struct.AsObject,
        news?: google_protobuf_struct_pb.Struct.AsObject,
        ignorechangesList: Array<string>,
        oldInputs?: google_protobuf_struct_pb.Struct.AsObject,
        name: string,
        type: string,
    }
}

export class PropertyDiff extends jspb.Message { 
    getKind(): PropertyDiff.Kind;
    setKind(value: PropertyDiff.Kind): PropertyDiff;
    getInputdiff(): boolean;
    setInputdiff(value: boolean): PropertyDiff;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PropertyDiff.AsObject;
    static toObject(includeInstance: boolean, msg: PropertyDiff): PropertyDiff.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PropertyDiff, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PropertyDiff;
    static deserializeBinaryFromReader(message: PropertyDiff, reader: jspb.BinaryReader): PropertyDiff;
}

export namespace PropertyDiff {
    export type AsObject = {
        kind: PropertyDiff.Kind,
        inputdiff: boolean,
    }

    export enum Kind {
    ADD = 0,
    ADD_REPLACE = 1,
    DELETE = 2,
    DELETE_REPLACE = 3,
    UPDATE = 4,
    UPDATE_REPLACE = 5,
    }

}

export class DiffResponse extends jspb.Message { 
    clearReplacesList(): void;
    getReplacesList(): Array<string>;
    setReplacesList(value: Array<string>): DiffResponse;
    addReplaces(value: string, index?: number): string;
    clearStablesList(): void;
    getStablesList(): Array<string>;
    setStablesList(value: Array<string>): DiffResponse;
    addStables(value: string, index?: number): string;
    getDeletebeforereplace(): boolean;
    setDeletebeforereplace(value: boolean): DiffResponse;
    getChanges(): DiffResponse.DiffChanges;
    setChanges(value: DiffResponse.DiffChanges): DiffResponse;
    clearDiffsList(): void;
    getDiffsList(): Array<string>;
    setDiffsList(value: Array<string>): DiffResponse;
    addDiffs(value: string, index?: number): string;

    getDetaileddiffMap(): jspb.Map<string, PropertyDiff>;
    clearDetaileddiffMap(): void;
    getHasdetaileddiff(): boolean;
    setHasdetaileddiff(value: boolean): DiffResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DiffResponse.AsObject;
    static toObject(includeInstance: boolean, msg: DiffResponse): DiffResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DiffResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DiffResponse;
    static deserializeBinaryFromReader(message: DiffResponse, reader: jspb.BinaryReader): DiffResponse;
}

export namespace DiffResponse {
    export type AsObject = {
        replacesList: Array<string>,
        stablesList: Array<string>,
        deletebeforereplace: boolean,
        changes: DiffResponse.DiffChanges,
        diffsList: Array<string>,

        detaileddiffMap: Array<[string, PropertyDiff.AsObject]>,
        hasdetaileddiff: boolean,
    }

    export enum DiffChanges {
    DIFF_UNKNOWN = 0,
    DIFF_NONE = 1,
    DIFF_SOME = 2,
    }

}

export class CreateRequest extends jspb.Message { 
    getUrn(): string;
    setUrn(value: string): CreateRequest;

    hasProperties(): boolean;
    clearProperties(): void;
    getProperties(): google_protobuf_struct_pb.Struct | undefined;
    setProperties(value?: google_protobuf_struct_pb.Struct): CreateRequest;
    getTimeout(): number;
    setTimeout(value: number): CreateRequest;
    getPreview(): boolean;
    setPreview(value: boolean): CreateRequest;
    getName(): string;
    setName(value: string): CreateRequest;
    getType(): string;
    setType(value: string): CreateRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CreateRequest.AsObject;
    static toObject(includeInstance: boolean, msg: CreateRequest): CreateRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CreateRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CreateRequest;
    static deserializeBinaryFromReader(message: CreateRequest, reader: jspb.BinaryReader): CreateRequest;
}

export namespace CreateRequest {
    export type AsObject = {
        urn: string,
        properties?: google_protobuf_struct_pb.Struct.AsObject,
        timeout: number,
        preview: boolean,
        name: string,
        type: string,
    }
}

export class CreateResponse extends jspb.Message { 
    getId(): string;
    setId(value: string): CreateResponse;

    hasProperties(): boolean;
    clearProperties(): void;
    getProperties(): google_protobuf_struct_pb.Struct | undefined;
    setProperties(value?: google_protobuf_struct_pb.Struct): CreateResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CreateResponse.AsObject;
    static toObject(includeInstance: boolean, msg: CreateResponse): CreateResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CreateResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CreateResponse;
    static deserializeBinaryFromReader(message: CreateResponse, reader: jspb.BinaryReader): CreateResponse;
}

export namespace CreateResponse {
    export type AsObject = {
        id: string,
        properties?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class ReadRequest extends jspb.Message { 
    getId(): string;
    setId(value: string): ReadRequest;
    getUrn(): string;
    setUrn(value: string): ReadRequest;

    hasProperties(): boolean;
    clearProperties(): void;
    getProperties(): google_protobuf_struct_pb.Struct | undefined;
    setProperties(value?: google_protobuf_struct_pb.Struct): ReadRequest;

    hasInputs(): boolean;
    clearInputs(): void;
    getInputs(): google_protobuf_struct_pb.Struct | undefined;
    setInputs(value?: google_protobuf_struct_pb.Struct): ReadRequest;
    getName(): string;
    setName(value: string): ReadRequest;
    getType(): string;
    setType(value: string): ReadRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ReadRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ReadRequest): ReadRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ReadRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ReadRequest;
    static deserializeBinaryFromReader(message: ReadRequest, reader: jspb.BinaryReader): ReadRequest;
}

export namespace ReadRequest {
    export type AsObject = {
        id: string,
        urn: string,
        properties?: google_protobuf_struct_pb.Struct.AsObject,
        inputs?: google_protobuf_struct_pb.Struct.AsObject,
        name: string,
        type: string,
    }
}

export class ReadResponse extends jspb.Message { 
    getId(): string;
    setId(value: string): ReadResponse;

    hasProperties(): boolean;
    clearProperties(): void;
    getProperties(): google_protobuf_struct_pb.Struct | undefined;
    setProperties(value?: google_protobuf_struct_pb.Struct): ReadResponse;

    hasInputs(): boolean;
    clearInputs(): void;
    getInputs(): google_protobuf_struct_pb.Struct | undefined;
    setInputs(value?: google_protobuf_struct_pb.Struct): ReadResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ReadResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ReadResponse): ReadResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ReadResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ReadResponse;
    static deserializeBinaryFromReader(message: ReadResponse, reader: jspb.BinaryReader): ReadResponse;
}

export namespace ReadResponse {
    export type AsObject = {
        id: string,
        properties?: google_protobuf_struct_pb.Struct.AsObject,
        inputs?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class UpdateRequest extends jspb.Message { 
    getId(): string;
    setId(value: string): UpdateRequest;
    getUrn(): string;
    setUrn(value: string): UpdateRequest;

    hasOlds(): boolean;
    clearOlds(): void;
    getOlds(): google_protobuf_struct_pb.Struct | undefined;
    setOlds(value?: google_protobuf_struct_pb.Struct): UpdateRequest;

    hasNews(): boolean;
    clearNews(): void;
    getNews(): google_protobuf_struct_pb.Struct | undefined;
    setNews(value?: google_protobuf_struct_pb.Struct): UpdateRequest;
    getTimeout(): number;
    setTimeout(value: number): UpdateRequest;
    clearIgnorechangesList(): void;
    getIgnorechangesList(): Array<string>;
    setIgnorechangesList(value: Array<string>): UpdateRequest;
    addIgnorechanges(value: string, index?: number): string;
    getPreview(): boolean;
    setPreview(value: boolean): UpdateRequest;

    hasOldInputs(): boolean;
    clearOldInputs(): void;
    getOldInputs(): google_protobuf_struct_pb.Struct | undefined;
    setOldInputs(value?: google_protobuf_struct_pb.Struct): UpdateRequest;
    getName(): string;
    setName(value: string): UpdateRequest;
    getType(): string;
    setType(value: string): UpdateRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UpdateRequest.AsObject;
    static toObject(includeInstance: boolean, msg: UpdateRequest): UpdateRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UpdateRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UpdateRequest;
    static deserializeBinaryFromReader(message: UpdateRequest, reader: jspb.BinaryReader): UpdateRequest;
}

export namespace UpdateRequest {
    export type AsObject = {
        id: string,
        urn: string,
        olds?: google_protobuf_struct_pb.Struct.AsObject,
        news?: google_protobuf_struct_pb.Struct.AsObject,
        timeout: number,
        ignorechangesList: Array<string>,
        preview: boolean,
        oldInputs?: google_protobuf_struct_pb.Struct.AsObject,
        name: string,
        type: string,
    }
}

export class UpdateResponse extends jspb.Message { 

    hasProperties(): boolean;
    clearProperties(): void;
    getProperties(): google_protobuf_struct_pb.Struct | undefined;
    setProperties(value?: google_protobuf_struct_pb.Struct): UpdateResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UpdateResponse.AsObject;
    static toObject(includeInstance: boolean, msg: UpdateResponse): UpdateResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UpdateResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UpdateResponse;
    static deserializeBinaryFromReader(message: UpdateResponse, reader: jspb.BinaryReader): UpdateResponse;
}

export namespace UpdateResponse {
    export type AsObject = {
        properties?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class DeleteRequest extends jspb.Message { 
    getId(): string;
    setId(value: string): DeleteRequest;
    getUrn(): string;
    setUrn(value: string): DeleteRequest;

    hasProperties(): boolean;
    clearProperties(): void;
    getProperties(): google_protobuf_struct_pb.Struct | undefined;
    setProperties(value?: google_protobuf_struct_pb.Struct): DeleteRequest;
    getTimeout(): number;
    setTimeout(value: number): DeleteRequest;

    hasOldInputs(): boolean;
    clearOldInputs(): void;
    getOldInputs(): google_protobuf_struct_pb.Struct | undefined;
    setOldInputs(value?: google_protobuf_struct_pb.Struct): DeleteRequest;
    getName(): string;
    setName(value: string): DeleteRequest;
    getType(): string;
    setType(value: string): DeleteRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeleteRequest.AsObject;
    static toObject(includeInstance: boolean, msg: DeleteRequest): DeleteRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeleteRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeleteRequest;
    static deserializeBinaryFromReader(message: DeleteRequest, reader: jspb.BinaryReader): DeleteRequest;
}

export namespace DeleteRequest {
    export type AsObject = {
        id: string,
        urn: string,
        properties?: google_protobuf_struct_pb.Struct.AsObject,
        timeout: number,
        oldInputs?: google_protobuf_struct_pb.Struct.AsObject,
        name: string,
        type: string,
    }
}

export class ConstructRequest extends jspb.Message { 
    getProject(): string;
    setProject(value: string): ConstructRequest;
    getStack(): string;
    setStack(value: string): ConstructRequest;

    getConfigMap(): jspb.Map<string, string>;
    clearConfigMap(): void;
    getDryrun(): boolean;
    setDryrun(value: boolean): ConstructRequest;
    getParallel(): number;
    setParallel(value: number): ConstructRequest;
    getMonitorendpoint(): string;
    setMonitorendpoint(value: string): ConstructRequest;
    getType(): string;
    setType(value: string): ConstructRequest;
    getName(): string;
    setName(value: string): ConstructRequest;
    getParent(): string;
    setParent(value: string): ConstructRequest;

    hasInputs(): boolean;
    clearInputs(): void;
    getInputs(): google_protobuf_struct_pb.Struct | undefined;
    setInputs(value?: google_protobuf_struct_pb.Struct): ConstructRequest;

    getInputdependenciesMap(): jspb.Map<string, ConstructRequest.PropertyDependencies>;
    clearInputdependenciesMap(): void;

    getProvidersMap(): jspb.Map<string, string>;
    clearProvidersMap(): void;
    clearDependenciesList(): void;
    getDependenciesList(): Array<string>;
    setDependenciesList(value: Array<string>): ConstructRequest;
    addDependencies(value: string, index?: number): string;
    clearConfigsecretkeysList(): void;
    getConfigsecretkeysList(): Array<string>;
    setConfigsecretkeysList(value: Array<string>): ConstructRequest;
    addConfigsecretkeys(value: string, index?: number): string;
    getOrganization(): string;
    setOrganization(value: string): ConstructRequest;
    getProtect(): boolean;
    setProtect(value: boolean): ConstructRequest;
    clearAliasesList(): void;
    getAliasesList(): Array<string>;
    setAliasesList(value: Array<string>): ConstructRequest;
    addAliases(value: string, index?: number): string;
    clearAdditionalsecretoutputsList(): void;
    getAdditionalsecretoutputsList(): Array<string>;
    setAdditionalsecretoutputsList(value: Array<string>): ConstructRequest;
    addAdditionalsecretoutputs(value: string, index?: number): string;

    hasCustomtimeouts(): boolean;
    clearCustomtimeouts(): void;
    getCustomtimeouts(): ConstructRequest.CustomTimeouts | undefined;
    setCustomtimeouts(value?: ConstructRequest.CustomTimeouts): ConstructRequest;
    getDeletedwith(): string;
    setDeletedwith(value: string): ConstructRequest;
    getDeletebeforereplace(): boolean;
    setDeletebeforereplace(value: boolean): ConstructRequest;
    clearIgnorechangesList(): void;
    getIgnorechangesList(): Array<string>;
    setIgnorechangesList(value: Array<string>): ConstructRequest;
    addIgnorechanges(value: string, index?: number): string;
    clearReplaceonchangesList(): void;
    getReplaceonchangesList(): Array<string>;
    setReplaceonchangesList(value: Array<string>): ConstructRequest;
    addReplaceonchanges(value: string, index?: number): string;
    getRetainondelete(): boolean;
    setRetainondelete(value: boolean): ConstructRequest;
    getAcceptsOutputValues(): boolean;
    setAcceptsOutputValues(value: boolean): ConstructRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConstructRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ConstructRequest): ConstructRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConstructRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConstructRequest;
    static deserializeBinaryFromReader(message: ConstructRequest, reader: jspb.BinaryReader): ConstructRequest;
}

export namespace ConstructRequest {
    export type AsObject = {
        project: string,
        stack: string,

        configMap: Array<[string, string]>,
        dryrun: boolean,
        parallel: number,
        monitorendpoint: string,
        type: string,
        name: string,
        parent: string,
        inputs?: google_protobuf_struct_pb.Struct.AsObject,

        inputdependenciesMap: Array<[string, ConstructRequest.PropertyDependencies.AsObject]>,

        providersMap: Array<[string, string]>,
        dependenciesList: Array<string>,
        configsecretkeysList: Array<string>,
        organization: string,
        protect: boolean,
        aliasesList: Array<string>,
        additionalsecretoutputsList: Array<string>,
        customtimeouts?: ConstructRequest.CustomTimeouts.AsObject,
        deletedwith: string,
        deletebeforereplace: boolean,
        ignorechangesList: Array<string>,
        replaceonchangesList: Array<string>,
        retainondelete: boolean,
        acceptsOutputValues: boolean,
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

export class ConstructResponse extends jspb.Message { 
    getUrn(): string;
    setUrn(value: string): ConstructResponse;

    hasState(): boolean;
    clearState(): void;
    getState(): google_protobuf_struct_pb.Struct | undefined;
    setState(value?: google_protobuf_struct_pb.Struct): ConstructResponse;

    getStatedependenciesMap(): jspb.Map<string, ConstructResponse.PropertyDependencies>;
    clearStatedependenciesMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConstructResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ConstructResponse): ConstructResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConstructResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConstructResponse;
    static deserializeBinaryFromReader(message: ConstructResponse, reader: jspb.BinaryReader): ConstructResponse;
}

export namespace ConstructResponse {
    export type AsObject = {
        urn: string,
        state?: google_protobuf_struct_pb.Struct.AsObject,

        statedependenciesMap: Array<[string, ConstructResponse.PropertyDependencies.AsObject]>,
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

export class ErrorResourceInitFailed extends jspb.Message { 
    getId(): string;
    setId(value: string): ErrorResourceInitFailed;

    hasProperties(): boolean;
    clearProperties(): void;
    getProperties(): google_protobuf_struct_pb.Struct | undefined;
    setProperties(value?: google_protobuf_struct_pb.Struct): ErrorResourceInitFailed;
    clearReasonsList(): void;
    getReasonsList(): Array<string>;
    setReasonsList(value: Array<string>): ErrorResourceInitFailed;
    addReasons(value: string, index?: number): string;

    hasInputs(): boolean;
    clearInputs(): void;
    getInputs(): google_protobuf_struct_pb.Struct | undefined;
    setInputs(value?: google_protobuf_struct_pb.Struct): ErrorResourceInitFailed;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ErrorResourceInitFailed.AsObject;
    static toObject(includeInstance: boolean, msg: ErrorResourceInitFailed): ErrorResourceInitFailed.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ErrorResourceInitFailed, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ErrorResourceInitFailed;
    static deserializeBinaryFromReader(message: ErrorResourceInitFailed, reader: jspb.BinaryReader): ErrorResourceInitFailed;
}

export namespace ErrorResourceInitFailed {
    export type AsObject = {
        id: string,
        properties?: google_protobuf_struct_pb.Struct.AsObject,
        reasonsList: Array<string>,
        inputs?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class GetMappingRequest extends jspb.Message { 
    getKey(): string;
    setKey(value: string): GetMappingRequest;
    getProvider(): string;
    setProvider(value: string): GetMappingRequest;

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
        key: string,
        provider: string,
    }
}

export class GetMappingResponse extends jspb.Message { 
    getProvider(): string;
    setProvider(value: string): GetMappingResponse;
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
        provider: string,
        data: Uint8Array | string,
    }
}

export class GetMappingsRequest extends jspb.Message { 
    getKey(): string;
    setKey(value: string): GetMappingsRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetMappingsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetMappingsRequest): GetMappingsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetMappingsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetMappingsRequest;
    static deserializeBinaryFromReader(message: GetMappingsRequest, reader: jspb.BinaryReader): GetMappingsRequest;
}

export namespace GetMappingsRequest {
    export type AsObject = {
        key: string,
    }
}

export class GetMappingsResponse extends jspb.Message { 
    clearProvidersList(): void;
    getProvidersList(): Array<string>;
    setProvidersList(value: Array<string>): GetMappingsResponse;
    addProviders(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetMappingsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetMappingsResponse): GetMappingsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetMappingsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetMappingsResponse;
    static deserializeBinaryFromReader(message: GetMappingsResponse, reader: jspb.BinaryReader): GetMappingsResponse;
}

export namespace GetMappingsResponse {
    export type AsObject = {
        providersList: Array<string>,
    }
}
