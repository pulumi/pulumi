// Copyright 2026, Pulumi Corporation.
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

package rapidresource

import (
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// MapStructurallyTyped reports whether m matches the given property list:
// every present key is declared, every required property is present (null is
// permitted when the declared type accepts it), and every value is itself
// [valueStructurallyTyped] against the property's declared schema type.
func MapStructurallyTyped(m property.Map, props []*schema.Property) bool {
	byName := make(map[string]*schema.Property, len(props))
	for _, p := range props {
		byName[p.Name] = p
	}
	for k, v := range m.AsMap() {
		p, ok := byName[k]
		if !ok {
			return false
		}
		if !valueStructurallyTyped(v, p.Type) {
			return false
		}
	}
	for _, p := range props {
		if !p.IsRequired() {
			continue
		}
		if _, ok := m.GetOk(p.Name); !ok {
			return false
		}
	}
	return true
}

// valueStructurallyTyped reports whether v matches the given schema type.
// Wrappers (OptionalType, InputType, TokenType) are stripped first; null
// values are accepted for OptionalType. Composite types recurse; primitive
// types check the property.Value's discriminator.
func valueStructurallyTyped(v property.Value, typ schema.Type) bool {
	if opt, ok := typ.(*schema.OptionalType); ok {
		if v.IsNull() {
			return true
		}
		typ = opt.ElementType
	}
	if tok, ok := typ.(*schema.TokenType); ok {
		if tok.UnderlyingType != nil {
			typ = tok.UnderlyingType
		} else {
			return v.IsString()
		}
	}
	typ = codegen.UnwrapType(typ)

	switch typ {
	case schema.BoolType:
		return v.IsBool()
	case schema.IntType, schema.NumberType:
		return v.IsNumber()
	case schema.StringType:
		return v.IsString()
	case schema.ArchiveType:
		return v.IsArchive()
	case schema.AssetType:
		return v.IsAsset()
	case schema.JSONType:
		return jsonLikeValue(v)
	case schema.AnyType, schema.AnyResourceType:
		return jsonLikeValue(v) || v.IsAsset() || v.IsArchive()
	}

	switch tt := typ.(type) {
	case *schema.MapType:
		if !v.IsMap() {
			return false
		}
		for _, vv := range v.AsMap().AsMap() {
			if !valueStructurallyTyped(vv, tt.ElementType) {
				return false
			}
		}
		return true
	case *schema.ArrayType:
		if !v.IsArray() {
			return false
		}
		for _, e := range v.AsArray().AsSlice() {
			if !valueStructurallyTyped(e, tt.ElementType) {
				return false
			}
		}
		return true
	case *schema.ObjectType:
		if !v.IsMap() {
			return false
		}
		return MapStructurallyTyped(v.AsMap(), tt.Properties)
	case *schema.UnionType:
		for _, e := range tt.ElementTypes {
			if valueStructurallyTyped(v, e) {
				return true
			}
		}
		return false
	case *schema.EnumType:
		return enumValueMatches(v, tt)
	case *schema.ResourceType:
		return v.IsResourceReference()
	}
	return false
}

func enumValueMatches(v property.Value, e *schema.EnumType) bool {
	for _, el := range e.Elements {
		if propertyValueMatchesAny(v, el.Value) {
			return true
		}
	}
	return false
}

func propertyValueMatchesAny(v property.Value, want any) bool {
	switch w := want.(type) {
	case bool:
		return v.IsBool() && v.AsBool() == w
	case string:
		return v.IsString() && v.AsString() == w
	case int32:
		return v.IsNumber() && v.AsNumber() == float64(w)
	case int:
		return v.IsNumber() && v.AsNumber() == float64(w)
	case int64:
		return v.IsNumber() && v.AsNumber() == float64(w)
	case float64:
		return v.IsNumber() && v.AsNumber() == w
	}
	return false
}

// jsonLikeValue reports whether v has a shape that can round-trip through
// JSON: a primitive (including null), an array of jsonLikeValues, or a map
// to jsonLikeValues.
func jsonLikeValue(v property.Value) bool {
	switch {
	case v.IsNull(), v.IsBool(), v.IsNumber(), v.IsString():
		return true
	case v.IsArray():
		for _, e := range v.AsArray().AsSlice() {
			if !jsonLikeValue(e) {
				return false
			}
		}
		return true
	case v.IsMap():
		for _, vv := range v.AsMap().AsMap() {
			if !jsonLikeValue(vv) {
				return false
			}
		}
		return true
	}
	return false
}
