// package: codegen
// file: pulumi/codegen/workflow.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class WorkflowPackageDescriptor extends jspb.Message { 
    getName(): string;
    setName(value: string): WorkflowPackageDescriptor;
    getVersion(): string;
    setVersion(value: string): WorkflowPackageDescriptor;
    getDownloadUrl(): string;
    setDownloadUrl(value: string): WorkflowPackageDescriptor;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowPackageDescriptor.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowPackageDescriptor): WorkflowPackageDescriptor.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowPackageDescriptor, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowPackageDescriptor;
    static deserializeBinaryFromReader(message: WorkflowPackageDescriptor, reader: jspb.BinaryReader): WorkflowPackageDescriptor;
}

export namespace WorkflowPackageDescriptor {
    export type AsObject = {
        name: string,
        version: string,
        downloadUrl: string,
    }
}

export class GetWorkflowPackageInfoRequest extends jspb.Message { 

    hasPackage(): boolean;
    clearPackage(): void;
    getPackage(): WorkflowPackageDescriptor | undefined;
    setPackage(value?: WorkflowPackageDescriptor): GetWorkflowPackageInfoRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetWorkflowPackageInfoRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetWorkflowPackageInfoRequest): GetWorkflowPackageInfoRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetWorkflowPackageInfoRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetWorkflowPackageInfoRequest;
    static deserializeBinaryFromReader(message: GetWorkflowPackageInfoRequest, reader: jspb.BinaryReader): GetWorkflowPackageInfoRequest;
}

export namespace GetWorkflowPackageInfoRequest {
    export type AsObject = {
        pb_package?: WorkflowPackageDescriptor.AsObject,
    }
}

export class GetWorkflowGraphsRequest extends jspb.Message { 

    hasPackage(): boolean;
    clearPackage(): void;
    getPackage(): WorkflowPackageDescriptor | undefined;
    setPackage(value?: WorkflowPackageDescriptor): GetWorkflowGraphsRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetWorkflowGraphsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetWorkflowGraphsRequest): GetWorkflowGraphsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetWorkflowGraphsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetWorkflowGraphsRequest;
    static deserializeBinaryFromReader(message: GetWorkflowGraphsRequest, reader: jspb.BinaryReader): GetWorkflowGraphsRequest;
}

export namespace GetWorkflowGraphsRequest {
    export type AsObject = {
        pb_package?: WorkflowPackageDescriptor.AsObject,
    }
}

export class GetWorkflowGraphRequest extends jspb.Message { 

    hasPackage(): boolean;
    clearPackage(): void;
    getPackage(): WorkflowPackageDescriptor | undefined;
    setPackage(value?: WorkflowPackageDescriptor): GetWorkflowGraphRequest;
    getToken(): string;
    setToken(value: string): GetWorkflowGraphRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetWorkflowGraphRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetWorkflowGraphRequest): GetWorkflowGraphRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetWorkflowGraphRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetWorkflowGraphRequest;
    static deserializeBinaryFromReader(message: GetWorkflowGraphRequest, reader: jspb.BinaryReader): GetWorkflowGraphRequest;
}

export namespace GetWorkflowGraphRequest {
    export type AsObject = {
        pb_package?: WorkflowPackageDescriptor.AsObject,
        token: string,
    }
}

export class GetWorkflowTriggersRequest extends jspb.Message { 

    hasPackage(): boolean;
    clearPackage(): void;
    getPackage(): WorkflowPackageDescriptor | undefined;
    setPackage(value?: WorkflowPackageDescriptor): GetWorkflowTriggersRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetWorkflowTriggersRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetWorkflowTriggersRequest): GetWorkflowTriggersRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetWorkflowTriggersRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetWorkflowTriggersRequest;
    static deserializeBinaryFromReader(message: GetWorkflowTriggersRequest, reader: jspb.BinaryReader): GetWorkflowTriggersRequest;
}

export namespace GetWorkflowTriggersRequest {
    export type AsObject = {
        pb_package?: WorkflowPackageDescriptor.AsObject,
    }
}

export class GetWorkflowTriggerRequest extends jspb.Message { 

    hasPackage(): boolean;
    clearPackage(): void;
    getPackage(): WorkflowPackageDescriptor | undefined;
    setPackage(value?: WorkflowPackageDescriptor): GetWorkflowTriggerRequest;
    getToken(): string;
    setToken(value: string): GetWorkflowTriggerRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetWorkflowTriggerRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetWorkflowTriggerRequest): GetWorkflowTriggerRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetWorkflowTriggerRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetWorkflowTriggerRequest;
    static deserializeBinaryFromReader(message: GetWorkflowTriggerRequest, reader: jspb.BinaryReader): GetWorkflowTriggerRequest;
}

export namespace GetWorkflowTriggerRequest {
    export type AsObject = {
        pb_package?: WorkflowPackageDescriptor.AsObject,
        token: string,
    }
}

export class GetWorkflowJobsRequest extends jspb.Message { 

    hasPackage(): boolean;
    clearPackage(): void;
    getPackage(): WorkflowPackageDescriptor | undefined;
    setPackage(value?: WorkflowPackageDescriptor): GetWorkflowJobsRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetWorkflowJobsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetWorkflowJobsRequest): GetWorkflowJobsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetWorkflowJobsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetWorkflowJobsRequest;
    static deserializeBinaryFromReader(message: GetWorkflowJobsRequest, reader: jspb.BinaryReader): GetWorkflowJobsRequest;
}

export namespace GetWorkflowJobsRequest {
    export type AsObject = {
        pb_package?: WorkflowPackageDescriptor.AsObject,
    }
}

export class GetWorkflowJobRequest extends jspb.Message { 

    hasPackage(): boolean;
    clearPackage(): void;
    getPackage(): WorkflowPackageDescriptor | undefined;
    setPackage(value?: WorkflowPackageDescriptor): GetWorkflowJobRequest;
    getToken(): string;
    setToken(value: string): GetWorkflowJobRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetWorkflowJobRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetWorkflowJobRequest): GetWorkflowJobRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetWorkflowJobRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetWorkflowJobRequest;
    static deserializeBinaryFromReader(message: GetWorkflowJobRequest, reader: jspb.BinaryReader): GetWorkflowJobRequest;
}

export namespace GetWorkflowJobRequest {
    export type AsObject = {
        pb_package?: WorkflowPackageDescriptor.AsObject,
        token: string,
    }
}

export class TypeReference extends jspb.Message { 
    getToken(): string;
    setToken(value: string): TypeReference;

    hasObject(): boolean;
    clearObject(): void;
    getObject(): StructObject | undefined;
    setObject(value?: StructObject): TypeReference;

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
        object?: StructObject.AsObject,
    }
}

export class PropertySpec extends jspb.Message { 
    getType(): string;
    setType(value: string): PropertySpec;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PropertySpec.AsObject;
    static toObject(includeInstance: boolean, msg: PropertySpec): PropertySpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PropertySpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PropertySpec;
    static deserializeBinaryFromReader(message: PropertySpec, reader: jspb.BinaryReader): PropertySpec;
}

export namespace PropertySpec {
    export type AsObject = {
        type: string,
    }
}

export class StructObject extends jspb.Message { 

    getPropertiesMap(): jspb.Map<string, PropertySpec>;
    clearPropertiesMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): StructObject.AsObject;
    static toObject(includeInstance: boolean, msg: StructObject): StructObject.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: StructObject, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): StructObject;
    static deserializeBinaryFromReader(message: StructObject, reader: jspb.BinaryReader): StructObject;
}

export namespace StructObject {
    export type AsObject = {

        propertiesMap: Array<[string, PropertySpec.AsObject]>,
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

export class GetJobResponse extends jspb.Message { 

    hasJob(): boolean;
    clearJob(): void;
    getJob(): JobInfo | undefined;
    setJob(value?: JobInfo): GetJobResponse;

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
    }
}
