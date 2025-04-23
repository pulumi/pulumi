// Copyright 2022-2024, Pulumi Corporation.
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
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/segmentio/encoding/json"
)

// A PackageReference represents a references Pulumi Package. Applications that do not need access to the entire
// definition of a Pulumi Package should use PackageReference rather than Package, as the former uses memory more
// efficiently than the latter by binding package members on-demand.
type PackageReference interface {
	// Name returns the package name.
	Name() string
	// Version returns the package version.
	Version() *semver.Version

	// Description returns the packages description.
	Description() string

	// Publisher returns the package publisher.
	Publisher() string
	// Namespace returns the package namespace.
	Namespace() string
	// Repository returns the package repository.
	Repository() string

	// SupportPack specifies the package definition can be packed by language plugins, this is always true for
	// parameterized packages.
	SupportPack() bool

	// Types returns the package's types.
	Types() PackageTypes
	// Config returns the package's configuration variables, if any.
	Config() ([]*Property, error)
	// Provider returns the package's provider.
	Provider() (*Resource, error)
	// Resources returns the package's resources.
	Resources() PackageResources
	// Functions returns the package's functions.
	Functions() PackageFunctions

	// The language specific metadata for a given language.
	//
	// The package must have been originally bound with a matching [Language]
	// importer.
	Language(string) (any, error)

	// TokenToModule extracts a package member's module name from its token.
	TokenToModule(token string) string

	// Definition fully loads the referenced package and returns the result.
	Definition() (*Package, error)
}

// PackageTypes provides random and sequential access to a package's types.
type PackageTypes interface {
	// Range returns a range iterator for the package's types. Call Next to
	// advance the iterator, and Token/Type to access each entry. Type definitions
	// are loaded on demand. Iteration order is undefined.
	//
	// Example:
	//
	//     for it := pkg.Types().Range(); it.Next(); {
	//         token := it.Token()
	//         typ, err := it.Type()
	//         ...
	//     }
	//
	Range() TypesIter

	// Get finds and loads the type with the given token. If the type is not found,
	// this function returns (nil, false, nil).
	Get(token string) (Type, bool, error)
}

// TypesIter is an iterator for ranging over a package's types. See PackageTypes.Range.
type TypesIter interface {
	Token() string
	Type() (Type, error)
	Next() bool
}

// PackageResources provides random and sequential access to a package's resources.
type PackageResources interface {
	// Range returns a range iterator for the package's resources. Call Next to
	// advance the iterator, and Token/Resource to access each entry. Resource definitions
	// are loaded on demand. Iteration order is undefined.
	//
	// Example:
	//
	//     for it := pkg.Resources().Range(); it.Next(); {
	//         token := it.Token()
	//         res, err := it.Resource()
	//         ...
	//     }
	//
	Range() ResourcesIter

	// Get finds and loads the resource with the given token. If the resource is not found,
	// this function returns (nil, false, nil).
	Get(token string) (*Resource, bool, error)

	// GetType loads the *ResourceType that corresponds to a given resource definition.
	GetType(token string) (*ResourceType, bool, error)
}

// ResourcesIter is an iterator for ranging over a package's resources. See PackageResources.Range.
type ResourcesIter interface {
	Token() string
	Resource() (*Resource, error)
	Next() bool
}

// PackageFunctions provides random and sequential access to a package's functions.
type PackageFunctions interface {
	// Range returns a range iterator for the package's functions. Call Next to
	// advance the iterator, and Token/Function to access each entry. Function definitions
	// are loaded on demand. Iteration order is undefined.
	//
	// Example:
	//
	//     for it := pkg.Functions().Range(); it.Next(); {
	//         token := it.Token()
	//         fn, err := it.Function()
	//         ...
	//     }
	//
	Range() FunctionsIter

	// Get finds and loads the function with the given token. If the function is not found,
	// this function returns (nil, false, nil).
	Get(token string) (*Function, bool, error)
}

// FunctionsIter is an iterator for ranging over a package's functions. See PackageFunctions.Range.
type FunctionsIter interface {
	Token() string
	Function() (*Function, error)
	Next() bool
}

// packageDefRef is an implementation of PackageReference backed by a *Package.
type packageDefRef struct {
	pkg *Package
}

func (p packageDefRef) Name() string {
	return p.pkg.Name
}

func (p packageDefRef) Version() *semver.Version {
	return p.pkg.Version
}

func (p packageDefRef) Description() string {
	return p.pkg.Description
}

func (p packageDefRef) Publisher() string {
	return p.pkg.Publisher
}

func (p packageDefRef) Namespace() string {
	return p.pkg.Namespace
}

func (p packageDefRef) Repository() string {
	return p.pkg.Repository
}

func (p packageDefRef) SupportPack() bool {
	return p.pkg.SupportPack
}

func (p packageDefRef) Types() PackageTypes {
	return packageDefTypes{p.pkg}
}

func (p packageDefRef) Config() ([]*Property, error) {
	return p.pkg.Config, nil
}

func (p packageDefRef) Provider() (*Resource, error) {
	return p.pkg.Provider, nil
}

func (p packageDefRef) Resources() PackageResources {
	return packageDefResources{p.pkg}
}

func (p packageDefRef) Functions() PackageFunctions {
	return packageDefFunctions{p.pkg}
}

func (p packageDefRef) Language(language string) (any, error) {
	return p.pkg.Language[language], nil
}

func (p packageDefRef) TokenToModule(token string) string {
	return p.pkg.TokenToModule(token)
}

func (p packageDefRef) Definition() (*Package, error) {
	return p.pkg, nil
}

type packageDefTypes struct {
	*Package
}

func (p packageDefTypes) Range() TypesIter {
	return &packageDefTypesIter{types: p.Types}
}

func (p packageDefTypes) Get(token string) (Type, bool, error) {
	def, ok := p.typeTable[token]
	return def, ok, nil
}

type packageDefTypesIter struct {
	types []Type
	t     Type
}

func (i *packageDefTypesIter) Token() string {
	if obj, ok := i.t.(*ObjectType); ok {
		return obj.Token
	}
	return i.t.(*EnumType).Token
}

func (i *packageDefTypesIter) Type() (Type, error) {
	return i.t, nil
}

func (i *packageDefTypesIter) Next() bool {
	if len(i.types) == 0 {
		return false
	}
	i.t, i.types = i.types[0], i.types[1:]
	return true
}

type packageDefResources struct {
	*Package
}

func (p packageDefResources) Range() ResourcesIter {
	return &packageDefResourcesIter{resources: p.Resources}
}

func (p packageDefResources) Get(token string) (*Resource, bool, error) {
	if token == p.Provider.Token {
		return p.Provider, true, nil
	}

	def, ok := p.resourceTable[token]
	return def, ok, nil
}

func (p packageDefResources) GetType(token string) (*ResourceType, bool, error) {
	typ, ok := p.GetResourceType(token)
	return typ, ok, nil
}

type packageDefResourcesIter struct {
	resources []*Resource
	r         *Resource
}

func (i *packageDefResourcesIter) Token() string {
	return i.r.Token
}

func (i *packageDefResourcesIter) Resource() (*Resource, error) {
	return i.r, nil
}

func (i *packageDefResourcesIter) Next() bool {
	if len(i.resources) == 0 {
		return false
	}
	i.r, i.resources = i.resources[0], i.resources[1:]
	return true
}

type packageDefFunctions struct {
	*Package
}

func (p packageDefFunctions) Range() FunctionsIter {
	return &packageDefFunctionsIter{functions: p.Functions}
}

func (p packageDefFunctions) Get(token string) (*Function, bool, error) {
	def, ok := p.functionTable[token]
	return def, ok, nil
}

type packageDefFunctionsIter struct {
	functions []*Function
	f         *Function
}

func (i *packageDefFunctionsIter) Token() string {
	return i.f.Token
}

func (i *packageDefFunctionsIter) Function() (*Function, error) {
	return i.f, nil
}

func (i *packageDefFunctionsIter) Next() bool {
	if len(i.functions) == 0 {
		return false
	}
	i.f, i.functions = i.functions[0], i.functions[1:]
	return true
}

// PartialPackage is an implementation of PackageReference that loads and binds package members on demand. A
// PartialPackage is backed by a PartialPackageSpec, which leaves package members in their JSON-encoded form until
// they are required. PartialPackages are created using ImportPartialSpec.
type PartialPackage struct {
	// This mutex guards two operations:
	// - access to PartialPackage.def
	// - package binding operations
	m sync.Mutex

	spec      *PartialPackageSpec
	languages map[string]Language
	types     *types

	config []*Property

	def *Package
}

func (p *PartialPackage) Name() string {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def.Name
	}
	return p.types.pkg.Name
}

func (p *PartialPackage) Version() *semver.Version {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def.Version
	}
	return p.types.pkg.Version
}

func (p *PartialPackage) Description() string {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def.Description
	}
	return p.types.pkg.Description
}

func (p *PartialPackage) Publisher() string {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def.Publisher
	}
	return p.types.pkg.Publisher
}

func (p *PartialPackage) Namespace() string {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def.Namespace
	}
	return p.types.pkg.Namespace
}

func (p *PartialPackage) Repository() string {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def.Repository
	}
	return p.types.pkg.Repository
}

func (p *PartialPackage) SupportPack() bool {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def.SupportPack
	}
	return p.types.pkg.SupportPack
}

func (p *PartialPackage) Types() PackageTypes {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return packageDefTypes{p.def}
	}
	return partialPackageTypes{p}
}

func (p *PartialPackage) Config() ([]*Property, error) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def.Config, nil
	}

	if p.config != nil {
		return p.config, nil
	}

	return p.bindConfig()
}

func (p *PartialPackage) bindConfig() ([]*Property, error) {
	var spec ConfigSpec
	if err := parseJSONPropertyValue(p.spec.Config, &spec); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	config, diags, err := bindConfig(spec, p.types, SchemaValidationOptions{})
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}
	p.config = config

	return p.config, nil
}

func (p *PartialPackage) Provider() (*Resource, error) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def.Provider, nil
	}

	provider, diags, err := p.types.bindResourceDef("pulumi:providers:"+p.spec.Name, SchemaValidationOptions{})
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, err
	}
	return provider, nil
}

func (p *PartialPackage) Resources() PackageResources {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return packageDefResources{p.def}
	}
	return partialPackageResources{p}
}

func (p *PartialPackage) Functions() PackageFunctions {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return packageDefFunctions{p.def}
	}
	return partialPackageFunctions{p}
}

func (p *PartialPackage) Language(language string) (any, error) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		if l, ok := p.def.Language[language]; ok {
			return l, nil
		}
	}

	val, ok := p.spec.Language[language]
	if !ok {
		return nil, nil
	}
	importer, ok := p.languages[language]
	if !ok {
		return nil, nil
	}
	imported, err := importer.ImportPackageSpec(json.RawMessage(val))
	if err != nil {
		return nil, err
	}
	if p.def == nil {
		p.def = new(Package)
	}
	if p.def.Language == nil {
		p.def.Language = map[string]any{}
	}
	p.def.Language[language] = imported

	return imported, nil
}

func (p *PartialPackage) TokenToModule(token string) string {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def.TokenToModule(token)
	}
	return p.types.pkg.TokenToModule(token)
}

func (p *PartialPackage) Definition() (*Package, error) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def, nil
	}

	config, err := p.bindConfig()
	if err != nil {
		return nil, err
	}

	var diags hcl.Diagnostics
	provider, resources, resourceDiags, err := p.types.finishResources(sortedKeys(p.spec.Resources), SchemaValidationOptions{})
	if err != nil {
		return nil, err
	}
	diags = diags.Extend(resourceDiags)

	functions, functionDiags, err := p.types.finishFunctions(sortedKeys(p.spec.Functions), SchemaValidationOptions{})
	if err != nil {
		return nil, err
	}
	diags = diags.Extend(functionDiags)

	typeList, typeDiags, err := p.types.finishTypes(sortedKeys(p.spec.Types), SchemaValidationOptions{})
	if err != nil {
		return nil, err
	}
	diags = diags.Extend(typeDiags)

	pkg := p.types.pkg
	pkg.Config = config
	pkg.Types = typeList
	pkg.Provider = provider
	pkg.Resources = resources
	pkg.Functions = functions
	pkg.resourceTable = p.types.resourceDefs
	pkg.functionTable = p.types.functionDefs
	pkg.typeTable = p.types.typeDefs
	pkg.resourceTypeTable = p.types.resources
	if p.spec.Parameterization != nil {
		pkg.Parameterization = &Parameterization{
			BaseProvider: BaseProvider{
				Name:    p.spec.Parameterization.BaseProvider.Name,
				Version: semver.MustParse(p.spec.Parameterization.BaseProvider.Version),
			},
			Parameter: p.spec.Parameterization.Parameter,
		}
	}
	if err := pkg.ImportLanguages(p.languages); err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}

	contract.IgnoreClose(p.types)
	p.spec = nil
	p.types = nil
	p.languages = nil
	p.config = nil
	p.def = pkg

	return pkg, nil
}

// Snapshot returns a definition for the package that contains only the members that have been accessed thus far. If
// Definition has been called, the returned definition will include all of the package's members. It is safe to call
// Snapshot multiple times.
func (p *PartialPackage) Snapshot() (*Package, error) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.def != nil {
		return p.def, nil
	}

	config := p.config
	provider := p.types.resourceDefs["pulumi:providers:"+p.spec.Name]

	resources := slice.Prealloc[*Resource](len(p.types.resourceDefs))
	resourceDefs := make(map[string]*Resource, len(p.types.resourceDefs))
	for token, res := range p.types.resourceDefs {
		resources, resourceDefs[token] = append(resources, res), res
	}
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Token < resources[j].Token
	})

	functions := slice.Prealloc[*Function](len(p.types.functionDefs))
	functionDefs := make(map[string]*Function, len(p.types.functionDefs))
	for token, fn := range p.types.functionDefs {
		functions, functionDefs[token] = append(functions, fn), fn
	}
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Token < functions[j].Token
	})

	typeList, diags, err := p.types.finishTypes(nil, SchemaValidationOptions{})
	contract.AssertNoErrorf(err, "error snapshotting types")
	contract.Assertf(len(diags) == 0, "unexpected diagnostics: %v", diags)

	typeDefs := make(map[string]Type, len(p.types.typeDefs))
	for token, typ := range p.types.typeDefs {
		typeDefs[token] = typ
	}
	resourceTypes := make(map[string]*ResourceType, len(p.types.resources))
	for token, typ := range p.types.resources {
		resourceTypes[token] = typ
	}

	// NOTE: these writes are very much not concurrency-safe. There is a data race on each write to a slice-typed field
	// because slices are multi-word values. Unfortunately, fixing this is rather involved. The simplest solution--
	// returning a copy of p.types.pkg--breaks package membership tests that use pointer equality (e.g. if the result
	// of Snapshot() is in a variable named `pkg`, `pkg.Resources[0].Package == pkg` will evaluate to `false`). It is
	// likely that we will need to make a breaking change in order to fix this.
	//
	// There is also a race between the call to ImportLanguages and readers of the Language property on the various
	// types.
	pkg := p.types.pkg
	pkg.Config = config
	pkg.Types = typeList
	pkg.Provider = provider
	pkg.Resources = resources
	pkg.Functions = functions
	pkg.resourceTable = resourceDefs
	pkg.functionTable = functionDefs
	pkg.typeTable = typeDefs
	pkg.resourceTypeTable = resourceTypes
	if err := pkg.ImportLanguages(p.languages); err != nil {
		return nil, err
	}

	return pkg, nil
}

type partialPackageTypes struct {
	*PartialPackage
}

func (p partialPackageTypes) Range() TypesIter {
	return &partialPackageTypesIter{
		types: p,
		iter:  reflect.ValueOf(p.spec.Types).MapRange(),
	}
}

func (p partialPackageTypes) Get(token string) (Type, bool, error) {
	p.m.Lock()
	defer p.m.Unlock()

	typ, diags, err := p.types.bindTypeDef(token, SchemaValidationOptions{})
	if err != nil {
		return nil, false, err
	}
	if diags.HasErrors() {
		return nil, false, diags
	}
	return typ, typ != nil, nil
}

type partialPackageTypesIter struct {
	types partialPackageTypes
	iter  *reflect.MapIter
}

func (i *partialPackageTypesIter) Token() string {
	return i.iter.Key().String()
}

func (i *partialPackageTypesIter) Type() (Type, error) {
	typ, _, err := i.types.Get(i.Token())
	return typ, err
}

func (i *partialPackageTypesIter) Next() bool {
	return i.iter.Next()
}

type partialPackageResources struct {
	*PartialPackage
}

func (p partialPackageResources) Range() ResourcesIter {
	return &partialPackageResourcesIter{
		resources: p,
		iter:      reflect.ValueOf(p.spec.Resources).MapRange(),
	}
}

func (p partialPackageResources) Get(token string) (*Resource, bool, error) {
	p.m.Lock()
	defer p.m.Unlock()

	res, diags, err := p.types.bindResourceDef(token, SchemaValidationOptions{})
	if err != nil {
		return nil, false, err
	}
	if diags.HasErrors() {
		return nil, false, diags
	}
	return res, res != nil, nil
}

func (p partialPackageResources) GetType(token string) (*ResourceType, bool, error) {
	p.m.Lock()
	defer p.m.Unlock()

	typ, diags, err := p.types.bindResourceTypeDef(token, SchemaValidationOptions{})
	if err != nil {
		return nil, false, err
	}
	if diags.HasErrors() {
		return nil, false, diags
	}
	return typ, typ != nil, nil
}

type partialPackageResourcesIter struct {
	resources partialPackageResources
	iter      *reflect.MapIter
}

func (i *partialPackageResourcesIter) Token() string {
	return i.iter.Key().String()
}

func (i *partialPackageResourcesIter) Resource() (*Resource, error) {
	res, _, err := i.resources.Get(i.Token())
	return res, err
}

func (i *partialPackageResourcesIter) Next() bool {
	return i.iter.Next()
}

type partialPackageFunctions struct {
	*PartialPackage
}

func (p partialPackageFunctions) Range() FunctionsIter {
	return &partialPackageFunctionsIter{
		functions: p,
		iter:      reflect.ValueOf(p.spec.Functions).MapRange(),
	}
}

func (p partialPackageFunctions) Get(token string) (*Function, bool, error) {
	p.m.Lock()
	defer p.m.Unlock()

	fn, diags, err := p.types.bindFunctionDef(token, SchemaValidationOptions{})
	if err != nil {
		return nil, false, err
	}
	if diags.HasErrors() {
		return nil, false, diags
	}
	return fn, fn != nil, nil
}

type partialPackageFunctionsIter struct {
	functions partialPackageFunctions
	iter      *reflect.MapIter
}

func (i *partialPackageFunctionsIter) Token() string {
	return i.iter.Key().String()
}

func (i *partialPackageFunctionsIter) Function() (*Function, error) {
	fn, _, err := i.functions.Get(i.Token())
	return fn, err
}

func (i *partialPackageFunctionsIter) Next() bool {
	return i.iter.Next()
}

// parseJSONPropertyValue parses a JSON value from the given RawMessage. If the RawMessage is empty, parsing is skipped
// and the function returns nil.
func parseJSONPropertyValue(raw json.RawMessage, dest interface{}) error {
	if len(raw) == 0 {
		return nil
	}
	_, err := json.Parse([]byte(raw), dest, json.ZeroCopy)
	return err
}
