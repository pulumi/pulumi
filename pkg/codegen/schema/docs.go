// Copyright 2026, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// PulumiRefResolver resolves a parsed doc reference to the textual name that should be substituted into the
// surrounding documentation. It returns the substituted name and a boolean indicating whether the ref was
// resolved; if false, the caller falls back to a default rendering of the ref.
type PulumiRefResolver func(ref DocRef) (string, bool)

// interpretPulumiRefs parses a {{% ref %}} shortcode node and passes it to the `resolveRefToName` function to replace
// the shortcode with literal text.
func interpretPulumiRefs(
	path string, types *types, options ValidationOptions,
	node ast.Node, resolveRefToName PulumiRefResolver,
) hcl.Diagnostics {
	var diags hcl.Diagnostics

	var c, next ast.Node
	for c = node.FirstChild(); c != nil; c = next {
		subdiags := interpretPulumiRefs(path, types, options, c, resolveRefToName)
		diags = append(diags, subdiags...)

		next = c.NextSibling()

		cref, ok := c.(*Ref)
		if !ok {
			continue
		}

		iref := parseDocRef(cref.Destination)
		ref := DocRef{Ref: iref.Ref, Kind: iref.Kind, Property: iref.Property}
		switch iref.Kind {
		case DocRefKindUnknown:
			subdiags = hcl.Diagnostics{errorf(path, "invalid doc ref: %s", cref.Destination)}
		case DocRefKindResource, DocRefKindResourceProperty, DocRefKindResourceInputProperty:
			res, ok := types.resources[string(iref.Token)]
			if !ok {
				subdiags = hcl.Diagnostics{errorf(path, "reference to resource '%s' not found in package %s",
					iref.Ref[1:], types.pkg.Name)}
			} else {
				ref.Type = res
				switch iref.Kind { //nolint:exhaustive
				case DocRefKindResourceProperty:
					if !hasPropertyNamed(res.Resource.Properties, iref.Property) {
						subdiags = hcl.Diagnostics{errorf(path,
							"property '%s' not found on resource '%s'", iref.Property, iref.Token)}
					}
				case DocRefKindResourceInputProperty:
					if !hasPropertyNamed(res.Resource.InputProperties, iref.Property) {
						subdiags = hcl.Diagnostics{errorf(path,
							"input property '%s' not found on resource '%s'", iref.Property, iref.Token)}
					}
				default:
					contract.Failf("unexpected resource ref kind: %v", iref.Kind)
				}
			}
		case DocRefKindType, DocRefKindTypeProperty:
			typ, ok := types.typeDefs[string(iref.Token)]
			if !ok {
				subdiags = hcl.Diagnostics{errorf(path, "reference to type '%s' not found in package %s",
					iref.Ref[1:], types.pkg.Name)}
			} else {
				ref.Type = typ
				if iref.Kind == DocRefKindTypeProperty {
					obj, isObj := typ.(*ObjectType)
					if !isObj {
						subdiags = hcl.Diagnostics{errorf(path,
							"type '%s' is not an object type", iref.Token)}
					} else if _, ok := obj.Property(iref.Property); !ok {
						subdiags = hcl.Diagnostics{errorf(path,
							"property '%s' not found on type '%s'", iref.Property, iref.Token)}
					}
				}
			}
		case DocRefKindFunction, DocRefKindFunctionInputProperty, DocRefKindFunctionOutputProperty:
			fun, has := types.functionDefs[string(iref.Token)]
			if !has {
				subdiags = hcl.Diagnostics{errorf(path, "reference to function '%s' not found in package %s",
					iref.Ref[1:], types.pkg.Name)}
			} else {
				ref.Function = fun
				switch iref.Kind { //nolint:exhaustive
				case DocRefKindFunctionInputProperty:
					if fun.Inputs == nil {
						subdiags = hcl.Diagnostics{errorf(path,
							"function '%s' has no inputs", iref.Token)}
					} else if _, ok := fun.Inputs.Property(iref.Property); !ok {
						subdiags = hcl.Diagnostics{errorf(path,
							"input property '%s' not found on function '%s'", iref.Property, iref.Token)}
					}
				case DocRefKindFunctionOutputProperty:
					outputs := fun.Outputs
					if outputs == nil {
						if obj, ok := fun.ReturnType.(*ObjectType); ok {
							outputs = obj
						}
					}
					if outputs == nil {
						subdiags = hcl.Diagnostics{errorf(path,
							"function '%s' has no outputs", iref.Token)}
					} else if _, ok := outputs.Property(iref.Property); !ok {
						subdiags = hcl.Diagnostics{errorf(path,
							"output property '%s' not found on function '%s'", iref.Property, iref.Token)}
					}
				default:
					contract.Failf("unexpected resource ref kind: %v", iref.Kind)
				}
			}
		}

		var name string
		if !subdiags.HasErrors() {
			name, ok = resolveRefToName(ref)
		}
		diags = append(diags, subdiags...)

		if !ok {
			// If we didn't resolve the ref via `resolveRefToName` then just return a sensible default textual value.
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
	return diags
}

func hasPropertyNamed(props []*Property, name string) bool {
	for _, p := range props {
		if p.Name == name {
			return true
		}
	}
	return false
}

// DocRefKind identifies what kind of schema entity a doc ref points to (a resource, function, type, or a
// property of one of those).
type DocRefKind string

const (
	// DocRefKindUnknown is used for doc refs that could not be parsed or did not resolve to a known entity.
	DocRefKindUnknown DocRefKind = ""
	// DocRefKindResource refers to a resource (`#/resources/{token}`).
	DocRefKindResource DocRefKind = "resource"
	// DocRefKindFunction refers to a function (`#/functions/{token}`).
	DocRefKindFunction DocRefKind = "function"
	// DocRefKindType refers to a named type — an object type or enum (`#/types/{token}`).
	DocRefKindType DocRefKind = "type"
	// DocRefKindResourceProperty refers to an output property on a resource
	// (`#/resources/{token}/properties/{property}`).
	DocRefKindResourceProperty DocRefKind = "resourceProperty"
	// DocRefKindResourceInputProperty refers to an input property on a resource
	// (`#/resources/{token}/inputProperties/{property}`).
	DocRefKindResourceInputProperty DocRefKind = "resourceInputProperty"
	// DocRefKindFunctionInputProperty refers to an input property on a function
	// (`#/functions/{token}/inputs/properties/{property}`).
	DocRefKindFunctionInputProperty DocRefKind = "functionInputProperty"
	// DocRefKindFunctionOutputProperty refers to an output property on a function
	// (`#/functions/{token}/outputs/properties/{property}`).
	DocRefKindFunctionOutputProperty DocRefKind = "functionOutputProperty"
	// DocRefKindTypeProperty refers to a property on a named object type
	// (`#/types/{token}/properties/{property}`).
	DocRefKindTypeProperty DocRefKind = "typeProperty"
)

// DocRef is a parsed and (when possible) bound reference to a schema entity that appears in a documentation
// string. It carries enough information for language-specific codegen to render the reference as a name in
// the target language.
type DocRef struct {
	// Ref is the original ref string as it appeared in the source documentation (e.g. `#/resources/foo:bar:Baz`).
	Ref string
	// Kind identifies what sort of entity the ref points to. See the DocRefKind constants.
	Kind DocRefKind
	// Type is the bound schema type, if Kind refers to a resource or named type. Nil otherwise.
	Type Type
	// Function is the bound schema function, if Kind refers to a function or one of its properties. Nil otherwise.
	Function *Function
	// Property is the referenced property name for property-kind refs, or empty if the ref is to a top-level entity.
	Property string
}

// ResourceToken returns the token of the resource this ref points to.
// Only valid for resource-kind refs (DocRefKindResource, DocRefKindResourceProperty,
// DocRefKindResourceInputProperty).
func (r DocRef) ResourceToken() string {
	rt, ok := r.Type.(*ResourceType)
	contract.Assertf(ok, "ResourceToken called on non-resource ref (kind=%v)", r.Kind)
	return rt.Token
}

// tokenString extracts the token string from the Ref field.
// Returns "" if the Ref is empty or cannot be parsed.
func (r DocRef) tokenString() string {
	parts := strings.Split(r.Ref, "/")
	if len(parts) < 3 || parts[0] != "#" {
		return ""
	}
	tok, err := url.PathUnescape(parts[2])
	if err != nil {
		return ""
	}
	return tok
}

// IsWithin returns true if r is a property ref within the entity described by other.
// This is used during doc ref interpretation to determine if a referenced property
// belongs to the entity currently being documented (selfRef).
func (r DocRef) IsWithin(other DocRef) bool {
	rTok := r.tokenString()
	oTok := other.tokenString()
	if rTok == "" || oTok == "" {
		return false
	}
	switch r.Kind {
	case DocRefKindUnknown, DocRefKindResource, DocRefKindFunction, DocRefKindType:
		return false
	case DocRefKindResourceProperty, DocRefKindResourceInputProperty:
		return other.Kind == DocRefKindResource && rTok == oTok
	case DocRefKindFunctionInputProperty, DocRefKindFunctionOutputProperty:
		return other.Kind == DocRefKindFunction && rTok == oTok
	case DocRefKindTypeProperty:
		return other.Kind == DocRefKindType && rTok == oTok
	}
	return false
}

// DocRefForType returns a DocRef for the given named schema type.
// Handles *ResourceType, *ObjectType, and *EnumType.
// Returns an empty DocRef for other types.
func DocRefForType(t Type) DocRef {
	switch v := t.(type) {
	case *ResourceType:
		return DocRef{Kind: DocRefKindResource, Type: v, Ref: "#/resources/" + url.PathEscape(v.Token)}
	case *ObjectType:
		return DocRef{Kind: DocRefKindType, Type: v, Ref: "#/types/" + url.PathEscape(v.Token)}
	case *EnumType:
		return DocRef{Kind: DocRefKindType, Type: v, Ref: "#/types/" + url.PathEscape(v.Token)}
	default:
		return DocRef{}
	}
}

// DocRefForResource returns a DocRef for the given resource.
func DocRefForResource(r *Resource) DocRef {
	return DocRef{Kind: DocRefKindResource, Ref: "#/resources/" + url.PathEscape(r.Token)}
}

// DocRefForFunction returns a DocRef for the given function.
func DocRefForFunction(f *Function) DocRef {
	return DocRef{Kind: DocRefKindFunction, Function: f, Ref: "#/functions/" + url.PathEscape(f.Token)}
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
