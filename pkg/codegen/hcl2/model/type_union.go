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

package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
)

// UnionType represents values that may be any one of a specified set of types.
type UnionType struct {
	// ElementTypes are the allowable types for the union type.
	ElementTypes []Type
	// Annotations records any annotations associated with the object type.
	Annotations []interface{}

	s string
}

// NewUnionTypeAnnotated creates a new union type with the given element types and annotations.
// Any element types that are union types are replaced with their element types.
func NewUnionTypeAnnotated(types []Type, annotations ...interface{}) Type {
	var elementTypes []Type
	for _, t := range types {
		if union, isUnion := t.(*UnionType); isUnion {
			elementTypes = append(elementTypes, union.ElementTypes...)
		} else {
			elementTypes = append(elementTypes, t)
		}
	}

	sort.Slice(elementTypes, func(i, j int) bool {
		return elementTypes[i].String() < elementTypes[j].String()
	})

	dst := 0
	for src := 0; src < len(elementTypes); {
		for src < len(elementTypes) && elementTypes[src].Equals(elementTypes[dst]) {
			src++
		}
		dst++

		if src < len(elementTypes) {
			elementTypes[dst] = elementTypes[src]
		}
	}
	elementTypes = elementTypes[:dst]

	if len(elementTypes) == 1 {
		return elementTypes[0]
	}

	return &UnionType{ElementTypes: elementTypes, Annotations: annotations}
}

// NewUnionType creates a new union type with the given element types. Any element types that are union types are
// replaced with their element types.
func NewUnionType(types ...Type) Type {
	var annotations []interface{}
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

// Traverse attempts to traverse the union type with the given traverser. This always fails.
func (t *UnionType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	var types []Type
	for _, t := range t.ElementTypes {
		// We handle 'none' specially here: so that traversing an optional type returns an optional type.
		if t == NoneType {
			types = append(types, NoneType)
		} else {
			// Note that we intentionally drop errors here and assume that the traversal will dynamically succeed.
			et, diags := t.Traverse(traverser)
			if !diags.HasErrors() {
				types = append(types, et.(Type))
			}
		}
	}

	switch len(types) {
	case 0:
		return DynamicType, hcl.Diagnostics{unsupportedReceiverType(t, traverser.SourceRange())}
	case 1:
		if types[0] == NoneType {
			return DynamicType, hcl.Diagnostics{unsupportedReceiverType(t, traverser.SourceRange())}
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

func (t *UnionType) equals(other Type, seen map[Type]struct{}) bool {
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
	for i, t := range t.ElementTypes {
		if !t.equals(otherUnion.ElementTypes[i], seen) {
			return false
		}
	}
	return true
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
// type is convertible from a source type if any of its elements are convertible from the source type. If any element
// type is safely convertible, the conversion is safe; if no element is safely convertible but some element is unsafely
// convertible, the conversion is unsafe.
func (t *UnionType) ConversionFrom(src Type) ConversionKind {
	kind, _ := t.conversionFrom(src, false, nil)
	return kind
}

func (t *UnionType) conversionFrom(src Type, unifying bool, seen map[Type]struct{}) (ConversionKind, lazyDiagnostics) {
	return conversionFrom(t, src, unifying, seen, func() (ConversionKind, lazyDiagnostics) {
		var conversionKind ConversionKind
		var diags []lazyDiagnostics

		// Fast path: see if the source type is equal to any of the element types. Equality checks are generally
		// less expensive that full convertibility checks.
		for _, t := range t.ElementTypes {
			if src.Equals(t) {
				return SafeConversion, nil
			}
		}

		for _, t := range t.ElementTypes {
			ck, why := t.conversionFrom(src, unifying, seen)
			if ck > conversionKind {
				conversionKind = ck
			} else if why != nil {
				diags = append(diags, why)
			}
		}
		if conversionKind == NoConversion {
			return NoConversion, func() hcl.Diagnostics {
				var all hcl.Diagnostics
				for _, why := range diags {
					//nolint:errcheck
					all.Extend(why())
				}
				return all
			}
		}
		return conversionKind, nil
	})
}

// If all conversions to a dest type from a union type are safe, the conversion is safe.
// If no conversions to a dest type from a union type exist, the conversion does not exist.
// Otherwise, the conversion is unsafe.
func (t *UnionType) conversionTo(dest Type, unifying bool, seen map[Type]struct{}) (ConversionKind, lazyDiagnostics) {
	conversionKind, exists := SafeConversion, false
	for _, t := range t.ElementTypes {
		switch kind, _ := dest.conversionFrom(t, unifying, seen); kind {
		case SafeConversion:
			exists = true
		case UnsafeConversion:
			conversionKind, exists = UnsafeConversion, true
		case NoConversion:
			conversionKind = UnsafeConversion
		}
	}
	if !exists {
		return NoConversion, nil
	}
	return conversionKind, nil
}

func (t *UnionType) String() string {
	return t.string(nil)
}

func (t *UnionType) string(seen map[Type]struct{}) string {
	if t.s == "" {
		elements := make([]string, len(t.ElementTypes))
		for i, e := range t.ElementTypes {
			elements[i] = e.string(seen)
		}

		annotations := ""
		if len(t.Annotations) != 0 {
			annotations = fmt.Sprintf(", annotated(%p)", t)
		}

		t.s = fmt.Sprintf("union(%s%v)", strings.Join(elements, ", "), annotations)
	}
	return t.s
}

func (t *UnionType) unify(other Type) (Type, ConversionKind) {
	return unify(t, other, func() (Type, ConversionKind) {
		return t.unifyTo(other)
	})
}

func (t *UnionType) unifyTo(other Type) (Type, ConversionKind) {
	switch other := other.(type) {
	case *UnionType:
		// If the other type is also a union type, produce a new type that is the union of their elements.
		elements := make([]Type, 0, len(t.ElementTypes)+len(other.ElementTypes))
		elements = append(elements, t.ElementTypes...)
		elements = append(elements, other.ElementTypes...)
		return NewUnionType(elements...), SafeConversion
	default:
		// Otherwise, unify the other type with each element of the union and return a new union type.
		elements, conversionKind := make([]Type, len(t.ElementTypes)), SafeConversion
		for i, t := range t.ElementTypes {
			element, ck := t.unify(other)
			if ck < conversionKind {
				conversionKind = ck
			}
			elements[i] = element
		}
		return NewUnionType(elements...), conversionKind
	}
}

func (*UnionType) isType() {}
