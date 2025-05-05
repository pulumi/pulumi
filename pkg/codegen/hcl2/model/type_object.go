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
	"sync/atomic"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/pretty"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// ObjectType represents schematized maps from strings to particular types.
type ObjectType struct {
	// Properties records the types of the object's properties.
	Properties map[string]Type
	// Annotations records any annotations associated with the object type.
	Annotations []interface{}

	propertyUnion Type
	s             atomic.Value // Value<string>

	cache conversionCache
}

// NewObjectType creates a new object type with the given properties and annotations.
func NewObjectType(properties map[string]Type, annotations ...interface{}) *ObjectType {
	return &ObjectType{Properties: properties, Annotations: annotations}
}

// Annotate adds annotations to the object type. Annotations may be retrieved by GetObjectTypeAnnotation.
func (t *ObjectType) Annotate(annotations ...interface{}) {
	t.Annotations = append(t.Annotations, annotations...)
}

// GetObjectTypeAnnotation retrieves an annotation of the given type from the object type, if one exists.
func GetObjectTypeAnnotation[T any](t *ObjectType) (T, bool) {
	var result T
	found := false
	for _, a := range t.Annotations {
		if v, ok := a.(T); ok {
			result = v
			found = true
			break
		}
	}
	return result, found
}

// SyntaxNode returns the syntax node for the type. This is always syntax.None.
func (*ObjectType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (t *ObjectType) pretty(seenFormatters map[Type]pretty.Formatter) pretty.Formatter {
	if existingFormatter, ok := seenFormatters[t]; ok {
		return existingFormatter
	}

	m := make(map[string]pretty.Formatter, len(t.Properties))
	seenFormatters[t] = &pretty.Object{Properties: m}
	for k, v := range t.Properties {
		if seenFormatter, ok := seenFormatters[v]; ok {
			m[k] = seenFormatter
		} else {
			formatter := v.pretty(seenFormatters)
			seenFormatters[v] = formatter
			m[k] = formatter
		}
	}

	return seenFormatters[t]
}

func (t *ObjectType) Pretty() pretty.Formatter {
	seenFormatters := map[Type]pretty.Formatter{}
	return t.pretty(seenFormatters)
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
			types := slice.Prealloc[Type](len(t.Properties))
			for _, t := range t.Properties {
				types = append(types, t)
			}
			t.propertyUnion = NewUnionType(types...)
		}
		return t.propertyUnion, nil
	}

	keyString, err := convert.Convert(key, cty.String)
	contract.Assertf(err == nil, "error converting key (%#v) to string", key)

	propertiesLower := make(map[string]string)
	for p := range t.Properties {
		propertiesLower[strings.ToLower(p)] = p
	}

	propertyName := keyString.AsString()
	propertyType, hasProperty := t.Properties[propertyName]
	if !hasProperty {
		propertyNameLower := strings.ToLower(propertyName)
		if propertyNameOrig, ok := propertiesLower[propertyNameLower]; ok {
			propertyType = t.Properties[propertyNameOrig]
			rng := traverser.SourceRange()
			return propertyType, hcl.Diagnostics{
				{
					Severity: hcl.DiagWarning,
					Subject:  &rng,
					Summary:  "Found matching case-insensitive property",
					Detail:   fmt.Sprintf("Matched %s with %s", propertyName, propertyNameOrig),
				},
			}
		}
		props := slice.Prealloc[string](len(t.Properties))
		for k := range t.Properties {
			props = append(props, k)
		}
		return DynamicType, hcl.Diagnostics{unknownObjectProperty(propertyName, traverser.SourceRange(), props)}
	}
	return propertyType, nil
}

// Equals returns true if this type has the same identity as the given type.
func (t *ObjectType) Equals(other Type) bool {
	return t.equals(other, nil)
}

func (t *ObjectType) equals(other Type, seen map[Type]struct{}) bool {
	if t == other {
		return true
	}
	if seen != nil {
		if _, ok := seen[t]; ok {
			return true
		}
	} else {
		seen = map[Type]struct{}{}
	}
	seen[t] = struct{}{}

	otherObject, ok := other.(*ObjectType)
	if !ok {
		return false
	}
	if len(t.Properties) != len(otherObject.Properties) {
		return false
	}
	for k, t := range t.Properties {
		if u, ok := otherObject.Properties[k]; !ok || !t.equals(u, seen) {
			return false
		}
	}
	return true
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

// ConversionFrom returns the kind of conversion (if any) that is possible from the source type to this type.
//
// An object({K_0 = T_0, ..., K_N = T_N}) is convertible from object({K_0 = U_0, ... K_M = U_M}) if all properties
// that exist in both types are convertible, and any keys that do not exist in the source type are optional in
// the destination type. If any of these conversions are unsafe, the whole conversion is unsafe; otherwise, the
// conversion is safe.
//
// An object({K_0 = T_0, ..., K_N = T_N}) is convertible from a map(U) if U is convertible to all of T_0 through T_N.
// This conversion is always unsafe, and may fail if the map does not contain an appropriate set of keys for the
// destination type.
func (t *ObjectType) ConversionFrom(src Type) ConversionKind {
	kind, _ := t.conversionFrom(src, false, nil)
	return kind
}

func (t *ObjectType) conversionFrom(src Type, unifying bool, seen map[Type]struct{}) (ConversionKind, lazyDiagnostics) {
	if t.cache == nil {
		t.cache = make(conversionCache)
	}
	return conversionFrom(t, src, unifying, seen, t.cache, func() (ConversionKind, lazyDiagnostics) {
		switch src := src.(type) {
		case *ObjectType:
			if seen != nil {
				if _, ok := seen[t]; ok {
					return NoConversion, func() hcl.Diagnostics { return hcl.Diagnostics{invalidRecursiveType(t)} }
				}
			} else {
				seen = map[Type]struct{}{}
			}
			seen[t] = struct{}{}
			defer delete(seen, t)

			if unifying {
				var unifier objectTypeUnifier
				unifier.unify(t)
				unifier.unify(src)
				return unifier.conversionKind, nil
			}

			conversionKind := SafeConversion
			var diags lazyDiagnostics
			for k, dst := range t.Properties {
				src, ok := src.Properties[k]
				if !ok {
					src = NoneType
				}
				if ck, why := dst.conversionFrom(src, unifying, seen); ck < conversionKind {
					conversionKind, diags = ck, why
					if conversionKind == NoConversion {
						break
					}
				}
			}
			return conversionKind, diags
		case *MapType:
			conversionKind := UnsafeConversion
			var diags lazyDiagnostics
			for _, dst := range t.Properties {
				if ck, why := dst.conversionFrom(src.ElementType, unifying, seen); ck < conversionKind {
					conversionKind, diags = ck, why
					if conversionKind == NoConversion {
						break
					}
				}
			}
			return conversionKind, diags
		}
		return NoConversion, func() hcl.Diagnostics { return hcl.Diagnostics{typeNotConvertible(t, src)} }
	})
}

func (t *ObjectType) String() string {
	return t.string(nil)
}

func (t *ObjectType) string(seen map[Type]struct{}) string {
	if s := t.s.Load(); s != nil {
		return s.(string)
	}

	if seen != nil {
		if _, ok := seen[t]; ok {
			return "..."
		}
	} else {
		seen = map[Type]struct{}{}
	}
	seen[t] = struct{}{}

	properties := slice.Prealloc[string](len(t.Properties))
	for k, v := range t.Properties {
		properties = append(properties, fmt.Sprintf("%s = %s", k, v.string(seen)))
	}
	sort.Strings(properties)

	annotations := ""
	if len(t.Annotations) != 0 {
		annotations = fmt.Sprintf(", annotated(%p)", t)
	}

	s := fmt.Sprintf("object({%s}%v)", strings.Join(properties, ", "), annotations)
	t.s.Store(s)
	return s
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
			kind, _ := t.conversionFrom(other, true, nil)
			return t, kind
		}
	})
}

func (*ObjectType) isType() {}
