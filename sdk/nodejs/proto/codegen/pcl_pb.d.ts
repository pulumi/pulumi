// package: codegen
// file: pulumi/codegen/pcl.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class Package extends jspb.Message { 
    clearNodesList(): void;
    getNodesList(): Array<Node>;
    setNodesList(value: Array<Node>): Package;
    addNodes(value?: Node, index?: number): Node;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Package.AsObject;
    static toObject(includeInstance: boolean, msg: Package): Package.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Package, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Package;
    static deserializeBinaryFromReader(message: Package, reader: jspb.BinaryReader): Package;
}

export namespace Package {
    export type AsObject = {
        nodesList: Array<Node.AsObject>,
    }
}

export class Node extends jspb.Message { 

    hasResource(): boolean;
    clearResource(): void;
    getResource(): ResourceNode | undefined;
    setResource(value?: ResourceNode): Node;

    getValueCase(): Node.ValueCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Node.AsObject;
    static toObject(includeInstance: boolean, msg: Node): Node.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Node, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Node;
    static deserializeBinaryFromReader(message: Node, reader: jspb.BinaryReader): Node;
}

export namespace Node {
    export type AsObject = {
        resource?: ResourceNode.AsObject,
    }

    export enum ValueCase {
        VALUE_NOT_SET = 0,
        RESOURCE = 1,
    }

}

export class ResourceNode extends jspb.Message { 
    clearInputsList(): void;
    getInputsList(): Array<Attribute>;
    setInputsList(value: Array<Attribute>): ResourceNode;
    addInputs(value?: Attribute, index?: number): Attribute;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceNode.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceNode): ResourceNode.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceNode, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceNode;
    static deserializeBinaryFromReader(message: ResourceNode, reader: jspb.BinaryReader): ResourceNode;
}

export namespace ResourceNode {
    export type AsObject = {
        inputsList: Array<Attribute.AsObject>,
    }
}

export class Attribute extends jspb.Message { 
    getName(): string;
    setName(value: string): Attribute;

    hasValue(): boolean;
    clearValue(): void;
    getValue(): Expression | undefined;
    setValue(value?: Expression): Attribute;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Attribute.AsObject;
    static toObject(includeInstance: boolean, msg: Attribute): Attribute.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Attribute, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Attribute;
    static deserializeBinaryFromReader(message: Attribute, reader: jspb.BinaryReader): Attribute;
}

export namespace Attribute {
    export type AsObject = {
        name: string,
        value?: Expression.AsObject,
    }
}

export class Expression extends jspb.Message { 

    hasConstnumber(): boolean;
    clearConstnumber(): void;
    getConstnumber(): number;
    setConstnumber(value: number): Expression;

    hasConststring(): boolean;
    clearConststring(): void;
    getConststring(): string;
    setConststring(value: string): Expression;

    getValueCase(): Expression.ValueCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Expression.AsObject;
    static toObject(includeInstance: boolean, msg: Expression): Expression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Expression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Expression;
    static deserializeBinaryFromReader(message: Expression, reader: jspb.BinaryReader): Expression;
}

export namespace Expression {
    export type AsObject = {
        constnumber: number,
        conststring: string,
    }

    export enum ValueCase {
        VALUE_NOT_SET = 0,
        CONSTNUMBER = 1,
        CONSTSTRING = 2,
    }

}

export class ObjectConstExpression extends jspb.Message { 
    clearItemsList(): void;
    getItemsList(): Array<ObjectConsItem>;
    setItemsList(value: Array<ObjectConsItem>): ObjectConstExpression;
    addItems(value?: ObjectConsItem, index?: number): ObjectConsItem;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ObjectConstExpression.AsObject;
    static toObject(includeInstance: boolean, msg: ObjectConstExpression): ObjectConstExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ObjectConstExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ObjectConstExpression;
    static deserializeBinaryFromReader(message: ObjectConstExpression, reader: jspb.BinaryReader): ObjectConstExpression;
}

export namespace ObjectConstExpression {
    export type AsObject = {
        itemsList: Array<ObjectConsItem.AsObject>,
    }
}

export class ObjectConsItem extends jspb.Message { 

    hasKey(): boolean;
    clearKey(): void;
    getKey(): Expression | undefined;
    setKey(value?: Expression): ObjectConsItem;

    hasValue(): boolean;
    clearValue(): void;
    getValue(): Expression | undefined;
    setValue(value?: Expression): ObjectConsItem;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ObjectConsItem.AsObject;
    static toObject(includeInstance: boolean, msg: ObjectConsItem): ObjectConsItem.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ObjectConsItem, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ObjectConsItem;
    static deserializeBinaryFromReader(message: ObjectConsItem, reader: jspb.BinaryReader): ObjectConsItem;
}

export namespace ObjectConsItem {
    export type AsObject = {
        key?: Expression.AsObject,
        value?: Expression.AsObject,
    }
}
