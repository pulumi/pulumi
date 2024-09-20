// package: google.rpc
// file: google/protobuf/error_details.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_duration_pb from "google-protobuf/google/protobuf/duration_pb";

export class ErrorInfo extends jspb.Message { 
    getReason(): string;
    setReason(value: string): ErrorInfo;
    getDomain(): string;
    setDomain(value: string): ErrorInfo;

    getMetadataMap(): jspb.Map<string, string>;
    clearMetadataMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ErrorInfo.AsObject;
    static toObject(includeInstance: boolean, msg: ErrorInfo): ErrorInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ErrorInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ErrorInfo;
    static deserializeBinaryFromReader(message: ErrorInfo, reader: jspb.BinaryReader): ErrorInfo;
}

export namespace ErrorInfo {
    export type AsObject = {
        reason: string,
        domain: string,

        metadataMap: Array<[string, string]>,
    }
}

export class RetryInfo extends jspb.Message { 

    hasRetryDelay(): boolean;
    clearRetryDelay(): void;
    getRetryDelay(): google_protobuf_duration_pb.Duration | undefined;
    setRetryDelay(value?: google_protobuf_duration_pb.Duration): RetryInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RetryInfo.AsObject;
    static toObject(includeInstance: boolean, msg: RetryInfo): RetryInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RetryInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RetryInfo;
    static deserializeBinaryFromReader(message: RetryInfo, reader: jspb.BinaryReader): RetryInfo;
}

export namespace RetryInfo {
    export type AsObject = {
        retryDelay?: google_protobuf_duration_pb.Duration.AsObject,
    }
}

export class DebugInfo extends jspb.Message { 
    clearStackEntriesList(): void;
    getStackEntriesList(): Array<string>;
    setStackEntriesList(value: Array<string>): DebugInfo;
    addStackEntries(value: string, index?: number): string;
    getDetail(): string;
    setDetail(value: string): DebugInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DebugInfo.AsObject;
    static toObject(includeInstance: boolean, msg: DebugInfo): DebugInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DebugInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DebugInfo;
    static deserializeBinaryFromReader(message: DebugInfo, reader: jspb.BinaryReader): DebugInfo;
}

export namespace DebugInfo {
    export type AsObject = {
        stackEntriesList: Array<string>,
        detail: string,
    }
}

export class QuotaFailure extends jspb.Message { 
    clearViolationsList(): void;
    getViolationsList(): Array<QuotaFailure.Violation>;
    setViolationsList(value: Array<QuotaFailure.Violation>): QuotaFailure;
    addViolations(value?: QuotaFailure.Violation, index?: number): QuotaFailure.Violation;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): QuotaFailure.AsObject;
    static toObject(includeInstance: boolean, msg: QuotaFailure): QuotaFailure.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: QuotaFailure, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): QuotaFailure;
    static deserializeBinaryFromReader(message: QuotaFailure, reader: jspb.BinaryReader): QuotaFailure;
}

export namespace QuotaFailure {
    export type AsObject = {
        violationsList: Array<QuotaFailure.Violation.AsObject>,
    }


    export class Violation extends jspb.Message { 
        getSubject(): string;
        setSubject(value: string): Violation;
        getDescription(): string;
        setDescription(value: string): Violation;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): Violation.AsObject;
        static toObject(includeInstance: boolean, msg: Violation): Violation.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: Violation, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): Violation;
        static deserializeBinaryFromReader(message: Violation, reader: jspb.BinaryReader): Violation;
    }

    export namespace Violation {
        export type AsObject = {
            subject: string,
            description: string,
        }
    }

}

export class PreconditionFailure extends jspb.Message { 
    clearViolationsList(): void;
    getViolationsList(): Array<PreconditionFailure.Violation>;
    setViolationsList(value: Array<PreconditionFailure.Violation>): PreconditionFailure;
    addViolations(value?: PreconditionFailure.Violation, index?: number): PreconditionFailure.Violation;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PreconditionFailure.AsObject;
    static toObject(includeInstance: boolean, msg: PreconditionFailure): PreconditionFailure.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PreconditionFailure, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PreconditionFailure;
    static deserializeBinaryFromReader(message: PreconditionFailure, reader: jspb.BinaryReader): PreconditionFailure;
}

export namespace PreconditionFailure {
    export type AsObject = {
        violationsList: Array<PreconditionFailure.Violation.AsObject>,
    }


    export class Violation extends jspb.Message { 
        getType(): string;
        setType(value: string): Violation;
        getSubject(): string;
        setSubject(value: string): Violation;
        getDescription(): string;
        setDescription(value: string): Violation;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): Violation.AsObject;
        static toObject(includeInstance: boolean, msg: Violation): Violation.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: Violation, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): Violation;
        static deserializeBinaryFromReader(message: Violation, reader: jspb.BinaryReader): Violation;
    }

    export namespace Violation {
        export type AsObject = {
            type: string,
            subject: string,
            description: string,
        }
    }

}

export class BadRequest extends jspb.Message { 
    clearFieldViolationsList(): void;
    getFieldViolationsList(): Array<BadRequest.FieldViolation>;
    setFieldViolationsList(value: Array<BadRequest.FieldViolation>): BadRequest;
    addFieldViolations(value?: BadRequest.FieldViolation, index?: number): BadRequest.FieldViolation;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): BadRequest.AsObject;
    static toObject(includeInstance: boolean, msg: BadRequest): BadRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: BadRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): BadRequest;
    static deserializeBinaryFromReader(message: BadRequest, reader: jspb.BinaryReader): BadRequest;
}

export namespace BadRequest {
    export type AsObject = {
        fieldViolationsList: Array<BadRequest.FieldViolation.AsObject>,
    }


    export class FieldViolation extends jspb.Message { 
        getField(): string;
        setField(value: string): FieldViolation;
        getDescription(): string;
        setDescription(value: string): FieldViolation;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): FieldViolation.AsObject;
        static toObject(includeInstance: boolean, msg: FieldViolation): FieldViolation.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: FieldViolation, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): FieldViolation;
        static deserializeBinaryFromReader(message: FieldViolation, reader: jspb.BinaryReader): FieldViolation;
    }

    export namespace FieldViolation {
        export type AsObject = {
            field: string,
            description: string,
        }
    }

}

export class RequestInfo extends jspb.Message { 
    getRequestId(): string;
    setRequestId(value: string): RequestInfo;
    getServingData(): string;
    setServingData(value: string): RequestInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RequestInfo.AsObject;
    static toObject(includeInstance: boolean, msg: RequestInfo): RequestInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RequestInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RequestInfo;
    static deserializeBinaryFromReader(message: RequestInfo, reader: jspb.BinaryReader): RequestInfo;
}

export namespace RequestInfo {
    export type AsObject = {
        requestId: string,
        servingData: string,
    }
}

export class ResourceInfo extends jspb.Message { 
    getResourceType(): string;
    setResourceType(value: string): ResourceInfo;
    getResourceName(): string;
    setResourceName(value: string): ResourceInfo;
    getOwner(): string;
    setOwner(value: string): ResourceInfo;
    getDescription(): string;
    setDescription(value: string): ResourceInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceInfo.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceInfo): ResourceInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceInfo;
    static deserializeBinaryFromReader(message: ResourceInfo, reader: jspb.BinaryReader): ResourceInfo;
}

export namespace ResourceInfo {
    export type AsObject = {
        resourceType: string,
        resourceName: string,
        owner: string,
        description: string,
    }
}

export class Help extends jspb.Message { 
    clearLinksList(): void;
    getLinksList(): Array<Help.Link>;
    setLinksList(value: Array<Help.Link>): Help;
    addLinks(value?: Help.Link, index?: number): Help.Link;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Help.AsObject;
    static toObject(includeInstance: boolean, msg: Help): Help.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Help, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Help;
    static deserializeBinaryFromReader(message: Help, reader: jspb.BinaryReader): Help;
}

export namespace Help {
    export type AsObject = {
        linksList: Array<Help.Link.AsObject>,
    }


    export class Link extends jspb.Message { 
        getDescription(): string;
        setDescription(value: string): Link;
        getUrl(): string;
        setUrl(value: string): Link;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): Link.AsObject;
        static toObject(includeInstance: boolean, msg: Link): Link.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: Link, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): Link;
        static deserializeBinaryFromReader(message: Link, reader: jspb.BinaryReader): Link;
    }

    export namespace Link {
        export type AsObject = {
            description: string,
            url: string,
        }
    }

}

export class LocalizedMessage extends jspb.Message { 
    getLocale(): string;
    setLocale(value: string): LocalizedMessage;
    getMessage(): string;
    setMessage(value: string): LocalizedMessage;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): LocalizedMessage.AsObject;
    static toObject(includeInstance: boolean, msg: LocalizedMessage): LocalizedMessage.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: LocalizedMessage, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): LocalizedMessage;
    static deserializeBinaryFromReader(message: LocalizedMessage, reader: jspb.BinaryReader): LocalizedMessage;
}

export namespace LocalizedMessage {
    export type AsObject = {
        locale: string,
        message: string,
    }
}
