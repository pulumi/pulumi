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

package do

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWireMatchesResourceType asserts that a *schema.ResourceType union member only matches an actual resource
// reference value, rather than falling through to wireMatches's permissive default for annotations it doesn't
// recognize. Without this, a plain object value could spuriously "match" a resource-typed variant and cause
// wireDiscriminatedVariant to resolve to the wrong member of a union that also has an object-shaped variant.
func TestWireMatchesResourceType(t *testing.T) {
	t.Parallel()

	resourceType := &schema.ResourceType{Token: "azure:index:Widget"}

	ref := resource.NewProperty(resource.ResourceReference{URN: "urn:pulumi:stack::project::azure:index:Widget::name"})
	assert.True(t, wireMatches(ref, resourceType, false))

	obj := resource.NewProperty(resource.PropertyMap{
		"name": resource.NewProperty("not-a-resource-reference"),
	})
	assert.False(t, wireMatches(obj, resourceType, false))
}

// TestConstValueMatchesIntegerConstant asserts that constValueMatches matches an integer schema constant against a
// numeric property value regardless of which Go integer/float type the constant happens to be stored as. Schema
// binding stores integer constants as int32 (see bindConstValue in pkg/codegen/schema/bind.go), so a constValueMatches
// that only handled `int` would silently never match a real schema's integer discriminator.
func TestConstValueMatchesIntegerConstant(t *testing.T) {
	t.Parallel()

	prop := resource.NewProperty(2.0)
	assert.True(t, constValueMatches(prop, int32(2)))
	assert.True(t, constValueMatches(prop, int64(2)))
	assert.True(t, constValueMatches(prop, 2))
	assert.True(t, constValueMatches(prop, float32(2)))
	assert.True(t, constValueMatches(prop, float64(2)))
	assert.False(t, constValueMatches(prop, int32(3)))
}

// TestWireDiscriminatedVariantClosedThenOpen asserts that wireDiscriminatedVariant resolves every value of a union
// codegen.IsWireDiscriminatableUnionType accepts when the value carries only declared properties, even if it also
// satisfies another variant's required properties, and that it still resolves a value carrying an undeclared key
// when only one variant's required properties are present.
func TestWireDiscriminatedVariantClosedThenOpen(t *testing.T) {
	t.Parallel()

	short := &schema.ObjectType{
		Token: "pkg:index:Short",
		Properties: []*schema.Property{
			{Name: "x", Type: schema.StringType},
		},
	}
	long := &schema.ObjectType{
		Token: "pkg:index:Long",
		Properties: []*schema.Property{
			{Name: "x", Type: schema.StringType},
			{Name: "y", Type: schema.StringType},
		},
	}
	union := &schema.UnionType{ElementTypes: []schema.Type{short, long}}
	require.True(t, codegen.IsWireDiscriminatableUnionType(union))

	obj := func(keys ...string) resource.PropertyValue {
		m := resource.PropertyMap{}
		for _, k := range keys {
			m[resource.PropertyKey(k)] = resource.NewProperty("v")
		}
		return resource.NewProperty(m)
	}

	// Satisfies both variants' required properties, but only Long declares y.
	assert.Equal(t, long, wireDiscriminatedVariant(obj("x", "y"), union))
	assert.Equal(t, short, wireDiscriminatedVariant(obj("x"), union))
	// An undeclared key rules out both variants under the closed reading; only Short's required properties are all
	// present, so the open reading resolves it.
	assert.Equal(t, short, wireDiscriminatedVariant(obj("x", "extra"), union))
	// Under the open reading both variants match, so neither is trusted.
	assert.Nil(t, wireDiscriminatedVariant(obj("x", "y", "extra"), union))
}
