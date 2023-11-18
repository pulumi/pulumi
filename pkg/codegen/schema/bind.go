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
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/segmentio/encoding/json"
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

func sortedKeys(m interface{}) []string {
	rv := reflect.ValueOf(m)
	keys := slice.Prealloc[string](rv.Len())
	for it := rv.MapRange(); it.Next(); {
		keys = append(keys, it.Key().String())
	}
	sort.Strings(keys)
	return keys
}

func memberPath(section, token string, rest ...string) string {
	path := fmt.Sprintf("#/%v/%v", section, url.PathEscape(token))
	if len(rest) != 0 {
		path += "/" + strings.Join(rest, "/")
	}
	return path
}

func errorf(path, message string, args ...interface{}) *hcl.Diagnostic {
	contract.Requiref(path != "", "path", "must not be empty")

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
//   - Unless an error is *fatal*--i.e. binding is fundamentally unable to proceed (e.g. because a provider for a
//     package failed to load)--errors should be communicated as diagnostics. Fatal errors should be communicated as
//     error values.
//   - Semantic errors during type binding should not be fatal. Instead, they should return an `InvalidType`. The
//     invalid type is accepted in any position, and carries diagnostics that explain the semantic error during binding.
//     This allows binding to continue and produce as much information as possible for the end user.
//   - Diagnostics may be rendered to users by downstream tools, and should be written with schema authors in mind.
//   - Diagnostics _must_ contain enough contextual information for a user to be able to understand the source of the
//     diagnostic. Until we have line/column information, we use JSON pointers to the offending entities. These pointers
//     are passed around using `path` parameters. The `errorf` function is provided as a utility to easily create a
//     diagnostic error that is appropriately tagged with a JSON pointer.
func bindSpec(spec PackageSpec, languages map[string]Language, loader Loader,
	validate bool,
) (*Package, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	// Validate the package against the metaschema.
	if validate {
		validationDiags, err := validateSpec(spec)
		if err != nil {
			return nil, nil, fmt.Errorf("validating spec: %w", err)
		}
		diags = diags.Extend(validationDiags)
	}

	types, pkgDiags, err := newBinder(spec.Info(), packageSpecSource{&spec}, loader, nil)
	if err != nil {
		return nil, nil, err
	}
	defer contract.IgnoreClose(types)
	diags = diags.Extend(pkgDiags)

	diags = diags.Extend(spec.validateTypeTokens())

	config, configDiags, err := bindConfig(spec.Config, types)
	if err != nil {
		return nil, nil, err
	}
	diags = diags.Extend(configDiags)

	provider, resources, resourceDiags, err := types.finishResources(sortedKeys(spec.Resources))
	if err != nil {
		return nil, nil, err
	}
	diags = diags.Extend(resourceDiags)

	functions, functionDiags, err := types.finishFunctions(sortedKeys(spec.Functions))
	if err != nil {
		return nil, nil, err
	}
	diags = diags.Extend(functionDiags)

	typeList, typeDiags, err := types.finishTypes(sortedKeys(spec.Types))
	if err != nil {
		return nil, nil, err
	}
	diags = diags.Extend(typeDiags)

	pkg := types.pkg
	pkg.Config = config
	pkg.Types = typeList
	pkg.Provider = provider
	pkg.Resources = resources
	pkg.Functions = functions
	pkg.resourceTable = types.resourceDefs
	pkg.functionTable = types.functionDefs
	pkg.typeTable = types.typeDefs
	pkg.resourceTypeTable = types.resources
	if err := pkg.ImportLanguages(languages); err != nil {
		return nil, nil, err
	}
	return pkg, diags, nil
}

// Create a new binder.
//
// bindTo overrides the PackageReference field contained in generated types.
func newBinder(info PackageInfoSpec, spec specSource, loader Loader,
	bindTo PackageReference,
) (*types, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	// Validate that there is a name
	if info.Name == "" {
		diags = diags.Append(errorf("#/name", "no name provided"))
	}

	// Parse the version, if any.
	var version *semver.Version
	if info.Version != "" {
		v, err := semver.ParseTolerant(info.Version)
		if err != nil {
			diags = diags.Append(errorf("#/version", "failed to parse semver: %v", err))
		} else {
			version = &v
		}
	}

	// Parse the module format, if any.
	moduleFormat := "(.*)"
	if info.Meta != nil && info.Meta.ModuleFormat != "" {
		moduleFormat = info.Meta.ModuleFormat
	}
	moduleFormatRegexp, err := regexp.Compile(moduleFormat)
	if err != nil {
		diags = diags.Append(errorf("#/meta/moduleFormat", "failed to compile regex: %v", err))
	}

	language := make(map[string]interface{}, len(info.Language))
	for name, v := range info.Language {
		language[name] = json.RawMessage(v)
	}

	pkg := &Package{
		moduleFormat:        moduleFormatRegexp,
		Name:                info.Name,
		DisplayName:         info.DisplayName,
		Version:             version,
		Description:         info.Description,
		Keywords:            info.Keywords,
		Homepage:            info.Homepage,
		License:             info.License,
		Attribution:         info.Attribution,
		Repository:          info.Repository,
		PluginDownloadURL:   info.PluginDownloadURL,
		Publisher:           info.Publisher,
		AllowedPackageNames: info.AllowedPackageNames,
		LogoURL:             info.LogoURL,
		Language:            language,
	}

	// We want to use the same loader instance for all referenced packages, so only instantiate the loader if the
	// reference is nil.
	var loadCtx io.Closer
	if loader == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, nil, err
		}
		ctx, err := plugin.NewContext(nil, nil, nil, nil, cwd, nil, false, nil)
		if err != nil {
			return nil, nil, err
		}

		loader, loadCtx = NewPluginLoader(ctx.Host), ctx
	}

	// Create a type binder.
	types := &types{
		pkg:          pkg,
		spec:         spec,
		loader:       loader,
		loadCtx:      loadCtx,
		typeDefs:     map[string]Type{},
		functionDefs: map[string]*Function{},
		resourceDefs: map[string]*Resource{},
		resources:    map[string]*ResourceType{},
		arrays:       map[Type]*ArrayType{},
		maps:         map[Type]*MapType{},
		unions:       map[string]*UnionType{},
		tokens:       map[string]*TokenType{},
		inputs:       map[Type]*InputType{},
		optionals:    map[Type]*OptionalType{},

		bindToReference: bindTo,
	}

	return types, diags, nil
}

// BindSpec converts a serializable PackageSpec into a Package. Any semantic errors encountered during binding are
// contained in the returned diagnostics. The returned error is only non-nil if a fatal error was encountered.
func BindSpec(spec PackageSpec, loader Loader) (*Package, hcl.Diagnostics, error) {
	return bindSpec(spec, nil, loader, true)
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

// ImportPartialSpec converts a serializable PartialPackageSpec into a PartialPackage. Unlike a typical Package, a
// PartialPackage loads and binds its members on-demand rather than at import time. This is useful when the entire
// contents of a package are not needed (e.g. for referenced packages).
func ImportPartialSpec(spec PartialPackageSpec, languages map[string]Language, loader Loader) (*PartialPackage, error) {
	pkg := &PartialPackage{
		spec:      &spec,
		languages: languages,
	}
	types, diags, err := newBinder(spec.PackageInfoSpec, partialPackageSpecSource{&spec}, loader, pkg)
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}
	pkg.types = types
	return pkg, nil
}

type specSource interface {
	GetTypeDefSpec(token string) (ComplexTypeSpec, bool, error)
	GetFunctionSpec(token string) (FunctionSpec, bool, error)
	GetResourceSpec(token string) (ResourceSpec, bool, error)
}

type packageSpecSource struct {
	spec *PackageSpec
}

func (s packageSpecSource) GetTypeDefSpec(token string) (ComplexTypeSpec, bool, error) {
	spec, ok := s.spec.Types[token]
	return spec, ok, nil
}

func (s packageSpecSource) GetFunctionSpec(token string) (FunctionSpec, bool, error) {
	spec, ok := s.spec.Functions[token]
	return spec, ok, nil
}

func (s packageSpecSource) GetResourceSpec(token string) (ResourceSpec, bool, error) {
	if token == "pulumi:providers:"+s.spec.Name {
		return s.spec.Provider, true, nil
	}
	spec, ok := s.spec.Resources[token]
	return spec, ok, nil
}

type partialPackageSpecSource struct {
	spec *PartialPackageSpec
}

func (s partialPackageSpecSource) GetTypeDefSpec(token string) (ComplexTypeSpec, bool, error) {
	rawSpec, ok := s.spec.Types[token]
	if !ok {
		return ComplexTypeSpec{}, false, nil
	}

	var spec ComplexTypeSpec
	if err := parseJSONPropertyValue(rawSpec, &spec); err != nil {
		return ComplexTypeSpec{}, false, err
	}
	return spec, true, nil
}

func (s partialPackageSpecSource) GetFunctionSpec(token string) (FunctionSpec, bool, error) {
	rawSpec, ok := s.spec.Functions[token]
	if !ok {
		return FunctionSpec{}, false, nil
	}

	var spec FunctionSpec
	if err := parseJSONPropertyValue(rawSpec, &spec); err != nil {
		return FunctionSpec{}, false, err
	}
	return spec, true, nil
}

func (s partialPackageSpecSource) GetResourceSpec(token string) (ResourceSpec, bool, error) {
	var rawSpec json.RawMessage
	if token == "pulumi:providers:"+s.spec.Name {
		rawSpec = s.spec.Provider
	} else {
		raw, ok := s.spec.Resources[token]
		if !ok {
			return ResourceSpec{}, false, nil
		}
		rawSpec = raw
	}

	var spec ResourceSpec
	if err := parseJSONPropertyValue(rawSpec, &spec); err != nil {
		return ResourceSpec{}, false, err
	}
	return spec, true, nil
}

// types facilitates interning (only storing a single reference to an object) during schema processing. The fields
// correspond to fields in the schema, and are populated during the binding process.
type types struct {
	pkg     *Package
	spec    specSource
	loader  Loader
	loadCtx io.Closer

	typeDefs     map[string]Type      // objects and enums
	functionDefs map[string]*Function // function definitions
	resourceDefs map[string]*Resource // resource definitions

	resources map[string]*ResourceType
	arrays    map[Type]*ArrayType
	maps      map[Type]*MapType
	unions    map[string]*UnionType
	tokens    map[string]*TokenType
	inputs    map[Type]*InputType
	optionals map[Type]*OptionalType

	// A pointer to the package reference that `types` is a part of if it exists.
	bindToReference PackageReference
}

func (t *types) Close() error {
	if t.loadCtx != nil {
		return t.loadCtx.Close()
	}
	return nil
}

// The package which bound types will link back to.
func (t *types) externalPackage() PackageReference {
	if t.bindToReference != nil {
		return t.bindToReference
	}
	return t.pkg.Reference()
}

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
	var diags hcl.Diagnostics

	path := memberPath(section, token)
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		err := errorf(path, "invalid token '%s' (should have three parts)", token)
		diags = diags.Append(err)
		// Early return because the other two error checks panic if len(parts) < 3
		return diags
	}
	if !allowedPackageNames[parts[0]] {
		err := errorf(path, "invalid token '%s' (must have package name '%s')", token, spec.Name)
		diags = diags.Append(err)
	}
	if (parts[1] == "" || strings.EqualFold(parts[1], "index")) && strings.EqualFold(parts[2], "provider") {
		err := errorf(path, "invalid token '%s' (provider is a reserved word for the root module)", token)
		diags = diags.Append(err)
	}
	return diags
}

// This is for validating non-reference type tokens.
func (spec *PackageSpec) validateTypeTokens() hcl.Diagnostics {
	var diags hcl.Diagnostics
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
	elements []Type, defaultType Type, discriminator string, mapping map[string]string,
) *UnionType {
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

func (t *types) bindTypeDef(token string) (Type, hcl.Diagnostics, error) {
	// Check to see if this type has already been bound.
	if typ, ok := t.typeDefs[token]; ok {
		return typ, nil, nil
	}

	// Check to see if we have a definition for this type. If we don't, just return nil.
	spec, ok, err := t.spec.GetTypeDefSpec(token)
	if err != nil || !ok {
		return nil, nil, err
	}

	path := memberPath("types", token)

	// Is this an object type?
	if spec.Type == "object" {
		// Declare the type.
		//
		// It's important that we set the token here. This package interns types so that they can be equality-compared
		// for identity. Types are interned based on their string representation, and the string representation of an
		// object type is its token. While this doesn't affect object types directly, it breaks the interning of types
		// that reference object types (e.g. arrays, maps, unions)
		obj := &ObjectType{Token: token, IsOverlay: spec.IsOverlay}
		obj.InputShape = &ObjectType{Token: token, PlainShape: obj, IsOverlay: spec.IsOverlay}
		t.typeDefs[token] = obj

		diags, err := t.bindObjectTypeDetails(path, obj, token, spec.ObjectTypeSpec)
		if err != nil {
			return nil, diags, err
		}
		return obj, diags, nil
	}

	// Otherwise, bind an enum type.
	enum, diags := t.bindEnumType(token, spec)
	t.typeDefs[token] = enum
	return enum, diags, nil
}

func (t *types) bindResourceTypeDef(token string) (*ResourceType, hcl.Diagnostics, error) {
	if typ, ok := t.resources[token]; ok {
		return typ, nil, nil
	}

	res, diags, err := t.bindResourceDef(token)
	if err != nil {
		return nil, nil, err
	}
	if res == nil {
		return nil, nil, nil
	}
	typ := &ResourceType{Token: token, Resource: res}
	t.resources[token] = typ
	return typ, diags, nil
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
		pkg, err := LoadPackageReference(t.loader, ref.Package, ref.Version)
		if err != nil {
			return nil, nil, fmt.Errorf("resolving package %v: %w", ref.URL, err)
		}

		switch ref.Kind {
		case typesRef:
			typ, ok, err := pkg.Types().Get(ref.Token)
			if err != nil {
				return nil, nil, fmt.Errorf("loading type %v: %w", ref.Token, err)
			}
			if !ok {
				typ, diags := invalidType(errorf(path, "type %v not found in package %v", ref.Token, ref.Package))
				return typ, diags, nil
			}
			if obj, ok := typ.(*ObjectType); ok && inputShape {
				typ = obj.InputShape
			}
			return typ, nil, nil
		case resourcesRef, providerRef:
			typ, ok, err := pkg.Resources().GetType(ref.Token)
			if err != nil {
				return nil, nil, fmt.Errorf("loading type %v: %w", ref.Token, err)
			}
			if !ok {
				typ, diags := invalidType(errorf(path, "resource type %v not found in package %v", ref.Token, ref.Package))
				return typ, diags, nil
			}
			return typ, nil, nil
		}
	}

	switch ref.Kind {
	case typesRef:
		// Try to bind this as a reference to a type defined by this package.
		typ, diags, err := t.bindTypeDef(ref.Token)
		if err != nil {
			return nil, diags, err
		}
		switch typ := typ.(type) {
		case *ObjectType:
			// If the type is an object type, we might need to return its input shape.
			if inputShape {
				return typ.InputShape, diags, nil
			}
			return typ, diags, nil
		case *EnumType:
			return typ, diags, nil
		default:
			contract.Assertf(typ == nil, "unexpected type %T", typ)
		}

		// If the type is not a known type, bind it as an opaque token type.
		tokenType, ok := t.tokens[ref.Token]
		if !ok {
			tokenType = &TokenType{Token: ref.Token}
			if spec.Type != "" {
				ut, primDiags := t.bindPrimitiveType(path, spec.Type)
				diags = diags.Extend(primDiags)

				tokenType.UnderlyingType = ut
			}
			t.tokens[ref.Token] = tokenType
		}
		return tokenType, diags, nil
	case resourcesRef, providerRef:
		typ, diags, err := t.bindResourceTypeDef(ref.Token)
		if err != nil {
			return nil, diags, err
		}
		if typ == nil {
			typ, diags := invalidType(errorf(path, "resource type %v not found in package %v", ref.Token, ref.Package))
			return typ, diags, nil
		}
		return typ, diags, nil
	default:
		typ, diags := invalidType(errorf(path, "failed to parse ref %s", spec.Ref))
		return typ, diags, nil
	}
}

func (t *types) bindTypeSpecOneOf(path string, spec TypeSpec, inputShape bool) (Type, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics
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
		e, typDiags, err := t.bindTypeSpec(fmt.Sprintf("%s/oneOf/%v", path, i), spec, inputShape)
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

func (t *types) bindTypeSpec(path string, spec TypeSpec,
	inputShape bool,
) (result Type, diags hcl.Diagnostics, err error) {
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
		return t.bindTypeSpecOneOf(path, spec, inputShape)
	}

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

		elementType, elementDiags, err := t.bindTypeSpec(path+"/items", *spec.Items, inputShape)
		diags = diags.Extend(elementDiags)
		if err != nil {
			return nil, diags, err
		}

		return t.newArrayType(elementType), diags, nil
	case "object":
		elementType, elementDiags, err := t.bindTypeSpec(path, TypeSpec{Type: "string"}, inputShape)
		contract.Assertf(len(elementDiags) == 0, "unexpected diagnostics: %v", elementDiags)
		contract.Assertf(err == nil, "error binding type spec")

		if spec.AdditionalProperties != nil {
			et, elementDiags, err := t.bindTypeSpec(path+"/additionalProperties", *spec.AdditionalProperties, inputShape)
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
		v, ok := value.(bool)
		if !ok {
			return false, typeError("boolean")
		}
		return v, nil
	case IntType:
		v, ok := value.(int)
		if !ok {
			v, ok := value.(float64)
			if !ok {
				return 0, typeError("integer")
			}
			if math.Trunc(v) != v || v < math.MinInt32 || v > math.MaxInt32 {
				return 0, typeError("integer")
			}
			return int32(v), nil
		}
		if v < math.MinInt32 || v > math.MaxInt32 {
			return 0, typeError("integer")
		}
		return int32(v), nil
	case NumberType:
		v, ok := value.(float64)
		if !ok {
			return 0.0, typeError("number")
		}
		return v, nil
	case StringType:
		v, ok := value.(string)
		if !ok {
			return 0.0, typeError("string")
		}
		return v, nil
	default:
		if _, isInvalid := typ.(*InvalidType); isInvalid {
			return nil, nil
		}
		return nil, hcl.Diagnostics{errorf(path, "type %v cannot have a constant value; only booleans, integers, "+
			"numbers and strings may have constant values", typ)}
	}
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
	inputShape bool,
) ([]*Property, map[string]*Property, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	// Bind property types and constant or default values.
	propertyMap := map[string]*Property{}
	result := slice.Prealloc[*Property](len(properties))
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
		typ, typDiags, err := t.bindTypeSpec(propertyPath, spec.TypeSpec, inputShape)
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
			Name:                 name,
			Comment:              spec.Description,
			Type:                 t.newOptionalType(typ),
			ConstValue:           cv,
			DefaultValue:         dv,
			DeprecationMessage:   spec.DeprecationMessage,
			Language:             language,
			Secret:               spec.Secret,
			ReplaceOnChanges:     spec.ReplaceOnChanges,
			WillReplaceOnChanges: spec.WillReplaceOnChanges,
			Plain:                spec.Plain,
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
	spec ObjectTypeSpec,
) (hcl.Diagnostics, error) {
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

	obj.PackageReference = t.externalPackage()
	obj.Token = token
	obj.Comment = spec.Description
	obj.Language = language
	obj.Properties = properties
	obj.properties = propertyMap
	obj.IsOverlay = spec.IsOverlay

	obj.InputShape.PackageReference = t.externalPackage()
	obj.InputShape.Token = token
	obj.InputShape.Comment = spec.Description
	obj.InputShape.Language = language
	obj.InputShape.Properties = inputProperties
	obj.InputShape.properties = inputPropertyMap

	return diags, nil
}

// bindAnonymousObjectType is used for binding object types that do not appear as part of a package's defined types.
// This includes state inputs for resources that have them and function inputs and outputs.
// Object types defined by a package are bound by bindTypeDef.
func (t *types) bindAnonymousObjectType(path, token string, spec ObjectTypeSpec) (*ObjectType, hcl.Diagnostics, error) {
	obj := &ObjectType{}
	obj.InputShape = &ObjectType{PlainShape: obj}
	obj.IsOverlay = spec.IsOverlay

	diags, err := t.bindObjectTypeDetails(path, obj, token, spec)
	if err != nil {
		return nil, diags, err
	}
	return obj, diags, nil
}

func (t *types) bindEnumType(token string, spec ComplexTypeSpec) (*EnumType, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	path := memberPath("types", token)

	typ, typDiags := t.bindPrimitiveType(path+"/type", spec.Type)
	diags = diags.Extend(typDiags)

	switch typ {
	case StringType, IntType, NumberType, BoolType:
		// OK
	default:
		if _, isInvalid := typ.(*InvalidType); !isInvalid {
			diags = diags.Append(errorf(path+"/type",
				"enums may only be of type string, integer, number or boolean"))
		}
	}

	values := make([]*Enum, len(spec.Enum))
	for i, spec := range spec.Enum {
		value, valueDiags := bindConstValue(fmt.Sprintf("%s/enum/%v/value", path, i), "enum", spec.Value, typ)
		diags = diags.Extend(valueDiags)

		values[i] = &Enum{
			Value:              value,
			Comment:            spec.Description,
			Name:               spec.Name,
			DeprecationMessage: spec.DeprecationMessage,
		}
	}

	return &EnumType{
		PackageReference: t.externalPackage(),
		Token:            token,
		Elements:         values,
		ElementType:      typ,
		Comment:          spec.Description,
		IsOverlay:        spec.IsOverlay,
	}, diags
}

func (t *types) finishTypes(tokens []string) ([]Type, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	// Ensure all of the types defined by the package are bound.
	for _, token := range tokens {
		_, typeDiags, err := t.bindTypeDef(token)
		if err != nil {
			return nil, nil, fmt.Errorf("error binding type %v", token)
		}
		diags = diags.Extend(typeDiags)
	}

	// Build the type list.
	typeList := slice.Prealloc[Type](len(t.resources))
	for _, t := range t.resources {
		typeList = append(typeList, t)
	}
	for _, t := range t.typeDefs {
		typeList = append(typeList, t)
		if obj, ok := t.(*ObjectType); ok {
			// t is a plain shape: add it and its corresponding input shape to the type list.
			typeList = append(typeList, obj.InputShape)
		}
	}
	for _, t := range t.arrays {
		typeList = append(typeList, t)
	}
	for _, t := range t.maps {
		typeList = append(typeList, t)
	}
	for _, t := range t.unions {
		typeList = append(typeList, t)
	}
	for _, t := range t.tokens {
		typeList = append(typeList, t)
	}

	sort.Slice(typeList, func(i, j int) bool {
		return typeList[i].String() < typeList[j].String()
	})

	return typeList, diags, nil
}

func bindMethods(path, resourceToken string, methods map[string]string,
	types *types,
) ([]*Method, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	names := slice.Prealloc[string](len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)

	result := slice.Prealloc[*Method](len(methods))
	for _, name := range names {
		token := methods[name]

		methodPath := path + "/" + name

		function, functionDiags, err := types.bindFunctionDef(token)
		if err != nil {
			return nil, nil, err
		}
		diags = diags.Extend(functionDiags)

		if function == nil {
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
	return result, diags, nil
}

func bindConfig(spec ConfigSpec, types *types) ([]*Property, hcl.Diagnostics, error) {
	properties, _, diags, err := types.bindProperties("#/config/variables", spec.Variables,
		"#/config/defaults", spec.Required, false)
	return properties, diags, err
}

func (t *types) bindResourceDef(token string) (res *Resource, diags hcl.Diagnostics, err error) {
	if res, ok := t.resourceDefs[token]; ok {
		return res, nil, nil
	}

	// Declare the resource.
	res = &Resource{}

	if token == "pulumi:providers:"+t.pkg.Name {
		t.resourceDefs[token] = res
		diags, err = t.bindProvider(res)
	} else {
		spec, ok, specErr := t.spec.GetResourceSpec(token)
		if specErr != nil || !ok {
			return nil, nil, err
		}
		t.resourceDefs[token] = res
		diags, err = t.bindResourceDetails(memberPath("resources", token), token, spec, res)
	}
	if err != nil {
		return nil, diags, err
	}
	return res, diags, nil
}

func (t *types) bindResourceDetails(path, token string, spec ResourceSpec, decl *Resource) (hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	if len(spec.Plain) > 0 {
		diags = diags.Append(errorf(path+"/plain", "plain has been removed; property types must be marked as plain instead"))
	}
	if len(spec.PlainInputs) > 0 {
		diags = diags.Append(errorf(path+"/plainInputs",
			"plainInputs has been removed; individual property types must be marked as plain instead"))
	}

	properties, _, propertyDiags, err := t.bindProperties(path+"/properties", spec.Properties,
		path+"/required", spec.Required, false)
	diags = diags.Extend(propertyDiags)
	if err != nil {
		return diags, fmt.Errorf("failed to bind properties for %v: %w", token, err)
	}

	inputProperties, _, inputDiags, err := t.bindProperties(path+"/inputProperties", spec.InputProperties,
		path+"/requiredInputs", spec.RequiredInputs, true)
	diags = diags.Extend(inputDiags)
	if err != nil {
		return diags, fmt.Errorf("failed to bind input properties for %v: %w", token, err)
	}

	methods, methodDiags, err := bindMethods(path+"/methods", token, spec.Methods, t)
	diags = diags.Extend(methodDiags)
	if err != nil {
		return diags, fmt.Errorf("failed to bind methods for %v: %w", token, err)
	}

	for _, method := range methods {
		if _, ok := spec.Properties[method.Name]; ok {
			diags = diags.Append(errorf(path+"/methods/"+method.Name, "%v already has a property named %s", token, method.Name))
		}
	}

	var stateInputs *ObjectType
	if spec.StateInputs != nil {
		si, stateDiags, err := t.bindAnonymousObjectType(path+"/stateInputs", token+"Args", *spec.StateInputs)
		diags = diags.Extend(stateDiags)
		if err != nil {
			return diags, fmt.Errorf("error binding inputs for %v: %w", token, err)
		}
		stateInputs = si.InputShape
	}

	aliases := slice.Prealloc[*Alias](len(spec.Aliases))
	for _, a := range spec.Aliases {
		aliases = append(aliases, &Alias{Name: a.Name, Project: a.Project, Type: a.Type})
	}

	language := make(map[string]interface{})
	for name, raw := range spec.Language {
		language[name] = json.RawMessage(raw)
	}

	*decl = Resource{
		PackageReference:   t.externalPackage(),
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
	}
	return diags, nil
}

func (t *types) bindProvider(decl *Resource) (hcl.Diagnostics, error) {
	spec, ok, err := t.spec.GetResourceSpec("pulumi:providers:" + t.pkg.Name)
	if err != nil {
		return nil, err
	}
	contract.Assertf(ok, "provider resource %q not found", t.pkg.Name)

	diags, err := t.bindResourceDetails("#/provider", "pulumi:providers:"+t.pkg.Name, spec, decl)
	if err != nil {
		return diags, err
	}
	decl.IsProvider = true

	// Since non-primitive provider configuration is currently JSON serialized, we can't handle it without
	// modifying the path by which it's looked up. As a temporary workaround to enable access to config which
	// values which are primitives, we'll simply remove any properties for the provider resource which are not
	// strings, or types with an underlying type of string, before we generate the provider code.
	stringProperties := slice.Prealloc[*Property](len(decl.Properties))
	for _, prop := range decl.Properties {
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
	decl.Properties = stringProperties

	return diags, nil
}

func (t *types) finishResources(tokens []string) (*Resource, []*Resource, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	provider, provDiags, err := t.bindResourceTypeDef("pulumi:providers:" + t.pkg.Name)
	if err != nil {
		return nil, nil, diags, fmt.Errorf("error binding provider: %w", err)
	}
	diags = diags.Extend(provDiags)

	resources := slice.Prealloc[*Resource](len(tokens))
	for _, token := range tokens {
		res, resDiags, err := t.bindResourceTypeDef(token)
		diags = diags.Extend(resDiags)
		if err != nil {
			return nil, nil, diags, fmt.Errorf("error binding resource %v: %w", token, err)
		}
		resources = append(resources, res.Resource)
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Token < resources[j].Token
	})

	return provider.Resource, resources, diags, nil
}

func (t *types) bindFunctionDef(token string) (*Function, hcl.Diagnostics, error) {
	if fn, ok := t.functionDefs[token]; ok {
		return fn, nil, nil
	}

	spec, ok, err := t.spec.GetFunctionSpec(token)
	if err != nil || !ok {
		return nil, nil, nil
	}

	var diags hcl.Diagnostics

	path := memberPath("functions", token)

	// Check that spec.MultiArgumentInputs => spec.Inputs
	if len(spec.MultiArgumentInputs) > 0 && spec.Inputs == nil {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "cannot specify multi-argument inputs without specifying inputs",
		})
	}
	var inputs *ObjectType
	if spec.Inputs != nil {
		ins, inDiags, err := t.bindAnonymousObjectType(path+"/inputs", token+"Args", *spec.Inputs)
		diags = diags.Extend(inDiags)
		if err != nil {
			return nil, diags, fmt.Errorf("error binding inputs for function %v: %w", token, err)
		}

		if len(spec.MultiArgumentInputs) > 0 {
			idx := make(map[string]int, len(spec.MultiArgumentInputs))
			for i, k := range spec.MultiArgumentInputs {
				idx[k] = i
			}
			// Check that MultiArgumentInputs matches up 1:1 with the input properties
			for k, i := range idx {
				if _, ok := spec.Inputs.Properties[k]; !ok {
					diags = diags.Append(&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  fmt.Sprintf("multiArgumentInputs[%d] refers to non-existent property %#v", i, k),
					})
				}
			}
			var detailGiven bool
			for k := range spec.Inputs.Properties {
				if _, ok := idx[k]; !ok {
					diag := hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  fmt.Sprintf("Property %#v not specified by multiArgumentInputs", k),
					}
					if !detailGiven {
						detailGiven = true
						diag.Detail = "If multiArgumentInputs is given, all properties must be specified"
					}
					diags = diags.Append(&diag)
				}
			}

			// Order output properties as specified by MultiArgumentInputs
			sortProps := func(props []*Property) {
				sort.Slice(props, func(i, j int) bool {
					return idx[props[i].Name] < idx[props[j].Name]
				})
			}
			sortProps(ins.Properties)
			if ins.InputShape != nil {
				sortProps(ins.InputShape.Properties)
			}
			if ins.PlainShape != nil {
				sortProps(ins.PlainShape.Properties)
			}
		}

		inputs = ins
	}

	var outputs *ObjectType

	language := make(map[string]interface{})
	for name, raw := range spec.Language {
		language[name] = json.RawMessage(raw)
	}

	var inlineObjectAsReturnType bool
	var returnType Type
	var returnTypePlain bool
	if spec.ReturnType != nil && spec.Outputs == nil {
		// compute the return type from the spec
		if spec.ReturnType.ObjectTypeSpec != nil {
			// bind as an object type
			outs, outDiags, err := t.bindAnonymousObjectType(path+"/outputs", token+"Result", *spec.ReturnType.ObjectTypeSpec)
			diags = diags.Extend(outDiags)
			if err != nil {
				return nil, diags, fmt.Errorf("error binding outputs for function %v: %w", token, err)
			}
			returnType = outs
			outputs = outs
			inlineObjectAsReturnType = true
			returnTypePlain = spec.ReturnType.ObjectTypeSpecIsPlain
		} else if spec.ReturnType.TypeSpec != nil {
			out, outDiags, err := t.bindTypeSpec(path+"/outputs", *spec.ReturnType.TypeSpec, false)
			diags = diags.Extend(outDiags)
			if err != nil {
				return nil, diags, fmt.Errorf("error binding outputs for function %v: %w", token, err)
			}
			returnType = out
			returnTypePlain = spec.ReturnType.TypeSpec.Plain
		} else {
			// Setting `spec.ReturnType` to a value without setting either `TypeSpec` or `ObjectTypeSpec`
			// indicates a logical bug in our marshaling code.
			return nil, diags, fmt.Errorf("error binding outputs for function %v: invalid return type", token)
		}
	} else if spec.Outputs != nil {
		// bind the outputs when the specs don't rely on the new ReturnType field
		outs, outDiags, err := t.bindAnonymousObjectType(path+"/outputs", token+"Result", *spec.Outputs)
		diags = diags.Extend(outDiags)
		if err != nil {
			return nil, diags, fmt.Errorf("error binding outputs for function %v: %w", token, err)
		}
		outputs = outs
		returnType = outs
		inlineObjectAsReturnType = true
	}

	fn := &Function{
		PackageReference:         t.externalPackage(),
		Token:                    token,
		Comment:                  spec.Description,
		Inputs:                   inputs,
		MultiArgumentInputs:      len(spec.MultiArgumentInputs) > 0,
		InlineObjectAsReturnType: inlineObjectAsReturnType,
		Outputs:                  outputs,
		ReturnType:               returnType,
		ReturnTypePlain:          returnTypePlain,
		DeprecationMessage:       spec.DeprecationMessage,
		Language:                 language,
		IsOverlay:                spec.IsOverlay,
	}
	t.functionDefs[token] = fn

	return fn, diags, nil
}

func (t *types) finishFunctions(tokens []string) ([]*Function, hcl.Diagnostics, error) {
	var diags hcl.Diagnostics

	functions := slice.Prealloc[*Function](len(tokens))
	for _, token := range tokens {
		f, fdiags, err := t.bindFunctionDef(token)
		diags = diags.Extend(fdiags)
		if err != nil {
			return nil, diags, fmt.Errorf("error binding function %v: %w", token, err)
		}
		functions = append(functions, f)
	}
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Token < functions[j].Token
	})

	return functions, diags, nil
}
