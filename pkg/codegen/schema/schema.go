package schema

import schema "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/schema"

// Type represents a datatype in the Pulumi Schema. Types created by this package are identical if they are
// equal values.
type Type = schema.Type

// An InvalidType represents an invalid type with associated diagnostics.
type InvalidType = schema.InvalidType

// MapType represents maps from strings to particular element types.
type MapType = schema.MapType

// ArrayType represents arrays of particular element types.
type ArrayType = schema.ArrayType

// EnumType represents an enum.
type EnumType = schema.EnumType

// Enum contains information about an enum.
type Enum = schema.Enum

// UnionType represents values that may be any one of a specified set of types.
type UnionType = schema.UnionType

// ObjectType represents schematized maps from strings to particular types.
type ObjectType = schema.ObjectType

type ResourceType = schema.ResourceType

// TokenType represents an opaque type that is referred to only by its token. A TokenType may have an underlying type
// that can be used in place of the token.
type TokenType = schema.TokenType

// InputType represents a type that accepts either a prompt value or an output value.
type InputType = schema.InputType

// OptionalType represents a type that accepts an optional value.
type OptionalType = schema.OptionalType

// DefaultValue describes a default value for a property.
type DefaultValue = schema.DefaultValue

// Property describes an object or resource property.
type Property = schema.Property

// Alias describes an alias for a Pulumi resource.
type Alias = schema.Alias

// Resource describes a Pulumi resource.
type Resource = schema.Resource

type Method = schema.Method

// Function describes a Pulumi function.
type Function = schema.Function

// BaseProvider
type BaseProvider = schema.BaseProvider

type Parameterization = schema.Parameterization

// Package describes a Pulumi package.
type Package = schema.Package

// Language provides hooks for importing language-specific metadata in a package.
type Language = schema.Language

type RawMessage = schema.RawMessage

// TypeSpec is the serializable form of a reference to a type.
type TypeSpec = schema.TypeSpec

// DiscriminatorSpec informs the consumer of an alternative schema based on the value associated with it.
type DiscriminatorSpec = schema.DiscriminatorSpec

// DefaultSpec is the serializable form of extra information about the default value for a property.
type DefaultSpec = schema.DefaultSpec

// PropertySpec is the serializable form of an object or resource property.
type PropertySpec = schema.PropertySpec

// ObjectTypeSpec is the serializable form of an object type.
type ObjectTypeSpec = schema.ObjectTypeSpec

// ComplexTypeSpec is the serializable form of an object or enum type.
type ComplexTypeSpec = schema.ComplexTypeSpec

// EnumValueSpec is the serializable form of the values metadata associated with an enum type.
type EnumValueSpec = schema.EnumValueSpec

// AliasSpec is the serializable form of an alias description.
type AliasSpec = schema.AliasSpec

// ResourceSpec is the serializable form of a resource description.
type ResourceSpec = schema.ResourceSpec

// ReturnTypeSpec is either ObjectTypeSpec or TypeSpec.
type ReturnTypeSpec = schema.ReturnTypeSpec

// Deprecated.
type Decoder = schema.Decoder

// FunctionSpec is the serializable form of a function description.
type FunctionSpec = schema.FunctionSpec

// ConfigSpec is the serializable description of a package's configuration variables.
type ConfigSpec = schema.ConfigSpec

// MetadataSpec contains information for the importer about this package.
type MetadataSpec = schema.MetadataSpec

// PackageInfoSpec is the serializable description of a Pulumi package's metadata.
type PackageInfoSpec = schema.PackageInfoSpec

// BaseProviderSpec is the serializable description of a Pulumi base provider.
type BaseProviderSpec = schema.BaseProviderSpec

// ParameterizationSpec is the serializable description of a provider parameterization.
type ParameterizationSpec = schema.ParameterizationSpec

// PackageSpec is the serializable description of a Pulumi package.
type PackageSpec = schema.PackageSpec

// PartialPackageSpec is a serializable description of a Pulumi package that defers the deserialization of most package
// members until they are needed. Used to support PartialPackage and PackageReferences.
type PartialPackageSpec = schema.PartialPackageSpec

var BoolType = schema.BoolType

var IntType = schema.IntType

var NumberType = schema.NumberType

var StringType = schema.StringType

var ArchiveType = schema.ArchiveType

var AssetType = schema.AssetType

var JSONType = schema.JSONType

var AnyType = schema.AnyType

var AnyResourceType = schema.AnyResourceType

// IsPrimitiveType returns true if the given Type is a primitive type. The primitive types are bool, int, number,
// string, archive, asset, and any.
func IsPrimitiveType(t Type) bool {
	return schema.IsPrimitiveType(t)
}

// Joins the output of `ReplaceOnChanges` into property path names.
// 
// For example, given an input [[B, D], [C]] where each property has a name
// equivalent to it's variable, this function should yield: ["B.D", "C"]
func PropertyListJoinToString(propertyList [][]*Property, nameConverter func(string) string) []string {
	return schema.PropertyListJoinToString(propertyList, nameConverter)
}

func TokenToRuntimeModule(tok string) string {
	return schema.TokenToRuntimeModule(tok)
}

