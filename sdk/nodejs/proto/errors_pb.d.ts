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

export class InvalidInputProperties extends jspb.Message { 
    clearErrorsList(): void;
    getErrorsList(): Array<InvalidInputProperties.InvalidInputProperty>;
    setErrorsList(value: Array<InvalidInputProperties.InvalidInputProperty>): InvalidInputProperties;
    addErrors(value?: InvalidInputProperties.InvalidInputProperty, index?: number): InvalidInputProperties.InvalidInputProperty;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): InvalidInputProperties.AsObject;
    static toObject(includeInstance: boolean, msg: InvalidInputProperties): InvalidInputProperties.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: InvalidInputProperties, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): InvalidInputProperties;
    static deserializeBinaryFromReader(message: InvalidInputProperties, reader: jspb.BinaryReader): InvalidInputProperties;
}

export namespace InvalidInputProperties {
    export type AsObject = {
        errorsList: Array<InvalidInputProperties.InvalidInputProperty.AsObject>,
    }


    export class InvalidInputProperty extends jspb.Message { 
        getPropertyName(): string;
        setPropertyName(value: string): InvalidInputProperty;
        getReason(): string;
        setReason(value: string): InvalidInputProperty;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): InvalidInputProperty.AsObject;
        static toObject(includeInstance: boolean, msg: InvalidInputProperty): InvalidInputProperty.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: InvalidInputProperty, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): InvalidInputProperty;
        static deserializeBinaryFromReader(message: InvalidInputProperty, reader: jspb.BinaryReader): InvalidInputProperty;
    }

    export namespace InvalidInputProperty {
        export type AsObject = {
            propertyName: string,
            reason: string,
        }
    }

}
