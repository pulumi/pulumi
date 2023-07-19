// Copyright 2016-2022, Pulumi Corporation.
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
package nodejs

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs/tstypes"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// The minimum version of @pulumi/pulumi compatible with the generated SDK.
const (
	MinimumValidSDKVersion   string = "^3.42.0"
	MinimumTypescriptVersion string = "^4.3.5"
	MinimumNodeTypesVersion  string = "^14"
)

type typeDetails struct {
	outputType bool
	inputType  bool

	usedInFunctionOutputVersionInputs bool // helps decide naming under the tfbridge20 flag
}

// title capitalizes the first rune in s.
//
// Examples:
// "hello"   => "Hello"
// "hiAlice" => "HiAlice"
// "hi.Bob"  => "Hi.Bob"
//
// Note: This is expected to work on strings which are not valid identifiers.
func title(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	return string(append([]rune{unicode.ToUpper(runes[0])}, runes[1:]...))
}

// camel converts s to camel case.
//
// Examples:
// "helloWorld"    => "helloWorld"
// "HelloWorld"    => "helloWorld"
// "JSONObject"    => "jsonobject"
// "My-FRIEND.Bob" => "my-FRIEND.Bob"
func camel(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	res := slice.Prealloc[rune](len(runes))
	for i, r := range runes {
		if unicode.IsLower(r) {
			res = append(res, runes[i:]...)
			break
		}
		res = append(res, unicode.ToLower(r))
	}
	return string(res)
}

// pascal converts s to pascal case. Word breaks are signified by illegal
// identifier runes (excluding '.'). These are found by use of
// isLegalIdentifierPart.
//
// Examples:
// "My-Friend.Bob"  => "MyFriend.Bob"
// "JSONObject"     => "JSONObject"'
// "a-glad-dayTime" => "AGladDayTime"
//
// Note: because camel aggressively down-cases the first continuous sub-string
// of uppercase characters, we cannot define pascal as title(camel(x)).
func pascal(s string) string {
	split := [][]rune{{}}
	for _, r := range s {
		if !isLegalIdentifierPart(r) && r != '.' {
			split = append(split, []rune{})
		} else {
			split[len(split)-1] = append(split[len(split)-1], r)
		}
	}
	words := make([]string, len(split))
	for i, v := range split {
		words[i] = title(string(v))
	}
	return strings.Join(words, "")
}

// externalModuleName Formats the name of package to comply with an external
// module.
func externalModuleName(s string) string {
	return fmt.Sprintf("pulumi%s", pascal(s))
}

type modContext struct {
	pkg              schema.PackageReference
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

	// Determine whether to lift single-value method return values
	liftSingleValueMethodReturns bool
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
		modName = strings.ReplaceAll(modName, "/", ".") + "."
	}

	return modName
}

func (mod *modContext) namingContext(pkg schema.PackageReference) (namingCtx *modContext, pkgName string, external bool) {
	namingCtx = mod
	if pkg != nil && !codegen.PkgEquals(pkg, mod.pkg) {
		external = true
		pkgName = pkg.Name() + "."

		var info NodePackageInfo
		def, err := pkg.Definition()
		contract.AssertNoErrorf(err, "error loading definition for package %q", pkg.Name())
		contract.AssertNoErrorf(
			def.ImportLanguages(map[string]schema.Language{"nodejs": Importer}),
			"failed to import nodejs language for package %v", pkg.Name())
		if v, ok := def.Language["nodejs"].(NodePackageInfo); ok {
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

func (mod *modContext) objectType(pkg schema.PackageReference, details *typeDetails, tok string, input, args, enum bool) string {
	root := "outputs."
	if input {
		root = "inputs."
	}

	namingCtx, pkgName, external := mod.namingContext(pkg)
	if external {
		pkgName = externalModuleName(pkgName)
		root = "types.output."
		if input {
			root = "types.input."
		}
	}

	modName, name := namingCtx.tokenToModName(tok), tokenToName(tok)

	if enum {
		prefix := "enums."
		if external {
			prefix = pkgName
		}
		return prefix + modName + title(name)
	}

	if args && input && details != nil && details.usedInFunctionOutputVersionInputs {
		name += "Args"
	} else if args && namingCtx.compatibility != tfbridge20 && namingCtx.compatibility != kubernetes20 {
		name += "Args"
	}

	return pkgName + root + modName + title(name)
}

func (mod *modContext) resourceType(r *schema.ResourceType) string {
	if strings.HasPrefix(r.Token, "pulumi:providers:") {
		pkgName := strings.TrimPrefix(r.Token, "pulumi:providers:")
		if pkgName != mod.pkg.Name() {
			pkgName = externalModuleName(pkgName)
		}

		return fmt.Sprintf("%s.Provider", pkgName)
	}

	pkg := mod.pkg
	if r.Resource != nil {
		pkg = r.Resource.PackageReference
	}
	namingCtx, pkgName, external := mod.namingContext(pkg)
	if !external {
		name := tokenToName(r.Token)
		return title(name)
	}

	pkgName = externalModuleName(pkgName)
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

func (mod *modContext) typeAst(t schema.Type, input bool, constValue interface{}) tstypes.TypeAst {
	switch t := t.(type) {
	case *schema.OptionalType:
		return tstypes.Union(
			mod.typeAst(t.ElementType, input, constValue),
			tstypes.Identifier("undefined"),
		)
	case *schema.InputType:
		typ := mod.typeString(codegen.SimplifyInputUnion(t.ElementType), input, constValue)
		if typ == "any" {
			return tstypes.Identifier("any")
		}
		return tstypes.Identifier(fmt.Sprintf("pulumi.Input<%s>", typ))
	case *schema.EnumType:
		return tstypes.Identifier(mod.objectType(t.PackageReference, nil, t.Token, input, false, true))
	case *schema.ArrayType:
		return tstypes.Array(mod.typeAst(t.ElementType, input, constValue))
	case *schema.MapType:
		return tstypes.StringMap(mod.typeAst(t.ElementType, input, constValue))
	case *schema.ObjectType:
		details := mod.details(t)
		return tstypes.Identifier(mod.objectType(t.PackageReference, details, t.Token, input, t.IsInputShape(), false))
	case *schema.ResourceType:
		return tstypes.Identifier(mod.resourceType(t))
	case *schema.TokenType:
		return tstypes.Identifier(tokenToName(t.Token))
	case *schema.UnionType:
		if !input && mod.disableUnionOutputTypes {
			if t.DefaultType != nil {
				return mod.typeAst(t.DefaultType, input, constValue)
			}
			return tstypes.Identifier("any")
		}

		elements := make([]tstypes.TypeAst, len(t.ElementTypes))
		for i, e := range t.ElementTypes {
			elements[i] = mod.typeAst(e, input, constValue)
		}
		return tstypes.Union(elements...)
	default:
		switch t {
		case schema.BoolType:
			return tstypes.Identifier("boolean")
		case schema.IntType, schema.NumberType:
			return tstypes.Identifier("number")
		case schema.StringType:
			if constValue != nil {
				return tstypes.Identifier(fmt.Sprintf("%q", constValue.(string)))
			}
			return tstypes.Identifier("string")
		case schema.ArchiveType:
			return tstypes.Identifier("pulumi.asset.Archive")
		case schema.AssetType:
			return tstypes.Union(
				tstypes.Identifier("pulumi.asset.Asset"),
				tstypes.Identifier("pulumi.asset.Archive"),
			)
		case schema.JSONType:
			fallthrough
		case schema.AnyType:
			return tstypes.Identifier("any")
		}
	}
	panic(fmt.Errorf("unexpected type %T", t))
}

func (mod *modContext) typeString(t schema.Type, input bool, constValue interface{}) string {
	return tstypes.TypeLiteral(tstypes.Normalize(mod.typeAst(t, input, constValue)))
}

func isStringType(t schema.Type) bool {
	t = codegen.UnwrapType(t)

	switch typ := t.(type) {
	case *schema.TokenType:
		t = typ.UnderlyingType
	case *schema.EnumType:
		t = typ.ElementType
	case *schema.UnionType:
		// The following case detects for relaxed string enums. If it's a Union, check if one ElementType is an EnumType.
		// If yes, t is the ElementType of the EnumType.
		for _, tt := range typ.ElementTypes {
			t = codegen.UnwrapType(tt)
			if typ, ok := t.(*schema.EnumType); ok {
				t = typ.ElementType
			}
		}
	}

	return t == schema.StringType
}

func sanitizeComment(str string) string {
	return strings.ReplaceAll(str, "*/", "*&#47;")
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

// Generates a plain interface type.
//
// We use this to represent both argument and plain object types.
func (mod *modContext) genPlainType(w io.Writer, name, comment string,
	properties []*schema.Property, input, readonly bool, level int,
) error {
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
	return nil
}

// Generate a provide defaults function for an associated plain object.
func (mod *modContext) genPlainObjectDefaultFunc(w io.Writer, name string,
	properties []*schema.Property, input, readonly bool, level int,
) error {
	indent := strings.Repeat("    ", level)
	defaults := []string{}
	for _, p := range properties {
		if p.DefaultValue != nil {
			dv, err := mod.getDefaultValue(p.DefaultValue, codegen.UnwrapType(p.Type))
			if err != nil {
				return err
			}
			defaults = append(defaults, fmt.Sprintf("%s: (val.%s) ?? %s", p.Name, p.Name, dv))
		} else if funcName := mod.provideDefaultsFuncName(p.Type, input); funcName != "" {
			// ProvideDefaults functions have the form `(Input<shape> | undefined) ->
			// Output<shape> | undefined`. We need to disallow the undefined. This is safe
			// because val.%arg existed in the input (type system enforced).
			var compositeObject string
			if codegen.IsNOptionalInput(p.Type) {
				compositeObject = fmt.Sprintf("pulumi.output(val.%s).apply(%s)", p.Name, funcName)
			} else {
				compositeObject = fmt.Sprintf("%s(val.%s)", funcName, p.Name)
			}
			if !p.IsRequired() {
				compositeObject = fmt.Sprintf("(val.%s ? %s : undefined)", p.Name, compositeObject)
			}
			defaults = append(defaults, fmt.Sprintf("%s: %s", p.Name, compositeObject))
		}
	}

	// There are no defaults, so don't generate a default function.
	if len(defaults) == 0 {
		return nil
	}
	// Generates a function header that looks like this:
	// export function %sProvideDefaults(val: pulumi.Input<%s> | undefined): pulumi.Output<%s> | undefined {
	//     const def = (val: LayeredTypeArgs) => ({
	//         ...val,
	defaultProvderName := provideDefaultsFuncNameFromName(name)
	printComment(w, fmt.Sprintf("%s sets the appropriate defaults for %s",
		defaultProvderName, name), "", indent)
	fmt.Fprintf(w, "%sexport function %s(val: %s): "+
		"%s {\n", indent, defaultProvderName, name, name)
	fmt.Fprintf(w, "%s    return {\n", indent)
	fmt.Fprintf(w, "%s        ...val,\n", indent)

	// Fields look as follows
	// %s: (val.%s) ?? devValue,
	for _, val := range defaults {
		fmt.Fprintf(w, "%s        %s,\n", indent, val)
	}
	fmt.Fprintf(w, "%s    };\n", indent)
	fmt.Fprintf(w, "%s}\n", indent)
	return nil
}

// The name of the helper function used to provide default values to plain
// types, derived purely from the name of the enclosing type. Prefer to use
// provideDefaultsFuncName when full type information is available.
func provideDefaultsFuncNameFromName(typeName string) string {
	var i int
	if in := strings.LastIndex(typeName, "."); in != -1 {
		i = in
	}
	// path + camel(name) + ProvideDefaults suffix
	return typeName[:i] + camel(typeName[i:]) + "ProvideDefaults"
}

// The name of the function used to set defaults on the plain type.
//
// `type` is the type which the function applies to.
// `input` indicates whither `type` is an input type.
func (mod *modContext) provideDefaultsFuncName(typ schema.Type, input bool) string {
	if !codegen.IsProvideDefaultsFuncRequired(typ) {
		return ""
	}
	requiredType := codegen.UnwrapType(typ)
	typeName := mod.typeString(requiredType, input, nil)
	return provideDefaultsFuncNameFromName(typeName)
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
		return "", fmt.Errorf("unsupported default value of type %T", value)
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
		if t != schema.StringType && getType == "" {
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

func (mod *modContext) genResource(w io.Writer, r *schema.Resource) (resourceFileInfo, error) {
	info := resourceFileInfo{}

	// Create a resource module file into which all of this resource's types will go.
	name := resourceName(r)
	info.resourceClassName = name

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
	fmt.Fprintf(w, "export class %s extends pulumi.%s {\n", info.resourceClassName, baseType)

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
			fmt.Fprintf(w, "        pulumi.log.warn(\"%s is deprecated: %s\")\n", name, escape(r.DeprecationMessage))
		}
		fmt.Fprintf(w, "        return new %s(name, %s{ ...opts, id: id });\n", name, stateRef)
		fmt.Fprintf(w, "    }\n")
		fmt.Fprintf(w, "\n")
	}

	pulumiType := r.Token
	if r.IsProvider {
		pulumiType = mod.pkg.Name()
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

	typeExpression := fmt.Sprintf("%s.__pulumiType", name)
	if r.IsProvider {
		// We pass __pulumiType to the ProviderResource constructor as the "type" for this provider, the
		// ProviderResource constructor in the SDK then prefixes "pulumi:providers:" to that token and passes that
		// down to the CustomResource constructor, which then assigns that type token to the newly constructed
		// objects __pulumiType field. As such we also need to prefix "pulumi:providers:" when doing the equality
		// check here.
		typeExpression = "\"pulumi:providers:\" + " + typeExpression
	}
	fmt.Fprintf(w, "        return obj['__pulumiType'] === %s;\n", typeExpression)
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
			applyDefaults := func(arg string) string {
				if name := mod.provideDefaultsFuncName(prop.Type, true /*input*/); name != "" {
					var body string
					if codegen.IsNOptionalInput(prop.Type) {
						body = fmt.Sprintf("pulumi.output(%[2]s).apply(%[1]s)", name, arg)
					} else {
						body = fmt.Sprintf("%s(%s)", name, arg)
					}
					return fmt.Sprintf("(%s ? %s : undefined)", arg, body)
				}
				return arg
			}

			argValue := applyDefaults(fmt.Sprintf("args.%s", prop.Name))
			if prop.Secret {
				arg = fmt.Sprintf("args?.%[1]s ? pulumi.secret(%[2]s) : undefined", prop.Name, argValue)
			} else {
				arg = fmt.Sprintf("args ? %[1]s : undefined", argValue)
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
			fmt.Fprintf(w, "%sresourceInputs[\"%s\"] = %s;\n", prefix, prop.Name, arg)
		}

		for _, prop := range r.Properties {
			prefix := "            "
			if !ins.Has(prop.Name) {
				fmt.Fprintf(w, "%sresourceInputs[\"%s\"] = undefined /*out*/;\n", prefix, prop.Name)
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
			fmt.Fprintf(w, "        pulumi.log.warn(\"%s is deprecated: %s\")\n", name, escape(r.DeprecationMessage))
		}
		fmt.Fprintf(w, "        let resourceInputs: pulumi.Inputs = {};\n")
		fmt.Fprintf(w, "        opts = opts || {};\n")

		if r.StateInputs != nil {
			// The lookup case:
			fmt.Fprintf(w, "        if (opts.id) {\n")
			fmt.Fprintf(w, "            const state = argsOrState as %[1]s | undefined;\n", stateType)
			for _, prop := range r.StateInputs.Properties {
				fmt.Fprintf(w, "            resourceInputs[\"%[1]s\"] = state ? state.%[1]s : undefined;\n", prop.Name)
			}
			// The creation case (with args):
			fmt.Fprintf(w, "        } else {\n")
			fmt.Fprintf(w, "            const args = argsOrState as %s | undefined;\n", argsType)
			err := genInputProps()
			if err != nil {
				return resourceFileInfo{}, err
			}
		} else {
			// The creation case:
			fmt.Fprintf(w, "        if (!opts.id) {\n")
			err := genInputProps()
			if err != nil {
				return resourceFileInfo{}, err
			}
			// The get case:
			fmt.Fprintf(w, "        } else {\n")
			for _, prop := range r.Properties {
				fmt.Fprintf(w, "            resourceInputs[\"%[1]s\"] = undefined /*out*/;\n", prop.Name)
			}
		}
	} else {
		fmt.Fprintf(w, "        let resourceInputs: pulumi.Inputs = {};\n")
		fmt.Fprintf(w, "        opts = opts || {};\n")
		fmt.Fprintf(w, "        {\n")
		err := genInputProps()
		if err != nil {
			return resourceFileInfo{}, err
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
	// If a `pluginDownloadURL` was supplied by the generating schema, we supply a default facility
	// much like for version. Both operations are handled in the utilities library.
	fmt.Fprint(w, "        opts = pulumi.mergeOptions(utilities.resourceOptsDefaults(), opts);\n")

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

	replaceOnChanges, errList := r.ReplaceOnChanges()
	for _, err := range errList {
		cmdutil.Diag().Warningf(&diag.Diag{Message: err.Error()})
	}
	replaceOnChangesStrings := schema.PropertyListJoinToString(replaceOnChanges,
		func(x string) string { return x })
	if len(replaceOnChanges) > 0 {
		fmt.Fprintf(w, `        const replaceOnChanges = { replaceOnChanges: ["%s"] };`, strings.Join(replaceOnChangesStrings, `", "`))
		fmt.Fprintf(w, "\n        opts = pulumi.mergeOptions(opts, replaceOnChanges);\n")
	}

	// If it's a ComponentResource, set the remote option.
	if r.IsComponent {
		fmt.Fprintf(w, "        super(%s.__pulumiType, name, resourceInputs, opts, true /*remote*/);\n", name)
	} else {
		fmt.Fprintf(w, "        super(%s.__pulumiType, name, resourceInputs, opts);\n", name)
	}

	fmt.Fprintf(w, "    }\n")

	// Generate methods.
	genMethod := func(method *schema.Method) {
		methodName := camel(method.Name)
		fun := method.Function

		var objectReturnType *schema.ObjectType
		if fun.ReturnType != nil {
			if objectType, ok := fun.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				objectReturnType = objectType
			} else if !fun.ReturnTypePlain {
				// Currently the code only knows how to generate code for methods returning an
				// ObjectType or methods returning a plain resource All other methods are simply
				// skipped; bail here.
				return
			}
		}

		liftReturn := mod.liftSingleValueMethodReturns && objectReturnType != nil && len(objectReturnType.Properties) == 1

		// Write the TypeDoc/JSDoc for the data source function.
		fmt.Fprint(w, "\n")
		printComment(w, codegen.FilterExamples(fun.Comment, "typescript"), fun.DeprecationMessage, "    ")

		// Now, emit the method signature.
		var args []*schema.Property
		var argsig string
		argsOptional := true
		if fun.Inputs != nil {
			// Filter out the __self__ argument from the inputs.
			args = slice.Prealloc[*schema.Property](len(fun.Inputs.InputShape.Properties))
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
		if fun.ReturnType == nil {
			retty = "void"
		} else if fun.ReturnTypePlain {
			var innerType string
			if objectReturnType == nil {
				innerType = mod.typeString(fun.ReturnType, false, nil)
			} else {
				innerType = fmt.Sprintf("%s.%sResult", name, title(method.Name))
			}
			retty = fmt.Sprintf("Promise<%s>", innerType)
		} else if liftReturn {
			retty = fmt.Sprintf("pulumi.Output<%s>", mod.typeString(objectReturnType.Properties[0].Type, false, nil))
		} else {
			retty = fmt.Sprintf("pulumi.Output<%s.%sResult>", name, title(method.Name))
		}
		fmt.Fprintf(w, "    %s(%s): %s {\n", methodName, argsig, retty)
		if fun.DeprecationMessage != "" {
			fmt.Fprintf(w, "        pulumi.log.warn(\"%s.%s is deprecated: %s\")\n", name, methodName,
				escape(fun.DeprecationMessage))
		}

		// Zero initialize the args if empty and necessary.
		if len(args) > 0 && argsOptional {
			fmt.Fprintf(w, "        args = args || {};\n")
		}

		// Now simply call the runtime function with the arguments, returning the results.
		var ret string
		if fun.ReturnType != nil {
			if liftReturn {
				ret = fmt.Sprintf("const result: pulumi.Output<%s.%sResult> = ", name, title(method.Name))
			} else {
				ret = "return "
			}
		}

		if fun.ReturnTypePlain {
			fmt.Fprintf(w, "        %sutilities.callAsync(\"%s\", {\n", ret, fun.Token)
		} else {
			fmt.Fprintf(w, "        %spulumi.runtime.call(\"%s\", {\n", ret, fun.Token)
		}

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
		fmt.Fprintf(w, "        }, this")

		if fun.ReturnTypePlain {
			// Unwrap magic property "res" for methods that return a plain non-object-type.
			if objectReturnType == nil {
				fmt.Fprintf(w, `, {property: "res"});`)
			} else {
				fmt.Fprintf(w, `, {});`)
			}
		} else {
			fmt.Fprintf(w, ");\n")
		}

		if liftReturn {
			fmt.Fprintf(w, "        return result.%s;\n", camel(objectReturnType.Properties[0].Name))
		}
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
		if err := mod.genPlainType(w, stateType, r.StateInputs.Comment, r.StateInputs.Properties, true, false, 0); err != nil {
			return resourceFileInfo{}, err
		}
		info.stateInterfaceName = stateType
	}

	// Emit the argument type for construction.
	fmt.Fprintf(w, "\n")
	argsComment := fmt.Sprintf("The set of arguments for constructing a %s resource.", name)
	if err := mod.genPlainType(w, argsType, argsComment, r.InputProperties, true, false, 0); err != nil {
		return resourceFileInfo{}, err
	}
	info.resourceArgsInterfaceName = argsType

	// Emit any method types inside a namespace merged with the class, to represent types nested in the class.
	// https://www.typescriptlang.org/docs/handbook/declaration-merging.html#merging-namespaces-with-classes
	genMethodTypes := func(w io.Writer, method *schema.Method) error {
		fun := method.Function
		methodName := title(method.Name)
		if fun.Inputs != nil {
			args := slice.Prealloc[*schema.Property](len(fun.Inputs.InputShape.Properties))
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
				if err := mod.genPlainType(w, methodName+"Args", comment, args, true, false, 1); err != nil {
					return err
				}
				fmt.Fprintf(w, "\n")
			}
		}

		if fun.ReturnType != nil {
			genReturnType := func(properties []*schema.Property) error {
				comment := fun.Inputs.Comment
				if comment == "" {
					comment = fmt.Sprintf("The results of the %s.%s method.", name, method.Name)
				}
				if err := mod.genPlainType(w, methodName+"Result", comment, properties, false, true, 1); err != nil {
					return err
				}
				fmt.Fprintf(w, "\n")
				return nil
			}
			if objectType, ok := fun.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				if err := genReturnType(objectType.Properties); err != nil {
					return err
				}
			}
			// For non-object types with fun.ReturnTypePlain return type is not needed.
		}
		return nil
	}
	types := &bytes.Buffer{}
	for _, method := range r.Methods {
		if err := genMethodTypes(types, method); err != nil {
			return resourceFileInfo{}, err
		}
	}
	typesString := types.String()
	if typesString != "" {
		fmt.Fprintf(w, "\nexport namespace %s {\n", name)
		fmt.Fprint(w, typesString)
		fmt.Fprint(w, "}\n")
		info.methodsNamespaceName = name
	}
	return info, nil
}

func (mod *modContext) functionReturnType(fun *schema.Function) string {
	name := tokenToFunctionName(fun.Token)
	if fun.ReturnType == nil {
		return "void"
	}

	if _, isObject := fun.ReturnType.(*schema.ObjectType); isObject && fun.InlineObjectAsReturnType {
		return title(name) + "Result"
	}

	return mod.typeString(fun.ReturnType, false, nil)
}

// runtimeInvokeFunction returns the name of the Invoke function to use at runtime
// from the SDK for the given provider function. This is necessary because some
// functions have simple return types such as number, string, array<string> etc.
// and the SDK's invoke function cannot handle these types since the engine expects
// the result of invokes to be a dictionary.
//
// We use invoke for functions with object return types and invokeSingle for everything else.
func runtimeInvokeFunction(fun *schema.Function) string {
	switch fun.ReturnType.(type) {
	// If the function has no return type, it is a void function.
	case nil:
		return "invoke"
	// If the function has an object return type, it is a normal invoke function.
	case *schema.ObjectType:
		return "invoke"
	// If the function has an object return type, it is also a normal invoke function.
	// because the deserialization can handle it
	case *schema.MapType:
		return "invoke"
	default:
		// Anything else needs to be handled by InvokeSingle
		// which expects an object with a single property to be returned
		// then unwraps the value from that property
		return "invokeSingle"
	}
}

func (mod *modContext) genFunction(w io.Writer, fun *schema.Function) (functionFileInfo, error) {
	name := tokenToFunctionName(fun.Token)
	info := functionFileInfo{functionName: name}

	// Write the TypeDoc/JSDoc for the data source function.
	printComment(w, codegen.FilterExamples(fun.Comment, "typescript"), "", "")

	if fun.DeprecationMessage != "" {
		fmt.Fprintf(w, "/** @deprecated %s */\n", fun.DeprecationMessage)
	}

	// Now, emit the function signature.
	var argsig string
	argsOptional := functionArgsOptional(fun)
	if fun.Inputs != nil {
		optFlag := ""
		if argsOptional {
			optFlag = "?"
		}
		argsig = fmt.Sprintf("args%s: %sArgs, ", optFlag, title(name))
	}

	funReturnType := mod.functionReturnType(fun)

	fmt.Fprintf(w, "export function %s(", name)
	if fun.MultiArgumentInputs {
		for _, prop := range fun.Inputs.Properties {
			if prop.IsRequired() {
				fmt.Fprintf(w, "%s: ", prop.Name)
				fmt.Fprintf(w, "%s, ", mod.typeString(prop.Type, false, nil))
			} else {
				fmt.Fprintf(w, "%s?: ", prop.Name)
				// since we already applied the '?' to the type, we can simplify
				// the optional-ness of the type
				propType := prop.Type.(*schema.OptionalType)
				fmt.Fprintf(w, "%s, ", mod.typeString(propType.ElementType, false, nil))
			}
		}
	} else {
		fmt.Fprintf(w, "%s", argsig)
	}

	fmt.Fprintf(w, "opts?: pulumi.InvokeOptions): Promise<%s> {\n", funReturnType)
	if fun.DeprecationMessage != "" && mod.compatibility != kubernetes20 {
		fmt.Fprintf(w, "    pulumi.log.warn(\"%s is deprecated: %s\")\n", name, escape(fun.DeprecationMessage))
	}

	// Zero initialize the args if empty and necessary.
	if fun.Inputs != nil && argsOptional && !fun.MultiArgumentInputs {
		fmt.Fprintf(w, "    args = args || {};\n")
	}

	fmt.Fprint(w, "\n")
	// If the caller didn't request a specific version, supply one using the version of this library.
	fmt.Fprintf(w, "    opts = pulumi.mergeOptions(utilities.resourceOptsDefaults(), opts || {});\n")
	invokeCall := runtimeInvokeFunction(fun)
	// Now simply invoke the runtime function with the arguments, returning the results.
	fmt.Fprintf(w, "    return pulumi.runtime.%s(\"%s\", {\n", invokeCall, fun.Token)
	if fun.Inputs != nil {
		for _, p := range fun.Inputs.Properties {
			// Pass the argument to the invocation.
			body := fmt.Sprintf("args.%s", p.Name)
			if fun.MultiArgumentInputs {
				body = p.Name
			}

			if name := mod.provideDefaultsFuncName(p.Type, true /*input*/); name != "" {
				if codegen.IsNOptionalInput(p.Type) {
					body = fmt.Sprintf("pulumi.output(%s).apply(%s)", body, name)
				} else {
					body = fmt.Sprintf("%s(%s)", name, body)
				}
				body = fmt.Sprintf("args.%s ? %s : undefined", p.Name, body)
			}
			fmt.Fprintf(w, "        \"%[1]s\": %[2]s,\n", p.Name, body)
		}
	}

	fmt.Fprint(w, "    }, opts);\n")
	fmt.Fprint(w, "}\n")

	// If there are argument and/or return types, emit them.
	if fun.Inputs != nil && !fun.MultiArgumentInputs {
		fmt.Fprintf(w, "\n")
		argsInterfaceName := title(name) + "Args"
		if err := mod.genPlainType(w, argsInterfaceName, fun.Inputs.Comment, fun.Inputs.Properties, true, false, 0); err != nil {
			return info, err
		}
		info.functionArgsInterfaceName = argsInterfaceName
	}

	// if the return type is an inline object definition (not a reference), emit it.
	if fun.ReturnType != nil {
		if objectType, ok := fun.ReturnType.(*schema.ObjectType); ok && fun.InlineObjectAsReturnType {
			fmt.Fprintf(w, "\n")
			resultInterfaceName := title(name) + "Result"
			if err := mod.genPlainType(w, resultInterfaceName,
				objectType.Comment, objectType.Properties, false, true, 0); err != nil {
				return info, err
			}
			info.functionResultInterfaceName = resultInterfaceName
		}
	}

	return mod.genFunctionOutputVersion(w, fun, info)
}

func functionArgsOptional(fun *schema.Function) bool {
	if fun.Inputs != nil {
		for _, p := range fun.Inputs.Properties {
			if p.IsRequired() {
				return false
			}
		}
	}
	return true
}

// Generates `function ${fn}Output(..)` version lifted to work on
// `Input`-warpped arguments and producing an `Output`-wrapped result.
func (mod *modContext) genFunctionOutputVersion(
	w io.Writer,
	fun *schema.Function,
	info functionFileInfo,
) (functionFileInfo, error) {
	if fun.ReturnType == nil {
		return info, nil
	}

	originalName := tokenToFunctionName(fun.Token)
	fnOutput := fmt.Sprintf("%sOutput", originalName)
	returnType := mod.functionReturnType(fun)
	info.functionOutputVersionName = fnOutput
	argTypeName := fmt.Sprintf("%sArgs", title(fnOutput))

	argsig := ""
	if fun.Inputs != nil && len(fun.Inputs.Properties) > 0 {
		argsOptional := functionArgsOptional(fun)
		optFlag := ""
		if argsOptional {
			optFlag = "?"
		}
		argsig = fmt.Sprintf("args%s: %s, ", optFlag, argTypeName)
	}

	// Write the TypeDoc/JSDoc for the data source function.
	printComment(w, codegen.FilterExamples(fun.Comment, "typescript"), "", "")

	if fun.DeprecationMessage != "" {
		fmt.Fprintf(w, "/** @deprecated %s */\n", fun.DeprecationMessage)
	}
	if !fun.MultiArgumentInputs {
		if argsig != "" {
			fmt.Fprintf(w, `export function %s(%sopts?: pulumi.InvokeOptions): pulumi.Output<%s> {
    return pulumi.output(args).apply((a: any) => %s(a, opts))
}
`, fnOutput, argsig, returnType, originalName)
		} else {
			fmt.Fprintf(w, `export function %s(opts?: pulumi.InvokeOptions): pulumi.Output<%s> {
    return pulumi.output(%s(opts))
}
`, fnOutput, returnType, originalName)
		}
	} else {
		fmt.Fprintf(w, "export function %s(", fnOutput)
		var properties []*schema.Property
		if fun.Inputs != nil {
			properties = fun.Inputs.Properties
		}

		for _, prop := range properties {
			paramDeclaration := ""
			propertyType := &schema.InputType{ElementType: prop.Type}
			argumentType := mod.typeString(propertyType, true /* input */, nil)
			if prop.IsRequired() {
				paramDeclaration = fmt.Sprintf("%s: %s", prop.Name, argumentType)
			} else {
				paramDeclaration = fmt.Sprintf("%s?: %s", prop.Name, argumentType)
			}

			fmt.Fprintf(w, "%s, ", paramDeclaration)
		}

		fmt.Fprintf(w, "opts?: pulumi.InvokeOptions): pulumi.Output<%s> {\n", returnType)
		fmt.Fprint(w, "    var args = {\n")
		for _, p := range properties {
			fmt.Fprintf(w, "        \"%s\": %s,\n", p.Name, p.Name)
		}
		fmt.Fprint(w, "    };\n")
		fmt.Fprintf(w, "    return pulumi.output(args).apply((resolvedArgs: any) => %s(", originalName)
		for _, p := range properties {
			// Pass the argument to the invocation.
			fmt.Fprintf(w, "resolvedArgs.%s, ", p.Name)
		}
		fmt.Fprint(w, "opts))\n")
		fmt.Fprint(w, "}\n")
	}

	if !fun.MultiArgumentInputs && fun.Inputs != nil && len(fun.Inputs.Properties) > 0 {
		fmt.Fprintf(w, "\n")
		info.functionOutputVersionArgsInterfaceName = argTypeName
		if err := mod.genPlainType(w,
			argTypeName,
			fun.Inputs.Comment,
			fun.Inputs.InputShape.Properties,
			true,  /* input */
			false, /* readonly */
			0 /* level */); err != nil {
			return info, err
		}
	}

	return info, nil
}

func visitObjectTypes(properties []*schema.Property, visitor func(*schema.ObjectType)) {
	codegen.VisitTypeClosure(properties, func(t schema.Type) {
		if o, ok := t.(*schema.ObjectType); ok {
			visitor(o)
		}
	})
}

func (mod *modContext) genType(w io.Writer, obj *schema.ObjectType, input bool, level int) error {
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
				newp := *p
				if required.Has(p.Name) {
					newp.Type = codegen.RequiredType(&newp)
				} else {
					newp.Type = codegen.OptionalType(&newp)
				}
				properties[i] = &newp
			}
		}
	}

	name := mod.getObjectName(obj, input)
	err := mod.genPlainType(w, name, obj.Comment, properties, input, false, level)
	if err != nil {
		return err
	}
	return mod.genPlainObjectDefaultFunc(w, name, properties, input, false, level)
}

// getObjectName recovers the name of `obj` as a type.
func (mod *modContext) getObjectName(obj *schema.ObjectType, input bool) string {
	name := tokenToName(obj.Token)

	details := mod.details(obj)

	if obj.IsInputShape() && input && details != nil && details.usedInFunctionOutputVersionInputs {
		name += "Args"
	} else if obj.IsInputShape() && mod.compatibility != tfbridge20 && mod.compatibility != kubernetes20 {
		name += "Args"
	}
	return name
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
			contract.AssertNoErrorf(err, "cannot make %q relative to %q", modName, mod.mod)
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
	def, err := mod.pkg.Definition()
	contract.AssertNoErrorf(err, "error loading definition for package %v", mod.pkg.Name())
	if languageInfo, hasLanguageInfo := def.Language["nodejs"]; hasLanguageInfo {
		nodePackageInfo = languageInfo.(NodePackageInfo)
	}

	writeImports := func(pkg string) {
		if imp, ok := nodePackageInfo.ProviderNameToModuleName[pkg]; ok {
			externalImports.Add(fmt.Sprintf("import * as %s from \"%s\";", externalModuleName(pkg), imp))
		} else {
			externalImports.Add(fmt.Sprintf("import * as %s from \"@pulumi/%s\";", externalModuleName(pkg), pkg))
		}
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
		// If the enum is from another package, add an import for the external package.
		if t.PackageReference != nil && !codegen.PkgEquals(t.PackageReference, mod.pkg) {
			pkg := t.PackageReference.Name()
			writeImports(pkg)
			return false
		}
		return true
	case *schema.ObjectType:
		// If it's from another package, add an import for the external package.
		if t.PackageReference != nil && !codegen.PkgEquals(t.PackageReference, mod.pkg) {
			pkg := t.PackageReference.Name()
			writeImports(pkg)
			return false
		}

		for _, p := range t.Properties {
			mod.getTypeImports(p.Type, recurse, externalImports, imports, seen)
		}
		return true
	case *schema.ResourceType:
		// If it's from another package, add an import for the external package.
		if t.Resource != nil && !codegen.PkgEquals(t.Resource.PackageReference, mod.pkg) {
			pkg := t.Resource.PackageReference.Name()
			writeImports(pkg)
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
					if p.Name == "__self__" {
						continue
					}
					needsTypes = mod.getTypeImportsForResource(p.Type, false, externalImports, imports, seen, res) || needsTypes
				}
			}

			if method.Function.ReturnType != nil {
				if objectType, ok := method.Function.ReturnType.(*schema.ObjectType); ok && objectType != nil {
					for _, p := range objectType.Properties {
						needsTypes = mod.getTypeImportsForResource(p.Type, false, externalImports, imports, seen, res) || needsTypes
					}
				} else if method.Function.ReturnTypePlain {
					needsTypes = mod.getTypeImportsForResource(
						method.Function.ReturnType, false, externalImports,
						imports, seen, res) || needsTypes
				}
			}
		}
		return needsTypes
	case *schema.Function:
		needsTypes := false
		if member.Inputs != nil {
			for _, p := range member.Inputs.Properties {
				needsTypes = mod.getTypeImports(p.Type, false, externalImports, imports, seen) || needsTypes
			}
		}
		if member.ReturnType != nil {
			// for object return types that are defined inline,
			// look through the properties to see if any of them need imports
			if objectType, ok := member.ReturnType.(*schema.ObjectType); ok && member.InlineObjectAsReturnType {
				for _, p := range objectType.Properties {
					needsTypes = mod.getTypeImports(p.Type, false, externalImports, imports, seen) || needsTypes
				}
			} else {
				// all other cases mean we have a more generic type like a reference to other types
				needsTypes = mod.getTypeImports(member.ReturnType, false, externalImports, imports, seen) || needsTypes
			}
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
	fmt.Fprintf(w, "const __config = new pulumi.Config(\"%v\");\n", mod.pkg.Name())
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
	contract.AssertNoErrorf(err, "could not turn %q into a relative path", mod.mod)
	return path.Dir(filepath.ToSlash(rel))
}

func (mod *modContext) sdkImports(nested, utilities bool) []string {
	imports := []string{"import * as pulumi from \"@pulumi/pulumi\";"}

	relRoot := mod.getRelativePath()
	if nested {
		imports = append(imports, []string{
			fmt.Sprintf(`import * as inputs from "%s/types/input";`, relRoot),
			fmt.Sprintf(`import * as outputs from "%s/types/output";`, relRoot),
		}...)

		def, err := mod.pkg.Definition()
		contract.AssertNoErrorf(err, "error loading package definition for %q", mod.pkg.Name())
		if def.Language["nodejs"].(NodePackageInfo).ContainsEnums {
			code := `import * as enums from "%s/types/enums";`
			if lookupNodePackageInfo(def).UseTypeOnlyReferences {
				code = `import type * as enums from "%s/types/enums";`
			}
			imports = append(imports, fmt.Sprintf(code, relRoot))
		}
	}

	if utilities {
		imports = append(imports, mod.utilitiesImport())
	}

	return imports
}

func (mod *modContext) utilitiesImport() string {
	relRoot := mod.getRelativePath()
	return fmt.Sprintf("import * as utilities from \"%s/utilities\";", relRoot)
}

func (mod *modContext) genTypes() (string, string, error) {
	externalImports, imports := codegen.NewStringSet(), map[string]codegen.StringSet{}
	var hasDefaultObjects bool
	for _, t := range mod.types {
		if t.IsOverlay {
			// This type is generated by the provider, so no further action is required.
			continue
		}

		mod.getImports(t, externalImports, imports)
		if codegen.IsProvideDefaultsFuncRequired(t) {
			hasDefaultObjects = true
		}
	}
	// Instantiating the default might require an environmental variable. This
	// uses utilities.
	if hasDefaultObjects {
		externalImports.Add(fmt.Sprintf("import * as utilities from \"%s/utilities\";", mod.getRelativePath()))
	}

	inputs, outputs := &bytes.Buffer{}, &bytes.Buffer{}
	mod.genHeader(inputs, mod.sdkImports(true, false), externalImports, imports)
	mod.genHeader(outputs, mod.sdkImports(true, false), externalImports, imports)

	// Build a namespace tree out of the types, then emit them.
	namespaces := mod.getNamespaces()
	if err := mod.genNamespace(inputs, namespaces[""], true, 0); err != nil {
		return "", "", err
	}
	if err := mod.genNamespace(outputs, namespaces[""], false, 0); err != nil {
		return "", "", err
	}

	return inputs.String(), outputs.String(), nil
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
		if t.IsOverlay {
			// This type is generated by the provider, so no further action is required.
			continue
		}

		modName := mod.pkg.TokenToModule(t.Token)
		if override, ok := mod.modToPkg[modName]; ok {
			modName = override
		}
		ns := getNamespace(modName)
		ns.types = append(ns.types, t)
	}

	return namespaces
}

func (mod *modContext) genNamespace(w io.Writer, ns *namespace, input bool, level int) error {
	indent := strings.Repeat("    ", level)

	// We generate the input and output namespaces when there are enums, regardless of if
	// they are empty.
	if ns == nil {
		return nil
	}

	sort.Slice(ns.types, func(i, j int) bool {
		return objectTypeLessThan(ns.types[i], ns.types[j])
	})
	sort.Slice(ns.enums, func(i, j int) bool {
		return tokenToName(ns.enums[i].Token) < tokenToName(ns.enums[j].Token)
	})
	for i, t := range ns.types {
		if input && mod.details(t).inputType || !input && mod.details(t).outputType {
			if err := mod.genType(w, t, input, level); err != nil {
				return err
			}
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
		if err := mod.genNamespace(w, child, input, level+1); err != nil {
			return err
		}
		fmt.Fprintf(w, "%s}\n", indent)
		if i != len(ns.children)-1 {
			fmt.Fprintf(w, "\n")
		}
	}
	return nil
}

func enumMemberName(typeName string, member *schema.Enum) (string, error) {
	if member.Name == "" {
		member.Name = fmt.Sprintf("%v", member.Value)
	}
	return makeSafeEnumName(member.Name, typeName)
}

func (mod *modContext) genEnum(w io.Writer, enum *schema.EnumType) error {
	indent := "    "
	enumName := tokenToName(enum.Token)
	fmt.Fprintf(w, "export const %s = {\n", enumName)
	for _, e := range enum.Elements {
		// If the enum doesn't have a name, set the value as the name.
		safeName, err := enumMemberName(enumName, e)
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

func (mod *modContext) isReservedSourceFileName(name string) bool {
	switch name {
	case "index.ts":
		return true
	case "input.ts", "output.ts":
		return len(mod.types) != 0
	case "utilities.ts":
		return mod.mod == ""
	case "vars.ts":
		config, err := mod.pkg.Config()
		contract.AssertNoErrorf(err, "failed to get config for package %q", mod.pkg.Name())
		return len(config) > 0
	default:
		return false
	}
}

func (mod *modContext) gen(fs codegen.Fs) error {
	files := slice.Prealloc[fileInfo](len(mod.extraSourceFiles))
	for _, path := range mod.extraSourceFiles {
		files = append(files, fileInfo{
			fileType:         otherFileType,
			pathToNodeModule: path,
		})
	}

	modDir := strings.ToLower(mod.mod)

	addFile := func(fileType fileType, name, contents string) {
		p := path.Join(modDir, name)
		files = append(files, fileInfo{
			fileType:         fileType,
			pathToNodeModule: p,
		})
		fs.Add(p, []byte(contents))
	}

	addResourceFile := func(resourceFileInfo resourceFileInfo, name, contents string) {
		p := path.Join(modDir, name)
		files = append(files, fileInfo{
			fileType:         resourceFileType,
			resourceFileInfo: resourceFileInfo,
			pathToNodeModule: p,
		})
		fs.Add(p, []byte(contents))
	}

	addFunctionFile := func(info functionFileInfo, name, contents string) {
		p := path.Join(modDir, name)
		files = append(files, fileInfo{
			fileType:         functionFileType,
			functionFileInfo: info,
			pathToNodeModule: p,
		})
		fs.Add(p, []byte(contents))
	}

	def, err := mod.pkg.Definition()
	if err != nil {
		return err
	}

	// Utilities, config, readme
	switch mod.mod {
	case "":
		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, nil, nil, nil)
		err := mod.genUtilitiesFile(buffer)
		if err != nil {
			return err
		}
		fs.Add(path.Join(modDir, "utilities.ts"), buffer.Bytes())

		// Ensure that the top-level (provider) module directory contains a README.md file.
		readme := def.Language["nodejs"].(NodePackageInfo).Readme
		if readme == "" {
			readme = def.Description
			if readme != "" && readme[len(readme)-1] != '\n' {
				readme += "\n"
			}
			if def.Attribution != "" {
				if len(readme) != 0 {
					readme += "\n"
				}
				readme += def.Attribution
			}
		}
		if readme != "" && readme[len(readme)-1] != '\n' {
			readme += "\n"
		}
		fs.Add(path.Join(modDir, "README.md"), []byte(readme))
	case "config":
		if len(def.Config) > 0 {
			buffer := &bytes.Buffer{}
			if err := mod.genConfig(buffer, def.Config); err != nil {
				return err
			}
			addFile(otherFileType, "vars.ts", buffer.String())
		}
	}

	// Resources
	for _, r := range mod.resources {
		if r.IsOverlay {
			// This resource code is generated by the provider, so no further action is required.
			continue
		}

		externalImports, imports := codegen.NewStringSet(), map[string]codegen.StringSet{}
		referencesNestedTypes := mod.getImportsForResource(r, externalImports, imports, r)

		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, mod.sdkImports(referencesNestedTypes, true), externalImports, imports)

		rinfo, err := mod.genResource(buffer, r)
		if err != nil {
			return err
		}

		fileName := mod.resourceFileName(r)
		addResourceFile(rinfo, fileName, buffer.String())
	}

	// Functions
	for _, f := range mod.functions {
		if f.IsOverlay {
			// This function code is generated by the provider, so no further action is required.
			continue
		}

		externalImports, imports := codegen.NewStringSet(), map[string]codegen.StringSet{}
		referencesNestedTypes := mod.getImports(f, externalImports, imports)

		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, mod.sdkImports(referencesNestedTypes, true), externalImports, imports)

		funInfo, err := mod.genFunction(buffer, f)
		if err != nil {
			return err
		}

		fileName := camel(tokenToName(f.Token)) + ".ts"
		if mod.isReservedSourceFileName(fileName) {
			fileName = camel(tokenToName(f.Token)) + "_.ts"
		}
		addFunctionFile(funInfo, fileName, buffer.String())
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
		fs.Add(fileName, buffer.Bytes())
	}

	// Nested types
	// Importing enums always imports inputs and outputs, so if we have enums we generate inputs and outputs
	if len(mod.types) > 0 || (def.Language["nodejs"].(NodePackageInfo).ContainsEnums && mod.mod == "types") {
		input, output, err := mod.genTypes()
		if err != nil {
			return err
		}
		fs.Add(path.Join(modDir, "input.ts"), []byte(input))
		fs.Add(path.Join(modDir, "output.ts"), []byte(output))
	}

	// Index
	fs.Add(path.Join(modDir, "index.ts"), []byte(mod.genIndex(files)))
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
func (mod *modContext) genIndex(exports []fileInfo) string {
	children := codegen.NewStringSet()

	for _, mod := range mod.children {
		child := getChildMod(mod.mod)
		children.Add(child)
	}

	if len(mod.types) > 0 {
		children.Add("input")
		children.Add("output")
	}

	w := &bytes.Buffer{}

	var imports []string
	// Include the SDK import if we'll be registering module resources.
	if len(mod.resources) != 0 {
		imports = mod.sdkImports(false /*nested*/, true /*utilities*/)
	} else if len(children) > 0 || len(mod.functions) > 0 {
		// Even if there are no resources, exports ref utilities.
		imports = append(imports, mod.utilitiesImport())
	}
	mod.genHeader(w, imports, nil, nil)

	// Export anything flatly that is a direct export rather than sub-module.
	if len(exports) > 0 {
		fmt.Fprintf(w, "// Export members:\n")
		sort.SliceStable(exports, func(i, j int) bool {
			return exports[i].pathToNodeModule < exports[j].pathToNodeModule
		})

		ll := newLazyLoadGen()
		modDir := strings.ToLower(mod.mod)
		for _, exp := range exports {
			rel, err := filepath.Rel(modDir, exp.pathToNodeModule)
			contract.AssertNoErrorf(err, "cannot make %q relative to %q", exp.pathToNodeModule, modDir)
			if path.Base(rel) == "." {
				rel = path.Dir(rel)
			}
			importPath := fmt.Sprintf(`./%s`, strings.TrimSuffix(rel, ".ts"))
			ll.genReexport(w, exp, importPath)
		}
	}

	def, err := mod.pkg.Definition()
	contract.AssertNoErrorf(err, "error loading package definition for %q", mod.pkg.Name())

	info, _ := def.Language["nodejs"].(NodePackageInfo)
	if info.ContainsEnums {
		if mod.mod == "types" {
			children.Add("enums")
			// input & output might be empty, but they will be imported with enums, so we
			// need to have them.
			children.Add("input")
			children.Add("output")
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
	contract.Assertf(len(mod.resources) != 0, "module %v has no resources", mod.mod)

	// Check for provider-only modules.
	var provider *schema.Resource
	if providerOnly := len(mod.resources) == 1 && mod.resources[0].IsProvider; providerOnly {
		provider = mod.resources[0]
	} else {
		registrations := codegen.StringSet{}
		for _, r := range mod.resources {
			if r.IsOverlay {
				// This resource code is generated by the provider, so no further action is required.
				continue
			}

			if r.IsProvider {
				contract.Assertf(provider == nil, "module %v has multiple providers", mod.mod)
				provider = r
				continue
			}

			registrations.Add(schema.TokenToRuntimeModule(r.Token))
		}

		fmt.Fprintf(w, "\nconst _module = {\n")
		fmt.Fprintf(w, "    version: utilities.getVersion(),\n")
		fmt.Fprintf(w, "    construct: (name: string, type: string, urn: string): pulumi.Resource => {\n")
		fmt.Fprintf(w, "        switch (type) {\n")

		for _, r := range mod.resources {
			if r.IsOverlay {
				// This resource code is generated by the provider, so no further action is required.
				continue
			}

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
			fmt.Fprintf(w, "pulumi.runtime.registerResourceModule(\"%v\", \"%v\", _module)\n", mod.pkg.Name(), name)
		}
	}

	if provider != nil {
		fmt.Fprintf(w, "pulumi.runtime.registerResourcePackage(\"%v\", {\n", mod.pkg.Name())
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
	for _, mod := range mod.children {
		if mod.hasEnums() {
			return true
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
func genPackageMetadata(pkg *schema.Package, info NodePackageInfo, fs codegen.Fs) error {
	// The generator already emitted Pulumi.yaml, so that leaves three more files to write out:
	//     1) package.json: minimal NPM package metadata
	//     2) tsconfig.json: instructions for TypeScript compilation
	fs.Add("package.json", []byte(genNPMPackageMetadata(pkg, info)))
	fs.Add("tsconfig.json", []byte(genTypeScriptProjectFile(info, fs)))
	return nil
}

type npmPackage struct {
	Name             string                  `json:"name"`
	Version          string                  `json:"version"`
	Description      string                  `json:"description,omitempty"`
	Keywords         []string                `json:"keywords,omitempty"`
	Homepage         string                  `json:"homepage,omitempty"`
	Repository       string                  `json:"repository,omitempty"`
	License          string                  `json:"license,omitempty"`
	Scripts          map[string]string       `json:"scripts,omitempty"`
	Dependencies     map[string]string       `json:"dependencies,omitempty"`
	DevDependencies  map[string]string       `json:"devDependencies,omitempty"`
	PeerDependencies map[string]string       `json:"peerDependencies,omitempty"`
	Resolutions      map[string]string       `json:"resolutions,omitempty"`
	Pulumi           plugin.PulumiPluginJSON `json:"pulumi,omitempty"`
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
		devDependencies["typescript"] = MinimumTypescriptVersion
	}
	devDependencies["@types/node"] = MinimumNodeTypesVersion

	version := "${VERSION}"
	pluginVersion := ""
	if pkg.Version != nil && info.RespectSchemaVersion {
		version = pkg.Version.String()
		pluginVersion = version
	}

	// Create info that will get serialized into an NPM package.json.
	npminfo := npmPackage{
		Name:        packageName,
		Version:     version,
		Description: info.PackageDescription,
		Keywords:    pkg.Keywords,
		Homepage:    pkg.Homepage,
		Repository:  pkg.Repository,
		License:     pkg.License,
		Scripts: map[string]string{
			"build": "tsc",
		},
		DevDependencies: devDependencies,
		Pulumi: plugin.PulumiPluginJSON{
			Resource: true,
			Server:   pkg.PluginDownloadURL,
			Name:     pkg.Name,
			Version:  pluginVersion,
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
		if npminfo.Dependencies == nil {
			npminfo.Dependencies = make(map[string]string)
		}
		npminfo.Dependencies["@pulumi/pulumi"] = MinimumValidSDKVersion
	}

	// Now write out the serialized form.
	npmjson, err := json.MarshalIndent(npminfo, "", "    ")
	contract.AssertNoErrorf(err, "error serializing package.json")
	return string(npmjson) + "\n"
}

func genTypeScriptProjectFile(info NodePackageInfo, files codegen.Fs) string {
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

	tsFiles = append(tsFiles, info.ExtraTypeScriptFiles...)
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
				pkg:                          pkg.Reference(),
				mod:                          modName,
				tool:                         tool,
				compatibility:                info.Compatibility,
				modToPkg:                     info.ModuleToPackage,
				disableUnionOutputTypes:      info.DisableUnionOutputTypes,
				liftSingleValueMethodReturns: info.LiftSingleValueMethodReturns,
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
		if r.IsOverlay {
			// This resource code is generated by the provider, so no further action is required.
			return
		}

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

			if f.NeedsOutputVersion() {
				visitObjectTypes(f.Inputs.InputShape.Properties, func(t *schema.ObjectType) {
					for _, mod := range []*modContext{types, getModFromToken(t.Token)} {
						det := mod.details(t)
						det.inputType = true
						det.usedInFunctionOutputVersionInputs = true
					}
				})
			}
		}
		if f.ReturnType != nil {
			// special case where the return type is defined inline with the function
			if objectType, ok := f.ReturnType.(*schema.ObjectType); ok && f.InlineObjectAsReturnType {
				visitObjectTypes(objectType.Properties, func(t *schema.ObjectType) {
					types.details(t).outputType = true
				})
			} else {
				// otherwise, the return type is or has a reference to a type defined elsewhere
				codegen.VisitType(f.ReturnType, func(schemaType schema.Type) {
					if t, ok := schemaType.(*schema.ObjectType); ok {
						types.details(t).outputType = true
					}
				})
			}
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
			if !typ.IsOverlay {
				info.ContainsEnums = true
				mod := getModFromToken(typ.Token)
				mod.enums = append(mod.enums, typ)
			}
		default:
			continue
		}
	}
	if len(types.types) > 0 || info.ContainsEnums {
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
			if r.IsOverlay {
				// This resource code is generated by the provider, so no further action is required.
				continue
			}

			packagePath := strings.ReplaceAll(modName, "/", ".")
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

	files := codegen.Fs{}
	for p, f := range extraFiles {
		files.Add(p, f)
	}
	for _, mod := range modules {
		if err := mod.gen(files); err != nil {
			return nil, err
		}
	}

	// Finally emit the package metadata (NPM, TypeScript, and so on).
	if err = genPackageMetadata(pkg, info, files); err != nil {
		return nil, err
	}
	return files, nil
}

//go:embed utilities.ts
var utilitiesFile string

func (mod *modContext) genUtilitiesFile(w io.Writer) error {
	def, err := mod.pkg.Definition()
	if err != nil {
		return err
	}
	code := utilitiesFile

	if url := def.PluginDownloadURL; url != "" {
		code = strings.ReplaceAll(code, "/*pluginDownloadURL*/",
			fmt.Sprintf(", pluginDownloadURL: %q", url))
	} else {
		code = strings.ReplaceAll(code, "/*pluginDownloadURL*/", "")
	}

	_, err = fmt.Fprintf(w, "%s", code)
	return err
}

// Used to sort ObjectType values.
func objectTypeLessThan(a, b *schema.ObjectType) bool {
	switch strings.Compare(tokenToName(a.Token), tokenToName(b.Token)) {
	case -1:
		return true
	case 0:
		tIsInput := a.PlainShape != nil
		otherIsInput := b.PlainShape != nil
		if !tIsInput && otherIsInput {
			return true
		}
		return false
	default:
		return false
	}
}
