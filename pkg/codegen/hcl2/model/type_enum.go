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
	"strings"
	"sync/atomic"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/pretty"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// EnumType represents values of a single type, and a closed set of possible values.
type EnumType struct {
	// Elements are the possible values of the enum.
	Elements []cty.Value

	// The type of the enum's values.
	Type Type

	// Token that uniquely identifies a EnumType.
	// Given EnumA, EnumB of type EnumType
	// 		EnumA.Token != EnumB.Token => EnumA != EnumB (regardless of other fields)
	//
	// It is the responsibility of enum constructors to ensure that given EnumA,
	// EnumB of type EnumType
	//
	// 		EnumA.Token = EnumB.Token => EnumA = EnumB (all fields match)
	//
	// Failure to do so may lead to panics.
	Token string
	// TODO: Refactor the token out into NamedType<EnumType>
	// See https://github.com/pulumi/pulumi/pull/9290#discussion_r851356288

	// Annotations records any annotations associated with the object type.
	Annotations []interface{}

	s atomic.Value // Value<string>

	cache *gsync.Map[Type, cacheEntry]
}

func NewEnumType(token string, typ Type, elements []cty.Value, annotations ...interface{}) *EnumType {
	contract.Assertf(len(elements) > 0, "Enums must be represent-able")

	t := elements[0].Type()
	for _, e := range elements[1:] {
		contract.Assertf(e.Type() == t,
			"Elements in an emum must have the same type")
	}

	return &EnumType{
		Type:        typ,
		Annotations: annotations,
		Elements:    elements,
		Token:       token,
	}
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*EnumType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *EnumType) pretty(seenFormatters map[Type]pretty.Formatter) pretty.Formatter {
	types := make([]pretty.Formatter, len(t.Elements))
	for i, c := range t.Elements {
		types[i] = pretty.FromStringer(
			NewConstType(ctyTypeToType(c.Type(), false), c).Pretty(),
		)
	}
	seenFormatters[t] = &pretty.Wrap{
		Prefix:  "enum(",
		Postfix: ")",
		Value: &pretty.List{
			Separator: " | ",
			Elements:  types,
		},
	}

	return seenFormatters[t]
}

func (t *EnumType) Pretty() pretty.Formatter {
	seenFormatters := map[Type]pretty.Formatter{}
	return t.pretty(seenFormatters)
}

// Traverse attempts to traverse the enum type with the given traverser. This always fails.
func (t *EnumType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	out := *t
	return &out, nil
}

// Equals returns true if this type has the same identity as the given type.
func (t *EnumType) Equals(other Type) bool {
	return t.equals(other, nil)
}

func (t *EnumType) equals(other Type, seen map[Type]struct{}) bool {
	if t == other {
		return true
	}
	otherEnum, ok := other.(*EnumType)
	if !ok {
		return false
	}
	if t.Token != otherEnum.Token {
		return false
	}
	contract.Assertf(len(t.Elements) == len(otherEnum.Elements),
		"The same token implies the same enum, this is just a reality check")

	return true
}

// AssignableFrom returns true if this type is assignable from the indicated
// source type. The whole point of enums is they are a unique type, so we can't
// convert between them.
func (t *EnumType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		return false
	})
}

// ConversionFrom returns the kind of conversion (if any) that is possible from
// the source type to this type. Most languages support casting between value
// types and an enum of that value. When the value is constant, we can determine
// if the cast is valid. Otherwise it is valid but unsafe.
func (t *EnumType) ConversionFrom(src Type) ConversionKind {
	kind, _ := t.conversionFrom(src, false, nil)
	return kind
}

func (t *EnumType) conversionFrom(src Type, unifying bool, seen map[Type]struct{}) (ConversionKind, lazyDiagnostics) {
	if t.cache == nil {
		t.cache = &gsync.Map[Type, cacheEntry]{}
	}
	return conversionFrom(t, src, unifying, seen, t.cache, func() (ConversionKind, lazyDiagnostics) {
		// We have a constant, of the correct type, so we might have a safe
		// conversion.
		if src, ok := src.(*ConstType); ok && !t.Type.Equals(src.Type) {
			for _, el := range t.Elements {
				if el.Equals(src.Value).True() {
					return SafeConversion, nil
				}
			}
		}
		con, diags := t.Type.conversionFrom(src, unifying, seen)
		if con == NoConversion {
			return NoConversion, diags
		}
		// We can perform a conversion. Because the value might not be valid, it
		// is always unsafe.
		return UnsafeConversion, diags
	})
}

func (t *EnumType) String() string {
	return t.string(nil)
}

func (t *EnumType) string(seen map[Type]struct{}) string {
	if s := t.s.Load(); s != nil {
		return s.(string)
	}

	underlying := t.Type.string(seen)
	elements := make([]string, len(t.Elements))
	for i, e := range t.Elements {
		elements[i] = e.GoString()
	}

	s := fmt.Sprintf("enum(%s(%s): %s)", t.Token, underlying, strings.Join(elements, ","))
	t.s.Store(s)
	return s
}

func (t *EnumType) unify(other Type) (Type, ConversionKind) {
	return unify(t, other, func() (Type, ConversionKind) {
		return nil, NoConversion
	})
}
func (*EnumType) isType() {}
