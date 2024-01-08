// package: pulumirpc
// file: pulumi/plugin.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class PluginInfo extends jspb.Message { 
    getVersion(): string;
    setVersion(value: string): PluginInfo;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PluginInfo.AsObject;
    static toObject(includeInstance: boolean, msg: PluginInfo): PluginInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PluginInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PluginInfo;
    static deserializeBinaryFromReader(message: PluginInfo, reader: jspb.BinaryReader): PluginInfo;
}

export namespace PluginInfo {
    export type AsObject = {
        version: string,
    }
}

export class PluginDependency extends jspb.Message { 
    getName(): string;
    setName(value: string): PluginDependency;
    getKind(): string;
    setKind(value: string): PluginDependency;
    getVersion(): string;
    setVersion(value: string): PluginDependency;
    getServer(): string;
    setServer(value: string): PluginDependency;

    getChecksumsMap(): jspb.Map<string, Uint8Array | string>;
    clearChecksumsMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PluginDependency.AsObject;
    static toObject(includeInstance: boolean, msg: PluginDependency): PluginDependency.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PluginDependency, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PluginDependency;
    static deserializeBinaryFromReader(message: PluginDependency, reader: jspb.BinaryReader): PluginDependency;
}

export namespace PluginDependency {
    export type AsObject = {
        name: string,
        kind: string,
        version: string,
        server: string,

        checksumsMap: Array<[string, Uint8Array | string]>,
    }
}

export class PluginAttach extends jspb.Message { 
    getAddress(): string;
    setAddress(value: string): PluginAttach;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PluginAttach.AsObject;
    static toObject(includeInstance: boolean, msg: PluginAttach): PluginAttach.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PluginAttach, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PluginAttach;
    static deserializeBinaryFromReader(message: PluginAttach, reader: jspb.BinaryReader): PluginAttach;
}

export namespace PluginAttach {
    export type AsObject = {
        address: string,
    }
}
