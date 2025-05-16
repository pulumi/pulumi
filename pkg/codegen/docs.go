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

package codegen

import (
	"github.com/pgavlin/goldmark/ast"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// DocLanguageHelper is an interface for extracting language-specific information from a Pulumi schema.
// See the implementation for this interface under each of the language code generators.
type DocLanguageHelper interface {
	// GetModuleName returns the in-language name of the module.
	//
	// For example, lets get the hypothetical name of the module for the "pkg:module/nestedMod:Type" token in python:
	//
	//	var python python_codegen.DocLanguageHelper
	//	python.GetModuleName(pkgRef, pkgRef.TokenToModule("pkg:module/nestedMod:Type")) // "module.nestedmod"
	GetModuleName(pkg schema.PackageReference, modName string) string

	GetPropertyName(p *schema.Property) (string, error)
	GetEnumName(e *schema.Enum, typeName string) (string, error)
	// GetTypeName gets the name of a type in the language of the DocLanguageHelper.
	//
	// relativeToModule describes the module that is consuming the type
	// name. Typically, GetTypeName will output an unqualified name if typ is native
	// to relativeToModule. Otherwise GetTypeName may return a qualified name.
	// relativeToModule should always be a module returned from pkg.TokenToModule. It
	// should not be language specialized.
	//
	// For example, lets get the name of a hypothetical python property type:
	//
	//	var pkg *schema.Package = getOurPackage(/* Schema{
	//		Name: "pkg",
	//		Resource: []{
	//			{
	//				Token: "pkg:myModule:Resource",
	//				Properties: []{
	//					{
	//						Type: Object{Name: "pkg:myModule:TheType"},
	//						Name: "theType",
	//					},
	//				},
	//			},
	//		},
	//	} */)
	//	var res *schema.Resource = pkg.Resources[i]
	//	var prop *schema.Property := res.Properties[j]
	//
	//	var python python_codegen.DocLanguageHelper
	//
	//	unqualifiedName := python.GetTypeName(pkg, prop.Type, false, pkg.TokenToModule(res.Token))
	//	fmt.Println(unqualifiedName) // Prints "TheType".
	//
	//	qualifiedName := python.GetTypeName(pkg, prop.Type, false, "")
	//	fmt.Println(qualifiedName) // Prints "my_module.TheType"
	GetTypeName(pkg schema.PackageReference, t schema.Type, input bool, relativeToModule string) string
	GetFunctionName(f *schema.Function) string

	// GetResourceFunctionResultName returns the name of the result type when a static resource function is used to lookup
	// an existing resource.
	GetResourceFunctionResultName(modName string, f *schema.Function) string

	// Methods
	GetMethodName(m *schema.Method) string
	GetMethodResultName(pkg schema.PackageReference, modName string, r *schema.Resource, m *schema.Method) string

	// Doc links
	GetDocLinkForResourceType(pkg *schema.Package, moduleName, typeName string) string
	GetDocLinkForPulumiType(pkg *schema.Package, typeName string) string
	GetDocLinkForResourceInputOrOutputType(pkg *schema.Package, moduleName, typeName string, input bool) string
	GetDocLinkForFunctionInputOrOutputType(pkg *schema.Package, moduleName, typeName string, input bool) string
}

func filterExamples(source []byte, node ast.Node, lang string) {
	var c, next ast.Node
	for c = node.FirstChild(); c != nil; c = next {
		filterExamples(source, c, lang)

		next = c.NextSibling()
		switch c := c.(type) {
		case *ast.FencedCodeBlock:
			sourceLang := string(c.Language(source))
			if sourceLang != lang && sourceLang != "sh" {
				node.RemoveChild(node, c)
			}
		case *schema.Shortcode:
			switch string(c.Name) {
			case schema.ExampleShortcode:
				hasCode := false
				for gc := c.FirstChild(); gc != nil; gc = gc.NextSibling() {
					if gc.Kind() == ast.KindFencedCodeBlock {
						hasCode = true
						break
					}
				}
				if hasCode {
					var grandchild, nextGrandchild ast.Node
					for grandchild = c.FirstChild(); grandchild != nil; grandchild = nextGrandchild {
						nextGrandchild = grandchild.NextSibling()
						node.InsertBefore(node, c, grandchild)
					}
				}
				node.RemoveChild(node, c)
			case schema.ExamplesShortcode:
				if first := c.FirstChild(); first != nil {
					first.SetBlankPreviousLines(c.HasBlankPreviousLines())
				}

				var grandchild, nextGrandchild ast.Node
				for grandchild = c.FirstChild(); grandchild != nil; grandchild = nextGrandchild {
					nextGrandchild = grandchild.NextSibling()
					node.InsertBefore(node, c, grandchild)
				}
				node.RemoveChild(node, c)
			}
		}
	}
}

// FilterExamples filters the code snippets in a schema docstring to include only those that target the given language.
func FilterExamples(description string, lang string) string {
	if description == "" {
		return ""
	}

	source := []byte(description)
	parsed := schema.ParseDocs(source)
	filterExamples(source, parsed, lang)
	return schema.RenderDocsToString(source, parsed)
}
