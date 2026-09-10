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

package codegen_test

import (
	"fmt"
	"slices"
	"testing"

	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils/rapidresource"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils/rapidschema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

// TestWireDiscriminatableUnionsAttributeUniquely is the soundness property for
// codegen.IsWireDiscriminatableUnionType: whenever the function reports a
// union as wire discriminatable, a fully-known value drawn from any member is
// matched by exactly that member. Unions are drawn from a deliberately tiny
// alphabet so members frequently share wire shapes, literals, and property
// names, making the discriminatable cases non-trivial. The result must also
// be invariant under member order.
func TestWireDiscriminatableUnionsAttributeUniquely(t *testing.T) {
	t.Parallel()

	discriminatable := 0
	rapid.Check(t, func(t *rapid.T) {
		u := drawCollisionUnion(t)

		permuted := &schema.UnionType{
			ElementTypes: rapid.Permutation(slices.Clone(u.ElementTypes)).Draw(t, "perm"),
		}
		require.Equal(t,
			codegen.IsWireDiscriminatableUnionType(u),
			codegen.IsWireDiscriminatableUnionType(permuted),
			"member order changed the result for %v", u)

		if !codegen.IsWireDiscriminatableUnionType(u) {
			return
		}
		discriminatable++
		requireUniqueAttribution(t, u)
	})
	require.Positive(t, discriminatable, "no discriminatable unions were generated; the property was vacuous")
}

// TestWireDiscriminatableUnionsFromRapidSchema runs the same soundness
// property over every union appearing in packages drawn by
// rapidschema.Package(), covering shapes the collision generator does not
// produce: nested unions, builtin refs, and deep object graphs.
func TestWireDiscriminatableUnionsFromRapidSchema(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		pkg := rapidschema.Package().Draw(t, "pkg")
		var unions []*schema.UnionType
		collect := func(typ schema.Type) {
			if u, ok := typ.(*schema.UnionType); ok {
				unions = append(unions, u)
			}
		}
		for _, r := range pkg.Resources {
			codegen.VisitTypeClosure(r.Properties, collect)
			codegen.VisitTypeClosure(r.InputProperties, collect)
			if r.StateInputs != nil {
				codegen.VisitTypeClosure(r.StateInputs.Properties, collect)
			}
		}

		for _, u := range unions {
			if codegen.IsWireDiscriminatableUnionType(u) {
				requireUniqueAttribution(t, u)
			}
		}
	})
}

// requireUniqueAttribution draws a value from one member of u and asserts that
// exactly that member matches it.
func requireUniqueAttribution(t *rapid.T, u *schema.UnionType) {
	idx := rapid.IntRange(0, len(u.ElementTypes)-1).Draw(t, "member")
	member := u.ElementTypes[idx]
	v := rapidresource.Value(member).Draw(t, "value")

	require.True(t, wireMatches(v, member),
		"value %v does not match the member %v it was drawn from", v, member)
	matched := 0
	for _, e := range u.ElementTypes {
		if wireMatches(v, e) {
			matched++
		}
	}
	require.Equal(t, 1, matched, "value %v of member %v matched %d members of %v", v, member, matched, u)
}

// wireMatches reports whether the fully-known wire value v belongs to typ. It
// is the ground-truth definition IsWireDiscriminatableUnionType is checked
// against, under the same closed-object reading: a value of an object type
// carries only that object's declared properties.
func wireMatches(v property.Value, typ schema.Type) bool {
	switch typ {
	case schema.BoolType:
		return v.IsBool()
	case schema.IntType, schema.NumberType:
		return v.IsNumber()
	case schema.StringType:
		return v.IsString()
	case schema.AssetType:
		return v.IsAsset()
	case schema.ArchiveType:
		return v.IsArchive()
	case schema.JSONType, schema.AnyType, schema.AnyResourceType:
		return true
	}

	switch typ := typ.(type) {
	case *schema.OptionalType:
		return v.IsNull() || wireMatches(v, typ.ElementType)
	case *schema.InputType:
		return wireMatches(v, typ.ElementType)
	case *schema.TokenType:
		if typ.UnderlyingType != nil {
			return wireMatches(v, typ.UnderlyingType)
		}
		return v.IsString()
	case *schema.UnionType:
		for _, e := range typ.ElementTypes {
			if wireMatches(v, e) {
				return true
			}
		}
		return false
	case *schema.EnumType:
		if len(typ.Elements) == 0 {
			return wireMatches(v, typ.ElementType)
		}
		for _, e := range typ.Elements {
			if v.Equals(liftConstant(e.Value)) {
				return true
			}
		}
		return false
	case *schema.ArrayType:
		if !v.IsArray() {
			return false
		}
		for _, e := range v.AsArray().AsSlice() {
			if !wireMatches(e, typ.ElementType) {
				return false
			}
		}
		return true
	case *schema.MapType:
		if !v.IsMap() {
			return false
		}
		ok := true
		v.AsMap().All(func(_ string, e property.Value) bool {
			ok = wireMatches(e, typ.ElementType)
			return ok
		})
		return ok
	case *schema.ObjectType:
		return objectWireMatches(v, typ)
	case *schema.ResourceType:
		return v.IsResourceReference() && v.AsResourceReference().URN.Type() == tokens.Type(typ.Token)
	}
	return false
}

func objectWireMatches(v property.Value, typ *schema.ObjectType) bool {
	if !v.IsMap() {
		return false
	}
	m := v.AsMap()
	ok := true
	m.All(func(name string, e property.Value) bool {
		p, declared := typ.Property(name)
		switch {
		case !declared:
			ok = false
		case p.ConstValue != nil:
			// An optional constant property may also be explicitly null.
			ok = e.Equals(liftConstant(p.ConstValue)) || (!p.IsRequired() && e.IsNull())
		default:
			ok = wireMatches(e, p.Type)
		}
		return ok
	})
	if !ok {
		return false
	}
	for _, p := range typ.Properties {
		if p.IsRequired() {
			if _, has := m.GetOk(p.Name); !has {
				return false
			}
		}
	}
	return true
}

// liftConstant lifts a bound schema constant or enum value into its wire
// value: all numbers arrive as float64.
func liftConstant(v any) property.Value {
	switch v := v.(type) {
	case bool:
		return property.New(v)
	case int:
		return property.New(float64(v))
	case int32:
		return property.New(float64(v))
	case int64:
		return property.New(float64(v))
	case float64:
		return property.New(v)
	case string:
		return property.New(v)
	}
	panic(fmt.Sprintf("unexpected constant %v (%[1]T)", v))
}

// Tiny alphabets: three property names, three string literals, three integer
// literals. Small domains make members collide often, so the discriminatable
// unions the test exercises are the barely-discriminatable ones.
var (
	collisionPropertyNames = []string{"a", "b", "c"}
	collisionStrings       = []string{"x", "y", "z"}
	collisionInts          = []int32{1, 2, 3}
)

func drawCollisionUnion(t *rapid.T) *schema.UnionType {
	n := rapid.IntRange(2, 4).Draw(t, "members")
	members := make([]schema.Type, n)
	for i := range n {
		members[i] = drawCollisionMember(t, fmt.Sprintf("m%d", i))
	}
	return &schema.UnionType{ElementTypes: members}
}

var collisionMemberKinds = []string{
	"bool", "int", "number", "string",
	"stringEnum", "intEnum",
	"optional", "array", "map",
	"asset", "archive", "resource", "any",
	// Objects are the interesting case; weight them heavily.
	"object", "object", "object", "object", "object",
}

func drawCollisionMember(t *rapid.T, label string) schema.Type {
	switch kind := rapid.SampledFrom(collisionMemberKinds).Draw(t, label+":kind"); kind {
	case "bool":
		return schema.BoolType
	case "int":
		return schema.IntType
	case "number":
		return schema.NumberType
	case "string":
		return schema.StringType
	case "stringEnum":
		return drawCollisionEnum(t, label, schema.StringType)
	case "intEnum":
		return drawCollisionEnum(t, label, schema.IntType)
	case "optional":
		return &schema.OptionalType{ElementType: drawCollisionPrimitive(t, label)}
	case "array":
		return &schema.ArrayType{ElementType: drawCollisionPrimitive(t, label)}
	case "map":
		return &schema.MapType{ElementType: drawCollisionPrimitive(t, label)}
	case "asset":
		return schema.AssetType
	case "archive":
		return schema.ArchiveType
	case "resource":
		return &schema.ResourceType{
			Token: rapid.SampledFrom([]string{"pkg:index:A", "pkg:index:B"}).Draw(t, label+":token"),
		}
	case "any":
		return schema.AnyType
	case "object":
		return drawCollisionObjectDepth(t, label, 1)
	default:
		panic("unknown member kind " + kind)
	}
}

func drawCollisionPrimitive(t *rapid.T, label string) schema.Type {
	return rapid.SampledFrom([]schema.Type{
		schema.BoolType, schema.IntType, schema.NumberType, schema.StringType,
	}).Draw(t, label+":prim")
}

func drawCollisionEnum(t *rapid.T, label string, element schema.Type) *schema.EnumType {
	e := &schema.EnumType{ElementType: element}
	n := rapid.IntRange(1, 2).Draw(t, label+":nvalues")
	switch element {
	case schema.StringType:
		values := rapid.SliceOfNDistinct(rapid.SampledFrom(collisionStrings), n, n, rapid.ID[string]).
			Draw(t, label+":strings")
		for _, s := range values {
			e.Elements = append(e.Elements, &schema.Enum{Value: s})
		}
	case schema.IntType:
		values := rapid.SliceOfNDistinct(rapid.SampledFrom(collisionInts), n, n, rapid.ID[int32]).
			Draw(t, label+":ints")
		for _, i := range values {
			e.Elements = append(e.Elements, &schema.Enum{Value: i})
		}
	}
	return e
}

func drawCollisionObjectDepth(t *rapid.T, label string, depth int) *schema.ObjectType {
	n := rapid.IntRange(1, 3).Draw(t, label+":nprops")
	names := rapid.SliceOfNDistinct(rapid.SampledFrom(collisionPropertyNames), n, n, rapid.ID[string]).
		Draw(t, label+":names")
	kinds := []string{"string", "int", "stringConst", "intConst", "enum", "enumUnion"}
	if depth > 0 {
		kinds = append(kinds, "object")
	}
	props := make([]*schema.Property, n)
	for i, name := range names {
		p := &schema.Property{Name: name}
		propLabel := label + ":" + name
		switch rapid.SampledFrom(kinds).Draw(t, propLabel+":kind") {
		case "string":
			p.Type = schema.StringType
		case "int":
			p.Type = schema.IntType
		case "stringConst":
			p.Type = schema.StringType
			p.ConstValue = rapid.SampledFrom(collisionStrings).Draw(t, propLabel+":const")
		case "intConst":
			p.Type = schema.IntType
			p.ConstValue = rapid.SampledFrom(collisionInts).Draw(t, propLabel+":const")
		case "enum":
			p.Type = drawCollisionEnum(t, propLabel, schema.StringType)
		case "enumUnion":
			p.Type = &schema.UnionType{ElementTypes: []schema.Type{
				drawCollisionEnum(t, propLabel+":u0", schema.StringType),
				drawCollisionEnum(t, propLabel+":u1", schema.StringType),
			}}
		case "object":
			p.Type = drawCollisionObjectDepth(t, propLabel, depth-1)
		}
		if rapid.Bool().Draw(t, propLabel+":optional") {
			p.Type = &schema.OptionalType{ElementType: p.Type}
		}
		props[i] = p
	}
	return &schema.ObjectType{Token: "pkg:index:T", Properties: props}
}
