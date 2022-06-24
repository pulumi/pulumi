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

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
)

// SetType represents sets of particular element types.
type SetType struct {
	// ElementType is the element type of the set.
	ElementType Type
}

// NewSetType creates a new set type with the given element type.
func NewSetType(elementType Type) *SetType {
	return &SetType{ElementType: elementType}
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*SetType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// Traverse attempts to traverse the optional type with the given traverser. This always fails.
func (t *SetType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	return DynamicType, hcl.Diagnostics{unsupportedReceiverType(t, traverser.SourceRange())}
}

// Equals returns true if this type has the same identity as the given type.
func (t *SetType) Equals(other Type) bool {
	return t.equals(other, nil)

}
func (t *SetType) equals(other Type, seen map[Type]struct{}) bool {
	if t == other {
		return true
	}
	otherSet, ok := other.(*SetType)
	return ok && t.ElementType.equals(otherSet.ElementType, seen)
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A set(T) is assignable
// from values of type set(U) where T is assignable from U.
func (t *SetType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		if src, ok := src.(*SetType); ok {
			return t.ElementType.AssignableFrom(src.ElementType)
		}
		return false
	})
}

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type.
// A set(T) is convertible from a set(U) if a conversion exists from U to T. If the conversion from U to T is unsafe,
// the entire conversion is unsafe; otherwise the conversion is safe. An unsafe conversion exists from list(U) or
// or tuple(U_0 ... U_N) to set(T) if a conversion exists from each U to T.
func (t *SetType) ConversionFrom(src Type) ConversionKind {
	kind, _ := t.conversionFrom(src, false, nil)
	return kind
}

func (t *SetType) conversionFrom(src Type, unifying bool, seen map[Type]struct{}) (ConversionKind, lazyDiagnostics) {
	return conversionFrom(t, src, unifying, seen, func() (ConversionKind, lazyDiagnostics) {
		switch src := src.(type) {
		case *SetType:
			return t.ElementType.conversionFrom(src.ElementType, unifying, seen)
		case *ListType:
			if conversionKind, why := t.ElementType.conversionFrom(src.ElementType, unifying, seen); conversionKind ==
				NoConversion {
				return NoConversion, why
			}
			return UnsafeConversion, nil
		case *TupleType:
			if conversionKind, why := NewListType(t.ElementType).conversionFrom(src, unifying, seen); conversionKind ==
				NoConversion {
				return NoConversion, why
			}
			return UnsafeConversion, nil
		}
		return NoConversion, func() hcl.Diagnostics { return hcl.Diagnostics{typeNotConvertible(t, src)} }
	})
}

func (t *SetType) String() string {
	return t.string(nil)
}

func (t *SetType) string(seen map[Type]struct{}) string {
	return fmt.Sprintf("set(%s)", t.ElementType.string(seen))
}

func (t *SetType) unify(other Type) (Type, ConversionKind) {
	return unify(t, other, func() (Type, ConversionKind) {
		switch other := other.(type) {
		case *SetType:
			// If the other type is a set type, unify based on the element type.
			elementType, conversionKind := t.ElementType.unify(other.ElementType)
			return NewSetType(elementType), conversionKind
		case *ListType:
			// Prefer the list type, but unify the element types.
			element, conversionKind := t.ElementType.unify(other.ElementType)
			return NewListType(element), conversionKind
		case *TupleType:
			// Prefer the set type, but unify the element type.
			elementType, conversionKind := t.ElementType, UnsafeConversion
			for _, other := range other.ElementTypes {
				element, ck := elementType.unify(other)
				if ck < conversionKind {
					conversionKind = ck
				}
				elementType = element
			}
			return NewSetType(elementType), conversionKind
		default:
			// Prefer the set type.
			kind, _ := t.conversionFrom(other, true, nil)
			return t, kind
		}
	})
}

func (*SetType) isType() {}
