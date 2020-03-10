//go:generate go run bundler.go

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
	"github.com/pulumi/pulumi/pkg/codegen"
	"github.com/pulumi/pulumi/pkg/codegen/dotnet"
	go_gen "github.com/pulumi/pulumi/pkg/codegen/go"
	"github.com/pulumi/pulumi/pkg/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/codegen/python"
	"github.com/pulumi/pulumi/pkg/codegen/schema"
)

type functionDocArgs struct {
	Header

	ResourceName       string
	DeprecationMessage string
	Comment            string

	FunctionArgs   map[string]string
	FunctionResult map[string]PropertyType

	InputProperties  map[string][]Property
	OutputProperties map[string][]Property
}

// getFunctionResourceInfo returns a map of per-language information about
// the resource being looked-up using a static "getter" function.
func (mod *modContext) getFunctionResourceInfo(resourceTypeName string) map[string]PropertyType {
	resourceMap := make(map[string]PropertyType)

	var resultTypeName string
	for _, lang := range supportedLanguages {
		var docLangHelper codegen.DocLanguageHelper
		switch lang {
		case "nodejs":
			docLangHelper = nodejs.DocLanguageHelper{}
			resultTypeName = docLangHelper.GetResourceFunctionResultName(resourceTypeName)
		case "go":
			docLangHelper = go_gen.DocLanguageHelper{}
			resultTypeName = docLangHelper.GetResourceFunctionResultName(resourceTypeName)
		case "csharp":
			docLangHelper = dotnet.DocLanguageHelper{}
			resultTypeName = docLangHelper.GetResourceFunctionResultName(resourceTypeName)
			resultTypeName = fmt.Sprintf("Pulumi.%s.%s.%s", strings.Title(mod.pkg.Name), strings.Title(mod.mod), resultTypeName)
		case "python":
			// Pulumi's Python language SDK does not have "types" yet, so we will skip it for now.
			continue
		default:
			panic(errors.Errorf("cannot generate function resource info for unhandled language %q", lang))
		}

		resourceMap[lang] = PropertyType{
			Name: resultTypeName,
			Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, mod.mod, resultTypeName),
		}
	}

	return resourceMap
}

func (mod *modContext) genFunctionTS(f *schema.Function) []ConstructorParam {
	resourceName := tokenToName(f.Token)
	argsType := resourceName + "Args"

	docLangHelper := nodejs.DocLanguageHelper{}
	return []ConstructorParam{
		{
			Name:         "args",
			OptionalFlag: "",
			Type: PropertyType{
				Name: argsType,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, mod.mod, argsType),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			Type: PropertyType{
				Name: "pulumi.InvokeOptions",
				Link: docLangHelper.GetDocLinkForResourceType("pulumi", "pulumi", "InvokeOptions"),
			},
		},
	}
}

func (mod *modContext) genFunctionGo(f *schema.Function) []ConstructorParam {
	resourceName := tokenToName(f.Token)
	argsType := resourceName + "Args"

	docLangHelper := go_gen.DocLanguageHelper{}
	return []ConstructorParam{
		{
			Name:         "ctx",
			OptionalFlag: "*",
			Type: PropertyType{
				Name: "pulumi.Context",
				Link: "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/go/pulumi?tab=doc#Context",
			},
		},
		{
			Name:         "args",
			OptionalFlag: "",
			Type: PropertyType{
				Name: argsType,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, mod.mod, argsType),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "...",
			Type: PropertyType{
				Name: "pulumi.InvokeOption",
				Link: "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/go/pulumi?tab=doc#InvokeOption",
			},
		},
	}
}

func (mod *modContext) genFunctionCS(f *schema.Function) []ConstructorParam {
	resourceName := tokenToName(f.Token)
	argsType := resourceName + "Args"
	argsSchemaType := &schema.ObjectType{
		Token: f.Token,
	}
	argLangType := mod.typeString(argsSchemaType, "csharp", true /* input */, false /* optional */, false /* insertWordBreaks */)

	docLangHelper := dotnet.DocLanguageHelper{}
	return []ConstructorParam{
		{
			Name:         "args",
			OptionalFlag: "",
			DefaultValue: "",
			Type: PropertyType{
				Name: argsType,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, "", argLangType.Name),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			DefaultValue: " = null",
			Type: PropertyType{
				Name: "InvokeOptions",
				Link: docLangHelper.GetDocLinkForResourceType("", "", "InvokeOptions"),
			},
		},
	}
}

func (mod *modContext) genFunctionPython(f *schema.Function) []ConstructorParam {
	var params []ConstructorParam

	// Some functions don't have any inputs other than the InvokeOptions.
	// For example, the `get_billing_service_account` function.
	if f.Inputs != nil {
		params = make([]ConstructorParam, 0, len(f.Inputs.Properties))
		for _, prop := range f.Inputs.Properties {
			fArg := ConstructorParam{
				Name:         python.PyName(prop.Name),
				DefaultValue: "=None",
			}
			params = append(params, fArg)
		}
	} else {
		params = make([]ConstructorParam, 0, 1)
	}

	params = append(params, ConstructorParam{
		Name:         "opts",
		DefaultValue: "=None",
	})

	return params
}

// genFunctionArgs generates the arguments string for a given Function that can be
// rendered directly into a template.
func (mod *modContext) genFunctionArgs(f *schema.Function) map[string]string {
	functionParams := make(map[string]string)

	for _, lang := range supportedLanguages {
		var (
			paramTemplate string
			params        []ConstructorParam
		)
		b := &bytes.Buffer{}

		switch lang {
		case "nodejs":
			params = mod.genFunctionTS(f)
			paramTemplate = "ts_constructor_param"
		case "go":
			params = mod.genFunctionGo(f)
			paramTemplate = "go_constructor_param"
		case "csharp":
			params = mod.genFunctionCS(f)
			paramTemplate = "csharp_constructor_param"
		case "python":
			params = mod.genFunctionPython(f)
			paramTemplate = "py_function_param"
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

	inputProps := make(map[string][]Property)
	outputProps := make(map[string][]Property)
	for _, lang := range supportedLanguages {
		if f.Inputs != nil {
			inputProps[lang] = mod.getProperties(f.Inputs.Properties, lang, true)
		}
		if f.Outputs != nil {
			outputProps[lang] = mod.getProperties(f.Outputs.Properties, lang, false)
		}
	}

	args := functionDocArgs{
		Header: Header{
			Title: name,
		},

		ResourceName:   resourceName,
		FunctionArgs:   mod.genFunctionArgs(f),
		FunctionResult: mod.getFunctionResourceInfo(resourceName),

		Comment:            f.Comment,
		DeprecationMessage: f.DeprecationMessage,

		InputProperties:  inputProps,
		OutputProperties: outputProps,
	}

	return args
}
