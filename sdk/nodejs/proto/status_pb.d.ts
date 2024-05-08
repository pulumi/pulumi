// package: google.rpc
// file: google/protobuf/status.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_any_pb from "google-protobuf/google/protobuf/any_pb";

export class Status extends jspb.Message { 
    getCode(): number;
    setCode(value: number): Status;
    getMessage(): string;
    setMessage(value: string): Status;
    clearDetailsList(): void;
    getDetailsList(): Array<google_protobuf_any_pb.Any>;
    setDetailsList(value: Array<google_protobuf_any_pb.Any>): Status;
    addDetails(value?: google_protobuf_any_pb.Any, index?: number): google_protobuf_any_pb.Any;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Status.AsObject;
    static toObject(includeInstance: boolean, msg: Status): Status.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Status, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Status;
    static deserializeBinaryFromReader(message: Status, reader: jspb.BinaryReader): Status;
}

export namespace Status {
    export type AsObject = {
        code: number,
        message: string,
        detailsList: Array<google_protobuf_any_pb.Any.AsObject>,
    }
}
