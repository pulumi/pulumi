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

package importer

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils/rapidresource"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils/rapidschema"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

// TestRapidResourceValuesAreStructurallyTyped draws a schema, then for each
// resource in the schema draws inputs/outputs/state and asserts that the
// resulting property maps are structurally typed against the resource's
// declared property lists.
func TestRapidResourceValuesAreStructurallyTyped(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		pkg := rapidschema.Package().Draw(t, "pkg")
		for _, r := range pkg.Resources {
			inputs := rapidresource.ResourceInputs(r).Draw(t, "inputs:"+r.Token)
			assertMapStructurallyTyped(t, inputs, r.InputProperties)

			outputs := rapidresource.ResourceProperties(r).Draw(t, "outputs:"+r.Token)
			assertMapStructurallyTyped(t, outputs, r.Properties)

			state := rapidresource.ResourceState(r).Draw(t, "state:"+r.Token)
			if r.StateInputs == nil {
				require.Nil(t, state, "state should be nil when StateInputs is nil")
			} else {
				require.NotNil(t, state)
				assertMapStructurallyTyped(t, *state, r.StateInputs.Properties)
			}
		}
	})
}

// assertMapStructurallyTyped checks that every present key in m is declared
// in props, every required prop is present (null is permitted when the
// declared type accepts it), and every value matches its declared schema
// type.
func assertMapStructurallyTyped(t require.TestingT, m property.Map, props []*schema.Property) {
	require.Truef(t, mapMatches(m, props), "map %v does not match property list", m)
}

func mapMatches(m property.Map, props []*schema.Property) bool {
	byName := map[string]*schema.Property{}
	for _, p := range props {
		byName[p.Name] = p
	}
	for k, v := range m.AsMap() {
		p, ok := byName[k]
		if !ok {
			return false
		}
		if !valueMatches(v, p.Type) {
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

// valueMatches recursively checks v against typ, handling the wrapper and
// builtin types that valueStructurallyTypedAs does not. For ObjectType /
// UnionType / ArrayType / primitive cases it delegates to
// valueStructurallyTypedAs.
func valueMatches(v property.Value, typ schema.Type) bool {
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
			if !valueMatches(vv, tt.ElementType) {
				return false
			}
		}
		return true
	case *schema.EnumType:
		return enumValueMatches(v, tt)
	case *schema.ResourceType:
		return v.IsResourceReference()
	case *schema.ObjectType:
		if !v.IsMap() {
			return false
		}
		return mapMatches(v.AsMap(), tt.Properties)
	case *schema.ArrayType:
		if !v.IsArray() {
			return false
		}
		for _, e := range v.AsArray().AsSlice() {
			if !valueMatches(e, tt.ElementType) {
				return false
			}
		}
		return true
	case *schema.UnionType:
		for _, e := range tt.ElementTypes {
			if valueMatches(v, e) {
				return true
			}
		}
		return false
	}

	// Primitive cases: bool, number/int, string.
	return valueStructurallyTypedAs(v, typ)
}

// enumValueMatches returns true if v equals one of the enum's declared values.
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

// jsonLikeValue accepts any property.Value shape that can round-trip through
// JSON: primitives (incl. null), arrays of jsonLikeValue, maps to
// jsonLikeValue.
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
