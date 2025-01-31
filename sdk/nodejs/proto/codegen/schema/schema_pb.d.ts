// package: pulumirpc.codegen
// file: pulumi/codegen/schema/schema.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class PackageInfo extends jspb.Message { 
    getName(): string;
    setName(value: string): PackageInfo;

    hasDisplayName(): boolean;
    clearDisplayName(): void;
    getDisplayName(): string | undefined;
    setDisplayName(value: string): PackageInfo;

    hasVersion(): boolean;
    clearVersion(): void;
    getVersion(): string | undefined;
    setVersion(value: string): PackageInfo;

    hasDescription(): boolean;
    clearDescription(): void;
    getDescription(): string | undefined;
    setDescription(value: string): PackageInfo;
    clearKeywordsList(): void;
    getKeywordsList(): Array<string>;
    setKeywordsList(value: Array<string>): PackageInfo;
    addKeywords(value: string, index?: number): string;

    hasHomepage(): boolean;
    clearHomepage(): void;
    getHomepage(): string | undefined;
    setHomepage(value: string): PackageInfo;

    hasLicense(): boolean;
    clearLicense(): void;
    getLicense(): string | undefined;
    setLicense(value: string): PackageInfo;

    hasAttribution(): boolean;
    clearAttribution(): void;
    getAttribution(): string | undefined;
    setAttribution(value: string): PackageInfo;

    hasRepository(): boolean;
    clearRepository(): void;
    getRepository(): string | undefined;
    setRepository(value: string): PackageInfo;

    hasLogoUrl(): boolean;
    clearLogoUrl(): void;
    getLogoUrl(): string | undefined;
    setLogoUrl(value: string): PackageInfo;

    hasPluginDownloadUrl(): boolean;
    clearPluginDownloadUrl(): void;
    getPluginDownloadUrl(): string | undefined;
    setPluginDownloadUrl(value: string): PackageInfo;

    hasPublisher(): boolean;
    clearPublisher(): void;
    getPublisher(): string | undefined;
    setPublisher(value: string): PackageInfo;

    hasMeta(): boolean;
    clearMeta(): void;
    getMeta(): Meta | undefined;
    setMeta(value?: Meta): PackageInfo;

    hasProvider(): boolean;
    clearProvider(): void;
    getProvider(): ResourceSpec | undefined;
    setProvider(value?: ResourceSpec): PackageInfo;

    getLanguageMap(): jspb.Map<string, Uint8Array | string>;
    clearLanguageMap(): void;

    hasParameterization(): boolean;
    clearParameterization(): void;
    getParameterization(): Parameterization | undefined;
    setParameterization(value?: Parameterization): PackageInfo;
    clearAllowedPackageNamesList(): void;
    getAllowedPackageNamesList(): Array<string>;
    setAllowedPackageNamesList(value: Array<string>): PackageInfo;
    addAllowedPackageNames(value: string, index?: number): string;

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
        displayName?: string,
        version?: string,
        description?: string,
        keywordsList: Array<string>,
        homepage?: string,
        license?: string,
        attribution?: string,
        repository?: string,
        logoUrl?: string,
        pluginDownloadUrl?: string,
        publisher?: string,
        meta?: Meta.AsObject,
        provider?: ResourceSpec.AsObject,

        languageMap: Array<[string, Uint8Array | string]>,
        parameterization?: Parameterization.AsObject,
        allowedPackageNamesList: Array<string>,
    }
}

export class Parameterization extends jspb.Message { 

    hasBaseProvider(): boolean;
    clearBaseProvider(): void;
    getBaseProvider(): BaseProvider | undefined;
    setBaseProvider(value?: BaseProvider): Parameterization;

    hasParameter(): boolean;
    clearParameter(): void;
    getParameter(): Uint8Array | string;
    getParameter_asU8(): Uint8Array;
    getParameter_asB64(): string;
    setParameter(value: Uint8Array | string): Parameterization;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Parameterization.AsObject;
    static toObject(includeInstance: boolean, msg: Parameterization): Parameterization.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Parameterization, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Parameterization;
    static deserializeBinaryFromReader(message: Parameterization, reader: jspb.BinaryReader): Parameterization;
}

export namespace Parameterization {
    export type AsObject = {
        baseProvider?: BaseProvider.AsObject,
        parameter: Uint8Array | string,
    }
}

export class BaseProvider extends jspb.Message { 
    getName(): string;
    setName(value: string): BaseProvider;
    getVersion(): string;
    setVersion(value: string): BaseProvider;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): BaseProvider.AsObject;
    static toObject(includeInstance: boolean, msg: BaseProvider): BaseProvider.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: BaseProvider, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): BaseProvider;
    static deserializeBinaryFromReader(message: BaseProvider, reader: jspb.BinaryReader): BaseProvider;
}

export namespace BaseProvider {
    export type AsObject = {
        name: string,
        version: string,
    }
}

export class Meta extends jspb.Message { 

    hasModuleFormat(): boolean;
    clearModuleFormat(): void;
    getModuleFormat(): string | undefined;
    setModuleFormat(value: string): Meta;

    hasSupportPack(): boolean;
    clearSupportPack(): void;
    getSupportPack(): boolean | undefined;
    setSupportPack(value: boolean): Meta;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Meta.AsObject;
    static toObject(includeInstance: boolean, msg: Meta): Meta.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Meta, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Meta;
    static deserializeBinaryFromReader(message: Meta, reader: jspb.BinaryReader): Meta;
}

export namespace Meta {
    export type AsObject = {
        moduleFormat?: string,
        supportPack?: boolean,
    }
}

export class TypeSpec extends jspb.Message { 

    hasPlain(): boolean;
    clearPlain(): void;
    getPlain(): boolean | undefined;
    setPlain(value: boolean): TypeSpec;

    hasPrimitiveType(): boolean;
    clearPrimitiveType(): void;
    getPrimitiveType(): string;
    setPrimitiveType(value: string): TypeSpec;

    hasArrayType(): boolean;
    clearArrayType(): void;
    getArrayType(): TypeList | undefined;
    setArrayType(value?: TypeList): TypeSpec;

    hasMapType(): boolean;
    clearMapType(): void;
    getMapType(): TypeMap | undefined;
    setMapType(value?: TypeMap): TypeSpec;

    hasRef(): boolean;
    clearRef(): void;
    getRef(): string;
    setRef(value: string): TypeSpec;

    hasUnion(): boolean;
    clearUnion(): void;
    getUnion(): UnionType | undefined;
    setUnion(value?: UnionType): TypeSpec;

    getTypeCase(): TypeSpec.TypeCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TypeSpec.AsObject;
    static toObject(includeInstance: boolean, msg: TypeSpec): TypeSpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TypeSpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TypeSpec;
    static deserializeBinaryFromReader(message: TypeSpec, reader: jspb.BinaryReader): TypeSpec;
}

export namespace TypeSpec {
    export type AsObject = {
        plain?: boolean,
        primitiveType: string,
        arrayType?: TypeList.AsObject,
        mapType?: TypeMap.AsObject,
        ref: string,
        union?: UnionType.AsObject,
    }

    export enum TypeCase {
        TYPE_NOT_SET = 0,
        PRIMITIVE_TYPE = 2,
        ARRAY_TYPE = 3,
        MAP_TYPE = 4,
        REF = 5,
        UNION = 6,
    }

}

export class TypeList extends jspb.Message { 
    clearTypeList(): void;
    getTypeList(): Array<TypeSpec>;
    setTypeList(value: Array<TypeSpec>): TypeList;
    addType(value?: TypeSpec, index?: number): TypeSpec;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TypeList.AsObject;
    static toObject(includeInstance: boolean, msg: TypeList): TypeList.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TypeList, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TypeList;
    static deserializeBinaryFromReader(message: TypeList, reader: jspb.BinaryReader): TypeList;
}

export namespace TypeList {
    export type AsObject = {
        typeList: Array<TypeSpec.AsObject>,
    }
}

export class TypeMap extends jspb.Message { 

    getTypeMap(): jspb.Map<string, TypeSpec>;
    clearTypeMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TypeMap.AsObject;
    static toObject(includeInstance: boolean, msg: TypeMap): TypeMap.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TypeMap, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TypeMap;
    static deserializeBinaryFromReader(message: TypeMap, reader: jspb.BinaryReader): TypeMap;
}

export namespace TypeMap {
    export type AsObject = {

        typeMap: Array<[string, TypeSpec.AsObject]>,
    }
}

export class PropertySpec extends jspb.Message { 

    hasTypeSpec(): boolean;
    clearTypeSpec(): void;
    getTypeSpec(): TypeSpec | undefined;
    setTypeSpec(value?: TypeSpec): PropertySpec;

    hasDescription(): boolean;
    clearDescription(): void;
    getDescription(): string | undefined;
    setDescription(value: string): PropertySpec;

    hasConst(): boolean;
    clearConst(): void;
    getConst(): Value | undefined;
    setConst(value?: Value): PropertySpec;

    hasDefault(): boolean;
    clearDefault(): void;
    getDefault(): Value | undefined;
    setDefault(value?: Value): PropertySpec;

    hasDefaultInfo(): boolean;
    clearDefaultInfo(): void;
    getDefaultInfo(): DefaultInfo | undefined;
    setDefaultInfo(value?: DefaultInfo): PropertySpec;

    hasDeprecationMessage(): boolean;
    clearDeprecationMessage(): void;
    getDeprecationMessage(): string | undefined;
    setDeprecationMessage(value: string): PropertySpec;

    getLanguageMap(): jspb.Map<string, string>;
    clearLanguageMap(): void;

    hasSecret(): boolean;
    clearSecret(): void;
    getSecret(): boolean | undefined;
    setSecret(value: boolean): PropertySpec;

    hasReplaceOnChanges(): boolean;
    clearReplaceOnChanges(): void;
    getReplaceOnChanges(): boolean | undefined;
    setReplaceOnChanges(value: boolean): PropertySpec;

    hasWillReplaceOnChanges(): boolean;
    clearWillReplaceOnChanges(): void;
    getWillReplaceOnChanges(): boolean | undefined;
    setWillReplaceOnChanges(value: boolean): PropertySpec;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PropertySpec.AsObject;
    static toObject(includeInstance: boolean, msg: PropertySpec): PropertySpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PropertySpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PropertySpec;
    static deserializeBinaryFromReader(message: PropertySpec, reader: jspb.BinaryReader): PropertySpec;
}

export namespace PropertySpec {
    export type AsObject = {
        typeSpec?: TypeSpec.AsObject,
        description?: string,
        pb_const?: Value.AsObject,
        pb_default?: Value.AsObject,
        defaultInfo?: DefaultInfo.AsObject,
        deprecationMessage?: string,

        languageMap: Array<[string, string]>,
        secret?: boolean,
        replaceOnChanges?: boolean,
        willReplaceOnChanges?: boolean,
    }
}

export class ObjectTypeSpec extends jspb.Message { 

    hasDescription(): boolean;
    clearDescription(): void;
    getDescription(): string | undefined;
    setDescription(value: string): ObjectTypeSpec;

    getPropertiesMap(): jspb.Map<string, PropertySpec>;
    clearPropertiesMap(): void;

    hasType(): boolean;
    clearType(): void;
    getType(): string | undefined;
    setType(value: string): ObjectTypeSpec;
    clearRequiredList(): void;
    getRequiredList(): Array<string>;
    setRequiredList(value: Array<string>): ObjectTypeSpec;
    addRequired(value: string, index?: number): string;

    getLanguageMap(): jspb.Map<string, string>;
    clearLanguageMap(): void;

    hasIsOverlay(): boolean;
    clearIsOverlay(): void;
    getIsOverlay(): boolean | undefined;
    setIsOverlay(value: boolean): ObjectTypeSpec;
    clearOverlaySupportedLanguagesList(): void;
    getOverlaySupportedLanguagesList(): Array<string>;
    setOverlaySupportedLanguagesList(value: Array<string>): ObjectTypeSpec;
    addOverlaySupportedLanguages(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ObjectTypeSpec.AsObject;
    static toObject(includeInstance: boolean, msg: ObjectTypeSpec): ObjectTypeSpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ObjectTypeSpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ObjectTypeSpec;
    static deserializeBinaryFromReader(message: ObjectTypeSpec, reader: jspb.BinaryReader): ObjectTypeSpec;
}

export namespace ObjectTypeSpec {
    export type AsObject = {
        description?: string,

        propertiesMap: Array<[string, PropertySpec.AsObject]>,
        type?: string,
        requiredList: Array<string>,

        languageMap: Array<[string, string]>,
        isOverlay?: boolean,
        overlaySupportedLanguagesList: Array<string>,
    }
}

export class ResourceSpec extends jspb.Message { 

    hasObjectTypeSpec(): boolean;
    clearObjectTypeSpec(): void;
    getObjectTypeSpec(): ObjectTypeSpec | undefined;
    setObjectTypeSpec(value?: ObjectTypeSpec): ResourceSpec;
    clearRequiredInputsList(): void;
    getRequiredInputsList(): Array<string>;
    setRequiredInputsList(value: Array<string>): ResourceSpec;
    addRequiredInputs(value: string, index?: number): string;

    hasStateInputs(): boolean;
    clearStateInputs(): void;
    getStateInputs(): ObjectTypeSpec | undefined;
    setStateInputs(value?: ObjectTypeSpec): ResourceSpec;
    clearAliasesList(): void;
    getAliasesList(): Array<Alias>;
    setAliasesList(value: Array<Alias>): ResourceSpec;
    addAliases(value?: Alias, index?: number): Alias;

    hasDeprecationMessage(): boolean;
    clearDeprecationMessage(): void;
    getDeprecationMessage(): string | undefined;
    setDeprecationMessage(value: string): ResourceSpec;

    hasIsComponent(): boolean;
    clearIsComponent(): void;
    getIsComponent(): boolean | undefined;
    setIsComponent(value: boolean): ResourceSpec;

    getMethodsMap(): jspb.Map<string, string>;
    clearMethodsMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceSpec.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceSpec): ResourceSpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceSpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceSpec;
    static deserializeBinaryFromReader(message: ResourceSpec, reader: jspb.BinaryReader): ResourceSpec;
}

export namespace ResourceSpec {
    export type AsObject = {
        objectTypeSpec?: ObjectTypeSpec.AsObject,
        requiredInputsList: Array<string>,
        stateInputs?: ObjectTypeSpec.AsObject,
        aliasesList: Array<Alias.AsObject>,
        deprecationMessage?: string,
        isComponent?: boolean,

        methodsMap: Array<[string, string]>,
    }
}

export class Alias extends jspb.Message { 

    hasName(): boolean;
    clearName(): void;
    getName(): string | undefined;
    setName(value: string): Alias;

    hasProject(): boolean;
    clearProject(): void;
    getProject(): string | undefined;
    setProject(value: string): Alias;

    hasType(): boolean;
    clearType(): void;
    getType(): string | undefined;
    setType(value: string): Alias;

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
        name?: string,
        project?: string,
        type?: string,
    }
}

export class Value extends jspb.Message { 

    hasNumberValue(): boolean;
    clearNumberValue(): void;
    getNumberValue(): number;
    setNumberValue(value: number): Value;

    hasStringValue(): boolean;
    clearStringValue(): void;
    getStringValue(): string;
    setStringValue(value: string): Value;

    hasBoolValue(): boolean;
    clearBoolValue(): void;
    getBoolValue(): boolean;
    setBoolValue(value: boolean): Value;

    getKindCase(): Value.KindCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Value.AsObject;
    static toObject(includeInstance: boolean, msg: Value): Value.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Value, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Value;
    static deserializeBinaryFromReader(message: Value, reader: jspb.BinaryReader): Value;
}

export namespace Value {
    export type AsObject = {
        numberValue: number,
        stringValue: string,
        boolValue: boolean,
    }

    export enum KindCase {
        KIND_NOT_SET = 0,
        NUMBER_VALUE = 1,
        STRING_VALUE = 2,
        BOOL_VALUE = 3,
    }

}

export class UnionType extends jspb.Message { 
    clearOneOfList(): void;
    getOneOfList(): Array<TypeSpec>;
    setOneOfList(value: Array<TypeSpec>): UnionType;
    addOneOf(value?: TypeSpec, index?: number): TypeSpec;

    hasDiscriminator(): boolean;
    clearDiscriminator(): void;
    getDiscriminator(): Discriminator | undefined;
    setDiscriminator(value?: Discriminator): UnionType;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UnionType.AsObject;
    static toObject(includeInstance: boolean, msg: UnionType): UnionType.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UnionType, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UnionType;
    static deserializeBinaryFromReader(message: UnionType, reader: jspb.BinaryReader): UnionType;
}

export namespace UnionType {
    export type AsObject = {
        oneOfList: Array<TypeSpec.AsObject>,
        discriminator?: Discriminator.AsObject,
    }
}

export class Discriminator extends jspb.Message { 
    getPropertyName(): string;
    setPropertyName(value: string): Discriminator;

    getMappingMap(): jspb.Map<string, string>;
    clearMappingMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Discriminator.AsObject;
    static toObject(includeInstance: boolean, msg: Discriminator): Discriminator.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Discriminator, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Discriminator;
    static deserializeBinaryFromReader(message: Discriminator, reader: jspb.BinaryReader): Discriminator;
}

export namespace Discriminator {
    export type AsObject = {
        propertyName: string,

        mappingMap: Array<[string, string]>,
    }
}

export class DefaultInfo extends jspb.Message { 
    clearEnvironmentList(): void;
    getEnvironmentList(): Array<string>;
    setEnvironmentList(value: Array<string>): DefaultInfo;
    addEnvironment(value: string, index?: number): string;

    getLanguageMap(): jspb.Map<string, string>;
    clearLanguageMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DefaultInfo.AsObject;
    static toObject(includeInstance: boolean, msg: DefaultInfo): DefaultInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DefaultInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DefaultInfo;
    static deserializeBinaryFromReader(message: DefaultInfo, reader: jspb.BinaryReader): DefaultInfo;
}

export namespace DefaultInfo {
    export type AsObject = {
        environmentList: Array<string>,

        languageMap: Array<[string, string]>,
    }
}

export enum Language {
    NODEJS = 0,
    PYTHON = 1,
    GO = 2,
    CSHARP = 3,
    JAVA = 4,
    YAML = 5,
}
