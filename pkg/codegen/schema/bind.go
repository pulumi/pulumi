// Copyright 2016-2021, Pulumi Corporation.
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
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed pulumi.json
var metaSchema string

var MetaSchema *jsonschema.Schema

func init() {
	compiler := jsonschema.NewCompiler()
	compiler.LoadURL = func(u string) (io.ReadCloser, error) {
		if u == "blob://pulumi.json" {
			return io.NopCloser(strings.NewReader(metaSchema)), nil
		}
		return jsonschema.LoadURL(u)
	}
	MetaSchema = compiler.MustCompile("blob://pulumi.json")
}

func memberPath(section, token string, rest ...string) string {
	path := fmt.Sprintf("#/%v/%v", section, url.PathEscape(token))
	if len(rest) != 0 {
		path += "/" + strings.Join(rest, "/")
	}
	return path
}

func errorf(path, message string, args ...interface{}) *hcl.Diagnostic {
	contract.Require(path != "", "path")

	summary := path + ": " + fmt.Sprintf(message, args...)
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  summary,
	}
}

func validateSpec(spec PackageSpec) (hcl.Diagnostics, error) {
	bytes, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	var raw interface{}
	if err = json.Unmarshal(bytes, &raw); err != nil {
		return nil, err
	}

	if err = MetaSchema.Validate(raw); err == nil {
		return nil, nil
	}
	validationError, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return nil, err
	}

	var diags hcl.Diagnostics
	var appendError func(err *jsonschema.ValidationError)
	appendError = func(err *jsonschema.ValidationError) {
		if err.InstanceLocation != "" && err.Message != "" {
			diags = diags.Append(errorf("#"+err.InstanceLocation, "%v", err.Message))
		}
		for _, err := range err.Causes {
			appendError(err)
		}
	}
	appendError(validationError)

	return diags, nil
}

// bindSpec converts a serializable PackageSpec into a Package. This function includes a loader parameter which
// works as a singleton -- if it is nil, a new loader is instantiated, else the provided loader is used. This avoids
// breaking downstream consumers of ImportSpec while allowing us to extend schema support to external packages.
//
// A few notes on diagnostics and errors in spec binding:
//
// - Unless an error is *fatal*--i.e. binding is fundamentally unable to proceed (e.g. because a provider for a package
//   failed to load)--errors should be communicated as diagnostics. Fatal errors should be communicated as error values.
// - Semantic errors during type binding should not be fatal. Instead, they should return an `InvalidType`. The invalid
//   type is accepted in any position, and carries diagnostics that explain the semantic error during binding. This
//   allows binding to continue and produce as much information as possible for the end user.
// - Diagnostics may be rendered to users by downstream tools, and should be written with schema authors in mind.
// - Diagnostics _must_ contain enough contextual information for a user to be able to understand the source of the
//   diagnostic. Until we have line/column information, we use JSON pointers to the offending entities. These pointers
//   are passed around using `path` parameters. The `errorf` function is provided as a utility to easily create a
//   diagnostic error that is appropriately tagged with a JSON pointer.
//
func bindSpec(spec PackageSpec, languages map[string]Language, loader Loader,
	validate bool) (*Package, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	// Validate the package against the metaschema.
	if validate {
		validationDiags, err := validateSpec(spec)
		if err != nil {
			return nil, nil, fmt.Errorf("validating spec: %w", err)
		}
		diags = diags.Extend(validationDiags)
	}

	// Validate that there is a name
	if spec.Name == "" {
		diags = diags.Append(errorf("#/name", "no name provided"))
	}

	// Parse the version, if any.
	var version *semver.Version
	if spec.Version != "" {
		v, err := semver.ParseTolerant(spec.Version)
		if err != nil {
			diags = diags.Append(errorf("#/version", "failed to parse semver: %v", err))
		} else {
			version = &v
		}
	}

	// Parse the module format, if any.
	moduleFormat := "(.*)"
	if spec.Meta != nil && spec.Meta.ModuleFormat != "" {
		moduleFormat = spec.Meta.ModuleFormat
	}
	moduleFormatRegexp, err := regexp.Compile(moduleFormat)
	if err != nil {
		diags = diags.Append(errorf("#/meta/moduleFormat", "failed to compile regex: %v", err))
	}

	diags = diags.Extend(spec.validateTypeTokens())

	pkg := &Package{}

	// We want to use the same loader instance for all referenced packages, so only instantiate the loader if the
	// reference is nil.
	if loader == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, nil, err
		}
		ctx, err := plugin.NewContext(nil, nil, nil, nil, cwd, nil, false, nil)
		if err != nil {
			return nil, nil, err
		}
		defer contract.IgnoreClose(ctx)

		loader = NewPluginLoader(ctx.Host)
	}

	types, typeDiags, err := bindTypes(pkg, spec.Types, loader)
	if err != nil {
		return nil, nil, err
	}
	diags = diags.Extend(typeDiags)

	config, configDiags, err := bindConfig(spec.Config, types)
	if err != nil {
		return nil, nil, err
	}
	diags = diags.Extend(configDiags)

	functions, functionTable, functionDiags, err := bindFunctions(spec.Functions, types)
	if err != nil {
		return nil, nil, err
	}
	diags = diags.Extend(functionDiags)

	provider, providerDiags, err := bindProvider(spec.Name, spec.Provider, types, functionTable)
	if err != nil {
		return nil, nil, err
	}
	diags = diags.Extend(providerDiags)

	resources, resourceTable, resourceDiags, err := bindResources(spec.Resources, types, functionTable)
	if err != nil {
		return nil, nil, err
	}
	diags = diags.Extend(resourceDiags)

	// Build the type list.
	var typeList []Type
	for _, t := range types.resources {
		typeList = append(typeList, t)
	}
	for _, t := range types.objects {
		// t is a plain shape: add it and its corresponding input shape to the type list.
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
		DisplayName:       spec.DisplayName,
		Version:           version,
		Description:       spec.Description,
		Keywords:          spec.Keywords,
		Homepage:          spec.Homepage,
		License:           spec.License,
		Attribution:       spec.Attribution,
		Repository:        spec.Repository,
		PluginDownloadURL: spec.PluginDownloadURL,
		Publisher:         spec.Publisher,
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
		return nil, nil, err
	}
	return pkg, diags, nil

}

// BindSpec converts a serializable PackageSpec into a Package. Any semantic errors encountered during binding are
// contained in the returned diagnostics. The returned error is only non-nil if a fatal error was encountered.
func BindSpec(spec PackageSpec, languages map[string]Language) (*Package, hcl.Diagnostics, error) {
	return bindSpec(spec, languages, nil, true)
}

// ImportSpec converts a serializable PackageSpec into a Package. Unlike BindSpec, ImportSpec does not validate its
// input against the Pulumi package metaschema. ImportSpec should only be used to load packages that are assumed to be
// well-formed (e.g. packages referenced for program code generation or by a root package being used for SDK
// generation). BindSpec should be used to load and validate a package spec prior to generating its SDKs.
func ImportSpec(spec PackageSpec, languages map[string]Language) (*Package, error) {
	// Call the internal implementation that includes a loader parameter.
	pkg, diags, err := bindSpec(spec, languages, nil, false)
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}
	return pkg, nil
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

//nolint: goconst
func (t *types) bindPrimitiveType(path, name string) (Type, hcl.Diagnostics) {
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
		return invalidType(errorf(path, "unknown primitive type %v", name))
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

// Validate an individual name token.
func (spec *PackageSpec) validateTypeToken(allowedPackageNames map[string]bool, section, token string) hcl.Diagnostics {
	diags := hcl.Diagnostics{}

	path := memberPath(section, token)
	var packageName string
	if i := strings.Index(token, ":"); i != -1 {
		packageName = token[:i]
	}
	if !allowedPackageNames[packageName] {
		error := errorf(path, "invalid token '%s' (must have package name '%s')", token, spec.Name)
		diags = diags.Append(error)
	}

	return diags
}

// This is for validating non-reference type tokens.
func (spec *PackageSpec) validateTypeTokens() hcl.Diagnostics {
	diags := hcl.Diagnostics{}
	allowedPackageNames := map[string]bool{spec.Name: true}
	for _, prefix := range spec.AllowedPackageNames {
		allowedPackageNames[prefix] = true
	}
	for t := range spec.Resources {
		diags = diags.Extend(spec.validateTypeToken(allowedPackageNames, "resources", t))
	}
	for t := range spec.Types {
		diags = diags.Extend(spec.validateTypeToken(allowedPackageNames, "types", t))
	}
	for t := range spec.Functions {
		diags = diags.Extend(spec.validateTypeToken(allowedPackageNames, "functions", t))
	}
	return diags
}

// Regex used to parse external schema paths. This is declared at the package scope to avoid repeated recompilation.
var refPathRegex = regexp.MustCompile(`^/?(?P<package>[-\w]+)/(?P<version>v[^/]*)/schema\.json$`)

func (t *types) parseTypeSpecRef(refPath, ref string) (typeSpecRef, hcl.Diagnostics) {
	parsedURL, err := url.Parse(ref)
	if err != nil {
		return typeSpecRef{}, hcl.Diagnostics{errorf(refPath, "failed to parse ref URL '%s': %v", ref, err)}
	}

	// Parse the package name and version if the URL contains a path. If there is no path--if the URL is just a
	// fragment--then the reference refers to the package being bound.
	pkgName, pkgVersion := t.pkg.Name, t.pkg.Version
	if len(parsedURL.Path) > 0 {
		path, err := url.PathUnescape(parsedURL.Path)
		if err != nil {
			return typeSpecRef{}, hcl.Diagnostics{errorf(refPath, "failed to unescape path '%s': %v", parsedURL.Path, err)}
		}

		pathMatch := refPathRegex.FindStringSubmatch(path)
		if len(pathMatch) != 3 {
			return typeSpecRef{}, hcl.Diagnostics{errorf(refPath, "failed to parse path '%s'", path)}
		}

		pkg, versionToken := pathMatch[1], pathMatch[2]
		version, err := semver.ParseTolerant(versionToken)
		if err != nil {
			return typeSpecRef{}, hcl.Diagnostics{errorf(refPath, "failed to parse package version '%s': %v", versionToken, err)}
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
			return typeSpecRef{}, hcl.Diagnostics{errorf(refPath, "invalid provider reference '%v'", ref)}
		}
		token = "pulumi:providers:" + pkgName
	case "resources", "types":
		token, err = url.PathUnescape(token)
		if err != nil {
			return typeSpecRef{}, hcl.Diagnostics{errorf(refPath, "failed to unescape token '%s': %v", token, err)}
		}
	default:
		return typeSpecRef{}, hcl.Diagnostics{errorf(refPath, "invalid type reference '%v'", ref)}
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

func (t *types) bindTypeSpecRef(path string, spec TypeSpec, inputShape bool) (Type, hcl.Diagnostics, error) {
	path = path + "/$ref"

	// Explicitly handle built-in types so that we don't have to handle this type of path during ref parsing.
	switch spec.Ref {
	case "pulumi.json#/Archive":
		return ArchiveType, nil, nil
	case "pulumi.json#/Asset":
		return AssetType, nil, nil
	case "pulumi.json#/Json":
		return JSONType, nil, nil
	case "pulumi.json#/Any":
		return AnyType, nil, nil
	}

	ref, refDiags := t.parseTypeSpecRef(path, spec.Ref)
	if refDiags.HasErrors() {
		typ, _ := invalidType(refDiags...)
		return typ, refDiags, nil
	}

	// If this is a reference to an external sch
	referencesExternalSchema := ref.Package != t.pkg.Name || !versionEquals(ref.Version, t.pkg.Version)
	if referencesExternalSchema {
		pkg, err := t.loader.LoadPackage(ref.Package, ref.Version)
		if err != nil {
			return nil, nil, fmt.Errorf("resolving package %v: %w", ref.URL, err)
		}

		switch ref.Kind {
		case typesRef:
			typ, ok := pkg.GetType(ref.Token)
			if !ok {
				typ, diags := invalidType(errorf(path, "type %v not found in package %v", ref.Token, ref.Package))
				return typ, diags, nil
			}
			if obj, ok := typ.(*ObjectType); ok && inputShape {
				typ = obj.InputShape
			}
			return typ, nil, nil
		case resourcesRef, providerRef:
			typ, ok := pkg.GetResourceType(ref.Token)
			if !ok {
				typ, diags := invalidType(errorf(path, "resource type %v not found in package %v", ref.Token, ref.Package))
				return typ, diags, nil
			}
			return typ, nil, nil
		}
	}

	switch ref.Kind {
	case typesRef:
		if typ, ok := t.objects[ref.Token]; ok {
			if inputShape {
				return typ.InputShape, nil, nil
			}
			return typ, nil, nil
		}
		if typ, ok := t.enums[ref.Token]; ok {
			return typ, nil, nil
		}

		var diags hcl.Diagnostics
		typ, ok := t.tokens[ref.Token]
		if !ok {
			typ = &TokenType{Token: ref.Token}
			if spec.Type != "" {
				ut, primDiags := t.bindPrimitiveType(path, spec.Type)
				diags = diags.Extend(primDiags)

				typ.UnderlyingType = ut
			}
			t.tokens[ref.Token] = typ
		}
		return typ, diags, nil
	case resourcesRef, providerRef:
		typ, ok := t.resources[ref.Token]
		if !ok {
			typ = &ResourceType{Token: ref.Token}
			t.resources[ref.Token] = typ
		}
		return typ, nil, nil
	default:
		typ, diags := invalidType(errorf(path, "failed to parse ref %s", spec.Ref))
		return typ, diags, nil
	}
}

func (t *types) bindType(path string, spec TypeSpec, inputShape bool) (result Type, diags hcl.Diagnostics, err error) {
	// NOTE: `spec.Plain` is the spec of the type, not to be confused with the
	// `Plain` property of the underlying `Property`, which is passed as
	// `plainProperty`.
	if inputShape && !spec.Plain {
		defer func() {
			result = t.newInputType(result)
		}()
	}

	if spec.Ref != "" {
		return t.bindTypeSpecRef(path, spec, inputShape)
	}

	if spec.OneOf != nil {
		if len(spec.OneOf) < 2 {
			diags = diags.Append(errorf(path+"/oneOf", "oneOf should list at least two types"))
		}

		var defaultType Type
		if spec.Type != "" {
			dt, primDiags := t.bindPrimitiveType(path+"/type", spec.Type)
			diags = diags.Extend(primDiags)

			defaultType = dt
		}

		elements := make([]Type, len(spec.OneOf))
		for i, spec := range spec.OneOf {
			e, typDiags, err := t.bindType(fmt.Sprintf("%s/oneOf/%v", path, i), spec, inputShape)
			diags = diags.Extend(typDiags)

			if err != nil {
				return nil, diags, err
			}

			elements[i] = e
		}

		var discriminator string
		var mapping map[string]string
		if spec.Discriminator != nil {
			if spec.Discriminator.PropertyName == "" {
				diags = diags.Append(errorf(path, "discriminator must provide a property name"))
			}
			discriminator = spec.Discriminator.PropertyName
			mapping = spec.Discriminator.Mapping
		}

		return t.newUnionType(elements, defaultType, discriminator, mapping), diags, nil
	}

	// nolint: goconst
	switch spec.Type {
	case "boolean", "integer", "number", "string":
		typ, typDiags := t.bindPrimitiveType(path+"/type", spec.Type)
		diags = diags.Extend(typDiags)

		return typ, diags, nil
	case "array":
		if spec.Items == nil {
			diags = diags.Append(errorf(path, "missing \"items\" property in array type spec"))
			typ, _ := invalidType(diags...)
			return typ, diags, nil
		}

		elementType, elementDiags, err := t.bindType(path+"/items", *spec.Items, inputShape)
		diags = diags.Extend(elementDiags)
		if err != nil {
			return nil, diags, err
		}

		return t.newArrayType(elementType), diags, nil
	case "object":
		elementType, elementDiags, err := t.bindType(path, TypeSpec{Type: "string"}, inputShape)
		contract.Assert(len(elementDiags) == 0)
		contract.Assert(err == nil)

		if spec.AdditionalProperties != nil {
			et, elementDiags, err := t.bindType(path+"/additionalProperties", *spec.AdditionalProperties, inputShape)
			diags = diags.Extend(elementDiags)
			if err != nil {
				return nil, diags, err
			}

			elementType = et
		}

		return t.newMapType(elementType), diags, nil
	default:
		diags = diags.Append(errorf(path+"/type", "unknown type kind %v", spec.Type))
		typ, _ := invalidType(diags...)
		return typ, diags, nil
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

func bindConstValue(path, kind string, value interface{}, typ Type) (interface{}, hcl.Diagnostics) {
	if value == nil {
		return nil, nil
	}

	typeError := func(expectedType string) hcl.Diagnostics {
		return hcl.Diagnostics{errorf(path, "invalid constant of type %T for %v %v", value, expectedType, kind)}
	}

	switch typ = plainType(typ); typ {
	case BoolType:
		if _, ok := value.(bool); !ok {
			return false, typeError("boolean")
		}
	case IntType:
		v, ok := value.(float64)
		if !ok {
			return 0, typeError("integer")
		}
		if math.Trunc(v) != v || v < math.MinInt32 || v > math.MaxInt32 {
			return 0, typeError("integer")
		}
		value = int32(v)
	case NumberType:
		if _, ok := value.(float64); !ok {
			return 0.0, typeError("number")
		}
	case StringType:
		if _, ok := value.(string); !ok {
			return 0.0, typeError("string")
		}
	default:
		if _, isInvalid := typ.(*InvalidType); isInvalid {
			return nil, nil
		}
		return nil, hcl.Diagnostics{errorf(path, "type %v cannot have a constant value; only booleans, integers, "+
			"numbers and strings may have constant values", typ)}
	}

	return value, nil
}

func bindDefaultValue(path string, value interface{}, spec *DefaultSpec, typ Type) (*DefaultValue, hcl.Diagnostics) {
	if value == nil && spec == nil {
		return nil, nil
	}

	var diags hcl.Diagnostics
	if value != nil {
		typ = plainType(typ)
		switch typ := typ.(type) {
		case *UnionType:
			if typ.DefaultType != nil {
				return bindDefaultValue(path, value, spec, typ.DefaultType)
			}
			for _, elementType := range typ.ElementTypes {
				v, diags := bindDefaultValue(path, value, spec, elementType)
				if !diags.HasErrors() {
					return v, diags
				}
			}
		case *EnumType:
			return bindDefaultValue(path, value, spec, typ.ElementType)
		}

		v, valueDiags := bindConstValue(path, "default", value, typ)
		diags = diags.Extend(valueDiags)
		value = v
	}

	dv := &DefaultValue{Value: value}
	if spec != nil {
		language := make(map[string]interface{})
		for name, raw := range spec.Language {
			language[name] = json.RawMessage(raw)
		}
		if len(spec.Environment) == 0 {
			diags = diags.Append(errorf(path, "Default must specify an environment"))
		}

		dv.Environment, dv.Language = spec.Environment, language
	}
	return dv, diags
}

// bindProperties binds the map of property specs and list of required properties into a sorted list of properties and
// a lookup table.
func (t *types) bindProperties(path string, properties map[string]PropertySpec, requiredPath string, required []string,
	inputShape bool) ([]*Property, map[string]*Property, hcl.Diagnostics, error) {

	var diags hcl.Diagnostics

	// Bind property types and constant or default values.
	propertyMap := map[string]*Property{}
	var result []*Property
	for name, spec := range properties {
		propertyPath := path + "/" + name
		// NOTE: The correct determination for if we should bind an input is:
		//
		// inputShape && !spec.Plain
		//
		// We will then be able to remove the markedPlain field of t.bindType
		// since `arg(inputShape, t.bindType) <=> inputShape && !spec.Plain`.
		// Unfortunately, this fix breaks backwards compatibility in a major
		// way, across all providers.
		typ, typDiags, err := t.bindType(propertyPath, spec.TypeSpec, inputShape)
		diags = diags.Extend(typDiags)
		if err != nil {
			return nil, nil, diags, fmt.Errorf("error binding type for property %q: %w", name, err)
		}

		cv, cvDiags := bindConstValue(propertyPath+"/const", "constant", spec.Const, typ)
		diags = diags.Extend(cvDiags)

		dv, dvDiags := bindDefaultValue(propertyPath+"/default", spec.Default, spec.DefaultInfo, typ)
		diags = diags.Extend(dvDiags)

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
			ReplaceOnChanges:   spec.ReplaceOnChanges,
			Plain:              spec.Plain,
		}

		propertyMap[name], result = p, append(result, p)
	}

	// Compute required properties.
	for i, name := range required {
		p, ok := propertyMap[name]
		if !ok {
			diags = diags.Append(errorf(fmt.Sprintf("%s/%v", requiredPath, i), "unknown required property %q", name))
			continue
		}
		if typ, ok := p.Type.(*OptionalType); ok {
			p.Type = typ.ElementType
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, propertyMap, diags, nil
}

func (t *types) bindObjectTypeDetails(path string, obj *ObjectType, token string,
	spec ObjectTypeSpec) (hcl.Diagnostics, error) {

	var diags hcl.Diagnostics

	if len(spec.Plain) > 0 {
		diags = diags.Append(errorf(path+"/plain",
			"plain has been removed; the property type must be marked as plain instead"))
	}

	properties, propertyMap, propertiesDiags, err := t.bindProperties(path+"/properties", spec.Properties,
		path+"/required", spec.Required, false)
	diags = diags.Extend(propertiesDiags)
	if err != nil {
		return diags, err
	}

	inputProperties, inputPropertyMap, inputPropertiesDiags, err := t.bindProperties(
		path+"/properties", spec.Properties, path+"/required", spec.Required, true)
	diags = diags.Extend(inputPropertiesDiags)
	if err != nil {
		return diags, err
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
	obj.IsOverlay = spec.IsOverlay

	obj.InputShape.Package = t.pkg
	obj.InputShape.Token = token
	obj.InputShape.Comment = spec.Description
	obj.InputShape.Language = language
	obj.InputShape.Properties = inputProperties
	obj.InputShape.properties = inputPropertyMap

	return diags, nil
}

func (t *types) bindObjectType(path, token string, spec ObjectTypeSpec) (*ObjectType, hcl.Diagnostics, error) {
	obj := &ObjectType{}
	obj.InputShape = &ObjectType{PlainShape: obj}
	obj.IsOverlay = spec.IsOverlay

	diags, err := t.bindObjectTypeDetails(path, obj, token, spec)
	if err != nil {
		return nil, diags, err
	}
	return obj, diags, nil
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

func (t *types) bindEnumTypeDetails(enum *EnumType, token string, spec ComplexTypeSpec) hcl.Diagnostics {
	var diags hcl.Diagnostics

	typ, typDiags := t.bindPrimitiveType(memberPath("types", token, "type"), spec.Type)
	diags = diags.Extend(typDiags)

	switch typ {
	case StringType, IntType, NumberType, BoolType:
		// OK
	default:
		if _, isInvalid := typ.(*InvalidType); !isInvalid {
			diags = diags.Append(errorf(memberPath("types", token, "type"),
				"enums may only be of type string, integer, number or boolean"))
		}
	}

	values, valuesDiags := t.bindEnumValues(memberPath("types", token, "enum"), spec.Enum, typ)
	diags = diags.Extend(valuesDiags)

	enum.Package = t.pkg
	enum.Token = token
	enum.Elements = values
	enum.ElementType = typ
	enum.Comment = spec.Description
	enum.IsOverlay = spec.IsOverlay

	return diags
}

func (t *types) bindEnumValues(path string, values []EnumValueSpec, typ Type) ([]*Enum, hcl.Diagnostics) {
	var enums []*Enum
	var diags hcl.Diagnostics

	for i, spec := range values {
		value, valueDiags := bindConstValue(fmt.Sprintf("%s/%v/value", path, i), "enum", spec.Value, typ)
		diags = diags.Extend(valueDiags)

		enum := &Enum{
			Value:              value,
			Comment:            spec.Description,
			Name:               spec.Name,
			DeprecationMessage: spec.DeprecationMessage,
		}
		enums = append(enums, enum)
	}
	return enums, diags
}

func (t *types) bindEnumType(token string, spec ComplexTypeSpec) (*EnumType, error) {
	enum := &EnumType{}
	if err := t.bindEnumTypeDetails(enum, token, spec); err != nil {
		return nil, err
	}
	return enum, nil
}

func bindTypes(pkg *Package, complexTypes map[string]ComplexTypeSpec, loader Loader) (*types, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

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
			typ.InputShape = &ObjectType{Token: token, PlainShape: typ, IsOverlay: spec.IsOverlay}
			typs.objects[token] = typ
			typs.named[token] = typ
		} else if len(spec.Enum) > 0 {
			typ := &EnumType{Token: token}
			typs.enums[token] = typ
			typs.named[token] = typ

			// Bind enums before object types because object type generation depends on enum values to be present.
			enumDiags := typs.bindEnumTypeDetails(typs.enums[token], token, spec)
			diags = diags.Extend(enumDiags)
		}
	}

	// Process resources.
	for _, r := range pkg.Resources {
		typs.resources[r.Token] = &ResourceType{Token: r.Token}
	}

	// Process object types.
	for token, spec := range complexTypes {
		if spec.Type == "object" {
			path := memberPath("types", token)
			objDiags, err := typs.bindObjectTypeDetails(path, typs.objects[token], token, spec.ObjectTypeSpec)
			diags = diags.Extend(objDiags)

			if err != nil {
				return nil, diags, fmt.Errorf("failed to bind type %s: %w", token, err)
			}
		}
	}

	return typs, diags, nil
}

func bindMethods(path, resourceToken string, methods map[string]string,
	functionTable map[string]*Function) ([]*Method, hcl.Diagnostics) {

	var diags hcl.Diagnostics

	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]*Method, 0, len(methods))
	for _, name := range names {
		token := methods[name]

		methodPath := path + "/" + name

		function, ok := functionTable[token]
		if !ok {
			diags = diags.Append(errorf(methodPath, "unknown function %s", token))
			continue
		}
		if function.IsMethod {
			diags = diags.Append(errorf(methodPath, "function %s is already a method", token))
			continue
		}
		idx := strings.LastIndex(function.Token, "/")
		if idx == -1 || function.Token[:idx] != resourceToken {
			diags = diags.Append(errorf(methodPath, "invalid function token format %s", token))
			continue
		}
		if function.Inputs == nil || function.Inputs.Properties == nil || len(function.Inputs.Properties) == 0 ||
			function.Inputs.Properties[0].Name != "__self__" {
			diags = diags.Append(errorf(methodPath, "function %s has no __self__ parameter", token))
			continue
		}
		function.IsMethod = true
		result = append(result, &Method{
			Name:     name,
			Function: function,
		})
	}
	return result, diags
}

func bindConfig(spec ConfigSpec, types *types) ([]*Property, hcl.Diagnostics, error) {
	properties, _, diags, err := types.bindProperties("#/config/variables", spec.Variables,
		"#/config/defaults", spec.Required, false)
	return properties, diags, err
}

func bindResource(path, token string, spec ResourceSpec, types *types,
	functionTable map[string]*Function) (*Resource, hcl.Diagnostics, error) {

	var diags hcl.Diagnostics

	if len(spec.Plain) > 0 {
		diags = diags.Append(errorf(path+"/plain", "plain has been removed; property types must be marked as plain instead"))
	}
	if len(spec.PlainInputs) > 0 {
		diags = diags.Append(errorf(path+"/plainInputs",
			"plainInputs has been removed; individual property types must be marked as plain instead"))
	}

	properties, _, propertyDiags, err := types.bindProperties(path+"/properties", spec.Properties,
		path+"/required", spec.Required, false)
	diags = diags.Extend(propertyDiags)
	if err != nil {
		return nil, diags, fmt.Errorf("failed to bind properties for %v: %w", token, err)
	}

	inputProperties, _, inputDiags, err := types.bindProperties(path+"/inputProperties", spec.InputProperties,
		path+"/requiredInputs", spec.RequiredInputs, true)
	diags = diags.Extend(inputDiags)
	if err != nil {
		return nil, diags, fmt.Errorf("failed to bind input properties for %v: %w", token, err)
	}

	methods, methodDiags := bindMethods(path+"/methods", token, spec.Methods, functionTable)
	diags = diags.Extend(methodDiags)

	for _, method := range methods {
		if _, ok := spec.Properties[method.Name]; ok {
			diags = diags.Append(errorf(path+"/methods/"+method.Name, "%v already has a property named %s", token, method.Name))
		}
	}

	var stateInputs *ObjectType
	if spec.StateInputs != nil {
		si, stateDiags, err := types.bindObjectType(path+"/stateInputs", token+"Args", *spec.StateInputs)
		diags = diags.Extend(stateDiags)
		if err != nil {
			return nil, diags, fmt.Errorf("error binding inputs for %v: %w", token, err)
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
		IsOverlay:          spec.IsOverlay,
	}, diags, nil
}

func bindProvider(pkgName string, spec ResourceSpec, types *types,
	functionTable map[string]*Function) (*Resource, hcl.Diagnostics, error) {

	res, diags, err := bindResource("#/provider", "pulumi:providers:"+pkgName, spec, types, functionTable)
	if err != nil {
		return nil, diags, fmt.Errorf("error binding provider: %w", err)
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

	return res, diags, nil
}

func bindResources(specs map[string]ResourceSpec, types *types,
	functionTable map[string]*Function) ([]*Resource, map[string]*Resource, hcl.Diagnostics, error) {

	var diags hcl.Diagnostics

	resourceTable := map[string]*Resource{}
	var resources []*Resource
	for token, spec := range specs {
		res, resDiags, err := bindResource(memberPath("resources", token), token, spec, types, functionTable)
		diags = diags.Extend(resDiags)
		if err != nil {
			return nil, nil, diags, fmt.Errorf("error binding resource %v: %w", token, err)
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

	return resources, resourceTable, diags, nil
}

func bindFunction(token string, spec FunctionSpec, types *types) (*Function, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	path := memberPath("functions", token)

	var inputs *ObjectType
	if spec.Inputs != nil {
		ins, inDiags, err := types.bindObjectType(path+"/inputs", token+"Args", *spec.Inputs)
		diags = diags.Extend(inDiags)
		if err != nil {
			return nil, diags, fmt.Errorf("error binding inputs for function %v: %w", token, err)
		}
		inputs = ins
	}

	var outputs *ObjectType
	if spec.Outputs != nil {
		outs, outDiags, err := types.bindObjectType(path+"/outputs", token+"Result", *spec.Outputs)
		diags = diags.Extend(outDiags)
		if err != nil {
			return nil, diags, fmt.Errorf("error binding outputs for function %v: %w", token, err)
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
		IsOverlay:          spec.IsOverlay,
	}, diags, nil
}

func bindFunctions(specs map[string]FunctionSpec,
	types *types) ([]*Function, map[string]*Function, hcl.Diagnostics, error) {

	var diags hcl.Diagnostics

	functionTable := map[string]*Function{}
	var functions []*Function
	for token, spec := range specs {
		f, fdiags, err := bindFunction(token, spec, types)
		diags = diags.Extend(fdiags)
		if err != nil {
			return nil, nil, diags, fmt.Errorf("error binding function %v: %w", token, err)
		}
		functionTable[token] = f
		functions = append(functions, f)
	}

	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Token < functions[j].Token
	})

	return functions, functionTable, diags, nil
}
