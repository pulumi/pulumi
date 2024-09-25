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
    getErrorsList(): Array<InvalidInputPropertiesError.InvalidInputPropertyError>;
    setErrorsList(value: Array<InvalidInputPropertiesError.InvalidInputPropertyError>): InvalidInputPropertiesError;
    addErrors(value?: InvalidInputPropertiesError.InvalidInputPropertyError, index?: number): InvalidInputPropertiesError.InvalidInputPropertyError;

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
        errorsList: Array<InvalidInputPropertiesError.InvalidInputPropertyError.AsObject>,
    }


    export class InvalidInputPropertyError extends jspb.Message { 
        getPropertyPath(): string;
        setPropertyPath(value: string): InvalidInputPropertyError;
        getReason(): string;
        setReason(value: string): InvalidInputPropertyError;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): InvalidInputPropertyError.AsObject;
        static toObject(includeInstance: boolean, msg: InvalidInputPropertyError): InvalidInputPropertyError.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: InvalidInputPropertyError, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): InvalidInputPropertyError;
        static deserializeBinaryFromReader(message: InvalidInputPropertyError, reader: jspb.BinaryReader): InvalidInputPropertyError;
    }

    export namespace InvalidInputPropertyError {
        export type AsObject = {
            propertyPath: string,
            reason: string,
        }
    }

}
