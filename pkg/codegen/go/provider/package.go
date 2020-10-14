package provider

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"golang.org/x/tools/go/packages"
)

type typeSet map[string]interface{}

func (s typeSet) set(t *types.Named, v interface{}) {
	s[t.Obj().Id()] = v
}

func (s typeSet) add(t *types.Named) {
	s.set(t, nil)
}

func (s typeSet) get(t *types.Named) interface{} {
	v := s[t.Obj().Id()]
	return v
}

func (s typeSet) has(t *types.Named) bool {
	_, ok := s[t.Obj().Id()]
	return ok
}

func (s typeSet) delete(t *types.Named) {
	delete(s, t.Obj().Id())
}

type pulumiSDK struct {
	types *types.Package

	context           types.Type
	resourceOption    types.Type
	componentResource types.Type
	input             types.Type
	output            types.Type
}

func (sdk *pulumiSDK) hasOutputType(typ *types.Named) (*types.Named, bool) {
	if sdk == nil {
		return nil, false
	}

	if types.ConvertibleTo(typ, sdk.output) {
		return typ, true
	}
	if !types.ConvertibleTo(typ, sdk.input) {
		return nil, false
	}

	toOutput, ok := getMethod(typ, fmt.Sprintf("To%vOutput", strings.TrimSuffix(typ.Obj().Name(), "Input")))
	if !ok {
		return nil, false
	}
	signature := toOutput.Type().(*types.Signature)

	typ, ok = signature.Results().At(0).Type().(*types.Named)
	if !ok || !types.ConvertibleTo(typ, sdk.output) {
		return nil, false
	}
	return typ, true
}

type providerSDK struct {
	types *types.Package

	id            types.Type
	context       types.Type
	createOptions types.Type
	updateOptions types.Type
	readOptions   types.Type
	deleteOptions types.Type
	callOptions   types.Type
}

type pulumiType struct {
	token  string
	doc    *ast.CommentGroup
	syntax *ast.TypeSpec
	typ    *types.Named
}

type pulumiResource struct {
	token       string
	isComponent bool
	doc         *ast.CommentGroup
	syntax      *ast.TypeSpec
	typ         types.Type // either a *types.Pointer or the invalid type
	args        types.Type
}

type pulumiFunction struct {
	token  string
	syntax *ast.FuncDecl
	args   types.Type
	result types.Type
}

type pulumiModule struct {
	name string

	pulumiPackage *pulumiPackage
	goPackage     *packages.Package
	pulumiSDK     *pulumiSDK
	providerSDK   *providerSDK

	paramsTypes    typeSet
	componentTypes typeSet

	types        []*pulumiType
	resources    []*pulumiResource
	constructors []*pulumiFunction
	functions    []*pulumiFunction
}

func (m *pulumiModule) errorf(node ast.Node, message string, args ...interface{}) *hcl.Diagnostic {
	return newError(m.goPackage.Fset, node, fmt.Sprintf(message, args...))
}

func (m *pulumiModule) warningf(node ast.Node, message string, args ...interface{}) *hcl.Diagnostic {
	return newWarning(m.goPackage.Fset, node, fmt.Sprintf(message, args...))
}

type packageSet map[*types.Package]struct{}

func newPackageSet(packages ...*packages.Package) packageSet {
	s := packageSet{}
	for _, p := range packages {
		s.add(p.Types)
	}
	return s
}

func (s packageSet) add(p *types.Package) {
	s[p] = struct{}{}
}

func (s packageSet) has(p *types.Package) bool {
	_, ok := s[p]
	return ok
}

type pulumiPackage struct {
	name            string
	rootPackagePath string

	providerModule *pulumiModule
	provider       *pulumiResource

	modules []*pulumiModule

	roots       packageSet
	typeClosure typeSet
	outputTypes typeSet
}
