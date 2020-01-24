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
package gen

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/codegen/schema"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type stringSet map[string]struct{}

func newStringSet(s ...string) stringSet {
	ss := stringSet{}
	for _, s := range s {
		ss.add(s)
	}
	return ss
}

func (ss stringSet) add(s string) {
	ss[s] = struct{}{}
}

func (ss stringSet) has(s string) bool {
	_, ok := ss[s]
	return ok
}

type typeDetails struct {
	ptrElement   bool
	arrayElement bool
	mapElement   bool
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

type pkgContext struct {
	pkg           *schema.Package
	mod           string
	typeDetails   map[*schema.ObjectType]*typeDetails
	types         []*schema.ObjectType
	resources     []*schema.Resource
	functions     []*schema.Function
	names         stringSet
	functionNames map[*schema.Function]string
	needsUtils    bool
	tool          string
}

func (pkg *pkgContext) details(t *schema.ObjectType) *typeDetails {
	details, ok := pkg.typeDetails[t]
	if !ok {
		details = &typeDetails{}
		pkg.typeDetails[t] = details
	}
	return details
}

func (pkg *pkgContext) tokenToType(tok string) string {
	// token := pkg : module : member
	// module := path/to/module

	components := strings.Split(tok, ":")
	contract.Assert(len(components) == 3)

	mod, name := pkg.pkg.TokenToModule(tok), components[2]
	if mod == pkg.mod {
		name := title(name)
		if pkg.names.has(name) {
			name += "Type"
		}
		return name
	}
	return strings.Replace(mod, "/", "", -1) + "." + title(name)
}

func tokenToName(tok string) string {
	components := strings.Split(tok, ":")
	contract.Assert(len(components) == 3)
	return title(components[2])
}

func resourceName(r *schema.Resource) string {
	if r.IsProvider {
		return "Provider"
	}
	return tokenToName(r.Token)
}

func (pkg *pkgContext) plainType(t schema.Type, optional bool) string {
	var typ string
	switch t := t.(type) {
	case *schema.ArrayType:
		return "[]" + pkg.plainType(t.ElementType, false)
	case *schema.MapType:
		return "map[string]" + pkg.plainType(t.ElementType, false)
	case *schema.ObjectType:
		typ = pkg.tokenToType(t.Token)
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return pkg.plainType(t.UnderlyingType, optional)
		}
		typ = pkg.tokenToType(t.Token)
	case *schema.UnionType:
		// TODO(pdg): union types
		return "interface{}"
	default:
		switch t {
		case schema.BoolType:
			typ = "bool"
		case schema.IntType:
			typ = "int"
		case schema.NumberType:
			typ = "float64"
		case schema.StringType:
			typ = "string"
		case schema.ArchiveType:
			return "pulumi.Archive"
		case schema.AssetType:
			return "pulumi.AssetOrArchive"
		case schema.AnyType:
			return "interface{}"
		}
	}

	if optional {
		return "*" + typ
	}
	return typ
}

func (pkg *pkgContext) inputType(t schema.Type, optional bool) string {
	var typ string
	switch t := t.(type) {
	case *schema.ArrayType:
		en := pkg.inputType(t.ElementType, false)
		return strings.TrimSuffix(en, "Input") + "ArrayInput"
	case *schema.MapType:
		en := pkg.inputType(t.ElementType, false)
		return strings.TrimSuffix(en, "Input") + "MapInput"
	case *schema.ObjectType:
		typ = pkg.tokenToType(t.Token)
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return pkg.inputType(t.UnderlyingType, optional)
		}
		typ = pkg.tokenToType(t.Token)
	case *schema.UnionType:
		// TODO(pdg): union types
		return "pulumi.Input"
	default:
		switch t {
		case schema.BoolType:
			typ = "pulumi.Bool"
		case schema.IntType:
			typ = "pulumi.Int"
		case schema.NumberType:
			typ = "pulumi.Float64"
		case schema.StringType:
			typ = "pulumi.String"
		case schema.ArchiveType:
			return "pulumi.ArchiveInput"
		case schema.AssetType:
			return "pulumi.AssetOrArchiveInput"
		case schema.AnyType:
			return "pulumi.Input"
		}
	}

	if optional {
		return typ + "PtrInput"
	}
	return typ + "Input"
}

func (pkg *pkgContext) outputType(t schema.Type, optional bool) string {
	var typ string
	switch t := t.(type) {
	case *schema.ArrayType:
		en := strings.TrimSuffix(pkg.outputType(t.ElementType, false), "Output")
		if en == "pulumi.Any" {
			return "pulumi.ArrayOutput"
		}
		return en + "ArrayOutput"
	case *schema.MapType:
		en := strings.TrimSuffix(pkg.outputType(t.ElementType, false), "Output")
		if en == "pulumi.Any" {
			return "pulumi.MapOutput"
		}
		return en + "MapOutput"
	case *schema.ObjectType:
		typ = pkg.tokenToType(t.Token)
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return pkg.outputType(t.UnderlyingType, optional)
		}
		typ = pkg.tokenToType(t.Token)
	case *schema.UnionType:
		// TODO(pdg): union types
		return "pulumi.AnyOutput"
	default:
		switch t {
		case schema.BoolType:
			typ = "pulumi.Bool"
		case schema.IntType:
			typ = "pulumi.Int"
		case schema.NumberType:
			typ = "pulumi.Float64"
		case schema.StringType:
			typ = "pulumi.String"
		case schema.ArchiveType:
			return "pulumi.ArchiveOutput"
		case schema.AssetType:
			return "pulumi.AssetOrArchiveOutput"
		case schema.AnyType:
			return "pulumi.AnyOutput"
		}
	}

	if optional {
		return typ + "PtrOutput"
	}
	return typ + "Output"
}

func printComment(w io.Writer, comment string, indent bool) {
	lines := strings.Split(comment, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, l := range lines {
		if indent {
			fmt.Fprintf(w, "\t")
		}
		fmt.Fprintf(w, "// %s\n", l)
	}
}

func genInputInterface(w io.Writer, name string) {
	fmt.Fprintf(w, "type %sInput interface {\n", name)
	fmt.Fprintf(w, "\tpulumi.Input\n\n")
	fmt.Fprintf(w, "\tTo%sOutput() %sOutput\n", title(name), name)
	fmt.Fprintf(w, "\tTo%sOutputWithContext(context.Context) %sOutput\n", title(name), name)
	fmt.Fprintf(w, "}\n\n")
}

func genInputMethods(w io.Writer, name, receiverType, elementType string, ptrMethods bool) {
	fmt.Fprintf(w, "func (%s) ElementType() reflect.Type {\n", receiverType)
	fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s)(nil)).Elem()\n", elementType)
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (i %s) To%sOutput() %sOutput {\n", receiverType, title(name), name)
	fmt.Fprintf(w, "\treturn i.To%sOutputWithContext(context.Background())\n", title(name))
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (i %s) To%sOutputWithContext(ctx context.Context) %sOutput {\n", receiverType, title(name), name)
	fmt.Fprintf(w, "\treturn pulumi.ToOutputWithContext(ctx, i).(%sOutput)\n", name)
	fmt.Fprintf(w, "}\n\n")

	if ptrMethods {
		fmt.Fprintf(w, "func (i %s) To%sPtrOutput() %sPtrOutput {\n", receiverType, title(name), name)
		fmt.Fprintf(w, "\treturn i.To%sPtrOutputWithContext(context.Background())\n", title(name))
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (i %s) To%sPtrOutputWithContext(ctx context.Context) %sPtrOutput {\n", receiverType, title(name), name)
		fmt.Fprintf(w, "\treturn pulumi.ToOutputWithContext(ctx, i).(%[1]sOutput).To%[1]sPtrOutputWithContext(ctx)\n", name)
		fmt.Fprintf(w, "}\n\n")
	}
}

func (pkg *pkgContext) genPlainType(w io.Writer, name, comment string, properties []*schema.Property) {
	printComment(w, comment, false)
	fmt.Fprintf(w, "type %s struct {\n", name)
	for _, p := range properties {
		printComment(w, p.Comment, true)
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", title(p.Name), pkg.plainType(p.Type, !p.IsRequired), p.Name)
	}
	fmt.Fprintf(w, "}\n\n")
}

func (pkg *pkgContext) genInputTypes(w io.Writer, t *schema.ObjectType, details *typeDetails) {
	name := pkg.tokenToType(t.Token)

	// Generate the plain inputs.
	genInputInterface(w, name)

	printComment(w, t.Comment, false)
	fmt.Fprintf(w, "type %sArgs struct {\n", name)
	for _, p := range t.Properties {
		printComment(w, p.Comment, true)
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", title(p.Name), pkg.inputType(p.Type, !p.IsRequired), p.Name)
	}
	fmt.Fprintf(w, "}\n\n")

	genInputMethods(w, name, name+"Args", name, details.ptrElement)

	// Generate the pointer input.
	if details.ptrElement {
		genInputInterface(w, name+"Ptr")

		ptrTypeName := camel(name) + "PtrType"

		fmt.Fprintf(w, "type %s %sArgs\n\n", ptrTypeName, name)

		fmt.Fprintf(w, "func %[1]sPtr(v *%[1]sArgs) %[1]sPtrInput {", name)
		fmt.Fprintf(w, "\treturn (*%s)(v)\n", ptrTypeName)
		fmt.Fprintf(w, "}\n\n")

		genInputMethods(w, name+"Ptr", "*"+ptrTypeName, "*"+name, false)
	}

	// Generate the array input.
	if details.arrayElement {
		genInputInterface(w, name+"Array")

		fmt.Fprintf(w, "type %[1]sArray []%[1]sInput\n\n", name)

		genInputMethods(w, name+"Array", name+"Array", "[]"+name, false)
	}

	// Generate the map input.
	if details.mapElement {
		genInputInterface(w, name+"Map")

		fmt.Fprintf(w, "type %[1]sMap map[string]%[1]sInput\n\n", name)

		genInputMethods(w, name+"Map", name+"Map", "map[string]"+name, false)
	}
}

func genOutputMethods(w io.Writer, name, elementType string) {
	fmt.Fprintf(w, "func (%sOutput) ElementType() reflect.Type {\n", name)
	fmt.Fprintf(w, "\treturn reflect.TypeOf((*%s)(nil)).Elem()\n", elementType)
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sOutput() %[1]sOutput {\n", name, title(name))
	fmt.Fprintf(w, "\treturn o\n")
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sOutputWithContext(ctx context.Context) %[1]sOutput {\n", name, title(name))
	fmt.Fprintf(w, "\treturn o\n")
	fmt.Fprintf(w, "}\n\n")
}

func (pkg *pkgContext) genOutputTypes(w io.Writer, t *schema.ObjectType, details *typeDetails) {
	name := pkg.tokenToType(t.Token)

	printComment(w, t.Comment, false)
	fmt.Fprintf(w, "type %sOutput struct { *pulumi.OutputState }\n\n", name)

	genOutputMethods(w, name, name)

	if details.ptrElement {
		fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutput() %[1]sPtrOutput {\n", name, title(name))
		fmt.Fprintf(w, "\treturn o.To%sPtrOutputWithContext(context.Background())\n", title(name))
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (o %[1]sOutput) To%[2]sPtrOutputWithContext(ctx context.Context) %[1]sPtrOutput {\n", name, title(name))
		fmt.Fprintf(w, "\treturn o.ApplyT(func(v %[1]s) *%[1]s {\n", name)
		fmt.Fprintf(w, "\t\treturn &v\n")
		fmt.Fprintf(w, "\t}).(%sPtrOutput)\n", name)
		fmt.Fprintf(w, "}\n")
	}

	for _, p := range t.Properties {
		printComment(w, p.Comment, false)
		outputType, applyType := pkg.outputType(p.Type, !p.IsRequired), pkg.plainType(p.Type, !p.IsRequired)

		fmt.Fprintf(w, "func (o %sOutput) %s() %s {\n", name, title(p.Name), outputType)
		fmt.Fprintf(w, "\treturn o.ApplyT(func (v %s) %s { return v.%s }).(%s)\n", name, applyType, title(p.Name), outputType)
		fmt.Fprintf(w, "}\n\n")
	}

	if details.ptrElement {
		fmt.Fprintf(w, "type %sPtrOutput struct { *pulumi.OutputState}\n\n", name)

		genOutputMethods(w, name+"Ptr", "*"+name)

		fmt.Fprintf(w, "func (o %[1]sPtrOutput) Elem() %[1]sOutput {\n", name)
		fmt.Fprintf(w, "\treturn o.ApplyT(func (v *%[1]s) %[1]s { return *v }).(%[1]sOutput)\n", name)
		fmt.Fprintf(w, "}\n\n")

		for _, p := range t.Properties {
			printComment(w, p.Comment, false)
			outputType, applyType := pkg.outputType(p.Type, !p.IsRequired), pkg.plainType(p.Type, !p.IsRequired)

			fmt.Fprintf(w, "func (o %sPtrOutput) %s() %s {\n", name, title(p.Name), outputType)
			fmt.Fprintf(w, "\treturn o.ApplyT(func (v %s) %s { return v.%s }).(%s)\n", name, applyType, title(p.Name), outputType)
			fmt.Fprintf(w, "}\n\n")
		}
	}

	if details.arrayElement {
		fmt.Fprintf(w, "type %sArrayOutput struct { *pulumi.OutputState}\n\n", name)

		genOutputMethods(w, name+"Array", "[]"+name)

		fmt.Fprintf(w, "func (o %[1]sArrayOutput) Index(i pulumi.IntInput) %[1]sOutput {\n", name)
		fmt.Fprintf(w, "\treturn pulumi.All(o, i).ApplyT(func (vs []interface{}) %s {\n", name)
		fmt.Fprintf(w, "\t\treturn vs[0].([]%s)[vs[1].(int)]\n", name)
		fmt.Fprintf(w, "\t}).(%sOutput)\n", name)
		fmt.Fprintf(w, "}\n\n")
	}

	if details.mapElement {
		fmt.Fprintf(w, "type %sMapOutput struct { *pulumi.OutputState}\n\n", name)

		genOutputMethods(w, name+"Map", "map[string]"+name)

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

func (pkg *pkgContext) genResource(w io.Writer, r *schema.Resource) error {
	name := resourceName(r)

	printComment(w, r.Comment, false)
	fmt.Fprintf(w, "type %s struct {\n", name)

	if r.IsProvider {
		fmt.Fprintf(w, "\tpulumi.ProviderResourceState\n\n")
	} else {
		fmt.Fprintf(w, "\tpulumi.CustomResourceState\n\n")
	}
	for _, p := range r.Properties {
		printComment(w, p.Comment, true)
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", title(p.Name), pkg.outputType(p.Type, !p.IsRequired), p.Name)
	}
	fmt.Fprintf(w, "}\n\n")

	// Create a constructor function that registers a new instance of this resource.
	fmt.Fprintf(w, "// New%s registers a new resource with the given unique name, arguments, and options.\n", name)
	fmt.Fprintf(w, "func New%s(ctx *pulumi.Context,\n", name)
	fmt.Fprintf(w, "\tname string, args *%[1]sArgs, opts ...pulumi.ResourceOption) (*%[1]s, error) {\n", name)

	// Ensure required arguments are present.
	for _, p := range r.InputProperties {
		if p.IsRequired {
			fmt.Fprintf(w, "\tif args == nil || args.%s == nil {\n", title(p.Name))
			fmt.Fprintf(w, "\t\treturn nil, errors.New(\"missing required argument '%s'\")\n", title(p.Name))
			fmt.Fprintf(w, "\t}\n")
		}
	}

	// Produce the inputs.
	fmt.Fprintf(w, "\tif args == nil {\n")
	fmt.Fprintf(w, "\t\targs = &%sArgs{}\n", name)
	fmt.Fprintf(w, "\t}\n")
	for _, p := range r.InputProperties {
		if p.DefaultValue != nil {
			v, err := pkg.getDefaultValue(p.DefaultValue, p.Type)
			if err != nil {
				return err
			}

			t := strings.TrimSuffix(pkg.inputType(p.Type, !p.IsRequired), "Input")
			if t == "pulumi." {
				t = "pulumi.Any"
			}

			fmt.Fprintf(w, "\tif args.%s == nil {\n", title(p.Name))
			fmt.Fprintf(w, "\t\targs.%s = %s(%s)\n", title(p.Name), t, v)
			fmt.Fprintf(w, "\t}\n")
		}
	}

	// Finally make the call to registration.
	fmt.Fprintf(w, "\tvar resource %s\n", name)
	fmt.Fprintf(w, "\terr := ctx.RegisterResource(\"%s\", name, args, &resource, opts...)\n", r.Token)
	fmt.Fprintf(w, "\tif err != nil {\n")
	fmt.Fprintf(w, "\t\treturn nil, err\n")
	fmt.Fprintf(w, "\t}\n")
	fmt.Fprintf(w, "\treturn &resource, nil\n")
	fmt.Fprintf(w, "}\n\n")

	// Emit a factory function that reads existing instances of this resource.
	if !r.IsProvider {
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
		for _, p := range r.Properties {
			printComment(w, p.Comment, true)
			fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", title(p.Name), pkg.plainType(p.Type, true), p.Name)
		}
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "type %sState struct {\n", name)
		for _, p := range r.Properties {
			printComment(w, p.Comment, true)
			fmt.Fprintf(w, "\t%s %s\n", title(p.Name), pkg.inputType(p.Type, true))
		}
		fmt.Fprintf(w, "}\n\n")

		fmt.Fprintf(w, "func (%sState) ElementType() reflect.Type {\n", name)
		fmt.Fprintf(w, "\treturn reflect.TypeOf((*%sState)(nil)).Elem()\n", camel(name))
		fmt.Fprintf(w, "}\n\n")
	}

	// Emit the args types.
	fmt.Fprintf(w, "type %sArgs struct {\n", camel(name))
	for _, p := range r.InputProperties {
		printComment(w, p.Comment, true)
		fmt.Fprintf(w, "\t%s %s `pulumi:\"%s\"`\n", title(p.Name), pkg.plainType(p.Type, !p.IsRequired), p.Name)
	}
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "// The set of arguments for constructing a %s resource.\n", name)
	fmt.Fprintf(w, "type %sArgs struct {\n", name)
	for _, p := range r.InputProperties {
		printComment(w, p.Comment, true)
		fmt.Fprintf(w, "\t%s %s\n", title(p.Name), pkg.inputType(p.Type, !p.IsRequired))
	}
	fmt.Fprintf(w, "}\n\n")

	fmt.Fprintf(w, "func (%sArgs) ElementType() reflect.Type {\n", name)
	fmt.Fprintf(w, "\treturn reflect.TypeOf((*%sArgs)(nil)).Elem()\n", camel(name))
	fmt.Fprintf(w, "}\n\n")

	return nil
}

func (pkg *pkgContext) genFunction(w io.Writer, f *schema.Function) {
	// If the function starts with New or Get, it will conflict; so rename them.
	name := pkg.functionNames[f]

	printComment(w, f.Comment, false)

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
		pkg.genPlainType(w, fmt.Sprintf("%sArgs", name), f.Inputs.Comment, f.Inputs.Properties)
	}
	if f.Outputs != nil {
		fmt.Fprintf(w, "\n")
		pkg.genPlainType(w, fmt.Sprintf("%sResult", name), f.Outputs.Comment, f.Outputs.Properties)
	}
}

func (pkg *pkgContext) genType(w io.Writer, obj *schema.ObjectType) {
	pkg.genPlainType(w, pkg.tokenToType(obj.Token), obj.Comment, obj.Properties)
	pkg.genInputTypes(w, obj, pkg.details(obj))
	pkg.genOutputTypes(w, obj, pkg.details(obj))
}

func (pkg *pkgContext) genInitFn(w io.Writer, types []*schema.ObjectType) {
	fmt.Fprintf(w, "func init() {\n")
	for _, obj := range types {
		name, details := pkg.tokenToType(obj.Token), pkg.details(obj)

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
	fmt.Fprintf(w, "}\n")
}

func (pkg *pkgContext) getTypeImports(t schema.Type, imports stringSet) {
	switch t := t.(type) {
	case *schema.ArrayType:
		pkg.getTypeImports(t.ElementType, imports)
	case *schema.MapType:
		pkg.getTypeImports(t.ElementType, imports)
	case *schema.ObjectType:
		mod := pkg.pkg.TokenToModule(t.Token)
		if mod != pkg.mod {
			imports.add(path.Join(pkg.pkg.Repository, mod))
		}

		for _, p := range t.Properties {
			pkg.getTypeImports(p.Type, imports)
		}
	case *schema.UnionType:
		// TODO(pdg): union types
	}
}

func (pkg *pkgContext) getImports(member interface{}, imports stringSet) {
	switch member := member.(type) {
	case *schema.ObjectType:
		pkg.getTypeImports(member, imports)
	case *schema.Resource:
		for _, p := range member.Properties {
			pkg.getTypeImports(p.Type, imports)
		}
		for _, p := range member.InputProperties {
			pkg.getTypeImports(p.Type, imports)

			if p.IsRequired {
				imports.add("github.com/pkg/errors")
			}
		}
	case *schema.Function:
		if member.Inputs != nil {
			pkg.getTypeImports(member.Inputs, imports)
		}
		if member.Outputs != nil {
			pkg.getTypeImports(member.Outputs, imports)
		}
	case []*schema.Property:
		for _, p := range member {
			pkg.getTypeImports(p.Type, imports)
		}
	default:
		return
	}

	imports.add("github.com/pulumi/pulumi/sdk/go/pulumi")
}

func (pkg *pkgContext) genHeader(w io.Writer, goImports []string, importedPackages stringSet) {
	fmt.Fprintf(w, "// *** WARNING: this file was generated by %v. ***\n", pkg.tool)
	fmt.Fprintf(w, "// *** Do not edit by hand unless you're certain you know what you are doing! ***\n\n")

	var pkgName string
	if pkg.mod == "" {
		pkgName = pkg.pkg.Name
	} else {
		pkgName = path.Base(pkg.mod)
	}

	fmt.Fprintf(w, "// nolint: lll\n")
	fmt.Fprintf(w, "package %s\n\n", pkgName)

	var imports []string
	if len(importedPackages) > 0 {
		for k := range importedPackages {
			imports = append(imports, k)
		}
		sort.Strings(imports)
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
				fmt.Fprintf(w, "\t\"%s\"\n", i)
			}
		}
		fmt.Fprintf(w, ")\n\n")
	}
}

func (pkg *pkgContext) genConfig(w io.Writer, variables []*schema.Property) error {
	imports := newStringSet("github.com/pulumi/pulumi/sdk/go/pulumi/config")
	pkg.getImports(variables, imports)

	pkg.genHeader(w, nil, imports)

	for _, p := range variables {
		getfunc := "Get"

		var getType string
		var funcType string
		switch p.Type {
		case schema.BoolType:
			getType, funcType = "bool", "Bool"
		case schema.IntType:
			getType, funcType = "int", "Int"
		case schema.NumberType:
			getType, funcType = "float64", "Float64"
		default:
			getType, funcType = "string", ""
		}

		printComment(w, p.Comment, false)
		configKey := fmt.Sprintf("\"%s:%s\"", pkg.pkg.Name, camel(p.Name))

		fmt.Fprintf(w, "func Get%s(ctx *pulumi.Context) %s {\n", title(p.Name), getType)
		if p.DefaultValue != nil {
			defaultValue, err := pkg.getDefaultValue(p.DefaultValue, p.Type)
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

func GeneratePackage(tool string, pkg *schema.Package) (map[string][]byte, error) {
	// group resources, types, and functions into Go packages
	packages := map[string]*pkgContext{}
	getPkg := func(token string) *pkgContext {
		mod := pkg.TokenToModule(token)
		pack, ok := packages[mod]
		if !ok {
			pack = &pkgContext{
				pkg:           pkg,
				mod:           mod,
				typeDetails:   map[*schema.ObjectType]*typeDetails{},
				names:         stringSet{},
				functionNames: map[*schema.Function]string{},
				tool:          tool,
			}
			packages[mod] = pack
		}
		return pack
	}

	for _, t := range pkg.Types {
		switch t := t.(type) {
		case *schema.ArrayType:
			if obj, ok := t.ElementType.(*schema.ObjectType); ok {
				getPkg(obj.Token).details(obj).arrayElement = true
			}
		case *schema.MapType:
			if obj, ok := t.ElementType.(*schema.ObjectType); ok {
				getPkg(obj.Token).details(obj).mapElement = true
			}
		case *schema.ObjectType:
			pkg := getPkg(t.Token)
			pkg.types = append(pkg.types, t)

			for _, p := range t.Properties {
				if obj, ok := p.Type.(*schema.ObjectType); ok && !p.IsRequired {
					getPkg(obj.Token).details(obj).ptrElement = true
				}
			}
		}
	}

	scanResource := func(r *schema.Resource) {
		pkg := getPkg(r.Token)
		pkg.resources = append(pkg.resources, r)

		pkg.names.add(resourceName(r))
		pkg.names.add(resourceName(r) + "Args")
		pkg.names.add(camel(resourceName(r)) + "Args")
		pkg.names.add("New" + resourceName(r))
		if !r.IsProvider {
			pkg.names.add(resourceName(r) + "State")
			pkg.names.add(camel(resourceName(r)) + "State")
			pkg.names.add("Get" + resourceName(r))
		}

		for _, p := range r.InputProperties {
			if obj, ok := p.Type.(*schema.ObjectType); ok && (!r.IsProvider || !p.IsRequired) {
				getPkg(obj.Token).details(obj).ptrElement = true
			}
		}
		for _, p := range r.Properties {
			if obj, ok := p.Type.(*schema.ObjectType); ok && (!r.IsProvider || !p.IsRequired) {
				getPkg(obj.Token).details(obj).ptrElement = true
			}
		}
	}

	scanResource(pkg.Provider)
	for _, r := range pkg.Resources {
		scanResource(r)
	}

	for _, f := range pkg.Functions {
		pkg := getPkg(f.Token)
		pkg.functions = append(pkg.functions, f)

		name := tokenToName(f.Token)
		if pkg.names.has(name) {
			switch {
			case strings.HasPrefix(name, "New"):
				name = "Create" + name[3:]
			case strings.HasPrefix(name, "Get"):
				name = "Lookup" + name[3:]
			}
		}
		pkg.names.add(name)
		pkg.functionNames[f] = name

		if f.Inputs != nil {
			pkg.names.add(name + "Args")
		}
		if f.Outputs != nil {
			pkg.names.add(name + "Result")
		}
	}

	// emit each package
	var pkgMods []string
	for mod := range packages {
		pkgMods = append(pkgMods, mod)
	}
	sort.Strings(pkgMods)

	files := map[string][]byte{}
	setFile := func(relPath, contents string) {
		relPath = path.Join(pkg.Name, relPath)
		if _, ok := files[relPath]; ok {
			panic(errors.Errorf("duplicate file: %s", relPath))
		}
		files[relPath] = []byte(contents)
	}

	name := pkg.Name
	for _, mod := range pkgMods {
		pkg := packages[mod]

		// Config, description
		switch mod {
		case "":
			buffer := &bytes.Buffer{}
			fmt.Fprintf(buffer, "// Package %[1]s exports types, functions, subpackages for provisioning %[1]s resources.", pkg.pkg.Name)
			fmt.Fprintf(buffer, "//\n")
			if pkg.pkg.Description != "" {
				printComment(buffer, pkg.pkg.Description, false)
				fmt.Fprintf(buffer, "//\n")
			}
			fmt.Fprintf(buffer, "// nolint: lll\n")
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
			imports := stringSet{}
			pkg.getImports(r, imports)

			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, []string{"reflect"}, imports)

			if err := pkg.genResource(buffer, r); err != nil {
				return nil, err
			}

			setFile(path.Join(mod, camel(resourceName(r))+".go"), buffer.String())
		}

		// Functions
		for _, f := range pkg.functions {
			imports := stringSet{}
			pkg.getImports(f, imports)

			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, nil, imports)

			pkg.genFunction(buffer, f)

			setFile(path.Join(mod, camel(tokenToName(f.Token))+".go"), buffer.String())
		}

		// Types
		if len(pkg.types) > 0 {
			imports := stringSet{}
			for _, t := range pkg.types {
				pkg.getImports(t, imports)
			}

			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, []string{"context", "reflect"}, imports)

			for _, t := range pkg.types {
				pkg.genType(buffer, t)
			}

			pkg.genInitFn(buffer, pkg.types)

			setFile(path.Join(mod, "pulumiTypes.go"), buffer.String())
		}

		// Utilities
		if pkg.needsUtils {
			buffer := &bytes.Buffer{}
			pkg.genHeader(buffer, []string{"os", "strconv"}, nil)

			fmt.Fprintf(buffer, "%s", utilitiesFile)

			setFile(path.Join(mod, "pulumiUtilities.go"), buffer.String())
		}
	}

	return files, nil
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
`
