// Copyright 2016-2020, Pulumi Corporation.
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

package schema

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
)

// TODO:
// - Providerless packages
// - Adjustments to accommodate docs + cross-lang packages (e.g references to resources from POD types)

// Type represents a datatype in the Pulumi Schema. Types created by this package are identical if they are
// equal values.
type Type interface {
	String() string

	isType()
}

type primitiveType int

const (
	boolType    primitiveType = 1
	intType     primitiveType = 2
	numberType  primitiveType = 3
	stringType  primitiveType = 4
	archiveType primitiveType = 5
	assetType   primitiveType = 6
	anyType     primitiveType = 7
	jsonType    primitiveType = 8
)

//nolint: goconst
func (t primitiveType) String() string {
	switch t {
	case boolType:
		return "boolean"
	case intType:
		return "integer"
	case numberType:
		return "number"
	case stringType:
		return "string"
	case archiveType:
		return "pulumi:pulumi:Archive"
	case assetType:
		return "pulumi:pulumi:Asset"
	case jsonType:
		fallthrough
	case anyType:
		return "pulumi:pulumi:Any"
	default:
		panic("unknown primitive type")
	}
}

func (primitiveType) isType() {}

// IsPrimitiveType returns true if the given Type is a primitive type. The primitive types are bool, int, number,
// string, archive, asset, and any.
func IsPrimitiveType(t Type) bool {
	_, ok := t.(primitiveType)
	return ok
}

var (
	// BoolType represents the set of boolean values.
	BoolType Type = boolType
	// IntType represents the set of 32-bit integer values.
	IntType Type = intType
	// NumberType represents the set of IEEE754 double-precision values.
	NumberType Type = numberType
	// StringType represents the set of UTF-8 string values.
	StringType Type = stringType
	// ArchiveType represents the set of Pulumi Archive values.
	ArchiveType Type = archiveType
	// AssetType represents the set of Pulumi Asset values.
	AssetType Type = assetType
	// JSONType represents the set of JSON-encoded values.
	JSONType Type = jsonType
	// AnyType represents the complete set of values.
	AnyType Type = anyType
)

// MapType represents maps from strings to particular element types.
type MapType struct {
	// ElementType is the element type of the map.
	ElementType Type
}

func (t *MapType) String() string {
	return fmt.Sprintf("Map<%v>", t.ElementType)
}

func (*MapType) isType() {}

// ArrayType represents arrays of particular element types.
type ArrayType struct {
	// ElementType is the element type of the array.
	ElementType Type
}

func (t *ArrayType) String() string {
	return fmt.Sprintf("Array<%v>", t.ElementType)
}

func (*ArrayType) isType() {}

// EnumType represents an enum.
type EnumType struct {
	// ElementType is the element type of the enum.
	ElementType Type
}

func (t *EnumType) String() string {
	return fmt.Sprintf("Enum<%v>", t.ElementType)
}

func (*EnumType) isType() {}

// UnionType represents values that may be any one of a specified set of types.
type UnionType struct {
	// ElementTypes are the allowable types for the union type.
	ElementTypes []Type
	// DefaultType is the default type, if any, for the union type. This can be used by targets that do not support
	// unions, or in positions where unions are not appropriate.
	DefaultType Type
}

func (t *UnionType) String() string {
	elements := make([]string, len(t.ElementTypes)+1)
	for i, e := range t.ElementTypes {
		elements[i] = e.String()
	}

	def := "default="
	if t.DefaultType != nil {
		def += t.DefaultType.String()
	}
	elements[len(elements)-1] = def

	return fmt.Sprintf("Union<%v>", strings.Join(elements, ", "))
}

func (*UnionType) isType() {}

// ObjectType represents schematized maps from strings to particular types.
type ObjectType struct {
	// Package is the package that defines the resource.
	Package *Package
	// Token is the type's Pulumi type token.
	Token string
	// Comment is the description of the type, if any.
	Comment string
	// Properties is the list of the type's properties.
	Properties []*Property
	// Language specifies additional language-specific data about the object type.
	Language map[string]interface{}

	properties map[string]*Property
}

func (t *ObjectType) Property(name string) (*Property, bool) {
	if t.properties == nil && len(t.Properties) > 0 {
		t.properties = make(map[string]*Property)
		for _, p := range t.Properties {
			t.properties[p.Name] = p
		}
	}
	p, ok := t.properties[name]
	return p, ok
}

func (t *ObjectType) String() string {
	return t.Token
}

func (*ObjectType) isType() {}

// TokenType represents an opaque type that is referred to only by its token. A TokenType may have an underlying type
// that can be used in place of the token.
type TokenType struct {
	// Token is the type's Pulumi type token.
	Token string
	// Underlying type is the type's underlying type, if any.
	UnderlyingType Type
}

func (t *TokenType) String() string {
	return t.Token
}

func (*TokenType) isType() {}

// DefaultValue describes a default value for a property.
type DefaultValue struct {
	// Value specifies a static default value, if any. This value must be representable in the Pulumi schema type
	// system, and its type must be assignable to that of the property to which the default applies.
	Value interface{}
	// Environment specifies a set of environment variables to probe for a default value.
	Environment []string
	// Language specifies additional language-specific data about the default value.
	Language map[string]interface{}
}

// Property describes an object or resource property.
type Property struct {
	// Name is the name of the property.
	Name string
	// Comment is the description of the property, if any.
	Comment string
	// Type is the type of the property.
	Type Type
	// ConstValue is the constant value for the property, if any.
	ConstValue interface{}
	// DefaultValue is the default value for the property, if any.
	DefaultValue *DefaultValue
	// IsRequired is true if the property must always be populated.
	IsRequired bool
	// DeprecationMessage indicates whether or not the property is deprecated.
	DeprecationMessage string
	// Language specifies additional language-specific data about the property.
	Language map[string]interface{}
	// Secret is true if the property is secret (default false).
	Secret bool
}

// Alias describes an alias for a Pulumi resource.
type Alias struct {
	// Name is the "name" portion of the alias, if any.
	Name *string
	// Project is the "project" portion of the alias, if any.
	Project *string
	// Type is the "type" portion of the alias, if any.
	Type *string
}

// Resource describes a Pulumi resource.
type Resource struct {
	// Package is the package that defines the resource.
	Package *Package
	// Token is the resource's Pulumi type token.
	Token string
	// Comment is the description of the resource, if any.
	Comment string
	// IsProvider is true if the resource is a provider resource.
	IsProvider bool
	// InputProperties is the list of the resource's input properties.
	InputProperties []*Property
	// Properties is the list of the resource's output properties. This should be a superset of the input properties.
	Properties []*Property
	// StateInputs is the set of inputs used to get an existing resource, if any.
	StateInputs *ObjectType
	// Aliases is the list of aliases for the resource.
	Aliases []*Alias
	// DeprecationMessage indicates whether or not the resource is deprecated.
	DeprecationMessage string
	// Language specifies additional language-specific data about the resource.
	Language map[string]interface{}
}

// Function describes a Pulumi function.
type Function struct {
	// Package is the package that defines the function.
	Package *Package
	// Token is the function's Pulumi type token.
	Token string
	// Comment is the description of the function, if any.
	Comment string
	// Inputs is the bag of input values for the function, if any.
	Inputs *ObjectType
	// Outputs is the bag of output values for the function, if any.
	Outputs *ObjectType
	// DeprecationMessage indicates whether or not the function is deprecated.
	DeprecationMessage string
	// Language specifies additional language-specific data about the function.
	Language map[string]interface{}
}

// Package describes a Pulumi package.
type Package struct {
	moduleFormat *regexp.Regexp

	// Name is the unqualified name of the package (e.g. "aws", "azure", "gcp", "kubernetes". "random")
	Name string
	// Version is the version of the package.
	Version *semver.Version
	// Description is the description of the package.
	Description string
	// Keywords is the list of keywords that are associated with the package, if any.
	Keywords []string
	// Homepage is the package's homepage.
	Homepage string
	// License indicates which license is used for the package's contents.
	License string
	// Attribution allows freeform text attribution of derived work, if needed.
	Attribution string
	// Repository is the URL at which the source for the package can be found.
	Repository string
	// LogoURL is the URL for the package's logo, if any.
	LogoURL string
	// PluginDownloadURL is the URL to use to acquire the provider plugin binary, if any.
	PluginDownloadURL string

	// Types is the list of non-resource types defined by the package.
	Types []Type
	// Config is the set of configuration properties defined by the package.
	Config []*Property
	// Provider is the resource provider for the package, if any.
	Provider *Resource
	// Resources is the list of resource types defined by the package.
	Resources []*Resource
	// Functions is the list of functions defined by the package.
	Functions []*Function
	// Language specifies additional language-specific data about the package.
	Language map[string]interface{}

	resourceTable map[string]*Resource
	functionTable map[string]*Function
}

// Language provides hooks for importing language-specific metadata in a package.
type Language interface {
	// ImportDefaultSpec decodes language-specific metadata associated with a DefaultValue.
	ImportDefaultSpec(def *DefaultValue, bytes json.RawMessage) (interface{}, error)
	// ImportPropertySpec decodes language-specific metadata associated with a Property.
	ImportPropertySpec(property *Property, bytes json.RawMessage) (interface{}, error)
	// ImportObjectTypeSpec decodes language-specific metadata associated with a ObjectType.
	ImportObjectTypeSpec(object *ObjectType, bytes json.RawMessage) (interface{}, error)
	// ImportResourceSpec decodes language-specific metadata associated with a Resource.
	ImportResourceSpec(resource *Resource, bytes json.RawMessage) (interface{}, error)
	// ImportFunctionSpec decodes language-specific metadata associated with a Function.
	ImportFunctionSpec(function *Function, bytes json.RawMessage) (interface{}, error)
	// ImportPackageSpec decodes language-specific metadata associated with a Package.
	ImportPackageSpec(pkg *Package, bytes json.RawMessage) (interface{}, error)
}

func sortedLanguageNames(metadata map[string]interface{}) []string {
	names := make([]string, 0, len(metadata))
	for lang := range metadata {
		names = append(names, lang)
	}
	sort.Strings(names)
	return names
}

func importDefaultLanguages(def *DefaultValue, languages map[string]Language) error {
	for _, name := range sortedLanguageNames(def.Language) {
		val := def.Language[name]
		if raw, ok := val.(json.RawMessage); ok {
			if lang, ok := languages[name]; ok {
				val, err := lang.ImportDefaultSpec(def, raw)
				if err != nil {
					return errors.Wrapf(err, "importing %v metadata", name)
				}
				def.Language[name] = val
			}
		}
	}
	return nil
}

func importPropertyLanguages(property *Property, languages map[string]Language) error {
	if property.DefaultValue != nil {
		if err := importDefaultLanguages(property.DefaultValue, languages); err != nil {
			return errors.Wrapf(err, "importing default value")
		}
	}

	for _, name := range sortedLanguageNames(property.Language) {
		val := property.Language[name]
		if raw, ok := val.(json.RawMessage); ok {
			if lang, ok := languages[name]; ok {
				val, err := lang.ImportPropertySpec(property, raw)
				if err != nil {
					return errors.Wrapf(err, "importing %v metadata", name)
				}
				property.Language[name] = val
			}
		}
	}
	return nil
}

func importObjectTypeLanguages(object *ObjectType, languages map[string]Language) error {
	for _, property := range object.Properties {
		if err := importPropertyLanguages(property, languages); err != nil {
			return errors.Wrapf(err, "importing property %v", property.Name)
		}
	}

	for _, name := range sortedLanguageNames(object.Language) {
		val := object.Language[name]
		if raw, ok := val.(json.RawMessage); ok {
			if lang, ok := languages[name]; ok {
				val, err := lang.ImportObjectTypeSpec(object, raw)
				if err != nil {
					return errors.Wrapf(err, "importing %v metadata", name)
				}
				object.Language[name] = val
			}
		}
	}
	return nil
}

func importResourceLanguages(resource *Resource, languages map[string]Language) error {
	for _, property := range resource.InputProperties {
		if err := importPropertyLanguages(property, languages); err != nil {
			return errors.Wrapf(err, "importing input property %v", property.Name)
		}
	}

	for _, property := range resource.Properties {
		if err := importPropertyLanguages(property, languages); err != nil {
			return errors.Wrapf(err, "importing property %v", property.Name)
		}
	}

	if resource.StateInputs != nil {
		for _, property := range resource.StateInputs.Properties {
			if err := importPropertyLanguages(property, languages); err != nil {
				return errors.Wrapf(err, "importing state input property %v", property.Name)
			}
		}
	}

	for _, name := range sortedLanguageNames(resource.Language) {
		val := resource.Language[name]
		if raw, ok := val.(json.RawMessage); ok {
			if lang, ok := languages[name]; ok {
				val, err := lang.ImportResourceSpec(resource, raw)
				if err != nil {
					return errors.Wrapf(err, "importing %v metadata", name)
				}
				resource.Language[name] = val
			}
		}
	}
	return nil
}

func importFunctionLanguages(function *Function, languages map[string]Language) error {
	if function.Inputs != nil {
		if err := importObjectTypeLanguages(function.Inputs, languages); err != nil {
			return errors.Wrapf(err, "importing inputs")
		}
	}
	if function.Outputs != nil {
		if err := importObjectTypeLanguages(function.Outputs, languages); err != nil {
			return errors.Wrapf(err, "importing outputs")
		}
	}

	for _, name := range sortedLanguageNames(function.Language) {
		val := function.Language[name]
		if raw, ok := val.(json.RawMessage); ok {
			if lang, ok := languages[name]; ok {
				val, err := lang.ImportFunctionSpec(function, raw)
				if err != nil {
					return errors.Wrapf(err, "importing %v metadata", name)
				}
				function.Language[name] = val
			}
		}
	}
	return nil
}

func (pkg *Package) ImportLanguages(languages map[string]Language) error {
	if len(languages) == 0 {
		return nil
	}

	for _, t := range pkg.Types {
		if object, ok := t.(*ObjectType); ok {
			if err := importObjectTypeLanguages(object, languages); err != nil {
				return errors.Wrapf(err, "importing object type %v", object.Token)
			}
		}
	}

	for _, config := range pkg.Config {
		if err := importPropertyLanguages(config, languages); err != nil {
			return errors.Wrapf(err, "importing configuration property %v", config.Name)
		}
	}

	if pkg.Provider != nil {
		if err := importResourceLanguages(pkg.Provider, languages); err != nil {
			return errors.Wrapf(err, "importing provider")
		}
	}

	for _, resource := range pkg.Resources {
		if err := importResourceLanguages(resource, languages); err != nil {
			return errors.Wrapf(err, "importing resource %v", resource.Token)
		}
	}

	for _, function := range pkg.Functions {
		if err := importFunctionLanguages(function, languages); err != nil {
			return errors.Wrapf(err, "importing function %v", function.Token)
		}
	}

	for _, name := range sortedLanguageNames(pkg.Language) {
		val := pkg.Language[name]
		if raw, ok := val.(json.RawMessage); ok {
			if lang, ok := languages[name]; ok {
				val, err := lang.ImportPackageSpec(pkg, raw)
				if err != nil {
					return errors.Wrapf(err, "importing %v metadata", name)
				}
				pkg.Language[name] = val
			}
		}
	}

	return nil
}

func (pkg *Package) TokenToModule(tok string) string {
	// token := pkg ":" module ":" member

	components := strings.Split(tok, ":")
	if len(components) != 3 {
		return ""
	}

	switch components[1] {
	case "providers":
		return ""
	default:
		matches := pkg.moduleFormat.FindStringSubmatch(components[1])
		if len(matches) < 2 || matches[1] == "index" {
			return ""
		}
		return matches[1]
	}
}

func (pkg *Package) GetResource(token string) (*Resource, bool) {
	r, ok := pkg.resourceTable[token]
	return r, ok
}

func (pkg *Package) GetFunction(token string) (*Function, bool) {
	f, ok := pkg.functionTable[token]
	return f, ok
}

// TypeSpec is the serializable form of a reference to a type.
type TypeSpec struct {
	// Type is the primitive or composite type, if any. May be "bool", "integer", "number", "string", "array", or
	// "object".
	Type string `json:"type,omitempty"`
	// Ref is a reference to a type in this or another document. For example, the built-in Archive, Asset, and Any
	// types are referenced as "pulumi.json#/Archive", "pulumi.json#/Asset", and "pulumi.json#/Any", respectively.
	// A type from this document is referenced as "#/types/pulumi:type:token".
	Ref string `json:"$ref,omitempty"`
	// AdditionalProperties, if set, describes the element type of an "object" (i.e. a string -> value map).
	AdditionalProperties *TypeSpec `json:"additionalProperties,omitempty"`
	// Items, if set, describes the element type of an array.
	Items *TypeSpec `json:"items,omitempty"`
	// OneOf indicates that values of the type may be one of any of the listed types.
	OneOf []TypeSpec `json:"oneOf,omitempty"`
	// Enum, if set, lists the allowed values for an enum type.
	Enum []string `json:"enum,omitempty"`
	// EnumMetadata, if set, is the metadata associated with an enum type.
	EnumMetadata *EnumMetadataSpec `json:"enumMetadata,omitempty"`
}

// EnumMetadataSpec is the serializable form of the metadata associated with an enum type.
type EnumMetadataSpec struct {
	// Name is the name of the enum type
	Name string `json:"name"`
	// ModelAsString, when true, indicates that an enum should be modelled as a string and not strictly enforced.
	ModelAsString bool `json:"modelAsString"`
	// Values contains metadata associated with each value of the enum.
	Values []interface{} `json:"values,omitempty"`
}

// DefaultSpec is the serializable form of extra information about the default value for a property.
type DefaultSpec struct {
	// Environment specifies a set of environment variables to probe for a default value.
	Environment []string `json:"environment,omitempty"`
	// Language specifies additional language-specific data about the default value.
	Language map[string]json.RawMessage `json:"language,omitempty"`
}

// PropertySpec is the serializable form of an object or resource property.
type PropertySpec struct {
	TypeSpec

	// Description is the description of the property, if any.
	Description string `json:"description,omitempty"`
	// Const is the constant value for the property, if any. The type of the value must be assignable to the type of
	// the property.
	Const interface{} `json:"const,omitempty"`
	// Default is the default value for the property, if any. The type of the value must be assignable to the type of
	// the property.
	Default interface{} `json:"default,omitempty"`
	// DefaultInfo contains additional information about the property's default value, if any.
	DefaultInfo *DefaultSpec `json:"defaultInfo,omitempty"`
	// DeprecationMessage indicates whether or not the property is deprecated.
	DeprecationMessage string `json:"deprecationMessage,omitempty"`
	// Language specifies additional language-specific data about the property.
	Language map[string]json.RawMessage `json:"language,omitempty"`
	// Secret specifies if the property is secret (default false).
	Secret bool `json:"secret,omitempty"`
}

// ObjectTypeSpec is the serializable form of an object type.
type ObjectTypeSpec struct {
	// Description is the description of the type, if any.
	Description string `json:"description,omitempty"`
	// Properties is a map from property name to PropertySpec that describes the type's properties.
	Properties map[string]PropertySpec `json:"properties,omitempty"`
	// Type must be "object".
	Type string `json:"type,omitempty"`
	// Required is a list of the names of the type's required properties. These properties must be set for inputs and
	// will always be set for outputs.
	Required []string `json:"required,omitempty"`
	// Language specifies additional language-specific data about the type.
	Language map[string]json.RawMessage `json:"language,omitempty"`
}

// AliasSpec is the serializable form of an alias description.
type AliasSpec struct {
	// Name is the name portion of the alias, if any.
	Name *string `json:"name,omitempty"`
	// Project is the project portion of the alias, if any.
	Project *string `json:"project,omitempty"`
	// Type is the type portion of the alias, if any.
	Type *string `json:"type,omitempty"`
}

// ResourceSpec is the serializable form of a resource description.
type ResourceSpec struct {
	ObjectTypeSpec

	// InputProperties is a map from property name to PropertySpec that describes the resource's input properties.
	InputProperties map[string]PropertySpec `json:"inputProperties,omitempty"`
	// RequiredInputs is a list of the names of the resource's required input properties.
	RequiredInputs []string `json:"requiredInputs,omitempty"`
	// StateInputs is an optional ObjectTypeSpec that describes additional inputs that mau be necessary to get an
	// existing resource. If this is unset, only an ID is necessary.
	StateInputs *ObjectTypeSpec `json:"stateInputs,omitempty"`
	// Aliases is the list of aliases for the resource.
	Aliases []AliasSpec `json:"aliases,omitempty"`
	// DeprecationMessage indicates whether or not the resource is deprecated.
	DeprecationMessage string `json:"deprecationMessage,omitempty"`
	// Language specifies additional language-specific data about the resource.
	Language map[string]json.RawMessage `json:"language,omitempty"`
}

// FunctionSpec is the serializable form of a function description.
type FunctionSpec struct {
	// Description is the description of the function, if any.
	Description string `json:"description,omitempty"`
	// Inputs is the bag of input values for the function, if any.
	Inputs *ObjectTypeSpec `json:"inputs,omitempty"`
	// Outputs is the bag of output values for the function, if any.
	Outputs *ObjectTypeSpec `json:"outputs,omitempty"`
	// DeprecationMessage indicates whether or not the function is deprecated.
	DeprecationMessage string `json:"deprecationMessage,omitempty"`
	// Language specifies additional language-specific data about the function.
	Language map[string]json.RawMessage `json:"language,omitempty"`
}

// ConfigSpec is the serializable description of a package's configuration variables.
type ConfigSpec struct {
	// Variables is a map from variable name to PropertySpec that describes a package's configuration variables.
	Variables map[string]PropertySpec `json:"variables,omitempty"`
	// Required is a list of the names of the package's required configuration variables.
	Required []string `json:"defaults,omitempty"`
}

// MetadataSpec contains information for the importer about this package.
type MetadataSpec struct {
	// ModuleFormat is a regex that is used by the importer to extract a module name from the module portion of a
	// type token. Pacakages that use the module format "namespace1/namespace2/.../namespaceN" do not need to specify
	// a format. The regex must define one capturing group that contains the module name, which must be formatted as
	// "namespace1/namespace2/...namespaceN".
	ModuleFormat string `json:"moduleFormat,omitempty"`
}

// PackageSpec is the serializable description of a Pulumi package.
type PackageSpec struct {
	// Name is the unqualified name of the package (e.g. "aws", "azure", "gcp", "kubernetes". "random")
	Name string `json:"name"`
	// Version is the version of the package. The version must be valid semver.
	Version string `json:"version,omitempty"`
	// Description is the description of the package.
	Description string `json:"description,omitempty"`
	// Keywords is the list of keywords that are associated with the package, if any.
	Keywords []string `json:"keywords,omitempty"`
	// Homepage is the package's homepage.
	Homepage string `json:"homepage,omitempty"`
	// License indicates which license is used for the package's contents.
	License string `json:"license,omitempty"`
	// Attribution allows freeform text attribution of derived work, if needed.
	Attribution string `json:"attribution,omitempty"`
	// Repository is the URL at which the source for the package can be found.
	Repository string `json:"repository,omitempty"`
	// LogoURL is the URL for the package's logo, if any.
	LogoURL string `json:"logoUrl,omitempty"`
	// PluginDownloadURL is the URL to use to acquire the provider plugin binary, if any.
	PluginDownloadURL string `json:"pluginDownloadURL,omitempty"`

	// Meta contains information for the importer about this package.
	Meta *MetadataSpec `json:"meta,omitempty"`

	// Config describes the set of configuration variables defined by this package.
	Config ConfigSpec `json:"config"`
	// Types is a map from type token to ObjectTypeSpec that describes the set of object types defined by this package.
	Types map[string]ObjectTypeSpec `json:"types,omitempty"`
	// Provider describes the provider type for this package.
	Provider ResourceSpec `json:"provider"`
	// Resources is a map from type token to ResourceSpec that describes the set of resources defined by this package.
	Resources map[string]ResourceSpec `json:"resources,omitempty"`
	// Functions is a map from token to FunctionSpec that describes the set of functions defined by this package.
	Functions map[string]FunctionSpec `json:"functions,omitempty"`
	// Language specifies additional language-specific data about the package.
	Language map[string]json.RawMessage `json:"language,omitempty"`
}

// ImportSpec converts a serializable PackageSpec into a Package.
func ImportSpec(spec PackageSpec, languages map[string]Language) (*Package, error) {
	// Parse the version, if any.
	var version *semver.Version
	if spec.Version != "" {
		v, err := semver.ParseTolerant(spec.Version)
		if err != nil {
			return nil, errors.Wrap(err, "parsing package version")
		}
		version = &v
	}

	// Parse the module format, if any.
	moduleFormat := "(.*)"
	if spec.Meta != nil && spec.Meta.ModuleFormat != "" {
		moduleFormat = spec.Meta.ModuleFormat
	}
	moduleFormatRegexp, err := regexp.Compile(moduleFormat)
	if err != nil {
		return nil, errors.Wrap(err, "compiling module format regexp")
	}

	pkg := &Package{}

	types, err := bindTypes(pkg, spec.Types)
	if err != nil {
		return nil, errors.Wrap(err, "binding types")
	}

	config, err := bindConfig(spec.Config, types)
	if err != nil {
		return nil, errors.Wrap(err, "binding config")
	}

	provider, err := bindProvider(spec.Name, spec.Provider, types)
	if err != nil {
		return nil, errors.Wrap(err, "binding provider")
	}

	resources, resourceTable, err := bindResources(spec.Resources, types)
	if err != nil {
		return nil, errors.Wrap(err, "binding resources")
	}

	functions, functionTable, err := bindFunctions(spec.Functions, types)
	if err != nil {
		return nil, errors.Wrap(err, "binding functions")
	}

	// Build the type list.
	var typeList []Type
	for _, t := range types.objects {
		typeList = append(typeList, t)
	}
	for _, t := range types.arrays {
		typeList = append(typeList, t)
	}
	for _, t := range types.maps {
		typeList = append(typeList, t)
	}
	for _, t := range types.unions {
		typeList = append(typeList, t)
	}
	for _, t := range types.tokens {
		typeList = append(typeList, t)
	}

	sort.Slice(typeList, func(i, j int) bool {
		return typeList[i].String() < typeList[j].String()
	})

	language := make(map[string]interface{})
	for name, raw := range spec.Language {
		language[name] = raw
	}

	*pkg = Package{
		moduleFormat:      moduleFormatRegexp,
		Name:              spec.Name,
		Version:           version,
		Description:       spec.Description,
		Keywords:          spec.Keywords,
		Homepage:          spec.Homepage,
		License:           spec.License,
		Attribution:       spec.Attribution,
		Repository:        spec.Repository,
		PluginDownloadURL: spec.PluginDownloadURL,
		Config:            config,
		Types:             typeList,
		Provider:          provider,
		Resources:         resources,
		Functions:         functions,
		Language:          language,
		resourceTable:     resourceTable,
		functionTable:     functionTable,
	}
	if err := pkg.ImportLanguages(languages); err != nil {
		return nil, err
	}
	return pkg, nil
}

type types struct {
	pkg *Package

	objects map[string]*ObjectType
	arrays  map[Type]*ArrayType
	maps    map[Type]*MapType
	unions  map[string]*UnionType
	tokens  map[string]*TokenType
}

func (t *types) bindPrimitiveType(name string) (Type, error) {
	switch name {
	case "boolean":
		return BoolType, nil
	case "integer":
		return IntType, nil
	case "number":
		return NumberType, nil
	case "string":
		return StringType, nil
	default:
		return nil, errors.Errorf("unknown primitive type %v", name)
	}
}

func (t *types) bindType(spec TypeSpec) (Type, error) {
	if spec.Ref != "" {
		switch spec.Ref {
		case "pulumi.json#/Archive":
			return ArchiveType, nil
		case "pulumi.json#/Asset":
			return AssetType, nil
		case "pulumi.json#/Json":
			return JSONType, nil
		case "pulumi.json#/Any":
			return AnyType, nil
		}

		// Parse the ref and look up the type in the type map.
		if !strings.HasPrefix(spec.Ref, "#/types/") {
			return nil, errors.Errorf("failed to parse ref %s", spec.Ref)
		}

		token, err := url.PathUnescape(spec.Ref[len("#/types/"):])
		if err != nil {
			return nil, errors.Errorf("failed to parse ref %s", spec.Ref)
		}
		if typ, ok := t.objects[token]; ok {
			return typ, nil
		}
		typ, ok := t.tokens[token]
		if !ok {
			typ = &TokenType{Token: token}
			if spec.Type != "" {
				ut, err := t.bindType(TypeSpec{Type: spec.Type})
				if err != nil {
					return nil, err
				}
				typ.UnderlyingType = ut
			}
			t.tokens[token] = typ
		}
		return typ, nil
	}

	if spec.OneOf != nil {
		if len(spec.OneOf) < 2 {
			return nil, errors.New("oneOf should list at least two types")
		}

		var defaultType Type
		if spec.Type != "" {
			dt, err := t.bindPrimitiveType(spec.Type)
			if err != nil {
				return nil, err
			}
			defaultType = dt
		}

		elements := make([]Type, len(spec.OneOf))
		for i, spec := range spec.OneOf {
			e, err := t.bindType(spec)
			if err != nil {
				return nil, err
			}
			elements[i] = e
		}

		union := &UnionType{
			ElementTypes: elements,
			DefaultType:  defaultType,
		}
		if typ, ok := t.unions[union.String()]; ok {
			return typ, nil
		}
		t.unions[union.String()] = union
		return union, nil
	}

	switch spec.Type {
	case "boolean", "integer", "number", "string":
		return t.bindPrimitiveType(spec.Type)
	case "array":
		if spec.Items == nil {
			return nil, errors.Errorf("missing \"items\" property in type spec")
		}

		elementType, err := t.bindType(*spec.Items)
		if err != nil {
			return nil, err
		}

		typ, ok := t.arrays[elementType]
		if !ok {
			typ = &ArrayType{ElementType: elementType}
			t.arrays[elementType] = typ
		}
		return typ, nil
	case "object":
		elementType := StringType
		if spec.AdditionalProperties != nil {
			et, err := t.bindType(*spec.AdditionalProperties)
			if err != nil {
				return nil, err
			}
			elementType = et
		}

		typ, ok := t.maps[elementType]
		if !ok {
			typ = &MapType{ElementType: elementType}
			t.maps[elementType] = typ
		}
		return typ, nil
	default:
		return nil, errors.Errorf("unknown type kind %v", spec.Type)
	}
}

func bindConstValue(value interface{}, typ Type) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	switch typ {
	case BoolType:
		if _, ok := value.(bool); !ok {
			return nil, errors.Errorf("invalid constant of type %T for boolean property", value)
		}
	case IntType:
		v, ok := value.(float64)
		if !ok {
			return nil, errors.Errorf("invalid constant of type %T for integer property", value)
		}
		if math.Trunc(v) != v || v < math.MinInt32 || v > math.MaxInt32 {
			return nil, errors.Errorf("invalid constant of type number for integer property")
		}
		value = int32(v)
	case NumberType:
		if _, ok := value.(float64); !ok {
			return nil, errors.Errorf("invalid constant of type %T for number property", value)
		}
	case StringType:
		if _, ok := value.(string); !ok {
			return nil, errors.Errorf("invalid constant of type %T for string property", value)
		}
	default:
		return nil, errors.Errorf("constant values may only be provided for boolean, integer, number, and string properties")
	}

	return value, nil
}

func bindDefaultValue(value interface{}, spec *DefaultSpec, typ Type) (*DefaultValue, error) {
	if value == nil && spec == nil {
		return nil, nil
	}

	if value != nil {
		switch typ {
		case BoolType:
			if _, ok := value.(bool); !ok {
				return nil, errors.Errorf("invalid default of type %T for boolean property", value)
			}
		case IntType:
			v, ok := value.(float64)
			if !ok {
				return nil, errors.Errorf("invalid default of type %T for integer property", value)
			}
			if math.Trunc(v) != v || v < math.MinInt32 || v > math.MaxInt32 {
				return nil, errors.Errorf("invalid default of type number for integer property")
			}
			value = int32(v)
		case NumberType:
			if _, ok := value.(float64); !ok {
				return nil, errors.Errorf("invalid default of type %T for number property", value)
			}
		case StringType:
			if _, ok := value.(string); !ok {
				return nil, errors.Errorf("invalid default of type %T for string property", value)
			}
		default:
			return nil, errors.Errorf("default values may only be provided for boolean, integer, number, and string properties")
		}
	}

	dv := &DefaultValue{Value: value}
	if spec != nil {
		language := make(map[string]interface{})
		for name, raw := range spec.Language {
			language[name] = raw
		}

		dv.Environment, dv.Language = spec.Environment, language
	}
	return dv, nil
}

// bindProperties binds the map of property specs and list of required properties into a sorted list of properties and
// a lookup table.
func (t *types) bindProperties(properties map[string]PropertySpec,
	required []string) ([]*Property, map[string]*Property, error) {

	// Bind property types and constant or default values.
	propertyMap := map[string]*Property{}
	var result []*Property
	for name, spec := range properties {
		typ, err := t.bindType(spec.TypeSpec)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error binding type for property %s", name)
		}

		cv, err := bindConstValue(spec.Const, typ)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error binding constant value for property %s", name)
		}

		dv, err := bindDefaultValue(spec.Default, spec.DefaultInfo, typ)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error binding default value for property %s", name)
		}

		language := make(map[string]interface{})
		for name, raw := range spec.Language {
			language[name] = raw
		}

		p := &Property{
			Name:               name,
			Comment:            spec.Description,
			Type:               typ,
			ConstValue:         cv,
			DefaultValue:       dv,
			DeprecationMessage: spec.DeprecationMessage,
			Language:           language,
			Secret:             spec.Secret,
		}

		propertyMap[name], result = p, append(result, p)
	}

	// Compute required properties.
	for _, name := range required {
		p, ok := propertyMap[name]
		if !ok {
			return nil, nil, errors.Errorf("unknown required property %s", name)
		}
		p.IsRequired = true
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, propertyMap, nil
}

func (t *types) bindObjectTypeDetails(obj *ObjectType, token string, spec ObjectTypeSpec) error {
	properties, propertyMap, err := t.bindProperties(spec.Properties, spec.Required)
	if err != nil {
		return err
	}

	language := make(map[string]interface{})
	for name, raw := range spec.Language {
		language[name] = raw
	}

	obj.Package = t.pkg
	obj.Token = token
	obj.Comment = spec.Description
	obj.Language = language
	obj.Properties = properties
	obj.properties = propertyMap
	return nil
}

func (t *types) bindObjectType(token string, spec ObjectTypeSpec) (*ObjectType, error) {
	obj := &ObjectType{}
	if err := t.bindObjectTypeDetails(obj, token, spec); err != nil {
		return nil, err
	}
	return obj, nil
}

func bindTypes(pkg *Package, objects map[string]ObjectTypeSpec) (*types, error) {
	typs := &types{
		pkg:     pkg,
		objects: map[string]*ObjectType{},
		arrays:  map[Type]*ArrayType{},
		maps:    map[Type]*MapType{},
		unions:  map[string]*UnionType{},
		tokens:  map[string]*TokenType{},
	}

	// Declare object types before processing properties.
	for token, spec := range objects {
		if spec.Type != "object" {
			return nil, errors.Errorf("type %s must be an object, not a %s", token, spec.Type)
		}

		// It's important that we set the token here. This package interns types so that they can be equality-compared
		// for identity. Types are interned based on their string representation, and the string representation of an
		// object type is its token. While this doesn't affect object types directly, it breaks the interning of types
		// that reference object types (e.g. arrays, maps, unions)
		typs.objects[token] = &ObjectType{Token: token}
	}

	// Process properties.
	for token, spec := range objects {
		if err := typs.bindObjectTypeDetails(typs.objects[token], token, spec); err != nil {
			return nil, errors.Wrapf(err, "failed to bind type %s", token)
		}
	}

	return typs, nil
}

func bindConfig(spec ConfigSpec, types *types) ([]*Property, error) {
	properties, _, err := types.bindProperties(spec.Variables, spec.Required)
	return properties, err
}

func bindResource(token string, spec ResourceSpec, types *types) (*Resource, error) {
	properties, _, err := types.bindProperties(spec.Properties, spec.Required)
	if err != nil {
		return nil, errors.Wrap(err, "failed to bind properties")
	}

	inputProperties, _, err := types.bindProperties(spec.InputProperties, spec.RequiredInputs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to bind properties")
	}

	var stateInputs *ObjectType
	if spec.StateInputs != nil {
		si, err := types.bindObjectType(token+"Args", *spec.StateInputs)
		if err != nil {
			return nil, errors.Wrap(err, "error binding inputs")
		}
		stateInputs = si
	}

	var aliases []*Alias
	for _, a := range spec.Aliases {
		aliases = append(aliases, &Alias{Name: a.Name, Project: a.Project, Type: a.Type})
	}

	language := make(map[string]interface{})
	for name, raw := range spec.Language {
		language[name] = raw
	}

	return &Resource{
		Package:            types.pkg,
		Token:              token,
		Comment:            spec.Description,
		InputProperties:    inputProperties,
		Properties:         properties,
		StateInputs:        stateInputs,
		Aliases:            aliases,
		DeprecationMessage: spec.DeprecationMessage,
		Language:           language,
	}, nil
}

func bindProvider(pkgName string, spec ResourceSpec, types *types) (*Resource, error) {
	res, err := bindResource("pulumi:providers:"+pkgName, spec, types)
	if err != nil {
		return nil, errors.Wrap(err, "error binding provider")
	}
	res.IsProvider = true
	return res, nil
}

func bindResources(specs map[string]ResourceSpec, types *types) ([]*Resource, map[string]*Resource, error) {
	resourceTable := map[string]*Resource{}
	var resources []*Resource
	for token, spec := range specs {
		res, err := bindResource(token, spec, types)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error binding resource %v", token)
		}
		resourceTable[token] = res
		resources = append(resources, res)
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Token < resources[j].Token
	})

	return resources, resourceTable, nil
}

func bindFunction(token string, spec FunctionSpec, types *types) (*Function, error) {
	var inputs *ObjectType
	if spec.Inputs != nil {
		ins, err := types.bindObjectType(token+"Args", *spec.Inputs)
		if err != nil {
			return nil, errors.Wrap(err, "error binding inputs")
		}
		inputs = ins
	}

	var outputs *ObjectType
	if spec.Outputs != nil {
		outs, err := types.bindObjectType(token+"Result", *spec.Outputs)
		if err != nil {
			return nil, errors.Wrap(err, "error binding inputs")
		}
		outputs = outs
	}

	language := make(map[string]interface{})
	for name, raw := range spec.Language {
		language[name] = raw
	}

	return &Function{
		Package:            types.pkg,
		Token:              token,
		Comment:            spec.Description,
		Inputs:             inputs,
		Outputs:            outputs,
		DeprecationMessage: spec.DeprecationMessage,
		Language:           language,
	}, nil
}

func bindFunctions(specs map[string]FunctionSpec, types *types) ([]*Function, map[string]*Function, error) {
	functionTable := map[string]*Function{}
	var functions []*Function
	for token, spec := range specs {
		f, err := bindFunction(token, spec, types)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error binding function %v", token)
		}
		functionTable[token] = f
		functions = append(functions, f)
	}

	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Token < functions[j].Token
	})

	return functions, functionTable, nil
}
