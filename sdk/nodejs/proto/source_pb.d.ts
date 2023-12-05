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
