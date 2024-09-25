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

export class InvalidInputPropertiesError extends jspb.Message { 
    clearErrorsList(): void;
    getErrorsList(): Array<InvalidInputPropertiesError.PropertyError>;
    setErrorsList(value: Array<InvalidInputPropertiesError.PropertyError>): InvalidInputPropertiesError;
    addErrors(value?: InvalidInputPropertiesError.PropertyError, index?: number): InvalidInputPropertiesError.PropertyError;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): InvalidInputPropertiesError.AsObject;
    static toObject(includeInstance: boolean, msg: InvalidInputPropertiesError): InvalidInputPropertiesError.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: InvalidInputPropertiesError, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): InvalidInputPropertiesError;
    static deserializeBinaryFromReader(message: InvalidInputPropertiesError, reader: jspb.BinaryReader): InvalidInputPropertiesError;
}

export namespace InvalidInputPropertiesError {
    export type AsObject = {
        errorsList: Array<InvalidInputPropertiesError.PropertyError.AsObject>,
    }


    export class PropertyError extends jspb.Message { 
        getPropertyPath(): string;
        setPropertyPath(value: string): PropertyError;
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
            propertyPath: string,
            reason: string,
        }
    }

}
