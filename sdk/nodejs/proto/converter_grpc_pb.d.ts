// package: pulumirpc
// file: pulumi/converter.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_converter_pb from "./converter_pb";
import * as pulumi_codegen_hcl_pb from "./codegen/hcl_pb";

interface IConverterService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    convertState: IConverterService_IConvertState;
    convertProgram: IConverterService_IConvertProgram;
}

interface IConverterService_IConvertState extends grpc.MethodDefinition<pulumi_converter_pb.ConvertStateRequest, pulumi_converter_pb.ConvertStateResponse> {
    path: "/pulumirpc.Converter/ConvertState";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_converter_pb.ConvertStateRequest>;
    requestDeserialize: grpc.deserialize<pulumi_converter_pb.ConvertStateRequest>;
    responseSerialize: grpc.serialize<pulumi_converter_pb.ConvertStateResponse>;
    responseDeserialize: grpc.deserialize<pulumi_converter_pb.ConvertStateResponse>;
}
interface IConverterService_IConvertProgram extends grpc.MethodDefinition<pulumi_converter_pb.ConvertProgramRequest, pulumi_converter_pb.ConvertProgramResponse> {
    path: "/pulumirpc.Converter/ConvertProgram";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_converter_pb.ConvertProgramRequest>;
    requestDeserialize: grpc.deserialize<pulumi_converter_pb.ConvertProgramRequest>;
    responseSerialize: grpc.serialize<pulumi_converter_pb.ConvertProgramResponse>;
    responseDeserialize: grpc.deserialize<pulumi_converter_pb.ConvertProgramResponse>;
}

export const ConverterService: IConverterService;

export interface IConverterServer extends grpc.UntypedServiceImplementation {
    convertState: grpc.handleUnaryCall<pulumi_converter_pb.ConvertStateRequest, pulumi_converter_pb.ConvertStateResponse>;
    convertProgram: grpc.handleUnaryCall<pulumi_converter_pb.ConvertProgramRequest, pulumi_converter_pb.ConvertProgramResponse>;
}

export interface IConverterClient {
    convertState(request: pulumi_converter_pb.ConvertStateRequest, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertStateResponse) => void): grpc.ClientUnaryCall;
    convertState(request: pulumi_converter_pb.ConvertStateRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertStateResponse) => void): grpc.ClientUnaryCall;
    convertState(request: pulumi_converter_pb.ConvertStateRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertStateResponse) => void): grpc.ClientUnaryCall;
    convertProgram(request: pulumi_converter_pb.ConvertProgramRequest, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertProgramResponse) => void): grpc.ClientUnaryCall;
    convertProgram(request: pulumi_converter_pb.ConvertProgramRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertProgramResponse) => void): grpc.ClientUnaryCall;
    convertProgram(request: pulumi_converter_pb.ConvertProgramRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertProgramResponse) => void): grpc.ClientUnaryCall;
}

export class ConverterClient extends grpc.Client implements IConverterClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public convertState(request: pulumi_converter_pb.ConvertStateRequest, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertStateResponse) => void): grpc.ClientUnaryCall;
    public convertState(request: pulumi_converter_pb.ConvertStateRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertStateResponse) => void): grpc.ClientUnaryCall;
    public convertState(request: pulumi_converter_pb.ConvertStateRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertStateResponse) => void): grpc.ClientUnaryCall;
    public convertProgram(request: pulumi_converter_pb.ConvertProgramRequest, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertProgramResponse) => void): grpc.ClientUnaryCall;
    public convertProgram(request: pulumi_converter_pb.ConvertProgramRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertProgramResponse) => void): grpc.ClientUnaryCall;
    public convertProgram(request: pulumi_converter_pb.ConvertProgramRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_converter_pb.ConvertProgramResponse) => void): grpc.ClientUnaryCall;
}
