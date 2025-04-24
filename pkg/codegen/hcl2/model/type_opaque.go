// Copyright 2016-2022, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/pretty"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// OpaqueType represents a type that is named by a string.
type OpaqueType string

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

// Equals returns true if this type has the same identity as the given type.
func (t *OpaqueType) Equals(other Type) bool {
	return t.equals(other, nil)
}

func (t *OpaqueType) equals(other Type, seen map[Type]struct{}) bool {
	if o, ok := other.(*OpaqueType); ok {
		return *o == *t
	}
	return t == other
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A token(name) is assignable
// from token(name).
func (t *OpaqueType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		return false
	})
}

func (t *OpaqueType) conversionFromImpl(
	src Type, unifying, checkUnsafe bool, seen map[Type]struct{},
) (ConversionKind, lazyDiagnostics) {
	return conversionFrom(t, src, unifying, seen, func() (ConversionKind, lazyDiagnostics) {
		if constType, ok := src.(*ConstType); ok {
			return t.conversionFrom(constType.Type, unifying, seen)
		}
		switch {
		case t == NumberType:
			// src == NumberType is handled by t == src above
			contract.Assertf(src != NumberType, "unexpected number-to-number conversion")

			cki, _ := IntType.conversionFromImpl(src, unifying, false, seen)
			switch cki {
			case SafeConversion:
				return SafeConversion, nil
			case UnsafeConversion:
				return UnsafeConversion, nil
			case NoConversion:
				if checkUnsafe {
					if kind, _ := StringType.conversionFromImpl(src, unifying, false, seen); kind.Exists() {
						return UnsafeConversion, nil
					}
				}
			}
			return NoConversion, nil
		case t == IntType:
			if checkUnsafe {
				if kind, _ := NumberType.conversionFromImpl(src, unifying, true, seen); kind.Exists() {
					return UnsafeConversion, nil
				}
			}
			return NoConversion, nil
		case t == BoolType:
			if checkUnsafe {
				if kind, _ := StringType.conversionFromImpl(src, unifying, false, seen); kind.Exists() {
					return UnsafeConversion, nil
				}
			}
			return NoConversion, nil
		case t == StringType:
			ckb, _ := BoolType.conversionFromImpl(src, unifying, false, seen)
			ckn, _ := NumberType.conversionFromImpl(src, unifying, false, seen)
			if ckb == SafeConversion || ckn == SafeConversion {
				return SafeConversion, nil
			}
			if ckb == UnsafeConversion || ckn == UnsafeConversion {
				return UnsafeConversion, nil
			}
			return NoConversion, nil
		default:
			return NoConversion, nil
		}
	})
}

func (t *OpaqueType) conversionFrom(src Type, unifying bool, seen map[Type]struct{}) (ConversionKind, lazyDiagnostics) {
	return t.conversionFromImpl(src, unifying, true, seen)
}

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type.
//
// In general, an opaque type is only convertible from itself (in addition to the standard dynamic and union
// conversions). However, there are special rules for the builtin types:
//
// - The dynamic type is safely convertible from any other type, and is unsafely convertible _to_ any other type
// - The string type is safely convertible from bool, number, and int
// - The number type is safely convertible from int and unsafely convertible from string
// - The int type is unsafely convertible from string
// - The bool type is unsafely convertible from string
func (t *OpaqueType) ConversionFrom(src Type) ConversionKind {
	kind, _ := t.conversionFrom(src, false, nil)
	return kind
}

func (t *OpaqueType) String() string {
	switch t {
	case NumberType:
		return "number"
	case IntType:
		return "int"
	case BoolType:
		return "bool"
	case StringType:
		return "string"
	default:
		if hclsyntax.ValidIdentifier(string(*t)) {
			return string(*t)
		}

		return fmt.Sprintf("type(%s)", string(*t))
	}
}

func NewOpaqueType(name string) *OpaqueType {
	t := OpaqueType(name)
	return &t
}

func (t *OpaqueType) pretty(seenFormatters map[Type]pretty.Formatter) pretty.Formatter {
	return pretty.FromStringer(t)
}

func (t *OpaqueType) Pretty() pretty.Formatter {
	seenFormatters := map[Type]pretty.Formatter{}
	return t.pretty(seenFormatters)
}

func (t *OpaqueType) string(_ map[Type]struct{}) string {
	return t.String()
}

var opaquePrecedence = []Type{StringType, NumberType, IntType, BoolType}

func (t *OpaqueType) unify(other Type) (Type, ConversionKind) {
	return unify(t, other, func() (Type, ConversionKind) {
		if t == DynamicType || other == DynamicType {
			// These should have been handled by unify.
			contract.Failf("unexpected type %v in OpaqueType.unify", t)
			return DynamicType, SafeConversion
		}

		for _, goal := range opaquePrecedence {
			if t == goal {
				kind, _ := goal.conversionFrom(other, true, nil)
				return goal, kind
			}
			if other == goal {
				kind, _ := goal.conversionFrom(t, true, nil)
				return goal, kind
			}
		}

		// There should be a total order on conversions to and from these types, so there should be a total order
		// on unifications with these types.
		return DynamicType, SafeConversion
	})
}

func (*OpaqueType) isType() {}
