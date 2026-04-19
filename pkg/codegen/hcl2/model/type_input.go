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
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/gsync"
)

// InputType represents values that may be provided either directly or eventually.
type InputType struct {
	// ElementType is the wrapped element type.
	ElementType Type

	cache *gsync.Map[Type, cacheEntry]
}

// newInputType creates a new input wrapper with the given element type.
func newInputType(elementType Type) *InputType {
	if input, ok := elementType.(*InputType); ok {
		return input
	}
	return &InputType{ElementType: elementType, cache: &gsync.Map[Type, cacheEntry]{}}
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*InputType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *InputType) pretty(seenFormatters map[Type]pretty.Formatter) pretty.Formatter {
	var formatter pretty.Formatter
	if seenFormatter, ok := seenFormatters[t.ElementType]; ok {
		formatter = seenFormatter
	} else {
		formatter = t.ElementType.pretty(seenFormatters)
	}

	return &pretty.Wrap{
		Prefix:  "input(",
		Postfix: ")",
		Value:   formatter,
	}
}

func (t *InputType) Pretty() pretty.Formatter {
	seenFormatters := map[Type]pretty.Formatter{}
	return t.pretty(seenFormatters)
}

// Traverse attempts to traverse the input type with the given traverser. The result type of traverse(input(T))
// is input(traverse(T)).
func (t *InputType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	element, diagnostics := t.ElementType.Traverse(traverser)
	return newInputType(element.(Type)), diagnostics
}

// Equals returns true if this type has the same identity as the given type.
func (t *InputType) Equals(other Type) bool {
	return t.equals(other, nil)
}

func (t *InputType) equals(other Type, seen map[Type]struct{}) bool {
	if t == other {
		return true
	}
	otherInput, ok := other.(*InputType)
	return ok && t.ElementType.equals(otherInput.ElementType, seen)
}

// AssignableFrom returns true if this type is assignable from the indicated source type.
func (t *InputType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		if t.ElementType.AssignableFrom(src) {
			return true
		}
		return NewOutputType(t.ElementType).AssignableFrom(src)
	})
}

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type.
func (t *InputType) ConversionFrom(src Type) ConversionKind {
	kind, _ := t.conversionFrom(src, false, nil)
	return kind
}

func (t *InputType) conversionFrom(src Type, unifying bool, seen map[Type]struct{}) (ConversionKind, lazyDiagnostics) {
	return conversionFrom(t, src, unifying, seen, t.cache, func() (ConversionKind, lazyDiagnostics) {
		elementKind, elementDiags := t.ElementType.conversionFrom(src, unifying, seen)
		outputKind, outputDiags := NewOutputType(t.ElementType).conversionFrom(src, unifying, seen)

		if outputKind > elementKind {
			return outputKind, outputDiags
		}
		return elementKind, elementDiags
	})
}

func (t *InputType) String() string {
	return t.string(nil)
}

func (t *InputType) string(seen map[Type]struct{}) string {
	return fmt.Sprintf("input(%s)", t.ElementType.string(seen))
}

func (t *InputType) unify(other Type) (Type, ConversionKind) {
	return unify(t, other, func() (Type, ConversionKind) {
		switch other := other.(type) {
		case *InputType:
			elementType, conversionKind := t.ElementType.unify(other.ElementType)
			return newInputType(elementType), conversionKind
		default:
			kind, _ := t.conversionFrom(other, true, nil)
			return t, kind
		}
	})
}

func (*InputType) isType() {}
