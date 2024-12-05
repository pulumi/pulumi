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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
//nolint:lll, goconst
package gen

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"go/format"
	"io"
	"net/url"
	"os"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A signifier that the module is external, and will never match.
//
// This token is always an invalid module since ':' is not allowed within modules.
const ExternalModuleSig = ":always-external:"

const (
	GenericsSettingNone         = "none"
	GenericsSettingSideBySide   = "side-by-side"
	GenericsSettingGenericsOnly = "generics-only"
)

type typeDetails struct {
	// Note: if any of {ptr,array,map}Input are set, input and the corresponding output field must also be set. The
	// mark* functions ensure that these invariants hold.
	input      bool
	ptrInput   bool
	arrayInput bool
	mapInput   bool

	// Note: if any of {ptr,array,map}Output are set, output must also be set. The mark* functions ensure that these
	// invariants hold.
	output      bool
	ptrOutput   bool
	arrayOutput bool
	mapOutput   bool
}

func (d *typeDetails) hasOutputs() bool {
	return d.output || d.ptrOutput || d.arrayOutput || d.mapOutput
}

func (d *typeDetails) mark(input, output bool) {
	d.input = d.input || input
	d.output = d.output || input || output
}

func (d *typeDetails) markPtr(input, output bool) {
	d.mark(input, output)
	d.ptrInput = d.ptrInput || input
	d.ptrOutput = d.ptrOutput || input || output
}

func (d *typeDetails) markArray(input, output bool) {
	d.mark(input, output)
	d.arrayInput = d.arrayInput || input
	d.arrayOutput = d.arrayOutput || input || output
}

func (d *typeDetails) markMap(input, output bool) {
	d.mark(input, output)
	d.mapInput = d.mapInput || input
	d.mapOutput = d.mapOutput || input || output
}

// Title converts the input string to a title case
// where only the initial letter is upper-cased.
// It also removes $-prefix if any.
func Title(s string) string {
	if s == "" {
		return ""
	}
	if s[0] == '$' {
		return Title(s[1:])
	}
	s = cgstrings.UppercaseFirst(s)
	s = cgstrings.Unhyphenate(s)
	return s
}

func tokenToPackage(pkg schema.PackageReference, overrides map[string]string, tok string) string {
	mod := pkg.TokenToModule(tok)
	if override, ok := overrides[mod]; ok {
		mod = override
	}
	return strings.ToLower(mod)
}

// A threadsafe cache for sharing between invocations of GenerateProgram.
type Cache struct {
	externalPackages map[*schema.Package]map[string]*pkgContext
	m                *sync.Mutex
}

var globalCache = NewCache()

func NewCache() *Cache {
	return &Cache{
		externalPackages: map[*schema.Package]map[string]*pkgContext{},
		m:                new(sync.Mutex),
	}
}

func (c *Cache) lookupContextMap(pkg *schema.Package) (map[string]*pkgContext, bool) {
	c.m.Lock()
	defer c.m.Unlock()
	m, ok := c.externalPackages[pkg]
	return m, ok
}

func (c *Cache) setContextMap(pkg *schema.Package, m map[string]*pkgContext) {
	c.m.Lock()
	defer c.m.Unlock()
	c.externalPackages[pkg] = m
}

type pkgContext struct {
	pkg             schema.PackageReference
	mod             string
	importBasePath  string
	rootPackageName string
	typeDetails     map[schema.Type]*typeDetails
	enums           []*schema.EnumType
	types           []*schema.ObjectType
	resources       []*schema.Resource
	functions       []*schema.Function

	// schemaNames tracks the names of types/resources as specified in the schema
	schemaNames codegen.StringSet
	names       codegen.StringSet
	renamed     map[string]string

	// A mapping between external packages and their bound contents.
	externalPackages *Cache

	// duplicateTokens tracks tokens that exist for both types and resources
	duplicateTokens map[string]bool
	functionNames   map[*schema.Function]string
	tool            string
	packages        map[string]*pkgContext

	// Name overrides set in GoPackageInfo
	modToPkg         map[string]string // Module name -> package name
	pkgImportAliases map[string]string // Package name -> import alias
	// the name used for the internal module, defaults to "internal" if not set by the schema
	internalModuleName string

	// Determines whether to make single-return-value methods return an output struct or the value
	liftSingleValueMethodReturns bool

	// Determines if we should emit type registration code
	disableInputTypeRegistrations bool

	// Determines if we should emit object defaults code
	disableObjectDefaults bool
}

func (pkg *pkgContext) detailsForType(t schema.Type) *typeDetails {
	if obj, ok := t.(*schema.ObjectType); ok && obj.IsInputShape() {
		t = obj.PlainShape
	}

	details, ok := pkg.typeDetails[t]
	if !ok {
		details = &typeDetails{}
		pkg.typeDetails[t] = details
	}
	return details
}

func (pkg *pkgContext) tokenToPackage(tok string) string {
	return tokenToPackage(pkg.pkg, pkg.modToPkg, tok)
}

func (pkg *pkgContext) tokenToType(tok string) string {
	// token := pkg : module : member
	// module := path/to/module

	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "tok: %s", tok)
	contract.Assertf(pkg != nil, "pkg is nil. token %s", tok)
	contract.Assertf(pkg.pkg != nil, "pkg.pkg is nil. token %s", tok)

	mod, name := pkg.tokenToPackage(tok), components[2]

	name = Title(name)
	if modPkg, ok := pkg.packages[mod]; ok {
		newName, renamed := modPkg.renamed[name]
		if renamed {
			name = newName
		} else if modPkg.duplicateTokens[strings.ToLower(tok)] {
			// maintain support for duplicate tokens for types and resources in Kubernetes
			name += "Type"
		}
	}

	if mod == pkg.mod {
		return name
	}
	if mod == "" {
		var err error
		mod, err = packageRoot(pkg.pkg)
		contract.AssertNoErrorf(err, "Unable to determine package root")
	}

	var importPath string
	if alias, hasAlias := pkg.pkgImportAliases[path.Join(pkg.importBasePath, mod)]; hasAlias {
		importPath = alias
	} else {
		importPath = mod[strings.IndexRune(mod, '/')+1:]
		importPath = strings.ReplaceAll(importPath, "-", "")
	}

	return strings.ReplaceAll(importPath+"."+name, "-provider", "")
}

// Resolve a enum type to its name.
func (pkg *pkgContext) resolveEnumType(t *schema.EnumType) string {
	if !pkg.isExternalReference(t) {
		return pkg.tokenToEnum(t.Token)
	}

	extPkgCtx, _ := pkg.contextForExternalReference(t)
	enumType := extPkgCtx.typeString(t)
	return enumType
}

func (pkg *pkgContext) tokenToEnum(tok string) string {
	// token := pkg : module : member
	// module := path/to/module

	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "Token must have 3 components, got %d", len(components))
	contract.Assertf(pkg != nil, "pkg is nil. token %s", tok)
	contract.Assertf(pkg.pkg != nil, "pkg.pkg is nil. token %s", tok)

	mod, name := pkg.tokenToPackage(tok), components[2]

	name = Title(name)

	if modPkg, ok := pkg.packages[mod]; ok {
		newName, renamed := modPkg.renamed[name]
		if renamed {
			name = newName
		} else if modPkg.duplicateTokens[tok] {
			// If the package containing the enum's token already has a resource or type with the
			// same name, add an `Enum` suffix.
			name += "Enum"
		}
	}

	if mod == pkg.mod {
		return name
	}
	if mod == "" {
		mod = components[0]
	}

	var importPath string
	if alias, hasAlias := pkg.pkgImportAliases[path.Join(pkg.importBasePath, mod)]; hasAlias {
		importPath = alias
	} else {
		importPath = strings.ReplaceAll(mod, "/", "")
	}

	return importPath + "." + name
}

func (pkg *pkgContext) tokenToResource(tok string) string {
	// token := pkg : module : member
	// module := path/to/module

	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "Token must have 3 components, got %d", len(components))
	if pkg == nil {
		panic(fmt.Errorf("pkg is nil. token %s", tok))
	}
	if pkg.pkg == nil {
		panic(fmt.Errorf("pkg.pkg is nil. token %s", tok))
	}

	// Is it a provider resource?
	if components[0] == "pulumi" && components[1] == "providers" {
		return components[2] + ".Provider"
	}

	mod, name := pkg.tokenToPackage(tok), components[2]

	name = Title(name)

	if mod == pkg.mod {
		return name
	}
	if mod == "" {
		mod = components[0]
	}

	var importPath string
	if alias, hasAlias := pkg.pkgImportAliases[path.Join(pkg.importBasePath, mod)]; hasAlias {
		importPath = alias
	} else {
		importPath = strings.ReplaceAll(mod, "/", "")
	}

	return importPath + "." + name
}

func tokenToModule(tok string) string {
	// token := pkg : module : member
	// module := path/to/module

	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "Token must have 3 components, got %d", len(components))
	return components[1]
}

func tokenToName(tok string) string {
	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "Token must have 3 components, got %d", len(components))
	return Title(components[2])
}

// disambiguatedResourceName gets the name of a resource as it should appear in source, resolving conflicts in the process.
func disambiguatedResourceName(r *schema.Resource, pkg *pkgContext) string {
	name := rawResourceName(r)
	if renamed, ok := pkg.renamed[name]; ok {
		name = renamed
	}
	return name
}

// rawResourceName produces raw resource name translated from schema type token without resolving conflicts or dupes.
func rawResourceName(r *schema.Resource) string {
	if r.IsProvider {
		return "Provider"
	}
	return tokenToName(r.Token)
}

// If `nil` is a valid value of type `t`.
func isNilType(t schema.Type) bool {
	switch t := t.(type) {
	case *schema.OptionalType, *schema.ArrayType, *schema.MapType, *schema.ResourceType, *schema.InputType:
		return true
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return isNilType(t.UnderlyingType)
		}
	case *schema.UnionType:
		// If the union is actually a relaxed enum type, use the underlying
		// type for the enum instead
		for _, e := range t.ElementTypes {
			if typ, ok := e.(*schema.EnumType); ok {
				return isNilType(typ.ElementType)
			}
		}
	default:
		switch t {
		case schema.ArchiveType, schema.AssetType, schema.JSONType, schema.AnyType:
			return true
		}
	}
	return false
}

func (pkg *pkgContext) inputType(t schema.Type) (result string) {
	switch t := codegen.SimplifyInputUnion(t).(type) {
	case *schema.OptionalType:
		return pkg.typeString(t)
	case *schema.InputType:
		return pkg.inputType(t.ElementType)
	case *schema.EnumType:
		// Since enum type is itself an input
		return pkg.resolveEnumType(t) + "Input"
	case *schema.ArrayType:
		en := pkg.inputType(t.ElementType)
		return strings.TrimSuffix(en, "Input") + "ArrayInput"
	case *schema.MapType:
		en := pkg.inputType(t.ElementType)
		return strings.TrimSuffix(en, "Input") + "MapInput"
	case *schema.ObjectType:
		if t.IsInputShape() {
			t = t.PlainShape
		}
		return pkg.resolveObjectType(t) + "Input"
	case *schema.ResourceType:
		return pkg.resolveResourceType(t) + "Input"
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return pkg.inputType(t.UnderlyingType)
		}
		return pkg.tokenToType(t.Token) + "Input"
	case *schema.UnionType:
		// If the union is actually a relaxed enum type, use the underlying
		// type for the input instead
		for _, e := range t.ElementTypes {
			if typ, ok := e.(*schema.EnumType); ok {
				return pkg.inputType(typ.ElementType)
			}
		}
		// TODO(pdg): union types
		return "pulumi.Input"
	default:
		switch t {
		case schema.BoolType:
			return "pulumi.BoolInput"
		case schema.IntType:
			return "pulumi.IntInput"
		case schema.NumberType:
			return "pulumi.Float64Input"
		case schema.StringType:
			return "pulumi.StringInput"
		case schema.ArchiveType:
			return "pulumi.ArchiveInput"
		case schema.AssetType:
			return "pulumi.AssetOrArchiveInput"
		case schema.JSONType:
			fallthrough
		case schema.AnyType:
			return "pulumi.Input"
		}
	}

	panic(fmt.Errorf("unexpected type %T", t))
}

func (pkg *pkgContext) genericInputTypeImpl(t schema.Type) string {
	switch t := codegen.SimplifyInputUnion(t).(type) {
	case *schema.OptionalType:
		return pkg.genericInputTypeImpl(t.ElementType)
	case *schema.InputType:
		return pkg.genericInputTypeImpl(t.ElementType)
	case *schema.EnumType:
		return pkg.resolveEnumType(t)
	case *schema.ArrayType:
		elementType := pkg.genericInputTypeImpl(t.ElementType)
		return "[]" + elementType
	case *schema.MapType:
		elementType := pkg.genericInputTypeImpl(t.ElementType)
		return "map[string]" + elementType
	case *schema.ObjectType:
		elementType := pkg.resolveObjectType(t)
		return "*" + elementType
	case *schema.UnionType:
		// If the union is actually a relaxed enum type, use the underlying
		// type for the input instead
		for _, e := range t.ElementTypes {
			if typ, ok := e.(*schema.EnumType); ok {
				return pkg.genericInputTypeImpl(typ.ElementType)
			}
		}

		return "any"
	default:
		elementType, _ := pkg.genericElementType(t)
		return elementType
	}
}

func isArrayType(t schema.Type) bool {
	switch t.(type) {
	case *schema.ArrayType:
		return true
	default:
		return false
	}
}

func isMapType(t schema.Type) bool {
	switch t.(type) {
	case *schema.MapType:
		return true
	default:
		return false
	}
}

func isOptionalType(t schema.Type) bool {
	switch t.(type) {
	case *schema.OptionalType:
		return true
	default:
		return false
	}
}

func reduceInputType(t schema.Type) schema.Type {
	switch t := t.(type) {
	case *schema.InputType:
		return reduceInputType(t.ElementType)
	default:
		return t
	}
}

func (pkg *pkgContext) genericTypeNeedsPointer(t schema.Type) bool {
	return isOptionalType(reduceInputType(t)) && !isArrayType(codegen.UnwrapType(t)) && !isMapType(codegen.UnwrapType(t))
}

func (pkg *pkgContext) genericInputType(t schema.Type) string {
	optionalPointer := ""
	if pkg.genericTypeNeedsPointer(t) {
		optionalPointer = "*"
	}

	inputType := pkg.genericInputTypeImpl(t)
	if strings.HasPrefix(inputType, "*") {
		optionalPointer = ""
	}

	return fmt.Sprintf("pulumix.Input[%s%s]", optionalPointer, inputType)
}

func (pkg *pkgContext) plainGenericInputType(t schema.Type) string {
	optionalPointer := ""
	if pkg.genericTypeNeedsPointer(t) {
		optionalPointer = "*"
	}

	inputType := pkg.genericInputTypeImpl(t)
	if strings.HasPrefix(inputType, "*") {
		optionalPointer = ""
	}

	return fmt.Sprintf("%s%s", optionalPointer, inputType)
}

func (pkg *pkgContext) argsTypeImpl(t schema.Type) (result string) {
	switch t := codegen.SimplifyInputUnion(t).(type) {
	case *schema.OptionalType:
		return pkg.typeStringImpl(t, true)
	case *schema.InputType:
		return pkg.argsTypeImpl(t.ElementType)
	case *schema.EnumType:
		// Since enum type is itself an input
		return pkg.resolveEnumType(t)
	case *schema.ArrayType:
		en := pkg.argsTypeImpl(t.ElementType)
		if en == "pulumi.Any" {
			en = strings.TrimSuffix(en, "Any")
		}
		return strings.TrimSuffix(en, "Args") + "Array"
	case *schema.MapType:
		en := pkg.argsTypeImpl(t.ElementType)
		if en == "pulumi.Any" {
			en = strings.TrimSuffix(en, "Any")
		}
		return strings.TrimSuffix(en, "Args") + "Map"
	case *schema.ObjectType:
		return pkg.resolveObjectType(t)
	case *schema.ResourceType:
		return pkg.resolveResourceType(t)
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return pkg.argsTypeImpl(t.UnderlyingType)
		}
		return pkg.tokenToType(t.Token)
	case *schema.UnionType:
		// If the union is actually a relaxed enum type, use the underlying
		// type for the input instead
		for _, e := range t.ElementTypes {
			if typ, ok := e.(*schema.EnumType); ok {
				return pkg.argsTypeImpl(typ.ElementType)
			}
		}
		return "pulumi.Any"
	default:
		switch t {
		case schema.BoolType:
			return "pulumi.Bool"
		case schema.IntType:
			return "pulumi.Int"
		case schema.NumberType:
			return "pulumi.Float64"
		case schema.StringType:
			return "pulumi.String"
		case schema.ArchiveType:
			return "pulumi.Archive"
		case schema.AssetType:
			return "pulumi.AssetOrArchive"
		case schema.JSONType:
			fallthrough
		case schema.AnyType:
			return "pulumi.Any"
		}
	}

	panic(fmt.Errorf("unexpected type %T", t))
}

func (pkg *pkgContext) argsType(t schema.Type) string {
	return pkg.typeStringImpl(t, true)
}

func (pkg *pkgContext) typeStringImpl(t schema.Type, argsType bool) string {
	switch t := t.(type) {
	case *schema.OptionalType:
		if input, isInputType := t.ElementType.(*schema.InputType); isInputType {
			elem := pkg.inputType(input.ElementType)
			if isNilType(input.ElementType) || elem == "pulumi.Input" {
				return elem
			}
			if pkg.isExternalReference(input.ElementType) {
				_, details := pkg.contextForExternalReference(input.ElementType)

				switch input.ElementType.(type) {
				case *schema.ObjectType:
					if !details.ptrInput {
						return "*" + elem
					}
				case *schema.EnumType:
					if !(details.ptrInput || details.input) {
						return "*" + elem
					}
				}
			}
			if argsType {
				return elem + "Ptr"
			}
			return strings.TrimSuffix(elem, "Input") + "PtrInput"
		}

		elementType := pkg.typeStringImpl(t.ElementType, argsType)
		if isNilType(t.ElementType) || elementType == "interface{}" {
			return elementType
		}
		return "*" + elementType
	case *schema.InputType:
		if argsType {
			return pkg.argsTypeImpl(t.ElementType)
		}
		return pkg.inputType(t.ElementType)
	case *schema.EnumType:
		return pkg.resolveEnumType(t)
	case *schema.ArrayType:
		typ := "[]"
		return typ + pkg.typeStringImpl(t.ElementType, argsType)
	case *schema.MapType:
		typ := "map[string]"
		return typ + pkg.typeStringImpl(t.ElementType, argsType)
	case *schema.ObjectType:
		return pkg.resolveObjectType(t)
	case *schema.ResourceType:
		return "*" + pkg.resolveResourceType(t)
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return pkg.typeStringImpl(t.UnderlyingType, argsType)
		}
		return pkg.tokenToType(t.Token)
	case *schema.UnionType:
		// If the union is actually a relaxed enum type, use the underlying
		// type for the enum instead
		for _, e := range t.ElementTypes {
			if typ, ok := e.(*schema.EnumType); ok {
				return pkg.typeStringImpl(typ.ElementType, argsType)
			}
		}
		// TODO(pdg): union types
		return "interface{}"
	default:
		switch t {
		case schema.BoolType:
			return "bool"
		case schema.IntType:
			return "int"
		case schema.NumberType:
			return "float64"
		case schema.StringType:
			return "string"
		case schema.ArchiveType:
			return "pulumi.Archive"
		case schema.AssetType:
			return "pulumi.AssetOrArchive"
		case schema.JSONType:
			fallthrough
		case schema.AnyType:
			return "interface{}"
		}
	}

	panic(fmt.Errorf("unexpected type %T", t))
}

func (pkg *pkgContext) typeString(t schema.Type) string {
	s := pkg.typeStringImpl(t, false)
	if s == "pulumi." {
		return "pulumi.Any"
	}
	return s
}

func (pkg *pkgContext) isExternalReference(t schema.Type) bool {
	isExternal, _, _ := pkg.isExternalReferenceWithPackage(t)
	return isExternal
}

// Return if `t` is external to `pkg`. If so, the associated foreign schema.Package is returned.
func (pkg *pkgContext) isExternalReferenceWithPackage(t schema.Type) (
	isExternal bool, extPkg schema.PackageReference, token string,
) {
	switch typ := t.(type) {
	case *schema.ObjectType:
		isExternal = typ.PackageReference != nil && !codegen.PkgEquals(typ.PackageReference, pkg.pkg)
		if isExternal {
			extPkg = typ.PackageReference
			token = typ.Token
		}
		return
	case *schema.ResourceType:
		isExternal = typ.Resource != nil && pkg.pkg != nil && !codegen.PkgEquals(typ.Resource.PackageReference, pkg.pkg)
		if isExternal {
			extPkg = typ.Resource.PackageReference
			token = typ.Token
		}
		return
	case *schema.EnumType:
		isExternal = pkg.pkg != nil && !codegen.PkgEquals(typ.PackageReference, pkg.pkg)
		if isExternal {
			extPkg = typ.PackageReference
			token = typ.Token
		}
		return
	}
	return
}

// resolveResourceType resolves resource references in properties while
// taking into account potential external resources. Returned type is
// always marked as required. Caller should check if the property is
// optional and convert the type to a pointer if necessary.
func (pkg *pkgContext) resolveResourceType(t *schema.ResourceType) string {
	if !pkg.isExternalReference(t) {
		return pkg.tokenToResource(t.Token)
	}
	extPkgCtx, _ := pkg.contextForExternalReference(t)
	resType := extPkgCtx.tokenToResource(t.Token)
	if !strings.Contains(resType, ".") {
		resType = fmt.Sprintf("%s.%s", extPkgCtx.pkg.Name(), resType)
	}
	return resType
}

// resolveObjectType resolves resource references in properties while
// taking into account potential external resources. Returned type is
// always marked as required. Caller should check if the property is
// optional and convert the type to a pointer if necessary.
func (pkg *pkgContext) resolveObjectType(t *schema.ObjectType) string {
	isExternal, _, _ := pkg.isExternalReferenceWithPackage(t)

	if !isExternal {
		name := pkg.tokenToType(t.Token)
		if t.IsInputShape() {
			return name + "Args"
		}
		return name
	}
	extPkg, _ := pkg.contextForExternalReference(t)
	return extPkg.typeString(t)
}

func (pkg *pkgContext) contextForExternalReference(t schema.Type) (*pkgContext, typeDetails) {
	isExternal, extPkg, token := pkg.isExternalReferenceWithPackage(t)
	contract.Assertf(isExternal, "Expected external reference for %v", t)

	var goInfo GoPackageInfo
	extDef, err := extPkg.Definition()
	contract.AssertNoErrorf(err, "Could not load definition for %q", extPkg.Name())
	contract.AssertNoErrorf(extDef.ImportLanguages(map[string]schema.Language{"go": Importer}),
		"Failed to import languages")
	if info, ok := extDef.Language["go"].(GoPackageInfo); ok {
		goInfo = info
	} else {
		goInfo.ImportBasePath = extractImportBasePath(extPkg)
	}

	pkgImportAliases := goInfo.PackageImportAliases

	// Ensure that any package import aliases we have specified locally take precedence over those
	// specified in the remote package.
	def, err := pkg.pkg.Definition()
	contract.AssertNoErrorf(err, "Could not load definition for %q", pkg.pkg.Name())
	if ourPkgGoInfoI, has := def.Language["go"]; has {
		ourPkgGoInfo := ourPkgGoInfoI.(GoPackageInfo)
		if len(ourPkgGoInfo.PackageImportAliases) > 0 {
			pkgImportAliases = make(map[string]string)
			// Copy the external import aliases.
			for k, v := range goInfo.PackageImportAliases {
				pkgImportAliases[k] = v
			}
			// Copy the local import aliases, overwriting any external aliases.
			for k, v := range ourPkgGoInfo.PackageImportAliases {
				pkgImportAliases[k] = v
			}
		}
	}

	var maps map[string]*pkgContext

	if extMap, ok := pkg.externalPackages.lookupContextMap(extDef); ok {
		maps = extMap
	} else {
		maps, err = generatePackageContextMap(pkg.tool, extPkg, goInfo, pkg.externalPackages)
		contract.AssertNoErrorf(err, "Could not generate package context map")
		pkg.externalPackages.setContextMap(extDef, maps)
	}
	extPkgCtx := maps[""]
	extPkgCtx.pkgImportAliases = pkgImportAliases
	extPkgCtx.externalPackages = pkg.externalPackages
	mod := tokenToPackage(extPkg, goInfo.ModuleToPackage, token)
	extPkgCtx.mod = ExternalModuleSig

	return extPkgCtx, *maps[mod].detailsForType(t)
}

// outputTypeImpl does the meat of the generation of output type names from schema types. This function should only be
// called with a fully-resolved type (e.g. the result of codegen.ResolvedType). Instead of calling this function, you
// probably want to call pkgContext.outputType, which ensures that its argument is resolved.
func (pkg *pkgContext) outputTypeImpl(t schema.Type) string {
	switch t := t.(type) {
	case *schema.OptionalType:
		elem := pkg.outputTypeImpl(t.ElementType)
		if isNilType(t.ElementType) || elem == "pulumi.AnyOutput" {
			return elem
		}
		if pkg.isExternalReference(t.ElementType) {
			_, details := pkg.contextForExternalReference(t.ElementType)
			switch t.ElementType.(type) {
			case *schema.ObjectType:
				if !details.ptrOutput {
					return "*" + elem
				}
			case *schema.EnumType:
				if !(details.ptrOutput || details.output) {
					return "*" + elem
				}
			}
		}
		return strings.TrimSuffix(elem, "Output") + "PtrOutput"
	case *schema.EnumType:
		return pkg.resolveEnumType(t) + "Output"
	case *schema.ArrayType:
		en := strings.TrimSuffix(pkg.outputTypeImpl(t.ElementType), "Output")
		if en == "pulumi.Any" {
			return "pulumi.ArrayOutput"
		}
		return en + "ArrayOutput"
	case *schema.MapType:
		en := strings.TrimSuffix(pkg.outputTypeImpl(t.ElementType), "Output")
		if en == "pulumi.Any" {
			return "pulumi.MapOutput"
		}
		return en + "MapOutput"
	case *schema.ObjectType:
		return pkg.resolveObjectType(t) + "Output"
	case *schema.ResourceType:
		return pkg.resolveResourceType(t) + "Output"
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return pkg.outputTypeImpl(t.UnderlyingType)
		}
		return pkg.tokenToType(t.Token) + "Output"
	case *schema.UnionType:
		// If the union is actually a relaxed enum type, use the underlying
		// type for the output instead
		for _, e := range t.ElementTypes {
			if typ, ok := e.(*schema.EnumType); ok {
				return pkg.outputTypeImpl(typ.ElementType)
			}
		}
		// TODO(pdg): union types
		return "pulumi.AnyOutput"
	case *schema.InputType:
		// We can't make output types for input types. We instead strip the input and try again.
		return pkg.outputTypeImpl(t.ElementType)
	default:
		switch t {
		case schema.BoolType:
			return "pulumi.BoolOutput"
		case schema.IntType:
			return "pulumi.IntOutput"
		case schema.NumberType:
			return "pulumi.Float64Output"
		case schema.StringType:
			return "pulumi.StringOutput"
		case schema.ArchiveType:
			return "pulumi.ArchiveOutput"
		case schema.AssetType:
			return "pulumi.AssetOrArchiveOutput"
		case schema.JSONType:
			fallthrough
		case schema.AnyType:
			return "pulumi.AnyOutput"
		}
	}

	panic(fmt.Errorf("unexpected type %T", t))
}

func isAssetOrArchive(t schema.Type) bool {
	switch t {
	case schema.ArchiveType, schema.AssetType:
		return true
	default:
		return false
	}
}

func (pkg *pkgContext) genericElementType(schemaType schema.Type) (string, bool) {
	switch schemaType {
	case schema.StringType:
		return "string", true
	case schema.BoolType:
		return "bool", true
	case schema.IntType:
		return "int", true
	case schema.NumberType:
		return "float64", true
	case schema.ArchiveType:
		return "pulumi.Archive", true
	case schema.AssetType:
		return "pulumi.AssetOrArchive", true
	default:
		switch schemaType := schemaType.(type) {
		case *schema.ObjectType:
			return pkg.resolveObjectType(schemaType), false
		case *schema.EnumType:
			return pkg.resolveEnumType(schemaType), true
		case *schema.ResourceType:
			return pkg.resolveResourceType(schemaType), false
		case *schema.TokenType:
			return pkg.genericElementType(schemaType.UnderlyingType)
		case *schema.ArrayType:
			elementType, _ := pkg.genericElementType(schemaType.ElementType)
			return "[]" + elementType, false
		case *schema.MapType:
			elementType, _ := pkg.genericElementType(schemaType.ElementType)
			return "map[string]" + elementType, false
		case *schema.UnionType:
			for _, e := range schemaType.ElementTypes {
				if enumType, ok := e.(*schema.EnumType); ok {
					return pkg.genericElementType(enumType.ElementType)
				}
			}
			return "any", true
		default:
			return "any", true
		}
	}
}

// genericOutputTypeImpl is similar to outputTypeImpl, but it generates the generic variant.
// for example instead of pulumi.StringOutput, it generates pulumix.Output[string]
func (pkg *pkgContext) genericOutputTypeImpl(t schema.Type) string {
	switch t := t.(type) {
	case *schema.OptionalType:
		elementType, isPrimitive := pkg.genericElementType(t.ElementType)
		if elementType == "any" {
			return fmt.Sprintf("pulumix.Output[%s]", elementType)
		}

		if isPrimitive {
			// for example OptionalType{StringType} becomes pulumix.Output[*string]
			return fmt.Sprintf("pulumix.Output[*%s]", elementType)
		}

		if pkg.isExternalReference(t.ElementType) {
			_, details := pkg.contextForExternalReference(t.ElementType)
			switch t.ElementType.(type) {
			case *schema.ObjectType:
				if !details.ptrOutput {
					return "*" + elementType
				}
			case *schema.EnumType:
				if !(details.ptrOutput || details.output) {
					return "*" + elementType
				}
			}
		}

		return pkg.genericOutputTypeImpl(t.ElementType)
	case *schema.EnumType:
		elementType, _ := pkg.genericElementType(t)
		return fmt.Sprintf("pulumix.Output[%s]", elementType)
	case *schema.ArrayType:
		elementType, isPrimitive := pkg.genericElementType(t.ElementType)
		if isPrimitive {
			return fmt.Sprintf("pulumix.ArrayOutput[%s]", elementType)
		}

		// for non-primitive types such as objects and resources
		// use GArrayOutput[Type, TypeOutput]
		return fmt.Sprintf("pulumix.GArrayOutput[%s, %sOutput]", elementType, elementType)
	case *schema.MapType:
		elementType, isPrimitive := pkg.genericElementType(t.ElementType)
		if isPrimitive {
			return fmt.Sprintf("pulumix.MapOutput[%s]", elementType)
		}

		// for non-primitive types such as objects and resources
		// use GMapOutput[Type, TypeOutput]
		return fmt.Sprintf("pulumix.GMapOutput[%s, %sOutput]", elementType, elementType)
	case *schema.ObjectType:
		objectTypeName, _ := pkg.genericElementType(t)
		return fmt.Sprintf("pulumix.GPtrOutput[%s, %sOutput]", objectTypeName, objectTypeName)
	case *schema.ResourceType:
		resourceTypeName, _ := pkg.genericElementType(t)
		// element type of a ResourceOutput is Resource
		return fmt.Sprintf("pulumix.GPtrOutput[%s, %sOutput]", resourceTypeName, resourceTypeName)
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return pkg.genericOutputType(t.UnderlyingType)
		}

		tokenType := pkg.tokenToType(t.Token)
		return fmt.Sprintf("pulumix.Output[%s]", tokenType)
	case *schema.UnionType:
		// If the union is actually a relaxed enum type, use the underlying
		// type for the enum instead
		for _, e := range t.ElementTypes {
			if typ, ok := e.(*schema.EnumType); ok {
				return pkg.genericOutputTypeImpl(typ.ElementType)
			}
		}
		// TODO(pdg): union types
		return "pulumix.Output[interface{}]"
	case *schema.InputType:
		// We can't make output types for input types. We instead strip the input and try again.
		return pkg.genericOutputTypeImpl(t.ElementType)
	default:
		elementType, _ := pkg.genericElementType(t)
		return fmt.Sprintf("pulumix.Output[%s]", elementType)
	}
}

// outputType returns a reference to the Go output type that corresponds to the given schema type. For example, given
// a schema.String, outputType returns "pulumi.String", and given a *schema.ObjectType with the token pkg:mod:Name,
// outputType returns "mod.NameOutput" or "NameOutput", depending on whether or not the object type lives in a
// different module than the one associated with the receiver.
func (pkg *pkgContext) outputType(t schema.Type) string {
	return pkg.outputTypeImpl(codegen.ResolvedType(t))
}

// genericOutputType returns a reference to the Go output type that corresponds to the given schema type.
// For example, given a schema.StringType, genericOutputType returns "pulumix.Output[string]",
// and given a *schema.ObjectType with the token pkg:mod:Name,
// outputType returns "mod.NameOutput" or "NameOutput", depending on whether the object type lives in a
// different module than the one associated with the receiver.
func (pkg *pkgContext) genericOutputType(t schema.Type) string {
	return pkg.genericOutputTypeImpl(codegen.ResolvedType(t))
}

// toOutputMethod returns the name of the "ToXXXOutput" method for the given schema type. For example, given a
// schema.String, toOutputMethod returns "ToStringOutput", and given a *schema.ObjectType with the token pkg:mod:Name,
// outputType returns "ToNameOutput".
func (pkg *pkgContext) toOutputMethod(t schema.Type) string {
	outputTypeName := pkg.outputType(t)
	if i := strings.LastIndexByte(outputTypeName, '.'); i != -1 {
		outputTypeName = outputTypeName[i+1:]
	}
	return "To" + outputTypeName
}

// printComment filters examples for the Go languages and prepends double forward slash to each line in the given
// comment. If indent is true, each line is indented with tab character. It returns the number of lines in the
// resulting comment. It guarantees that each line is terminated with newline character.
func printComment(w io.Writer, comment string, indent bool) int {
	comment = codegen.FilterExamples(comment, "go")

	lines := strings.Split(comment, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, l := range lines {
		if indent {
			fmt.Fprintf(w, "\t")
		}
		if l == "" {
			fmt.Fprintf(w, "//\n")
		} else {
			fmt.Fprintf(w, "// %s\n", l)
		}
	}
	return len(lines)
}

func printCommentWithDeprecationMessage(w io.Writer, comment, deprecationMessage string, indent bool) {
	lines := printComment(w, comment, indent)
	if deprecationMessage != "" {
		if lines > 0 {
			fmt.Fprintf(w, "//\n")
		}
		printComment(w, "Deprecated: "+deprecationMessage, indent)
	}
}

func (pkg *pkgContext) genInputInterface(w io.Writer, name string) {
	printComment(w, pkg.getInputUsage(name), false)
	fmt.Fprintf(w, "type %sInput interface {\n", name)
	fmt.Fprintf(w, "\tpulumi.Input\n\n")
	fmt.Fprintf(w, "\tTo%sOutput() %sOutput\n", Title(name), name)
	fmt.Fprintf(w, "\tTo%sOutputWithContext(context.Context) %sOutput\n", Title(name), name)
	fmt.Fprintf(w, "}\n\n")
}

func (pkg *pkgContext) genEnumInputInterface(w io.Writer, name string, enumType *schema.EnumType) {
	enumCases := []string{}
	for _, enumCase := range enumType.Elements {
		if enumCase.DeprecationMessage != "" {
			// skip deprecated enum cases
			continue
		}
		enumCases = append(enumCases, "\t\t"+enumCase.Name)
	}

	enumUsage := strings.Join([]string{
		fmt.Sprintf("%sInput is an input type that accepts values of the %s enum", name, name),
		fmt.Sprintf("A concrete instance of `%sInput` can be one of the following:", name),
		"",
		strings.Join(enumCases, "\n"),
		" ",
	}, "\n")

	printComment(w, enumUsage, false)
	fmt.Fprintf(w, "type %sInput interface {\n", name)
	fmt.Fprintf(w, "\tpulumi.Input\n\n")
	fmt.Fprintf(w, "\tTo%sOutput() %sOutput\n", Title(name), name)
	fmt.Fprintf(w, "\tTo%sOutputWithContext(context.Context) %sOutput\n", Title(name), name)
	fmt.Fprintf(w, "}\n\n")
}

func (pkg *pkgContext) getUsageForNestedType(name, baseTypeName string) string {
	const defaultExampleFormat = "%sArgs{...}"
	example := fmt.Sprintf(defaultExampleFormat, baseTypeName)

	trimmer := func(typeName string) string {
		if strings.HasSuffix(typeName, "Array") {
			return typeName[:strings.LastIndex(typeName, "Array")]
		}
		if strings.HasSuffix(typeName, "Map") {
			return typeName[:strings.LastIndex(typeName, "Map")]
		}
		return typeName
	}

	// If not a nested collection type, use the default example format
	if trimmer(name) == name {
		return example
	}

	if strings.HasSuffix(name, "Map") {
		if pkg.schemaNames.Has(baseTypeName) {
			return fmt.Sprintf("%s{ \"key\": %s }", name, example)
		}
		return fmt.Sprintf("%s{ \"key\": %s }", name, pkg.getUsageForNestedType(baseTypeName, trimmer(baseTypeName)))
	}

	if strings.HasSuffix(name, "Array") {
		if pkg.schemaNames.Has(baseTypeName) {
			return fmt.Sprintf("%s{ %s }", name, example)
		}
		return fmt.Sprintf("%s{ %s }", name, pkg.getUsageForNestedType(baseTypeName, trimmer(baseTypeName)))
	}
	return example
}

func (pkg *pkgContext) getInputUsage(name string) string {
	if strings.HasSuffix(name, "Array") {
		baseTypeName := name[:strings.LastIndex(name, "Array")]
		return strings.Join([]string{
			fmt.Sprintf("%sInput is an input type that accepts %s and %sOutput values.", name, name, name),
			fmt.Sprintf("You can construct a concrete instance of `%sInput` via:", name),
			"",
			"\t\t " + pkg.getUsageForNestedType(name, baseTypeName),
			" ",
		}, "\n")
	}

	if strings.HasSuffix(name, "Map") {
		baseTypeName := name[:strings.LastIndex(name, "Map")]
		return strings.Join([]string{
			fmt.Sprintf("%sInput is an input type that accepts %s and %sOutput values.", name, name, name),
			fmt.Sprintf("You can construct a concrete instance of `%sInput` via:", name),
			"",
			"\t\t " + pkg.getUsageForNestedType(name, baseTypeName),
			" ",
		}, "\n")
	}

	if strings.HasSuffix(name, "Ptr") {
		baseTypeName := name[:strings.LastIndex(name, "Ptr")]
		return strings.Join([]string{
			fmt.Sprintf("%sInput is an input type that accepts %sArgs, %s and %sOutput values.", name, baseTypeName, name, name),
			fmt.Sprintf("You can construct a concrete instance of `%sInput` via:", name),
			"",
			fmt.Sprintf("\t\t %sArgs{...}", baseTypeName),
			"",
			" or:",
			"",
			"\t\t nil",
			" ",
		}, "\n")
	}

	return strings.Join([]string{
		fmt.Sprintf("%sInput is an input type that accepts %sArgs and %sOutput values.", name, name, name),
		fmt.Sprintf("You can construct a concrete instance of `%sInput` via:", name),
		"",
		fmt.Sprintf("\t\t %sArgs{...}", name),
		" ",
	}, "\n")
}

type genInputImplementationArgs struct {
	name              string
	receiverType      string
	elementType       string
	ptrMethods        bool
	toOutputMethods   bool
	usingGenericTypes bool
	goPackageinfo     GoPackageInfo
}

func (pkg *pkgContext) genInputImplementation(
	w io.Writer,
	name string,
	receiverType string,
	elementType string,
	ptrMethods bool,
	usingGenericTypes bool,
) {
	genInputImplementationWithArgs(w, genInputImplementationArgs{
		name:              name,
		receiverType:      receiverType,
		elementType:       elementType,
		ptrMethods:        ptrMethods,
		toOutputMethods:   true,
		usingGenericTypes: usingGenericTypes,
		goPackageinfo:     goPackageInfo(pkg.pkg),
	})
}

func genInputImplementationWithArgs(w io.Writer, genArgs genInputImplementationArgs) {
	name := genArgs.name
	receiverType := genArgs.receiverType
	elementType := genArgs.elementType

	fmt.Fprintf(w, "func (%s) ElementType() reflect.Type {\n", receiverType)
	fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s)(nil)).Elem()\n", elementType)
	fmt.Fprintf(w, "}\n\n")

	var hasToOutput bool
	if genArgs.toOutputMethods {
		fmt.Fprintf(w, "func (i %s) To%sOutput() %sOutput {\n", receiverType, Title(name), name)
		fmt.Fprintf(w, "\treturn i.To%sOutputWithContext(context.Background())\n", Title(name))
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (i %s) To%sOutputWithContext(ctx context.Context) %sOutput {\n", receiverType, Title(name), name)
		fmt.Fprintf(w, "\treturn pulumi.ToOutputWithContext(ctx, i).(%sOutput)\n", name)
		fmt.Fprintf(w, "}\n\n")

		if !genArgs.usingGenericTypes {
			// Generate 'ToOuput(context.Context) pulumix.Output[T]' method
			// to satisfy pulumix.Input[T].
			if genArgs.goPackageinfo.Generics == GenericsSettingSideBySide {
				fmt.Fprintf(w, "func (i %s) ToOutput(ctx context.Context) pulumix.Output[%s] {\n", receiverType, elementType)
				fmt.Fprintf(w, "\treturn pulumix.Output[%s]{\n", elementType)
				fmt.Fprintf(w, "\t\tOutputState: i.To%sOutputWithContext(ctx).OutputState,\n", Title(name))
				fmt.Fprintf(w, "\t}\n")
				fmt.Fprintf(w, "}\n\n")
				hasToOutput = true
			}
		} else {
			// Generate 'ToOuput(context.Context) pulumix.Output[T]' method which lifts the receiver type *T
			// to satisfy pulumix.Input[*T].
			fmt.Fprintf(w, "func (i *%s) ToOutput(ctx context.Context) pulumix.Output[*%s] {\n", receiverType, receiverType)
			fmt.Fprint(w, "\treturn pulumix.Val(i)\n")
			fmt.Fprint(w, "}\n\n")
		}
	}

	if genArgs.ptrMethods && !genArgs.usingGenericTypes {
		fmt.Fprintf(w, "func (i %s) To%sPtrOutput() %sPtrOutput {\n", receiverType, Title(name), name)
		fmt.Fprintf(w, "\treturn i.To%sPtrOutputWithContext(context.Background())\n", Title(name))
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (i %s) To%sPtrOutputWithContext(ctx context.Context) %sPtrOutput {\n", receiverType, Title(name), name)
		if strings.HasSuffix(receiverType, "Args") {
			fmt.Fprintf(w, "\treturn pulumi.ToOutputWithContext(ctx, i).(%[1]sOutput).To%[1]sPtrOutputWithContext(ctx)\n", name)
		} else {
			fmt.Fprintf(w, "\treturn pulumi.ToOutputWithContext(ctx, i).(%sPtrOutput)\n", name)
		}
		fmt.Fprintf(w, "}\n\n")

		if !hasToOutput {
			// Generate 'ToOuput(context.Context) pulumix.Output[*T]' method
			// to satisfy pulumix.Input[*T].
			if genArgs.goPackageinfo.Generics == GenericsSettingSideBySide {
				fmt.Fprintf(w, "func (i %s) ToOutput(ctx context.Context) pulumix.Output[*%s] {\n", receiverType, elementType)
				fmt.Fprintf(w, "\treturn pulumix.Output[*%s]{\n", elementType)
				fmt.Fprintf(w, "\t\tOutputState: i.To%sPtrOutputWithContext(ctx).OutputState,\n", Title(name))
				fmt.Fprintf(w, "\t}\n")
				fmt.Fprintf(w, "}\n\n")
			}
		}
	}
}

func (pkg *pkgContext) genOutputType(w io.Writer, baseName, elementType string, ptrMethods, usingGenericTypes bool) {
	fmt.Fprintf(w, "type %sOutput struct { *pulumi.OutputState }\n\n", baseName)

	fmt.Fprintf(w, "func (%sOutput) ElementType() reflect.Type {\n", baseName)
	fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s)(nil)).Elem()\n", elementType)
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sOutput() %[1]sOutput {\n", baseName, Title(baseName))
	fmt.Fprintf(w, "\treturn o\n")
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sOutputWithContext(ctx context.Context) %[1]sOutput {\n", baseName, Title(baseName))
	fmt.Fprintf(w, "\treturn o\n")
	fmt.Fprintf(w, "}\n\n")

	if ptrMethods && !usingGenericTypes {
		fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutput() %[1]sPtrOutput {\n", baseName, Title(baseName))
		fmt.Fprintf(w, "\treturn o.To%sPtrOutputWithContext(context.Background())\n", Title(baseName))
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutputWithContext(ctx context.Context) %[1]sPtrOutput {\n", baseName, Title(baseName))
		fmt.Fprintf(w, "\treturn o.ApplyTWithContext(ctx, func(_ context.Context, v %[1]s) *%[1]s {\n", elementType)
		fmt.Fprintf(w, "\t\treturn &v\n")
		fmt.Fprintf(w, "\t}).(%sPtrOutput)\n", baseName)
		fmt.Fprintf(w, "}\n\n")
	}

	// Generate 'ToOuput(context.Context) pulumix.Output[T]' method
	// to satisfy pulumix.Input[T].
	goPackageInfo := goPackageInfo(pkg.pkg)
	if goPackageInfo.Generics == GenericsSettingSideBySide || goPackageInfo.Generics == GenericsSettingGenericsOnly {
		fmt.Fprintf(w, "func (o %sOutput) ToOutput(ctx context.Context) pulumix.Output[%s] {\n", baseName, elementType)
		fmt.Fprintf(w, "\treturn pulumix.Output[%s]{\n", elementType)
		fmt.Fprintf(w, "\t\tOutputState: o.OutputState,\n")
		fmt.Fprintf(w, "\t}\n")
		fmt.Fprintf(w, "}\n\n")
	}
}

func (pkg *pkgContext) genArrayOutput(w io.Writer, baseName, elementType string) {
	pkg.genOutputType(w, baseName+"Array", "[]"+elementType, false, false)

	fmt.Fprintf(w, "func (o %[1]sArrayOutput) Index(i pulumi.IntInput) %[1]sOutput {\n", baseName)
	fmt.Fprintf(w, "\treturn pulumi.All(o, i).ApplyT(func (vs []interface{}) %s {\n", elementType)
	fmt.Fprintf(w, "\t\treturn vs[0].([]%s)[vs[1].(int)]\n", elementType)
	fmt.Fprintf(w, "\t}).(%sOutput)\n", baseName)
	fmt.Fprintf(w, "}\n\n")
}

func (pkg *pkgContext) genMapOutput(w io.Writer, baseName, elementType string) {
	pkg.genOutputType(w, baseName+"Map", "map[string]"+elementType, false, false)

	fmt.Fprintf(w, "func (o %[1]sMapOutput) MapIndex(k pulumi.StringInput) %[1]sOutput {\n", baseName)
	fmt.Fprintf(w, "\treturn pulumi.All(o, k).ApplyT(func (vs []interface{}) %s{\n", elementType)
	fmt.Fprintf(w, "\t\treturn vs[0].(map[string]%s)[vs[1].(string)]\n", elementType)
	fmt.Fprintf(w, "\t}).(%sOutput)\n", baseName)
	fmt.Fprintf(w, "}\n\n")
}

func (pkg *pkgContext) genPtrOutput(w io.Writer, baseName, elementType string) {
	pkg.genOutputType(w, baseName+"Ptr", "*"+elementType, false, false)

	fmt.Fprintf(w, "func (o %[1]sPtrOutput) Elem() %[1]sOutput {\n", baseName)
	fmt.Fprintf(w, "\treturn o.ApplyT(func(v *%[1]s) %[1]s {\n", baseName)
	fmt.Fprint(w, "\t\tif v != nil {\n")
	fmt.Fprintf(w, "\t\t\treturn *v\n")
	fmt.Fprint(w, "\t\t}\n")
	fmt.Fprintf(w, "\t\tvar ret %s\n", baseName)
	fmt.Fprint(w, "\t\treturn ret\n")
	fmt.Fprintf(w, "\t}).(%sOutput)\n", baseName)
	fmt.Fprint(w, "}\n\n")
}

func (pkg *pkgContext) genEnum(w io.Writer, enumType *schema.EnumType, usingGenericTypes bool) error {
	name := pkg.tokenToEnum(enumType.Token)

	mod := pkg.tokenToPackage(enumType.Token)
	modPkg, ok := pkg.packages[mod]
	contract.Assertf(ok, "Context for module %q not found", mod)

	printCommentWithDeprecationMessage(w, enumType.Comment, "", false)

	elementArgsType := pkg.argsTypeImpl(enumType.ElementType)
	elementGoType := pkg.typeString(enumType.ElementType)
	asFuncName := strings.TrimPrefix(elementArgsType, "pulumi.")

	fmt.Fprintf(w, "type %s %s\n\n", name, elementGoType)

	fmt.Fprintln(w, "const (")
	for _, e := range enumType.Elements {
		printCommentWithDeprecationMessage(w, e.Comment, e.DeprecationMessage, true)

		elementName := e.Name
		if e.Name == "" {
			elementName = fmt.Sprintf("%v", e.Value)
		}
		enumName, err := makeSafeEnumName(elementName, name)
		if err != nil {
			return err
		}
		e.Name = enumName
		contract.Assertf(!modPkg.names.Has(e.Name), "Name collision for enum constant: %s for %s",
			e.Name, enumType.Token)

		//nolint:exhaustive // Default case handles the rest of the values
		switch reflect.TypeOf(e.Value).Kind() {
		case reflect.String:
			fmt.Fprintf(w, "%s = %s(%q)\n", e.Name, name, e.Value)
		default:
			fmt.Fprintf(w, "%s = %s(%v)\n", e.Name, name, e.Value)
		}
	}
	fmt.Fprintln(w, ")")

	if usingGenericTypes {
		// no need to generate the rest of the enum output/input types
		return nil
	}

	details := pkg.detailsForType(enumType)
	if details.input || details.ptrInput {
		inputType := pkg.inputType(enumType)
		pkg.genEnumInputFuncs(w, name, enumType, elementArgsType, inputType, asFuncName)
	}

	if details.output || details.ptrOutput {
		pkg.genEnumOutputTypes(w, name, elementArgsType, elementGoType, asFuncName)
	}
	if details.input || details.ptrInput {
		pkg.genEnumInputTypes(w, name, enumType, elementGoType)
	}

	// Generate the array input.
	if details.arrayInput {
		pkg.genInputInterface(w, name+"Array")

		fmt.Fprintf(w, "type %[1]sArray []%[1]s\n\n", name)

		pkg.genInputImplementation(w, name+"Array", name+"Array", "[]"+name, false, usingGenericTypes)
	}

	// Generate the map input.
	if details.mapInput {
		pkg.genInputInterface(w, name+"Map")

		fmt.Fprintf(w, "type %[1]sMap map[string]%[1]s\n\n", name)

		pkg.genInputImplementation(w, name+"Map", name+"Map", "map[string]"+name, false, usingGenericTypes)
	}

	// Generate the array output
	if details.arrayOutput {
		pkg.genArrayOutput(w, name, name)
	}

	// Generate the map output.
	if details.mapOutput {
		pkg.genMapOutput(w, name, name)
	}

	return nil
}

func (pkg *pkgContext) genEnumOutputTypes(w io.Writer, name, elementArgsType, elementGoType, asFuncName string) {
	pkg.genOutputType(w, name, name, true, false)

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sOutput() %[3]sOutput {\n", name, asFuncName, elementArgsType)
	fmt.Fprintf(w, "return o.To%sOutputWithContext(context.Background())\n", asFuncName)
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sOutputWithContext(ctx context.Context) %[3]sOutput {\n", name, asFuncName, elementArgsType)
	fmt.Fprintf(w, "return o.ApplyTWithContext(ctx, func(_ context.Context, e %s) %s {\n", name, elementGoType)
	fmt.Fprintf(w, "return %s(e)\n", elementGoType)
	fmt.Fprintf(w, "}).(%sOutput)\n", elementArgsType)
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutput() %[3]sPtrOutput {\n", name, asFuncName, elementArgsType)
	fmt.Fprintf(w, "return o.To%sPtrOutputWithContext(context.Background())\n", asFuncName)
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutputWithContext(ctx context.Context) %[3]sPtrOutput {\n", name, asFuncName, elementArgsType)
	fmt.Fprintf(w, "return o.ApplyTWithContext(ctx, func(_ context.Context, e %s) *%s {\n", name, elementGoType)
	fmt.Fprintf(w, "v := %s(e)\n", elementGoType)
	fmt.Fprintf(w, "return &v\n")
	fmt.Fprintf(w, "}).(%sPtrOutput)\n", elementArgsType)
	fmt.Fprint(w, "}\n\n")

	pkg.genPtrOutput(w, name, name)

	fmt.Fprintf(w, "func (o %[1]sPtrOutput) To%[2]sPtrOutput() %[3]sPtrOutput {\n", name, asFuncName, elementArgsType)
	fmt.Fprintf(w, "return o.To%sPtrOutputWithContext(context.Background())\n", asFuncName)
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sPtrOutput) To%[2]sPtrOutputWithContext(ctx context.Context) %[3]sPtrOutput {\n", name, asFuncName, elementArgsType)
	fmt.Fprintf(w, "return o.ApplyTWithContext(ctx, func(_ context.Context, e *%s) *%s {\n", name, elementGoType)
	fmt.Fprintf(w, "if e == nil {\n")
	fmt.Fprintf(w, "return nil\n")
	fmt.Fprintf(w, "}\n")
	fmt.Fprintf(w, "v := %s(*e)\n", elementGoType)
	fmt.Fprintf(w, "return &v\n")
	fmt.Fprintf(w, "}).(%sPtrOutput)\n", elementArgsType)
	fmt.Fprint(w, "}\n\n")
}

func (pkg *pkgContext) genEnumInputTypes(w io.Writer, name string, enumType *schema.EnumType, elementGoType string) {
	pkg.genEnumInputInterface(w, name, enumType)
	goPkgInfo := goPackageInfo(pkg.pkg)
	typeName := cgstrings.Camel(name)
	fmt.Fprintf(w, "var %sPtrType = reflect.TypeOf((**%s)(nil)).Elem()\n", typeName, name)
	fmt.Fprintln(w)

	fmt.Fprintf(w, "type %sPtrInput interface {\n", name)
	fmt.Fprint(w, "pulumi.Input\n\n")
	fmt.Fprintf(w, "To%[1]sPtrOutput() %[1]sPtrOutput\n", name)
	fmt.Fprintf(w, "To%[1]sPtrOutputWithContext(context.Context) %[1]sPtrOutput\n", name)
	fmt.Fprintf(w, "}\n")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "type %sPtr %s\n", typeName, elementGoType)
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func %[1]sPtr(v %[2]s) %[1]sPtrInput {\n", name, elementGoType)
	fmt.Fprintf(w, "return (*%sPtr)(&v)\n", typeName)
	fmt.Fprintf(w, "}\n")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (*%sPtr) ElementType() reflect.Type {\n", typeName)
	fmt.Fprintf(w, "return %sPtrType\n", typeName)
	fmt.Fprintf(w, "}\n")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (in *%[1]sPtr) To%[2]sPtrOutput() %[2]sPtrOutput {\n", typeName, name)
	fmt.Fprintf(w, "return pulumi.ToOutput(in).(%sPtrOutput)\n", name)
	fmt.Fprintf(w, "}\n")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (in *%[1]sPtr) To%[2]sPtrOutputWithContext(ctx context.Context) %[2]sPtrOutput {\n", cgstrings.Camel(name), name)
	fmt.Fprintf(w, "return pulumi.ToOutputWithContext(ctx, in).(%sPtrOutput)\n", name)
	fmt.Fprintf(w, "}\n")
	fmt.Fprintln(w)

	if goPkgInfo.Generics != GenericsSettingNone {
		// ToOutput implementation for pulumix.Input.
		fmt.Fprintf(w, "func (in *%sPtr) ToOutput(ctx context.Context) pulumix.Output[*%s] {\n", typeName, name)
		fmt.Fprintf(w, "\treturn pulumix.Output[*%s]{\n", name)
		fmt.Fprintf(w, "\t\tOutputState: in.To%sPtrOutputWithContext(ctx).OutputState,\n", name)
		fmt.Fprintf(w, "\t}\n")
		fmt.Fprintf(w, "}\n\n")
	}
}

func (pkg *pkgContext) genEnumInputFuncs(w io.Writer, typeName string, enum *schema.EnumType, elementArgsType, inputType, asFuncName string) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "func (%s) ElementType() reflect.Type {\n", typeName)
	fmt.Fprintf(w, "return reflect.TypeOf((*%s)(nil)).Elem()\n", typeName)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (e %[1]s) To%[1]sOutput() %[1]sOutput {\n", typeName)
	fmt.Fprintf(w, "return pulumi.ToOutput(e).(%sOutput)\n", typeName)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (e %[1]s) To%[1]sOutputWithContext(ctx context.Context) %[1]sOutput {\n", typeName)
	fmt.Fprintf(w, "return pulumi.ToOutputWithContext(ctx, e).(%sOutput)\n", typeName)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (e %[1]s) To%[1]sPtrOutput() %[1]sPtrOutput {\n", typeName)
	fmt.Fprintf(w, "return e.To%sPtrOutputWithContext(context.Background())\n", typeName)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (e %[1]s) To%[1]sPtrOutputWithContext(ctx context.Context) %[1]sPtrOutput {\n", typeName)
	fmt.Fprintf(w, "return %[1]s(e).To%[1]sOutputWithContext(ctx).To%[1]sPtrOutputWithContext(ctx)\n", typeName)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (e %[1]s) To%[2]sOutput() %[3]sOutput {\n", typeName, asFuncName, elementArgsType)
	fmt.Fprintf(w, "return pulumi.ToOutput(%[1]s(e)).(%[1]sOutput)\n", elementArgsType)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (e %[1]s) To%[2]sOutputWithContext(ctx context.Context) %[3]sOutput {\n", typeName, asFuncName, elementArgsType)
	fmt.Fprintf(w, "return pulumi.ToOutputWithContext(ctx, %[1]s(e)).(%[1]sOutput)\n", elementArgsType)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (e %[1]s) To%[2]sPtrOutput() %[3]sPtrOutput {\n", typeName, asFuncName, elementArgsType)
	fmt.Fprintf(w, "return %s(e).To%sPtrOutputWithContext(context.Background())\n", elementArgsType, asFuncName)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (e %[1]s) To%[2]sPtrOutputWithContext(ctx context.Context) %[3]sPtrOutput {\n", typeName, asFuncName, elementArgsType)
	fmt.Fprintf(w, "return %[1]s(e).To%[2]sOutputWithContext(ctx).To%[2]sPtrOutputWithContext(ctx)\n", elementArgsType, asFuncName)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)
}

func (pkg *pkgContext) assignProperty(
	w io.Writer,
	p *schema.Property,
	object,
	value string,
	indirectAssign bool,
	useGenericTypes bool,
) {
	t := strings.TrimSuffix(pkg.typeString(p.Type), "Input")
	if useGenericTypes {
		t = "pulumix.Val"
		if isOptionalType(reduceInputType(p.Type)) {
			t = "pulumix.Ptr"
		}
	}
	switch codegen.UnwrapType(p.Type).(type) {
	case *schema.EnumType:
		if !useGenericTypes {
			t = ""
		}
	}

	if codegen.IsNOptionalInput(p.Type) {
		if t != "" {
			value = fmt.Sprintf("%s(%s)", t, value)
		}
		fmt.Fprintf(w, "\t%s.%s = %s\n", object, pkg.fieldName(nil, p), value)
	} else if indirectAssign {
		tmpName := cgstrings.Camel(p.Name) + "_"
		fmt.Fprintf(w, "%s := %s\n", tmpName, value)
		fmt.Fprintf(w, "%s.%s = &%s\n", object, pkg.fieldName(nil, p), tmpName)
	} else {
		fmt.Fprintf(w, "%s.%s = %s\n", object, pkg.fieldName(nil, p), value)
	}
}

func (pkg *pkgContext) fieldName(r *schema.Resource, field *schema.Property) string {
	contract.Assertf(field != nil, "Field must not be nil")
	return fieldName(pkg, r, field)
}

func (pkg *pkgContext) genPlainType(w io.Writer, name, comment, deprecationMessage string,
	properties []*schema.Property,
) {
	printCommentWithDeprecationMessage(w, comment, deprecationMessage, false)
	fmt.Fprintf(w, "type %s struct {\n", name)
	for _, p := range properties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", pkg.fieldName(nil, p), pkg.typeString(codegen.ResolvedType(p.Type)), p.Name)
	}
	fmt.Fprintf(w, "}\n\n")
}

// genGenericPlainType is the same as genPlainType, but used for generic variant SDKs
// where it maintains optionalness of property types
func (pkg *pkgContext) genGenericPlainType(w io.Writer, name, comment, deprecationMessage string,
	properties []*schema.Property,
) {
	printCommentWithDeprecationMessage(w, comment, deprecationMessage, false)
	fmt.Fprintf(w, "type %s struct {\n", name)
	for _, p := range properties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", pkg.fieldName(nil, p), pkg.plainGenericInputType(p.Type), p.Name)
	}
	fmt.Fprintf(w, "}\n\n")
}

func (pkg *pkgContext) genObjectDefaultFunc(w io.Writer, name string,
	properties []*schema.Property,
	useGenericTypes bool,
) error {
	defaults := []*schema.Property{}
	for _, p := range properties {
		if p.DefaultValue != nil || codegen.IsProvideDefaultsFuncRequired(p.Type) {
			defaults = append(defaults, p)
		}
	}

	// There are no defaults, so we don't need to generate a defaults function.
	if len(defaults) == 0 {
		return nil
	}

	printComment(w, fmt.Sprintf("%s sets the appropriate defaults for %s", ProvideDefaultsMethodName, name), false)
	fmt.Fprintf(w, "func (val *%[1]s) %[2]s() *%[1]s {\n", name, ProvideDefaultsMethodName)
	fmt.Fprintf(w, "if val == nil {\n return nil\n}\n")
	fmt.Fprintf(w, "tmp := *val\n")
	for _, p := range defaults {
		if p.DefaultValue != nil {
			if isNilType(p.Type) {
				fmt.Fprintf(w, "if tmp.%s == nil {\n", pkg.fieldName(nil, p))
			} else {
				fmt.Fprintf(w, "if %s.IsZero(tmp.%s) {\n", pkg.internalModuleName, pkg.fieldName(nil, p))
			}
			err := pkg.setDefaultValue(w, p.DefaultValue, codegen.UnwrapType(p.Type), func(w io.Writer, dv string) error {
				pkg.assignProperty(w, p, "tmp", dv, !p.IsRequired(), useGenericTypes)
				return nil
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "}\n")
		} else if funcName := pkg.provideDefaultsFuncName(p.Type); funcName != "" {
			var member string
			if codegen.IsNOptionalInput(p.Type) {
				// f := fmt.Sprintf("func(v %[1]s) %[1]s { return *v.%[2]s() }", name, funcName)
				// member = fmt.Sprintf("tmp.%[1]s.ApplyT(%[2]s)", pkg.fieldName(nil, p), f)
			} else {
				member = fmt.Sprintf("tmp.%[1]s.%[2]s()", pkg.fieldName(nil, p), funcName)
				sigil := ""
				if p.IsRequired() {
					sigil = "*"
				}
				pkg.assignProperty(w, p, "tmp", sigil+member, false, useGenericTypes)
			}
			fmt.Fprintln(w)
		} else {
			panic(fmt.Sprintf("Property %s[%s] should not be in the default list", p.Name, p.Type.String()))
		}
	}

	fmt.Fprintf(w, "return &tmp\n}\n")
	return nil
}

// The name of the method used to instantiate defaults.
const ProvideDefaultsMethodName = "Defaults"

func (pkg *pkgContext) provideDefaultsFuncName(typ schema.Type) string {
	if !codegen.IsProvideDefaultsFuncRequired(typ) {
		return ""
	}
	return ProvideDefaultsMethodName
}

func (pkg *pkgContext) genInputTypes(
	w io.Writer,
	t *schema.ObjectType,
	details *typeDetails,
	usingGenericTypes bool,
) error {
	contract.Assertf(t.IsInputShape(), "Object type must have input shape")

	name := pkg.tokenToType(t.Token)

	// Generate the plain inputs.
	if details.input {
		if !usingGenericTypes {
			pkg.genInputInterface(w, name)
		}

		inputName := name + "Args"
		pkg.genInputArgsStruct(w, inputName, t, usingGenericTypes)
		if !pkg.disableObjectDefaults {
			if err := pkg.genObjectDefaultFunc(w, inputName, t.Properties, usingGenericTypes); err != nil {
				return err
			}
		}

		pkg.genInputImplementation(w, name, inputName, name, details.ptrInput, usingGenericTypes)
	}

	// Generate the pointer input.
	if details.ptrInput && !usingGenericTypes {
		pkg.genInputInterface(w, name+"Ptr")

		ptrTypeName := cgstrings.Camel(name) + "PtrType"

		fmt.Fprintf(w, "type %s %sArgs\n\n", ptrTypeName, name)

		fmt.Fprintf(w, "func %[1]sPtr(v *%[1]sArgs) %[1]sPtrInput {", name)
		fmt.Fprintf(w, "\treturn (*%s)(v)\n", ptrTypeName)
		fmt.Fprintf(w, "}\n\n")

		pkg.genInputImplementation(w, name+"Ptr", "*"+ptrTypeName, "*"+name, false, usingGenericTypes)
	}

	// Generate the array input.
	if details.arrayInput && !pkg.names.Has(name+"Array") && !usingGenericTypes {
		pkg.genInputInterface(w, name+"Array")

		fmt.Fprintf(w, "type %[1]sArray []%[1]sInput\n\n", name)

		pkg.genInputImplementation(w, name+"Array", name+"Array", "[]"+name, false, usingGenericTypes)
	}

	// Generate the map input.
	if details.mapInput && !pkg.names.Has(name+"Map") && !usingGenericTypes {
		pkg.genInputInterface(w, name+"Map")

		fmt.Fprintf(w, "type %[1]sMap map[string]%[1]sInput\n\n", name)

		pkg.genInputImplementation(w, name+"Map", name+"Map", "map[string]"+name, false, usingGenericTypes)
	}
	return nil
}

func (pkg *pkgContext) genInputArgsStruct(
	w io.Writer,
	typeName string,
	t *schema.ObjectType,
	useGenericTypes bool,
) {
	contract.Assertf(t.IsInputShape(), "Object type must have input shape")

	printComment(w, t.Comment, false)
	fmt.Fprintf(w, "type %s struct {\n", typeName)
	for _, p := range t.Properties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		inputType := pkg.typeString(p.Type)
		if useGenericTypes {
			if p.Plain {
				inputType = pkg.plainGenericInputType(p.Type)
			} else {
				inputType = pkg.genericInputType(p.Type)
			}
		}
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", pkg.fieldName(nil, p), inputType, p.Name)
	}
	fmt.Fprintf(w, "}\n\n")
}

type genOutputTypesArgs struct {
	t                 *schema.ObjectType
	usingGenericTypes bool

	// optional type name override
	name   string
	output bool
}

func (pkg *pkgContext) genOutputTypes(w io.Writer, genArgs genOutputTypesArgs) {
	t := genArgs.t
	details := pkg.detailsForType(t)

	contract.Assertf(!t.IsInputShape(), "Object type must not have input shape")

	name := genArgs.name
	if name == "" {
		name = pkg.tokenToType(t.Token)
	}

	if details.output || genArgs.output {
		printComment(w, t.Comment, false)
		pkg.genOutputType(w,
			name,                      /* baseName */
			name,                      /* elementType */
			details.ptrInput,          /* ptrMethods */
			genArgs.usingGenericTypes, /* usingGenericTypes */
		)

		for _, p := range t.Properties {
			printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, false)
			outputType, applyType := pkg.outputType(p.Type), pkg.typeString(p.Type)
			if genArgs.usingGenericTypes {
				outputType = pkg.genericOutputType(p.Type)
			}

			propName := pkg.fieldName(nil, p)
			switch strings.ToLower(p.Name) {
			case "elementtype", "issecret":
				propName = "Get" + propName
			}
			fmt.Fprintf(w, "func (o %sOutput) %s() %s {\n", name, propName, outputType)
			if !genArgs.usingGenericTypes {
				fmt.Fprintf(w, "\treturn o.ApplyT(func (v %s) %s { return v.%s }).(%s)\n",
					name, applyType, pkg.fieldName(nil, p), outputType)
			} else {
				needsCast := genericTypeNeedsExplicitCasting(outputType)
				if !needsCast {
					fmt.Fprintf(w, "\treturn pulumix.Apply[%s](o, func (v %s) %s { return v.%s })\n",
						name, name, pkg.plainGenericInputType(p.Type), pkg.fieldName(nil, p))
				} else {
					fmt.Fprintf(w, "\tvalue := pulumix.Apply[%s](o, func (v %s) %s { return v.%s })\n",
						name, name, pkg.plainGenericInputType(p.Type), pkg.fieldName(nil, p))
					fmt.Fprintf(w, "\treturn %s{OutputState: value.OutputState}\n", outputType)
				}
			}

			fmt.Fprintf(w, "}\n\n")
		}
	}

	if details.ptrOutput && !genArgs.usingGenericTypes {
		pkg.genPtrOutput(w, name, name)

		for _, p := range t.Properties {
			printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, false)
			optionalType := codegen.OptionalType(p)
			outputType, applyType := pkg.outputType(optionalType), pkg.typeString(optionalType)
			deref := ""
			// If the property was required, but the type it needs to return is an explicit pointer type, then we need
			// to dereference it, unless it is a resource type which should remain a pointer.
			_, isResourceType := p.Type.(*schema.ResourceType)
			if p.IsRequired() && applyType[0] == '*' && !isResourceType {
				deref = "&"
			}

			funcName := Title(p.Name)
			// Avoid conflicts with Output interface for lifted attributes.
			switch funcName {
			case "IsSecret", "ElementType":
				funcName = funcName + "Prop"
			}

			fmt.Fprintf(w, "func (o %sPtrOutput) %s() %s {\n", name, funcName, outputType)
			fmt.Fprintf(w, "\treturn o.ApplyT(func (v *%s) %s {\n", name, applyType)
			fmt.Fprintf(w, "\t\tif v == nil {\n")
			fmt.Fprintf(w, "\t\t\treturn nil\n")
			fmt.Fprintf(w, "\t\t}\n")
			fmt.Fprintf(w, "\t\treturn %sv.%s\n", deref, pkg.fieldName(nil, p))
			fmt.Fprintf(w, "\t}).(%s)\n", outputType)
			fmt.Fprintf(w, "}\n\n")
		}
	}

	if details.arrayOutput && !pkg.names.Has(name+"Array") && !genArgs.usingGenericTypes {
		pkg.genArrayOutput(w, name, name)
	}

	if details.mapOutput && !pkg.names.Has(name+"Map") && !genArgs.usingGenericTypes {
		pkg.genMapOutput(w, name, name)
	}
}

func goPrimitiveValue(value interface{}) (string, error) {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	//nolint:exhaustive // Only a subset of types have a default value.
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			return "true", nil
		}
		return "false", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		value := strconv.FormatFloat(v.Float(), 'f', -1, 64)
		if !strings.ContainsRune(value, '.') {
			value += ".0"
		}
		return value, nil
	case reflect.String:
		return fmt.Sprintf("%q", v.String()), nil
	default:
		return "", fmt.Errorf("unsupported default value of type %T", value)
	}
}

func (pkg *pkgContext) getConstValue(cv interface{}) (string, error) {
	var val string
	if cv != nil {
		v, err := goPrimitiveValue(cv)
		if err != nil {
			return "", err
		}
		val = v
	}

	return val, nil
}

// setDefaultValue generates a statement that assigns the default value of a
// property to a variable.
//
// The assign function is invoked with an expression that evaluates to the
// default value.
// It should return a statement that assigns that default value to the relevant
// variable.
// For example,
//
//	err := pkg.setDefaultValue(w, dv, t, func(w io.Writer, value string) error {
//		_, err := fmt.Fprintf(w, "v.%s = %s", fieldName, value)
//		return err
//	})
func (pkg *pkgContext) setDefaultValue(
	w io.Writer,
	dv *schema.DefaultValue,
	t schema.Type,
	assign func(io.Writer, string) error,
) error {
	contract.Requiref(dv.Value != nil || len(dv.Environment) > 0,
		"dv", "must have either a value or an environment variable override")

	var val string
	if dv.Value != nil {
		v, err := goPrimitiveValue(dv.Value)
		if err != nil {
			return err
		}
		val = v
		switch t.(type) {
		case *schema.EnumType:
			typeName := strings.TrimSuffix(pkg.typeString(codegen.UnwrapType(t)), "Input")
			val = fmt.Sprintf("%s(%s)", typeName, val)
		}
	}

	if len(dv.Environment) == 0 {
		// If there's no environment variable override,
		// assign and we're done.
		return assign(w, val)
	}

	// For environment variable override, we will assign only
	// if the environment variable is set.

	parser, typ := "nil", "string"
	switch codegen.UnwrapType(t).(type) {
	case *schema.ArrayType:
		parser, typ = pkg.internalModuleName+".ParseEnvStringArray", "pulumi.StringArray"
	}
	switch t {
	case schema.BoolType:
		parser, typ = pkg.internalModuleName+".ParseEnvBool", "bool"
	case schema.IntType:
		parser, typ = pkg.internalModuleName+".ParseEnvInt", "int"
	case schema.NumberType:
		parser, typ = pkg.internalModuleName+".ParseEnvFloat", "float64"
	}

	if val == "" {
		// If there's no explicit default value,
		// use nil so that we can assign conditionally.
		val = "nil"
	}

	// Roughly, we generate:
	//
	//	if d := internal.getEnvOrDefault(defaultValue, parser, "ENV_VAR"); d != nil {
	//		$assign(d.(type))
	//	}
	//
	// This has the following effect:
	//
	//  - if an environment variable was set, read from that
	//  - if a default value was specified, use that
	//  - otherwise, leave the variable unset
	fmt.Fprintf(w, "if d := %s.GetEnvOrDefault(%s, %s", pkg.internalModuleName, val, parser)
	for _, e := range dv.Environment {
		fmt.Fprintf(w, ", %q", e)
	}
	fmt.Fprintf(w, "); d != nil {\n\t")
	if err := assign(w, fmt.Sprintf("d.(%v)", typ)); err != nil {
		return err
	}
	fmt.Fprintf(w, "}\n")
	return nil
}

func (pkg *pkgContext) genResource(
	w io.Writer,
	r *schema.Resource,
	generateResourceContainerTypes bool,
	useGenericVariant bool,
) error {
	name := disambiguatedResourceName(r, pkg)
	printCommentWithDeprecationMessage(w, r.Comment, r.DeprecationMessage, false)
	fmt.Fprintf(w, "type %s struct {\n", name)

	switch {
	case r.IsProvider:
		fmt.Fprintf(w, "\tpulumi.ProviderResourceState\n\n")
	case r.IsComponent:
		fmt.Fprintf(w, "\tpulumi.ResourceState\n\n")
	default:
		fmt.Fprintf(w, "\tpulumi.CustomResourceState\n\n")
	}

	var secretProps []*schema.Property
	var secretInputProps []*schema.Property

	for _, p := range r.Properties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		outputType := pkg.outputType(p.Type)
		if useGenericVariant {
			outputType = pkg.genericOutputType(p.Type)
		}

		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", pkg.fieldName(r, p), outputType, p.Name)

		if p.Secret {
			secretProps = append(secretProps, p)
		}
	}
	fmt.Fprintf(w, "}\n\n")

	// Create a constructor function that registers a new instance of this resource.
	fmt.Fprintf(w, "// New%s registers a new resource with the given unique name, arguments, and options.\n", name)
	fmt.Fprintf(w, "func New%s(ctx *pulumi.Context,\n", name)
	fmt.Fprintf(w, "\tname string, args *%[1]sArgs, opts ...pulumi.ResourceOption) (*%[1]s, error) {\n", name)

	// Ensure required arguments are present.
	hasRequired := false
	for _, p := range r.InputProperties {
		if p.IsRequired() {
			hasRequired = true
		}
	}

	// Various validation checks
	fmt.Fprintf(w, "\tif args == nil {\n")
	if !hasRequired {
		fmt.Fprintf(w, "\t\targs = &%sArgs{}\n", name)
	} else {
		fmt.Fprintln(w, "\t\treturn nil, errors.New(\"missing one or more required arguments\")")
	}
	fmt.Fprintf(w, "\t}\n\n")

	// Produce the inputs.

	// Check all required inputs are present
	for _, p := range r.InputProperties {
		if p.IsRequired() && isNilType(p.Type) && p.DefaultValue == nil {
			fmt.Fprintf(w, "\tif args.%s == nil {\n", pkg.fieldName(r, p))
			fmt.Fprintf(w, "\t\treturn nil, errors.New(\"invalid value for required argument '%s'\")\n", pkg.fieldName(r, p))
			fmt.Fprintf(w, "\t}\n")
		}

		if p.Secret {
			secretInputProps = append(secretInputProps, p)
		}
	}

	assign := func(w io.Writer, p *schema.Property, value string) {
		pkg.assignProperty(w, p, "args", value, isNilType(p.Type), useGenericVariant)
	}

	for _, p := range r.InputProperties {
		if p.ConstValue != nil {
			v, err := pkg.getConstValue(p.ConstValue)
			if err != nil {
				return err
			}
			assign(w, p, v)
		} else if p.DefaultValue != nil {
			if isNilType(p.Type) {
				fmt.Fprintf(w, "\tif args.%s == nil {\n", pkg.fieldName(r, p))
			} else {
				fmt.Fprintf(w, "\tif %s.IsZero(args.%s) {\n", pkg.internalModuleName, pkg.fieldName(r, p))
			}

			err := pkg.setDefaultValue(w, p.DefaultValue, codegen.UnwrapType(p.Type), func(w io.Writer, dv string) error {
				assign(w, p, dv)
				return nil
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "\t}\n")
		} else if name := pkg.provideDefaultsFuncName(p.Type); name != "" && !pkg.disableObjectDefaults {
			optionalDeref := ""
			if p.IsRequired() {
				optionalDeref = "*"
			}

			toOutputMethod := pkg.toOutputMethod(p.Type)
			outputType := pkg.outputType(p.Type)
			resolvedType := pkg.typeString(codegen.ResolvedType(p.Type))
			originalValue := fmt.Sprintf("args.%s.%s()", pkg.fieldName(r, p), toOutputMethod)
			valueWithDefaults := fmt.Sprintf("%[1]v.ApplyT(func (v %[2]s) %[2]s { return %[3]sv.%[4]s() }).(%[5]s)",
				originalValue, resolvedType, optionalDeref, name, outputType)
			if p.Plain {
				valueWithDefaults = fmt.Sprintf("args.%v.Defaults()", pkg.fieldName(r, p))
			}

			if useGenericVariant {
				fmt.Fprintf(w, "if args.%s != nil {\n", pkg.fieldName(r, p))
				t := p.Type
				optionalPointer := ""
				if isOptionalType(reduceInputType(t)) && !isArrayType(codegen.UnwrapType(t)) && !isMapType(codegen.UnwrapType(t)) {
					optionalPointer = "*"
				}

				inputType := pkg.genericInputTypeImpl(t)
				if strings.HasPrefix(inputType, "*") {
					optionalPointer = ""
				}

				fmt.Fprintf(w, "args.%s = pulumix.Apply(args.%s, func(o %s%s) %s%s { return o.Defaults() })\n",
					pkg.fieldName(r, p),
					pkg.fieldName(r, p),
					optionalPointer,
					inputType,
					optionalPointer,
					inputType)

				fmt.Fprintf(w, "}\n")
			} else {
				if !p.IsRequired() {
					fmt.Fprintf(w, "if args.%s != nil {\n", pkg.fieldName(r, p))
					fmt.Fprintf(w, "args.%[1]s = %s\n", pkg.fieldName(r, p), valueWithDefaults)
					fmt.Fprint(w, "}\n")
				} else {
					fmt.Fprintf(w, "args.%[1]s = %s\n", pkg.fieldName(r, p), valueWithDefaults)
				}
			}
		}
	}

	// Set any defined aliases.
	if len(r.Aliases) > 0 {
		fmt.Fprintf(w, "\taliases := pulumi.Aliases([]pulumi.Alias{\n")
		for _, alias := range r.Aliases {
			s := "\t\t{\n"
			if alias.Type != nil {
				s += fmt.Sprintf("\t\t\tType: pulumi.String(%q),\n", *alias.Type)
			}
			s += "\t\t},\n"
			fmt.Fprint(w, s)
		}
		fmt.Fprintf(w, "\t})\n")
		fmt.Fprintf(w, "\topts = append(opts, aliases)\n")
	}

	// Setup secrets
	for _, p := range secretInputProps {
		fmt.Fprintf(w, "\tif args.%s != nil {\n", pkg.fieldName(r, p))

		if !useGenericVariant {
			fmt.Fprintf(w, "\t\targs.%[1]s = pulumi.ToSecret(args.%[1]s).(%[2]s)\n",
				pkg.fieldName(r, p),
				pkg.typeString(p.Type))
		} else {
			fmt.Fprintf(w, "\t\tuntypedSecretValue := pulumi.ToSecret(args.%s.ToOutput(ctx.Context()).Untyped())\n",
				pkg.fieldName(r, p))

			t := p.Type
			optionalPointer := ""
			if isOptionalType(reduceInputType(t)) && !isArrayType(codegen.UnwrapType(t)) && !isMapType(codegen.UnwrapType(t)) {
				optionalPointer = "*"
			}

			inputType := pkg.genericInputTypeImpl(t)
			if strings.HasPrefix(inputType, "*") {
				optionalPointer = ""
			}
			fmt.Fprintf(w, "\t\targs.%s = pulumix.MustConvertTyped[%s%s](untypedSecretValue)\n",
				pkg.fieldName(r, p),
				optionalPointer,
				inputType)
		}

		fmt.Fprintf(w, "\t}\n")
	}
	if len(secretProps) > 0 {
		fmt.Fprintf(w, "\tsecrets := pulumi.AdditionalSecretOutputs([]string{\n")
		for _, sp := range secretProps {
			fmt.Fprintf(w, "\t\t\t%q,\n", sp.Name)
		}
		fmt.Fprintf(w, "\t})\n")
		fmt.Fprintf(w, "\topts = append(opts, secrets)\n")
	}

	// Setup replaceOnChange
	replaceOnChangesProps, errList := r.ReplaceOnChanges()
	for _, err := range errList {
		cmdutil.Diag().Warningf(&diag.Diag{Message: err.Error()})
	}
	replaceOnChangesStrings := schema.PropertyListJoinToString(replaceOnChangesProps,
		func(x string) string { return x })
	if len(replaceOnChangesProps) > 0 {
		fmt.Fprint(w, "\treplaceOnChanges := pulumi.ReplaceOnChanges([]string{\n")
		for _, p := range replaceOnChangesStrings {
			fmt.Fprintf(w, "\t\t%q,\n", p)
		}
		fmt.Fprint(w, "\t})\n")
		fmt.Fprint(w, "\topts = append(opts, replaceOnChanges)\n")
	}

	err := pkg.GenPkgDefaultsOptsCall(w, false /*invoke*/)
	if err != nil {
		return err
	}

	// If this is a parameterized resource we need the package ref.
	def, err := pkg.pkg.Definition()
	if err != nil {
		return err
	}
	assignment := ":="
	packageRef := ""
	packageArg := ""
	if def.Parameterization != nil {
		assignment = "="
		packageRef = "Package"
		packageArg = "ref, "
		err = pkg.GenPkgGetPackageRefCall(w, "nil")
		if err != nil {
			return err
		}
	}

	// Finally make the call to registration.
	fmt.Fprintf(w, "\tvar resource %s\n", name)
	component := ""
	if r.IsComponent {
		component = "RemoteComponent"
	}

	fmt.Fprintf(w, "\terr %s ctx.Register%s%sResource(\"%s\", name, args, &resource, %sopts...)\n",
		assignment, packageRef, component, r.Token, packageArg)
	fmt.Fprintf(w, "\tif err != nil {\n")
	fmt.Fprintf(w, "\t\treturn nil, err\n")
	fmt.Fprintf(w, "\t}\n")
	fmt.Fprintf(w, "\treturn &resource, nil\n")
	fmt.Fprintf(w, "}\n\n")

	// Emit a factory function that reads existing instances of this resource.
	if !r.IsProvider && !r.IsComponent {
		fmt.Fprintf(w, "// Get%[1]s gets an existing %[1]s resource's state with the given name, ID, and optional\n", name)
		fmt.Fprintf(w, "// state properties that are used to uniquely qualify the lookup (nil if not required).\n")
		fmt.Fprintf(w, "func Get%s(ctx *pulumi.Context,\n", name)
		fmt.Fprintf(w, "\tname string, id pulumi.IDInput, state *%[1]sState, opts ...pulumi.ResourceOption) (*%[1]s, error) {\n", name)
		fmt.Fprintf(w, "\tvar resource %s\n", name)

		// If this is a parameterized resource we need the package ref.
		def, err := pkg.pkg.Definition()
		if err != nil {
			return err
		}
		assignment := ":="
		packageRef := ""
		packageArg := ""
		if def.Parameterization != nil {
			assignment = "="
			packageRef = "Package"
			packageArg = "ref, "
			err = pkg.GenPkgGetPackageRefCall(w, "nil")
			if err != nil {
				return err
			}
		}

		fmt.Fprintf(w, "\terr %s ctx.Read%sResource(\"%s\", name, id, state, &resource, %sopts...)\n",
			assignment, packageRef, r.Token, packageArg)
		fmt.Fprintf(w, "\tif err != nil {\n")
		fmt.Fprintf(w, "\t\treturn nil, err\n")
		fmt.Fprintf(w, "\t}\n")
		fmt.Fprintf(w, "\treturn &resource, nil\n")
		fmt.Fprintf(w, "}\n\n")

		// Emit the state types for get methods.
		fmt.Fprintf(w, "// Input properties used for looking up and filtering %s resources.\n", name)
		fmt.Fprintf(w, "type %sState struct {\n", cgstrings.Camel(name))
		if r.StateInputs != nil {
			for _, p := range r.StateInputs.Properties {
				printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
				fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", pkg.fieldName(r, p), pkg.typeString(codegen.ResolvedType(codegen.OptionalType(p))), p.Name)
			}
		}
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "type %sState struct {\n", name)
		if r.StateInputs != nil {
			for _, p := range r.StateInputs.Properties {
				printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
				inputType := pkg.inputType(p.Type)
				if useGenericVariant {
					inputType = pkg.genericInputType(codegen.OptionalType(p))
				}
				fmt.Fprintf(w, "\t%s %s\n", pkg.fieldName(r, p), inputType)
			}
		}
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (%sState) ElementType() reflect.Type {\n", name)
		fmt.Fprintf(w, "\treturn reflect.TypeOf((*%sState)(nil)).Elem()\n", cgstrings.Camel(name))
		fmt.Fprintf(w, "}\n\n")
	}

	// Emit the args types.
	fmt.Fprintf(w, "type %sArgs struct {\n", cgstrings.Camel(name))
	for _, p := range r.InputProperties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		inputTypeName := pkg.typeString(codegen.ResolvedType(p.Type))
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", pkg.fieldName(r, p), inputTypeName, p.Name)
	}
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "// The set of arguments for constructing a %s resource.\n", name)
	fmt.Fprintf(w, "type %sArgs struct {\n", name)
	for _, p := range r.InputProperties {
		typ := p.Type
		if p.Plain {
			typ = codegen.MapOptionalType(typ, func(typ schema.Type) schema.Type {
				if input, ok := typ.(*schema.InputType); ok {
					return input.ElementType
				}
				return typ
			})
		}

		inputTypeName := pkg.inputType(typ)
		if p.Plain {
			inputTypeName = pkg.typeString(typ)
		}

		if useGenericVariant {
			if p.Plain {
				inputTypeName = pkg.typeString(codegen.ResolvedType(typ))
			} else {
				inputTypeName = pkg.genericInputType(typ)
			}
		}

		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		fmt.Fprintf(w, "\t%s %s\n", pkg.fieldName(r, p), inputTypeName)
	}
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (%sArgs) ElementType() reflect.Type {\n", name)
	fmt.Fprintf(w, "\treturn reflect.TypeOf((*%sArgs)(nil)).Elem()\n", cgstrings.Camel(name))
	fmt.Fprintf(w, "}\n")

	// Emit resource methods.
	for _, method := range r.Methods {
		methodName := Title(method.Name)
		f := method.Function

		var objectReturnType *schema.ObjectType
		if f.ReturnType != nil {
			if objectType, ok := f.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				objectReturnType = objectType
			}
		}

		liftReturn := pkg.liftSingleValueMethodReturns && objectReturnType != nil && len(objectReturnType.Properties) == 1

		var args []*schema.Property
		if f.Inputs != nil {
			for _, arg := range f.Inputs.InputShape.Properties {
				if arg.Name == "__self__" {
					continue
				}
				args = append(args, arg)
			}
		}

		// Now emit the method signature.
		argsig := "ctx *pulumi.Context"
		if len(args) > 0 {
			argsig = fmt.Sprintf("%s, args *%s%sArgs", argsig, name, methodName)
		}
		var retty string
		if f.ReturnTypePlain {
			if objectReturnType == nil {
				t := pkg.typeString(codegen.ResolvedType(f.ReturnType))
				retty = fmt.Sprintf("(o %s, e error)", t)
			} else {
				retty = fmt.Sprintf("(o %s%sResult, e error)", name, methodName)
			}
		} else if objectReturnType == nil {
			retty = "error"
		} else if liftReturn {
			if useGenericVariant {
				retty = fmt.Sprintf("(%s, error)", pkg.genericOutputType(objectReturnType.Properties[0].Type))
			} else {
				retty = fmt.Sprintf("(%s, error)", pkg.outputType(objectReturnType.Properties[0].Type))
			}
		} else {
			retty = fmt.Sprintf("(%s%sResultOutput, error)", name, methodName)
		}
		fmt.Fprintf(w, "\n")
		printCommentWithDeprecationMessage(w, f.Comment, f.DeprecationMessage, false)
		fmt.Fprintf(w, "func (r *%s) %s(%s) %s {\n", name, methodName, argsig, retty)

		resultVar := "_"
		if objectReturnType != nil {
			resultVar = "out"
		}

		// Make a map of inputs to pass to the runtime function.
		inputsVar := "nil"
		if len(args) > 0 {
			inputsVar = "args"
		}

		// Now simply invoke the runtime function with the arguments.
		outputsType := "pulumi.AnyOutput"
		if objectReturnType != nil || f.ReturnTypePlain {
			if liftReturn {
				outputsType = fmt.Sprintf("%s%sResultOutput", cgstrings.Camel(name), methodName)
			} else {
				outputsType = fmt.Sprintf("%s%sResultOutput", name, methodName)
			}
		}

		// If this is a parameterized resource we need the package ref.
		def, err := pkg.pkg.Definition()
		if err != nil {
			return err
		}
		packageRef := ""
		packageArg := ""
		if def.Parameterization != nil {
			packageRef = "Package"
			packageArg = ", ref"
			err = pkg.GenPkgGetPackageRefCall(w, outputsType+"{}")
			if err != nil {
				return err
			}
		}

		if !f.ReturnTypePlain {
			fmt.Fprintf(w, "\t%s, err := ctx.Call%s(%q, %s, %s{}, r%s)\n",
				resultVar, packageRef, f.Token, inputsVar, outputsType, packageArg)
		}

		if f.ReturnTypePlain {
			// single-value returning methods use a magic property "res" on the wire
			property := ""
			if objectReturnType == nil {
				property = cgstrings.UppercaseFirst("res")
			}
			fmt.Fprintf(w, "\tinternal.CallPlain(ctx, %q, %s, %s{}, r, %q, reflect.ValueOf(&o), &e)\n",
				f.Token, inputsVar, outputsType, property)
			fmt.Fprintf(w, "\treturn\n")
		} else if objectReturnType == nil {
			fmt.Fprintf(w, "\treturn err\n")
		} else if liftReturn {
			// Check the error before proceeding.
			fmt.Fprintf(w, "\tif err != nil {\n")
			if useGenericVariant {
				fmt.Fprint(w, "\t\treturn nil, err\n")
			} else {
				fmt.Fprintf(w, "\t\treturn %s{}, err\n", pkg.outputType(objectReturnType.Properties[0].Type))
			}

			fmt.Fprintf(w, "\t}\n")

			// Get the name of the method to return the output
			fmt.Fprintf(w, "\treturn %s.(%s).%s(), nil\n", resultVar, cgstrings.Camel(outputsType), Title(objectReturnType.Properties[0].Name))
		} else {
			// Check the error before proceeding.
			fmt.Fprintf(w, "\tif err != nil {\n")
			fmt.Fprintf(w, "\t\treturn %s{}, err\n", outputsType)
			fmt.Fprintf(w, "\t}\n")

			// Return the result.
			fmt.Fprintf(w, "\treturn %s.(%s), nil\n", resultVar, outputsType)
		}
		fmt.Fprintf(w, "}\n")

		// If there are argument and/or return types, emit them.
		if len(args) > 0 {
			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "type %s%sArgs struct {\n", cgstrings.Camel(name), methodName)
			for _, p := range args {
				printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
				inputTypeName := pkg.typeString(codegen.ResolvedType(p.Type))
				if useGenericVariant {
					inputTypeName = pkg.genericInputType(codegen.ResolvedType(p.Type))
				}
				fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", pkg.fieldName(nil, p), inputTypeName, p.Name)
			}
			fmt.Fprintf(w, "}\n\n")

			fmt.Fprintf(w, "// The set of arguments for the %s method of the %s resource.\n", methodName, name)
			fmt.Fprintf(w, "type %s%sArgs struct {\n", name, methodName)
			for _, p := range args {
				printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
				inputTypeName := pkg.typeString(p.Type)
				if useGenericVariant {
					inputTypeName = pkg.genericInputType(codegen.ResolvedType(p.Type))
				}
				fmt.Fprintf(w, "\t%s %s\n", pkg.fieldName(nil, p), inputTypeName)
			}
			fmt.Fprintf(w, "}\n\n")

			fmt.Fprintf(w, "func (%s%sArgs) ElementType() reflect.Type {\n", name, methodName)
			fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s%sArgs)(nil)).Elem()\n", cgstrings.Camel(name), methodName)
			fmt.Fprintf(w, "}\n\n")
		}
		if objectReturnType != nil || f.ReturnTypePlain {
			outputStructName := name

			var comment string
			var properties []*schema.Property
			if f.ReturnTypePlain && objectReturnType == nil {
				properties = []*schema.Property{
					{
						Name:  "res",
						Type:  f.ReturnType,
						Plain: true,
					},
				}
			} else {
				properties = objectReturnType.Properties
				comment = objectReturnType.Comment
			}

			// Don't export the result struct if we're lifting the value
			if liftReturn {
				outputStructName = cgstrings.Camel(name)
			}

			fmt.Fprintf(w, "\n")
			pkg.genPlainType(w, fmt.Sprintf("%s%sResult", outputStructName, methodName), comment, "", properties)

			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "type %s%sResultOutput struct{ *pulumi.OutputState }\n\n", outputStructName, methodName)

			fmt.Fprintf(w, "func (%s%sResultOutput) ElementType() reflect.Type {\n", outputStructName, methodName)
			fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s%sResult)(nil)).Elem()\n", outputStructName, methodName)
			fmt.Fprintf(w, "}\n")

			for _, p := range properties {
				fmt.Fprintf(w, "\n")
				outputTypeName := pkg.outputType(p.Type)
				if useGenericVariant {
					outputTypeName = pkg.genericOutputType(p.Type)
				}
				printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, false)
				fmt.Fprintf(w, "func (o %s%sResultOutput) %s() %s {\n", outputStructName, methodName, Title(p.Name),
					outputTypeName)
				if !useGenericVariant {
					fmt.Fprintf(w, "\treturn o.ApplyT(func (v %s%sResult) %s { return v.%s }).(%s)\n", outputStructName, methodName,
						pkg.typeString(codegen.ResolvedType(p.Type)), Title(p.Name), outputTypeName)
				} else {
					fmt.Fprintf(w, "\treturn pulumix.Apply(o, func(v %s%sResult) %s { return v.%s })\n", outputStructName, methodName,
						pkg.typeString(codegen.ResolvedType(p.Type)), Title(p.Name))
				}

				fmt.Fprintf(w, "}\n")
			}
		}
	}

	if !useGenericVariant {
		// Emit the resource input type.
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "type %sInput interface {\n", name)
		fmt.Fprintf(w, "\tpulumi.Input\n\n")
		fmt.Fprintf(w, "\tTo%[1]sOutput() %[1]sOutput\n", name)
		fmt.Fprintf(w, "\tTo%[1]sOutputWithContext(ctx context.Context) %[1]sOutput\n", name)
		fmt.Fprintf(w, "}\n\n")

		pkg.genInputImplementation(w, name, "*"+name, "*"+name, false, false)

		if generateResourceContainerTypes && !r.IsProvider {
			// Generate the resource array input.
			pkg.genInputInterface(w, name+"Array")
			fmt.Fprintf(w, "type %[1]sArray []%[1]sInput\n\n", name)
			pkg.genInputImplementation(w, name+"Array", name+"Array", "[]*"+name, false, false)

			// Generate the resource map input.
			pkg.genInputInterface(w, name+"Map")
			fmt.Fprintf(w, "type %[1]sMap map[string]%[1]sInput\n\n", name)
			pkg.genInputImplementation(w, name+"Map", name+"Map", "map[string]*"+name, false, false)
		}
	}

	outputElementType := "*" + name
	if useGenericVariant {
		outputElementType = name
	}
	pkg.genOutputType(w, name, outputElementType, false, useGenericVariant)

	// Emit chaining methods for the resource output type.
	for _, p := range r.Properties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, false)
		outputType := pkg.outputType(p.Type)
		if useGenericVariant {
			outputType = pkg.genericOutputType(p.Type)
		}

		propName := pkg.fieldName(r, p)
		switch strings.ToLower(p.Name) {
		case "elementtype", "issecret":
			propName = "Get" + propName
		}
		fmt.Fprintf(w, "func (o %sOutput) %s() %s {\n", name, propName, outputType)
		if !useGenericVariant {
			fmt.Fprintf(w, "\treturn o.ApplyT(func (v *%s) %s { return v.%s }).(%s)\n",
				name, outputType, pkg.fieldName(r, p), outputType)
		} else {
			needsCast := genericTypeNeedsExplicitCasting(outputType)

			elementType := pkg.typeString(codegen.ResolvedType(p.Type))

			if strings.HasPrefix(outputType, "pulumix.GPtrOutput") && !strings.HasPrefix(elementType, "*") {
				elementType = "*" + elementType
			}

			isOptionalAssetOrArchive := isOptionalType(reduceInputType(p.Type)) &&
				isAssetOrArchive(codegen.UnwrapType(p.Type))
			if isOptionalAssetOrArchive && !strings.HasPrefix(elementType, "*") {
				elementType = "*" + elementType
			}

			if needsCast {
				// needs an explicit cast operation to align the types
				fmt.Fprintf(w, "\tvalue := pulumix.Apply[%s](o, func (v %s) %s { return v.%s })\n",
					name, name, outputType, pkg.fieldName(r, p))
				fmt.Fprintf(w, "\tunwrapped := pulumix.Flatten[%s, %s](value)\n",
					elementType, outputType)
				fmt.Fprintf(w, "\treturn %s{OutputState: unwrapped.OutputState}\n", outputType)
			} else {
				fmt.Fprintf(w, "\tvalue := pulumix.Apply[%s](o, func (v %s) %s { return v.%s })\n",
					name, name, outputType, pkg.fieldName(r, p))
				if !p.Plain {
					fmt.Fprintf(w, "\treturn pulumix.Flatten[%s, %s](value)\n",
						elementType, outputType)
				} else {
					fmt.Fprintf(w, "\treturn value\n")
				}
			}
		}

		fmt.Fprintf(w, "}\n\n")
	}

	if generateResourceContainerTypes && !r.IsProvider && !useGenericVariant {
		pkg.genArrayOutput(w, name, "*"+name)
		pkg.genMapOutput(w, name, "*"+name)
	}

	pkg.genResourceRegistrations(w, r, generateResourceContainerTypes, useGenericVariant)

	return nil
}

func goPackageInfo(packageReference schema.PackageReference) GoPackageInfo {
	if packageReference == nil {
		return GoPackageInfo{}
	}

	def, err := packageReference.Definition()
	contract.AssertNoErrorf(err, "Could not load definition for %q", packageReference.Name())
	contract.AssertNoErrorf(def.ImportLanguages(map[string]schema.Language{"go": Importer}),
		"Could not import languages")
	if info, ok := def.Language["go"].(GoPackageInfo); ok {
		if info.Generics == "" {
			info.Generics = GenericsSettingNone
		}
		return info
	}
	return GoPackageInfo{}
}

func NeedsGoOutputVersion(f *schema.Function) bool {
	goInfo := goPackageInfo(f.PackageReference)

	if goInfo.DisableFunctionOutputVersions {
		return false
	}

	return f.ReturnType != nil
}

func (pkg *pkgContext) genFunctionCodeFile(f *schema.Function) (string, error) {
	importsAndAliases := map[string]string{}
	pkg.getImports(f, importsAndAliases)
	importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumi"] = ""
	importsAndAliases[path.Join(pkg.importBasePath, pkg.internalModuleName)] = ""
	buffer := &bytes.Buffer{}
	goInfo := goPackageInfo(pkg.pkg)
	var imports []string
	if f.ReturnType != nil {
		imports = []string{"context", "reflect"}
		if goInfo.Generics == GenericsSettingSideBySide {
			importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumix"] = ""
		}
	}

	pkg.genHeader(buffer, imports, importsAndAliases, false /* isUtil */)
	emitGenericVariant := false
	if err := pkg.genFunction(buffer, f, emitGenericVariant); err != nil {
		return "", err
	}
	pkg.genFunctionOutputVersion(buffer, f, emitGenericVariant)
	return buffer.String(), nil
}

func (pkg *pkgContext) genGenericVariantFunctionCodeFile(f *schema.Function) (string, error) {
	importsAndAliases := map[string]string{}
	pkg.getImports(f, importsAndAliases)
	importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumi"] = ""
	if f.NeedsOutputVersion() {
		importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumix"] = ""
	}

	importsAndAliases[path.Join(pkg.importBasePath, pkg.internalModuleName)] = ""
	buffer := &bytes.Buffer{}

	var imports []string
	if NeedsGoOutputVersion(f) {
		imports = []string{"context", "reflect"}
	}

	pkg.genHeader(buffer, imports, importsAndAliases, false /* isUtil */)
	useGenericTypes := true
	if err := pkg.genFunction(buffer, f, useGenericTypes); err != nil {
		return "", err
	}
	pkg.genFunctionOutputVersion(buffer, f, useGenericTypes)
	return buffer.String(), nil
}

func (pkg *pkgContext) genFunction(w io.Writer, f *schema.Function, useGenericTypes bool) error {
	name := pkg.functionName(f)

	if f.MultiArgumentInputs {
		return fmt.Errorf("go SDK-gen does not implement MultiArgumentInputs for function '%s'", f.Token)
	}

	var objectReturnType *schema.ObjectType
	if f.ReturnType != nil {
		if objectType, ok := f.ReturnType.(*schema.ObjectType); ok {
			objectReturnType = objectType
		} else {
			// TODO: remove when we add support for generalized return type for go
			return fmt.Errorf("go sdk-gen doesn't support non-Object return types for function %s", f.Token)
		}
	}

	printCommentWithDeprecationMessage(w, f.Comment, f.DeprecationMessage, false)

	// Now, emit the function signature.
	argsig := "ctx *pulumi.Context"
	if f.Inputs != nil {
		argsig = fmt.Sprintf("%s, args *%sArgs", argsig, name)
	}
	var retty string
	if objectReturnType == nil {
		retty = "error"
	} else {
		retty = fmt.Sprintf("(*%sResult, error)", name)
	}
	fmt.Fprintf(w, "func %s(%s, opts ...pulumi.InvokeOption) %s {\n", name, argsig, retty)

	// Make a map of inputs to pass to the runtime function.
	var inputsVar string
	if f.Inputs == nil {
		inputsVar = "nil"
	} else if codegen.IsProvideDefaultsFuncRequired(f.Inputs) && !pkg.disableObjectDefaults {
		inputsVar = "args.Defaults()"
	} else {
		inputsVar = "args"
	}

	// Now simply invoke the runtime function with the arguments.
	var outputsType string
	if objectReturnType == nil {
		outputsType = "struct{}"
	} else {
		outputsType = name + "Result"
	}

	err := pkg.GenPkgDefaultsOptsCall(w, true /*invoke*/)
	if err != nil {
		return err
	}

	// If this is a parameterized resource we need the package ref.
	def, err := pkg.pkg.Definition()
	if err != nil {
		return err
	}
	assignment := ":="
	packageRef := ""
	packageArg := ""
	if def.Parameterization != nil {
		assignment = "="
		packageRef = "Package"
		packageArg = "ref, "
		err = pkg.GenPkgGetPackageRefCall(w, "nil")
		if err != nil {
			return err
		}
	}

	fmt.Fprintf(w, "\tvar rv %s\n", outputsType)
	fmt.Fprintf(w, "\terr %s ctx.Invoke%s(\"%s\", %s, &rv, %sopts...)\n",
		assignment, packageRef, f.Token, inputsVar, packageArg)

	if objectReturnType == nil {
		fmt.Fprintf(w, "\treturn err\n")
	} else {
		// Check the error before proceeding.
		fmt.Fprintf(w, "\tif err != nil {\n")
		fmt.Fprintf(w, "\t\treturn nil, err\n")
		fmt.Fprintf(w, "\t}\n")

		// Return the result.
		var retValue string
		if codegen.IsProvideDefaultsFuncRequired(objectReturnType) && !pkg.disableObjectDefaults {
			retValue = "rv.Defaults()"
		} else {
			retValue = "&rv"
		}
		fmt.Fprintf(w, "\treturn %s, nil\n", retValue)
	}
	fmt.Fprintf(w, "}\n")

	// If there are argument and/or return types, emit them.
	if f.Inputs != nil {
		fmt.Fprintf(w, "\n")
		fnInputsName := pkg.functionArgsTypeName(f)
		pkg.genPlainType(w, fnInputsName, f.Inputs.Comment, "", f.Inputs.Properties)
		if codegen.IsProvideDefaultsFuncRequired(f.Inputs) && !pkg.disableObjectDefaults {
			if err := pkg.genObjectDefaultFunc(w, fnInputsName, f.Inputs.Properties, useGenericTypes); err != nil {
				return err
			}
		}
	}
	if objectReturnType != nil {
		fmt.Fprintf(w, "\n")
		fnOutputsName := pkg.functionResultTypeName(f)
		pkg.genPlainType(w, fnOutputsName, objectReturnType.Comment, "", objectReturnType.Properties)
		if codegen.IsProvideDefaultsFuncRequired(objectReturnType) && !pkg.disableObjectDefaults {
			if err := pkg.genObjectDefaultFunc(w, fnOutputsName, objectReturnType.Properties, useGenericTypes); err != nil {
				return err
			}
		}
	}
	return nil
}

func (pkg *pkgContext) functionName(f *schema.Function) string {
	// If the function starts with New or Get, it will conflict; so rename them.
	name, hasName := pkg.functionNames[f]

	if !hasName {
		panic(fmt.Sprintf("No function name found for %v", f))
	}

	return name
}

func (pkg *pkgContext) functionArgsTypeName(f *schema.Function) string {
	name := pkg.functionName(f)
	return name + "Args"
}

func (pkg *pkgContext) functionResultTypeName(f *schema.Function) string {
	name := pkg.functionName(f)
	return name + "Result"
}

func genericTypeNeedsExplicitCasting(outputType string) bool {
	return strings.HasPrefix(outputType, "pulumix.ArrayOutput") ||
		strings.HasPrefix(outputType, "pulumix.MapOutput") ||
		strings.HasPrefix(outputType, "pulumix.GPtrOutput") ||
		strings.HasPrefix(outputType, "pulumix.GArrayOutput") ||
		strings.HasPrefix(outputType, "pulumix.GMapOutput")
}

func (pkg *pkgContext) genFunctionOutputGenericVersion(w io.Writer, f *schema.Function) {
	originalName := pkg.functionName(f)
	name := originalName + "Output"
	originalResultTypeName := pkg.functionResultTypeName(f)
	resultTypeName := originalResultTypeName + "Output"

	var code string

	if f.Inputs != nil {
		code = `
func ${fn}Output(ctx *pulumi.Context, args ${fn}OutputArgs, opts ...pulumi.InvokeOption) ${outputType} {
	outputResult := pulumix.ApplyErr[*${fn}Args](args.ToOutput(), func(plainArgs *${fn}Args) (*${fn}Result, error) {
		return ${fn}(ctx, plainArgs, opts...)
	})

	return pulumix.Cast[${outputType}, *${fn}Result](outputResult)
}
`
	} else {
		code = `
func ${fn}Output(ctx *pulumi.Context, opts ...pulumi.InvokeOption) ${outputType} {
	outputResult := pulumix.ApplyErr[int](pulumix.Val(0), func(_ int) (*${fn}Result, error) {
		return ${fn}(ctx, opts...)
	})

	return pulumix.Cast[${outputType}, *${fn}Result](outputResult)
}
`
	}

	code = strings.ReplaceAll(code, "${fn}", originalName)
	code = strings.ReplaceAll(code, "${outputType}", resultTypeName)
	fmt.Fprint(w, code)

	if f.Inputs != nil {
		useGenericTypes := true
		pkg.genInputArgsStruct(w, name+"Args", f.Inputs.InputShape, useGenericTypes)

		receiverType := name + "Args"
		plainType := originalName + "Args"

		fmt.Fprintf(w, "func (args %s) ToOutput() pulumix.Output[*%s] {\n", receiverType, plainType)
		fmt.Fprint(w, "\tallArgs := pulumix.All(\n")
		for i, p := range f.Inputs.Properties {
			fmt.Fprintf(w, "\t\targs.%s.ToOutput(context.Background()).AsAny()", pkg.fieldName(nil, p))
			if i < len(f.Inputs.Properties)-1 {
				fmt.Fprint(w, ",\n")
			}
		}
		fmt.Fprint(w, ")\n")

		fmt.Fprintf(w, "\treturn pulumix.Apply[[]any](allArgs, func(resolvedArgs []interface{}) *%s {\n", plainType)
		fmt.Fprintf(w, "\t\treturn &%s{\n", plainType)
		for i, p := range f.Inputs.Properties {
			fmt.Fprintf(w, "\t\t\t%s: resolvedArgs[%d].(%s),\n",
				pkg.fieldName(nil, p),
				i,
				pkg.typeString(p.Type))
		}
		fmt.Fprintf(w, "\t\t}\n")
		fmt.Fprintf(w, "\t})\n")
		fmt.Fprintf(w, "}\n\n")
	}

	var objectReturnType *schema.ObjectType
	if f.ReturnType != nil {
		if objectType, ok := f.ReturnType.(*schema.ObjectType); ok && objectType != nil {
			objectReturnType = objectType
		}
	}

	if objectReturnType != nil {
		fmt.Fprintf(w, "type %sOutput struct { *pulumi.OutputState }\n\n", originalResultTypeName)

		fmt.Fprintf(w, "func (%sOutput) ElementType() reflect.Type {\n", originalResultTypeName)
		fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s)(nil)).Elem()\n", originalResultTypeName)
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (o %sOutput) ToOutput(context.Context) pulumix.Output[*%s] {\n",
			originalResultTypeName,
			originalResultTypeName)
		fmt.Fprintf(w, "\treturn pulumix.Output[*%s]{\n", originalResultTypeName)
		fmt.Fprint(w, "\t\tOutputState: o.OutputState,\n")
		fmt.Fprint(w, "\t}\n")
		fmt.Fprint(w, "}\n\n")

		// generate accessors for each property of the output
		for _, p := range objectReturnType.Properties {
			outputType := pkg.genericOutputType(p.Type)

			fmt.Fprintf(w, "func (o %s) %s() %s {\n", resultTypeName, pkg.fieldName(nil, p), outputType)

			needsCast := genericTypeNeedsExplicitCasting(outputType)

			if !needsCast {
				fmt.Fprintf(w, "\treturn pulumix.Apply[*%s](o, func (v *%s) %s { return v.%s })\n",
					originalResultTypeName,
					originalResultTypeName,
					pkg.typeString(p.Type),
					pkg.fieldName(nil, p))
			} else {
				fmt.Fprintf(w, "\tvalue := pulumix.Apply[*%s](o, func (v *%s) %s { return v.%s })\n",
					originalResultTypeName,
					originalResultTypeName,
					pkg.typeString(p.Type),
					pkg.fieldName(nil, p))

				fmt.Fprintf(w, "\treturn %s{\n", outputType)
				fmt.Fprintf(w, "\t\tOutputState: value.OutputState,\n")
				fmt.Fprintf(w, "\t}\n")
			}

			fmt.Fprintf(w, "}\n\n")
		}
	}
}

func (pkg *pkgContext) genFunctionOutputVersion(w io.Writer, f *schema.Function, useGenericTypes bool) {
	if f.ReturnType == nil {
		return
	}

	if useGenericTypes {
		pkg.genFunctionOutputGenericVersion(w, f)
		return
	}

	originalName := pkg.functionName(f)
	name := originalName + "Output"
	originalResultTypeName := pkg.functionResultTypeName(f)
	resultTypeName := originalResultTypeName + "Output"

	var code string

	var inputsVar string
	if f.Inputs == nil {
		inputsVar = "nil"
	} else if codegen.IsProvideDefaultsFuncRequired(f.Inputs) && !pkg.disableObjectDefaults {
		inputsVar = "args.Defaults()"
	} else {
		inputsVar = "args"
	}

	if f.Inputs != nil {
		code = `
func ${fn}Output(ctx *pulumi.Context, args ${fn}OutputArgs, opts ...pulumi.InvokeOption) ${outputType} {
	return pulumi.ToOutputWithContext(context.Background(), args).
		ApplyT(func(v interface{}) (${outputType}, error) {
			args := v.(${fn}Args)
			opts = ${internalModule}.PkgInvokeDefaultOpts(opts)
			var rv ${fn}Result
			secret, err := ctx.InvokePackageRaw("${token}", ${args}, &rv, "", opts...)
			if err != nil {
				return ${outputType}{}, err
			}

			output := pulumi.ToOutput(rv).(${outputType})
			if secret {
				return pulumi.ToSecret(output).(${outputType}), nil
			}
			return output, nil
		}).(${outputType})
}

`
	} else {
		code = `
func ${fn}Output(ctx *pulumi.Context, opts ...pulumi.InvokeOption) ${outputType} {
	return pulumi.ToOutput(0).ApplyT(func(int) (${outputType}, error) {
		opts = ${internalModule}.PkgInvokeDefaultOpts(opts)
		var rv ${fn}Result
		secret, err := ctx.InvokePackageRaw("${token}", nil, &rv, "", opts...)
		if err != nil {
			return ${outputType}{}, err
		}

		output := pulumi.ToOutput(rv).(${outputType})
		if secret {
			return pulumi.ToSecret(output).(${outputType}), nil
		}
		return output, nil
	}).(${outputType})
}

`
	}

	code = strings.ReplaceAll(code, "${fn}", originalName)
	code = strings.ReplaceAll(code, "${outputType}", resultTypeName)
	code = strings.ReplaceAll(code, "${token}", f.Token)
	code = strings.ReplaceAll(code, "${args}", inputsVar)
	code = strings.ReplaceAll(code, "${internalModule}", pkg.internalModuleName)
	fmt.Fprint(w, code)

	if f.Inputs != nil {
		pkg.genInputArgsStruct(w, name+"Args", f.Inputs.InputShape, false /*emitGenericVariant*/)

		genInputImplementationWithArgs(w, genInputImplementationArgs{
			name:              name + "Args",
			receiverType:      name + "Args",
			elementType:       pkg.functionArgsTypeName(f),
			usingGenericTypes: useGenericTypes,
		})
	}
	if f.ReturnType != nil {
		if objectType, ok := f.ReturnType.(*schema.ObjectType); ok && objectType != nil {
			pkg.genOutputTypes(w, genOutputTypesArgs{
				t:      objectType,
				name:   originalResultTypeName,
				output: true,
			})
		}
	}

	// Assuming the file represented by `w` only has one function,
	// generate an `init()` for Output type init.
	initCode := `
func init() {
        pulumi.RegisterOutputType(${outputType}{})
}

`
	initCode = strings.ReplaceAll(initCode, "${outputType}", resultTypeName)
	fmt.Fprint(w, initCode)
}

type objectProperty struct {
	object   *schema.ObjectType
	property *schema.Property
}

// When computing the type name for a field of an object type, we must ensure that we do not generate invalid recursive
// struct types. A struct type T contains invalid recursion if the closure of its fields and its struct-typed fields'
// fields includes a field of type T. A few examples:
//
// Directly invalid:
//
//	type T struct {
//	    Invalid T
//	}
//
// Indirectly invalid:
//
//	type T struct {
//	    Invalid S
//	}
//
//	type S struct {
//	    Invalid T
//	}
//
// In order to avoid generating invalid struct types, we replace all references to types involved in a cyclical
// definition with *T. The examples above therefore become:
//
// (1)
//
//	type T struct {
//	    Valid *T
//	}
//
// (2)
//
//	type T struct {
//	    Valid *S
//	}
//
//	type S struct {
//	    Valid *T
//	}
//
// We do this using a rewriter that turns all fields involved in reference cycles into optional fields.
func rewriteCyclicField(rewritten codegen.Set, path []objectProperty, op objectProperty) {
	// If this property refers to an Input<> type, unwrap the type. This ensures that the plain and input shapes of an
	// object type remain identical.
	t := op.property.Type
	if inputType, isInputType := op.property.Type.(*schema.InputType); isInputType {
		t = inputType.ElementType
	}

	// If this property does not refer to an object type, it cannot be involved in a cycle. Skip it.
	objectType, isObjectType := t.(*schema.ObjectType)
	if !isObjectType {
		return
	}

	path = append(path, op)

	// Check the current path for cycles by crawling backwards until reaching the start of the path
	// or finding a property that is a member of the current object type.
	var cycle []objectProperty
	for i := len(path) - 1; i > 0; i-- {
		if path[i].object == objectType {
			cycle = path[i:]
			break
		}
	}

	// If the current path does not involve a cycle, recur into the current object type.
	if len(cycle) == 0 {
		rewriteCyclicFields(rewritten, path, objectType)
		return
	}

	// If we've found a cycle, mark each property involved in the cycle as optional.
	//
	// NOTE: this overestimates the set of properties that must be marked as optional. For example, in case (2) above,
	// only one of T.Invalid or S.Invalid needs to be marked as optional in order to break the cycle. However, choosing
	// a minimal set of properties that is also deterministic and resilient to changes in visit order is difficult and
	// seems to add little value.
	for _, p := range cycle {
		p.property.Type = codegen.OptionalType(p.property)
	}
}

func rewriteCyclicFields(rewritten codegen.Set, path []objectProperty, obj *schema.ObjectType) {
	if !rewritten.Has(obj) {
		rewritten.Add(obj)
		for _, property := range obj.Properties {
			rewriteCyclicField(rewritten, path, objectProperty{obj, property})
		}
	}
}

func rewriteCyclicObjectFields(pkg *schema.Package) {
	rewritten := codegen.Set{}
	for _, t := range pkg.Types {
		if obj, ok := t.(*schema.ObjectType); ok && !obj.IsInputShape() {
			rewriteCyclicFields(rewritten, nil, obj)
			rewriteCyclicFields(rewritten, nil, obj.InputShape)
		}
	}
}

func (pkg *pkgContext) genType(w io.Writer, obj *schema.ObjectType, usingGenericTypes bool) error {
	contract.Assertf(!obj.IsInputShape(), "Object type must not have input shape")
	if obj.IsOverlay {
		// This type is generated by the provider, so no further action is required.
		return nil
	}

	plainName := pkg.tokenToType(obj.Token)
	if !usingGenericTypes {
		pkg.genPlainType(w, plainName, obj.Comment, "", obj.Properties)
	} else {
		pkg.genGenericPlainType(w, plainName, obj.Comment, "", obj.Properties)
	}

	if !pkg.disableObjectDefaults {
		if err := pkg.genObjectDefaultFunc(w, plainName, obj.Properties, usingGenericTypes); err != nil {
			return err
		}
	}

	if err := pkg.genInputTypes(w, obj.InputShape, pkg.detailsForType(obj), usingGenericTypes); err != nil {
		return err
	}
	pkg.genOutputTypes(w, genOutputTypesArgs{
		t:                 obj,
		usingGenericTypes: usingGenericTypes,
	})
	return nil
}

func (pkg *pkgContext) addSuffixesToName(typ schema.Type, name string) []string {
	var names []string
	details := pkg.detailsForType(typ)
	if details.arrayInput {
		names = append(names, name+"ArrayInput")
	}
	if details.arrayOutput || details.arrayInput {
		names = append(names, name+"ArrayOutput")
	}
	if details.mapInput {
		names = append(names, name+"MapInput")
	}
	if details.mapOutput || details.mapInput {
		names = append(names, name+"MapOutput")
	}
	return names
}

type nestedTypeInfo struct {
	resolvedElementType string
	names               map[string]bool
}

func innerMostType(t schema.Type) schema.Type {
	switch t := t.(type) {
	case *schema.ArrayType:
		return innerMostType(t.ElementType)
	case *schema.MapType:
		return innerMostType(t.ElementType)
	case *schema.OptionalType:
		return innerMostType(t.ElementType)
	case *schema.InputType:
		return innerMostType(t.ElementType)
	default:
		return t
	}
}

// collectNestedCollectionTypes builds a deduped mapping of element types -> associated collection types.
// different shapes of known types can resolve to the same element type. by collecting types in one step and emitting types
// in a second step, we avoid collision and redeclaration.
func (pkg *pkgContext) collectNestedCollectionTypes(types map[string]*nestedTypeInfo, typ schema.Type) {
	var elementTypeName string
	var names []string
	switch t := typ.(type) {
	case *schema.ArrayType:
		// Builtins already cater to primitive arrays
		if schema.IsPrimitiveType(innerMostType(t.ElementType)) {
			return
		}
		elementTypeName = pkg.nestedTypeToType(t.ElementType)
		elementTypeName = strings.TrimSuffix(elementTypeName, "Args") + "Array"

		// We make sure that subsidiary elements are marked for array as well
		details := pkg.detailsForType(t)
		pkg.detailsForType(t.ElementType).markArray(details.arrayInput, details.arrayOutput)

		names = pkg.addSuffixesToName(t, elementTypeName)
		defer pkg.collectNestedCollectionTypes(types, t.ElementType)
	case *schema.MapType:
		// Builtins already cater to primitive maps
		if schema.IsPrimitiveType(innerMostType(t.ElementType)) {
			return
		}
		elementTypeName = pkg.nestedTypeToType(t.ElementType)
		elementTypeName = strings.TrimSuffix(elementTypeName, "Args") + "Map"

		// We make sure that subsidiary elements are marked for map as well
		details := pkg.detailsForType(t)
		pkg.detailsForType(t.ElementType).markMap(details.mapInput, details.mapOutput)

		names = pkg.addSuffixesToName(t, elementTypeName)
		defer pkg.collectNestedCollectionTypes(types, t.ElementType)
	default:
		return
	}
	nti, ok := types[elementTypeName]
	if !ok {
		nti = &nestedTypeInfo{
			names:               map[string]bool{},
			resolvedElementType: pkg.typeString(codegen.ResolvedType(typ)),
		}
		types[elementTypeName] = nti
	}
	for _, n := range names {
		nti.names[n] = true
	}
}

// genNestedCollectionTypes emits nested collection types given the deduped mapping of element types -> associated collection types.
// different shapes of known types can resolve to the same element type. by collecting types in one step and emitting types
// in a second step, we avoid collision and redeclaration.
func (pkg *pkgContext) genNestedCollectionTypes(w io.Writer, types map[string]*nestedTypeInfo) []string {
	var names []string

	// map iteration is unstable so sort items for deterministic codegen
	sortedElems := []string{}
	for k := range types {
		sortedElems = append(sortedElems, k)
	}
	sort.Strings(sortedElems)

	for _, elementTypeName := range sortedElems {
		info := types[elementTypeName]

		collectionTypes := []string{}
		for k := range info.names {
			collectionTypes = append(collectionTypes, k)
		}
		sort.Strings(collectionTypes)
		for _, name := range collectionTypes {
			names = append(names, name)
			switch {
			case strings.HasSuffix(name, "ArrayInput"):
				name = strings.TrimSuffix(name, "Input")
				fmt.Fprintf(w, "type %s []%sInput\n\n", name, elementTypeName)
				pkg.genInputImplementation(w, name, name, "[]"+info.resolvedElementType, false, false)

				pkg.genInputInterface(w, name)
			case strings.HasSuffix(name, "ArrayOutput"):
				pkg.genArrayOutput(w, strings.TrimSuffix(name, "ArrayOutput"), info.resolvedElementType)
			case strings.HasSuffix(name, "MapInput"):
				name = strings.TrimSuffix(name, "Input")
				fmt.Fprintf(w, "type %s map[string]%sInput\n\n", name, elementTypeName)
				pkg.genInputImplementation(w, name, name, "map[string]"+info.resolvedElementType, false, false)

				pkg.genInputInterface(w, name)
			case strings.HasSuffix(name, "MapOutput"):
				pkg.genMapOutput(w, strings.TrimSuffix(name, "MapOutput"), info.resolvedElementType)
			}
		}
	}

	return names
}

func (pkg *pkgContext) nestedTypeToType(typ schema.Type) string {
	switch t := codegen.UnwrapType(typ).(type) {
	case *schema.ArrayType:
		return pkg.nestedTypeToType(t.ElementType) + "Array"
	case *schema.MapType:
		return pkg.nestedTypeToType(t.ElementType) + "Map"
	case *schema.ObjectType:
		return pkg.resolveObjectType(t)
	case *schema.EnumType:
		return pkg.resolveEnumType(t)
	}
	return strings.TrimSuffix(pkg.tokenToType(typ.String()), "Args")
}

func (pkg *pkgContext) genTypeRegistrations(
	w io.Writer,
	objTypes []*schema.ObjectType,
	usingGenericTypes bool,
	types ...string,
) {
	fmt.Fprintf(w, "func init() {\n")

	// Input types.
	if !pkg.disableInputTypeRegistrations && !usingGenericTypes {
		for _, obj := range objTypes {
			if obj.IsOverlay {
				// This type is generated by the provider, so no further action is required.
				continue
			}
			name, details := pkg.tokenToType(obj.Token), pkg.detailsForType(obj)
			if details.input {
				fmt.Fprintf(w,
					"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sInput)(nil)).Elem(), %[1]sArgs{})\n", name)
			}
			if details.ptrInput {
				fmt.Fprintf(w,
					"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sPtrInput)(nil)).Elem(), %[1]sArgs{})\n", name)
			}
			if details.arrayInput && !pkg.names.Has(name+"Array") {
				fmt.Fprintf(w,
					"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sArrayInput)(nil)).Elem(), %[1]sArray{})\n", name)
			}
			if details.mapInput && !pkg.names.Has(name+"Map") {
				fmt.Fprintf(w,
					"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sMapInput)(nil)).Elem(), %[1]sMap{})\n", name)
			}
		}
		for _, t := range types {
			if strings.HasSuffix(t, "Input") {
				fmt.Fprintf(w, "\tpulumi.RegisterInputType(reflect.TypeOf((*%s)(nil)).Elem(), %s{})\n", t, strings.TrimSuffix(t, "Input"))
			}
		}
	}

	// Output types.
	for _, obj := range objTypes {
		if obj.IsOverlay {
			// This type is generated by the provider, so no further action is required.
			continue
		}
		name, details := pkg.tokenToType(obj.Token), pkg.detailsForType(obj)
		if details.output {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sOutput{})\n", name)
		}
		if details.ptrOutput && !usingGenericTypes {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sPtrOutput{})\n", name)
		}
		if details.arrayOutput && !usingGenericTypes {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sArrayOutput{})\n", name)
		}
		if details.mapOutput && !usingGenericTypes {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sMapOutput{})\n", name)
		}
	}
	for _, t := range types {
		if strings.HasSuffix(t, "Output") {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%s{})\n", t)
		}
	}

	fmt.Fprintf(w, "}\n")
}

func (pkg *pkgContext) genEnumRegistrations(w io.Writer) {
	fmt.Fprintf(w, "func init() {\n")
	// Register all input types
	if !pkg.disableInputTypeRegistrations {
		for _, e := range pkg.enums {
			// Enums are guaranteed to have at least one element when they are
			// bound into a schema.
			contract.Assertf(len(e.Elements) > 0, "Enum must have at least one element")
			name, details := pkg.tokenToEnum(e.Token), pkg.detailsForType(e)
			instance := fmt.Sprintf("%#v", e.Elements[0].Value)
			if details.input || details.ptrInput {
				fmt.Fprintf(w,
					"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sInput)(nil)).Elem(), %[1]s(%[2]s))\n",
					name, instance)
				fmt.Fprintf(w,
					"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sPtrInput)(nil)).Elem(), %[1]s(%[2]s))\n",
					name, instance)
			}
			if details.arrayInput {
				fmt.Fprintf(w,
					"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sArrayInput)(nil)).Elem(), %[1]sArray{})\n",
					name)
			}
			if details.mapInput {
				fmt.Fprintf(w,
					"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sMapInput)(nil)).Elem(), %[1]sMap{})\n",
					name)
			}
		}
	}
	// Register all output types
	for _, e := range pkg.enums {
		name, details := pkg.tokenToEnum(e.Token), pkg.detailsForType(e)
		if details.output || details.ptrOutput {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sOutput{})\n", name)
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sPtrOutput{})\n", name)
		}
		if details.arrayOutput {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sArrayOutput{})\n", name)
		}
		if details.mapOutput {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sMapOutput{})\n", name)
		}
	}
	fmt.Fprintf(w, "}\n\n")
}

func (pkg *pkgContext) genResourceRegistrations(
	w io.Writer,
	r *schema.Resource,
	generateResourceContainerTypes bool,
	usingGenericTypes bool,
) {
	name := disambiguatedResourceName(r, pkg)
	fmt.Fprintf(w, "func init() {\n")
	// Register input type
	if !pkg.disableInputTypeRegistrations && !usingGenericTypes {
		fmt.Fprintf(w,
			"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sInput)(nil)).Elem(), &%[1]s{})\n",
			name)
		if generateResourceContainerTypes && !r.IsProvider {
			fmt.Fprintf(w,
				"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sArrayInput)(nil)).Elem(), %[1]sArray{})\n",
				name)
			fmt.Fprintf(w,
				"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sMapInput)(nil)).Elem(), %[1]sMap{})\n",
				name)
		}
	}
	// Register all output types
	fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sOutput{})\n", name)
	for _, method := range r.Methods {
		var objectReturnType *schema.ObjectType
		if method.Function.ReturnType != nil {
			if objectType, ok := method.Function.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				objectReturnType = objectType
			}
		}

		if objectReturnType != nil {
			if pkg.liftSingleValueMethodReturns && len(objectReturnType.Properties) == 1 {
				fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%s%sResultOutput{})\n", cgstrings.Camel(name), Title(method.Name))
			} else {
				fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%s%sResultOutput{})\n", name, Title(method.Name))
			}
		}
	}

	if generateResourceContainerTypes && !r.IsProvider && !usingGenericTypes {
		fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sArrayOutput{})\n", name)
		fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sMapOutput{})\n", name)
	}
	fmt.Fprintf(w, "}\n\n")
}

func (pkg *pkgContext) getTypeImports(t schema.Type, recurse bool, importsAndAliases map[string]string, seen map[schema.Type]struct{}) {
	if _, ok := seen[t]; ok {
		return
	}
	seen[t] = struct{}{}

	// Import an external type with `token` and return true.
	// If the type is not external, return false.
	importExternal := func(token string) bool {
		if pkg.isExternalReference(t) {
			extPkgCtx, _ := pkg.contextForExternalReference(t)
			mod := extPkgCtx.tokenToPackage(token)
			imp := path.Join(extPkgCtx.importBasePath, mod)
			importsAndAliases[imp] = extPkgCtx.pkgImportAliases[imp]
			return true
		}
		return false
	}

	switch t := t.(type) {
	case *schema.OptionalType:
		pkg.getTypeImports(t.ElementType, recurse, importsAndAliases, seen)
	case *schema.InputType:
		pkg.getTypeImports(t.ElementType, recurse, importsAndAliases, seen)
	case *schema.EnumType:
		if importExternal(t.Token) {
			break
		}

		mod := pkg.tokenToPackage(t.Token)
		if mod != pkg.mod {
			p := path.Join(pkg.importBasePath, mod)
			importsAndAliases[path.Join(pkg.importBasePath, mod)] = pkg.pkgImportAliases[p]
		}
	case *schema.ArrayType:
		pkg.getTypeImports(t.ElementType, recurse, importsAndAliases, seen)
	case *schema.MapType:
		pkg.getTypeImports(t.ElementType, recurse, importsAndAliases, seen)
	case *schema.ObjectType:
		if importExternal(t.Token) {
			break
		}

		mod := pkg.tokenToPackage(t.Token)
		if mod != pkg.mod {
			p := path.Join(pkg.importBasePath, mod)
			importsAndAliases[path.Join(pkg.importBasePath, mod)] = pkg.pkgImportAliases[p]
		}

		if recurse {
			for _, p := range t.Properties {
				// We only recurse one level into objects, since we need to name
				// their properties but not the properties named in their
				// properties.
				pkg.getTypeImports(p.Type, false, importsAndAliases, seen)
			}
		}
	case *schema.ResourceType:
		if importExternal(t.Token) {
			break
		}
		mod := pkg.tokenToPackage(t.Token)
		if mod != pkg.mod {
			p := path.Join(pkg.importBasePath, mod)
			importsAndAliases[path.Join(pkg.importBasePath, mod)] = pkg.pkgImportAliases[p]
		}
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
			pkg.getTypeImports(e, recurse, importsAndAliases, seen)
		}
	}
}

func extractModulePath(extPkg schema.PackageReference) string {
	var vPath string
	version := extPkg.Version()
	name := extPkg.Name()
	if version != nil && version.Major > 1 {
		vPath = fmt.Sprintf("/v%d", version.Major)
	}

	// Default to example.com/pulumi-pkg if we have no other information.
	root := "example.com/pulumi-" + name
	// But if we have a publisher use that instead, assuming it's from github
	if extPkg.Publisher() != "" {
		root = fmt.Sprintf("github.com/%s/pulumi-%s", extPkg.Publisher(), name)
	}
	// And if we have a repository, use that instead of the publisher
	if extPkg.Repository() != "" {
		url, err := url.Parse(extPkg.Repository())
		if err == nil {
			// If there's any errors parsing the URL ignore it. Else use the host and path as go doesn't expect http://
			root = url.Host + url.Path
		}
	}

	// Support pack sdks write a go mod inside the go folder. Old legacy sdks would manually write a go.mod in the sdk
	// folder. This happened to mean that sdk/dotnet, sdk/nodejs etc where also considered part of the go sdk module.
	if extPkg.SupportPack() {
		return fmt.Sprintf("%s/sdk/go%s", root, vPath)
	}

	return fmt.Sprintf("%s/sdk%s", root, vPath)
}

func extractImportBasePath(extPkg schema.PackageReference) string {
	modpath := extractModulePath(extPkg)
	name := extPkg.Name()

	// Support pack sdks write a go mod inside the go folder. Old legacy sdks would manually write a go.mod in the sdk
	// folder. This happened to mean that sdk/dotnet, sdk/nodejs etc where also considered part of the go sdk module.
	if extPkg.SupportPack() {
		return fmt.Sprintf("%s/%s", modpath, goPackage(name))
	}

	return fmt.Sprintf("%s/go/%s", modpath, name)
}

func (pkg *pkgContext) getImports(member interface{}, importsAndAliases map[string]string) {
	seen := map[schema.Type]struct{}{}
	switch member := member.(type) {
	case *schema.ObjectType:
		pkg.getTypeImports(member, true, importsAndAliases, seen)
	case *schema.ResourceType:
		pkg.getTypeImports(member, true, importsAndAliases, seen)
	case *schema.Resource:
		for _, p := range member.Properties {
			pkg.getTypeImports(p.Type, false, importsAndAliases, seen)
		}
		for _, p := range member.InputProperties {
			pkg.getTypeImports(p.Type, false, importsAndAliases, seen)

			if p.IsRequired() {
				importsAndAliases["errors"] = ""
			}
		}
		for _, method := range member.Methods {
			if method.Function.Inputs != nil {
				for _, p := range method.Function.Inputs.InputShape.Properties {
					if p.Name == "__self__" {
						continue
					}
					pkg.getTypeImports(p.Type, false, importsAndAliases, seen)
				}
			}

			if method.Function.ReturnType != nil {
				if objectType, ok := method.Function.ReturnType.(*schema.ObjectType); ok && objectType != nil {
					for _, p := range objectType.Properties {
						pkg.getTypeImports(p.Type, false, importsAndAliases, seen)
					}
				} else if method.Function.ReturnTypePlain {
					pkg.getTypeImports(method.Function.ReturnType, false, importsAndAliases, seen)
				}
			}
		}
	case *schema.Function:
		if member.Inputs != nil {
			pkg.getTypeImports(member.Inputs, true, importsAndAliases, seen)
		}

		var returnType *schema.ObjectType
		if member.ReturnType != nil {
			if objectType, ok := member.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				returnType = objectType
			}
		}

		if returnType != nil {
			pkg.getTypeImports(returnType, true, importsAndAliases, seen)
		}
	case []*schema.Property:
		for _, p := range member {
			pkg.getTypeImports(p.Type, false, importsAndAliases, seen)
		}
	default:
		return
	}
}

func (pkg *pkgContext) genHeader(w io.Writer, goImports []string, importsAndAliases map[string]string, isUtil bool) {
	fmt.Fprintf(w, "// Code generated by %v DO NOT EDIT.\n", pkg.tool)
	fmt.Fprintf(w, "// *** WARNING: Do not edit by hand unless you're certain you know what you are doing! ***\n\n")

	var pkgName string
	if pkg.mod == "" {
		if isUtil {
			// we place pulumiVersion and pulumiUtilities in an ./internal folder
			// the name of the folder can be overridden by the schema
			// so we use the computed internalModuleName which defaults to "internal" if not set
			pkgName = pkg.internalModuleName
		} else {
			def, err := pkg.pkg.Definition()
			contract.AssertNoErrorf(err, "Could not retrieve definition")
			pkgName = packageName(def)
		}
	} else {
		pkgName = path.Base(pkg.mod)
	}

	fmt.Fprintf(w, "package %s\n\n", pkgName)

	var imports []string
	if len(importsAndAliases) > 0 {
		for k := range importsAndAliases {
			imports = append(imports, k)
		}
		sort.Strings(imports)

		for i, k := range imports {
			if alias := importsAndAliases[k]; alias != "" {
				imports[i] = fmt.Sprintf(`%s "%s"`, alias, k)
			}
		}
	}

	if len(goImports) > 0 {
		if len(imports) > 0 {
			goImports = append(goImports, "")
		}
		imports = append(goImports, imports...)
	}
	if len(imports) > 0 {
		fmt.Fprintf(w, "import (\n")
		for _, i := range imports {
			if i == "" {
				fmt.Fprintf(w, "\n")
			} else {
				if strings.Contains(i, `"`) { // Imports with aliases already include quotes.
					fmt.Fprintf(w, "\t%s\n", i)
				} else {
					fmt.Fprintf(w, "\t%q\n", i)
				}
			}
		}
		fmt.Fprintf(w, ")\n\n")
	}
}

func (pkg *pkgContext) genConfig(w io.Writer, variables []*schema.Property) error {
	importsAndAliases := map[string]string{
		"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config": "",
		"github.com/pulumi/pulumi/sdk/v3/go/pulumi":        "",
	}
	importsAndAliases[path.Join(pkg.importBasePath, pkg.internalModuleName)] = ""
	pkg.genHeader(w, nil, importsAndAliases, false /* isUtil */)

	// in case we're not using the internal package, assign to a blank var
	fmt.Fprintf(w, "var _ = %s.GetEnvOrDefault\n", pkg.internalModuleName)

	for _, p := range variables {
		getfunc := "Get"

		var getType string
		var funcType string
		switch codegen.UnwrapType(p.Type) {
		case schema.BoolType:
			getType, funcType = "bool", "Bool"
		case schema.IntType:
			getType, funcType = "int", "Int"
		case schema.NumberType:
			getType, funcType = "float64", "Float64"
		default:
			getType, funcType = "string", ""
		}

		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, false)
		configKey := fmt.Sprintf("\"%s:%s\"", pkg.pkg.Name(), cgstrings.Camel(p.Name))

		fmt.Fprintf(w, "func Get%s(ctx *pulumi.Context) %s {\n", Title(p.Name), getType)
		if p.DefaultValue != nil {
			fmt.Fprintf(w, "\tv, err := config.Try%s(ctx, %s)\n", funcType, configKey)
			fmt.Fprintf(w, "\tif err == nil {\n")
			fmt.Fprintf(w, "\t\treturn v\n")
			fmt.Fprintf(w, "\t}\n")

			fmt.Fprintf(w, "\tvar value %s\n", getType)
			err := pkg.setDefaultValue(w, p.DefaultValue, codegen.UnwrapType(p.Type), func(w io.Writer, dv string) error {
				_, err := fmt.Fprintf(w, "\tvalue = %s\n", dv)
				return err
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(w, "\treturn value\n")
		} else {
			fmt.Fprintf(w, "\treturn config.%s%s(ctx, %s)\n", getfunc, funcType, configKey)
		}
		fmt.Fprintf(w, "}\n")
	}

	return nil
}

// genResourceModule generates a ResourceModule definition and the code to register an instance thereof with the
// Pulumi runtime. The generated ResourceModule supports the deserialization of resource references into fully-
// hydrated Resource instances. If this is the root module, this function also generates a ResourcePackage
// definition and its registration to support rehydrating providers.
func (pkg *pkgContext) genResourceModule(w io.Writer) error {
	contract.Assertf(len(pkg.resources) != 0, "Package must have at least one resource")
	allResourcesAreOverlays := true
	for _, r := range pkg.resources {
		if !r.IsOverlay {
			allResourcesAreOverlays = false
			break
		}
	}
	if allResourcesAreOverlays {
		// If all resources in this module are overlays, skip further code generation.
		return nil
	}

	imports := map[string]string{
		"github.com/blang/semver":                   "",
		"github.com/pulumi/pulumi/sdk/v3/go/pulumi": "",
	}
	imports[path.Join(pkg.importBasePath, pkg.internalModuleName)] = ""

	// If there are any internal dependencies, include them as blank imports.
	def, err := pkg.pkg.Definition()
	if err != nil {
		return err
	}
	if goInfo, ok := def.Language["go"].(GoPackageInfo); ok {
		for _, dep := range goInfo.InternalDependencies {
			imports[dep] = "_"
		}
	}

	pkg.genHeader(w, []string{"fmt"}, imports, false /* isUtil */)

	var provider *schema.Resource
	registrations := codegen.StringSet{}
	if providerOnly := len(pkg.resources) == 1 && pkg.resources[0].IsProvider; providerOnly {
		provider = pkg.resources[0]
	} else {
		fmt.Fprintf(w, "type module struct {\n")
		fmt.Fprintf(w, "\tversion semver.Version\n")
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (m *module) Version() semver.Version {\n")
		fmt.Fprintf(w, "\treturn m.version\n")
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (m *module) Construct(ctx *pulumi.Context, name, typ, urn string) (r pulumi.Resource, err error) {\n")
		fmt.Fprintf(w, "\tswitch typ {\n")
		for _, r := range pkg.resources {
			if r.IsOverlay {
				// This resource code is generated by the provider, so no further action is required.
				continue
			}
			if r.IsProvider {
				contract.Assertf(provider == nil, "Provider must not be specified for Provider resources")
				provider = r
				continue
			}

			registrations.Add(tokenToModule(r.Token))
			fmt.Fprintf(w, "\tcase %q:\n", r.Token)
			fmt.Fprintf(w, "\t\tr = &%s{}\n", disambiguatedResourceName(r, pkg))
		}
		fmt.Fprintf(w, "\tdefault:\n")
		fmt.Fprintf(w, "\t\treturn nil, fmt.Errorf(\"unknown resource type: %%s\", typ)\n")
		fmt.Fprintf(w, "\t}\n\n")
		fmt.Fprintf(w, "\terr = ctx.RegisterResource(typ, name, nil, r, pulumi.URN_(urn))\n")
		fmt.Fprintf(w, "\treturn\n")
		fmt.Fprintf(w, "}\n\n")
	}

	if provider != nil {
		fmt.Fprintf(w, "type pkg struct {\n")
		fmt.Fprintf(w, "\tversion semver.Version\n")
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (p *pkg) Version() semver.Version {\n")
		fmt.Fprintf(w, "\treturn p.version\n")
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (p *pkg) ConstructProvider(ctx *pulumi.Context, name, typ, urn string) (pulumi.ProviderResource, error) {\n")
		fmt.Fprintf(w, "\tif typ != \"pulumi:providers:%s\" {\n", pkg.pkg.Name())
		fmt.Fprintf(w, "\t\treturn nil, fmt.Errorf(\"unknown provider type: %%s\", typ)\n")
		fmt.Fprintf(w, "\t}\n\n")
		fmt.Fprintf(w, "\tr := &Provider{}\n")
		fmt.Fprintf(w, "\terr := ctx.RegisterResource(typ, name, nil, r, pulumi.URN_(urn))\n")
		fmt.Fprintf(w, "\treturn r, err\n")
		fmt.Fprintf(w, "}\n\n")
	}

	fmt.Fprintf(w, "func init() {\n")

	fmt.Fprintf(w, "\tversion, err := %s.PkgVersion()\n", pkg.internalModuleName)
	// To avoid breaking compatibility, we don't change the function
	// signature. We instead just ignore the error.
	fmt.Fprintf(w, "\tif err != nil {\n")
	fmt.Fprintf(w, "\t\tversion = semver.Version{Major: 1}\n")
	fmt.Fprintf(w, "\t}\n")
	if len(registrations) > 0 {
		for _, mod := range registrations.SortedValues() {
			fmt.Fprintf(w, "\tpulumi.RegisterResourceModule(\n")
			fmt.Fprintf(w, "\t\t%q,\n", pkg.pkg.Name())
			fmt.Fprintf(w, "\t\t%q,\n", mod)
			fmt.Fprintf(w, "\t\t&module{version},\n")
			fmt.Fprintf(w, "\t)\n")
		}
	}
	if provider != nil {
		fmt.Fprintf(w, "\tpulumi.RegisterResourcePackage(\n")
		fmt.Fprintf(w, "\t\t%q,\n", pkg.pkg.Name())
		fmt.Fprintf(w, "\t\t&pkg{version},\n")
		fmt.Fprintf(w, "\t)\n")
	}
	_, err = fmt.Fprintf(w, "}\n")
	return err
}

// generatePackageContextMap groups resources, types, and functions into Go packages.
func generatePackageContextMap(tool string, pkg schema.PackageReference, goInfo GoPackageInfo, externalPkgs *Cache) (map[string]*pkgContext, error) {
	packages := map[string]*pkgContext{}

	// Share the cache
	if externalPkgs == nil {
		externalPkgs = globalCache
	}

	getPkg := func(mod string) *pkgContext {
		pack, ok := packages[mod]
		if !ok {
			internalModuleName := "internal"
			if goInfo.InternalModuleName != "" {
				internalModuleName = goInfo.InternalModuleName
			}

			importBasePath := goInfo.ImportBasePath
			if importBasePath == "" {
				// Default to a path based on the package name.
				importBasePath = extractImportBasePath(pkg)
			}

			pack = &pkgContext{
				pkg:                           pkg,
				mod:                           mod,
				importBasePath:                importBasePath,
				rootPackageName:               goInfo.RootPackageName,
				typeDetails:                   map[schema.Type]*typeDetails{},
				names:                         codegen.NewStringSet(),
				schemaNames:                   codegen.NewStringSet(),
				renamed:                       map[string]string{},
				duplicateTokens:               map[string]bool{},
				functionNames:                 map[*schema.Function]string{},
				tool:                          tool,
				modToPkg:                      goInfo.ModuleToPackage,
				pkgImportAliases:              goInfo.PackageImportAliases,
				packages:                      packages,
				liftSingleValueMethodReturns:  goInfo.LiftSingleValueMethodReturns,
				disableInputTypeRegistrations: goInfo.DisableInputTypeRegistrations,
				disableObjectDefaults:         goInfo.DisableObjectDefaults,
				internalModuleName:            internalModuleName,
				externalPackages:              externalPkgs,
			}
			packages[mod] = pack
		}
		return pack
	}

	getPkgFromToken := func(token string) *pkgContext {
		return getPkg(tokenToPackage(pkg, goInfo.ModuleToPackage, token))
	}

	var getPkgFromType func(schema.Type) *pkgContext
	getPkgFromType = func(typ schema.Type) *pkgContext {
		switch t := codegen.UnwrapType(typ).(type) {
		case *schema.ArrayType:
			return getPkgFromType(t.ElementType)
		case *schema.MapType:
			return getPkgFromType(t.ElementType)
		case *schema.ObjectType:
			return getPkgFromToken(t.Token)
		case *schema.EnumType:
			return getPkgFromToken(t.Token)
		default:
			return getPkgFromToken(t.String())
		}
	}

	config, err := pkg.Config()
	if err != nil {
		return nil, err
	}
	if len(config) > 0 {
		_ = getPkg("config")
	}

	// For any optional properties, we must generate a pointer type for the corresponding property type.
	// In addition, if the optional property's type is itself an object type, we also need to generate pointer
	// types corresponding to all of it's nested properties, as our accessor methods will lift `nil` into
	// those nested types.
	var populateDetailsForPropertyTypes func(seen codegen.StringSet, props []*schema.Property, optional, input, output bool)
	var populateDetailsForTypes func(seen codegen.StringSet, schemaType schema.Type, optional, input, output bool)

	seenKey := func(t schema.Type, optional, input, output bool) string {
		var key string
		switch t := t.(type) {
		case *schema.ObjectType:
			key = t.Token
		case *schema.EnumType:
			key = t.Token
		default:
			key = t.String()
		}
		if optional {
			key += ",optional"
		}
		if input {
			key += ",input"
		}
		if output {
			key += ",output"
		}
		return key
	}

	populateDetailsForPropertyTypes = func(seen codegen.StringSet, props []*schema.Property, optional, input, output bool) {
		for _, p := range props {
			if obj, ok := codegen.UnwrapType(p.Type).(*schema.ObjectType); ok && p.Plain {
				pkg := getPkgFromToken(obj.Token)
				details := pkg.detailsForType(obj)
				details.mark(true, false)
				input = true
				_, hasOptional := p.Type.(*schema.OptionalType)
				details.markPtr(hasOptional, false)
			}
			populateDetailsForTypes(seen, p.Type, !p.IsRequired() || optional, input, output)
		}
	}

	populateDetailsForTypes = func(seen codegen.StringSet, schemaType schema.Type, optional, input, output bool) {
		key := seenKey(schemaType, optional, input, output)
		if seen.Has(key) {
			return
		}
		seen.Add(key)

		switch typ := schemaType.(type) {
		case *schema.InputType:
			populateDetailsForTypes(seen, typ.ElementType, optional, true, false)
		case *schema.OptionalType:
			populateDetailsForTypes(seen, typ.ElementType, true, input, output)
		case *schema.ObjectType:
			pkg := getPkgFromToken(typ.Token)
			pkg.detailsForType(typ).mark(input || goInfo.GenerateExtraInputTypes, output)

			if optional {
				pkg.detailsForType(typ).markPtr(input || goInfo.GenerateExtraInputTypes, output)
			}

			pkg.schemaNames.Add(tokenToName(typ.Token))

			populateDetailsForPropertyTypes(seen, typ.Properties, optional, input, output)
		case *schema.UnionType:
			for _, e := range typ.ElementTypes {
				populateDetailsForTypes(seen, e, optional, input, output)
			}
		case *schema.EnumType:
			pkg := getPkgFromToken(typ.Token)
			pkg.detailsForType(typ).mark(input || goInfo.GenerateExtraInputTypes, output)

			if optional {
				pkg.detailsForType(typ).markPtr(input || goInfo.GenerateExtraInputTypes, output)
			}

			pkg.schemaNames.Add(tokenToName(typ.Token))
		case *schema.ArrayType:
			details := getPkgFromType(typ.ElementType).detailsForType(codegen.UnwrapType(typ.ElementType))
			details.markArray(input || goInfo.GenerateExtraInputTypes, output)
			populateDetailsForTypes(seen, typ.ElementType, false, input, output)
		case *schema.MapType:
			details := getPkgFromType(typ.ElementType).detailsForType(codegen.UnwrapType(typ.ElementType))
			details.markMap(input || goInfo.GenerateExtraInputTypes, output)
			populateDetailsForTypes(seen, typ.ElementType, false, input, output)
		}
	}

	// Rewrite cyclic types. See the docs on rewriteCyclicFields for the motivation.
	def, err := pkg.Definition()
	if err != nil {
		return nil, err
	}
	rewriteCyclicObjectFields(def)

	// Use a string set to track object types that have already been processed.
	// This avoids recursively processing the same type. For example, in the
	// Kubernetes package, JSONSchemaProps have properties whose type is itself.
	seenMap := codegen.NewStringSet()
	for _, t := range def.Types {
		switch typ := t.(type) {
		case *schema.ArrayType:
			details := getPkgFromType(typ.ElementType).detailsForType(codegen.UnwrapType(typ.ElementType))
			details.markArray(goInfo.GenerateExtraInputTypes, false)
		case *schema.MapType:
			details := getPkgFromType(typ.ElementType).detailsForType(codegen.UnwrapType(typ.ElementType))
			details.markMap(goInfo.GenerateExtraInputTypes, false)
		case *schema.ObjectType:
			pkg := getPkgFromToken(typ.Token)
			if !typ.IsInputShape() {
				pkg.types = append(pkg.types, typ)
			}
			populateDetailsForTypes(seenMap, typ, false, false, false)
		case *schema.EnumType:
			if !typ.IsOverlay {
				pkg := getPkgFromToken(typ.Token)
				pkg.enums = append(pkg.enums, typ)

				populateDetailsForTypes(seenMap, typ, false, false, false)
			}
		}
	}

	resSeen := map[string]bool{}
	typeSeen := map[string]bool{}

	// compute set of names generated by a resource
	// handling any potential collisions via remapping along the way
	scanResource := func(r *schema.Resource) {
		if resSeen[strings.ToLower(r.Token)] {
			return
		}
		resSeen[strings.ToLower(r.Token)] = true
		pkg := getPkgFromToken(r.Token)
		pkg.resources = append(pkg.resources, r)
		pkg.schemaNames.Add(tokenToName(r.Token))

		getNames := func(suffix string) []string {
			names := []string{}
			names = append(names, rawResourceName(r)+suffix)
			names = append(names, rawResourceName(r)+suffix+"Input")
			names = append(names, rawResourceName(r)+suffix+"Output")
			names = append(names, rawResourceName(r)+suffix+"Args")
			names = append(names, cgstrings.Camel(rawResourceName(r))+suffix+"Args")
			names = append(names, "New"+rawResourceName(r)+suffix)
			if !r.IsProvider && !r.IsComponent {
				names = append(names, rawResourceName(r)+suffix+"State")
				names = append(names, cgstrings.Camel(rawResourceName(r))+suffix+"State")
				names = append(names, "Get"+rawResourceName(r)+suffix)
			}
			if goInfo.GenerateResourceContainerTypes && !r.IsProvider {
				names = append(names, rawResourceName(r)+suffix+"Array")
				names = append(names, rawResourceName(r)+suffix+"Map")
			}
			return names
		}

		suffixes := []string{"", "Resource", "Res"}
		suffix := ""
		suffixIndex := 0
		canGenerate := false

		for !canGenerate && suffixIndex <= len(suffixes) {
			suffix = suffixes[suffixIndex]
			candidates := getNames(suffix)
			conflict := false
			for _, c := range candidates {
				if pkg.names.Has(c) {
					conflict = true
				}
			}
			if !conflict {
				canGenerate = true
				break
			}

			suffixIndex++
		}

		if !canGenerate {
			panic("unable to generate Go SDK, schema has unresolvable overlapping resource: " + rawResourceName(r))
		}

		names := getNames(suffix)
		originalNames := getNames("")
		for i, n := range names {
			pkg.names.Add(n)
			if suffix != "" {
				pkg.renamed[originalNames[i]] = names[i]
			}
		}

		populateDetailsForPropertyTypes(seenMap, r.InputProperties, r.IsProvider, false, false)
		populateDetailsForPropertyTypes(seenMap, r.Properties, r.IsProvider, false, true)

		if r.StateInputs != nil {
			populateDetailsForPropertyTypes(seenMap, r.StateInputs.Properties,
				r.IsProvider, false /*input*/, false /*output*/)
		}

		for _, method := range r.Methods {
			if method.Function.Inputs != nil {
				pkg.names.Add(rawResourceName(r) + Title(method.Name) + "Args")
			}
			if method.Function.ReturnType != nil {
				if _, ok := method.Function.ReturnType.(*schema.ObjectType); ok {
					pkg.names.Add(rawResourceName(r) + Title(method.Name) + "Result")
				}
			}
		}
	}

	scanResource(def.Provider)
	for _, r := range def.Resources {
		scanResource(r)
	}

	// compute set of names generated by a type
	// handling any potential collisions via remapping along the way
	scanType := func(t schema.Type) {
		getNames := func(name, suffix string) []string {
			return []string{name + suffix, name + suffix + "Input", name + suffix + "Output"}
		}

		switch t := t.(type) {
		case *schema.ObjectType:
			pkg := getPkgFromToken(t.Token)
			// maintain support for duplicate tokens for types and resources in Kubernetes
			if resSeen[strings.ToLower(t.Token)] {
				pkg.duplicateTokens[strings.ToLower(t.Token)] = true
			}
			if typeSeen[strings.ToLower(t.Token)] {
				return
			}
			typeSeen[strings.ToLower(t.Token)] = true

			name := pkg.tokenToType(t.Token)
			suffixes := []string{"", "Type", "Typ"}
			suffix := ""
			suffixIndex := 0
			canGenerate := false

			for !canGenerate && suffixIndex <= len(suffixes) {
				suffix = suffixes[suffixIndex]
				candidates := getNames(name, suffix)
				conflict := false
				for _, c := range candidates {
					if pkg.names.Has(c) {
						conflict = true
					}
				}
				if !conflict {
					canGenerate = true
					break
				}

				suffixIndex++
			}

			if !canGenerate {
				panic("unable to generate Go SDK, schema has unresolvable overlapping type: " + name)
			}

			names := getNames(name, suffix)
			originalNames := getNames(name, "")
			for i, n := range names {
				pkg.names.Add(n)
				if suffix != "" {
					pkg.renamed[originalNames[i]] = names[i]
				}
			}
		case *schema.EnumType:
			pkg := getPkgFromToken(t.Token)
			if resSeen[t.Token] {
				pkg.duplicateTokens[strings.ToLower(t.Token)] = true
			}
			if typeSeen[t.Token] {
				return
			}
			typeSeen[t.Token] = true

			name := pkg.tokenToEnum(t.Token)
			suffixes := []string{"", "Enum"}
			suffix := ""
			suffixIndex := 0
			canGenerate := false

			for !canGenerate && suffixIndex <= len(suffixes) {
				suffix = suffixes[suffixIndex]
				candidates := getNames(name, suffix)
				conflict := false
				for _, c := range candidates {
					if pkg.names.Has(c) {
						conflict = true
					}
				}
				if !conflict {
					canGenerate = true
					break
				}

				suffixIndex++
			}

			if !canGenerate {
				panic("unable to generate Go SDK, schema has unresolvable overlapping type: " + name)
			}

			names := getNames(name, suffix)
			originalNames := getNames(name, "")
			for i, n := range names {
				pkg.names.Add(n)
				if suffix != "" {
					pkg.renamed[originalNames[i]] = names[i]
				}
			}
		default:
			return
		}
	}

	for _, t := range def.Types {
		scanType(t)
	}

	// For fnApply function versions, we need to register any
	// input or output property type metadata, in case they have
	// types used in array or pointer element positions.
	if !goInfo.DisableFunctionOutputVersions || goInfo.GenerateExtraInputTypes {
		for _, f := range def.Functions {
			if f.NeedsOutputVersion() || goInfo.GenerateExtraInputTypes {
				optional := false
				if f.Inputs != nil {
					populateDetailsForPropertyTypes(seenMap, f.Inputs.InputShape.Properties, optional, false, false)
				}
				if f.ReturnType != nil {
					populateDetailsForTypes(seenMap, f.ReturnType, optional, false, true)
				}
			}
		}
	}

	for _, f := range def.Functions {
		if f.IsMethod {
			continue
		}

		pkg := getPkgFromToken(f.Token)
		pkg.functions = append(pkg.functions, f)

		name := tokenToName(f.Token)

		if pkg.names.Has(name) ||
			pkg.names.Has(name+"Args") ||
			pkg.names.Has(name+"Result") {
			switch {
			case strings.HasPrefix(name, "New"):
				name = "Create" + name[3:]
			case strings.HasPrefix(name, "Get"):
				name = "Lookup" + name[3:]
			}
		}
		pkg.names.Add(name)
		pkg.functionNames[f] = name

		if f.Inputs != nil && !f.MultiArgumentInputs {
			pkg.names.Add(name + "Args")
		}

		if f.ReturnType != nil {
			if objectType, ok := f.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				pkg.names.Add(name + "Result")
			}
		}
	}

	return packages, nil
}

// LanguageResource is derived from the schema and can be used by downstream codegen.
type LanguageResource struct {
	*schema.Resource

	Alias   string // The package alias (e.g. appsv1)
	Name    string // The resource name (e.g. Deployment)
	Package string // The package name (e.g. github.com/pulumi/pulumi-kubernetes/sdk/v2/go/kubernetes/apps/v1)
}

// LanguageResources returns a map of resources that can be used by downstream codegen. The map
// key is the resource schema token.
func LanguageResources(tool string, pkg *schema.Package) (map[string]LanguageResource, error) {
	resources := map[string]LanguageResource{}

	if err := pkg.ImportLanguages(map[string]schema.Language{"go": Importer}); err != nil {
		return nil, err
	}

	var goPkgInfo GoPackageInfo
	if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
		goPkgInfo = goInfo
	}
	packages, err := generatePackageContextMap(tool, pkg.Reference(), goPkgInfo, globalCache)
	if err != nil {
		return nil, err
	}

	// emit each package
	pkgMods := slice.Prealloc[string](len(packages))
	for mod := range packages {
		pkgMods = append(pkgMods, mod)
	}
	sort.Strings(pkgMods)

	for _, mod := range pkgMods {
		if mod == "" {
			continue
		}
		pkg := packages[mod]

		for _, r := range pkg.resources {
			if r.IsOverlay {
				// This resource code is generated by the provider, so no further action is required.
				continue
			}

			packagePath := path.Join(goPkgInfo.ImportBasePath, pkg.mod)
			resources[r.Token] = LanguageResource{
				Resource: r,
				Alias:    goPkgInfo.PackageImportAliases[packagePath],
				Name:     tokenToName(r.Token),
				Package:  packagePath,
			}
		}
	}

	return resources, nil
}

// packageRoot is the relative root file for go code. That means that every go
// source file should be under this root. For example:
//
// root = aws => sdk/go/aws/*.go
func packageRoot(pkg schema.PackageReference) (string, error) {
	def, err := pkg.Definition()
	if err != nil {
		return "", err
	}
	var info GoPackageInfo
	if goInfo, ok := def.Language["go"].(GoPackageInfo); ok {
		info = goInfo
	}
	if info.RootPackageName != "" {
		// package structure is flat
		return "", nil
	}
	if info.ImportBasePath != "" {
		return path.Base(info.ImportBasePath), nil
	}
	return goPackage(pkg.Name()), nil
}

// packageName is the go package name for the generated package.
func packageName(pkg *schema.Package) string {
	var info GoPackageInfo
	if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
		info = goInfo
	}
	if info.RootPackageName != "" {
		return info.RootPackageName
	}
	root, err := packageRoot(pkg.Reference())
	contract.AssertNoErrorf(err, "We generated the ref from a pkg, so we know its a valid ref")
	return goPackage(root)
}

func GeneratePackage(tool string,
	pkg *schema.Package,
	localDependencies map[string]string,
) (map[string][]byte, error) {
	if err := pkg.ImportLanguages(map[string]schema.Language{"go": Importer}); err != nil {
		return nil, err
	}

	var goPkgInfo GoPackageInfo
	if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
		goPkgInfo = goInfo
	}

	if goPkgInfo.ImportBasePath == "" {
		goPkgInfo.ImportBasePath = extractImportBasePath(pkg.Reference())
	}

	packages, err := generatePackageContextMap(tool, pkg.Reference(), goPkgInfo, NewCache())
	if err != nil {
		return nil, err
	}

	// emit each package
	pkgMods := slice.Prealloc[string](len(packages))
	for mod := range packages {
		pkgMods = append(pkgMods, mod)
	}
	sort.Strings(pkgMods)

	name := packageName(pkg)
	pathPrefix, err := packageRoot(pkg.Reference())
	if err != nil {
		return nil, err
	}

	files := codegen.Fs{}

	// Generate pulumi-plugin.json
	pulumiPlugin := &plugin.PulumiPluginJSON{
		Resource: true,
		Name:     pkg.Name,
		Server:   pkg.PluginDownloadURL,
	}
	if goPkgInfo.RespectSchemaVersion && pkg.Version != nil {
		pulumiPlugin.Version = pkg.Version.String()
	}

	if pkg.Parameterization != nil {
		// For a parameterized package the plugin name/version is from the base provider information, not the
		// top-level package name/version.
		pulumiPlugin.Parameterization = &plugin.PulumiParameterizationJSON{
			Name:    pulumiPlugin.Name,
			Version: pulumiPlugin.Version,
			Value:   pkg.Parameterization.Parameter,
		}
		pulumiPlugin.Name = pkg.Parameterization.BaseProvider.Name
		pulumiPlugin.Version = pkg.Parameterization.BaseProvider.Version.String()
	}

	pulumiPluginJSON, err := pulumiPlugin.JSON()
	if err != nil {
		return nil, fmt.Errorf("Failed to format pulumi-plugin.json: %w", err)
	}
	files.Add(path.Join(pathPrefix, "pulumi-plugin.json"), pulumiPluginJSON)

	setFileContent := func(root, relPath, contents string) {
		relPath = path.Join(root, relPath)

		// Run Go formatter on the code before saving to disk
		formattedSource, err := format.Source([]byte(contents))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid content:\n%s\n%s\n", relPath, contents)
			panic(fmt.Errorf("invalid Go source code:\n\n%s\n: %w", relPath, err))
		}

		files.Add(relPath, formattedSource)
	}

	if goPkgInfo.Generics == "" {
		// default is emitting the non-generic variant only
		goPkgInfo.Generics = GenericsSettingNone
	}

	emitOnlyGenericVariant := goPkgInfo.Generics == GenericsSettingGenericsOnly
	emitOnlyLegacyVariant := goPkgInfo.Generics == GenericsSettingNone

	setFile := func(relPath, contents string) {
		if emitOnlyGenericVariant {
			// if we only want the generic variant to be emitted
			// skip generating the default "legacy" variant
			return
		}

		setFileContent(pathPrefix, relPath, contents)
	}

	setGenericVariantFile := func(relPath, contents string) {
		if emitOnlyLegacyVariant {
			// if we only want the legacy variant to be emitted
			// skip generating the generic variant
			return
		}

		root := path.Join(pathPrefix, "x")
		if emitOnlyGenericVariant {
			// if we only want the generic variant to be emitted
			// emit it at the root of the package as the default package
			root = pathPrefix
		}
		setFileContent(root, relPath, contents)
	}

	for _, mod := range pkgMods {
		pkg := packages[mod]

		// Config, description
		switch mod {
		case "":
			buffer := &bytes.Buffer{}
			if pkg.pkg.Description() != "" {
				printComment(buffer, pkg.pkg.Description(), false)
			} else {
				fmt.Fprintf(buffer, "// Package %[1]s exports types, functions, subpackages for provisioning %[1]s resources.\n", name)
			}
			fmt.Fprintf(buffer, "package %s\n", name)

			setFile(path.Join(mod, "doc.go"), buffer.String())
			setGenericVariantFile(path.Join(mod, "doc.go"), buffer.String())

			// Version
			versionBuf := &bytes.Buffer{}
			importsAndAliases := map[string]string{}
			pkg.genHeader(versionBuf, []string{"github.com/blang/semver"}, importsAndAliases, true /* isUtil */)
			err = pkg.GenVersionFile(versionBuf)
			if err != nil {
				return nil, err
			}

			versionFilePath := pkg.internalModuleName + "/pulumiVersion.go"
			setFile(path.Join(mod, versionFilePath), versionBuf.String())
			if emitOnlyGenericVariant {
				setGenericVariantFile(path.Join(mod, versionFilePath), versionBuf.String())
			}

		case "config":
			config, err := pkg.pkg.Config()
			if err != nil {
				return nil, err
			}
			if len(config) > 0 {
				buffer := &bytes.Buffer{}
				if err := pkg.genConfig(buffer, config); err != nil {
					return nil, err
				}

				configFilePath := path.Join(mod, "config.go")
				setFile(configFilePath, buffer.String())
				setGenericVariantFile(configFilePath, buffer.String())
			}
		}

		// Resources
		for _, resource := range pkg.resources {
			if resource.IsOverlay {
				// This resource code is generated by the provider, so no further action is required.
				continue
			}

			importsAndAliases := map[string]string{}
			pkg.getImports(resource, importsAndAliases)
			importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumi"] = ""
			importsAndAliases[path.Join(pkg.importBasePath, pkg.internalModuleName)] = ""
			if goPkgInfo.Generics == GenericsSettingSideBySide {
				importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumix"] = ""
			}

			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, []string{"context", "reflect"}, importsAndAliases, false /* isUtil */)

			if err := pkg.genResource(
				buffer,
				resource,
				goPkgInfo.GenerateResourceContainerTypes,
				false /* useGenericVariant */); err != nil {
				return nil, err
			}

			resourceFilePath := path.Join(mod, cgstrings.Camel(rawResourceName(resource))+".go")
			setFile(resourceFilePath, buffer.String())

			genericVariantBuffer := &bytes.Buffer{}
			importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumix"] = ""
			pkg.genHeader(genericVariantBuffer, []string{"context", "reflect"}, importsAndAliases, false /* isUtil */)
			if err := pkg.genResource(
				genericVariantBuffer,
				resource,
				goPkgInfo.GenerateResourceContainerTypes,
				true /* useGenericVariant */); err != nil {
				return nil, err
			}

			setGenericVariantFile(resourceFilePath, genericVariantBuffer.String())
		}

		// Functions
		for _, f := range pkg.functions {
			if f.IsOverlay {
				// This function code is generated by the provider, so no further action is required.
				continue
			}

			fileName := path.Join(mod, cgstrings.Camel(tokenToName(f.Token))+".go")
			code, err := pkg.genFunctionCodeFile(f)
			if err != nil {
				return nil, err
			}
			setFile(fileName, code)

			genericCodeVariant, err := pkg.genGenericVariantFunctionCodeFile(f)
			if err != nil {
				return nil, err
			}
			setGenericVariantFile(fileName, genericCodeVariant)
		}

		knownTypes := make(map[schema.Type]struct{}, len(pkg.typeDetails))
		for t := range pkg.typeDetails {
			knownTypes[t] = struct{}{}
		}

		// Enums
		if len(pkg.enums) > 0 {
			hasOutputs, imports := false, map[string]string{}
			for _, e := range pkg.enums {
				pkg.getImports(e, imports)
				hasOutputs = hasOutputs || pkg.detailsForType(e).hasOutputs()
			}
			var goImports []string
			if hasOutputs {
				goImports = []string{"context", "reflect"}
				imports["github.com/pulumi/pulumi/sdk/v3/go/pulumi"] = ""
				if goPkgInfo.Generics != GenericsSettingNone {
					imports["github.com/pulumi/pulumi/sdk/v3/go/pulumix"] = ""
				}
			}

			buffer := &bytes.Buffer{}
			genericVariantBuffer := &bytes.Buffer{}
			pkg.genHeader(buffer, goImports, imports, false /* isUtil */)
			// we do not need any imports for the generic variant
			pkg.genHeader(genericVariantBuffer, []string{}, map[string]string{}, false /* isUtil */)

			for _, e := range pkg.enums {
				// generate enums for legacy variant
				if err := pkg.genEnum(buffer, e, false); err != nil {
					return nil, err
				}

				// generate enums for generic variant
				if err := pkg.genEnum(genericVariantBuffer, e, true); err != nil {
					return nil, err
				}
				delete(knownTypes, e)
			}
			pkg.genEnumRegistrations(buffer)
			setFile(path.Join(mod, "pulumiEnums.go"), buffer.String())
			setGenericVariantFile(path.Join(mod, "pulumiEnums.go"), genericVariantBuffer.String())
		}

		// Types
		sortedKnownTypes := slice.Prealloc[schema.Type](len(knownTypes))
		for k := range knownTypes {
			sortedKnownTypes = append(sortedKnownTypes, k)
		}
		sort.Slice(sortedKnownTypes, func(i, j int) bool {
			return sortedKnownTypes[i].String() < sortedKnownTypes[j].String()
		})

		if len(pkg.types) == 0 && len(pkg.enums) > 0 {
			// If there are no types, but there are enums, we still need to generate the types file.
			// with the associated nested collection enum types such as arrays of enums, maps of enums etc.

			collectionTypes := map[string]*nestedTypeInfo{}
			for _, t := range sortedKnownTypes {
				pkg.collectNestedCollectionTypes(collectionTypes, t)
			}

			if len(collectionTypes) > 0 {
				buffer := &bytes.Buffer{}
				useGenericVariant := false
				err := generateTypes(buffer, pkg, []*schema.ObjectType{}, sortedKnownTypes, useGenericVariant)
				if err != nil {
					return nil, err
				}
				typeFilePath := path.Join(mod, "pulumiTypes.go")
				setFile(typeFilePath, buffer.String())

				genericVariantBuffer := &bytes.Buffer{}
				useGenericVariant = true
				err = generateTypes(genericVariantBuffer, pkg, []*schema.ObjectType{}, sortedKnownTypes, useGenericVariant)
				if err != nil {
					return nil, err
				}
				setGenericVariantFile(typeFilePath, genericVariantBuffer.String())
			}
		}

		for types, i := pkg.types, 0; len(types) > 0; i++ {
			// 500 types corresponds to approximately 5M or 40_000 lines of code.
			const chunkSize = 500
			chunk := types
			if len(chunk) > chunkSize {
				chunk = chunk[:chunkSize]
			}
			types = types[len(chunk):]

			// To avoid duplicating collection types into every chunk, only pass known to chunk i=0.
			known := sortedKnownTypes
			if i != 0 {
				known = nil
			}

			buffer := &bytes.Buffer{}
			useGenericVariant := false
			err := generateTypes(buffer, pkg, chunk, known, useGenericVariant)
			if err != nil {
				return nil, err
			}

			typePath := "pulumiTypes"
			if i != 0 {
				typePath = fmt.Sprintf("%s%d", typePath, i)
			}

			typeFilePath := path.Join(mod, typePath+".go")
			setFile(typeFilePath, buffer.String())

			genericVariantBuffer := &bytes.Buffer{}
			useGenericVariant = true
			err = generateTypes(genericVariantBuffer, pkg, chunk, known, useGenericVariant)
			if err != nil {
				return nil, err
			}

			setGenericVariantFile(typeFilePath, genericVariantBuffer.String())
		}

		// Utilities
		if len(mod) == 0 {
			buffer := &bytes.Buffer{}
			importsAndAliases := map[string]string{
				"github.com/blang/semver":                   "",
				"github.com/pulumi/pulumi/sdk/v3/go/pulumi": "",
			}

			imports := []string{"fmt", "os", "reflect", "regexp", "strconv", "strings"}

			def, err := pkg.pkg.Definition()
			if err != nil {
				return nil, err
			}
			if def.Parameterization != nil {
				imports = append(imports, "encoding/base64")
				importsAndAliases["github.com/pulumi/pulumi/sdk/v3/proto/go"] = "pulumirpc"
			}

			pkg.genHeader(buffer, imports, importsAndAliases, true /* isUtil */)

			packageRegex := fmt.Sprintf("^.*/pulumi-%s/sdk(/v\\d+)?", pkg.pkg.Name())
			if pkg.rootPackageName != "" {
				packageRegex = fmt.Sprintf("^%s(/v\\d+)?", pkg.importBasePath)
			}
			err = pkg.GenUtilitiesFile(buffer, packageRegex)
			if err != nil {
				return nil, err
			}

			utilFilePath := pkg.internalModuleName + "/pulumiUtilities.go"
			setFile(path.Join(mod, utilFilePath), buffer.String())
			if emitOnlyGenericVariant {
				setGenericVariantFile(path.Join(mod, utilFilePath), buffer.String())
			}
		}

		// If there are resources in this module, register the module with the runtime.
		if len(pkg.resources) != 0 && !allResourcesAreOverlays(pkg.resources) {
			buffer := &bytes.Buffer{}
			err := pkg.genResourceModule(buffer)
			if err != nil {
				return nil, err
			}

			setFile(path.Join(mod, "init.go"), buffer.String())

			genericVariantBuffer := &bytes.Buffer{}
			if err := pkg.genResourceModule(genericVariantBuffer); err != nil {
				return nil, err
			}

			setGenericVariantFile(path.Join(mod, "init.go"), genericVariantBuffer.String())
		}
	}

	// create a go.mod file with references to local dependencies
	if pkg.SupportPack {
		var vPath string
		if pkg.Version != nil && pkg.Version.Major > 1 {
			vPath = fmt.Sprintf("/v%d", pkg.Version.Major)
		}

		modulePath := extractModulePath(pkg.Reference())
		if langInfo, found := pkg.Language["go"]; found {
			goInfo, ok := langInfo.(GoPackageInfo)
			if ok && goInfo.ModulePath != "" {
				modulePath = goInfo.ModulePath
			} else if ok && goInfo.ImportBasePath != "" {
				separatorIndex := strings.Index(goInfo.ImportBasePath, vPath)
				if separatorIndex >= 0 {
					modulePrefix := goInfo.ImportBasePath[:separatorIndex]
					modulePath = fmt.Sprintf("%s%s", modulePrefix, vPath)
				}
			}
		}

		var gomod modfile.File
		err = gomod.AddModuleStmt(modulePath)
		contract.AssertNoErrorf(err, "could not add module statement to go.mod")
		err = gomod.AddGoStmt("1.20")
		contract.AssertNoErrorf(err, "could not add Go statement to go.mod")
		pulumiPackagePath := "github.com/pulumi/pulumi/sdk/v3"
		pulumiVersion := "v3.30.0"
		if pkg.Parameterization != nil {
			pulumiVersion = "v3.133.0"
		}
		err = gomod.AddRequire(pulumiPackagePath, pulumiVersion)
		contract.AssertNoErrorf(err, "could not add require statement to go.mod")
		if replacementPath, hasReplacement := localDependencies["pulumi"]; hasReplacement {
			err = gomod.AddReplace(pulumiPackagePath, "", replacementPath, "")
			contract.AssertNoErrorf(err, "could not add replace statement to go.mod")
		}

		files["go.mod"], err = gomod.Format()
		contract.AssertNoErrorf(err, "could not format go.mod")
	}

	return files, nil
}

func generateTypes(
	w io.Writer,
	pkg *pkgContext,
	types []*schema.ObjectType,
	knownTypes []schema.Type,
	useGenericTypes bool,
) error {
	hasOutputs, importsAndAliases := false, map[string]string{}
	for _, t := range types {
		pkg.getImports(t, importsAndAliases)
		hasOutputs = hasOutputs || pkg.detailsForType(t).hasOutputs()
	}

	collectionTypes := map[string]*nestedTypeInfo{}
	for _, t := range knownTypes {
		pkg.collectNestedCollectionTypes(collectionTypes, t)
	}

	// All collection types have Outputs
	if len(collectionTypes) > 0 {
		hasOutputs = true
	}

	goInfo := goPackageInfo(pkg.pkg)
	var goImports []string
	if hasOutputs {
		goImports = []string{"context", "reflect"}
		importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumi"] = ""
		if goInfo.Generics == GenericsSettingSideBySide {
			importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumix"] = ""
		}
	}

	if useGenericTypes && hasOutputs {
		importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumix"] = ""
	}

	importsAndAliases[path.Join(pkg.importBasePath, pkg.internalModuleName)] = ""
	pkg.genHeader(w, goImports, importsAndAliases, false /* isUtil */)
	// in case we're not using the internal package, assign to a blank var
	fmt.Fprintf(w, "var _ = %s.GetEnvOrDefault\n", pkg.internalModuleName)

	for _, t := range types {
		if err := pkg.genType(w, t, useGenericTypes); err != nil {
			return err
		}
	}

	typeNames := []string{}
	if !useGenericTypes {
		typeNames = pkg.genNestedCollectionTypes(w, collectionTypes)
	}

	pkg.genTypeRegistrations(w, types, useGenericTypes, typeNames...)
	return nil
}

func allResourcesAreOverlays(resources []*schema.Resource) bool {
	for _, r := range resources {
		if !r.IsOverlay {
			return false
		}
	}
	return true
}

// goPackage returns the suggested package name for the given string.
func goPackage(name string) string {
	return strings.ReplaceAll(name, "-", "")
}

//go:embed embeddedUtilities.go
var embeddedUtilities string

func (pkg *pkgContext) GenUtilitiesFile(w io.Writer, packageRegex string) error {
	subtitutions := map[string]string{
		`"${packageRegex}"`: fmt.Sprintf("%q", packageRegex),
	}
	i := strings.Index(embeddedUtilities, "package utilities")
	code := embeddedUtilities[i+len("package utilities"):]
	for x, y := range subtitutions {
		code = strings.ReplaceAll(code, x, y)
	}
	_, err := fmt.Fprintf(w, "%s", code)
	if err != nil {
		return err
	}
	return pkg.GenPkgDefaultOpts(w)
}

func (pkg *pkgContext) GenVersionFile(w io.Writer) error {
	const versionFile = `var SdkVersion semver.Version = semver.Version{}
var pluginDownloadURL string = ""
`
	_, err := fmt.Fprint(w, versionFile)
	return err
}

func (pkg *pkgContext) GenPkgDefaultOpts(w io.Writer) error {
	p, err := pkg.pkg.Definition()
	if err != nil {
		return err
	}
	url := p.PluginDownloadURL
	const template string = `
// Pkg%[1]sDefaultOpts provides package level defaults to pulumi.Option%[1]s.
func Pkg%[1]sDefaultOpts(opts []pulumi.%[1]sOption) []pulumi.%[1]sOption {
	defaults := []pulumi.%[1]sOption{}
		%[2]s
		version := %[3]s
		if !version.Equals(semver.Version{}){
			defaults = append(defaults, pulumi.Version(version.String()))
		}
	return append(defaults, opts...)
}
`
	var pluginDownloadURL string
	if url != "" {
		pluginDownloadURL = fmt.Sprintf(`defaults = append(defaults, pulumi.PluginDownloadURL("%s"))`, url)
	}

	versionPackageRef := "SdkVersion"

	versionPkgName := strings.ReplaceAll(pkg.pkg.Name(), "-", "")

	if pkg.mod != "" {
		versionPackageRef = versionPkgName + "." + versionPackageRef
	}
	if info := p.Language["go"]; info != nil {
		if info.(GoPackageInfo).RespectSchemaVersion && pkg.pkg.Version() != nil {
			versionPackageRef = fmt.Sprintf("semver.MustParse(%q)", p.Version.String())
		}
	} else if pkg.pkg.SupportPack() && pkg.pkg.Version() != nil {
		versionPackageRef = fmt.Sprintf("semver.MustParse(%q)", p.Version.String())
	}
	// Parameterized schemas _always_ respect schema version.
	if p.Parameterization != nil {
		if p.Version == nil {
			return errors.New("package version is required")
		}
		versionPackageRef = fmt.Sprintf("semver.MustParse(%q)", p.Version.String())

		const packageRefTemplate string = `
var packageRef *string
// PkgGetPackageRef returns the package reference for the current package.
func PkgGetPackageRef(ctx *pulumi.Context) (string, error) {
	if packageRef == nil {

		parameter, err := base64.StdEncoding.DecodeString(%q)
		if err != nil {
			return "", err
		}

		resp, err := ctx.RegisterPackage(&pulumirpc.RegisterPackageRequest{
			Name: %q,
			Version: %q,
			DownloadUrl: %q,
			Parameterization: &pulumirpc.Parameterization{
				Name: %q,
				Version: %q,
				Value: parameter,
			},
		})
		if err != nil {
			return "", err
		}
		packageRef = &resp.Ref
	}

	return *packageRef, nil
}
`

		value := base64.StdEncoding.EncodeToString(p.Parameterization.Parameter)
		_, err = fmt.Fprintf(w, packageRefTemplate,
			value,
			p.Parameterization.BaseProvider.Name, p.Parameterization.BaseProvider.Version.String(), p.PluginDownloadURL,
			p.Name, p.Version.String(),
		)
		if err != nil {
			return err
		}
	}
	for _, typ := range []string{"Resource", "Invoke"} {
		_, err := fmt.Fprintf(w, template, typ, pluginDownloadURL, versionPackageRef)
		if err != nil {
			return err
		}
	}

	return nil
}

// GenPkgDefaultsOptsCall generates a call to Pkg{TYPE}DefaultsOpts.
func (pkg *pkgContext) GenPkgDefaultsOptsCall(w io.Writer, invoke bool) error {
	typ := "Resource"
	if invoke {
		typ = "Invoke"
	}

	_, err := fmt.Fprintf(w, "\topts = %s.Pkg%sDefaultOpts(opts)\n", pkg.internalModuleName, typ)
	if err != nil {
		return err
	}

	return nil
}

// GenPkgGetPackageRefCall generates a call to PkgGetPackageRef.
func (pkg *pkgContext) GenPkgGetPackageRefCall(w io.Writer, errorResult string) error {
	_, err := fmt.Fprintf(w, "\tref, err := %s.PkgGetPackageRef(ctx)\n", pkg.internalModuleName)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "\tif err != nil {\n")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "\t\treturn %s, err\n", errorResult)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "\t}\n")
	if err != nil {
		return err
	}

	return nil
}
