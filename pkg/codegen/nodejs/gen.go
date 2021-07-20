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
package nodejs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
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
	outputType bool
	inputType  bool
}

func title(s string) string {
	if s == "" {
		return ""
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

type modContext struct {
	pkg              *schema.Package
	mod              string
	types            []*schema.ObjectType
	enums            []*schema.EnumType
	resources        []*schema.Resource
	functions        []*schema.Function
	typeDetails      map[*schema.ObjectType]*typeDetails
	children         []*modContext
	extraSourceFiles []string
	tool             string

	// Name overrides set in NodeJSInfo
	modToPkg                map[string]string // Module name -> package name
	compatibility           string            // Toggle compatibility mode for a specified target.
	disableUnionOutputTypes bool              // Disable unions in output types.
}

func (mod *modContext) String() string {
	return mod.mod
}

func (mod *modContext) details(t *schema.ObjectType) *typeDetails {
	details, ok := mod.typeDetails[t]
	if !ok {
		details = &typeDetails{}
		if mod.typeDetails == nil {
			mod.typeDetails = map[*schema.ObjectType]*typeDetails{}
		}
		mod.typeDetails[t] = details
	}
	return details
}

func (mod *modContext) tokenToModName(tok string) string {
	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", tok)

	modName := mod.pkg.TokenToModule(tok)
	if override, ok := mod.modToPkg[modName]; ok {
		modName = override
	}

	if modName != "" {
		modName = strings.Replace(modName, "/", ".", -1) + "."
	}

	return modName
}

func (mod *modContext) namingContext(pkg *schema.Package) (namingCtx *modContext, pkgName string, external bool) {
	namingCtx = mod
	if pkg != nil && pkg != mod.pkg {
		external = true
		pkgName = pkg.Name + "."

		var info NodePackageInfo
		contract.AssertNoError(pkg.ImportLanguages(map[string]schema.Language{"nodejs": Importer}))
		if v, ok := pkg.Language["nodejs"].(NodePackageInfo); ok {
			info = v
		}
		namingCtx = &modContext{
			pkg:           pkg,
			modToPkg:      info.ModuleToPackage,
			compatibility: info.Compatibility,
		}
	}
	return
}

func (mod *modContext) objectType(pkg *schema.Package, tok string, input, args, enum bool) string {
	root := "outputs."
	if input {
		root = "inputs."
	}

	namingCtx, pkgName, external := mod.namingContext(pkg)
	if external {
		pkgName = fmt.Sprintf("pulumi%s", title(pkgName))
		root = "types.output."
		if input {
			root = "types.input."
		}
	}

	modName, name := namingCtx.tokenToModName(tok), tokenToName(tok)

	if enum {
		return "enums." + modName + title(name)
	}

	if args && mod.compatibility != tfbridge20 && mod.compatibility != kubernetes20 {
		name += "Args"
	}
	return pkgName + root + modName + title(name)
}

func (mod *modContext) resourceType(r *schema.ResourceType) string {
	if strings.HasPrefix(r.Token, "pulumi:providers:") {
		pkgName := strings.TrimPrefix(r.Token, "pulumi:providers:")
		if pkgName != mod.pkg.Name {
			pkgName = fmt.Sprintf("pulumi%s", title(pkgName))
		}

		return fmt.Sprintf("%s.Provider", pkgName)
	}

	pkg := mod.pkg
	if r.Resource != nil {
		pkg = r.Resource.Package
	}
	namingCtx, pkgName, external := mod.namingContext(pkg)
	if external {
		pkgName = fmt.Sprintf("pulumi%s", title(pkgName))
	}

	modName, name := namingCtx.tokenToModName(r.Token), tokenToName(r.Token)

	return pkgName + modName + title(name)
}

func tokenToName(tok string) string {
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

func (mod *modContext) resourceFileName(r *schema.Resource) string {
	fileName := camel(resourceName(r)) + ".ts"
	if mod.isReservedSourceFileName(fileName) {
		fileName = camel(resourceName(r)) + "_.ts"
	}
	return fileName
}

func tokenToFunctionName(tok string) string {
	return camel(tokenToName(tok))
}

func (mod *modContext) typeString(t schema.Type, input bool, constValue interface{}) string {
	switch t := t.(type) {
	case *schema.OptionalType:
		return mod.typeString(t.ElementType, input, constValue) + " | undefined"
	case *schema.InputType:
		typ := mod.typeString(codegen.SimplifyInputUnion(t.ElementType), input, constValue)
		if typ == "any" {
			return typ
		}
		return fmt.Sprintf("pulumi.Input<%s>", typ)
	case *schema.EnumType:
		return mod.objectType(nil, t.Token, input, false, true)
	case *schema.ArrayType:
		return mod.typeString(t.ElementType, input, constValue) + "[]"
	case *schema.MapType:
		return fmt.Sprintf("{[key: string]: %v}", mod.typeString(t.ElementType, input, constValue))
	case *schema.ObjectType:
		return mod.objectType(t.Package, t.Token, input, t.IsInputShape(), false)
	case *schema.ResourceType:
		return mod.resourceType(t)
	case *schema.TokenType:
		return tokenToName(t.Token)
	case *schema.UnionType:
		if !input && mod.disableUnionOutputTypes {
			if t.DefaultType != nil {
				return mod.typeString(t.DefaultType, input, constValue)
			}
			return "any"
		}

		elements := make([]string, len(t.ElementTypes))
		for i, e := range t.ElementTypes {
			elements[i] = mod.typeString(e, input, constValue)
		}
		return strings.Join(elements, " | ")
	default:
		switch t {
		case schema.BoolType:
			return "boolean"
		case schema.IntType, schema.NumberType:
			return "number"
		case schema.StringType:
			if constValue != nil {
				return fmt.Sprintf("%q", constValue.(string))
			}
			return "string"
		case schema.ArchiveType:
			return "pulumi.asset.Archive"
		case schema.AssetType:
			return "pulumi.asset.Asset | pulumi.asset.Archive"
		case schema.JSONType:
			fallthrough
		case schema.AnyType:
			return "any"
		}
	}

	panic(fmt.Errorf("unexpected type %T", t))
}

func isStringType(t schema.Type) bool {
	t = codegen.UnwrapType(t)

	for tt, ok := t.(*schema.TokenType); ok; tt, ok = t.(*schema.TokenType) {
		t = tt.UnderlyingType
	}

	return t == schema.StringType
}

func sanitizeComment(str string) string {
	return strings.Replace(str, "*/", "*&#47;", -1)
}

func printComment(w io.Writer, comment, deprecationMessage, indent string) {
	if comment == "" && deprecationMessage == "" {
		return
	}

	lines := strings.Split(sanitizeComment(comment), "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	fmt.Fprintf(w, "%s/**\n", indent)
	for _, l := range lines {
		if l == "" {
			fmt.Fprintf(w, "%s *\n", indent)
		} else {
			fmt.Fprintf(w, "%s * %s\n", indent, l)
		}
	}
	if deprecationMessage != "" {
		if len(lines) > 0 {
			fmt.Fprintf(w, "%s *\n", indent)
		}
		fmt.Fprintf(w, "%s * @deprecated %s\n", indent, deprecationMessage)
	}
	fmt.Fprintf(w, "%s */\n", indent)
}

func (mod *modContext) genPlainType(w io.Writer, name, comment string, properties []*schema.Property, input, readonly bool, level int) {
	indent := strings.Repeat("    ", level)

	printComment(w, comment, "", indent)

	fmt.Fprintf(w, "%sexport interface %s {\n", indent, name)
	for _, p := range properties {
		printComment(w, p.Comment, p.DeprecationMessage, indent+"    ")

		prefix := ""
		if readonly {
			prefix = "readonly "
		}

		sigil, propertyType := "", p.Type
		if !p.IsRequired() {
			sigil, propertyType = "?", codegen.RequiredType(p)
		}

		typ := mod.typeString(propertyType, input, p.ConstValue)
		fmt.Fprintf(w, "%s    %s%s%s: %s;\n", indent, prefix, p.Name, sigil, typ)
	}
	fmt.Fprintf(w, "%s}\n", indent)
}

func tsPrimitiveValue(value interface{}) (string, error) {
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

func (mod *modContext) getConstValue(cv interface{}) (string, error) {
	if cv == nil {
		return "", nil
	}
	return tsPrimitiveValue(cv)
}

func (mod *modContext) getDefaultValue(dv *schema.DefaultValue, t schema.Type) (string, error) {
	var val string
	if dv.Value != nil {
		v, err := tsPrimitiveValue(dv.Value)
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
		case schema.IntType, schema.NumberType:
			getType = "Number"
		}

		envVars := fmt.Sprintf("%q", dv.Environment[0])
		for _, e := range dv.Environment[1:] {
			envVars += fmt.Sprintf(", %q", e)
		}

		cast := ""
		if t != schema.StringType {
			cast = "<any>"
		}

		getEnv := fmt.Sprintf("%sutilities.getEnv%s(%s)", cast, getType, envVars)
		if val != "" {
			val = fmt.Sprintf("(%s || %s)", getEnv, val)
		} else {
			val = getEnv
		}
	}

	return val, nil
}

func (mod *modContext) genAlias(w io.Writer, alias *schema.Alias) {
	fmt.Fprintf(w, "{ ")

	var parts []string
	if alias.Name != nil {
		parts = append(parts, fmt.Sprintf("name: \"%v\"", *alias.Name))
	}
	if alias.Project != nil {
		parts = append(parts, fmt.Sprintf("project: \"%v\"", *alias.Project))
	}
	if alias.Type != nil {
		parts = append(parts, fmt.Sprintf("type: \"%v\"", *alias.Type))
	}

	for i, part := range parts {
		if i > 0 {
			fmt.Fprintf(w, ", ")
		}

		fmt.Fprintf(w, "%s", part)
	}

	fmt.Fprintf(w, " }")
}

func (mod *modContext) genResource(w io.Writer, r *schema.Resource) error {
	// Create a resource module file into which all of this resource's types will go.
	name := resourceName(r)

	// Write the TypeDoc/JSDoc for the resource class
	printComment(w, codegen.FilterExamples(r.Comment, "typescript"), r.DeprecationMessage, "")

	var baseType, optionsType string
	switch {
	case r.IsComponent:
		baseType, optionsType = "ComponentResource", "ComponentResourceOptions"
	case r.IsProvider:
		baseType, optionsType = "ProviderResource", "ResourceOptions"
	default:
		baseType, optionsType = "CustomResource", "CustomResourceOptions"
	}

	// Begin defining the class.
	fmt.Fprintf(w, "export class %s extends pulumi.%s {\n", name, baseType)

	// Emit a static factory to read instances of this resource unless this is a provider resource or ComponentResource.
	stateType := name + "State"
	if !r.IsProvider && !r.IsComponent {
		fmt.Fprintf(w, "    /**\n")
		fmt.Fprintf(w, "     * Get an existing %s resource's state with the given name, ID, and optional extra\n", name)
		fmt.Fprintf(w, "     * properties used to qualify the lookup.\n")
		fmt.Fprintf(w, "     *\n")
		fmt.Fprintf(w, "     * @param name The _unique_ name of the resulting resource.\n")
		fmt.Fprintf(w, "     * @param id The _unique_ provider ID of the resource to lookup.\n")
		// TODO: Document id format: https://github.com/pulumi/pulumi/issues/4754
		if r.StateInputs != nil {
			fmt.Fprintf(w, "     * @param state Any extra arguments used during the lookup.\n")
		}
		fmt.Fprintf(w, "     * @param opts Optional settings to control the behavior of the CustomResource.\n")
		fmt.Fprintf(w, "     */\n")

		stateParam, stateRef := "", "undefined as any, "
		if r.StateInputs != nil {
			stateParam, stateRef = fmt.Sprintf("state?: %s, ", stateType), "<any>state, "
		}

		fmt.Fprintf(w, "    public static get(name: string, id: pulumi.Input<pulumi.ID>, %sopts?: pulumi.%s): %s {\n",
			stateParam, optionsType, name)
		if r.DeprecationMessage != "" && mod.compatibility != kubernetes20 {
			fmt.Fprintf(w, "        pulumi.log.warn(\"%s is deprecated: %s\")\n", name, r.DeprecationMessage)
		}
		fmt.Fprintf(w, "        return new %s(name, %s{ ...opts, id: id });\n", name, stateRef)
		fmt.Fprintf(w, "    }\n")
		fmt.Fprintf(w, "\n")
	}

	pulumiType := r.Token
	if r.IsProvider {
		pulumiType = mod.pkg.Name
	}

	fmt.Fprintf(w, "    /** @internal */\n")
	fmt.Fprintf(w, "    public static readonly __pulumiType = '%s';\n", pulumiType)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "    /**\n")
	fmt.Fprintf(w, "     * Returns true if the given object is an instance of %s.  This is designed to work even\n", name)
	fmt.Fprintf(w, "     * when multiple copies of the Pulumi SDK have been loaded into the same process.\n")
	fmt.Fprintf(w, "     */\n")
	fmt.Fprintf(w, "    public static isInstance(obj: any): obj is %s {\n", name)
	fmt.Fprintf(w, "        if (obj === undefined || obj === null) {\n")
	fmt.Fprintf(w, "            return false;\n")
	fmt.Fprintf(w, "        }\n")
	fmt.Fprintf(w, "        return obj['__pulumiType'] === %s.__pulumiType;\n", name)
	fmt.Fprintf(w, "    }\n")
	fmt.Fprintf(w, "\n")

	// Emit all properties (using their output types).
	// TODO[pulumi/pulumi#397]: represent sensitive types using a Secret<T> type.
	ins := codegen.NewStringSet()
	allOptionalInputs := true
	for _, prop := range r.InputProperties {
		ins.Add(prop.Name)
		allOptionalInputs = allOptionalInputs && !prop.IsRequired()
	}
	for _, prop := range r.Properties {
		printComment(w, prop.Comment, prop.DeprecationMessage, "    ")

		// Make a little comment in the code so it's easy to pick out output properties.
		var outcomment string
		if !ins.Has(prop.Name) {
			outcomment = "/*out*/ "
		}

		propertyType := prop.Type
		if mod.compatibility == kubernetes20 {
			propertyType = codegen.RequiredType(prop)
		}
		fmt.Fprintf(w, "    public %sreadonly %s!: pulumi.Output<%s>;\n", outcomment, prop.Name, mod.typeString(propertyType, false, prop.ConstValue))
	}
	fmt.Fprintf(w, "\n")

	// Now create a constructor that chains supercalls and stores into properties.
	fmt.Fprintf(w, "    /**\n")
	fmt.Fprintf(w, "     * Create a %s resource with the given unique name, arguments, and options.\n", name)
	fmt.Fprintf(w, "     *\n")
	fmt.Fprintf(w, "     * @param name The _unique_ name of the resource.\n")
	fmt.Fprintf(w, "     * @param args The arguments to use to populate this resource's properties.\n")
	fmt.Fprintf(w, "     * @param opts A bag of options that control this resource's behavior.\n")
	fmt.Fprintf(w, "     */\n")

	// k8s provider "get" methods don't require args, so make args optional.
	if mod.compatibility == kubernetes20 {
		allOptionalInputs = true
	}

	// Write out callable constructor: We only emit a single public constructor, even though we use a private signature
	// as well as part of the implementation of `.get`. This is complicated slightly by the fact that, if there is no
	// args type, we will emit a constructor lacking that parameter.
	var argsFlags string
	if allOptionalInputs {
		// If the number of required input properties was zero, we can make the args object optional.
		argsFlags = "?"
	}
	argsType := name + "Args"
	var trailingBrace string
	switch {
	case r.IsProvider, r.StateInputs == nil:
		trailingBrace = " {"
	default:
		trailingBrace = ""
	}

	if r.DeprecationMessage != "" {
		fmt.Fprintf(w, "    /** @deprecated %s */\n", r.DeprecationMessage)
	}
	fmt.Fprintf(w, "    constructor(name: string, args%s: %s, opts?: pulumi.%s)%s\n", argsFlags, argsType,
		optionsType, trailingBrace)

	genInputProps := func() error {
		for _, prop := range r.InputProperties {
			if prop.IsRequired() {
				fmt.Fprintf(w, "            if ((!args || args.%s === undefined) && !opts.urn) {\n", prop.Name)
				fmt.Fprintf(w, "                throw new Error(\"Missing required property '%s'\");\n", prop.Name)
				fmt.Fprintf(w, "            }\n")
			}
		}
		for _, prop := range r.InputProperties {
			var arg string
			if prop.Secret {
				arg = fmt.Sprintf("args?.%[1]s ? pulumi.secret(args.%[1]s) : undefined", prop.Name)
			} else {
				arg = fmt.Sprintf("args ? args.%[1]s : undefined", prop.Name)
			}

			prefix := "            "
			if prop.ConstValue != nil {
				cv, err := mod.getConstValue(prop.ConstValue)
				if err != nil {
					return err
				}
				arg = cv
			} else {
				if prop.DefaultValue != nil {
					dv, err := mod.getDefaultValue(prop.DefaultValue, codegen.UnwrapType(prop.Type))
					if err != nil {
						return err
					}

					arg = fmt.Sprintf("(%s) ?? %s", arg, dv)
				}

				// provider properties must be marshaled as JSON strings.
				if r.IsProvider && !isStringType(prop.Type) {
					arg = fmt.Sprintf("pulumi.output(%s).apply(JSON.stringify)", arg)
				}
			}
			fmt.Fprintf(w, "%sinputs[\"%s\"] = %s;\n", prefix, prop.Name, arg)
		}

		for _, prop := range r.Properties {
			prefix := "            "
			if !ins.Has(prop.Name) {
				fmt.Fprintf(w, "%sinputs[\"%s\"] = undefined /*out*/;\n", prefix, prop.Name)
			}
		}

		return nil
	}

	if !r.IsProvider {
		if r.StateInputs != nil {
			if r.DeprecationMessage != "" {
				fmt.Fprintf(w, "    /** @deprecated %s */\n", r.DeprecationMessage)
			}

			// Now write out a general purpose constructor implementation that can handle the public signature as well as the
			// signature to support construction via `.get`.  And then emit the body preamble which will pluck out the
			// conditional state into sensible variables using dynamic type tests.
			fmt.Fprintf(w, "    constructor(name: string, argsOrState?: %s | %s, opts?: pulumi.%s) {\n",
				argsType, stateType, optionsType)
		}
		if r.DeprecationMessage != "" && mod.compatibility != kubernetes20 {
			fmt.Fprintf(w, "        pulumi.log.warn(\"%s is deprecated: %s\")\n", name, r.DeprecationMessage)
		}
		fmt.Fprintf(w, "        let inputs: pulumi.Inputs = {};\n")
		fmt.Fprintf(w, "        opts = opts || {};\n")

		if r.StateInputs != nil {
			// The lookup case:
			fmt.Fprintf(w, "        if (opts.id) {\n")
			fmt.Fprintf(w, "            const state = argsOrState as %[1]s | undefined;\n", stateType)
			for _, prop := range r.StateInputs.Properties {
				fmt.Fprintf(w, "            inputs[\"%[1]s\"] = state ? state.%[1]s : undefined;\n", prop.Name)
			}
			// The creation case (with args):
			fmt.Fprintf(w, "        } else {\n")
			fmt.Fprintf(w, "            const args = argsOrState as %s | undefined;\n", argsType)
			err := genInputProps()
			if err != nil {
				return err
			}
		} else {
			// The creation case:
			fmt.Fprintf(w, "        if (!opts.id) {\n")
			err := genInputProps()
			if err != nil {
				return err
			}
			// The get case:
			fmt.Fprintf(w, "        } else {\n")
			for _, prop := range r.Properties {
				fmt.Fprintf(w, "            inputs[\"%[1]s\"] = undefined /*out*/;\n", prop.Name)
			}
		}
	} else {
		fmt.Fprintf(w, "        let inputs: pulumi.Inputs = {};\n")
		fmt.Fprintf(w, "        opts = opts || {};\n")
		fmt.Fprintf(w, "        {\n")
		err := genInputProps()
		if err != nil {
			return err
		}
	}
	var secretProps []string
	for _, prop := range r.Properties {
		if prop.Secret {
			secretProps = append(secretProps, prop.Name)
		}
	}
	fmt.Fprintf(w, "        }\n")

	// If the caller didn't request a specific version, supply one using the version of this library.
	fmt.Fprintf(w, "        if (!opts.version) {\n")
	fmt.Fprintf(w, "            opts = pulumi.mergeOptions(opts, { version: utilities.getVersion()});\n")
	fmt.Fprintf(w, "        }\n")

	// Now invoke the super constructor with the type, name, and a property map.
	if len(r.Aliases) > 0 {
		fmt.Fprintf(w, "        const aliasOpts = { aliases: [")
		for i, alias := range r.Aliases {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			mod.genAlias(w, alias)
		}
		fmt.Fprintf(w, "] };\n")
		fmt.Fprintf(w, "        opts = pulumi.mergeOptions(opts, aliasOpts);\n")
	}

	if len(secretProps) > 0 {
		fmt.Fprintf(w, `        const secretOpts = { additionalSecretOutputs: ["%s"] };`, strings.Join(secretProps, `", "`))
		fmt.Fprintf(w, "\n        opts = pulumi.mergeOptions(opts, secretOpts);\n")
	}

	// If it's a ComponentResource, set the remote option.
	if r.IsComponent {
		fmt.Fprintf(w, "        super(%s.__pulumiType, name, inputs, opts, true /*remote*/);\n", name)
	} else {
		fmt.Fprintf(w, "        super(%s.__pulumiType, name, inputs, opts);\n", name)
	}

	fmt.Fprintf(w, "    }\n")

	// Generate methods.
	genMethod := func(method *schema.Method) {
		methodName := camel(method.Name)
		fun := method.Function

		// Write the TypeDoc/JSDoc for the data source function.
		fmt.Fprint(w, "\n")
		printComment(w, codegen.FilterExamples(fun.Comment, "typescript"), fun.DeprecationMessage, "    ")

		// Now, emit the method signature.
		var args []*schema.Property
		var argsig string
		argsOptional := true
		if fun.Inputs != nil {
			// Filter out the __self__ argument from the inputs.
			args = make([]*schema.Property, 0, len(fun.Inputs.InputShape.Properties))
			for _, arg := range fun.Inputs.InputShape.Properties {
				if arg.Name == "__self__" {
					continue
				}
				if arg.IsRequired() {
					argsOptional = false
				}
				args = append(args, arg)
			}

			if len(args) > 0 {
				optFlag := ""
				if argsOptional {
					optFlag = "?"
				}
				argsig = fmt.Sprintf("args%s: %s.%sArgs", optFlag, name, title(method.Name))
			}
		}
		var retty string
		if fun.Outputs == nil {
			retty = "void"
		} else {
			retty = fmt.Sprintf("pulumi.Output<%s.%sResult>", name, title(method.Name))
		}
		fmt.Fprintf(w, "    %s(%s): %s {\n", methodName, argsig, retty)
		if fun.DeprecationMessage != "" {
			fmt.Fprintf(w, "        pulumi.log.warn(\"%s.%s is deprecated: %s\")\n", name, methodName,
				fun.DeprecationMessage)
		}

		// Zero initialize the args if empty and necessary.
		if len(args) > 0 && argsOptional {
			fmt.Fprintf(w, "        args = args || {};\n")
		}

		// Now simply call the runtime function with the arguments, returning the results.
		var ret string
		if fun.Outputs != nil {
			ret = "return "
		}
		fmt.Fprintf(w, "        %spulumi.runtime.call(\"%s\", {\n", ret, fun.Token)
		if fun.Inputs != nil {
			for _, p := range fun.Inputs.InputShape.Properties {
				// Pass the argument to the invocation.
				if p.Name == "__self__" {
					fmt.Fprintf(w, "            \"%s\": this,\n", p.Name)
				} else {
					fmt.Fprintf(w, "            \"%[1]s\": args.%[1]s,\n", p.Name)
				}
			}
		}
		fmt.Fprintf(w, "        }, this);\n")
		fmt.Fprintf(w, "    }\n")
	}
	for _, method := range r.Methods {
		genMethod(method)
	}

	// Finish the class.
	fmt.Fprintf(w, "}\n")

	// Emit the state type for get methods.
	if r.StateInputs != nil {
		fmt.Fprintf(w, "\n")
		mod.genPlainType(w, stateType, r.StateInputs.Comment, r.StateInputs.Properties, true, false, 0)
	}

	// Emit the argument type for construction.
	fmt.Fprintf(w, "\n")
	argsComment := fmt.Sprintf("The set of arguments for constructing a %s resource.", name)
	mod.genPlainType(w, argsType, argsComment, r.InputProperties, true, false, 0)

	// Emit any method types inside a namespace merged with the class, to represent types nested in the class.
	// https://www.typescriptlang.org/docs/handbook/declaration-merging.html#merging-namespaces-with-classes
	genMethodTypes := func(w io.Writer, method *schema.Method) {
		fun := method.Function
		methodName := title(method.Name)
		if fun.Inputs != nil {
			args := make([]*schema.Property, 0, len(fun.Inputs.InputShape.Properties))
			for _, arg := range fun.Inputs.InputShape.Properties {
				if arg.Name == "__self__" {
					continue
				}
				args = append(args, arg)
			}
			if len(args) > 0 {
				comment := fun.Inputs.Comment
				if comment == "" {
					comment = fmt.Sprintf("The set of arguments for the %s.%s method.", name, method.Name)
				}
				mod.genPlainType(w, methodName+"Args", comment, args, true, false, 1)
				fmt.Fprintf(w, "\n")
			}
		}
		if fun.Outputs != nil {
			comment := fun.Inputs.Comment
			if comment == "" {
				comment = fmt.Sprintf("The results of the %s.%s method.", name, method.Name)
			}
			mod.genPlainType(w, methodName+"Result", comment, fun.Outputs.Properties, false, true, 1)
			fmt.Fprintf(w, "\n")
		}
	}
	types := &bytes.Buffer{}
	for _, method := range r.Methods {
		genMethodTypes(types, method)
	}
	typesString := types.String()
	if typesString != "" {
		fmt.Fprintf(w, "\nexport namespace %s {\n", name)
		fmt.Fprintf(w, typesString)
		fmt.Fprintf(w, "}\n")
	}
	return nil
}

func (mod *modContext) genFunction(w io.Writer, fun *schema.Function) {
	name := tokenToFunctionName(fun.Token)

	// Write the TypeDoc/JSDoc for the data source function.
	printComment(w, codegen.FilterExamples(fun.Comment, "typescript"), "", "")

	if fun.DeprecationMessage != "" {
		fmt.Fprintf(w, "/** @deprecated %s */\n", fun.DeprecationMessage)
	}

	// Now, emit the function signature.
	var argsig string
	argsOptional := true
	if fun.Inputs != nil {
		for _, p := range fun.Inputs.Properties {
			if p.IsRequired() {
				argsOptional = false
				break
			}
		}

		optFlag := ""
		if argsOptional {
			optFlag = "?"
		}
		argsig = fmt.Sprintf("args%s: %sArgs, ", optFlag, title(name))
	}
	var retty string
	if fun.Outputs == nil {
		retty = "void"
	} else {
		retty = title(name) + "Result"
	}
	fmt.Fprintf(w, "export function %s(%sopts?: pulumi.InvokeOptions): Promise<%s> {\n", name, argsig, retty)
	if fun.DeprecationMessage != "" && mod.compatibility != kubernetes20 {
		fmt.Fprintf(w, "    pulumi.log.warn(\"%s is deprecated: %s\")\n", name, fun.DeprecationMessage)
	}

	// Zero initialize the args if empty and necessary.
	if fun.Inputs != nil && argsOptional {
		fmt.Fprintf(w, "    args = args || {};\n")
	}

	// If the caller didn't request a specific version, supply one using the version of this library.
	fmt.Fprintf(w, "    if (!opts) {\n")
	fmt.Fprintf(w, "        opts = {}\n")
	fmt.Fprintf(w, "    }\n")
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "    if (!opts.version) {\n")
	fmt.Fprintf(w, "        opts.version = utilities.getVersion();\n")
	fmt.Fprintf(w, "    }\n")

	// Now simply invoke the runtime function with the arguments, returning the results.
	fmt.Fprintf(w, "    return pulumi.runtime.invoke(\"%s\", {\n", fun.Token)
	if fun.Inputs != nil {
		for _, p := range fun.Inputs.Properties {
			// Pass the argument to the invocation.
			fmt.Fprintf(w, "        \"%[1]s\": args.%[1]s,\n", p.Name)
		}
	}
	fmt.Fprintf(w, "    }, opts);\n")
	fmt.Fprintf(w, "}\n")

	// If there are argument and/or return types, emit them.
	if fun.Inputs != nil {
		fmt.Fprintf(w, "\n")
		mod.genPlainType(w, title(name)+"Args", fun.Inputs.Comment, fun.Inputs.Properties, true, false, 0)
	}
	if fun.Outputs != nil {
		fmt.Fprintf(w, "\n")
		mod.genPlainType(w, title(name)+"Result", fun.Outputs.Comment, fun.Outputs.Properties, false, true, 0)
	}
}

func visitObjectTypes(properties []*schema.Property, visitor func(*schema.ObjectType)) {
	codegen.VisitTypeClosure(properties, func(t schema.Type) {
		if o, ok := t.(*schema.ObjectType); ok {
			visitor(o)
		}
	})
}

func (mod *modContext) genType(w io.Writer, obj *schema.ObjectType, input bool, level int) {
	properties := obj.Properties
	info, hasInfo := obj.Language["nodejs"]
	if hasInfo {
		var requiredProperties []string
		if input {
			requiredProperties = info.(NodeObjectInfo).RequiredInputs
		} else {
			requiredProperties = info.(NodeObjectInfo).RequiredOutputs
		}

		if requiredProperties != nil {
			required := codegen.StringSet{}
			for _, name := range requiredProperties {
				required.Add(name)
			}

			properties = make([]*schema.Property, len(obj.Properties))
			for i, p := range obj.Properties {
				copy := *p
				if required.Has(p.Name) {
					copy.Type = codegen.RequiredType(&copy)
				} else {
					copy.Type = codegen.OptionalType(&copy)
				}
				properties[i] = &copy
			}
		}
	}

	name := tokenToName(obj.Token)
	if obj.IsInputShape() && mod.compatibility != tfbridge20 && mod.compatibility != kubernetes20 {
		name += "Args"
	}

	mod.genPlainType(w, name, obj.Comment, properties, input, false, level)
}

func (mod *modContext) getTypeImports(t schema.Type, recurse bool, externalImports codegen.StringSet, imports map[string]codegen.StringSet, seen codegen.Set) bool {
	return mod.getTypeImportsForResource(t, recurse, externalImports, imports, seen, nil)
}

func (mod *modContext) getTypeImportsForResource(t schema.Type, recurse bool, externalImports codegen.StringSet, imports map[string]codegen.StringSet, seen codegen.Set, res *schema.Resource) bool {
	if seen.Has(t) {
		return false
	}
	seen.Add(t)

	resourceOrTokenImport := func(tok string) bool {
		modName, name, modPath := mod.pkg.TokenToModule(tok), tokenToName(tok), "./index"
		if override, ok := mod.modToPkg[modName]; ok {
			modName = override
		}
		if modName != mod.mod {
			mp, err := filepath.Rel(mod.mod, modName)
			contract.Assert(err == nil)
			if path.Base(mp) == "." {
				mp = path.Dir(mp)
			}
			modPath = filepath.ToSlash(mp)
		}
		if imports[modPath] == nil {
			imports[modPath] = codegen.NewStringSet()
		}
		imports[modPath].Add(name)
		return false
	}

	var nodePackageInfo NodePackageInfo
	if languageInfo, hasLanguageInfo := mod.pkg.Language["nodejs"]; hasLanguageInfo {
		nodePackageInfo = languageInfo.(NodePackageInfo)
	}

	switch t := t.(type) {
	case *schema.OptionalType:
		return mod.getTypeImports(t.ElementType, recurse, externalImports, imports, seen)
	case *schema.InputType:
		return mod.getTypeImports(t.ElementType, recurse, externalImports, imports, seen)
	case *schema.ArrayType:
		return mod.getTypeImports(t.ElementType, recurse, externalImports, imports, seen)
	case *schema.MapType:
		return mod.getTypeImports(t.ElementType, recurse, externalImports, imports, seen)
	case *schema.EnumType:
		return true
	case *schema.ObjectType:
		// If it's from another package, add an import for the external package.
		if t.Package != nil && t.Package != mod.pkg {
			pkg := t.Package.Name
			if imp, ok := nodePackageInfo.ProviderNameToModuleName[pkg]; ok {
				externalImports.Add(fmt.Sprintf("import * as %s from \"%s\";", fmt.Sprintf("pulumi%s", title(pkg)), imp))
			} else {
				externalImports.Add(fmt.Sprintf("import * as %s from \"@pulumi/%s\";", fmt.Sprintf("pulumi%s", title(pkg)), pkg))
			}
			return false
		}

		for _, p := range t.Properties {
			mod.getTypeImports(p.Type, recurse, externalImports, imports, seen)
		}
		return true
	case *schema.ResourceType:
		// If it's from another package, add an import for the external package.
		if t.Resource != nil && t.Resource.Package != mod.pkg {
			pkg := t.Resource.Package.Name
			if imp, ok := nodePackageInfo.ProviderNameToModuleName[pkg]; ok {
				externalImports.Add(fmt.Sprintf("import * as %s from \"%s\";", fmt.Sprintf("pulumi%s", title(pkg)), imp))
			} else {
				externalImports.Add(fmt.Sprintf("import * as %s from \"@pulumi/%s\";", fmt.Sprintf("pulumi%s", title(pkg)), pkg))
			}
			return false
		}

		// Don't import itself.
		if t.Resource == res {
			return false
		}

		return resourceOrTokenImport(t.Token)
	case *schema.TokenType:
		return resourceOrTokenImport(t.Token)
	case *schema.UnionType:
		needsTypes := false
		for _, e := range t.ElementTypes {
			needsTypes = mod.getTypeImports(e, recurse, externalImports, imports, seen) || needsTypes
		}
		return needsTypes
	default:
		return false
	}
}

func (mod *modContext) getImports(member interface{}, externalImports codegen.StringSet, imports map[string]codegen.StringSet) bool {
	return mod.getImportsForResource(member, externalImports, imports, nil)
}

func (mod *modContext) getImportsForResource(member interface{}, externalImports codegen.StringSet, imports map[string]codegen.StringSet, res *schema.Resource) bool {
	seen := codegen.Set{}
	switch member := member.(type) {
	case *schema.ObjectType:
		needsTypes := false
		for _, p := range member.Properties {
			needsTypes = mod.getTypeImports(p.Type, true, externalImports, imports, seen) || needsTypes
		}
		return needsTypes
	case *schema.ResourceType:
		mod.getTypeImports(member, true, externalImports, imports, seen)
		return false
	case *schema.Resource:
		needsTypes := false
		for _, p := range member.Properties {
			needsTypes = mod.getTypeImportsForResource(p.Type, false, externalImports, imports, seen, res) || needsTypes
		}
		for _, p := range member.InputProperties {
			needsTypes = mod.getTypeImportsForResource(p.Type, false, externalImports, imports, seen, res) || needsTypes
		}
		for _, method := range member.Methods {
			if method.Function.Inputs != nil {
				for _, p := range method.Function.Inputs.Properties {
					needsTypes =
						mod.getTypeImportsForResource(p.Type, false, externalImports, imports, seen, res) || needsTypes
				}
			}
			if method.Function.Outputs != nil {
				for _, p := range method.Function.Outputs.Properties {
					needsTypes =
						mod.getTypeImportsForResource(p.Type, false, externalImports, imports, seen, res) || needsTypes
				}
			}
		}
		return needsTypes
	case *schema.Function:
		needsTypes := false
		if member.Inputs != nil {
			needsTypes = mod.getTypeImports(member.Inputs, false, externalImports, imports, seen) || needsTypes
		}
		if member.Outputs != nil {
			needsTypes = mod.getTypeImports(member.Outputs, false, externalImports, imports, seen) || needsTypes
		}
		return needsTypes
	case []*schema.Property:
		needsTypes := false
		for _, p := range member {
			needsTypes = mod.getTypeImports(p.Type, false, externalImports, imports, seen) || needsTypes
		}
		return needsTypes
	default:
		return false
	}
}

func (mod *modContext) genHeader(w io.Writer, imports []string, externalImports codegen.StringSet, importedTypes map[string]codegen.StringSet) {
	fmt.Fprintf(w, "// *** WARNING: this file was generated by %v. ***\n", mod.tool)
	fmt.Fprintf(w, "// *** Do not edit by hand unless you're certain you know what you are doing! ***\n\n")

	if len(imports) > 0 {
		for _, i := range imports {
			fmt.Fprintf(w, "%s\n", i)
		}
		fmt.Fprintf(w, "\n")
	}

	if externalImports.Any() {
		for _, i := range externalImports.SortedValues() {
			fmt.Fprintf(w, "%s\n", i)
		}
		fmt.Fprintf(w, "\n")
	}

	if len(importedTypes) > 0 {
		var modules []string
		for module := range importedTypes {
			modules = append(modules, module)
		}
		sort.Strings(modules)

		for _, module := range modules {
			fmt.Fprintf(w, "import {")
			for i, name := range importedTypes[module].SortedValues() {
				if i > 0 {
					fmt.Fprint(w, ", ")
				}
				fmt.Fprint(w, name)
			}
			fmt.Fprintf(w, "} from \"%s\";\n", module)
		}
		fmt.Fprintf(w, "\n")
	}
}

// configGetter returns the name of the config.get* method used for a configuration variable and the cast necessary
// for the result of the call, if any.
func (mod *modContext) configGetter(v *schema.Property) (string, string) {
	typ := codegen.RequiredType(v)

	if typ == schema.StringType {
		return "get", ""
	}

	if tok, ok := typ.(*schema.TokenType); ok && tok.UnderlyingType == schema.StringType {
		return "get", fmt.Sprintf("<%s>", mod.typeString(typ, false, nil))
	}

	// Only try to parse a JSON object if the config isn't a straight string.
	return fmt.Sprintf("getObject<%s>", mod.typeString(typ, false, nil)), ""
}

func (mod *modContext) genConfig(w io.Writer, variables []*schema.Property) error {
	externalImports, imports := codegen.NewStringSet(), map[string]codegen.StringSet{}
	referencesNestedTypes := mod.getImports(variables, externalImports, imports)

	mod.genHeader(w, mod.sdkImports(referencesNestedTypes, true), externalImports, imports)

	fmt.Fprintf(w, "declare var exports: any;\n")

	// Create a config bag for the variables to pull from.
	fmt.Fprintf(w, "const __config = new pulumi.Config(\"%v\");\n", mod.pkg.Name)
	fmt.Fprintf(w, "\n")

	// Emit an entry for all config variables.
	for _, p := range variables {
		getfunc, cast := mod.configGetter(p)

		printComment(w, p.Comment, "", "")

		configFetch := fmt.Sprintf("%s__config.%s(\"%s\")", cast, getfunc, p.Name)
		// TODO: handle ConstValues https://github.com/pulumi/pulumi/issues/4755
		if p.DefaultValue != nil {
			v, err := mod.getDefaultValue(p.DefaultValue, codegen.UnwrapType(p.Type))
			if err != nil {
				return err
			}
			configFetch += " ?? " + v
		}
		optType := codegen.OptionalType(p)
		if p.DefaultValue != nil && p.DefaultValue.Value != nil {
			optType = codegen.RequiredType(p)
		}

		fmt.Fprintf(w, "export declare const %s: %s;\n", p.Name, mod.typeString(optType, false, nil))
		fmt.Fprintf(w, "Object.defineProperty(exports, %q, {\n", p.Name)
		fmt.Fprintf(w, "    get() {\n")
		fmt.Fprintf(w, "        return %s;\n", configFetch)
		fmt.Fprintf(w, "    },\n")
		fmt.Fprintf(w, "    enumerable: true,\n")
		fmt.Fprintf(w, "});\n\n")
	}

	return nil
}

func (mod *modContext) getRelativePath() string {
	rel, err := filepath.Rel(mod.mod, "")
	contract.Assert(err == nil)
	return path.Dir(filepath.ToSlash(rel))
}

func (mod *modContext) sdkImports(nested, utilities bool) []string {
	imports := []string{"import * as pulumi from \"@pulumi/pulumi\";"}

	relRoot := mod.getRelativePath()
	if nested {
		enumsImport := ""
		containsEnums := mod.pkg.Language["nodejs"].(NodePackageInfo).ContainsEnums
		if containsEnums {
			enumsImport = ", enums"
		}
		imports = append(imports, fmt.Sprintf(`import { input as inputs, output as outputs%s } from "%s/types";`, enumsImport, relRoot))
	}

	if utilities {
		imports = append(imports, fmt.Sprintf("import * as utilities from \"%s/utilities\";", relRoot))
	}

	return imports
}

func (mod *modContext) genTypes() (string, string) {
	externalImports, imports := codegen.NewStringSet(), map[string]codegen.StringSet{}
	for _, t := range mod.types {
		mod.getImports(t, externalImports, imports)
	}

	inputs, outputs := &bytes.Buffer{}, &bytes.Buffer{}

	mod.genHeader(inputs, mod.sdkImports(true, false), externalImports, imports)
	mod.genHeader(outputs, mod.sdkImports(true, false), externalImports, imports)

	// Build a namespace tree out of the types, then emit them.
	namespaces := mod.getNamespaces()
	mod.genNamespace(inputs, namespaces[""], true, 0)
	mod.genNamespace(outputs, namespaces[""], false, 0)

	return inputs.String(), outputs.String()
}

type namespace struct {
	name     string
	types    []*schema.ObjectType
	enums    []*schema.EnumType
	children []*namespace
}

func (mod *modContext) getNamespaces() map[string]*namespace {
	namespaces := map[string]*namespace{}
	var getNamespace func(string) *namespace
	getNamespace = func(mod string) *namespace {
		ns, ok := namespaces[mod]
		if !ok {
			name := mod
			if mod != "" {
				name = path.Base(mod)
			}

			ns = &namespace{name: name}
			if mod != "" {
				parentMod := path.Dir(mod)
				if parentMod == "." {
					parentMod = ""
				}
				parent := getNamespace(parentMod)
				parent.children = append(parent.children, ns)
			}

			namespaces[mod] = ns
		}
		return ns
	}

	for _, t := range mod.types {
		modName := mod.pkg.TokenToModule(t.Token)
		if override, ok := mod.modToPkg[modName]; ok {
			modName = override
		}
		ns := getNamespace(modName)
		ns.types = append(ns.types, t)
	}

	return namespaces
}

func (mod *modContext) genNamespace(w io.Writer, ns *namespace, input bool, level int) {
	indent := strings.Repeat("    ", level)

	sort.Slice(ns.types, func(i, j int) bool {
		return tokenToName(ns.types[i].Token) < tokenToName(ns.types[j].Token)
	})
	sort.Slice(ns.enums, func(i, j int) bool {
		return tokenToName(ns.enums[i].Token) < tokenToName(ns.enums[j].Token)
	})
	for i, t := range ns.types {
		if input && mod.details(t).inputType || !input && mod.details(t).outputType {
			mod.genType(w, t, input, level)
			if i != len(ns.types)-1 {
				fmt.Fprintf(w, "\n")
			}
		}
	}

	sort.Slice(ns.children, func(i, j int) bool {
		return ns.children[i].name < ns.children[j].name
	})
	for i, child := range ns.children {
		fmt.Fprintf(w, "%sexport namespace %s {\n", indent, child.name)
		mod.genNamespace(w, child, input, level+1)
		fmt.Fprintf(w, "%s}\n", indent)
		if i != len(ns.children)-1 {
			fmt.Fprintf(w, "\n")
		}
	}
}

func (mod *modContext) genEnum(w io.Writer, enum *schema.EnumType) error {
	indent := "    "
	enumName := tokenToName(enum.Token)
	fmt.Fprintf(w, "export const %s = {\n", enumName)
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

		printComment(w, e.Comment, e.DeprecationMessage, indent)
		fmt.Fprintf(w, "%s%s: ", indent, e.Name)
		if val, ok := e.Value.(string); ok {
			fmt.Fprintf(w, "%q,\n", val)
		} else {
			fmt.Fprintf(w, "%v,\n", e.Value)
		}
	}
	fmt.Fprintf(w, "} as const;\n")
	fmt.Fprintf(w, "\n")

	printComment(w, enum.Comment, "", "")
	fmt.Fprintf(w, "export type %[1]s = (typeof %[1]s)[keyof typeof %[1]s];\n", enumName)
	return nil
}

type fs map[string][]byte

func (fs fs) add(path string, contents []byte) {
	_, has := fs[path]
	contract.Assertf(!has, "duplicate file: %s", path)
	fs[path] = contents
}

func (mod *modContext) isReservedSourceFileName(name string) bool {
	switch name {
	case "index.ts":
		return true
	case "input.ts", "output.ts":
		return len(mod.types) != 0
	case "utilities.ts":
		return mod.mod == ""
	case "vars.ts":
		return len(mod.pkg.Config) > 0
	default:
		return false
	}
}

func (mod *modContext) gen(fs fs) error {
	files := append([]string(nil), mod.extraSourceFiles...)

	modDir := strings.ToLower(mod.mod)

	addFile := func(name, contents string) {
		p := path.Join(modDir, name)
		files = append(files, p)
		fs.add(p, []byte(contents))
	}

	// Utilities, config, readme
	switch mod.mod {
	case "":
		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, nil, nil, nil)
		fmt.Fprintf(buffer, "%s", utilitiesFile)
		fs.add(path.Join(modDir, "utilities.ts"), buffer.Bytes())

		// Ensure that the top-level (provider) module directory contains a README.md file.
		readme := mod.pkg.Language["nodejs"].(NodePackageInfo).Readme
		if readme == "" {
			readme = mod.pkg.Description
			if readme != "" && readme[len(readme)-1] != '\n' {
				readme += "\n"
			}
			if mod.pkg.Attribution != "" {
				if len(readme) != 0 {
					readme += "\n"
				}
				readme += mod.pkg.Attribution
			}
		}
		if readme != "" && readme[len(readme)-1] != '\n' {
			readme += "\n"
		}
		fs.add(path.Join(modDir, "README.md"), []byte(readme))
	case "config":
		if len(mod.pkg.Config) > 0 {
			buffer := &bytes.Buffer{}
			if err := mod.genConfig(buffer, mod.pkg.Config); err != nil {
				return err
			}
			addFile("vars.ts", buffer.String())
		}
	}

	// Resources
	for _, r := range mod.resources {
		externalImports, imports := codegen.NewStringSet(), map[string]codegen.StringSet{}
		referencesNestedTypes := mod.getImportsForResource(r, externalImports, imports, r)

		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, mod.sdkImports(referencesNestedTypes, true), externalImports, imports)

		if err := mod.genResource(buffer, r); err != nil {
			return err
		}

		fileName := mod.resourceFileName(r)
		addFile(fileName, buffer.String())
	}

	// Functions
	for _, f := range mod.functions {
		externalImports, imports := codegen.NewStringSet(), map[string]codegen.StringSet{}
		referencesNestedTypes := mod.getImports(f, externalImports, imports)

		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, mod.sdkImports(referencesNestedTypes, true), externalImports, imports)

		mod.genFunction(buffer, f)

		fileName := camel(tokenToName(f.Token)) + ".ts"
		if mod.isReservedSourceFileName(fileName) {
			fileName = camel(tokenToName(f.Token)) + "_.ts"
		}
		addFile(fileName, buffer.String())
	}

	if mod.hasEnums() {
		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, []string{}, nil, nil)

		err := mod.genEnums(buffer, mod.enums)
		if err != nil {
			return err
		}

		var fileName string
		if modDir == "" {
			fileName = "index.ts"
		} else {
			fileName = path.Join(modDir, "index.ts")
		}
		fileName = path.Join("types", "enums", fileName)
		fs.add(fileName, buffer.Bytes())
	}

	// Nested types
	if len(mod.types) > 0 {
		input, output := mod.genTypes()
		fs.add(path.Join(modDir, "input.ts"), []byte(input))
		fs.add(path.Join(modDir, "output.ts"), []byte(output))
	}

	// Index
	fs.add(path.Join(modDir, "index.ts"), []byte(mod.genIndex(files)))
	return nil
}

func getChildMod(modName string) string {
	child := strings.ToLower(modName)
	// Extract version suffix from child modules. Nested versions will have their own index.ts file.
	// Example: apps/v1beta1 -> v1beta1
	parts := strings.SplitN(child, "/", 2)
	if len(parts) == 2 {
		child = parts[1]
	}
	return child
}

// genIndex emits an index module, optionally re-exporting other members or submodules.
func (mod *modContext) genIndex(exports []string) string {
	w := &bytes.Buffer{}

	var imports []string
	// Include the SDK import if we'll be registering module resources.
	if len(mod.resources) != 0 {
		imports = mod.sdkImports(false /*nested*/, true /*utilities*/)
	}
	mod.genHeader(w, imports, nil, nil)

	// Export anything flatly that is a direct export rather than sub-module.
	if len(exports) > 0 {
		modDir := strings.ToLower(mod.mod)
		fmt.Fprintf(w, "// Export members:\n")
		sort.Strings(exports)
		for _, exp := range exports {
			rel, err := filepath.Rel(modDir, exp)
			contract.Assert(err == nil)
			if path.Base(rel) == "." {
				rel = path.Dir(rel)
			}
			fmt.Fprintf(w, "export * from \"./%s\";\n", strings.TrimSuffix(rel, ".ts"))
		}
	}

	children := codegen.NewStringSet()

	for _, mod := range mod.children {
		child := getChildMod(mod.mod)
		children.Add(child)
	}

	if len(mod.types) > 0 {
		children.Add("input")
		children.Add("output")
	}

	info, _ := mod.pkg.Language["nodejs"].(NodePackageInfo)
	if info.ContainsEnums {
		if mod.mod == "types" {
			children.Add("enums")
		} else if len(mod.enums) > 0 {
			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "// Export enums:\n")
			rel := mod.getRelativePath()
			var filePath string
			if mod.mod == "" {
				filePath = ""
			} else {
				filePath = fmt.Sprintf("/%s", mod.mod)
			}
			fmt.Fprintf(w, "export * from \"%s/types/enums%s\";\n", rel, filePath)
		}
	}

	// If there are submodules, export them.
	if len(children) > 0 {
		if len(exports) > 0 {
			fmt.Fprintf(w, "\n")
		}
		fmt.Fprintf(w, "// Export sub-modules:\n")

		directChildren := codegen.NewStringSet()
		for _, child := range children.SortedValues() {
			directChildren.Add(path.Base(child))
		}
		sorted := directChildren.SortedValues()

		for _, mod := range sorted {
			fmt.Fprintf(w, "import * as %[1]s from \"./%[1]s\";\n", mod)
		}

		printExports(w, sorted)
	}

	// If there are resources in this module, register the module with the runtime.
	if len(mod.resources) != 0 {
		mod.genResourceModule(w)
	}

	return w.String()
}

// genResourceModule generates a ResourceModule definition and the code to register an instance thereof with the
// Pulumi runtime. The generated ResourceModule supports the deserialization of resource references into fully-
// hydrated Resource instances. If this is the root module, this function also generates a ResourcePackage
// definition and its registration to support rehydrating providers.
func (mod *modContext) genResourceModule(w io.Writer) {
	contract.Assert(len(mod.resources) != 0)

	// Check for provider-only modules.
	var provider *schema.Resource
	if providerOnly := len(mod.resources) == 1 && mod.resources[0].IsProvider; providerOnly {
		provider = mod.resources[0]
	} else {
		registrations, first := codegen.StringSet{}, true
		for _, r := range mod.resources {
			if r.IsProvider {
				contract.Assert(provider == nil)
				provider = r
				continue
			}

			registrations.Add(mod.pkg.TokenToRuntimeModule(r.Token))

			if first {
				first = false
				fmt.Fprintf(w, "\n// Import resources to register:\n")
			}
			fileName := strings.TrimSuffix(mod.resourceFileName(r), ".ts")
			fmt.Fprintf(w, "import { %s } from \"./%s\";\n", resourceName(r), fileName)
		}

		fmt.Fprintf(w, "\nconst _module = {\n")
		fmt.Fprintf(w, "    version: utilities.getVersion(),\n")
		fmt.Fprintf(w, "    construct: (name: string, type: string, urn: string): pulumi.Resource => {\n")
		fmt.Fprintf(w, "        switch (type) {\n")

		for _, r := range mod.resources {
			if r.IsProvider {
				continue
			}

			fmt.Fprintf(w, "            case \"%v\":\n", r.Token)
			fmt.Fprintf(w, "                return new %v(name, <any>undefined, { urn })\n", resourceName(r))
		}

		fmt.Fprintf(w, "            default:\n")
		fmt.Fprintf(w, "                throw new Error(`unknown resource type ${type}`);\n")
		fmt.Fprintf(w, "        }\n")
		fmt.Fprintf(w, "    },\n")
		fmt.Fprintf(w, "};\n")
		for _, name := range registrations.SortedValues() {
			fmt.Fprintf(w, "pulumi.runtime.registerResourceModule(\"%v\", \"%v\", _module)\n", mod.pkg.Name, name)
		}
	}

	if provider != nil {
		fmt.Fprintf(w, "\nimport { Provider } from \"./provider\";\n\n")
		fmt.Fprintf(w, "pulumi.runtime.registerResourcePackage(\"%v\", {\n", mod.pkg.Name)
		fmt.Fprintf(w, "    version: utilities.getVersion(),\n")
		fmt.Fprintf(w, "    constructProvider: (name: string, type: string, urn: string): pulumi.ProviderResource => {\n")
		fmt.Fprintf(w, "        if (type !== \"%v\") {\n", provider.Token)
		fmt.Fprintf(w, "            throw new Error(`unknown provider type ${type}`);\n")
		fmt.Fprintf(w, "        }\n")
		fmt.Fprintf(w, "        return new Provider(name, <any>undefined, { urn });\n")
		fmt.Fprintf(w, "    },\n")
		fmt.Fprintf(w, "});\n")
	}
}

func printExports(w io.Writer, exports []string) {
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "export {\n")
	for _, mod := range exports {
		fmt.Fprintf(w, "    %s,\n", mod)
	}
	fmt.Fprintf(w, "};\n")
}

func (mod *modContext) hasEnums() bool {
	if mod.mod == "types" {
		return false
	}
	if len(mod.enums) > 0 {
		return true
	}
	if len(mod.children) > 0 {
		for _, mod := range mod.children {
			if mod.hasEnums() {
				return true
			}
		}
	}
	return false
}

func (mod *modContext) genEnums(buffer *bytes.Buffer, enums []*schema.EnumType) error {
	if len(mod.children) > 0 {
		children := codegen.NewStringSet()

		for _, mod := range mod.children {
			child := getChildMod(mod.mod)
			if mod.hasEnums() {
				children.Add(child)
			}
		}

		if len(children) > 0 {
			fmt.Fprintf(buffer, "// Export sub-modules:\n")

			directChildren := codegen.NewStringSet()
			for _, child := range children.SortedValues() {
				directChildren.Add(path.Base(child))
			}
			sorted := directChildren.SortedValues()

			for _, mod := range sorted {
				fmt.Fprintf(buffer, "import * as %[1]s from \"./%[1]s\";\n", mod)
			}
			printExports(buffer, sorted)
		}
	}
	if len(enums) > 0 {
		fmt.Fprintf(buffer, "\n")
		for i, enum := range enums {
			err := mod.genEnum(buffer, enum)
			if err != nil {
				return err
			}
			if i != len(enums)-1 {
				fmt.Fprintf(buffer, "\n")
			}
		}
	}
	return nil
}

// genPackageMetadata generates all the non-code metadata required by a Pulumi package.
func genPackageMetadata(pkg *schema.Package, info NodePackageInfo, files fs) {
	// The generator already emitted Pulumi.yaml, so that leaves two more files to write out:
	//     1) package.json: minimal NPM package metadata
	//     2) tsconfig.json: instructions for TypeScript compilation
	files.add("package.json", []byte(genNPMPackageMetadata(pkg, info)))
	files.add("tsconfig.json", []byte(genTypeScriptProjectFile(info, files)))
}

type npmPackage struct {
	Name             string            `json:"name"`
	Version          string            `json:"version"`
	Description      string            `json:"description,omitempty"`
	Keywords         []string          `json:"keywords,omitempty"`
	Homepage         string            `json:"homepage,omitempty"`
	Repository       string            `json:"repository,omitempty"`
	License          string            `json:"license,omitempty"`
	Scripts          map[string]string `json:"scripts,omitempty"`
	Dependencies     map[string]string `json:"dependencies,omitempty"`
	DevDependencies  map[string]string `json:"devDependencies,omitempty"`
	PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
	Resolutions      map[string]string `json:"resolutions,omitempty"`
	Pulumi           npmPulumiManifest `json:"pulumi,omitempty"`
}

type npmPulumiManifest struct {
	Resource          bool   `json:"resource,omitempty"`
	PluginDownloadURL string `json:"pluginDownloadURL,omitempty"`
}

func genNPMPackageMetadata(pkg *schema.Package, info NodePackageInfo) string {
	packageName := info.PackageName
	if packageName == "" {
		packageName = fmt.Sprintf("@pulumi/%s", pkg.Name)
	}

	devDependencies := map[string]string{}
	if info.TypeScriptVersion != "" {
		devDependencies["typescript"] = info.TypeScriptVersion
	} else {
		devDependencies["typescript"] = "^4.3.5"
	}

	// Create info that will get serialized into an NPM package.json.
	npminfo := npmPackage{
		Name:        packageName,
		Version:     "${VERSION}",
		Description: info.PackageDescription,
		Keywords:    pkg.Keywords,
		Homepage:    pkg.Homepage,
		Repository:  pkg.Repository,
		License:     pkg.License,
		// Ideally, this `scripts` section would include an install script that installs the provider, however, doing
		// so causes problems when we try to restore package dependencies, since we must do an install for that. So
		// we have another process that adds the install script when generating the package.json that we actually
		// publish.
		Scripts: map[string]string{
			"build": "tsc",
		},
		DevDependencies: devDependencies,
		Pulumi: npmPulumiManifest{
			Resource:          true,
			PluginDownloadURL: pkg.PluginDownloadURL,
		},
	}

	// Copy the overlay dependencies, if any.
	for depk, depv := range info.Dependencies {
		if npminfo.Dependencies == nil {
			npminfo.Dependencies = make(map[string]string)
		}
		npminfo.Dependencies[depk] = depv
	}
	for depk, depv := range info.DevDependencies {
		if npminfo.DevDependencies == nil {
			npminfo.DevDependencies = make(map[string]string)
		}
		npminfo.DevDependencies[depk] = depv
	}
	for depk, depv := range info.PeerDependencies {
		if npminfo.PeerDependencies == nil {
			npminfo.PeerDependencies = make(map[string]string)
		}
		npminfo.PeerDependencies[depk] = depv
	}
	for resk, resv := range info.Resolutions {
		if npminfo.Resolutions == nil {
			npminfo.Resolutions = make(map[string]string)
		}
		npminfo.Resolutions[resk] = resv
	}

	// If there is no @pulumi/pulumi, add "latest" as a peer dependency (for npm linking style usage).
	sdkPack := "@pulumi/pulumi"
	if npminfo.Dependencies[sdkPack] == "" &&
		npminfo.DevDependencies[sdkPack] == "" &&
		npminfo.PeerDependencies[sdkPack] == "" {
		if npminfo.PeerDependencies == nil {
			npminfo.PeerDependencies = make(map[string]string)
		}
		npminfo.PeerDependencies["@pulumi/pulumi"] = "latest"
	}

	// Now write out the serialized form.
	npmjson, err := json.MarshalIndent(npminfo, "", "    ")
	contract.Assert(err == nil)
	return string(npmjson) + "\n"
}

func genTypeScriptProjectFile(info NodePackageInfo, files fs) string {
	w := &bytes.Buffer{}

	fmt.Fprintf(w, `{
    "compilerOptions": {
        "outDir": "bin",
        "target": "es2016",
        "module": "commonjs",
        "moduleResolution": "node",
        "declaration": true,
        "sourceMap": true,
        "stripInternal": true,
        "experimentalDecorators": true,
        "noFallthroughCasesInSwitch": true,
        "forceConsistentCasingInFileNames": true,
        "strict": true
    },
    "files": [
`)

	var tsFiles []string
	for f := range files {
		if path.Ext(f) == ".ts" {
			tsFiles = append(tsFiles, f)
		}
	}
	sort.Strings(tsFiles)

	for i, file := range tsFiles {
		var suffix string
		if i != len(tsFiles)-1 {
			suffix = ","
		}
		fmt.Fprintf(w, "        \"%s\"%s\n", file, suffix)
	}
	fmt.Fprintf(w, `    ]
}
`)
	return w.String()
}

// generateModuleContextMap groups resources, types, and functions into NodeJS packages.
func generateModuleContextMap(tool string, pkg *schema.Package, extraFiles map[string][]byte,
) (map[string]*modContext, NodePackageInfo, error) {
	if err := pkg.ImportLanguages(map[string]schema.Language{"nodejs": Importer}); err != nil {
		return nil, NodePackageInfo{}, err
	}
	info, _ := pkg.Language["nodejs"].(NodePackageInfo)

	// group resources, types, and functions into NodeJS packages
	modules := map[string]*modContext{}

	var getMod func(modName string) *modContext
	getMod = func(modName string) *modContext {
		if override, ok := info.ModuleToPackage[modName]; ok {
			modName = override
		}
		mod, ok := modules[modName]
		if !ok {
			mod = &modContext{
				pkg:                     pkg,
				mod:                     modName,
				tool:                    tool,
				compatibility:           info.Compatibility,
				modToPkg:                info.ModuleToPackage,
				disableUnionOutputTypes: info.DisableUnionOutputTypes,
			}

			if modName != "" {
				parentName := path.Dir(modName)
				if parentName == "." {
					parentName = ""
				}
				parent := getMod(parentName)
				parent.children = append(parent.children, mod)
			}

			modules[modName] = mod
		}
		return mod
	}

	getModFromToken := func(token string) *modContext {
		return getMod(pkg.TokenToModule(token))
	}

	// Create a temporary module for type information.
	types := &modContext{}

	// Create the config module if necessary.
	if len(pkg.Config) > 0 &&
		info.Compatibility != kubernetes20 { // k8s SDK doesn't use config.
		_ = getMod("config")
	}

	visitObjectTypes(pkg.Config, func(t *schema.ObjectType) {
		types.details(t).outputType = true
	})

	scanResource := func(r *schema.Resource) {
		mod := getModFromToken(r.Token)
		mod.resources = append(mod.resources, r)
		visitObjectTypes(r.Properties, func(t *schema.ObjectType) {
			types.details(t).outputType = true
		})
		visitObjectTypes(r.InputProperties, func(t *schema.ObjectType) {
			types.details(t).inputType = true
		})
		if r.StateInputs != nil {
			visitObjectTypes(r.StateInputs.Properties, func(t *schema.ObjectType) {
				types.details(t).inputType = true
			})
		}
	}

	scanResource(pkg.Provider)
	for _, r := range pkg.Resources {
		scanResource(r)
	}

	// Clear the input and outputs sets: we want the visitors below to touch the transitive closure of types reachable
	// from function inputs and outputs, including types that have already been visited.
	for _, f := range pkg.Functions {
		mod := getModFromToken(f.Token)
		if !f.IsMethod {
			mod.functions = append(mod.functions, f)
		}
		if f.Inputs != nil {
			visitObjectTypes(f.Inputs.Properties, func(t *schema.ObjectType) {
				types.details(t).inputType = true
			})
		}
		if f.Outputs != nil {
			visitObjectTypes(f.Outputs.Properties, func(t *schema.ObjectType) {
				types.details(t).outputType = true
			})
		}
	}

	if _, ok := modules["types"]; ok {
		return nil, info, errors.New("this provider has a `types` module which is reserved for input/output types")
	}

	// Create the types module.
	for _, t := range pkg.Types {
		switch typ := t.(type) {
		case *schema.ObjectType:
			types.types = append(types.types, typ)
		case *schema.EnumType:
			info.ContainsEnums = true
			mod := getModFromToken(typ.Token)
			mod.enums = append(mod.enums, typ)
		default:
			continue
		}
	}
	if len(types.types) > 0 {
		typeDetails, typeList := types.typeDetails, types.types
		types = getMod("types")
		types.typeDetails, types.types = typeDetails, typeList
	}

	// Add Typescript source files to the corresponding modules. Note that we only add the file names; the contents are
	// still laid out manually in GeneratePackage.
	for p := range extraFiles {
		if path.Ext(p) != ".ts" {
			continue
		}

		modName := path.Dir(p)
		if modName == "/" || modName == "." {
			modName = ""
		}
		mod := getMod(modName)
		mod.extraSourceFiles = append(mod.extraSourceFiles, p)
	}

	return modules, info, nil
}

// LanguageResource holds information about a resource to be used by downstream codegen.
type LanguageResource struct {
	*schema.Resource

	Name       string             // The resource name (e.g., "FlowSchema")
	Package    string             // The name of the package containing the resource definition (e.g., "flowcontrol.v1alpha1")
	Properties []LanguageProperty // Properties of the resource
}

// LanguageProperty holds information about a resource property to be used by downstream codegen.
type LanguageProperty struct {
	ConstValue string // If set, the constant value of the property (e.g., "flowcontrol.apiserver.k8s.io/v1alpha1")
	Name       string // The name of the property (e.g., "FlowSchemaSpec")
	Package    string // The package path containing the property definition (e.g., "outputs.flowcontrol.v1alpha1")
}

// LanguageResources returns a map of resources that can be used by downstream codegen. The map
// key is the resource schema token.
func LanguageResources(pkg *schema.Package) (map[string]LanguageResource, error) {
	resources := map[string]LanguageResource{}

	modules, _, err := generateModuleContextMap("", pkg, nil)
	if err != nil {
		return nil, err
	}

	for modName, mod := range modules {
		for _, r := range mod.resources {
			packagePath := strings.Replace(modName, "/", ".", -1)
			lr := LanguageResource{
				Resource: r,
				Name:     resourceName(r),
				Package:  packagePath,
			}
			for _, p := range r.Properties {
				lp := LanguageProperty{
					Name: p.Name,
				}
				if p.ConstValue != nil {
					lp.ConstValue = mod.typeString(p.Type, false, p.ConstValue)
				} else {
					lp.Package = mod.typeString(p.Type, false, nil)
				}
				lr.Properties = append(lr.Properties, lp)
			}
			resources[r.Token] = lr
		}
	}

	return resources, nil
}

func GeneratePackage(tool string, pkg *schema.Package, extraFiles map[string][]byte) (map[string][]byte, error) {
	modules, info, err := generateModuleContextMap(tool, pkg, extraFiles)
	if err != nil {
		return nil, err
	}
	pkg.Language["nodejs"] = info

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
	genPackageMetadata(pkg, info, files)
	return files, nil
}

const utilitiesFile = `
export function getEnv(...vars: string[]): string | undefined {
    for (const v of vars) {
        const value = process.env[v];
        if (value) {
            return value;
        }
    }
    return undefined;
}

export function getEnvBoolean(...vars: string[]): boolean | undefined {
    const s = getEnv(...vars);
    if (s !== undefined) {
        // NOTE: these values are taken from https://golang.org/src/strconv/atob.go?s=351:391#L1, which is what
        // Terraform uses internally when parsing boolean values.
        if (["1", "t", "T", "true", "TRUE", "True"].find(v => v === s) !== undefined) {
            return true;
        }
        if (["0", "f", "F", "false", "FALSE", "False"].find(v => v === s) !== undefined) {
            return false;
        }
    }
    return undefined;
}

export function getEnvNumber(...vars: string[]): number | undefined {
    const s = getEnv(...vars);
    if (s !== undefined) {
        const f = parseFloat(s);
        if (!isNaN(f)) {
            return f;
        }
    }
    return undefined;
}

export function getVersion(): string {
    let version = require('./package.json').version;
    // Node allows for the version to be prefixed by a "v", while semver doesn't.
    // If there is a v, strip it off.
    if (version.indexOf('v') === 0) {
        version = version.slice(1);
    }
    return version;
}
`
