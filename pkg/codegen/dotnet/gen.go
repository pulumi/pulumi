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
package dotnet

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v2/codegen"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

type stringSet map[string]struct{}

func (ss stringSet) add(s string) {
	ss[s] = struct{}{}
}

func (ss stringSet) has(s string) bool {
	_, ok := ss[s]
	return ok
}

type typeDetails struct {
	outputType   bool
	inputType    bool
	stateType    bool
	functionType bool
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
	if strings.HasPrefix(s, "$") {
		s = s[1:]
	}

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
	if _, ok := t.(*schema.EnumType); ok {
		return true
	}
	switch t {
	case schema.BoolType, schema.IntType, schema.NumberType:
		return true
	default:
		return false
	}
}

func namespaceName(namespaces map[string]string, name string) string {
	if ns, ok := namespaces[name]; ok {
		return ns
	}
	return Title(name)
}

type modContext struct {
	pkg                    *schema.Package
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
	return tokenToName(r.Token)
}

func tokenToFunctionName(tok string) string {
	return tokenToName(tok)
}

func (mod *modContext) isK8sCompatMode() bool {
	return mod.compatibility == "kubernetes20"
}

func (mod *modContext) tokenToNamespace(tok string, qualifier string) string {
	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", tok)

	pkg, nsName := "Pulumi."+namespaceName(mod.namespaces, components[0]), mod.pkg.TokenToModule(tok)

	if mod.isK8sCompatMode() {
		return pkg + ".Types." + qualifier + "." + namespaceName(mod.namespaces, nsName)
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

func (mod *modContext) typeString(t schema.Type, qualifier string, input, state, wrapInput, requireInitializers, optional bool) string {
	var typ string
	switch t := t.(type) {
	case *schema.EnumType:
		typ = mod.tokenToNamespace(t.Token, "")
		typ += "."
		typ += tokenToName(t.Token)
	case *schema.ArrayType:
		var listFmt string
		switch {
		case wrapInput:
			listFmt, optional = "InputList<%v>", false
		case requireInitializers:
			listFmt = "List<%v>"
		default:
			listFmt, optional = "ImmutableArray<%v>", false
		}

		wrapInput = false
		typ = fmt.Sprintf(listFmt, mod.typeString(t.ElementType, qualifier, input, state, false, false, false))
	case *schema.MapType:
		var mapFmt string
		switch {
		case wrapInput:
			mapFmt, optional = "InputMap<%v>", false
		case requireInitializers:
			mapFmt = "Dictionary<string, %v>"
		default:
			mapFmt = "ImmutableDictionary<string, %v>"
		}

		wrapInput = false
		typ = fmt.Sprintf(mapFmt, mod.typeString(t.ElementType, qualifier, input, state, false, false, false))
	case *schema.ObjectType:
		namingCtx := mod
		if t.Package != mod.pkg {
			// If object type belongs to another package, we apply naming convensions from that package,
			// including namespace naming and compatibility mode.
			var info CSharpPackageInfo
			if v, ok := t.Package.Language["csharp"].(CSharpPackageInfo); ok {
				info = v
			}
			namingCtx = &modContext{
				pkg:           t.Package,
				namespaces:    info.Namespaces,
				compatibility: info.Compatibility,
			}
		}
		typ = namingCtx.tokenToNamespace(t.Token, qualifier)
		if (typ == namingCtx.namespaceName && qualifier == "") || typ == namingCtx.namespaceName+"."+qualifier {
			typ = qualifier
		}
		if typ != "" {
			typ += "."
		}
		typ += tokenToName(t.Token)
		switch {
		case state:
			typ += "GetArgs"
		case input:
			typ += "Args"
		case mod.details(t).functionType:
			typ += "Result"
		}
	case *schema.ResourceType:
		if strings.HasPrefix(t.Token, "pulumi:providers:") {
			pkgName := strings.TrimPrefix(t.Token, "pulumi:providers:")
			typ = fmt.Sprintf("Pulumi.%s.Provider", namespaceName(mod.namespaces, pkgName))
		} else {
			typ = mod.tokenToNamespace(t.Token, "")
			if typ != "" {
				typ += "."
			}
			typ += tokenToName(t.Token)
		}
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return mod.typeString(t.UnderlyingType, qualifier, input, state, wrapInput, requireInitializers, optional)
		}

		typ = tokenToName(t.Token)
		if ns := mod.tokenToNamespace(t.Token, qualifier); ns != mod.namespaceName {
			typ = ns + "." + typ
		}
	case *schema.UnionType:
		unionT := "Union"
		if wrapInput {
			unionT = "InputUnion"
		}

		elementTypeSet := stringSet{}
		var elementTypes []string
		for _, e := range t.ElementTypes {
			// If this is an output and a "relaxed" enum, emit the type as the underlying primitive type rather than the union.
			// Eg. Output<string> rather than Output<Union<EnumType, string>>
			if typ, ok := e.(*schema.EnumType); ok && !input {
				return mod.typeString(typ.ElementType, qualifier, input, state, wrapInput, requireInitializers, optional)
			}

			et := mod.typeString(e, qualifier, input, state, false, false, false)
			if !elementTypeSet.has(et) {
				elementTypeSet.add(et)
				elementTypes = append(elementTypes, et)
			}
		}

		if len(elementTypes) == 1 {
			return mod.typeString(t.ElementTypes[0], qualifier, input, state, wrapInput, requireInitializers, optional)
		}

		for _, e := range elementTypes[:len(elementTypes)-1] {
			typ = fmt.Sprintf("%s%s<%s, ", typ, unionT, e)
		}
		last := elementTypes[len(elementTypes)-1]
		term := strings.Repeat(">", len(elementTypes)-1)

		wrapInput = false
		typ += last + term
	default:
		switch t {
		case schema.BoolType:
			typ = "bool"
		case schema.IntType:
			typ = "int"
		case schema.NumberType:
			typ = "double"
		case schema.StringType:
			typ = "string"
		case schema.ArchiveType:
			typ = "Archive"
		case schema.AssetType:
			typ = "AssetOrArchive"
		case schema.JSONType:
			if wrapInput {
				typ = "InputJson"
				wrapInput = false
			} else {
				typ = "System.Text.Json.JsonElement"
			}
		case schema.AnyType:
			typ = "object"
		}
	}

	if wrapInput {
		typ = fmt.Sprintf("Input<%s>", typ)
	}
	if optional {
		typ += "?"
	}
	return typ
}

var docCommentEscaper = strings.NewReplacer(
	`&`, "&amp;",
	`<`, "&lt;",
	`>`, "&gt;",
)

func printComment(w io.Writer, comment string, indent string) {
	lines := strings.Split(docCommentEscaper.Replace(comment), "\n")
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
	baseClass             string
	propertyTypeQualifier string
	properties            []*schema.Property
	wrapInput             bool
	state                 bool
}

func (pt *plainType) genInputProperty(w io.Writer, prop *schema.Property, indent string) {
	wireName := prop.Name
	propertyName := pt.mod.propertyName(prop)
	propertyType := pt.mod.typeString(prop.Type, pt.propertyTypeQualifier, true, pt.state, pt.wrapInput, false, !prop.IsRequired)

	// First generate the input attribute.
	attributeArgs := ""
	if prop.IsRequired {
		attributeArgs = ", required: true"
	}
	if pt.res != nil && pt.res.IsProvider {
		json := true
		if prop.Type == schema.StringType {
			json = false
		} else if t, ok := prop.Type.(*schema.TokenType); ok && t.UnderlyingType == schema.StringType {
			json = false
		}
		if json {
			attributeArgs += ", json: true"
		}
	}

	indent = strings.Repeat(indent, 2)

	// Next generate the input property itself. The way this is generated depends on the type of the property:
	// complex types like lists and maps need a backing field.
	switch prop.Type.(type) {
	case *schema.ArrayType, *schema.MapType:
		backingFieldName := "_" + prop.Name
		requireInitializers := !pt.wrapInput
		backingFieldType := pt.mod.typeString(prop.Type, pt.propertyTypeQualifier, true, pt.state, pt.wrapInput, requireInitializers, false)

		fmt.Fprintf(w, "%s[Input(\"%s\"%s)]\n", indent, wireName, attributeArgs)
		fmt.Fprintf(w, "%sprivate %s? %s;\n", indent, backingFieldType, backingFieldName)

		if prop.Comment != "" {
			fmt.Fprintf(w, "\n")
			printComment(w, prop.Comment, indent)
		}
		printObsoleteAttribute(w, prop.DeprecationMessage, indent)

		// Note that we use the backing field type--which is just the property type without any nullable annotation--to
		// ensure that the user does not see warnings when initializing these properties using object or collection
		// initializers.
		fmt.Fprintf(w, "%spublic %s %s\n", indent, backingFieldType, propertyName)
		fmt.Fprintf(w, "%s{\n", indent)
		fmt.Fprintf(w, "%s    get => %[2]s ?? (%[2]s = new %[3]s());\n", indent, backingFieldName, backingFieldType)
		fmt.Fprintf(w, "%s    set => %s = value;\n", indent, backingFieldName)
		fmt.Fprintf(w, "%s}\n", indent)
	default:
		initializer := ""
		if prop.IsRequired && (!isValueType(prop.Type) || pt.wrapInput) {
			initializer = " = null!;"
		}

		printComment(w, prop.Comment, indent)
		fmt.Fprintf(w, "%s[Input(\"%s\"%s)]\n", indent, wireName, attributeArgs)
		fmt.Fprintf(w, "%spublic %s %s { get; set; }%s\n", indent, propertyType, propertyName, initializer)
	}
}

// Set to avoid generating a class with the same name twice.
var generatedTypes = codegen.Set{}

func (pt *plainType) genInputType(w io.Writer, level int) error {
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
	printComment(w, pt.comment, indent)
	fmt.Fprintf(w, "%spublic %sclass %s : Pulumi.%s\n", indent, sealed, pt.name, pt.baseClass)
	fmt.Fprintf(w, "%s{\n", indent)

	// Declare each input property.
	for _, p := range pt.properties {
		pt.genInputProperty(w, p, indent)
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

	// Close the class.
	fmt.Fprintf(w, "%s}\n", indent)

	return nil
}

func (pt *plainType) genOutputType(w io.Writer, level int) {
	indent := strings.Repeat("    ", level)

	fmt.Fprintf(w, "\n")

	// Open the class and attribute it appropriately.
	fmt.Fprintf(w, "%s[OutputType]\n", indent)
	fmt.Fprintf(w, "%spublic sealed class %s\n", indent, pt.name)
	fmt.Fprintf(w, "%s{\n", indent)

	// Generate each output field.
	for _, prop := range pt.properties {
		fieldName := pt.mod.propertyName(prop)
		required := prop.IsRequired || pt.mod.isK8sCompatMode()
		fieldType := pt.mod.typeString(prop.Type, pt.propertyTypeQualifier, false, false, false, false, !required)
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
		required := prop.IsRequired || pt.mod.isK8sCompatMode()
		paramType := pt.mod.typeString(prop.Type, pt.propertyTypeQualifier, false, false, false, false, !required)

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

func (mod *modContext) getDefaultValue(dv *schema.DefaultValue, t schema.Type) (string, error) {
	var val string
	if dv.Value != nil {
		v, err := primitiveValue(dv.Value)
		if err != nil {
			return "", err
		}
		val = v
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

func genAlias(w io.Writer, alias *schema.Alias) {
	fmt.Fprintf(w, "new Pulumi.Alias { ")

	parts := []string{}
	if alias.Name != nil {
		parts = append(parts, fmt.Sprintf("Name = \"%v\"", *alias.Name))
	}
	if alias.Project != nil {
		parts = append(parts, fmt.Sprintf("Project = \"%v\"", *alias.Project))
	}
	if alias.Type != nil {
		parts = append(parts, fmt.Sprintf("Type = \"%v\"", *alias.Type))
	}

	for i, part := range parts {
		if i > 0 {
			fmt.Fprintf(w, ", ")
		}

		fmt.Fprintf(w, "%s", part)
	}

	fmt.Fprintf(w, "}")
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
		baseType = "Pulumi.ProviderResource"
	case mod.isK8sCompatMode():
		baseType = "KubernetesResource"
	case r.IsComponent:
		baseType = "Pulumi.ComponentResource"
		optionsType = "ComponentResourceOptions"
	default:
		baseType = "Pulumi.CustomResource"
	}

	if r.DeprecationMessage != "" {
		fmt.Fprintf(w, "    [Obsolete(@\"%s\")]\n", strings.Replace(r.DeprecationMessage, `"`, `""`, -1))
	}
	fmt.Fprintf(w, "    [%sResourceType(\"%s\")]\n", namespaceName(mod.namespaces, mod.pkg.Name), r.Token)
	fmt.Fprintf(w, "    public partial class %s : %s\n", className, baseType)
	fmt.Fprintf(w, "    {\n")

	var secretProps []string
	// Emit all output properties.
	for _, prop := range r.Properties {
		// Write the property attribute
		wireName := prop.Name
		propertyName := mod.propertyName(prop)
		required := prop.IsRequired || mod.isK8sCompatMode()
		propertyType := mod.typeString(prop.Type, "Outputs", false, false, false, false, !required)

		// Workaround the fact that provider inputs come back as strings.
		if r.IsProvider && !schema.IsPrimitiveType(prop.Type) {
			propertyType = "string"
			if !prop.IsRequired {
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
		allOptionalInputs = allOptionalInputs && !prop.IsRequired
		hasConstInputs = hasConstInputs || prop.ConstValue != nil
	}
	if allOptionalInputs || mod.isK8sCompatMode() {
		// If the number of required input properties was zero, we can make the args object optional.
		argsDefault = " = null"
		argsType += "?"
	}

	tok := r.Token
	if r.IsProvider {
		tok = mod.pkg.Name
	}

	argsOverride := fmt.Sprintf("args ?? new %sArgs()", className)
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
		fmt.Fprintf(w, "            : base(\"%s\", name, %s, MakeResourceOptions(options, \"\"))\n", tok, argsOverride)
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
			stateParam, stateRef = fmt.Sprintf("%sState? state = null, ", className), "state"
		}

		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "        private %s(string name, Input<string> id, %s%s? options = null)\n", className, stateParam, optionsType)
		fmt.Fprintf(w, "            : base(\"%s\", name, %s, MakeResourceOptions(options, id))\n", tok, stateRef)
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

	if len(r.Aliases) > 0 {
		fmt.Fprintf(w, "                Aliases =\n")
		fmt.Fprintf(w, "                {\n")
		for _, alias := range r.Aliases {
			fmt.Fprintf(w, "                    ")
			genAlias(w, alias)
			fmt.Fprintf(w, ",\n")
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
			stateParam, stateRef = fmt.Sprintf("%sState? state = null, ", className), "state, "
			fmt.Fprintf(w, "        /// <param name=\"state\">Any extra arguments used during the lookup.</param>\n")
		}

		fmt.Fprintf(w, "        /// <param name=\"options\">A bag of options that control this resource's behavior</param>\n")
		fmt.Fprintf(w, "        public static %s Get(string name, Input<string> id, %s%s? options = null)\n", className, stateParam, optionsType)
		fmt.Fprintf(w, "        {\n")
		fmt.Fprintf(w, "            return new %s(name, id, %soptions);\n", className, stateRef)
		fmt.Fprintf(w, "        }\n")
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
		wrapInput:             true,
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
			wrapInput:             true,
			state:                 true,
		}
		if err := state.genInputType(w, 1); err != nil {
			return err
		}
	}

	// Close the namespace.
	fmt.Fprintf(w, "}\n")

	return nil
}

func (mod *modContext) genFunction(w io.Writer, fun *schema.Function) error {
	className := tokenToFunctionName(fun.Token)

	fmt.Fprintf(w, "namespace %s\n", mod.tokenToNamespace(fun.Token, ""))
	fmt.Fprintf(w, "{\n")

	var typeParameter string
	if fun.Outputs != nil {
		typeParameter = fmt.Sprintf("<%sResult>", className)
	}

	var argsParamDef string
	argsParamRef := "InvokeArgs.Empty"
	if fun.Inputs != nil {
		allOptionalInputs := true
		for _, prop := range fun.Inputs.Properties {
			allOptionalInputs = allOptionalInputs && !prop.IsRequired
		}

		var argsDefault, sigil string
		if allOptionalInputs {
			// If the number of required input properties was zero, we can make the args object optional.
			argsDefault, sigil = " = null", "?"
		}

		argsParamDef = fmt.Sprintf("%sArgs%s args%s, ", className, sigil, argsDefault)
		argsParamRef = fmt.Sprintf("args ?? new %sArgs()", className)
	}

	if fun.DeprecationMessage != "" {
		fmt.Fprintf(w, "    [Obsolete(@\"%s\")]\n", strings.Replace(fun.DeprecationMessage, `"`, `""`, -1))
	}
	// Open the class we'll use for datasources.
	fmt.Fprintf(w, "    public static class %s\n", className)
	fmt.Fprintf(w, "    {\n")

	// Emit the doc comment, if any.
	printComment(w, fun.Comment, "        ")

	// Emit the datasource method.
	fmt.Fprintf(w, "        public static Task%s InvokeAsync(%sInvokeOptions? options = null)\n",
		typeParameter, argsParamDef)
	fmt.Fprintf(w, "            => Pulumi.Deployment.Instance.InvokeAsync%s(\"%s\", %s, options.WithVersion());\n",
		typeParameter, fun.Token, argsParamRef)

	// Close the class.
	fmt.Fprintf(w, "    }\n")

	// Emit the args and result types, if any.
	if fun.Inputs != nil {
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
	if fun.Outputs != nil {
		fmt.Fprintf(w, "\n")

		res := &plainType{
			mod:                   mod,
			name:                  className + "Result",
			propertyTypeQualifier: "Outputs",
			properties:            fun.Outputs.Properties,
		}
		res.genOutputType(w, 1)
	}

	// Close the namespace.
	fmt.Fprintf(w, "}\n")
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
		fmt.Fprintf(w, "%s[Obsolete(@\"%s\")]\n", indent, strings.Replace(deprecationMessage, `"`, `""`, -1))
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

		safeName, err := makeSafeEnumName(e.Name)
		if err != nil {
			return err
		}
		e.Name = safeName
	}

	// Print documentation comment
	printComment(w, enum.Comment, indent)

	underlyingType := mod.typeString(enum.ElementType, "", false, false, false, false, false)
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

func visitObjectTypesAcc(t schema.Type, visitor func(*schema.ObjectType), visited codegen.Set) {
	if visited.Has(t) {
		return
	}
	visited.Add(t)

	switch t := t.(type) {
	case *schema.ArrayType:
		visitObjectTypesAcc(t.ElementType, visitor, visited)
	case *schema.MapType:
		visitObjectTypesAcc(t.ElementType, visitor, visited)
	case *schema.ObjectType:
		for _, p := range t.Properties {
			visitObjectTypesAcc(p.Type, visitor, visited)
		}
		visitor(t)
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
			visitObjectTypesAcc(e, visitor, visited)
		}
	}
}

func visitObjectTypes(t schema.Type, visitor func(*schema.ObjectType)) {
	// Accumulator to avoid visiting the same node twice in case of recursive types.
	visited := codegen.Set{}
	visitObjectTypesAcc(t, visitor, visited)
}

func (mod *modContext) genType(w io.Writer, obj *schema.ObjectType, propertyTypeQualifier string, input, state bool, level int) error {
	name := tokenToName(obj.Token)
	switch {
	case state:
		name += "GetArgs"
	case input:
		name += "Args"
	case mod.details(obj).functionType:
		name += "Result"
	}

	pt := &plainType{
		mod:                   mod,
		name:                  name,
		comment:               obj.Comment,
		propertyTypeQualifier: propertyTypeQualifier,
		properties:            obj.Properties,
		state:                 state,
	}

	if input {
		pt.baseClass, pt.wrapInput = "ResourceArgs", true
		if mod.details(obj).functionType {
			pt.baseClass, pt.wrapInput = "InvokeArgs", false
		}
		return pt.genInputType(w, level)
	}

	pt.genOutputType(w, level)
	return nil
}

// pulumiImports is a slice of common imports that are used with the genHeader method.
var pulumiImports = []string{
	"System",
	"System.Collections.Generic",
	"System.Collections.Immutable",
	"System.Threading.Tasks",
	"Pulumi.Serialization",
}

func (mod *modContext) getTypeImports(t schema.Type, recurse bool, imports map[string]codegen.StringSet, seen codegen.Set) {
	if seen.Has(t) {
		return
	}
	seen.Add(t)

	switch t := t.(type) {
	case *schema.ArrayType:
		mod.getTypeImports(t.ElementType, recurse, imports, seen)
		return
	case *schema.MapType:
		mod.getTypeImports(t.ElementType, recurse, imports, seen)
		return
	case *schema.ObjectType:
		for _, p := range t.Properties {
			mod.getTypeImports(p.Type, recurse, imports, seen)
		}
		return
	case *schema.ResourceType:
		modName, name, modPath := mod.pkg.TokenToModule(t.Token), tokenToName(t.Token), ""
		if modName != mod.mod {
			mp, err := filepath.Rel(mod.mod, modName)
			contract.Assert(err == nil)
			if path.Base(mp) == "." {
				mp = path.Dir(mp)
			}
			modPath = filepath.ToSlash(mp)
		}
		if len(modPath) == 0 {
			return
		}
		if imports[modPath] == nil {
			imports[modPath] = codegen.NewStringSet()
		}
		imports[modPath].Add(name)
		return
	case *schema.TokenType:
		return
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
			mod.getTypeImports(e, recurse, imports, seen)
		}
		return
	default:
		return
	}
}

func (mod *modContext) getImports(member interface{}, imports map[string]codegen.StringSet) {
	seen := codegen.Set{}
	switch member := member.(type) {
	case *schema.ObjectType:
		for _, p := range member.Properties {
			mod.getTypeImports(p.Type, true, imports, seen)
		}
		return
	case *schema.ResourceType:
		mod.getTypeImports(member, true, imports, seen)
		return
	case *schema.Resource:
		for _, p := range member.Properties {
			mod.getTypeImports(p.Type, false, imports, seen)
		}
		for _, p := range member.InputProperties {
			mod.getTypeImports(p.Type, false, imports, seen)
		}
		return
	case *schema.Function:
		if member.Inputs != nil {
			mod.getTypeImports(member.Inputs, false, imports, seen)
		}
		if member.Outputs != nil {
			mod.getTypeImports(member.Outputs, false, imports, seen)
		}
		return
	case []*schema.Property:
		for _, p := range member {
			mod.getTypeImports(p.Type, false, imports, seen)
		}
		return
	default:
		return
	}
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
	propertyType := mod.typeString(
		schemaType, "Types", false, false, false /*wrapInputs*/, false /*requireInitializers*/, false)

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

	mod.genHeader(w, []string{"System.Collections.Immutable"})
	// Use the root namespace to avoid `Pulumi.Provider.Config.Config.VarName` usage.
	fmt.Fprintf(w, "namespace %s\n", mod.namespaceName)
	fmt.Fprintf(w, "{\n")

	// Open the config class.
	fmt.Fprintf(w, "    public static class Config\n")
	fmt.Fprintf(w, "    {\n")

	// Create a config bag for the variables to pull from.
	fmt.Fprintf(w, "        private static readonly Pulumi.Config __config = new Pulumi.Config(\"%v\");", mod.pkg.Name)
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

		printComment(w, p.Comment, "        ")
		fmt.Fprintf(w, "        public static %s %s { get; set; } = %s;\n", propertyType, propertyName, initializer)
		fmt.Fprintf(w, "\n")
	}

	// Emit any nested types.
	if len(mod.types) > 0 {
		fmt.Fprintf(w, "        public static class Types\n")
		fmt.Fprintf(w, "        {\n")

		for _, typ := range mod.types {
			fmt.Fprintf(w, "\n")

			// Open the class.
			fmt.Fprintf(w, "             public class %s\n", tokenToName(typ.Token))
			fmt.Fprintf(w, "             {\n")

			// Generate each output field.
			for _, prop := range typ.Properties {
				name := mod.propertyName(prop)
				typ := mod.typeString(prop.Type, "Types", false, false, false /*wrapInput*/, false, !prop.IsRequired)

				initializer := ""
				if !prop.IsRequired && !isValueType(prop.Type) && !isImmutableArrayType(prop.Type, false) {
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

type fs map[string][]byte

func (fs fs) add(path string, contents []byte) {
	_, has := fs[path]
	contract.Assertf(!has, "duplicate file: %s", path)
	fs[path] = contents
}

func (mod *modContext) genUtilities() (string, error) {
	// Strip any 'v' off of the version.
	w := &bytes.Buffer{}
	err := csharpUtilitiesTemplate.Execute(w, csharpUtilitiesTemplateContext{
		Name:      namespaceName(mod.namespaces, mod.pkg.Name),
		Namespace: mod.namespaceName,
		ClassName: "Utilities",
		Tool:      mod.tool,
	})
	if err != nil {
		return "", err
	}

	return w.String(), nil
}

func (mod *modContext) gen(fs fs) error {
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
		fs.add(p, []byte(contents))
	}

	// Ensure that the target module directory contains a README.md file.
	readme := mod.pkg.Description
	if readme != "" && readme[len(readme)-1] != '\n' {
		readme += "\n"
	}
	fs.add(filepath.Join(dir, "README.md"), []byte(readme))

	// Utilities, config
	switch mod.mod {
	case "":
		utilities, err := mod.genUtilities()
		if err != nil {
			return err
		}
		fs.add("Utilities.cs", []byte(utilities))
	case "config":
		if len(mod.pkg.Config) > 0 {
			config, err := mod.genConfig(mod.pkg.Config)
			if err != nil {
				return err
			}
			addFile("Config.cs", config)
			return nil
		}
	}

	// Resources
	for _, r := range mod.resources {
		imports := map[string]codegen.StringSet{}
		mod.getImports(r, imports)

		buffer := &bytes.Buffer{}
		importStrings := pulumiImports
		for _, i := range imports {
			importStrings = append(importStrings, i.SortedValues()...)
		}
		mod.genHeader(buffer, importStrings)

		if err := mod.genResource(buffer, r); err != nil {
			return err
		}

		addFile(resourceName(r)+".cs", buffer.String())
	}

	// Functions
	for _, f := range mod.functions {
		imports := map[string]codegen.StringSet{}
		mod.getImports(f, imports)

		buffer := &bytes.Buffer{}
		importStrings := pulumiImports
		for _, i := range imports {
			importStrings = append(importStrings, i.SortedValues()...)
		}
		mod.genHeader(buffer, importStrings)

		if err := mod.genFunction(buffer, f); err != nil {
			return err
		}

		addFile(tokenToName(f.Token)+".cs", buffer.String())
	}

	// Nested types
	for _, t := range mod.types {
		if mod.details(t).inputType {
			buffer := &bytes.Buffer{}
			mod.genHeader(buffer, pulumiImports)

			fmt.Fprintf(buffer, "namespace %s\n", mod.tokenToNamespace(t.Token, "Inputs"))
			fmt.Fprintf(buffer, "{\n")
			if err := mod.genType(buffer, t, "Inputs", true, false, 1); err != nil {
				return err
			}
			fmt.Fprintf(buffer, "}\n")

			addFile(path.Join("Inputs", tokenToName(t.Token)+"Args.cs"), buffer.String())
		}
		if mod.details(t).stateType {
			buffer := &bytes.Buffer{}
			mod.genHeader(buffer, pulumiImports)

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
			mod.genHeader(buffer, pulumiImports)

			fmt.Fprintf(buffer, "namespace %s\n", mod.tokenToNamespace(t.Token, "Outputs"))
			fmt.Fprintf(buffer, "{\n")
			if err := mod.genType(buffer, t, "Outputs", false, false, 1); err != nil {
				return err
			}
			fmt.Fprintf(buffer, "}\n")

			suffix := ""
			if mod.details(t).functionType {
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
func genPackageMetadata(pkg *schema.Package, assemblyName string, packageReferences map[string]string, files fs) error {
	projectFile, err := genProjectFile(pkg, assemblyName, packageReferences)
	if err != nil {
		return err
	}
	logo, err := getLogo(pkg)
	if err != nil {
		return err
	}

	files.add(assemblyName+".csproj", projectFile)
	files.add("logo.png", logo)
	return nil
}

// genProjectFile emits a C# project file into the configured output directory.
func genProjectFile(pkg *schema.Package, assemblyName string, packageReferences map[string]string) ([]byte, error) {
	w := &bytes.Buffer{}
	err := csharpProjectFileTemplate.Execute(w, csharpProjectFileTemplateContext{
		XMLDoc:            fmt.Sprintf(`.\%s.xml`, assemblyName),
		Package:           pkg,
		PackageReferences: packageReferences,
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
		url = "https://raw.githubusercontent.com/pulumi/pulumi/394c91d7f6ab7a4096f4454827690a460f665433/sdk/dotnet/pulumi_logo_64x64.png"
	}

	// Get the data.
	// nolint: gosec
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(resp.Body)

	return ioutil.ReadAll(resp.Body)
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
	var getPackageInfo = func(p *schema.Package) *CSharpPackageInfo {
		info, ok := infos[p]
		if !ok {
			if err := p.ImportLanguages(map[string]schema.Language{"csharp": Importer}); err != nil {
				panic(err)
			}
			csharpInfo, _ := pkg.Language["csharp"].(CSharpPackageInfo)
			info = &csharpInfo
			infos[p] = info
		}
		return info
	}
	infos[pkg] = getPackageInfo(pkg)

	propertyNames := map[*schema.Property]string{}
	computePropertyNames(pkg.Config, propertyNames)
	computePropertyNames(pkg.Provider.InputProperties, propertyNames)
	for _, r := range pkg.Resources {
		computePropertyNames(r.Properties, propertyNames)
		computePropertyNames(r.InputProperties, propertyNames)
		if r.StateInputs != nil {
			computePropertyNames(r.StateInputs.Properties, propertyNames)
		}
	}
	for _, f := range pkg.Functions {
		if f.Inputs != nil {
			computePropertyNames(f.Inputs.Properties, propertyNames)
		}
		if f.Outputs != nil {
			computePropertyNames(f.Outputs.Properties, propertyNames)
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

	var getMod func(modName string, p *schema.Package) *modContext
	getMod = func(modName string, p *schema.Package) *modContext {
		mod, ok := modules[modName]
		if !ok {
			info := getPackageInfo(p)
			ns := "Pulumi." + namespaceName(info.Namespaces, pkg.Name)
			if modName != "" {
				ns += "." + namespaceName(info.Namespaces, modName)
			}
			mod = &modContext{
				pkg:                    p,
				mod:                    modName,
				tool:                   tool,
				namespaceName:          ns,
				namespaces:             info.Namespaces,
				typeDetails:            details,
				propertyNames:          propertyNames,
				compatibility:          info.Compatibility,
				dictionaryConstructors: info.DictionaryConstructors,
			}

			if modName != "" {
				parentName := path.Dir(modName)
				if parentName == "." {
					parentName = ""
				}
				parent := getMod(parentName, p)
				parent.children = append(parent.children, mod)
			}

			modules[modName] = mod
		}
		return mod
	}

	getModFromToken := func(token string, p *schema.Package) *modContext {
		return getMod(p.TokenToModule(token), p)
	}

	// Create the config module if necessary.
	if len(pkg.Config) > 0 {
		cfg := getMod("config", pkg)
		cfg.namespaceName = "Pulumi." + namespaceName(infos[pkg].Namespaces, pkg.Name)
	}

	for _, v := range pkg.Config {
		visitObjectTypes(v.Type, func(t *schema.ObjectType) { getModFromToken(t.Token, pkg).details(t).outputType = true })
	}

	// Find input and output types referenced by resources.
	scanResource := func(r *schema.Resource) {
		mod := getModFromToken(r.Token, pkg)
		mod.resources = append(mod.resources, r)
		for _, p := range r.Properties {
			visitObjectTypes(p.Type, func(t *schema.ObjectType) {
				getModFromToken(t.Token, t.Package).details(t).outputType = true
			})
		}
		for _, p := range r.InputProperties {
			visitObjectTypes(p.Type, func(t *schema.ObjectType) {
				if r.IsProvider {
					getModFromToken(t.Token, t.Package).details(t).outputType = true
				}
				getModFromToken(t.Token, t.Package).details(t).inputType = true
			})
		}
		if r.StateInputs != nil {
			visitObjectTypes(r.StateInputs, func(t *schema.ObjectType) {
				getModFromToken(t.Token, t.Package).details(t).inputType = true
				getModFromToken(t.Token, t.Package).details(t).stateType = true
			})
		}
	}

	scanResource(pkg.Provider)
	for _, r := range pkg.Resources {
		scanResource(r)
	}

	// Find input and output types referenced by functions.
	for _, f := range pkg.Functions {
		mod := getModFromToken(f.Token, pkg)
		mod.functions = append(mod.functions, f)
		if f.Inputs != nil {
			visitObjectTypes(f.Inputs, func(t *schema.ObjectType) {
				details := getModFromToken(t.Token, t.Package).details(t)
				if !details.inputType {
					details.inputType = true
					details.functionType = true
				}
			})
		}
		if f.Outputs != nil {
			visitObjectTypes(f.Outputs, func(t *schema.ObjectType) {
				details := getModFromToken(t.Token, t.Package).details(t)
				if !details.outputType {
					details.outputType = true
					details.functionType = true
				}
			})
		}
	}

	// Find nested types.
	for _, t := range pkg.Types {
		switch typ := t.(type) {
		case *schema.ObjectType:
			mod := getModFromToken(typ.Token, pkg)
			mod.types = append(mod.types, typ)
		case *schema.EnumType:
			mod := getModFromToken(typ.Token, pkg)
			mod.enums = append(mod.enums, typ)
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
			lr := LanguageResource{
				Resource: r,
				Package:  namespaceName(info.Namespaces, modName),
				Name:     tokenToName(r.Token),
			}
			resources[r.Token] = lr
		}
	}

	return resources, nil
}

func GeneratePackage(tool string, pkg *schema.Package, extraFiles map[string][]byte) (map[string][]byte, error) {
	modules, info, err := generateModuleContextMap(tool, pkg)
	if err != nil {
		return nil, err
	}

	assemblyName := "Pulumi." + namespaceName(info.Namespaces, pkg.Name)

	// Generate each module.
	files := fs{}
	for p, f := range extraFiles {
		files.add(p, f)

	}
	for _, mod := range modules {
		if err := mod.gen(files); err != nil {
			return nil, err
		}
	}

	// Finally emit the package metadata.
	if err := genPackageMetadata(pkg, assemblyName, info.PackageReferences, files); err != nil {
		return nil, err
	}
	return files, nil
}
