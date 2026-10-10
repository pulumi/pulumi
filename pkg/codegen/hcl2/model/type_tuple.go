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
	"math/big"
	"strings"
	"sync/atomic"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/pretty"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
)

// TupleType represents values that are a sequence of independently-typed elements.
type TupleType struct {
	// ElementTypes are the types of the tuple's elements.
	ElementTypes []Type

	elementUnion Type
	s            atomic.Value // Value<string>

	cache *typeCache
}

// NewTupleType creates a new tuple type with the given element types.
func NewTupleType(elementTypes ...Type) Type {
	return &TupleType{ElementTypes: elementTypes, cache: &typeCache{}}
}

func (t *TupleType) pretty(seenFormatters map[Type]pretty.Formatter) pretty.Formatter {
	elements := make([]pretty.Formatter, len(t.ElementTypes))
	for i, el := range t.ElementTypes {
		formatter, ok := seenFormatters[el]
		if ok {
			elements[i] = formatter
		} else {
			seenFormatters[el] = el.pretty(seenFormatters)
			elements[i] = seenFormatters[el]
		}
	}
	return &pretty.Wrap{
		Prefix: "(",
		Value: &pretty.List{
			AdjoinSeparator: true,
			Separator:       ", ",
			Elements:        elements,
		},
		Postfix: ")",
	}
}

func (t *TupleType) Pretty() pretty.Formatter {
	seenFormatters := map[Type]pretty.Formatter{}
	return t.pretty(seenFormatters)
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*TupleType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// Traverse attempts to traverse the tuple type with the given traverser. This always fails.
func (t *TupleType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	key, keyType := GetTraverserKey(traverser)

	if !InputType(NumberType).AssignableFrom(keyType) {
		return DynamicType, hcl.Diagnostics{unsupportedTupleIndex(traverser.SourceRange())}
	}

	if key == cty.DynamicVal {
		if t.elementUnion == nil {
			t.elementUnion = NewUnionType(t.ElementTypes...)
		}
		return t.elementUnion, nil
	}

	elementIndex, acc := key.AsBigFloat().Int64()
	if acc != big.Exact {
		return DynamicType, hcl.Diagnostics{unsupportedTupleIndex(traverser.SourceRange())}
	}
	if elementIndex < 0 || elementIndex >= int64(len(t.ElementTypes)) {
		return DynamicType, hcl.Diagnostics{tupleIndexOutOfRange(len(t.ElementTypes), traverser.SourceRange())}
	}
	return t.ElementTypes[int(elementIndex)], nil
}

// Equals returns true if this type has the same identity as the given type.
func (t *TupleType) Equals(other Type) bool {
	return t.equals(other, nil)
}

func (t *TupleType) equals(other Type, seen equalPairs) bool {
	if t == other {
		return true
	}
	otherTuple, ok := other.(*TupleType)
	if !ok {
		return false
	}
	if len(t.ElementTypes) != len(otherTuple.ElementTypes) {
		return false
	}
	for i, t := range t.ElementTypes {
		if !t.equals(otherTuple.ElementTypes[i], seen) {
			return false
		}
	}
	return true
}

// AssignableFrom returns true if this type is assignable from the indicated source type..
func (t *TupleType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		if src, ok := src.(*TupleType); ok {
			for i := 0; i < len(t.ElementTypes); i++ {
				srcElement := NoneType
				if i < len(src.ElementTypes) {
					srcElement = src.ElementTypes[i]
				}
				if !t.ElementTypes[i].AssignableFrom(srcElement) {
					return false
				}
			}
			return true
		}
		return false
	})
}

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type. A tuple
// converts from a tuple of the same length element by element, and unsafely from a list or a set whose element type
// converts to every element type.
func (t *TupleType) ConversionFrom(src Type) ConversionKind {
	return cachedConversionFrom(t, src, t.cache)
}

func (t *TupleType) String() string {
	return t.string(nil)
}

func (t *TupleType) string(seen map[Type]struct{}) string {
	if s := t.s.Load(); s != nil {
		return s.(string)
	}

	elements := make([]string, len(t.ElementTypes))
	for i, e := range t.ElementTypes {
		elements[i] = e.string(seen)
	}

	s := fmt.Sprintf("tuple(%s)", strings.Join(elements, ", "))
	t.s.Store(s)
	return s
}

func (*TupleType) isType() {}
