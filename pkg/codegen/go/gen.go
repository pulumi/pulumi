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

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type typeDetails struct {
	ptrElement   bool
	arrayElement bool
	mapElement   bool
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
	schemaNames   codegen.StringSet
	names         codegen.StringSet
	renamed       map[string]string
	functionNames map[*schema.Function]string
	needsUtils    bool
	tool          string
	packages      map[string]*pkgContext

	// Name overrides set in GoPackageInfo
	modToPkg         map[string]string // Module name -> package name
	pkgImportAliases map[string]string // Package name -> import alias
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

	modPkg, ok := pkg.packages[mod]
	name = Title(name)

	if ok {
		newName, renamed := modPkg.renamed[name]
		if renamed {
			name = newName
		} else if modPkg.names.Has(name) {
			// If the package containing the type's token already has a resource with the
			// same name, add a `Type` suffix.
			newName = name + "Type"
			modPkg.renamed[name] = newName
			modPkg.names.Add(newName)
			name = newName
		}
	}

	if mod == pkg.mod {
		return name
	}
	if mod == "" {
		mod = components[0]
	}
	mod = strings.Replace(mod, "/", "", -1) + "." + name
	return strings.Replace(mod, "-provider", "", -1)
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
	return strings.Replace(mod, "/", "", -1) + "." + name
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

func resourceName(r *schema.Resource) string {
	if r.IsProvider {
		return "Provider"
	}
	return tokenToName(r.Token)
}

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
		return pkg.tokenToEnum(t.Token) + "Input"
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
		return pkg.tokenToEnum(t.Token)
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
		return pkg.tokenToEnum(t.Token)
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
	return pkg.typeStringImpl(t, false)
}

func (pkg *pkgContext) isExternalReference(t schema.Type) bool {
	switch typ := t.(type) {
	case *schema.ObjectType:
		return typ.Package != nil && pkg.pkg != nil && typ.Package != pkg.pkg
	case *schema.ResourceType:
		return typ.Resource != nil && pkg.pkg != nil && typ.Resource.Package != pkg.pkg
	}
	return false
}

func (pkg *pkgContext) isExternalObjectType(t schema.Type) bool {
	obj, ok := t.(*schema.ObjectType)
	return ok && obj.Package != nil && pkg.pkg != nil && obj.Package != pkg.pkg
}

// resolveResourceType resolves resource references in properties while
// taking into account potential external resources. Returned type is
// always marked as required. Caller should check if the property is
// optional and convert the type to a pointer if necessary.
func (pkg *pkgContext) resolveResourceType(t *schema.ResourceType) string {
	if !pkg.isExternalReference(t) {
		return pkg.tokenToResource(t.Token)
	}
	extPkg := t.Resource.Package
	var goInfo GoPackageInfo

	contract.AssertNoError(extPkg.ImportLanguages(map[string]schema.Language{"go": Importer}))
	if info, ok := extPkg.Language["go"].(GoPackageInfo); ok {
		goInfo = info
	}
	extPkgCtx := &pkgContext{
		pkg:              extPkg,
		importBasePath:   goInfo.ImportBasePath,
		pkgImportAliases: goInfo.PackageImportAliases,
		modToPkg:         goInfo.ModuleToPackage,
	}
	resType := extPkgCtx.tokenToResource(t.Token)
	if !strings.Contains(resType, ".") {
		resType = fmt.Sprintf("%s.%s", extPkg.Name, resType)
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
	extPkg := t.Package
	var goInfo GoPackageInfo

	contract.AssertNoError(extPkg.ImportLanguages(map[string]schema.Language{"go": Importer}))
	if info, ok := extPkg.Language["go"].(GoPackageInfo); ok {
		goInfo = info
	}
	extPkgCtx := &pkgContext{
		pkg:              extPkg,
		importBasePath:   goInfo.ImportBasePath,
		pkgImportAliases: goInfo.PackageImportAliases,
		modToPkg:         goInfo.ModuleToPackage,
	}
	return extPkgCtx.typeString(t)
}

func (pkg *pkgContext) outputType(t schema.Type) string {
	switch t := t.(type) {
	case *schema.OptionalType:
		elem := pkg.outputType(t.ElementType)
		if isNilType(t.ElementType) || elem == "pulumi.AnyOutput" {
			return elem
		}
		return strings.TrimSuffix(elem, "Output") + "PtrOutput"
	case *schema.EnumType:
		return pkg.tokenToEnum(t.Token) + "Output"
	case *schema.ArrayType:
		en := strings.TrimSuffix(pkg.outputType(t.ElementType), "Output")
		if en == "pulumi.Any" {
			return "pulumi.ArrayOutput"
		}
		return en + "ArrayOutput"
	case *schema.MapType:
		en := strings.TrimSuffix(pkg.outputType(t.ElementType), "Output")
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
			return pkg.outputType(t.UnderlyingType)
		}
		return pkg.tokenToType(t.Token) + "Output"
	case *schema.UnionType:
		// If the union is actually a relaxed enum type, use the underlying
		// type for the output instead
		for _, e := range t.ElementTypes {
			if typ, ok := e.(*schema.EnumType); ok {
				return pkg.outputType(typ.ElementType)
			}
		}
		// TODO(pdg): union types
		return "pulumi.AnyOutput"
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

// genResourceContainerInput handles generating container (slice/map) wrappers around
// resources to facilitate external references.
func genResourceContainerInput(w io.Writer, name, receiverType, elementType string) {
	fmt.Fprintf(w, "func (%s) ElementType() reflect.Type {\n", receiverType)
	fmt.Fprintf(w, "\treturn reflect.TypeOf((%s)(nil))\n", elementType)
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (i %s) To%sOutput() %sOutput {\n", receiverType, Title(name), name)
	fmt.Fprintf(w, "\treturn i.To%sOutputWithContext(context.Background())\n", Title(name))
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (i %s) To%sOutputWithContext(ctx context.Context) %sOutput {\n", receiverType, Title(name), name)
	if strings.HasSuffix(name, "Ptr") {
		base := name[:len(name)-3]
		fmt.Fprintf(w, "\treturn pulumi.ToOutputWithContext(ctx, i).(%sOutput).To%sOutput()\n", base, Title(name))
	} else {
		fmt.Fprintf(w, "\treturn pulumi.ToOutputWithContext(ctx, i).(%sOutput)\n", name)
	}
	fmt.Fprintf(w, "}\n\n")
}

func genInputMethods(w io.Writer, name, receiverType, elementType string, ptrMethods, resourceType bool) {
	fmt.Fprintf(w, "func (%s) ElementType() reflect.Type {\n", receiverType)
	if resourceType {
		fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s)(nil))\n", elementType)
	} else {
		fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s)(nil)).Elem()\n", elementType)
	}
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (i %s) To%sOutput() %sOutput {\n", receiverType, Title(name), name)
	fmt.Fprintf(w, "\treturn i.To%sOutputWithContext(context.Background())\n", Title(name))
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (i %s) To%sOutputWithContext(ctx context.Context) %sOutput {\n", receiverType, Title(name), name)
	fmt.Fprintf(w, "\treturn pulumi.ToOutputWithContext(ctx, i).(%sOutput)\n", name)
	fmt.Fprintf(w, "}\n\n")

	if ptrMethods {
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

func (pkg *pkgContext) genEnum(w io.Writer, enum *schema.EnumType) error {
	return pkg.genEnumType(w, pkg.tokenToEnum(enum.Token), enum)
}

func (pkg *pkgContext) genEnumType(w io.Writer, name string, enumType *schema.EnumType) error {
	mod := pkg.tokenToPackage(enumType.Token)
	modPkg, ok := pkg.packages[mod]
	contract.Assert(ok)
	printCommentWithDeprecationMessage(w, enumType.Comment, "", false)
	elementType := pkg.enumElementType(enumType.ElementType, false)
	goElementType := enumType.ElementType.String()
	switch goElementType {
	case "integer":
		goElementType = "int"
	case "number":
		goElementType = "float64"
	}
	asFuncName := strings.TrimPrefix(elementType, "pulumi.")

	fmt.Fprintf(w, "type %s %s\n\n", name, goElementType)

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

	inputType := pkg.inputType(enumType)
	pkg.genEnumInputFuncs(w, name, enumType, elementType, inputType, asFuncName)

	pkg.genEnumOutputTypes(w, name, elementType, goElementType, asFuncName)
	pkg.genEnumInputTypes(w, name, enumType, goElementType)

	details := pkg.detailsForType(enumType)
	// Generate the array input.
	if details.arrayElement {
		pkg.genInputInterface(w, name+"Array")

		fmt.Fprintf(w, "type %[1]sArray []%[1]s\n\n", name)

		genInputMethods(w, name+"Array", name+"Array", "[]"+name, false, false)
	}

	// Generate the map input.
	if details.mapElement {
		pkg.genInputInterface(w, name+"Map")

		fmt.Fprintf(w, "type %[1]sMap map[string]%[1]s\n\n", name)

		genInputMethods(w, name+"Map", name+"Map", "map[string]"+name, false, false)
	}

	// Generate the array output
	if details.arrayElement {
		fmt.Fprintf(w, "type %sArrayOutput struct { *pulumi.OutputState }\n\n", name)

		genOutputMethods(w, name+"Array", "[]"+name, false)

		fmt.Fprintf(w, "func (o %[1]sArrayOutput) Index(i pulumi.IntInput) %[1]sOutput {\n", name)
		fmt.Fprintf(w, "\treturn pulumi.All(o, i).ApplyT(func (vs []interface{}) %sOutput {\n", name)
		fmt.Fprintf(w, "\t\treturn vs[0].([]%[1]s)[vs[1].(int)].To%[1]sOutput()\n", name)
		fmt.Fprintf(w, "\t}).(%sOutput)\n", name)
		fmt.Fprintf(w, "}\n\n")
	}

	// Generate the map output.
	if details.mapElement {
		fmt.Fprintf(w, "type %sMapOutput struct { *pulumi.OutputState }\n\n", name)

		genOutputMethods(w, name+"Map", "map[string]"+name, false)

		fmt.Fprintf(w, "func (o %[1]sMapOutput) MapIndex(k pulumi.StringInput) %[1]sOutput {\n", name)
		fmt.Fprintf(w, "\treturn pulumi.All(o, k).ApplyT(func (vs []interface{}) %sOutput {\n", name)
		fmt.Fprintf(w, "\t\treturn vs[0].(map[string]%[1]s)[vs[1].(string)].To%[1]sOutput()\n", name)
		fmt.Fprintf(w, "\t}).(%sOutput)\n", name)
		fmt.Fprintf(w, "}\n\n")
	}

	return nil
}

func (pkg *pkgContext) genEnumOutputTypes(w io.Writer, name, elementType, goElementType, asFuncName string) {
	fmt.Fprintf(w, "type %sOutput struct{ *pulumi.OutputState }\n\n", name)
	genOutputMethods(w, name, name, false)

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[1]sPtrOutput() %[1]sPtrOutput {\n", name)
	fmt.Fprintf(w, "return o.To%sPtrOutputWithContext(context.Background())\n", name)
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[1]sPtrOutputWithContext(ctx context.Context) %[1]sPtrOutput {\n", name)
	fmt.Fprintf(w, "return o.ApplyTWithContext(ctx, func(_ context.Context, v %[1]s) *%[1]s {\n", name)
	fmt.Fprintf(w, "return &v\n")
	fmt.Fprintf(w, "}).(%sPtrOutput)\n", name)
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sOutput() %[3]sOutput {\n", name, asFuncName, elementType)
	fmt.Fprintf(w, "return o.To%sOutputWithContext(context.Background())\n", asFuncName)
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sOutputWithContext(ctx context.Context) %[3]sOutput {\n", name, asFuncName, elementType)
	fmt.Fprintf(w, "return o.ApplyTWithContext(ctx, func(_ context.Context, e %s) %s {\n", name, goElementType)
	fmt.Fprintf(w, "return %s(e)\n", goElementType)
	fmt.Fprintf(w, "}).(%sOutput)\n", elementType)
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutput() %[3]sPtrOutput {\n", name, asFuncName, elementType)
	fmt.Fprintf(w, "return o.To%sPtrOutputWithContext(context.Background())\n", asFuncName)
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutputWithContext(ctx context.Context) %[3]sPtrOutput {\n", name, asFuncName, elementType)
	fmt.Fprintf(w, "return o.ApplyTWithContext(ctx, func(_ context.Context, e %s) *%s {\n", name, goElementType)
	fmt.Fprintf(w, "v := %s(e)\n", goElementType)
	fmt.Fprintf(w, "return &v\n")
	fmt.Fprintf(w, "}).(%sPtrOutput)\n", elementType)
	fmt.Fprint(w, "}\n\n")

	ptrName := name + "Ptr"
	fmt.Fprintf(w, "type %sOutput struct{ *pulumi.OutputState }\n\n", ptrName)

	fmt.Fprintf(w, "func (%[1]sPtrOutput) ElementType() reflect.Type {\n", name)
	fmt.Fprintf(w, "return %sPtrType\n", camel(name))
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sPtrOutput) To%[1]sPtrOutput() %[1]sPtrOutput {\n", name)
	fmt.Fprintf(w, "return o\n")
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sPtrOutput) To%[1]sPtrOutputWithContext(ctx context.Context) %[1]sPtrOutput {\n", name)
	fmt.Fprintf(w, "return o\n")
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sPtrOutput) To%[2]sPtrOutput() %[3]sPtrOutput {\n", name, asFuncName, elementType)
	fmt.Fprintf(w, "return o.To%sPtrOutputWithContext(context.Background())\n", asFuncName)
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sPtrOutput) To%[2]sPtrOutputWithContext(ctx context.Context) %[3]sPtrOutput {\n", name, asFuncName, elementType)
	fmt.Fprintf(w, "return o.ApplyTWithContext(ctx, func(_ context.Context, e *%s) *%s {\n", name, goElementType)
	fmt.Fprintf(w, "if e == nil {\n")
	fmt.Fprintf(w, "return nil\n")
	fmt.Fprintf(w, "}\n")
	fmt.Fprintf(w, "v := %s(*e)\n", goElementType)
	fmt.Fprintf(w, "return &v\n")
	fmt.Fprintf(w, "}).(%sPtrOutput)\n", elementType)
	fmt.Fprint(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sPtrOutput) Elem() %[1]sOutput {\n", name)
	fmt.Fprintf(w, "return o.ApplyT(func(v *%[1]s) %[1]s {\n", name)
	fmt.Fprintf(w, "var ret %s\n", name)
	fmt.Fprint(w, "if v != nil {\n")
	fmt.Fprint(w, "ret = *v\n")
	fmt.Fprint(w, "}\n")
	fmt.Fprint(w, "return ret\n")
	fmt.Fprintf(w, "}).(%sOutput)\n", name)
	fmt.Fprint(w, "}\n\n")
}

func (pkg *pkgContext) genEnumInputTypes(w io.Writer, name string, enumType *schema.EnumType, goElementType string) {
	pkg.genInputInterface(w, name)

	fmt.Fprintf(w, "var %sPtrType = reflect.TypeOf((**%s)(nil)).Elem()\n", camel(name), name)
	fmt.Fprintln(w)

	fmt.Fprintf(w, "type %sPtrInput interface {\n", name)
	fmt.Fprint(w, "pulumi.Input\n\n")
	fmt.Fprintf(w, "To%[1]sPtrOutput() %[1]sPtrOutput\n", name)
	fmt.Fprintf(w, "To%[1]sPtrOutputWithContext(context.Context) %[1]sPtrOutput\n", name)
	fmt.Fprintf(w, "}\n")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "type %sPtr %s\n", camel(name), goElementType)
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func %[1]sPtr(v %[2]s) %[1]sPtrInput {\n", name, goElementType)
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

func (pkg *pkgContext) enumElementType(t schema.Type, optional bool) string {
	suffix := ""
	if optional {
		suffix = "Ptr"
	}
	switch t {
	case schema.BoolType:
		return "pulumi.Bool" + suffix
	case schema.IntType:
		return "pulumi.Int" + suffix
	case schema.NumberType:
		return "pulumi.Float64" + suffix
	case schema.StringType:
		return "pulumi.String" + suffix
	default:
		// We only expect to support the above element types for enums
		panic(fmt.Sprintf("Invalid enum type: %s", t))
	}
}

func (pkg *pkgContext) genEnumInputFuncs(w io.Writer, typeName string, enum *schema.EnumType, elementType, inputType, asFuncName string) {
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

	fmt.Fprintf(w, "func (e %[1]s) To%[2]sOutput() %[3]sOutput {\n", typeName, asFuncName, elementType)
	fmt.Fprintf(w, "return pulumi.ToOutput(%[1]s(e)).(%[1]sOutput)\n", elementType)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (e %[1]s) To%[2]sOutputWithContext(ctx context.Context) %[3]sOutput {\n", typeName, asFuncName, elementType)
	fmt.Fprintf(w, "return pulumi.ToOutputWithContext(ctx, %[1]s(e)).(%[1]sOutput)\n", elementType)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (e %[1]s) To%[2]sPtrOutput() %[3]sPtrOutput {\n", typeName, asFuncName, elementType)
	fmt.Fprintf(w, "return %s(e).To%sPtrOutputWithContext(context.Background())\n", elementType, asFuncName)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "func (e %[1]s) To%[2]sPtrOutputWithContext(ctx context.Context) %[3]sPtrOutput {\n", typeName, asFuncName, elementType)
	fmt.Fprintf(w, "return %[1]s(e).To%[2]sOutputWithContext(ctx).To%[2]sPtrOutputWithContext(ctx)\n", elementType, asFuncName)
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)
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

func (pkg *pkgContext) genInputTypes(w io.Writer, t *schema.ObjectType, details *typeDetails) {
	contract.Assert(t.IsInputShape())

	name := pkg.tokenToType(t.Token)

	// Generate the plain inputs.
	pkg.genInputInterface(w, name)

	printComment(w, t.Comment, false)
	fmt.Fprintf(w, "type %sArgs struct {\n", name)
	for _, p := range t.Properties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", Title(p.Name), pkg.typeString(p.Type), p.Name)
	}
	fmt.Fprintf(w, "}\n\n")

	genInputMethods(w, name, name+"Args", name, details.ptrElement, false)

	// Generate the pointer input.
	if details.ptrElement {
		pkg.genInputInterface(w, name+"Ptr")

		ptrTypeName := camel(name) + "PtrType"

		fmt.Fprintf(w, "type %s %sArgs\n\n", ptrTypeName, name)

		fmt.Fprintf(w, "func %[1]sPtr(v *%[1]sArgs) %[1]sPtrInput {", name)
		fmt.Fprintf(w, "\treturn (*%s)(v)\n", ptrTypeName)
		fmt.Fprintf(w, "}\n\n")

		genInputMethods(w, name+"Ptr", "*"+ptrTypeName, "*"+name, false, false)
	}

	// Generate the array input.
	if details.arrayElement {
		pkg.genInputInterface(w, name+"Array")

		fmt.Fprintf(w, "type %[1]sArray []%[1]sInput\n\n", name)

		genInputMethods(w, name+"Array", name+"Array", "[]"+name, false, false)
	}

	// Generate the map input.
	if details.mapElement {
		pkg.genInputInterface(w, name+"Map")

		fmt.Fprintf(w, "type %[1]sMap map[string]%[1]sInput\n\n", name)

		genInputMethods(w, name+"Map", name+"Map", "map[string]"+name, false, false)
	}
}

func genOutputMethods(w io.Writer, name, elementType string, resourceType bool) {
	fmt.Fprintf(w, "func (%sOutput) ElementType() reflect.Type {\n", name)
	if resourceType {
		fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s)(nil))\n", elementType)
	} else {
		fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s)(nil)).Elem()\n", elementType)
	}
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sOutput() %[1]sOutput {\n", name, Title(name))
	fmt.Fprintf(w, "\treturn o\n")
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sOutputWithContext(ctx context.Context) %[1]sOutput {\n", name, Title(name))
	fmt.Fprintf(w, "\treturn o\n")
	fmt.Fprintf(w, "}\n\n")
}

func (pkg *pkgContext) genOutputTypes(w io.Writer, t *schema.ObjectType, details *typeDetails) {
	contract.Assert(!t.IsInputShape())

	name := pkg.tokenToType(t.Token)

	printComment(w, t.Comment, false)
	fmt.Fprintf(w, "type %sOutput struct { *pulumi.OutputState }\n\n", name)

	genOutputMethods(w, name, name, false)

	if details.ptrElement {
		fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutput() %[1]sPtrOutput {\n", name, Title(name))
		fmt.Fprintf(w, "\treturn o.To%sPtrOutputWithContext(context.Background())\n", Title(name))
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutputWithContext(ctx context.Context) %[1]sPtrOutput {\n", name, Title(name))
		fmt.Fprintf(w, "\treturn o.ApplyT(func(v %[1]s) *%[1]s {\n", name)
		fmt.Fprintf(w, "\t\treturn &v\n")
		fmt.Fprintf(w, "\t}).(%sPtrOutput)\n", name)
		fmt.Fprintf(w, "}\n")
	}

	for _, p := range t.Properties {
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, false)
		outputType, applyType := pkg.outputType(p.Type), pkg.typeString(p.Type)

		propName := Title(p.Name)
		switch strings.ToLower(p.Name) {
		case "elementtype", "issecret":
			propName = "Get" + propName
		}
		fmt.Fprintf(w, "func (o %sOutput) %s() %s {\n", name, propName, outputType)
		fmt.Fprintf(w, "\treturn o.ApplyT(func (v %s) %s { return v.%s }).(%s)\n", name, applyType, Title(p.Name), outputType)
		fmt.Fprintf(w, "}\n\n")
	}

	if details.ptrElement {
		fmt.Fprintf(w, "type %sPtrOutput struct { *pulumi.OutputState }\n\n", name)

		genOutputMethods(w, name+"Ptr", "*"+name, false)

		fmt.Fprintf(w, "func (o %[1]sPtrOutput) Elem() %[1]sOutput {\n", name)
		fmt.Fprintf(w, "\treturn o.ApplyT(func (v *%[1]s) %[1]s { return *v }).(%[1]sOutput)\n", name)
		fmt.Fprintf(w, "}\n\n")

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

	if details.arrayElement {
		fmt.Fprintf(w, "type %sArrayOutput struct { *pulumi.OutputState }\n\n", name)

		genOutputMethods(w, name+"Array", "[]"+name, false)

		fmt.Fprintf(w, "func (o %[1]sArrayOutput) Index(i pulumi.IntInput) %[1]sOutput {\n", name)
		fmt.Fprintf(w, "\treturn pulumi.All(o, i).ApplyT(func (vs []interface{}) %s {\n", name)
		fmt.Fprintf(w, "\t\treturn vs[0].([]%s)[vs[1].(int)]\n", name)
		fmt.Fprintf(w, "\t}).(%sOutput)\n", name)
		fmt.Fprintf(w, "}\n\n")
	}

	if details.mapElement {
		fmt.Fprintf(w, "type %sMapOutput struct { *pulumi.OutputState }\n\n", name)

		genOutputMethods(w, name+"Map", "map[string]"+name, false)

		fmt.Fprintf(w, "func (o %[1]sMapOutput) MapIndex(k pulumi.StringInput) %[1]sOutput {\n", name)
		fmt.Fprintf(w, "\treturn pulumi.All(o, k).ApplyT(func (vs []interface{}) %s {\n", name)
		fmt.Fprintf(w, "\t\treturn vs[0].(map[string]%s)[vs[1].(string)]\n", name)
		fmt.Fprintf(w, "\t}).(%sOutput)\n", name)
		fmt.Fprintf(w, "}\n\n")
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
		return strconv.FormatFloat(v.Float(), 'f', -1, 64), nil
	case reflect.String:
		return fmt.Sprintf("%q", v.String()), nil
	default:
		return "", errors.Errorf("unsupported default value of type %T", value)
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
	name := resourceName(r)

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
	for _, p := range r.InputProperties {
		if p.IsRequired() && isNilType(p.Type) && p.DefaultValue == nil {
			fmt.Fprintf(w, "\tif args.%s == nil {\n", Title(p.Name))
			fmt.Fprintf(w, "\t\treturn nil, errors.New(\"invalid value for required argument '%s'\")\n", Title(p.Name))
			fmt.Fprintf(w, "\t}\n")
		}
	}

	for _, p := range r.InputProperties {
		if p.ConstValue != nil {
			v, err := pkg.getConstValue(p.ConstValue)
			if err != nil {
				return err
			}

			t := strings.TrimSuffix(pkg.inputType(p.Type), "Input")
			if t == "pulumi." {
				t = "pulumi.Any"
			}

			fmt.Fprintf(w, "\targs.%s = %s(%s)\n", Title(p.Name), t, v)
		}
		if p.DefaultValue != nil {
			v, err := pkg.getDefaultValue(p.DefaultValue, codegen.UnwrapType(p.Type))
			if err != nil {
				return err
			}

			t := strings.TrimSuffix(pkg.inputType(p.Type), "Input")
			if t == "pulumi." {
				t = "pulumi.Any"
			}

			switch codegen.UnwrapType(p.Type).(type) {
			case *schema.EnumType:
				fmt.Fprintf(w, "\tif args.%s == nil {\n", Title(p.Name))

				fmt.Fprintf(w, "\t\targs.%s = %s(%s)\n", Title(p.Name), strings.TrimSuffix(t, "Ptr"), v)
				fmt.Fprintf(w, "\t}\n")
			default:
				fmt.Fprintf(w, "\tif args.%s == nil {\n", Title(p.Name))
				fmt.Fprintf(w, "\t\targs.%s = %s(%s)\n", Title(p.Name), t, v)
				fmt.Fprintf(w, "\t}\n")
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
	if len(secretProps) > 0 {
		for _, p := range secretProps {
			fmt.Fprintf(w, "\tif args.%s != nil {\n", Title(p.Name))
			fmt.Fprintf(w, "\t\targs.%[1]s = pulumi.ToSecret(args.%[1]s).(%[2]s)\n", Title(p.Name), pkg.outputType(p.Type))
			fmt.Fprintf(w, "\t}\n")
		}
		fmt.Fprintf(w, "\tsecrets := pulumi.AdditionalSecretOutputs([]string{\n")
		for _, sp := range secretProps {
			fmt.Fprintf(w, "\t\t\t%q,\n", sp.Name)
		}
		fmt.Fprintf(w, "\t})\n")
		fmt.Fprintf(w, "\topts = append(opts, secrets)\n")
	}

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
		printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, true)
		fmt.Fprintf(w, "\t%s %s\n", Title(p.Name), pkg.typeString(p.Type))
	}
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (%sArgs) ElementType() reflect.Type {\n", name)
	fmt.Fprintf(w, "\treturn reflect.TypeOf((*%sArgs)(nil)).Elem()\n", camel(name))
	fmt.Fprintf(w, "}\n")

	// Emit resource methods.
	for _, method := range r.Methods {
		methodName := Title(method.Name)
		f := method.Function

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
			outputsType = fmt.Sprintf("%s%sResultOutput", name, methodName)
		}
		fmt.Fprintf(w, "\t%s, err := ctx.Call(%q, %s, %s{}, r)\n", resultVar, f.Token, inputsVar, outputsType)
		if f.Outputs == nil {
			fmt.Fprintf(w, "\treturn err\n")
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
			fmt.Fprintf(w, "\n")
			pkg.genPlainType(w, fmt.Sprintf("%s%sResult", name, methodName), f.Outputs.Comment, "",
				f.Outputs.Properties)

			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "type %s%sResultOutput struct{ *pulumi.OutputState }\n\n", name, methodName)

			fmt.Fprintf(w, "func (%s%sResultOutput) ElementType() reflect.Type {\n", name, methodName)
			fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s%sResult)(nil)).Elem()\n", name, methodName)
			fmt.Fprintf(w, "}\n")

			for _, p := range f.Outputs.Properties {
				fmt.Fprintf(w, "\n")
				printCommentWithDeprecationMessage(w, p.Comment, p.DeprecationMessage, false)
				fmt.Fprintf(w, "func (o %s%sResultOutput) %s() %s {\n", name, methodName, Title(p.Name),
					pkg.outputType(p.Type))
				fmt.Fprintf(w, "\treturn o.ApplyT(func(v %s%sResult) %s { return v.%s }).(%s)\n", name, methodName,
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

	genInputMethods(w, name, "*"+name, name, generateResourceContainerTypes, true)

	if generateResourceContainerTypes {
		// Emit the resource pointer input type.
		fmt.Fprintf(w, "type %sPtrInput interface {\n", name)
		fmt.Fprintf(w, "\tpulumi.Input\n\n")
		fmt.Fprintf(w, "\tTo%[1]sPtrOutput() %[1]sPtrOutput\n", name)
		fmt.Fprintf(w, "\tTo%[1]sPtrOutputWithContext(ctx context.Context) %[1]sPtrOutput\n", name)
		fmt.Fprintf(w, "}\n\n")
		ptrTypeName := camel(name) + "PtrType"
		fmt.Fprintf(w, "type %s %sArgs\n\n", ptrTypeName, name)
		genInputMethods(w, name+"Ptr", "*"+ptrTypeName, "*"+name, false, true)

		if !r.IsProvider {
			// Generate the resource array input.
			pkg.genInputInterface(w, name+"Array")
			fmt.Fprintf(w, "type %[1]sArray []%[1]sInput\n\n", name)
			genResourceContainerInput(w, name+"Array", name+"Array", "[]*"+name)

			// Generate the resource map input.
			pkg.genInputInterface(w, name+"Map")
			fmt.Fprintf(w, "type %[1]sMap map[string]%[1]sInput\n\n", name)
			genResourceContainerInput(w, name+"Map", name+"Map", "map[string]*"+name)
		}
	}

	// Emit the resource output type.
	fmt.Fprintf(w, "type %sOutput struct {\n", name)
	fmt.Fprintf(w, "\t*pulumi.OutputState\n")
	fmt.Fprintf(w, "}\n\n")
	genOutputMethods(w, name, name, true)
	fmt.Fprintf(w, "\n")
	if generateResourceContainerTypes {
		fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutput() %[1]sPtrOutput {\n", name, Title(name))
		fmt.Fprintf(w, "\treturn o.To%sPtrOutputWithContext(context.Background())\n", Title(name))
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutputWithContext(ctx context.Context) %[1]sPtrOutput {\n", name, Title(name))
		fmt.Fprintf(w, "\treturn o.ApplyT(func(v %[1]s) *%[1]s {\n", name)
		fmt.Fprintf(w, "\t\treturn &v\n")
		fmt.Fprintf(w, "\t}).(%sPtrOutput)\n", name)
		fmt.Fprintf(w, "}\n")
		fmt.Fprintf(w, "\n")

		// Emit the resource pointer output type.
		fmt.Fprintf(w, "type %sOutput struct {\n", name+"Ptr")
		fmt.Fprintf(w, "\t*pulumi.OutputState\n")
		fmt.Fprintf(w, "}\n\n")
		genOutputMethods(w, name+"Ptr", "*"+name, true)

		if !r.IsProvider {
			// Emit the array output type
			fmt.Fprintf(w, "type %sArrayOutput struct { *pulumi.OutputState }\n\n", name)
			genOutputMethods(w, name+"Array", "[]"+name, true)
			fmt.Fprintf(w, "func (o %[1]sArrayOutput) Index(i pulumi.IntInput) %[1]sOutput {\n", name)
			fmt.Fprintf(w, "\treturn pulumi.All(o, i).ApplyT(func (vs []interface{}) %s {\n", name)
			fmt.Fprintf(w, "\t\treturn vs[0].([]%s)[vs[1].(int)]\n", name)
			fmt.Fprintf(w, "\t}).(%sOutput)\n", name)
			fmt.Fprintf(w, "}\n\n")
			// Emit the map output type
			fmt.Fprintf(w, "type %sMapOutput struct { *pulumi.OutputState }\n\n", name)
			genOutputMethods(w, name+"Map", "map[string]"+name, true)
			fmt.Fprintf(w, "func (o %[1]sMapOutput) MapIndex(k pulumi.StringInput) %[1]sOutput {\n", name)
			fmt.Fprintf(w, "\treturn pulumi.All(o, k).ApplyT(func (vs []interface{}) %s {\n", name)
			fmt.Fprintf(w, "\t\treturn vs[0].(map[string]%s)[vs[1].(string)]\n", name)
			fmt.Fprintf(w, "\t}).(%sOutput)\n", name)
			fmt.Fprintf(w, "}\n\n")
		}
	}
	// Register all output types
	fmt.Fprintf(w, "func init() {\n")
	fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sOutput{})\n", name)
	for _, method := range r.Methods {
		if method.Function.Outputs != nil {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%s%sResultOutput{})\n", name, Title(method.Name))
		}
	}

	if generateResourceContainerTypes {
		fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sPtrOutput{})\n", name)
		if !r.IsProvider {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sArrayOutput{})\n", name)
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sMapOutput{})\n", name)
		}
	}
	fmt.Fprintf(w, "}\n\n")

	return nil
}

func (pkg *pkgContext) genFunction(w io.Writer, f *schema.Function) {
	// If the function starts with New or Get, it will conflict; so rename them.
	name := pkg.functionNames[f]

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
		fmt.Fprintf(w, "\treturn &rv, nil\n")
	}
	fmt.Fprintf(w, "}\n")

	// If there are argument and/or return types, emit them.
	if f.Inputs != nil {
		fmt.Fprintf(w, "\n")
		pkg.genPlainType(w, fmt.Sprintf("%sArgs", name), f.Inputs.Comment, "", f.Inputs.Properties)
	}
	if f.Outputs != nil {
		fmt.Fprintf(w, "\n")
		pkg.genPlainType(w, fmt.Sprintf("%sResult", name), f.Outputs.Comment, "", f.Outputs.Properties)
	}
}

func (pkg *pkgContext) genType(w io.Writer, obj *schema.ObjectType) {
	contract.Assert(!obj.IsInputShape())

	pkg.genPlainType(w, pkg.tokenToType(obj.Token), obj.Comment, "", obj.Properties)
	pkg.genInputTypes(w, obj.InputShape, pkg.detailsForType(obj))
	pkg.genOutputTypes(w, obj, pkg.detailsForType(obj))
}

func (pkg *pkgContext) addSuffixesToName(typ schema.Type, name string) []string {
	var names []string
	details := pkg.detailsForType(typ)
	if details.arrayElement {
		names = append(names, name+"Array")
	}
	if details.mapElement {
		names = append(names, name+"Map")
	}
	return names
}

func (pkg *pkgContext) genNestedCollectionType(w io.Writer, typ schema.Type) []string {
	var elementTypeName string
	var names []string
	switch t := typ.(type) {
	case *schema.ArrayType:
		// Builtins already cater to primitive arrays
		if schema.IsPrimitiveType(t.ElementType) {
			return nil
		}
		elementTypeName = pkg.nestedTypeToType(t.ElementType)
		elementTypeName += "Array"
		names = pkg.addSuffixesToName(t, elementTypeName)
	case *schema.MapType:
		// Builtins already cater to primitive maps
		if schema.IsPrimitiveType(t.ElementType) {
			return nil
		}
		elementTypeName = pkg.nestedTypeToType(t.ElementType)
		elementTypeName += "Map"
		names = pkg.addSuffixesToName(t, elementTypeName)
	default:
		contract.Failf("unexpected type %T in genNestedCollectionType", t)
	}

	for _, name := range names {
		if strings.HasSuffix(name, "Array") {
			fmt.Fprintf(w, "type %s []%sInput\n\n", name, elementTypeName)
			genInputMethods(w, name, name, elementTypeName, false, false)

			fmt.Fprintf(w, "type %sOutput struct { *pulumi.OutputState }\n\n", name)
			genOutputMethods(w, name, elementTypeName, false)

			fmt.Fprintf(w, "func (o %sOutput) Index(i pulumi.IntInput) %sOutput {\n", name, elementTypeName)
			fmt.Fprintf(w, "\treturn pulumi.All(o, i).ApplyT(func (vs []interface{}) %s {\n", elementTypeName)
			fmt.Fprintf(w, "\t\treturn vs[0].([]%s)[vs[1].(int)]\n", elementTypeName)
			fmt.Fprintf(w, "\t}).(%sOutput)\n", elementTypeName)
			fmt.Fprintf(w, "}\n\n")
		}

		if strings.HasSuffix(name, "Map") {
			fmt.Fprintf(w, "type %s map[string]%sInput\n\n", name, elementTypeName)
			genInputMethods(w, name, name, elementTypeName, false, false)

			fmt.Fprintf(w, "type %sOutput struct { *pulumi.OutputState }\n\n", name)
			genOutputMethods(w, name, elementTypeName, false)

			// Emit the map output type
			fmt.Fprintf(w, "func (o %sOutput) MapIndex(k pulumi.StringInput) %sOutput {\n", name, elementTypeName)
			fmt.Fprintf(w, "\treturn pulumi.All(o, k).ApplyT(func (vs []interface{}) %s {\n", elementTypeName)
			fmt.Fprintf(w, "\t\treturn vs[0].(map[string]%s)[vs[1].(string)]\n", elementTypeName)
			fmt.Fprintf(w, "\t}).(%sOutput)\n", elementTypeName)
			fmt.Fprintf(w, "}\n\n")
		}
		pkg.genInputInterface(w, name)
	}

	return names
}

func (pkg *pkgContext) nestedTypeToType(typ schema.Type) string {
	switch t := codegen.UnwrapType(typ).(type) {
	case *schema.ArrayType:
		return pkg.nestedTypeToType(t.ElementType)
	case *schema.MapType:
		return pkg.nestedTypeToType(t.ElementType)
	case *schema.ObjectType:
		return pkg.resolveObjectType(t)
	}
	return pkg.tokenToType(typ.String())
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

	modPkg, ok := pkg.packages[mod]
	name = Title(name)

	if ok {
		newName, renamed := modPkg.renamed[name]
		if renamed {
			name = newName
		} else if modPkg.names.Has(name) {
			// If the package containing the enum's token already has a resource with the
			// same name, add a `Enum` suffix.
			newName := name + "Enum"
			modPkg.renamed[name] = newName
			modPkg.names.Add(newName)
			name = newName
		}
	}

	if mod == pkg.mod {
		return name
	}
	if mod == "" {
		mod = components[0]
	}
	return strings.Replace(mod, "/", "", -1) + "." + name
}

func (pkg *pkgContext) genTypeRegistrations(w io.Writer, objTypes []*schema.ObjectType, types ...string) {
	fmt.Fprintf(w, "func init() {\n")
	for _, obj := range objTypes {
		name, details := pkg.tokenToType(obj.Token), pkg.detailsForType(obj)
		fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sOutput{})\n", name)
		if details.ptrElement {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sPtrOutput{})\n", name)
		}
		if details.arrayElement {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sArrayOutput{})\n", name)
		}
		if details.mapElement {
			fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sMapOutput{})\n", name)
		}
	}

	for _, t := range types {
		fmt.Fprintf(w, "\tpulumi.RegisterOutputType(%sOutput{})\n", t)
	}
	fmt.Fprintf(w, "}\n")
}

func (pkg *pkgContext) getTypeImports(t schema.Type, recurse bool, importsAndAliases map[string]string, seen map[schema.Type]struct{}) {
	if _, ok := seen[t]; ok {
		return
	}
	seen[t] = struct{}{}
	switch t := t.(type) {
	case *schema.OptionalType:
		pkg.getTypeImports(t.ElementType, recurse, importsAndAliases, seen)
	case *schema.InputType:
		pkg.getTypeImports(t.ElementType, recurse, importsAndAliases, seen)
	case *schema.EnumType:
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
		if t.Package != nil && pkg.pkg != nil && t.Package != pkg.pkg {
			extPkg := t.Package
			var goInfo GoPackageInfo

			contract.AssertNoError(extPkg.ImportLanguages(map[string]schema.Language{"go": Importer}))
			if info, ok := extPkg.Language["go"].(GoPackageInfo); ok {
				goInfo = info
			} else {
				// tests don't include ImportBasePath
				goInfo.ImportBasePath = extractImportBasePath(extPkg)
			}
			extPkgCtx := &pkgContext{
				pkg:              extPkg,
				importBasePath:   goInfo.ImportBasePath,
				pkgImportAliases: goInfo.PackageImportAliases,
				modToPkg:         goInfo.ModuleToPackage,
			}
			mod := extPkgCtx.tokenToPackage(t.Token)
			imp := path.Join(goInfo.ImportBasePath, mod)
			importsAndAliases[imp] = goInfo.PackageImportAliases[imp]
			break
		}
		mod := pkg.tokenToPackage(t.Token)
		if mod != pkg.mod {
			p := path.Join(pkg.importBasePath, mod)
			importsAndAliases[path.Join(pkg.importBasePath, mod)] = pkg.pkgImportAliases[p]
		}

		if recurse {
			for _, p := range t.Properties {
				pkg.getTypeImports(p.Type, recurse, importsAndAliases, seen)
			}
		}
	case *schema.ResourceType:
		if t.Resource != nil && pkg.pkg != nil && t.Resource.Package != pkg.pkg {
			extPkg := t.Resource.Package
			var goInfo GoPackageInfo

			contract.AssertNoError(extPkg.ImportLanguages(map[string]schema.Language{"go": Importer}))
			if info, ok := extPkg.Language["go"].(GoPackageInfo); ok {
				goInfo = info
			} else {
				// tests don't include ImportBasePath
				goInfo.ImportBasePath = extractImportBasePath(extPkg)
			}
			extPkgCtx := &pkgContext{
				pkg:              extPkg,
				importBasePath:   goInfo.ImportBasePath,
				pkgImportAliases: goInfo.PackageImportAliases,
				modToPkg:         goInfo.ModuleToPackage,
			}
			mod := extPkgCtx.tokenToPackage(t.Token)
			imp := path.Join(goInfo.ImportBasePath, mod)
			importsAndAliases[imp] = goInfo.PackageImportAliases[imp]
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
	case *schema.EnumType: // Just need pulumi sdk, see below
	default:
		return
	}

	importsAndAliases["github.com/pulumi/pulumi/sdk/v3/go/pulumi"] = ""
}

func (pkg *pkgContext) genHeader(w io.Writer, goImports []string, importsAndAliases map[string]string) {
	fmt.Fprintf(w, "// *** WARNING: this file was generated by %v. ***\n", pkg.tool)
	fmt.Fprintf(w, "// *** Do not edit by hand unless you're certain you know what you are doing! ***\n\n")

	var pkgName string
	if pkg.mod == "" {
		pkgName = pkg.rootPackageName
		if pkgName == "" {
			pkgName = goPackage(pkg.pkg.Name)
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
	importsAndAliases := map[string]string{"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config": ""}
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

	basePath := pkg.importBasePath

	// TODO: importBasePath isn't currently set for schemas generated by pulumi-terraform-bridge.
	//		 Remove this once the linked issue is fixed. https://github.com/pulumi/pulumi-terraform-bridge/issues/320
	if len(basePath) == 0 {
		basePath = fmt.Sprintf("github.com/pulumi/pulumi-%[1]s/sdk/v2/go/%[1]s", pkg.pkg.Name)
	}

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
			if r.IsProvider {
				contract.Assert(provider == nil)
				provider = r
				continue
			}

			registrations.Add(tokenToModule(r.Token))
			fmt.Fprintf(w, "\tcase %q:\n", r.Token)
			fmt.Fprintf(w, "\t\tr = &%s{}\n", resourceName(r))
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
		fmt.Fprintf(w, "\tversion, err := PkgVersion()\n")
	} else {
		// Some package names contain '-' characters, so grab the name from the base path, unless there is an alias
		// in which case we use that instead.
		var pkgName string
		if alias, ok := pkg.pkgImportAliases[basePath]; ok {
			pkgName = alias
		} else {
			pkgName = basePath[strings.LastIndex(basePath, "/")+1:]
		}
		fmt.Fprintf(w, "\tversion, err := %s.PkgVersion()\n", pkgName)
	}
	fmt.Fprintf(w, "\tif err != nil {\n")
	fmt.Fprintf(w, "\t\tfmt.Printf(\"failed to determine package version. defaulting to v1: %%v\\n\", err)\n")
	fmt.Fprintf(w, "\t}\n")
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
				pkg:              pkg,
				mod:              mod,
				importBasePath:   goInfo.ImportBasePath,
				rootPackageName:  goInfo.RootPackageName,
				typeDetails:      map[schema.Type]*typeDetails{},
				names:            codegen.NewStringSet(),
				schemaNames:      codegen.NewStringSet(),
				renamed:          map[string]string{},
				functionNames:    map[*schema.Function]string{},
				tool:             tool,
				modToPkg:         goInfo.ModuleToPackage,
				pkgImportAliases: goInfo.PackageImportAliases,
				packages:         packages,
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
	var populateDetailsForPropertyTypes func(seen codegen.StringSet, props []*schema.Property, parentOptional bool)
	var populateDetailsForTypes func(seen codegen.StringSet, schemaType schema.Type, isRequired bool, parentOptional bool)

	populateDetailsForPropertyTypes = func(seen codegen.StringSet, props []*schema.Property, parentOptional bool) {
		for _, p := range props {
			populateDetailsForTypes(seen, p.Type, p.IsRequired(), parentOptional)
		}
	}

	populateDetailsForTypes = func(seen codegen.StringSet, schemaType schema.Type, isRequired bool, parentOptional bool) {
		switch typ := schemaType.(type) {
		case *schema.InputType:
			populateDetailsForTypes(seen, typ.ElementType, isRequired, parentOptional)
		case *schema.OptionalType:
			populateDetailsForTypes(seen, typ.ElementType, false, true)
		case *schema.ObjectType:
			pkg := getPkgFromToken(typ.Token)
			if !isRequired || parentOptional {
				if seen.Has(typ.Token) {
					return
				}
				seen.Add(typ.Token)
				pkg.detailsForType(typ).ptrElement = true
				populateDetailsForPropertyTypes(seen, typ.Properties, true)
			}
			pkg.schemaNames.Add(tokenToName(typ.Token))
		case *schema.EnumType:
			if seen.Has(typ.Token) {
				return
			}
			seen.Add(typ.Token)
			pkg := getPkgFromToken(typ.Token)
			if !isRequired || parentOptional {
				pkg.detailsForType(typ).ptrElement = true
			}
			pkg.schemaNames.Add(tokenToName(typ.Token))
		case *schema.ArrayType:
			if seen.Has(typ.String()) {
				return
			}
			seen.Add(typ.String())
			getPkgFromType(typ.ElementType).detailsForType(codegen.UnwrapType(typ.ElementType)).arrayElement = true
			populateDetailsForTypes(seen, typ.ElementType, true, false)
		case *schema.MapType:
			if seen.Has(typ.String()) {
				return
			}
			seen.Add(typ.String())
			getPkgFromType(typ.ElementType).detailsForType(codegen.UnwrapType(typ.ElementType)).mapElement = true
			populateDetailsForTypes(seen, typ.ElementType, true, false)
		}
	}

	// Use a string set to track object types that have already been processed.
	// This avoids recursively processing the same type. For example, in the
	// Kubernetes package, JSONSchemaProps have properties whose type is itself.
	seenMap := codegen.NewStringSet()
	for _, t := range pkg.Types {
		switch typ := t.(type) {
		case *schema.ArrayType:
			getPkgFromType(typ.ElementType).detailsForType(typ.ElementType).arrayElement = true
		case *schema.MapType:
			getPkgFromType(typ.ElementType).detailsForType(typ.ElementType).mapElement = true
		case *schema.ObjectType:
			pkg := getPkgFromToken(typ.Token)
			if !typ.IsInputShape() {
				pkg.types = append(pkg.types, typ)
			}
			populateDetailsForPropertyTypes(seenMap, typ.Properties, false)
		case *schema.EnumType:
			pkg := getPkgFromToken(typ.Token)
			pkg.enums = append(pkg.enums, typ)
		}
	}

	scanResource := func(r *schema.Resource) {
		pkg := getPkgFromToken(r.Token)
		pkg.resources = append(pkg.resources, r)
		pkg.schemaNames.Add(tokenToName(r.Token))

		pkg.names.Add(resourceName(r))
		pkg.names.Add(resourceName(r) + "Input")
		pkg.names.Add(resourceName(r) + "Output")
		pkg.names.Add(resourceName(r) + "Args")
		pkg.names.Add(camel(resourceName(r)) + "Args")
		pkg.names.Add("New" + resourceName(r))
		if !r.IsProvider && !r.IsComponent {
			pkg.names.Add(resourceName(r) + "State")
			pkg.names.Add(camel(resourceName(r)) + "State")
			pkg.names.Add("Get" + resourceName(r))
		}

		populateDetailsForPropertyTypes(seenMap, r.InputProperties, !r.IsProvider)
		populateDetailsForPropertyTypes(seenMap, r.Properties, !r.IsProvider)

		for _, method := range r.Methods {
			if method.Function.Inputs != nil {
				pkg.names.Add(resourceName(r) + Title(method.Name) + "Args")
			}
			if method.Function.Outputs != nil {
				pkg.names.Add(resourceName(r) + Title(method.Name) + "Result")
			}
		}
	}

	scanResource(pkg.Provider)
	for _, r := range pkg.Resources {
		scanResource(r)
	}

	for _, f := range pkg.Functions {
		if f.IsMethod {
			continue
		}

		pkg := getPkgFromToken(f.Token)
		pkg.functions = append(pkg.functions, f)

		name := tokenToName(f.Token)
		originalName := name
		if pkg.names.Has(name) {
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
			if originalName != name {
				pkg.renamed[originalName+"Args"] = name + "Args"
			}
		}
		if f.Outputs != nil {
			pkg.names.Add(name + "Result")
			if originalName != name {
				pkg.renamed[originalName+"Result"] = name + "Result"
			}
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

	name := goPkgInfo.RootPackageName
	if name == "" {
		name = goPackage(pkg.Name)
	}

	files := map[string][]byte{}
	setFile := func(relPath, contents string) {
		if goPkgInfo.RootPackageName == "" {
			relPath = path.Join(goPackage(name), relPath)
		}
		if _, ok := files[relPath]; ok {
			panic(errors.Errorf("duplicate file: %s", relPath))
		}

		// Run Go formatter on the code before saving to disk
		formattedSource, err := format.Source([]byte(contents))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid content:\n%s\n%s\n", relPath, contents)
			panic(errors.Wrapf(err, "invalid Go source code:\n\n%s\n", relPath))
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
			importsAndAliases := map[string]string{}
			pkg.getImports(r, importsAndAliases)

			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, []string{"context", "reflect"}, importsAndAliases)

			if err := pkg.genResource(buffer, r, goPkgInfo.GenerateResourceContainerTypes); err != nil {
				return nil, err
			}

			setFile(path.Join(mod, camel(resourceName(r))+".go"), buffer.String())
		}

		// Functions
		for _, f := range pkg.functions {
			importsAndAliases := map[string]string{}
			pkg.getImports(f, importsAndAliases)

			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, nil, importsAndAliases)

			pkg.genFunction(buffer, f)

			setFile(path.Join(mod, camel(tokenToName(f.Token))+".go"), buffer.String())
		}

		knownTypes := make(map[schema.Type]struct{}, len(pkg.typeDetails))
		for t := range pkg.typeDetails {
			knownTypes[t] = struct{}{}
		}

		// Enums
		if len(pkg.enums) > 0 {
			imports := map[string]string{}
			for _, e := range pkg.enums {
				pkg.getImports(e, imports)
			}

			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, []string{"context", "reflect"}, imports)

			for _, e := range pkg.enums {
				if err := pkg.genEnum(buffer, e); err != nil {
					return nil, err
				}
				delete(knownTypes, e)
			}
			// Register all output types
			fmt.Fprintf(buffer, "func init() {\n")
			for _, e := range pkg.enums {
				name := pkg.tokenToEnum(e.Token)
				fmt.Fprintf(buffer, "\tpulumi.RegisterOutputType(%sOutput{})\n", name)
				fmt.Fprintf(buffer, "\tpulumi.RegisterOutputType(%sPtrOutput{})\n", name)
				details := pkg.detailsForType(e)
				if details.arrayElement {
					fmt.Fprintf(buffer, "\tpulumi.RegisterOutputType(%sArrayOutput{})\n", name)
				}
				if details.mapElement {
					fmt.Fprintf(buffer, "\tpulumi.RegisterOutputType(%sMapOutput{})\n", name)
				}
			}
			fmt.Fprintf(buffer, "}\n\n")
			setFile(path.Join(mod, "pulumiEnums.go"), buffer.String())
		}

		// Types
		if len(pkg.types) > 0 {
			importsAndAliases := map[string]string{}
			for _, t := range pkg.types {
				pkg.getImports(t, importsAndAliases)
			}

			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, []string{"context", "reflect"}, importsAndAliases)

			for _, t := range pkg.types {
				pkg.genType(buffer, t)
				delete(knownTypes, t)
			}

			sortedKnownTypes := make([]schema.Type, 0, len(knownTypes))
			for k := range knownTypes {
				sortedKnownTypes = append(sortedKnownTypes, k)
			}
			sort.Slice(sortedKnownTypes, func(i, j int) bool {
				return sortedKnownTypes[i].String() < sortedKnownTypes[j].String()
			})

			var types []string
			for _, t := range sortedKnownTypes {
				switch typ := t.(type) {
				case *schema.ArrayType, *schema.MapType:
					types = pkg.genNestedCollectionType(buffer, typ)
				}
			}

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

			_, err := fmt.Fprintf(buffer, utilitiesFile, packageRegex)
			if err != nil {
				return nil, err
			}

			setFile(path.Join(mod, "pulumiUtilities.go"), buffer.String())
		}

		// If there are resources in this module, register the module with the runtime.
		if len(pkg.resources) != 0 {
			buffer := &bytes.Buffer{}
			pkg.genResourceModule(buffer)

			setFile(path.Join(mod, "init.go"), buffer.String())
		}
	}

	return files, nil
}

// goPackage returns the suggested package name for the given string.
func goPackage(name string) string {
	return strings.Split(name, "-")[0]
}

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
	return semver.Version{}, fmt.Errorf("failed to determine the package version from %%s", pkgPath)
}
`
