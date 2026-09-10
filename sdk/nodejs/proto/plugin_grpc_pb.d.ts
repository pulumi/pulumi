// package: pulumirpc
// file: pulumi/plugin.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_plugin_pb from "./plugin_pb";

interface IPackageResolverService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    resolvePackage: IPackageResolverService_IResolvePackage;
}

interface IPackageResolverService_IResolvePackage extends grpc.MethodDefinition<pulumi_plugin_pb.PackageSpec, pulumi_plugin_pb.PackageDependency> {
    path: "/pulumirpc.PackageResolver/ResolvePackage";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_plugin_pb.PackageSpec>;
    requestDeserialize: grpc.deserialize<pulumi_plugin_pb.PackageSpec>;
    responseSerialize: grpc.serialize<pulumi_plugin_pb.PackageDependency>;
    responseDeserialize: grpc.deserialize<pulumi_plugin_pb.PackageDependency>;
}

export const PackageResolverService: IPackageResolverService;

export interface IPackageResolverServer extends grpc.UntypedServiceImplementation {
    resolvePackage: grpc.handleUnaryCall<pulumi_plugin_pb.PackageSpec, pulumi_plugin_pb.PackageDependency>;
}

export interface IPackageResolverClient {
    resolvePackage(request: pulumi_plugin_pb.PackageSpec, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PackageDependency) => void): grpc.ClientUnaryCall;
    resolvePackage(request: pulumi_plugin_pb.PackageSpec, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PackageDependency) => void): grpc.ClientUnaryCall;
    resolvePackage(request: pulumi_plugin_pb.PackageSpec, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PackageDependency) => void): grpc.ClientUnaryCall;
}

export class PackageResolverClient extends grpc.Client implements IPackageResolverClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public resolvePackage(request: pulumi_plugin_pb.PackageSpec, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PackageDependency) => void): grpc.ClientUnaryCall;
    public resolvePackage(request: pulumi_plugin_pb.PackageSpec, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PackageDependency) => void): grpc.ClientUnaryCall;
    public resolvePackage(request: pulumi_plugin_pb.PackageSpec, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PackageDependency) => void): grpc.ClientUnaryCall;
}
