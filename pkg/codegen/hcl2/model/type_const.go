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

// ConstType represents a type that is a single constant value.
type ConstType struct {
	// Type is the underlying value type.
	Type Type
	// Value is the constant value.
	Value interface{}
}

// NewConstType creates a new constant type with the given type and value.
func NewConstType(typ Type, value interface{}) *ConstType {
	return &ConstType{Type: typ, Value: value}
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*ConstType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// Traverse attempts to traverse the type with the given traverser. The result is the traversal
// result of the underlying type.
func (t *ConstType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	return t.Type.Traverse(traverser)
}

// Equals returns true if this type has the same identity as the given type.
func (t *ConstType) Equals(other Type) bool {
	return t.equals(other, nil)
}

func (t *ConstType) equals(other Type, seen map[Type]struct{}) bool {
	if t == other {
		return true
	}

	otherConst, ok := other.(*ConstType)
	return ok && t.Value == otherConst.Value && t.Type.equals(otherConst.Type, seen)
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A token(name) is assignable
// from token(name).
func (t *ConstType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		return false
	})
}

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type.
// The const type is only convertible from itself.
func (t *ConstType) ConversionFrom(src Type) ConversionKind {
	return t.conversionFrom(src, false)
}

func (t *ConstType) conversionFrom(src Type, unifying bool) ConversionKind {
	return conversionFrom(t, src, unifying, func() ConversionKind {
		return NoConversion
	})
}

func (t *ConstType) String() string {
	return fmt.Sprintf("%v", t.Value)
}

func (t *ConstType) unify(other Type) (Type, ConversionKind) {
	return unify(t, other, func() (Type, ConversionKind) {
		return t, other.ConversionFrom(t)
	})
}

func (*ConstType) isType() {}
