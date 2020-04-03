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
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/codegen"
	"github.com/pulumi/pulumi/pkg/codegen/dotnet"
	go_gen "github.com/pulumi/pulumi/pkg/codegen/go"
	"github.com/pulumi/pulumi/pkg/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/codegen/python"
	"github.com/pulumi/pulumi/pkg/codegen/schema"
	"github.com/pulumi/pulumi/sdk/go/common/util/contract"
)

var (
	supportedLanguages = []string{"csharp", "go", "nodejs", "python"}
	templates          *template.Template
	packagedTemplates  map[string][]byte
	docHelpers         map[string]codegen.DocLanguageHelper

	// The following property case maps are for rendering property
	// names of nested properties in Python language with the correct
	// casing.
	snakeCaseToCamelCase map[string]string
	camelCaseToSnakeCase map[string]string

	// The language-specific info objects for a certain package (provider).
	goPkgInfo     go_gen.GoInfo
	csharpPkgInfo dotnet.CSharpPackageInfo

	// langModuleNameLookup is a map of module name to its language-specific
	// name.
	langModuleNameLookup map[string]string
)

func init() {
	docHelpers = make(map[string]codegen.DocLanguageHelper)
	for _, lang := range supportedLanguages {
		switch lang {
		case "csharp":
			docHelpers[lang] = &dotnet.DocLanguageHelper{}
		case "go":
			docHelpers[lang] = &go_gen.DocLanguageHelper{}
		case "nodejs":
			docHelpers[lang] = &nodejs.DocLanguageHelper{}
		case "python":
			docHelpers[lang] = &python.DocLanguageHelper{}
		}
	}

	snakeCaseToCamelCase = map[string]string{}
	camelCaseToSnakeCase = map[string]string{}
	langModuleNameLookup = map[string]string{}
}

// header represents the header of each resource markdown file.
type header struct {
	Title string
}

// property represents an input or an output property.
type property struct {
	// DisplayName is the property name with word-breaks.
	DisplayName        string
	Name               string
	Comment            string
	Type               propertyType
	DeprecationMessage string

	IsRequired bool
	IsInput    bool
}

// apiTypeDocLinks represents the links for a type's input and output API doc.
type apiTypeDocLinks struct {
	InputType  string
	OutputType string
}

// docNestedType represents a complex type.
type docNestedType struct {
	Name        string
	APIDocLinks map[string]apiTypeDocLinks
	Properties  map[string][]property
}

// propertyType represents the type of a property.
type propertyType struct {
	DisplayName string
	Name        string
	// Link can be a link to an anchor tag on the same
	// page, or to another page/site.
	Link string
}

// formalParam represents the formal parameters of a constructor
// or a lookup function.
type formalParam struct {
	Name string
	Type propertyType

	// This is the language specific optional type indicator.
	// For example, in nodejs this is the character "?" and in Go
	// it's "*".
	OptionalFlag string

	DefaultValue string
}

type packageDetails struct {
	Repository string
	License    string
	Notes      string
}

type resourceDocArgs struct {
	Header header

	// Comment represents the introductory resource comment.
	Comment string

	ConstructorParams map[string]string
	// ConstructorResource is the resource that is being constructed or
	// is the result of a constructor-like function.
	ConstructorResource map[string]propertyType
	// ArgsRequired is a flag indicating if the args param is required
	// when creating a new resource.
	ArgsRequired bool

	// InputProperties is a map per language and a corresponding slice of
	// input properties accepted as args while creating a new resource.
	InputProperties map[string][]property
	// OutputProperties is a map per language and a corresponding slice of
	// output properties returned when a new instance of the resource is
	// created.
	OutputProperties map[string][]property

	// LookupParams is a map of the param string to be rendered per language
	// for looking-up a resource.
	LookupParams map[string]string
	// StateInputs is a map per language and the corresponding slice of
	// state input properties required while looking-up an existing resource.
	StateInputs map[string][]property
	// StateParam is the type name of the state param, if any.
	StateParam string

	// NestedTypes is a slice of the nested types used in the input and
	// output properties.
	NestedTypes []docNestedType

	PackageDetails packageDetails
}

// typeUsage represents a nested type's usage.
type typeUsage struct {
	Input  bool
	Output bool
}

// nestedTypeUsageInfo is a type-alias for a map of Pulumi type-tokens
// and whether or not the type is used as an input and/or output
// properties.
type nestedTypeUsageInfo map[string]typeUsage

func (ss nestedTypeUsageInfo) add(s string, input bool) {
	if v, ok := ss[s]; ok {
		if input {
			v.Input = true
		} else {
			v.Output = true
		}
		ss[s] = v
		return
	}

	ss[s] = typeUsage{
		Input:  input,
		Output: !input,
	}
}

// contains returns true if the token already exists and matches the
// input or output flag of the token.
func (ss nestedTypeUsageInfo) contains(token string, input bool) bool {
	a, ok := ss[token]
	if !ok {
		return false
	}

	if input && a.Input {
		return true
	} else if !input && a.Output {
		return true
	}
	return false
}

type modContext struct {
	pkg       *schema.Package
	mod       string
	resources []*schema.Resource
	functions []*schema.Function
	children  []*modContext
	tool      string
}

func resourceName(r *schema.Resource) string {
	if r.IsProvider {
		return "Provider"
	}
	return tokenToName(r.Token)
}

func getLanguageDocHelper(lang string) codegen.DocLanguageHelper {
	if h, ok := docHelpers[lang]; ok {
		return h
	}
	panic(errors.Errorf("could not find a doc lang helper for %s", lang))
}

type propertyCharacteristics struct {
	// input is a flag indicating if the property is an input type.
	input bool
	// optional is a flag indicating if the property is optional.
	optional bool
}

// getLanguageModuleName returns the module name mapped to its language-specific
// equivalent if the schema for this provider has any overrides for that language.
// Otherwise, returns the module name as-is.
func getLanguageModuleName(lang string, mod string) string {
	modName := mod
	lookupKey := lang + "_" + modName
	if v, ok := langModuleNameLookup[lookupKey]; ok {
		return v
	}

	switch lang {
	case "go":
		if override, ok := goPkgInfo.ModuleToPackage[modName]; ok {
			modName = override
		}
	case "csharp":
		if override, ok := csharpPkgInfo.Namespaces[modName]; ok {
			modName = override
		}
	}

	langModuleNameLookup[lookupKey] = modName
	return modName
}

// typeString returns a property type suitable for docs with its display name and the anchor link to
// a type if the type of the property is an array or an object.
func (mod *modContext) typeString(t schema.Type, lang string, characteristics propertyCharacteristics, insertWordBreaks bool) propertyType {
	docLanguageHelper := getLanguageDocHelper(lang)
	modName := getLanguageModuleName(lang, mod.mod)
	langTypeString := docLanguageHelper.GetLanguageTypeString(mod.pkg, modName, t, characteristics.input, characteristics.optional)

	// If the type is an object type, let's also wrap it with a link to the supporting type
	// on the same page using an anchor tag.
	var href string
	switch t := t.(type) {
	case *schema.ArrayType:
		elementLangType := mod.typeString(t.ElementType, lang, characteristics, false)
		href = elementLangType.Link
	case *schema.ObjectType:
		tokenName := tokenToName(t.Token)
		// Links to anchor targs on the same page must be lower-cased.
		href = "#" + lower(tokenName)
	}

	// Strip the namespace/module prefix for the type's display name.
	var parts []string
	var displayName string
	if lang == "csharp" {
		// C# types can be wrapped in enumerable types such as List<> or Dictionary<>, so we have to
		// only replace the namespace string within the < and the >.
		qualifier := "Inputs"
		if !characteristics.input {
			qualifier = "Outputs"
		}

		var csharpNS string
		// This type could be at the package-level, so it won't have a module name.
		if modName != "" {
			csharpNS = fmt.Sprintf("Pulumi.%s.%s.%s.", strings.Title(mod.pkg.Name), strings.Title(modName), qualifier)
		} else {
			csharpNS = fmt.Sprintf("Pulumi.%s.%s.", strings.Title(mod.pkg.Name), qualifier)
		}
		displayName = strings.ReplaceAll(langTypeString, csharpNS, "")
	} else {
		parts = strings.Split(langTypeString, ".")
		displayName = parts[len(parts)-1]
	}

	// If word-breaks need to be inserted, then the type string
	// should be html-encoded first if the language is C# in order
	// to avoid confusing the Hugo rendering where the word-break
	// tags are inserted.
	if insertWordBreaks {
		if lang == "csharp" {
			displayName = html.EscapeString(displayName)
		}
		displayName = wbr(displayName)
	}
	return propertyType{
		Name:        langTypeString,
		DisplayName: displayName,
		Link:        href,
	}
}

func (mod *modContext) genConstructorTS(r *schema.Resource, argsOptional bool) []formalParam {
	name := resourceName(r)
	argsType := name + "Args"
	argsFlag := ""
	if argsOptional {
		argsFlag = "?"
	}

	docLangHelper := getLanguageDocHelper("nodejs")
	return []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
				Link: nodejs.GetDocLinkForBuiltInType("string"),
			},
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			Type: propertyType{
				Name: argsType,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, mod.mod, argsType),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			Type: propertyType{
				Name: "pulumi.CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForResourceType("pulumi", "", "CustomResourceOptions"),
			},
		},
	}
}

func (mod *modContext) genConstructorGo(r *schema.Resource, argsOptional bool) []formalParam {
	name := resourceName(r)
	argsType := name + "Args"
	argsFlag := ""
	if argsOptional {
		argsFlag = "*"
	}

	docLangHelper := getLanguageDocHelper("go")
	// return fmt.Sprintf("func New%s(ctx *pulumi.Context, name string, args *%s, opts ...pulumi.ResourceOption) (*%s, error)\n", name, argsType, name)
	return []formalParam{
		{
			Name:         "ctx",
			OptionalFlag: "*",
			Type: propertyType{
				Name: "pulumi.Context",
				Link: docLangHelper.GetDocLinkForResourceType("", "pulumi", "Context"),
			},
		},
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
				Link: go_gen.GetDocLinkForBuiltInType("string"),
			},
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			Type: propertyType{
				Name: argsType,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, mod.mod, argsType),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "...",
			Type: propertyType{
				Name: "pulumi.ResourceOption",
				Link: docLangHelper.GetDocLinkForResourceType("", "pulumi", "ResourceOption"),
			},
		},
	}
}

func (mod *modContext) genConstructorCS(r *schema.Resource, argsOptional bool) []formalParam {
	name := resourceName(r)
	argsSchemaType := &schema.ObjectType{
		Token: r.Token,
	}
	// Get the C#-specific name for the args type, which will be the fully-qualified name.
	characteristics := propertyCharacteristics{
		input:    true,
		optional: argsOptional,
	}
	argLangType := mod.typeString(argsSchemaType, "csharp", characteristics, false)
	// The args type for a resource isn't part of "Inputs" namespace, so remove the "Inputs"
	// namespace qualifier.
	argLangTypeName := strings.ReplaceAll(argLangType.Name, "Inputs.", "")

	var argsFlag string
	var argsDefault string
	if argsOptional {
		// If the number of required input properties was zero, we can make the args object optional.
		argsDefault = " = null"
		argsFlag = "?"
	}

	docLangHelper := getLanguageDocHelper("csharp")
	return []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
				Link: "https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/builtin-types/built-in-types",
			},
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			DefaultValue: argsDefault,
			Type: propertyType{
				Name: name + "Args",
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, "", argLangTypeName),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			DefaultValue: " = null",
			Type: propertyType{
				Name: "CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForResourceType("", "", "Pulumi.CustomResourceOptions"),
			},
		},
	}
}

func (mod *modContext) genNestedTypes(member interface{}, resourceType bool) []docNestedType {
	tokens := nestedTypeUsageInfo{}
	// Collect all of the types for this "member" as a map of resource names
	// and if it appears in an input object and/or output object.
	mod.getTypes(member, tokens)

	var objs []docNestedType
	for token, tyUsage := range tokens {
		for _, t := range mod.pkg.Types {
			if obj, ok := t.(*schema.ObjectType); ok && obj.Token == token {
				if len(obj.Properties) == 0 {
					continue
				}

				// Create maps to hold the per-language properties of this object and links to
				// the API doc for each language.
				props := make(map[string][]property)
				apiDocLinks := make(map[string]apiTypeDocLinks)
				for _, lang := range supportedLanguages {
					// The nested type may be under a different package in a language.
					// For example, in k8s, common types are in the core/v1 module and can appear in
					// nested types elsewhere. So we use the appropriate name of that type,
					// as well as its language-specific name. For example, module name for use as a C# namespace
					// or as a Go package name.
					modName := getLanguageModuleName(lang, mod.mod)
					nestedTypeModName := mod.pkg.TokenToModule(token)
					if nestedTypeModName != mod.mod {
						modName = getLanguageModuleName(lang, nestedTypeModName)
					}

					docLangHelper := getLanguageDocHelper(lang)
					inputCharacteristics := propertyCharacteristics{
						input:    true,
						optional: true,
					}
					outputCharacteristics := propertyCharacteristics{
						input:    false,
						optional: true,
					}
					inputObjLangType := mod.typeString(t, lang, inputCharacteristics, false /*insertWordBreaks*/)
					outputObjLangType := mod.typeString(t, lang, outputCharacteristics, false /*insertWordBreaks*/)

					// Get the doc link for this nested type based on whether the type is for a Function or a Resource.
					var inputTypeDocLink string
					var outputTypeDocLink string
					if resourceType {
						if tyUsage.Input {
							inputTypeDocLink = docLangHelper.GetDocLinkForResourceInputOrOutputType(mod.pkg.Name, modName, inputObjLangType.Name, true)
						}
						if tyUsage.Output {
							outputTypeDocLink = docLangHelper.GetDocLinkForResourceInputOrOutputType(mod.pkg.Name, modName, outputObjLangType.Name, false)
						}
					} else {
						if tyUsage.Input {
							inputTypeDocLink = docLangHelper.GetDocLinkForFunctionInputOrOutputType(mod.pkg.Name, modName, inputObjLangType.Name, true)
						}
						if tyUsage.Output {
							outputTypeDocLink = docLangHelper.GetDocLinkForFunctionInputOrOutputType(mod.pkg.Name, modName, outputObjLangType.Name, false)
						}
					}
					apiDocLinks[lang] = apiTypeDocLinks{
						InputType:  inputTypeDocLink,
						OutputType: outputTypeDocLink,
					}
					props[lang] = mod.getProperties(obj.Properties, lang, true, true)
				}

				objs = append(objs, docNestedType{
					Name:        wbr(tokenToName(obj.Token)),
					APIDocLinks: apiDocLinks,
					Properties:  props,
				})
			}
		}
	}

	sort.Slice(objs, func(i, j int) bool {
		return objs[i].Name < objs[j].Name
	})

	return objs
}

// getProperties returns a slice of properties that can be rendered for docs for
// the provided slice of properties in the schema.
func (mod *modContext) getProperties(properties []*schema.Property, lang string, input, nested bool) []property {
	if len(properties) == 0 {
		return nil
	}

	docProperties := make([]property, 0, len(properties))
	for _, prop := range properties {
		if prop == nil {
			continue
		}

		characteristics := propertyCharacteristics{
			input:    input,
			optional: !prop.IsRequired,
		}

		langDocHelper := getLanguageDocHelper(lang)
		var propLangName string
		switch lang {
		case "python":
			pyName := python.PyName(prop.Name)
			// The default casing for a Python property name is snake_case unless
			// it is a property of a nested object, in which case, we should check the property
			// case maps.
			propLangName = pyName

			if nested {
				if snakeCase, ok := camelCaseToSnakeCase[prop.Name]; ok {
					propLangName = snakeCase
				} else if camelCase, ok := snakeCaseToCamelCase[pyName]; ok {
					propLangName = camelCase
				} else {
					// If neither of the property case maps have the property
					// then use the default name of the property.
					propLangName = prop.Name
				}
			}
		default:
			name, err := langDocHelper.GetPropertyName(prop)
			if err != nil {
				panic(err)
			}

			propLangName = name
		}

		docProperties = append(docProperties, property{
			DisplayName:        wbr(propLangName),
			Name:               propLangName,
			Comment:            prop.Comment,
			DeprecationMessage: prop.DeprecationMessage,
			IsRequired:         prop.IsRequired,
			IsInput:            input,
			Type:               mod.typeString(prop.Type, lang, characteristics, true),
		})
	}

	return docProperties
}

func (mod *modContext) genConstructors(r *schema.Resource, allOptionalInputs bool) map[string]string {
	constructorParams := make(map[string]string)
	for _, lang := range supportedLanguages {
		var (
			paramTemplate string
			params        []formalParam
		)
		b := &bytes.Buffer{}

		switch lang {
		case "nodejs":
			params = mod.genConstructorTS(r, allOptionalInputs)
			paramTemplate = "ts_formal_param"
		case "go":
			params = mod.genConstructorGo(r, allOptionalInputs)
			paramTemplate = "go_formal_param"
		case "csharp":
			params = mod.genConstructorCS(r, allOptionalInputs)
			paramTemplate = "csharp_formal_param"
		case "python":
			paramTemplate = "py_formal_param"
			// The Pulumi Python SDK does not have types for constructor args.
			// The input properties for a resource needs to be exploded as
			// individual constructor params.
			params = make([]formalParam, 0, len(r.InputProperties))
			for _, p := range r.InputProperties {
				params = append(params, formalParam{
					Name:         python.PyName(p.Name),
					DefaultValue: "=None",
				})
			}
		}

		n := len(params)
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
		constructorParams[lang] = b.String()
	}
	return constructorParams
}

// getConstructorResourceInfo returns a map of per-language information about
// the resource being constructed.
func (mod *modContext) getConstructorResourceInfo(resourceTypeName string) map[string]propertyType {
	resourceMap := make(map[string]propertyType)
	resourceDisplayName := resourceTypeName

	for _, lang := range supportedLanguages {
		// Reset the type name back to the display name.
		resourceTypeName = resourceDisplayName

		docLangHelper := getLanguageDocHelper(lang)
		switch lang {
		case "nodejs", "go":
			// Intentionally left blank.
		case "csharp":
			resourceTypeName = fmt.Sprintf("Pulumi.%s.%s.%s", strings.Title(mod.pkg.Name), strings.Title(mod.mod), resourceTypeName)
		case "python":
			// Pulumi's Python language SDK does not have "types" yet, so we will skip it for now.
			continue
		default:
			panic(errors.Errorf("cannot generate constructor info for unhandled language %q", lang))
		}

		parts := strings.Split(resourceTypeName, ".")
		displayName := parts[len(parts)-1]
		resourceMap[lang] = propertyType{
			Name:        resourceDisplayName,
			DisplayName: displayName,
			Link:        docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, mod.mod, resourceTypeName),
		}
	}

	return resourceMap
}

func (mod *modContext) getTSLookupParams(r *schema.Resource, stateParam string) []formalParam {
	docLangHelper := getLanguageDocHelper("nodejs")
	return []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
				Link: nodejs.GetDocLinkForBuiltInType("string"),
			},
		},
		{
			Name: "id",
			Type: propertyType{
				Name: "Input<ID>",
				Link: docLangHelper.GetDocLinkForResourceType("", "pulumi", "ID"),
			},
		},
		{
			Name:         "state",
			OptionalFlag: "?",
			Type: propertyType{
				Name: stateParam,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, mod.mod, stateParam),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			Type: propertyType{
				Name: "CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForResourceType("", "pulumi", "CustomResourceOptions"),
			},
		},
	}
}

func (mod *modContext) getGoLookupParams(r *schema.Resource, stateParam string) []formalParam {
	docLangHelper := getLanguageDocHelper("go")
	// return fmt.Sprintf("func New%s(ctx *pulumi.Context, name string, args *%s, opts ...pulumi.ResourceOption) (*%s, error)\n", name, argsType, name)
	return []formalParam{
		{
			Name:         "ctx",
			OptionalFlag: "*",
			Type: propertyType{
				Name: "Context",
				Link: docLangHelper.GetDocLinkForResourceType("", "pulumi", "Context"),
			},
		},
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
				Link: go_gen.GetDocLinkForBuiltInType("string"),
			},
		},
		{
			Name: "id",
			Type: propertyType{
				Name: "IDInput",
				Link: docLangHelper.GetDocLinkForResourceType("", "pulumi", "IDInput"),
			},
		},
		{
			Name:         "state",
			OptionalFlag: "*",
			Type: propertyType{
				Name: stateParam,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, mod.mod, stateParam),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "...",
			Type: propertyType{
				Name: "ResourceOption",
				Link: docLangHelper.GetDocLinkForResourceType("", "pulumi", "ResourceOption"),
			},
		},
	}
}

func (mod *modContext) getCSLookupParams(r *schema.Resource, stateParam string) []formalParam {
	stateParamFQDN := fmt.Sprintf("Pulumi.%s.%s.%s", strings.Title(mod.pkg.Name), strings.Title(mod.mod), stateParam)

	docLangHelper := getLanguageDocHelper("csharp")
	return []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
				Link: "https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/builtin-types/built-in-types",
			},
		},
		{
			Name: "id",
			Type: propertyType{
				Name: "Input<string>",
				Link: docLangHelper.GetDocLinkForResourceType("", "", "Pulumi.Input"),
			},
		},
		{
			Name:         "state",
			OptionalFlag: "?",
			Type: propertyType{
				Name: stateParam,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, "", stateParamFQDN),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			DefaultValue: " = null",
			Type: propertyType{
				Name: "CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForResourceType("", "", "Pulumi.CustomResourceOptions"),
			},
		},
	}
}

// genLookupParams generates a map of per-language way of rendering the formal parameters of the lookup function
// used to lookup an existing resource.
func (mod *modContext) genLookupParams(r *schema.Resource, stateParam string) map[string]string {
	lookupParams := make(map[string]string)
	if r.StateInputs == nil {
		return lookupParams
	}

	for _, lang := range supportedLanguages {
		var (
			paramTemplate string
			params        []formalParam
		)
		b := &bytes.Buffer{}

		switch lang {
		case "nodejs":
			params = mod.getTSLookupParams(r, stateParam)
			paramTemplate = "ts_formal_param"
		case "go":
			params = mod.getGoLookupParams(r, stateParam)
			paramTemplate = "go_formal_param"
		case "csharp":
			params = mod.getCSLookupParams(r, stateParam)
			paramTemplate = "csharp_formal_param"
		case "python":
			paramTemplate = "py_formal_param"
			// The Pulumi Python SDK does not yet have types for formal parameters.
			// The input properties for a resource needs to be exploded as
			// individual constructor params.
			params = make([]formalParam, 0, len(r.StateInputs.Properties))
			for _, p := range r.StateInputs.Properties {
				params = append(params, formalParam{
					Name:         python.PyName(p.Name),
					DefaultValue: "=None",
				})
			}
		}

		n := len(params)
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
		lookupParams[lang] = b.String()
	}
	return lookupParams
}

// genResource is the entrypoint for generating a doc for a resource
// from its Pulumi schema.
func (mod *modContext) genResource(r *schema.Resource) resourceDocArgs {
	// Create a resource module file into which all of this resource's types will go.
	name := resourceName(r)

	// TODO: Unlike the other languages, Python does not have a separate Args object for inputs.
	// The args are all just named parameters of the constructor. Consider injecting
	// `resource_name` and `opts` as the first two items in the table of properties.
	inputProps := make(map[string][]property)
	outputProps := make(map[string][]property)
	stateInputs := make(map[string][]property)
	for _, lang := range supportedLanguages {
		inputProps[lang] = mod.getProperties(r.InputProperties, lang, true, false)
		if r.IsProvider {
			continue
		}
		outputProps[lang] = mod.getProperties(r.Properties, lang, false, false)
		if r.StateInputs != nil {
			stateInputs[lang] = mod.getProperties(r.StateInputs.Properties, lang, true, false)
		}
	}

	allOptionalInputs := true
	for _, prop := range r.InputProperties {
		// If at least one prop is required, then break.
		if prop.IsRequired {
			allOptionalInputs = false
			break
		}
	}

	packageDetails := packageDetails{
		Repository: mod.pkg.Repository,
		License:    mod.pkg.License,
		Notes:      mod.pkg.Attribution,
	}

	stateParam := name + "State"
	data := resourceDocArgs{
		Header: header{
			Title: name,
		},

		Comment: r.Comment,

		ConstructorParams:   mod.genConstructors(r, allOptionalInputs),
		ConstructorResource: mod.getConstructorResourceInfo(name),
		ArgsRequired:        !allOptionalInputs,

		InputProperties:  inputProps,
		OutputProperties: outputProps,
		LookupParams:     mod.genLookupParams(r, stateParam),
		StateInputs:      stateInputs,
		StateParam:       stateParam,
		NestedTypes:      mod.genNestedTypes(r, true /*resourceType*/),

		PackageDetails: packageDetails,
	}

	return data
}

func (mod *modContext) getNestedTypes(t schema.Type, types nestedTypeUsageInfo, input bool) {
	switch t := t.(type) {
	case *schema.ArrayType:
		glog.V(4).Infof("visiting array %s\n", t.ElementType.String())
		skip := false
		if o, ok := t.ElementType.(*schema.ObjectType); ok && types.contains(o.Token, input) {
			glog.V(4).Infof("already added %s. skipping...\n", o.Token)
			skip = true
		}

		if !skip {
			mod.getNestedTypes(t.ElementType, types, input)
		}
	case *schema.MapType:
		glog.V(4).Infof("visiting map %s\n", t.ElementType.String())
		skip := false
		if o, ok := t.ElementType.(*schema.ObjectType); ok && types.contains(o.Token, input) {
			glog.V(4).Infof("already added %s. skipping...\n", o.Token)
			skip = true
		}

		if !skip {
			mod.getNestedTypes(t.ElementType, types, input)
		}
	case *schema.ObjectType:
		glog.V(4).Infof("visiting object %s\n", t.Token)
		types.add(t.Token, input)
		for _, p := range t.Properties {
			if o, ok := p.Type.(*schema.ObjectType); ok && types.contains(o.Token, input) {
				glog.V(4).Infof("already added %s. skipping...\n", o.Token)
				continue
			}
			glog.V(4).Infof("visiting object property %s\n", p.Type.String())
			mod.getNestedTypes(p.Type, types, input)
		}
	case *schema.UnionType:
		glog.V(4).Infof("visiting union type %s\n", t.String())
		for _, e := range t.ElementTypes {
			if o, ok := e.(*schema.ObjectType); ok && types.contains(o.Token, input) {
				glog.V(4).Infof("already added %s. skipping...\n", o.Token)
				continue
			}
			glog.V(4).Infof("visiting union element type %s\n", e.String())
			mod.getNestedTypes(e, types, input)
		}
	}
}

func (mod *modContext) getTypes(member interface{}, types nestedTypeUsageInfo) {
	glog.V(3).Infoln("getting nested types for module", mod.mod)

	switch t := member.(type) {
	case *schema.ObjectType:
		for _, p := range t.Properties {
			mod.getNestedTypes(p.Type, types, false)
		}
	case *schema.Resource:
		for _, p := range t.Properties {
			mod.getNestedTypes(p.Type, types, false)
		}
		for _, p := range t.InputProperties {
			mod.getNestedTypes(p.Type, types, true)
		}
	case *schema.Function:
		if t.Inputs != nil {
			mod.getNestedTypes(t.Inputs, types, true)
		}
		if t.Outputs != nil {
			mod.getNestedTypes(t.Outputs, types, false)
		}
	}
}

func (mod *modContext) genHeader(w io.Writer, title string, menu bool) {
	// TODO: Generate the actual front matter we want for these pages.
	// Example:
	// title: "Package @pulumi/aws"
	// title_tag: "Package @pulumi/aws | Node.js SDK"
	// linktitle: "@pulumi/aws"
	// meta_desc: "Explore members of the @pulumi/aws package."
	// menu:
	// 	   reference:
	// 		   parent: API Reference
	// 		   identifier: Server

	fmt.Fprintf(w, "---\n")
	fmt.Fprintf(w, "title: %q\n", title)
	fmt.Fprintf(w, "block_external_search_index: true\n")

	// Only add the menu section for pages that should be in the TOC.
	if menu {
		fmt.Fprintf(w, "menu:\n")
		fmt.Fprintf(w, "    reference:\n")
		fmt.Fprintf(w, "        parent: API Reference\n")
	}

	fmt.Fprintf(w, "---\n\n")
	fmt.Fprintf(w, "<!-- WARNING: this file was generated by %v. -->\n", mod.tool)
	fmt.Fprintf(w, "<!-- Do not edit by hand unless you're certain you know what you are doing! -->\n\n")
}

type fs map[string][]byte

func (fs fs) add(path string, contents []byte) {
	_, has := fs[path]
	contract.Assertf(!has, "duplicate file: %s", path)
	fs[path] = contents
}

func (mod *modContext) gen(fs fs) error {
	var files []string
	for p := range fs {
		d := path.Dir(p)
		if d == "." {
			d = ""
		}
		if d == mod.mod {
			files = append(files, p)
		}
	}

	addFile := func(name, contents string) {
		p := path.Join(mod.mod, name)
		files = append(files, p)
		fs.add(p, []byte(contents))
	}

	// Resources
	for _, r := range mod.resources {
		data := mod.genResource(r)

		title := resourceName(r)
		buffer := &bytes.Buffer{}
		err := templates.ExecuteTemplate(buffer, "resource.tmpl", data)
		if err != nil {
			panic(err)
		}
		addFile(lower(title)+".md", buffer.String())
	}

	// Functions
	for _, f := range mod.functions {
		data := mod.genFunction(f)

		buffer := &bytes.Buffer{}
		err := templates.ExecuteTemplate(buffer, "function.tmpl", data)
		if err != nil {
			panic(err)
		}
		addFile(lower(tokenToName(f.Token))+".md", buffer.String())
	}

	// Index
	fs.add(path.Join(mod.mod, "_index.md"), []byte(mod.genIndex(files)))
	return nil
}

// genIndex emits an _index.md file for the module.
func (mod *modContext) genIndex(exports []string) string {
	w := &bytes.Buffer{}

	name := mod.mod
	menu := false
	if name == "" {
		name = mod.pkg.Name

		// Flag top-level entries for inclusion in the table-of-contents menu.
		menu = true
	}

	mod.genHeader(w, name, menu)

	// If this is the root module, write out the package description.
	if mod.mod == "" {
		description := mod.pkg.Description
		if description != "" {
			description += "\n\n"
		}
		fmt.Fprint(w, description)
	}

	// If there are submodules, list them.
	var children []string
	for _, mod := range mod.children {
		children = append(children, mod.mod)
	}
	if len(children) > 0 {
		sort.Strings(children)
		fmt.Fprintf(w, "<h3>Modules</h3>\n")
		fmt.Fprintf(w, "<ul class=\"api\">\n")
		for _, mod := range children {
			fmt.Fprintf(w, "    <li><a href=\"%s/\"><span class=\"symbol module\"></span>%s</a></li>\n", mod, mod)
		}
		fmt.Fprintf(w, "</ul>\n\n")
	}

	// If there are resources in the root, list them.
	var resources []string
	for _, r := range mod.resources {
		resources = append(resources, resourceName(r))
	}
	if len(resources) > 0 {
		sort.Strings(resources)
		fmt.Fprintf(w, "<h3>Resources</h3>\n")
		fmt.Fprintf(w, "<ul class=\"api\">\n")
		for _, r := range resources {
			fmt.Fprintf(w, "    <li><a href=\"%s\"><span class=\"symbol resource\"></span>%s</a></li>\n", lower(r), r)
		}
		fmt.Fprintf(w, "</ul>\n\n")
	}

	// If there are functions in the root, list them.
	var functions []string
	for _, f := range mod.functions {
		functions = append(functions, tokenToName(f.Token))
	}
	if len(functions) > 0 {
		sort.Strings(functions)
		fmt.Fprintf(w, "<h3>Functions</h3>\n")
		fmt.Fprintf(w, "<ul class=\"api\">\n")
		for _, f := range functions {
			// TODO: We want to use "function" rather than "data source" terminology. Need to add a
			// "function" class in the docs repo to replace "datasource".
			fmt.Fprintf(w, "    <li><a href=\"%s\"><span class=\"symbol datasource\"></span>%s</a></li>\n", lower(f), f)
		}
		fmt.Fprintf(w, "</ul>\n\n")
	}

	fmt.Fprintf(w, "<h3>Package Details</h3>\n")
	fmt.Fprintf(w, "<dl class=\"package-details\">\n")
	fmt.Fprintf(w, "	<dt>Repository</dt>\n")
	fmt.Fprintf(w, "	<dd>%s</dd>\n", mod.pkg.Repository)
	fmt.Fprintf(w, "	<dt>License</dt>\n")
	fmt.Fprintf(w, "	<dd>%s</dd>\n", mod.pkg.License)
	if mod.pkg.Attribution != "" {
		fmt.Fprintf(w, "	<dt>Notes</dt>\n")
		fmt.Fprintf(w, "	<dd>%s</dd>\n", mod.pkg.Attribution)
	}
	fmt.Fprintf(w, "</dl>\n\n")

	return w.String()
}

func decodeLangSpecificInfo(pkg *schema.Package, lang string, obj interface{}) error {
	if csharp, ok := pkg.Language[lang]; ok {
		if err := json.Unmarshal([]byte(csharp), &obj); err != nil {
			return errors.Wrap(err, "decoding csharp package info")
		}
	}
	return nil
}

func generateModulesFromSchemaPackage(tool string, pkg *schema.Package) map[string]*modContext {
	// Group resources, types, and functions into modules.
	modules := map[string]*modContext{}

	var getMod func(token string) *modContext
	getMod = func(token string) *modContext {
		modName := pkg.TokenToModule(token)
		mod, ok := modules[modName]
		if !ok {
			mod = &modContext{
				pkg:  pkg,
				mod:  modName,
				tool: tool,
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

	// Decode Go-specific language info.
	if err := decodeLangSpecificInfo(pkg, "go", &goPkgInfo); err != nil {
		panic(errors.Wrap(err, "error decoding go language info"))
	}
	goLangHelper := getLanguageDocHelper("go").(*go_gen.DocLanguageHelper)
	// Generate the Go package map info now, so we can use that to get the type string
	// names later.
	goLangHelper.GeneratePackagesMap(pkg, tool, goPkgInfo)

	// Decode C#-specific language info.
	if err := decodeLangSpecificInfo(pkg, "csharp", &csharpPkgInfo); err != nil {
		panic(errors.Wrap(err, "error decoding c# language info"))
	}
	csharpLangHelper := getLanguageDocHelper("csharp").(*dotnet.DocLanguageHelper)
	csharpLangHelper.Namespaces = csharpPkgInfo.Namespaces

	pyLangHelper := getLanguageDocHelper("python").(*python.DocLanguageHelper)
	scanResource := func(r *schema.Resource) {
		mod := getMod(r.Token)
		mod.resources = append(mod.resources, r)

		for _, p := range r.Properties {
			pyLangHelper.GenPropertyCaseMap(mod.pkg, mod.mod, tool, p, snakeCaseToCamelCase, camelCaseToSnakeCase)
		}

		for _, p := range r.InputProperties {
			pyLangHelper.GenPropertyCaseMap(mod.pkg, mod.mod, tool, p, snakeCaseToCamelCase, camelCaseToSnakeCase)
		}
	}

	scanK8SResource := func(r *schema.Resource) {
		mod := getMod(r.Token)
		mod.resources = append(mod.resources, r)

		// For k8s, all nested properties will use a snake_case,
		// so we'll just add them all to the respective property
		// case maps.
		for _, p := range r.Properties {
			n := p.Name
			snakeCase := python.PyName(n)
			snakeCaseToCamelCase[snakeCase] = n
			camelCaseToSnakeCase[n] = snakeCase
		}

		for _, p := range r.InputProperties {
			n := p.Name
			snakeCase := python.PyName(n)
			snakeCaseToCamelCase[snakeCase] = n
			camelCaseToSnakeCase[n] = snakeCase
		}
	}

	glog.V(3).Infoln("scanning resources")
	if pkg.Name == "kubernetes" {
		scanK8SResource(pkg.Provider)
		for _, r := range pkg.Resources {
			scanK8SResource(r)
		}
	} else {
		scanResource(pkg.Provider)
		for _, r := range pkg.Resources {
			scanResource(r)
		}
	}
	glog.V(3).Infoln("done scanning resources")

	for _, f := range pkg.Functions {
		mod := getMod(f.Token)
		mod.functions = append(mod.functions, f)
	}
	return modules
}

// GeneratePackage generates the docs package with docs for each resource given the Pulumi
// schema.
func GeneratePackage(tool string, pkg *schema.Package) (map[string][]byte, error) {
	templates = template.New("").Funcs(template.FuncMap{
		"htmlSafe": func(html string) template.HTML {
			// Markdown fragments in the templates need to be rendered as-is,
			// so that html/template package doesn't try to inject data into it,
			// which will most certainly fail.
			// nolint gosec
			return template.HTML(html)
		},
		"pyName": func(str string) string {
			return python.PyName(str)
		},
	})

	for name, b := range packagedTemplates {
		template.Must(templates.New(name).Parse(string(b)))
	}

	defer glog.Flush()

	// Generate the modules from the schema, and for every module
	// run the generator functions to generate markdown files.
	modules := generateModulesFromSchemaPackage(tool, pkg)
	glog.V(3).Infoln("generating package now...")
	files := fs{}
	for _, mod := range modules {
		if err := mod.gen(files); err != nil {
			return nil, err
		}
	}

	return files, nil
}
