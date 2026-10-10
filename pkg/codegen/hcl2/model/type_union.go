// Copyright 2016, Pulumi Corporation.
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

package model

import (
	"fmt"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/pretty"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

// UnionType represents values that may be any one of a specified set of types.
type UnionType struct {
	// ElementTypes are the allowable types for the union type.
	ElementTypes []Type
	// Annotations records any annotations associated with the object type.
	Annotations []any

	s atomic.Value // Value<string>

	cache *typeCache
}

// NewUnionTypeAnnotated creates a new union type with the given element types and annotations.
// NewUnionTypeAnnotated enforces 3 properties on the returned type:
// 1. Any element types that are union types are replaced with their element types.
// 2. Any duplicate types are removed.
// 3. Unions have have more then 1 type. If only a single type is left after (1) and (2),
// it is returned as is.
func NewUnionTypeAnnotated(types []Type, annotations ...any) Type {
	elementTypes := canonicalMembers(types)

	// If the union turns out to be the union of a single type, just return the underlying
	// type.
	if len(elementTypes) == 1 {
		return elementTypes[0]
	}

	return &UnionType{ElementTypes: elementTypes, Annotations: annotations, cache: &typeCache{}}
}

// canonicalMembers is the member list of README §3: the members of each union and every other type, sorted with
// Compare and without duplicates.
func canonicalMembers(types []Type) []Type {
	var elementTypes []Type
	for _, t := range types {
		if union, isUnion := t.(*UnionType); isUnion {
			elementTypes = append(elementTypes, union.ElementTypes...)
		} else {
			elementTypes = append(elementTypes, t)
		}
	}
	slices.SortFunc(elementTypes, Compare)
	return slices.CompactFunc(elementTypes, Type.Equals)
}

// NewUnionType creates a new union type with the given element types. Any element types that are union types are
// replaced with their element types.
func NewUnionType(types ...Type) Type {
	var annotations []any
	for _, t := range types {
		if union, isUnion := t.(*UnionType); isUnion {
			annotations = append(annotations, union.Annotations...)
		}
	}
	return NewUnionTypeAnnotated(types, annotations...)
}

// NewOptionalType returns a new union(T, None).
func NewOptionalType(t Type) Type {
	return NewUnionType(t, NoneType)
}

// IsOptionalType returns true if t is an optional type.
func IsOptionalType(t Type) bool {
	return t != DynamicType && t.AssignableFrom(NoneType)
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*UnionType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *UnionType) pretty(seenFormatters map[Type]pretty.Formatter) pretty.Formatter {
	elements := slice.Prealloc[pretty.Formatter](len(t.ElementTypes))
	isOptional := false
	unionFormatter := &pretty.List{
		Separator: " | ",
		Elements:  elements,
	}

	seenFormatters[t] = unionFormatter

	for _, el := range t.ElementTypes {
		if el == NoneType {
			isOptional = true
			continue
		}
		if seenFormatter, ok := seenFormatters[el]; ok {
			unionFormatter.Elements = append(unionFormatter.Elements, seenFormatter)
		} else {
			formatter := el.pretty(seenFormatters)
			seenFormatters[el] = formatter
			unionFormatter.Elements = append(unionFormatter.Elements, formatter)
		}
	}

	if isOptional {
		return &pretty.Wrap{
			Value:           seenFormatters[t],
			Postfix:         "?",
			PostfixSameline: true,
		}
	}

	return seenFormatters[t]
}

func (t *UnionType) Pretty() pretty.Formatter {
	seenFormatters := map[Type]pretty.Formatter{}
	return t.pretty(seenFormatters)
}

// Traverse attempts to traverse the union type with the given traverser. This always fails.
func (t *UnionType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	var types []Type
	var foundDiags hcl.Diagnostics
	for _, t := range t.ElementTypes {
		// We handle 'none' specially here: so that traversing an optional type returns an optional type.
		switch t {
		case NoneType:
			types = append(types, NoneType)
		default:
			// Note that we only report errors when the entire operation fails. We try to
			// strike a balance between assuming that the traversal will dynamically
			// succeed and good error reporting.
			et, diags := t.Traverse(traverser)
			if !diags.HasErrors() {
				types = append(types, et.(Type))
			}
			if len(diags) > 0 {
				foundDiags = append(foundDiags, diags...)
			}
		}
	}

	switch len(types) {
	case 0:
		return DynamicType, foundDiags.Append(unsupportedReceiverType(t, traverser.SourceRange()))
	case 1:
		if types[0] == NoneType {
			return DynamicType, foundDiags.Append(unsupportedReceiverType(t, traverser.SourceRange()))
		}
		return types[0], nil
	default:
		return NewUnionType(types...), nil
	}
}

// Equals returns true if this type has the same identity as the given type.
func (t *UnionType) Equals(other Type) bool {
	return t.equals(other, nil)
}

func (t *UnionType) equals(other Type, seen equalPairs) bool {
	if t == other {
		return true
	}
	otherUnion, ok := other.(*UnionType)
	if !ok {
		return false
	}
	if len(t.ElementTypes) != len(otherUnion.ElementTypes) {
		return false
	}
	elementTypes, otherElementTypes := t.sortedElementTypes(nil), otherUnion.sortedElementTypes(nil)
	for i, t := range elementTypes {
		if !t.equals(otherElementTypes[i], seen) {
			return false
		}
	}
	return true
}

// sortedElementTypes returns the members in the order Compare defines. They are sorted when the union is built,
// but a member that was a recursive object type still under construction may have sorted differently, so a union
// is compared and tested for equality as a set. The comparison in flight, if any, passes its pairs so that a
// member that refers back to an object being compared does not start the comparison over.
func (t *UnionType) sortedElementTypes(seen comparePairs) []Type {
	compare := func(a, b Type) int { return compareTypes(a, b, seen) }
	if slices.IsSortedFunc(t.ElementTypes, compare) {
		return t.ElementTypes
	}
	return slices.SortedFunc(slices.Values(t.ElementTypes), compare)
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A union(T_0, ..., T_N)
// from values of type union(U_0, ..., U_M) where all of U_0 through U_M are assignable to some type in
// (T_0, ..., T_N) and V where V is assignable to at least one of (T_0, ..., T_N).
func (t *UnionType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		for _, t := range t.ElementTypes {
			if t.AssignableFrom(src) {
				return true
			}
		}
		return false
	})
}

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type. A union
// type converts from a source type as the best of the conversions to its elements (README §4, C-UDst). A union source
// converts to any type safely when every element does, unsafely when some element converts, and otherwise not at all
// (C-USrc).
func (t *UnionType) ConversionFrom(src Type) ConversionKind {
	return cachedConversionFrom(t, src, t.cache)
}

func (t *UnionType) String() string {
	return t.string(nil)
}

func (t *UnionType) string(seen map[Type]struct{}) string {
	if s := t.s.Load(); s != nil {
		return s.(string)
	}

	elements := make([]string, len(t.ElementTypes))
	for i, e := range t.ElementTypes {
		elements[i] = e.string(seen)
	}

	annotations := ""
	if len(t.Annotations) != 0 {
		annotations = fmt.Sprintf(", annotated(%p)", t)
	}

	s := fmt.Sprintf("union(%s%v)", strings.Join(elements, ", "), annotations)
	t.s.Store(s)
	return s
}

func (*UnionType) isType() {}
