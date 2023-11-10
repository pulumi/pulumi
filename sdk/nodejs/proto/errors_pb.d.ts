// package: pulumirpc
// file: pulumi/errors.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class ErrorCause extends jspb.Message { 
    getMessage(): string;
    setMessage(value: string): ErrorCause;
    getStacktrace(): string;
    setStacktrace(value: string): ErrorCause;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ErrorCause.AsObject;
    static toObject(includeInstance: boolean, msg: ErrorCause): ErrorCause.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ErrorCause, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ErrorCause;
    static deserializeBinaryFromReader(message: ErrorCause, reader: jspb.BinaryReader): ErrorCause;
}

export namespace ErrorCause {
    export type AsObject = {
        message: string,
        stacktrace: string,
    }
}
