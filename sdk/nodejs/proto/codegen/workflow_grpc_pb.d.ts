// package: codegen
// file: pulumi/codegen/workflow.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_codegen_workflow_pb from "../codegen/workflow_pb";

interface IWorkflowLoaderService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    getPackageInfo: IWorkflowLoaderService_IGetPackageInfo;
    getGraphs: IWorkflowLoaderService_IGetGraphs;
    getGraph: IWorkflowLoaderService_IGetGraph;
    getTriggers: IWorkflowLoaderService_IGetTriggers;
    getTrigger: IWorkflowLoaderService_IGetTrigger;
    getJobs: IWorkflowLoaderService_IGetJobs;
    getJob: IWorkflowLoaderService_IGetJob;
    getSteps: IWorkflowLoaderService_IGetSteps;
    getStep: IWorkflowLoaderService_IGetStep;
}

interface IWorkflowLoaderService_IGetPackageInfo extends grpc.MethodDefinition<pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest, pulumi_codegen_workflow_pb.GetPackageInfoResponse> {
    path: "/codegen.WorkflowLoader/GetPackageInfo";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetPackageInfoResponse>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetPackageInfoResponse>;
}
interface IWorkflowLoaderService_IGetGraphs extends grpc.MethodDefinition<pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest, pulumi_codegen_workflow_pb.GetGraphsResponse> {
    path: "/codegen.WorkflowLoader/GetGraphs";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetGraphsResponse>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetGraphsResponse>;
}
interface IWorkflowLoaderService_IGetGraph extends grpc.MethodDefinition<pulumi_codegen_workflow_pb.GetWorkflowGraphRequest, pulumi_codegen_workflow_pb.GetGraphResponse> {
    path: "/codegen.WorkflowLoader/GetGraph";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetWorkflowGraphRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetWorkflowGraphRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetGraphResponse>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetGraphResponse>;
}
interface IWorkflowLoaderService_IGetTriggers extends grpc.MethodDefinition<pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest, pulumi_codegen_workflow_pb.GetTriggersResponse> {
    path: "/codegen.WorkflowLoader/GetTriggers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetTriggersResponse>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetTriggersResponse>;
}
interface IWorkflowLoaderService_IGetTrigger extends grpc.MethodDefinition<pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest, pulumi_codegen_workflow_pb.GetTriggerResponse> {
    path: "/codegen.WorkflowLoader/GetTrigger";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetTriggerResponse>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetTriggerResponse>;
}
interface IWorkflowLoaderService_IGetJobs extends grpc.MethodDefinition<pulumi_codegen_workflow_pb.GetWorkflowJobsRequest, pulumi_codegen_workflow_pb.GetJobsResponse> {
    path: "/codegen.WorkflowLoader/GetJobs";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetWorkflowJobsRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetWorkflowJobsRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetJobsResponse>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetJobsResponse>;
}
interface IWorkflowLoaderService_IGetJob extends grpc.MethodDefinition<pulumi_codegen_workflow_pb.GetWorkflowJobRequest, pulumi_codegen_workflow_pb.GetJobResponse> {
    path: "/codegen.WorkflowLoader/GetJob";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetWorkflowJobRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetWorkflowJobRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetJobResponse>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetJobResponse>;
}
interface IWorkflowLoaderService_IGetSteps extends grpc.MethodDefinition<pulumi_codegen_workflow_pb.GetWorkflowStepsRequest, pulumi_codegen_workflow_pb.GetStepsResponse> {
    path: "/codegen.WorkflowLoader/GetSteps";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetWorkflowStepsRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetWorkflowStepsRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetStepsResponse>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetStepsResponse>;
}
interface IWorkflowLoaderService_IGetStep extends grpc.MethodDefinition<pulumi_codegen_workflow_pb.GetWorkflowStepRequest, pulumi_codegen_workflow_pb.GetStepResponse> {
    path: "/codegen.WorkflowLoader/GetStep";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetWorkflowStepRequest>;
    requestDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetWorkflowStepRequest>;
    responseSerialize: grpc.serialize<pulumi_codegen_workflow_pb.GetStepResponse>;
    responseDeserialize: grpc.deserialize<pulumi_codegen_workflow_pb.GetStepResponse>;
}

export const WorkflowLoaderService: IWorkflowLoaderService;

export interface IWorkflowLoaderServer extends grpc.UntypedServiceImplementation {
    getPackageInfo: grpc.handleUnaryCall<pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest, pulumi_codegen_workflow_pb.GetPackageInfoResponse>;
    getGraphs: grpc.handleUnaryCall<pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest, pulumi_codegen_workflow_pb.GetGraphsResponse>;
    getGraph: grpc.handleUnaryCall<pulumi_codegen_workflow_pb.GetWorkflowGraphRequest, pulumi_codegen_workflow_pb.GetGraphResponse>;
    getTriggers: grpc.handleUnaryCall<pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest, pulumi_codegen_workflow_pb.GetTriggersResponse>;
    getTrigger: grpc.handleUnaryCall<pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest, pulumi_codegen_workflow_pb.GetTriggerResponse>;
    getJobs: grpc.handleUnaryCall<pulumi_codegen_workflow_pb.GetWorkflowJobsRequest, pulumi_codegen_workflow_pb.GetJobsResponse>;
    getJob: grpc.handleUnaryCall<pulumi_codegen_workflow_pb.GetWorkflowJobRequest, pulumi_codegen_workflow_pb.GetJobResponse>;
    getSteps: grpc.handleUnaryCall<pulumi_codegen_workflow_pb.GetWorkflowStepsRequest, pulumi_codegen_workflow_pb.GetStepsResponse>;
    getStep: grpc.handleUnaryCall<pulumi_codegen_workflow_pb.GetWorkflowStepRequest, pulumi_codegen_workflow_pb.GetStepResponse>;
}

export interface IWorkflowLoaderClient {
    getPackageInfo(request: pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    getPackageInfo(request: pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    getGraphs(request: pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    getGraphs(request: pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    getGraphs(request: pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    getGraph(request: pulumi_codegen_workflow_pb.GetWorkflowGraphRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    getGraph(request: pulumi_codegen_workflow_pb.GetWorkflowGraphRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    getGraph(request: pulumi_codegen_workflow_pb.GetWorkflowGraphRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    getTriggers(request: pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggersResponse) => void): grpc.ClientUnaryCall;
    getTriggers(request: pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggersResponse) => void): grpc.ClientUnaryCall;
    getTriggers(request: pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggersResponse) => void): grpc.ClientUnaryCall;
    getTrigger(request: pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggerResponse) => void): grpc.ClientUnaryCall;
    getTrigger(request: pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggerResponse) => void): grpc.ClientUnaryCall;
    getTrigger(request: pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggerResponse) => void): grpc.ClientUnaryCall;
    getJobs(request: pulumi_codegen_workflow_pb.GetWorkflowJobsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    getJobs(request: pulumi_codegen_workflow_pb.GetWorkflowJobsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    getJobs(request: pulumi_codegen_workflow_pb.GetWorkflowJobsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    getJob(request: pulumi_codegen_workflow_pb.GetWorkflowJobRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
    getJob(request: pulumi_codegen_workflow_pb.GetWorkflowJobRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
    getJob(request: pulumi_codegen_workflow_pb.GetWorkflowJobRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
    getSteps(request: pulumi_codegen_workflow_pb.GetWorkflowStepsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepsResponse) => void): grpc.ClientUnaryCall;
    getSteps(request: pulumi_codegen_workflow_pb.GetWorkflowStepsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepsResponse) => void): grpc.ClientUnaryCall;
    getSteps(request: pulumi_codegen_workflow_pb.GetWorkflowStepsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepsResponse) => void): grpc.ClientUnaryCall;
    getStep(request: pulumi_codegen_workflow_pb.GetWorkflowStepRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepResponse) => void): grpc.ClientUnaryCall;
    getStep(request: pulumi_codegen_workflow_pb.GetWorkflowStepRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepResponse) => void): grpc.ClientUnaryCall;
    getStep(request: pulumi_codegen_workflow_pb.GetWorkflowStepRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepResponse) => void): grpc.ClientUnaryCall;
}

export class WorkflowLoaderClient extends grpc.Client implements IWorkflowLoaderClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public getPackageInfo(request: pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    public getPackageInfo(request: pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetPackageInfoResponse) => void): grpc.ClientUnaryCall;
    public getGraphs(request: pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    public getGraphs(request: pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    public getGraphs(request: pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphsResponse) => void): grpc.ClientUnaryCall;
    public getGraph(request: pulumi_codegen_workflow_pb.GetWorkflowGraphRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    public getGraph(request: pulumi_codegen_workflow_pb.GetWorkflowGraphRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    public getGraph(request: pulumi_codegen_workflow_pb.GetWorkflowGraphRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetGraphResponse) => void): grpc.ClientUnaryCall;
    public getTriggers(request: pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggersResponse) => void): grpc.ClientUnaryCall;
    public getTriggers(request: pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggersResponse) => void): grpc.ClientUnaryCall;
    public getTriggers(request: pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggersResponse) => void): grpc.ClientUnaryCall;
    public getTrigger(request: pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggerResponse) => void): grpc.ClientUnaryCall;
    public getTrigger(request: pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggerResponse) => void): grpc.ClientUnaryCall;
    public getTrigger(request: pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetTriggerResponse) => void): grpc.ClientUnaryCall;
    public getJobs(request: pulumi_codegen_workflow_pb.GetWorkflowJobsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    public getJobs(request: pulumi_codegen_workflow_pb.GetWorkflowJobsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    public getJobs(request: pulumi_codegen_workflow_pb.GetWorkflowJobsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobsResponse) => void): grpc.ClientUnaryCall;
    public getJob(request: pulumi_codegen_workflow_pb.GetWorkflowJobRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
    public getJob(request: pulumi_codegen_workflow_pb.GetWorkflowJobRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
    public getJob(request: pulumi_codegen_workflow_pb.GetWorkflowJobRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetJobResponse) => void): grpc.ClientUnaryCall;
    public getSteps(request: pulumi_codegen_workflow_pb.GetWorkflowStepsRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepsResponse) => void): grpc.ClientUnaryCall;
    public getSteps(request: pulumi_codegen_workflow_pb.GetWorkflowStepsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepsResponse) => void): grpc.ClientUnaryCall;
    public getSteps(request: pulumi_codegen_workflow_pb.GetWorkflowStepsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepsResponse) => void): grpc.ClientUnaryCall;
    public getStep(request: pulumi_codegen_workflow_pb.GetWorkflowStepRequest, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepResponse) => void): grpc.ClientUnaryCall;
    public getStep(request: pulumi_codegen_workflow_pb.GetWorkflowStepRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepResponse) => void): grpc.ClientUnaryCall;
    public getStep(request: pulumi_codegen_workflow_pb.GetWorkflowStepRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: pulumi_codegen_workflow_pb.GetStepResponse) => void): grpc.ClientUnaryCall;
}
