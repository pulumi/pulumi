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
package docs

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type methodDocArgs struct {
	Title string

	ResourceName string

	DeprecationMessage string
	Comment            string
	ExamplesSection    []exampleSection

	// MethodName is a map of the language and the method name in that language.
	MethodName map[string]string
	// MethodArgs is map per language view of the parameters
	// in the method.
	MethodArgs map[string]string
	// MethodResult is a map per language property types
	// that is returned as a result of calling a method.
	MethodResult map[string]propertyType

	// InputProperties is a map per language and the corresponding slice
	// of input properties accepted by the method.
	InputProperties map[string][]property
	// OutputProperties is a map per language and the corresponding slice
	// of output properties, which are properties of the MethodResult type.
	OutputProperties map[string][]property
}

func (mod *modContext) genMethods(r *schema.Resource) []methodDocArgs {
	methods := make([]methodDocArgs, 0, len(r.Methods))
	for _, m := range r.Methods {
		methods = append(methods, mod.genMethod(r, m))
	}
	return methods
}

func (mod *modContext) genMethod(r *schema.Resource, m *schema.Method) methodDocArgs {
	f := m.Function
	inputProps, outputProps := make(map[string][]property), make(map[string][]property)
	for _, lang := range supportedLanguages {
		if f.Inputs != nil {
			exclude := func(name string) bool {
				return name == "__self__"
			}
			props := mod.getPropertiesWithIDPrefixAndExclude(f.Inputs.Properties, lang, true, false, false,
				fmt.Sprintf("%s_arg_", m.Name), exclude)
			if len(props) > 0 {
				inputProps[lang] = props
			}
		}
		if f.Outputs != nil {
			outputProps[lang] = mod.getPropertiesWithIDPrefixAndExclude(f.Outputs.Properties, lang, false, false, false,
				fmt.Sprintf("%s_result_", m.Name), nil)
		}
	}

	// Generate the per-language map for the method name.
	methodNameMap := map[string]string{}
	for _, lang := range supportedLanguages {
		docHelper := getLanguageDocHelper(lang)
		methodNameMap[lang] = docHelper.GetMethodName(m)
	}

	docInfo := decomposeDocstring(f.Comment)
	args := methodDocArgs{
		Title: title(m.Name, ""),

		ResourceName: resourceName(r),

		MethodName:   methodNameMap,
		MethodArgs:   mod.genMethodArgs(r, m, methodNameMap),
		MethodResult: mod.getMethodResult(r, m),

		Comment:            docInfo.description,
		DeprecationMessage: f.DeprecationMessage,
		ExamplesSection:    docInfo.examples,

		InputProperties:  inputProps,
		OutputProperties: outputProps,
	}

	return args
}

func (mod *modContext) genMethodTS(f *schema.Function, resourceName, methodName string,
	optionalArgs bool) []formalParam {

	argsType := fmt.Sprintf("%s.%sArgs", resourceName, title(methodName, "nodejs"))

	var optionalFlag string
	if optionalArgs {
		optionalFlag = "?"
	}

	var params []formalParam
	if f.Inputs != nil {
		params = append(params, formalParam{
			Name:         "args",
			OptionalFlag: optionalFlag,
			Type: propertyType{
				Name: argsType,
			},
		})
	}
	return params
}

func (mod *modContext) genMethodGo(f *schema.Function, resourceName, methodName string,
	optionalArgs bool) []formalParam {

	argsType := fmt.Sprintf("%s%sArgs", resourceName, title(methodName, "go"))

	params := []formalParam{
		{
			Name:         "ctx",
			OptionalFlag: "*",
			Type: propertyType{
				Name: "Context",
				Link: "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi?tab=doc#Context",
			},
		},
	}

	var optionalFlag string
	if optionalArgs {
		optionalFlag = "*"
	}

	if f.Inputs != nil {
		params = append(params, formalParam{
			Name:         "args",
			OptionalFlag: optionalFlag,
			Type: propertyType{
				Name: argsType,
			},
		})
	}
	return params
}

func (mod *modContext) genMethodCS(f *schema.Function, resourceName, methodName string, optionalArgs bool) []formalParam {
	argsType := fmt.Sprintf("%s.%sArgs", resourceName, title(methodName, "csharp"))
	var params []formalParam
	if f.Inputs != nil {
		var optionalFlag string
		if optionalArgs {
			optionalFlag = "?"
		}
		params = append(params, formalParam{
			Name:         "args",
			OptionalFlag: optionalFlag,
			DefaultValue: "",
			Type: propertyType{
				Name: argsType,
			},
		})
	}
	return params
}

func (mod *modContext) genMethodPython(f *schema.Function) []formalParam {
	docLanguageHelper := getLanguageDocHelper("python")
	var params []formalParam

	params = append(params, formalParam{
		Name: "self",
	})

	if f.Inputs != nil {
		// Filter out the __self__ argument from the inputs.
		args := make([]*schema.Property, 0, len(f.Inputs.InputShape.Properties)-1)
		for _, arg := range f.Inputs.InputShape.Properties {
			if arg.Name == "__self__" {
				continue
			}
			args = append(args, arg)
		}
		// Sort required args first.
		sort.Slice(args, func(i, j int) bool {
			pi, pj := args[i], args[j]
			switch {
			case pi.IsRequired() != pj.IsRequired():
				return pi.IsRequired() && !pj.IsRequired()
			default:
				return pi.Name < pj.Name
			}
		})
		for _, arg := range args {
			typ := docLanguageHelper.GetLanguageTypeString(mod.pkg, mod.mod, arg.Type, true /*input*/)
			var defaultValue string
			if !arg.IsRequired() {
				defaultValue = " = None"
			}
			params = append(params, formalParam{
				Name:         python.PyName(arg.Name),
				DefaultValue: defaultValue,
				Type: propertyType{
					Name: typ,
				},
			})
		}
	}
	return params
}

// genMethodArgs generates the arguments string for a given method that can be
// rendered directly into a template. An empty string indicates no args.
func (mod *modContext) genMethodArgs(r *schema.Resource, m *schema.Method,
	methodNameMap map[string]string) map[string]string {

	f := m.Function

	functionParams := make(map[string]string)
	for _, lang := range supportedLanguages {
		var (
			paramTemplate string
			params        []formalParam
		)
		b := &bytes.Buffer{}

		paramSeparatorTemplate := "param_separator"
		ps := paramSeparator{}

		var hasArgs bool
		optionalArgs := true
		if f.Inputs != nil {
			for _, arg := range f.Inputs.InputShape.Properties {
				if arg.Name == "__self__" {
					continue
				}
				hasArgs = true
				if arg.IsRequired() {
					optionalArgs = false
				}
			}
		}

		if !hasArgs {
			functionParams[lang] = ""
			continue
		}

		switch lang {
		case "nodejs":
			params = mod.genMethodTS(f, resourceName(r), methodNameMap["nodejs"], optionalArgs)
			paramTemplate = "ts_formal_param"
		case "go":
			params = mod.genMethodGo(f, resourceName(r), methodNameMap["go"], optionalArgs)
			paramTemplate = "go_formal_param"
		case "csharp":
			params = mod.genMethodCS(f, resourceName(r), methodNameMap["csharp"], optionalArgs)
			paramTemplate = "csharp_formal_param"
		case "python":
			params = mod.genMethodPython(f)
			paramTemplate = "py_formal_param"
			paramSeparatorTemplate = "py_param_separator"

			docHelper := getLanguageDocHelper(lang)
			methodName := docHelper.GetMethodName(m)
			ps = paramSeparator{Indent: strings.Repeat(" ", len("def (")+len(methodName))}
		}

		n := len(params)
		if n == 0 {
			functionParams[lang] = ""
			continue
		}

		for i, p := range params {
			if err := templates.ExecuteTemplate(b, paramTemplate, p); err != nil {
				panic(err)
			}
			if i != n-1 {
				if err := templates.ExecuteTemplate(b, paramSeparatorTemplate, ps); err != nil {
					panic(err)
				}
			}
		}
		functionParams[lang] = b.String()
	}
	return functionParams
}

// getMethodResult returns a map of per-language information about the method result.
// An empty propertyType.Name indicates no result.
func (mod *modContext) getMethodResult(r *schema.Resource, m *schema.Method) map[string]propertyType {
	resourceMap := make(map[string]propertyType)

	var resultTypeName string
	for _, lang := range supportedLanguages {
		if m.Function.Outputs != nil && len(m.Function.Outputs.Properties) > 0 {
			resultTypeName = getLanguageDocHelper(lang).GetMethodResultName(r, m)
		}
		resourceMap[lang] = propertyType{
			Name: resultTypeName,
			Link: fmt.Sprintf("#method_%s_result", title(m.Name, "")),
		}
	}

	return resourceMap
}
