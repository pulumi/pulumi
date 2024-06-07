// package: pulumirpc
// file: pulumi/language.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_language_pb from "./language_pb";
import * as pulumi_codegen_hcl_pb from "./codegen/hcl_pb";
import * as pulumi_plugin_pb from "./plugin_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

interface ILanguageRuntimeService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    getRequiredPlugins: ILanguageRuntimeService_IGetRequiredPlugins;
    run: ILanguageRuntimeService_IRun;
    getPluginInfo: ILanguageRuntimeService_IGetPluginInfo;
    installDependencies: ILanguageRuntimeService_IInstallDependencies;
    runtimeOptionsPrompts: ILanguageRuntimeService_IRuntimeOptionsPrompts;
    about: ILanguageRuntimeService_IAbout;
    getProgramDependencies: ILanguageRuntimeService_IGetProgramDependencies;
    runPlugin: ILanguageRuntimeService_IRunPlugin;
    generateProgram: ILanguageRuntimeService_IGenerateProgram;
    generateProject: ILanguageRuntimeService_IGenerateProject;
    generatePackage: ILanguageRuntimeService_IGeneratePackage;
    pack: ILanguageRuntimeService_IPack;
}

interface ILanguageRuntimeService_IGetRequiredPlugins extends grpc.MethodDefinition<pulumi_language_pb.GetRequiredPluginsRequest, pulumi_language_pb.GetRequiredPluginsResponse> {
    path: "/pulumirpc.LanguageRuntime/GetRequiredPlugins";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_language_pb.GetRequiredPluginsRequest>;
    requestDeserialize: grpc.deserialize<pulumi_language_pb.GetRequiredPluginsRequest>;
    responseSerialize: grpc.serialize<pulumi_language_pb.GetRequiredPluginsResponse>;
    responseDeserialize: grpc.deserialize<pulumi_language_pb.GetRequiredPluginsResponse>;
}
interface ILanguageRuntimeService_IRun extends grpc.MethodDefinition<pulumi_language_pb.RunRequest, pulumi_language_pb.RunResponse> {
    path: "/pulumirpc.LanguageRuntime/Run";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_language_pb.RunRequest>;
    requestDeserialize: grpc.deserialize<pulumi_language_pb.RunRequest>;
    responseSerialize: grpc.serialize<pulumi_language_pb.RunResponse>;
    responseDeserialize: grpc.deserialize<pulumi_language_pb.RunResponse>;
}
interface ILanguageRuntimeService_IGetPluginInfo extends grpc.MethodDefinition<google_protobuf_empty_pb.Empty, pulumi_plugin_pb.PluginInfo> {
    path: "/pulumirpc.LanguageRuntime/GetPluginInfo";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    requestDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
    responseSerialize: grpc.serialize<pulumi_plugin_pb.PluginInfo>;
    responseDeserialize: grpc.deserialize<pulumi_plugin_pb.PluginInfo>;
}
interface ILanguageRuntimeService_IInstallDependencies extends grpc.MethodDefinition<pulumi_language_pb.InstallDependenciesRequest, pulumi_language_pb.InstallDependenciesResponse> {
    path: "/pulumirpc.LanguageRuntime/InstallDependencies";
    requestStream: false;
    responseStream: true;
    requestSerialize: grpc.serialize<pulumi_language_pb.InstallDependenciesRequest>;
    requestDeserialize: grpc.deserialize<pulumi_language_pb.InstallDependenciesRequest>;
    responseSerialize: grpc.serialize<pulumi_language_pb.InstallDependenciesResponse>;
    responseDeserialize: grpc.deserialize<pulumi_language_pb.InstallDependenciesResponse>;
}
interface ILanguageRuntimeService_IRuntimeOptionsPrompts extends grpc.MethodDefinition<pulumi_language_pb.RuntimeOptionsRequest, pulumi_language_pb.RuntimeOptionsResponse> {
    path: "/pulumirpc.LanguageRuntime/RuntimeOptionsPrompts";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_language_pb.RuntimeOptionsRequest>;
    requestDeserialize: grpc.deserialize<pulumi_language_pb.RuntimeOptionsRequest>;
    responseSerialize: grpc.serialize<pulumi_language_pb.RuntimeOptionsResponse>;
    responseDeserialize: grpc.deserialize<pulumi_language_pb.RuntimeOptionsResponse>;
}
interface ILanguageRuntimeService_IAbout extends grpc.MethodDefinition<pulumi_language_pb.AboutRequest, pulumi_language_pb.AboutResponse> {
    path: "/pulumirpc.LanguageRuntime/About";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_language_pb.AboutRequest>;
    requestDeserialize: grpc.deserialize<pulumi_language_pb.AboutRequest>;
    responseSerialize: grpc.serialize<pulumi_language_pb.AboutResponse>;
    responseDeserialize: grpc.deserialize<pulumi_language_pb.AboutResponse>;
}
interface ILanguageRuntimeService_IGetProgramDependencies extends grpc.MethodDefinition<pulumi_language_pb.GetProgramDependenciesRequest, pulumi_language_pb.GetProgramDependenciesResponse> {
    path: "/pulumirpc.LanguageRuntime/GetProgramDependencies";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_language_pb.GetProgramDependenciesRequest>;
    requestDeserialize: grpc.deserialize<pulumi_language_pb.GetProgramDependenciesRequest>;
    responseSerialize: grpc.serialize<pulumi_language_pb.GetProgramDependenciesResponse>;
    responseDeserialize: grpc.deserialize<pulumi_language_pb.GetProgramDependenciesResponse>;
}
interface ILanguageRuntimeService_IRunPlugin extends grpc.MethodDefinition<pulumi_language_pb.RunPluginRequest, pulumi_language_pb.RunPluginResponse> {
    path: "/pulumirpc.LanguageRuntime/RunPlugin";
    requestStream: false;
    responseStream: true;
    requestSerialize: grpc.serialize<pulumi_language_pb.RunPluginRequest>;
    requestDeserialize: grpc.deserialize<pulumi_language_pb.RunPluginRequest>;
    responseSerialize: grpc.serialize<pulumi_language_pb.RunPluginResponse>;
    responseDeserialize: grpc.deserialize<pulumi_language_pb.RunPluginResponse>;
}
interface ILanguageRuntimeService_IGenerateProgram extends grpc.MethodDefinition<pulumi_language_pb.GenerateProgramRequest, pulumi_language_pb.GenerateProgramResponse> {
    path: "/pulumirpc.LanguageRuntime/GenerateProgram";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_language_pb.GenerateProgramRequest>;
    requestDeserialize: grpc.deserialize<pulumi_language_pb.GenerateProgramRequest>;
    responseSerialize: grpc.serialize<pulumi_language_pb.GenerateProgramResponse>;
    responseDeserialize: grpc.deserialize<pulumi_language_pb.GenerateProgramResponse>;
}
interface ILanguageRuntimeService_IGenerateProject extends grpc.MethodDefinition<pulumi_language_pb.GenerateProjectRequest, pulumi_language_pb.GenerateProjectResponse> {
    path: "/pulumirpc.LanguageRuntime/GenerateProject";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_language_pb.GenerateProjectRequest>;
    requestDeserialize: grpc.deserialize<pulumi_language_pb.GenerateProjectRequest>;
    responseSerialize: grpc.serialize<pulumi_language_pb.GenerateProjectResponse>;
    responseDeserialize: grpc.deserialize<pulumi_language_pb.GenerateProjectResponse>;
}
interface ILanguageRuntimeService_IGeneratePackage extends grpc.MethodDefinition<pulumi_language_pb.GeneratePackageRequest, pulumi_language_pb.GeneratePackageResponse> {
    path: "/pulumirpc.LanguageRuntime/GeneratePackage";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_language_pb.GeneratePackageRequest>;
    requestDeserialize: grpc.deserialize<pulumi_language_pb.GeneratePackageRequest>;
    responseSerialize: grpc.serialize<pulumi_language_pb.GeneratePackageResponse>;
    responseDeserialize: grpc.deserialize<pulumi_language_pb.GeneratePackageResponse>;
}
interface ILanguageRuntimeService_IPack extends grpc.MethodDefinition<pulumi_language_pb.PackRequest, pulumi_language_pb.PackResponse> {
    path: "/pulumirpc.LanguageRuntime/Pack";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_language_pb.PackRequest>;
    requestDeserialize: grpc.deserialize<pulumi_language_pb.PackRequest>;
    responseSerialize: grpc.serialize<pulumi_language_pb.PackResponse>;
    responseDeserialize: grpc.deserialize<pulumi_language_pb.PackResponse>;
}

export const LanguageRuntimeService: ILanguageRuntimeService;

export interface ILanguageRuntimeServer extends grpc.UntypedServiceImplementation {
    getRequiredPlugins: grpc.handleUnaryCall<pulumi_language_pb.GetRequiredPluginsRequest, pulumi_language_pb.GetRequiredPluginsResponse>;
    run: grpc.handleUnaryCall<pulumi_language_pb.RunRequest, pulumi_language_pb.RunResponse>;
    getPluginInfo: grpc.handleUnaryCall<google_protobuf_empty_pb.Empty, pulumi_plugin_pb.PluginInfo>;
    installDependencies: grpc.handleServerStreamingCall<pulumi_language_pb.InstallDependenciesRequest, pulumi_language_pb.InstallDependenciesResponse>;
    runtimeOptionsPrompts: grpc.handleUnaryCall<pulumi_language_pb.RuntimeOptionsRequest, pulumi_language_pb.RuntimeOptionsResponse>;
    about: grpc.handleUnaryCall<pulumi_language_pb.AboutRequest, pulumi_language_pb.AboutResponse>;
    getProgramDependencies: grpc.handleUnaryCall<pulumi_language_pb.GetProgramDependenciesRequest, pulumi_language_pb.GetProgramDependenciesResponse>;
    runPlugin: grpc.handleServerStreamingCall<pulumi_language_pb.RunPluginRequest, pulumi_language_pb.RunPluginResponse>;
    generateProgram: grpc.handleUnaryCall<pulumi_language_pb.GenerateProgramRequest, pulumi_language_pb.GenerateProgramResponse>;
    generateProject: grpc.handleUnaryCall<pulumi_language_pb.GenerateProjectRequest, pulumi_language_pb.GenerateProjectResponse>;
    generatePackage: grpc.handleUnaryCall<pulumi_language_pb.GeneratePackageRequest, pulumi_language_pb.GeneratePackageResponse>;
    pack: grpc.handleUnaryCall<pulumi_language_pb.PackRequest, pulumi_language_pb.PackResponse>;
}

export interface ILanguageRuntimeClient {
    getRequiredPlugins(request: pulumi_language_pb.GetRequiredPluginsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetRequiredPluginsResponse) => void): grpc.ClientUnaryCall;
    getRequiredPlugins(request: pulumi_language_pb.GetRequiredPluginsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetRequiredPluginsResponse) => void): grpc.ClientUnaryCall;
    getRequiredPlugins(request: pulumi_language_pb.GetRequiredPluginsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetRequiredPluginsResponse) => void): grpc.ClientUnaryCall;
    run(request: pulumi_language_pb.RunRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RunResponse) => void): grpc.ClientUnaryCall;
    run(request: pulumi_language_pb.RunRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RunResponse) => void): grpc.ClientUnaryCall;
    run(request: pulumi_language_pb.RunRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RunResponse) => void): grpc.ClientUnaryCall;
    getPluginInfo(request: google_protobuf_empty_pb.Empty, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    installDependencies(request: pulumi_language_pb.InstallDependenciesRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_language_pb.InstallDependenciesResponse>;
    installDependencies(request: pulumi_language_pb.InstallDependenciesRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_language_pb.InstallDependenciesResponse>;
    runtimeOptionsPrompts(request: pulumi_language_pb.RuntimeOptionsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RuntimeOptionsResponse) => void): grpc.ClientUnaryCall;
    runtimeOptionsPrompts(request: pulumi_language_pb.RuntimeOptionsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RuntimeOptionsResponse) => void): grpc.ClientUnaryCall;
    runtimeOptionsPrompts(request: pulumi_language_pb.RuntimeOptionsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RuntimeOptionsResponse) => void): grpc.ClientUnaryCall;
    about(request: pulumi_language_pb.AboutRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.AboutResponse) => void): grpc.ClientUnaryCall;
    about(request: pulumi_language_pb.AboutRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.AboutResponse) => void): grpc.ClientUnaryCall;
    about(request: pulumi_language_pb.AboutRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.AboutResponse) => void): grpc.ClientUnaryCall;
    getProgramDependencies(request: pulumi_language_pb.GetProgramDependenciesRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetProgramDependenciesResponse) => void): grpc.ClientUnaryCall;
    getProgramDependencies(request: pulumi_language_pb.GetProgramDependenciesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetProgramDependenciesResponse) => void): grpc.ClientUnaryCall;
    getProgramDependencies(request: pulumi_language_pb.GetProgramDependenciesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetProgramDependenciesResponse) => void): grpc.ClientUnaryCall;
    runPlugin(request: pulumi_language_pb.RunPluginRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_language_pb.RunPluginResponse>;
    runPlugin(request: pulumi_language_pb.RunPluginRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_language_pb.RunPluginResponse>;
    generateProgram(request: pulumi_language_pb.GenerateProgramRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProgramResponse) => void): grpc.ClientUnaryCall;
    generateProgram(request: pulumi_language_pb.GenerateProgramRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProgramResponse) => void): grpc.ClientUnaryCall;
    generateProgram(request: pulumi_language_pb.GenerateProgramRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProgramResponse) => void): grpc.ClientUnaryCall;
    generateProject(request: pulumi_language_pb.GenerateProjectRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProjectResponse) => void): grpc.ClientUnaryCall;
    generateProject(request: pulumi_language_pb.GenerateProjectRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProjectResponse) => void): grpc.ClientUnaryCall;
    generateProject(request: pulumi_language_pb.GenerateProjectRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProjectResponse) => void): grpc.ClientUnaryCall;
    generatePackage(request: pulumi_language_pb.GeneratePackageRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GeneratePackageResponse) => void): grpc.ClientUnaryCall;
    generatePackage(request: pulumi_language_pb.GeneratePackageRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GeneratePackageResponse) => void): grpc.ClientUnaryCall;
    generatePackage(request: pulumi_language_pb.GeneratePackageRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GeneratePackageResponse) => void): grpc.ClientUnaryCall;
    pack(request: pulumi_language_pb.PackRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.PackResponse) => void): grpc.ClientUnaryCall;
    pack(request: pulumi_language_pb.PackRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.PackResponse) => void): grpc.ClientUnaryCall;
    pack(request: pulumi_language_pb.PackRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.PackResponse) => void): grpc.ClientUnaryCall;
}

export class LanguageRuntimeClient extends grpc.Client implements ILanguageRuntimeClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public getRequiredPlugins(request: pulumi_language_pb.GetRequiredPluginsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetRequiredPluginsResponse) => void): grpc.ClientUnaryCall;
    public getRequiredPlugins(request: pulumi_language_pb.GetRequiredPluginsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetRequiredPluginsResponse) => void): grpc.ClientUnaryCall;
    public getRequiredPlugins(request: pulumi_language_pb.GetRequiredPluginsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetRequiredPluginsResponse) => void): grpc.ClientUnaryCall;
    public run(request: pulumi_language_pb.RunRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RunResponse) => void): grpc.ClientUnaryCall;
    public run(request: pulumi_language_pb.RunRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RunResponse) => void): grpc.ClientUnaryCall;
    public run(request: pulumi_language_pb.RunRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RunResponse) => void): grpc.ClientUnaryCall;
    public getPluginInfo(request: google_protobuf_empty_pb.Empty, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    public getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    public getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    public installDependencies(request: pulumi_language_pb.InstallDependenciesRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_language_pb.InstallDependenciesResponse>;
    public installDependencies(request: pulumi_language_pb.InstallDependenciesRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_language_pb.InstallDependenciesResponse>;
    public runtimeOptionsPrompts(request: pulumi_language_pb.RuntimeOptionsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RuntimeOptionsResponse) => void): grpc.ClientUnaryCall;
    public runtimeOptionsPrompts(request: pulumi_language_pb.RuntimeOptionsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RuntimeOptionsResponse) => void): grpc.ClientUnaryCall;
    public runtimeOptionsPrompts(request: pulumi_language_pb.RuntimeOptionsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.RuntimeOptionsResponse) => void): grpc.ClientUnaryCall;
    public about(request: pulumi_language_pb.AboutRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.AboutResponse) => void): grpc.ClientUnaryCall;
    public about(request: pulumi_language_pb.AboutRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.AboutResponse) => void): grpc.ClientUnaryCall;
    public about(request: pulumi_language_pb.AboutRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.AboutResponse) => void): grpc.ClientUnaryCall;
    public getProgramDependencies(request: pulumi_language_pb.GetProgramDependenciesRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetProgramDependenciesResponse) => void): grpc.ClientUnaryCall;
    public getProgramDependencies(request: pulumi_language_pb.GetProgramDependenciesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetProgramDependenciesResponse) => void): grpc.ClientUnaryCall;
    public getProgramDependencies(request: pulumi_language_pb.GetProgramDependenciesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GetProgramDependenciesResponse) => void): grpc.ClientUnaryCall;
    public runPlugin(request: pulumi_language_pb.RunPluginRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_language_pb.RunPluginResponse>;
    public runPlugin(request: pulumi_language_pb.RunPluginRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_language_pb.RunPluginResponse>;
    public generateProgram(request: pulumi_language_pb.GenerateProgramRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProgramResponse) => void): grpc.ClientUnaryCall;
    public generateProgram(request: pulumi_language_pb.GenerateProgramRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProgramResponse) => void): grpc.ClientUnaryCall;
    public generateProgram(request: pulumi_language_pb.GenerateProgramRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProgramResponse) => void): grpc.ClientUnaryCall;
    public generateProject(request: pulumi_language_pb.GenerateProjectRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProjectResponse) => void): grpc.ClientUnaryCall;
    public generateProject(request: pulumi_language_pb.GenerateProjectRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProjectResponse) => void): grpc.ClientUnaryCall;
    public generateProject(request: pulumi_language_pb.GenerateProjectRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GenerateProjectResponse) => void): grpc.ClientUnaryCall;
    public generatePackage(request: pulumi_language_pb.GeneratePackageRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GeneratePackageResponse) => void): grpc.ClientUnaryCall;
    public generatePackage(request: pulumi_language_pb.GeneratePackageRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GeneratePackageResponse) => void): grpc.ClientUnaryCall;
    public generatePackage(request: pulumi_language_pb.GeneratePackageRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.GeneratePackageResponse) => void): grpc.ClientUnaryCall;
    public pack(request: pulumi_language_pb.PackRequest, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.PackResponse) => void): grpc.ClientUnaryCall;
    public pack(request: pulumi_language_pb.PackRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.PackResponse) => void): grpc.ClientUnaryCall;
    public pack(request: pulumi_language_pb.PackRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_language_pb.PackResponse) => void): grpc.ClientUnaryCall;
}
