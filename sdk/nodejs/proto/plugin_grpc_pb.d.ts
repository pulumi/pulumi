// package: pulumirpc
// file: pulumi/plugin.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_plugin_pb from "./plugin_pb";

interface IPackageResolverService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    resolvePackage: IPackageResolverService_IResolvePackage;
}

interface IPackageResolverService_IResolvePackage extends grpc.MethodDefinition<pulumi_plugin_pb.ResolvePackageRequest, pulumi_plugin_pb.ResolvePackageResponse> {
    path: "/pulumirpc.PackageResolver/ResolvePackage";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_plugin_pb.ResolvePackageRequest>;
    requestDeserialize: grpc.deserialize<pulumi_plugin_pb.ResolvePackageRequest>;
    responseSerialize: grpc.serialize<pulumi_plugin_pb.ResolvePackageResponse>;
    responseDeserialize: grpc.deserialize<pulumi_plugin_pb.ResolvePackageResponse>;
}

export const PackageResolverService: IPackageResolverService;

export interface IPackageResolverServer extends grpc.UntypedServiceImplementation {
    resolvePackage: grpc.handleUnaryCall<pulumi_plugin_pb.ResolvePackageRequest, pulumi_plugin_pb.ResolvePackageResponse>;
}

export interface IPackageResolverClient {
    resolvePackage(request: pulumi_plugin_pb.ResolvePackageRequest, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.ResolvePackageResponse) => void): grpc.ClientUnaryCall;
    resolvePackage(request: pulumi_plugin_pb.ResolvePackageRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.ResolvePackageResponse) => void): grpc.ClientUnaryCall;
    resolvePackage(request: pulumi_plugin_pb.ResolvePackageRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.ResolvePackageResponse) => void): grpc.ClientUnaryCall;
}

export class PackageResolverClient extends grpc.Client implements IPackageResolverClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public resolvePackage(request: pulumi_plugin_pb.ResolvePackageRequest, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.ResolvePackageResponse) => void): grpc.ClientUnaryCall;
    public resolvePackage(request: pulumi_plugin_pb.ResolvePackageRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.ResolvePackageResponse) => void): grpc.ClientUnaryCall;
    public resolvePackage(request: pulumi_plugin_pb.ResolvePackageRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.ResolvePackageResponse) => void): grpc.ClientUnaryCall;
}
