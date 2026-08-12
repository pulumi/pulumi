// package: pulumirpc
// file: pulumi/source.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class SourcePosition extends jspb.Message { 
    getUri(): string;
    setUri(value: string): SourcePosition;
    getLine(): number;
    setLine(value: number): SourcePosition;
    getColumn(): number;
    setColumn(value: number): SourcePosition;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SourcePosition.AsObject;
    static toObject(includeInstance: boolean, msg: SourcePosition): SourcePosition.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SourcePosition, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SourcePosition;
    static deserializeBinaryFromReader(message: SourcePosition, reader: jspb.BinaryReader): SourcePosition;
}

export namespace SourcePosition {
    export type AsObject = {
        uri: string,
        line: number,
        column: number,
    }
}

export class StackFrame extends jspb.Message { 

    hasPc(): boolean;
    clearPc(): void;
    getPc(): SourcePosition | undefined;
    setPc(value?: SourcePosition): StackFrame;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): StackFrame.AsObject;
    static toObject(includeInstance: boolean, msg: StackFrame): StackFrame.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: StackFrame, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): StackFrame;
    static deserializeBinaryFromReader(message: StackFrame, reader: jspb.BinaryReader): StackFrame;
}

export namespace StackFrame {
    export type AsObject = {
        pc?: SourcePosition.AsObject,
    }
}

export class StackTrace extends jspb.Message { 
    clearFramesList(): void;
    getFramesList(): Array<StackFrame>;
    setFramesList(value: Array<StackFrame>): StackTrace;
    addFrames(value?: StackFrame, index?: number): StackFrame;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): StackTrace.AsObject;
    static toObject(includeInstance: boolean, msg: StackTrace): StackTrace.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: StackTrace, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): StackTrace;
    static deserializeBinaryFromReader(message: StackTrace, reader: jspb.BinaryReader): StackTrace;
}

export namespace StackTrace {
    export type AsObject = {
        framesList: Array<StackFrame.AsObject>,
    }
}
