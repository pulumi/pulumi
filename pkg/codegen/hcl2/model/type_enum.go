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

	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// EnumType represents values of a single type, and a closed set of possible values.
type EnumType struct {
	// Elements are the possible values of the enum.
	Elements []cty.Value

	// The type of the enum's values.
	Type Type

	// The associated schema type. It is necessary to include this, since
	// otherwise identical types are distinct if they have a different schema
	// type.
	Token string

	s   string
	src *schema.EnumType
}

func NewEnumType(typ Type, src *schema.EnumType, elements ...cty.Value) *EnumType {
	if len(elements) != 0 {
		t := elements[0].Type()
		for _, e := range elements[1:] {
			contract.Assertf(e.Type() == t,
				"Elements in an emum must have the same type")
		}
	}
	return &EnumType{
		Type:     typ,
		Token:    src.Token,
		Elements: elements,
		src:      src,
	}
}

// Member returns the name of the member that matches the given `value`. If no
// member if found, the empty string is returned. If the type of `value` does
// not match the type of the enum, Member will panic.
func (t *EnumType) Member(value cty.Value) *schema.Enum {
	switch {
	case t.Type.Equals(StringType):
		s := value.AsString()
		for _, el := range t.src.Elements {
			v := el.Value.(string)
			if v == s {
				return el
			}
		}
	case t.Type.Equals(NumberType):
		f, _ := value.AsBigFloat().Float64()
		for _, el := range t.src.Elements {
			if el.Value.(float64) == f {
				return el
			}
		}
	case t.Type.Equals(IntType):
		f, _ := value.AsBigFloat().Int64()
		for _, el := range t.src.Elements {
			if el.Value.(int64) == f {
				return el
			}
		}
	default:
		contract.Failf("Unknown enum type '%s' for '%s'", t.Type, t.Token)
	}
	return nil
}

// LanguageOptions provides the language map associated with the enums
// *schema.Package.
func (t *EnumType) LanguageOptions() map[string]interface{} {
	return t.src.Package.Language
}

// GenEnum is a helper function when generating an enum.
// Given an enum, and instructions on what to do when you find a known value,
// and an unknown value, return a function that will generate an the given enum
// from the given expression.
//
// This function should probably live in the `codegen` namespace, but cannot
// because of import cycles.
func (t *EnumType) GenEnum(
	from Expression,
	safeEnum func(member *schema.Enum),
	unsafeEnum func(from Expression),
) {
	known := cty.NilVal
	if from, ok := from.(*TemplateExpression); ok && len(from.Parts) == 1 {
		if from, ok := from.Parts[0].(*LiteralValueExpression); ok {
			known = from.Value
		}
	}
	if from, ok := from.(*LiteralValueExpression); ok {
		known = from.Value
	}
	if known != cty.NilVal {
		// If the value is known, but we can't find a member, we should have
		// indicated a conversion is impossible when type checking.
		member := t.Member(known)
		contract.Assertf(member != nil,
			"We have determined %s is a safe enum, which we define as "+
				"being able to calculate a member for", t)
		safeEnum(member)
	} else {
		unsafeEnum(from)
	}
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*EnumType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// Traverse attempts to traverse the enum type with the given traverser. This always fails.
func (t *EnumType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	return &*t, nil
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
	return conversionFrom(t, src, unifying, seen, func() (ConversionKind, lazyDiagnostics) {
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
	if t.s == "" {
		underlying := t.Type.string(seen)
		elements := make([]string, len(t.Elements))
		for i, e := range t.Elements {
			elements[i] = e.GoString()
		}

		t.s = fmt.Sprintf("enum(%s(%s): %s)", t.Token, underlying, strings.Join(elements, ","))
	}
	return t.s
}

func (t *EnumType) unify(other Type) (Type, ConversionKind) {
	return unify(t, other, func() (Type, ConversionKind) {
		return nil, NoConversion
	})
}
func (*EnumType) isType() {}
