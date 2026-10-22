// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// A description of the schema for a Pulumi Package.
export interface PackageSpec {
    // The unqualified name of the package (e.g., "aws", "azure", "gcp", "kubernetes", "random").
    name: string;
    // The human-friendly name of the package.
    displayName?: string;
    // The version of the package. The version must be valid semver.
    version?: string;
    // The description of the package. Descriptions are interpreted as Markdown.
    description?: string;
    // The list of keywords that are associated with the package, if any.
    keywords?: string[];
    // The package's homepage.
    homepage?: string;
    // The name of the license used for the package's contents.
    license?: string;
    // Freeform text attribution of derived work, if required.
    attribution?: string;
    // The URL at which the package's sources can be found.
    repository?: string;
    // The URL of the package's logo, if any.
    logoUrl?: string;
    // The URL to use when downloading the provider plugin binary.
    pluginDownloadURL?: string;
    // The name of the person or organization that authored and published the package.
    publisher?: string;
    // Format metadata about this package.
    meta?: MetadataSpec;
    // A list of allowed package name in addition to the Name property.
    allowedPackageNames?: string[];
    // Additional language-specific data about the package.
    language?: Record<string, any>;
    // The package's configuration variables.
    config?: ConfigSpec;
    // A map from type token to complexTypeSpec that describes the set of complex types (i.e. object, enum)
    // defined by this package.
    types?: Record<string, ComplexTypeSpec>;
    // The provider type for this package.
    provider?: ResourceSpec;
    // A map from type token to resourceSpec that describes the set of resources and components defined by
    // this package.
    resources?: Record<string, ResourceSpec>;
    // A map from token to functionSpec that describes the set of functions defined by this package.
    functions?: Record<string, FunctionSpec>;
    // An optional object to define parameterization for the package.
    parameterization?: ParameterizationSpec;
}

// Contains information for the importer about this package.
export interface MetadataSpec {
    // A regex that is used by the importer to extract a module name from the module portion of a type token.
    // Packages that use the module format "namespace1/namespace2/.../namespaceN" do not need to specify a format.
    // The regex must define one capturing group that contains the module name, which must be formatted as
    // "namespace1/namespace2/...namespaceN".
    moduleFormat?: string;
    // Write the package to support the pack command.
    supportPack?: boolean;
}

// A description of a package's configuration variables.
export interface ConfigSpec {
    // A map from variable name to propertySpec that describes a package's configuration variables.
    variables?: Record<string, PropertySpec>;
    // Required is a list of the names of the package's required configuration variables.
    required?: string[];
}

// The serializable description of a Pulumi base provider.
export interface BaseProviderSpec {
    // The unqualified name of the package (e.g. "aws", "azure", "gcp", "kubernetes", "random").
    name: string;
    // The version of the package. The version must be valid semver.
    version: string;
    // The URL to use when downloading the provider plugin binary.
    pluginDownloadURL?: string;
}

// The serializable description of a provider parameterization.
export interface ParameterizationSpec {
    // Base provider information for parameterized packages
    baseProvider?: BaseProviderSpec;
    // The parameter for the provider.
    parameter?: string;
}

// A reference to a type. The particular kind of type referenced is determined based on the contents of
// the "type" property and the presence or absence of the "additionalProperties", "items", "oneOf", and
// "$ref" properties.
export interface TypeSpec {
    // The type identifier.
    type?: string;
    // The URI of the referenced type. For example, the built-in Archive, Asset, and Any
    // types are referenced as "pulumi.json#/Archive", "pulumi.json#/Asset", and "pulumi.json#/Any", respectively.
    // A type from this document is referenced as "#/types/pulumi:type:token".
    // A type from another document is referenced as "path#/types/pulumi:type:token", where path is of the form:
    // "/provider/vX.Y.Z/schema.json" or "pulumi.json" or "http[s]://example.com/provider/vX.Y.Z/schema.json"
    // A resource from this document is referenced as "#/resources/pulumi:type:token".
    // A resource from another document is referenced as "path#/resources/pulumi:type:token", where path is of the form:
    // "/provider/vX.Y.Z/schema.json" or "pulumi.json" or "http[s]://example.com/provider/vX.Y.Z/schema.json".
    $ref?: string;
    // For map types, the element type of the map. Defaults to "string" when omitted.
    additionalProperties?: TypeSpec;
    // For array types, the element type of the array
    items?: TypeSpec;
    // For union types, if present, indicates that values of the type may be one of any of the listed types.
    oneOf?: TypeSpec[];
    // For union types, informs the consumer of an alternative schema based on the value associated with it.
    discriminator?: DiscriminatorSpec;
    // Indicates that when used as an input, this type does not accept eventual values.
    plain?: boolean;
}

// The serializable form of a reference to a type.
export interface DiscriminatorSpec {
    // PropertyName is the name of the property in the payload that will hold the discriminator value.
    propertyName: string;
    // An optional object to hold mappings between payload values and schema names or references.
    mapping?: Record<string, string>;
}

// Describes an object or resource property.
export interface PropertySpec extends TypeSpec {
    // The description of the property, if any. Interpreted as Markdown.
    description?: string;
    // The constant value for the property, if any. The type of the value must be assignable to the type
    // of the property.
    const?: string;
    // The default value for the property, if any. The type of the value must be assignable to the type
    // of the property.
    default?: string;
    // Additional information about the property's default value, if any.
    defaultInfo?: DefaultSpec;
    // Indicates whether the property is deprecated.
    deprecationMessage?: string;
    // Additional language-specific data about the property.
    language?: Record<string, any>;
    // Specifies whether the property is secret (default false).
    secret?: boolean;
    // Specifies whether a change to the property causes its containing resource to be replaced instead of updated (default false).
    replaceOnChanges?: boolean;
    // Indicates that the provider will replace the resource when this property is changed
    willReplaceOnChanges?: boolean;
}

// The serializable form of extra information about the default value for a property.
export interface DefaultSpec {
    // A set of environment variables to probe for a default value.
    environment: string[];
    // Additional language-specific data about the default value.
    language?: Record<string, any>;
}

// Describes an object type.
export interface ObjectTypeSpec {
    // Description is the description of the type, if any.
    description?: string;
    // Properties, if present, is a map from property name to PropertySpec that describes the type's properties.
    properties?: Record<string, PropertySpec>;
    // Type must be "object" if this is an object type, or the underlying type for an enum.
    type?: string;
    // Required, if present, is a list of the names of an object type's required properties. These properties must be set
    // for inputs and will always be set for outputs.
    required?: string[];
    // Plain, was a list of the names of an object type's plain properties. This property is ignored: instead, property
    // types should be marked as plain where necessary.
    plain?: string[];
    // Language specifies additional language-specific data about the type.
    language?: Record<string, any>;
    // IsOverlay indicates whether the type is an overlay provided by the package. Overlay code is generated by the
    // package rather than using the core Pulumi codegen libraries.
    isOverlay?: boolean;
    // OverlaySupportedLanguages indicates what languages the overlay supports. This only has an effect if
    // the Resource is an Overlay (IsOverlay == true).
    // Supported values are "nodejs", "python", "go", "csharp", "java", "yaml"
    overlaySupportedLanguages?: string[];
}

export interface EnumValueSpec {
    // If present, overrides the name of the enum value that would usually be derived from the value.
    name?: string;
    // The description of the enum value, if any. Interpreted as Markdown.
    description?: string;
    // The enum value itself.
    value: string | number;
    // Indicates whether the value is deprecated.
    deprecationMessage?: string;
}

// The serializable form of an object or enum type.
export interface ComplexTypeSpec extends ObjectTypeSpec {
    // The list of possible values for the enum.
    enum?: EnumValueSpec[];
}

// The serializable form of an alias description.
export interface AliasSpec {
    // The name portion of the alias, if any.
    name?: string;
    // The project portion of the alias, if any.
    project?: string;
    // The type portion of the alias, if any
    type?: string;
}

// Describes a resource or component.
export interface ResourceSpec extends ObjectTypeSpec {
    // A map from property name to propertySpec that describes the resource's input properties.
    inputProperties?: Record<string, PropertySpec>;
    // A list of the names of the resource's required input properties.
    requiredInputs?: string[];
    // An optional objectTypeSpec that describes additional inputs that may be necessary
    // to get an existing resource. If this is unset, only an ID is necessary.
    stateInputs?: ObjectTypeSpec;
    // The list of aliases for the resource.
    aliases?: AliasSpec[];
    // Indicates whether the resource is deprecated.
    deprecationMessage?: string;
    // Indicates whether the resource is a component.
    isComponent?: boolean;
    // A map from method name to function token that describes the resource's method set.
    methods?: Record<string, string>;
}

// Describes a function.
export interface FunctionSpec {
    // The description of the function, if any. Interpreted as Markdown.
    description?: string;
    // Input parameter specifications
    inputs?: ObjectTypeSpec;
    // A list of parameter names that determines whether the input bag should be treated as a single argument or as multiple arguments. The list corresponds to the order in which the parameters should be passed to the function.
    multiArgumentInputs?: string[];
    // Specifies the return type of the function definition.
    outputs?: TypeSpec | ObjectTypeSpec;
    // Indicates whether the function is deprecated.
    deprecationMessage?: string;
    // Additional language-specific data about the function.
    language?: Record<string, any>;
    // Whether this is an overlay function
    isOverlay?: boolean;
    // Indicates what languages the overlay supports. This only has an effect if
    // the Resource is an Overlay (IsOverlay == true).
    // Supported values are "nodejs", "python", "go", "csharp", "java", "yaml"
    overlaySupportedLanguages?: string[];
}

// C# Language Overrides Definition.
export interface CSharpLanguageSpec {
    compatibility?: string;
    namespaces?: { [key: string]: string };
    packageReferences?: { [key: string]: string };
    // Root namespace for the generated .NET SDK.
    rootNamespace?: string;
    // Respect the Pkg.Version field for emitted code.
    respectSchemaVersion?: boolean;
}

// Go Language Overrides Definition.
export interface GoLanguageSpec {
    generateExtraInputTypes?: boolean;
    generateResourceContainerTypes?: boolean;
    // Base import path for the package.
    importBasePath?: string;
    // Respect the Pkg.Version field for emitted code.
    respectSchemaVersion?: boolean;
}

// NodeJS Language Overrides Definition.
export interface NodeJSLanguageSpec {
    // The NPM package name (includes @namespace).
    packageName?: string;
    // The NPM package description.
    packageDescription?: string;
    // Content of the generated package README.md file.
    readme?: string;
    compatibility?: "tfbridge20";
    // NPM package dependencies
    dependencies?: { [key: string]: string };
    // NPM package devDependencies.
    devDependencies?: { [key: string]: string };
    disableUnionOutputTypes?: boolean;
    // TypeScript Version.
    typescriptVersion?: string;
    // Respect the Pkg.Version field for emitted code.
    respectSchemaVersion?: boolean;
}

// Python Language Overrides Definition.
export interface PythonLanguageSpec {
    // The PyPI package name.
    packageName?: string;
    // Content of the generated package README.md file.
    readme?: string;
    compatibility?: "tfbridge20";
    // Python package dependencies for the generated Python SDK
    requires?: { [key: string]: string };
    // espect the Pkg.Version field for emitted code.
    respectSchemaVersion?: boolean;
}

// Java Language Overrides Definition
export interface JavaLanguageSpec {
    packages?: { [key: string]: string };
    // Base Java package name for the generated Java provider SDK.
    basePackage?: string;
    // If set to "gradle" enables a generation of a basic set of Gradle build files.
    buildFiles?: string;
    // Specifies Maven-style dependencies for the generated code.
    dependencies?: { [key: string]: string };
    // Enables the use of a given version of io.github.gradle-nexus.publish-plugin in the generated
    // Gradle build files (only when `buildFiles="gradle").
    gradleNexusPublishPluginVersion?: string;
    // generates a test section to enable `gradle test` command to run unit tests over the generated code.
    // Supported values: "JUnitPlatform" (only when `buildFiles="gradle")",
    gradleTest?: "JUnitPlatform";
}
