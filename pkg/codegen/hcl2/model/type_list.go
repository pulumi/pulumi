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
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/syntax"
)

// ListType represents lists of particular element types.
type ListType struct {
	// ElementType is the element type of the list.
	ElementType Type
}

// NewListType creates a new list type with the given element type.
func NewListType(elementType Type) *ListType {
	return &ListType{ElementType: elementType}
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*ListType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// Traverse attempts to traverse the optional type with the given traverser. The result type of traverse(list(T))
// is T; the traversal fails if the traverser is not a number.
func (t *ListType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	_, indexType := GetTraverserKey(traverser)

	var diagnostics hcl.Diagnostics
	if !InputType(NumberType).ConversionFrom(indexType).Exists() {
		diagnostics = hcl.Diagnostics{unsupportedListIndex(traverser.SourceRange())}
	}
	return t.ElementType, diagnostics
}

// Equals returns true if this type has the same identity as the given type.
func (t *ListType) Equals(other Type) bool {
	if t == other {
		return true
	}

	otherList, ok := other.(*ListType)
	return ok && t.ElementType.Equals(otherList.ElementType)
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A list(T) is assignable
// from values of type list(U) where T is assignable from U.
func (t *ListType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		switch src := src.(type) {
		case *ListType:
			return t.ElementType.AssignableFrom(src.ElementType)
		case *TupleType:
			for _, src := range src.ElementTypes {
				if !t.ElementType.AssignableFrom(src) {
					return false
				}
			}
			return true
		}
		return false
	})
}

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type. A list(T)
// is safely convertible from list(U), set(U), or tuple(U_0 ... U_N) if the element type(s) U is/are safely convertible
// to T. If any element type is unsafely convertible to T and no element type is safely convertible to T, the
// conversion is unsafe. Otherwise, no conversion exists.
func (t *ListType) ConversionFrom(src Type) ConversionKind {
	return t.conversionFrom(src, false)
}

func (t *ListType) conversionFrom(src Type, unifying bool) ConversionKind {
	return conversionFrom(t, src, unifying, func() ConversionKind {
		switch src := src.(type) {
		case *ListType:
			return t.ElementType.conversionFrom(src.ElementType, unifying)
		case *SetType:
			return t.ElementType.conversionFrom(src.ElementType, unifying)
		case *TupleType:
			conversionKind := SafeConversion
			for _, src := range src.ElementTypes {
				if ck := t.ElementType.conversionFrom(src, unifying); ck < conversionKind {
					conversionKind = ck
				}
			}
			return conversionKind
		}
		return NoConversion
	})
}

func (t *ListType) String() string {
	return fmt.Sprintf("list(%v)", t.ElementType)
}

func (t *ListType) unify(other Type) (Type, ConversionKind) {
	return unify(t, other, func() (Type, ConversionKind) {
		switch other := other.(type) {
		case *TupleType:
			// If the other element is a list type, prefer the list type, but unify the element type.
			elementType, conversionKind := t.ElementType, SafeConversion
			for _, other := range other.ElementTypes {
				element, ck := elementType.unify(other)
				if ck < conversionKind {
					conversionKind = ck
				}
				elementType = element
			}
			return NewListType(elementType), conversionKind
		case *SetType:
			// If the other element is a set type, prefer the list type, but unify the element types.
			elementType, conversionKind := t.ElementType.unify(other.ElementType)
			return NewListType(elementType), conversionKind
		case *ListType:
			// If the other type is a list type, unify based on the element type.
			elementType, conversionKind := t.ElementType.unify(other.ElementType)
			return NewListType(elementType), conversionKind
		default:
			// Prefer the list type.
			return t, t.conversionFrom(other, true)
		}
	})
}

func (*ListType) isType() {}
