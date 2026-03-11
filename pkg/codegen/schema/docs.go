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

package schema

import (
	"net/url"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pgavlin/goldmark/ast"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type PulumiRefResolver func(ref DocRef) (string, bool)

func interpretPulumiRefs(path string, types *types, options ValidationOptions, node ast.Node, resolveRefToName PulumiRefResolver) hcl.Diagnostics {
	var diags hcl.Diagnostics

	var c, next ast.Node
	for c = node.FirstChild(); c != nil; c = next {
		subdiags := interpretPulumiRefs(path, types, options, c, resolveRefToName)
		diags = append(diags, subdiags...)

		next = c.NextSibling()
		switch c := c.(type) {
		case *Ref:
			var subdiags hcl.Diagnostics
			var ref DocRef

			iref := parseDocRef(c.Destination)
			if iref.Kind == DocRefKindUnknown {
				subdiags = hcl.Diagnostics{errorf(path, "invalid doc ref: %s", c.Destination)}
			} else {
				ref = DocRef{Ref: iref.Ref, Kind: iref.Kind, Property: iref.Property}
				switch iref.Kind {
				case DocRefKindResource, DocRefKindResourceProperty, DocRefKindResourceInputProperty:
					res, ok := types.resources[string(iref.Token)]
					if !ok {
						subdiags = hcl.Diagnostics{errorf(path, "reference to resource '%s' not found in package %s",
							iref.Ref[1:], types.pkg.Name)}
					} else {
						ref.Type = res
					}
				case DocRefKindType, DocRefKindTypeProperty:
					typ, ok := types.typeDefs[string(iref.Token)]
					if !ok {
						subdiags = hcl.Diagnostics{errorf(path, "reference to type '%s' not found in package %s",
							iref.Ref[1:], types.pkg.Name)}
					} else {
						ref.Type = typ
					}
				case DocRefKindFunction, DocRefKindFunctionInputProperty, DocRefKindFunctionOutputProperty:
					fun, has := types.functionDefs[string(iref.Token)]
					if !has {
						subdiags = hcl.Diagnostics{errorf(path, "function %s not found", iref.Token)}
					}
					ref.Function = fun
				}
			}

			var name string
			var ok bool
			if !subdiags.HasErrors() {
				name, ok = resolveRefToName(ref)
			}
			diags = append(diags, subdiags...)

			if !ok {
				if ref.Property != "" {
					name = ref.Property
				} else if ref.Type != nil {
					name = ref.Type.String()
				} else if ref.Function != nil {
					name = ref.Function.Token
				} else {
					name = ref.Ref
				}
			}

			textNode := ast.NewString([]byte(name))
			node.InsertAfter(node, c, textNode)
			node.RemoveChild(node, c)
		}
	}
	return diags
}

type DocRefKind string

const (
	DocRefKindUnknown                DocRefKind = ""
	DocRefKindResource               DocRefKind = "resource"
	DocRefKindFunction               DocRefKind = "function"
	DocRefKindType                   DocRefKind = "type"
	DocRefKindResourceProperty       DocRefKind = "resourceProperty"
	DocRefKindResourceInputProperty  DocRefKind = "resourceInputProperty"
	DocRefKindFunctionInputProperty  DocRefKind = "functionInputProperty"
	DocRefKindFunctionOutputProperty DocRefKind = "functionOutputProperty"
	DocRefKindTypeProperty           DocRefKind = "typeProperty"
)

type DocRef struct {
	// Original parsed ref
	Ref string
	// The type of the parsed ref
	Kind DocRefKind
	// If a ref for a resource or type, the bound type.
	Type Type
	// If a ref for a function, the bound function.
	Function *Function
	// The referenced property name, or empty if not applicable.
	Property string
}

type internalDocRef struct {
	// Original parsed ref
	Ref string
	// The type of the parsed ref
	Kind DocRefKind
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
func parseDocRef(ref string) internalDocRef {
	docRefUnknown := internalDocRef{Ref: ref, Kind: DocRefKindUnknown}
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
		return internalDocRef{Ref: ref, Kind: DocRefKind(topLevelType), Token: token}
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
				return internalDocRef{Ref: ref, Kind: DocRefKind(topLevelType + "Property"), Token: token, Property: property}
			}
			return docRefUnknown
		default:
			// Properties isn't a valid ref
			return docRefUnknown
		}
	case "inputProperties":
		switch {
		case topLevelType == "resource" && len(parts) == 5:
			return internalDocRef{Ref: ref, Kind: DocRefKind(topLevelType + "InputProperty"), Token: token, Property: property}
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
			return internalDocRef{Ref: ref, Kind: DocRefKindFunctionInputProperty, Token: token, Property: property}
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
			return internalDocRef{Ref: ref, Kind: DocRefKindFunctionOutputProperty, Token: token, Property: property}
		default:
			// Outputs isn't a valid ref
			return docRefUnknown
		}
	default:
		return docRefUnknown
	}
}
