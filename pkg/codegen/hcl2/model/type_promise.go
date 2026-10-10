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

// PromiseType represents eventual values that do not carry additional information.
type PromiseType struct {
	// ElementType is the element type of the promise.
	ElementType Type

	cache *typeCache
}

// NewPromiseType creates a new promise type with the given element type after replacing any promise types within
// the element type with their respective element types.
func NewPromiseType(elementType Type) *PromiseType {
	return newPromiseType(ResolvePromises(elementType))
}

// newPromiseType wraps an element type that holds no promise.
func newPromiseType(resolved Type) *PromiseType {
	return &PromiseType{ElementType: resolved, cache: &typeCache{}}
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*PromiseType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *PromiseType) pretty(seenFormatters map[Type]pretty.Formatter) pretty.Formatter {
	var formatter pretty.Formatter
	if seenFormatter, ok := seenFormatters[t.ElementType]; ok {
		formatter = seenFormatter
	} else {
		formatter = t.ElementType.pretty(seenFormatters)
	}

	return &pretty.Wrap{
		Prefix:  "promise(",
		Postfix: ")",
		Value:   formatter,
	}
}

func (t *PromiseType) Pretty() pretty.Formatter {
	seenFormatters := map[Type]pretty.Formatter{}
	return t.pretty(seenFormatters)
}

// Traverse attempts to traverse the promise type with the given traverser. The result type of traverse(promise(T))
// is promise(traverse(T)).
func (t *PromiseType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	element, diagnostics := t.ElementType.Traverse(traverser)
	return NewPromiseType(element.(Type)), diagnostics
}

// Equals returns true if this type has the same identity as the given type.
func (t *PromiseType) Equals(other Type) bool {
	return t.equals(other, nil)
}

func (t *PromiseType) equals(other Type, seen equalPairs) bool {
	if t == other {
		return true
	}
	otherPromise, ok := other.(*PromiseType)
	return ok && t.ElementType.equals(otherPromise.ElementType, seen)
}

// AssignableFrom returns true if this type is assignable from the indicated source type. A promise(T) is assignable
// from values of type promise(U) and U, where T is assignable from U.
func (t *PromiseType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		if src, ok := src.(*PromiseType); ok {
			return t.ElementType.AssignableFrom(src.ElementType)
		}
		return t.ElementType.AssignableFrom(src)
	})
}

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type. A
// promise(T) converts from no output, and from any other type U as T converts from U with every promise at any
// depth of U removed (README §4, C-Prom).
func (t *PromiseType) ConversionFrom(src Type) ConversionKind {
	return cachedConversionFrom(t, src, t.cache)
}

func (t *PromiseType) String() string {
	return t.string(nil)
}

func (t *PromiseType) string(seen map[Type]struct{}) string {
	return fmt.Sprintf("promise(%s)", t.ElementType.string(seen))
}

func (t *PromiseType) isType() {}
