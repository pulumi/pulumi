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
package dotnet

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type typeDetails struct {
	outputType                        bool
	inputType                         bool
	stateType                         bool
	plainType                         bool
	usedInFunctionOutputVersionInputs bool
}

// Title converts the input string to a title case
// where only the initial letter is upper-cased.
func Title(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	return string(append([]rune{unicode.ToUpper(runes[0])}, runes[1:]...))
}

func csharpIdentifier(s string) string {
	// Some schema field names may look like $ref or $schema. Remove the leading $ to make a valid identifier.
	// This could lead to a clash if both `$foo` and `foo` are defined, but we don't try to de-duplicate now.
	s = strings.TrimPrefix(s, "$")

	switch s {
	case "abstract", "as", "base", "bool",
		"break", "byte", "case", "catch",
		"char", "checked", "class", "const",
		"continue", "decimal", "default", "delegate",
		"do", "double", "else", "enum",
		"event", "explicit", "extern", "false",
		"finally", "fixed", "float", "for",
		"foreach", "goto", "if", "implicit",
		"in", "int", "interface", "internal",
		"is", "lock", "long", "namespace",
		"new", "null", "object", "operator",
		"out", "override", "params", "private",
		"protected", "public", "readonly", "ref",
		"return", "sbyte", "sealed", "short",
		"sizeof", "stackalloc", "static", "string",
		"struct", "switch", "this", "throw",
		"true", "try", "typeof", "uint",
		"ulong", "unchecked", "unsafe", "ushort",
		"using", "virtual", "void", "volatile", "while":
		return "@" + s

	default:
		return s
	}
}

func isImmutableArrayType(t schema.Type, wrapInput bool) bool {
	_, isArray := t.(*schema.ArrayType)
	return isArray && !wrapInput
}

func isValueType(t schema.Type) bool {
	switch t := t.(type) {
	case *schema.OptionalType:
		return isValueType(t.ElementType)
	case *schema.EnumType:
		return true
	default:
		switch t {
		case schema.BoolType, schema.IntType, schema.NumberType:
			return true
		default:
			return false
		}
	}
}

func namespaceName(namespaces map[string]string, name string) string {
	if ns, ok := namespaces[name]; ok {
		return ns
	}

	// name could be a qualified module name so first split on /
	parts := strings.Split(name, tokens.QNameDelimiter)
	for i, part := range parts {
		names := strings.Split(part, "-")
		for j, name := range names {
			names[j] = Title(name)
		}
		parts[i] = strings.Join(names, "")
	}
	return strings.Join(parts, ".")
}

type modContext struct {
	pkg                    schema.PackageReference
	mod                    string
	propertyNames          map[*schema.Property]string
	types                  []*schema.ObjectType
	enums                  []*schema.EnumType
	resources              []*schema.Resource
	functions              []*schema.Function
	typeDetails            map[*schema.ObjectType]*typeDetails
	children               []*modContext
	tool                   string
	namespaceName          string
	namespaces             map[string]string
	compatibility          string
	dictionaryConstructors bool

	// If types in the Input namespace are used.
	fullyQualifiedInputs bool

	// Determine whether to lift single-value method return values
	liftSingleValueMethodReturns bool

	// The root namespace to use, if any.
	rootNamespace    string
	parameterization *schema.Parameterization
}

func (mod *modContext) RootNamespace() string {
	if mod.rootNamespace != "" {
		return mod.rootNamespace
	}
	return "Pulumi"
}

func (mod *modContext) propertyName(p *schema.Property) string {
	if n, ok := mod.propertyNames[p]; ok {
		return n
	}
	return Title(p.Name)
}

func (mod *modContext) details(t *schema.ObjectType) *typeDetails {
	details, ok := mod.typeDetails[t]
	if !ok {
		details = &typeDetails{}
		mod.typeDetails[t] = details
	}
	return details
}

func tokenToName(tok string) string {
	// token := pkg : module : member
	// module := path/to/module

	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", tok)
	return Title(components[2])
}

func resourceName(r *schema.Resource) string {
	if r.IsProvider {
		return "Provider"
	}

	if val1, ok := r.Language["csharp"]; ok {
		val2, ok := val1.(CSharpResourceInfo)
		contract.Assertf(ok, "dotnet specific settings for resources should be of type CSharpResourceInfo")
		return Title(val2.Name)
	}

	return tokenToName(r.Token)
}

func tokenToFunctionName(tok string) string {
	return tokenToName(tok)
}

func (mod *modContext) isK8sCompatMode() bool {
	return mod.compatibility == "kubernetes20"
}

func (mod *modContext) isTFCompatMode() bool {
	return mod.compatibility == "tfbridge20"
}

func (mod *modContext) tokenToNamespace(tok string, qualifier string) string {
	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", tok)

	pkg, nsName := mod.RootNamespace()+"."+namespaceName(mod.namespaces, components[0]), mod.pkg.TokenToModule(tok)

	if mod.isK8sCompatMode() {
		if qualifier != "" {
			return pkg + ".Types." + qualifier + "." + namespaceName(mod.namespaces, nsName)
		}
	}

	typ := pkg
	if nsName != "" {
		typ += "." + namespaceName(mod.namespaces, nsName)
	}
	if qualifier != "" {
		typ += "." + qualifier
	}
	return typ
}

func (mod *modContext) typeName(t *schema.ObjectType, state, input, args bool) string {
	name := tokenToName(t.Token)
	if state {
		return name + "GetArgs"
	}
	if !mod.isTFCompatMode() && !mod.isK8sCompatMode() {
		if args {
			return name + "Args"
		}
		return name
	}

	switch {
	case input && args && mod.details(t).usedInFunctionOutputVersionInputs:
		return name + "InputArgs"
	case input:
		return name + "Args"
	case mod.details(t).plainType:
		return name + "Result"
	}
	return name
}

func isInputType(t schema.Type) bool {
	if optional, ok := t.(*schema.OptionalType); ok {
		t = optional.ElementType
	}
	_, isInputType := t.(*schema.InputType)
	return isInputType
}

func ignoreOptional(t *schema.OptionalType, requireInitializers bool) bool {
	switch t := t.ElementType.(type) {
	case *schema.InputType:
		switch t.ElementType.(type) {
		case *schema.ArrayType, *schema.MapType:
			return true
		}
	case *schema.ArrayType:
		return !requireInitializers
	}
	return false
}

func simplifyInputUnion(union *schema.UnionType) *schema.UnionType {
	elements := make([]schema.Type, len(union.ElementTypes))
	for i, et := range union.ElementTypes {
		if input, ok := et.(*schema.InputType); ok {
			switch input.ElementType.(type) {
			case *schema.ArrayType, *schema.MapType:
				// Instead of just replacing Input<{Array,Map}<T>> with {Array,Map}<T>, replace it with
				// {Array,Map}<Plain(T)>. This matches the behavior of typeString when presented with an
				// Input<{Array,Map}<T>>.
				elements[i] = codegen.PlainType(input.ElementType)
			default:
				elements[i] = input.ElementType
			}
		} else {
			elements[i] = et
		}
	}
	return &schema.UnionType{
		ElementTypes:  elements,
		DefaultType:   union.DefaultType,
		Discriminator: union.Discriminator,
		Mapping:       union.Mapping,
	}
}

func (mod *modContext) unionTypeString(t *schema.UnionType, qualifier string, input, wrapInput, state, requireInitializers bool) string {
	elementTypeSet := codegen.StringSet{}
	var elementTypes []string
	for _, e := range t.ElementTypes {
		// If this is an output and a "relaxed" enum, emit the type as the underlying primitive type rather than the union.
		// Eg. Output<string> rather than Output<Union<EnumType, string>>
		if typ, ok := e.(*schema.EnumType); ok && !input {
			return mod.typeString(typ.ElementType, qualifier, input, state, requireInitializers)
		}

		et := mod.typeString(e, qualifier, input, state, false)
		if !elementTypeSet.Has(et) {
			elementTypeSet.Add(et)
			elementTypes = append(elementTypes, et)
		}
	}

	switch len(elementTypes) {
	case 1:
		if wrapInput {
			return fmt.Sprintf("Input<%s>", elementTypes[0])
		}
		return elementTypes[0]
	case 2:
		unionT := "Union"
		if wrapInput {
			unionT = "InputUnion"
		}
		return fmt.Sprintf("%s<%s>", unionT, strings.Join(elementTypes, ", "))
	default:
		return "object"
	}
}

func (mod *modContext) typeString(t schema.Type, qualifier string, input, state, requireInitializers bool) string {
	switch t := t.(type) {
	case *schema.OptionalType:
		elem := mod.typeString(t.ElementType, qualifier, input, state, requireInitializers)
		if ignoreOptional(t, requireInitializers) {
			return elem
		}
		return elem + "?"
	case *schema.InputType:
		inputType := "Input"
		elem := t.ElementType
		switch e := t.ElementType.(type) {
		case *schema.ArrayType:
			inputType, elem = "InputList", codegen.PlainType(e.ElementType)
		case *schema.MapType:
			inputType, elem = "InputMap", codegen.PlainType(e.ElementType)
		default:
			if e == schema.JSONType {
				return "InputJson"
			}
		}

		if union, ok := elem.(*schema.UnionType); ok {
			union = simplifyInputUnion(union)
			if inputType == "Input" {
				return mod.unionTypeString(union, qualifier, input, true, state, requireInitializers)
			}
			elem = union
		}
		return fmt.Sprintf("%s<%s>", inputType, mod.typeString(elem, qualifier, input, state, requireInitializers))
	case *schema.EnumType:
		return fmt.Sprintf("%s.%s", mod.tokenToNamespace(t.Token, ""), tokenToName(t.Token))
	case *schema.ArrayType:
		listType := "ImmutableArray"
		if requireInitializers {
			listType = "List"
		}
		return fmt.Sprintf("%v<%v>", listType, mod.typeString(t.ElementType, qualifier, input, state, false))
	case *schema.MapType:
		mapType := "ImmutableDictionary"
		if requireInitializers {
			mapType = "Dictionary"
		}
		return fmt.Sprintf("%v<string, %v>", mapType, mod.typeString(t.ElementType, qualifier, input, state, false))
	case *schema.ObjectType:
		namingCtx := mod
		if !codegen.PkgEquals(t.PackageReference, mod.pkg) {
			// If object type belongs to another package, we apply naming conventions from that package,
			// including namespace naming and compatibility mode.
			extPkg := t.PackageReference
			var info CSharpPackageInfo
			def, err := extPkg.Definition()
			contract.AssertNoErrorf(err, "error loading definition for package %q", extPkg.Name())
			contract.AssertNoErrorf(def.ImportLanguages(map[string]schema.Language{"csharp": Importer}),
				"error importing csharp for package %q", extPkg.Name())
			if v, ok := def.Language["csharp"].(CSharpPackageInfo); ok {
				info = v
			}
			namingCtx = &modContext{
				pkg:           extPkg,
				namespaces:    info.Namespaces,
				rootNamespace: info.GetRootNamespace(),
				compatibility: info.Compatibility,
			}
		}
		typ := namingCtx.tokenToNamespace(t.Token, qualifier)
		if (typ == namingCtx.namespaceName && qualifier == "") || typ == namingCtx.namespaceName+"."+qualifier {
			typ = qualifier
		}
		if typ == "Inputs" && mod.fullyQualifiedInputs {
			typ = mod.namespaceName + ".Inputs"
		}
		if typ != "" {
			typ += "."
		}
		return typ + mod.typeName(t, state, input, t.IsInputShape())
	case *schema.ResourceType:
		if strings.HasPrefix(t.Token, "pulumi:providers:") {
			pkgName := strings.TrimPrefix(t.Token, "pulumi:providers:")
			return fmt.Sprintf("%s.%s.Provider", mod.RootNamespace(), namespaceName(mod.namespaces, pkgName))
		}

		namingCtx := mod
		if t.Resource != nil && !codegen.PkgEquals(t.Resource.PackageReference, mod.pkg) {
			// If resource type belongs to another package, we apply naming conventions from that package,
			// including namespace naming and compatibility mode.
			extPkg := t.Resource.PackageReference
			var info CSharpPackageInfo
			def, err := extPkg.Definition()
			contract.AssertNoErrorf(err, "error loading definition for package %q", extPkg.Name())
			contract.AssertNoErrorf(def.ImportLanguages(map[string]schema.Language{"csharp": Importer}),
				"error importing csharp for package %q", extPkg.Name())
			if v, ok := def.Language["csharp"].(CSharpPackageInfo); ok {
				info = v
			}
			namingCtx = &modContext{
				pkg:           extPkg,
				namespaces:    info.Namespaces,
				rootNamespace: info.GetRootNamespace(),
				compatibility: info.Compatibility,
			}
		}
		typ := namingCtx.tokenToNamespace(t.Token, "")
		if typ != "" {
			typ += "."
		}
		return typ + resourceName(t.Resource)
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return mod.typeString(t.UnderlyingType, qualifier, input, state, requireInitializers)
		}

		typ := tokenToName(t.Token)
		if ns := mod.tokenToNamespace(t.Token, qualifier); ns != mod.namespaceName {
			typ = ns + "." + typ
		}
		return typ
	case *schema.UnionType:
		return mod.unionTypeString(t, qualifier, input, false, state, requireInitializers)
	default:
		switch t {
		case schema.BoolType:
			return "bool"
		case schema.IntType:
			return "int"
		case schema.NumberType:
			return "double"
		case schema.StringType:
			return "string"
		case schema.ArchiveType:
			return "Archive"
		case schema.AssetType:
			return "AssetOrArchive"
		case schema.JSONType:
			return "System.Text.Json.JsonElement"
		case schema.AnyType:
			return "object"
		}
	}

	panic(fmt.Errorf("unexpected type %T", t))
}

var docCommentEscaper = strings.NewReplacer(
	`&`, "&amp;",
	`<`, "&lt;",
	`>`, "&gt;",
)

func printComment(w io.Writer, comment, indent string) {
	printCommentWithOptions(w, comment, indent, true /*escape*/)
}

func printCommentWithOptions(w io.Writer, comment, indent string, escape bool) {
	if escape {
		comment = docCommentEscaper.Replace(comment)
	}

	lines := strings.Split(comment, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) > 0 {
		fmt.Fprintf(w, "%s/// <summary>\n", indent)
		for _, l := range lines {
			fmt.Fprintf(w, "%s/// %s\n", indent, l)
		}
		fmt.Fprintf(w, "%s/// </summary>\n", indent)
	}
}

type plainType struct {
	mod                   *modContext
	res                   *schema.Resource
	name                  string
	comment               string
	unescapeComment       bool
	baseClass             string
	propertyTypeQualifier string
	properties            []*schema.Property
	args                  bool
	state                 bool
	internal              bool
}

func (pt *plainType) genInputPropertyAttribute(w io.Writer, indent string, prop *schema.Property) {
	wireName := prop.Name
	attributeArgs := ""
	if prop.IsRequired() {
		attributeArgs = ", required: true"
	}
	if pt.res != nil && pt.res.IsProvider {
		json := true
		typ := codegen.UnwrapType(prop.Type)
		if typ == schema.StringType {
			json = false
		} else if t, ok := typ.(*schema.TokenType); ok && t.UnderlyingType == schema.StringType {
			json = false
		}
		if json {
			attributeArgs += ", json: true"
		}
	}
	fmt.Fprintf(w, "%s[Input(\"%s\"%s)]\n", indent, wireName, attributeArgs)
}

func (pt *plainType) genInputProperty(w io.Writer, prop *schema.Property, indent string, generateInputAttribute bool) {
	propertyName := pt.mod.propertyName(prop)
	propertyType := pt.mod.typeString(prop.Type, pt.propertyTypeQualifier, true, pt.state, false)

	indent = strings.Repeat(indent, 2)

	// Next generate the input property itself. The way this is generated depends on the type of the property:
	// complex types like lists and maps need a backing field.
	needsBackingField := false
	switch codegen.UnwrapType(prop.Type).(type) {
	case *schema.ArrayType, *schema.MapType:
		needsBackingField = true
	}
	if prop.Secret {
		needsBackingField = true
	}

	// Next generate the input property itself. The way this is generated depends on the type of the property:
	// complex types like lists and maps need a backing field. Secret properties also require a backing field.
	if needsBackingField {
		backingFieldName := "_" + prop.Name
		requireInitializers := !pt.args || !isInputType(prop.Type)
		backingFieldType := pt.mod.typeString(codegen.RequiredType(prop), pt.propertyTypeQualifier, true, pt.state, requireInitializers)

		if generateInputAttribute {
			pt.genInputPropertyAttribute(w, indent, prop)
		}

		fmt.Fprintf(w, "%sprivate %s? %s;\n", indent, backingFieldType, backingFieldName)

		if prop.Comment != "" {
			fmt.Fprintf(w, "\n")
			printComment(w, prop.Comment, indent)
		}
		printObsoleteAttribute(w, prop.DeprecationMessage, indent)

		switch codegen.UnwrapType(prop.Type).(type) {
		case *schema.ArrayType, *schema.MapType:
			// Note that we use the backing field type--which is just the property type without any nullable annotation--to
			// ensure that the user does not see warnings when initializing these properties using object or collection
			// initializers.
			fmt.Fprintf(w, "%spublic %s %s\n", indent, backingFieldType, propertyName)
			fmt.Fprintf(w, "%s{\n", indent)
			fmt.Fprintf(w, "%s    get => %[2]s ?? (%[2]s = new %[3]s());\n", indent, backingFieldName, backingFieldType)
		default:
			fmt.Fprintf(w, "%spublic %s? %s\n", indent, backingFieldType, propertyName)
			fmt.Fprintf(w, "%s{\n", indent)
			fmt.Fprintf(w, "%s    get => %s;\n", indent, backingFieldName)
		}
		if prop.Secret && isInputType(prop.Type) {
			fmt.Fprintf(w, "%s    set\n", indent)
			fmt.Fprintf(w, "%s    {\n", indent)
			// Since we can't directly assign the Output from CreateSecret to the property, use an Output.All or
			// Output.Tuple to enable the secret flag on the data. (If any input to the All/Tuple is secret, then the
			// Output will also be secret.)
			switch t := codegen.UnwrapType(prop.Type).(type) {
			case *schema.ArrayType:
				elemType := pt.mod.typeString(codegen.PlainType(t.ElementType), pt.propertyTypeQualifier, true, pt.state, false)
				fmt.Fprintf(w, "%s        var emptySecret = Output.CreateSecret(ImmutableArray.Create<%s>());\n", indent, elemType)
				fmt.Fprintf(w, "%s        %s = Output.All(value, emptySecret).Apply(v => v[0]);\n", indent, backingFieldName)
			case *schema.MapType:
				elemType := pt.mod.typeString(codegen.PlainType(t.ElementType), pt.propertyTypeQualifier, true, pt.state, false)
				fmt.Fprintf(w, "%s        var emptySecret = Output.CreateSecret(ImmutableDictionary.Create<string, %s>());\n", indent, elemType)
				fmt.Fprintf(w, "%s        %s = Output.All(value, emptySecret).Apply(v => v[0]);\n", indent, backingFieldName)
			default:
				fmt.Fprintf(w, "%s        var emptySecret = Output.CreateSecret(0);\n", indent)
				fmt.Fprintf(w, "%s        %s = Output.Tuple<%s?, int>(value, emptySecret).Apply(t => t.Item1);\n", indent, backingFieldName, backingFieldType)
			}
			fmt.Fprintf(w, "%s    }\n", indent)
		} else {
			fmt.Fprintf(w, "%s    set => %s = value;\n", indent, backingFieldName)
		}
		fmt.Fprintf(w, "%s}\n", indent)
	} else {
		initializer := ""
		if prop.IsRequired() && !isValueType(prop.Type) {
			initializer = " = null!;"
		}

		printComment(w, prop.Comment, indent)

		if generateInputAttribute {
			pt.genInputPropertyAttribute(w, indent, prop)
		}

		fmt.Fprintf(w, "%spublic %s %s { get; set; }%s\n", indent, propertyType, propertyName, initializer)
	}
}

// Set to avoid generating a class with the same name twice.
var generatedTypes = codegen.Set{}

func (pt *plainType) genInputType(w io.Writer, level int) error {
	return pt.genInputTypeWithFlags(w, level, true /* generateInputAttributes */)
}

func (pt *plainType) genInputTypeWithFlags(w io.Writer, level int, generateInputAttributes bool) error {
	// The way the legacy codegen for kubernetes is structured, inputs for a resource args type and resource args
	// subtype could become a single class because of the name + namespace clash. We use a set of generated types
	// to prevent generating classes with equal full names in multiple files. The check should be removed if we
	// ever change the namespacing in the k8s SDK to the standard one.
	if pt.mod.isK8sCompatMode() {
		key := pt.mod.namespaceName + pt.name
		if generatedTypes.Has(key) {
			return nil
		}
		generatedTypes.Add(key)
	}

	indent := strings.Repeat("    ", level)

	fmt.Fprintf(w, "\n")

	sealed := "sealed "
	if pt.mod.isK8sCompatMode() && (pt.res == nil || !pt.res.IsProvider) {
		sealed = ""
	}

	// Open the class.
	printCommentWithOptions(w, pt.comment, indent, !pt.unescapeComment)

	var suffix string
	if pt.baseClass != "" {
		suffix = " : global::Pulumi." + pt.baseClass
	}

	fmt.Fprintf(w, "%spublic %sclass %s%s\n", indent, sealed, pt.name, suffix)
	fmt.Fprintf(w, "%s{\n", indent)

	// Declare each input property.
	for _, p := range pt.properties {
		pt.genInputProperty(w, p, indent, generateInputAttributes)
		fmt.Fprintf(w, "\n")
	}

	// Generate a constructor that will set default values.
	fmt.Fprintf(w, "%s    public %s()\n", indent, pt.name)
	fmt.Fprintf(w, "%s    {\n", indent)
	for _, prop := range pt.properties {
		if prop.DefaultValue != nil {
			dv, err := pt.mod.getDefaultValue(prop.DefaultValue, prop.Type)
			if err != nil {
				return err
			}
			propertyName := pt.mod.propertyName(prop)
			fmt.Fprintf(w, "%s        %s = %s;\n", indent, propertyName, dv)
		}
	}
	fmt.Fprintf(w, "%s    }\n", indent)

	// override Empty static property from inherited ResourceArgs
	// and make it return a concrete args type instead of inherited ResourceArgs
	fmt.Fprintf(w, "%s    public static new %s Empty => new %s();\n", indent, pt.name, pt.name)

	// Close the class.
	fmt.Fprintf(w, "%s}\n", indent)

	return nil
}

func (pt *plainType) genOutputType(w io.Writer, level int) {
	indent := strings.Repeat("    ", level)

	fmt.Fprintf(w, "\n")

	// Open the class and attribute it appropriately.
	printCommentWithOptions(w, pt.comment, indent, !pt.unescapeComment)
	fmt.Fprintf(w, "%s[OutputType]\n", indent)

	visibility := "public"
	if pt.internal {
		visibility = "internal"
	}

	fmt.Fprintf(w, "%s%s sealed class %s\n", indent, visibility, pt.name)
	fmt.Fprintf(w, "%s{\n", indent)

	// Generate each output field.
	for _, prop := range pt.properties {
		fieldName := pt.mod.propertyName(prop)
		typ := prop.Type
		if !prop.IsRequired() && pt.mod.isK8sCompatMode() {
			typ = codegen.RequiredType(prop)
		}
		fieldType := pt.mod.typeString(typ, pt.propertyTypeQualifier, false, false, false)
		printComment(w, prop.Comment, indent+"    ")
		fmt.Fprintf(w, "%s    public readonly %s %s;\n", indent, fieldType, fieldName)
	}
	if len(pt.properties) > 0 {
		fmt.Fprintf(w, "\n")
	}

	// Generate an appropriately-attributed constructor that will set this types' fields.
	fmt.Fprintf(w, "%s    [OutputConstructor]\n", indent)
	fmt.Fprintf(w, "%s    private %s(", indent, pt.name)

	// Generate the constructor parameters.
	for i, prop := range pt.properties {
		paramName := csharpIdentifier(prop.Name)
		typ := prop.Type
		if !prop.IsRequired() && pt.mod.isK8sCompatMode() {
			typ = codegen.RequiredType(prop)
		}
		paramType := pt.mod.typeString(typ, pt.propertyTypeQualifier, false, false, false)

		terminator := ""
		if i != len(pt.properties)-1 {
			terminator = ",\n"
		}

		paramDef := fmt.Sprintf("%s %s%s", paramType, paramName, terminator)
		if len(pt.properties) > 1 {
			paramDef = fmt.Sprintf("\n%s        %s", indent, paramDef)
		}
		fmt.Fprint(w, paramDef)
	}
	fmt.Fprintf(w, ")\n")

	// Generate the constructor body.
	fmt.Fprintf(w, "%s    {\n", indent)
	for _, prop := range pt.properties {
		paramName := csharpIdentifier(prop.Name)
		fieldName := pt.mod.propertyName(prop)
		if fieldName == paramName {
			// Avoid a no-op in case of field and property name collision.
			fieldName = "this." + fieldName
		}
		fmt.Fprintf(w, "%s        %s = %s;\n", indent, fieldName, paramName)
	}
	fmt.Fprintf(w, "%s    }\n", indent)

	// Close the class.
	fmt.Fprintf(w, "%s}\n", indent)
}

func primitiveValue(value interface{}) (string, error) {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	//nolint:exhaustive // We only support default values for a subset of types.
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
		return "", fmt.Errorf("unsupported default value of type %T", value)
	}
}

func (mod *modContext) getDefaultValue(dv *schema.DefaultValue, t schema.Type) (string, error) {
	t = codegen.UnwrapType(t)

	var val string
	if dv.Value != nil {
		switch enum := t.(type) {
		case *schema.EnumType:
			enumName := tokenToName(enum.Token)
			for _, e := range enum.Elements {
				if e.Value != dv.Value {
					continue
				}

				elName := e.Name
				if elName == "" {
					elName = fmt.Sprintf("%v", e.Value)
				}
				safeName, err := makeSafeEnumName(elName, enumName)
				if err != nil {
					return "", err
				}
				val = fmt.Sprintf("%s.%s.%s", mod.namespaceName, enumName, safeName)
				break
			}
			if val == "" {
				return "", fmt.Errorf("default value '%v' not found in enum '%s'", dv.Value, enumName)
			}
		default:
			v, err := primitiveValue(dv.Value)
			if err != nil {
				return "", err
			}
			val = v
		}
	}

	if len(dv.Environment) != 0 {
		getType := ""
		switch t {
		case schema.BoolType:
			getType = "Boolean"
		case schema.IntType:
			getType = "Int32"
		case schema.NumberType:
			getType = "Double"
		}

		envVars := fmt.Sprintf("%q", dv.Environment[0])
		for _, e := range dv.Environment[1:] {
			envVars += fmt.Sprintf(", %q", e)
		}

		getEnv := fmt.Sprintf("Utilities.GetEnv%s(%s)", getType, envVars)
		if val != "" {
			val = fmt.Sprintf("%s ?? %s", getEnv, val)
		} else {
			val = getEnv
		}
	}

	return val, nil
}

func (mod *modContext) genResource(w io.Writer, r *schema.Resource) error {
	// Create a resource module file into which all of this resource's types will go.
	name := resourceName(r)

	// Open the namespace.
	fmt.Fprintf(w, "namespace %s\n", mod.namespaceName)
	fmt.Fprintf(w, "{\n")

	// Write the documentation comment for the resource class
	printComment(w, codegen.FilterExamples(r.Comment, "csharp"), "    ")

	// Open the class.
	className := name
	var baseType string
	optionsType := "CustomResourceOptions"
	switch {
	case r.IsProvider:
		baseType = "global::Pulumi.ProviderResource"
	case mod.isK8sCompatMode() && !r.IsComponent:
		baseType = "KubernetesResource"
	case r.IsComponent:
		baseType = "global::Pulumi.ComponentResource"
		optionsType = "ComponentResourceOptions"
	default:
		baseType = "global::Pulumi.CustomResource"
	}

	if r.DeprecationMessage != "" {
		fmt.Fprintf(w, "    [Obsolete(@\"%s\")]\n", strings.ReplaceAll(r.DeprecationMessage, `"`, `""`))
	}
	fmt.Fprintf(w, "    [%sResourceType(\"%s\")]\n", namespaceName(mod.namespaces, mod.pkg.Name()), r.Token)
	fmt.Fprintf(w, "    public partial class %s : %s\n", className, baseType)
	fmt.Fprintf(w, "    {\n")

	var secretProps []string
	// Emit all output properties.
	for _, prop := range r.Properties {
		// Write the property attribute
		wireName := prop.Name
		propertyName := mod.propertyName(prop)

		typ := prop.Type
		if !prop.IsRequired() && mod.isK8sCompatMode() {
			typ = codegen.RequiredType(prop)
		}

		propertyType := mod.typeString(typ, "Outputs", false, false, false)

		// Workaround the fact that provider inputs come back as strings.
		if r.IsProvider && !schema.IsPrimitiveType(prop.Type) {
			propertyType = "string"
			if !prop.IsRequired() {
				propertyType += "?"
			}
		}

		if prop.Secret {
			secretProps = append(secretProps, prop.Name)
		}

		printComment(w, prop.Comment, "        ")
		fmt.Fprintf(w, "        [Output(\"%s\")]\n", wireName)
		fmt.Fprintf(w, "        public Output<%s> %s { get; private set; } = null!;\n", propertyType, propertyName)
		fmt.Fprintf(w, "\n")
	}
	if len(r.Properties) > 0 {
		fmt.Fprintf(w, "\n")
	}

	// Emit the class constructor.
	argsClassName := className + "Args"
	if mod.isK8sCompatMode() && !r.IsProvider {
		argsClassName = fmt.Sprintf("%s.%sArgs", mod.tokenToNamespace(r.Token, "Inputs"), className)
	}
	argsType := argsClassName

	var argsDefault string
	allOptionalInputs := true
	hasConstInputs := false
	for _, prop := range r.InputProperties {
		allOptionalInputs = allOptionalInputs && !prop.IsRequired()
		hasConstInputs = hasConstInputs || prop.ConstValue != nil
	}
	if allOptionalInputs || mod.isK8sCompatMode() {
		// If the number of required input properties was zero, we can make the args object optional.
		argsDefault = " = null"
		argsType += "?"
	}

	tok := r.Token
	if r.IsProvider {
		tok = mod.pkg.Name()
	}

	argsOverride := fmt.Sprintf("args ?? new %s()", argsClassName)
	if hasConstInputs {
		argsOverride = "MakeArgs(args)"
	}

	// Write a comment prior to the constructor.
	fmt.Fprintf(w, "        /// <summary>\n")
	fmt.Fprintf(w, "        /// Create a %s resource with the given unique name, arguments, and options.\n", className)
	fmt.Fprintf(w, "        /// </summary>\n")
	fmt.Fprintf(w, "        ///\n")
	fmt.Fprintf(w, "        /// <param name=\"name\">The unique name of the resource</param>\n")
	fmt.Fprintf(w, "        /// <param name=\"args\">The arguments used to populate this resource's properties</param>\n")
	fmt.Fprintf(w, "        /// <param name=\"options\">A bag of options that control this resource's behavior</param>\n")

	fmt.Fprintf(w, "        public %s(string name, %s args%s, %s? options = null)\n", className, argsType, argsDefault, optionsType)
	if r.IsComponent {
		fmt.Fprintf(w, "            : base(\"%s\", name, %s, MakeResourceOptions(options, \"\"), remote: true)\n", tok, argsOverride)
	} else {
		if mod.parameterization != nil {
			fmt.Fprintf(w, "            : base(\"%s\", name, %s, MakeResourceOptions(options, \"\"), %s)\n",
				tok,
				argsOverride,
				"Utilities.PackageParameterization()")
		} else {
			fmt.Fprintf(w, "            : base(\"%s\", name, %s, MakeResourceOptions(options, \"\"))\n", tok, argsOverride)
		}
	}
	fmt.Fprintf(w, "        {\n")
	fmt.Fprintf(w, "        }\n")

	// Write a dictionary constructor.
	if mod.dictionaryConstructors && !r.IsComponent {
		fmt.Fprintf(w, "        internal %s(string name, ImmutableDictionary<string, object?> dictionary, %s? options = null)\n",
			className, optionsType)
		if r.IsComponent {
			fmt.Fprintf(w, "            : base(\"%s\", name, new DictionaryResourceArgs(dictionary), MakeResourceOptions(options, \"\"), remote: true)\n", tok)
		} else {
			fmt.Fprintf(w, "            : base(\"%s\", name, new DictionaryResourceArgs(dictionary), MakeResourceOptions(options, \"\"))\n", tok)
		}
		fmt.Fprintf(w, "        {\n")
		fmt.Fprintf(w, "        }\n")
	}

	// Write a private constructor for the use of `Get`.
	if !r.IsProvider && !r.IsComponent {
		stateParam, stateRef := "", "null"
		if r.StateInputs != nil {
			stateParam, stateRef = className+"State? state = null, ", "state"
		}

		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "        private %s(string name, Input<string> id, %s%s? options = null)\n", className, stateParam, optionsType)
		if mod.parameterization != nil {
			fmt.Fprintf(w, "            : base(\"%s\", name, %s, MakeResourceOptions(options, id), %s)\n",
				tok,
				stateRef,
				"Utilities.PackageParameterization()")
		} else {
			fmt.Fprintf(w, "            : base(\"%s\", name, %s, MakeResourceOptions(options, id))\n", tok, stateRef)
		}

		fmt.Fprintf(w, "        {\n")
		fmt.Fprintf(w, "        }\n")
	}

	if hasConstInputs {
		// Write the method that will calculate the resource arguments.
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "        private static %[1]s MakeArgs(%[1]s args)\n", argsType)
		fmt.Fprintf(w, "        {\n")
		fmt.Fprintf(w, "            args ??= new %s();\n", argsClassName)
		for _, prop := range r.InputProperties {
			if prop.ConstValue != nil {
				v, err := primitiveValue(prop.ConstValue)
				if err != nil {
					return err
				}
				fmt.Fprintf(w, "            args.%s = %s;\n", mod.propertyName(prop), v)
			}
		}
		fmt.Fprintf(w, "            return args;\n")
		fmt.Fprintf(w, "        }\n")
	}

	// Write the method that will calculate the resource options.
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "        private static %[1]s MakeResourceOptions(%[1]s? options, Input<string>? id)\n", optionsType)
	fmt.Fprintf(w, "        {\n")
	fmt.Fprintf(w, "            var defaultOptions = new %s\n", optionsType)
	fmt.Fprintf(w, "            {\n")
	fmt.Fprintf(w, "                Version = Utilities.Version,\n")
	def, err := mod.pkg.Definition()
	if err != nil {
		return err
	}
	if url := def.PluginDownloadURL; url != "" {
		fmt.Fprintf(w, "                PluginDownloadURL = %q,\n", url)
	}

	if len(r.Aliases) > 0 {
		fmt.Fprintf(w, "                Aliases =\n")
		fmt.Fprintf(w, "                {\n")
		for _, alias := range r.Aliases {
			fmt.Fprintf(w, "                    ")
			if alias.Type != nil {
				fmt.Fprintf(w, "new global::Pulumi.Alias { Type = \"%v\" },\n", *alias.Type)
			}
		}
		fmt.Fprintf(w, "                },\n")
	}
	if len(secretProps) > 0 {
		fmt.Fprintf(w, "                AdditionalSecretOutputs =\n")
		fmt.Fprintf(w, "                {\n")
		for _, sp := range secretProps {
			fmt.Fprintf(w, "                    ")
			fmt.Fprintf(w, "%q", sp)
			fmt.Fprintf(w, ",\n")
		}
		fmt.Fprintf(w, "                },\n")
	}

	replaceOnChangesProps, errList := r.ReplaceOnChanges()
	for _, err := range errList {
		cmdutil.Diag().Warningf(&diag.Diag{Message: err.Error()})
	}
	if len(replaceOnChangesProps) > 0 {
		fmt.Fprint(w, "                ReplaceOnChanges =\n")
		fmt.Fprintf(w, "                {\n")
		for _, n := range schema.PropertyListJoinToString(replaceOnChangesProps,
			func(s string) string { return s }) {
			fmt.Fprintf(w, "                    ")
			fmt.Fprintf(w, "%q,\n", n)
		}
		fmt.Fprintf(w, "                },\n")
	}

	fmt.Fprintf(w, "            };\n")
	fmt.Fprintf(w, "            var merged = %s.Merge(defaultOptions, options);\n", optionsType)
	fmt.Fprintf(w, "            // Override the ID if one was specified for consistency with other language SDKs.\n")
	fmt.Fprintf(w, "            merged.Id = id ?? merged.Id;\n")
	fmt.Fprintf(w, "            return merged;\n")
	fmt.Fprintf(w, "        }\n")

	// Write the `Get` method for reading instances of this resource unless this is a provider resource or ComponentResource.
	if !r.IsProvider && !r.IsComponent {
		fmt.Fprintf(w, "        /// <summary>\n")
		fmt.Fprintf(w, "        /// Get an existing %s resource's state with the given name, ID, and optional extra\n", className)
		fmt.Fprintf(w, "        /// properties used to qualify the lookup.\n")
		fmt.Fprintf(w, "        /// </summary>\n")
		fmt.Fprintf(w, "        ///\n")
		fmt.Fprintf(w, "        /// <param name=\"name\">The unique name of the resulting resource.</param>\n")
		fmt.Fprintf(w, "        /// <param name=\"id\">The unique provider ID of the resource to lookup.</param>\n")

		stateParam, stateRef := "", ""
		if r.StateInputs != nil {
			stateParam, stateRef = className+"State? state = null, ", "state, "
			fmt.Fprintf(w, "        /// <param name=\"state\">Any extra arguments used during the lookup.</param>\n")
		}

		fmt.Fprintf(w, "        /// <param name=\"options\">A bag of options that control this resource's behavior</param>\n")
		fmt.Fprintf(w, "        public static %s Get(string name, Input<string> id, %s%s? options = null)\n", className, stateParam, optionsType)
		fmt.Fprintf(w, "        {\n")
		fmt.Fprintf(w, "            return new %s(name, id, %soptions);\n", className, stateRef)
		fmt.Fprintf(w, "        }\n")
	}

	// Generate methods.
	genMethod := func(method *schema.Method) {
		methodName := Title(method.Name)
		fun := method.Function

		var objectReturnType *schema.ObjectType

		if fun.ReturnType != nil {
			if objectType, ok := fun.ReturnType.(*schema.ObjectType); ok && fun.InlineObjectAsReturnType {
				objectReturnType = objectType
			}
		}

		liftReturn := mod.liftSingleValueMethodReturns && objectReturnType != nil && len(objectReturnType.Properties) == 1

		fmt.Fprintf(w, "\n")

		returnType, typeParameter, lift := "void", "", ""
		if fun.ReturnType != nil {
			typeParameter = fmt.Sprintf("<%s%sResult>", className, methodName)
			if liftReturn {
				returnType = fmt.Sprintf("global::Pulumi.Output<%s>",
					mod.typeString(objectReturnType.Properties[0].Type, "", false, false, false))

				fieldName := mod.propertyName(objectReturnType.Properties[0])
				lift = fmt.Sprintf(".Apply(v => v.%s)", fieldName)
			} else {
				returnType = "global::Pulumi.Output" + typeParameter
			}
		}

		var argsParamDef string
		argsParamRef := "CallArgs.Empty"
		if fun.Inputs != nil {
			var hasArgs bool
			allOptionalInputs := true
			for _, arg := range fun.Inputs.InputShape.Properties {
				if arg.Name == "__self__" {
					continue
				}
				hasArgs = true
				allOptionalInputs = allOptionalInputs && !arg.IsRequired()
			}
			if hasArgs {
				var argsDefault, sigil string
				if allOptionalInputs {
					// If the number of required input properties was zero, we can make the args object optional.
					argsDefault, sigil = " = null", "?"
				}

				argsParamDef = fmt.Sprintf("%s%sArgs%s args%s", className, methodName, sigil, argsDefault)
				argsParamRef = fmt.Sprintf("args ?? new %s%sArgs()", className, methodName)
			}
		}

		// Emit the doc comment, if any.
		printComment(w, fun.Comment, "        ")

		if fun.DeprecationMessage != "" {
			fmt.Fprintf(w, "        [Obsolete(@\"%s\")]\n", strings.ReplaceAll(fun.DeprecationMessage, `"`, `""`))
		}

		fmt.Fprintf(w, "        public %s %s(%s)\n", returnType, methodName, argsParamDef)
		if mod.parameterization != nil {
			// pass null for CallOptions parameter
			fmt.Fprintf(w, "            => global::Pulumi.Deployment.Instance.Call%s(\"%s\", %s, this, null, %s)%s;\n",
				typeParameter, fun.Token, argsParamRef, "Utilities.PackageParameterization()", lift)
		} else {
			fmt.Fprintf(w, "            => global::Pulumi.Deployment.Instance.Call%s(\"%s\", %s, this)%s;\n",
				typeParameter, fun.Token, argsParamRef, lift)
		}
	}
	for _, method := range r.Methods {
		genMethod(method)
	}

	// Close the class.
	fmt.Fprintf(w, "    }\n")

	// Arguments are in a different namespace for the Kubernetes SDK.
	if mod.isK8sCompatMode() && !r.IsProvider {
		// Close the namespace.
		fmt.Fprintf(w, "}\n")

		// Open the namespace.
		fmt.Fprintf(w, "namespace %s\n", mod.tokenToNamespace(r.Token, "Inputs"))
		fmt.Fprintf(w, "{\n")
	}

	// Generate the resource args type.
	args := &plainType{
		mod:                   mod,
		res:                   r,
		name:                  name + "Args",
		baseClass:             "ResourceArgs",
		propertyTypeQualifier: "Inputs",
		properties:            r.InputProperties,
		args:                  true,
	}
	if err := args.genInputType(w, 1); err != nil {
		return err
	}

	// Generate the `Get` args type, if any.
	if r.StateInputs != nil {
		state := &plainType{
			mod:                   mod,
			res:                   r,
			name:                  name + "State",
			baseClass:             "ResourceArgs",
			propertyTypeQualifier: "Inputs",
			properties:            r.StateInputs.Properties,
			args:                  true,
			state:                 true,
		}
		if err := state.genInputType(w, 1); err != nil {
			return err
		}
	}

	// Generate method types.
	genMethodTypes := func(method *schema.Method) error {
		methodName := Title(method.Name)
		fun := method.Function

		// Generate args type.
		var args []*schema.Property
		if fun.Inputs != nil {
			// Filter out the __self__ argument from the inputs.
			args = slice.Prealloc[*schema.Property](len(fun.Inputs.InputShape.Properties) - 1)
			for _, arg := range fun.Inputs.InputShape.Properties {
				if arg.Name == "__self__" {
					continue
				}
				args = append(args, arg)
			}
		}
		if len(args) > 0 {
			comment, escape := fun.Inputs.Comment, true
			if comment == "" {
				comment, escape = fmt.Sprintf(
					"The set of arguments for the <see cref=\"%s.%s\"/> method.", className, methodName), false
			}
			argsType := &plainType{
				mod:                   mod,
				comment:               comment,
				unescapeComment:       !escape,
				name:                  fmt.Sprintf("%s%sArgs", className, methodName),
				baseClass:             "CallArgs",
				propertyTypeQualifier: "Inputs",
				properties:            args,
				args:                  true,
			}
			if err := argsType.genInputType(w, 1); err != nil {
				return err
			}
		}

		var objectReturnType *schema.ObjectType
		if fun.ReturnType != nil {
			if objectType, ok := fun.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				objectReturnType = objectType
			}
		}

		// Generate result type.
		if objectReturnType != nil {
			shouldLiftReturn := mod.liftSingleValueMethodReturns && len(objectReturnType.Properties) == 1

			comment, escape := fun.Inputs.Comment, true
			if comment == "" {
				comment, escape = fmt.Sprintf(
					"The results of the <see cref=\"%s.%s\"/> method.", className, methodName), false
			}
			resultType := &plainType{
				mod:                   mod,
				comment:               comment,
				unescapeComment:       !escape,
				name:                  fmt.Sprintf("%s%sResult", className, methodName),
				propertyTypeQualifier: "Outputs",
				properties:            objectReturnType.Properties,
				internal:              shouldLiftReturn,
			}
			resultType.genOutputType(w, 1)
		}

		return nil
	}
	for _, method := range r.Methods {
		if err := genMethodTypes(method); err != nil {
			return err
		}
	}

	// Close the namespace.
	fmt.Fprintf(w, "}\n")

	return nil
}

func (mod *modContext) genFunctionFileCode(f *schema.Function) (string, error) {
	buffer := &bytes.Buffer{}
	importStrings := mod.pulumiImports()

	// True if the function has a non-standard namespace.
	nonStandardNamespace := mod.namespaceName != mod.tokenToNamespace(f.Token, "")
	// If so, we need to import our project defined types.
	if nonStandardNamespace {
		importStrings = append(importStrings, mod.namespaceName)
	}

	// We need to qualify input types when we are not in the same module as them.
	if nonStandardNamespace {
		defer func(current bool) { mod.fullyQualifiedInputs = current }(mod.fullyQualifiedInputs)
		mod.fullyQualifiedInputs = true
	}
	mod.genHeader(buffer, importStrings)
	if err := mod.genFunction(buffer, f); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func allOptionalInputs(fun *schema.Function) bool {
	if fun.Inputs != nil {
		for _, prop := range fun.Inputs.Properties {
			if prop.IsRequired() {
				return false
			}
		}
	}
	return true
}

func typeParamOrEmpty(typeParamName string) string {
	if typeParamName != "" {
		return fmt.Sprintf("<%s>", typeParamName)
	}

	return ""
}

func (mod *modContext) functionReturnType(fun *schema.Function) string {
	className := tokenToFunctionName(fun.Token)
	if fun.ReturnType != nil {
		if _, ok := fun.ReturnType.(*schema.ObjectType); ok && fun.InlineObjectAsReturnType {
			// for object return types, assume a Result type is generated in the same class as it's function
			// and reference it from here directly
			return className + "Result"
		}

		// otherwise, the object type is a reference to an output type
		return mod.typeString(fun.ReturnType, "Outputs", false, false, true)
	}

	return ""
}

// runtimeInvokeFunction returns the name of the Invoke function to use at runtime
// from the SDK for the given provider function. This is necessary because some
// functions have simple return types such as number, string, array<string> etc.
// and the SDK's Invoke function cannot handle these types since the engine expects
// the result of invokes to be a dictionary.
//
// We use Invoke for functions with object return types and InvokeSingle for everything else.
func runtimeInvokeFunction(fun *schema.Function) string {
	switch fun.ReturnType.(type) {
	// If the function has no return type, it is a void function.
	case nil:
		return "Invoke"
	// If the function has an object return type, it is a normal invoke function.
	case *schema.ObjectType:
		return "Invoke"
	// If the function has an object return type, it is also a normal invoke function.
	// because the deserialization can handle it
	case *schema.MapType:
		return "Invoke"
	default:
		// Anything else needs to be handled by InvokeSingle
		// which expects an object with a single property to be returned
		// then unwraps the value from that property
		return "InvokeSingle"
	}
}

func (mod *modContext) genFunction(w io.Writer, fun *schema.Function) error {
	className := tokenToFunctionName(fun.Token)

	fmt.Fprintf(w, "namespace %s\n", mod.tokenToNamespace(fun.Token, ""))
	fmt.Fprintf(w, "{\n")

	typeParameter := mod.functionReturnType(fun)

	var argsParamDef string
	argsParamRef := "InvokeArgs.Empty"
	if fun.Inputs != nil {
		var argsDefault, sigil string
		if allOptionalInputs(fun) {
			// If the number of required input properties was zero, we can make the args object optional.
			argsDefault, sigil = " = null", "?"
		}

		argsParamDef = fmt.Sprintf("%sArgs%s args%s, ", className, sigil, argsDefault)
		argsParamRef = fmt.Sprintf("args ?? new %sArgs()", className)
	}

	if fun.DeprecationMessage != "" {
		fmt.Fprintf(w, "    [Obsolete(@\"%s\")]\n", strings.ReplaceAll(fun.DeprecationMessage, `"`, `""`))
	}

	// Open the class we'll use for data sources.
	fmt.Fprintf(w, "    public static class %s\n", className)
	fmt.Fprintf(w, "    {\n")

	// Emit the doc comment, if any.
	printComment(w, fun.Comment, "        ")
	invokeCall := runtimeInvokeFunction(fun)
	if !fun.MultiArgumentInputs {
		// Emit the datasource method.
		// this is default behavior for all functions.
		fmt.Fprintf(w, "        public static Task%s InvokeAsync(%sInvokeOptions? options = null)\n",
			typeParamOrEmpty(typeParameter), argsParamDef)
		// new line and indent
		fmt.Fprint(w, "            ")
		fmt.Fprintf(w, "=> global::Pulumi.Deployment.Instance.%sAsync%s", invokeCall, typeParamOrEmpty(typeParameter))
		if mod.parameterization != nil {
			fmt.Fprintf(w, "(\"%s\", %s, options.WithDefaults(), %s);\n",
				fun.Token,
				argsParamRef,
				"Utilities.PackageParameterization()")
		} else {
			fmt.Fprintf(w, "(\"%s\", %s, options.WithDefaults());\n", fun.Token, argsParamRef)
		}
	} else {
		// multi-argument inputs and output property bag
		// first generate the function definition
		fmt.Fprintf(w, "        public static async Task%s InvokeAsync(", typeParamOrEmpty(typeParameter))
		for _, prop := range fun.Inputs.Properties {
			argumentName := LowerCamelCase(prop.Name)
			argumentType := mod.typeString(prop.Type, "", false, false, true)
			paramDeclaration := fmt.Sprintf("%s %s", argumentType, argumentName)
			if !prop.IsRequired() {
				paramDeclaration += " = null"
			}

			fmt.Fprintf(w, "%s", paramDeclaration)
			fmt.Fprint(w, ", ")
		}

		fmt.Fprint(w, "InvokeOptions? invokeOptions = null)\n")

		funcBodyIndent := func() {
			fmt.Fprintf(w, "            ")
		}

		// now the function body
		fmt.Fprint(w, "        {\n")
		// generate a dictionary where each entry is a key-value pair made out of the inputs of the function
		funcBodyIndent()
		fmt.Fprint(w, "var builder = ImmutableDictionary.CreateBuilder<string, object?>();\n")
		for _, prop := range fun.Inputs.Properties {
			argumentName := LowerCamelCase(prop.Name)
			funcBodyIndent()
			fmt.Fprintf(w, "builder[\"%s\"] = %s;\n", prop.Name, argumentName)
		}

		funcBodyIndent()
		fmt.Fprint(w, "var args = new global::Pulumi.DictionaryInvokeArgs(builder.ToImmutableDictionary());\n")
		funcBodyIndent()
		// full invoke call
		fmt.Fprint(w, "return await global::Pulumi.Deployment.Instance.")
		fmt.Fprintf(w, "%sAsync%s", invokeCall, typeParamOrEmpty(typeParameter))
		if mod.parameterization != nil {
			fmt.Fprintf(w, "(\"%s\", args, invokeOptions.WithDefaults(), Utilities.PackageParameterization());\n", fun.Token)
		} else {
			fmt.Fprintf(w, "(\"%s\", args, invokeOptions.WithDefaults());\n", fun.Token)
		}

		fmt.Fprint(w, "        }\n")
	}

	// Emit the Output method with InvokeOptions
	err := mod.genFunctionOutputVersion(w, fun, false /* outputOptions */)
	if err != nil {
		return err
	}

	// Emit the Output method with InvokeOutputOptions
	//
	// We have to make the `InvokeOutputOptions options` required so that
	// it can be differentiated from the overload that takes `InvokeOptions
	// options`. This means that preceding arguments to the method must also be
	// required.
	//
	// When using multi-argument inputs, this would make all the optional
	// arguments required. For now we skip the InvokeOutputOptions variant for
	// multi-argument inputs.
	if !fun.MultiArgumentInputs {
		err = mod.genFunctionOutputVersion(w, fun, true /* outputOptions */)
		if err != nil {
			return err
		}
	}

	// Close the class.
	fmt.Fprintf(w, "    }\n")

	// Emit the args and result types, if any.
	if fun.Inputs != nil && !fun.MultiArgumentInputs {
		fmt.Fprintf(w, "\n")

		args := &plainType{
			mod:                   mod,
			name:                  className + "Args",
			baseClass:             "InvokeArgs",
			propertyTypeQualifier: "Inputs",
			properties:            fun.Inputs.Properties,
		}
		if err := args.genInputType(w, 1); err != nil {
			return err
		}
	}

	err = mod.genFunctionOutputVersionTypes(w, fun)
	if err != nil {
		return err
	}

	if fun.ReturnType != nil {
		if objectType, ok := fun.ReturnType.(*schema.ObjectType); ok && fun.InlineObjectAsReturnType {
			fmt.Fprintf(w, "\n")

			res := &plainType{
				mod:                   mod,
				name:                  className + "Result",
				propertyTypeQualifier: "Outputs",
				properties:            objectType.Properties,
			}
			res.genOutputType(w, 1)
		}
	}

	// Close the namespace.
	fmt.Fprintf(w, "}\n")
	return nil
}

func functionOutputVersionArgsTypeName(fun *schema.Function) string {
	className := tokenToFunctionName(fun.Token)
	return className + "InvokeArgs"
}

// Generates `${fn}Output(..)` version lifted to work on
// `Input`-wrapped arguments and producing an `Output`-wrapped result.
func (mod *modContext) genFunctionOutputVersion(w io.Writer, fun *schema.Function, outputOptions bool) error {
	if fun.ReturnType == nil {
		// no need to generate an output version if the function doesn't return anything
		return nil
	}

	if fun.MultiArgumentInputs && outputOptions {
		return errors.New("outputOptions is not supported when using multi-argument inputs")
	}

	var argsDefault, sigil string
	if allOptionalInputs(fun) {
		// If the number of required input properties was zero, we can make the args object optional.
		argsDefault, sigil = " = null", "?"
	}

	typeParameter := mod.functionReturnType(fun)
	invokeCall := runtimeInvokeFunction(fun)
	argsTypeName := functionOutputVersionArgsTypeName(fun)
	outputArgsParamDef := fmt.Sprintf("%s%s args%s, ", argsTypeName, sigil, argsDefault)
	outputArgsParamRef := fmt.Sprintf("args ?? new %s()", argsTypeName)

	if fun.Inputs == nil || len(fun.Inputs.Properties) == 0 {
		outputArgsParamDef = ""
		outputArgsParamRef = "InvokeArgs.Empty"
	}

	fmt.Fprintf(w, "\n")
	// Emit the doc comment, if any.
	printComment(w, fun.Comment, "        ")

	if !fun.MultiArgumentInputs {
		if outputOptions {
			// For the InvokeOutputOptions variant, arguments can not optional, otherwise we
			// can't differentiate between the two options variants.
			if outputArgsParamDef != "" && argsDefault != "" {
				outputArgsParamDef = argsTypeName + " args, "
			}
			fmt.Fprintf(w, "        public static Output%s Invoke(%sInvokeOutputOptions options)\n",
				typeParamOrEmpty(typeParameter), outputArgsParamDef)
		} else {
			fmt.Fprintf(w, "        public static Output%s Invoke(%sInvokeOptions? options = null)\n",
				typeParamOrEmpty(typeParameter), outputArgsParamDef)
		}
		fmt.Fprintf(w, "            => global::Pulumi.Deployment.Instance.%s%s(\"%s\", %s, options.WithDefaults());\n",
			invokeCall, typeParamOrEmpty(typeParameter), fun.Token, outputArgsParamRef)
	} else {
		fmt.Fprintf(w, "        public static Output%s Invoke(", typeParamOrEmpty(typeParameter))
		for _, prop := range fun.Inputs.Properties {
			var paramDeclaration string
			argumentName := LowerCamelCase(prop.Name)
			propertyType := &schema.InputType{ElementType: prop.Type}
			argumentType := mod.typeString(propertyType, "", true /* input */, false, true)
			if prop.IsRequired() {
				paramDeclaration = fmt.Sprintf("%s %s", argumentType, argumentName)
			} else {
				paramDeclaration = fmt.Sprintf("%s? %s = null", argumentType, argumentName)
			}

			fmt.Fprintf(w, "%s", paramDeclaration)
			fmt.Fprint(w, ", ")
		}

		fmt.Fprint(w, "InvokeOptions? invokeOptions = null)\n")

		// now the function body
		fmt.Fprint(w, "        {\n")
		fmt.Fprint(w, "            var builder = ImmutableDictionary.CreateBuilder<string, object?>();\n")
		if fun.Inputs != nil {
			for _, prop := range fun.Inputs.Properties {
				argumentName := LowerCamelCase(prop.Name)
				fmt.Fprintf(w, "            builder[\"%s\"] = %s;\n", prop.Name, argumentName)
			}
		}
		fmt.Fprint(w, "            var args = new global::Pulumi.DictionaryInvokeArgs(builder.ToImmutableDictionary());\n")
		fmt.Fprintf(w, "            return global::Pulumi.Deployment.Instance.%s%s(\"%s\", args, invokeOptions.WithDefaults());\n",
			invokeCall, typeParamOrEmpty(typeParameter), fun.Token)
		fmt.Fprint(w, "        }\n")
	}

	return nil
}

// Generate helper type definitions referred to in `genFunctionOutputVersion`.
func (mod *modContext) genFunctionOutputVersionTypes(w io.Writer, fun *schema.Function) error {
	if fun.Inputs == nil || fun.ReturnType == nil || len(fun.Inputs.Properties) == 0 {
		return nil
	}

	if !fun.MultiArgumentInputs {
		applyArgs := &plainType{
			mod:                   mod,
			name:                  functionOutputVersionArgsTypeName(fun),
			propertyTypeQualifier: "Inputs",
			baseClass:             "InvokeArgs",
			properties:            fun.Inputs.InputShape.Properties,
			args:                  true,
		}

		if err := applyArgs.genInputTypeWithFlags(w, 1, true /* generateInputAttributes */); err != nil {
			return err
		}
	}

	return nil
}

func (mod *modContext) genEnums(w io.Writer, enums []*schema.EnumType) error {
	// Open the namespace.
	fmt.Fprintf(w, "namespace %s\n", mod.namespaceName)
	fmt.Fprintf(w, "{\n")

	for i, enum := range enums {
		err := mod.genEnum(w, enum)
		if err != nil {
			return err
		}
		if i != len(enums)-1 {
			fmt.Fprintf(w, "\n")
		}
	}

	// Close the namespace.
	fmt.Fprintf(w, "}\n")

	return nil
}

func printObsoleteAttribute(w io.Writer, deprecationMessage, indent string) {
	if deprecationMessage != "" {
		fmt.Fprintf(w, "%s[Obsolete(@\"%s\")]\n", indent, strings.ReplaceAll(deprecationMessage, `"`, `""`))
	}
}

func (mod *modContext) genEnum(w io.Writer, enum *schema.EnumType) error {
	indent := "    "
	enumName := tokenToName(enum.Token)

	// Fix up identifiers for each enum value.
	for _, e := range enum.Elements {
		// If the enum doesn't have a name, set the value as the name.
		if e.Name == "" {
			e.Name = fmt.Sprintf("%v", e.Value)
		}

		safeName, err := makeSafeEnumName(e.Name, enumName)
		if err != nil {
			return err
		}
		e.Name = safeName
	}

	// Print documentation comment
	printComment(w, enum.Comment, indent)

	underlyingType := mod.typeString(enum.ElementType, "", false, false, false)
	switch enum.ElementType {
	case schema.StringType, schema.NumberType:
		// EnumType attribute
		fmt.Fprintf(w, "%s[EnumType]\n", indent)

		// Open struct declaration
		fmt.Fprintf(w, "%[1]spublic readonly struct %[2]s : IEquatable<%[2]s>\n", indent, enumName)
		fmt.Fprintf(w, "%s{\n", indent)
		indent := strings.Repeat(indent, 2)
		fmt.Fprintf(w, "%sprivate readonly %s _value;\n", indent, underlyingType)
		fmt.Fprintf(w, "\n")

		// Constructor
		fmt.Fprintf(w, "%sprivate %s(%s value)\n", indent, enumName, underlyingType)
		fmt.Fprintf(w, "%s{\n", indent)
		fmt.Fprintf(w, "%s    _value = value", indent)
		if enum.ElementType == schema.StringType {
			fmt.Fprintf(w, " ?? throw new ArgumentNullException(nameof(value))")
		}
		fmt.Fprintf(w, ";\n")
		fmt.Fprintf(w, "%s}\n", indent)
		fmt.Fprintf(w, "\n")

		// Enum values
		for _, e := range enum.Elements {
			printComment(w, e.Comment, indent)
			printObsoleteAttribute(w, e.DeprecationMessage, indent)
			fmt.Fprintf(w, "%[1]spublic static %[2]s %[3]s { get; } = new %[2]s(", indent, enumName, e.Name)
			if enum.ElementType == schema.StringType {
				fmt.Fprintf(w, "%q", e.Value)
			} else {
				fmt.Fprintf(w, "%v", e.Value)
			}
			fmt.Fprintf(w, ");\n")
		}
		fmt.Fprintf(w, "\n")

		// Equality and inequality operators
		fmt.Fprintf(w, "%[1]spublic static bool operator ==(%[2]s left, %[2]s right) => left.Equals(right);\n", indent, enumName)
		fmt.Fprintf(w, "%[1]spublic static bool operator !=(%[2]s left, %[2]s right) => !left.Equals(right);\n", indent, enumName)
		fmt.Fprintf(w, "\n")

		// Explicit conversion operator
		fmt.Fprintf(w, "%[1]spublic static explicit operator %s(%s value) => value._value;\n", indent, underlyingType, enumName)
		fmt.Fprintf(w, "\n")

		// Equals override
		fmt.Fprintf(w, "%s[EditorBrowsable(EditorBrowsableState.Never)]\n", indent)
		fmt.Fprintf(w, "%spublic override bool Equals(object? obj) => obj is %s other && Equals(other);\n", indent, enumName)
		fmt.Fprintf(w, "%spublic bool Equals(%s other) => ", indent, enumName)
		if enum.ElementType == schema.StringType {
			fmt.Fprintf(w, "string.Equals(_value, other._value, StringComparison.Ordinal)")
		} else {
			fmt.Fprintf(w, "_value == other._value")
		}
		fmt.Fprintf(w, ";\n")
		fmt.Fprintf(w, "\n")

		// GetHashCode override
		fmt.Fprintf(w, "%s[EditorBrowsable(EditorBrowsableState.Never)]\n", indent)
		fmt.Fprintf(w, "%spublic override int GetHashCode() => _value", indent)
		if enum.ElementType == schema.StringType {
			fmt.Fprintf(w, "?")
		}
		fmt.Fprintf(w, ".GetHashCode()")
		if enum.ElementType == schema.StringType {
			fmt.Fprintf(w, " ?? 0")
		}
		fmt.Fprintf(w, ";\n")
		fmt.Fprintf(w, "\n")

		// ToString override
		fmt.Fprintf(w, "%spublic override string ToString() => _value", indent)
		if enum.ElementType == schema.NumberType {
			fmt.Fprintf(w, ".ToString()")
		}
		fmt.Fprintf(w, ";\n")
	case schema.IntType:
		// Open enum declaration
		fmt.Fprintf(w, "%spublic enum %s\n", indent, enumName)
		fmt.Fprintf(w, "%s{\n", indent)
		for _, e := range enum.Elements {
			indent := strings.Repeat(indent, 2)
			printComment(w, e.Comment, indent)
			printObsoleteAttribute(w, e.DeprecationMessage, indent)
			fmt.Fprintf(w, "%s%s = %v,\n", indent, e.Name, e.Value)
		}
	default:
		// Issue to implement boolean-based enums: https://github.com/pulumi/pulumi/issues/5652
		return fmt.Errorf("enums of type %s are not yet implemented for this language", enum.ElementType.String())
	}

	// Close the declaration
	fmt.Fprintf(w, "%s}\n", indent)

	return nil
}

func visitObjectTypes(properties []*schema.Property, visitor func(*schema.ObjectType)) {
	codegen.VisitTypeClosure(properties, func(t schema.Type) {
		if o, ok := t.(*schema.ObjectType); ok {
			visitor(o)
		}
	})
}

func (mod *modContext) genType(w io.Writer, obj *schema.ObjectType, propertyTypeQualifier string, input, state bool, level int) error {
	args := obj.IsInputShape()

	pt := &plainType{
		mod:                   mod,
		name:                  mod.typeName(obj, state, input, args),
		comment:               obj.Comment,
		propertyTypeQualifier: propertyTypeQualifier,
		properties:            obj.Properties,
		state:                 state,
		args:                  args,
	}

	if input {
		pt.baseClass = "ResourceArgs"
		if !args && mod.details(obj).plainType {
			pt.baseClass = "InvokeArgs"
		}
		return pt.genInputType(w, level)
	}

	pt.genOutputType(w, level)
	return nil
}

// pulumiImports is a slice of common imports that are used with the genHeader method.
func (mod *modContext) pulumiImports() []string {
	pulumiImports := []string{
		"System",
		"System.Collections.Generic",
		"System.Collections.Immutable",
		"System.Threading.Tasks",
		"Pulumi.Serialization",
	}
	if mod.RootNamespace() != "Pulumi" {
		pulumiImports = append(pulumiImports, "Pulumi")
	}
	return pulumiImports
}

func (mod *modContext) genHeader(w io.Writer, using []string) {
	fmt.Fprintf(w, "// *** WARNING: this file was generated by %v. ***\n", mod.tool)
	fmt.Fprintf(w, "// *** Do not edit by hand unless you're certain you know what you are doing! ***\n")
	fmt.Fprintf(w, "\n")

	for _, u := range using {
		fmt.Fprintf(w, "using %s;\n", u)
	}
	if len(using) > 0 {
		fmt.Fprintf(w, "\n")
	}
}

func (mod *modContext) getConfigProperty(schemaType schema.Type) (string, string) {
	schemaType = codegen.UnwrapType(schemaType)

	qualifier := "Types"
	propertyType := mod.typeString(schemaType, qualifier, false, false, false /*requireInitializers*/)

	var getFunc string
	nullableSigil := "?"
	switch schemaType {
	case schema.StringType:
		getFunc = "Get"
	case schema.BoolType:
		getFunc = "GetBoolean"
	case schema.IntType:
		getFunc = "GetInt32"
	case schema.NumberType:
		getFunc = "GetDouble"
	default:
		switch t := schemaType.(type) {
		case *schema.TokenType:
			if t.UnderlyingType != nil {
				return mod.getConfigProperty(t.UnderlyingType)
			}
		}

		getFunc = "GetObject<" + propertyType + ">"
		if _, ok := schemaType.(*schema.ArrayType); ok {
			nullableSigil = ""
		}
	}
	return propertyType + nullableSigil, getFunc
}

func (mod *modContext) genConfig(variables []*schema.Property) (string, error) {
	w := &bytes.Buffer{}

	mod.genHeader(w, []string{"System", "System.Collections.Immutable"})
	// Use the root namespace to avoid `Pulumi.Provider.Config.Config.VarName` usage.
	fmt.Fprintf(w, "namespace %s\n", mod.namespaceName)
	fmt.Fprintf(w, "{\n")

	// Open the config class.
	fmt.Fprintf(w, "    public static class Config\n")
	fmt.Fprintf(w, "    {\n")

	fmt.Fprintf(w, "        [global::System.Diagnostics.CodeAnalysis.SuppressMessage(\"Microsoft.Design\", \"IDE1006\", Justification = \n")
	fmt.Fprintf(w, "        \"Double underscore prefix used to avoid conflicts with variable names.\")]\n")
	fmt.Fprintf(w, "        private sealed class __Value<T>\n")
	fmt.Fprintf(w, "        {\n")

	fmt.Fprintf(w, "            private readonly Func<T> _getter;\n")
	fmt.Fprintf(w, "            private T _value = default!;\n")
	fmt.Fprintf(w, "            private bool _set;\n")
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "            public __Value(Func<T> getter)\n")
	fmt.Fprintf(w, "            {\n")
	fmt.Fprintf(w, "                _getter = getter;\n")
	fmt.Fprintf(w, "            }\n")
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "            public T Get() => _set ? _value : _getter();\n")
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "            public void Set(T value)\n")
	fmt.Fprintf(w, "            {\n")
	fmt.Fprintf(w, "                _value = value;\n")
	fmt.Fprintf(w, "                _set = true;\n")
	fmt.Fprintf(w, "            }\n")
	fmt.Fprintf(w, "        }\n")
	fmt.Fprintf(w, "\n")

	// Create a config bag for the variables to pull from.
	fmt.Fprintf(w, "        private static readonly global::Pulumi.Config __config = new global::Pulumi.Config(\"%v\");\n", mod.pkg.Name())
	fmt.Fprintf(w, "\n")

	// Emit an entry for all config variables.
	for _, p := range variables {
		propertyType, getFunc := mod.getConfigProperty(p.Type)

		propertyName := mod.propertyName(p)

		initializer := fmt.Sprintf("__config.%s(\"%s\")", getFunc, p.Name)
		if p.DefaultValue != nil {
			dv, err := mod.getDefaultValue(p.DefaultValue, p.Type)
			if err != nil {
				return "", err
			}
			initializer += " ?? " + dv
		}

		fmt.Fprintf(w, "        private static readonly __Value<%[1]s> _%[2]s = new __Value<%[1]s>(() => %[3]s);\n", propertyType, p.Name, initializer)
		printComment(w, p.Comment, "        ")
		fmt.Fprintf(w, "        public static %s %s\n", propertyType, propertyName)
		fmt.Fprintf(w, "        {\n")
		fmt.Fprintf(w, "            get => _%s.Get();\n", p.Name)
		fmt.Fprintf(w, "            set => _%s.Set(value);\n", p.Name)
		fmt.Fprintf(w, "        }\n")
		fmt.Fprintf(w, "\n")
	}

	// generate config-friendly object types used in config
	// regardless of whether they are defined inline or from a shared type
	var usedObjectTypes []*schema.ObjectType
	visitObjectTypes(variables, func(objectType *schema.ObjectType) {
		usedObjectTypes = append(usedObjectTypes, objectType)
	})

	// Emit any nested types.
	if len(usedObjectTypes) > 0 {
		fmt.Fprintf(w, "        public static class Types\n")
		fmt.Fprintf(w, "        {\n")

		for _, typ := range usedObjectTypes {
			fmt.Fprintf(w, "\n")

			// Open the class.
			fmt.Fprintf(w, "             public class %s\n", tokenToName(typ.Token))
			fmt.Fprintf(w, "             {\n")

			// Generate each output field.
			for _, prop := range typ.Properties {
				name := mod.propertyName(prop)
				typ := mod.typeString(prop.Type, "Types", false, false, false)

				initializer := ""
				if !prop.IsRequired() && !isValueType(prop.Type) && !isImmutableArrayType(codegen.UnwrapType(prop.Type), false) {
					initializer = " = null!;"
				}

				printComment(w, prop.Comment, "            ")
				fmt.Fprintf(w, "                public %s %s { get; set; }%s\n", typ, name, initializer)
			}

			// Close the class.
			fmt.Fprintf(w, "            }\n")
		}

		fmt.Fprintf(w, "        }\n")
	}

	// Close the config class and namespace.
	fmt.Fprintf(w, "    }\n")

	// Close the namespace.
	fmt.Fprintf(w, "}\n")

	return w.String(), nil
}

func (mod *modContext) genUtilities() (string, error) {
	// Strip any 'v' off of the version.
	w := &bytes.Buffer{}
	def, err := mod.pkg.Definition()
	if err != nil {
		return "", err
	}

	var version string
	if def.Version != nil {
		version = def.Version.String()
	}
	templateData := csharpUtilitiesTemplateContext{
		Name:                namespaceName(mod.namespaces, mod.pkg.Name()),
		Namespace:           mod.namespaceName,
		ClassName:           "Utilities",
		Tool:                mod.tool,
		PluginDownloadURL:   def.PluginDownloadURL,
		HasParameterization: def.Parameterization != nil,
		PackageName:         def.Name,
		PackageVersion:      version,
	}

	if def.Parameterization != nil {
		templateData.BaseProviderName = def.Parameterization.BaseProvider.Name
		templateData.BaseProviderVersion = def.Parameterization.BaseProvider.Version.String()
		templateData.BaseProviderPluginDownloadURL = def.PluginDownloadURL
		templateData.ParameterValue = base64.StdEncoding.EncodeToString(def.Parameterization.Parameter)
	}

	err = csharpUtilitiesTemplate.Execute(w, templateData)
	if err != nil {
		return "", err
	}

	return w.String(), nil
}

func (mod *modContext) gen(fs codegen.Fs) error {
	nsComponents := strings.Split(mod.namespaceName, ".")
	if len(nsComponents) > 0 {
		// Trim off "Pulumi.Pkg"
		nsComponents = nsComponents[2:]
	}

	dir := path.Join(nsComponents...)
	if mod.mod == "config" {
		dir = "Config"
	}

	var files []string
	for p := range fs {
		d := path.Dir(p)
		if d == "." {
			d = ""
		}
		if d == dir {
			files = append(files, p)
		}
	}

	addFile := func(name, contents string) {
		p := path.Join(dir, name)
		files = append(files, p)
		fs.Add(p, []byte(contents))
	}

	// Ensure that the target module directory contains a README.md file.
	readme := mod.pkg.Description()
	if readme != "" && readme[len(readme)-1] != '\n' {
		readme += "\n"
	}
	fs.Add(filepath.Join(dir, "README.md"), []byte(readme))

	// Utilities, config
	switch mod.mod {
	case "":
		utilities, err := mod.genUtilities()
		if err != nil {
			return err
		}
		fs.Add("Utilities.cs", []byte(utilities))
	case "config":
		config, err := mod.pkg.Config()
		if err != nil {
			return err
		}
		if len(config) > 0 {
			config, err := mod.genConfig(config)
			if err != nil {
				return err
			}
			addFile("Config.cs", config)
			return nil
		}
	}

	// Resources
	for _, r := range mod.resources {
		if r.IsOverlay {
			// This resource code is generated by the provider, so no further action is required.
			continue
		}

		buffer := &bytes.Buffer{}
		importStrings := mod.pulumiImports()
		mod.genHeader(buffer, importStrings)

		if err := mod.genResource(buffer, r); err != nil {
			return err
		}

		addFile(resourceName(r)+".cs", buffer.String())
	}

	// Functions
	for _, f := range mod.functions {
		if f.IsOverlay {
			// This function code is generated by the provider, so no further action is required.
			continue
		}

		code, err := mod.genFunctionFileCode(f)
		if err != nil {
			return err
		}
		addFile(tokenToName(f.Token)+".cs", code)
	}

	// Nested types
	for _, t := range mod.types {
		if t.IsOverlay {
			// This type is generated by the provider, so no further action is required.
			continue
		}

		if mod.details(t).inputType {
			buffer := &bytes.Buffer{}
			mod.genHeader(buffer, mod.pulumiImports())

			fmt.Fprintf(buffer, "namespace %s\n", mod.tokenToNamespace(t.Token, "Inputs"))
			fmt.Fprintf(buffer, "{\n")

			if err := mod.genType(buffer, t, "Inputs", true, false, 1); err != nil {
				return err
			}

			fmt.Fprintf(buffer, "}\n")

			name := tokenToName(t.Token)
			if t.IsInputShape() {
				name += "Args"
			}
			addFile(path.Join("Inputs", name+".cs"), buffer.String())
		}
		if mod.details(t).stateType {
			buffer := &bytes.Buffer{}
			mod.genHeader(buffer, mod.pulumiImports())

			fmt.Fprintf(buffer, "namespace %s\n", mod.tokenToNamespace(t.Token, "Inputs"))
			fmt.Fprintf(buffer, "{\n")
			if err := mod.genType(buffer, t, "Inputs", true, true, 1); err != nil {
				return err
			}
			fmt.Fprintf(buffer, "}\n")
			addFile(path.Join("Inputs", tokenToName(t.Token)+"GetArgs.cs"), buffer.String())
		}
		if mod.details(t).outputType {
			buffer := &bytes.Buffer{}
			mod.genHeader(buffer, mod.pulumiImports())

			fmt.Fprintf(buffer, "namespace %s\n", mod.tokenToNamespace(t.Token, "Outputs"))
			fmt.Fprintf(buffer, "{\n")
			if err := mod.genType(buffer, t, "Outputs", false, false, 1); err != nil {
				return err
			}
			fmt.Fprintf(buffer, "}\n")

			suffix := ""
			if (mod.isTFCompatMode() || mod.isK8sCompatMode()) && mod.details(t).plainType {
				suffix = "Result"
			}
			addFile(path.Join("Outputs", tokenToName(t.Token)+suffix+".cs"), buffer.String())
		}
	}

	// Enums
	if len(mod.enums) > 0 {
		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, []string{"System", "System.ComponentModel", "Pulumi"})

		if err := mod.genEnums(buffer, mod.enums); err != nil {
			return err
		}

		addFile("Enums.cs", buffer.String())
	}
	return nil
}

// genPackageMetadata generates all the non-code metadata required by a Pulumi package.
func genPackageMetadata(pkg *schema.Package,
	assemblyName string,
	packageReferences map[string]string,
	projectReferences []string,
	files codegen.Fs,
	localDependencies map[string]string,
) error {
	version := ""
	lang, ok := pkg.Language["csharp"].(CSharpPackageInfo)
	if pkg.Version != nil && ok && lang.RespectSchemaVersion {
		version = pkg.Version.String()
		files.Add("version.txt", []byte(version))
	} else if pkg.SupportPack {
		if pkg.Version == nil {
			return errors.New("package version is required")
		}
		version = pkg.Version.String()
		files.Add("version.txt", []byte(version))
	}

	projectFile, err := genProjectFile(pkg, assemblyName, packageReferences, projectReferences, version, localDependencies)
	if err != nil {
		return err
	}
	logo, err := getLogo(pkg)
	if err != nil {
		return err
	}

	pulumiPlugin := &plugin.PulumiPluginJSON{
		Resource: true,
		Name:     pkg.Name,
		Server:   pkg.PluginDownloadURL,
		Version:  version,
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

	plugin, err := (pulumiPlugin).JSON()
	if err != nil {
		return err
	}

	files.Add(assemblyName+".csproj", projectFile)
	files.Add("logo.png", logo)
	files.Add("pulumi-plugin.json", plugin)
	return nil
}

// genProjectFile emits a C# project file into the configured output directory.
func genProjectFile(pkg *schema.Package,
	assemblyName string,
	packageReferences map[string]string,
	projectReferences []string,
	version string,
	localDependencies map[string]string,
) ([]byte, error) {
	if packageReferences == nil {
		packageReferences = map[string]string{}
	}

	// Find all the local dependency folders
	folders := mapset.NewSet[string]()
	for _, dep := range localDependencies {
		folders.Add(path.Dir(dep))
	}
	restoreSources := ""
	if len(folders.ToSlice()) > 0 {
		restoreSources = strings.Join(folders.ToSlice(), ";")
	}

	// Add local package references
	pkgs := codegen.SortedKeys(localDependencies)
	for _, pkg := range pkgs {
		nugetFilePath := localDependencies[pkg]
		if packageName, version, ok := extractNugetPackageNameAndVersion(nugetFilePath); ok {
			packageReferences[packageName] = version
		} else {
			return nil, fmt.Errorf("unable to extract package name and version from nuget file path %s", nugetFilePath)
		}
	}

	// if we don't have a package reference to Pulumi SDK from nuget
	// we need to add it, unless we are referencing a local Pulumi SDK project via a project reference
	if _, ok := packageReferences["Pulumi"]; !ok {
		referencedLocalPulumiProject := false
		for _, projectReference := range projectReferences {
			if strings.HasSuffix(projectReference, "Pulumi.csproj") {
				referencedLocalPulumiProject = true
				break
			}
		}

		// only add a package reference to Pulumi if we're not referencing a local Pulumi project
		// which we usually do when testing schemas locally
		if !referencedLocalPulumiProject {
			packageReferences["Pulumi"] = "[3.71.0.0,4)"
		}
	}

	w := &bytes.Buffer{}
	err := csharpProjectFileTemplate.Execute(w, csharpProjectFileTemplateContext{
		XMLDoc:            fmt.Sprintf(`.\%s.xml`, assemblyName),
		Package:           pkg,
		PackageReferences: packageReferences,
		ProjectReferences: projectReferences,
		Version:           version,
		RestoreSources:    restoreSources,
	})
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

// emitLogo downloads an image and saves it as logo.png into the configured output directory.
func getLogo(pkg *schema.Package) ([]byte, error) {
	url := pkg.LogoURL
	if url == "" {
		// Default to a generic Pulumi logo from the parent repository.
		url = "https://raw.githubusercontent.com/pulumi/pulumi/dbc96206bec722b7791a22ff50e895ab7c0abdc0/sdk/dotnet/pulumi_logo_64x64.png"
	}

	// Get the data.
	//nolint:gosec
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(resp.Body)

	return io.ReadAll(resp.Body)
}

func computePropertyNames(props []*schema.Property, names map[*schema.Property]string) {
	for _, p := range props {
		if info, ok := p.Language["csharp"].(CSharpPropertyInfo); ok && info.Name != "" {
			names[p] = info.Name
		}
	}
}

// LanguageResource is derived from the schema and can be used by downstream codegen.
type LanguageResource struct {
	*schema.Resource

	Name    string // The resource name (e.g. Deployment)
	Package string // The package name (e.g. Apps.V1)
}

func generateModuleContextMap(tool string, pkg *schema.Package) (map[string]*modContext, *CSharpPackageInfo, error) {
	// Decode .NET-specific info for each package as we discover them.
	infos := map[*schema.Package]*CSharpPackageInfo{}
	getPackageInfo := func(p schema.PackageReference) *CSharpPackageInfo {
		def, err := p.Definition()
		contract.AssertNoErrorf(err, "error loading definition for package %v", p.Name())
		info, ok := infos[def]
		if !ok {
			err := def.ImportLanguages(map[string]schema.Language{"csharp": Importer})
			contract.AssertNoErrorf(err, "error importing csharp language info for package %q", p.Name())
			csharpInfo, _ := pkg.Language["csharp"].(CSharpPackageInfo)
			info = &csharpInfo
			infos[def] = info
		}
		return info
	}
	infos[pkg] = getPackageInfo(pkg.Reference())

	propertyNames := map[*schema.Property]string{}
	computePropertyNames(pkg.Config, propertyNames)
	computePropertyNames(pkg.Provider.InputProperties, propertyNames)
	for _, r := range pkg.Resources {
		if r.IsOverlay {
			// This resource code is generated by the provider, so no further action is required.
			continue
		}

		computePropertyNames(r.Properties, propertyNames)
		computePropertyNames(r.InputProperties, propertyNames)
		if r.StateInputs != nil {
			computePropertyNames(r.StateInputs.Properties, propertyNames)
		}
	}
	for _, f := range pkg.Functions {
		if f.IsOverlay {
			// This function code is generated by the provider, so no further action is required.
			continue
		}

		if f.Inputs != nil {
			computePropertyNames(f.Inputs.Properties, propertyNames)
		}
		if f.ReturnType != nil {
			if objectType, ok := f.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				computePropertyNames(objectType.Properties, propertyNames)
			}
		}
	}

	for _, t := range pkg.Types {
		if obj, ok := t.(*schema.ObjectType); ok {
			computePropertyNames(obj.Properties, propertyNames)
		}
	}

	// group resources, types, and functions into Go packages
	modules := map[string]*modContext{}
	details := map[*schema.ObjectType]*typeDetails{}

	var getMod func(modName string, p schema.PackageReference) *modContext
	getMod = func(modName string, p schema.PackageReference) *modContext {
		mod, ok := modules[modName]
		if !ok {
			info := getPackageInfo(p)
			ns := info.GetRootNamespace() + "." + namespaceName(info.Namespaces, pkg.Name)
			if modName != "" {
				ns += "." + namespaceName(info.Namespaces, modName)
			}
			mod = &modContext{
				pkg:                          p,
				mod:                          modName,
				tool:                         tool,
				namespaceName:                ns,
				namespaces:                   info.Namespaces,
				rootNamespace:                info.GetRootNamespace(),
				typeDetails:                  details,
				propertyNames:                propertyNames,
				compatibility:                info.Compatibility,
				dictionaryConstructors:       info.DictionaryConstructors,
				liftSingleValueMethodReturns: info.LiftSingleValueMethodReturns,
				parameterization:             pkg.Parameterization,
			}

			if modName != "" {
				parentName := path.Dir(modName)
				if parentName == "." {
					parentName = ""
				}
				parent := getMod(parentName, p)
				parent.children = append(parent.children, mod)
			}

			// Save the module only if it's for the current package.
			// This way, modules for external packages are not saved.
			if codegen.PkgEquals(p, pkg.Reference()) {
				modules[modName] = mod
			}
		}
		return mod
	}

	getModFromToken := func(token string, p schema.PackageReference) *modContext {
		return getMod(p.TokenToModule(token), p)
	}

	// Create the config module if necessary.
	if len(pkg.Config) > 0 {
		cfg := getMod("config", pkg.Reference())
		cfg.namespaceName = fmt.Sprintf("%s.%s", cfg.RootNamespace(), namespaceName(infos[pkg].Namespaces, pkg.Name))
	}

	// Find input and output types referenced by resources.
	scanResource := func(r *schema.Resource) {
		mod := getModFromToken(r.Token, pkg.Reference())
		mod.resources = append(mod.resources, r)
		visitObjectTypes(r.Properties, func(t *schema.ObjectType) {
			getModFromToken(t.Token, t.PackageReference).details(t).outputType = true
		})
		visitObjectTypes(r.InputProperties, func(t *schema.ObjectType) {
			getModFromToken(t.Token, t.PackageReference).details(t).inputType = true
		})
		if r.StateInputs != nil {
			visitObjectTypes(r.StateInputs.Properties, func(t *schema.ObjectType) {
				getModFromToken(t.Token, t.PackageReference).details(t).inputType = true
				getModFromToken(t.Token, t.PackageReference).details(t).stateType = true
			})
		}
	}

	scanResource(pkg.Provider)
	for _, r := range pkg.Resources {
		scanResource(r)
	}

	// Find input and output types referenced by functions.
	for _, f := range pkg.Functions {
		if f.IsOverlay {
			// This function code is generated by the provider, so no further action is required.
			continue
		}

		mod := getModFromToken(f.Token, pkg.Reference())
		if !f.IsMethod {
			mod.functions = append(mod.functions, f)
		}
		if f.Inputs != nil {
			visitObjectTypes(f.Inputs.Properties, func(t *schema.ObjectType) {
				details := getModFromToken(t.Token, t.PackageReference).details(t)
				details.inputType = true
				details.plainType = true
			})
			if f.NeedsOutputVersion() {
				visitObjectTypes(f.Inputs.InputShape.Properties, func(t *schema.ObjectType) {
					details := getModFromToken(t.Token, t.PackageReference).details(t)
					details.inputType = true
					details.usedInFunctionOutputVersionInputs = true
				})
			}
		}
		if f.ReturnType != nil {
			// special case where the return type is defined inline with the function
			if objectType, ok := f.ReturnType.(*schema.ObjectType); ok && f.InlineObjectAsReturnType {
				visitObjectTypes(objectType.Properties, func(t *schema.ObjectType) {
					details := getModFromToken(t.Token, t.PackageReference).details(t)
					details.outputType = true
					details.plainType = true
				})
			} else {
				// otherwise, the return type is a reference to a type defined elsewhere
				codegen.VisitType(f.ReturnType, func(schemaType schema.Type) {
					if t, ok := schemaType.(*schema.ObjectType); ok {
						details := getModFromToken(t.Token, t.PackageReference).details(t)
						details.outputType = true
						details.plainType = true
					}
				})
			}
		}
	}

	// Find nested types.
	for _, t := range pkg.Types {
		switch typ := t.(type) {
		case *schema.ObjectType:
			mod := getModFromToken(typ.Token, pkg.Reference())
			mod.types = append(mod.types, typ)
		case *schema.EnumType:
			if !typ.IsOverlay {
				mod := getModFromToken(typ.Token, pkg.Reference())
				mod.enums = append(mod.enums, typ)
			}
		default:
			continue
		}
	}

	return modules, infos[pkg], nil
}

// LanguageResources returns a map of resources that can be used by downstream codegen. The map
// key is the resource schema token.
func LanguageResources(tool string, pkg *schema.Package) (map[string]LanguageResource, error) {
	modules, info, err := generateModuleContextMap(tool, pkg)
	if err != nil {
		return nil, err
	}

	resources := map[string]LanguageResource{}
	for modName, mod := range modules {
		if modName == "" {
			continue
		}
		for _, r := range mod.resources {
			if r.IsOverlay {
				// This resource code is generated by the provider, so no further action is required.
				continue
			}

			lr := LanguageResource{
				Resource: r,
				Package:  namespaceName(info.Namespaces, modName),
				Name:     resourceName(r),
			}
			resources[r.Token] = lr
		}
	}

	return resources, nil
}

func GeneratePackage(
	tool string, pkg *schema.Package, extraFiles map[string][]byte, localDependencies map[string]string,
) (map[string][]byte, error) {
	modules, info, err := generateModuleContextMap(tool, pkg)
	if err != nil {
		return nil, err
	}

	assemblyName := info.GetRootNamespace() + "." + namespaceName(info.Namespaces, pkg.Name)

	// Generate each module.
	files := codegen.Fs{}
	for p, f := range extraFiles {
		files.Add(p, f)
	}
	for _, mod := range modules {
		if err := mod.gen(files); err != nil {
			return nil, err
		}
	}

	// Finally emit the package metadata.
	if err := genPackageMetadata(pkg,
		assemblyName,
		info.PackageReferences,
		info.ProjectReferences,
		files,
		localDependencies); err != nil {
		return nil, err
	}
	return files, nil
}
