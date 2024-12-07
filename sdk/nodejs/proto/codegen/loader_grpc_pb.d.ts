// package: codegen
// file: pulumi/codegen/loader.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_codegen_loader_pb from "../codegen/loader_pb";
import * as pulumi_codegen_schema_schema_pb from "../codegen/schema/schema_pb";

interface ILoaderService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    getSchema: ILoaderService_IGetSchema;
    getPackageInfo: ILoaderService_IGetPackageInfo;
    getResources: ILoaderService_IGetResources;
    getResource: ILoaderService_IGetResource;
}

interface ILoaderService_IGetSchema extends grpc.MethodDefinition<pulumi_codegen_loader_pb.GetSchemaRequest, pulumi_codegen_loader_pb.GetSchemaResponse> {
    path: "/codegen.Loader/GetSchema";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_loader_pb.GetSchemaRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.GetSchemaRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_loader_pb.GetSchemaResponse>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.GetSchemaResponse>;
}
interface ILoaderService_IGetPackageInfo extends grpc.MethodDefinition<pulumi_codegen_loader_pb.GetSchemaRequest, pulumi_codegen_schema_schema_pb.PackageInfo> {
    path: "/codegen.Loader/GetPackageInfo";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_loader_pb.GetSchemaRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.GetSchemaRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_schema_schema_pb.PackageInfo>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_schema_schema_pb.PackageInfo>;
}
interface ILoaderService_IGetResources extends grpc.MethodDefinition<pulumi_codegen_loader_pb.GetSchemaRequest, pulumi_codegen_schema_schema_pb.List> {
    path: "/codegen.Loader/GetResources";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_loader_pb.GetSchemaRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.GetSchemaRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_schema_schema_pb.List>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_schema_schema_pb.List>;
}
interface ILoaderService_IGetResource extends grpc.MethodDefinition<pulumi_codegen_loader_pb.GetSchemaPartRequest, pulumi_codegen_schema_schema_pb.Resource> {
    path: "/codegen.Loader/GetResource";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_loader_pb.GetSchemaPartRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.GetSchemaPartRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_schema_schema_pb.Resource>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_schema_schema_pb.Resource>;
}

export const LoaderService: ILoaderService;

export interface ILoaderServer extends grpc.UntypedServiceImplementation {
    getSchema: grpc.handleUnaryCall<pulumi_codegen_loader_pb.GetSchemaRequest, pulumi_codegen_loader_pb.GetSchemaResponse>;
    getPackageInfo: grpc.handleUnaryCall<pulumi_codegen_loader_pb.GetSchemaRequest, pulumi_codegen_schema_schema_pb.PackageInfo>;
    getResources: grpc.handleUnaryCall<pulumi_codegen_loader_pb.GetSchemaRequest, pulumi_codegen_schema_schema_pb.List>;
    getResource: grpc.handleUnaryCall<pulumi_codegen_loader_pb.GetSchemaPartRequest, pulumi_codegen_schema_schema_pb.Resource>;
}

export interface ILoaderClient {
    getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    getResources(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    getResources(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    getResources(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    getResource(request: pulumi_codegen_loader_pb.GetSchemaPartRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
    getResource(request: pulumi_codegen_loader_pb.GetSchemaPartRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
    getResource(request: pulumi_codegen_loader_pb.GetSchemaPartRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
}

export class LoaderClient extends grpc.Client implements ILoaderClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    public getResources(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    public getResources(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    public getResources(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    public getResource(request: pulumi_codegen_loader_pb.GetSchemaPartRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
    public getResource(request: pulumi_codegen_loader_pb.GetSchemaPartRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
    public getResource(request: pulumi_codegen_loader_pb.GetSchemaPartRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
}

interface IPartialLoaderService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    getPackageInfo: IPartialLoaderService_IGetPackageInfo;
    getResources: IPartialLoaderService_IGetResources;
    getResource: IPartialLoaderService_IGetResource;
}

interface IPartialLoaderService_IGetPackageInfo extends grpc.MethodDefinition<pulumi_codegen_loader_pb.GetPartialSchemaRequest, pulumi_codegen_schema_schema_pb.PackageInfo> {
    path: "/codegen.PartialLoader/GetPackageInfo";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_loader_pb.GetPartialSchemaRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.GetPartialSchemaRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_schema_schema_pb.PackageInfo>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_schema_schema_pb.PackageInfo>;
}
interface IPartialLoaderService_IGetResources extends grpc.MethodDefinition<pulumi_codegen_loader_pb.GetPartialSchemaRequest, pulumi_codegen_schema_schema_pb.List> {
    path: "/codegen.PartialLoader/GetResources";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_loader_pb.GetPartialSchemaRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.GetPartialSchemaRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_schema_schema_pb.List>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_schema_schema_pb.List>;
}
interface IPartialLoaderService_IGetResource extends grpc.MethodDefinition<pulumi_codegen_loader_pb.GetPartialSchemaPartRequest, pulumi_codegen_schema_schema_pb.Resource> {
    path: "/codegen.PartialLoader/GetResource";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_loader_pb.GetPartialSchemaPartRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.GetPartialSchemaPartRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_schema_schema_pb.Resource>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_schema_schema_pb.Resource>;
}

export const PartialLoaderService: IPartialLoaderService;

export interface IPartialLoaderServer extends grpc.UntypedServiceImplementation {
    getPackageInfo: grpc.handleUnaryCall<pulumi_codegen_loader_pb.GetPartialSchemaRequest, pulumi_codegen_schema_schema_pb.PackageInfo>;
    getResources: grpc.handleUnaryCall<pulumi_codegen_loader_pb.GetPartialSchemaRequest, pulumi_codegen_schema_schema_pb.List>;
    getResource: grpc.handleUnaryCall<pulumi_codegen_loader_pb.GetPartialSchemaPartRequest, pulumi_codegen_schema_schema_pb.Resource>;
}

export interface IPartialLoaderClient {
    getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    getResources(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    getResources(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    getResources(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    getResource(request: pulumi_codegen_loader_pb.GetPartialSchemaPartRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
    getResource(request: pulumi_codegen_loader_pb.GetPartialSchemaPartRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
    getResource(request: pulumi_codegen_loader_pb.GetPartialSchemaPartRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
}

export class PartialLoaderClient extends grpc.Client implements IPartialLoaderClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    public getResources(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    public getResources(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    public getResources(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.List) => void): grpc.ClientUnaryCall;
    public getResource(request: pulumi_codegen_loader_pb.GetPartialSchemaPartRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
    public getResource(request: pulumi_codegen_loader_pb.GetPartialSchemaPartRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
    public getResource(request: pulumi_codegen_loader_pb.GetPartialSchemaPartRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_schema_schema_pb.Resource) => void): grpc.ClientUnaryCall;
}
