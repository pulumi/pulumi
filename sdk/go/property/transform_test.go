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

package property

import (
	"math"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransform(t *testing.T) {
	t.Parallel()

	leaf := New("before").WithSecret(true).WithDependencies([]urn.URN{"leaf"})
	original := New([]Value{New(map[string]Value{"key": leaf})}).
		WithSecret(true).WithDependencies([]urn.URN{"root"})
	var visited []string
	got := original.Transform(func(v Value) Value {
		switch {
		case v.IsString():
			visited = append(visited, "string")
			return WithGoValue(v, "after")
		case v.IsMap():
			visited = append(visited, "map")
			assert.Equal(t, "after", v.AsMap().Get("key").AsString())
		case v.IsArray():
			visited = append(visited, "array")
			assert.Equal(t, "after", v.AsArray().Get(0).AsMap().Get("key").AsString())
		}
		return v
	})
	assert.Equal(t, []string{"string", "map", "array"}, visited)
	assert.Equal(t, "before", original.AsArray().Get(0).AsMap().Get("key").AsString())
	assert.Equal(t, original.SecretID(), got.SecretID())
	assert.Equal(t, original.Dependencies(), got.Dependencies())
	child := got.AsArray().Get(0).AsMap().Get("key")
	assert.Equal(t, leaf.SecretID(), child.SecretID())
	assert.Equal(t, leaf.Dependencies(), child.Dependencies())
}

//nolint:paralleltest // AllocsPerRun cannot run in parallel with other tests.
func TestTransformSharing(t *testing.T) {
	unchanged := New(map[string]Value{"key": New("unchanged")})
	v := New([]Value{unchanged, New("change"), unchanged, New("change")})
	result := v.Transform(func(v Value) Value {
		if v.IsString() && v.AsString() == "change" {
			return New("changed")
		}
		return v
	})
	assert.True(t, unchanged.transformIdentical(result.AsArray().Get(0)))
	assert.True(t, unchanged.transformIdentical(result.AsArray().Get(2)))
	assert.Equal(t, "changed", result.AsArray().Get(1).AsString())
	assert.Equal(t, "changed", result.AsArray().Get(3).AsString())
	assert.Equal(t, "change", v.AsArray().Get(1).AsString())
	assert.Equal(t, "change", v.AsArray().Get(3).AsString())

	identity := func(v Value) Value { return v }
	assert.True(t, v.transformIdentical(v.Transform(identity)))
	assert.Zero(t, testing.AllocsPerRun(100, func() { v.Transform(identity) }))
}

func TestTransformLeaves(t *testing.T) {
	t.Parallel()

	for _, v := range []Value{
		New(Null), New(Computed), New(true), New(1.0), New("text"),
		New(math.NaN()), New(math.Copysign(0, -1)),
		New(&asset.Asset{}), New(&archive.Archive{}),
		New(ResourceReference{ID: New("id")}),
		New(Array{}), New(Map{}),
	} {
		calls := 0
		result := v.Transform(func(value Value) Value {
			calls++
			assert.True(t, v.transformIdentical(value))
			return New([]Value{New("replacement")})
		})
		assert.Equal(t, 1, calls)
		require.True(t, result.IsArray())
		assert.Equal(t, "replacement", result.AsArray().Get(0).AsString())
	}
}

func TestTransformEqualReplacement(t *testing.T) {
	t.Parallel()

	before := New("secret").WithSecret(true)
	after := New("secret").WithSecret(true)
	v := New(map[string]Value{"key": before})
	got := v.Transform(func(v Value) Value {
		if v.IsString() {
			return after
		}
		return v
	})
	assert.Equal(t, after.SecretID(), got.AsMap().Get("key").SecretID())
	assert.Equal(t, before.SecretID(), v.AsMap().Get("key").SecretID())
}
