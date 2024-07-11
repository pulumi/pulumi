// package: pulumirpc
// file: pulumi/resource.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_resource_pb from "./resource_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as pulumi_provider_pb from "./provider_pb";
import * as pulumi_alias_pb from "./alias_pb";
import * as pulumi_source_pb from "./source_pb";
import * as pulumi_callback_pb from "./callback_pb";

interface IResourceMonitorService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    supportsFeature: IResourceMonitorService_ISupportsFeature;
    invoke: IResourceMonitorService_IInvoke;
    streamInvoke: IResourceMonitorService_IStreamInvoke;
    call: IResourceMonitorService_ICall;
    readResource: IResourceMonitorService_IReadResource;
    registerResource: IResourceMonitorService_IRegisterResource;
    registerResourceOutputs: IResourceMonitorService_IRegisterResourceOutputs;
    registerStackTransform: IResourceMonitorService_IRegisterStackTransform;
    registerStackInvokeTransform: IResourceMonitorService_IRegisterStackInvokeTransform;
    registerPackage: IResourceMonitorService_IRegisterPackage;
}

interface IResourceMonitorService_ISupportsFeature extends grpc.MethodDefinition<pulumi_resource_pb.SupportsFeatureRequest, pulumi_resource_pb.SupportsFeatureResponse> {
    path: "/pulumirpc.ResourceMonitor/SupportsFeature";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_resource_pb.SupportsFeatureRequest>;
    requestDeserialize: grpc.deserialize<pulumi_resource_pb.SupportsFeatureRequest>;
    responseSerialize: grpc.serialize<pulumi_resource_pb.SupportsFeatureResponse>;
    responseDeserialize: grpc.deserialize<pulumi_resource_pb.SupportsFeatureResponse>;
}
interface IResourceMonitorService_IInvoke extends grpc.MethodDefinition<pulumi_resource_pb.ResourceInvokeRequest, pulumi_provider_pb.InvokeResponse> {
    path: "/pulumirpc.ResourceMonitor/Invoke";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_resource_pb.ResourceInvokeRequest>;
    requestDeserialize: grpc.deserialize<pulumi_resource_pb.ResourceInvokeRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.InvokeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.InvokeResponse>;
}
interface IResourceMonitorService_IStreamInvoke extends grpc.MethodDefinition<pulumi_resource_pb.ResourceInvokeRequest, pulumi_provider_pb.InvokeResponse> {
    path: "/pulumirpc.ResourceMonitor/StreamInvoke";
    requestStream: false;
    responseStream: true;
    requestSerialize: grpc.serialize<pulumi_resource_pb.ResourceInvokeRequest>;
    requestDeserialize: grpc.deserialize<pulumi_resource_pb.ResourceInvokeRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.InvokeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.InvokeResponse>;
}
interface IResourceMonitorService_ICall extends grpc.MethodDefinition<pulumi_resource_pb.ResourceCallRequest, pulumi_provider_pb.CallResponse> {
    path: "/pulumirpc.ResourceMonitor/Call";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_resource_pb.ResourceCallRequest>;
    requestDeserialize: grpc.deserialize<pulumi_resource_pb.ResourceCallRequest>;
    responseSerialize: grpc.serialize<pulumi_provider_pb.CallResponse>;
    responseDeserialize: grpc.deserialize<pulumi_provider_pb.CallResponse>;
}
interface IResourceMonitorService_IReadResource extends grpc.MethodDefinition<pulumi_resource_pb.ReadResourceRequest, pulumi_resource_pb.ReadResourceResponse> {
    path: "/pulumirpc.ResourceMonitor/ReadResource";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_resource_pb.ReadResourceRequest>;
    requestDeserialize: grpc.deserialize<pulumi_resource_pb.ReadResourceRequest>;
    responseSerialize: grpc.serialize<pulumi_resource_pb.ReadResourceResponse>;
    responseDeserialize: grpc.deserialize<pulumi_resource_pb.ReadResourceResponse>;
}
interface IResourceMonitorService_IRegisterResource extends grpc.MethodDefinition<pulumi_resource_pb.RegisterResourceRequest, pulumi_resource_pb.RegisterResourceResponse> {
    path: "/pulumirpc.ResourceMonitor/RegisterResource";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_resource_pb.RegisterResourceRequest>;
    requestDeserialize: grpc.deserialize<pulumi_resource_pb.RegisterResourceRequest>;
    responseSerialize: grpc.serialize<pulumi_resource_pb.RegisterResourceResponse>;
    responseDeserialize: grpc.deserialize<pulumi_resource_pb.RegisterResourceResponse>;
}
interface IResourceMonitorService_IRegisterResourceOutputs extends grpc.MethodDefinition<pulumi_resource_pb.RegisterResourceOutputsRequest, google_protobuf_empty_pb.Empty> {
    path: "/pulumirpc.ResourceMonitor/RegisterResourceOutputs";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_resource_pb.RegisterResourceOutputsRequest>;
    requestDeserialize: grpc.deserialize<pulumi_resource_pb.RegisterResourceOutputsRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IResourceMonitorService_IRegisterStackTransform extends grpc.MethodDefinition<pulumi_callback_pb.Callback, google_protobuf_empty_pb.Empty> {
    path: "/pulumirpc.ResourceMonitor/RegisterStackTransform";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_callback_pb.Callback>;
    requestDeserialize: grpc.deserialize<pulumi_callback_pb.Callback>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IResourceMonitorService_IRegisterStackInvokeTransform extends grpc.MethodDefinition<pulumi_callback_pb.Callback, google_protobuf_empty_pb.Empty> {
    path: "/pulumirpc.ResourceMonitor/RegisterStackInvokeTransform";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_callback_pb.Callback>;
    requestDeserialize: grpc.deserialize<pulumi_callback_pb.Callback>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IResourceMonitorService_IRegisterPackage extends grpc.MethodDefinition<pulumi_resource_pb.RegisterPackageRequest, pulumi_resource_pb.RegisterPackageResponse> {
    path: "/pulumirpc.ResourceMonitor/RegisterPackage";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_resource_pb.RegisterPackageRequest>;
    requestDeserialize: grpc.deserialize<pulumi_resource_pb.RegisterPackageRequest>;
    responseSerialize: grpc.serialize<pulumi_resource_pb.RegisterPackageResponse>;
    responseDeserialize: grpc.deserialize<pulumi_resource_pb.RegisterPackageResponse>;
}

export const ResourceMonitorService: IResourceMonitorService;

export interface IResourceMonitorServer extends grpc.UntypedServiceImplementation {
    supportsFeature: grpc.handleUnaryCall<pulumi_resource_pb.SupportsFeatureRequest, pulumi_resource_pb.SupportsFeatureResponse>;
    invoke: grpc.handleUnaryCall<pulumi_resource_pb.ResourceInvokeRequest, pulumi_provider_pb.InvokeResponse>;
    streamInvoke: grpc.handleServerStreamingCall<pulumi_resource_pb.ResourceInvokeRequest, pulumi_provider_pb.InvokeResponse>;
    call: grpc.handleUnaryCall<pulumi_resource_pb.ResourceCallRequest, pulumi_provider_pb.CallResponse>;
    readResource: grpc.handleUnaryCall<pulumi_resource_pb.ReadResourceRequest, pulumi_resource_pb.ReadResourceResponse>;
    registerResource: grpc.handleUnaryCall<pulumi_resource_pb.RegisterResourceRequest, pulumi_resource_pb.RegisterResourceResponse>;
    registerResourceOutputs: grpc.handleUnaryCall<pulumi_resource_pb.RegisterResourceOutputsRequest, google_protobuf_empty_pb.Empty>;
    registerStackTransform: grpc.handleUnaryCall<pulumi_callback_pb.Callback, google_protobuf_empty_pb.Empty>;
    registerStackInvokeTransform: grpc.handleUnaryCall<pulumi_callback_pb.Callback, google_protobuf_empty_pb.Empty>;
    registerPackage: grpc.handleUnaryCall<pulumi_resource_pb.RegisterPackageRequest, pulumi_resource_pb.RegisterPackageResponse>;
}

export interface IResourceMonitorClient {
    supportsFeature(request: pulumi_resource_pb.SupportsFeatureRequest, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.SupportsFeatureResponse) => void): grpc.ClientUnaryCall;
    supportsFeature(request: pulumi_resource_pb.SupportsFeatureRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.SupportsFeatureResponse) => void): grpc.ClientUnaryCall;
    supportsFeature(request: pulumi_resource_pb.SupportsFeatureRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.SupportsFeatureResponse) => void): grpc.ClientUnaryCall;
    invoke(request: pulumi_resource_pb.ResourceInvokeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    invoke(request: pulumi_resource_pb.ResourceInvokeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    invoke(request: pulumi_resource_pb.ResourceInvokeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    streamInvoke(request: pulumi_resource_pb.ResourceInvokeRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_provider_pb.InvokeResponse>;
    streamInvoke(request: pulumi_resource_pb.ResourceInvokeRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_provider_pb.InvokeResponse>;
    call(request: pulumi_resource_pb.ResourceCallRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    call(request: pulumi_resource_pb.ResourceCallRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    call(request: pulumi_resource_pb.ResourceCallRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    readResource(request: pulumi_resource_pb.ReadResourceRequest, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.ReadResourceResponse) => void): grpc.ClientUnaryCall;
    readResource(request: pulumi_resource_pb.ReadResourceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.ReadResourceResponse) => void): grpc.ClientUnaryCall;
    readResource(request: pulumi_resource_pb.ReadResourceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.ReadResourceResponse) => void): grpc.ClientUnaryCall;
    registerResource(request: pulumi_resource_pb.RegisterResourceRequest, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterResourceResponse) => void): grpc.ClientUnaryCall;
    registerResource(request: pulumi_resource_pb.RegisterResourceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterResourceResponse) => void): grpc.ClientUnaryCall;
    registerResource(request: pulumi_resource_pb.RegisterResourceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterResourceResponse) => void): grpc.ClientUnaryCall;
    registerResourceOutputs(request: pulumi_resource_pb.RegisterResourceOutputsRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    registerResourceOutputs(request: pulumi_resource_pb.RegisterResourceOutputsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    registerResourceOutputs(request: pulumi_resource_pb.RegisterResourceOutputsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    registerStackTransform(request: pulumi_callback_pb.Callback, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    registerStackTransform(request: pulumi_callback_pb.Callback, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    registerStackTransform(request: pulumi_callback_pb.Callback, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    registerStackInvokeTransform(request: pulumi_callback_pb.Callback, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    registerStackInvokeTransform(request: pulumi_callback_pb.Callback, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    registerStackInvokeTransform(request: pulumi_callback_pb.Callback, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    registerPackage(request: pulumi_resource_pb.RegisterPackageRequest, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterPackageResponse) => void): grpc.ClientUnaryCall;
    registerPackage(request: pulumi_resource_pb.RegisterPackageRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterPackageResponse) => void): grpc.ClientUnaryCall;
    registerPackage(request: pulumi_resource_pb.RegisterPackageRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterPackageResponse) => void): grpc.ClientUnaryCall;
}

export class ResourceMonitorClient extends grpc.Client implements IResourceMonitorClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public supportsFeature(request: pulumi_resource_pb.SupportsFeatureRequest, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.SupportsFeatureResponse) => void): grpc.ClientUnaryCall;
    public supportsFeature(request: pulumi_resource_pb.SupportsFeatureRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.SupportsFeatureResponse) => void): grpc.ClientUnaryCall;
    public supportsFeature(request: pulumi_resource_pb.SupportsFeatureRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.SupportsFeatureResponse) => void): grpc.ClientUnaryCall;
    public invoke(request: pulumi_resource_pb.ResourceInvokeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    public invoke(request: pulumi_resource_pb.ResourceInvokeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    public invoke(request: pulumi_resource_pb.ResourceInvokeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.InvokeResponse) => void): grpc.ClientUnaryCall;
    public streamInvoke(request: pulumi_resource_pb.ResourceInvokeRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_provider_pb.InvokeResponse>;
    public streamInvoke(request: pulumi_resource_pb.ResourceInvokeRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<pulumi_provider_pb.InvokeResponse>;
    public call(request: pulumi_resource_pb.ResourceCallRequest, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    public call(request: pulumi_resource_pb.ResourceCallRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    public call(request: pulumi_resource_pb.ResourceCallRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_provider_pb.CallResponse) => void): grpc.ClientUnaryCall;
    public readResource(request: pulumi_resource_pb.ReadResourceRequest, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.ReadResourceResponse) => void): grpc.ClientUnaryCall;
    public readResource(request: pulumi_resource_pb.ReadResourceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.ReadResourceResponse) => void): grpc.ClientUnaryCall;
    public readResource(request: pulumi_resource_pb.ReadResourceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.ReadResourceResponse) => void): grpc.ClientUnaryCall;
    public registerResource(request: pulumi_resource_pb.RegisterResourceRequest, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterResourceResponse) => void): grpc.ClientUnaryCall;
    public registerResource(request: pulumi_resource_pb.RegisterResourceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterResourceResponse) => void): grpc.ClientUnaryCall;
    public registerResource(request: pulumi_resource_pb.RegisterResourceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterResourceResponse) => void): grpc.ClientUnaryCall;
    public registerResourceOutputs(request: pulumi_resource_pb.RegisterResourceOutputsRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public registerResourceOutputs(request: pulumi_resource_pb.RegisterResourceOutputsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public registerResourceOutputs(request: pulumi_resource_pb.RegisterResourceOutputsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public registerStackTransform(request: pulumi_callback_pb.Callback, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public registerStackTransform(request: pulumi_callback_pb.Callback, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public registerStackTransform(request: pulumi_callback_pb.Callback, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public registerStackInvokeTransform(request: pulumi_callback_pb.Callback, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public registerStackInvokeTransform(request: pulumi_callback_pb.Callback, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public registerStackInvokeTransform(request: pulumi_callback_pb.Callback, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public registerPackage(request: pulumi_resource_pb.RegisterPackageRequest, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterPackageResponse) => void): grpc.ClientUnaryCall;
    public registerPackage(request: pulumi_resource_pb.RegisterPackageRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterPackageResponse) => void): grpc.ClientUnaryCall;
    public registerPackage(request: pulumi_resource_pb.RegisterPackageRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_resource_pb.RegisterPackageResponse) => void): grpc.ClientUnaryCall;
}
