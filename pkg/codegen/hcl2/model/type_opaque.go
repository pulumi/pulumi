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

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/pretty"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
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

func (t *OpaqueType) equals(other Type, seen equalPairs) bool {
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

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type.
//
// In general, an opaque type is only convertible from itself (in addition to the standard dynamic and union
// conversions). However, there are special rules for the builtin types (README §4.4):
//
// - The dynamic type is safely convertible from any other type, and is unsafely convertible _to_ any other type
// - The string type and the id type are safely convertible from every other builtin type
// - The number type is safely convertible from int
// - Every other pair of builtin types is unsafely convertible
func (t *OpaqueType) ConversionFrom(src Type) ConversionKind {
	return cachedConversionFrom(t, src, nil)
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
	case IDType:
		return "id"
	default:
		if hclsyntax.ValidIdentifier(string(*t)) {
			return string(*t)
		}

		return fmt.Sprintf("type(%s)", string(*t))
	}
}

func NewOpaqueType(name string) *OpaqueType {
	return new(OpaqueType(name))
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

func (*OpaqueType) isType() {}
