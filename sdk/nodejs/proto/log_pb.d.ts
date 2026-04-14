// package: pulumirpc
// file: pulumi/log.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

export class LogPropertyValue extends jspb.Message { 
    getMagic(): number;
    setMagic(value: number): LogPropertyValue;

    hasValue(): boolean;
    clearValue(): void;
    getValue(): google_protobuf_struct_pb.Struct | undefined;
    setValue(value?: google_protobuf_struct_pb.Struct): LogPropertyValue;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): LogPropertyValue.AsObject;
    static toObject(includeInstance: boolean, msg: LogPropertyValue): LogPropertyValue.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: LogPropertyValue, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): LogPropertyValue;
    static deserializeBinaryFromReader(message: LogPropertyValue, reader: jspb.BinaryReader): LogPropertyValue;
}

export namespace LogPropertyValue {
    export type AsObject = {
        magic: number,
        value?: google_protobuf_struct_pb.Struct.AsObject,
    }
}
