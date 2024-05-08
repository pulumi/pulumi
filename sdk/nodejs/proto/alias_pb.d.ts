// package: pulumirpc
// file: pulumi/alias.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class Alias extends jspb.Message { 

    hasUrn(): boolean;
    clearUrn(): void;
    getUrn(): string;
    setUrn(value: string): Alias;

    hasSpec(): boolean;
    clearSpec(): void;
    getSpec(): Alias.Spec | undefined;
    setSpec(value?: Alias.Spec): Alias;

    getAliasCase(): Alias.AliasCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Alias.AsObject;
    static toObject(includeInstance: boolean, msg: Alias): Alias.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Alias, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Alias;
    static deserializeBinaryFromReader(message: Alias, reader: jspb.BinaryReader): Alias;
}

export namespace Alias {
    export type AsObject = {
        urn: string,
        spec?: Alias.Spec.AsObject,
    }


    export class Spec extends jspb.Message { 
        getName(): string;
        setName(value: string): Spec;
        getType(): string;
        setType(value: string): Spec;
        getStack(): string;
        setStack(value: string): Spec;
        getProject(): string;
        setProject(value: string): Spec;

        hasParenturn(): boolean;
        clearParenturn(): void;
        getParenturn(): string;
        setParenturn(value: string): Spec;

        hasNoparent(): boolean;
        clearNoparent(): void;
        getNoparent(): boolean;
        setNoparent(value: boolean): Spec;

        getParentCase(): Spec.ParentCase;

        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): Spec.AsObject;
        static toObject(includeInstance: boolean, msg: Spec): Spec.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: Spec, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): Spec;
        static deserializeBinaryFromReader(message: Spec, reader: jspb.BinaryReader): Spec;
    }

    export namespace Spec {
        export type AsObject = {
            name: string,
            type: string,
            stack: string,
            project: string,
            parenturn: string,
            noparent: boolean,
        }

        export enum ParentCase {
            PARENT_NOT_SET = 0,
            PARENTURN = 5,
            NOPARENT = 6,
        }

    }


    export enum AliasCase {
        ALIAS_NOT_SET = 0,
        URN = 1,
        SPEC = 2,
    }

}
