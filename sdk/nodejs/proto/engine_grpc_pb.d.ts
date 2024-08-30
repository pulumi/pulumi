// package: pulumirpc
// file: pulumi/engine.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_engine_pb from "./engine_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

interface IEngineService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    log: IEngineService_ILog;
    getRootResource: IEngineService_IGetRootResource;
    setRootResource: IEngineService_ISetRootResource;
    startDebugger: IEngineService_IStartDebugger;
}

interface IEngineService_ILog extends grpc.MethodDefinition<pulumi_engine_pb.LogRequest, google_protobuf_empty_pb.Empty> {
    path: "/pulumirpc.Engine/Log";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_engine_pb.LogRequest>;
    requestDeserialize: grpc.deserialize<pulumi_engine_pb.LogRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IEngineService_IGetRootResource extends grpc.MethodDefinition<pulumi_engine_pb.GetRootResourceRequest, pulumi_engine_pb.GetRootResourceResponse> {
    path: "/pulumirpc.Engine/GetRootResource";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_engine_pb.GetRootResourceRequest>;
    requestDeserialize: grpc.deserialize<pulumi_engine_pb.GetRootResourceRequest>;
    responseSerialize: grpc.serialize<pulumi_engine_pb.GetRootResourceResponse>;
    responseDeserialize: grpc.deserialize<pulumi_engine_pb.GetRootResourceResponse>;
}
interface IEngineService_ISetRootResource extends grpc.MethodDefinition<pulumi_engine_pb.SetRootResourceRequest, pulumi_engine_pb.SetRootResourceResponse> {
    path: "/pulumirpc.Engine/SetRootResource";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_engine_pb.SetRootResourceRequest>;
    requestDeserialize: grpc.deserialize<pulumi_engine_pb.SetRootResourceRequest>;
    responseSerialize: grpc.serialize<pulumi_engine_pb.SetRootResourceResponse>;
    responseDeserialize: grpc.deserialize<pulumi_engine_pb.SetRootResourceResponse>;
}
interface IEngineService_IStartDebugger extends grpc.MethodDefinition<pulumi_engine_pb.StartDebuggerRequest, google_protobuf_empty_pb.Empty> {
    path: "/pulumirpc.Engine/StartDebugger";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_engine_pb.StartDebuggerRequest>;
    requestDeserialize: grpc.deserialize<pulumi_engine_pb.StartDebuggerRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}

export const EngineService: IEngineService;

export interface IEngineServer extends grpc.UntypedServiceImplementation {
    log: grpc.handleUnaryCall<pulumi_engine_pb.LogRequest, google_protobuf_empty_pb.Empty>;
    getRootResource: grpc.handleUnaryCall<pulumi_engine_pb.GetRootResourceRequest, pulumi_engine_pb.GetRootResourceResponse>;
    setRootResource: grpc.handleUnaryCall<pulumi_engine_pb.SetRootResourceRequest, pulumi_engine_pb.SetRootResourceResponse>;
    startDebugger: grpc.handleUnaryCall<pulumi_engine_pb.StartDebuggerRequest, google_protobuf_empty_pb.Empty>;
}

export interface IEngineClient {
    log(request: pulumi_engine_pb.LogRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    log(request: pulumi_engine_pb.LogRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    log(request: pulumi_engine_pb.LogRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    getRootResource(request: pulumi_engine_pb.GetRootResourceRequest, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.GetRootResourceResponse) => void): grpc.ClientUnaryCall;
    getRootResource(request: pulumi_engine_pb.GetRootResourceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.GetRootResourceResponse) => void): grpc.ClientUnaryCall;
    getRootResource(request: pulumi_engine_pb.GetRootResourceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.GetRootResourceResponse) => void): grpc.ClientUnaryCall;
    setRootResource(request: pulumi_engine_pb.SetRootResourceRequest, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.SetRootResourceResponse) => void): grpc.ClientUnaryCall;
    setRootResource(request: pulumi_engine_pb.SetRootResourceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.SetRootResourceResponse) => void): grpc.ClientUnaryCall;
    setRootResource(request: pulumi_engine_pb.SetRootResourceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.SetRootResourceResponse) => void): grpc.ClientUnaryCall;
    startDebugger(request: pulumi_engine_pb.StartDebuggerRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    startDebugger(request: pulumi_engine_pb.StartDebuggerRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    startDebugger(request: pulumi_engine_pb.StartDebuggerRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
}

export class EngineClient extends grpc.Client implements IEngineClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public log(request: pulumi_engine_pb.LogRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public log(request: pulumi_engine_pb.LogRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public log(request: pulumi_engine_pb.LogRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public getRootResource(request: pulumi_engine_pb.GetRootResourceRequest, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.GetRootResourceResponse) => void): grpc.ClientUnaryCall;
    public getRootResource(request: pulumi_engine_pb.GetRootResourceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.GetRootResourceResponse) => void): grpc.ClientUnaryCall;
    public getRootResource(request: pulumi_engine_pb.GetRootResourceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.GetRootResourceResponse) => void): grpc.ClientUnaryCall;
    public setRootResource(request: pulumi_engine_pb.SetRootResourceRequest, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.SetRootResourceResponse) => void): grpc.ClientUnaryCall;
    public setRootResource(request: pulumi_engine_pb.SetRootResourceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.SetRootResourceResponse) => void): grpc.ClientUnaryCall;
    public setRootResource(request: pulumi_engine_pb.SetRootResourceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_engine_pb.SetRootResourceResponse) => void): grpc.ClientUnaryCall;
    public startDebugger(request: pulumi_engine_pb.StartDebuggerRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public startDebugger(request: pulumi_engine_pb.StartDebuggerRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public startDebugger(request: pulumi_engine_pb.StartDebuggerRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
}
