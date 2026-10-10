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

// MapType represents maps from strings to particular element types.
type MapType struct {
	// ElementType is the element type of the map.
	ElementType Type

	cache *typeCache
}

// NewMapType creates a new map type with the given element type.
func NewMapType(elementType Type) *MapType {
	return &MapType{ElementType: elementType, cache: &typeCache{}}
}

func (t *MapType) pretty(seenFormatters map[Type]pretty.Formatter) pretty.Formatter {
	var formatter pretty.Formatter
	if seenFormatter, ok := seenFormatters[t.ElementType]; ok {
		formatter = seenFormatter
	} else {
		formatter = t.ElementType.pretty(seenFormatters)
	}

	return &pretty.Wrap{
		Prefix:  "map(",
		Postfix: ")",
		Value:   formatter,
	}
}

func (t *MapType) Pretty() pretty.Formatter {
	seenFormatters := map[Type]pretty.Formatter{}
	return t.pretty(seenFormatters)
}

// Traverse attempts to traverse the optional type with the given traverser. The result type of traverse(map(T))
// is T; the traversal fails if the traverser is not a string.
func (t *MapType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	_, keyType := GetTraverserKey(traverser)

	var diagnostics hcl.Diagnostics
	if !InputType(StringType).ConversionFrom(keyType).Exists() {
		diagnostics = hcl.Diagnostics{unsupportedMapKey(traverser.SourceRange())}
	}
	return t.ElementType, diagnostics
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*MapType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// Equals returns true if this type has the same identity as the given type.
func (t *MapType) Equals(other Type) bool {
	return t.equals(other, nil)
}

func (t *MapType) equals(other Type, seen equalPairs) bool {
	if t == other {
		return true
	}

	otherMap, ok := other.(*MapType)
	return ok && t.ElementType.equals(otherMap.ElementType, seen)
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A map(T) is assignable
// from values of type map(U) where T is assignable from U or object(K_0=U_0, ..., K_N=U_N) if T is assignable from the
// unified type of U_0 through U_N.
func (t *MapType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		switch src := src.(type) {
		case *MapType:
			return t.ElementType.AssignableFrom(src.ElementType)
		case *ObjectType:
			for _, src := range src.Properties {
				if !t.ElementType.AssignableFrom(src) {
					return false
				}
			}
			return true
		}
		return false
	})
}

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type. A map(T)
// converts from map(U) or object({K_0 = U_0 ... K_N = U_N}) as the least safe of the conversions from the element
// types U to T (README §4, C-Map).
func (t *MapType) ConversionFrom(src Type) ConversionKind {
	return cachedConversionFrom(t, src, t.cache)
}

func (t *MapType) String() string {
	return t.string(nil)
}

func (t *MapType) string(seen map[Type]struct{}) string {
	return fmt.Sprintf("map(%s)", t.ElementType.string(seen))
}

func (*MapType) isType() {}
