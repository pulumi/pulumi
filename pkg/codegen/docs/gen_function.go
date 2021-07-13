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
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// functionDocArgs represents the args that a Function doc template needs.
type functionDocArgs struct {
	Header header

	Tool string

	DeprecationMessage string
	Comment            string
	ExamplesSection    []exampleSection

	// FunctionName is a map of the language and the function name in that language.
	FunctionName map[string]string
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

	PackageDetails packageDetails
}

// getFunctionResourceInfo returns a map of per-language information about
// the resource being looked-up using a static "getter" function.
func (mod *modContext) getFunctionResourceInfo(f *schema.Function) map[string]propertyType {
	resourceMap := make(map[string]propertyType)

	var resultTypeName string
	for _, lang := range supportedLanguages {
		docLangHelper := getLanguageDocHelper(lang)
		switch lang {
		case "nodejs":
			resultTypeName = docLangHelper.GetResourceFunctionResultName(mod.mod, f)
		case "go":
			resultTypeName = docLangHelper.GetResourceFunctionResultName(mod.mod, f)
		case "csharp":
			namespace := title(mod.pkg.Name, lang)
			if ns, ok := csharpPkgInfo.Namespaces[mod.pkg.Name]; ok {
				namespace = ns
			}
			resultTypeName = docLangHelper.GetResourceFunctionResultName(mod.mod, f)
			if mod.mod == "" {
				resultTypeName = fmt.Sprintf("Pulumi.%s.%s", namespace, resultTypeName)
			} else {
				resultTypeName = fmt.Sprintf("Pulumi.%s.%s.%s", namespace, title(mod.mod, lang), resultTypeName)
			}

		case "python":
			resultTypeName = docLangHelper.GetResourceFunctionResultName(mod.mod, f)
		default:
			panic(errors.Errorf("cannot generate function resource info for unhandled language %q", lang))
		}

		parts := strings.Split(resultTypeName, ".")
		displayName := parts[len(parts)-1]
		resourceMap[lang] = propertyType{
			Name:        resultTypeName,
			DisplayName: displayName,
			Link:        "#result",
		}
	}

	return resourceMap
}

func (mod *modContext) genFunctionTS(f *schema.Function, funcName string) []formalParam {
	argsType := title(funcName+"Args", "nodejs")
	docLangHelper := getLanguageDocHelper("nodejs")
	var params []formalParam
	if f.Inputs != nil {
		params = append(params, formalParam{
			Name:         "args",
			OptionalFlag: "",
			Type: propertyType{
				Name: argsType,
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

func (mod *modContext) genFunctionGo(f *schema.Function, funcName string) []formalParam {
	argsType := funcName + "Args"

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

	if f.Inputs != nil {
		params = append(params, formalParam{
			Name:         "args",
			OptionalFlag: "*",
			Type: propertyType{
				Name: argsType,
			},
		})
	}

	params = append(params, formalParam{
		Name:         "opts",
		OptionalFlag: "...",
		Type: propertyType{
			Name: "InvokeOption",
			Link: "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi?tab=doc#InvokeOption",
		},
	})
	return params
}

func (mod *modContext) genFunctionCS(f *schema.Function, funcName string) []formalParam {
	argsType := funcName + "Args"
	docLangHelper := getLanguageDocHelper("csharp")
	var params []formalParam
	if f.Inputs != nil {
		params = append(params, formalParam{
			Name:         "args",
			OptionalFlag: "",
			DefaultValue: "",
			Type: propertyType{
				Name: argsType,
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
	docLanguageHelper := getLanguageDocHelper("python")
	var params []formalParam

	// Some functions don't have any inputs other than the InvokeOptions.
	// For example, the `get_billing_service_account` function.
	if f.Inputs != nil {
		params = make([]formalParam, 0, len(f.Inputs.Properties))
		for _, prop := range f.Inputs.Properties {
			typ := docLanguageHelper.GetLanguageTypeString(mod.pkg, mod.mod, codegen.PlainType(codegen.OptionalType(prop)), true /*input*/)
			params = append(params, formalParam{
				Name:         python.PyName(prop.Name),
				DefaultValue: " = None",
				Type: propertyType{
					Name: typ,
				},
			})
		}
	} else {
		params = make([]formalParam, 0, 1)
	}

	params = append(params, formalParam{
		Name:         "opts",
		DefaultValue: " = None",
		Type: propertyType{
			Name: "Optional[InvokeOptions]",
			Link: "/docs/reference/pkg/python/pulumi/#pulumi.InvokeOptions",
		},
	})

	return params
}

// genFunctionArgs generates the arguments string for a given Function that can be
// rendered directly into a template.
func (mod *modContext) genFunctionArgs(f *schema.Function, funcNameMap map[string]string) map[string]string {
	functionParams := make(map[string]string)

	for _, lang := range supportedLanguages {
		var (
			paramTemplate string
			params        []formalParam
		)
		b := &bytes.Buffer{}

		paramSeparatorTemplate := "param_separator"
		ps := paramSeparator{}

		switch lang {
		case "nodejs":
			params = mod.genFunctionTS(f, funcNameMap["nodejs"])
			paramTemplate = "ts_formal_param"
		case "go":
			params = mod.genFunctionGo(f, funcNameMap["go"])
			paramTemplate = "go_formal_param"
		case "csharp":
			params = mod.genFunctionCS(f, funcNameMap["csharp"])
			paramTemplate = "csharp_formal_param"
		case "python":
			params = mod.genFunctionPython(f, funcNameMap["python"])
			paramTemplate = "py_formal_param"
			paramSeparatorTemplate = "py_param_separator"

			docHelper := getLanguageDocHelper(lang)
			funcName := docHelper.GetFunctionName(mod.mod, f)
			ps = paramSeparator{Indent: strings.Repeat(" ", len("def (")+len(funcName))}
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
				if err := templates.ExecuteTemplate(b, paramSeparatorTemplate, ps); err != nil {
					panic(err)
				}
			}
		}
		functionParams[lang] = b.String()
	}
	return functionParams
}

func (mod *modContext) genFunctionHeader(f *schema.Function) header {
	funcName := tokenToName(f.Token)
	var baseDescription string
	var titleTag string
	if mod.mod == "" {
		baseDescription = fmt.Sprintf("Documentation for the %s.%s function "+
			"with examples, input properties, output properties, "+
			"and supporting types.", mod.pkg.Name, funcName)
		titleTag = fmt.Sprintf("%s.%s", mod.pkg.Name, funcName)
	} else {
		baseDescription = fmt.Sprintf("Documentation for the %s.%s.%s function "+
			"with examples, input properties, output properties, "+
			"and supporting types.", mod.pkg.Name, mod.mod, funcName)
		titleTag = fmt.Sprintf("%s.%s.%s", mod.pkg.Name, mod.mod, funcName)
	}

	return header{
		Title:    funcName,
		TitleTag: titleTag,
		MetaDesc: baseDescription,
	}
}

// genFunction is the main entrypoint for generating docs for a Function.
// Returns args type that can be used to execute the `function.tmpl` doc template.
func (mod *modContext) genFunction(f *schema.Function) functionDocArgs {
	inputProps := make(map[string][]property)
	outputProps := make(map[string][]property)
	for _, lang := range supportedLanguages {
		if f.Inputs != nil {
			inputProps[lang] = mod.getProperties(f.Inputs.Properties, lang, true, false, false)
		}
		if f.Outputs != nil {
			outputProps[lang] = mod.getProperties(f.Outputs.Properties, lang, false, false, false)
		}
	}

	nestedTypes := mod.genNestedTypes(f, false /*resourceType*/)

	// Generate the per-language map for the function name.
	funcNameMap := map[string]string{}
	for _, lang := range supportedLanguages {
		docHelper := getLanguageDocHelper(lang)
		funcNameMap[lang] = docHelper.GetFunctionName(mod.mod, f)
	}

	packageDetails := packageDetails{
		Repository: mod.pkg.Repository,
		License:    mod.pkg.License,
		Notes:      mod.pkg.Attribution,
	}

	docInfo := decomposeDocstring(f.Comment)
	args := functionDocArgs{
		Header: mod.genFunctionHeader(f),

		Tool: mod.tool,

		FunctionName:   funcNameMap,
		FunctionArgs:   mod.genFunctionArgs(f, funcNameMap),
		FunctionResult: mod.getFunctionResourceInfo(f),

		Comment:            docInfo.description,
		DeprecationMessage: f.DeprecationMessage,
		ExamplesSection:    docInfo.examples,

		InputProperties:  inputProps,
		OutputProperties: outputProps,

		NestedTypes: nestedTypes,

		PackageDetails: packageDetails,
	}

	return args
}
