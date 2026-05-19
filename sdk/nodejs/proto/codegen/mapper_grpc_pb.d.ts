// package: codegen
// file: pulumi/codegen/mapper.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_codegen_mapper_pb from "../codegen/mapper_pb";

interface IMapperService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    getMapping: IMapperService_IGetMapping;
}

interface IMapperService_IGetMapping extends grpc.MethodDefinition<pulumi_codegen_mapper_pb.GetMappingRequest, pulumi_codegen_mapper_pb.GetMappingResponse> {
    path: "/codegen.Mapper/GetMapping";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_mapper_pb.GetMappingRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_mapper_pb.GetMappingRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_mapper_pb.GetMappingResponse>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_mapper_pb.GetMappingResponse>;
}

export const MapperService: IMapperService;

export interface IMapperServer extends grpc.UntypedServiceImplementation {
    getMapping: grpc.handleUnaryCall<pulumi_codegen_mapper_pb.GetMappingRequest, pulumi_codegen_mapper_pb.GetMappingResponse>;
}

export interface IMapperClient {
    getMapping(request: pulumi_codegen_mapper_pb.GetMappingRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_mapper_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
    getMapping(request: pulumi_codegen_mapper_pb.GetMappingRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_mapper_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
    getMapping(request: pulumi_codegen_mapper_pb.GetMappingRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_mapper_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
}

export class MapperClient extends grpc.Client implements IMapperClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public getMapping(request: pulumi_codegen_mapper_pb.GetMappingRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_mapper_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
    public getMapping(request: pulumi_codegen_mapper_pb.GetMappingRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_mapper_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
    public getMapping(request: pulumi_codegen_mapper_pb.GetMappingRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_mapper_pb.GetMappingResponse) => void): grpc.ClientUnaryCall;
}
