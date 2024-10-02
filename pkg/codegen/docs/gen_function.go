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
//nolint:lll, goconst
package docs

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	go_gen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// functionDocArgs represents the args that a Function doc template needs.
type functionDocArgs struct {
	Header header

	Tool string
	// LangChooserLanguages is a comma-separated list of languages to pass to the
	// language chooser shortcode. Use this to customize the languages shown for a
	// function. By default, the language chooser will show all languages supported
	// by Pulumi.
	// Supported values are "typescript", "python", "go", "csharp", "java", "yaml"
	LangChooserLanguages string

	DeprecationMessage string
	Comment            string
	ExamplesSection    examplesSection

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

	// Check if the function supports an `Output` version that is
	// automatically lifted to accept `Input` values and return an
	// `Output` (per language).
	HasOutputVersion map[string]bool

	// True if any of the entries in `HasOutputVersion` are true.
	AnyLanguageHasOutputVersion bool

	// Same as FunctionArgs, but specific to the Output version of
	// the function.
	FunctionArgsOutputVersion map[string]string

	// Same as FunctionResult, but specific to the Output version
	// of the function. In languages like Go, `Output<Result>`
	// gets a dedicated nominal type to emulate generics, which
	// will be passed in here.
	FunctionResultOutputVersion map[string]propertyType
}

// getFunctionResourceInfo returns a map of per-language information about
// the resource being looked-up using a static "getter" function.
func (mod *modContext) getFunctionResourceInfo(f *schema.Function, outputVersion bool) map[string]propertyType {
	dctx := mod.docGenContext
	resourceMap := make(map[string]propertyType)

	var resultTypeName string
	for _, lang := range dctx.supportedLanguages {
		docLangHelper := dctx.getLanguageDocHelper(lang)
		switch lang {
		case "nodejs":
			resultTypeName = docLangHelper.GetResourceFunctionResultName(mod.mod, f)
		case "go":
			resultTypeName = docLangHelper.GetResourceFunctionResultName(mod.mod, f)
			if outputVersion {
				resultTypeName = resultTypeName + "Output"
			}
		case "csharp":
			namespace := title(mod.pkg.Name(), lang)
			if ns, ok := dctx.csharpPkgInfo.Namespaces[mod.pkg.Name()]; ok {
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
		case "java":
			resultTypeName = docLangHelper.GetResourceFunctionResultName(mod.mod, f)
		case "yaml":
			resultTypeName = docLangHelper.GetResourceFunctionResultName(mod.mod, f)
		default:
			panic(fmt.Errorf("cannot generate function resource info for unhandled language %q", lang))
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

func (mod *modContext) genFunctionTS(f *schema.Function, funcName string, outputVersion bool) []formalParam {
	dctx := mod.docGenContext

	argsTypeSuffix := "Args"
	if outputVersion {
		argsTypeSuffix = "OutputArgs"
	}

	argsType := title(fmt.Sprintf("%s%s", funcName, argsTypeSuffix), "nodejs")

	docLangHelper := dctx.getLanguageDocHelper("nodejs")
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
	def, err := mod.pkg.Definition()
	contract.AssertNoErrorf(err, "failed to get definition for package %q", mod.pkg.Name())
	params = append(params, formalParam{
		Name:         "opts",
		OptionalFlag: "?",
		Type: propertyType{
			Name: "InvokeOptions",
			Link: docLangHelper.GetDocLinkForPulumiType(def, "InvokeOptions"),
		},
	})

	return params
}

func (mod *modContext) genFunctionGo(f *schema.Function, funcName string, outputVersion bool) []formalParam {
	argsTypeSuffix := "Args"
	if outputVersion {
		argsTypeSuffix = "OutputArgs"
	}

	argsType := fmt.Sprintf("%s%s", funcName, argsTypeSuffix)

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

func (mod *modContext) genFunctionCS(f *schema.Function, funcName string, outputVersion bool) []formalParam {
	dctx := mod.docGenContext

	argsTypeSuffix := "Args"
	if outputVersion {
		argsTypeSuffix = "InvokeArgs"
	}

	argsType := funcName + argsTypeSuffix
	docLangHelper := dctx.getLanguageDocHelper("csharp")
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

	def, err := mod.pkg.Definition()
	contract.AssertNoErrorf(err, "failed to get definition for package %q", mod.pkg.Name())
	params = append(params, formalParam{
		Name:         "opts",
		OptionalFlag: "?",
		DefaultValue: " = null",
		Type: propertyType{
			Name: "InvokeOptions",
			Link: docLangHelper.GetDocLinkForPulumiType(def, "Pulumi.InvokeOptions"),
		},
	})
	return params
}

func (mod *modContext) genFunctionJava(f *schema.Function, funcName string, outputVersion bool) []formalParam {
	dctx := mod.docGenContext

	argsTypeSuffix := "Args"
	if outputVersion {
		argsTypeSuffix = "InvokeArgs"
	}

	argsType := title(funcName+argsTypeSuffix, "java")
	docLangHelper := dctx.getLanguageDocHelper("java")
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
	def, err := mod.pkg.Definition()
	contract.AssertNoErrorf(err, "failed to get definition for package %q", mod.pkg.Name())

	params = append(params, formalParam{
		Name:         "options",
		OptionalFlag: "@Nullable",
		Type: propertyType{
			Name: "InvokeOptions",
			Link: docLangHelper.GetDocLinkForPulumiType(def, "InvokeOptions"),
		},
	})
	return params
}

func (mod *modContext) genFunctionPython(f *schema.Function, resourceName string, outputVersion bool) []formalParam {
	dctx := mod.docGenContext
	docLanguageHelper := dctx.getLanguageDocHelper("python")
	var params []formalParam

	// Some functions don't have any inputs other than the InvokeOptions.
	// For example, the `get_billing_service_account` function.
	if f.Inputs != nil {
		inputs := f.Inputs
		if outputVersion {
			inputs = inputs.InputShape
		}

		params = slice.Prealloc[formalParam](len(inputs.Properties))
		for _, prop := range inputs.Properties {
			var schemaType schema.Type
			if outputVersion {
				schemaType = codegen.OptionalType(prop)
			} else {
				schemaType = codegen.PlainType(codegen.OptionalType(prop))
			}

			def, err := mod.pkg.Definition()
			contract.AssertNoErrorf(err, "failed to get definition for package %q", mod.pkg.Name())

			typ := docLanguageHelper.GetLanguageTypeString(def, mod.mod,
				schemaType, true /*input*/)
			params = append(params, formalParam{
				Name:         python.PyName(prop.Name),
				DefaultValue: " = None",
				Type: propertyType{
					Name: typ,
				},
			})
		}
	} else {
		params = slice.Prealloc[formalParam](1)
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
func (mod *modContext) genFunctionArgs(f *schema.Function, funcNameMap map[string]string, outputVersion bool) map[string]string {
	dctx := mod.docGenContext
	functionParams := make(map[string]string)

	for _, lang := range dctx.supportedLanguages {
		var (
			paramTemplate string
			params        []formalParam
		)
		b := &bytes.Buffer{}

		paramSeparatorTemplate := "param_separator"
		ps := paramSeparator{}

		switch lang {
		case "nodejs":
			params = mod.genFunctionTS(f, funcNameMap["nodejs"], outputVersion)
			paramTemplate = "ts_formal_param"
		case "go":
			params = mod.genFunctionGo(f, funcNameMap["go"], outputVersion)
			paramTemplate = "go_formal_param"
		case "csharp":
			params = mod.genFunctionCS(f, funcNameMap["csharp"], outputVersion)
			paramTemplate = "csharp_formal_param"
		case "java":
			params = mod.genFunctionJava(f, funcNameMap["java"], outputVersion)
			paramTemplate = "java_formal_param"
		case "yaml":
			// Left blank
		case "python":
			params = mod.genFunctionPython(f, funcNameMap["python"], outputVersion)
			paramTemplate = "py_formal_param"
			paramSeparatorTemplate = "py_param_separator"

			docHelper := dctx.getLanguageDocHelper(lang)
			funcName := docHelper.GetFunctionName(mod.mod, f)
			ps = paramSeparator{Indent: strings.Repeat(" ", len("def (")+len(funcName))}
		}

		n := len(params)
		if n == 0 {
			continue
		}

		for i, p := range params {
			if err := dctx.templates.ExecuteTemplate(b, paramTemplate, p); err != nil {
				panic(err)
			}
			if i != n-1 {
				if err := dctx.templates.ExecuteTemplate(b, paramSeparatorTemplate, ps); err != nil {
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
			"and supporting types.", mod.pkg.Name(), funcName)
		titleTag = fmt.Sprintf("%s.%s", mod.pkg.Name(), funcName)
	} else {
		baseDescription = fmt.Sprintf("Documentation for the %s.%s.%s function "+
			"with examples, input properties, output properties, "+
			"and supporting types.", mod.pkg.Name(), mod.mod, funcName)
		titleTag = fmt.Sprintf("%s.%s.%s", mod.pkg.Name(), mod.mod, funcName)
	}

	return header{
		Title:    funcName,
		TitleTag: titleTag,
		MetaDesc: baseDescription,
	}
}

func (mod *modContext) genFunctionOutputVersionMap(f *schema.Function) map[string]bool {
	dctx := mod.docGenContext
	result := map[string]bool{}
	for _, lang := range dctx.supportedLanguages {
		hasOutputVersion := f.NeedsOutputVersion()
		if lang == "go" {
			hasOutputVersion = go_gen.NeedsGoOutputVersion(f)
		}
		if lang == "java" || lang == "yaml" {
			hasOutputVersion = false
		}
		result[lang] = hasOutputVersion
	}
	return result
}

// genFunction is the main entrypoint for generating docs for a Function.
// Returns args type that can be used to execute the `function.tmpl` doc template.
func (mod *modContext) genFunction(f *schema.Function) functionDocArgs {
	dctx := mod.docGenContext
	inputProps := make(map[string][]property)
	outputProps := make(map[string][]property)
	for _, lang := range dctx.supportedLanguages {
		if f.Inputs != nil {
			inputProps[lang] = mod.getProperties(f.Inputs.Properties, lang, true, false, false)
		}
		if f.ReturnType != nil {
			if objectObject, ok := f.ReturnType.(*schema.ObjectType); ok {
				outputProps[lang] = mod.getProperties(objectObject.Properties,
					lang, false, false, false)
			}
		}
	}

	nestedTypes := mod.genNestedTypes(f, false /*resourceType*/, false /*isProvider*/)

	// Generate the per-language map for the function name.
	funcNameMap := map[string]string{}
	for _, lang := range dctx.supportedLanguages {
		docHelper := dctx.getLanguageDocHelper(lang)
		funcNameMap[lang] = docHelper.GetFunctionName(mod.mod, f)
	}

	def, err := mod.pkg.Definition()
	contract.AssertNoErrorf(err, "failed to get definition for package %q", mod.pkg.Name())

	packageDetails := packageDetails{
		DisplayName:    getPackageDisplayName(def.Name),
		Repository:     def.Repository,
		RepositoryName: getRepositoryName(def.Repository),
		License:        def.License,
		Notes:          def.Attribution,
	}

	supportedSnippetLanguages := mod.docGenContext.getSupportedSnippetLanguages(f.IsOverlay, f.OverlaySupportedLanguages)
	docInfo := dctx.decomposeDocstring(f.Comment, supportedSnippetLanguages)
	args := functionDocArgs{
		Header: mod.genFunctionHeader(f),

		Tool:                 mod.tool,
		LangChooserLanguages: supportedSnippetLanguages,

		FunctionName:   funcNameMap,
		FunctionArgs:   mod.genFunctionArgs(f, funcNameMap, false /*outputVersion*/),
		FunctionResult: mod.getFunctionResourceInfo(f, false /*outputVersion*/),

		Comment:            docInfo.description,
		DeprecationMessage: f.DeprecationMessage,
		ExamplesSection: examplesSection{
			Examples:             docInfo.examples,
			LangChooserLanguages: supportedSnippetLanguages,
		},

		InputProperties:  inputProps,
		OutputProperties: outputProps,

		NestedTypes: nestedTypes,

		PackageDetails: packageDetails,
	}

	args.HasOutputVersion = mod.genFunctionOutputVersionMap(f)

	for _, hasOutputVersion := range args.HasOutputVersion {
		if hasOutputVersion {
			args.AnyLanguageHasOutputVersion = true
			continue
		}
	}

	if f.NeedsOutputVersion() {
		args.FunctionArgsOutputVersion = mod.genFunctionArgs(f, funcNameMap, true /*outputVersion*/)
		args.FunctionResultOutputVersion = mod.getFunctionResourceInfo(f, true /*outputVersion*/)
	}

	return args
}
