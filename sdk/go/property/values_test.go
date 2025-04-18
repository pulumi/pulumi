// Copyright 2016-2025, Pulumi Corporation.
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
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
)

// Calling == does not implement desirable behavior, so we ensure that it is invalid.
func TestCannotCompareValues(t *testing.T) {
	t.Parallel()
	assert.False(t, reflect.TypeOf(Value{}).Comparable())
}

func TestNullEquivalence(t *testing.T) {
	t.Parallel()
	assert.Nil(t, New(Null).v)

	assert.True(t, New(Null).Equals(Value{}))

	assert.True(t, New(Null).IsNull())
}

// This test locks in the nil vs null behavior.
//
// In go, nil is a value but not a type. For example:
//
//	(any)([]A(nil)) != nil
//
// This is because when comparing values typed as `any` still carry their underlying type
// information. This is true even when:
//
//	[]A(nil) == nil
//
// Unlike the Go language, Value's Null is not typed. This means that:
//
//	New[Array](nil).Equals(New(Null))
//
// Further, since Null is it's own type, it is the case that:
//
//	New[Array](nil).IsArray() == false
//
//	New[Map](nil).IsNull() == true
func TestNil(t *testing.T) {
	t.Parallel()

	var nullValue Value

	// []T based type, zero value is Array(nil)
	t.Run("array", func(t *testing.T) {
		t.Parallel()
		nilArray := New[[]Value](nil)

		assert.False(t, nilArray.IsArray())
		assert.True(t, nilArray.IsNull())
		assert.True(t, nilArray.Equals(nullValue))

		assert.True(t, New(Array{}).IsArray())
	})

	t.Run("map", func(t *testing.T) {
		t.Parallel()
		nilMap := New[map[string]Value](nil)

		assert.False(t, nilMap.IsMap())
		assert.True(t, nilMap.IsNull())
		assert.True(t, nilMap.Equals(nullValue))

		assert.True(t, New(Map{}).IsMap())
	})

	// *T based type, zero value is *resource.Asset(nil)
	t.Run("asset", func(t *testing.T) {
		t.Parallel()
		nilAsset := New[Asset](nil)

		assert.False(t, nilAsset.IsAsset())
		assert.True(t, nilAsset.IsNull())
		assert.True(t, nilAsset.Equals(nullValue))

		a, err := asset.FromText("")
		require.NoError(t, err)
		assert.True(t, New(a).IsAsset())
	})

	// string based type, zero value is ""
	t.Run("string", func(t *testing.T) {
		t.Parallel()
		emptyString := New("")

		assert.True(t, emptyString.IsString())
		assert.False(t, emptyString.IsNull())
		assert.False(t, emptyString.Equals(nullValue))
	})
}

func TestAny(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input       any
		expected    Value
		expectedErr string
	}{
		{
			input:    true,
			expected: New(true),
		},
		{
			input:    3.14,
			expected: New(3.14),
		},
		{
			input:    "example",
			expected: New("example"),
		},
		{
			input:    []Value{New(1.0), New("two")},
			expected: New([]Value{New(1.0), New("two")}),
		},
		{
			input:    NewArray([]Value{New(1.0), New("two")}),
			expected: New([]Value{New(1.0), New("two")}),
		},
		{
			input:    map[string]Value{"key": New("value")},
			expected: New(map[string]Value{"key": New("value")}),
		},
		{
			input:    NewMap(map[string]Value{"key": New("value")}),
			expected: New(map[string]Value{"key": New("value")}),
		},
		{
			input:    &asset.Asset{Text: "example"},
			expected: New(&asset.Asset{Text: "example"}),
		},
		{
			input:    &archive.Archive{},
			expected: New(&archive.Archive{}),
		},
		{
			input:    Computed,
			expected: New(Computed),
		},
		{
			input:    Null,
			expected: Value{}, // or New(Null)
		},
		{
			input:    ResourceReference{ID: New("123")},
			expected: New(ResourceReference{ID: New("123")}),
		},
		{
			input:       struct{ A string }{"A"},
			expectedErr: "invalid type: {A} of type struct { A string }",
		},
	}

	for _, tt := range testCases {
		t.Run(fmt.Sprintf("%T", tt.input), func(t *testing.T) {
			t.Parallel()
			result, err := Any(tt.input)
			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAsset(t *testing.T) {
	t.Parallel()

	originalText := "original text"
	a, err := asset.FromText(originalText)
	assert.NoError(t, err)

	v := New(a)

	assert.True(t, v.IsAsset())
	assert.Equal(t, a, v.AsAsset())

	a.Text = "other text"

	assert.NotEqual(t, a, v.AsAsset())
	assert.Equal(t, originalText, v.AsAsset().Text)

	v.AsAsset().Text = "other test"
	assert.Equal(t, originalText, v.AsAsset().Text)
}

func TestArchive(t *testing.T) {
	t.Parallel()

	mkArchive := func(t *testing.T) Archive {
		archive, err := archive.FromAssets(map[string]any{
			"f1": must(asset.FromText("some text")),
			"f2": must(asset.FromText("more text")),
			"d1": must(archive.FromAssets(map[string]any{
				"f3": must(asset.FromText("nested text")),
			})),
		})
		assert.NoError(t, err)
		return archive
	}

	// Create an initial archive from path
	archive := mkArchive(t)
	v := New(archive)

	assert.True(t, v.IsArchive())
	assert.Equal(t, archive, v.AsArchive())

	// Attempt to mutate the original archive
	archive.Assets["f1"] = must(asset.FromText("different text"))
	archive.Assets["d1"].(Archive).Assets["f3"] = must(asset.FromText("different text"))
	assert.Equal(t, mkArchive(t), v.AsArchive())

	// Attempt to mutate the returned archive
	archive = v.AsArchive()
	archive.Assets["f1"] = must(asset.FromText("different text"))
	archive.Assets["d1"].(Archive).Assets["f3"] = must(asset.FromText("different text"))
	assert.Equal(t, mkArchive(t), v.AsArchive())
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestWithGoValue(t *testing.T) {
	t.Parallel()

	t.Run("set-computed", func(t *testing.T) {
		t.Parallel()
		initialValue := New("initial")
		newValue := WithGoValue(initialValue, Computed)
		expectedValue := New(Computed)
		assert.Equal(t, expectedValue, newValue)
	})

	t.Run("flag-secret", func(t *testing.T) {
		t.Parallel()
		initialValue := New("initial").WithSecret(true)
		newGoValue := "newSecretValue"
		newValue := WithGoValue(initialValue, newGoValue)
		expectedValue := New(newGoValue).WithSecret(true)
		assert.Equal(t, expectedValue, newValue)
	})

	t.Run("set-null", func(t *testing.T) {
		t.Parallel()
		initialValue := New("initial")
		newValue := WithGoValue(initialValue, Null)
		expectedValue := New(Null)
		assert.Equal(t, expectedValue, newValue)
	})

	t.Run("set", func(t *testing.T) {
		t.Parallel()
		initialValue := New("initial")
		newGoValue := "newValue"
		newValue := WithGoValue(initialValue, newGoValue)
		expectedValue := New(newGoValue)
		assert.Equal(t, expectedValue, newValue)
	})
}

func TestResourceReference(t *testing.T) {
	t.Parallel()

	mkRef := func() ResourceReference {
		return ResourceReference{
			URN: "some-urn",
			ID:  New(Computed),
		}
	}

	v := New(mkRef())

	assert.True(t, v.IsResourceReference())
	assert.Equal(t, mkRef(), v.AsResourceReference())
}

func TestHasSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		v         Value
		hasSecret bool
	}{
		{
			name:      "plain value is not secret",
			v:         New("noSecret"),
			hasSecret: false,
		},
		{
			name:      "plain value is secret",
			v:         New("secret").WithSecret(true),
			hasSecret: true,
		},
		{
			name:      "array contains secret",
			v:         New([]Value{New("secret").WithSecret(true)}),
			hasSecret: true,
		},
		{
			name:      "array is secret",
			v:         New([]Value{New("secret")}).WithSecret(true),
			hasSecret: true,
		},
		{
			name:      "map contains a secret",
			v:         New(map[string]Value{"key": New("secret").WithSecret(true)}),
			hasSecret: true,
		},
		{
			name:      "map is a secret",
			v:         New(map[string]Value{"key": New("secret")}).WithSecret(true),
			hasSecret: true,
		},
		{
			name:      "ComputedNoSecret",
			v:         New(Computed),
			hasSecret: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.hasSecret, tt.v.HasSecrets())
		})
	}
}

func TestHasComputed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		v           Value
		hasComputed bool
	}{
		{
			name:        "plain value is not computed",
			v:           New("noComputed"),
			hasComputed: false,
		},
		{
			name:        "is computed",
			v:           New(Computed),
			hasComputed: true,
		},
		{
			name:        "array contains computed",
			v:           New([]Value{New(Computed)}),
			hasComputed: true,
		},
		{
			name:        "map contains a computed",
			v:           New(map[string]Value{"key": New(Computed)}),
			hasComputed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.hasComputed, tt.v.HasComputed())
		})
	}
}

func TestWithDependencies(t *testing.T) {
	t.Parallel()

	t.Run("empty-is-nil", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, New("1"), New("1").WithDependencies(nil))
		assert.Equal(t, New("1"), New("1").WithDependencies([]urn.URN{}))

		assert.Nil(t, New("1").Dependencies())
		assert.Nil(t, New("1").WithDependencies(nil).Dependencies())
		assert.Nil(t, New("1").WithDependencies([]urn.URN{}).Dependencies())
	})

	t.Run("copy", func(t *testing.T) {
		t.Parallel()

		deps := []urn.URN{"1", "2"}
		v := New("1").WithDependencies(deps)
		assert.Equal(t, []urn.URN{"1", "2"}, v.Dependencies())
		deps[0] = "0" // Mutate the slice we passed in
		assert.Equal(t, []urn.URN{"1", "2"}, v.Dependencies())
	})
}
