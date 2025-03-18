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
	"net/url"
	"regexp"
	"strings"

	"github.com/pgavlin/goldmark/ast"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
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

// Matches the format: <pulumi ref="..."/>, allowing for additional whitespace.
var matchPulumiRef = regexp.MustCompile(`<pulumi\s+ref="([^"]*)"\s*\/>`)

func InterpretPulumiRefs(description string, resolveRefToName func(ref DocRef) (string, bool)) string {
	if description == "" {
		return ""
	}
	return matchPulumiRef.ReplaceAllStringFunc(description, func(match string) string {
		submatches := matchPulumiRef.FindStringSubmatch(match)
		if len(submatches) > 1 {
			ref := submatches[1]
			docRef := parseDocRef(ref)
			if name, ok := resolveRefToName(docRef); ok {
				return name
			}
			// Fallback to a default of the property name or token display name
			if docRef.Property != "" {
				return docRef.Property
			}
			if docRef.Token != "" {
				return docRef.Token.DisplayName()
			}
			return ref
		}
		return match
	})
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

	if len(parts) != 5 {
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
			return DocRef{Ref: ref, Type: DocRefType(topLevelType + "Property"), Token: token, Property: property}
		default:
			// Properties isn't a valid ref
			return docRefUnknown
		}
	case "inputProperties":
		switch topLevelType {
		case "resource", "function":
			return DocRef{Ref: ref, Type: DocRefType(topLevelType + "InputProperty"), Token: token, Property: property}
		default:
			// Properties isn't a valid ref
			return docRefUnknown
		}
	case "outputProperties":
		switch topLevelType {
		case "function":
			return DocRef{Ref: ref, Type: DocRefType(topLevelType + "OutputProperty"), Token: token, Property: property}
		default:
			// Properties isn't a valid ref
			return docRefUnknown
		}
	default:
		return docRefUnknown
	}
}
