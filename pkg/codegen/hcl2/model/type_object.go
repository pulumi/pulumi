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
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

// ObjectType represents schematized maps from strings to particular types.
type ObjectType struct {
	// Properties records the types of the object's properties.
	Properties map[string]Type

	propertyUnion Type
	s             string
}

// The set of object types, indexed by string representation.
var objectTypes = map[string]*ObjectType{}

// NewObjectType creates a new object type with the given properties.
func NewObjectType(properties map[string]Type) *ObjectType {
	t := &ObjectType{Properties: properties}
	if t, ok := objectTypes[t.String()]; ok {
		return t
	}
	objectTypes[t.String()] = t
	return t
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*ObjectType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// Traverse attempts to traverse the optional type with the given traverser. The result type of
// traverse(object({K_0 = T_0, ..., K_N = T_N})) is T_i if the traverser is the string literal K_i. If the traverser is
// a string but not a literal, the result type is any.
func (t *ObjectType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	key, keyType := GetTraverserKey(traverser)

	if !InputType(StringType).ConversionFrom(keyType).Exists() {
		return DynamicType, hcl.Diagnostics{unsupportedObjectProperty(traverser.SourceRange())}
	}

	if key == cty.DynamicVal {
		if t.propertyUnion == nil {
			types := make([]Type, 0, len(t.Properties))
			for _, t := range t.Properties {
				types = append(types, t)
			}
			t.propertyUnion = NewUnionType(types...)
		}
		return t.propertyUnion, nil
	}

	keyString, err := convert.Convert(key, cty.String)
	contract.Assert(err == nil)

	propertyName := keyString.AsString()
	propertyType, hasProperty := t.Properties[propertyName]
	if !hasProperty {
		return DynamicType, hcl.Diagnostics{unknownObjectProperty(propertyName, traverser.SourceRange())}
	}
	return propertyType, nil
}

// AssignableFrom returns true if this type is assignable from the indicated source type.
// An object({K_0 = T_0, ..., K_N = T_N}) is assignable from U = object({K_0 = U_0, ... K_M = U_M}), where T_I is
// assignable from U[K_I] for all I from 0 to N.
func (t *ObjectType) AssignableFrom(src Type) bool {
	return assignableFrom(t, src, func() bool {
		if src, ok := src.(*ObjectType); ok {
			for key, t := range t.Properties {
				src, ok := src.Properties[key]
				if !ok {
					src = NoneType
				}
				if !t.AssignableFrom(src) {
					return false
				}
			}
			return true
		}
		return false
	})
}

type objectTypeUnifier struct {
	properties     map[string]Type
	any            bool
	conversionKind ConversionKind
}

func (u *objectTypeUnifier) unify(t *ObjectType) {
	if !u.any {
		u.properties = map[string]Type{}
		for k, t := range t.Properties {
			u.properties[k] = t
		}
		u.any, u.conversionKind = true, SafeConversion
	} else {
		for key, pt := range u.properties {
			if _, exists := t.Properties[key]; !exists {
				u.properties[key] = NewOptionalType(pt)
			}
		}

		for key, t := range t.Properties {
			if pt, exists := u.properties[key]; exists {
				unified, ck := pt.unify(t)
				if ck < u.conversionKind {
					u.conversionKind = ck
				}
				u.properties[key] = unified
			} else {
				u.properties[key] = NewOptionalType(t)
			}
		}
	}
}

func (t *ObjectType) ConversionFrom(src Type) ConversionKind {
	return t.conversionFrom(src, false)
}

func (t *ObjectType) conversionFrom(src Type, unifying bool) ConversionKind {
	return conversionFrom(t, src, unifying, func() ConversionKind {
		switch src := src.(type) {
		case *ObjectType:
			if unifying {
				var unifier objectTypeUnifier
				unifier.unify(t)
				unifier.unify(src)
				return unifier.conversionKind
			}

			conversionKind := SafeConversion
			for k, dst := range t.Properties {
				src, ok := src.Properties[k]
				if !ok {
					src = NoneType
				}
				if ck := dst.conversionFrom(src, unifying); ck < conversionKind {
					conversionKind = ck
				}
			}
			return conversionKind
		case *MapType:
			conversionKind := UnsafeConversion
			for _, dst := range t.Properties {
				if ck := dst.conversionFrom(src.ElementType, unifying); ck < conversionKind {
					conversionKind = ck
				}
			}
			return conversionKind
		}
		return NoConversion
	})
}

func (t *ObjectType) String() string {
	if t.s == "" {
		var properties []string
		for k, v := range t.Properties {
			properties = append(properties, fmt.Sprintf("%s = %v", k, v))
		}
		sort.Strings(properties)

		t.s = fmt.Sprintf("object({%s})", strings.Join(properties, ", "))
	}
	return t.s
}

func (t *ObjectType) unify(other Type) (Type, ConversionKind) {
	return unify(t, other, func() (Type, ConversionKind) {
		switch other := other.(type) {
		case *MapType:
			// Prefer the map type, but unify the element type.
			elementType, conversionKind := other.ElementType, SafeConversion
			for _, t := range t.Properties {
				element, ck := elementType.unify(t)
				if ck < conversionKind {
					conversionKind = ck
				}
				elementType = element
			}
			return NewMapType(elementType), conversionKind
		case *ObjectType:
			// If the other type is an object type, produce a new type whose properties are the union of the two types.
			// The types of intersecting properties will be unified.
			var unifier objectTypeUnifier
			unifier.unify(t)
			unifier.unify(other)
			return NewObjectType(unifier.properties), unifier.conversionKind
		default:
			// Otherwise, prefer the object type.
			return t, t.conversionFrom(other, true)
		}
	})
}

func (*ObjectType) isType() {}
