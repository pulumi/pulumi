// package: pulumirpc
// file: pulumi/events.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";

export class EventRequest extends jspb.Message { 
    getEvent(): string;
    setEvent(value: string): EventRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): EventRequest.AsObject;
    static toObject(includeInstance: boolean, msg: EventRequest): EventRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: EventRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): EventRequest;
    static deserializeBinaryFromReader(message: EventRequest, reader: jspb.BinaryReader): EventRequest;
}

export namespace EventRequest {
    export type AsObject = {
        event: string,
    }
}
