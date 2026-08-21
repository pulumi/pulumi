// package: pulumirpc
// file: pulumi/workflow.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class WorkflowCursor extends jspb.Message { 
    getName(): string;
    setName(value: string): WorkflowCursor;

    hasValues(): boolean;
    clearValues(): void;
    getValues(): google_protobuf_struct_pb.Struct | undefined;
    setValues(value?: google_protobuf_struct_pb.Struct): WorkflowCursor;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowCursor.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowCursor): WorkflowCursor.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowCursor, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowCursor;
    static deserializeBinaryFromReader(message: WorkflowCursor, reader: jspb.BinaryReader): WorkflowCursor;
}

export namespace WorkflowCursor {
    export type AsObject = {
        name: string,
        values?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class WorkflowVisit extends jspb.Message { 
    getCursor(): string;
    setCursor(value: string): WorkflowVisit;

    hasEntered(): boolean;
    clearEntered(): void;
    getEntered(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setEntered(value?: google_protobuf_timestamp_pb.Timestamp): WorkflowVisit;

    hasLeft(): boolean;
    clearLeft(): void;
    getLeft(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setLeft(value?: google_protobuf_timestamp_pb.Timestamp): WorkflowVisit;

    hasInputs(): boolean;
    clearInputs(): void;
    getInputs(): google_protobuf_struct_pb.Struct | undefined;
    setInputs(value?: google_protobuf_struct_pb.Struct): WorkflowVisit;

    hasOutputs(): boolean;
    clearOutputs(): void;
    getOutputs(): google_protobuf_struct_pb.Struct | undefined;
    setOutputs(value?: google_protobuf_struct_pb.Struct): WorkflowVisit;
    getError(): string;
    setError(value: string): WorkflowVisit;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowVisit.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowVisit): WorkflowVisit.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowVisit, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowVisit;
    static deserializeBinaryFromReader(message: WorkflowVisit, reader: jspb.BinaryReader): WorkflowVisit;
}

export namespace WorkflowVisit {
    export type AsObject = {
        cursor: string,
        entered?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        left?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        inputs?: google_protobuf_struct_pb.Struct.AsObject,
        outputs?: google_protobuf_struct_pb.Struct.AsObject,
        error: string,
    }
}

export class WorkflowNodeState extends jspb.Message { 
    getOccupant(): string;
    setOccupant(value: string): WorkflowNodeState;
    clearHistoryList(): void;
    getHistoryList(): Array<WorkflowVisit>;
    setHistoryList(value: Array<WorkflowVisit>): WorkflowNodeState;
    addHistory(value?: WorkflowVisit, index?: number): WorkflowVisit;

    hasLastRun(): boolean;
    clearLastRun(): void;
    getLastRun(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setLastRun(value?: google_protobuf_timestamp_pb.Timestamp): WorkflowNodeState;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowNodeState.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowNodeState): WorkflowNodeState.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowNodeState, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowNodeState;
    static deserializeBinaryFromReader(message: WorkflowNodeState, reader: jspb.BinaryReader): WorkflowNodeState;
}

export namespace WorkflowNodeState {
    export type AsObject = {
        occupant: string,
        historyList: Array<WorkflowVisit.AsObject>,
        lastRun?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    }
}

export class WorkflowView extends jspb.Message { 

    getNodesMap(): jspb.Map<string, WorkflowNodeState>;
    clearNodesMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowView.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowView): WorkflowView.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowView, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowView;
    static deserializeBinaryFromReader(message: WorkflowView, reader: jspb.BinaryReader): WorkflowView;
}

export namespace WorkflowView {
    export type AsObject = {

        nodesMap: Array<[string, WorkflowNodeState.AsObject]>,
    }
}

export class WorkflowNodeRequest extends jspb.Message { 
    getMonitorAddr(): string;
    setMonitorAddr(value: string): WorkflowNodeRequest;
    getEngineAddr(): string;
    setEngineAddr(value: string): WorkflowNodeRequest;
    getProject(): string;
    setProject(value: string): WorkflowNodeRequest;
    getStack(): string;
    setStack(value: string): WorkflowNodeRequest;
    getOrganization(): string;
    setOrganization(value: string): WorkflowNodeRequest;

    getConfigMap(): jspb.Map<string, string>;
    clearConfigMap(): void;
    clearConfigSecretKeysList(): void;
    getConfigSecretKeysList(): Array<string>;
    setConfigSecretKeysList(value: Array<string>): WorkflowNodeRequest;
    addConfigSecretKeys(value: string, index?: number): string;
    getParallel(): number;
    setParallel(value: number): WorkflowNodeRequest;
    getNode(): string;
    setNode(value: string): WorkflowNodeRequest;

    hasCursor(): boolean;
    clearCursor(): void;
    getCursor(): WorkflowCursor | undefined;
    setCursor(value?: WorkflowCursor): WorkflowNodeRequest;

    hasView(): boolean;
    clearView(): void;
    getView(): WorkflowView | undefined;
    setView(value?: WorkflowView): WorkflowNodeRequest;
    getReconcile(): boolean;
    setReconcile(value: boolean): WorkflowNodeRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowNodeRequest.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowNodeRequest): WorkflowNodeRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowNodeRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowNodeRequest;
    static deserializeBinaryFromReader(message: WorkflowNodeRequest, reader: jspb.BinaryReader): WorkflowNodeRequest;
}

export namespace WorkflowNodeRequest {
    export type AsObject = {
        monitorAddr: string,
        engineAddr: string,
        project: string,
        stack: string,
        organization: string,

        configMap: Array<[string, string]>,
        configSecretKeysList: Array<string>,
        parallel: number,
        node: string,
        cursor?: WorkflowCursor.AsObject,
        view?: WorkflowView.AsObject,
        reconcile: boolean,
    }
}

export class WorkflowNodeResponse extends jspb.Message { 

    hasOutputs(): boolean;
    clearOutputs(): void;
    getOutputs(): google_protobuf_struct_pb.Struct | undefined;
    setOutputs(value?: google_protobuf_struct_pb.Struct): WorkflowNodeResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowNodeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowNodeResponse): WorkflowNodeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowNodeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowNodeResponse;
    static deserializeBinaryFromReader(message: WorkflowNodeResponse, reader: jspb.BinaryReader): WorkflowNodeResponse;
}

export namespace WorkflowNodeResponse {
    export type AsObject = {
        outputs?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class WorkflowConditionRequest extends jspb.Message { 

    hasCursor(): boolean;
    clearCursor(): void;
    getCursor(): WorkflowCursor | undefined;
    setCursor(value?: WorkflowCursor): WorkflowConditionRequest;

    hasView(): boolean;
    clearView(): void;
    getView(): WorkflowView | undefined;
    setView(value?: WorkflowView): WorkflowConditionRequest;
    getEdge(): string;
    setEdge(value: string): WorkflowConditionRequest;
    getCondition(): string;
    setCondition(value: string): WorkflowConditionRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowConditionRequest.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowConditionRequest): WorkflowConditionRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowConditionRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowConditionRequest;
    static deserializeBinaryFromReader(message: WorkflowConditionRequest, reader: jspb.BinaryReader): WorkflowConditionRequest;
}

export namespace WorkflowConditionRequest {
    export type AsObject = {
        cursor?: WorkflowCursor.AsObject,
        view?: WorkflowView.AsObject,
        edge: string,
        condition: string,
    }
}

export class WorkflowConditionResponse extends jspb.Message { 
    getPass(): boolean;
    setPass(value: boolean): WorkflowConditionResponse;

    hasOverlay(): boolean;
    clearOverlay(): void;
    getOverlay(): google_protobuf_struct_pb.Struct | undefined;
    setOverlay(value?: google_protobuf_struct_pb.Struct): WorkflowConditionResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowConditionResponse.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowConditionResponse): WorkflowConditionResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowConditionResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowConditionResponse;
    static deserializeBinaryFromReader(message: WorkflowConditionResponse, reader: jspb.BinaryReader): WorkflowConditionResponse;
}

export namespace WorkflowConditionResponse {
    export type AsObject = {
        pass: boolean,
        overlay?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class WorkflowMergeRequest extends jspb.Message { 
    clearCandidatesList(): void;
    getCandidatesList(): Array<WorkflowMergeRequest.Candidate>;
    setCandidatesList(value: Array<WorkflowMergeRequest.Candidate>): WorkflowMergeRequest;
    addCandidates(value?: WorkflowMergeRequest.Candidate, index?: number): WorkflowMergeRequest.Candidate;

    hasView(): boolean;
    clearView(): void;
    getView(): WorkflowView | undefined;
    setView(value?: WorkflowView): WorkflowMergeRequest;
    getEdge(): string;
    setEdge(value: string): WorkflowMergeRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowMergeRequest.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowMergeRequest): WorkflowMergeRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowMergeRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowMergeRequest;
    static deserializeBinaryFromReader(message: WorkflowMergeRequest, reader: jspb.BinaryReader): WorkflowMergeRequest;
}

export namespace WorkflowMergeRequest {
    export type AsObject = {
        candidatesList: Array<WorkflowMergeRequest.Candidate.AsObject>,
        view?: WorkflowView.AsObject,
        edge: string,
    }


    export class Candidate extends jspb.Message { 
        getFrom(): string;
        setFrom(value: string): Candidate;

        hasCursor(): boolean;
        clearCursor(): void;
        getCursor(): WorkflowCursor | undefined;
        setCursor(value?: WorkflowCursor): Candidate;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): Candidate.AsObject;
        static toObject(includeInstance: boolean, msg: Candidate): Candidate.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: Candidate, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): Candidate;
        static deserializeBinaryFromReader(message: Candidate, reader: jspb.BinaryReader): Candidate;
    }

    export namespace Candidate {
        export type AsObject = {
            from: string,
            cursor?: WorkflowCursor.AsObject,
        }
    }

}

export class WorkflowMergeResponse extends jspb.Message { 
    getMerge(): boolean;
    setMerge(value: boolean): WorkflowMergeResponse;
    getName(): string;
    setName(value: string): WorkflowMergeResponse;

    hasValues(): boolean;
    clearValues(): void;
    getValues(): google_protobuf_struct_pb.Struct | undefined;
    setValues(value?: google_protobuf_struct_pb.Struct): WorkflowMergeResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowMergeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowMergeResponse): WorkflowMergeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowMergeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowMergeResponse;
    static deserializeBinaryFromReader(message: WorkflowMergeResponse, reader: jspb.BinaryReader): WorkflowMergeResponse;
}

export namespace WorkflowMergeResponse {
    export type AsObject = {
        merge: boolean,
        name: string,
        values?: google_protobuf_struct_pb.Struct.AsObject,
    }
}
