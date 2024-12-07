// package: codegen
// file: pulumi/codegen/loader.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_codegen_loader_pb from "../codegen/loader_pb";

interface ILoaderService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    getSchema: ILoaderService_IGetSchema;
    getPackageSpec: ILoaderService_IGetPackageSpec;
    getResourceSpec: ILoaderService_IGetResourceSpec;
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
interface ILoaderService_IGetPackageSpec extends grpc.MethodDefinition<pulumi_codegen_loader_pb.PackageDescriptor, pulumi_codegen_loader_pb.PackageSpec> {
    path: "/codegen.Loader/GetPackageSpec";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_loader_pb.PackageDescriptor>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.PackageDescriptor>;
    responseSerialize: grpc.serialize<pulumi_codegen_loader_pb.PackageSpec>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.PackageSpec>;
}
interface ILoaderService_IGetResourceSpec extends grpc.MethodDefinition<pulumi_codegen_loader_pb.PackageDescriptorMember, pulumi_codegen_loader_pb.ResourceSpec> {
    path: "/codegen.Loader/GetResourceSpec";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_loader_pb.PackageDescriptorMember>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.PackageDescriptorMember>;
    responseSerialize: grpc.serialize<pulumi_codegen_loader_pb.ResourceSpec>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_loader_pb.ResourceSpec>;
}

export const LoaderService: ILoaderService;

export interface ILoaderServer extends grpc.UntypedServiceImplementation {
    getSchema: grpc.handleUnaryCall<pulumi_codegen_loader_pb.GetSchemaRequest, pulumi_codegen_loader_pb.GetSchemaResponse>;
    getPackageSpec: grpc.handleUnaryCall<pulumi_codegen_loader_pb.PackageDescriptor, pulumi_codegen_loader_pb.PackageSpec>;
    getResourceSpec: grpc.handleUnaryCall<pulumi_codegen_loader_pb.PackageDescriptorMember, pulumi_codegen_loader_pb.ResourceSpec>;
}

export interface ILoaderClient {
    getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    getPackageSpec(request: pulumi_codegen_loader_pb.PackageDescriptor, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageSpec) => void): grpc.ClientUnaryCall;
    getPackageSpec(request: pulumi_codegen_loader_pb.PackageDescriptor, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageSpec) => void): grpc.ClientUnaryCall;
    getPackageSpec(request: pulumi_codegen_loader_pb.PackageDescriptor, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageSpec) => void): grpc.ClientUnaryCall;
    getResourceSpec(request: pulumi_codegen_loader_pb.PackageDescriptorMember, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.ResourceSpec) => void): grpc.ClientUnaryCall;
    getResourceSpec(request: pulumi_codegen_loader_pb.PackageDescriptorMember, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.ResourceSpec) => void): grpc.ClientUnaryCall;
    getResourceSpec(request: pulumi_codegen_loader_pb.PackageDescriptorMember, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.ResourceSpec) => void): grpc.ClientUnaryCall;
}

export class LoaderClient extends grpc.Client implements ILoaderClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public getSchema(request: pulumi_codegen_loader_pb.GetSchemaRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.GetSchemaResponse) => void): grpc.ClientUnaryCall;
    public getPackageSpec(request: pulumi_codegen_loader_pb.PackageDescriptor, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageSpec) => void): grpc.ClientUnaryCall;
    public getPackageSpec(request: pulumi_codegen_loader_pb.PackageDescriptor, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageSpec) => void): grpc.ClientUnaryCall;
    public getPackageSpec(request: pulumi_codegen_loader_pb.PackageDescriptor, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.PackageSpec) => void): grpc.ClientUnaryCall;
    public getResourceSpec(request: pulumi_codegen_loader_pb.PackageDescriptorMember, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.ResourceSpec) => void): grpc.ClientUnaryCall;
    public getResourceSpec(request: pulumi_codegen_loader_pb.PackageDescriptorMember, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.ResourceSpec) => void): grpc.ClientUnaryCall;
    public getResourceSpec(request: pulumi_codegen_loader_pb.PackageDescriptorMember, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_loader_pb.ResourceSpec) => void): grpc.ClientUnaryCall;
}
