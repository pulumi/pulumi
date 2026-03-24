// package: pulumirpc
// file: pulumi/workflow.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_workflow_pb from "./workflow_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

interface IWorkflowEvaluatorService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    handshake: IWorkflowEvaluatorService_IHandshake;
    getPackageInfo: IWorkflowEvaluatorService_IGetPackageInfo;
    getGraphs: IWorkflowEvaluatorService_IGetGraphs;
    getGraph: IWorkflowEvaluatorService_IGetGraph;
    getJobs: IWorkflowEvaluatorService_IGetJobs;
    getJob: IWorkflowEvaluatorService_IGetJob;
    generateJob: IWorkflowEvaluatorService_IGenerateJob;
    generateGraph: IWorkflowEvaluatorService_IGenerateGraph;
    runSensor: IWorkflowEvaluatorService_IRunSensor;
    runStep: IWorkflowEvaluatorService_IRunStep;
    runFilter: IWorkflowEvaluatorService_IRunFilter;
    runOnError: IWorkflowEvaluatorService_IRunOnError;
}

interface IWorkflowEvaluatorService_IHandshake extends grpc.MethodDefinition<pulumi_workflow_pb.WorkflowHandshakeRequest, pulumi_workflow_pb.WorkflowHandshakeResponse> {
    path: "/pulumirpc.WorkflowEvaluator/Handshake";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.WorkflowHandshakeRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.WorkflowHandshakeRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.WorkflowHandshakeResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.WorkflowHandshakeResponse>;
}
interface IWorkflowEvaluatorService_IGetPackageInfo extends grpc.MethodDefinition<pulumi_workflow_pb.GetPackageInfoRequest, pulumi_workflow_pb.GetPackageInfoResponse> {
    path: "/pulumirpc.WorkflowEvaluator/GetPackageInfo";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.GetPackageInfoRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.GetPackageInfoRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.GetPackageInfoResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.GetPackageInfoResponse>;
}
interface IWorkflowEvaluatorService_IGetGraphs extends grpc.MethodDefinition<pulumi_workflow_pb.GetGraphsRequest, pulumi_workflow_pb.GetGraphsResponse> {
    path: "/pulumirpc.WorkflowEvaluator/GetGraphs";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.GetGraphsRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.GetGraphsRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.GetGraphsResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.GetGraphsResponse>;
}
interface IWorkflowEvaluatorService_IGetGraph extends grpc.MethodDefinition<pulumi_workflow_pb.GetGraphRequest, pulumi_workflow_pb.GetGraphResponse> {
    path: "/pulumirpc.WorkflowEvaluator/GetGraph";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.GetGraphRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.GetGraphRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.GetGraphResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.GetGraphResponse>;
}
interface IWorkflowEvaluatorService_IGetJobs extends grpc.MethodDefinition<pulumi_workflow_pb.GetJobsRequest, pulumi_workflow_pb.GetJobsResponse> {
    path: "/pulumirpc.WorkflowEvaluator/GetJobs";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.GetJobsRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.GetJobsRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.GetJobsResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.GetJobsResponse>;
}
interface IWorkflowEvaluatorService_IGetJob extends grpc.MethodDefinition<pulumi_workflow_pb.GetJobRequest, pulumi_workflow_pb.GetJobResponse> {
    path: "/pulumirpc.WorkflowEvaluator/GetJob";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_workflow_pb.GetJobRequest>;
    requestDeserialize: grpc.deserialize<pulumi_workflow_pb.GetJobRequest>;
    responseSerialize: grpc.serialize<pulumi_workflow_pb.GetJobResponse>;
    responseDeserialize: grpc.deserialize<pulumi_workflow_pb.GetJobResponse>;
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
    handshake: grpc.handleUnaryCall<pulumi_workflow_pb.WorkflowHandshakeRequest, pulumi_workflow_pb.WorkflowHandshakeResponse>;
    getPackageInfo: grpc.handleUnaryCall<pulumi_workflow_pb.GetPackageInfoRequest, pulumi_workflow_pb.GetPackageInfoResponse>;
    getGraphs: grpc.handleUnaryCall<pulumi_workflow_pb.GetGraphsRequest, pulumi_workflow_pb.GetGraphsResponse>;
    getGraph: grpc.handleUnaryCall<pulumi_workflow_pb.GetGraphRequest, pulumi_workflow_pb.GetGraphResponse>;
    getJobs: grpc.handleUnaryCall<pulumi_workflow_pb.GetJobsRequest, pulumi_workflow_pb.GetJobsResponse>;
    getJob: grpc.handleUnaryCall<pulumi_workflow_pb.GetJobRequest, pulumi_workflow_pb.GetJobResponse>;
    generateJob: grpc.handleUnaryCall<pulumi_workflow_pb.GenerateJobRequest, pulumi_workflow_pb.GenerateNodeResponse>;
    generateGraph: grpc.handleUnaryCall<pulumi_workflow_pb.GenerateGraphRequest, pulumi_workflow_pb.GenerateNodeResponse>;
    runSensor: grpc.handleUnaryCall<pulumi_workflow_pb.RunSensorRequest, pulumi_workflow_pb.RunSensorResponse>;
    runStep: grpc.handleUnaryCall<pulumi_workflow_pb.RunStepRequest, pulumi_workflow_pb.RunStepResponse>;
    runFilter: grpc.handleUnaryCall<pulumi_workflow_pb.RunFilterRequest, pulumi_workflow_pb.RunFilterResponse>;
    runOnError: grpc.handleUnaryCall<pulumi_workflow_pb.RunOnErrorRequest, pulumi_workflow_pb.RunOnErrorResponse>;
}

export interface IWorkflowEvaluatorClient {
    handshake(request: pulumi_workflow_pb.WorkflowHandshakeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowHandshakeResponse) => void): grpc.ClientUnaryCall;
    handshake(request: pulumi_workflow_pb.WorkflowHandshakeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowHandshakeResponse) => void): grpc.ClientUnaryCall;
    handshake(request: pulumi_workflow_pb.WorkflowHandshakeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowHandshakeResponse) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_workflow_pb.GetPackageInfoRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_workflow_pb.GetPackageInfoRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_workflow_pb.GetPackageInfoRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    getGraphs(request: pulumi_workflow_pb.GetGraphsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    getGraphs(request: pulumi_workflow_pb.GetGraphsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    getGraphs(request: pulumi_workflow_pb.GetGraphsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    getGraph(request: pulumi_workflow_pb.GetGraphRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    getGraph(request: pulumi_workflow_pb.GetGraphRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    getGraph(request: pulumi_workflow_pb.GetGraphRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    getJobs(request: pulumi_workflow_pb.GetJobsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    getJobs(request: pulumi_workflow_pb.GetJobsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    getJobs(request: pulumi_workflow_pb.GetJobsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    getJob(request: pulumi_workflow_pb.GetJobRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
    getJob(request: pulumi_workflow_pb.GetJobRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
    getJob(request: pulumi_workflow_pb.GetJobRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
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
    public handshake(request: pulumi_workflow_pb.WorkflowHandshakeRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowHandshakeResponse) => void): grpc.ClientUnaryCall;
    public handshake(request: pulumi_workflow_pb.WorkflowHandshakeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowHandshakeResponse) => void): grpc.ClientUnaryCall;
    public handshake(request: pulumi_workflow_pb.WorkflowHandshakeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.WorkflowHandshakeResponse) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_workflow_pb.GetPackageInfoRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_workflow_pb.GetPackageInfoRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_workflow_pb.GetPackageInfoRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    public getGraphs(request: pulumi_workflow_pb.GetGraphsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    public getGraphs(request: pulumi_workflow_pb.GetGraphsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    public getGraphs(request: pulumi_workflow_pb.GetGraphsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    public getGraph(request: pulumi_workflow_pb.GetGraphRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    public getGraph(request: pulumi_workflow_pb.GetGraphRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    public getGraph(request: pulumi_workflow_pb.GetGraphRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    public getJobs(request: pulumi_workflow_pb.GetJobsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    public getJobs(request: pulumi_workflow_pb.GetJobsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    public getJobs(request: pulumi_workflow_pb.GetJobsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    public getJob(request: pulumi_workflow_pb.GetJobRequest, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
    public getJob(request: pulumi_workflow_pb.GetJobRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
    public getJob(request: pulumi_workflow_pb.GetJobRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
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
