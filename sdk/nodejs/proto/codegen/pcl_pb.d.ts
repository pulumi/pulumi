// package: codegen
// file: pulumi/codegen/pcl.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class PclProtobufProgram extends jspb.Message { 
    clearNodesList(): void;
    getNodesList(): Array<Node>;
    setNodesList(value: Array<Node>): PclProtobufProgram;
    addNodes(value?: Node, index?: number): Node;
    clearPluginsList(): void;
    getPluginsList(): Array<PluginReference>;
    setPluginsList(value: Array<PluginReference>): PclProtobufProgram;
    addPlugins(value?: PluginReference, index?: number): PluginReference;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PclProtobufProgram.AsObject;
    static toObject(includeInstance: boolean, msg: PclProtobufProgram): PclProtobufProgram.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PclProtobufProgram, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PclProtobufProgram;
    static deserializeBinaryFromReader(message: PclProtobufProgram, reader: jspb.BinaryReader): PclProtobufProgram;
}

export namespace PclProtobufProgram {
    export type AsObject = {
        nodesList: Array<Node.AsObject>,
        pluginsList: Array<PluginReference.AsObject>,
    }
}

export class PluginReference extends jspb.Message { 
    getName(): string;
    setName(value: string): PluginReference;
    getVersion(): string;
    setVersion(value: string): PluginReference;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PluginReference.AsObject;
    static toObject(includeInstance: boolean, msg: PluginReference): PluginReference.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PluginReference, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PluginReference;
    static deserializeBinaryFromReader(message: PluginReference, reader: jspb.BinaryReader): PluginReference;
}

export namespace PluginReference {
    export type AsObject = {
        name: string,
        version: string,
    }
}

export class Node extends jspb.Message { 

    hasResource(): boolean;
    clearResource(): void;
    getResource(): Resource | undefined;
    setResource(value?: Resource): Node;

    hasLocalvariable(): boolean;
    clearLocalvariable(): void;
    getLocalvariable(): LocalVariable | undefined;
    setLocalvariable(value?: LocalVariable): Node;

    hasConfigvariable(): boolean;
    clearConfigvariable(): void;
    getConfigvariable(): ConfigVariable | undefined;
    setConfigvariable(value?: ConfigVariable): Node;

    hasOutputvariable(): boolean;
    clearOutputvariable(): void;
    getOutputvariable(): OutputVariable | undefined;
    setOutputvariable(value?: OutputVariable): Node;

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
        resource?: Resource.AsObject,
        localvariable?: LocalVariable.AsObject,
        configvariable?: ConfigVariable.AsObject,
        outputvariable?: OutputVariable.AsObject,
    }

    export enum ValueCase {
        VALUE_NOT_SET = 0,
        RESOURCE = 1,
        LOCALVARIABLE = 2,
        CONFIGVARIABLE = 3,
        OUTPUTVARIABLE = 4,
    }

}

export class Resource extends jspb.Message { 
    getName(): string;
    setName(value: string): Resource;
    getLogicalname(): string;
    setLogicalname(value: string): Resource;
    getToken(): string;
    setToken(value: string): Resource;
    clearInputsList(): void;
    getInputsList(): Array<ResourceInput>;
    setInputsList(value: Array<ResourceInput>): Resource;
    addInputs(value?: ResourceInput, index?: number): ResourceInput;

    hasOptions(): boolean;
    clearOptions(): void;
    getOptions(): ResourceOptions | undefined;
    setOptions(value?: ResourceOptions): Resource;

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
        name: string,
        logicalname: string,
        token: string,
        inputsList: Array<ResourceInput.AsObject>,
        options?: ResourceOptions.AsObject,
    }
}

export class ResourceInput extends jspb.Message { 
    getName(): string;
    setName(value: string): ResourceInput;

    hasValue(): boolean;
    clearValue(): void;
    getValue(): Expression | undefined;
    setValue(value?: Expression): ResourceInput;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceInput.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceInput): ResourceInput.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceInput, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceInput;
    static deserializeBinaryFromReader(message: ResourceInput, reader: jspb.BinaryReader): ResourceInput;
}

export namespace ResourceInput {
    export type AsObject = {
        name: string,
        value?: Expression.AsObject,
    }
}

export class ResourceOptions extends jspb.Message { 

    hasDependson(): boolean;
    clearDependson(): void;
    getDependson(): Expression | undefined;
    setDependson(value?: Expression): ResourceOptions;

    hasProtect(): boolean;
    clearProtect(): void;
    getProtect(): Expression | undefined;
    setProtect(value?: Expression): ResourceOptions;

    hasParent(): boolean;
    clearParent(): void;
    getParent(): Expression | undefined;
    setParent(value?: Expression): ResourceOptions;

    hasIgnorechanges(): boolean;
    clearIgnorechanges(): void;
    getIgnorechanges(): Expression | undefined;
    setIgnorechanges(value?: Expression): ResourceOptions;

    hasProvider(): boolean;
    clearProvider(): void;
    getProvider(): Expression | undefined;
    setProvider(value?: Expression): ResourceOptions;

    hasVersion(): boolean;
    clearVersion(): void;
    getVersion(): Expression | undefined;
    setVersion(value?: Expression): ResourceOptions;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceOptions.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceOptions): ResourceOptions.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceOptions, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceOptions;
    static deserializeBinaryFromReader(message: ResourceOptions, reader: jspb.BinaryReader): ResourceOptions;
}

export namespace ResourceOptions {
    export type AsObject = {
        dependson?: Expression.AsObject,
        protect?: Expression.AsObject,
        parent?: Expression.AsObject,
        ignorechanges?: Expression.AsObject,
        provider?: Expression.AsObject,
        version?: Expression.AsObject,
    }
}

export class LocalVariable extends jspb.Message { 
    getName(): string;
    setName(value: string): LocalVariable;
    getLogicalname(): string;
    setLogicalname(value: string): LocalVariable;

    hasValue(): boolean;
    clearValue(): void;
    getValue(): Expression | undefined;
    setValue(value?: Expression): LocalVariable;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): LocalVariable.AsObject;
    static toObject(includeInstance: boolean, msg: LocalVariable): LocalVariable.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: LocalVariable, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): LocalVariable;
    static deserializeBinaryFromReader(message: LocalVariable, reader: jspb.BinaryReader): LocalVariable;
}

export namespace LocalVariable {
    export type AsObject = {
        name: string,
        logicalname: string,
        value?: Expression.AsObject,
    }
}

export class ConfigVariable extends jspb.Message { 
    getName(): string;
    setName(value: string): ConfigVariable;
    getLogicalname(): string;
    setLogicalname(value: string): ConfigVariable;
    getConfigtype(): ConfigType;
    setConfigtype(value: ConfigType): ConfigVariable;

    hasDefaultvalue(): boolean;
    clearDefaultvalue(): void;
    getDefaultvalue(): Expression | undefined;
    setDefaultvalue(value?: Expression): ConfigVariable;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConfigVariable.AsObject;
    static toObject(includeInstance: boolean, msg: ConfigVariable): ConfigVariable.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConfigVariable, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConfigVariable;
    static deserializeBinaryFromReader(message: ConfigVariable, reader: jspb.BinaryReader): ConfigVariable;
}

export namespace ConfigVariable {
    export type AsObject = {
        name: string,
        logicalname: string,
        configtype: ConfigType,
        defaultvalue?: Expression.AsObject,
    }
}

export class OutputVariable extends jspb.Message { 
    getName(): string;
    setName(value: string): OutputVariable;
    getLogicalname(): string;
    setLogicalname(value: string): OutputVariable;

    hasValue(): boolean;
    clearValue(): void;
    getValue(): Expression | undefined;
    setValue(value?: Expression): OutputVariable;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): OutputVariable.AsObject;
    static toObject(includeInstance: boolean, msg: OutputVariable): OutputVariable.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: OutputVariable, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): OutputVariable;
    static deserializeBinaryFromReader(message: OutputVariable, reader: jspb.BinaryReader): OutputVariable;
}

export namespace OutputVariable {
    export type AsObject = {
        name: string,
        logicalname: string,
        value?: Expression.AsObject,
    }
}

export class Expression extends jspb.Message { 

    hasLiteralvalueexpression(): boolean;
    clearLiteralvalueexpression(): void;
    getLiteralvalueexpression(): LiteralValueExpression | undefined;
    setLiteralvalueexpression(value?: LiteralValueExpression): Expression;

    hasTemplateexpression(): boolean;
    clearTemplateexpression(): void;
    getTemplateexpression(): TemplateExpression | undefined;
    setTemplateexpression(value?: TemplateExpression): Expression;

    hasIndexexpression(): boolean;
    clearIndexexpression(): void;
    getIndexexpression(): IndexExpression | undefined;
    setIndexexpression(value?: IndexExpression): Expression;

    hasObjectconsexpression(): boolean;
    clearObjectconsexpression(): void;
    getObjectconsexpression(): ObjectConsExpression | undefined;
    setObjectconsexpression(value?: ObjectConsExpression): Expression;

    hasTupleconsexpression(): boolean;
    clearTupleconsexpression(): void;
    getTupleconsexpression(): TupleConsExpression | undefined;
    setTupleconsexpression(value?: TupleConsExpression): Expression;

    hasFunctioncallexpression(): boolean;
    clearFunctioncallexpression(): void;
    getFunctioncallexpression(): FunctionCallExpression | undefined;
    setFunctioncallexpression(value?: FunctionCallExpression): Expression;

    hasRelativetraversalexpression(): boolean;
    clearRelativetraversalexpression(): void;
    getRelativetraversalexpression(): RelativeTraversalExpression | undefined;
    setRelativetraversalexpression(value?: RelativeTraversalExpression): Expression;

    hasScopetraversalexpression(): boolean;
    clearScopetraversalexpression(): void;
    getScopetraversalexpression(): ScopeTraversalExpression | undefined;
    setScopetraversalexpression(value?: ScopeTraversalExpression): Expression;

    hasAnonymousfunctionexpression(): boolean;
    clearAnonymousfunctionexpression(): void;
    getAnonymousfunctionexpression(): AnonymousFunctionExpression | undefined;
    setAnonymousfunctionexpression(value?: AnonymousFunctionExpression): Expression;

    hasConditionalexpression(): boolean;
    clearConditionalexpression(): void;
    getConditionalexpression(): ConditionalExpression | undefined;
    setConditionalexpression(value?: ConditionalExpression): Expression;

    hasBinaryopexpression(): boolean;
    clearBinaryopexpression(): void;
    getBinaryopexpression(): BinaryOpExpression | undefined;
    setBinaryopexpression(value?: BinaryOpExpression): Expression;

    hasUnaryopexpression(): boolean;
    clearUnaryopexpression(): void;
    getUnaryopexpression(): UnaryOpExpression | undefined;
    setUnaryopexpression(value?: UnaryOpExpression): Expression;

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
        literalvalueexpression?: LiteralValueExpression.AsObject,
        templateexpression?: TemplateExpression.AsObject,
        indexexpression?: IndexExpression.AsObject,
        objectconsexpression?: ObjectConsExpression.AsObject,
        tupleconsexpression?: TupleConsExpression.AsObject,
        functioncallexpression?: FunctionCallExpression.AsObject,
        relativetraversalexpression?: RelativeTraversalExpression.AsObject,
        scopetraversalexpression?: ScopeTraversalExpression.AsObject,
        anonymousfunctionexpression?: AnonymousFunctionExpression.AsObject,
        conditionalexpression?: ConditionalExpression.AsObject,
        binaryopexpression?: BinaryOpExpression.AsObject,
        unaryopexpression?: UnaryOpExpression.AsObject,
    }

    export enum ValueCase {
        VALUE_NOT_SET = 0,
        LITERALVALUEEXPRESSION = 1,
        TEMPLATEEXPRESSION = 2,
        INDEXEXPRESSION = 3,
        OBJECTCONSEXPRESSION = 4,
        TUPLECONSEXPRESSION = 5,
        FUNCTIONCALLEXPRESSION = 6,
        RELATIVETRAVERSALEXPRESSION = 7,
        SCOPETRAVERSALEXPRESSION = 8,
        ANONYMOUSFUNCTIONEXPRESSION = 9,
        CONDITIONALEXPRESSION = 10,
        BINARYOPEXPRESSION = 11,
        UNARYOPEXPRESSION = 12,
    }

}

export class LiteralValueExpression extends jspb.Message { 

    hasUnknownvalue(): boolean;
    clearUnknownvalue(): void;
    getUnknownvalue(): boolean;
    setUnknownvalue(value: boolean): LiteralValueExpression;

    hasStringvalue(): boolean;
    clearStringvalue(): void;
    getStringvalue(): string;
    setStringvalue(value: string): LiteralValueExpression;

    hasNumbervalue(): boolean;
    clearNumbervalue(): void;
    getNumbervalue(): number;
    setNumbervalue(value: number): LiteralValueExpression;

    hasBoolvalue(): boolean;
    clearBoolvalue(): void;
    getBoolvalue(): boolean;
    setBoolvalue(value: boolean): LiteralValueExpression;

    getValueCase(): LiteralValueExpression.ValueCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): LiteralValueExpression.AsObject;
    static toObject(includeInstance: boolean, msg: LiteralValueExpression): LiteralValueExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: LiteralValueExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): LiteralValueExpression;
    static deserializeBinaryFromReader(message: LiteralValueExpression, reader: jspb.BinaryReader): LiteralValueExpression;
}

export namespace LiteralValueExpression {
    export type AsObject = {
        unknownvalue: boolean,
        stringvalue: string,
        numbervalue: number,
        boolvalue: boolean,
    }

    export enum ValueCase {
        VALUE_NOT_SET = 0,
        UNKNOWNVALUE = 1,
        STRINGVALUE = 2,
        NUMBERVALUE = 3,
        BOOLVALUE = 4,
    }

}

export class TemplateExpression extends jspb.Message { 
    clearPartsList(): void;
    getPartsList(): Array<Expression>;
    setPartsList(value: Array<Expression>): TemplateExpression;
    addParts(value?: Expression, index?: number): Expression;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TemplateExpression.AsObject;
    static toObject(includeInstance: boolean, msg: TemplateExpression): TemplateExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TemplateExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TemplateExpression;
    static deserializeBinaryFromReader(message: TemplateExpression, reader: jspb.BinaryReader): TemplateExpression;
}

export namespace TemplateExpression {
    export type AsObject = {
        partsList: Array<Expression.AsObject>,
    }
}

export class IndexExpression extends jspb.Message { 

    hasCollection(): boolean;
    clearCollection(): void;
    getCollection(): Expression | undefined;
    setCollection(value?: Expression): IndexExpression;

    hasKey(): boolean;
    clearKey(): void;
    getKey(): Expression | undefined;
    setKey(value?: Expression): IndexExpression;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): IndexExpression.AsObject;
    static toObject(includeInstance: boolean, msg: IndexExpression): IndexExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: IndexExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): IndexExpression;
    static deserializeBinaryFromReader(message: IndexExpression, reader: jspb.BinaryReader): IndexExpression;
}

export namespace IndexExpression {
    export type AsObject = {
        collection?: Expression.AsObject,
        key?: Expression.AsObject,
    }
}

export class ObjectConsExpression extends jspb.Message { 

    getPropertiesMap(): jspb.Map<string, Expression>;
    clearPropertiesMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ObjectConsExpression.AsObject;
    static toObject(includeInstance: boolean, msg: ObjectConsExpression): ObjectConsExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ObjectConsExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ObjectConsExpression;
    static deserializeBinaryFromReader(message: ObjectConsExpression, reader: jspb.BinaryReader): ObjectConsExpression;
}

export namespace ObjectConsExpression {
    export type AsObject = {

        propertiesMap: Array<[string, Expression.AsObject]>,
    }
}

export class TupleConsExpression extends jspb.Message { 
    clearItemsList(): void;
    getItemsList(): Array<Expression>;
    setItemsList(value: Array<Expression>): TupleConsExpression;
    addItems(value?: Expression, index?: number): Expression;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TupleConsExpression.AsObject;
    static toObject(includeInstance: boolean, msg: TupleConsExpression): TupleConsExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TupleConsExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TupleConsExpression;
    static deserializeBinaryFromReader(message: TupleConsExpression, reader: jspb.BinaryReader): TupleConsExpression;
}

export namespace TupleConsExpression {
    export type AsObject = {
        itemsList: Array<Expression.AsObject>,
    }
}

export class FunctionCallExpression extends jspb.Message { 
    getName(): string;
    setName(value: string): FunctionCallExpression;
    clearArgsList(): void;
    getArgsList(): Array<Expression>;
    setArgsList(value: Array<Expression>): FunctionCallExpression;
    addArgs(value?: Expression, index?: number): Expression;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): FunctionCallExpression.AsObject;
    static toObject(includeInstance: boolean, msg: FunctionCallExpression): FunctionCallExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: FunctionCallExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): FunctionCallExpression;
    static deserializeBinaryFromReader(message: FunctionCallExpression, reader: jspb.BinaryReader): FunctionCallExpression;
}

export namespace FunctionCallExpression {
    export type AsObject = {
        name: string,
        argsList: Array<Expression.AsObject>,
    }
}

export class RelativeTraversalExpression extends jspb.Message { 

    hasSource(): boolean;
    clearSource(): void;
    getSource(): Expression | undefined;
    setSource(value?: Expression): RelativeTraversalExpression;

    hasTraversal(): boolean;
    clearTraversal(): void;
    getTraversal(): Traversal | undefined;
    setTraversal(value?: Traversal): RelativeTraversalExpression;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RelativeTraversalExpression.AsObject;
    static toObject(includeInstance: boolean, msg: RelativeTraversalExpression): RelativeTraversalExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RelativeTraversalExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RelativeTraversalExpression;
    static deserializeBinaryFromReader(message: RelativeTraversalExpression, reader: jspb.BinaryReader): RelativeTraversalExpression;
}

export namespace RelativeTraversalExpression {
    export type AsObject = {
        source?: Expression.AsObject,
        traversal?: Traversal.AsObject,
    }
}

export class ScopeTraversalExpression extends jspb.Message { 
    getRootname(): string;
    setRootname(value: string): ScopeTraversalExpression;

    hasTraversal(): boolean;
    clearTraversal(): void;
    getTraversal(): Traversal | undefined;
    setTraversal(value?: Traversal): ScopeTraversalExpression;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ScopeTraversalExpression.AsObject;
    static toObject(includeInstance: boolean, msg: ScopeTraversalExpression): ScopeTraversalExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ScopeTraversalExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ScopeTraversalExpression;
    static deserializeBinaryFromReader(message: ScopeTraversalExpression, reader: jspb.BinaryReader): ScopeTraversalExpression;
}

export namespace ScopeTraversalExpression {
    export type AsObject = {
        rootname: string,
        traversal?: Traversal.AsObject,
    }
}

export class AnonymousFunctionExpression extends jspb.Message { 

    hasBody(): boolean;
    clearBody(): void;
    getBody(): Expression | undefined;
    setBody(value?: Expression): AnonymousFunctionExpression;
    clearParametersList(): void;
    getParametersList(): Array<string>;
    setParametersList(value: Array<string>): AnonymousFunctionExpression;
    addParameters(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AnonymousFunctionExpression.AsObject;
    static toObject(includeInstance: boolean, msg: AnonymousFunctionExpression): AnonymousFunctionExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AnonymousFunctionExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AnonymousFunctionExpression;
    static deserializeBinaryFromReader(message: AnonymousFunctionExpression, reader: jspb.BinaryReader): AnonymousFunctionExpression;
}

export namespace AnonymousFunctionExpression {
    export type AsObject = {
        body?: Expression.AsObject,
        parametersList: Array<string>,
    }
}

export class ConditionalExpression extends jspb.Message { 

    hasCondition(): boolean;
    clearCondition(): void;
    getCondition(): Expression | undefined;
    setCondition(value?: Expression): ConditionalExpression;

    hasTrueexpr(): boolean;
    clearTrueexpr(): void;
    getTrueexpr(): Expression | undefined;
    setTrueexpr(value?: Expression): ConditionalExpression;

    hasFalseexpr(): boolean;
    clearFalseexpr(): void;
    getFalseexpr(): Expression | undefined;
    setFalseexpr(value?: Expression): ConditionalExpression;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConditionalExpression.AsObject;
    static toObject(includeInstance: boolean, msg: ConditionalExpression): ConditionalExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConditionalExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConditionalExpression;
    static deserializeBinaryFromReader(message: ConditionalExpression, reader: jspb.BinaryReader): ConditionalExpression;
}

export namespace ConditionalExpression {
    export type AsObject = {
        condition?: Expression.AsObject,
        trueexpr?: Expression.AsObject,
        falseexpr?: Expression.AsObject,
    }
}

export class BinaryOpExpression extends jspb.Message { 
    getOperation(): Operation;
    setOperation(value: Operation): BinaryOpExpression;

    hasLeft(): boolean;
    clearLeft(): void;
    getLeft(): Expression | undefined;
    setLeft(value?: Expression): BinaryOpExpression;

    hasRight(): boolean;
    clearRight(): void;
    getRight(): Expression | undefined;
    setRight(value?: Expression): BinaryOpExpression;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): BinaryOpExpression.AsObject;
    static toObject(includeInstance: boolean, msg: BinaryOpExpression): BinaryOpExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: BinaryOpExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): BinaryOpExpression;
    static deserializeBinaryFromReader(message: BinaryOpExpression, reader: jspb.BinaryReader): BinaryOpExpression;
}

export namespace BinaryOpExpression {
    export type AsObject = {
        operation: Operation,
        left?: Expression.AsObject,
        right?: Expression.AsObject,
    }
}

export class UnaryOpExpression extends jspb.Message { 
    getOperation(): Operation;
    setOperation(value: Operation): UnaryOpExpression;

    hasOperand(): boolean;
    clearOperand(): void;
    getOperand(): Expression | undefined;
    setOperand(value?: Expression): UnaryOpExpression;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UnaryOpExpression.AsObject;
    static toObject(includeInstance: boolean, msg: UnaryOpExpression): UnaryOpExpression.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UnaryOpExpression, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UnaryOpExpression;
    static deserializeBinaryFromReader(message: UnaryOpExpression, reader: jspb.BinaryReader): UnaryOpExpression;
}

export namespace UnaryOpExpression {
    export type AsObject = {
        operation: Operation,
        operand?: Expression.AsObject,
    }
}

export class Traversal extends jspb.Message { 
    clearEachList(): void;
    getEachList(): Array<Traverser>;
    setEachList(value: Array<Traverser>): Traversal;
    addEach(value?: Traverser, index?: number): Traverser;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Traversal.AsObject;
    static toObject(includeInstance: boolean, msg: Traversal): Traversal.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Traversal, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Traversal;
    static deserializeBinaryFromReader(message: Traversal, reader: jspb.BinaryReader): Traversal;
}

export namespace Traversal {
    export type AsObject = {
        eachList: Array<Traverser.AsObject>,
    }
}

export class Traverser extends jspb.Message { 

    hasTraverseattr(): boolean;
    clearTraverseattr(): void;
    getTraverseattr(): TraverseAttr | undefined;
    setTraverseattr(value?: TraverseAttr): Traverser;

    hasTraverseindex(): boolean;
    clearTraverseindex(): void;
    getTraverseindex(): TraverseIndex | undefined;
    setTraverseindex(value?: TraverseIndex): Traverser;

    hasTraverseroot(): boolean;
    clearTraverseroot(): void;
    getTraverseroot(): TraverseRoot | undefined;
    setTraverseroot(value?: TraverseRoot): Traverser;

    hasTraversesplat(): boolean;
    clearTraversesplat(): void;
    getTraversesplat(): TraverseSplat | undefined;
    setTraversesplat(value?: TraverseSplat): Traverser;

    getValueCase(): Traverser.ValueCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Traverser.AsObject;
    static toObject(includeInstance: boolean, msg: Traverser): Traverser.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Traverser, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Traverser;
    static deserializeBinaryFromReader(message: Traverser, reader: jspb.BinaryReader): Traverser;
}

export namespace Traverser {
    export type AsObject = {
        traverseattr?: TraverseAttr.AsObject,
        traverseindex?: TraverseIndex.AsObject,
        traverseroot?: TraverseRoot.AsObject,
        traversesplat?: TraverseSplat.AsObject,
    }

    export enum ValueCase {
        VALUE_NOT_SET = 0,
        TRAVERSEATTR = 1,
        TRAVERSEINDEX = 2,
        TRAVERSEROOT = 3,
        TRAVERSESPLAT = 4,
    }

}

export class TraverseAttr extends jspb.Message { 
    getName(): string;
    setName(value: string): TraverseAttr;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TraverseAttr.AsObject;
    static toObject(includeInstance: boolean, msg: TraverseAttr): TraverseAttr.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TraverseAttr, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TraverseAttr;
    static deserializeBinaryFromReader(message: TraverseAttr, reader: jspb.BinaryReader): TraverseAttr;
}

export namespace TraverseAttr {
    export type AsObject = {
        name: string,
    }
}

export class TraverseIndex extends jspb.Message { 
    getIndex(): number;
    setIndex(value: number): TraverseIndex;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TraverseIndex.AsObject;
    static toObject(includeInstance: boolean, msg: TraverseIndex): TraverseIndex.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TraverseIndex, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TraverseIndex;
    static deserializeBinaryFromReader(message: TraverseIndex, reader: jspb.BinaryReader): TraverseIndex;
}

export namespace TraverseIndex {
    export type AsObject = {
        index: number,
    }
}

export class TraverseRoot extends jspb.Message { 
    getName(): string;
    setName(value: string): TraverseRoot;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TraverseRoot.AsObject;
    static toObject(includeInstance: boolean, msg: TraverseRoot): TraverseRoot.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TraverseRoot, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TraverseRoot;
    static deserializeBinaryFromReader(message: TraverseRoot, reader: jspb.BinaryReader): TraverseRoot;
}

export namespace TraverseRoot {
    export type AsObject = {
        name: string,
    }
}

export class TraverseSplat extends jspb.Message { 

    hasEach(): boolean;
    clearEach(): void;
    getEach(): Traversal | undefined;
    setEach(value?: Traversal): TraverseSplat;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TraverseSplat.AsObject;
    static toObject(includeInstance: boolean, msg: TraverseSplat): TraverseSplat.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TraverseSplat, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TraverseSplat;
    static deserializeBinaryFromReader(message: TraverseSplat, reader: jspb.BinaryReader): TraverseSplat;
}

export namespace TraverseSplat {
    export type AsObject = {
        each?: Traversal.AsObject,
    }
}

export enum ConfigType {
    STRING = 0,
    NUMBER = 1,
    INT = 2,
    BOOL = 3,
}

export enum Operation {
    ADD = 0,
    DIVIDE = 1,
    EQUAL = 2,
    GREATER_THAN = 3,
    GREATER_THAN_OR_EQUAL = 4,
    LESS_THAN = 5,
    LESS_THAN_OR_EQUAL = 6,
    LOGICAL_AND = 7,
    LOGICAL_OR = 8,
    MODULO = 9,
    MULTIPLY = 10,
    NOT_EQUAL = 11,
    SUBTRACT = 12,
}
