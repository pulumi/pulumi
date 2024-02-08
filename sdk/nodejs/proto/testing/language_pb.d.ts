// package: pulumirpc.testing
// file: pulumi/testing/language.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class GetLanguageTestsRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetLanguageTestsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetLanguageTestsRequest): GetLanguageTestsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetLanguageTestsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetLanguageTestsRequest;
    static deserializeBinaryFromReader(message: GetLanguageTestsRequest, reader: jspb.BinaryReader): GetLanguageTestsRequest;
}

export namespace GetLanguageTestsRequest {
    export type AsObject = {
    }
}

export class GetLanguageTestsResponse extends jspb.Message { 
    clearTestsList(): void;
    getTestsList(): Array<string>;
    setTestsList(value: Array<string>): GetLanguageTestsResponse;
    addTests(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetLanguageTestsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetLanguageTestsResponse): GetLanguageTestsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetLanguageTestsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetLanguageTestsResponse;
    static deserializeBinaryFromReader(message: GetLanguageTestsResponse, reader: jspb.BinaryReader): GetLanguageTestsResponse;
}

export namespace GetLanguageTestsResponse {
    export type AsObject = {
        testsList: Array<string>,
    }
}

export class PrepareLanguageTestsRequest extends jspb.Message { 
    getLanguagePluginName(): string;
    setLanguagePluginName(value: string): PrepareLanguageTestsRequest;
    getLanguagePluginTarget(): string;
    setLanguagePluginTarget(value: string): PrepareLanguageTestsRequest;
    getSnapshotDirectory(): string;
    setSnapshotDirectory(value: string): PrepareLanguageTestsRequest;
    getTemporaryDirectory(): string;
    setTemporaryDirectory(value: string): PrepareLanguageTestsRequest;
    getCoreSdkDirectory(): string;
    setCoreSdkDirectory(value: string): PrepareLanguageTestsRequest;
    getCoreSdkVersion(): string;
    setCoreSdkVersion(value: string): PrepareLanguageTestsRequest;
    clearSnapshotEditsList(): void;
    getSnapshotEditsList(): Array<PrepareLanguageTestsRequest.Replacement>;
    setSnapshotEditsList(value: Array<PrepareLanguageTestsRequest.Replacement>): PrepareLanguageTestsRequest;
    addSnapshotEdits(value?: PrepareLanguageTestsRequest.Replacement, index?: number): PrepareLanguageTestsRequest.Replacement;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PrepareLanguageTestsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: PrepareLanguageTestsRequest): PrepareLanguageTestsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PrepareLanguageTestsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PrepareLanguageTestsRequest;
    static deserializeBinaryFromReader(message: PrepareLanguageTestsRequest, reader: jspb.BinaryReader): PrepareLanguageTestsRequest;
}

export namespace PrepareLanguageTestsRequest {
    export type AsObject = {
        languagePluginName: string,
        languagePluginTarget: string,
        snapshotDirectory: string,
        temporaryDirectory: string,
        coreSdkDirectory: string,
        coreSdkVersion: string,
        snapshotEditsList: Array<PrepareLanguageTestsRequest.Replacement.AsObject>,
    }


    export class Replacement extends jspb.Message { 
        getPath(): string;
        setPath(value: string): Replacement;
        getPattern(): string;
        setPattern(value: string): Replacement;
        getReplacement(): string;
        setReplacement(value: string): Replacement;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): Replacement.AsObject;
        static toObject(includeInstance: boolean, msg: Replacement): Replacement.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: Replacement, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): Replacement;
        static deserializeBinaryFromReader(message: Replacement, reader: jspb.BinaryReader): Replacement;
    }

    export namespace Replacement {
        export type AsObject = {
            path: string,
            pattern: string,
            replacement: string,
        }
    }

}

export class PrepareLanguageTestsResponse extends jspb.Message { 
    getToken(): string;
    setToken(value: string): PrepareLanguageTestsResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PrepareLanguageTestsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: PrepareLanguageTestsResponse): PrepareLanguageTestsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PrepareLanguageTestsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PrepareLanguageTestsResponse;
    static deserializeBinaryFromReader(message: PrepareLanguageTestsResponse, reader: jspb.BinaryReader): PrepareLanguageTestsResponse;
}

export namespace PrepareLanguageTestsResponse {
    export type AsObject = {
        token: string,
    }
}

export class RunLanguageTestRequest extends jspb.Message { 
    getToken(): string;
    setToken(value: string): RunLanguageTestRequest;
    getTest(): string;
    setTest(value: string): RunLanguageTestRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunLanguageTestRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RunLanguageTestRequest): RunLanguageTestRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunLanguageTestRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunLanguageTestRequest;
    static deserializeBinaryFromReader(message: RunLanguageTestRequest, reader: jspb.BinaryReader): RunLanguageTestRequest;
}

export namespace RunLanguageTestRequest {
    export type AsObject = {
        token: string,
        test: string,
    }
}

export class RunLanguageTestResponse extends jspb.Message { 
    getSuccess(): boolean;
    setSuccess(value: boolean): RunLanguageTestResponse;
    clearMessagesList(): void;
    getMessagesList(): Array<string>;
    setMessagesList(value: Array<string>): RunLanguageTestResponse;
    addMessages(value: string, index?: number): string;
    getStdout(): string;
    setStdout(value: string): RunLanguageTestResponse;
    getStderr(): string;
    setStderr(value: string): RunLanguageTestResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RunLanguageTestResponse.AsObject;
    static toObject(includeInstance: boolean, msg: RunLanguageTestResponse): RunLanguageTestResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RunLanguageTestResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RunLanguageTestResponse;
    static deserializeBinaryFromReader(message: RunLanguageTestResponse, reader: jspb.BinaryReader): RunLanguageTestResponse;
}

export namespace RunLanguageTestResponse {
    export type AsObject = {
        success: boolean,
        messagesList: Array<string>,
        stdout: string,
        stderr: string,
    }
}
