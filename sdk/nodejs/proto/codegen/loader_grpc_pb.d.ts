// package: codegen
// file: pulumi/codegen/loader.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_codegen_loader_pb from "../codegen/loader_pb";

interface ILoaderService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    getSchema: ILoaderService_IGetSchema;
    getPackageInfo: ILoaderService_IGetPackageInfo;
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
interface ILoaderService_IGetPackageInfo extends grpc.MethodDefinition<pulumi_codegen_loader_pb.GetSchemaRequest, pulumi_codegen_loader_pb.PackageInfo> {
    path: "/codegen.Loader/GetPackageInfo";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_loader_pb.GetSchemaRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.GetSchemaRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_loader_pb.PackageInfo>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.PackageInfo>;
}

export const LoaderService: ILoaderService;

export interface ILoaderServer extends grpc.UntypedServiceImplementation {
    getSchema: grpc.handleUnaryCall<pulumi_codegen_loader_pb.GetSchemaRequest, pulumi_codegen_loader_pb.GetSchemaResponse>;
    getPackageInfo: grpc.handleUnaryCall<pulumi_codegen_loader_pb.GetSchemaRequest, pulumi_codegen_loader_pb.PackageInfo>;
}

export interface ILoaderClient {
    getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
}

export class LoaderClient extends grpc.Client implements ILoaderClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
}

interface IPartialLoaderService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    getPackageInfo: IPartialLoaderService_IGetPackageInfo;
}

interface IPartialLoaderService_IGetPackageInfo extends grpc.MethodDefinition<pulumi_codegen_loader_pb.GetPartialSchemaRequest, pulumi_codegen_loader_pb.PackageInfo> {
    path: "/codegen.PartialLoader/GetPackageInfo";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_loader_pb.GetPartialSchemaRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.GetPartialSchemaRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_loader_pb.PackageInfo>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.PackageInfo>;
}

export const PartialLoaderService: IPartialLoaderService;

export interface IPartialLoaderServer extends grpc.UntypedServiceImplementation {
    getPackageInfo: grpc.handleUnaryCall<pulumi_codegen_loader_pb.GetPartialSchemaRequest, pulumi_codegen_loader_pb.PackageInfo>;
}

export interface IPartialLoaderClient {
    getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
}

export class PartialLoaderClient extends grpc.Client implements IPartialLoaderClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_loader_pb.GetPartialSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageInfo) => void): grpc.ClientUnaryCall;
}
