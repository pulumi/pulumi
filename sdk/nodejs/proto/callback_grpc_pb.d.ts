// package: pulumirpc
// file: pulumi/callback.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_callback_pb from "./callback_pb";

interface ICallbacksService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    invoke: ICallbacksService_IInvoke;
}

interface ICallbacksService_IInvoke extends grpc.MethodDefinition<pulumi_callback_pb.CallbackInvokeRequest, pulumi_callback_pb.CallbackInvokeResponse> {
    path: "/pulumirpc.Callbacks/Invoke";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_callback_pb.CallbackInvokeRequest>;
    requestDeserialize: grpc.deserialize<pulumi_callback_pb.CallbackInvokeRequest>;
    responseSerialize: grpc.serialize<pulumi_callback_pb.CallbackInvokeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_callback_pb.CallbackInvokeResponse>;
}

export const CallbacksService: ICallbacksService;

export interface ICallbacksServer extends grpc.UntypedServiceImplementation {
    invoke: grpc.handleUnaryCall<pulumi_callback_pb.CallbackInvokeRequest, pulumi_callback_pb.CallbackInvokeResponse>;
}

export interface ICallbacksClient {
    invoke(request: pulumi_callback_pb.CallbackInvokeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_callback_pb.CallbackInvokeResponse) => void): grpc.ClientUnaryCall;
    invoke(request: pulumi_callback_pb.CallbackInvokeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_callback_pb.CallbackInvokeResponse) => void): grpc.ClientUnaryCall;
    invoke(request: pulumi_callback_pb.CallbackInvokeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_callback_pb.CallbackInvokeResponse) => void): grpc.ClientUnaryCall;
}

export class CallbacksClient extends grpc.Client implements ICallbacksClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public invoke(request: pulumi_callback_pb.CallbackInvokeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_callback_pb.CallbackInvokeResponse) => void): grpc.ClientUnaryCall;
    public invoke(request: pulumi_callback_pb.CallbackInvokeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_callback_pb.CallbackInvokeResponse) => void): grpc.ClientUnaryCall;
    public invoke(request: pulumi_callback_pb.CallbackInvokeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_callback_pb.CallbackInvokeResponse) => void): grpc.ClientUnaryCall;
}
