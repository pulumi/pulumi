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

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

// TestWireMatchesResourceType asserts that a *schema.ResourceType union member only matches an actual resource
// reference value, rather than falling through to wireMatches's permissive default for annotations it doesn't
// recognize. Without this, a plain object value could spuriously "match" a resource-typed variant and cause
// wireDiscriminatedVariant to resolve to the wrong member of a union that also has an object-shaped variant.
func TestWireMatchesResourceType(t *testing.T) {
	t.Parallel()

	resourceType := &schema.ResourceType{Token: "azure:index:Widget"}

	ref := resource.NewProperty(resource.ResourceReference{URN: "urn:pulumi:stack::project::azure:index:Widget::name"})
	assert.True(t, wireMatches(ref, resourceType))

	obj := resource.NewProperty(resource.PropertyMap{
		"name": resource.NewProperty("not-a-resource-reference"),
	})
	assert.False(t, wireMatches(obj, resourceType))
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
