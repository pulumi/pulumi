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
	"fmt"
	"net/url"
	"strings"

	"github.com/pgavlin/goldmark/ast"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type PulumiRefResolver func(ref DocRef) (string, bool)

func interpretPulumiRefs(types *types, node ast.Node, resolveRefToName PulumiRefResolver) error {
	var c, next ast.Node
	for c = node.FirstChild(); c != nil; c = next {
		err := interpretPulumiRefs(types, c, resolveRefToName)
		if err != nil {
			return err
		}

		next = c.NextSibling()
		switch c := c.(type) {
		case *Ref:
			ref := parseDocRef(c.Destination)
			if ref.Type == DocRefTypeUnknown {
				return fmt.Errorf("invalid doc ref: %s", c.Destination)
			}

			name, ok := resolveRefToName(ref)
			if !ok {
				if ref.Property != "" {
					name = ref.Property
				} else if ref.Token != "" {
					name = ref.Token.DisplayName()
				} else {
					name = ref.Ref
				}
			}

			textNode := ast.NewString([]byte(name))
			node.InsertAfter(node, c, textNode)
			node.RemoveChild(node, c)
		}
	}
	return nil
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
