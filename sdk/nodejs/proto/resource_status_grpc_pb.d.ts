// package: pulumirpc
// file: pulumi/resource_status.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_resource_status_pb from "./resource_status_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as pulumi_provider_pb from "./provider_pb";

interface IResourceStatusService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    publishViewSteps: IResourceStatusService_IPublishViewSteps;
}

interface IResourceStatusService_IPublishViewSteps extends grpc.MethodDefinition<pulumi_resource_status_pb.PublishViewStepsRequest, pulumi_resource_status_pb.PublishViewStepsResponse> {
    path: "/pulumirpc.ResourceStatus/PublishViewSteps";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_resource_status_pb.PublishViewStepsRequest>;
    requestDeserialize: grpc.deserialize<pulumi_resource_status_pb.PublishViewStepsRequest>;
    responseSerialize: grpc.serialize<pulumi_resource_status_pb.PublishViewStepsResponse>;
    responseDeserialize: grpc.deserialize<pulumi_resource_status_pb.PublishViewStepsResponse>;
}

export const ResourceStatusService: IResourceStatusService;

export interface IResourceStatusServer extends grpc.UntypedServiceImplementation {
    publishViewSteps: grpc.handleUnaryCall<pulumi_resource_status_pb.PublishViewStepsRequest, pulumi_resource_status_pb.PublishViewStepsResponse>;
}

export interface IResourceStatusClient {
    publishViewSteps(request: pulumi_resource_status_pb.PublishViewStepsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_resource_status_pb.PublishViewStepsResponse) => void): grpc.ClientUnaryCall;
    publishViewSteps(request: pulumi_resource_status_pb.PublishViewStepsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_resource_status_pb.PublishViewStepsResponse) => void): grpc.ClientUnaryCall;
    publishViewSteps(request: pulumi_resource_status_pb.PublishViewStepsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_resource_status_pb.PublishViewStepsResponse) => void): grpc.ClientUnaryCall;
}

export class ResourceStatusClient extends grpc.Client implements IResourceStatusClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public publishViewSteps(request: pulumi_resource_status_pb.PublishViewStepsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_resource_status_pb.PublishViewStepsResponse) => void): grpc.ClientUnaryCall;
    public publishViewSteps(request: pulumi_resource_status_pb.PublishViewStepsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_resource_status_pb.PublishViewStepsResponse) => void): grpc.ClientUnaryCall;
    public publishViewSteps(request: pulumi_resource_status_pb.PublishViewStepsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_resource_status_pb.PublishViewStepsResponse) => void): grpc.ClientUnaryCall;
}
