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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newDocCmd() *cobra.Command {
	var runtime string
	var schemas []string

	var cmd = &cobra.Command{
		Use:               "doc <member>[#/<json-pointer>]",
		Args:              cmdutil.ExactArgs(1),
		ValidArgsFunction: getValidDocArgs,
		Short:             "Display docs for a package member",
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
			if runtime == "" {
				r, err := getRuntimeName()
				if err != nil {
					return fmt.Errorf("Failed to get docs: %w", err)
				}
				runtime = r
			}
			res := newDocstringResolver(runtime)
			if err := loadSchemaList(res, schemas); err != nil {
				return err
			}
			docstring, err := res.findDocstring(pointer)
			if err != nil {
				return err
			}
			if term.IsTerminal(int(os.Stdout.Fd())) {
				return renderLiveView(pointer, docstring, res)
			}
			// Rendering into a pipe
			return renderDocstring(os.Stdout, docstring)
		}),
	}

	cmd.PersistentFlags().StringVarP(&runtime, "language", "l", "",
		"The language to use for example code. When unset, will default to the language of the current project "+
			"or Typescript if no project exists.")

	cmd.PersistentFlags().StringSliceVarP(&schemas, "schema", "s", nil,
		"A set of schema to be read in. These schema take precedence over standard package loading.")

	return cmd
}

func loadSchemaList(res *docstringResolver, schemas []string) error {
	for _, s := range schemas {
		// Load schema
		var spec schema.PackageSpec
		schemaBytes, err := ioutil.ReadFile(s)
		if err != nil {
			return fmt.Errorf("Failed to read schema %s: %w", s, err)
		}
		if err := json.Unmarshal(schemaBytes, &spec); err != nil {
			return err
		}

		p, diags, err := schema.BindSpec(spec, nil)
		if err != nil {
			return err
		}
		if diags.HasErrors() {
			return diags
		}
		res.storePackage(p)
	}
	return nil
}

func getRuntimeName() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Now that we got here, we have a path, so we will try to load it.
	path, err := workspace.DetectProjectPathFrom(pwd)
	if err != nil {
		return "", fmt.Errorf("failed to find current Pulumi project because of "+
			"an error when searching for the Pulumi.yaml file (searching upwards from %s)"+": %w", pwd, err)

	} else if path == "" {
		// Default to NodeJS if we're not within a project.
		return langNodejs, nil
	}
	proj, err := workspace.LoadProject(path)
	if err != nil {
		return "", fmt.Errorf("failed to load Pulumi project located at %q: %w", path, err)
	}
	return proj.Runtime.Name(), nil
}

type docstringResolver struct {
	lang   string
	helper codegen.DocLanguageHelper
	pkgs   []*schema.Package
}

func newDocstringResolver(runtime string) *docstringResolver {
	var lang string
	var helper codegen.DocLanguageHelper
	switch runtime {
	case "dotnet", "csharp":
		lang, helper = "csharp", dotnetgen.DocLanguageHelper{}
	case "go":
		lang, helper = "go", gogen.DocLanguageHelper{}
	case langPython:
		lang, helper = langPython, pythongen.DocLanguageHelper{}
	default:
		lang, helper = "typescript", nodejsgen.DocLanguageHelper{}
	}

	return &docstringResolver{lang, helper, nil}

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

func (r *docstringResolver) storePackage(pkg *schema.Package) {
	contract.Assert(pkg != nil)
	r.pkgs = append(r.pkgs, pkg)
}

func (r *docstringResolver) ensureSchema(pkgName string) (*schema.Package, error) {
	for _, p := range r.pkgs {
		if p.Name == pkgName {
			return p, nil
		}
	}
	pkg, err := getPackageSchema(pkgName)
	if err != nil {
		return nil, err
	}
	r.storePackage(pkg)
	return pkg, nil
}

type memberNotFound struct {
	pointer string
}

func (err *memberNotFound) Error() string {
	return fmt.Sprintf("could not find package member %v", err.pointer)
}

func (r *docstringResolver) findDocstring(pointer string) (string, error) {
	docstring, ok, err := r.findWholeDocstring(pointer)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", &memberNotFound{pointer}
	}
	return codegen.FilterExamples(docstring, r.lang), nil

}

func (r *docstringResolver) findWholeDocstring(pointer string) (string, bool, error) {
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
	pkg, err := r.ensureSchema(pkgName)
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

	switch object := object.(type) {
	case *schema.Property:
		docstring, err := genPropertyDocstring(object, pkg, r.lang, r.helper)
		return docstring, true, err
	case *schema.Enum:
		docstring := genEnumDocstring(object, r.lang, r.helper)
		return docstring, true, nil
	case *schema.EnumType:
		docstring := genEnumTypeDocstring(object, r.lang, r.helper)
		return docstring, true, nil
	case *schema.ObjectType:
		docstring, err := genObjectTypeDocstring(object, r.lang, r.helper)
		return docstring, true, err
	case *schema.Resource:
		docstring, err := genResourceDocstring(object, r.lang, r.helper)
		return docstring, true, err
	case *schema.Function:
		docstring := genFunctionDocstring(object, r.lang, r.helper)
		return docstring, true, nil
	case *schema.Package:
		docstring, err := genPackageDocstring(pkg)
		return docstring, true, err
	default:
		return "", false, fmt.Errorf("unexpected member of type %T", member)
	}
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

	// Union types in TypeScript are displayed with the `|` operator, which conflicts with how
	// markdown renders tables.
	if strings.Contains(typ, "|") {
		typ = fmt.Sprintf("`%s`", typ)
	}
	if path := getSchemaPath(property.Type, pkg); path != "" {
		typ = fmt.Sprintf("[%s](%s)", typ, path)
	}

	return propertySummary{
		Name: name,
		Type: typ,
	}, nil
}

func getSchemaPath(t schema.Type, pkg *schema.Package) string {
	t = codegen.UnwrapType(t)
	if schema.IsPrimitiveType(t) {
		return ""
	}
	switch t := t.(type) {
	// We are really interested in the element type. We link directly to that.
	case *schema.MapType:
		return getSchemaPath(t.ElementType, pkg)
	case *schema.ArrayType:
		return getSchemaPath(t.ElementType, pkg)

	// These are package specific types. We link to these.
	case *schema.EnumType:
		return t.Token
	case *schema.ResourceType:
		return t.Token
	case *schema.ObjectType:
		return t.Token
	default:
		return ""
	}
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

//go:embed doc_object_type.tmpl
var objectTemplateText string
var objectTemplate = template.Must(template.New("object").Parse(objectTemplateText))

func genObjectTypeDocstring(object *schema.ObjectType, lang string, helper codegen.DocLanguageHelper) (string, error) {
	properties, err := summarizeProperties(object.Properties, object.Package, helper)
	if err != nil {
		return "", err
	}
	context := map[string]interface{}{
		"Name":       object.Token,
		"Comment":    object.Comment,
		"Properties": properties,
	}
	var buf strings.Builder
	if err := objectTemplate.Execute(&buf, context); err != nil {
		return "", err
	}
	return buf.String(), nil
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

//go:embed doc_package.tmpl
var packageTemplateText string
var packageTemplate = template.Must(template.New("package").Funcs(template.FuncMap{
	"getTypes": getPackageDefinedTypes,
}).Parse(packageTemplateText))

func getPackageDefinedTypes(pkg *schema.Package) []schema.Type {
	var types []schema.Type
	for _, t := range pkg.Types {
		switch t := t.(type) {
		case *schema.EnumType, *schema.ObjectType:
			types = append(types, t)
		}
	}
	return types
}

func genPackageDocstring(pkg *schema.Package) (string, error) {
	var buf strings.Builder
	if err := packageTemplate.Execute(&buf, pkg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderLiveView(title, docstring string, r *docstringResolver) error {
	if ti, _, err := dynamic.LoadTerminfo(os.Getenv("TERM")); err == nil {
		terminfo.AddTerminfo(ti)
	}
	app := tview.NewApplication()
	reader := newMarkdownReader(title, docstring, styles.Pulumi, app)
	reader.externalLinkResolver = func(link string, reader *markdownReader) (bool, error) {
		docstring, err := r.findDocstring(link)
		memberError := &memberNotFound{}
		if errors.Is(err, memberError) {
			return false, nil
		}
		if err != nil {
			return true, err
		}
		reader.SetSource(link, docstring)
		return true, nil
	}
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
		renderer.WithSoftBreak(false),
		renderer.WithHyperlinks(true),
	)
	renderer := goldmark_renderer.NewRenderer(
		goldmark_renderer.WithNodeRenderers(util.Prioritized(r, 100)))
	return renderer.Render(w, source, document)
}

func getValidDocArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// do we have a valid member token? if so, list available pointers
	member := ""
	if hash := strings.Index(toComplete, "#"); hash != -1 {
		member = toComplete[:hash]
	}
	if memberToken, err := tokens.ParseModuleMember(member); err == nil {
		pointers, err := listAvailablePointers(memberToken)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return pointers, cobra.ShellCompDirectiveDefault
	}

	// do we have a partial member token from which we can pull a package name? if so, list available tokens
	if idx := strings.IndexByte(member, ':'); idx != -1 {
		tokens, err := listAvailableTokens(member[:idx])
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return tokens, cobra.ShellCompDirectiveDefault
	}

	// otherwise, return either the list of pacakges or the list of available tokens if the package is present in
	// the list.
	packages, err := listAvailablePackages()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	for _, p := range packages {
		if p == toComplete {
			tokens, err := listAvailableTokens(p)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			return tokens, cobra.ShellCompDirectiveDefault
		}
	}
	return packages, cobra.ShellCompDirectiveDefault
}

func listAvailablePackages() ([]string, error) {
	plugins, err := workspace.GetPlugins()
	if err != nil {
		return nil, err
	}

	var pkgs []string
	for _, p := range plugins {
		if p.Kind == workspace.ResourcePlugin {
			pkgs = append(pkgs, p.Name)
		}
	}
	return pkgs, nil
}

func listAvailableTokens(pkgName string) ([]string, error) {
	pkg, err := getPackageSchema(pkgName)
	if err != nil {
		return nil, err
	}

	var tokens []string
	for _, r := range pkg.Resources {
		tokens = append(tokens, r.Token)
	}
	for _, f := range pkg.Functions {
		tokens = append(tokens, f.Token)
	}
	for _, t := range pkg.Types {
		switch t := t.(type) {
		case *schema.EnumType:
			tokens = append(tokens, t.Token)
		case *schema.ObjectType:
			tokens = append(tokens, t.Token)
		}
	}
	return tokens, nil
}

func listAvailablePointers(member tokens.ModuleMember) ([]string, error) {
	pkg, err := getPackageSchema(string(member.Package()))
	if err != nil {
		return nil, err
	}

	object := interface{}(pkg)
	if member != "" {
		// find the referenced member
		if member == "provider" {
			object = pkg.Provider
		} else if res, ok := pkg.GetResource(string(member)); ok {
			object = res
		} else if fn, ok := pkg.GetFunction(string(member)); ok {
			object = fn
		} else if typ, ok := pkg.GetType(string(member)); ok {
			object = typ.(schema.HasMembers)
		}
	}

	return object.(schema.HasMembers).ListMembers(), nil
}
