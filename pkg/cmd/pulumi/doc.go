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

package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/gdamore/tcell/terminfo"
	"github.com/gdamore/tcell/terminfo/dynamic"
	"github.com/pgavlin/goldmark"
	"github.com/pgavlin/goldmark/extension"
	goldmark_parser "github.com/pgavlin/goldmark/parser"
	goldmark_renderer "github.com/pgavlin/goldmark/renderer"
	"github.com/pgavlin/goldmark/text"
	"github.com/pgavlin/goldmark/util"
	"github.com/pgavlin/markdown-kit/renderer"
	"github.com/pgavlin/markdown-kit/styles"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	dotnetgen "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	nodejsgen "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	pythongen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newDocCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "doc <member>[#/<json-pointer>]",
		Args:  cmdutil.ExactArgs(1),
		Short: "Display docs for a package member",
		Long: "Display docs for a package member.\n" +
			"\n" +
			"This command prints the documentation associated with the package\n" +
			"member identified by a JSON pointer. Package members include object\n" +
			"or enum type definitions, resource type definitions, function\n" +
			"definitions, property definitions, config value defintiions, and\n" +
			"provider definitions.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			pointer := args[0]

			// Fetch the project and filter examples to the project's language.
			proj, _, err := readProject()
			if err != nil {
				return fmt.Errorf("Failed to get docs: %w", err)
			}

			var lang string
			var helper codegen.DocLanguageHelper
			switch proj.Runtime.Name() {
			case "dotnet":
				lang, helper = "csharp", dotnetgen.DocLanguageHelper{}
			case "go":
				lang, helper = "go", gogen.DocLanguageHelper{}
			case langPython:
				lang, helper = langPython, pythongen.DocLanguageHelper{}
			default:
				lang, helper = "typescript", nodejsgen.DocLanguageHelper{}
			}

			docstring, ok, err := findDocstring(pointer, lang, helper)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("could not find package member %v", pointer)
			}
			docstring = codegen.FilterExamples(docstring, lang)
			if term.IsTerminal(int(os.Stdout.Fd())) {

				return renderLiveView(docstring)
			}
			// Rendering into a pipe
			return renderDocstring(os.Stdout, docstring)
		}),
	}

	return cmd
}

func getPackageSchema(pkg string) (*schema.Package, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ctx, err := plugin.NewContext(nil, nil, nil, nil, cwd, nil, false, nil)
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(ctx)

	loader := schema.NewPluginLoader(ctx.Host)
	return loader.LoadPackage(pkg, nil)
}

func findDocstring(pointer, lang string, helper codegen.DocLanguageHelper) (string, bool, error) {
	// path should be in `member` or `member#/pointer` format.
	pkgName, member := "", pointer
	if hash := strings.Index(pointer, "#"); hash != -1 {
		member, pointer = pointer[:hash], pointer[hash+1:]
	} else {
		pointer = ""
	}
	if memberToken, err := tokens.ParseModuleMember(member); err == nil {
		pkgName = string(memberToken.Package())
		if pkgName == "pulumi" && memberToken.Module() == "providers" {
			pkgName, member = string(memberToken.Name()), "provider"
		}
	} else {
		pkgName, member = member, ""
	}

	// get the package's schema
	pkg, err := getPackageSchema(pkgName)
	if err != nil {
		return "", false, err
	}

	object := interface{}(pkg)
	if member != "" {
		// find the referenced member
		if member == "provider" {
			object = pkg.Provider
		} else if res, ok := pkg.GetResource(member); ok {
			object = res
		} else if fn, ok := pkg.GetFunction(member); ok {
			object = fn
		} else if typ, ok := pkg.GetType(member); ok {
			object = typ.(schema.HasMembers)
		}
	}

	// resolve the JSON pointer, if any
	if pointer != "" {
		member, ok := object.(schema.HasMembers).GetMember(pointer)
		if !ok {
			return "", false, err
		}
		object = member
	}

	var docstring string
	err = nil
	switch object := object.(type) {
	case *schema.Property:
		docstring, err = genPropertyDocstring(object, pkg, lang, helper)
	case *schema.Enum:
		docstring = genEnumDocstring(object, lang, helper)
	case *schema.EnumType:
		docstring = genEnumTypeDocstring(object, lang, helper)
	case *schema.ObjectType:
		docstring = genObjectTypeDocstring(object, lang, helper)
	case *schema.Resource:
		docstring, err = genResourceDocstring(object, lang, helper)
	case *schema.Function:
		docstring = genFunctionDocstring(object, lang, helper)
	default:
		return "", false, fmt.Errorf("unexpected member of type %T", member)
	}
	return docstring, true, err
}

type propertySummary struct {
	Name string
	Type string
}

func summarizeProperty(property *schema.Property, pkg *schema.Package,
	helper codegen.DocLanguageHelper) (propertySummary, error) {
	name, err := helper.GetPropertyName(property)
	if err != nil {
		return propertySummary{}, err
	}

	typ := helper.GetLanguageTypeString(pkg, "", property.Type, false)

	return propertySummary{
		Name: name,
		Type: typ,
	}, nil
}

func summarizeProperties(properties []*schema.Property, pkg *schema.Package,
	helper codegen.DocLanguageHelper) ([]propertySummary, error) {
	summaries := make([]propertySummary, len(properties))
	for i, p := range properties {
		summary, err := summarizeProperty(p, pkg, helper)
		if err != nil {
			return nil, err
		}
		summaries[i] = summary
	}
	return summaries, nil
}

func summarizeObjectProperties(object *schema.ObjectType, pkg *schema.Package,
	helper codegen.DocLanguageHelper) ([]propertySummary, error) {
	if object != nil {
		return summarizeProperties(object.Properties, pkg, helper)
	}
	return nil, nil
}

//go:embed doc_property.tmpl
var propertyTemplateText string
var propertyTemplate = template.Must(template.New("property").Parse(propertyTemplateText))

func genPropertyDocstring(property *schema.Property, pkg *schema.Package,
	lang string, helper codegen.DocLanguageHelper) (string, error) {
	summary, err := summarizeProperty(property, pkg, helper)
	if err != nil {
		return "", err
	}

	context := map[string]interface{}{
		"Language":           lang,
		"Name":               summary.Name,
		"Type":               summary.Type,
		"DeprecationMessage": property.DeprecationMessage,
		"Secret":             property.Secret,
		"Comment":            property.Comment,
	}
	if property.ConstValue != nil {
		context["Constant"] = property.ConstValue
	} else {
		context["Constant"] = "<none>"
	}
	if property.DefaultValue != nil {
		context["Default"] = property.DefaultValue.Value
	} else {
		context["Default"] = "<none>"
	}

	var buf strings.Builder
	if err := propertyTemplate.Execute(&buf, context); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func genEnumDocstring(enum *schema.Enum, lang string, helper codegen.DocLanguageHelper) string {
	return enum.Comment
}

func genEnumTypeDocstring(enumType *schema.EnumType, lang string, helper codegen.DocLanguageHelper) string {
	return enumType.Comment
}

func genObjectTypeDocstring(object *schema.ObjectType, lang string, helper codegen.DocLanguageHelper) string {
	return object.Comment
}

//go:embed doc_resource.tmpl
var resourceTemplateText string
var resourceTemplate = template.Must(template.New("resource").Parse(resourceTemplateText))

func genResourceDocstring(resource *schema.Resource, lang string, helper codegen.DocLanguageHelper) (string, error) {
	properties, err := summarizeProperties(resource.Properties, resource.Package, helper)
	if err != nil {
		return "", err
	}
	inputProperties, err := summarizeProperties(resource.InputProperties, resource.Package, helper)
	if err != nil {
		return "", err
	}
	stateInputs, err := summarizeObjectProperties(resource.StateInputs, resource.Package, helper)
	if err != nil {
		return "", err
	}

	methods := make([]string, len(resource.Methods))
	for i, m := range resource.Methods {
		methods[i] = helper.GetMethodName(m)
	}

	context := map[string]interface{}{
		"Name":                 resource.Token,
		"Language":             lang,
		"DeprecationMessage":   resource.DeprecationMessage,
		"Comment":              resource.Comment,
		"Properties":           properties,
		"InputProperties":      inputProperties,
		"StateInputProperties": stateInputs,
		"Methods":              methods,
	}

	var buf strings.Builder
	if err := resourceTemplate.Execute(&buf, context); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func genFunctionDocstring(function *schema.Function, lang string, helper codegen.DocLanguageHelper) string {
	return function.Comment
}

func renderLiveView(docstring string) error {
	if ti, _, err := dynamic.LoadTerminfo(os.Getenv("TERM")); err == nil {
		terminfo.AddTerminfo(ti)
	}
	app := tview.NewApplication()
	reader := newMarkdownReader("Pulumi Docs", docstring, styles.Pulumi, app)
	app.SetRoot(reader, true)
	app.SetFocus(reader)
	return app.Run()
}

func renderDocstring(w io.Writer, docstring string) error {
	source := []byte(docstring)
	parser := goldmark.DefaultParser()
	parser.AddOptions(goldmark_parser.WithParagraphTransformers(
		util.Prioritized(extension.NewTableParagraphTransformer(), 200),
	))

	document := parser.Parse(text.NewReader(source))

	r := renderer.New(
		renderer.WithSoftBreak(false))
	renderer := goldmark_renderer.NewRenderer(goldmark_renderer.WithNodeRenderers(util.Prioritized(r, 100)))
	return renderer.Render(w, source, document)
}
