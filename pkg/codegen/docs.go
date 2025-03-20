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
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/pgavlin/goldmark/ast"
	"github.com/pgavlin/goldmark/renderer"
	"github.com/pgavlin/goldmark/renderer/markdown"
	"github.com/pgavlin/goldmark/util"
	"golang.org/x/net/html"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// DocLanguageHelper is an interface for extracting language-specific information from a Pulumi schema.
// See the implementation for this interface under each of the language code generators.
type DocLanguageHelper interface {
	GetPropertyName(p *schema.Property) (string, error)
	GetEnumName(e *schema.Enum, typeName string) (string, error)
	GetDocLinkForResourceType(pkg *schema.Package, moduleName, typeName string) string
	GetDocLinkForPulumiType(pkg *schema.Package, typeName string) string
	GetDocLinkForResourceInputOrOutputType(pkg *schema.Package, moduleName, typeName string, input bool) string
	GetDocLinkForFunctionInputOrOutputType(pkg *schema.Package, moduleName, typeName string, input bool) string
	GetLanguageTypeString(pkg *schema.Package, moduleName string, t schema.Type, input bool) string

	GetFunctionName(modName string, f *schema.Function) string
	// GetResourceFunctionResultName returns the name of the result type when a static resource function is used to lookup
	// an existing resource.
	GetResourceFunctionResultName(modName string, f *schema.Function) string

	GetMethodName(m *schema.Method) string
	GetMethodResultName(pkg *schema.Package, modName string, r *schema.Resource, m *schema.Method) string

	// GetModuleDocLink returns the display name and the link for a module (including root modules) in a given package.
	GetModuleDocLink(pkg *schema.Package, modName string) (string, string)
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

type PulumiRefResolver func(ref DocRef) (string, bool)

func InterpretPulumiRefs(description string, resolveRefToName PulumiRefResolver) string {
	if description == "" {
		return ""
	}

	source := []byte(description)
	parsed := schema.ParseDocs(source)

	md := &markdown.Renderer{}

	refRenderer := &pulumiRefNodeRenderer{resolveRefToName, 0}
	r := renderer.NewRenderer(renderer.WithNodeRenderers(
		util.Prioritized(refRenderer, 100),
		util.Prioritized(schema.NewRenderer(), 150),
		util.Prioritized(md, 200),
	))
	var buf bytes.Buffer
	err := r.Render(&buf, source, parsed)
	contract.AssertNoErrorf(err, "error rendering docs")
	// Avoid reformatting if no refs found.
	if refRenderer.renderedCount == 0 {
		return description
	}
	newDescription := buf.String()
	// Cut trailing newline if the original description didn't have one.
	if !strings.HasSuffix(description, "\n") {
		newDescription, _ = strings.CutSuffix(newDescription, "\n")
	}
	return newDescription
}

type pulumiRefNodeRenderer struct {
	resolveRefToName PulumiRefResolver
	renderedCount    int
}

func (r *pulumiRefNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindRawHTML,
		func(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}

			raw := n.(*ast.RawHTML)
			var htmlSource bytes.Buffer
			for i := range raw.Segments.Len() {
				segment := raw.Segments.At(i)
				htmlSource.Write(segment.Value(source))
			}

			ref, ok := parsePulumiRef(htmlSource.String())
			var err error
			if !ok {
				_, err = writer.Write(htmlSource.Bytes())
			} else {
				name, ok := r.resolveRefToName(ref)
				if !ok {
					name = defaultRefRender(ref)
				}
				_, err = writer.Write([]byte(name))
				r.renderedCount++
			}
			if err != nil {
				return ast.WalkStop, fmt.Errorf("error writing pulumi ref: %w", err)
			}

			return ast.WalkSkipChildren, nil
		},
	)
}

func defaultRefRender(docRef DocRef) string {
	if docRef.Property != "" {
		return docRef.Property
	}
	if docRef.Token != "" {
		return docRef.Token.DisplayName()
	}
	return docRef.Ref
}

func parsePulumiRef(htmlFragment string) (DocRef, bool) {
	nodes, err := html.ParseFragment(strings.NewReader(htmlFragment), &html.Node{Type: html.ElementNode})
	if err != nil || len(nodes) != 1 {
		return DocRef{}, false
	}
	node := nodes[0]
	if node.Type != html.ElementNode || node.Data != "pulumi" {
		return DocRef{}, false
	}

	for _, attr := range node.Attr {
		if attr.Key == "ref" {
			return parseDocRef(attr.Val), true
		}
	}
	return DocRef{}, false
}

type DocRefType string

const (
	DocRefTypeUnknown                DocRefType = ""
	DocRefTypeResource               DocRefType = "resource"
	DocRefTypeFunction               DocRefType = "function"
	DocRefTypeType                   DocRefType = "type"
	DocRefTypeResourceProperty       DocRefType = "resourceProperty"
	DocRefTypeResourceInputProperty  DocRefType = "resourceInputProperty"
	DocRefTypeFunctionInputProperty  DocRefType = "functionInputProperty"
	DocRefTypeFunctionOutputProperty DocRefType = "functionOutputProperty"
	DocRefTypeTypeProperty           DocRefType = "typeProperty"
)

type DocRef struct {
	// Original parsed ref
	Ref string
	// The type of the parsed ref
	Type DocRefType
	// The token of the resource, function, or type, or empty if not applicable.
	Token tokens.Type
	// The referenced property name, or empty if not applicable.
	Property string
}

// Returns true is the current reference is a property within the other reference.
func (r DocRef) IsWithin(other DocRef) bool {
	switch r.Type {
	case DocRefTypeResourceProperty, DocRefTypeResourceInputProperty:
		return other.Type == DocRefTypeResource && r.Token == other.Token
	case DocRefTypeFunctionInputProperty, DocRefTypeFunctionOutputProperty:
		return other.Type == DocRefTypeFunction && r.Token == other.Token
	case DocRefTypeTypeProperty:
		return other.Type == DocRefTypeType && r.Token == other.Token
	case DocRefTypeResource, DocRefTypeFunction, DocRefTypeType, DocRefTypeUnknown:
		return false
	}
	return false
}

// NewDocRef constructs a validated DocRef from the given type, token, and property.
// `token` is required for all types except `DocRefTypeUnknown`.
// `property` is required for property types.
func NewDocRef(docRefType DocRefType, token string, property string) DocRef {
	var ref string
	switch docRefType {
	case DocRefTypeResource:
		contract.Assertf(token != "", "resource token must be provided")
		contract.Assertf(property == "", "property name must not be set for resource")
		ref = "#/resources/" + url.PathEscape(token)
	case DocRefTypeFunction:
		contract.Assertf(token != "", "function token must be provided")
		contract.Assertf(property == "", "property name must not be set for function")
		ref = "#/functions/" + url.PathEscape(token)
	case DocRefTypeType:
		contract.Assertf(token != "", "type token must be provided")
		contract.Assertf(property == "", "property name must not be set for type")
		ref = "#/types/" + url.PathEscape(token)
	case DocRefTypeResourceProperty:
		contract.Assertf(token != "", "resource token must be provided")
		contract.Assertf(property != "", "property name must be provided")
		ref = fmt.Sprintf("#/resources/%s/properties/%s", url.PathEscape(token), url.PathEscape(property))
	case DocRefTypeResourceInputProperty:
		contract.Assertf(token != "", "resource token must be provided")
		contract.Assertf(property != "", "property name must be provided")
		ref = fmt.Sprintf("#/resources/%s/inputProperties/%s", url.PathEscape(token), url.PathEscape(property))
	case DocRefTypeFunctionInputProperty:
		contract.Assertf(token != "", "function token must be provided")
		contract.Assertf(property != "", "property name must be provided")
		ref = fmt.Sprintf("#/functions/%s/inputs/properties/%s", url.PathEscape(token), url.PathEscape(property))
	case DocRefTypeFunctionOutputProperty:
		contract.Assertf(token != "", "function token must be provided")
		contract.Assertf(property != "", "property name must be provided")
		ref = fmt.Sprintf("#/functions/%s/outputs/properties/%s", url.PathEscape(token), url.PathEscape(property))
	case DocRefTypeTypeProperty:
		contract.Assertf(token != "", "type token must be provided")
		contract.Assertf(property != "", "property name must be provided")
		ref = fmt.Sprintf("#/types/%s/properties/%s", url.PathEscape(token), url.PathEscape(property))
	case DocRefTypeUnknown:
		return DocRef{Type: DocRefTypeUnknown}
	default:
		contract.Failf("unsupported doc ref type %s", docRefType)
	}
	return DocRef{
		Ref:      ref,
		Type:     docRefType,
		Token:    tokens.Type(token),
		Property: property,
	}
}

// Parses a doc reference string into a DocRef struct.
// The supported formats of the ref is:
//
//	#/resources/{token}
//	#/functions/{token}
//	#/types/{token}
//	#/resources/{token}/properties/{property}
//	#/resources/{token}/inputProperties/{property}
//	#/functions/{token}/inputProperties/{property}
//	#/functions/{token}/outputProperties/{property}
//	#/types/{token}/properties/{property}
//
// Note: Tokens containing a slash ("/") must be encoded as "%2F".
func parseDocRef(ref string) DocRef {
	docRefUnknown := DocRef{Ref: ref, Type: DocRefTypeUnknown}
	parts := strings.Split(ref, "/")
	if len(parts) < 3 || parts[0] != "#" {
		return docRefUnknown
	}
	// Extract which top-level type the ref is referring to.
	var topLevelType string
	switch parts[1] {
	case "resources", "functions", "types":
		// Strip "s" from the end of the type.
		topLevelType = parts[1][:len(parts[1])-1]
	default:
		return docRefUnknown
	}
	tokenString, err := url.PathUnescape(parts[2])
	if err != nil || tokenString == "" {
		return docRefUnknown
	}
	token, err := tokens.ParseTypeToken(tokenString)
	if err != nil {
		return docRefUnknown
	}

	if len(parts) == 3 {
		return DocRef{Ref: ref, Type: DocRefType(topLevelType), Token: token}
	}

	if len(parts) < 5 {
		return docRefUnknown
	}

	property, err := url.PathUnescape(parts[4])
	if err != nil || property == "" {
		return docRefUnknown
	}

	// Extract the property kind and name.
	switch parts[3] {
	case "properties":
		switch topLevelType {
		case "resource", "type":
			if len(parts) == 5 {
				return DocRef{Ref: ref, Type: DocRefType(topLevelType + "Property"), Token: token, Property: property}
			}
			return docRefUnknown
		default:
			// Properties isn't a valid ref
			return docRefUnknown
		}
	case "inputProperties":
		switch {
		case topLevelType == "resource" && len(parts) == 5:
			return DocRef{Ref: ref, Type: DocRefType(topLevelType + "InputProperty"), Token: token, Property: property}
		default:
			// Properties isn't a valid ref
			return docRefUnknown
		}
	case "inputs":
		switch {
		case topLevelType == "function" && parts[4] == "properties" && len(parts) == 6:
			property, err := url.PathUnescape(parts[5])
			if err != nil || property == "" {
				return docRefUnknown
			}
			return DocRef{Ref: ref, Type: DocRefTypeFunctionInputProperty, Token: token, Property: property}
		default:
			// Inputs isn't a valid ref
			return docRefUnknown
		}
	case "outputs":
		switch {
		case topLevelType == "function" && parts[4] == "properties" && len(parts) == 6:
			property, err := url.PathUnescape(parts[5])
			if err != nil || property == "" {
				return docRefUnknown
			}
			return DocRef{Ref: ref, Type: DocRefTypeFunctionOutputProperty, Token: token, Property: property}
		default:
			// Outputs isn't a valid ref
			return docRefUnknown
		}
	default:
		return docRefUnknown
	}
}
