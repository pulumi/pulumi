// package: pulumirpc.codegen
// file: pulumi/codegen/hcl.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class Pos extends jspb.Message { 
    getLine(): number;
    setLine(value: number): Pos;
    getColumn(): number;
    setColumn(value: number): Pos;
    getByte(): number;
    setByte(value: number): Pos;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Pos.AsObject;
    static toObject(includeInstance: boolean, msg: Pos): Pos.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Pos, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Pos;
    static deserializeBinaryFromReader(message: Pos, reader: jspb.BinaryReader): Pos;
}

export namespace Pos {
    export type AsObject = {
        line: number,
        column: number,
        pb_byte: number,
    }
}

export class Range extends jspb.Message { 
    getFilename(): string;
    setFilename(value: string): Range;

    hasStart(): boolean;
    clearStart(): void;
    getStart(): Pos | undefined;
    setStart(value?: Pos): Range;

    hasEnd(): boolean;
    clearEnd(): void;
    getEnd(): Pos | undefined;
    setEnd(value?: Pos): Range;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Range.AsObject;
    static toObject(includeInstance: boolean, msg: Range): Range.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Range, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Range;
    static deserializeBinaryFromReader(message: Range, reader: jspb.BinaryReader): Range;
}

export namespace Range {
    export type AsObject = {
        filename: string,
        start?: Pos.AsObject,
        end?: Pos.AsObject,
    }
}

export class Diagnostic extends jspb.Message { 
    getSeverity(): DiagnosticSeverity;
    setSeverity(value: DiagnosticSeverity): Diagnostic;
    getSummary(): string;
    setSummary(value: string): Diagnostic;
    getDetail(): string;
    setDetail(value: string): Diagnostic;

    hasSubject(): boolean;
    clearSubject(): void;
    getSubject(): Range | undefined;
    setSubject(value?: Range): Diagnostic;

    hasContext(): boolean;
    clearContext(): void;
    getContext(): Range | undefined;
    setContext(value?: Range): Diagnostic;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Diagnostic.AsObject;
    static toObject(includeInstance: boolean, msg: Diagnostic): Diagnostic.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Diagnostic, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Diagnostic;
    static deserializeBinaryFromReader(message: Diagnostic, reader: jspb.BinaryReader): Diagnostic;
}

export namespace Diagnostic {
    export type AsObject = {
        severity: DiagnosticSeverity,
        summary: string,
        detail: string,
        subject?: Range.AsObject,
        context?: Range.AsObject,
    }
}

export enum DiagnosticSeverity {
    DIAG_INVALID = 0,
    DIAG_ERROR = 1,
    DIAG_WARNING = 2,
}
