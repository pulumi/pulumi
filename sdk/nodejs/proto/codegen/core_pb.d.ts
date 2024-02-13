// package: pulumirpc.codegen
// file: pulumi/codegen/core.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class Core extends jspb.Message { 

    hasSdk(): boolean;
    clearSdk(): void;
    getSdk(): SDK | undefined;
    setSdk(value?: SDK): Core;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Core.AsObject;
    static toObject(includeInstance: boolean, msg: Core): Core.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Core, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Core;
    static deserializeBinaryFromReader(message: Core, reader: jspb.BinaryReader): Core;
}

export namespace Core {
    export type AsObject = {
        sdk?: SDK.AsObject,
    }
}

export class TypeReference extends jspb.Message { 

    hasPrimitive(): boolean;
    clearPrimitive(): void;
    getPrimitive(): PrimitiveType;
    setPrimitive(value: PrimitiveType): TypeReference;

    hasArray(): boolean;
    clearArray(): void;
    getArray(): TypeReference | undefined;
    setArray(value?: TypeReference): TypeReference;

    hasMap(): boolean;
    clearMap(): void;
    getMap(): TypeReference | undefined;
    setMap(value?: TypeReference): TypeReference;

    hasRef(): boolean;
    clearRef(): void;
    getRef(): string;
    setRef(value: string): TypeReference;

    getElementCase(): TypeReference.ElementCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TypeReference.AsObject;
    static toObject(includeInstance: boolean, msg: TypeReference): TypeReference.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TypeReference, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TypeReference;
    static deserializeBinaryFromReader(message: TypeReference, reader: jspb.BinaryReader): TypeReference;
}

export namespace TypeReference {
    export type AsObject = {
        primitive: PrimitiveType,
        array?: TypeReference.AsObject,
        map?: TypeReference.AsObject,
        ref: string,
    }

    export enum ElementCase {
        ELEMENT_NOT_SET = 0,
        PRIMITIVE = 1,
        ARRAY = 2,
        MAP = 3,
        REF = 4,
    }

}

export class EnumerationValue extends jspb.Message { 
    getName(): string;
    setName(value: string): EnumerationValue;
    getDescription(): string;
    setDescription(value: string): EnumerationValue;
    getProtobufValue(): string;
    setProtobufValue(value: string): EnumerationValue;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): EnumerationValue.AsObject;
    static toObject(includeInstance: boolean, msg: EnumerationValue): EnumerationValue.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: EnumerationValue, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): EnumerationValue;
    static deserializeBinaryFromReader(message: EnumerationValue, reader: jspb.BinaryReader): EnumerationValue;
}

export namespace EnumerationValue {
    export type AsObject = {
        name: string,
        description: string,
        protobufValue: string,
    }
}

export class Enumeration extends jspb.Message { 
    getName(): string;
    setName(value: string): Enumeration;
    getDescription(): string;
    setDescription(value: string): Enumeration;
    clearValuesList(): void;
    getValuesList(): Array<EnumerationValue>;
    setValuesList(value: Array<EnumerationValue>): Enumeration;
    addValues(value?: EnumerationValue, index?: number): EnumerationValue;
    getProtobufEnum(): string;
    setProtobufEnum(value: string): Enumeration;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Enumeration.AsObject;
    static toObject(includeInstance: boolean, msg: Enumeration): Enumeration.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Enumeration, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Enumeration;
    static deserializeBinaryFromReader(message: Enumeration, reader: jspb.BinaryReader): Enumeration;
}

export namespace Enumeration {
    export type AsObject = {
        name: string,
        description: string,
        valuesList: Array<EnumerationValue.AsObject>,
        protobufEnum: string,
    }
}

export class TypeDeclaration extends jspb.Message { 

    hasRecord(): boolean;
    clearRecord(): void;
    getRecord(): Record | undefined;
    setRecord(value?: Record): TypeDeclaration;

    hasInterface(): boolean;
    clearInterface(): void;
    getInterface(): Interface | undefined;
    setInterface(value?: Interface): TypeDeclaration;

    hasEnumeration(): boolean;
    clearEnumeration(): void;
    getEnumeration(): Enumeration | undefined;
    setEnumeration(value?: Enumeration): TypeDeclaration;

    getElementCase(): TypeDeclaration.ElementCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TypeDeclaration.AsObject;
    static toObject(includeInstance: boolean, msg: TypeDeclaration): TypeDeclaration.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TypeDeclaration, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TypeDeclaration;
    static deserializeBinaryFromReader(message: TypeDeclaration, reader: jspb.BinaryReader): TypeDeclaration;
}

export namespace TypeDeclaration {
    export type AsObject = {
        record?: Record.AsObject,
        pb_interface?: Interface.AsObject,
        enumeration?: Enumeration.AsObject,
    }

    export enum ElementCase {
        ELEMENT_NOT_SET = 0,
        RECORD = 1,
        INTERFACE = 2,
        ENUMERATION = 3,
    }

}

export class Property extends jspb.Message { 
    getName(): string;
    setName(value: string): Property;
    getDescription(): string;
    setDescription(value: string): Property;

    hasType(): boolean;
    clearType(): void;
    getType(): TypeReference | undefined;
    setType(value?: TypeReference): Property;
    getProtobufField(): string;
    setProtobufField(value: string): Property;
    getProtobufMapping(): CustomPropertyMapping;
    setProtobufMapping(value: CustomPropertyMapping): Property;
    getProtobufPresenceField(): string;
    setProtobufPresenceField(value: string): Property;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Property.AsObject;
    static toObject(includeInstance: boolean, msg: Property): Property.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Property, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Property;
    static deserializeBinaryFromReader(message: Property, reader: jspb.BinaryReader): Property;
}

export namespace Property {
    export type AsObject = {
        name: string,
        description: string,
        type?: TypeReference.AsObject,
        protobufField: string,
        protobufMapping: CustomPropertyMapping,
        protobufPresenceField: string,
    }
}

export class Record extends jspb.Message { 
    getName(): string;
    setName(value: string): Record;
    getDescription(): string;
    setDescription(value: string): Record;
    clearPropertiesList(): void;
    getPropertiesList(): Array<Property>;
    setPropertiesList(value: Array<Property>): Record;
    addProperties(value?: Property, index?: number): Property;
    getProtobufMessage(): string;
    setProtobufMessage(value: string): Record;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Record.AsObject;
    static toObject(includeInstance: boolean, msg: Record): Record.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Record, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Record;
    static deserializeBinaryFromReader(message: Record, reader: jspb.BinaryReader): Record;
}

export namespace Record {
    export type AsObject = {
        name: string,
        description: string,
        propertiesList: Array<Property.AsObject>,
        protobufMessage: string,
    }
}

export class Request extends jspb.Message { 
    getName(): string;
    setName(value: string): Request;
    getDescription(): string;
    setDescription(value: string): Request;
    getType(): string;
    setType(value: string): Request;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Request.AsObject;
    static toObject(includeInstance: boolean, msg: Request): Request.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Request, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Request;
    static deserializeBinaryFromReader(message: Request, reader: jspb.BinaryReader): Request;
}

export namespace Request {
    export type AsObject = {
        name: string,
        description: string,
        type: string,
    }
}

export class Method extends jspb.Message { 
    getName(): string;
    setName(value: string): Method;
    getDescription(): string;
    setDescription(value: string): Method;

    hasRequest(): boolean;
    clearRequest(): void;
    getRequest(): Request | undefined;
    setRequest(value?: Request): Method;
    getResponseType(): string;
    setResponseType(value: string): Method;
    getGrpcMethod(): string;
    setGrpcMethod(value: string): Method;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Method.AsObject;
    static toObject(includeInstance: boolean, msg: Method): Method.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Method, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Method;
    static deserializeBinaryFromReader(message: Method, reader: jspb.BinaryReader): Method;
}

export namespace Method {
    export type AsObject = {
        name: string,
        description: string,
        request?: Request.AsObject,
        responseType: string,
        grpcMethod: string,
    }
}

export class Interface extends jspb.Message { 
    getName(): string;
    setName(value: string): Interface;
    getDescription(): string;
    setDescription(value: string): Interface;
    clearMethodsList(): void;
    getMethodsList(): Array<Method>;
    setMethodsList(value: Array<Method>): Interface;
    addMethods(value?: Method, index?: number): Method;
    getGrpcService(): string;
    setGrpcService(value: string): Interface;
    getGrpcKind(): GrpcKind;
    setGrpcKind(value: GrpcKind): Interface;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Interface.AsObject;
    static toObject(includeInstance: boolean, msg: Interface): Interface.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Interface, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Interface;
    static deserializeBinaryFromReader(message: Interface, reader: jspb.BinaryReader): Interface;
}

export namespace Interface {
    export type AsObject = {
        name: string,
        description: string,
        methodsList: Array<Method.AsObject>,
        grpcService: string,
        grpcKind: GrpcKind,
    }
}

export class SDK extends jspb.Message { 
    clearTypeDeclarationsList(): void;
    getTypeDeclarationsList(): Array<TypeDeclaration>;
    setTypeDeclarationsList(value: Array<TypeDeclaration>): SDK;
    addTypeDeclarations(value?: TypeDeclaration, index?: number): TypeDeclaration;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SDK.AsObject;
    static toObject(includeInstance: boolean, msg: SDK): SDK.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SDK, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SDK;
    static deserializeBinaryFromReader(message: SDK, reader: jspb.BinaryReader): SDK;
}

export namespace SDK {
    export type AsObject = {
        typeDeclarationsList: Array<TypeDeclaration.AsObject>,
    }
}

export enum PrimitiveType {
    BOOL = 0,
    BYTE = 1,
    INT = 2,
    STRING = 3,
    DURATION = 4,
    PROPERTY_VALUE = 5,
}

export enum CustomPropertyMapping {
    NONE = 0,
    URN_NAME = 1,
    URN_TYPE = 2,
}

export enum GrpcKind {
    KIND_BOTH = 0,
    KIND_SERVER = 1,
    KIND_CLIENT = 2,
}
