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

export class InputPropertiesError extends jspb.Message { 
    clearErrorsList(): void;
    getErrorsList(): Array<InputPropertiesError.PropertyError>;
    setErrorsList(value: Array<InputPropertiesError.PropertyError>): InputPropertiesError;
    addErrors(value?: InputPropertiesError.PropertyError, index?: number): InputPropertiesError.PropertyError;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): InputPropertiesError.AsObject;
    static toObject(includeInstance: boolean, msg: InputPropertiesError): InputPropertiesError.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: InputPropertiesError, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): InputPropertiesError;
    static deserializeBinaryFromReader(message: InputPropertiesError, reader: jspb.BinaryReader): InputPropertiesError;
}

export namespace InputPropertiesError {
    export type AsObject = {
        errorsList: Array<InputPropertiesError.PropertyError.AsObject>,
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
