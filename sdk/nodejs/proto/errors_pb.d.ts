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

export class PropertiesError extends jspb.Message { 
    clearErrorsList(): void;
    getErrorsList(): Array<PropertiesError.PropertyError>;
    setErrorsList(value: Array<PropertiesError.PropertyError>): PropertiesError;
    addErrors(value?: PropertiesError.PropertyError, index?: number): PropertiesError.PropertyError;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PropertiesError.AsObject;
    static toObject(includeInstance: boolean, msg: PropertiesError): PropertiesError.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PropertiesError, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PropertiesError;
    static deserializeBinaryFromReader(message: PropertiesError, reader: jspb.BinaryReader): PropertiesError;
}

export namespace PropertiesError {
    export type AsObject = {
        errorsList: Array<PropertiesError.PropertyError.AsObject>,
    }


    export class PropertyError extends jspb.Message { 
        getPropertyName(): string;
        setPropertyName(value: string): PropertyError;
        getReason(): string;
        setReason(value: string): PropertyError;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): PropertyError.AsObject;
        static toObject(includeInstance: boolean, msg: PropertyError): PropertyError.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: PropertyError, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): PropertyError;
        static deserializeBinaryFromReader(message: PropertyError, reader: jspb.BinaryReader): PropertyError;
    }

    export namespace PropertyError {
        export type AsObject = {
            propertyName: string,
            reason: string,
        }
    }

}
