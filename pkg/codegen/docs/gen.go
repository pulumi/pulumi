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
	"html/template"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/codegen"
	"github.com/pulumi/pulumi/pkg/codegen/dotnet"
	go_gen "github.com/pulumi/pulumi/pkg/codegen/go"
	"github.com/pulumi/pulumi/pkg/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/codegen/python"
	"github.com/pulumi/pulumi/pkg/codegen/schema"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

var supportedLanguages = []string{"csharp", "go", "nodejs", "python"}

var templates *template.Template
var packagedTemplates map[string][]byte

// Header represents the header of each resource markdown file.
type Header struct {
	Title string
}

type exampleUsage struct {
	Heading string
	Code    string
}

// Property represents an input or an output property.
type Property struct {
	Name               string
	Comment            string
	Type               PropertyType
	DeprecationMessage string

	IsRequired bool
	// IsInput is a flag to indicate if a property is an input
	// property.
	IsInput bool
}

// DocNestedType represents a complex type.
type DocNestedType struct {
	Name        string
	APIDocLinks map[string]string
	Properties  map[string][]Property
}

// PropertyType represents the type of a property.
type PropertyType struct {
	Name string
	// Link can be a link to an anchor tag on the same
	// page, or to another page/site.
	Link string
}

// ConstructorParam represents the formal parameters of a constructor.
type ConstructorParam struct {
	Name string
	Type PropertyType

	// This is the language specific optional type indicator.
	// For example, in nodejs this is the character "?" and in Go
	// it's "*".
	OptionalFlag string

	DefaultValue string
}

type resourceArgs struct {
	Header

	Comment  string
	Examples []exampleUsage

	ConstructorParams map[string]string
	// ConstructorResource is the resource that is being constructed or
	// is the result of a constructor-like function.
	ConstructorResource map[string]PropertyType
	ArgsRequired        bool

	InputProperties  map[string][]Property
	OutputProperties map[string][]Property
	StateInputs      map[string][]Property
	StateParam       string

	NestedTypes []DocNestedType
}

type stringSet map[string]struct{}

func (ss stringSet) add(s string) {
	ss[s] = struct{}{}
}

type typeDetails struct {
	outputType   bool
	inputType    bool
	functionType bool
}

type modContext struct {
	pkg         *schema.Package
	mod         string
	resources   []*schema.Resource
	functions   []*schema.Function
	typeDetails map[*schema.ObjectType]*typeDetails
	children    []*modContext
	tool        string
}

func resourceName(r *schema.Resource) string {
	if r.IsProvider {
		return "Provider"
	}
	return tokenToName(r.Token)
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

func (mod *modContext) typeString(t schema.Type, lang string, input, optional bool, insertWordBreaks bool) PropertyType {
	var langType string

	var docLanguageHelper codegen.DocLanguageHelper
	switch lang {
	case "nodejs":
		docLanguageHelper = nodejs.DocLanguageHelper{}
	case "go":
		docLanguageHelper = go_gen.DocLanguageHelper{}
	case "csharp":
		docLanguageHelper = dotnet.DocLanguageHelper{}
	case "python":
		docLanguageHelper = python.DocLanguageHelper{}
	default:
		panic(errors.Errorf("Unknown language (%q) passed!", lang))
	}

	langType = docLanguageHelper.GetLanguageType(mod.pkg, mod.mod, t, input, optional)

	// If the type is an object type, let's also wrap it with a link to the supporting type
	// on the same page using an anchor tag.
	var href string
	switch t := t.(type) {
	case *schema.ArrayType:
		elementLangType := mod.typeString(t.ElementType, lang, input, optional, false)
		href = elementLangType.Link
	case *schema.ObjectType:
		tokenName := tokenToName(t.Token)
		// Links to anchor targs on the same page must be lower-cased.
		href = "#" + lower(tokenName)
	}

	if insertWordBreaks {
		langType = wbr(langType)
	}
	return PropertyType{
		Link: href,
		Name: langType,
	}
}

func (mod *modContext) genConstructorTS(r *schema.Resource, argsOptional bool) []ConstructorParam {
	name := resourceName(r)
	argsType := name + "Args"
	argsFlag := ""
	if argsOptional {
		argsFlag = "?"
	}

	docLangHelper := nodejs.DocLanguageHelper{}
	return []ConstructorParam{
		{
			Name: "name",
			Type: PropertyType{
				Name: "string",
				Link: nodejs.GetDocLinkForBuiltInType("string"),
			},
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			Type: PropertyType{
				Name: argsType,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, mod.mod, argsType),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			Type: PropertyType{
				Name: "pulumi.CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForResourceType("pulumi", "pulumi", "CustomResourceOptions"),
			},
		},
	}
}

func (mod *modContext) genConstructorGo(r *schema.Resource, argsOptional bool) []ConstructorParam {
	name := resourceName(r)
	argsType := name + "Args"
	argsFlag := ""
	if argsOptional {
		argsFlag = "*"
	}

	docLangHelper := go_gen.DocLanguageHelper{}
	// return fmt.Sprintf("func New%s(ctx *pulumi.Context, name string, args *%s, opts ...pulumi.ResourceOption) (*%s, error)\n", name, argsType, name)
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
			Name: "name",
			Type: PropertyType{
				Name: "string",
				Link: go_gen.GetDocLinkForBuiltInType("string"),
			},
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			Type: PropertyType{
				Name: argsType,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, mod.mod, argsType),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "...",
			Type: PropertyType{
				Name: "pulumi.ResourceOption",
				Link: "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/go/pulumi?tab=doc#ResourceOption",
			},
		},
	}
}

func (mod *modContext) genConstructorCS(r *schema.Resource, argsOptional bool) []ConstructorParam {
	name := resourceName(r)
	argsType := name + "Args"
	argsSchemaType := &schema.ObjectType{
		Token: r.Token,
	}
	argLangType := mod.typeString(argsSchemaType, "csharp", true, argsOptional, false)

	var argsFlag string
	var argsDefault string
	if argsOptional {
		// If the number of required input properties was zero, we can make the args object optional.
		argsDefault = " = null"
		argsFlag = "?"
	}

	optionsType := "Pulumi.CustomResourceOptions"
	if r.IsProvider {
		optionsType = "Pulumi.ResourceOptions"
	}

	docLangHelper := dotnet.DocLanguageHelper{}
	return []ConstructorParam{
		{
			Name: "name",
			Type: PropertyType{
				Name: "string",
				Link: "https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/builtin-types/built-in-types",
			},
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			DefaultValue: argsDefault,
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
				Name: optionsType,
				Link: docLangHelper.GetDocLinkForResourceType("", "", optionsType),
			},
		},
	}
}

func (mod *modContext) genNestedTypes(properties []*schema.Property, input bool) []DocNestedType {
	tokens := stringSet{}
	mod.getTypes(properties, tokens)

	var objs []DocNestedType
	for token := range tokens {
		for _, t := range mod.pkg.Types {
			if obj, ok := t.(*schema.ObjectType); ok && obj.Token == token {
				if len(obj.Properties) == 0 {
					continue
				}

				// Create maps to hold the per-language properties of this object and links to
				// the API doc fpr each language.
				props := make(map[string][]Property)
				apiDocLinks := make(map[string]string)
				for _, lang := range supportedLanguages {
					var docLangHelper codegen.DocLanguageHelper

					inputObjLangType := mod.typeString(t, lang, true /*input*/, true /*optional*/, false /*insertWordBreaks*/)
					switch lang {
					case "csharp":
						docLangHelper = dotnet.DocLanguageHelper{}
					case "go":
						docLangHelper = go_gen.DocLanguageHelper{}
					case "nodejs":
						docLangHelper = nodejs.DocLanguageHelper{}
					case "python":
						// Pulumi's Python language SDK does not have "types" yet, so we will skip it for now.
						continue
					default:
						panic(errors.Errorf("cannot generate nested type doc link for unhandled language %q", lang))
					}
					apiDocLinks[lang] = docLangHelper.GetDocLinkForInputType(mod.pkg.Name, mod.mod, inputObjLangType.Name)
					props[lang] = mod.getProperties(obj.Properties, lang, true)
				}

				objs = append(objs, DocNestedType{
					Name:        tokenToName(obj.Token),
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
func (mod *modContext) getProperties(properties []*schema.Property, lang string, isInput bool) []Property {
	if len(properties) == 0 {
		return nil
	}

	docProperties := make([]Property, 0, len(properties))
	for _, prop := range properties {
		if prop == nil {
			continue
		}
		docProperties = append(docProperties, Property{
			Name:               getLanguagePropertyName(prop.Name, lang),
			Comment:            prop.Comment,
			DeprecationMessage: prop.DeprecationMessage,
			IsRequired:         prop.IsRequired,
			IsInput:            isInput,
			Type:               mod.typeString(prop.Type, lang, isInput, !prop.IsRequired, true),
		})
	}

	return docProperties
}

func (mod *modContext) genConstructors(r *schema.Resource, allOptionalInputs bool) map[string]string {
	constructorParams := make(map[string]string)
	for _, lang := range supportedLanguages {
		var (
			paramTemplate string
			params        []ConstructorParam
		)
		b := &bytes.Buffer{}

		switch lang {
		case "nodejs":
			params = mod.genConstructorTS(r, allOptionalInputs)
			paramTemplate = "ts_constructor_param"
		case "go":
			params = mod.genConstructorGo(r, allOptionalInputs)
			paramTemplate = "go_constructor_param"
		case "csharp":
			params = mod.genConstructorCS(r, allOptionalInputs)
			paramTemplate = "csharp_constructor_param"
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
func (mod *modContext) getConstructorResourceInfo(resourceTypeName string) map[string]PropertyType {
	resourceMap := make(map[string]PropertyType)
	resourceDisplayName := resourceTypeName

	for _, lang := range supportedLanguages {
		var docLangHelper codegen.DocLanguageHelper
		switch lang {
		case "nodejs":
			docLangHelper = nodejs.DocLanguageHelper{}
		case "go":
			docLangHelper = go_gen.DocLanguageHelper{}
		case "csharp":
			docLangHelper = dotnet.DocLanguageHelper{}
			resourceTypeName = fmt.Sprintf("Pulumi.%s.%s.%s", strings.Title(mod.pkg.Name), strings.Title(mod.mod), resourceTypeName)
		case "python":
			// Pulumi's Python language SDK does not have "types" yet, so we will skip it for now.
			continue
		default:
			panic(errors.Errorf("cannot generate constructor info for unhandled language %q", lang))
		}

		resourceMap[lang] = PropertyType{
			Name: resourceDisplayName,
			Link: docLangHelper.GetDocLinkForResourceType(mod.pkg.Name, mod.mod, resourceTypeName),
		}
	}

	return resourceMap
}

// genResource is the entrypoint for generating a doc for a resource
// from its Pulumi schema.
func (mod *modContext) genResource(r *schema.Resource) resourceArgs {
	// Create a resource module file into which all of this resource's types will go.
	name := resourceName(r)

	// TODO: Unlike the other languages, Python does not have a separate Args object for inputs.
	// The args are all just named parameters of the constructor. Consider injecting
	// `resource_name` and `opts` as the first two items in the table of properties.
	inputProps := make(map[string][]Property)
	outputProps := make(map[string][]Property)
	stateInputs := make(map[string][]Property)
	for _, lang := range supportedLanguages {
		inputProps[lang] = mod.getProperties(r.InputProperties, lang, true)
		if r.IsProvider {
			continue
		}
		outputProps[lang] = mod.getProperties(r.Properties, lang, false)
		if r.StateInputs != nil {
			stateInputs[lang] = mod.getProperties(r.StateInputs.Properties, lang, true)
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

	data := resourceArgs{
		Header: Header{
			Title: name,
		},

		Comment: r.Comment,
		// TODO: This is just temporary to include some data we don't have available yet.
		Examples: mod.getMockupExamples(r),

		ConstructorParams:   mod.genConstructors(r, allOptionalInputs),
		ConstructorResource: mod.getConstructorResourceInfo(name),
		ArgsRequired:        !allOptionalInputs,

		InputProperties:  inputProps,
		OutputProperties: outputProps,
		StateInputs:      stateInputs,
		StateParam:       name + "State",
		NestedTypes:      mod.genNestedTypes(r.InputProperties, true),
	}

	return data
}

func (mod *modContext) genFunction(w io.Writer, fun *schema.Function) {
	fmt.Fprintf(w, "%s\n\n", fun.Comment)

	// TODO: Emit the page for functions, similar to the page for resources.
	fmt.Fprintf(w, "TODO\n\n")
}

func visitObjectTypes(t schema.Type, visitor func(*schema.ObjectType)) {
	switch t := t.(type) {
	case *schema.ArrayType:
		visitObjectTypes(t.ElementType, visitor)
	case *schema.MapType:
		visitObjectTypes(t.ElementType, visitor)
	case *schema.ObjectType:
		for _, p := range t.Properties {
			visitObjectTypes(p.Type, visitor)
		}
		visitor(t)
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
			visitObjectTypes(e, visitor)
		}
	}
}

func (mod *modContext) getNestedTypes(t schema.Type, types stringSet) {
	switch t := t.(type) {
	case *schema.ArrayType:
		mod.getNestedTypes(t.ElementType, types)
	case *schema.MapType:
		mod.getNestedTypes(t.ElementType, types)
	case *schema.ObjectType:
		types.add(t.Token)
		mod.getTypes(t.Properties, types)
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
			mod.getNestedTypes(e, types)
		}
	}
}

func (mod *modContext) getTypes(member interface{}, types stringSet) {
	switch member := member.(type) {
	case *schema.ObjectType:
		for _, p := range member.Properties {
			mod.getNestedTypes(p.Type, types)
		}
	case *schema.Resource:
		for _, p := range member.Properties {
			mod.getNestedTypes(p.Type, types)
		}
		for _, p := range member.InputProperties {
			mod.getNestedTypes(p.Type, types)
		}
	case *schema.Function:
		if member.Inputs != nil {
			mod.getNestedTypes(member.Inputs, types)
		}
		if member.Outputs != nil {
			mod.getNestedTypes(member.Outputs, types)
		}
	case []*schema.Property:
		for _, p := range member {
			mod.getNestedTypes(p.Type, types)
		}
	}
}

func (mod *modContext) genHeader(w io.Writer, title string) {
	// TODO: Generate the actual front matter we want for these pages.
	// Example:
	// title: "Package @pulumi/aws"
	// title_tag: "Package @pulumi/aws | Node.js SDK"
	// linktitle: "@pulumi/aws"
	// meta_desc: "Explore members of the @pulumi/aws package."

	fmt.Fprintf(w, "---\n")
	fmt.Fprintf(w, "title: %q\n", title)
	fmt.Fprintf(w, "---\n\n")

	fmt.Fprintf(w, "<!-- WARNING: this file was generated by %v. -->\n", mod.tool)
	fmt.Fprintf(w, "<!-- Do not edit by hand unless you're certain you know what you are doing! -->\n\n")

	// TODO: Move styles into a .scss file in the docs repo instead of emitting it inline here.
	// Note: In general, we should prefer using TailwindCSS classes whenever possible.
	// These styles are only for elements that we can't easily add a class to.
	fmt.Fprintf(w, "<style>\n")
	fmt.Fprintf(w, "  table td p { margin-top: 0; margin-bottom: 0; }\n")
	fmt.Fprintf(w, "</style>\n\n")
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
		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, tokenToName(f.Token))

		mod.genFunction(buffer, f)

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
	if name == "" {
		name = mod.pkg.Name
	}

	mod.genHeader(w, name)

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

	return w.String()
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
	})

	for name, b := range packagedTemplates {
		template.Must(templates.New(name).Parse(string(b)))
	}

	// group resources, types, and functions into modules
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

	types := &modContext{pkg: pkg, mod: "types", tool: tool}

	for _, v := range pkg.Config {
		visitObjectTypes(v.Type, func(t *schema.ObjectType) { types.details(t).outputType = true })
	}

	scanResource := func(r *schema.Resource) {
		mod := getMod(r.Token)
		mod.resources = append(mod.resources, r)
		for _, p := range r.Properties {
			visitObjectTypes(p.Type, func(t *schema.ObjectType) { types.details(t).outputType = true })
		}
		for _, p := range r.InputProperties {
			visitObjectTypes(p.Type, func(t *schema.ObjectType) {
				if r.IsProvider {
					types.details(t).outputType = true
				}
				types.details(t).inputType = true
			})
		}
		if r.StateInputs != nil {
			visitObjectTypes(r.StateInputs, func(t *schema.ObjectType) { types.details(t).inputType = true })
		}
	}

	scanResource(pkg.Provider)
	for _, r := range pkg.Resources {
		scanResource(r)
	}

	for _, f := range pkg.Functions {
		mod := getMod(f.Token)
		mod.functions = append(mod.functions, f)
		if f.Inputs != nil {
			visitObjectTypes(f.Inputs, func(t *schema.ObjectType) {
				types.details(t).inputType = true
				types.details(t).functionType = true
			})
		}
		if f.Outputs != nil {
			visitObjectTypes(f.Outputs, func(t *schema.ObjectType) {
				types.details(t).outputType = true
				types.details(t).functionType = true
			})
		}
	}

	files := fs{}
	for _, mod := range modules {
		if err := mod.gen(files); err != nil {
			return nil, err
		}
	}

	return files, nil
}

// TODO: Remove this when we have real examples available.
func (mod *modContext) getMockupExamples(r *schema.Resource) []exampleUsage {

	if resourceName(r) != "Bucket" {
		return nil
	}

	examples := []exampleUsage{
		{
			Heading: "Private Bucket w/ Tags",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const bucket = new aws.s3.Bucket("b", {
	acl: "private",
	tags: {
		Environment: "Dev",
		Name: "My bucket",
	},
});
`,
		},
		{
			Heading: "Static Website Hosting",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import * as fs from "fs";

const bucket = new aws.s3.Bucket("b", {
	acl: "public-read",
	policy: fs.readFileSync("policy.json", "utf-8"),
	website: {
		errorDocument: "error.html",
		indexDocument: "index.html",
		routingRules: ` + "`" + `[{
	"Condition": {
		"KeyPrefixEquals": "docs/"
	},
	"Redirect": {
		"ReplaceKeyPrefixWith": "documents/"
	}
}]
` + "`" + `,
	},
});
`,
		},
		{
			Heading: "Using CORS",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const bucket = new aws.s3.Bucket("b", {
	acl: "public-read",
	corsRules: [{
		allowedHeaders: ["*"],
		allowedMethods: [
			"PUT",
			"POST",
		],
		allowedOrigins: ["https://s3-website-test.mydomain.com"],
		exposeHeaders: ["ETag"],
		maxAgeSeconds: 3000,
	}],
});
`,
		},
		{
			Heading: "Using versioning",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const bucket = new aws.s3.Bucket("b", {
	acl: "private",
	versioning: {
		enabled: true,
	},
});
`,
		},
		{
			Heading: "Enable Logging",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const logBucket = new aws.s3.Bucket("logBucket", {
	acl: "log-delivery-write",
});
const bucket = new aws.s3.Bucket("b", {
	acl: "private",
	loggings: [{
		targetBucket: logBucket.id,
		targetPrefix: "log/",
	}],
});
`,
		},
		{
			Heading: "Using object lifecycle",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const bucket = new aws.s3.Bucket("bucket", {
	acl: "private",
	lifecycleRules: [
		{
			enabled: true,
			expiration: {
				days: 90,
			},
			id: "log",
			prefix: "log/",
			tags: {
				autoclean: "true",
				rule: "log",
			},
			transitions: [
				{
					days: 30,
					storageClass: "STANDARD_IA", // or "ONEZONE_IA"
				},
				{
					days: 60,
					storageClass: "GLACIER",
				},
			],
		},
		{
			enabled: true,
			expiration: {
				date: "2016-01-12",
			},
			id: "tmp",
			prefix: "tmp/",
		},
	],
});
const versioningBucket = new aws.s3.Bucket("versioningBucket", {
	acl: "private",
	lifecycleRules: [{
		enabled: true,
		noncurrentVersionExpiration: {
			days: 90,
		},
		noncurrentVersionTransitions: [
			{
				days: 30,
				storageClass: "STANDARD_IA",
			},
			{
				days: 60,
				storageClass: "GLACIER",
			},
		],
		prefix: "config/",
	}],
	versioning: {
		enabled: true,
	},
});
`,
		},
		{
			Heading: "Using replication configuration",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const central = new aws.Provider("central", {
	region: "eu-central-1",
});
const replicationRole = new aws.iam.Role("replication", {
	assumeRolePolicy: ` + "`" + `{
	"Version": "2012-10-17",
	"Statement": [
	{
		"Action": "sts:AssumeRole",
		"Principal": {
		"Service": "s3.amazonaws.com"
		},
		"Effect": "Allow",
		"Sid": ""
	}
	]
}
` + "`" + `,
});
const destination = new aws.s3.Bucket("destination", {
	region: "eu-west-1",
	versioning: {
		enabled: true,
	},
});
const bucket = new aws.s3.Bucket("bucket", {
	acl: "private",
	region: "eu-central-1",
	replicationConfiguration: {
		role: replicationRole.arn,
		rules: [{
			destination: {
				bucket: destination.arn,
				storageClass: "STANDARD",
			},
			id: "foobar",
			prefix: "foo",
			status: "Enabled",
		}],
	},
	versioning: {
		enabled: true,
	},
}, {provider: central});
const replicationPolicy = new aws.iam.Policy("replication", {
	policy: pulumi.interpolate` + "`" + `{
	"Version": "2012-10-17",
	"Statement": [
	{
		"Action": [
		"s3:GetReplicationConfiguration",
		"s3:ListBucket"
		],
		"Effect": "Allow",
		"Resource": [
		"${bucket.arn}"
		]
	},
	{
		"Action": [
		"s3:GetObjectVersion",
		"s3:GetObjectVersionAcl"
		],
		"Effect": "Allow",
		"Resource": [
		"${bucket.arn}/*"
		]
	},
	{
		"Action": [
		"s3:ReplicateObject",
		"s3:ReplicateDelete"
		],
		"Effect": "Allow",
		"Resource": "${destination.arn}/*"
	}
	]
}
` + "`" + `,
});
const replicationRolePolicyAttachment = new aws.iam.RolePolicyAttachment("replication", {
	policyArn: replicationPolicy.arn,
	role: replicationRole.name,
});
`,
		},
		{
			Heading: "Enable Default Server Side Encryption",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const mykey = new aws.kms.Key("mykey", {
	deletionWindowInDays: 10,
	description: "This key is used to encrypt bucket objects",
});
const mybucket = new aws.s3.Bucket("mybucket", {
	serverSideEncryptionConfiguration: {
		rule: {
			applyServerSideEncryptionByDefault: {
				kmsMasterKeyId: mykey.arn,
				sseAlgorithm: "aws:kms",
			},
		},
	},
});
`,
		},
	}

	return examples
}
