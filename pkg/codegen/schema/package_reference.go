package schema

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/segmentio/encoding/json"
)

type PackageReference interface {
	Name() string
	Version() *semver.Version

	Types() PackageTypes
	Config() ([]*Property, error)
	Provider() (*Resource, error)
	Resources() PackageResources
	Functions() PackageFunctions

	TokenToModule(token string) string

	Definition() (*Package, error)
}

type PackageTypes interface {
	Range() TypesIter
	Get(token string) (Type, bool, error)
}

type TypesIter interface {
	Token() string
	Type() (Type, error)
	Next() bool
}

type PackageResources interface {
	Range() ResourcesIter
	Get(token string) (*Resource, bool, error)
	GetType(token string) (Type, bool, error)
}

type ResourcesIter interface {
	Token() string
	Resource() (*Resource, error)
	Next() bool
}

type PackageFunctions interface {
	Range() FunctionsIter
	Get(token string) (*Function, bool, error)
}

type FunctionsIter interface {
	Token() string
	Function() (*Function, error)
	Next() bool
}

type packageDefRef struct {
	pkg *Package
}

func (p packageDefRef) Name() string {
	return p.pkg.Name
}

func (p packageDefRef) Version() *semver.Version {
	return p.pkg.Version
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

func (p packageDefResources) GetType(token string) (Type, bool, error) {
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

type PartialPackage struct {
	spec      *PartialPackageSpec
	languages map[string]Language
	types     *types

	config []*Property

	def *Package
}

func (p *PartialPackage) Name() string {
	if p.def != nil {
		return p.def.Name
	}
	return p.types.pkg.Name
}

func (p *PartialPackage) Version() *semver.Version {
	if p.def != nil {
		return p.def.Version
	}
	return p.types.pkg.Version
}

func (p *PartialPackage) Types() PackageTypes {
	if p.def != nil {
		return packageDefTypes{p.def}
	}
	return partialPackageTypes{p}
}

func (p *PartialPackage) Config() ([]*Property, error) {
	if p.def != nil {
		return p.def.Config, nil
	}

	if p.config != nil {
		return p.config, nil
	}

	var spec ConfigSpec
	if err := parseJSONPropertyValue(p.spec.Config, &spec); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	config, diags, err := bindConfig(spec, p.types)
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
	if p.def != nil {
		return p.def.Provider, nil
	}

	provider, diags, err := p.types.bindResourceDef("pulumi:providers:" + p.spec.Name)
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, err
	}
	return provider, nil
}

func (p *PartialPackage) Resources() PackageResources {
	if p.def != nil {
		return packageDefResources{p.def}
	}
	return partialPackageResources{p}
}

func (p *PartialPackage) Functions() PackageFunctions {
	if p.def != nil {
		return packageDefFunctions{p.def}
	}
	return partialPackageFunctions{p}
}

func (p *PartialPackage) TokenToModule(token string) string {
	if p.def != nil {
		return p.def.TokenToModule(token)
	}
	return p.types.pkg.TokenToModule(token)
}

func (p *PartialPackage) Definition() (*Package, error) {
	if p.def != nil {
		return p.def, nil
	}

	config, err := p.Config()
	if err != nil {
		return nil, err
	}

	var diags hcl.Diagnostics
	provider, resources, resourceDiags, err := p.types.finishResources(sortedKeys(p.spec.Resources))
	if err != nil {
		return nil, err
	}
	diags = diags.Extend(resourceDiags)

	functions, functionDiags, err := p.types.finishFunctions(sortedKeys(p.spec.Functions))
	if err != nil {
		return nil, err
	}
	diags = diags.Extend(functionDiags)

	typeList, typeDiags, err := p.types.finishTypes(sortedKeys(p.spec.Types))
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
	if p.def != nil {
		return p.def, nil
	}

	config := p.config
	provider := p.types.resourceDefs["pulumi:providers:"+p.spec.Name]

	resources := make([]*Resource, 0, len(p.types.resourceDefs))
	for _, res := range p.types.resourceDefs {
		resources = append(resources, res)
	}
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Token < resources[j].Token
	})

	functions := make([]*Function, 0, len(p.types.functionDefs))
	for _, fn := range p.types.functionDefs {
		functions = append(functions, fn)
	}
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Token < functions[j].Token
	})

	typeList, diags, err := p.types.finishTypes(nil)
	contract.Assert(err == nil)
	contract.Assert(len(diags) == 0)

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
	typ, diags, err := p.types.bindTypeDef(token)
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
	res, diags, err := p.types.bindResourceDef(token)
	if err != nil {
		return nil, false, err
	}
	if diags.HasErrors() {
		return nil, false, diags
	}
	return res, res != nil, nil
}

func (p partialPackageResources) GetType(token string) (Type, bool, error) {
	typ, diags, err := p.types.bindResourceTypeDef(token)
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
	fn, diags, err := p.types.bindFunctionDef(token)
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
