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
// nolint: lll, goconst
package gen

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"os"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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
	runes := []rune(s)
	return string(append([]rune{unicode.ToUpper(runes[0])}, runes[1:]...))
}

func camel(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	res := make([]rune, 0, len(runes))
	for i, r := range runes {
		if unicode.IsLower(r) {
			res = append(res, runes[i:]...)
			break
		}
		res = append(res, unicode.ToLower(r))
	}
	return string(res)
}

func tokenToPackage(pkg *schema.Package, overrides map[string]string, tok string) string {
	mod := pkg.TokenToModule(tok)
	if override, ok := overrides[mod]; ok {
		mod = override
	}
	return strings.ToLower(mod)
}

type pkgContext struct {
	pkg             *schema.Package
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
	externalPackages map[*schema.Package]map[string]*pkgContext

	// duplicateTokens tracks tokens that exist for both types and resources
	duplicateTokens map[string]bool
	functionNames   map[*schema.Function]string
	needsUtils      bool
	tool            string
	packages        map[string]*pkgContext

	// Name overrides set in GoPackageInfo
	modToPkg         map[string]string // Module name -> package name
	pkgImportAliases map[string]string // Package name -> import alias

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
	if pkg == nil {
		panic(fmt.Errorf("pkg is nil. token %s", tok))
	}
	if pkg.pkg == nil {
		panic(fmt.Errorf("pkg.pkg is nil. token %s", tok))
	}

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
		mod = packageRoot(pkg.pkg)
	}

	var importPath string
	if alias, hasAlias := pkg.pkgImportAliases[path.Join(pkg.importBasePath, mod)]; hasAlias {
		importPath = alias
	} else {
		importPath = strings.ReplaceAll(mod, "/", "")
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
	enumType := extPkgCtx.tokenToEnum(t.Token)
	if !strings.Contains(enumType, ".") {
		enumType = fmt.Sprintf("%s.%s", extPkgCtx.pkg.Name, enumType)
	}
	return enumType
}

func (pkg *pkgContext) tokenToEnum(tok string) string {
	// token := pkg : module : member
	// module := path/to/module

	components := strings.Split(tok, ":")
	contract.Assert(len(components) == 3)
	if pkg == nil {
		panic(fmt.Errorf("pkg is nil. token %s", tok))
	}
	if pkg.pkg == nil {
		panic(fmt.Errorf("pkg.pkg is nil. token %s", tok))
	}

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
	contract.Assert(len(components) == 3)
	if pkg == nil {
		panic(fmt.Errorf("pkg is nil. token %s", tok))
	}
	if pkg.pkg == nil {
		panic(fmt.Errorf("pkg.pkg is nil. token %s", tok))
	}

	// Is it a provider resource?
	if components[0] == "pulumi" && components[1] == "providers" {
		return fmt.Sprintf("%s.Provider", components[2])
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
	contract.Assert(len(components) == 3)
	return components[1]
}

func tokenToName(tok string) string {
	components := strings.Split(tok, ":")
	contract.Assert(len(components) == 3)
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
		return strings.TrimSuffix(en, "Args") + "Array"
	case *schema.MapType:
		en := pkg.argsTypeImpl(t.ElementType)
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
	isExternal bool, extPkg *schema.Package, token string) {
	var err error
	switch typ := t.(type) {
	case *schema.ObjectType:
		isExternal = typ.Package != nil && pkg.pkg != nil && typ.Package != pkg.pkg
		if isExternal {
			extPkg, err = typ.PackageReference.Definition()
			contract.AssertNoError(err)
			token = typ.Token
		}
		return
	case *schema.ResourceType:
		isExternal = typ.Resource != nil && pkg.pkg != nil && typ.Resource.Package != pkg.pkg
		if isExternal {
			extPkg, err = typ.Resource.PackageReference.Definition()
			contract.AssertNoError(err)
			token = typ.Token
		}
		return
	case *schema.EnumType:
		isExternal = pkg.pkg != nil && typ.Package != pkg.pkg
		if isExternal {
			extPkg, err = typ.PackageReference.Definition()
			contract.AssertNoError(err)
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
		resType = fmt.Sprintf("%s.%s", extPkgCtx.pkg.Name, resType)
	}
	return resType
}

// resolveObjectType resolves resource references in properties while
// taking into account potential external resources. Returned type is
// always marked as required. Caller should check if the property is
// optional and convert the type to a pointer if necessary.
func (pkg *pkgContext) resolveObjectType(t *schema.ObjectType) string {
	if !pkg.isExternalReference(t) {
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
	contract.Assert(isExternal)

	var goInfo GoPackageInfo
	contract.AssertNoError(extPkg.ImportLanguages(map[string]schema.Language{"go": Importer}))
	if info, ok := extPkg.Language["go"].(GoPackageInfo); ok {
		goInfo = info
	} else {
		goInfo.ImportBasePath = extractImportBasePath(extPkg)
	}

	pkgImportAliases := goInfo.PackageImportAliases

	// Ensure that any package import aliases we have specified locally take precedence over those
	// specified in the remote package.
	if ourPkgGoInfoI, has := pkg.pkg.Language["go"]; has {
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
	if pkg.externalPackages == nil {
		pkg.externalPackages = map[*schema.Package]map[string]*pkgContext{}
	}
	if extMap, ok := pkg.externalPackages[extPkg]; ok {
		maps = extMap
	} else {
		maps = generatePackageContextMap(pkg.tool, extPkg, goInfo)
		pkg.externalPackages[extPkg] = maps
	}
	extPkgCtx := maps[""]
	extPkgCtx.pkgImportAliases = pkgImportAliases
	mod := tokenToPackage(extPkg, goInfo.ModuleToPackage, token)

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

// outputType returns a reference to the Go output type that corresponds to the given schema type. For example, given
// a schema.String, outputType returns "pulumi.String", and given a *schema.ObjectType with the token pkg:mod:Name,
// outputType returns "mod.NameOutput" or "NameOutput", depending on whether or not the object type lives in a
// different module than the one associated with the receiver.
func (pkg *pkgContext) outputType(t schema.Type) string {
	return pkg.outputTypeImpl(codegen.ResolvedType(t))
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
		printComment(w, fmt.Sprintf("Deprecated: %s", deprecationMessage), indent)
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
	name            string
	receiverType    string
	elementType     string
	ptrMethods      bool
	toOutputMethods bool
}

func genInputImplementation(w io.Writer, name, receiverType, elementType string, ptrMethods bool) {
	genInputImplementationWithArgs(w, genInputImplementationArgs{
		name:            name,
		receiverType:    receiverType,
		elementType:     elementType,
		ptrMethods:      ptrMethods,
		toOutputMethods: true,
	})
}

func genInputImplementationWithArgs(w io.Writer, genArgs genInputImplementationArgs) {
	name := genArgs.name
	receiverType := genArgs.receiverType
	elementType := genArgs.elementType

	fmt.Fprintf(w, "func (%s) ElementType() reflect.Type {\n", receiverType)
	fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s)(nil)).Elem()\n", elementType)
	fmt.Fprintf(w, "}\n\n")

	if genArgs.toOutputMethods {
		fmt.Fprintf(w, "func (i %s) To%sOutput() %sOutput {\n", receiverType, Title(name), name)
		fmt.Fprintf(w, "\treturn i.To%sOutputWithContext(context.Background())\n", Title(name))
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (i %s) To%sOutputWithContext(ctx context.Context) %sOutput {\n", receiverType, Title(name), name)
		fmt.Fprintf(w, "\treturn pulumi.ToOutputWithContext(ctx, i).(%sOutput)\n", name)
		fmt.Fprintf(w, "}\n\n")
	}

	if genArgs.ptrMethods {
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
	}
}

func genOutputType(w io.Writer, baseName, elementType string, ptrMethods bool) {
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

	if ptrMethods {
		fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutput() %[1]sPtrOutput {\n", baseName, Title(baseName))
		fmt.Fprintf(w, "\treturn o.To%sPtrOutputWithContext(context.Background())\n", Title(baseName))
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutputWithContext(ctx context.Context) %[1]sPtrOutput {\n", baseName, Title(baseName))
		fmt.Fprintf(w, "\treturn o.ApplyTWithContext(ctx, func(_ context.Context, v %[1]s) *%[1]s {\n", elementType)
		fmt.Fprintf(w, "\t\treturn &v\n")
		fmt.Fprintf(w, "\t}).(%sPtrOutput)\n", baseName)
		fmt.Fprintf(w, "}\n\n")
	}
}

func genArrayOutput(w io.Writer, baseName, elementType string) {
	genOutputType(w, baseName+"Array", "[]"+elementType, false)

	fmt.Fprintf(w, "func (o %[1]sArrayOutput) Index(i pulumi.IntInput) %[1]sOutput {\n", baseName)
	fmt.Fprintf(w, "\treturn pulumi.All(o, i).ApplyT(func (vs []interface{}) %s {\n", elementType)
	fmt.Fprintf(w, "\t\treturn vs[0].([]%s)[vs[1].(int)]\n", elementType)
	fmt.Fprintf(w, "\t}).(%sOutput)\n", baseName)
	fmt.Fprintf(w, "}\n\n")
}

func genMapOutput(w io.Writer, baseName, elementType string) {
	genOutputType(w, baseName+"Map", "map[string]"+elementType, false)

	fmt.Fprintf(w, "func (o %[1]sMapOutput) MapIndex(k pulumi.StringInput) %[1]sOutput {\n", baseName)
	fmt.Fprintf(w, "\treturn pulumi.All(o, k).ApplyT(func (vs []interface{}) %s{\n", elementType)
	fmt.Fprintf(w, "\t\treturn vs[0].(map[string]%s)[vs[1].(string)]\n", elementType)
	fmt.Fprintf(w, "\t}).(%sOutput)\n", baseName)
	fmt.Fprintf(w, "}\n\n")
}

func genPtrOutput(w io.Writer, baseName, elementType string) {
	genOutputType(w, baseName+"Ptr", "*"+elementType, false)

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

func (pkg *pkgContext) genEnum(w io.Writer, enumType *schema.EnumType) error {
	name := pkg.tokenToEnum(enumType.Token)

	mod := pkg.tokenToPackage(enumType.Token)
	modPkg, ok := pkg.packages[mod]
	contract.Assert(ok)

	printCommentWithDeprecationMessage(w, enumType.Comment, "", false)

	elementArgsType := pkg.argsTypeImpl(enumType.ElementType)
	elementGoType := pkg.typeString(enumType.ElementType)
	asFuncName := strings.TrimPrefix(elementArgsType, "pulumi.")

	fmt.Fprintf(w, "type %s %s\n\n", name, elementGoType)

	fmt.Fprintln(w, "const (")
	for _, e := range enumType.Elements {
		printCommentWithDeprecationMessage(w, e.Comment, e.DeprecationMessage, true)

		var elementName = e.Name
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

		switch reflect.TypeOf(e.Value).Kind() {
		case reflect.String:
			fmt.Fprintf(w, "%s = %s(%q)\n", e.Name, name, e.Value)
		default:
			fmt.Fprintf(w, "%s = %s(%v)\n", e.Name, name, e.Value)
		}
	}
	fmt.Fprintln(w, ")")

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

		genInputImplementation(w, name+"Array", name+"Array", "[]"+name, false)
	}

	// Generate the map input.
	if details.mapInput {
		pkg.genInputInterface(w, name+"Map")

		fmt.Fprintf(w, "type %[1]sMap map[string]%[1]s\n\n", name)

		genInputImplementation(w, name+"Map", name+"Map", "map[string]"+name, false)
	}

	// Generate the array output
	if details.arrayOutput {
		genArrayOutput(w, name, name)
	}

	// Generate the map output.
	if details.mapOutput {
		genMapOutput(w, name, name)
	}

	return nil
}

func (pkg *pkgContext) genEnumOutputTypes(w io.Writer, name, elementArgsType, elementGoType, asFuncName string) {
	genOutputType(w, name, name, true)

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

	genPtrOutput(w, name, name)

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
	pkg.genInputInterface(w, name)

	fmt.Fprintf(w, "var %sPtrType = reflect.TypeOf((**%s)(nil)).Elem()\n", camel(name), name)
	fmt.Fprintln(w)

	fmt.Fprintf(w, "type %sPtrInput interface {\n", name)
	fmt.Fprint(w, "pulumi.Input\n\n")
	fmt.Fprintf(w, "To%[1]sPtrOutput() %[1]sPtrOutput\n", name)
	fmt.Fprintf(w, "To%[1]sPtrOutputWithContext(context.Context) %[1]sPtrOutput\n", name)
	fmt.Fprintf(w, "}\n")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "type %sPtr %s\n", camel(name), elementGoType)
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func %[1]sPtr(v %[2]s) %[1]sPtrInput {\n", name, elementGoType)
	fmt.Fprintf(w, "return (*%sPtr)(&v)\n", camel(name))
	fmt.Fprintf(w, "}\n")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (*%sPtr) ElementType() reflect.Type {\n", camel(name))
	fmt.Fprintf(w, "return %sPtrType\n", camel(name))
	fmt.Fprintf(w, "}\n")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (in *%[1]sPtr) To%[2]sPtrOutput() %[2]sPtrOutput {\n", camel(name), name)
	fmt.Fprintf(w, "return pulumi.ToOutput(in).(%sPtrOutput)\n", name)
	fmt.Fprintf(w, "}\n")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (in *%[1]sPtr) To%[2]sPtrOutputWithContext(ctx context.Context) %[2]sPtrOutput {\n", camel(name), name)
	fmt.Fprintf(w, "return pulumi.ToOutputWithContext(ctx, in).(%sPtrOutput)\n", name)
	fmt.Fprintf(w, "}\n")
	fmt.Fprintln(w)
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

func (pkg *pkgContext) assignProperty(w io.Writer, p *schema.Property, object, value string, indirectAssign bool) {
	t := strings.TrimSuffix(pkg.typeString(p.Type), "Input")
	switch codegen.UnwrapType(p.Type).(type) {
	case *schema.EnumType:
		t = ""
	}

	if codegen.IsNOptionalInput(p.Type) {
		if t != "" {
			value = fmt.Sprintf("%s(%s)", t, value)
		}
		fmt.Fprintf(w, "\t%s.%s = %s\n", object, Title(p.Name), value)
	} else if indirectAssign {
		tmpName := camel(p.Name) + "_"
		fmt.Fprintf(w, "%s := %s\n", tmpName, value)
		fmt.Fprintf(w, "%s.%s = &%s\n", object, Title(p.Name), tmpName)
	} else {
		fmt.Fprintf(w, "%s.%s = %s\n", object, Title(p.Name), value)
	}
}

func (pkg *pkgContext) genPlainType(w io.Writer, name, comment, deprecationMessage string,
	properties []*schema.Property) {

	printCommentWithDeprecationMessage(w, comment, deprecationMessage, false)
	fmt.Fprintf(w, "type %s struct {\n", name)
	for _, p := range properties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", Title(p.Name), pkg.typeString(codegen.ResolvedType(p.Type)), p.Name)
	}
	fmt.Fprintf(w, "}\n\n")
}

func (pkg *pkgContext) genObjectDefaultFunc(w io.Writer, name string,
	properties []*schema.Property) error {
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
			dv, err := pkg.getDefaultValue(p.DefaultValue, codegen.UnwrapType(p.Type))
			if err != nil {
				return err
			}
			pkg.needsUtils = true
			fmt.Fprintf(w, "if isZero(tmp.%s) {\n", Title(p.Name))
			pkg.assignProperty(w, p, "tmp", dv, !p.IsRequired())
			fmt.Fprintf(w, "}\n")
		} else if funcName := pkg.provideDefaultsFuncName(p.Type); funcName != "" {
			var member string
			if codegen.IsNOptionalInput(p.Type) {
				// f := fmt.Sprintf("func(v %[1]s) %[1]s { return *v.%[2]s() }", name, funcName)
				// member = fmt.Sprintf("tmp.%[1]s.ApplyT(%[2]s)", Title(p.Name), f)
			} else {
				member = fmt.Sprintf("tmp.%[1]s.%[2]s()", Title(p.Name), funcName)
				sigil := ""
				if p.IsRequired() {
					sigil = "*"
				}
				pkg.assignProperty(w, p, "tmp", sigil+member, false)
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

func (pkg *pkgContext) genInputTypes(w io.Writer, t *schema.ObjectType, details *typeDetails) error {
	contract.Assert(t.IsInputShape())

	name := pkg.tokenToType(t.Token)

	// Generate the plain inputs.
	if details.input {
		pkg.genInputInterface(w, name)

		inputName := name + "Args"
		pkg.genInputArgsStruct(w, inputName, t)
		if !pkg.disableObjectDefaults {
			if err := pkg.genObjectDefaultFunc(w, inputName, t.Properties); err != nil {
				return err
			}
		}

		genInputImplementation(w, name, inputName, name, details.ptrInput)

	}

	// Generate the pointer input.
	if details.ptrInput {
		pkg.genInputInterface(w, name+"Ptr")

		ptrTypeName := camel(name) + "PtrType"

		fmt.Fprintf(w, "type %s %sArgs\n\n", ptrTypeName, name)

		fmt.Fprintf(w, "func %[1]sPtr(v *%[1]sArgs) %[1]sPtrInput {", name)
		fmt.Fprintf(w, "\treturn (*%s)(v)\n", ptrTypeName)
		fmt.Fprintf(w, "}\n\n")

		genInputImplementation(w, name+"Ptr", "*"+ptrTypeName, "*"+name, false)
	}

	// Generate the array input.
	if details.arrayInput {
		pkg.genInputInterface(w, name+"Array")

		fmt.Fprintf(w, "type %[1]sArray []%[1]sInput\n\n", name)

		genInputImplementation(w, name+"Array", name+"Array", "[]"+name, false)
	}

	// Generate the map input.
	if details.mapInput {
		pkg.genInputInterface(w, name+"Map")

		fmt.Fprintf(w, "type %[1]sMap map[string]%[1]sInput\n\n", name)

		genInputImplementation(w, name+"Map", name+"Map", "map[string]"+name, false)
	}
	return nil
}

func (pkg *pkgContext) genInputArgsStruct(w io.Writer, typeName string, t *schema.ObjectType) {
	contract.Assert(t.IsInputShape())

	printComment(w, t.Comment, false)
	fmt.Fprintf(w, "type %s struct {\n", typeName)
	for _, p := range t.Properties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", Title(p.Name), pkg.typeString(p.Type), p.Name)
	}
	fmt.Fprintf(w, "}\n\n")
}

type genOutputTypesArgs struct {
	t *schema.ObjectType

	// optional type name override
	name string
}

func (pkg *pkgContext) genOutputTypes(w io.Writer, genArgs genOutputTypesArgs) {
	t := genArgs.t
	details := pkg.detailsForType(t)

	contract.Assert(!t.IsInputShape())

	name := genArgs.name
	if name == "" {
		name = pkg.tokenToType(t.Token)
	}

	if details.output {
		printComment(w, t.Comment, false)
		genOutputType(w,
			name,             /* baseName */
			name,             /* elementType */
			details.ptrInput, /* ptrMethods */
		)

		for _, p := range t.Properties {
			printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, false)
			outputType, applyType := pkg.outputType(p.Type), pkg.typeString(p.Type)

			propName := Title(p.Name)
			switch strings.ToLower(p.Name) {
			case "elementtype", "issecret":
				propName = "Get" + propName
			}
			fmt.Fprintf(w, "func (o %sOutput) %s() %s {\n", name, propName, outputType)
			fmt.Fprintf(w, "\treturn o.ApplyT(func (v %s) %s { return v.%s }).(%s)\n",
				name, applyType, Title(p.Name), outputType)
			fmt.Fprintf(w, "}\n\n")
		}
	}

	if details.ptrOutput {
		genPtrOutput(w, name, name)

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
			fmt.Fprintf(w, "\t\treturn %sv.%s\n", deref, Title(p.Name))
			fmt.Fprintf(w, "\t}).(%s)\n", outputType)
			fmt.Fprintf(w, "}\n\n")
		}
	}

	if details.arrayOutput {
		genArrayOutput(w, name, name)
	}

	if details.mapOutput {
		genMapOutput(w, name, name)
	}
}

func goPrimitiveValue(value interface{}) (string, error) {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

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

func (pkg *pkgContext) getDefaultValue(dv *schema.DefaultValue, t schema.Type) (string, error) {
	var val string
	if dv.Value != nil {
		v, err := goPrimitiveValue(dv.Value)
		if err != nil {
			return "", err
		}
		val = v
		switch t.(type) {
		case *schema.EnumType:
			typeName := strings.TrimSuffix(pkg.typeString(codegen.UnwrapType(t)), "Input")
			val = fmt.Sprintf("%s(%s)", typeName, val)
		}
	}

	if len(dv.Environment) > 0 {
		pkg.needsUtils = true

		parser, typDefault, typ := "nil", "\"\"", "string"
		switch codegen.UnwrapType(t).(type) {
		case *schema.ArrayType:
			parser, typDefault, typ = "parseEnvStringArray", "pulumi.StringArray{}", "pulumi.StringArray"
		}
		switch t {
		case schema.BoolType:
			parser, typDefault, typ = "parseEnvBool", "false", "bool"
		case schema.IntType:
			parser, typDefault, typ = "parseEnvInt", "0", "int"
		case schema.NumberType:
			parser, typDefault, typ = "parseEnvFloat", "0.0", "float64"
		}

		if val == "" {
			val = typDefault
		}

		val = fmt.Sprintf("getEnvOrDefault(%s, %s", val, parser)
		for _, e := range dv.Environment {
			val += fmt.Sprintf(", %q", e)
		}
		val = fmt.Sprintf("%s).(%s)", val, typ)
	}

	return val, nil
}

func (pkg *pkgContext) genResource(w io.Writer, r *schema.Resource, generateResourceContainerTypes bool) error {
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
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", Title(p.Name), pkg.outputType(p.Type), p.Name)

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
			fmt.Fprintf(w, "\tif args.%s == nil {\n", Title(p.Name))
			fmt.Fprintf(w, "\t\treturn nil, errors.New(\"invalid value for required argument '%s'\")\n", Title(p.Name))
			fmt.Fprintf(w, "\t}\n")
		}

		if p.Secret {
			secretInputProps = append(secretInputProps, p)
		}
	}

	assign := func(p *schema.Property, value string) {
		pkg.assignProperty(w, p, "args", value, isNilType(p.Type))
	}

	for _, p := range r.InputProperties {
		if p.ConstValue != nil {
			v, err := pkg.getConstValue(p.ConstValue)
			if err != nil {
				return err
			}
			assign(p, v)
		} else if p.DefaultValue != nil {
			dv, err := pkg.getDefaultValue(p.DefaultValue, codegen.UnwrapType(p.Type))
			if err != nil {
				return err
			}
			pkg.needsUtils = true
			fmt.Fprintf(w, "\tif isZero(args.%s) {\n", Title(p.Name))
			assign(p, dv)
			fmt.Fprintf(w, "\t}\n")
		} else if name := pkg.provideDefaultsFuncName(p.Type); name != "" && !pkg.disableObjectDefaults {
			optionalDeref := ""
			if p.IsRequired() {
				optionalDeref = "*"
			}

			toOutputMethod := pkg.toOutputMethod(p.Type)
			outputType := pkg.outputType(p.Type)
			resolvedType := pkg.typeString(codegen.ResolvedType(p.Type))
			originalValue := fmt.Sprintf("args.%s.%s()", Title(p.Name), toOutputMethod)
			valueWithDefaults := fmt.Sprintf("%[1]v.ApplyT(func (v %[2]s) %[2]s { return %[3]sv.%[4]s() }).(%[5]s)",
				originalValue, resolvedType, optionalDeref, name, outputType)
			if p.Plain {
				valueWithDefaults = fmt.Sprintf("args.%v.Defaults()", Title(p.Name))
			}

			if !p.IsRequired() {
				fmt.Fprintf(w, "if args.%s != nil {\n", Title(p.Name))
				fmt.Fprintf(w, "args.%[1]s = %s\n", Title(p.Name), valueWithDefaults)
				fmt.Fprint(w, "}\n")
			} else {
				fmt.Fprintf(w, "args.%[1]s = %s\n", Title(p.Name), valueWithDefaults)
			}

		}
	}

	// Set any defined aliases.
	if len(r.Aliases) > 0 {
		fmt.Fprintf(w, "\taliases := pulumi.Aliases([]pulumi.Alias{\n")
		for _, alias := range r.Aliases {
			s := "\t\t{\n"
			if alias.Name != nil {
				s += fmt.Sprintf("\t\t\tName: pulumi.String(%q),\n", *alias.Name)
			}
			if alias.Project != nil {
				s += fmt.Sprintf("\t\t\tProject: pulumi.String(%q),\n", *alias.Project)
			}
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
		fmt.Fprintf(w, "\tif args.%s != nil {\n", Title(p.Name))
		fmt.Fprintf(w, "\t\targs.%[1]s = pulumi.ToSecret(args.%[1]s).(%[2]s)\n", Title(p.Name), pkg.outputType(p.Type))
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

	pkg.GenPkgDefaultsOptsCall(w, false /*invoke*/)

	// Finally make the call to registration.
	fmt.Fprintf(w, "\tvar resource %s\n", name)
	if r.IsComponent {
		fmt.Fprintf(w, "\terr := ctx.RegisterRemoteComponentResource(\"%s\", name, args, &resource, opts...)\n", r.Token)
	} else {
		fmt.Fprintf(w, "\terr := ctx.RegisterResource(\"%s\", name, args, &resource, opts...)\n", r.Token)
	}
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
		fmt.Fprintf(w, "\terr := ctx.ReadResource(\"%s\", name, id, state, &resource, opts...)\n", r.Token)
		fmt.Fprintf(w, "\tif err != nil {\n")
		fmt.Fprintf(w, "\t\treturn nil, err\n")
		fmt.Fprintf(w, "\t}\n")
		fmt.Fprintf(w, "\treturn &resource, nil\n")
		fmt.Fprintf(w, "}\n\n")

		// Emit the state types for get methods.
		fmt.Fprintf(w, "// Input properties used for looking up and filtering %s resources.\n", name)
		fmt.Fprintf(w, "type %sState struct {\n", camel(name))
		if r.StateInputs != nil {
			for _, p := range r.StateInputs.Properties {
				printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
				fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", Title(p.Name), pkg.typeString(codegen.ResolvedType(codegen.OptionalType(p))), p.Name)
			}
		}
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "type %sState struct {\n", name)
		if r.StateInputs != nil {
			for _, p := range r.StateInputs.Properties {
				printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
				fmt.Fprintf(w, "\t%s %s\n", Title(p.Name), pkg.inputType(p.Type))
			}
		}
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (%sState) ElementType() reflect.Type {\n", name)
		fmt.Fprintf(w, "\treturn reflect.TypeOf((*%sState)(nil)).Elem()\n", camel(name))
		fmt.Fprintf(w, "}\n\n")
	}

	// Emit the args types.
	fmt.Fprintf(w, "type %sArgs struct {\n", camel(name))
	for _, p := range r.InputProperties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", Title(p.Name), pkg.typeString(codegen.ResolvedType(p.Type)), p.Name)
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

		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		fmt.Fprintf(w, "\t%s %s\n", Title(p.Name), pkg.typeString(typ))
	}
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (%sArgs) ElementType() reflect.Type {\n", name)
	fmt.Fprintf(w, "\treturn reflect.TypeOf((*%sArgs)(nil)).Elem()\n", camel(name))
	fmt.Fprintf(w, "}\n")

	// Emit resource methods.
	for _, method := range r.Methods {
		methodName := Title(method.Name)
		f := method.Function

		shouldLiftReturn := pkg.liftSingleValueMethodReturns && f.Outputs != nil && len(f.Outputs.Properties) == 1

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
		if f.Outputs == nil {
			retty = "error"
		} else if shouldLiftReturn {
			retty = fmt.Sprintf("(%s, error)", pkg.outputType(f.Outputs.Properties[0].Type))
		} else {
			retty = fmt.Sprintf("(%s%sResultOutput, error)", name, methodName)
		}
		fmt.Fprintf(w, "\n")
		printCommentWithDeprecationMessage(w, f.Comment, f.DeprecationMessage, false)
		fmt.Fprintf(w, "func (r *%s) %s(%s) %s {\n", name, methodName, argsig, retty)

		resultVar := "_"
		if f.Outputs != nil {
			resultVar = "out"
		}

		// Make a map of inputs to pass to the runtime function.
		inputsVar := "nil"
		if len(args) > 0 {
			inputsVar = "args"
		}

		// Now simply invoke the runtime function with the arguments.
		outputsType := "pulumi.AnyOutput"
		if f.Outputs != nil {
			if shouldLiftReturn {
				outputsType = fmt.Sprintf("%s%sResultOutput", camel(name), methodName)
			} else {
				outputsType = fmt.Sprintf("%s%sResultOutput", name, methodName)
			}
		}
		fmt.Fprintf(w, "\t%s, err := ctx.Call(%q, %s, %s{}, r)\n", resultVar, f.Token, inputsVar, outputsType)
		if f.Outputs == nil {
			fmt.Fprintf(w, "\treturn err\n")
		} else if shouldLiftReturn {
			// Check the error before proceeding.
			fmt.Fprintf(w, "\tif err != nil {\n")
			fmt.Fprintf(w, "\t\treturn %s{}, err\n", pkg.outputType(f.Outputs.Properties[0].Type))
			fmt.Fprintf(w, "\t}\n")

			// Get the name of the method to return the output
			fmt.Fprintf(w, "\treturn %s.(%s).%s(), nil\n", resultVar, camel(outputsType), Title(f.Outputs.Properties[0].Name))
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
			fmt.Fprintf(w, "type %s%sArgs struct {\n", camel(name), methodName)
			for _, p := range args {
				printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
				fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", Title(p.Name), pkg.typeString(codegen.ResolvedType(p.Type)),
					p.Name)
			}
			fmt.Fprintf(w, "}\n\n")

			fmt.Fprintf(w, "// The set of arguments for the %s method of the %s resource.\n", methodName, name)
			fmt.Fprintf(w, "type %s%sArgs struct {\n", name, methodName)
			for _, p := range args {
				printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
				fmt.Fprintf(w, "\t%s %s\n", Title(p.Name), pkg.typeString(p.Type))
			}
			fmt.Fprintf(w, "}\n\n")

			fmt.Fprintf(w, "func (%s%sArgs) ElementType() reflect.Type {\n", name, methodName)
			fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s%sArgs)(nil)).Elem()\n", camel(name), methodName)
			fmt.Fprintf(w, "}\n\n")
		}
		if f.Outputs != nil {
			outputStructName := name

			// Don't export the result struct if we're lifting the value
			if shouldLiftReturn {
				outputStructName = camel(name)
			}

			fmt.Fprintf(w, "\n")
			pkg.genPlainType(w, fmt.Sprintf("%s%sResult", outputStructName, methodName), f.Outputs.Comment, "",
				f.Outputs.Properties)

			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "type %s%sResultOutput struct{ *pulumi.OutputState }\n\n", outputStructName, methodName)

			fmt.Fprintf(w, "func (%s%sResultOutput) ElementType() reflect.Type {\n", outputStructName, methodName)
			fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s%sResult)(nil)).Elem()\n", outputStructName, methodName)
			fmt.Fprintf(w, "}\n")

			for _, p := range f.Outputs.Properties {
				fmt.Fprintf(w, "\n")
				printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, false)
				fmt.Fprintf(w, "func (o %s%sResultOutput) %s() %s {\n", outputStructName, methodName, Title(p.Name),
					pkg.outputType(p.Type))
				fmt.Fprintf(w, "\treturn o.ApplyT(func(v %s%sResult) %s { return v.%s }).(%s)\n", outputStructName, methodName,
					pkg.typeString(codegen.ResolvedType(p.Type)), Title(p.Name), pkg.outputType(p.Type))
				fmt.Fprintf(w, "}\n")
			}
		}
	}

	// Emit the resource input type.
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "type %sInput interface {\n", name)
	fmt.Fprintf(w, "\tpulumi.Input\n\n")
	fmt.Fprintf(w, "\tTo%[1]sOutput() %[1]sOutput\n", name)
	fmt.Fprintf(w, "\tTo%[1]sOutputWithContext(ctx context.Context) %[1]sOutput\n", name)
	fmt.Fprintf(w, "}\n\n")

	genInputImplementation(w, name, "*"+name, "*"+name, false)

	if generateResourceContainerTypes && !r.IsProvider {
		// Generate the resource array input.
		pkg.genInputInterface(w, name+"Array")
		fmt.Fprintf(w, "type %[1]sArray []%[1]sInput\n\n", name)
		genInputImplementation(w, name+"Array", name+"Array", "[]*"+name, false)

		// Generate the resource map input.
		pkg.genInputInterface(w, name+"Map")
		fmt.Fprintf(w, "type %[1]sMap map[string]%[1]sInput\n\n", name)
		genInputImplementation(w, name+"Map", name+"Map", "map[string]*"+name, false)
	}

	// Emit the resource output type.
	genOutputType(w, name, "*"+name, false)

	// Emit chaining methods for the resource output type.
	for _, p := range r.Properties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, false)
		outputType := pkg.outputType(p.Type)

		propName := Title(p.Name)
		switch strings.ToLower(p.Name) {
		case "elementtype", "issecret":
			propName = "Get" + propName
		}
		fmt.Fprintf(w, "func (o %sOutput) %s() %s {\n", name, propName, outputType)
		fmt.Fprintf(w, "\treturn o.ApplyT(func (v *%s) %s { return v.%s }).(%s)\n",
			name, outputType, Title(p.Name), outputType)
		fmt.Fprintf(w, "}\n\n")
	}

	if generateResourceContainerTypes && !r.IsProvider {
		genArrayOutput(w, name, "*"+name)
		genMapOutput(w, name, "*"+name)
	}

	pkg.genResourceRegistrations(w, r, generateResourceContainerTypes)

	return nil
}

func NeedsGoOutputVersion(f *schema.Function) bool {
	fPkg := f.Package

	var goInfo GoPackageInfo

	contract.AssertNoError(fPkg.ImportLanguages(map[string]schema.Language{"go": Importer}))
	if info, ok := fPkg.Language["go"].(GoPackageInfo); ok {
		goInfo = info
	}

	if goInfo.DisableFunctionOutputVersions {
		return false
	}

	return f.NeedsOutputVersion()
}

func (pkg *pkgContext) genFunctionCodeFile(f *schema.Function) (string, error) {
	importsAndAliases := map[string]string{}
	pkg.getImports(f, importsAndAliases)
	importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumi"] = ""

	buffer := &bytes.Buffer{}

	var imports []string
	if NeedsGoOutputVersion(f) {
		imports = []string{"context", "reflect"}
	}

	pkg.genHeader(buffer, imports, importsAndAliases)
	if err := pkg.genFunction(buffer, f); err != nil {
		return "", err
	}
	pkg.genFunctionOutputVersion(buffer, f)
	return buffer.String(), nil
}

func (pkg *pkgContext) genFunction(w io.Writer, f *schema.Function) error {
	name := pkg.functionName(f)
	printCommentWithDeprecationMessage(w, f.Comment, f.DeprecationMessage, false)

	// Now, emit the function signature.
	argsig := "ctx *pulumi.Context"
	if f.Inputs != nil {
		argsig = fmt.Sprintf("%s, args *%sArgs", argsig, name)
	}
	var retty string
	if f.Outputs == nil {
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
	if f.Outputs == nil {
		outputsType = "struct{}"
	} else {
		outputsType = name + "Result"
	}

	pkg.GenPkgDefaultsOptsCall(w, true /*invoke*/)

	fmt.Fprintf(w, "\tvar rv %s\n", outputsType)
	fmt.Fprintf(w, "\terr := ctx.Invoke(\"%s\", %s, &rv, opts...)\n", f.Token, inputsVar)

	if f.Outputs == nil {
		fmt.Fprintf(w, "\treturn err\n")
	} else {
		// Check the error before proceeding.
		fmt.Fprintf(w, "\tif err != nil {\n")
		fmt.Fprintf(w, "\t\treturn nil, err\n")
		fmt.Fprintf(w, "\t}\n")

		// Return the result.
		var retValue string
		if codegen.IsProvideDefaultsFuncRequired(f.Outputs) && !pkg.disableObjectDefaults {
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
			if err := pkg.genObjectDefaultFunc(w, fnInputsName, f.Inputs.Properties); err != nil {
				return err
			}
		}
	}
	if f.Outputs != nil {
		fmt.Fprintf(w, "\n")
		fnOutputsName := pkg.functionResultTypeName(f)
		pkg.genPlainType(w, fnOutputsName, f.Outputs.Comment, "", f.Outputs.Properties)
		if codegen.IsProvideDefaultsFuncRequired(f.Outputs) && !pkg.disableObjectDefaults {
			if err := pkg.genObjectDefaultFunc(w, fnOutputsName, f.Outputs.Properties); err != nil {
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
	return fmt.Sprintf("%sArgs", name)
}

func (pkg *pkgContext) functionResultTypeName(f *schema.Function) string {
	name := pkg.functionName(f)
	return fmt.Sprintf("%sResult", name)
}

func (pkg *pkgContext) genFunctionOutputVersion(w io.Writer, f *schema.Function) {
	if !NeedsGoOutputVersion(f) {
		return
	}

	originalName := pkg.functionName(f)
	name := originalName + "Output"
	originalResultTypeName := pkg.functionResultTypeName(f)
	resultTypeName := originalResultTypeName + "Output"

	code := `
func ${fn}Output(ctx *pulumi.Context, args ${fn}OutputArgs, opts ...pulumi.InvokeOption) ${outputType} {
	return pulumi.ToOutputWithContext(context.Background(), args).
		ApplyT(func(v interface{}) (${fn}Result, error) {
			args := v.(${fn}Args)
			r, err := ${fn}(ctx, &args, opts...)
			var s ${fn}Result
			if r != nil {
				s = *r
			}
			return s, err
		}).(${outputType})
}

`

	code = strings.ReplaceAll(code, "${fn}", originalName)
	code = strings.ReplaceAll(code, "${outputType}", resultTypeName)
	fmt.Fprintf(w, code)

	pkg.genInputArgsStruct(w, name+"Args", f.Inputs.InputShape)

	genInputImplementationWithArgs(w, genInputImplementationArgs{
		name:         name + "Args",
		receiverType: name + "Args",
		elementType:  pkg.functionArgsTypeName(f),
	})

	pkg.genOutputTypes(w, genOutputTypesArgs{
		t:    f.Outputs,
		name: originalResultTypeName,
	})

	// Assuming the file represented by `w` only has one function,
	// generate an `init()` for Output type init.
	initCode := `
func init() {
        pulumi.RegisterOutputType(${outputType}{})
}

`
	initCode = strings.ReplaceAll(initCode, "${outputType}", resultTypeName)
	fmt.Fprintf(w, initCode)
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
//     type T struct {
//         Invalid T
//     }
//
// Indirectly invalid:
//
//     type T struct {
//         Invalid S
//     }
//
//     type S struct {
//         Invalid T
//     }
//
// In order to avoid generating invalid struct types, we replace all references to types involved in a cyclical
// definition with *T. The examples above therefore become:
//
// (1)
//     type T struct {
//         Valid *T
//     }
//
// (2)
//     type T struct {
//         Valid *S
//     }
//
//     type S struct {
//         Valid *T
//     }
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

func (pkg *pkgContext) genType(w io.Writer, obj *schema.ObjectType) error {
	contract.Assert(!obj.IsInputShape())
	if obj.IsOverlay {
		// This type is generated by the provider, so no further action is required.
		return nil
	}

	plainName := pkg.tokenToType(obj.Token)
	pkg.genPlainType(w, plainName, obj.Comment, "", obj.Properties)
	if !pkg.disableObjectDefaults {
		if err := pkg.genObjectDefaultFunc(w, plainName, obj.Properties); err != nil {
			return err
		}
	}

	if err := pkg.genInputTypes(w, obj.InputShape, pkg.detailsForType(obj)); err != nil {
		return err
	}
	pkg.genOutputTypes(w, genOutputTypesArgs{t: obj})
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

// collectNestedCollectionTypes builds a deduped mapping of element types -> associated collection types.
// different shapes of known types can resolve to the same element type. by collecting types in one step and emitting types
// in a second step, we avoid collision and redeclaration.
func (pkg *pkgContext) collectNestedCollectionTypes(types map[string]*nestedTypeInfo, typ schema.Type) {
	var elementTypeName string
	var names []string
	switch t := typ.(type) {
	case *schema.ArrayType:
		// Builtins already cater to primitive arrays
		if schema.IsPrimitiveType(t.ElementType) {
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
		if schema.IsPrimitiveType(t.ElementType) {
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
				genInputImplementation(w, name, name, "[]"+info.resolvedElementType, false)

				pkg.genInputInterface(w, name)
			case strings.HasSuffix(name, "ArrayOutput"):
				genArrayOutput(w, strings.TrimSuffix(name, "ArrayOutput"), info.resolvedElementType)
			case strings.HasSuffix(name, "MapInput"):
				name = strings.TrimSuffix(name, "Input")
				fmt.Fprintf(w, "type %s map[string]%sInput\n\n", name, elementTypeName)
				genInputImplementation(w, name, name, "map[string]"+info.resolvedElementType, false)

				pkg.genInputInterface(w, name)
			case strings.HasSuffix(name, "MapOutput"):
				genMapOutput(w, strings.TrimSuffix(name, "MapOutput"), info.resolvedElementType)
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
	}
	return strings.TrimSuffix(pkg.tokenToType(typ.String()), "Args")
}

func (pkg *pkgContext) genTypeRegistrations(w io.Writer, objTypes []*schema.ObjectType, types ...string) {
	fmt.Fprintf(w, "func init() {\n")

	// Input types.
	if !pkg.disableInputTypeRegistrations {
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
			if details.arrayInput {
				fmt.Fprintf(w,
					"\tpulumi.RegisterInputType(reflect.TypeOf((*%[1]sArrayInput)(nil)).Elem(), %[1]sArray{})\n", name)
			}
			if details.mapInput {
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
		if details.ptrOutput {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sPtrOutput{})\n", name)
		}
		if details.arrayOutput {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sArrayOutput{})\n", name)
		}
		if details.mapOutput {
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
			contract.Assert(len(e.Elements) > 0)
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

func (pkg *pkgContext) genResourceRegistrations(w io.Writer, r *schema.Resource, generateResourceContainerTypes bool) {
	name := disambiguatedResourceName(r, pkg)
	fmt.Fprintf(w, "func init() {\n")
	// Register input type
	if !pkg.disableInputTypeRegistrations {
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
		if method.Function.Outputs != nil {
			if pkg.liftSingleValueMethodReturns && len(method.Function.Outputs.Properties) == 1 {
				fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%s%sResultOutput{})\n", camel(name), Title(method.Name))
			} else {
				fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%s%sResultOutput{})\n", name, Title(method.Name))
			}
		}
	}

	if generateResourceContainerTypes && !r.IsProvider {
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

func extractImportBasePath(extPkg *schema.Package) string {
	version := extPkg.Version.Major
	var vPath string
	if version > 1 {
		vPath = fmt.Sprintf("/v%d", version)
	}
	return fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s", extPkg.Name, vPath, extPkg.Name)
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
				importsAndAliases["github.com/pkg/errors"] = ""
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
			if method.Function.Outputs != nil {
				for _, p := range method.Function.Outputs.Properties {
					pkg.getTypeImports(p.Type, false, importsAndAliases, seen)
				}
			}
		}
	case *schema.Function:
		if member.Inputs != nil {
			pkg.getTypeImports(member.Inputs, true, importsAndAliases, seen)
		}
		if member.Outputs != nil {
			pkg.getTypeImports(member.Outputs, true, importsAndAliases, seen)
		}
	case []*schema.Property:
		for _, p := range member {
			pkg.getTypeImports(p.Type, false, importsAndAliases, seen)
		}
	default:
		return
	}
}

func (pkg *pkgContext) genHeader(w io.Writer, goImports []string, importsAndAliases map[string]string) {
	fmt.Fprintf(w, "// Code generated by %v DO NOT EDIT.\n", pkg.tool)
	fmt.Fprintf(w, "// *** WARNING: Do not edit by hand unless you're certain you know what you are doing! ***\n\n")

	var pkgName string
	if pkg.mod == "" {
		pkgName = packageName(pkg.pkg)
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
	pkg.getImports(variables, importsAndAliases)

	pkg.genHeader(w, nil, importsAndAliases)

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
		configKey := fmt.Sprintf("\"%s:%s\"", pkg.pkg.Name, camel(p.Name))

		fmt.Fprintf(w, "func Get%s(ctx *pulumi.Context) %s {\n", Title(p.Name), getType)
		if p.DefaultValue != nil {
			defaultValue, err := pkg.getDefaultValue(p.DefaultValue, codegen.UnwrapType(p.Type))
			if err != nil {
				return err
			}

			fmt.Fprintf(w, "\tv, err := config.Try%s(ctx, %s)\n", funcType, configKey)
			fmt.Fprintf(w, "\tif err == nil {\n")
			fmt.Fprintf(w, "\t\treturn v\n")
			fmt.Fprintf(w, "\t}\n")
			fmt.Fprintf(w, "\treturn %s", defaultValue)
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
func (pkg *pkgContext) genResourceModule(w io.Writer) {
	contract.Assert(len(pkg.resources) != 0)
	allResourcesAreOverlays := true
	for _, r := range pkg.resources {
		if !r.IsOverlay {
			allResourcesAreOverlays = false
			break
		}
	}
	if allResourcesAreOverlays {
		// If all resources in this module are overlays, skip further code generation.
		return
	}

	basePath := pkg.importBasePath

	imports := map[string]string{
		"github.com/blang/semver":                   "",
		"github.com/pulumi/pulumi/sdk/v3/go/pulumi": "",
	}

	topLevelModule := pkg.mod == ""
	if !topLevelModule {
		if alias, ok := pkg.pkgImportAliases[basePath]; ok {
			imports[basePath] = alias
		} else {
			imports[basePath] = ""
		}
	}

	// If there are any internal dependencies, include them as blank imports.
	if topLevelModule {
		if goInfo, ok := pkg.pkg.Language["go"].(GoPackageInfo); ok {
			for _, dep := range goInfo.InternalDependencies {
				imports[dep] = "_"
			}
		}
	}

	pkg.genHeader(w, []string{"fmt"}, imports)

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
				contract.Assert(provider == nil)
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
		fmt.Fprintf(w, "\tif typ != \"pulumi:providers:%s\" {\n", pkg.pkg.Name)
		fmt.Fprintf(w, "\t\treturn nil, fmt.Errorf(\"unknown provider type: %%s\", typ)\n")
		fmt.Fprintf(w, "\t}\n\n")
		fmt.Fprintf(w, "\tr := &Provider{}\n")
		fmt.Fprintf(w, "\terr := ctx.RegisterResource(typ, name, nil, r, pulumi.URN_(urn))\n")
		fmt.Fprintf(w, "\treturn r, err\n")
		fmt.Fprintf(w, "}\n\n")
	}

	fmt.Fprintf(w, "func init() {\n")
	if topLevelModule {
		fmt.Fprintf(w, "\tversion, _ := PkgVersion()\n")
	} else {
		// Some package names contain '-' characters, so grab the name from the base path, unless there is an alias
		// in which case we use that instead.
		var pkgName string
		if alias, ok := pkg.pkgImportAliases[basePath]; ok {
			pkgName = alias
		} else {
			pkgName = basePath[strings.LastIndex(basePath, "/")+1:]
		}
		pkgName = strings.ReplaceAll(pkgName, "-", "")
		fmt.Fprintf(w, "\tversion, err := %s.PkgVersion()\n", pkgName)
		// To avoid breaking compatibility, we don't change the function
		// signature. We instead just ignore the error.
		fmt.Fprintf(w, "\tif err != nil {\n")
		fmt.Fprintf(w, "\t\tversion = semver.Version{Major: 1}\n")
		fmt.Fprintf(w, "\t}\n")
	}
	if len(registrations) > 0 {
		for _, mod := range registrations.SortedValues() {
			fmt.Fprintf(w, "\tpulumi.RegisterResourceModule(\n")
			fmt.Fprintf(w, "\t\t%q,\n", pkg.pkg.Name)
			fmt.Fprintf(w, "\t\t%q,\n", mod)
			fmt.Fprintf(w, "\t\t&module{version},\n")
			fmt.Fprintf(w, "\t)\n")
		}
	}
	if provider != nil {
		fmt.Fprintf(w, "\tpulumi.RegisterResourcePackage(\n")
		fmt.Fprintf(w, "\t\t%q,\n", pkg.pkg.Name)
		fmt.Fprintf(w, "\t\t&pkg{version},\n")
		fmt.Fprintf(w, "\t)\n")
	}
	fmt.Fprintf(w, "}\n")
}

// generatePackageContextMap groups resources, types, and functions into Go packages.
func generatePackageContextMap(tool string, pkg *schema.Package, goInfo GoPackageInfo) map[string]*pkgContext {
	packages := map[string]*pkgContext{}
	getPkg := func(mod string) *pkgContext {
		pack, ok := packages[mod]
		if !ok {
			pack = &pkgContext{
				pkg:                           pkg,
				mod:                           mod,
				importBasePath:                goInfo.ImportBasePath,
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
		default:
			return getPkgFromToken(t.String())
		}
	}

	if len(pkg.Config) > 0 {
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
	rewriteCyclicObjectFields(pkg)

	// Use a string set to track object types that have already been processed.
	// This avoids recursively processing the same type. For example, in the
	// Kubernetes package, JSONSchemaProps have properties whose type is itself.
	seenMap := codegen.NewStringSet()
	for _, t := range pkg.Types {
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
			names = append(names, camel(rawResourceName(r))+suffix+"Args")
			names = append(names, "New"+rawResourceName(r)+suffix)
			if !r.IsProvider && !r.IsComponent {
				names = append(names, rawResourceName(r)+suffix+"State")
				names = append(names, camel(rawResourceName(r))+suffix+"State")
				names = append(names, "Get"+rawResourceName(r)+suffix)
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
			panic(fmt.Sprintf("unable to generate Go SDK, schema has unresolvable overlapping resource: %s", rawResourceName(r)))
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
			if method.Function.Outputs != nil {
				pkg.names.Add(rawResourceName(r) + Title(method.Name) + "Result")
			}
		}
	}

	scanResource(pkg.Provider)
	for _, r := range pkg.Resources {
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
				panic(fmt.Sprintf("unable to generate Go SDK, schema has unresolvable overlapping type: %s", name))
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
				panic(fmt.Sprintf("unable to generate Go SDK, schema has unresolvable overlapping type: %s", name))
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

	for _, t := range pkg.Types {
		scanType(t)
	}

	// For fnApply function versions, we need to register any
	// input or output property type metadata, in case they have
	// types used in array or pointer element positions.
	if !goInfo.DisableFunctionOutputVersions || goInfo.GenerateExtraInputTypes {
		for _, f := range pkg.Functions {
			if f.NeedsOutputVersion() || goInfo.GenerateExtraInputTypes {
				optional := false
				if f.Inputs != nil {
					populateDetailsForPropertyTypes(seenMap, f.Inputs.InputShape.Properties, optional, false, false)
				}
				if f.Outputs != nil {
					populateDetailsForTypes(seenMap, f.Outputs, optional, false, true)
				}
			}
		}
	}

	for _, f := range pkg.Functions {
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

		if f.Inputs != nil {
			pkg.names.Add(name + "Args")
		}
		if f.Outputs != nil {
			pkg.names.Add(name + "Result")
		}
	}

	return packages
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
	packages := generatePackageContextMap(tool, pkg, goPkgInfo)

	// emit each package
	var pkgMods []string
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
func packageRoot(pkg *schema.Package) string {
	var info GoPackageInfo
	if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
		info = goInfo
	}
	if info.RootPackageName != "" {
		// package structure is flat
		return ""
	}
	if info.ImportBasePath != "" {
		return path.Base(info.ImportBasePath)
	}
	return goPackage(pkg.Name)
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
	return goPackage(packageRoot(pkg))
}

func GeneratePackage(tool string, pkg *schema.Package) (map[string][]byte, error) {
	if err := pkg.ImportLanguages(map[string]schema.Language{"go": Importer}); err != nil {
		return nil, err
	}

	var goPkgInfo GoPackageInfo
	if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
		goPkgInfo = goInfo
	}
	packages := generatePackageContextMap(tool, pkg, goPkgInfo)

	// emit each package
	var pkgMods []string
	for mod := range packages {
		pkgMods = append(pkgMods, mod)
	}
	sort.Strings(pkgMods)

	name := packageName(pkg)
	pathPrefix := packageRoot(pkg)

	files := map[string][]byte{}

	// Generate pulumi-plugin.json
	pulumiPlugin := &plugin.PulumiPluginJSON{
		Resource: true,
		Name:     pkg.Name,
		Server:   pkg.PluginDownloadURL,
	}
	if goPkgInfo.RespectSchemaVersion && pkg.Version != nil {
		pulumiPlugin.Version = pkg.Version.String()
	}
	pulumiPluginJSON, err := pulumiPlugin.JSON()
	if err != nil {
		return nil, fmt.Errorf("Failed to format pulumi-plugin.json: %w", err)
	}
	files[path.Join(pathPrefix, "pulumi-plugin.json")] = pulumiPluginJSON

	setFile := func(relPath, contents string) {
		relPath = path.Join(pathPrefix, relPath)
		if _, ok := files[relPath]; ok {
			panic(fmt.Errorf("duplicate file: %s", relPath))
		}

		// Run Go formatter on the code before saving to disk
		formattedSource, err := format.Source([]byte(contents))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid content:\n%s\n%s\n", relPath, contents)
			panic(fmt.Errorf("invalid Go source code:\n\n%s\n: %w", relPath, err))
		}

		files[relPath] = formattedSource
	}

	for _, mod := range pkgMods {
		pkg := packages[mod]

		// Config, description
		switch mod {
		case "":
			buffer := &bytes.Buffer{}
			if pkg.pkg.Description != "" {
				printComment(buffer, pkg.pkg.Description, false)
				fmt.Fprintf(buffer, "//\n")
			} else {
				fmt.Fprintf(buffer, "// Package %[1]s exports types, functions, subpackages for provisioning %[1]s resources.\n", pkg.pkg.Name)
				fmt.Fprintf(buffer, "//\n")
			}
			fmt.Fprintf(buffer, "package %s\n", name)

			setFile(path.Join(mod, "doc.go"), buffer.String())

		case "config":
			if len(pkg.pkg.Config) > 0 {
				buffer := &bytes.Buffer{}
				if err := pkg.genConfig(buffer, pkg.pkg.Config); err != nil {
					return nil, err
				}

				setFile(path.Join(mod, "config.go"), buffer.String())
			}
		}

		// Resources
		for _, r := range pkg.resources {
			if r.IsOverlay {
				// This resource code is generated by the provider, so no further action is required.
				continue
			}

			importsAndAliases := map[string]string{}
			pkg.getImports(r, importsAndAliases)
			importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumi"] = ""

			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, []string{"context", "reflect"}, importsAndAliases)

			if err := pkg.genResource(buffer, r, goPkgInfo.GenerateResourceContainerTypes); err != nil {
				return nil, err
			}

			setFile(path.Join(mod, camel(rawResourceName(r))+".go"), buffer.String())
		}

		// Functions
		for _, f := range pkg.functions {
			if f.IsOverlay {
				// This function code is generated by the provider, so no further action is required.
				continue
			}

			fileName := path.Join(mod, camel(tokenToName(f.Token))+".go")
			code, err := pkg.genFunctionCodeFile(f)
			if err != nil {
				return nil, err
			}
			setFile(fileName, code)
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
			}

			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, goImports, imports)

			for _, e := range pkg.enums {
				if err := pkg.genEnum(buffer, e); err != nil {
					return nil, err
				}
				delete(knownTypes, e)
			}
			pkg.genEnumRegistrations(buffer)
			setFile(path.Join(mod, "pulumiEnums.go"), buffer.String())
		}

		// Types
		if len(pkg.types) > 0 {
			hasOutputs, importsAndAliases := false, map[string]string{}
			for _, t := range pkg.types {
				pkg.getImports(t, importsAndAliases)
				hasOutputs = hasOutputs || pkg.detailsForType(t).hasOutputs()
			}

			sortedKnownTypes := make([]schema.Type, 0, len(knownTypes))
			for k := range knownTypes {
				sortedKnownTypes = append(sortedKnownTypes, k)
			}
			sort.Slice(sortedKnownTypes, func(i, j int) bool {
				return sortedKnownTypes[i].String() < sortedKnownTypes[j].String()
			})

			collectionTypes := map[string]*nestedTypeInfo{}
			for _, t := range sortedKnownTypes {
				pkg.collectNestedCollectionTypes(collectionTypes, t)
			}

			// All collection types have Outputs
			if len(collectionTypes) > 0 {
				hasOutputs = true
			}

			var goImports []string
			if hasOutputs {
				goImports = []string{"context", "reflect"}
				importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumi"] = ""
			}

			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, goImports, importsAndAliases)

			for _, t := range pkg.types {
				if err := pkg.genType(buffer, t); err != nil {
					return nil, err
				}
				delete(knownTypes, t)
			}

			types := pkg.genNestedCollectionTypes(buffer, collectionTypes)

			pkg.genTypeRegistrations(buffer, pkg.types, types...)

			setFile(path.Join(mod, "pulumiTypes.go"), buffer.String())
		}

		// Utilities
		if pkg.needsUtils || len(mod) == 0 {
			buffer := &bytes.Buffer{}
			importsAndAliases := map[string]string{
				"github.com/blang/semver":                   "",
				"github.com/pulumi/pulumi/sdk/v3/go/pulumi": "",
			}
			pkg.genHeader(buffer, []string{"fmt", "os", "reflect", "regexp", "strconv", "strings"}, importsAndAliases)

			packageRegex := fmt.Sprintf("^.*/pulumi-%s/sdk(/v\\d+)?", pkg.pkg.Name)
			if pkg.rootPackageName != "" {
				packageRegex = fmt.Sprintf("^%s(/v\\d+)?", pkg.importBasePath)
			}

			pkg.GenUtilitiesFile(buffer, packageRegex)

			setFile(path.Join(mod, "pulumiUtilities.go"), buffer.String())
		}

		// If there are resources in this module, register the module with the runtime.
		if len(pkg.resources) != 0 && !allResourcesAreOverlays(pkg.resources) {
			buffer := &bytes.Buffer{}
			pkg.genResourceModule(buffer)

			setFile(path.Join(mod, "init.go"), buffer.String())
		}
	}

	return files, nil
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

func (pkg *pkgContext) GenUtilitiesFile(w io.Writer, packageRegex string) {
	const utilitiesFile = `
type envParser func(v string) interface{}

func parseEnvBool(v string) interface{} {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return nil
	}
	return b
}

func parseEnvInt(v string) interface{} {
	i, err := strconv.ParseInt(v, 0, 0)
	if err != nil {
		return nil
	}
	return int(i)
}

func parseEnvFloat(v string) interface{} {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return f
}

func parseEnvStringArray(v string) interface{} {
	var result pulumi.StringArray
	for _, item := range strings.Split(v, ";") {
		result = append(result, pulumi.String(item))
	}
	return result
}

func getEnvOrDefault(def interface{}, parser envParser, vars ...string) interface{} {
	for _, v := range vars {
		if value := os.Getenv(v); value != "" {
			if parser != nil {
				return parser(value)
			}
			return value
		}
	}
	return def
}

// PkgVersion uses reflection to determine the version of the current package.
// If a version cannot be determined, v1 will be assumed. The second return
// value is always nil.
func PkgVersion() (semver.Version, error) {
	type sentinal struct{}
	pkgPath := reflect.TypeOf(sentinal{}).PkgPath()
	re := regexp.MustCompile(%q)
	if match := re.FindStringSubmatch(pkgPath); match != nil {
		vStr := match[1]
		if len(vStr) == 0 { // If the version capture group was empty, default to v1.
			return semver.Version{Major: 1}, nil
		}
		return semver.MustParse(fmt.Sprintf("%%s.0.0", vStr[2:])), nil
	}
	return semver.Version{Major: 1}, nil
}

// isZero is a null safe check for if a value is it's types zero value.
func isZero(v interface{}) bool {
	if v == nil {
		return true
	}
	return reflect.ValueOf(v).IsZero()
}
`
	_, err := fmt.Fprintf(w, utilitiesFile, packageRegex)
	contract.AssertNoError(err)
	pkg.GenPkgDefaultOpts(w)
}

func (pkg *pkgContext) GenPkgDefaultOpts(w io.Writer) {
	url := pkg.pkg.PluginDownloadURL
	if url == "" {
		return
	}
	const template string = `
// pkg%[1]sDefaultOpts provides package level defaults to pulumi.Option%[1]s.
func pkg%[1]sDefaultOpts(opts []pulumi.%[1]sOption) []pulumi.%[1]sOption {
	defaults := []pulumi.%[1]sOption{%[2]s%[3]s}

	return append(defaults, opts...)
}
`
	pluginDownloadURL := fmt.Sprintf("pulumi.PluginDownloadURL(%q)", url)
	version := ""
	if info := pkg.pkg.Language["go"]; info != nil {
		if info.(GoPackageInfo).RespectSchemaVersion && pkg.pkg.Version != nil {
			version = fmt.Sprintf(", pulumi.Version(%q)", pkg.pkg.Version.String())
		}
	}
	for _, typ := range []string{"Resource", "Invoke"} {
		_, err := fmt.Fprintf(w, template, typ, pluginDownloadURL, version)
		contract.AssertNoError(err)
	}
}

// GenPkgDefaultsOptsCall generates a call to Pkg{TYPE}DefaultsOpts.
func (pkg *pkgContext) GenPkgDefaultsOptsCall(w io.Writer, invoke bool) {
	// The `pkg%sDefaultOpts` call won't do anything, so we don't insert it.
	if pkg.pkg.PluginDownloadURL == "" {
		return
	}
	pkg.needsUtils = true
	typ := "Resource"
	if invoke {
		typ = "Invoke"
	}
	_, err := fmt.Fprintf(w, "\topts = pkg%sDefaultOpts(opts)\n", typ)
	contract.AssertNoError(err)
}
