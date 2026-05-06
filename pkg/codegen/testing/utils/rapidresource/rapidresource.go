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

// Package rapidresource provides rapid (property-based) generators for
// schema-conforming property maps. Given a bound schema.Resource, the
// generators draw values that satisfy the resource's input/output/state
// property declarations.
package rapidresource

import (
	"fmt"

	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	propertytest "github.com/pulumi/pulumi/sdk/v3/go/property/testing"
)

// ResourceInputs returns a generator for a property.Map matching the
// resource's input properties.
func ResourceInputs(r *schema.Resource) *rapid.Generator[property.Map] {
	return propertyMapGenerator(r.InputProperties)
}

// ResourceProperties returns a generator for a property.Map matching the
// resource's output properties.
func ResourceProperties(r *schema.Resource) *rapid.Generator[property.Map] {
	return propertyMapGenerator(r.Properties)
}

// ResourceState returns a generator for the resource's state-input map. When
// the resource declares no state inputs the generator yields nil.
func ResourceState(r *schema.Resource) *rapid.Generator[*property.Map] {
	if r.StateInputs == nil {
		return rapid.Just[*property.Map](nil)
	}
	inner := propertyMapGenerator(r.StateInputs.Properties)
	return rapid.Map(inner, func(m property.Map) *property.Map { return &m })
}

// maxValueDepth caps recursion through arrays, maps, and JSON-shaped values.
// The schema generator forbids required-recursive object cycles, so the only
// remaining unbounded recursion is through containers — which can still
// blow the stack at high rapid iteration counts without a cap.
const maxValueDepth = 6

func propertyMapGenerator(props []*schema.Property) *rapid.Generator[property.Map] {
	if len(props) == 0 {
		return rapid.Just(property.NewMap(nil))
	}
	return rapid.Custom(func(t *rapid.T) property.Map {
		return drawPropertyMap(t, props, maxValueDepth)
	})
}

// drawPropertyMap emits a property.Map for the given property list. Required
// properties are always present; optional properties are sometimes absent,
// sometimes explicitly null, and sometimes carry a typed value. At depth 0
// optional properties are always absent: this bounds otherwise-unbounded
// optional cycles (which the schema does not forbid because a null value
// always terminates them).
func drawPropertyMap(t *rapid.T, props []*schema.Property, depth int) property.Map {
	out := map[string]property.Value{}
	for _, p := range props {
		label := "prop:" + p.Name
		if _, isOpt := p.Type.(*schema.OptionalType); isOpt {
			if depth <= 0 {
				continue
			}
			switch rapid.IntRange(0, 2).Draw(t, label+":mode") {
			case 0:
				continue
			case 1:
				out[p.Name] = property.Value{}
			case 2:
				out[p.Name] = drawValue(t, codegen.UnwrapType(p.Type), label, depth)
			}
		} else {
			out[p.Name] = drawValue(t, codegen.UnwrapType(p.Type), label, depth)
		}
	}
	return property.NewMap(out)
}

// drawValue emits a property.Value matching typ. The schema is required to
// be free of required-recursive object cycles (enforced by
// schema.ValidationOptions.RejectRequiredObjectCycles), but values may still
// nest indefinitely through Array<T>/Map<T> chains; depth caps that.
func drawValue(t *rapid.T, typ schema.Type, label string, depth int) property.Value {
	if tok, ok := typ.(*schema.TokenType); ok {
		if tok.UnderlyingType != nil {
			return drawValue(t, tok.UnderlyingType, label, depth)
		}
		return drawString(t, label)
	}
	typ = codegen.UnwrapType(typ)

	switch typ {
	case schema.BoolType:
		return property.New(rapid.Bool().Draw(t, label+":b"))
	case schema.IntType:
		return property.New(float64(rapid.IntRange(-1000, 1000).Draw(t, label+":i")))
	case schema.NumberType:
		return property.New(rapid.Float64Range(-1e6, 1e6).Draw(t, label+":n"))
	case schema.StringType:
		return drawString(t, label)
	case schema.ArchiveType:
		return property.New(rapidArchive().Draw(t, label))
	case schema.AssetType:
		return property.New(rapidAsset().Draw(t, label))
	case schema.JSONType:
		return drawJSONLikeValue(t, label, depth)
	case schema.AnyType, schema.AnyResourceType:
		return drawAnyValue(t, label, depth)
	}

	switch tt := typ.(type) {
	case *schema.ArrayType:
		if depth <= 0 {
			return property.New(property.Array{})
		}
		n := rapid.IntRange(0, 4).Draw(t, label+":alen")
		elems := make([]property.Value, n)
		for i := 0; i < n; i++ {
			elems[i] = drawValue(t, tt.ElementType, fmt.Sprintf("%s:%d", label, i), depth-1)
		}
		return property.New(elems)
	case *schema.MapType:

		if depth <= 0 {
			return property.New(property.Map{})
		}
		n := rapid.IntRange(0, 4).Draw(t, label+":mlen")
		keys := rapid.SliceOfNDistinct(rapid.String(), n, n, rapid.ID[string]).Draw(t, label+":mk")
		m := map[string]property.Value{}
		for _, k := range keys {
			m[k] = drawValue(t, tt.ElementType, label+":"+k, depth-1)
		}
		return property.New(m)
	case *schema.ObjectType:
		return property.New(drawPropertyMap(t, tt.Properties, depth-1))
	case *schema.UnionType:
		return drawUnionValue(t, tt, label, depth)
	case *schema.EnumType:
		return drawEnumValue(t, tt, label)
	case *schema.ResourceType:
		return drawResourceReferenceValue(t, label)
	}

	panic(fmt.Sprintf("rapidresource: unhandled schema type %T (%v)", typ, typ))
}

func drawString(t *rapid.T, label string) property.Value {
	return property.New(rapid.String().Draw(t, label+":s"))
}

func rapidAsset() *rapid.Generator[*asset.Asset] {
	return rapid.Map(rapid.String(), func(s string) *asset.Asset {
		a, err := asset.FromText(s)
		contract.AssertNoErrorf(err, "failed to hash string")
		return a
	})
}

// archivePathPattern restricts archive entry names to a safe alphabet.
// asset.Archive serializes through tar, which rejects entries whose path is
// a directory marker like "/" or that contain NUL bytes — neither of which
// the importer round-trip needs to exercise.
const archivePathPattern = `^[a-zA-Z0-9_-]{1,16}(/[a-zA-Z0-9_-]{1,16}){0,2}$`

func rapidArchive() *rapid.Generator[*archive.Archive] {
	keys := rapid.StringMatching(archivePathPattern)
	return rapid.Map(rapid.MapOf(keys, rapidAsset()), func(m map[string]*asset.Asset) *archive.Archive {
		assets := make(map[string]any, len(m))
		for k, v := range m {
			assets[k] = v
		}
		a, err := archive.FromAssets(assets)
		contract.AssertNoErrorf(err, "failed to hash asset collection")
		return a
	})
}

// drawJSONLikeValue produces an arbitrary JSON-shaped value: primitives
// (including null), arrays, or maps. Used for schema.JSONType. depth shares
// the same budget as drawValue.
func drawJSONLikeValue(t *rapid.T, label string, depth int) property.Value {
	if depth <= 0 {
		return drawJSONPrimitive(t, label)
	}
	switch rapid.IntRange(0, 5).Draw(t, label+":jk") {
	case 0, 1, 2, 3:
		return drawJSONPrimitive(t, label)
	case 4:
		n := rapid.IntRange(0, 3).Draw(t, label+":jaln")
		elems := make([]property.Value, n)
		for i := 0; i < n; i++ {
			elems[i] = drawJSONLikeValue(t, fmt.Sprintf("%s:%d", label, i), depth-1)
		}
		return property.New(elems)
	case 5:
		n := rapid.IntRange(0, 3).Draw(t, label+":jmn")
		keys := rapid.SliceOfNDistinct(rapid.String(), n, n, rapid.ID[string]).Draw(t, label+":jmk")
		m := map[string]property.Value{}
		for _, k := range keys {
			m[k] = drawJSONLikeValue(t, label+":"+k, depth-1)
		}
		return property.New(m)
	}
	panic("unreachable")
}

func drawJSONPrimitive(t *rapid.T, label string) property.Value {
	switch rapid.IntRange(0, 3).Draw(t, label+":jp") {
	case 0:
		return drawString(t, label)
	case 1:
		return property.New(rapid.Bool().Draw(t, label+":jpb"))
	case 2:
		return property.New(rapid.Float64Range(-1e6, 1e6).Draw(t, label+":jpn"))
	case 3:
		return property.Value{}
	}
	panic("unreachable")
}

// drawAnyValue produces a value of schema.AnyType or AnyResourceType: a JSON
// primitive (including null), or an asset/archive.
func drawAnyValue(t *rapid.T, label string, depth int) property.Value {
	switch rapid.IntRange(0, 5).Draw(t, label+":any") {
	case 0, 1, 2, 3:
		return drawJSONLikeValue(t, label, depth)
	case 4:
		return property.New(rapidAsset().Draw(t, label))
	case 5:
		return property.New(rapidArchive().Draw(t, label))
	}
	panic("unreachable")
}

// drawUnionValue picks a branch and emits a value of that branch's type.
// Discriminators are not specially handled: at the value level a discriminator
// is just a regular property of the chosen object (or absent), and the schema
// generator does not synthesize one into the value.
func drawUnionValue(t *rapid.T, u *schema.UnionType, label string, depth int) property.Value {
	if len(u.ElementTypes) == 0 {
		return property.Value{}
	}
	idx := rapid.IntRange(0, len(u.ElementTypes)-1).Draw(t, label+":uidx")
	return drawValue(t, u.ElementTypes[idx], fmt.Sprintf("%s:u%d", label, idx), depth)
}

// drawEnumValue samples one of the enum's declared values and lifts it into a
// property.Value via property.Any. Bound enum values arrive as bool, int32,
// float64, or string; we widen integers to float64 since property.Value's
// number representation is float64.
func drawEnumValue(t *rapid.T, e *schema.EnumType, label string) property.Value {
	if len(e.Elements) == 0 {
		return drawValue(t, e.ElementType, label+":eunderlying", 0)
	}
	idx := rapid.IntRange(0, len(e.Elements)-1).Draw(t, label+":eidx")
	v, err := property.Any(toPropertyGoValue(e.Elements[idx].Value))
	if err != nil {
		panic(fmt.Sprintf("rapidresource: enum value %v (%[1]T): %v", e.Elements[idx].Value, err))
	}
	return v
}

func toPropertyGoValue(v any) any {
	switch v := v.(type) {
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	}
	return v
}

func drawResourceReferenceValue(t *rapid.T, label string) property.Value {
	return property.New(property.ResourceReference{
		URN: propertytest.URN().Draw(t, label+":urn"),
	})
}
