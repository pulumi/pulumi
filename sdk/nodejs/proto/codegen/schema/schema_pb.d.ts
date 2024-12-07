// package: pulumirpc.codegen.schema
// file: pulumi/codegen/schema/schema.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class PackageInfo extends jspb.Message { 
    getName(): string;
    setName(value: string): PackageInfo;

    hasVersion(): boolean;
    clearVersion(): void;
    getVersion(): string | undefined;
    setVersion(value: string): PackageInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PackageInfo.AsObject;
    static toObject(includeInstance: boolean, msg: PackageInfo): PackageInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PackageInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PackageInfo;
    static deserializeBinaryFromReader(message: PackageInfo, reader: jspb.BinaryReader): PackageInfo;
}

export namespace PackageInfo {
    export type AsObject = {
        name: string,
        version?: string,
    }
}

export class Resource extends jspb.Message { 

    hasDescription(): boolean;
    clearDescription(): void;
    getDescription(): string | undefined;
    setDescription(value: string): Resource;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Resource.AsObject;
    static toObject(includeInstance: boolean, msg: Resource): Resource.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Resource, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Resource;
    static deserializeBinaryFromReader(message: Resource, reader: jspb.BinaryReader): Resource;
}

export namespace Resource {
    export type AsObject = {
        description?: string,
    }
}

export class List extends jspb.Message { 
    clearItemsList(): void;
    getItemsList(): Array<string>;
    setItemsList(value: Array<string>): List;
    addItems(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): List.AsObject;
    static toObject(includeInstance: boolean, msg: List): List.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: List, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): List;
    static deserializeBinaryFromReader(message: List, reader: jspb.BinaryReader): List;
}

export namespace List {
    export type AsObject = {
        itemsList: Array<string>,
    }
}
