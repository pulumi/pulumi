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

package codegen

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvingPackageReferences(t *testing.T) {
	t.Parallel()

	testdataPath := filepath.Join("testing", "test", "testdata")
	loader := schema.NewPluginLoader(utils.NewContext(testdataPath))
	var pkgSpec schema.PackageSpec
	require.NoError(t, json.Unmarshal(utils.ReadSchema(t, "remoteref", "1.0.0"), &pkgSpec))
	pkg, diags, err := schema.BindSpec(pkgSpec, loader, schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	require.NotNil(t, pkg)
	require.NoError(t, err)
	require.Empty(t, diags)
	// ensure that package references return goalias because remoteref depends on goalias
	references := PackageReferences(pkg)
	require.Len(t, references, 1)
	assert.Equal(t, "goalias", references[0].Name())
}

func TestStringSetContains(t *testing.T) {
	t.Parallel()

	set123 := NewStringSet("1", "2", "3")
	set12 := NewStringSet("1", "2")
	set14 := NewStringSet("1", "4")
	setEmpty := NewStringSet()

	assert.True(t, set123.Contains(set123))
	assert.True(t, set123.Contains(set12))
	assert.False(t, set12.Contains(set123))
	assert.False(t, set123.Contains(set14))
	assert.True(t, set123.Contains(setEmpty))
}

func TestIsWireDiscriminatableUnionType(t *testing.T) {
	t.Parallel()

	stringEnum := func(values ...string) *schema.EnumType {
		e := &schema.EnumType{ElementType: schema.StringType}
		for _, v := range values {
			e.Elements = append(e.Elements, &schema.Enum{Value: v})
		}
		return e
	}
	object := func(props ...*schema.Property) *schema.ObjectType {
		return &schema.ObjectType{Properties: props}
	}
	constProp := func(name string, value any) *schema.Property {
		return &schema.Property{Name: name, Type: schema.StringType, ConstValue: value}
	}

	// nested builds {foo: {fizz: <fizzType>}} with both foo and fizz required.
	nested := func(fizzType schema.Type) *schema.ObjectType {
		return object(&schema.Property{
			Name: "foo",
			Type: object(&schema.Property{Name: "fizz", Type: fizzType}),
		})
	}

	recursiveA := &schema.ObjectType{}
	recursiveB := &schema.ObjectType{}
	recursiveA.Properties = []*schema.Property{{Name: "foo", Type: recursiveB}}
	recursiveB.Properties = []*schema.Property{{Name: "foo", Type: recursiveA}}

	cases := []struct {
		name     string
		union    *schema.UnionType
		expected bool
	}{
		{
			name:     "single member",
			union:    &schema.UnionType{ElementTypes: []schema.Type{schema.StringType}},
			expected: true,
		},
		{
			name:     "distinct primitive shapes",
			union:    &schema.UnionType{ElementTypes: []schema.Type{schema.StringType, schema.IntType, schema.BoolType}},
			expected: true,
		},
		{
			name:     "int and number share a wire shape",
			union:    &schema.UnionType{ElementTypes: []schema.Type{schema.IntType, schema.NumberType}},
			expected: false,
		},
		{
			name:     "literal and unbounded string",
			union:    &schema.UnionType{ElementTypes: []schema.Type{stringEnum("foo"), schema.StringType}},
			expected: false,
		},
		{
			name:     "disjoint string literals and int",
			union:    &schema.UnionType{ElementTypes: []schema.Type{stringEnum("foo"), stringEnum("bar"), schema.IntType}},
			expected: true,
		},
		{
			name:     "overlapping string literals",
			union:    &schema.UnionType{ElementTypes: []schema.Type{stringEnum("foo", "bar"), stringEnum("bar", "baz")}},
			expected: false,
		},
		{
			name: "int and float literals normalize to one wire number",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				&schema.EnumType{ElementType: schema.IntType, Elements: []*schema.Enum{{Value: 1}}},
				&schema.EnumType{ElementType: schema.NumberType, Elements: []*schema.Enum{{Value: float64(1)}}},
			}},
			expected: false,
		},
		{
			name: "objects with disjoint required constants",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				object(constProp("kind", "a"), &schema.Property{Name: "value", Type: schema.StringType}),
				object(constProp("kind", "b"), &schema.Property{Name: "value", Type: schema.StringType}),
			}},
			expected: true,
		},
		{
			name: "objects with a constant on only one side",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				object(constProp("kind", "a")),
				object(&schema.Property{Name: "kind", Type: schema.StringType}),
			}},
			expected: false,
		},
		{
			name: "constant disjoint from a union of enums",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				object(constProp("kind", "x")),
				object(&schema.Property{Name: "kind", Type: &schema.UnionType{
					ElementTypes: []schema.Type{stringEnum("y"), stringEnum("z")},
				}}),
			}},
			expected: true,
		},
		{
			name: "constant intersecting a union of enums",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				object(constProp("kind", "x")),
				object(&schema.Property{Name: "kind", Type: &schema.UnionType{
					ElementTypes: []schema.Type{stringEnum("x", "y"), stringEnum("z")},
				}}),
			}},
			expected: false,
		},
		{
			name: "objects with disjoint enum discriminators",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				object(&schema.Property{Name: "kind", Type: stringEnum("a", "b")}),
				object(&schema.Property{Name: "kind", Type: stringEnum("c")}),
			}},
			expected: true,
		},
		{
			name: "object requires a property the other lacks",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				object(&schema.Property{Name: "a", Type: schema.StringType}),
				object(&schema.Property{Name: "b", Type: schema.StringType}),
			}},
			expected: true,
		},
		{
			name:     "objects distinguished by nested required objects",
			union:    &schema.UnionType{ElementTypes: []schema.Type{nested(schema.IntType), nested(schema.BoolType)}},
			expected: true,
		},
		{
			name: "nested discriminating property is optional",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				object(&schema.Property{
					Name: "foo",
					Type: object(&schema.Property{Name: "fizz", Type: &schema.OptionalType{ElementType: schema.IntType}}),
				}),
				object(&schema.Property{
					Name: "foo",
					Type: object(&schema.Property{Name: "fizz", Type: &schema.OptionalType{ElementType: schema.BoolType}}),
				}),
			}},
			expected: false,
		},
		{
			name: "nested object is optional",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				object(&schema.Property{Name: "foo", Type: &schema.OptionalType{
					ElementType: object(&schema.Property{Name: "fizz", Type: schema.IntType}),
				}}),
				object(&schema.Property{Name: "foo", Type: &schema.OptionalType{
					ElementType: object(&schema.Property{Name: "fizz", Type: schema.BoolType}),
				}}),
			}},
			expected: false,
		},
		{
			// Required-recursive objects admit no finite value at all, so no value is ambiguous between them.
			name:     "required-recursive objects are uninhabited and vacuously disjoint",
			union:    &schema.UnionType{ElementTypes: []schema.Type{recursiveA, recursiveB}},
			expected: true,
		},
		{
			name: "nested discriminating property required on one side",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				object(&schema.Property{Name: "fizz", Type: schema.IntType}),
				object(&schema.Property{Name: "fizz", Type: &schema.OptionalType{ElementType: schema.BoolType}}),
			}},
			expected: true,
		},
		{
			name: "map and object collide",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				&schema.MapType{ElementType: schema.StringType},
				object(&schema.Property{Name: "a", Type: schema.StringType}),
			}},
			expected: false,
		},
		{
			name: "map disjoint from object by required property type",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				&schema.MapType{ElementType: schema.StringType},
				object(&schema.Property{Name: "a", Type: schema.IntType}),
			}},
			expected: true,
		},
		{
			name: "two arrays collide",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				&schema.ArrayType{ElementType: schema.StringType},
				&schema.ArrayType{ElementType: schema.IntType},
			}},
			expected: false,
		},
		{
			name:     "asset and archive are distinct",
			union:    &schema.UnionType{ElementTypes: []schema.Type{schema.AssetType, schema.ArchiveType, schema.StringType}},
			expected: true,
		},
		{
			name: "resource references of distinct types",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				&schema.ResourceType{Token: "pkg:index:A"},
				&schema.ResourceType{Token: "pkg:index:B"},
			}},
			expected: true,
		},
		{
			name: "resource references of one type collide",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				&schema.ResourceType{Token: "pkg:index:A"},
				&schema.ResourceType{Token: "pkg:index:A"},
			}},
			expected: false,
		},
		{
			name:     "any admits every shape",
			union:    &schema.UnionType{ElementTypes: []schema.Type{schema.AnyType, schema.StringType}},
			expected: false,
		},
		{
			name: "two optional members collide on null",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				&schema.OptionalType{ElementType: schema.StringType},
				&schema.OptionalType{ElementType: schema.IntType},
			}},
			expected: false,
		},
		{
			name: "one optional member",
			union: &schema.UnionType{ElementTypes: []schema.Type{
				&schema.OptionalType{ElementType: schema.StringType},
				schema.IntType,
			}},
			expected: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.expected, IsWireDiscriminatableUnionType(c.union))
		})
	}
}

func TestStringSetSubtract(t *testing.T) {
	t.Parallel()

	set1234 := NewStringSet("1", "2", "3", "4")
	set125 := NewStringSet("1", "2", "5")
	set34 := NewStringSet("3", "4")
	setEmpty := NewStringSet()

	assert.Equal(t, set34, set1234.Subtract(set125))
	assert.Equal(t, setEmpty, set1234.Subtract(set1234))
	assert.Equal(t, set1234, set1234.Subtract(setEmpty))
}
