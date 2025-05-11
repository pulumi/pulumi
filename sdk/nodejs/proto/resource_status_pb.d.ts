// package: pulumirpc
// file: pulumi/resource_status.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as pulumi_provider_pb from "./provider_pb";

export class PublishViewStepsRequest extends jspb.Message { 
    getToken(): string;
    setToken(value: string): PublishViewStepsRequest;
    clearStepsList(): void;
    getStepsList(): Array<ViewStep>;
    setStepsList(value: Array<ViewStep>): PublishViewStepsRequest;
    addSteps(value?: ViewStep, index?: number): ViewStep;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PublishViewStepsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: PublishViewStepsRequest): PublishViewStepsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PublishViewStepsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PublishViewStepsRequest;
    static deserializeBinaryFromReader(message: PublishViewStepsRequest, reader: jspb.BinaryReader): PublishViewStepsRequest;
}

export namespace PublishViewStepsRequest {
    export type AsObject = {
        token: string,
        stepsList: Array<ViewStep.AsObject>,
    }
}

export class PublishViewStepsResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PublishViewStepsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: PublishViewStepsResponse): PublishViewStepsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PublishViewStepsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PublishViewStepsResponse;
    static deserializeBinaryFromReader(message: PublishViewStepsResponse, reader: jspb.BinaryReader): PublishViewStepsResponse;
}

export namespace PublishViewStepsResponse {
    export type AsObject = {
    }
}

export class ViewStep extends jspb.Message { 
    getStatus(): ViewStep.Status;
    setStatus(value: ViewStep.Status): ViewStep;
    getError(): string;
    setError(value: string): ViewStep;
    getOp(): ViewStep.Op;
    setOp(value: ViewStep.Op): ViewStep;
    getType(): string;
    setType(value: string): ViewStep;
    getName(): string;
    setName(value: string): ViewStep;

    hasOld(): boolean;
    clearOld(): void;
    getOld(): ViewStepState | undefined;
    setOld(value?: ViewStepState): ViewStep;

    hasNew(): boolean;
    clearNew(): void;
    getNew(): ViewStepState | undefined;
    setNew(value?: ViewStepState): ViewStep;
    clearKeysList(): void;
    getKeysList(): Array<string>;
    setKeysList(value: Array<string>): ViewStep;
    addKeys(value: string, index?: number): string;
    clearDiffsList(): void;
    getDiffsList(): Array<string>;
    setDiffsList(value: Array<string>): ViewStep;
    addDiffs(value: string, index?: number): string;

    getDetailedDiffMap(): jspb.Map<string, pulumi_provider_pb.PropertyDiff>;
    clearDetailedDiffMap(): void;
    getHasDetailedDiff(): boolean;
    setHasDetailedDiff(value: boolean): ViewStep;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ViewStep.AsObject;
    static toObject(includeInstance: boolean, msg: ViewStep): ViewStep.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ViewStep, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ViewStep;
    static deserializeBinaryFromReader(message: ViewStep, reader: jspb.BinaryReader): ViewStep;
}

export namespace ViewStep {
    export type AsObject = {
        status: ViewStep.Status,
        error: string,
        op: ViewStep.Op,
        type: string,
        name: string,
        old?: ViewStepState.AsObject,
        pb_new?: ViewStepState.AsObject,
        keysList: Array<string>,
        diffsList: Array<string>,

        detailedDiffMap: Array<[string, pulumi_provider_pb.PropertyDiff.AsObject]>,
        hasDetailedDiff: boolean,
    }

    export enum Op {
    UNSPECIFIED = 0,
    SAME = 1,
    CREATE = 2,
    UPDATE = 3,
    DELETE = 4,
    REPLACE = 5,
    CREATE_REPLACEMENT = 6,
    DELETE_REPLACED = 7,
    READ = 8,
    READ_REPLACEMENT = 9,
    REFRESH = 10,
    READ_DISCARD = 11,
    DISCARD_REPLACED = 12,
    REMOVE_PENDING_REPLACE = 13,
    IMPORT = 14,
    IMPORT_REPLACEMENT = 15,
    }

    export enum Status {
    OK = 0,
    PARTIAL_FAILURE = 1,
    UNKNOWN = 2,
    }

}

export class ViewStepState extends jspb.Message { 
    getType(): string;
    setType(value: string): ViewStepState;
    getName(): string;
    setName(value: string): ViewStepState;
    getParentType(): string;
    setParentType(value: string): ViewStepState;
    getParentName(): string;
    setParentName(value: string): ViewStepState;

    hasInputs(): boolean;
    clearInputs(): void;
    getInputs(): google_protobuf_struct_pb.Struct | undefined;
    setInputs(value?: google_protobuf_struct_pb.Struct): ViewStepState;

    hasOutputs(): boolean;
    clearOutputs(): void;
    getOutputs(): google_protobuf_struct_pb.Struct | undefined;
    setOutputs(value?: google_protobuf_struct_pb.Struct): ViewStepState;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ViewStepState.AsObject;
    static toObject(includeInstance: boolean, msg: ViewStepState): ViewStepState.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ViewStepState, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ViewStepState;
    static deserializeBinaryFromReader(message: ViewStepState, reader: jspb.BinaryReader): ViewStepState;
}

export namespace ViewStepState {
    export type AsObject = {
        type: string,
        name: string,
        parentType: string,
        parentName: string,
        inputs?: google_protobuf_struct_pb.Struct.AsObject,
        outputs?: google_protobuf_struct_pb.Struct.AsObject,
    }
}
