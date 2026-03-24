// package: pulumirpc
// file: pulumi/workflow.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

export class WorkflowContext extends jspb.Message { 
    getWorkflowname(): string;
    setWorkflowname(value: string): WorkflowContext;
    getWorkflowversion(): string;
    setWorkflowversion(value: string): WorkflowContext;
    getExecutionid(): string;
    setExecutionid(value: string): WorkflowContext;

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
        workflowname: string,
        workflowversion: string,
        executionid: string,
    }
}

export class RegisterComponentRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): RegisterComponentRequest;
    getToken(): string;
    setToken(value: string): RegisterComponentRequest;
    getKind(): WorkflowComponentKind;
    setKind(value: WorkflowComponentKind): RegisterComponentRequest;

    hasMetadata(): boolean;
    clearMetadata(): void;
    getMetadata(): google_protobuf_struct_pb.Struct | undefined;
    setMetadata(value?: google_protobuf_struct_pb.Struct): RegisterComponentRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RegisterComponentRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RegisterComponentRequest): RegisterComponentRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RegisterComponentRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RegisterComponentRequest;
    static deserializeBinaryFromReader(message: RegisterComponentRequest, reader: jspb.BinaryReader): RegisterComponentRequest;
}

export namespace RegisterComponentRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        token: string,
        kind: WorkflowComponentKind,
        metadata?: google_protobuf_struct_pb.Struct.AsObject,
    }
}

export class WorkflowRegistryHandshakeRequest extends jspb.Message { 
    getEngineAddress(): string;
    setEngineAddress(value: string): WorkflowRegistryHandshakeRequest;

    hasRootDirectory(): boolean;
    clearRootDirectory(): void;
    getRootDirectory(): string | undefined;
    setRootDirectory(value: string): WorkflowRegistryHandshakeRequest;

    hasProgramDirectory(): boolean;
    clearProgramDirectory(): void;
    getProgramDirectory(): string | undefined;
    setProgramDirectory(value: string): WorkflowRegistryHandshakeRequest;
    getGraphMonitorAddress(): string;
    setGraphMonitorAddress(value: string): WorkflowRegistryHandshakeRequest;
    getGraphMonitorContextToken(): string;
    setGraphMonitorContextToken(value: string): WorkflowRegistryHandshakeRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowRegistryHandshakeRequest.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowRegistryHandshakeRequest): WorkflowRegistryHandshakeRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowRegistryHandshakeRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowRegistryHandshakeRequest;
    static deserializeBinaryFromReader(message: WorkflowRegistryHandshakeRequest, reader: jspb.BinaryReader): WorkflowRegistryHandshakeRequest;
}

export namespace WorkflowRegistryHandshakeRequest {
    export type AsObject = {
        engineAddress: string,
        rootDirectory?: string,
        programDirectory?: string,
        graphMonitorAddress: string,
        graphMonitorContextToken: string,
    }
}

export class WorkflowRegistryHandshakeResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowRegistryHandshakeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowRegistryHandshakeResponse): WorkflowRegistryHandshakeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowRegistryHandshakeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowRegistryHandshakeResponse;
    static deserializeBinaryFromReader(message: WorkflowRegistryHandshakeResponse, reader: jspb.BinaryReader): WorkflowRegistryHandshakeResponse;
}

export namespace WorkflowRegistryHandshakeResponse {
    export type AsObject = {
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

export class WorkflowValue extends jspb.Message { 
    getKnown(): boolean;
    setKnown(value: boolean): WorkflowValue;

    hasValue(): boolean;
    clearValue(): void;
    getValue(): google_protobuf_struct_pb.Value | undefined;
    setValue(value?: google_protobuf_struct_pb.Value): WorkflowValue;

    hasError(): boolean;
    clearError(): void;
    getError(): WorkflowError | undefined;
    setError(value?: WorkflowError): WorkflowValue;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): WorkflowValue.AsObject;
    static toObject(includeInstance: boolean, msg: WorkflowValue): WorkflowValue.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: WorkflowValue, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): WorkflowValue;
    static deserializeBinaryFromReader(message: WorkflowValue, reader: jspb.BinaryReader): WorkflowValue;
}

export namespace WorkflowValue {
    export type AsObject = {
        known: boolean,
        value?: google_protobuf_struct_pb.Value.AsObject,
        error?: WorkflowError.AsObject,
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
    getMinvcpu(): number;
    setMinvcpu(value: number): PlatformRequirements;
    getMinmemorygib(): number;
    setMinmemorygib(value: number): PlatformRequirements;

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
        minvcpu: number,
        minmemorygib: number,
    }
}

export class PlatformSelector extends jspb.Message { 
    getTarget(): string;
    setTarget(value: string): PlatformSelector;

    hasRequirements(): boolean;
    clearRequirements(): void;
    getRequirements(): PlatformRequirements | undefined;
    setRequirements(value?: PlatformRequirements): PlatformSelector;
    getMatchpolicy(): PlatformSelector.MatchPolicy;
    setMatchpolicy(value: PlatformSelector.MatchPolicy): PlatformSelector;

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
        matchpolicy: PlatformSelector.MatchPolicy,
    }

    export enum MatchPolicy {
    MATCH_POLICY_UNSPECIFIED = 0,
    MATCH_POLICY_EXACT = 1,
    MATCH_POLICY_CLOSEST = 2,
    }

}

export class ErrorRecord extends jspb.Message { 
    getSteppath(): string;
    setSteppath(value: string): ErrorRecord;
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
        steppath: string,
        reason: string,
        category: string,
    }
}

export class GenerateJobRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): GenerateJobRequest;
    getPath(): string;
    setPath(value: string): GenerateJobRequest;

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
        path: string,
    }
}

export class GenerateGraphRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): GenerateGraphRequest;
    getPath(): string;
    setPath(value: string): GenerateGraphRequest;

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
    getCursor(): WorkflowValue | undefined;
    setCursor(value?: WorkflowValue): RunSensorRequest;

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
        cursor?: WorkflowValue.AsObject,
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
    getCursor(): WorkflowValue | undefined;
    setCursor(value?: WorkflowValue): RunSensorResponse;

    hasEvent(): boolean;
    clearEvent(): void;
    getEvent(): WorkflowValue | undefined;
    setEvent(value?: WorkflowValue): RunSensorResponse;
    getNextinterval(): string;
    setNextinterval(value: string): RunSensorResponse;

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
        cursor?: WorkflowValue.AsObject,
        event?: WorkflowValue.AsObject,
        nextinterval: string,
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
    getResult(): WorkflowValue | undefined;
    setResult(value?: WorkflowValue): RunStepResponse;

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
        result?: WorkflowValue.AsObject,
    }
}

export class RunFilterRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): RunFilterRequest;
    getPath(): string;
    setPath(value: string): RunFilterRequest;

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
        context?: WorkflowContext.AsObject,
        path: string,
    }
}

export class RunFilterResponse extends jspb.Message { 

    hasError(): boolean;
    clearError(): void;
    getError(): WorkflowError | undefined;
    setError(value?: WorkflowError): RunFilterResponse;
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
        error?: WorkflowError.AsObject,
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
    getRetryafter(): string;
    setRetryafter(value: string): RunOnErrorResponse;

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
        retryafter: string,
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
    getHasfilter(): boolean;
    setHasfilter(value: boolean): RegisterTriggerRequest;

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
        hasfilter: boolean,
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
    getHasonerror(): boolean;
    setHasonerror(value: boolean): RegisterJobRequest;

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
        hasonerror: boolean,
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
    getHasonerror(): boolean;
    setHasonerror(value: boolean): RegisterGraphRequest;

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
        hasonerror: boolean,
    }
}

export class RegisterStepRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): RegisterStepRequest;
    getPath(): string;
    setPath(value: string): RegisterStepRequest;
    getJobpath(): string;
    setJobpath(value: string): RegisterStepRequest;

    hasDependencies(): boolean;
    clearDependencies(): void;
    getDependencies(): DependencyExpression | undefined;
    setDependencies(value?: DependencyExpression): RegisterStepRequest;
    getHasonerror(): boolean;
    setHasonerror(value: boolean): RegisterStepRequest;

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
        path: string,
        jobpath: string,
        dependencies?: DependencyExpression.AsObject,
        hasonerror: boolean,
    }
}

export class RegisterNodeResponse extends jspb.Message { 

    hasValue(): boolean;
    clearValue(): void;
    getValue(): WorkflowValue | undefined;
    setValue(value?: WorkflowValue): RegisterNodeResponse;

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
        value?: WorkflowValue.AsObject,
    }
}

export class GetStepResultRequest extends jspb.Message { 

    hasContext(): boolean;
    clearContext(): void;
    getContext(): WorkflowContext | undefined;
    setContext(value?: WorkflowContext): GetStepResultRequest;
    getPath(): string;
    setPath(value: string): GetStepResultRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetStepResultRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetStepResultRequest): GetStepResultRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetStepResultRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetStepResultRequest;
    static deserializeBinaryFromReader(message: GetStepResultRequest, reader: jspb.BinaryReader): GetStepResultRequest;
}

export namespace GetStepResultRequest {
    export type AsObject = {
        context?: WorkflowContext.AsObject,
        path: string,
    }
}

export class GetStepResultResponse extends jspb.Message { 

    hasResult(): boolean;
    clearResult(): void;
    getResult(): WorkflowValue | undefined;
    setResult(value?: WorkflowValue): GetStepResultResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetStepResultResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetStepResultResponse): GetStepResultResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetStepResultResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetStepResultResponse;
    static deserializeBinaryFromReader(message: GetStepResultResponse, reader: jspb.BinaryReader): GetStepResultResponse;
}

export namespace GetStepResultResponse {
    export type AsObject = {
        result?: WorkflowValue.AsObject,
    }
}

export enum WorkflowComponentKind {
    WORKFLOW_COMPONENT_KIND_UNSPECIFIED = 0,
    WORKFLOW_COMPONENT_KIND_GRAPH = 1,
    WORKFLOW_COMPONENT_KIND_JOB = 2,
    WORKFLOW_COMPONENT_KIND_SUBGRAPH = 3,
    WORKFLOW_COMPONENT_KIND_STEP = 4,
    WORKFLOW_COMPONENT_KIND_FUNCTION = 5,
}
