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
package docs

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v2/codegen/python"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
)

// functionDocArgs represents the args that a Function doc template needs.
type functionDocArgs struct {
	Header header

	ResourceName       string
	DeprecationMessage string
	Comment            string

	// FunctionArgs is map per language view of the parameters
	// in the Function.
	FunctionArgs map[string]string
	// FunctionResult is a map per language property types
	// that is returned as a result of calling a Function.
	FunctionResult map[string]propertyType

	// InputProperties is a map per language and the corresponding slice
	// of input properties accepted by the Function.
	InputProperties map[string][]property
	// InputProperties is a map per language and the corresponding slice
	// of output properties, which are properties of the FunctionResult type.
	OutputProperties map[string][]property

	// NestedTypes is a slice of the nested types used in the input and
	// output properties.
	NestedTypes []docNestedType
}

// getFunctionResourceInfo returns a map of per-language information about
// the resource being looked-up using a static "getter" function.
func (mod *modContext) getFunctionResourceInfo(resourceTypeName string) map[string]propertyType {
	resourceMap := make(map[string]propertyType)

	var resultTypeName string
	for _, lang := range supportedLanguages {
		docLangHelper := getLanguageDocHelper(lang)
		switch lang {
		case "nodejs":
			resultTypeName = docLangHelper.GetResourceFunctionResultName(resourceTypeName)
		case "go":
			resultTypeName = docLangHelper.GetResourceFunctionResultName(resourceTypeName)
		case "csharp":
			resultTypeName = docLangHelper.GetResourceFunctionResultName(resourceTypeName)
			if mod.mod == "" {
				resultTypeName = fmt.Sprintf("Pulumi.%s.%s", strings.Title(mod.pkg.Name), resultTypeName)
			} else {
				resultTypeName = fmt.Sprintf("Pulumi.%s.%s.%s", strings.Title(mod.pkg.Name), strings.Title(mod.mod), resultTypeName)
			}

		case "python":
			// Pulumi's Python language SDK does not have "types" yet, so we will skip it for now.
			continue
		default:
			panic(errors.Errorf("cannot generate function resource info for unhandled language %q", lang))
		}

		parts := strings.Split(resultTypeName, ".")
		displayName := parts[len(parts)-1]
		resourceMap[lang] = propertyType{
			Name:        resultTypeName,
			DisplayName: displayName,
			Link:        docLangHelper.GetDocLinkForResourceType(mod.pkg, mod.mod, resultTypeName),
		}
	}

	return resourceMap
}

func (mod *modContext) genFunctionTS(f *schema.Function, resourceName string) []formalParam {
	argsType := "Get" + resourceName + "Args"

	docLangHelper := getLanguageDocHelper("nodejs")
	var params []formalParam

	if f.Inputs != nil {
		params = append(params, formalParam{
			Name:         "args",
			OptionalFlag: "",
			Type: propertyType{
				Name: argsType,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg, mod.mod, argsType),
			},
		})
	}
	params = append(params, formalParam{
		Name:         "opts",
		OptionalFlag: "?",
		Type: propertyType{
			Name: "InvokeOptions",
			Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "InvokeOptions"),
		},
	})

	return params
}

func (mod *modContext) genFunctionGo(f *schema.Function, resourceName string) []formalParam {
	argsType := resourceName + "Args"
	if mod.mod == "" {
		argsType = "Get" + argsType
	} else {
		argsType = "Lookup" + argsType
	}

	docLangHelper := getLanguageDocHelper("go")
	params := []formalParam{
		{
			Name:         "ctx",
			OptionalFlag: "*",
			Type: propertyType{
				Name: "pulumi.Context",
				Link: "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v2/go/pulumi?tab=doc#Context",
			},
		},
	}

	if f.Inputs != nil {
		params = append(params, formalParam{
			Name:         "args",
			OptionalFlag: "",
			Type: propertyType{
				Name: argsType,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg, mod.mod, argsType),
			},
		})
	}

	params = append(params, formalParam{
		Name:         "opts",
		OptionalFlag: "...",
		Type: propertyType{
			Name: "pulumi.InvokeOption",
			Link: "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v2/go/pulumi?tab=doc#InvokeOption",
		},
	})
	return params
}

func (mod *modContext) genFunctionCS(f *schema.Function, resourceName string) []formalParam {
	argsType := "Get" + resourceName + "Args"
	argsSchemaType := &schema.ObjectType{
		Token: f.Token,
	}

	characteristics := propertyCharacteristics{
		input:    true,
		optional: false,
	}
	argLangType := mod.typeString(argsSchemaType, "csharp", characteristics, false /* insertWordBreaks */)
	// The args type for a resource isn't part of "Inputs" namespace, so remove the "Inputs"
	// namespace qualifier.
	argLangTypeName := strings.ReplaceAll(argLangType.Name, "Inputs.", "")

	docLangHelper := getLanguageDocHelper("csharp")
	var params []formalParam
	if f.Inputs != nil {
		params = append(params, formalParam{
			Name:         "args",
			OptionalFlag: "",
			DefaultValue: "",
			Type: propertyType{
				Name: argsType,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg, "", argLangTypeName),
			},
		})
	}

	params = append(params, formalParam{
		Name:         "opts",
		OptionalFlag: "?",
		DefaultValue: " = null",
		Type: propertyType{
			Name: "InvokeOptions",
			Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "Pulumi.InvokeOptions"),
		},
	})
	return params
}

func (mod *modContext) genFunctionPython(f *schema.Function, resourceName string) []formalParam {
	var params []formalParam

	// Some functions don't have any inputs other than the InvokeOptions.
	// For example, the `get_billing_service_account` function.
	if f.Inputs != nil {
		params = make([]formalParam, 0, len(f.Inputs.Properties))
		for _, prop := range f.Inputs.Properties {
			fArg := formalParam{
				Name:         python.PyName(prop.Name),
				DefaultValue: "=None",
			}
			params = append(params, fArg)
		}
	} else {
		params = make([]formalParam, 0, 1)
	}

	params = append(params, formalParam{
		Name:         "opts",
		DefaultValue: "=None",
	})

	return params
}

// genFunctionArgs generates the arguments string for a given Function that can be
// rendered directly into a template.
func (mod *modContext) genFunctionArgs(f *schema.Function, resourceName string) map[string]string {
	functionParams := make(map[string]string)

	for _, lang := range supportedLanguages {
		var (
			paramTemplate string
			params        []formalParam
		)
		b := &bytes.Buffer{}

		switch lang {
		case "nodejs":
			params = mod.genFunctionTS(f, resourceName)
			paramTemplate = "ts_formal_param"
		case "go":
			params = mod.genFunctionGo(f, resourceName)
			paramTemplate = "go_formal_param"
		case "csharp":
			params = mod.genFunctionCS(f, resourceName)
			paramTemplate = "csharp_formal_param"
		case "python":
			params = mod.genFunctionPython(f, resourceName)
			paramTemplate = "py_formal_param"
		}

		n := len(params)
		if n == 0 {
			continue
		}

		for i, p := range params {
			if err := templates.ExecuteTemplate(b, paramTemplate, p); err != nil {
				panic(err)
			}
			if i != n-1 {
				if err := templates.ExecuteTemplate(b, "param_separator", nil); err != nil {
					panic(err)
				}
			}
		}
		functionParams[lang] = b.String()
	}
	return functionParams
}

// genFunction is the main entrypoint for generating docs for a Function.
// Returns args type that can be used to execute the `function.tmpl` doc template.
func (mod *modContext) genFunction(f *schema.Function) functionDocArgs {
	name := tokenToName(f.Token)
	resourceName := strings.ReplaceAll(name, "Get", "")

	inputProps := make(map[string][]property)
	outputProps := make(map[string][]property)
	for _, lang := range supportedLanguages {
		if f.Inputs != nil {
			inputProps[lang] = mod.getProperties(f.Inputs.Properties, lang, true, false)
		}
		if f.Outputs != nil {
			outputProps[lang] = mod.getProperties(f.Outputs.Properties, lang, false, false)
		}
	}

	nestedTypes := mod.genNestedTypes(f, false /*resourceType*/)

	args := functionDocArgs{
		Header: header{
			Title: name,
		},

		ResourceName:   resourceName,
		FunctionArgs:   mod.genFunctionArgs(f, resourceName),
		FunctionResult: mod.getFunctionResourceInfo(resourceName),

		Comment:            f.Comment,
		DeprecationMessage: f.DeprecationMessage,

		InputProperties:  inputProps,
		OutputProperties: outputProps,

		NestedTypes: nestedTypes,
	}

	return args
}
