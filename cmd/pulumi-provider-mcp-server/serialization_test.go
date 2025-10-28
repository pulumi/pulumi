// Copyright 2016-2024, Pulumi Corporation.
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

package main

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONToPropertyMapBasicTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]any
		expected resource.PropertyMap
	}{
		{
			name: "string property",
			input: map[string]any{
				"foo": "bar",
			},
			expected: resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
			},
		},
		{
			name: "number property",
			input: map[string]any{
				"count": float64(42),
			},
			expected: resource.PropertyMap{
				"count": resource.NewNumberProperty(42),
			},
		},
		{
			name: "boolean property",
			input: map[string]any{
				"enabled": true,
			},
			expected: resource.PropertyMap{
				"enabled": resource.NewBoolProperty(true),
			},
		},
		{
			name: "null property",
			input: map[string]any{
				"nothing": nil,
			},
			expected: resource.PropertyMap{
				"nothing": resource.NewNullProperty(),
			},
		},
		{
			name: "array property",
			input: map[string]any{
				"items": []any{"a", "b", "c"},
			},
			expected: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("a"),
					resource.NewStringProperty("b"),
					resource.NewStringProperty("c"),
				}),
			},
		},
		{
			name: "object property",
			input: map[string]any{
				"nested": map[string]any{
					"key": "value",
				},
			},
			expected: resource.PropertyMap{
				"nested": resource.NewObjectProperty(resource.PropertyMap{
					"key": resource.NewStringProperty("value"),
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := JSONToPropertyMap(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPropertyMapToJSONBasicTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    resource.PropertyMap
		expected map[string]any
	}{
		{
			name: "string property",
			input: resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
			},
			expected: map[string]any{
				"foo": "bar",
			},
		},
		{
			name: "number property",
			input: resource.PropertyMap{
				"count": resource.NewNumberProperty(42),
			},
			expected: map[string]any{
				"count": float64(42),
			},
		},
		{
			name: "boolean property",
			input: resource.PropertyMap{
				"enabled": resource.NewBoolProperty(true),
			},
			expected: map[string]any{
				"enabled": true,
			},
		},
		{
			name: "null property",
			input: resource.PropertyMap{
				"nothing": resource.NewNullProperty(),
			},
			expected: map[string]any{
				"nothing": nil,
			},
		},
		{
			name: "array property",
			input: resource.PropertyMap{
				"items": resource.NewArrayProperty([]resource.PropertyValue{
					resource.NewStringProperty("a"),
					resource.NewStringProperty("b"),
				}),
			},
			expected: map[string]any{
				"items": []any{"a", "b"},
			},
		},
		{
			name: "object property",
			input: resource.PropertyMap{
				"nested": resource.NewObjectProperty(resource.PropertyMap{
					"key": resource.NewStringProperty("value"),
				}),
			},
			expected: map[string]any{
				"nested": map[string]any{
					"key": "value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := PropertyMapToJSON(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSecretRoundTrip(t *testing.T) {
	t.Parallel()

	// Create a secret property
	secretValue := resource.NewStringProperty("my-secret")
	secretProp := resource.MakeSecret(secretValue)

	// Convert to JSON
	jsonVal, err := propertyValueToJSON(secretProp)
	require.NoError(t, err)

	jsonMap, ok := jsonVal.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1b47061264138c4ac30d75fd1eb44270", jsonMap["4dabf18193072939515e22adb298388d"])
	assert.Equal(t, "my-secret", jsonMap["value"])

	// Convert back to PropertyValue
	result, err := jsonToPropertyValue(jsonVal)
	require.NoError(t, err)
	assert.True(t, result.IsSecret())
	assert.Equal(t, "my-secret", result.SecretValue().Element.StringValue())
}

func TestAssetRoundTrip(t *testing.T) {
	t.Parallel()

	// Create a text asset
	textAsset, err := asset.FromText("hello world")
	require.NoError(t, err)
	assetProp := resource.NewAssetProperty(textAsset)

	// Convert to JSON
	jsonVal, err := propertyValueToJSON(assetProp)
	require.NoError(t, err)

	jsonMap, ok := jsonVal.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "0def7320c3a5731c473e5ecbe6d01bc7", jsonMap["4dabf18193072939515e22adb298388d"])
	assert.Equal(t, "hello world", jsonMap["text"])

	// Convert back to PropertyValue
	result, err := jsonToPropertyValue(jsonVal)
	require.NoError(t, err)
	assert.True(t, result.IsAsset())
	assert.True(t, result.AssetValue().IsText())
	assert.Equal(t, "hello world", result.AssetValue().Text)
}

func TestResourceReferenceRoundTrip(t *testing.T) {
	t.Parallel()

	// Create a resource reference
	ref := resource.ResourceReference{
		URN: resource.URN("urn:pulumi:dev::my-project::aws:s3/bucket:Bucket::my-bucket"),
		ID:  resource.NewStringProperty("bucket-id-123"),
	}
	refProp := resource.NewResourceReferenceProperty(ref)

	// Convert to JSON
	jsonVal, err := propertyValueToJSON(refProp)
	require.NoError(t, err)

	jsonMap, ok := jsonVal.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cfe97e649c90f5f7c0d6c9c3b0c4e3e6", jsonMap["4dabf18193072939515e22adb298388d"])
	assert.Equal(t, "urn:pulumi:dev::my-project::aws:s3/bucket:Bucket::my-bucket", jsonMap["urn"])
	assert.Equal(t, "bucket-id-123", jsonMap["id"])

	// Convert back to PropertyValue
	result, err := jsonToPropertyValue(jsonVal)
	require.NoError(t, err)
	assert.True(t, result.IsResourceReference())
	assert.Equal(t, ref.URN, result.ResourceReferenceValue().URN)
	assert.Equal(t, "bucket-id-123", result.ResourceReferenceValue().ID.StringValue())
}

func TestComplexNestedStructure(t *testing.T) {
	t.Parallel()

	// Create a complex nested structure
	original := map[string]any{
		"name": "my-resource",
		"config": map[string]any{
			"enabled": true,
			"count":   float64(5),
			"tags": []any{
				"tag1",
				"tag2",
			},
			"nested": map[string]any{
				"key": "value",
			},
		},
		"items": []any{
			map[string]any{"id": float64(1)},
			map[string]any{"id": float64(2)},
		},
	}

	// Convert to PropertyMap
	props, err := JSONToPropertyMap(original)
	require.NoError(t, err)

	// Convert back to JSON
	result, err := PropertyMapToJSON(props)
	require.NoError(t, err)

	// Should be equivalent
	assert.Equal(t, original, result)
}

func TestEmptyPropertyMap(t *testing.T) {
	t.Parallel()

	// Test empty map
	result, err := JSONToPropertyMap(nil)
	require.NoError(t, err)
	assert.Equal(t, resource.PropertyMap{}, result)

	result, err = JSONToPropertyMap(map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, resource.PropertyMap{}, result)
}

func TestRandomSeedConversion(t *testing.T) {
	t.Parallel()

	// Test empty seed
	bytes, err := RandomSeedToBytes("")
	require.NoError(t, err)
	assert.Nil(t, bytes)

	// Test base64 encoding/decoding
	original := []byte{1, 2, 3, 4, 5}
	encoded := RandomSeedToString(original)
	decoded, err := RandomSeedToBytes(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestMarshalUnmarshalPropertyMap(t *testing.T) {
	t.Parallel()

	original := resource.PropertyMap{
		"string": resource.NewStringProperty("value"),
		"number": resource.NewNumberProperty(42),
		"bool":   resource.NewBoolProperty(true),
		"array": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
		}),
	}

	// Marshal to JSON bytes
	data, err := MarshalPropertyMap(original)
	require.NoError(t, err)

	// Unmarshal back
	result, err := UnmarshalPropertyMap(data)
	require.NoError(t, err)

	assert.Equal(t, original, result)
}
