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

// OutputType represents eventual values that carry additional application-specific information.
type OutputType struct {
	// ElementType is the element type of the output.
	ElementType Type
}

// NewOutputType creates a new output type with the given element type after replacing any output or promise types
// within the element type with their respective element types.
func NewOutputType(elementType Type) *OutputType {
	return &OutputType{ElementType: ResolveOutputs(elementType)}
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*OutputType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// Traverse attempts to traverse the output type with the given traverser. The result type of traverse(output(T))
// is output(traverse(T)).
func (t *OutputType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	element, diagnostics := t.ElementType.Traverse(traverser)
	return NewOutputType(element.(Type)), diagnostics
}

// Equals returns true if this type has the same identity as the given type.
func (t *OutputType) Equals(other Type) bool {
	return t.equals(other, nil)
}

func (t *OutputType) equals(other Type, seen map[Type]struct{}) bool {
	if t == other {
		return true
	}
	otherOutput, ok := other.(*OutputType)
	return ok && t.ElementType.equals(otherOutput.ElementType, seen)
}

// AssignableFrom returns true if this type is assignable from the indicated source type. An output(T) is assignable
// from values of type output(U), promise(U), and U, where T is assignable from U.
func (t *OutputType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		switch src := src.(type) {
		case *OutputType:
			return t.ElementType.AssignableFrom(src.ElementType)
		case *PromiseType:
			return t.ElementType.AssignableFrom(ResolveOutputs(src.ElementType))
		}
		return t.ElementType.AssignableFrom(src)
	})
}

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type. An
// output(T) is convertible from a type U, output(U), or promise(U) if U is convertible to T. If the conversion from
// U to T is unsafe, the entire conversion is unsafe. Otherwise, the conversion is safe.
func (t *OutputType) ConversionFrom(src Type) ConversionKind {
	kind, _ := t.conversionFrom(src, false, nil)
	return kind
}

func (t *OutputType) conversionFrom(src Type, unifying bool, seen map[Type]struct{}) (ConversionKind, lazyDiagnostics) {
	return conversionFrom(t, src, unifying, seen, func() (ConversionKind, lazyDiagnostics) {
		switch src := src.(type) {
		case *OutputType:
			return t.ElementType.conversionFrom(src.ElementType, unifying, seen)
		case *PromiseType:
			return t.ElementType.conversionFrom(ResolveOutputs(src.ElementType), unifying, seen)
		}
		return t.ElementType.conversionFrom(src, unifying, seen)
	})
}

func (t *OutputType) String() string {
	return t.string(nil)
}

func (t *OutputType) string(seen map[Type]struct{}) string {
	return fmt.Sprintf("output(%s)", t.ElementType.string(seen))
}

func (t *OutputType) unify(other Type) (Type, ConversionKind) {
	return unify(t, other, func() (Type, ConversionKind) {
		switch other := other.(type) {
		case *OutputType:
			// If the other type is an output type, unify based on the element type.
			elementType, conversionKind := t.ElementType.unify(other.ElementType)
			return NewOutputType(elementType), conversionKind
		case *PromiseType:
			// If the other type is a promise type, unify based on the element type.
			elementType, conversionKind := t.ElementType.unify(ResolveOutputs(other.ElementType))
			return NewOutputType(elementType), conversionKind
		default:
			// Prefer the output type.
			kind, _ := t.conversionFrom(other, true, nil)
			return t, kind
		}
	})
}

func (t *OutputType) isType() {}
