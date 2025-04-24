// package: pulumirpc
// file: pulumi/analyzer.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_analyzer_pb from "./analyzer_pb";
import * as pulumi_plugin_pb from "./plugin_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

interface IAnalyzerService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    analyze: IAnalyzerService_IAnalyze;
    analyzeStack: IAnalyzerService_IAnalyzeStack;
    remediate: IAnalyzerService_IRemediate;
    getAnalyzerInfo: IAnalyzerService_IGetAnalyzerInfo;
    getPluginInfo: IAnalyzerService_IGetPluginInfo;
    configure: IAnalyzerService_IConfigure;
}

interface IAnalyzerService_IAnalyze extends grpc.MethodDefinition<pulumi_analyzer_pb.AnalyzeRequest, pulumi_analyzer_pb.AnalyzeResponse> {
    path: "/pulumirpc.Analyzer/Analyze";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_analyzer_pb.AnalyzeRequest>;
    requestDeserialize: grpc.deserialize<pulumi_analyzer_pb.AnalyzeRequest>;
    responseSerialize: grpc.serialize<pulumi_analyzer_pb.AnalyzeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_analyzer_pb.AnalyzeResponse>;
}
interface IAnalyzerService_IAnalyzeStack extends grpc.MethodDefinition<pulumi_analyzer_pb.AnalyzeStackRequest, pulumi_analyzer_pb.AnalyzeResponse> {
    path: "/pulumirpc.Analyzer/AnalyzeStack";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_analyzer_pb.AnalyzeStackRequest>;
    requestDeserialize: grpc.deserialize<pulumi_analyzer_pb.AnalyzeStackRequest>;
    responseSerialize: grpc.serialize<pulumi_analyzer_pb.AnalyzeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_analyzer_pb.AnalyzeResponse>;
}
interface IAnalyzerService_IRemediate extends grpc.MethodDefinition<pulumi_analyzer_pb.AnalyzeRequest, pulumi_analyzer_pb.RemediateResponse> {
    path: "/pulumirpc.Analyzer/Remediate";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_analyzer_pb.AnalyzeRequest>;
    requestDeserialize: grpc.deserialize<pulumi_analyzer_pb.AnalyzeRequest>;
    responseSerialize: grpc.serialize<pulumi_analyzer_pb.RemediateResponse>;
    responseDeserialize: grpc.deserialize<pulumi_analyzer_pb.RemediateResponse>;
}
interface IAnalyzerService_IGetAnalyzerInfo extends grpc.MethodDefinition<google_protobuf_empty_pb.Empty, pulumi_analyzer_pb.AnalyzerInfo> {
    path: "/pulumirpc.Analyzer/GetAnalyzerInfo";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    requestDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
    responseSerialize: grpc.serialize<pulumi_analyzer_pb.AnalyzerInfo>;
    responseDeserialize: grpc.deserialize<pulumi_analyzer_pb.AnalyzerInfo>;
}
interface IAnalyzerService_IGetPluginInfo extends grpc.MethodDefinition<google_protobuf_empty_pb.Empty, pulumi_plugin_pb.PluginInfo> {
    path: "/pulumirpc.Analyzer/GetPluginInfo";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    requestDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
    responseSerialize: grpc.serialize<pulumi_plugin_pb.PluginInfo>;
    responseDeserialize: grpc.deserialize<pulumi_plugin_pb.PluginInfo>;
}
interface IAnalyzerService_IConfigure extends grpc.MethodDefinition<pulumi_analyzer_pb.ConfigureAnalyzerRequest, google_protobuf_empty_pb.Empty> {
    path: "/pulumirpc.Analyzer/Configure";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_analyzer_pb.ConfigureAnalyzerRequest>;
    requestDeserialize: grpc.deserialize<pulumi_analyzer_pb.ConfigureAnalyzerRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}

export const AnalyzerService: IAnalyzerService;

export interface IAnalyzerServer extends grpc.UntypedServiceImplementation {
    analyze: grpc.handleUnaryCall<pulumi_analyzer_pb.AnalyzeRequest, pulumi_analyzer_pb.AnalyzeResponse>;
    analyzeStack: grpc.handleUnaryCall<pulumi_analyzer_pb.AnalyzeStackRequest, pulumi_analyzer_pb.AnalyzeResponse>;
    remediate: grpc.handleUnaryCall<pulumi_analyzer_pb.AnalyzeRequest, pulumi_analyzer_pb.RemediateResponse>;
    getAnalyzerInfo: grpc.handleUnaryCall<google_protobuf_empty_pb.Empty, pulumi_analyzer_pb.AnalyzerInfo>;
    getPluginInfo: grpc.handleUnaryCall<google_protobuf_empty_pb.Empty, pulumi_plugin_pb.PluginInfo>;
    configure: grpc.handleUnaryCall<pulumi_analyzer_pb.ConfigureAnalyzerRequest, google_protobuf_empty_pb.Empty>;
}

export interface IAnalyzerClient {
    analyze(request: pulumi_analyzer_pb.AnalyzeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    analyze(request: pulumi_analyzer_pb.AnalyzeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    analyze(request: pulumi_analyzer_pb.AnalyzeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    analyzeStack(request: pulumi_analyzer_pb.AnalyzeStackRequest, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    analyzeStack(request: pulumi_analyzer_pb.AnalyzeStackRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    analyzeStack(request: pulumi_analyzer_pb.AnalyzeStackRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    remediate(request: pulumi_analyzer_pb.AnalyzeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.RemediateResponse) => void): grpc.ClientUnaryCall;
    remediate(request: pulumi_analyzer_pb.AnalyzeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.RemediateResponse) => void): grpc.ClientUnaryCall;
    remediate(request: pulumi_analyzer_pb.AnalyzeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.RemediateResponse) => void): grpc.ClientUnaryCall;
    getAnalyzerInfo(request: google_protobuf_empty_pb.Empty, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzerInfo) => void): grpc.ClientUnaryCall;
    getAnalyzerInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzerInfo) => void): grpc.ClientUnaryCall;
    getAnalyzerInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzerInfo) => void): grpc.ClientUnaryCall;
    getPluginInfo(request: google_protobuf_empty_pb.Empty, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    configure(request: pulumi_analyzer_pb.ConfigureAnalyzerRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    configure(request: pulumi_analyzer_pb.ConfigureAnalyzerRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    configure(request: pulumi_analyzer_pb.ConfigureAnalyzerRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
}

export class AnalyzerClient extends grpc.Client implements IAnalyzerClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public analyze(request: pulumi_analyzer_pb.AnalyzeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    public analyze(request: pulumi_analyzer_pb.AnalyzeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    public analyze(request: pulumi_analyzer_pb.AnalyzeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    public analyzeStack(request: pulumi_analyzer_pb.AnalyzeStackRequest, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    public analyzeStack(request: pulumi_analyzer_pb.AnalyzeStackRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    public analyzeStack(request: pulumi_analyzer_pb.AnalyzeStackRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzeResponse) => void): grpc.ClientUnaryCall;
    public remediate(request: pulumi_analyzer_pb.AnalyzeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.RemediateResponse) => void): grpc.ClientUnaryCall;
    public remediate(request: pulumi_analyzer_pb.AnalyzeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.RemediateResponse) => void): grpc.ClientUnaryCall;
    public remediate(request: pulumi_analyzer_pb.AnalyzeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.RemediateResponse) => void): grpc.ClientUnaryCall;
    public getAnalyzerInfo(request: google_protobuf_empty_pb.Empty, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzerInfo) => void): grpc.ClientUnaryCall;
    public getAnalyzerInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzerInfo) => void): grpc.ClientUnaryCall;
    public getAnalyzerInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_analyzer_pb.AnalyzerInfo) => void): grpc.ClientUnaryCall;
    public getPluginInfo(request: google_protobuf_empty_pb.Empty, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    public getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    public getPluginInfo(request: google_protobuf_empty_pb.Empty, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_plugin_pb.PluginInfo) => void): grpc.ClientUnaryCall;
    public configure(request: pulumi_analyzer_pb.ConfigureAnalyzerRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public configure(request: pulumi_analyzer_pb.ConfigureAnalyzerRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public configure(request: pulumi_analyzer_pb.ConfigureAnalyzerRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
}
