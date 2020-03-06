// Copyright 2016-2018, Pulumi Corporation.
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
	"encoding/json"
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
	"github.com/pulumi/pulumi/pkg/codegen/schema"
	"github.com/pulumi/pulumi/pkg/util/contract"
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

func title(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	return string(append([]rune{unicode.ToUpper(runes[0])}, runes[1:]...))
}

func csharpIdentifier(s string) string {
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

func isValueType(t schema.Type) bool {
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
	return title(name)
}

type modContext struct {
	pkg           *schema.Package
	mod           string
	propertyNames map[*schema.Property]string
	types         []*schema.ObjectType
	resources     []*schema.Resource
	functions     []*schema.Function
	typeDetails   map[*schema.ObjectType]*typeDetails
	children      []*modContext
	tool          string
	namespaceName string
	namespaces    map[string]string
}

func (mod *modContext) propertyName(p *schema.Property) string {
	if n, ok := mod.propertyNames[p]; ok {
		return n
	}
	return title(p.Name)
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
	return title(components[2])
}

func resourceName(r *schema.Resource) string {
	if r.IsProvider {
		return "Provider"
	}
	return tokenToName(r.Token)
}

func (mod *modContext) tokenToNamespace(tok string) string {
	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", tok)

	pkg, nsName := "Pulumi."+title(components[0]), mod.pkg.TokenToModule(tok)
	if nsName == "" {
		return pkg
	}

	return pkg + "." + namespaceName(mod.namespaces, nsName)
}

func (mod *modContext) typeString(t schema.Type, qualifier string, input, state, wrapInput, requireInitializers, optional bool) string {
	var typ string
	switch t := t.(type) {
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
		typ = mod.tokenToNamespace(t.Token)
		if typ == mod.namespaceName {
			typ = qualifier
		} else if qualifier != "" {
			typ += "." + qualifier
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
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return mod.typeString(t.UnderlyingType, qualifier, input, state, wrapInput, requireInitializers, optional)
		}

		typ = tokenToName(t.Token)
		if ns := mod.tokenToNamespace(t.Token); ns != mod.namespaceName {
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
	if pt.res != nil && pt.res.IsProvider && prop.Type != schema.StringType {
		attributeArgs += ", json: true"
	}

	// Next generate the input property itself. The way this is generated depends on the type of the property:
	// complex types like lists and maps need a backing field.
	switch prop.Type.(type) {
	case *schema.ArrayType, *schema.MapType:
		backingFieldName := "_" + prop.Name
		requireInitializers := !pt.wrapInput
		backingFieldType := pt.mod.typeString(prop.Type, pt.propertyTypeQualifier, true, pt.state, pt.wrapInput, requireInitializers, false)

		fmt.Fprintf(w, "%s    [Input(\"%s\"%s)]\n", indent, wireName, attributeArgs)
		fmt.Fprintf(w, "%s    private %s? %s;\n", indent, backingFieldType, backingFieldName)

		if prop.Comment != "" {
			fmt.Fprintf(w, "\n")
			printComment(w, prop.Comment, indent+"    ")
		}
		if prop.DeprecationMessage != "" {
			fmt.Fprintf(w, "%s    [Obsolete(@\"%s\")]\n", indent, strings.Replace(prop.DeprecationMessage, `"`, `""`, -1))
		}

		// Note that we use the backing field type--which is just the property type without any nullable annotation--to
		// ensure that the user does not see warnings when initializing these properties using object or collection
		// initializers.
		fmt.Fprintf(w, "%s    public %s %s\n", indent, backingFieldType, propertyName)
		fmt.Fprintf(w, "%s    {\n", indent)
		fmt.Fprintf(w, "%s        get => %[2]s ?? (%[2]s = new %[3]s());\n", indent, backingFieldName, backingFieldType)
		fmt.Fprintf(w, "%s        set => %s = value;\n", indent, backingFieldName)
		fmt.Fprintf(w, "%s    }\n", indent)
	default:
		initializer := ""
		if prop.IsRequired && (!isValueType(prop.Type) || pt.wrapInput) {
			initializer = " = null!;"
		}

		printComment(w, prop.Comment, indent+"    ")
		fmt.Fprintf(w, "%s    [Input(\"%s\"%s)]\n", indent, wireName, attributeArgs)
		fmt.Fprintf(w, "%s    public %s %s { get; set; }%s\n", indent, propertyType, propertyName, initializer)
	}
}

func (pt *plainType) genInputType(w io.Writer, level int) error {
	indent := strings.Repeat("    ", level)

	fmt.Fprintf(w, "\n")

	// Open the class.
	printComment(w, pt.comment, indent)
	fmt.Fprintf(w, "%spublic sealed class %s : Pulumi.%s\n", indent, pt.name, pt.baseClass)
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
		fieldType := pt.mod.typeString(prop.Type, pt.propertyTypeQualifier, false, false, false, false, !prop.IsRequired)
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
		paramType := pt.mod.typeString(prop.Type, pt.propertyTypeQualifier, false, false, false, false, !prop.IsRequired)

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
	fmt.Fprintf(w, "new Alias { ")

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

	// Write the TypeDoc/JSDoc for the resource class
	printComment(w, r.Comment, "    ")

	// Open the class.
	className := name
	baseType := "Pulumi.CustomResource"
	if r.IsProvider {
		baseType = "Pulumi.ProviderResource"
	}
	fmt.Fprintf(w, "    public partial class %s : %s\n", className, baseType)
	fmt.Fprintf(w, "    {\n")

	// Emit all output properties.
	for _, prop := range r.Properties {
		// Write the property attribute
		wireName := prop.Name
		propertyName := mod.propertyName(prop)
		propertyType := mod.typeString(prop.Type, "Outputs", false, false, false, false, !prop.IsRequired)

		// Workaround the fact that provider inputs come back as strings.
		if r.IsProvider && !schema.IsPrimitiveType(prop.Type) {
			propertyType = "string"
			if !prop.IsRequired {
				propertyType += "?"
			}
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
	argsType := className + "Args"

	var argsDefault string
	allOptionalInputs := true
	for _, prop := range r.InputProperties {
		allOptionalInputs = allOptionalInputs && !prop.IsRequired
	}
	if allOptionalInputs {
		// If the number of required input properties was zero, we can make the args object optional.
		argsDefault = " = null"
		argsType += "?"
	}

	optionsType := "CustomResourceOptions"
	if r.IsProvider {
		optionsType = "ResourceOptions"
	}

	tok := r.Token
	if r.IsProvider {
		tok = mod.pkg.Name
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
	fmt.Fprintf(w, "            : base(\"%s\", name, args ?? ResourceArgs.Empty, MakeResourceOptions(options, \"\"))\n", tok)
	fmt.Fprintf(w, "        {\n")
	fmt.Fprintf(w, "        }\n")

	// Write a private constructor for the use of `Get`.
	if !r.IsProvider {
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

	// Write the method that will calculate the resource options.
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "        private static %[1]s MakeResourceOptions(%[1]s? options, Input<string>? id)\n", optionsType)
	fmt.Fprintf(w, "        {\n")
	fmt.Fprintf(w, "            var defaultOptions = new %s\n", optionsType)
	fmt.Fprintf(w, "            {\n")
	fmt.Fprintf(w, "                Version = Utilities.Version,")

	switch len(r.Aliases) {
	case 0:
		fmt.Fprintf(w, "\n")
	case 1:
		fmt.Fprintf(w, "                Aliases = { ")
		genAlias(w, r.Aliases[0])
		fmt.Fprintf(w, " },\n")
	default:
		fmt.Fprintf(w, "                Aliases =\n")
		fmt.Fprintf(w, "                {\n")
		for _, alias := range r.Aliases {
			fmt.Fprintf(w, "                    ")
			genAlias(w, alias)
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

	// Write the `Get` method for reading instances of this resource unless this is a provider resource.
	if !r.IsProvider {
		stateParam, stateRef := "", ""
		if r.StateInputs != nil {
			stateParam, stateRef = fmt.Sprintf("%sState? state = null, ", className), "state, "
		}

		fmt.Fprintf(w, "        /// <summary>\n")
		fmt.Fprintf(w, "        /// Get an existing %s resource's state with the given name, ID, and optional extra\n", className)
		fmt.Fprintf(w, "        /// properties used to qualify the lookup.\n")
		fmt.Fprintf(w, "        /// </summary>\n")
		fmt.Fprintf(w, "        ///\n")
		fmt.Fprintf(w, "        /// <param name=\"name\">The unique name of the resulting resource.</param>\n")
		fmt.Fprintf(w, "        /// <param name=\"id\">The unique provider ID of the resource to lookup.</param>\n")
		fmt.Fprintf(w, "        /// <param name=\"state\">Any extra arguments used during the lookup.</param>\n")
		fmt.Fprintf(w, "        /// <param name=\"options\">A bag of options that control this resource's behavior</param>\n")
		fmt.Fprintf(w, "        public static %s Get(string name, Input<string> id, %s%s? options = null)\n", className, stateParam, optionsType)
		fmt.Fprintf(w, "        {\n")
		fmt.Fprintf(w, "            return new %s(name, id, %soptions);\n", className, stateRef)
		fmt.Fprintf(w, "        }\n")
	}

	// Close the class.
	fmt.Fprintf(w, "    }\n")

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
	methodName := tokenToName(fun.Token)

	fmt.Fprintf(w, "namespace %s\n", mod.tokenToNamespace(fun.Token))
	fmt.Fprintf(w, "{\n")

	// Open the partial class we'll use for datasources.
	// TODO(pdg): this needs a better name that is guaranteed to be unique.
	fmt.Fprintf(w, "    public static partial class Invokes\n")
	fmt.Fprintf(w, "    {\n")

	var typeParameter string
	if fun.Outputs != nil {
		typeParameter = fmt.Sprintf("<%sResult>", methodName)
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

		argsParamDef = fmt.Sprintf("%sArgs%s args%s, ", methodName, sigil, argsDefault)
		argsParamRef = "args ?? InvokeArgs.Empty"
	}

	// Emit the doc comment, if any.
	printComment(w, fun.Comment, "        ")

	// Emit the datasource method.
	fmt.Fprintf(w, "        public static Task%s %s(%sInvokeOptions? options = null)\n", typeParameter, methodName, argsParamDef)
	fmt.Fprintf(w, "            => Pulumi.Deployment.Instance.InvokeAsync%s(\"%s\", %s, options.WithVersion());\n", typeParameter, fun.Token, argsParamRef)

	// Close the class.
	fmt.Fprintf(w, "    }\n")

	// Emit the args and result types, if any.
	if fun.Inputs != nil {
		fmt.Fprintf(w, "\n")

		args := &plainType{
			mod:                   mod,
			name:                  methodName + "Args",
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
			name:                  methodName + "Result",
			propertyTypeQualifier: "Outputs",
			properties:            fun.Outputs.Properties,
		}
		res.genOutputType(w, 1)
	}

	// Close the namespace.
	fmt.Fprintf(w, "}\n")
	return nil
}

func visitObjectTypes(t schema.Type, visitor func(*schema.ObjectType)) {
	switch t := t.(type) {
	case *schema.ArrayType:
		visitObjectTypes(t.ElementType, visitor)
	case *schema.MapType:
		visitObjectTypes(t.ElementType, visitor)
	case *schema.ObjectType:
		for _, p := range t.Properties {
			visitObjectTypes(p.Type, visitor)
		}
		visitor(t)
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
			visitObjectTypes(e, visitor)
		}
	}
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

func (mod *modContext) genPulumiHeader(w io.Writer) {
	mod.genHeader(w, []string{
		"System.Collections.Generic",
		"System.Collections.Immutable",
		"System.Threading.Tasks",
		"Pulumi.Serialization",
	})
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
		propertyType := mod.typeString(
			p.Type, "Types", false, false, false /*wrapInputs*/, false /*requireInitializers*/, false)

		var getFunc string
		nullableSigil := "?"
		switch p.Type {
		case schema.StringType:
			getFunc = "Get"
		case schema.BoolType:
			getFunc = "GetBoolean"
		case schema.IntType:
			getFunc = "GetInt32"
		case schema.NumberType:
			getFunc = "GetDouble"
		default:
			getFunc = "GetObject<" + propertyType + ">"
			if _, ok := p.Type.(*schema.ArrayType); ok {
				nullableSigil = ""
			}
		}

		propertyName := strings.Title(p.Name)

		initializer := fmt.Sprintf("__config.%s(\"%s\")", getFunc, p.Name)
		if p.DefaultValue != nil {
			dv, err := mod.getDefaultValue(p.DefaultValue, p.Type)
			if err != nil {
				return "", err
			}
			initializer += " ?? " + dv
		}

		printComment(w, p.Comment, "        ")
		fmt.Fprintf(w, "        public static %s%s %s { get; set; } = %s;\n", propertyType, nullableSigil, propertyName, initializer)
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
				if !prop.IsRequired {
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
		Namespace: mod.namespaceName,
		ClassName: "Utilities",
		Tool:      mod.tool,
		Version:   mod.pkg.Version.String(),
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
		}
	}

	// Resources
	for _, r := range mod.resources {
		buffer := &bytes.Buffer{}
		mod.genPulumiHeader(buffer)

		if err := mod.genResource(buffer, r); err != nil {
			return err
		}

		addFile(resourceName(r)+".cs", buffer.String())
	}

	// Functions
	for _, f := range mod.functions {
		buffer := &bytes.Buffer{}
		mod.genPulumiHeader(buffer)

		if err := mod.genFunction(buffer, f); err != nil {
			return err
		}

		addFile(tokenToName(f.Token)+".cs", buffer.String())
	}

	// Nested types
	for _, t := range mod.types {
		if mod.details(t).inputType {
			buffer := &bytes.Buffer{}
			mod.genPulumiHeader(buffer)

			fmt.Fprintf(buffer, "namespace %s.Inputs\n", mod.namespaceName)
			fmt.Fprintf(buffer, "{\n")
			if err := mod.genType(buffer, t, "Inputs", true, false, 1); err != nil {
				return err
			}
			fmt.Fprintf(buffer, "}\n")

			addFile(path.Join("Inputs", tokenToName(t.Token)+"Args.cs"), buffer.String())
		}
		if mod.details(t).stateType {
			buffer := &bytes.Buffer{}
			mod.genPulumiHeader(buffer)

			fmt.Fprintf(buffer, "namespace %s.Inputs\n", mod.namespaceName)
			fmt.Fprintf(buffer, "{\n")
			if err := mod.genType(buffer, t, "Inputs", true, true, 1); err != nil {
				return err
			}
			fmt.Fprintf(buffer, "}\n")

			addFile(path.Join("Inputs", tokenToName(t.Token)+"GetArgs.cs"), buffer.String())
		}
		if mod.details(t).outputType {
			buffer := &bytes.Buffer{}
			mod.genPulumiHeader(buffer)

			fmt.Fprintf(buffer, "namespace %s.Outputs\n", mod.namespaceName)
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

type csharpPropertyInfo struct {
	Name string `json:"name,omitempty"`
}

func computePropertyNames(props []*schema.Property, names map[*schema.Property]string) error {
	for _, p := range props {
		if csharp, ok := p.Language["csharp"]; ok {
			var info csharpPropertyInfo
			if err := json.Unmarshal([]byte(csharp), &info); err != nil {
				return err
			}
			if info.Name != "" {
				names[p] = info.Name
			}
		}
	}
	return nil
}

type csharpPackageInfo struct {
	PackageReferences map[string]string `json:"packageReferences,omitempty"`
	Namespaces        map[string]string `json:"namespaces,omitempty"`
}

func GeneratePackage(tool string, pkg *schema.Package, extraFiles map[string][]byte) (map[string][]byte, error) {
	// Decode csharp-specific info
	var info csharpPackageInfo
	if csharp, ok := pkg.Language["csharp"]; ok {
		if err := json.Unmarshal([]byte(csharp), &info); err != nil {
			return nil, errors.Wrap(err, "decoding csharp package info")
		}
	}

	propertyNames := map[*schema.Property]string{}
	for _, r := range pkg.Resources {
		if err := computePropertyNames(r.Properties, propertyNames); err != nil {
			return nil, err
		}
		if err := computePropertyNames(r.InputProperties, propertyNames); err != nil {
			return nil, err
		}
		if r.StateInputs != nil {
			if err := computePropertyNames(r.StateInputs.Properties, propertyNames); err != nil {
				return nil, err
			}
		}
	}
	for _, f := range pkg.Functions {
		if f.Inputs != nil {
			if err := computePropertyNames(f.Inputs.Properties, propertyNames); err != nil {
				return nil, err
			}
		}
		if f.Outputs != nil {
			if err := computePropertyNames(f.Outputs.Properties, propertyNames); err != nil {
				return nil, err
			}
		}
	}
	for _, t := range pkg.Types {
		if obj, ok := t.(*schema.ObjectType); ok {
			if err := computePropertyNames(obj.Properties, propertyNames); err != nil {
				return nil, err
			}
		}
	}

	// group resources, types, and functions into Go packages
	modules := map[string]*modContext{}
	details := map[*schema.ObjectType]*typeDetails{}

	assemblyName := "Pulumi." + namespaceName(info.Namespaces, pkg.Name)

	var getMod func(token string) *modContext
	getMod = func(token string) *modContext {
		modName := pkg.TokenToModule(token)
		mod, ok := modules[modName]
		if !ok {
			ns := assemblyName
			if modName != "" {
				ns += "." + namespaceName(info.Namespaces, modName)
			}
			mod = &modContext{
				pkg:           pkg,
				mod:           modName,
				tool:          tool,
				namespaceName: ns,
				namespaces:    info.Namespaces,
				typeDetails:   details,
				propertyNames: propertyNames,
			}

			if modName != "" {
				parentName := path.Dir(modName)
				if parentName == "." || parentName == "" {
					parentName = ":index:"
				}
				parent := getMod(parentName)
				parent.children = append(parent.children, mod)
			}

			modules[modName] = mod
		}
		return mod
	}

	// Create the config module if necessary.
	if len(pkg.Config) > 0 {
		cfg := getMod(":config/config:")
		cfg.namespaceName = assemblyName
	}

	for _, v := range pkg.Config {
		visitObjectTypes(v.Type, func(t *schema.ObjectType) { getMod(t.Token).details(t).outputType = true })
	}

	// Find input and output types referenced by resources.
	scanResource := func(r *schema.Resource) {
		mod := getMod(r.Token)
		mod.resources = append(mod.resources, r)
		for _, p := range r.Properties {
			visitObjectTypes(p.Type, func(t *schema.ObjectType) { getMod(t.Token).details(t).outputType = true })
		}
		for _, p := range r.InputProperties {
			visitObjectTypes(p.Type, func(t *schema.ObjectType) {
				if r.IsProvider {
					getMod(t.Token).details(t).outputType = true
				}
				getMod(t.Token).details(t).inputType = true
			})
		}
		if r.StateInputs != nil {
			visitObjectTypes(r.StateInputs, func(t *schema.ObjectType) {
				getMod(t.Token).details(t).inputType = true
				getMod(t.Token).details(t).stateType = true
			})
		}
	}

	scanResource(pkg.Provider)
	for _, r := range pkg.Resources {
		scanResource(r)
	}

	// Find input and output types referenced by functions.
	for _, f := range pkg.Functions {
		mod := getMod(f.Token)
		mod.functions = append(mod.functions, f)
		if f.Inputs != nil {
			visitObjectTypes(f.Inputs, func(t *schema.ObjectType) {
				getMod(t.Token).details(t).inputType = true
				getMod(t.Token).details(t).functionType = true
			})
		}
		if f.Outputs != nil {
			visitObjectTypes(f.Outputs, func(t *schema.ObjectType) {
				getMod(t.Token).details(t).outputType = true
				getMod(t.Token).details(t).functionType = true
			})
		}
	}

	// Find nested types.
	for _, t := range pkg.Types {
		if obj, ok := t.(*schema.ObjectType); ok {
			mod := getMod(obj.Token)
			mod.types = append(mod.types, obj)
		}
	}

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

	// Finally emit the package metadata (NPM, TypeScript, and so on).
	if err := genPackageMetadata(pkg, assemblyName, info.PackageReferences, files); err != nil {
		return nil, err
	}
	return files, nil
}
