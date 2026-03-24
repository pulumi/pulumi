// package: pulumirpc
// file: pulumi/workflow.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_workflow_pb from "./workflow_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

interface IWorkflowEvaluatorService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    generateJob: IWorkflowEvaluatorService_IGenerateJob;
    generateGraph: IWorkflowEvaluatorService_IGenerateGraph;
    runSensor: IWorkflowEvaluatorService_IRunSensor;
    runStep: IWorkflowEvaluatorService_IRunStep;
    runFilter: IWorkflowEvaluatorService_IRunFilter;
    runOnError: IWorkflowEvaluatorService_IRunOnError;
}

interface IWorkflowEvaluatorService_IGenerateJob extends grpc.MethodDefinition<pulumi_workflow_pb.GenerateJobRequest, pulumi_workflow_pb.GenerateNodeResponse> {
    path: "/pulumirpc.WorkflowEvaluator/GenerateJob";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.GenerateJobRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.GenerateJobRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.GenerateNodeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.GenerateNodeResponse>;
}
interface IWorkflowEvaluatorService_IGenerateGraph extends grpc.MethodDefinition<pulumi_workflow_pb.GenerateGraphRequest, pulumi_workflow_pb.GenerateNodeResponse> {
    path: "/pulumirpc.WorkflowEvaluator/GenerateGraph";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.GenerateGraphRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.GenerateGraphRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.GenerateNodeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.GenerateNodeResponse>;
}
interface IWorkflowEvaluatorService_IRunSensor extends grpc.MethodDefinition<pulumi_workflow_pb.RunSensorRequest, pulumi_workflow_pb.RunSensorResponse> {
    path: "/pulumirpc.WorkflowEvaluator/RunSensor";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.RunSensorRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.RunSensorRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.RunSensorResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.RunSensorResponse>;
}
interface IWorkflowEvaluatorService_IRunStep extends grpc.MethodDefinition<pulumi_workflow_pb.RunStepRequest, pulumi_workflow_pb.RunStepResponse> {
    path: "/pulumirpc.WorkflowEvaluator/RunStep";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.RunStepRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.RunStepRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.RunStepResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.RunStepResponse>;
}
interface IWorkflowEvaluatorService_IRunFilter extends grpc.MethodDefinition<pulumi_workflow_pb.RunFilterRequest, pulumi_workflow_pb.RunFilterResponse> {
    path: "/pulumirpc.WorkflowEvaluator/RunFilter";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.RunFilterRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.RunFilterRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.RunFilterResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.RunFilterResponse>;
}
interface IWorkflowEvaluatorService_IRunOnError extends grpc.MethodDefinition<pulumi_workflow_pb.RunOnErrorRequest, pulumi_workflow_pb.RunOnErrorResponse> {
    path: "/pulumirpc.WorkflowEvaluator/RunOnError";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.RunOnErrorRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.RunOnErrorRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.RunOnErrorResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.RunOnErrorResponse>;
}

export const WorkflowEvaluatorService: IWorkflowEvaluatorService;

export interface IWorkflowEvaluatorServer extends grpc.UntypedServiceImplementation {
    generateJob: grpc.handleUnaryCall<pulumi_workflow_pb.GenerateJobRequest, pulumi_workflow_pb.GenerateNodeResponse>;
    generateGraph: grpc.handleUnaryCall<pulumi_workflow_pb.GenerateGraphRequest, pulumi_workflow_pb.GenerateNodeResponse>;
    runSensor: grpc.handleUnaryCall<pulumi_workflow_pb.RunSensorRequest, pulumi_workflow_pb.RunSensorResponse>;
    runStep: grpc.handleUnaryCall<pulumi_workflow_pb.RunStepRequest, pulumi_workflow_pb.RunStepResponse>;
    runFilter: grpc.handleUnaryCall<pulumi_workflow_pb.RunFilterRequest, pulumi_workflow_pb.RunFilterResponse>;
    runOnError: grpc.handleUnaryCall<pulumi_workflow_pb.RunOnErrorRequest, pulumi_workflow_pb.RunOnErrorResponse>;
}

export interface IWorkflowEvaluatorClient {
    generateJob(request: pulumi_workflow_pb.GenerateJobRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    generateJob(request: pulumi_workflow_pb.GenerateJobRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    generateJob(request: pulumi_workflow_pb.GenerateJobRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    generateGraph(request: pulumi_workflow_pb.GenerateGraphRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    generateGraph(request: pulumi_workflow_pb.GenerateGraphRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    generateGraph(request: pulumi_workflow_pb.GenerateGraphRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    runSensor(request: pulumi_workflow_pb.RunSensorRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunSensorResponse) => void): grpc.ClientUnaryCall;
    runSensor(request: pulumi_workflow_pb.RunSensorRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunSensorResponse) => void): grpc.ClientUnaryCall;
    runSensor(request: pulumi_workflow_pb.RunSensorRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunSensorResponse) => void): grpc.ClientUnaryCall;
    runStep(request: pulumi_workflow_pb.RunStepRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunStepResponse) => void): grpc.ClientUnaryCall;
    runStep(request: pulumi_workflow_pb.RunStepRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunStepResponse) => void): grpc.ClientUnaryCall;
    runStep(request: pulumi_workflow_pb.RunStepRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunStepResponse) => void): grpc.ClientUnaryCall;
    runFilter(request: pulumi_workflow_pb.RunFilterRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunFilterResponse) => void): grpc.ClientUnaryCall;
    runFilter(request: pulumi_workflow_pb.RunFilterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunFilterResponse) => void): grpc.ClientUnaryCall;
    runFilter(request: pulumi_workflow_pb.RunFilterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunFilterResponse) => void): grpc.ClientUnaryCall;
    runOnError(request: pulumi_workflow_pb.RunOnErrorRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunOnErrorResponse) => void): grpc.ClientUnaryCall;
    runOnError(request: pulumi_workflow_pb.RunOnErrorRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunOnErrorResponse) => void): grpc.ClientUnaryCall;
    runOnError(request: pulumi_workflow_pb.RunOnErrorRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunOnErrorResponse) => void): grpc.ClientUnaryCall;
}

export class WorkflowEvaluatorClient extends grpc.Client implements IWorkflowEvaluatorClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public generateJob(request: pulumi_workflow_pb.GenerateJobRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    public generateJob(request: pulumi_workflow_pb.GenerateJobRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    public generateJob(request: pulumi_workflow_pb.GenerateJobRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    public generateGraph(request: pulumi_workflow_pb.GenerateGraphRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    public generateGraph(request: pulumi_workflow_pb.GenerateGraphRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    public generateGraph(request: pulumi_workflow_pb.GenerateGraphRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GenerateNodeResponse) => void): grpc.ClientUnaryCall;
    public runSensor(request: pulumi_workflow_pb.RunSensorRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunSensorResponse) => void): grpc.ClientUnaryCall;
    public runSensor(request: pulumi_workflow_pb.RunSensorRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunSensorResponse) => void): grpc.ClientUnaryCall;
    public runSensor(request: pulumi_workflow_pb.RunSensorRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunSensorResponse) => void): grpc.ClientUnaryCall;
    public runStep(request: pulumi_workflow_pb.RunStepRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunStepResponse) => void): grpc.ClientUnaryCall;
    public runStep(request: pulumi_workflow_pb.RunStepRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunStepResponse) => void): grpc.ClientUnaryCall;
    public runStep(request: pulumi_workflow_pb.RunStepRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunStepResponse) => void): grpc.ClientUnaryCall;
    public runFilter(request: pulumi_workflow_pb.RunFilterRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunFilterResponse) => void): grpc.ClientUnaryCall;
    public runFilter(request: pulumi_workflow_pb.RunFilterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunFilterResponse) => void): grpc.ClientUnaryCall;
    public runFilter(request: pulumi_workflow_pb.RunFilterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunFilterResponse) => void): grpc.ClientUnaryCall;
    public runOnError(request: pulumi_workflow_pb.RunOnErrorRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunOnErrorResponse) => void): grpc.ClientUnaryCall;
    public runOnError(request: pulumi_workflow_pb.RunOnErrorRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunOnErrorResponse) => void): grpc.ClientUnaryCall;
    public runOnError(request: pulumi_workflow_pb.RunOnErrorRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RunOnErrorResponse) => void): grpc.ClientUnaryCall;
}

interface IWorkflowRegistryService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    handshake: IWorkflowRegistryService_IHandshake;
    registerComponent: IWorkflowRegistryService_IRegisterComponent;
}

interface IWorkflowRegistryService_IHandshake extends grpc.MethodDefinition<pulumi_workflow_pb.WorkflowRegistryHandshakeRequest, pulumi_workflow_pb.WorkflowRegistryHandshakeResponse> {
    path: "/pulumirpc.WorkflowRegistry/Handshake";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.WorkflowRegistryHandshakeRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.WorkflowRegistryHandshakeRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.WorkflowRegistryHandshakeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.WorkflowRegistryHandshakeResponse>;
}
interface IWorkflowRegistryService_IRegisterComponent extends grpc.MethodDefinition<pulumi_workflow_pb.RegisterComponentRequest, google_protobuf_empty_pb.Empty> {
    path: "/pulumirpc.WorkflowRegistry/RegisterComponent";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.RegisterComponentRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.RegisterComponentRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}

export const WorkflowRegistryService: IWorkflowRegistryService;

export interface IWorkflowRegistryServer extends grpc.UntypedServiceImplementation {
    handshake: grpc.handleUnaryCall<pulumi_workflow_pb.WorkflowRegistryHandshakeRequest, pulumi_workflow_pb.WorkflowRegistryHandshakeResponse>;
    registerComponent: grpc.handleUnaryCall<pulumi_workflow_pb.RegisterComponentRequest, google_protobuf_empty_pb.Empty>;
}

export interface IWorkflowRegistryClient {
    handshake(request: pulumi_workflow_pb.WorkflowRegistryHandshakeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowRegistryHandshakeResponse) => void): grpc.ClientUnaryCall;
    handshake(request: pulumi_workflow_pb.WorkflowRegistryHandshakeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowRegistryHandshakeResponse) => void): grpc.ClientUnaryCall;
    handshake(request: pulumi_workflow_pb.WorkflowRegistryHandshakeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowRegistryHandshakeResponse) => void): grpc.ClientUnaryCall;
    registerComponent(request: pulumi_workflow_pb.RegisterComponentRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    registerComponent(request: pulumi_workflow_pb.RegisterComponentRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    registerComponent(request: pulumi_workflow_pb.RegisterComponentRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
}

export class WorkflowRegistryClient extends grpc.Client implements IWorkflowRegistryClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public handshake(request: pulumi_workflow_pb.WorkflowRegistryHandshakeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowRegistryHandshakeResponse) => void): grpc.ClientUnaryCall;
    public handshake(request: pulumi_workflow_pb.WorkflowRegistryHandshakeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowRegistryHandshakeResponse) => void): grpc.ClientUnaryCall;
    public handshake(request: pulumi_workflow_pb.WorkflowRegistryHandshakeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowRegistryHandshakeResponse) => void): grpc.ClientUnaryCall;
    public registerComponent(request: pulumi_workflow_pb.RegisterComponentRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public registerComponent(request: pulumi_workflow_pb.RegisterComponentRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public registerComponent(request: pulumi_workflow_pb.RegisterComponentRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
}

interface IGraphMonitorService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    registerTrigger: IGraphMonitorService_IRegisterTrigger;
    registerSensor: IGraphMonitorService_IRegisterSensor;
    registerJob: IGraphMonitorService_IRegisterJob;
    registerGraph: IGraphMonitorService_IRegisterGraph;
    registerStep: IGraphMonitorService_IRegisterStep;
    getStepResult: IGraphMonitorService_IGetStepResult;
}

interface IGraphMonitorService_IRegisterTrigger extends grpc.MethodDefinition<pulumi_workflow_pb.RegisterTriggerRequest, pulumi_workflow_pb.RegisterNodeResponse> {
    path: "/pulumirpc.GraphMonitor/RegisterTrigger";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.RegisterTriggerRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.RegisterTriggerRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.RegisterNodeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.RegisterNodeResponse>;
}
interface IGraphMonitorService_IRegisterSensor extends grpc.MethodDefinition<pulumi_workflow_pb.RegisterSensorRequest, pulumi_workflow_pb.RegisterNodeResponse> {
    path: "/pulumirpc.GraphMonitor/RegisterSensor";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.RegisterSensorRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.RegisterSensorRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.RegisterNodeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.RegisterNodeResponse>;
}
interface IGraphMonitorService_IRegisterJob extends grpc.MethodDefinition<pulumi_workflow_pb.RegisterJobRequest, pulumi_workflow_pb.RegisterNodeResponse> {
    path: "/pulumirpc.GraphMonitor/RegisterJob";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.RegisterJobRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.RegisterJobRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.RegisterNodeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.RegisterNodeResponse>;
}
interface IGraphMonitorService_IRegisterGraph extends grpc.MethodDefinition<pulumi_workflow_pb.RegisterGraphRequest, pulumi_workflow_pb.RegisterNodeResponse> {
    path: "/pulumirpc.GraphMonitor/RegisterGraph";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.RegisterGraphRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.RegisterGraphRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.RegisterNodeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.RegisterNodeResponse>;
}
interface IGraphMonitorService_IRegisterStep extends grpc.MethodDefinition<pulumi_workflow_pb.RegisterStepRequest, pulumi_workflow_pb.RegisterNodeResponse> {
    path: "/pulumirpc.GraphMonitor/RegisterStep";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.RegisterStepRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.RegisterStepRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.RegisterNodeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.RegisterNodeResponse>;
}
interface IGraphMonitorService_IGetStepResult extends grpc.MethodDefinition<pulumi_workflow_pb.GetStepResultRequest, pulumi_workflow_pb.GetStepResultResponse> {
    path: "/pulumirpc.GraphMonitor/GetStepResult";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.GetStepResultRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.GetStepResultRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.GetStepResultResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.GetStepResultResponse>;
}

export const GraphMonitorService: IGraphMonitorService;

export interface IGraphMonitorServer extends grpc.UntypedServiceImplementation {
    registerTrigger: grpc.handleUnaryCall<pulumi_workflow_pb.RegisterTriggerRequest, pulumi_workflow_pb.RegisterNodeResponse>;
    registerSensor: grpc.handleUnaryCall<pulumi_workflow_pb.RegisterSensorRequest, pulumi_workflow_pb.RegisterNodeResponse>;
    registerJob: grpc.handleUnaryCall<pulumi_workflow_pb.RegisterJobRequest, pulumi_workflow_pb.RegisterNodeResponse>;
    registerGraph: grpc.handleUnaryCall<pulumi_workflow_pb.RegisterGraphRequest, pulumi_workflow_pb.RegisterNodeResponse>;
    registerStep: grpc.handleUnaryCall<pulumi_workflow_pb.RegisterStepRequest, pulumi_workflow_pb.RegisterNodeResponse>;
    getStepResult: grpc.handleUnaryCall<pulumi_workflow_pb.GetStepResultRequest, pulumi_workflow_pb.GetStepResultResponse>;
}

export interface IGraphMonitorClient {
    registerTrigger(request: pulumi_workflow_pb.RegisterTriggerRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerTrigger(request: pulumi_workflow_pb.RegisterTriggerRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerTrigger(request: pulumi_workflow_pb.RegisterTriggerRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerSensor(request: pulumi_workflow_pb.RegisterSensorRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerSensor(request: pulumi_workflow_pb.RegisterSensorRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerSensor(request: pulumi_workflow_pb.RegisterSensorRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerJob(request: pulumi_workflow_pb.RegisterJobRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerJob(request: pulumi_workflow_pb.RegisterJobRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerJob(request: pulumi_workflow_pb.RegisterJobRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerGraph(request: pulumi_workflow_pb.RegisterGraphRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerGraph(request: pulumi_workflow_pb.RegisterGraphRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerGraph(request: pulumi_workflow_pb.RegisterGraphRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerStep(request: pulumi_workflow_pb.RegisterStepRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerStep(request: pulumi_workflow_pb.RegisterStepRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    registerStep(request: pulumi_workflow_pb.RegisterStepRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    getStepResult(request: pulumi_workflow_pb.GetStepResultRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetStepResultResponse) => void): grpc.ClientUnaryCall;
    getStepResult(request: pulumi_workflow_pb.GetStepResultRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetStepResultResponse) => void): grpc.ClientUnaryCall;
    getStepResult(request: pulumi_workflow_pb.GetStepResultRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetStepResultResponse) => void): grpc.ClientUnaryCall;
}

export class GraphMonitorClient extends grpc.Client implements IGraphMonitorClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public registerTrigger(request: pulumi_workflow_pb.RegisterTriggerRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerTrigger(request: pulumi_workflow_pb.RegisterTriggerRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerTrigger(request: pulumi_workflow_pb.RegisterTriggerRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerSensor(request: pulumi_workflow_pb.RegisterSensorRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerSensor(request: pulumi_workflow_pb.RegisterSensorRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerSensor(request: pulumi_workflow_pb.RegisterSensorRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerJob(request: pulumi_workflow_pb.RegisterJobRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerJob(request: pulumi_workflow_pb.RegisterJobRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerJob(request: pulumi_workflow_pb.RegisterJobRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerGraph(request: pulumi_workflow_pb.RegisterGraphRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerGraph(request: pulumi_workflow_pb.RegisterGraphRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerGraph(request: pulumi_workflow_pb.RegisterGraphRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerStep(request: pulumi_workflow_pb.RegisterStepRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerStep(request: pulumi_workflow_pb.RegisterStepRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public registerStep(request: pulumi_workflow_pb.RegisterStepRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.RegisterNodeResponse) => void): grpc.ClientUnaryCall;
    public getStepResult(request: pulumi_workflow_pb.GetStepResultRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetStepResultResponse) => void): grpc.ClientUnaryCall;
    public getStepResult(request: pulumi_workflow_pb.GetStepResultRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetStepResultResponse) => void): grpc.ClientUnaryCall;
    public getStepResult(request: pulumi_workflow_pb.GetStepResultRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetStepResultResponse) => void): grpc.ClientUnaryCall;
}
