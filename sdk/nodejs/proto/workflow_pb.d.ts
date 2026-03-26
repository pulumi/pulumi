// package: pulumirpc
// file: pulumi/workflow.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

export class WorkflowContext extends jspb.Message { 
    getExecutionId(): string;
    setExecutionId(value: string): WorkflowContext;
    getWorkflowVersion(): string;
    setWorkflowVersion(value: string): WorkflowContext;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowContext.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowContext): WorkflowContext.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowContext, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowContext;
    static deserializeBinaryFromReader(message: WorkflowContext, reader: jspb.BinaryReader): WorkflowContext;
}

export namespace WorkflowContext {
    export type AsObject = {
        executionId: string,
        workflowVersion: string,
    }
}

export class WorkflowHandshakeRequest extends jspb.Message { 
    getEngineAddress(): string;
    setEngineAddress(value: string): WorkflowHandshakeRequest;

    hasRootDirectory(): boolean;
    clearRootDirectory(): void;
    getRootDirectory(): string | undefined;
    setRootDirectory(value: string): WorkflowHandshakeRequest;

    hasProgramDirectory(): boolean;
    clearProgramDirectory(): void;
    getProgramDirectory(): string | undefined;
    setProgramDirectory(value: string): WorkflowHandshakeRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowHandshakeRequest.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowHandshakeRequest): WorkflowHandshakeRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowHandshakeRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowHandshakeRequest;
    static deserializeBinaryFromReader(message: WorkflowHandshakeRequest, reader: jspb.BinaryReader): WorkflowHandshakeRequest;
}

export namespace WorkflowHandshakeRequest {
    export type AsObject = {
        engineAddress: string,
        rootDirectory?: string,
        programDirectory?: string,
    }
}

export class WorkflowHandshakeResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowHandshakeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowHandshakeResponse): WorkflowHandshakeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowHandshakeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowHandshakeResponse;
    static deserializeBinaryFromReader(message: WorkflowHandshakeResponse, reader: jspb.BinaryReader): WorkflowHandshakeResponse;
}

export namespace WorkflowHandshakeResponse {
    export type AsObject = {
    }
}

export class TypeReference extends jspb.Message { 
    getToken(): string;
    setToken(value: string): TypeReference;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TypeReference.AsObject;
    static toObject(includeInstance: boolean, msg: TypeReference): TypeReference.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TypeReference, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TypeReference;
    static deserializeBinaryFromReader(message: TypeReference, reader: jspb.BinaryReader): TypeReference;
}

export namespace TypeReference {
    export type AsObject = {
        token: string,
    }
}

export class PackageInfo extends jspb.Message { 
    getName(): string;
    setName(value: string): PackageInfo;
    getVersion(): string;
    setVersion(value: string): PackageInfo;
    getDisplayName(): string;
    setDisplayName(value: string): PackageInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PackageInfo.AsObject;
    static toObject(includeInstance: boolean, msg: PackageInfo): PackageInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PackageInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PackageInfo;
    static deserializeBinaryFromReader(message: PackageInfo, reader: jspb.BinaryReader): PackageInfo;
}

export namespace PackageInfo {
    export type AsObject = {
        name: string,
        version: string,
        displayName: string,
    }
}

export class GraphInfo extends jspb.Message { 
    getToken(): string;
    setToken(value: string): GraphInfo;

    hasInputType(): boolean;
    clearInputType(): void;
    getInputType(): TypeReference | undefined;
    setInputType(value?: TypeReference): GraphInfo;

    hasOutputType(): boolean;
    clearOutputType(): void;
    getOutputType(): TypeReference | undefined;
    setOutputType(value?: TypeReference): GraphInfo;
    getHasOnError(): boolean;
    setHasOnError(value: boolean): GraphInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GraphInfo.AsObject;
    static toObject(includeInstance: boolean, msg: GraphInfo): GraphInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GraphInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GraphInfo;
    static deserializeBinaryFromReader(message: GraphInfo, reader: jspb.BinaryReader): GraphInfo;
}

export namespace GraphInfo {
    export type AsObject = {
        token: string,
        inputType?: TypeReference.AsObject,
        outputType?: TypeReference.AsObject,
        hasOnError: boolean,
    }
}

export class JobInfo extends jspb.Message { 
    getToken(): string;
    setToken(value: string): JobInfo;

    hasInputType(): boolean;
    clearInputType(): void;
    getInputType(): TypeReference | undefined;
    setInputType(value?: TypeReference): JobInfo;

    hasOutputType(): boolean;
    clearOutputType(): void;
    getOutputType(): TypeReference | undefined;
    setOutputType(value?: TypeReference): JobInfo;
    getHasOnError(): boolean;
    setHasOnError(value: boolean): JobInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): JobInfo.AsObject;
    static toObject(includeInstance: boolean, msg: JobInfo): JobInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: JobInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): JobInfo;
    static deserializeBinaryFromReader(message: JobInfo, reader: jspb.BinaryReader): JobInfo;
}

export namespace JobInfo {
    export type AsObject = {
        token: string,
        inputType?: TypeReference.AsObject,
        outputType?: TypeReference.AsObject,
        hasOnError: boolean,
    }
}

export class GetPackageInfoRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetPackageInfoRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetPackageInfoRequest): GetPackageInfoRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetPackageInfoRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetPackageInfoRequest;
    static deserializeBinaryFromReader(message: GetPackageInfoRequest, reader: jspb.BinaryReader): GetPackageInfoRequest;
}

export namespace GetPackageInfoRequest {
    export type AsObject = {
    }
}

export class GetPackageInfoResponse extends jspb.Message { 

    hasPackage(): boolean;
    clearPackage(): void;
    getPackage(): PackageInfo | undefined;
    setPackage(value?: PackageInfo): GetPackageInfoResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetPackageInfoResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetPackageInfoResponse): GetPackageInfoResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetPackageInfoResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetPackageInfoResponse;
    static deserializeBinaryFromReader(message: GetPackageInfoResponse, reader: jspb.BinaryReader): GetPackageInfoResponse;
}

export namespace GetPackageInfoResponse {
    export type AsObject = {
        pb_package?: PackageInfo.AsObject,
    }
}

export class GetGraphsRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetGraphsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetGraphsRequest): GetGraphsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetGraphsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetGraphsRequest;
    static deserializeBinaryFromReader(message: GetGraphsRequest, reader: jspb.BinaryReader): GetGraphsRequest;
}

export namespace GetGraphsRequest {
    export type AsObject = {
    }
}

export class GetGraphsResponse extends jspb.Message { 
    clearGraphsList(): void;
    getGraphsList(): Array<GraphInfo>;
    setGraphsList(value: Array<GraphInfo>): GetGraphsResponse;
    addGraphs(value?: GraphInfo, index?: number): GraphInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetGraphsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetGraphsResponse): GetGraphsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetGraphsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetGraphsResponse;
    static deserializeBinaryFromReader(message: GetGraphsResponse, reader: jspb.BinaryReader): GetGraphsResponse;
}

export namespace GetGraphsResponse {
    export type AsObject = {
        graphsList: Array<GraphInfo.AsObject>,
    }
}

export class GetGraphRequest extends jspb.Message { 
    getToken(): string;
    setToken(value: string): GetGraphRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetGraphRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetGraphRequest): GetGraphRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetGraphRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetGraphRequest;
    static deserializeBinaryFromReader(message: GetGraphRequest, reader: jspb.BinaryReader): GetGraphRequest;
}

export namespace GetGraphRequest {
    export type AsObject = {
        token: string,
    }
}

export class GetGraphResponse extends jspb.Message { 

    hasGraph(): boolean;
    clearGraph(): void;
    getGraph(): GraphInfo | undefined;
    setGraph(value?: GraphInfo): GetGraphResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetGraphResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetGraphResponse): GetGraphResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetGraphResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetGraphResponse;
    static deserializeBinaryFromReader(message: GetGraphResponse, reader: jspb.BinaryReader): GetGraphResponse;
}

export namespace GetGraphResponse {
    export type AsObject = {
        graph?: GraphInfo.AsObject,
    }
}

export class GetTriggersRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetTriggersRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetTriggersRequest): GetTriggersRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetTriggersRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetTriggersRequest;
    static deserializeBinaryFromReader(message: GetTriggersRequest, reader: jspb.BinaryReader): GetTriggersRequest;
}

export namespace GetTriggersRequest {
    export type AsObject = {
    }
}

export class GetTriggersResponse extends jspb.Message { 
    clearTriggersList(): void;
    getTriggersList(): Array<string>;
    setTriggersList(value: Array<string>): GetTriggersResponse;
    addTriggers(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetTriggersResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetTriggersResponse): GetTriggersResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetTriggersResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetTriggersResponse;
    static deserializeBinaryFromReader(message: GetTriggersResponse, reader: jspb.BinaryReader): GetTriggersResponse;
}

export namespace GetTriggersResponse {
    export type AsObject = {
        triggersList: Array<string>,
    }
}

export class GetTriggerRequest extends jspb.Message { 
    getToken(): string;
    setToken(value: string): GetTriggerRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetTriggerRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetTriggerRequest): GetTriggerRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetTriggerRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetTriggerRequest;
    static deserializeBinaryFromReader(message: GetTriggerRequest, reader: jspb.BinaryReader): GetTriggerRequest;
}

export namespace GetTriggerRequest {
    export type AsObject = {
        token: string,
    }
}

export class GetTriggerResponse extends jspb.Message { 

    hasInputType(): boolean;
    clearInputType(): void;
    getInputType(): TypeReference | undefined;
    setInputType(value?: TypeReference): GetTriggerResponse;

    hasOutputType(): boolean;
    clearOutputType(): void;
    getOutputType(): TypeReference | undefined;
    setOutputType(value?: TypeReference): GetTriggerResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetTriggerResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetTriggerResponse): GetTriggerResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetTriggerResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetTriggerResponse;
    static deserializeBinaryFromReader(message: GetTriggerResponse, reader: jspb.BinaryReader): GetTriggerResponse;
}

export namespace GetTriggerResponse {
    export type AsObject = {
        inputType?: TypeReference.AsObject,
        outputType?: TypeReference.AsObject,
    }
}

export class GetJobsRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetJobsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetJobsRequest): GetJobsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetJobsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetJobsRequest;
    static deserializeBinaryFromReader(message: GetJobsRequest, reader: jspb.BinaryReader): GetJobsRequest;
}

export namespace GetJobsRequest {
    export type AsObject = {
    }
}

export class GetJobsResponse extends jspb.Message { 
    clearJobsList(): void;
    getJobsList(): Array<JobInfo>;
    setJobsList(value: Array<JobInfo>): GetJobsResponse;
    addJobs(value?: JobInfo, index?: number): JobInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetJobsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetJobsResponse): GetJobsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetJobsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetJobsResponse;
    static deserializeBinaryFromReader(message: GetJobsResponse, reader: jspb.BinaryReader): GetJobsResponse;
}

export namespace GetJobsResponse {
    export type AsObject = {
        jobsList: Array<JobInfo.AsObject>,
    }
}

export class GetJobRequest extends jspb.Message { 
    getToken(): string;
    setToken(value: string): GetJobRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetJobRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetJobRequest): GetJobRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetJobRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetJobRequest;
    static deserializeBinaryFromReader(message: GetJobRequest, reader: jspb.BinaryReader): GetJobRequest;
}

export namespace GetJobRequest {
    export type AsObject = {
        token: string,
    }
}

export class GetJobResponse extends jspb.Message { 

    hasJob(): boolean;
    clearJob(): void;
    getJob(): JobInfo | undefined;
    setJob(value?: JobInfo): GetJobResponse;
    clearInputPropertiesList(): void;
    getInputPropertiesList(): Array<InputProperty>;
    setInputPropertiesList(value: Array<InputProperty>): GetJobResponse;
    addInputProperties(value?: InputProperty, index?: number): InputProperty;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetJobResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetJobResponse): GetJobResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetJobResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetJobResponse;
    static deserializeBinaryFromReader(message: GetJobResponse, reader: jspb.BinaryReader): GetJobResponse;
}

export namespace GetJobResponse {
    export type AsObject = {
        job?: JobInfo.AsObject,
        inputPropertiesList: Array<InputProperty.AsObject>,
    }
}

export class GetStepsRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetStepsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetStepsRequest): GetStepsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetStepsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetStepsRequest;
    static deserializeBinaryFromReader(message: GetStepsRequest, reader: jspb.BinaryReader): GetStepsRequest;
}

export namespace GetStepsRequest {
    export type AsObject = {
    }
}

export class GetStepsResponse extends jspb.Message { 
    clearStepsList(): void;
    getStepsList(): Array<string>;
    setStepsList(value: Array<string>): GetStepsResponse;
    addSteps(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetStepsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetStepsResponse): GetStepsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetStepsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetStepsResponse;
    static deserializeBinaryFromReader(message: GetStepsResponse, reader: jspb.BinaryReader): GetStepsResponse;
}

export namespace GetStepsResponse {
    export type AsObject = {
        stepsList: Array<string>,
    }
}

export class GetStepRequest extends jspb.Message { 
    getToken(): string;
    setToken(value: string): GetStepRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetStepRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetStepRequest): GetStepRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetStepRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetStepRequest;
    static deserializeBinaryFromReader(message: GetStepRequest, reader: jspb.BinaryReader): GetStepRequest;
}

export namespace GetStepRequest {
    export type AsObject = {
        token: string,
    }
}

export class GetStepResponse extends jspb.Message { 

    hasInputType(): boolean;
    clearInputType(): void;
    getInputType(): TypeReference | undefined;
    setInputType(value?: TypeReference): GetStepResponse;

    hasOutputType(): boolean;
    clearOutputType(): void;
    getOutputType(): TypeReference | undefined;
    setOutputType(value?: TypeReference): GetStepResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetStepResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetStepResponse): GetStepResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetStepResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetStepResponse;
    static deserializeBinaryFromReader(message: GetStepResponse, reader: jspb.BinaryReader): GetStepResponse;
}

export namespace GetStepResponse {
    export type AsObject = {
        inputType?: TypeReference.AsObject,
        outputType?: TypeReference.AsObject,
    }
}

export class InputProperty extends jspb.Message { 
    getName(): string;
    setName(value: string): InputProperty;
    getType(): string;
    setType(value: string): InputProperty;
    getRequired(): boolean;
    setRequired(value: boolean): InputProperty;
    getDescription(): string;
    setDescription(value: string): InputProperty;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): InputProperty.AsObject;
    static toObject(includeInstance: boolean, msg: InputProperty): InputProperty.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: InputProperty, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): InputProperty;
    static deserializeBinaryFromReader(message: InputProperty, reader: jspb.BinaryReader): InputProperty;
}

export namespace InputProperty {
    export type AsObject = {
        name: string,
        type: string,
        required: boolean,
        description: string,
    }
}

export class WorkflowError extends jspb.Message { 
    getReason(): string;
    setReason(value: string): WorkflowError;
    getCategory(): string;
    setCategory(value: string): WorkflowError;

    hasDetails(): boolean;
    clearDetails(): void;
    getDetails(): google_protobuf_struct_pb.Struct | undefined;
    setDetails(value?: google_protobuf_struct_pb.Struct): WorkflowError;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowError.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowError): WorkflowError.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowError, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowError;
    static deserializeBinaryFromReader(message: WorkflowError, reader: jspb.BinaryReader): WorkflowError;
}

export namespace WorkflowError {
    export type AsObject = {
        reason: string,
        category: string,
        details?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class DependencyTerm extends jspb.Message { 

    hasPath(): boolean;
    clearPath(): void;
    getPath(): string;
    setPath(value: string): DependencyTerm;

    hasExpression(): boolean;
    clearExpression(): void;
    getExpression(): DependencyExpression | undefined;
    setExpression(value?: DependencyExpression): DependencyTerm;

    getTermCase(): DependencyTerm.TermCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DependencyTerm.AsObject;
    static toObject(includeInstance: boolean, msg: DependencyTerm): DependencyTerm.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DependencyTerm, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DependencyTerm;
    static deserializeBinaryFromReader(message: DependencyTerm, reader: jspb.BinaryReader): DependencyTerm;
}

export namespace DependencyTerm {
    export type AsObject = {
        path: string,
        expression?: DependencyExpression.AsObject,
    }

    export enum TermCase {
        TERM_NOT_SET = 0,
        PATH = 1,
        EXPRESSION = 2,
    }

}

export class DependencyExpression extends jspb.Message { 
    getOperator(): DependencyExpression.Operator;
    setOperator(value: DependencyExpression.Operator): DependencyExpression;
    clearTermsList(): void;
    getTermsList(): Array<DependencyTerm>;
    setTermsList(value: Array<DependencyTerm>): DependencyExpression;
    addTerms(value?: DependencyTerm, index?: number): DependencyTerm;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DependencyExpression.AsObject;
    static toObject(includeInstance: boolean, msg: DependencyExpression): DependencyExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DependencyExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DependencyExpression;
    static deserializeBinaryFromReader(message: DependencyExpression, reader: jspb.BinaryReader): DependencyExpression;
}

export namespace DependencyExpression {
    export type AsObject = {
        operator: DependencyExpression.Operator,
        termsList: Array<DependencyTerm.AsObject>,
    }

    export enum Operator {
    OPERATOR_UNSPECIFIED = 0,
    OPERATOR_ALL = 1,
    OPERATOR_ANY = 2,
    OPERATOR_STRICT = 3,
    OPERATOR_UNSTRICT = 4,
    }

}

export class PlatformRequirements extends jspb.Message { 
    getOs(): string;
    setOs(value: string): PlatformRequirements;
    getArch(): string;
    setArch(value: string): PlatformRequirements;
    getMinVcpu(): number;
    setMinVcpu(value: number): PlatformRequirements;
    getMinMemoryGib(): number;
    setMinMemoryGib(value: number): PlatformRequirements;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PlatformRequirements.AsObject;
    static toObject(includeInstance: boolean, msg: PlatformRequirements): PlatformRequirements.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PlatformRequirements, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PlatformRequirements;
    static deserializeBinaryFromReader(message: PlatformRequirements, reader: jspb.BinaryReader): PlatformRequirements;
}

export namespace PlatformRequirements {
    export type AsObject = {
        os: string,
        arch: string,
        minVcpu: number,
        minMemoryGib: number,
    }
}

export class PlatformSelector extends jspb.Message { 
    getTarget(): string;
    setTarget(value: string): PlatformSelector;

    hasRequirements(): boolean;
    clearRequirements(): void;
    getRequirements(): PlatformRequirements | undefined;
    setRequirements(value?: PlatformRequirements): PlatformSelector;
    getMatchPolicy(): PlatformSelector.MatchPolicy;
    setMatchPolicy(value: PlatformSelector.MatchPolicy): PlatformSelector;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PlatformSelector.AsObject;
    static toObject(includeInstance: boolean, msg: PlatformSelector): PlatformSelector.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PlatformSelector, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PlatformSelector;
    static deserializeBinaryFromReader(message: PlatformSelector, reader: jspb.BinaryReader): PlatformSelector;
}

export namespace PlatformSelector {
    export type AsObject = {
        target: string,
        requirements?: PlatformRequirements.AsObject,
        matchPolicy: PlatformSelector.MatchPolicy,
    }

    export enum MatchPolicy {
    MATCH_POLICY_UNSPECIFIED = 0,
    MATCH_POLICY_EXACT = 1,
    MATCH_POLICY_CLOSEST = 2,
    }

}

export class ErrorRecord extends jspb.Message { 
    getStepPath(): string;
    setStepPath(value: string): ErrorRecord;
    getReason(): string;
    setReason(value: string): ErrorRecord;
    getCategory(): string;
    setCategory(value: string): ErrorRecord;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ErrorRecord.AsObject;
    static toObject(includeInstance: boolean, msg: ErrorRecord): ErrorRecord.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ErrorRecord, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ErrorRecord;
    static deserializeBinaryFromReader(message: ErrorRecord, reader: jspb.BinaryReader): ErrorRecord;
}

export namespace ErrorRecord {
    export type AsObject = {
        stepPath: string,
        reason: string,
        category: string,
    }
}

export class GenerateJobRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): GenerateJobRequest;
    getName(): string;
    setName(value: string): GenerateJobRequest;
    getPath(): string;
    setPath(value: string): GenerateJobRequest;
    getGraphMonitorAddress(): string;
    setGraphMonitorAddress(value: string): GenerateJobRequest;

    hasInputValue(): boolean;
    clearInputValue(): void;
    getInputValue(): google_protobuf_struct_pb.Struct | undefined;
    setInputValue(value?: google_protobuf_struct_pb.Struct): GenerateJobRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GenerateJobRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GenerateJobRequest): GenerateJobRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GenerateJobRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GenerateJobRequest;
    static deserializeBinaryFromReader(message: GenerateJobRequest, reader: jspb.BinaryReader): GenerateJobRequest;
}

export namespace GenerateJobRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        name: string,
        path: string,
        graphMonitorAddress: string,
        inputValue?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class GenerateGraphRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): GenerateGraphRequest;
    getPath(): string;
    setPath(value: string): GenerateGraphRequest;
    getGraphMonitorAddress(): string;
    setGraphMonitorAddress(value: string): GenerateGraphRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GenerateGraphRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GenerateGraphRequest): GenerateGraphRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GenerateGraphRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GenerateGraphRequest;
    static deserializeBinaryFromReader(message: GenerateGraphRequest, reader: jspb.BinaryReader): GenerateGraphRequest;
}

export namespace GenerateGraphRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        path: string,
        graphMonitorAddress: string,
    }
}

export class GenerateNodeResponse extends jspb.Message { 

    hasError(): boolean;
    clearError(): void;
    getError(): WorkflowError | undefined;
    setError(value?: WorkflowError): GenerateNodeResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GenerateNodeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GenerateNodeResponse): GenerateNodeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GenerateNodeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GenerateNodeResponse;
    static deserializeBinaryFromReader(message: GenerateNodeResponse, reader: jspb.BinaryReader): GenerateNodeResponse;
}

export namespace GenerateNodeResponse {
    export type AsObject = {
        error?: WorkflowError.AsObject,
    }
}

export class RunSensorRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): RunSensorRequest;
    getPath(): string;
    setPath(value: string): RunSensorRequest;

    hasCursor(): boolean;
    clearCursor(): void;
    getCursor(): google_protobuf_struct_pb.Value | undefined;
    setCursor(value?: google_protobuf_struct_pb.Value): RunSensorRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunSensorRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RunSensorRequest): RunSensorRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunSensorRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunSensorRequest;
    static deserializeBinaryFromReader(message: RunSensorRequest, reader: jspb.BinaryReader): RunSensorRequest;
}

export namespace RunSensorRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        path: string,
        cursor?: google_protobuf_struct_pb.Value.AsObject,
    }
}

export class RunSensorResponse extends jspb.Message { 

    hasError(): boolean;
    clearError(): void;
    getError(): WorkflowError | undefined;
    setError(value?: WorkflowError): RunSensorResponse;
    getDecision(): RunSensorResponse.Decision;
    setDecision(value: RunSensorResponse.Decision): RunSensorResponse;

    hasCursor(): boolean;
    clearCursor(): void;
    getCursor(): google_protobuf_struct_pb.Value | undefined;
    setCursor(value?: google_protobuf_struct_pb.Value): RunSensorResponse;

    hasEvent(): boolean;
    clearEvent(): void;
    getEvent(): google_protobuf_struct_pb.Value | undefined;
    setEvent(value?: google_protobuf_struct_pb.Value): RunSensorResponse;
    getNextInterval(): string;
    setNextInterval(value: string): RunSensorResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunSensorResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RunSensorResponse): RunSensorResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunSensorResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunSensorResponse;
    static deserializeBinaryFromReader(message: RunSensorResponse, reader: jspb.BinaryReader): RunSensorResponse;
}

export namespace RunSensorResponse {
    export type AsObject = {
        error?: WorkflowError.AsObject,
        decision: RunSensorResponse.Decision,
        cursor?: google_protobuf_struct_pb.Value.AsObject,
        event?: google_protobuf_struct_pb.Value.AsObject,
        nextInterval: string,
    }

    export enum Decision {
    DECISION_UNSPECIFIED = 0,
    DECISION_SKIP = 1,
    DECISION_FIRE = 2,
    }

}

export class RunStepRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): RunStepRequest;
    getPath(): string;
    setPath(value: string): RunStepRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunStepRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RunStepRequest): RunStepRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunStepRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunStepRequest;
    static deserializeBinaryFromReader(message: RunStepRequest, reader: jspb.BinaryReader): RunStepRequest;
}

export namespace RunStepRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        path: string,
    }
}

export class RunStepResponse extends jspb.Message { 

    hasError(): boolean;
    clearError(): void;
    getError(): WorkflowError | undefined;
    setError(value?: WorkflowError): RunStepResponse;

    hasResult(): boolean;
    clearResult(): void;
    getResult(): google_protobuf_struct_pb.Value | undefined;
    setResult(value?: google_protobuf_struct_pb.Value): RunStepResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunStepResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RunStepResponse): RunStepResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunStepResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunStepResponse;
    static deserializeBinaryFromReader(message: RunStepResponse, reader: jspb.BinaryReader): RunStepResponse;
}

export namespace RunStepResponse {
    export type AsObject = {
        error?: WorkflowError.AsObject,
        result?: google_protobuf_struct_pb.Value.AsObject,
    }
}

export class ResolveJobResultRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): ResolveJobResultRequest;
    getPath(): string;
    setPath(value: string): ResolveJobResultRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResolveJobResultRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ResolveJobResultRequest): ResolveJobResultRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResolveJobResultRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResolveJobResultRequest;
    static deserializeBinaryFromReader(message: ResolveJobResultRequest, reader: jspb.BinaryReader): ResolveJobResultRequest;
}

export namespace ResolveJobResultRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        path: string,
    }
}

export class ResolveJobResultResponse extends jspb.Message { 

    hasError(): boolean;
    clearError(): void;
    getError(): WorkflowError | undefined;
    setError(value?: WorkflowError): ResolveJobResultResponse;

    hasResult(): boolean;
    clearResult(): void;
    getResult(): google_protobuf_struct_pb.Value | undefined;
    setResult(value?: google_protobuf_struct_pb.Value): ResolveJobResultResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResolveJobResultResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ResolveJobResultResponse): ResolveJobResultResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResolveJobResultResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResolveJobResultResponse;
    static deserializeBinaryFromReader(message: ResolveJobResultResponse, reader: jspb.BinaryReader): ResolveJobResultResponse;
}

export namespace ResolveJobResultResponse {
    export type AsObject = {
        error?: WorkflowError.AsObject,
        result?: google_protobuf_struct_pb.Value.AsObject,
    }
}

export class RunTriggerMockRequest extends jspb.Message { 
    getToken(): string;
    setToken(value: string): RunTriggerMockRequest;
    clearArgsList(): void;
    getArgsList(): Array<string>;
    setArgsList(value: Array<string>): RunTriggerMockRequest;
    addArgs(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunTriggerMockRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RunTriggerMockRequest): RunTriggerMockRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunTriggerMockRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunTriggerMockRequest;
    static deserializeBinaryFromReader(message: RunTriggerMockRequest, reader: jspb.BinaryReader): RunTriggerMockRequest;
}

export namespace RunTriggerMockRequest {
    export type AsObject = {
        token: string,
        argsList: Array<string>,
    }
}

export class RunTriggerMockResponse extends jspb.Message { 

    hasValue(): boolean;
    clearValue(): void;
    getValue(): google_protobuf_struct_pb.Struct | undefined;
    setValue(value?: google_protobuf_struct_pb.Struct): RunTriggerMockResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunTriggerMockResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RunTriggerMockResponse): RunTriggerMockResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunTriggerMockResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunTriggerMockResponse;
    static deserializeBinaryFromReader(message: RunTriggerMockResponse, reader: jspb.BinaryReader): RunTriggerMockResponse;
}

export namespace RunTriggerMockResponse {
    export type AsObject = {
        value?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class RunFilterRequest extends jspb.Message { 
    getPath(): string;
    setPath(value: string): RunFilterRequest;

    hasValue(): boolean;
    clearValue(): void;
    getValue(): google_protobuf_struct_pb.Value | undefined;
    setValue(value?: google_protobuf_struct_pb.Value): RunFilterRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunFilterRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RunFilterRequest): RunFilterRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunFilterRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunFilterRequest;
    static deserializeBinaryFromReader(message: RunFilterRequest, reader: jspb.BinaryReader): RunFilterRequest;
}

export namespace RunFilterRequest {
    export type AsObject = {
        path: string,
        value?: google_protobuf_struct_pb.Value.AsObject,
    }
}

export class RunFilterResponse extends jspb.Message { 
    getPass(): boolean;
    setPass(value: boolean): RunFilterResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunFilterResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RunFilterResponse): RunFilterResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunFilterResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunFilterResponse;
    static deserializeBinaryFromReader(message: RunFilterResponse, reader: jspb.BinaryReader): RunFilterResponse;
}

export namespace RunFilterResponse {
    export type AsObject = {
        pass: boolean,
    }
}

export class RunOnErrorRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): RunOnErrorRequest;
    getPath(): string;
    setPath(value: string): RunOnErrorRequest;
    clearErrorsList(): void;
    getErrorsList(): Array<ErrorRecord>;
    setErrorsList(value: Array<ErrorRecord>): RunOnErrorRequest;
    addErrors(value?: ErrorRecord, index?: number): ErrorRecord;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunOnErrorRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RunOnErrorRequest): RunOnErrorRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunOnErrorRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunOnErrorRequest;
    static deserializeBinaryFromReader(message: RunOnErrorRequest, reader: jspb.BinaryReader): RunOnErrorRequest;
}

export namespace RunOnErrorRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        path: string,
        errorsList: Array<ErrorRecord.AsObject>,
    }
}

export class RunOnErrorResponse extends jspb.Message { 

    hasError(): boolean;
    clearError(): void;
    getError(): WorkflowError | undefined;
    setError(value?: WorkflowError): RunOnErrorResponse;
    getRetry(): boolean;
    setRetry(value: boolean): RunOnErrorResponse;
    getRetryAfter(): string;
    setRetryAfter(value: string): RunOnErrorResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunOnErrorResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RunOnErrorResponse): RunOnErrorResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunOnErrorResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunOnErrorResponse;
    static deserializeBinaryFromReader(message: RunOnErrorResponse, reader: jspb.BinaryReader): RunOnErrorResponse;
}

export namespace RunOnErrorResponse {
    export type AsObject = {
        error?: WorkflowError.AsObject,
        retry: boolean,
        retryAfter: string,
    }
}

export class RegisterTriggerRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): RegisterTriggerRequest;
    getPath(): string;
    setPath(value: string): RegisterTriggerRequest;
    getType(): string;
    setType(value: string): RegisterTriggerRequest;

    hasSpec(): boolean;
    clearSpec(): void;
    getSpec(): google_protobuf_struct_pb.Struct | undefined;
    setSpec(value?: google_protobuf_struct_pb.Struct): RegisterTriggerRequest;
    getHasFilter(): boolean;
    setHasFilter(value: boolean): RegisterTriggerRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterTriggerRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterTriggerRequest): RegisterTriggerRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterTriggerRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterTriggerRequest;
    static deserializeBinaryFromReader(message: RegisterTriggerRequest, reader: jspb.BinaryReader): RegisterTriggerRequest;
}

export namespace RegisterTriggerRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        path: string,
        type: string,
        spec?: google_protobuf_struct_pb.Struct.AsObject,
        hasFilter: boolean,
    }
}

export class RegisterSensorRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): RegisterSensorRequest;
    getPath(): string;
    setPath(value: string): RegisterSensorRequest;

    hasSpec(): boolean;
    clearSpec(): void;
    getSpec(): google_protobuf_struct_pb.Struct | undefined;
    setSpec(value?: google_protobuf_struct_pb.Struct): RegisterSensorRequest;
    getInterval(): string;
    setInterval(value: string): RegisterSensorRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterSensorRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterSensorRequest): RegisterSensorRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterSensorRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterSensorRequest;
    static deserializeBinaryFromReader(message: RegisterSensorRequest, reader: jspb.BinaryReader): RegisterSensorRequest;
}

export namespace RegisterSensorRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        path: string,
        spec?: google_protobuf_struct_pb.Struct.AsObject,
        interval: string,
    }
}

export class RegisterJobRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): RegisterJobRequest;
    getPath(): string;
    setPath(value: string): RegisterJobRequest;

    hasDependencies(): boolean;
    clearDependencies(): void;
    getDependencies(): DependencyExpression | undefined;
    setDependencies(value?: DependencyExpression): RegisterJobRequest;

    hasPlatform(): boolean;
    clearPlatform(): void;
    getPlatform(): PlatformSelector | undefined;
    setPlatform(value?: PlatformSelector): RegisterJobRequest;
    getHasOnError(): boolean;
    setHasOnError(value: boolean): RegisterJobRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterJobRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterJobRequest): RegisterJobRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterJobRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterJobRequest;
    static deserializeBinaryFromReader(message: RegisterJobRequest, reader: jspb.BinaryReader): RegisterJobRequest;
}

export namespace RegisterJobRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        path: string,
        dependencies?: DependencyExpression.AsObject,
        platform?: PlatformSelector.AsObject,
        hasOnError: boolean,
    }
}

export class RegisterGraphRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): RegisterGraphRequest;
    getPath(): string;
    setPath(value: string): RegisterGraphRequest;

    hasDependencies(): boolean;
    clearDependencies(): void;
    getDependencies(): DependencyExpression | undefined;
    setDependencies(value?: DependencyExpression): RegisterGraphRequest;
    getHasOnError(): boolean;
    setHasOnError(value: boolean): RegisterGraphRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterGraphRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterGraphRequest): RegisterGraphRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterGraphRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterGraphRequest;
    static deserializeBinaryFromReader(message: RegisterGraphRequest, reader: jspb.BinaryReader): RegisterGraphRequest;
}

export namespace RegisterGraphRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        path: string,
        dependencies?: DependencyExpression.AsObject,
        hasOnError: boolean,
    }
}

export class RegisterStepRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): RegisterStepRequest;
    getName(): string;
    setName(value: string): RegisterStepRequest;
    getJob(): string;
    setJob(value: string): RegisterStepRequest;

    hasDependencies(): boolean;
    clearDependencies(): void;
    getDependencies(): DependencyExpression | undefined;
    setDependencies(value?: DependencyExpression): RegisterStepRequest;
    getHasOnError(): boolean;
    setHasOnError(value: boolean): RegisterStepRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterStepRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterStepRequest): RegisterStepRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterStepRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterStepRequest;
    static deserializeBinaryFromReader(message: RegisterStepRequest, reader: jspb.BinaryReader): RegisterStepRequest;
}

export namespace RegisterStepRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        name: string,
        job: string,
        dependencies?: DependencyExpression.AsObject,
        hasOnError: boolean,
    }
}

export class RegisterNodeResponse extends jspb.Message { 

    hasValue(): boolean;
    clearValue(): void;
    getValue(): google_protobuf_struct_pb.Value | undefined;
    setValue(value?: google_protobuf_struct_pb.Value): RegisterNodeResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterNodeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterNodeResponse): RegisterNodeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterNodeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterNodeResponse;
    static deserializeBinaryFromReader(message: RegisterNodeResponse, reader: jspb.BinaryReader): RegisterNodeResponse;
}

export namespace RegisterNodeResponse {
    export type AsObject = {
        value?: google_protobuf_struct_pb.Value.AsObject,
    }
}
