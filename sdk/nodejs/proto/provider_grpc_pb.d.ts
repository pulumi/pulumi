// package: pulumirpc
// file: pulumi/provider.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_provider_pb from "./provider_pb";
import * as pulumi_plugin_pb from "./plugin_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

interface IResourceProviderService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    parameterize: IResourceProviderService_IParameterize;
    getSchema: IResourceProviderService_IGetSchema;
    checkConfig: IResourceProviderService_ICheckConfig;
    diffConfig: IResourceProviderService_IDiffConfig;
    configure: IResourceProviderService_IConfigure;
    invoke: IResourceProviderService_IInvoke;
    streamInvoke: IResourceProviderService_IStreamInvoke;
    call: IResourceProviderService_ICall;
    check: IResourceProviderService_ICheck;
    diff: IResourceProviderService_IDiff;
    create: IResourceProviderService_ICreate;
    read: IResourceProviderService_IRead;
    update: IResourceProviderService_IUpdate;
    delete: IResourceProviderService_IDelete;
    construct: IResourceProviderService_IConstruct;
    cancel: IResourceProviderService_ICancel;
    getPluginInfo: IResourceProviderService_IGetPluginInfo;
    attach: IResourceProviderService_IAttach;
    getMapping: IResourceProviderService_IGetMapping;
    getMappings: IResourceProviderService_IGetMappings;
}

interface IResourceProviderService_IParameterize extends grpc.MethodDefinition<pulumi_provider_pb.ParameterizeRequest, pulumi_provider_pb.ParameterizeResponse> {
    path: "/pulumirpc.ResourceProvider/Parameterize";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.ParameterizeRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.ParameterizeRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.ParameterizeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.ParameterizeResponse>;
}
interface IResourceProviderService_IGetSchema extends grpc.MethodDefinition<pulumi_provider_pb.GetSchemaRequest, pulumi_provider_pb.GetSchemaResponse> {
    path: "/pulumirpc.ResourceProvider/GetSchema";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.GetSchemaRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.GetSchemaRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.GetSchemaResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.GetSchemaResponse>;
}
interface IResourceProviderService_ICheckConfig extends grpc.MethodDefinition<pulumi_provider_pb.CheckRequest, pulumi_provider_pb.CheckResponse> {
    path: "/pulumirpc.ResourceProvider/CheckConfig";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.CheckRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.CheckRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.CheckResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.CheckResponse>;
}
interface IResourceProviderService_IDiffConfig extends grpc.MethodDefinition<pulumi_provider_pb.DiffRequest, pulumi_provider_pb.DiffResponse> {
    path: "/pulumirpc.ResourceProvider/DiffConfig";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.DiffRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.DiffRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.DiffResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.DiffResponse>;
}
interface IResourceProviderService_IConfigure extends grpc.MethodDefinition<pulumi_provider_pb.ConfigureRequest, pulumi_provider_pb.ConfigureResponse> {
    path: "/pulumirpc.ResourceProvider/Configure";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.ConfigureRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.ConfigureRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.ConfigureResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.ConfigureResponse>;
}
interface IResourceProviderService_IInvoke extends grpc.MethodDefinition<pulumi_provider_pb.InvokeRequest, pulumi_provider_pb.InvokeResponse> {
    path: "/pulumirpc.ResourceProvider/Invoke";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.InvokeRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.InvokeRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.InvokeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.InvokeResponse>;
}
interface IResourceProviderService_IStreamInvoke extends grpc.MethodDefinition<pulumi_provider_pb.InvokeRequest, pulumi_provider_pb.InvokeResponse> {
    path: "/pulumirpc.ResourceProvider/StreamInvoke";
    requestStream: false;
    responseStream: true;
    requestSerialize: grpc.serialize<pulumi_provider_pb.InvokeRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.InvokeRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.InvokeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.InvokeResponse>;
}
interface IResourceProviderService_ICall extends grpc.MethodDefinition<pulumi_provider_pb.CallRequest, pulumi_provider_pb.CallResponse> {
    path: "/pulumirpc.ResourceProvider/Call";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.CallRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.CallRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.CallResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.CallResponse>;
}
interface IResourceProviderService_ICheck extends grpc.MethodDefinition<pulumi_provider_pb.CheckRequest, pulumi_provider_pb.CheckResponse> {
    path: "/pulumirpc.ResourceProvider/Check";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.CheckRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.CheckRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.CheckResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.CheckResponse>;
}
interface IResourceProviderService_IDiff extends grpc.MethodDefinition<pulumi_provider_pb.DiffRequest, pulumi_provider_pb.DiffResponse> {
    path: "/pulumirpc.ResourceProvider/Diff";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.DiffRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.DiffRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.DiffResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.DiffResponse>;
}
interface IResourceProviderService_ICreate extends grpc.MethodDefinition<pulumi_provider_pb.CreateRequest, pulumi_provider_pb.CreateResponse> {
    path: "/pulumirpc.ResourceProvider/Create";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.CreateRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.CreateRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.CreateResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.CreateResponse>;
}
interface IResourceProviderService_IRead extends grpc.MethodDefinition<pulumi_provider_pb.ReadRequest, pulumi_provider_pb.ReadResponse> {
    path: "/pulumirpc.ResourceProvider/Read";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.ReadRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.ReadRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.ReadResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.ReadResponse>;
}
interface IResourceProviderService_IUpdate extends grpc.MethodDefinition<pulumi_provider_pb.UpdateRequest, pulumi_provider_pb.UpdateResponse> {
    path: "/pulumirpc.ResourceProvider/Update";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.UpdateRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.UpdateRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.UpdateResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.UpdateResponse>;
}
interface IResourceProviderService_IDelete extends grpc.MethodDefinition<pulumi_provider_pb.DeleteRequest, google_protobuf_empty_pb.Empty> {
    path: "/pulumirpc.ResourceProvider/Delete";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.DeleteRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.DeleteRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IResourceProviderService_IConstruct extends grpc.MethodDefinition<pulumi_provider_pb.ConstructRequest, pulumi_provider_pb.ConstructResponse> {
    path: "/pulumirpc.ResourceProvider/Construct";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.ConstructRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.ConstructRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.ConstructResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.ConstructResponse>;
}
interface IResourceProviderService_ICancel extends grpc.MethodDefinition<google_protobuf_empty_pb.Empty, google_protobuf_empty_pb.Empty> {
    path: "/pulumirpc.ResourceProvider/Cancel";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    requestDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IResourceProviderService_IGetPluginInfo extends grpc.MethodDefinition<google_protobuf_empty_pb.Empty, pulumi_plugin_pb.PluginInfo> {
    path: "/pulumirpc.ResourceProvider/GetPluginInfo";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    requestDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
    responseSerialize: grpc.serialize<pulumi_plugin_pb.PluginInfo>;
    responseDeserialize: grpc.deserialize<pulumi_plugin_pb.PluginInfo>;
}
interface IResourceProviderService_IAttach extends grpc.MethodDefinition<pulumi_plugin_pb.PluginAttach, google_protobuf_empty_pb.Empty> {
    path: "/pulumirpc.ResourceProvider/Attach";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_plugin_pb.PluginAttach>;
    requestDeserialize: grpc.deserialize<pulumi_plugin_pb.PluginAttach>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IResourceProviderService_IGetMapping extends grpc.MethodDefinition<pulumi_provider_pb.GetMappingRequest, pulumi_provider_pb.GetMappingResponse> {
    path: "/pulumirpc.ResourceProvider/GetMapping";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.GetMappingRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.GetMappingRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.GetMappingResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.GetMappingResponse>;
}
interface IResourceProviderService_IGetMappings extends grpc.MethodDefinition<pulumi_provider_pb.GetMappingsRequest, pulumi_provider_pb.GetMappingsResponse> {
    path: "/pulumirpc.ResourceProvider/GetMappings";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_provider_pb.GetMappingsRequest>;
    requestDeserialize: grpc.deserialize<pulumi_provider_pb.GetMappingsRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.GetMappingsResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.GetMappingsResponse>;
}

export const ResourceProviderService: IResourceProviderService;

export interface IResourceProviderServer extends grpc.UntypedServiceImplementation {
    parameterize: grpc.handleUnaryCall<pulumi_provider_pb.ParameterizeRequest, pulumi_provider_pb.ParameterizeResponse>;
    getSchema: grpc.handleUnaryCall<pulumi_provider_pb.GetSchemaRequest, pulumi_provider_pb.GetSchemaResponse>;
    checkConfig: grpc.handleUnaryCall<pulumi_provider_pb.CheckRequest, pulumi_provider_pb.CheckResponse>;
    diffConfig: grpc.handleUnaryCall<pulumi_provider_pb.DiffRequest, pulumi_provider_pb.DiffResponse>;
    configure: grpc.handleUnaryCall<pulumi_provider_pb.ConfigureRequest, pulumi_provider_pb.ConfigureResponse>;
    invoke: grpc.handleUnaryCall<pulumi_provider_pb.InvokeRequest, pulumi_provider_pb.InvokeResponse>;
    streamInvoke: grpc.handleServerStreamingCall<pulumi_provider_pb.InvokeRequest, pulumi_provider_pb.InvokeResponse>;
    call: grpc.handleUnaryCall<pulumi_provider_pb.CallRequest, pulumi_provider_pb.CallResponse>;
    check: grpc.handleUnaryCall<pulumi_provider_pb.CheckRequest, pulumi_provider_pb.CheckResponse>;
    diff: grpc.handleUnaryCall<pulumi_provider_pb.DiffRequest, pulumi_provider_pb.DiffResponse>;
    create: grpc.handleUnaryCall<pulumi_provider_pb.CreateRequest, pulumi_provider_pb.CreateResponse>;
    read: grpc.handleUnaryCall<pulumi_provider_pb.ReadRequest, pulumi_provider_pb.ReadResponse>;
    update: grpc.handleUnaryCall<pulumi_provider_pb.UpdateRequest, pulumi_provider_pb.UpdateResponse>;
    delete: grpc.handleUnaryCall<pulumi_provider_pb.DeleteRequest, google_protobuf_empty_pb.Empty>;
    construct: grpc.handleUnaryCall<pulumi_provider_pb.ConstructRequest, pulumi_provider_pb.ConstructResponse>;
    cancel: grpc.handleUnaryCall<google_protobuf_empty_pb.Empty, google_protobuf_empty_pb.Empty>;
    getPluginInfo: grpc.handleUnaryCall<google_protobuf_empty_pb.Empty, pulumi_plugin_pb.PluginInfo>;
    attach: grpc.handleUnaryCall<pulumi_plugin_pb.PluginAttach, google_protobuf_empty_pb.Empty>;
    getMapping: grpc.handleUnaryCall<pulumi_provider_pb.GetMappingRequest, pulumi_provider_pb.GetMappingResponse>;
    getMappings: grpc.handleUnaryCall<pulumi_provider_pb.GetMappingsRequest, pulumi_provider_pb.GetMappingsResponse>;
}

export interface IResourceProviderClient {
    parameterize(request: pulumi_provider_pb.ParameterizeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ParameterizeResponse) => void): grpc.ClientUnaryCall;
    parameterize(request: pulumi_provider_pb.ParameterizeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ParameterizeResponse) => void): grpc.ClientUnaryCall;
    parameterize(request: pulumi_provider_pb.ParameterizeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ParameterizeResponse) => void): grpc.ClientUnaryCall;
    getSchema(request: pulumi_provider_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    getSchema(request: pulumi_provider_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    getSchema(request: pulumi_provider_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    checkConfig(request: pulumi_provider_pb.CheckRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    checkConfig(request: pulumi_provider_pb.CheckRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    checkConfig(request: pulumi_provider_pb.CheckRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    diffConfig(request: pulumi_provider_pb.DiffRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    diffConfig(request: pulumi_provider_pb.DiffRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    diffConfig(request: pulumi_provider_pb.DiffRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    configure(request: pulumi_provider_pb.ConfigureRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConfigureResponse) => void): grpc.ClientUnaryCall;
    configure(request: pulumi_provider_pb.ConfigureRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConfigureResponse) => void): grpc.ClientUnaryCall;
    configure(request: pulumi_provider_pb.ConfigureRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConfigureResponse) => void): grpc.ClientUnaryCall;
    invoke(request: pulumi_provider_pb.InvokeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    invoke(request: pulumi_provider_pb.InvokeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    invoke(request: pulumi_provider_pb.InvokeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    streamInvoke(request: pulumi_provider_pb.InvokeRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_provider_pb.InvokeResponse>;
    streamInvoke(request: pulumi_provider_pb.InvokeRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_provider_pb.InvokeResponse>;
    call(request: pulumi_provider_pb.CallRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    call(request: pulumi_provider_pb.CallRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    call(request: pulumi_provider_pb.CallRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    check(request: pulumi_provider_pb.CheckRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    check(request: pulumi_provider_pb.CheckRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    check(request: pulumi_provider_pb.CheckRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    diff(request: pulumi_provider_pb.DiffRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    diff(request: pulumi_provider_pb.DiffRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    diff(request: pulumi_provider_pb.DiffRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    create(request: pulumi_provider_pb.CreateRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CreateResponse) => void): grpc.ClientUnaryCall;
    create(request: pulumi_provider_pb.CreateRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CreateResponse) => void): grpc.ClientUnaryCall;
    create(request: pulumi_provider_pb.CreateRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CreateResponse) => void): grpc.ClientUnaryCall;
    read(request: pulumi_provider_pb.ReadRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ReadResponse) => void): grpc.ClientUnaryCall;
    read(request: pulumi_provider_pb.ReadRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ReadResponse) => void): grpc.ClientUnaryCall;
    read(request: pulumi_provider_pb.ReadRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ReadResponse) => void): grpc.ClientUnaryCall;
    update(request: pulumi_provider_pb.UpdateRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.UpdateResponse) => void): grpc.ClientUnaryCall;
    update(request: pulumi_provider_pb.UpdateRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.UpdateResponse) => void): grpc.ClientUnaryCall;
    update(request: pulumi_provider_pb.UpdateRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.UpdateResponse) => void): grpc.ClientUnaryCall;
    delete(request: pulumi_provider_pb.DeleteRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    delete(request: pulumi_provider_pb.DeleteRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    delete(request: pulumi_provider_pb.DeleteRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    construct(request: pulumi_provider_pb.ConstructRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConstructResponse) => void): grpc.ClientUnaryCall;
    construct(request: pulumi_provider_pb.ConstructRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConstructResponse) => void): grpc.ClientUnaryCall;
    construct(request: pulumi_provider_pb.ConstructRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConstructResponse) => void): grpc.ClientUnaryCall;
    cancel(request: google_protobuf_empty_pb.Empty, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    cancel(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    cancel(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    getPluginInfo(request: google_protobuf_empty_pb.Empty, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    attach(request: pulumi_plugin_pb.PluginAttach, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    attach(request: pulumi_plugin_pb.PluginAttach, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    attach(request: pulumi_plugin_pb.PluginAttach, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    getMapping(request: pulumi_provider_pb.GetMappingRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
    getMapping(request: pulumi_provider_pb.GetMappingRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
    getMapping(request: pulumi_provider_pb.GetMappingRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
    getMappings(request: pulumi_provider_pb.GetMappingsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingsResponse) => void): grpc.ClientUnaryCall;
    getMappings(request: pulumi_provider_pb.GetMappingsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingsResponse) => void): grpc.ClientUnaryCall;
    getMappings(request: pulumi_provider_pb.GetMappingsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingsResponse) => void): grpc.ClientUnaryCall;
}

export class ResourceProviderClient extends grpc.Client implements IResourceProviderClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public parameterize(request: pulumi_provider_pb.ParameterizeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ParameterizeResponse) => void): grpc.ClientUnaryCall;
    public parameterize(request: pulumi_provider_pb.ParameterizeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ParameterizeResponse) => void): grpc.ClientUnaryCall;
    public parameterize(request: pulumi_provider_pb.ParameterizeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ParameterizeResponse) => void): grpc.ClientUnaryCall;
    public getSchema(request: pulumi_provider_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public getSchema(request: pulumi_provider_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public getSchema(request: pulumi_provider_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public checkConfig(request: pulumi_provider_pb.CheckRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    public checkConfig(request: pulumi_provider_pb.CheckRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    public checkConfig(request: pulumi_provider_pb.CheckRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    public diffConfig(request: pulumi_provider_pb.DiffRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    public diffConfig(request: pulumi_provider_pb.DiffRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    public diffConfig(request: pulumi_provider_pb.DiffRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    public configure(request: pulumi_provider_pb.ConfigureRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConfigureResponse) => void): grpc.ClientUnaryCall;
    public configure(request: pulumi_provider_pb.ConfigureRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConfigureResponse) => void): grpc.ClientUnaryCall;
    public configure(request: pulumi_provider_pb.ConfigureRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConfigureResponse) => void): grpc.ClientUnaryCall;
    public invoke(request: pulumi_provider_pb.InvokeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    public invoke(request: pulumi_provider_pb.InvokeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    public invoke(request: pulumi_provider_pb.InvokeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    public streamInvoke(request: pulumi_provider_pb.InvokeRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_provider_pb.InvokeResponse>;
    public streamInvoke(request: pulumi_provider_pb.InvokeRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_provider_pb.InvokeResponse>;
    public call(request: pulumi_provider_pb.CallRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    public call(request: pulumi_provider_pb.CallRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    public call(request: pulumi_provider_pb.CallRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    public check(request: pulumi_provider_pb.CheckRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    public check(request: pulumi_provider_pb.CheckRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    public check(request: pulumi_provider_pb.CheckRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CheckResponse) => void): grpc.ClientUnaryCall;
    public diff(request: pulumi_provider_pb.DiffRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    public diff(request: pulumi_provider_pb.DiffRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    public diff(request: pulumi_provider_pb.DiffRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.DiffResponse) => void): grpc.ClientUnaryCall;
    public create(request: pulumi_provider_pb.CreateRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CreateResponse) => void): grpc.ClientUnaryCall;
    public create(request: pulumi_provider_pb.CreateRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CreateResponse) => void): grpc.ClientUnaryCall;
    public create(request: pulumi_provider_pb.CreateRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CreateResponse) => void): grpc.ClientUnaryCall;
    public read(request: pulumi_provider_pb.ReadRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ReadResponse) => void): grpc.ClientUnaryCall;
    public read(request: pulumi_provider_pb.ReadRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ReadResponse) => void): grpc.ClientUnaryCall;
    public read(request: pulumi_provider_pb.ReadRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ReadResponse) => void): grpc.ClientUnaryCall;
    public update(request: pulumi_provider_pb.UpdateRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.UpdateResponse) => void): grpc.ClientUnaryCall;
    public update(request: pulumi_provider_pb.UpdateRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.UpdateResponse) => void): grpc.ClientUnaryCall;
    public update(request: pulumi_provider_pb.UpdateRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.UpdateResponse) => void): grpc.ClientUnaryCall;
    public delete(request: pulumi_provider_pb.DeleteRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public delete(request: pulumi_provider_pb.DeleteRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public delete(request: pulumi_provider_pb.DeleteRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public construct(request: pulumi_provider_pb.ConstructRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConstructResponse) => void): grpc.ClientUnaryCall;
    public construct(request: pulumi_provider_pb.ConstructRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConstructResponse) => void): grpc.ClientUnaryCall;
    public construct(request: pulumi_provider_pb.ConstructRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.ConstructResponse) => void): grpc.ClientUnaryCall;
    public cancel(request: google_protobuf_empty_pb.Empty, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public cancel(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public cancel(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public getPluginInfo(request: google_protobuf_empty_pb.Empty, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    public getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    public getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    public attach(request: pulumi_plugin_pb.PluginAttach, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public attach(request: pulumi_plugin_pb.PluginAttach, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public attach(request: pulumi_plugin_pb.PluginAttach, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public getMapping(request: pulumi_provider_pb.GetMappingRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
    public getMapping(request: pulumi_provider_pb.GetMappingRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
    public getMapping(request: pulumi_provider_pb.GetMappingRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
    public getMappings(request: pulumi_provider_pb.GetMappingsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingsResponse) => void): grpc.ClientUnaryCall;
    public getMappings(request: pulumi_provider_pb.GetMappingsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingsResponse) => void): grpc.ClientUnaryCall;
    public getMappings(request: pulumi_provider_pb.GetMappingsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.GetMappingsResponse) => void): grpc.ClientUnaryCall;
}
