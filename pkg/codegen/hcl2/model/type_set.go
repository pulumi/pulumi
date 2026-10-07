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

// SetType represents sets of particular element types.
type SetType struct {
	// ElementType is the element type of the set.
	ElementType Type

	cache *typeCache
}

// NewSetType creates a new set type with the given element type.
func NewSetType(elementType Type) *SetType {
	return &SetType{ElementType: elementType, cache: &typeCache{}}
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

func (t *SetType) equals(other Type, seen equalPairs) bool {
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
// A set(T) converts from a set(U) as T converts from U, and at most unsafely from a list(U) or a
// tuple(U_0 ... U_N) whose element types convert to T (README §4, C-Set).
func (t *SetType) ConversionFrom(src Type) ConversionKind {
	return cachedConversionFrom(t, src, t.cache)
}

func (t *SetType) pretty(seenFormatters map[Type]pretty.Formatter) pretty.Formatter {
	var formatter pretty.Formatter
	if seenFormatter, ok := seenFormatters[t.ElementType]; ok {
		formatter = seenFormatter
	} else {
		formatter = t.ElementType.pretty(seenFormatters)
	}

	return &pretty.Wrap{
		Prefix:  "set(",
		Postfix: ")",
		Value:   formatter,
	}
}

func (t *SetType) Pretty() pretty.Formatter {
	seenFormatters := map[Type]pretty.Formatter{}
	return t.pretty(seenFormatters)
}

func (t *SetType) String() string {
	return t.string(nil)
}

func (t *SetType) string(seen map[Type]struct{}) string {
	return fmt.Sprintf("set(%s)", t.ElementType.string(seen))
}

func (*SetType) isType() {}
