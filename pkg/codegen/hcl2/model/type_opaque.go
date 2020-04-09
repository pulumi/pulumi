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
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

// OpaqueType represents a type that is named by a string.
type OpaqueType struct {
	// Name is the type's name.
	Name string

	s string
}

// The set of opaque types, indexed by name.
var opaqueTypes = map[string]*OpaqueType{}

// GetOpaqueType fetches the opaque type for the given name.
func GetOpaqueType(name string) (*OpaqueType, bool) {
	t, ok := opaqueTypes[name]
	return t, ok
}

// MustNewOpaqueType creates a new opaque type with the given name.
func MustNewOpaqueType(name string) *OpaqueType {
	t, err := NewOpaqueType(name)
	if err != nil {
		panic(err)
	}
	return t
}

// NewOpaqueType creates a new opaque type with the given name.
func NewOpaqueType(name string) (*OpaqueType, error) {
	if _, ok := opaqueTypes[name]; ok {
		return nil, errors.Errorf("opaque type %s is already defined", name)
	}

	t := &OpaqueType{Name: name}
	opaqueTypes[name] = t
	return t, nil
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*OpaqueType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// Traverse attempts to traverse the opaque type with the given traverser. The result type of traverse(opaque(name))
// is dynamic if name is "dynamic"; otherwise the traversal fails.
func (t *OpaqueType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	if t == DynamicType {
		return DynamicType, nil
	}

	return DynamicType, hcl.Diagnostics{unsupportedReceiverType(t, traverser.SourceRange())}
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A token(name) is assignable
// from token(name).
func (t *OpaqueType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		return false
	})
}

func (t *OpaqueType) conversionFromImpl(src Type, unifying, checkUnsafe bool) ConversionKind {
	return conversionFrom(t, src, unifying, func() ConversionKind {
		switch {
		case t == NumberType:
			// src == NumberType is handled by t == src above
			contract.Assert(src != NumberType)

			cki := IntType.conversionFromImpl(src, unifying, false)
			if cki == SafeConversion {
				return SafeConversion
			}
			if cki == UnsafeConversion || checkUnsafe && StringType.conversionFromImpl(src, unifying, false).Exists() {
				return UnsafeConversion
			}
			return NoConversion
		case t == IntType:
			if checkUnsafe && NumberType.conversionFromImpl(src, unifying, true).Exists() {
				return UnsafeConversion
			}
			return NoConversion
		case t == BoolType:
			if checkUnsafe && StringType.conversionFromImpl(src, unifying, false).Exists() {
				return UnsafeConversion
			}
			return NoConversion
		case t == StringType:
			ckb := BoolType.conversionFromImpl(src, unifying, false)
			ckn := NumberType.conversionFromImpl(src, unifying, false)
			if ckb == SafeConversion || ckn == SafeConversion {
				return SafeConversion
			}
			if ckb == UnsafeConversion || ckn == UnsafeConversion {
				return UnsafeConversion
			}
			return NoConversion
		default:
			return NoConversion
		}
	})
}

func (t *OpaqueType) conversionFrom(src Type, unifying bool) ConversionKind {
	return t.conversionFromImpl(src, unifying, true)
}

func (t *OpaqueType) ConversionFrom(src Type) ConversionKind {
	return t.conversionFrom(src, false)
}

func (t *OpaqueType) String() string {
	if t.s == "" {
		switch t {
		case NumberType:
			t.s = "number"
		case IntType:
			t.s = "int"
		case BoolType:
			t.s = "bool"
		case StringType:
			t.s = "string"
		default:
			if hclsyntax.ValidIdentifier(t.Name) {
				t.s = t.Name
			} else {
				t.s = fmt.Sprintf("type(%s)", t.Name)
			}
		}
	}
	return t.s
}

var opaquePrecedence = []*OpaqueType{StringType, NumberType, IntType, BoolType}

func (t *OpaqueType) unify(other Type) (Type, ConversionKind) {
	return unify(t, other, func() (Type, ConversionKind) {
		if t == DynamicType || other == DynamicType {
			// These should have been handled by unify.
			contract.Failf("unexpected type %v in OpaqueType.unify", t)
			return DynamicType, SafeConversion
		}

		for _, goal := range opaquePrecedence {
			if t == goal {
				return goal, goal.conversionFrom(other, true)
			}
			if other == goal {
				return goal, goal.conversionFrom(t, true)
			}
		}

		// There should be a total order on conversions to and from these types, so there should be a total order
		// on unifications with these types.
		return DynamicType, SafeConversion
	})
}

func (*OpaqueType) isType() {}
