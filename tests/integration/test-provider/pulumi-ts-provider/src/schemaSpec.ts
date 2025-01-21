/**
 * A description of the schema for a Pulumi Package
 */
export interface PulumiPackage {
    /**
     * Freeform text attribution of derived work, if required.
     */
    attribution?: string;
    /**
     * The package's configuration variables.
     */
    config?: Config;
    /**
     * The description of the package. Descriptions are interpreted as Markdown.
     */
    description?: string;
    /**
     * The human-friendly name of the package.
     */
    displayName?: string;
    /**
     * A map from token to functionSpec that describes the set of functions defined by this
     * package.
     */
    functions?: { [key: string]: FunctionDefinition };
    /**
     * The package's homepage.
     */
    homepage?: string;
    /**
     * The list of keywords that are associated with the package, if any.
     */
    keywords?: string[];
    /**
     * Additional language-specific data about the package.
     */
    language?: { [key: string]: any };
    /**
     * The name of the license used for the package's contents.
     */
    license?: string;
    /**
     * The URL of the package's logo, if any.
     */
    logoUrl?: string;
    /**
     * Format metadata about this package.
     */
    meta?: Meta;
    /**
     * The unqualified name of the package (e.g. "aws", "azure", "gcp", "kubernetes", "random")
     */
    name: string;
    /**
     * An optional object to define parameterization for the package.
     */
    parameterization?: Parameterization;
    /**
     * The URL to use when downloading the provider plugin binary.
     */
    pluginDownloadURL?: string;
    /**
     * The provider type for this package.
     */
    provider?: ResourceDefinition;
    /**
     * The name of the person or organization that authored and published the package.
     */
    publisher?: string;
    /**
     * The URL at which the package's sources can be found.
     */
    repository?: string;
    /**
     * A map from type token to resourceSpec that describes the set of resources and components
     * defined by this package.
     */
    resources?: { [key: string]: ResourceDefinition };
    /**
     * A map from type token to complexTypeSpec that describes the set of complex types (i.e.
     * object, enum) defined by this package.
     */
    types?: { [key: string]: TypeDefinition };
    /**
     * The version of the package. The version must be valid semver.
     */
    version?: string;
}

/**
 * The package's configuration variables.
 */
export interface Config {
    /**
     * A list of the names of the package's non-required configuration variables.
     */
    defaults?: string[];
    /**
     * A map from variable name to propertySpec that describes a package's configuration
     * variables.
     */
    variables?: { [key: string]: PropertyDefinition };
}

/**
 * Describes an object or resource property
 *
 * A reference to a type. The particular kind of type referenced is determined based on the
 * contents of the "type" property and the presence or absence of the
 * "additionalProperties", "items", "oneOf", and "$ref" properties.
 *
 * A reference to a primitive type. A primitive type must have only the "type" property
 * set.
 *
 * A reference to an array type. The "type" property must be set to "array" and the "items"
 * property must be present. No other properties may be present.
 *
 * A reference to a map type. The "type" property must be set to "object" and the
 * "additionalProperties" property may be present. No other properties may be present.
 *
 * A reference to a type in this or another document. The "$ref" property must be present.
 * The "type" property is ignored if it is present. No other properties may be present.
 *
 * A reference to a union type. The "oneOf" property must be present. The union may
 * additional specify an underlying primitive type via the "type" property and a
 * discriminator via the "discriminator" property. No other properties may be present.
 */
export interface PropertyDefinition {
    /**
     * The constant value for the property, if any. The type of the value must be assignable to
     * the type of the property.
     */
    const?: Const;
    /**
     * The default value for the property, if any. The type of the value must be assignable to
     * the type of the property.
     */
    default?: Const;
    /**
     * Additional information about the property's default value, if any.
     */
    defaultInfo?: DefaultInfo;
    /**
     * Indicates whether the property is deprecated
     */
    deprecationMessage?: string;
    /**
     * The description of the property, if any. Interpreted as Markdown.
     */
    description?: string;
    /**
     * Additional language-specific data about the property.
     */
    language?: { [key: string]: any };
    /**
     * Specifies whether a change to the property causes its containing resource to be replaced
     * instead of updated (default false).
     */
    replaceOnChanges?: boolean;
    /**
     * Specifies whether the property is secret (default false).
     */
    secret?: boolean;
    /**
     * Indicates that the provider will replace the resource when this property is changed.
     */
    willReplaceOnChanges?: boolean;
    /**
     * Indicates that when used as an input, this type does not accept eventual values.
     */
    plain?: boolean;
    /**
     * The primitive type, if any
     *
     * ignored; present for compatibility with existing schemas
     *
     * The underlying primitive type of the union, if any
     */
    type?: string;
    /**
     * The element type of the array
     */
    items?: TypeReference;
    /**
     * The element type of the map. Defaults to "string" when omitted.
     */
    additionalProperties?: TypeReference;
    /**
     * The URI of the referenced type. For example, the built-in Archive, Asset, and Any
     * types are referenced as "pulumi.json#/Archive", "pulumi.json#/Asset", and
     * "pulumi.json#/Any", respectively.
     * A type from this document is referenced as "#/types/pulumi:type:token".
     * A type from another document is referenced as "path#/types/pulumi:type:token", where path
     * is of the form:
     * "/provider/vX.Y.Z/schema.json" or "pulumi.json" or
     * "http[s]://example.com/provider/vX.Y.Z/schema.json"
     * A resource from this document is referenced as "#/resources/pulumi:type:token".
     * A resource from another document is referenced as "path#/resources/pulumi:type:token",
     * where path is of the form:
     * "/provider/vX.Y.Z/schema.json" or "pulumi.json" or
     * "http[s]://example.com/provider/vX.Y.Z/schema.json"
     */
    $ref?: string;
    /**
     * Informs the consumer of an alternative schema based on the value associated with it
     */
    discriminator?: Discriminator;
    /**
     * If present, indicates that values of the type may be one of any of the listed types
     */
    oneOf?: TypeReference[];
    [property: string]: any;
}

/**
 * A reference to a type. The particular kind of type referenced is determined based on the
 * contents of the "type" property and the presence or absence of the
 * "additionalProperties", "items", "oneOf", and "$ref" properties.
 *
 * The element type of the array
 *
 * The element type of the map. Defaults to "string" when omitted.
 *
 * A reference to a primitive type. A primitive type must have only the "type" property
 * set.
 *
 * A reference to an array type. The "type" property must be set to "array" and the "items"
 * property must be present. No other properties may be present.
 *
 * A reference to a map type. The "type" property must be set to "object" and the
 * "additionalProperties" property may be present. No other properties may be present.
 *
 * A reference to a type in this or another document. The "$ref" property must be present.
 * The "type" property is ignored if it is present. No other properties may be present.
 *
 * A reference to a union type. The "oneOf" property must be present. The union may
 * additional specify an underlying primitive type via the "type" property and a
 * discriminator via the "discriminator" property. No other properties may be present.
 */
export interface TypeReference {
    /**
     * Indicates that when used as an input, this type does not accept eventual values.
     */
    plain?: boolean;
    /**
     * The primitive type, if any
     *
     * ignored; present for compatibility with existing schemas
     *
     * The underlying primitive type of the union, if any
     */
    type?: string;
    /**
     * The element type of the array
     */
    items?: TypeReference;
    /**
     * The element type of the map. Defaults to "string" when omitted.
     */
    additionalProperties?: TypeReference;
    /**
     * The URI of the referenced type. For example, the built-in Archive, Asset, and Any
     * types are referenced as "pulumi.json#/Archive", "pulumi.json#/Asset", and
     * "pulumi.json#/Any", respectively.
     * A type from this document is referenced as "#/types/pulumi:type:token".
     * A type from another document is referenced as "path#/types/pulumi:type:token", where path
     * is of the form:
     * "/provider/vX.Y.Z/schema.json" or "pulumi.json" or
     * "http[s]://example.com/provider/vX.Y.Z/schema.json"
     * A resource from this document is referenced as "#/resources/pulumi:type:token".
     * A resource from another document is referenced as "path#/resources/pulumi:type:token",
     * where path is of the form:
     * "/provider/vX.Y.Z/schema.json" or "pulumi.json" or
     * "http[s]://example.com/provider/vX.Y.Z/schema.json"
     */
    $ref?: string;
    /**
     * Informs the consumer of an alternative schema based on the value associated with it
     */
    discriminator?: Discriminator;
    /**
     * If present, indicates that values of the type may be one of any of the listed types
     */
    oneOf?: TypeReference[];
    [property: string]: any;
}

/**
 * Informs the consumer of an alternative schema based on the value associated with it
 */
export interface Discriminator {
    /**
     * an optional object to hold mappings between payload values and schema names or references
     */
    mapping?: { [key: string]: string };
    /**
     * PropertyName is the name of the property in the payload that will hold the discriminator
     * value
     */
    propertyName: string;
    [property: string]: any;
}

/**
 * The constant value for the property, if any. The type of the value must be assignable to
 * the type of the property.
 *
 * The default value for the property, if any. The type of the value must be assignable to
 * the type of the property.
 */
export type Const = boolean | number | string;

/**
 * Additional information about the property's default value, if any.
 */
export interface DefaultInfo {
    /**
     * A set of environment variables to probe for a default value.
     */
    environment: string[];
    /**
     * Additional language-specific data about the default value.
     */
    language?: { [key: string]: any };
    [property: string]: any;
}

/**
 * Describes a function.
 */
export interface FunctionDefinition {
    /**
     * Indicates whether the function is deprecated
     */
    deprecationMessage?: string;
    /**
     * The description of the function, if any. Interpreted as Markdown.
     */
    description?: string;
    /**
     * The bag of input values for the function, if any.
     */
    inputs?: ObjectTypeDetails;
    /**
     * Indicates that the implementation of the function should not be generated from the
     * schema, and is instead provided out-of-band by the package author
     */
    isOverlay?: boolean;
    /**
     * Additional language-specific data about the function.
     */
    language?: { [key: string]: any };
    /**
     * A list of parameter names that determines whether the input bag should be treated as a
     * single argument or as multiple arguments. The list corresponds to the order in which the
     * parameters should be passed to the function.
     */
    multiArgumentInputs?: string[];
    /**
     * Specifies the return type of the function definition.
     */
    outputs?: OutputsObject;
    [property: string]: any;
}

/**
 * The bag of input values for the function, if any.
 *
 * Describes an object type
 *
 * An optional objectTypeSpec that describes additional inputs that mau be necessary to get
 * an existing resource. If this is unset, only an ID is necessary.
 */
export interface ObjectTypeDetails {
    /**
     * A map from property name to propertySpec that describes the object's properties.
     */
    properties?: { [key: string]: PropertyDefinition };
    /**
     * A list of the names of an object type's required properties. These properties must be set
     * for inputs and will always be set for outputs.
     */
    required?: string[];
    [property: string]: any;
}

/**
 * Specifies the return type of the function definition.
 *
 * A reference to a type. The particular kind of type referenced is determined based on the
 * contents of the "type" property and the presence or absence of the
 * "additionalProperties", "items", "oneOf", and "$ref" properties.
 *
 * The element type of the array
 *
 * The element type of the map. Defaults to "string" when omitted.
 *
 * A reference to a primitive type. A primitive type must have only the "type" property
 * set.
 *
 * A reference to an array type. The "type" property must be set to "array" and the "items"
 * property must be present. No other properties may be present.
 *
 * A reference to a map type. The "type" property must be set to "object" and the
 * "additionalProperties" property may be present. No other properties may be present.
 *
 * A reference to a type in this or another document. The "$ref" property must be present.
 * The "type" property is ignored if it is present. No other properties may be present.
 *
 * A reference to a union type. The "oneOf" property must be present. The union may
 * additional specify an underlying primitive type via the "type" property and a
 * discriminator via the "discriminator" property. No other properties may be present.
 *
 * The bag of input values for the function, if any.
 *
 * Describes an object type
 *
 * An optional objectTypeSpec that describes additional inputs that mau be necessary to get
 * an existing resource. If this is unset, only an ID is necessary.
 */
export interface OutputsObject {
    /**
     * Indicates that when used as an input, this type does not accept eventual values.
     */
    plain?: boolean;
    /**
     * The primitive type, if any
     *
     * ignored; present for compatibility with existing schemas
     *
     * The underlying primitive type of the union, if any
     */
    type?: string;
    /**
     * The element type of the array
     */
    items?: TypeReference;
    /**
     * The element type of the map. Defaults to "string" when omitted.
     */
    additionalProperties?: TypeReference;
    /**
     * The URI of the referenced type. For example, the built-in Archive, Asset, and Any
     * types are referenced as "pulumi.json#/Archive", "pulumi.json#/Asset", and
     * "pulumi.json#/Any", respectively.
     * A type from this document is referenced as "#/types/pulumi:type:token".
     * A type from another document is referenced as "path#/types/pulumi:type:token", where path
     * is of the form:
     * "/provider/vX.Y.Z/schema.json" or "pulumi.json" or
     * "http[s]://example.com/provider/vX.Y.Z/schema.json"
     * A resource from this document is referenced as "#/resources/pulumi:type:token".
     * A resource from another document is referenced as "path#/resources/pulumi:type:token",
     * where path is of the form:
     * "/provider/vX.Y.Z/schema.json" or "pulumi.json" or
     * "http[s]://example.com/provider/vX.Y.Z/schema.json"
     */
    $ref?: string;
    /**
     * Informs the consumer of an alternative schema based on the value associated with it
     */
    discriminator?: Discriminator;
    /**
     * If present, indicates that values of the type may be one of any of the listed types
     */
    oneOf?: TypeReference[];
    /**
     * A map from property name to propertySpec that describes the object's properties.
     */
    properties?: { [key: string]: PropertyDefinition };
    /**
     * A list of the names of an object type's required properties. These properties must be set
     * for inputs and will always be set for outputs.
     */
    required?: string[];
    [property: string]: any;
}

/**
 * Format metadata about this package.
 */
export interface Meta {
    /**
     * A regex that is used by the importer to extract a module name from the module portion of
     * a type token. Packages that use the module format "namespace1/namespace2/.../namespaceN"
     * do not need to specify a format. The regex must define one capturing group that contains
     * the module name, which must be formatted as "namespace1/namespace2/...namespaceN".
     */
    moduleFormat?: string;
    /**
     * Write the package to support the pack command.
     */
    supportPack?: boolean;
}

/**
 * An optional object to define parameterization for the package.
 */
export interface Parameterization {
    baseProvider?: BaseProvider;
    parameter?:    string;
}

export interface BaseProvider {
    /**
     * The unqualified name of the package (e.g. "aws", "azure", "gcp", "kubernetes", "random")
     */
    name: string;
    /**
     * The URL to use when downloading the provider plugin binary.
     */
    pluginDownloadURL?: string;
    /**
     * The version of the package. The version must be valid semver.
     */
    version: string;
}

/**
 * The provider type for this package.
 *
 * Describes a resource or component.
 *
 * The bag of input values for the function, if any.
 *
 * Describes an object type
 *
 * An optional objectTypeSpec that describes additional inputs that mau be necessary to get
 * an existing resource. If this is unset, only an ID is necessary.
 */
export interface ResourceDefinition {
    /**
     * The list of aliases for the resource.
     */
    aliases?: AliasDefinition[];
    /**
     * Indicates whether the resource is deprecated
     */
    deprecationMessage?: string;
    /**
     * The description of the resource, if any. Interpreted as Markdown.
     */
    description?: string;
    /**
     * A map from property name to propertySpec that describes the resource's input properties.
     */
    inputProperties?: { [key: string]: PropertyDefinition };
    /**
     * Indicates whether the resource is a component.
     */
    isComponent?: boolean;
    /**
     * Indicates that the implementation of the resource should not be generated from the
     * schema, and is instead provided out-of-band by the package author
     */
    isOverlay?: boolean;
    /**
     * A map from method name to function token that describes the resource's method set.
     */
    methods?: { [key: string]: string };
    /**
     * A list of the names of the resource's required input properties.
     */
    requiredInputs?: string[];
    /**
     * An optional objectTypeSpec that describes additional inputs that mau be necessary to get
     * an existing resource. If this is unset, only an ID is necessary.
     */
    stateInputs?: ObjectTypeDetails;
    /**
     * A map from property name to propertySpec that describes the object's properties.
     */
    properties?: { [key: string]: PropertyDefinition };
    /**
     * A list of the names of an object type's required properties. These properties must be set
     * for inputs and will always be set for outputs.
     */
    required?: string[];
    [property: string]: any;
}

export interface AliasDefinition {
    /**
     * The name portion of the alias, if any
     */
    name?: string;
    /**
     * The project portion of the alias, if any
     */
    project?: string;
    /**
     * The type portion of the alias, if any
     */
    type?: string;
    [property: string]: any;
}

/**
 * Describes an object or enum type.
 *
 * The bag of input values for the function, if any.
 *
 * Describes an object type
 *
 * An optional objectTypeSpec that describes additional inputs that mau be necessary to get
 * an existing resource. If this is unset, only an ID is necessary.
 *
 * Describes an enum type
 */
export interface TypeDefinition {
    /**
     * The description of the type, if any. Interpreted as Markdown.
     */
    description?: string;
    /**
     * Indicates that the implementation of the type should not be generated from the schema,
     * and is instead provided out-of-band by the package author
     */
    isOverlay?: boolean;
    /**
     * Additional language-specific data about the type.
     */
    language?: { [key: string]: any };
    /**
     * The underlying primitive type of the enum
     */
    type?: Type;
    /**
     * A map from property name to propertySpec that describes the object's properties.
     */
    properties?: { [key: string]: PropertyDefinition };
    /**
     * A list of the names of an object type's required properties. These properties must be set
     * for inputs and will always be set for outputs.
     */
    required?: string[];
    /**
     * The list of possible values for the enum
     */
    enum?: EnumValueDefinition[];
    [property: string]: any;
}

export interface EnumValueDefinition {
    /**
     * Indicates whether the value is deprecated.
     */
    deprecationMessage?: string;
    /**
     * The description of the enum value, if any. Interpreted as Markdown.
     */
    description?: string;
    /**
     * If present, overrides the name of the enum value that would usually be derived from the
     * value.
     */
    name?: string;
    /**
     * The enum value itself
     */
    value: Value;
    [property: string]: any;
}

/**
 * The enum value itself
 */
export type Value = boolean | number | number | string;

/**
 * The underlying primitive type of the enum
 */
export type Type = "boolean" | "integer" | "number" | "object" | "string";
