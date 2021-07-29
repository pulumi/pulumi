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
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// TODO:
// - Providerless packages

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
	_, ok := plainType(t).(primitiveType)
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
	// Package is the type's package.
	Package *Package
	// Token is the type's Pulumi type token.
	Token string
	// Comment is the description of the type, if any.
	Comment string
	// Elements are the predefined enum values.
	Elements []*Enum
	// ElementType is the underlying type for the enum.
	ElementType Type
}

// Enum contains information about an enum.
type Enum struct {
	// Value is the value of the enum.
	Value interface{}
	// Comment is the description for the enum value.
	Comment string
	// Name for the enum.
	Name string
	// DeprecationMessage indicates whether or not the value is deprecated.
	DeprecationMessage string
}

func (t *EnumType) String() string {
	return t.Token
}

func (*EnumType) isType() {}

// UnionType represents values that may be any one of a specified set of types.
type UnionType struct {
	// ElementTypes are the allowable types for the union type.
	ElementTypes []Type
	// DefaultType is the default type, if any, for the union type. This can be used by targets that do not support
	// unions, or in positions where unions are not appropriate.
	DefaultType Type
	// Discriminator informs the consumer of an alternative schema based on the value associated with it.
	Discriminator string
	// Mapping is an optional object to hold mappings between payload values and schema names or references.
	Mapping map[string]string
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

	// InputShape is the input shape for this object. Only valid if IsPlainShape returns true.
	InputShape *ObjectType
	// PlainShape is the plain shape for this object. Only valid if IsInputShape returns true.
	PlainShape *ObjectType

	properties map[string]*Property
}

// IsPlainShape returns true if this object type is the plain shape of a (plain, input)
// pair. The plain shape of an object does not contain *InputType values and only
// references other plain shapes.
func (t *ObjectType) IsPlainShape() bool {
	return t.PlainShape == nil
}

// IsInputShape returns true if this object type is the plain shape of a (plain, input)
// pair. The input shape of an object may contain *InputType values and may
// reference other input shapes.
func (t *ObjectType) IsInputShape() bool {
	return t.PlainShape != nil
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
	if t.PlainShape != nil {
		return t.Token + "•Input"
	}
	return t.Token
}

func (*ObjectType) isType() {}

type ResourceType struct {
	// Token is the type's Pulumi type token.
	Token string
	// Resource is the type's underlying resource.
	Resource *Resource
}

func (t *ResourceType) String() string {
	return t.Token
}

func (t *ResourceType) isType() {}

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

// InputType represents a type that accepts either a prompt value or an output value.
type InputType struct {
	// ElementType is the element type of the input.
	ElementType Type
}

func (t *InputType) String() string {
	return fmt.Sprintf("Input<%v>", t.ElementType)
}

func (*InputType) isType() {}

// OptionalType represents a type that accepts an optional value.
type OptionalType struct {
	// ElementType is the element type of the input.
	ElementType Type
}

func (t *OptionalType) String() string {
	return fmt.Sprintf("Optional<%v>", t.ElementType)
}

func (*OptionalType) isType() {}

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
	// DeprecationMessage indicates whether or not the property is deprecated.
	DeprecationMessage string
	// Language specifies additional language-specific data about the property.
	Language map[string]interface{}
	// Secret is true if the property is secret (default false).
	Secret bool
}

// IsRequired returns true if this property is required (i.e. its type is not Optional).
func (p *Property) IsRequired() bool {
	_, optional := p.Type.(*OptionalType)
	return !optional
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
	// IsComponent indicates whether the resource is a ComponentResource.
	IsComponent bool
	// Methods is the list of methods for the resource.
	Methods []*Method
}

type Method struct {
	// Name is the name of the method.
	Name string
	// Function is the function definition for the method.
	Function *Function
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
	// IsMethod indicates whether the function is a method of a resource.
	IsMethod bool
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

	resourceTable     map[string]*Resource
	resourceTypeTable map[string]*ResourceType
	functionTable     map[string]*Function
	typeTable         map[string]Type

	importedLanguages map[string]struct{}
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
	if pkg.importedLanguages == nil {
		pkg.importedLanguages = map[string]struct{}{}
	}

	any := false
	for lang := range languages {
		if _, ok := pkg.importedLanguages[lang]; !ok {
			any = true
			break
		}
	}
	if !any {
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

	for lang := range languages {
		pkg.importedLanguages[lang] = struct{}{}
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
		if pkg.moduleFormat == nil {
			pkg.moduleFormat = defaultModuleFormat
		}

		matches := pkg.moduleFormat.FindStringSubmatch(components[1])
		if len(matches) < 2 || strings.HasPrefix(matches[1], "index") {
			return ""
		}
		return matches[1]
	}
}

func (pkg *Package) TokenToRuntimeModule(tok string) string {
	// token := pkg ":" module ":" member

	components := strings.Split(tok, ":")
	if len(components) != 3 {
		return ""
	}
	return components[1]
}

func (pkg *Package) GetResource(token string) (*Resource, bool) {
	r, ok := pkg.resourceTable[token]
	return r, ok
}

func (pkg *Package) GetFunction(token string) (*Function, bool) {
	f, ok := pkg.functionTable[token]
	return f, ok
}

func (pkg *Package) GetResourceType(token string) (*ResourceType, bool) {
	t, ok := pkg.resourceTypeTable[token]
	return t, ok
}

func (pkg *Package) GetType(token string) (Type, bool) {
	t, ok := pkg.typeTable[token]
	return t, ok
}

func (pkg *Package) MarshalSpec() (spec *PackageSpec, err error) {
	version := ""
	if pkg.Version != nil {
		version = pkg.Version.String()
	}

	var metadata *MetadataSpec
	if pkg.moduleFormat != nil {
		metadata = &MetadataSpec{ModuleFormat: pkg.moduleFormat.String()}
	}

	spec = &PackageSpec{
		Name:              pkg.Name,
		Version:           version,
		Description:       pkg.Description,
		Keywords:          pkg.Keywords,
		Homepage:          pkg.Homepage,
		License:           pkg.License,
		Attribution:       pkg.Attribution,
		Repository:        pkg.Repository,
		LogoURL:           pkg.LogoURL,
		PluginDownloadURL: pkg.PluginDownloadURL,
		Meta:              metadata,
		Types:             map[string]ComplexTypeSpec{},
		Resources:         map[string]ResourceSpec{},
		Functions:         map[string]FunctionSpec{},
	}

	spec.Config.Required, spec.Config.Variables, err = pkg.marshalProperties(pkg.Config, true)
	if err != nil {
		return nil, fmt.Errorf("marshaling package config: %w", err)
	}

	spec.Provider, err = pkg.marshalResource(pkg.Provider)
	if err != nil {
		return nil, fmt.Errorf("marshaling provider: %w", err)
	}

	for _, t := range pkg.Types {
		switch t := t.(type) {
		case *ObjectType:
			if t.IsInputShape() {
				continue
			}

			// Use the input shape when marshaling in order to get the plain annotations right.
			o, err := pkg.marshalObject(t.InputShape, false)
			if err != nil {
				return nil, fmt.Errorf("marshaling type '%v': %w", t.Token, err)
			}
			spec.Types[t.Token] = o
		case *EnumType:
			spec.Types[t.Token] = pkg.marshalEnum(t)
		}
	}

	for _, res := range pkg.Resources {
		r, err := pkg.marshalResource(res)
		if err != nil {
			return nil, fmt.Errorf("marshaling resource '%v': %w", res.Token, err)
		}
		spec.Resources[res.Token] = r
	}

	for _, fn := range pkg.Functions {
		f, err := pkg.marshalFunction(fn)
		if err != nil {
			return nil, fmt.Errorf("marshaling function '%v': %w", fn.Token, err)
		}
		spec.Functions[fn.Token] = f
	}

	return spec, nil
}

func (pkg *Package) MarshalJSON() ([]byte, error) {
	spec, err := pkg.MarshalSpec()
	if err != nil {
		return nil, err
	}
	return jsonMarshal(spec)
}

func (pkg *Package) MarshalYAML() ([]byte, error) {
	spec, err := pkg.MarshalSpec()
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(spec); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (pkg *Package) marshalObjectData(
	comment string, properties []*Property, language map[string]interface{}, plain bool) (ObjectTypeSpec, error) {

	required, props, err := pkg.marshalProperties(properties, plain)
	if err != nil {
		return ObjectTypeSpec{}, err
	}

	lang, err := marshalLanguage(language)
	if err != nil {
		return ObjectTypeSpec{}, err
	}

	return ObjectTypeSpec{
		Description: comment,
		Properties:  props,
		Type:        "object",
		Required:    required,
		Language:    lang,
	}, nil
}

func (pkg *Package) marshalObject(t *ObjectType, plain bool) (ComplexTypeSpec, error) {
	data, err := pkg.marshalObjectData(t.Comment, t.Properties, t.Language, plain)
	if err != nil {
		return ComplexTypeSpec{}, err
	}
	return ComplexTypeSpec{ObjectTypeSpec: data}, nil
}

func (pkg *Package) marshalEnum(t *EnumType) ComplexTypeSpec {
	values := make([]EnumValueSpec, len(t.Elements))
	for i, el := range t.Elements {
		values[i] = EnumValueSpec{
			Name:               el.Name,
			Description:        el.Comment,
			Value:              el.Value,
			DeprecationMessage: el.DeprecationMessage,
		}
	}

	return ComplexTypeSpec{
		ObjectTypeSpec: ObjectTypeSpec{Type: pkg.marshalType(t.ElementType, false).Type},
		Enum:           values,
	}
}

func (pkg *Package) marshalResource(r *Resource) (ResourceSpec, error) {
	object, err := pkg.marshalObjectData(r.Comment, r.Properties, r.Language, true)
	if err != nil {
		return ResourceSpec{}, fmt.Errorf("marshaling properties: %w", err)
	}

	requiredInputs, inputs, err := pkg.marshalProperties(r.InputProperties, false)
	if err != nil {
		return ResourceSpec{}, fmt.Errorf("marshaling input properties: %w", err)
	}

	var stateInputs *ObjectTypeSpec
	if r.StateInputs != nil {
		o, err := pkg.marshalObject(r.StateInputs, false)
		if err != nil {
			return ResourceSpec{}, fmt.Errorf("marshaling state inputs: %w", err)
		}
		stateInputs = &o.ObjectTypeSpec
	}

	var aliases []AliasSpec
	for _, a := range r.Aliases {
		aliases = append(aliases, AliasSpec{
			Name:    a.Name,
			Project: a.Project,
			Type:    a.Type,
		})
	}

	var methods map[string]string
	if len(r.Methods) != 0 {
		methods = map[string]string{}
		for _, m := range r.Methods {
			methods[m.Name] = m.Function.Token
		}
	}

	return ResourceSpec{
		ObjectTypeSpec:     object,
		InputProperties:    inputs,
		RequiredInputs:     requiredInputs,
		StateInputs:        stateInputs,
		Aliases:            aliases,
		DeprecationMessage: r.DeprecationMessage,
		IsComponent:        r.IsComponent,
		Methods:            methods,
	}, nil
}

func (pkg *Package) marshalFunction(f *Function) (FunctionSpec, error) {
	var inputs *ObjectTypeSpec
	if f.Inputs != nil {
		ins, err := pkg.marshalObject(f.Inputs, true)
		if err != nil {
			return FunctionSpec{}, fmt.Errorf("marshaling inputs: %w", err)
		}
		inputs = &ins.ObjectTypeSpec
	}

	var outputs *ObjectTypeSpec
	if f.Outputs != nil {
		outs, err := pkg.marshalObject(f.Outputs, true)
		if err != nil {
			return FunctionSpec{}, fmt.Errorf("marshaloutg outputs: %w", err)
		}
		outputs = &outs.ObjectTypeSpec
	}

	lang, err := marshalLanguage(f.Language)
	if err != nil {
		return FunctionSpec{}, err
	}

	return FunctionSpec{
		Description: f.Comment,
		Inputs:      inputs,
		Outputs:     outputs,
		Language:    lang,
	}, nil
}

func (pkg *Package) marshalProperties(props []*Property, plain bool) (required []string, specs map[string]PropertySpec,
	err error) {

	if len(props) == 0 {
		return
	}

	specs = make(map[string]PropertySpec, len(props))
	for _, p := range props {
		typ := p.Type
		if t, optional := typ.(*OptionalType); optional {
			typ = t.ElementType
		} else {
			required = append(required, p.Name)
		}

		var defaultValue interface{}
		var defaultSpec *DefaultSpec
		if p.DefaultValue != nil {
			defaultValue = p.DefaultValue.Value
			if len(p.DefaultValue.Environment) != 0 || len(p.DefaultValue.Language) != 0 {
				lang, err := marshalLanguage(p.DefaultValue.Language)
				if err != nil {
					return nil, nil, fmt.Errorf("property '%v': %w", p.Name, err)
				}

				defaultSpec = &DefaultSpec{
					Environment: p.DefaultValue.Environment,
					Language:    lang,
				}
			}
		}

		lang, err := marshalLanguage(p.Language)
		if err != nil {
			return nil, nil, fmt.Errorf("property '%v': %w", p.Name, err)
		}

		specs[p.Name] = PropertySpec{
			TypeSpec:           pkg.marshalType(typ, plain),
			Description:        p.Comment,
			Const:              p.ConstValue,
			Default:            defaultValue,
			DefaultInfo:        defaultSpec,
			DeprecationMessage: p.DeprecationMessage,
			Language:           lang,
			Secret:             p.Secret,
		}
	}
	return required, specs, nil
}

// marshalType marshals the given type into a TypeSpec. If plain is true, then the type is being marshaled within a
// plain type context (e.g. a resource output property or a function input/output object type), and therefore does not
// require `Plain` annotations (hence the odd-looking `Plain: !plain` fields below).
func (pkg *Package) marshalType(t Type, plain bool) TypeSpec {
	switch t := t.(type) {
	case *InputType:
		el := pkg.marshalType(t.ElementType, plain)
		el.Plain = false
		return el
	case *ArrayType:
		el := pkg.marshalType(t.ElementType, plain)
		return TypeSpec{
			Type:  "array",
			Items: &el,
			Plain: !plain,
		}
	case *MapType:
		el := pkg.marshalType(t.ElementType, plain)
		return TypeSpec{
			Type:                 "object",
			AdditionalProperties: &el,
			Plain:                !plain,
		}
	case *UnionType:
		oneOf := make([]TypeSpec, len(t.ElementTypes))
		for i, el := range t.ElementTypes {
			oneOf[i] = pkg.marshalType(el, plain)
		}

		defaultType := ""
		if t.DefaultType != nil {
			defaultType = pkg.marshalType(t.DefaultType, plain).Type
		}

		var discriminator *DiscriminatorSpec
		if t.Discriminator != "" {
			discriminator = &DiscriminatorSpec{
				PropertyName: t.Discriminator,
				Mapping:      t.Mapping,
			}
		}

		return TypeSpec{
			Type:          defaultType,
			OneOf:         oneOf,
			Discriminator: discriminator,
			Plain:         !plain,
		}
	case *ObjectType:
		return TypeSpec{Ref: pkg.marshalTypeRef(t.Package, "types", t.Token)}
	case *EnumType:
		return TypeSpec{Ref: pkg.marshalTypeRef(t.Package, "types", t.Token)}
	case *ResourceType:
		return TypeSpec{Ref: pkg.marshalTypeRef(t.Resource.Package, "resources", t.Token)}
	case *TokenType:
		var defaultType string
		if t.UnderlyingType != nil {
			defaultType = pkg.marshalType(t.UnderlyingType, plain).Type
		}

		return TypeSpec{
			Type: defaultType,
			Ref:  t.Token,
		}
	default:
		switch t {
		case BoolType:
			return TypeSpec{Type: "boolean"}
		case StringType:
			return TypeSpec{Type: "string"}
		case IntType:
			return TypeSpec{Type: "integer"}
		case NumberType:
			return TypeSpec{Type: "number"}
		case AnyType:
			return TypeSpec{Ref: "pulumi.json#/Any"}
		case ArchiveType:
			return TypeSpec{Ref: "pulumi.json#/Archive"}
		case AssetType:
			return TypeSpec{Ref: "pulumi.json#/Asset"}
		case JSONType:
			return TypeSpec{Ref: "pulumi.json#/Json"}
		default:
			panic(fmt.Errorf("unexepcted type %v (%T)", t, t))
		}
	}
}

func (pkg *Package) marshalTypeRef(container *Package, section, token string) string {
	token = url.PathEscape(token)

	if container == pkg {
		return fmt.Sprintf("#/%s/%s", section, token)
	}

	// TODO(schema): this isn't quite right--it doesn't handle schemas sourced from URLs--but it's good enough for now.
	return fmt.Sprintf("/%s/%v/schema.json#/%s/%s", container.Name, container.Version, section, token)
}

func marshalLanguage(lang map[string]interface{}) (map[string]RawMessage, error) {
	if len(lang) == 0 {
		return nil, nil
	}

	result := map[string]RawMessage{}
	for name, data := range lang {
		bytes, err := jsonMarshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshaling %v language data: %w", name, err)
		}
		result[name] = RawMessage(bytes)
	}
	return result, nil
}

type RawMessage []byte

func (m RawMessage) MarshalJSON() ([]byte, error) {
	return []byte(m), nil
}

func (m *RawMessage) UnmarshalJSON(bytes []byte) error {
	*m = make([]byte, len(bytes))
	copy(*m, bytes)
	return nil
}

func (m RawMessage) MarshalYAML() ([]byte, error) {
	return []byte(m), nil
}

func (m *RawMessage) UnmarshalYAML(node *yaml.Node) error {
	var value interface{}
	if err := node.Decode(&value); err != nil {
		return err
	}
	bytes, err := jsonMarshal(value)
	if err != nil {
		return err
	}
	*m = bytes
	return nil
}

// TypeSpec is the serializable form of a reference to a type.
type TypeSpec struct {
	// Type is the primitive or composite type, if any. May be "bool", "integer", "number", "string", "array", or
	// "object".
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
	// Ref is a reference to a type in this or another document. For example, the built-in Archive, Asset, and Any
	// types are referenced as "pulumi.json#/Archive", "pulumi.json#/Asset", and "pulumi.json#/Any", respectively.
	// A type from this document is referenced as "#/types/pulumi:type:token".
	// A type from another document is referenced as "path#/types/pulumi:type:token", where path is of the form:
	//   "/provider/vX.Y.Z/schema.json" or "pulumi.json" or "http[s]://example.com/provider/vX.Y.Z/schema.json"
	// A resource from this document is referenced as "#/resources/pulumi:type:token".
	// A resource from another document is referenced as "path#/resources/pulumi:type:token", where path is of the form:
	//   "/provider/vX.Y.Z/schema.json" or "pulumi.json" or "http[s]://example.com/provider/vX.Y.Z/schema.json"
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	// AdditionalProperties, if set, describes the element type of an "object" (i.e. a string -> value map).
	AdditionalProperties *TypeSpec `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
	// Items, if set, describes the element type of an array.
	Items *TypeSpec `json:"items,omitempty" yaml:"items,omitempty"`
	// OneOf indicates that values of the type may be one of any of the listed types.
	OneOf []TypeSpec `json:"oneOf,omitempty" yaml:"oneOf,omitempty"`
	// Discriminator informs the consumer of an alternative schema based on the value associated with it.
	Discriminator *DiscriminatorSpec `json:"discriminator,omitempty" yaml:"discriminator,omitempty"`
	// Plain indicates that when used as an input, this type does not accept eventual values.
	Plain bool `json:"plain,omitempty" yaml:"plain,omitempty"`
}

// DiscriminatorSpec informs the consumer of an alternative schema based on the value associated with it.
type DiscriminatorSpec struct {
	// PropertyName is the name of the property in the payload that will hold the discriminator value.
	PropertyName string `json:"propertyName" yaml:"propertyName"`
	// Mapping is an optional object to hold mappings between payload values and schema names or references.
	Mapping map[string]string `json:"mapping,omitempty" yaml:"mapping,omitempty"`
}

// DefaultSpec is the serializable form of extra information about the default value for a property.
type DefaultSpec struct {
	// Environment specifies a set of environment variables to probe for a default value.
	Environment []string `json:"environment,omitempty" yaml:"environment,omitempty"`
	// Language specifies additional language-specific data about the default value.
	Language map[string]RawMessage `json:"language,omitempty" yaml:"language,omitempty"`
}

// PropertySpec is the serializable form of an object or resource property.
type PropertySpec struct {
	TypeSpec `yaml:",inline"`

	// Description is the description of the property, if any.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Const is the constant value for the property, if any. The type of the value must be assignable to the type of
	// the property.
	Const interface{} `json:"const,omitempty" yaml:"const,omitempty"`
	// Default is the default value for the property, if any. The type of the value must be assignable to the type of
	// the property.
	Default interface{} `json:"default,omitempty" yaml:"default,omitempty"`
	// DefaultInfo contains additional information about the property's default value, if any.
	DefaultInfo *DefaultSpec `json:"defaultInfo,omitempty" yaml:"defaultInfo,omitempty"`
	// DeprecationMessage indicates whether or not the property is deprecated.
	DeprecationMessage string `json:"deprecationMessage,omitempty" yaml:"deprecationMessage,omitempty"`
	// Language specifies additional language-specific data about the property.
	Language map[string]RawMessage `json:"language,omitempty" yaml:"language,omitempty"`
	// Secret specifies if the property is secret (default false).
	Secret bool `json:"secret,omitempty" yaml:"secret,omitempty"`
}

// ObjectTypeSpec is the serializable form of an object type.
type ObjectTypeSpec struct {
	// Description is the description of the type, if any.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Properties, if present, is a map from property name to PropertySpec that describes the type's properties.
	Properties map[string]PropertySpec `json:"properties,omitempty" yaml:"properties,omitempty"`
	// Type must be "object" if this is an object type, or the underlying type for an enum.
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
	// Required, if present, is a list of the names of an object type's required properties. These properties must be set
	// for inputs and will always be set for outputs.
	Required []string `json:"required,omitempty" yaml:"required,omitempty"`
	// Plain, was a list of the names of an object type's plain properties. This property is ignored: instead, property
	// types should be marked as plain where necessary.
	Plain []string `json:"plain,omitempty" yaml:"plain,omitempty"`
	// Language specifies additional language-specific data about the type.
	Language map[string]RawMessage `json:"language,omitempty" yaml:"language,omitempty"`
}

// ComplexTypeSpec is the serializable form of an object or enum type.
type ComplexTypeSpec struct {
	ObjectTypeSpec `yaml:",inline"`

	// Enum, if present, is the list of possible values for an enum type.
	Enum []EnumValueSpec `json:"enum,omitempty" yaml:"enum,omitempty"`
}

// EnumValuesSpec is the serializable form of the values metadata associated with an enum type.
type EnumValueSpec struct {
	// Name, if present, overrides the name of the enum value that would usually be derived from the value.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Description of the enum value.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Value is the enum value itself.
	Value interface{} `json:"value" yaml:"value"`
	// DeprecationMessage indicates whether or not the value is deprecated.
	DeprecationMessage string `json:"deprecationMessage,omitempty" yaml:"deprecationMessage,omitempty"`
}

// AliasSpec is the serializable form of an alias description.
type AliasSpec struct {
	// Name is the name portion of the alias, if any.
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
	// Project is the project portion of the alias, if any.
	Project *string `json:"project,omitempty" yaml:"project,omitempty"`
	// Type is the type portion of the alias, if any.
	Type *string `json:"type,omitempty" yaml:"type,omitempty"`
}

// ResourceSpec is the serializable form of a resource description.
type ResourceSpec struct {
	ObjectTypeSpec `yaml:",inline"`

	// InputProperties is a map from property name to PropertySpec that describes the resource's input properties.
	InputProperties map[string]PropertySpec `json:"inputProperties,omitempty" yaml:"inputProperties,omitempty"`
	// RequiredInputs is a list of the names of the resource's required input properties.
	RequiredInputs []string `json:"requiredInputs,omitempty" yaml:"requiredInputs,omitempty"`
	// PlainInputs was a list of the names of the resource's plain input properties. This property is ignored:
	// instead, property types should be marked as plain where necessary.
	PlainInputs []string `json:"plainInputs,omitempty" yaml:"plainInputs,omitempty"`
	// StateInputs is an optional ObjectTypeSpec that describes additional inputs that mau be necessary to get an
	// existing resource. If this is unset, only an ID is necessary.
	StateInputs *ObjectTypeSpec `json:"stateInputs,omitempty" yaml:"stateInputs,omitempty"`
	// Aliases is the list of aliases for the resource.
	Aliases []AliasSpec `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	// DeprecationMessage indicates whether or not the resource is deprecated.
	DeprecationMessage string `json:"deprecationMessage,omitempty" yaml:"deprecationMessage,omitempty"`
	// IsComponent indicates whether the resource is a ComponentResource.
	IsComponent bool `json:"isComponent,omitempty" yaml:"isComponent,omitempty"`
	// Methods maps method names to functions in this schema.
	Methods map[string]string `json:"methods,omitempty" yaml:"methods,omitempty"`
}

// FunctionSpec is the serializable form of a function description.
type FunctionSpec struct {
	// Description is the description of the function, if any.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Inputs is the bag of input values for the function, if any.
	Inputs *ObjectTypeSpec `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	// Outputs is the bag of output values for the function, if any.
	Outputs *ObjectTypeSpec `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	// DeprecationMessage indicates whether or not the function is deprecated.
	DeprecationMessage string `json:"deprecationMessage,omitempty" yaml:"deprecationMessage,omitempty"`
	// Language specifies additional language-specific data about the function.
	Language map[string]RawMessage `json:"language,omitempty" yaml:"language,omitempty"`
}

// ConfigSpec is the serializable description of a package's configuration variables.
type ConfigSpec struct {
	// Variables is a map from variable name to PropertySpec that describes a package's configuration variables.
	Variables map[string]PropertySpec `json:"variables,omitempty" yaml:"variables,omitempty"`
	// Required is a list of the names of the package's required configuration variables.
	Required []string `json:"defaults,omitempty" yaml:"defaults,omitempty"`
}

// MetadataSpec contains information for the importer about this package.
type MetadataSpec struct {
	// ModuleFormat is a regex that is used by the importer to extract a module name from the module portion of a
	// type token. Packages that use the module format "namespace1/namespace2/.../namespaceN" do not need to specify
	// a format. The regex must define one capturing group that contains the module name, which must be formatted as
	// "namespace1/namespace2/...namespaceN".
	ModuleFormat string `json:"moduleFormat,omitempty" yaml:"moduleFormat,omitempty"`
}

// PackageSpec is the serializable description of a Pulumi package.
type PackageSpec struct {
	// Name is the unqualified name of the package (e.g. "aws", "azure", "gcp", "kubernetes", "random")
	Name string `json:"name" yaml:"name"`
	// Version is the version of the package. The version must be valid semver.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// Description is the description of the package.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Keywords is the list of keywords that are associated with the package, if any.
	Keywords []string `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	// Homepage is the package's homepage.
	Homepage string `json:"homepage,omitempty" yaml:"homepage,omitempty"`
	// License indicates which license is used for the package's contents.
	License string `json:"license,omitempty" yaml:"license,omitempty"`
	// Attribution allows freeform text attribution of derived work, if needed.
	Attribution string `json:"attribution,omitempty" yaml:"attribution,omitempty"`
	// Repository is the URL at which the source for the package can be found.
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`
	// LogoURL is the URL for the package's logo, if any.
	LogoURL string `json:"logoUrl,omitempty" yaml:"logoUrl,omitempty"`
	// PluginDownloadURL is the URL to use to acquire the provider plugin binary, if any.
	PluginDownloadURL string `json:"pluginDownloadURL,omitempty" yaml:"pluginDownloadURL,omitempty"`

	// Meta contains information for the importer about this package.
	Meta *MetadataSpec `json:"meta,omitempty" yaml:"meta,omitempty"`

	// Config describes the set of configuration variables defined by this package.
	Config ConfigSpec `json:"config" yaml:"config"`
	// Types is a map from type token to ComplexTypeSpec that describes the set of complex types (ie. object, enum)
	// defined by this package.
	Types map[string]ComplexTypeSpec `json:"types,omitempty" yaml:"types,omitempty"`
	// Provider describes the provider type for this package.
	Provider ResourceSpec `json:"provider" yaml:"provider"`
	// Resources is a map from type token to ResourceSpec that describes the set of resources defined by this package.
	Resources map[string]ResourceSpec `json:"resources,omitempty" yaml:"resources,omitempty"`
	// Functions is a map from token to FunctionSpec that describes the set of functions defined by this package.
	Functions map[string]FunctionSpec `json:"functions,omitempty" yaml:"functions,omitempty"`
	// Language specifies additional language-specific data about the package.
	Language map[string]RawMessage `json:"language,omitempty" yaml:"language,omitempty"`
}

var defaultModuleFormat = regexp.MustCompile("(.*)")

// importSpec converts a serializable PackageSpec into a Package. This function includes a loader parameter which
// works as a singleton -- if it is nil, a new loader is instantiated, else the provided loader is used. This avoids
// breaking downstream consumers of ImportSpec while allowing us to extend schema support to external packages.
func importSpec(spec PackageSpec, languages map[string]Language, loader Loader) (*Package, error) {
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

	// We want to use the same loader instance for all referenced packages, so only instantiate the loader if the
	// reference is nil.
	if loader == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		ctx, err := plugin.NewContext(nil, nil, nil, nil, cwd, nil, false, nil)
		if err != nil {
			return nil, err
		}
		defer contract.IgnoreClose(ctx)

		loader = NewPluginLoader(ctx.Host)
	}

	types, err := bindTypes(pkg, spec.Types, loader)
	if err != nil {
		return nil, errors.Wrap(err, "binding types")
	}

	config, err := bindConfig(spec.Config, types)
	if err != nil {
		return nil, errors.Wrap(err, "binding config")
	}

	functions, functionTable, err := bindFunctions(spec.Functions, types)
	if err != nil {
		return nil, errors.Wrap(err, "binding functions")
	}

	provider, err := bindProvider(spec.Name, spec.Provider, types, functionTable)
	if err != nil {
		return nil, errors.Wrap(err, "binding provider")
	}

	resources, resourceTable, err := bindResources(spec.Resources, types, functionTable)
	if err != nil {
		return nil, errors.Wrap(err, "binding resources")
	}

	// Build the type list.
	var typeList []Type
	for _, t := range types.resources {
		typeList = append(typeList, t)
	}
	for _, t := range types.objects {
		// t is a plain shape: add it and its coresponding input shape to the type list.
		typeList = append(typeList, t)
		typeList = append(typeList, t.InputShape)
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
	for _, t := range types.enums {
		typeList = append(typeList, t)
	}

	sort.Slice(typeList, func(i, j int) bool {
		return typeList[i].String() < typeList[j].String()
	})

	language := make(map[string]interface{})
	for name, raw := range spec.Language {
		language[name] = json.RawMessage(raw)
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
		typeTable:         types.named,
		resourceTypeTable: types.resources,
	}
	if err := pkg.ImportLanguages(languages); err != nil {
		return nil, err
	}
	return pkg, nil

}

// ImportSpec converts a serializable PackageSpec into a Package.
func ImportSpec(spec PackageSpec, languages map[string]Language) (*Package, error) {
	// Call the internal implementation that includes a loader parameter.
	return importSpec(spec, languages, nil)
}

// types facilitates interning (only storing a single reference to an object) during schema processing. The fields
// correspond to fields in the schema, and are populated during the binding process.
type types struct {
	pkg    *Package
	loader Loader

	resources map[string]*ResourceType
	objects   map[string]*ObjectType
	arrays    map[Type]*ArrayType
	maps      map[Type]*MapType
	unions    map[string]*UnionType
	tokens    map[string]*TokenType
	enums     map[string]*EnumType
	named     map[string]Type // objects and enums
	inputs    map[Type]*InputType
	optionals map[Type]*OptionalType
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

// typeSpecRef contains the parsed fields from a type spec reference.
type typeSpecRef struct {
	URL *url.URL // The parsed URL

	Package string          // The package component of the schema ref
	Version *semver.Version // The version component of the schema ref

	Kind  string // The kind of reference: 'resources', 'types', or 'provider'
	Token string // The type token
}

const (
	resourcesRef = "resources"
	typesRef     = "types"
	providerRef  = "provider"
)

// Regex used to parse external schema paths. This is declared at the package scope to avoid repeated recompilation.
var refPathRegex = regexp.MustCompile(`^/?(?P<package>\w+)/(?P<version>v[^/]*)/schema\.json$`)

func (t *types) parseTypeSpecRef(ref string) (typeSpecRef, error) {
	parsedURL, err := url.Parse(ref)
	if err != nil {
		return typeSpecRef{}, errors.Wrapf(err, "failed to parse ref URL: %s", ref)
	}

	// Parse the package name and version if the URL contains a path. If there is no path--if the URL is just a
	// fragment--then the reference refers to the package being bound.
	pkgName, pkgVersion := t.pkg.Name, t.pkg.Version
	if len(parsedURL.Path) > 0 {
		path, err := url.PathUnescape(parsedURL.Path)
		if err != nil {
			return typeSpecRef{}, errors.Wrapf(err, "failed to unescape path: %s", parsedURL.Path)
		}

		pathMatch := refPathRegex.FindStringSubmatch(path)
		if len(pathMatch) != 3 {
			return typeSpecRef{}, fmt.Errorf("failed to parse path: %s", path)
		}

		pkg, versionToken := pathMatch[1], pathMatch[2]
		version, err := semver.ParseTolerant(versionToken)
		if err != nil {
			return typeSpecRef{}, errors.Wrapf(err, "failed to parse package version: %s", versionToken)
		}

		pkgName, pkgVersion = pkg, &version
	}

	// Parse the fragment into a reference kind and token. The fragment is in one of two forms:
	// 1. #/provider
	// 2. #/(resources|types)/some:type:token
	//
	// Unfortunately, early code generators were lax and emitted unescaped backslashes in the type token, so we can't
	// just split on "/".
	fragment := path.Clean(parsedURL.EscapedFragment())
	if path.IsAbs(fragment) {
		fragment = fragment[1:]
	}

	kind, token := "", ""
	slash := strings.Index(fragment, "/")
	if slash == -1 {
		kind = fragment
	} else {
		kind, token = fragment[:slash], fragment[slash+1:]
	}

	switch kind {
	case "provider":
		if token != "" {
			return typeSpecRef{}, fmt.Errorf("invalid provider reference '%v'", ref)
		}
		token = "pulumi:providers:" + pkgName
	case "resources", "types":
		token, err = url.PathUnescape(token)
		if err != nil {
			return typeSpecRef{}, errors.Wrapf(err, "failed to unescape token: %s", token)
		}
	default:
		return typeSpecRef{}, fmt.Errorf("invalid type reference '%v'", ref)
	}

	return typeSpecRef{
		URL:     parsedURL,
		Package: pkgName,
		Version: pkgVersion,
		Kind:    kind,
		Token:   token,
	}, nil
}

func versionEquals(a, b *semver.Version) bool {
	// We treat "nil" as "unconstrained".
	if a == nil || b == nil {
		return true
	}
	return a.Equals(*b)
}

func (t *types) newInputType(elementType Type) Type {
	if _, ok := elementType.(*InputType); ok {
		return elementType
	}

	typ, ok := t.inputs[elementType]
	if !ok {
		typ = &InputType{ElementType: elementType}
		t.inputs[elementType] = typ
	}
	return typ
}

func (t *types) newOptionalType(elementType Type) Type {
	if _, ok := elementType.(*OptionalType); ok {
		return elementType
	}
	typ, ok := t.optionals[elementType]
	if !ok {
		typ = &OptionalType{ElementType: elementType}
		t.optionals[elementType] = typ
	}
	return typ
}

func (t *types) newMapType(elementType Type) Type {
	typ, ok := t.maps[elementType]
	if !ok {
		typ = &MapType{ElementType: elementType}
		t.maps[elementType] = typ
	}
	return typ
}

func (t *types) newArrayType(elementType Type) Type {
	typ, ok := t.arrays[elementType]
	if !ok {
		typ = &ArrayType{ElementType: elementType}
		t.arrays[elementType] = typ
	}
	return typ
}

func (t *types) newUnionType(
	elements []Type, defaultType Type, discriminator string, mapping map[string]string) *UnionType {
	union := &UnionType{
		ElementTypes:  elements,
		DefaultType:   defaultType,
		Discriminator: discriminator,
		Mapping:       mapping,
	}
	if typ, ok := t.unions[union.String()]; ok {
		return typ
	}
	t.unions[union.String()] = union
	return union
}

func (t *types) bindTypeSpecRef(spec TypeSpec, inputShape bool) (Type, error) {
	// Explicitly handle built-in types so that we don't have to handle this type of path during ref parsing.
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

	ref, err := t.parseTypeSpecRef(spec.Ref)
	if err != nil {
		return nil, err
	}

	// If this is a reference to an external sch
	referencesExternalSchema := ref.Package != t.pkg.Name || !versionEquals(ref.Version, t.pkg.Version)
	if referencesExternalSchema {
		pkg, err := t.loader.LoadPackage(ref.Package, ref.Version)
		if err != nil {
			return nil, errors.Wrapf(err, "resolving package %v", ref.URL)
		}

		switch ref.Kind {
		case typesRef:
			typ, ok := pkg.GetType(ref.Token)
			if !ok {
				return nil, fmt.Errorf("type %v not found in package %v", ref.Token, ref.Package)
			}
			if obj, ok := typ.(*ObjectType); ok && inputShape {
				typ = obj.InputShape
			}
			return typ, nil
		case resourcesRef, providerRef:
			typ, ok := pkg.GetResourceType(ref.Token)
			if !ok {
				return nil, fmt.Errorf("resource type %v not found in package %v", ref.Token, ref.Package)
			}
			return typ, nil
		}
	}

	switch ref.Kind {
	case typesRef:
		if typ, ok := t.objects[ref.Token]; ok {
			if inputShape {
				return typ.InputShape, nil
			}
			return typ, nil
		}
		if typ, ok := t.enums[ref.Token]; ok {
			return typ, nil
		}
		typ, ok := t.tokens[ref.Token]
		if !ok {
			typ = &TokenType{Token: ref.Token}
			if spec.Type != "" {
				ut, err := t.bindPrimitiveType(spec.Type)
				if err != nil {
					return nil, err
				}
				typ.UnderlyingType = ut
			}
			t.tokens[ref.Token] = typ
		}
		return typ, nil
	case resourcesRef, providerRef:
		typ, ok := t.resources[ref.Token]
		if !ok {
			typ = &ResourceType{Token: ref.Token}
			t.resources[ref.Token] = typ
		}
		return typ, nil
	default:
		return nil, errors.Errorf("failed to parse ref %s", spec.Ref)
	}
}

func (t *types) bindType(spec TypeSpec, inputShape bool) (result Type, err error) {
	if inputShape && !spec.Plain {
		defer func() {
			result = t.newInputType(result)
		}()
	}

	if spec.Ref != "" {
		return t.bindTypeSpecRef(spec, inputShape)
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
			e, err := t.bindType(spec, inputShape)
			if err != nil {
				return nil, err
			}
			elements[i] = e
		}

		var discriminator string
		var mapping map[string]string
		if spec.Discriminator != nil {
			discriminator = spec.Discriminator.PropertyName
			mapping = spec.Discriminator.Mapping
		}

		return t.newUnionType(elements, defaultType, discriminator, mapping), nil
	}

	// nolint: goconst
	switch spec.Type {
	case "boolean", "integer", "number", "string":
		return t.bindPrimitiveType(spec.Type)
	case "array":
		if spec.Items == nil {
			return nil, errors.Errorf("missing \"items\" property in type spec")
		}

		elementType, err := t.bindType(*spec.Items, inputShape)
		if err != nil {
			return nil, err
		}

		return t.newArrayType(elementType), nil
	case "object":
		elementType, err := t.bindType(TypeSpec{Type: "string"}, inputShape)
		if err != nil {
			return nil, err
		}
		if spec.AdditionalProperties != nil {
			et, err := t.bindType(*spec.AdditionalProperties, inputShape)
			if err != nil {
				return nil, err
			}
			elementType = et
		}

		return t.newMapType(elementType), nil
	default:
		return nil, errors.Errorf("unknown type kind %v", spec.Type)
	}
}

func plainType(typ Type) Type {
	for {
		switch t := typ.(type) {
		case *InputType:
			typ = t.ElementType
		case *OptionalType:
			typ = t.ElementType
		case *ObjectType:
			if t.PlainShape == nil {
				return t
			}
			typ = t.PlainShape
		default:
			return t
		}
	}
}

func bindConstValue(value interface{}, typ Type) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	switch plainType(typ) {
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
		typ = plainType(typ)
		switch typ := typ.(type) {
		case *UnionType:
			if typ.DefaultType != nil {
				return bindDefaultValue(value, spec, typ.DefaultType)
			}
			for _, elementType := range typ.ElementTypes {
				v, err := bindDefaultValue(value, spec, elementType)
				if err == nil {
					return v, nil
				}
			}
		case *EnumType:
			return bindDefaultValue(value, spec, typ.ElementType)
		}

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
			language[name] = json.RawMessage(raw)
		}

		dv.Environment, dv.Language = spec.Environment, language
	}
	return dv, nil
}

// bindProperties binds the map of property specs and list of required properties into a sorted list of properties and
// a lookup table.
func (t *types) bindProperties(properties map[string]PropertySpec,
	required []string, inputShape bool) ([]*Property, map[string]*Property, error) {

	// Bind property types and constant or default values.
	propertyMap := map[string]*Property{}
	var result []*Property
	for name, spec := range properties {
		typ, err := t.bindType(spec.TypeSpec, inputShape)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error binding type for property %q", name)
		}

		cv, err := bindConstValue(spec.Const, typ)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error binding constant value for property %q", name)
		}

		dv, err := bindDefaultValue(spec.Default, spec.DefaultInfo, typ)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error binding default value for property %q", name)
		}

		language := make(map[string]interface{})
		for name, raw := range spec.Language {
			language[name] = json.RawMessage(raw)
		}

		p := &Property{
			Name:               name,
			Comment:            spec.Description,
			Type:               t.newOptionalType(typ),
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
			return nil, nil, errors.Errorf("unknown required property %q", name)
		}
		p.Type = p.Type.(*OptionalType).ElementType
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, propertyMap, nil
}

func (t *types) bindObjectTypeDetails(obj *ObjectType, token string, spec ObjectTypeSpec) error {
	if len(spec.Plain) > 0 {
		return errors.New("plain has been removed; the property type must be marked as plain instead")
	}

	properties, propertyMap, err := t.bindProperties(spec.Properties, spec.Required, false)
	if err != nil {
		return err
	}

	inputProperties, inputPropertyMap, err := t.bindProperties(spec.Properties, spec.Required, true)
	if err != nil {
		return err
	}

	language := make(map[string]interface{})
	for name, raw := range spec.Language {
		language[name] = json.RawMessage(raw)
	}

	obj.Package = t.pkg
	obj.Token = token
	obj.Comment = spec.Description
	obj.Language = language
	obj.Properties = properties
	obj.properties = propertyMap

	obj.InputShape.Package = t.pkg
	obj.InputShape.Token = token
	obj.InputShape.Comment = spec.Description
	obj.InputShape.Language = language
	obj.InputShape.Properties = inputProperties
	obj.InputShape.properties = inputPropertyMap

	return nil
}

func (t *types) bindObjectType(token string, spec ObjectTypeSpec) (*ObjectType, error) {
	obj := &ObjectType{}
	obj.InputShape = &ObjectType{PlainShape: obj}
	if err := t.bindObjectTypeDetails(obj, token, spec); err != nil {
		return nil, err
	}
	return obj, nil
}

func (t *types) bindResourceTypeDetails(obj *ResourceType, token string) error {
	obj.Token = token
	return nil
}

func (t *types) bindResourceType(token string) (*ResourceType, error) {
	r := &ResourceType{}
	if err := t.bindResourceTypeDetails(r, token); err != nil {
		return nil, err
	}
	return r, nil
}

func (t *types) bindEnumTypeDetails(enum *EnumType, token string, spec ComplexTypeSpec) error {
	typ, err := t.bindPrimitiveType(spec.Type)
	if err != nil {
		return err
	}

	values, err := t.bindEnumValues(spec.Enum, typ)
	if err != nil {
		return err
	}

	enum.Package = t.pkg
	enum.Token = token
	enum.Elements = values
	enum.ElementType = typ
	enum.Comment = spec.Description

	return nil
}

func (t *types) bindEnumValues(values []EnumValueSpec, typ Type) ([]*Enum, error) {
	var enums []*Enum

	errorMessage := func(val interface{}, expectedType string) error {
		return fmt.Errorf("cannot assign enum value of type '%T' to enum of type '%s'", val, expectedType)
	}
	for _, spec := range values {
		switch typ {
		case StringType:
			if _, ok := spec.Value.(string); !ok {
				return nil, errorMessage(spec.Value, typ.String())
			}
		case IntType:
			v, ok := spec.Value.(float64)
			if !ok {
				return nil, errorMessage(spec.Value, typ.String())
			}
			if math.Trunc(v) != v || v < math.MinInt32 || v > math.MaxInt32 {
				return nil, errors.Errorf("cannot assign enum value of type 'number' to enum of type 'integer'")
			}
			spec.Value = int32(v)
		case NumberType:
			if _, ok := spec.Value.(float64); !ok {
				return nil, errorMessage(spec.Value, typ.String())
			}
		case BoolType:
			if _, ok := spec.Value.(bool); !ok {
				return nil, errorMessage(spec.Value, typ.String())
			}
		default:
			return nil, fmt.Errorf("enum values may only be of string, integer, number or boolean types")
		}
		enum := &Enum{
			Value:              spec.Value,
			Comment:            spec.Description,
			Name:               spec.Name,
			DeprecationMessage: spec.DeprecationMessage,
		}
		enums = append(enums, enum)
	}
	return enums, nil
}

func (t *types) bindEnumType(token string, spec ComplexTypeSpec) (*EnumType, error) {
	enum := &EnumType{}
	if err := t.bindEnumTypeDetails(enum, token, spec); err != nil {
		return nil, err
	}
	return enum, nil
}

func bindTypes(pkg *Package, complexTypes map[string]ComplexTypeSpec, loader Loader) (*types, error) {
	typs := &types{
		pkg:       pkg,
		loader:    loader,
		resources: map[string]*ResourceType{},
		objects:   map[string]*ObjectType{},
		arrays:    map[Type]*ArrayType{},
		maps:      map[Type]*MapType{},
		unions:    map[string]*UnionType{},
		tokens:    map[string]*TokenType{},
		enums:     map[string]*EnumType{},
		named:     map[string]Type{},
		inputs:    map[Type]*InputType{},
		optionals: map[Type]*OptionalType{},
	}

	// Declare object and enum types before processing properties.
	for token, spec := range complexTypes {
		if spec.Type == "object" {
			// It's important that we set the token here. This package interns types so that they can be equality-compared
			// for identity. Types are interned based on their string representation, and the string representation of an
			// object type is its token. While this doesn't affect object types directly, it breaks the interning of types
			// that reference object types (e.g. arrays, maps, unions)
			typ := &ObjectType{Token: token}
			typ.InputShape = &ObjectType{Token: token, PlainShape: typ}
			typs.objects[token] = typ
			typs.named[token] = typ
		} else if len(spec.Enum) > 0 {
			typ := &EnumType{Token: token}
			typs.enums[token] = typ
			typs.named[token] = typ

			// Bind enums before object types because object type generation depends on enum values to be present.
			if err := typs.bindEnumTypeDetails(typs.enums[token], token, spec); err != nil {
				return nil, errors.Wrapf(err, "failed to bind type %s", token)
			}
		}
	}

	// Process resources.
	for _, r := range pkg.Resources {
		typs.resources[r.Token] = &ResourceType{Token: r.Token}
	}

	// Process object types.
	for token, spec := range complexTypes {
		if spec.Type == "object" {
			if err := typs.bindObjectTypeDetails(
				typs.objects[token], token, spec.ObjectTypeSpec); err != nil {
				return nil, errors.Wrapf(err, "failed to bind type %s", token)
			}
		}
	}

	return typs, nil
}

func bindMethods(resourceToken string, methods map[string]string,
	functionTable map[string]*Function) ([]*Method, error) {

	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]*Method, 0, len(methods))
	for _, name := range names {
		token := methods[name]
		function, ok := functionTable[token]
		if !ok {
			return nil, errors.Errorf("unknown function %s for method %s", token, name)
		}
		if function.IsMethod {
			return nil, errors.Errorf("function %s for method %s is already a method", token, name)
		}
		idx := strings.LastIndex(function.Token, "/")
		if idx == -1 || function.Token[:idx] != resourceToken {
			return nil, errors.Errorf("invalid function token format %s for method %s", token, name)
		}
		if function.Inputs == nil || function.Inputs.Properties == nil || len(function.Inputs.Properties) == 0 ||
			function.Inputs.Properties[0].Name != "__self__" {
			return nil, errors.Errorf("function %s for method %s is missing __self__ parameter", token, name)
		}
		function.IsMethod = true
		result = append(result, &Method{
			Name:     name,
			Function: function,
		})
	}
	return result, nil
}

func bindConfig(spec ConfigSpec, types *types) ([]*Property, error) {
	properties, _, err := types.bindProperties(spec.Variables, spec.Required, false)
	return properties, err
}

func bindResource(token string, spec ResourceSpec, types *types,
	functionTable map[string]*Function) (*Resource, error) {
	if len(spec.Plain) > 0 {
		return nil, errors.New("plain has been removed; property types must be marked as plain instead")
	}
	if len(spec.PlainInputs) > 0 {
		return nil, errors.New("plainInputs has been removed; individual property types must be marked as plain instead")
	}

	properties, _, err := types.bindProperties(spec.Properties, spec.Required, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to bind properties")
	}

	inputProperties, _, err := types.bindProperties(spec.InputProperties, spec.RequiredInputs, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to bind properties")
	}

	methods, err := bindMethods(token, spec.Methods, functionTable)
	if err != nil {
		return nil, errors.Wrap(err, "failed to bind methods")
	}

	for _, method := range methods {
		if _, ok := spec.Properties[method.Name]; ok {
			return nil, errors.Errorf("property and method have the same name %s", method.Name)
		}
	}

	var stateInputs *ObjectType
	if spec.StateInputs != nil {
		si, err := types.bindObjectType(token+"Args", *spec.StateInputs)
		if err != nil {
			return nil, errors.Wrap(err, "error binding inputs")
		}
		stateInputs = si.InputShape
	}

	var aliases []*Alias
	for _, a := range spec.Aliases {
		aliases = append(aliases, &Alias{Name: a.Name, Project: a.Project, Type: a.Type})
	}

	language := make(map[string]interface{})
	for name, raw := range spec.Language {
		language[name] = json.RawMessage(raw)
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
		IsComponent:        spec.IsComponent,
		Methods:            methods,
	}, nil
}

func bindProvider(pkgName string, spec ResourceSpec, types *types,
	functionTable map[string]*Function) (*Resource, error) {
	res, err := bindResource("pulumi:providers:"+pkgName, spec, types, functionTable)
	if err != nil {
		return nil, errors.Wrap(err, "error binding provider")
	}
	res.IsProvider = true

	// Since non-primitive provider configuration is currently JSON serialized, we can't handle it without
	// modifying the path by which it's looked up. As a temporary workaround to enable access to config which
	// values which are primitives, we'll simply remove any properties for the provider resource which are not
	// strings, or types with an underlying type of string, before we generate the provider code.
	var stringProperties []*Property
	for _, prop := range res.Properties {
		typ := plainType(prop.Type)
		if tokenType, isTokenType := typ.(*TokenType); isTokenType {
			if tokenType.UnderlyingType != stringType {
				continue
			}
		} else {
			if typ != stringType {
				continue
			}
		}

		stringProperties = append(stringProperties, prop)
	}
	res.Properties = stringProperties

	types.resources[res.Token] = &ResourceType{
		Token:    res.Token,
		Resource: res,
	}

	return res, nil
}

func bindResources(specs map[string]ResourceSpec, types *types,
	functionTable map[string]*Function) ([]*Resource, map[string]*Resource, error) {
	resourceTable := map[string]*Resource{}
	var resources []*Resource
	for token, spec := range specs {
		res, err := bindResource(token, spec, types, functionTable)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error binding resource %v", token)
		}
		resourceTable[token] = res

		if rt, ok := types.resources[token]; ok {
			if rt.Resource == nil {
				rt.Resource = res
			}
		} else {
			types.resources[token] = &ResourceType{
				Token:    res.Token,
				Resource: res,
			}
		}

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
		language[name] = json.RawMessage(raw)
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

func jsonMarshal(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
