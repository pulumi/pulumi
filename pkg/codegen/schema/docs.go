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
	"context"
	"net/url"
	"strings"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/pgavlin/goldmark/ast"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// PulumiRefResolver resolves a parsed doc reference to the textual name that should be substituted into the
// surrounding documentation. It returns the substituted name and a boolean indicating whether the ref was
// resolved; if false, the caller falls back to a default rendering of the ref.
type PulumiRefResolver func(ref DocRef) (string, bool)

// interpretPulumiRefsInDescription interprets Pulumi refs in a documentation string,
// then renders the result back to text.
func interpretPulumiRefsInDescription(
	description string, types *types, resolver PulumiRefResolver,
) (string, error) {
	if description == "" {
		return "", nil
	}

	source := []byte(description)
	parsed := ParseDocs(source)
	err := interpretPulumiRefs("", types, parsed, resolver)
	if err != nil {
		return "", err
	}

	return RenderDocsToString(source, parsed), nil
}

// interpretPulumiRefs parses all {{% ref %}} shortcodes that descend from the given node. Each ref is passed to the
// `resolveRefToName` callback to replace the shortcode with literal text.
func interpretPulumiRefs(
	path string, types *types,
	node ast.Node, resolveRefToName PulumiRefResolver,
) hcl.Diagnostics {
	var diags hcl.Diagnostics

	var c, next ast.Node
	for c = node.FirstChild(); c != nil; c = next {
		subdiags := interpretPulumiRefs(path, types, c, resolveRefToName)
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
			res, ok, err := types.lookupResourceForDocRef(iref)
			switch {
			case err != nil:
				subdiags = hcl.Diagnostics{errorf(path,
					"resolving reference to resource '%s': %v", iref.Ref[1:], err)}
			case !ok:
				subdiags = hcl.Diagnostics{errorf(path, "reference to resource '%s' not found in package %s",
					iref.Ref[1:], types.pkg.Name)}
			default:
				ref.Type = res
				switch iref.Kind { //nolint:exhaustive
				case DocRefKindResource:
					// Top-level resource ref; no property to validate.
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
			typ, ok, err := types.lookupTypeForDocRef(iref)
			switch {
			case err != nil:
				subdiags = hcl.Diagnostics{errorf(path,
					"resolving reference to type '%s': %v", iref.Ref[1:], err)}
			case !ok:
				subdiags = hcl.Diagnostics{errorf(path, "reference to type '%s' not found in package %s",
					iref.Ref[1:], types.pkg.Name)}
			default:
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
			fun, has, err := types.lookupFunctionForDocRef(iref)
			switch {
			case err != nil:
				subdiags = hcl.Diagnostics{errorf(path,
					"resolving reference to function '%s': %v", iref.Ref[1:], err)}
			case !has:
				subdiags = hcl.Diagnostics{errorf(path, "reference to function '%s' not found in package %s",
					iref.Ref[1:], types.pkg.Name)}
			default:
				ref.Function = fun
				switch iref.Kind { //nolint:exhaustive
				case DocRefKindFunction:
					// Top-level function ref; no property to validate.
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

// externalPackageForDocRef loads the external package the ref points to, or returns (nil, false, nil)
// if the ref is into the current package. The descriptor is taken from the current package's
// Dependencies list when one matches the ref's package name and version; otherwise a bare descriptor
// constructed from the ref's package name and version is used.
func (t *types) externalPackageForDocRef(iref internalDocRef) (PackageReference, bool, error) {
	if iref.Package == "" {
		return nil, false, nil
	}
	var descriptor *PackageDescriptor
	for i := range t.pkg.Dependencies {
		d := &t.pkg.Dependencies[i]
		name := d.Name
		version := d.Version
		if d.Parameterization != nil {
			name = d.Parameterization.Name
			version = &d.Parameterization.Version
		}
		if name == iref.Package && versionEquals(version, iref.Version) {
			descriptor = d
			break
		}
	}
	if descriptor == nil {
		descriptor = &PackageDescriptor{Name: iref.Package, Version: iref.Version}
	}
	pkg, err := LoadPackageReferenceV2(context.TODO(), t.loader, descriptor)
	if err != nil {
		return nil, true, err
	}
	return pkg, true, nil
}

// lookupResourceForDocRef resolves a resource doc ref to its *ResourceType, in either the current
// package or an external package reachable via Dependencies.
func (t *types) lookupResourceForDocRef(iref internalDocRef) (*ResourceType, bool, error) {
	pkg, external, err := t.externalPackageForDocRef(iref)
	if err != nil {
		return nil, false, err
	}
	if !external {
		rt, ok := t.resources[string(iref.Token)]
		return rt, ok, nil
	}
	return pkg.Resources().GetType(string(iref.Token))
}

// lookupTypeForDocRef resolves a type doc ref to its Type definition, in either the current package
// or an external package reachable via Dependencies.
func (t *types) lookupTypeForDocRef(iref internalDocRef) (Type, bool, error) {
	pkg, external, err := t.externalPackageForDocRef(iref)
	if err != nil {
		return nil, false, err
	}
	if !external {
		typ, ok := t.typeDefs[string(iref.Token)]
		return typ, ok, nil
	}
	return pkg.Types().Get(string(iref.Token))
}

// lookupFunctionForDocRef resolves a function doc ref to its *Function, in either the current package
// or an external package reachable via Dependencies.
func (t *types) lookupFunctionForDocRef(iref internalDocRef) (*Function, bool, error) {
	pkg, external, err := t.externalPackageForDocRef(iref)
	if err != nil {
		return nil, false, err
	}
	if !external {
		fun, ok := t.functionDefs[string(iref.Token)]
		return fun, ok, nil
	}
	return pkg.Functions().Get(string(iref.Token))
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
	// Package is the name of the package the ref points to. Empty for refs into the current package.
	Package string
	// Version is the version of the external package the ref points to, or nil if not specified.
	Version *semver.Version
	// The token of the resource, function, or type, or empty if not applicable.
	Token tokens.Type
	// The referenced property name, or empty if not applicable.
	Property string
}

// Parses a doc reference string into an internalDocRef. The supported formats mirror schema `$ref`: a
// fragment-only ref points to the current package, and a ref with a `/{pkg}/{version}/schema.json`
// path prefix points to a package in the Dependencies list.
//
//	#/resources/{token}
//	#/functions/{token}
//	#/types/{token}
//	#/resources/{token}/properties/{property}
//	#/resources/{token}/inputProperties/{property}
//	#/functions/{token}/inputs/properties/{property}
//	#/functions/{token}/outputs/properties/{property}
//	#/types/{token}/properties/{property}
//	/{pkg}/{version}/schema.json#/resources/{token}
//	...etc, with any of the fragments above.
//
// Note: Tokens containing a slash ("/") must be encoded as "%2F".
func parseDocRef(ref string) internalDocRef {
	docRefUnknown := internalDocRef{Ref: ref, Kind: DocRefKindUnknown}

	parsedURL, err := url.Parse(ref)
	if err != nil {
		return docRefUnknown
	}

	// A path indicates an external schema (e.g. `/aws/v6.0.0/schema.json`); a bare fragment refers
	// to the current package.
	var pkgName string
	var pkgVersion *semver.Version
	if parsedURL.Path != "" {
		pathStr, err := url.PathUnescape(parsedURL.Path)
		if err != nil {
			return docRefUnknown
		}
		m := refPathRegex.FindStringSubmatch(pathStr)
		if len(m) != 3 {
			return docRefUnknown
		}
		pkgName = m[1]
		v, err := semver.ParseTolerant(m[2])
		if err != nil {
			return docRefUnknown
		}
		pkgVersion = &v
	}

	parts := strings.Split(parsedURL.EscapedFragment(), "/")
	// EscapedFragment returns "/resources/token" for `#/resources/token`, so parts[0] is the empty
	// string between the leading slash and the rest.
	if len(parts) < 3 || parts[0] != "" {
		return docRefUnknown
	}
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

	base := internalDocRef{Ref: ref, Package: pkgName, Version: pkgVersion, Token: token}

	if len(parts) == 3 {
		base.Kind = DocRefKind(topLevelType)
		return base
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
				base.Kind = DocRefKind(topLevelType + "Property")
				base.Property = property
				return base
			}
		}
		return docRefUnknown
	case "inputProperties":
		if topLevelType == "resource" && len(parts) == 5 {
			base.Kind = DocRefKind(topLevelType + "InputProperty")
			base.Property = property
			return base
		}
		return docRefUnknown
	case "inputs":
		if topLevelType == "function" && parts[4] == "properties" && len(parts) == 6 {
			property, err := url.PathUnescape(parts[5])
			if err != nil || property == "" {
				return docRefUnknown
			}
			base.Kind = DocRefKindFunctionInputProperty
			base.Property = property
			return base
		}
		return docRefUnknown
	case "outputs":
		if topLevelType == "function" && parts[4] == "properties" && len(parts) == 6 {
			property, err := url.PathUnescape(parts[5])
			if err != nil || property == "" {
				return docRefUnknown
			}
			base.Kind = DocRefKindFunctionOutputProperty
			base.Property = property
			return base
		}
		return docRefUnknown
	default:
		return docRefUnknown
	}
}

func ParseDocRef(ref string) (token tokens.Type, property string, ok bool) {
	iref := parseDocRef(ref)
	if iref.Kind == DocRefKindUnknown {
		return "", "", false
	}
	return iref.Token, iref.Property, true
}
