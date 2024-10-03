// package: pulumirpc
// file: pulumi/language.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as pulumi_codegen_hcl_pb from "./codegen/hcl_pb";
import * as pulumi_plugin_pb from "./plugin_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

export class ProgramInfo extends jspb.Message { 
    getRootDirectory(): string;
    setRootDirectory(value: string): ProgramInfo;
    getProgramDirectory(): string;
    setProgramDirectory(value: string): ProgramInfo;
    getEntryPoint(): string;
    setEntryPoint(value: string): ProgramInfo;

    hasOptions(): boolean;
    clearOptions(): void;
    getOptions(): google_protobuf_struct_pb.Struct | undefined;
    setOptions(value?: google_protobuf_struct_pb.Struct): ProgramInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ProgramInfo.AsObject;
    static toObject(includeInstance: boolean, msg: ProgramInfo): ProgramInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ProgramInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ProgramInfo;
    static deserializeBinaryFromReader(message: ProgramInfo, reader: jspb.BinaryReader): ProgramInfo;
}

export namespace ProgramInfo {
    export type AsObject = {
        rootDirectory: string,
        programDirectory: string,
        entryPoint: string,
        options?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class AboutRequest extends jspb.Message { 

    hasInfo(): boolean;
    clearInfo(): void;
    getInfo(): ProgramInfo | undefined;
    setInfo(value?: ProgramInfo): AboutRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AboutRequest.AsObject;
    static toObject(includeInstance: boolean, msg: AboutRequest): AboutRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AboutRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AboutRequest;
    static deserializeBinaryFromReader(message: AboutRequest, reader: jspb.BinaryReader): AboutRequest;
}

export namespace AboutRequest {
    export type AsObject = {
        info?: ProgramInfo.AsObject,
    }
}

export class AboutResponse extends jspb.Message { 
    getExecutable(): string;
    setExecutable(value: string): AboutResponse;
    getVersion(): string;
    setVersion(value: string): AboutResponse;

    getMetadataMap(): jspb.Map<string, string>;
    clearMetadataMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AboutResponse.AsObject;
    static toObject(includeInstance: boolean, msg: AboutResponse): AboutResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AboutResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AboutResponse;
    static deserializeBinaryFromReader(message: AboutResponse, reader: jspb.BinaryReader): AboutResponse;
}

export namespace AboutResponse {
    export type AsObject = {
        executable: string,
        version: string,

        metadataMap: Array<[string, string]>,
    }
}

export class GetProgramDependenciesRequest extends jspb.Message { 
    getProject(): string;
    setProject(value: string): GetProgramDependenciesRequest;
    getPwd(): string;
    setPwd(value: string): GetProgramDependenciesRequest;
    getProgram(): string;
    setProgram(value: string): GetProgramDependenciesRequest;
    getTransitivedependencies(): boolean;
    setTransitivedependencies(value: boolean): GetProgramDependenciesRequest;

    hasInfo(): boolean;
    clearInfo(): void;
    getInfo(): ProgramInfo | undefined;
    setInfo(value?: ProgramInfo): GetProgramDependenciesRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetProgramDependenciesRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetProgramDependenciesRequest): GetProgramDependenciesRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetProgramDependenciesRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetProgramDependenciesRequest;
    static deserializeBinaryFromReader(message: GetProgramDependenciesRequest, reader: jspb.BinaryReader): GetProgramDependenciesRequest;
}

export namespace GetProgramDependenciesRequest {
    export type AsObject = {
        project: string,
        pwd: string,
        program: string,
        transitivedependencies: boolean,
        info?: ProgramInfo.AsObject,
    }
}

export class DependencyInfo extends jspb.Message { 
    getName(): string;
    setName(value: string): DependencyInfo;
    getVersion(): string;
    setVersion(value: string): DependencyInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DependencyInfo.AsObject;
    static toObject(includeInstance: boolean, msg: DependencyInfo): DependencyInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DependencyInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DependencyInfo;
    static deserializeBinaryFromReader(message: DependencyInfo, reader: jspb.BinaryReader): DependencyInfo;
}

export namespace DependencyInfo {
    export type AsObject = {
        name: string,
        version: string,
    }
}

export class GetProgramDependenciesResponse extends jspb.Message { 
    clearDependenciesList(): void;
    getDependenciesList(): Array<DependencyInfo>;
    setDependenciesList(value: Array<DependencyInfo>): GetProgramDependenciesResponse;
    addDependencies(value?: DependencyInfo, index?: number): DependencyInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetProgramDependenciesResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetProgramDependenciesResponse): GetProgramDependenciesResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetProgramDependenciesResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetProgramDependenciesResponse;
    static deserializeBinaryFromReader(message: GetProgramDependenciesResponse, reader: jspb.BinaryReader): GetProgramDependenciesResponse;
}

export namespace GetProgramDependenciesResponse {
    export type AsObject = {
        dependenciesList: Array<DependencyInfo.AsObject>,
    }
}

export class GetRequiredPluginsRequest extends jspb.Message { 
    getProject(): string;
    setProject(value: string): GetRequiredPluginsRequest;
    getPwd(): string;
    setPwd(value: string): GetRequiredPluginsRequest;
    getProgram(): string;
    setProgram(value: string): GetRequiredPluginsRequest;

    hasInfo(): boolean;
    clearInfo(): void;
    getInfo(): ProgramInfo | undefined;
    setInfo(value?: ProgramInfo): GetRequiredPluginsRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetRequiredPluginsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetRequiredPluginsRequest): GetRequiredPluginsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetRequiredPluginsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetRequiredPluginsRequest;
    static deserializeBinaryFromReader(message: GetRequiredPluginsRequest, reader: jspb.BinaryReader): GetRequiredPluginsRequest;
}

export namespace GetRequiredPluginsRequest {
    export type AsObject = {
        project: string,
        pwd: string,
        program: string,
        info?: ProgramInfo.AsObject,
    }
}

export class GetRequiredPluginsResponse extends jspb.Message { 
    clearPluginsList(): void;
    getPluginsList(): Array<pulumi_plugin_pb.PluginDependency>;
    setPluginsList(value: Array<pulumi_plugin_pb.PluginDependency>): GetRequiredPluginsResponse;
    addPlugins(value?: pulumi_plugin_pb.PluginDependency, index?: number): pulumi_plugin_pb.PluginDependency;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetRequiredPluginsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetRequiredPluginsResponse): GetRequiredPluginsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetRequiredPluginsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetRequiredPluginsResponse;
    static deserializeBinaryFromReader(message: GetRequiredPluginsResponse, reader: jspb.BinaryReader): GetRequiredPluginsResponse;
}

export namespace GetRequiredPluginsResponse {
    export type AsObject = {
        pluginsList: Array<pulumi_plugin_pb.PluginDependency.AsObject>,
    }
}

export class RunRequest extends jspb.Message { 
    getProject(): string;
    setProject(value: string): RunRequest;
    getStack(): string;
    setStack(value: string): RunRequest;
    getPwd(): string;
    setPwd(value: string): RunRequest;
    getProgram(): string;
    setProgram(value: string): RunRequest;
    clearArgsList(): void;
    getArgsList(): Array<string>;
    setArgsList(value: Array<string>): RunRequest;
    addArgs(value: string, index?: number): string;

    getConfigMap(): jspb.Map<string, string>;
    clearConfigMap(): void;
    getDryrun(): boolean;
    setDryrun(value: boolean): RunRequest;
    getParallel(): number;
    setParallel(value: number): RunRequest;
    getMonitorAddress(): string;
    setMonitorAddress(value: string): RunRequest;
    getQuerymode(): boolean;
    setQuerymode(value: boolean): RunRequest;
    clearConfigsecretkeysList(): void;
    getConfigsecretkeysList(): Array<string>;
    setConfigsecretkeysList(value: Array<string>): RunRequest;
    addConfigsecretkeys(value: string, index?: number): string;
    getOrganization(): string;
    setOrganization(value: string): RunRequest;

    hasConfigpropertymap(): boolean;
    clearConfigpropertymap(): void;
    getConfigpropertymap(): google_protobuf_struct_pb.Struct | undefined;
    setConfigpropertymap(value?: google_protobuf_struct_pb.Struct): RunRequest;

    hasInfo(): boolean;
    clearInfo(): void;
    getInfo(): ProgramInfo | undefined;
    setInfo(value?: ProgramInfo): RunRequest;
    getLoaderTarget(): string;
    setLoaderTarget(value: string): RunRequest;
    getAttachDebugger(): boolean;
    setAttachDebugger(value: boolean): RunRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RunRequest): RunRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunRequest;
    static deserializeBinaryFromReader(message: RunRequest, reader: jspb.BinaryReader): RunRequest;
}

export namespace RunRequest {
    export type AsObject = {
        project: string,
        stack: string,
        pwd: string,
        program: string,
        argsList: Array<string>,

        configMap: Array<[string, string]>,
        dryrun: boolean,
        parallel: number,
        monitorAddress: string,
        querymode: boolean,
        configsecretkeysList: Array<string>,
        organization: string,
        configpropertymap?: google_protobuf_struct_pb.Struct.AsObject,
        info?: ProgramInfo.AsObject,
        loaderTarget: string,
        attachDebugger: boolean,
    }
}

export class RunResponse extends jspb.Message { 
    getError(): string;
    setError(value: string): RunResponse;
    getBail(): boolean;
    setBail(value: boolean): RunResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RunResponse): RunResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunResponse;
    static deserializeBinaryFromReader(message: RunResponse, reader: jspb.BinaryReader): RunResponse;
}

export namespace RunResponse {
    export type AsObject = {
        error: string,
        bail: boolean,
    }
}

export class InstallDependenciesRequest extends jspb.Message { 
    getDirectory(): string;
    setDirectory(value: string): InstallDependenciesRequest;
    getIsTerminal(): boolean;
    setIsTerminal(value: boolean): InstallDependenciesRequest;

    hasInfo(): boolean;
    clearInfo(): void;
    getInfo(): ProgramInfo | undefined;
    setInfo(value?: ProgramInfo): InstallDependenciesRequest;
    getUseLanguageVersionTools(): boolean;
    setUseLanguageVersionTools(value: boolean): InstallDependenciesRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): InstallDependenciesRequest.AsObject;
    static toObject(includeInstance: boolean, msg: InstallDependenciesRequest): InstallDependenciesRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: InstallDependenciesRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): InstallDependenciesRequest;
    static deserializeBinaryFromReader(message: InstallDependenciesRequest, reader: jspb.BinaryReader): InstallDependenciesRequest;
}

export namespace InstallDependenciesRequest {
    export type AsObject = {
        directory: string,
        isTerminal: boolean,
        info?: ProgramInfo.AsObject,
        useLanguageVersionTools: boolean,
    }
}

export class InstallDependenciesResponse extends jspb.Message { 
    getStdout(): Uint8Array | string;
    getStdout_asU8(): Uint8Array;
    getStdout_asB64(): string;
    setStdout(value: Uint8Array | string): InstallDependenciesResponse;
    getStderr(): Uint8Array | string;
    getStderr_asU8(): Uint8Array;
    getStderr_asB64(): string;
    setStderr(value: Uint8Array | string): InstallDependenciesResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): InstallDependenciesResponse.AsObject;
    static toObject(includeInstance: boolean, msg: InstallDependenciesResponse): InstallDependenciesResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: InstallDependenciesResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): InstallDependenciesResponse;
    static deserializeBinaryFromReader(message: InstallDependenciesResponse, reader: jspb.BinaryReader): InstallDependenciesResponse;
}

export namespace InstallDependenciesResponse {
    export type AsObject = {
        stdout: Uint8Array | string,
        stderr: Uint8Array | string,
    }
}

export class RuntimeOptionsRequest extends jspb.Message { 

    hasInfo(): boolean;
    clearInfo(): void;
    getInfo(): ProgramInfo | undefined;
    setInfo(value?: ProgramInfo): RuntimeOptionsRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RuntimeOptionsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RuntimeOptionsRequest): RuntimeOptionsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RuntimeOptionsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RuntimeOptionsRequest;
    static deserializeBinaryFromReader(message: RuntimeOptionsRequest, reader: jspb.BinaryReader): RuntimeOptionsRequest;
}

export namespace RuntimeOptionsRequest {
    export type AsObject = {
        info?: ProgramInfo.AsObject,
    }
}

export class RuntimeOptionPrompt extends jspb.Message { 
    getKey(): string;
    setKey(value: string): RuntimeOptionPrompt;
    getDescription(): string;
    setDescription(value: string): RuntimeOptionPrompt;
    getPrompttype(): RuntimeOptionPrompt.RuntimeOptionType;
    setPrompttype(value: RuntimeOptionPrompt.RuntimeOptionType): RuntimeOptionPrompt;
    clearChoicesList(): void;
    getChoicesList(): Array<RuntimeOptionPrompt.RuntimeOptionValue>;
    setChoicesList(value: Array<RuntimeOptionPrompt.RuntimeOptionValue>): RuntimeOptionPrompt;
    addChoices(value?: RuntimeOptionPrompt.RuntimeOptionValue, index?: number): RuntimeOptionPrompt.RuntimeOptionValue;

    hasDefault(): boolean;
    clearDefault(): void;
    getDefault(): RuntimeOptionPrompt.RuntimeOptionValue | undefined;
    setDefault(value?: RuntimeOptionPrompt.RuntimeOptionValue): RuntimeOptionPrompt;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RuntimeOptionPrompt.AsObject;
    static toObject(includeInstance: boolean, msg: RuntimeOptionPrompt): RuntimeOptionPrompt.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RuntimeOptionPrompt, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RuntimeOptionPrompt;
    static deserializeBinaryFromReader(message: RuntimeOptionPrompt, reader: jspb.BinaryReader): RuntimeOptionPrompt;
}

export namespace RuntimeOptionPrompt {
    export type AsObject = {
        key: string,
        description: string,
        prompttype: RuntimeOptionPrompt.RuntimeOptionType,
        choicesList: Array<RuntimeOptionPrompt.RuntimeOptionValue.AsObject>,
        pb_default?: RuntimeOptionPrompt.RuntimeOptionValue.AsObject,
    }


    export class RuntimeOptionValue extends jspb.Message { 
        getPrompttype(): RuntimeOptionPrompt.RuntimeOptionType;
        setPrompttype(value: RuntimeOptionPrompt.RuntimeOptionType): RuntimeOptionValue;
        getStringvalue(): string;
        setStringvalue(value: string): RuntimeOptionValue;
        getInt32value(): number;
        setInt32value(value: number): RuntimeOptionValue;
        getDisplayname(): string;
        setDisplayname(value: string): RuntimeOptionValue;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): RuntimeOptionValue.AsObject;
        static toObject(includeInstance: boolean, msg: RuntimeOptionValue): RuntimeOptionValue.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: RuntimeOptionValue, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): RuntimeOptionValue;
        static deserializeBinaryFromReader(message: RuntimeOptionValue, reader: jspb.BinaryReader): RuntimeOptionValue;
    }

    export namespace RuntimeOptionValue {
        export type AsObject = {
            prompttype: RuntimeOptionPrompt.RuntimeOptionType,
            stringvalue: string,
            int32value: number,
            displayname: string,
        }
    }


    export enum RuntimeOptionType {
    STRING = 0,
    INT32 = 1,
    }

}

export class RuntimeOptionsResponse extends jspb.Message { 
    clearPromptsList(): void;
    getPromptsList(): Array<RuntimeOptionPrompt>;
    setPromptsList(value: Array<RuntimeOptionPrompt>): RuntimeOptionsResponse;
    addPrompts(value?: RuntimeOptionPrompt, index?: number): RuntimeOptionPrompt;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RuntimeOptionsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RuntimeOptionsResponse): RuntimeOptionsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RuntimeOptionsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RuntimeOptionsResponse;
    static deserializeBinaryFromReader(message: RuntimeOptionsResponse, reader: jspb.BinaryReader): RuntimeOptionsResponse;
}

export namespace RuntimeOptionsResponse {
    export type AsObject = {
        promptsList: Array<RuntimeOptionPrompt.AsObject>,
    }
}

export class RunPluginRequest extends jspb.Message { 
    getPwd(): string;
    setPwd(value: string): RunPluginRequest;
    getProgram(): string;
    setProgram(value: string): RunPluginRequest;
    clearArgsList(): void;
    getArgsList(): Array<string>;
    setArgsList(value: Array<string>): RunPluginRequest;
    addArgs(value: string, index?: number): string;
    clearEnvList(): void;
    getEnvList(): Array<string>;
    setEnvList(value: Array<string>): RunPluginRequest;
    addEnv(value: string, index?: number): string;

    hasInfo(): boolean;
    clearInfo(): void;
    getInfo(): ProgramInfo | undefined;
    setInfo(value?: ProgramInfo): RunPluginRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunPluginRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RunPluginRequest): RunPluginRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunPluginRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunPluginRequest;
    static deserializeBinaryFromReader(message: RunPluginRequest, reader: jspb.BinaryReader): RunPluginRequest;
}

export namespace RunPluginRequest {
    export type AsObject = {
        pwd: string,
        program: string,
        argsList: Array<string>,
        envList: Array<string>,
        info?: ProgramInfo.AsObject,
    }
}

export class RunPluginResponse extends jspb.Message { 

    hasStdout(): boolean;
    clearStdout(): void;
    getStdout(): Uint8Array | string;
    getStdout_asU8(): Uint8Array;
    getStdout_asB64(): string;
    setStdout(value: Uint8Array | string): RunPluginResponse;

    hasStderr(): boolean;
    clearStderr(): void;
    getStderr(): Uint8Array | string;
    getStderr_asU8(): Uint8Array;
    getStderr_asB64(): string;
    setStderr(value: Uint8Array | string): RunPluginResponse;

    hasExitcode(): boolean;
    clearExitcode(): void;
    getExitcode(): number;
    setExitcode(value: number): RunPluginResponse;

    getOutputCase(): RunPluginResponse.OutputCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunPluginResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RunPluginResponse): RunPluginResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunPluginResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunPluginResponse;
    static deserializeBinaryFromReader(message: RunPluginResponse, reader: jspb.BinaryReader): RunPluginResponse;
}

export namespace RunPluginResponse {
    export type AsObject = {
        stdout: Uint8Array | string,
        stderr: Uint8Array | string,
        exitcode: number,
    }

    export enum OutputCase {
        OUTPUT_NOT_SET = 0,
        STDOUT = 1,
        STDERR = 2,
        EXITCODE = 3,
    }

}

export class GenerateProgramRequest extends jspb.Message { 

    getSourceMap(): jspb.Map<string, string>;
    clearSourceMap(): void;
    getLoaderTarget(): string;
    setLoaderTarget(value: string): GenerateProgramRequest;
    getStrict(): boolean;
    setStrict(value: boolean): GenerateProgramRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GenerateProgramRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GenerateProgramRequest): GenerateProgramRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GenerateProgramRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GenerateProgramRequest;
    static deserializeBinaryFromReader(message: GenerateProgramRequest, reader: jspb.BinaryReader): GenerateProgramRequest;
}

export namespace GenerateProgramRequest {
    export type AsObject = {

        sourceMap: Array<[string, string]>,
        loaderTarget: string,
        strict: boolean,
    }
}

export class GenerateProgramResponse extends jspb.Message { 
    clearDiagnosticsList(): void;
    getDiagnosticsList(): Array<pulumi_codegen_hcl_pb.Diagnostic>;
    setDiagnosticsList(value: Array<pulumi_codegen_hcl_pb.Diagnostic>): GenerateProgramResponse;
    addDiagnostics(value?: pulumi_codegen_hcl_pb.Diagnostic, index?: number): pulumi_codegen_hcl_pb.Diagnostic;

    getSourceMap(): jspb.Map<string, Uint8Array | string>;
    clearSourceMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GenerateProgramResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GenerateProgramResponse): GenerateProgramResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GenerateProgramResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GenerateProgramResponse;
    static deserializeBinaryFromReader(message: GenerateProgramResponse, reader: jspb.BinaryReader): GenerateProgramResponse;
}

export namespace GenerateProgramResponse {
    export type AsObject = {
        diagnosticsList: Array<pulumi_codegen_hcl_pb.Diagnostic.AsObject>,

        sourceMap: Array<[string, Uint8Array | string]>,
    }
}

export class GenerateProjectRequest extends jspb.Message { 
    getSourceDirectory(): string;
    setSourceDirectory(value: string): GenerateProjectRequest;
    getTargetDirectory(): string;
    setTargetDirectory(value: string): GenerateProjectRequest;
    getProject(): string;
    setProject(value: string): GenerateProjectRequest;
    getStrict(): boolean;
    setStrict(value: boolean): GenerateProjectRequest;
    getLoaderTarget(): string;
    setLoaderTarget(value: string): GenerateProjectRequest;

    getLocalDependenciesMap(): jspb.Map<string, string>;
    clearLocalDependenciesMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GenerateProjectRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GenerateProjectRequest): GenerateProjectRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GenerateProjectRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GenerateProjectRequest;
    static deserializeBinaryFromReader(message: GenerateProjectRequest, reader: jspb.BinaryReader): GenerateProjectRequest;
}

export namespace GenerateProjectRequest {
    export type AsObject = {
        sourceDirectory: string,
        targetDirectory: string,
        project: string,
        strict: boolean,
        loaderTarget: string,

        localDependenciesMap: Array<[string, string]>,
    }
}

export class GenerateProjectResponse extends jspb.Message { 
    clearDiagnosticsList(): void;
    getDiagnosticsList(): Array<pulumi_codegen_hcl_pb.Diagnostic>;
    setDiagnosticsList(value: Array<pulumi_codegen_hcl_pb.Diagnostic>): GenerateProjectResponse;
    addDiagnostics(value?: pulumi_codegen_hcl_pb.Diagnostic, index?: number): pulumi_codegen_hcl_pb.Diagnostic;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GenerateProjectResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GenerateProjectResponse): GenerateProjectResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GenerateProjectResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GenerateProjectResponse;
    static deserializeBinaryFromReader(message: GenerateProjectResponse, reader: jspb.BinaryReader): GenerateProjectResponse;
}

export namespace GenerateProjectResponse {
    export type AsObject = {
        diagnosticsList: Array<pulumi_codegen_hcl_pb.Diagnostic.AsObject>,
    }
}

export class GeneratePackageRequest extends jspb.Message { 
    getDirectory(): string;
    setDirectory(value: string): GeneratePackageRequest;
    getSchema(): string;
    setSchema(value: string): GeneratePackageRequest;

    getExtraFilesMap(): jspb.Map<string, Uint8Array | string>;
    clearExtraFilesMap(): void;
    getLoaderTarget(): string;
    setLoaderTarget(value: string): GeneratePackageRequest;

    getLocalDependenciesMap(): jspb.Map<string, string>;
    clearLocalDependenciesMap(): void;
    getLocal(): boolean;
    setLocal(value: boolean): GeneratePackageRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GeneratePackageRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GeneratePackageRequest): GeneratePackageRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GeneratePackageRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GeneratePackageRequest;
    static deserializeBinaryFromReader(message: GeneratePackageRequest, reader: jspb.BinaryReader): GeneratePackageRequest;
}

export namespace GeneratePackageRequest {
    export type AsObject = {
        directory: string,
        schema: string,

        extraFilesMap: Array<[string, Uint8Array | string]>,
        loaderTarget: string,

        localDependenciesMap: Array<[string, string]>,
        local: boolean,
    }
}

export class GeneratePackageResponse extends jspb.Message { 
    clearDiagnosticsList(): void;
    getDiagnosticsList(): Array<pulumi_codegen_hcl_pb.Diagnostic>;
    setDiagnosticsList(value: Array<pulumi_codegen_hcl_pb.Diagnostic>): GeneratePackageResponse;
    addDiagnostics(value?: pulumi_codegen_hcl_pb.Diagnostic, index?: number): pulumi_codegen_hcl_pb.Diagnostic;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GeneratePackageResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GeneratePackageResponse): GeneratePackageResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GeneratePackageResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GeneratePackageResponse;
    static deserializeBinaryFromReader(message: GeneratePackageResponse, reader: jspb.BinaryReader): GeneratePackageResponse;
}

export namespace GeneratePackageResponse {
    export type AsObject = {
        diagnosticsList: Array<pulumi_codegen_hcl_pb.Diagnostic.AsObject>,
    }
}

export class PackRequest extends jspb.Message { 
    getPackageDirectory(): string;
    setPackageDirectory(value: string): PackRequest;
    getDestinationDirectory(): string;
    setDestinationDirectory(value: string): PackRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PackRequest.AsObject;
    static toObject(includeInstance: boolean, msg: PackRequest): PackRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PackRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PackRequest;
    static deserializeBinaryFromReader(message: PackRequest, reader: jspb.BinaryReader): PackRequest;
}

export namespace PackRequest {
    export type AsObject = {
        packageDirectory: string,
        destinationDirectory: string,
    }
}

export class PackResponse extends jspb.Message { 
    getArtifactPath(): string;
    setArtifactPath(value: string): PackResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PackResponse.AsObject;
    static toObject(includeInstance: boolean, msg: PackResponse): PackResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PackResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PackResponse;
    static deserializeBinaryFromReader(message: PackResponse, reader: jspb.BinaryReader): PackResponse;
}

export namespace PackResponse {
    export type AsObject = {
        artifactPath: string,
    }
}
