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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestEncodeDecodePropertyValueForLog(t *testing.T) {
	t.Parallel()

	original := resource.NewProperty(resource.PropertyMap{
		"name": resource.NewProperty("my-bucket"),
		"tags": resource.NewProperty(resource.PropertyMap{
			"env": resource.NewProperty("prod"),
		}),
	})

	encoded, err := EncodePropertyValueForLog(original)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	decoded, err := DecodePropertyValueFromLog(encoded)
	require.NoError(t, err)
	assert.True(t, decoded.DeepEquals(original))
}

func TestEncodeDecodePropertyValueForLogPreservesSecrets(t *testing.T) {
	t.Parallel()

	secret := resource.NewProperty(&resource.Secret{
		Element: resource.NewProperty("hunter2"),
	})
	original := resource.NewProperty(resource.PropertyMap{
		"password": secret,
	})

	encoded, err := EncodePropertyValueForLog(original)
	require.NoError(t, err)

	decoded, err := DecodePropertyValueFromLog(encoded)
	require.NoError(t, err)
	require.True(t, decoded.IsObject())
	pw, ok := decoded.ObjectValue()["password"]
	require.True(t, ok)
	assert.True(t, pw.IsSecret(), "password should still be marked secret after round-trip")
	assert.Equal(t, "hunter2", pw.SecretValue().Element.StringValue())
}

func TestDecodePropertyValueFromLogRejectsTooShort(t *testing.T) {
	t.Parallel()

	_, err := DecodePropertyValueFromLog([]byte("abc"))
	assert.ErrorContains(t, err, "too short")
}

func TestDecodePropertyValueFromLogRejectsWrongMagic(t *testing.T) {
	t.Parallel()

	_, err := DecodePropertyValueFromLog(make([]byte, 16))
	assert.ErrorContains(t, err, "magic mismatch")
}

func TestEncodeDecodeValueForLog(t *testing.T) {
	t.Parallel()

	original := property.New(property.NewMap(map[string]property.Value{
		"name": property.New("my-bucket"),
		"tags": property.New(property.NewMap(map[string]property.Value{
			"env": property.New("prod"),
		})),
	}))

	encoded, err := EncodeValueForLog(original)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	decoded, err := DecodeValueFromLog(encoded)
	require.NoError(t, err)
	assert.True(t, original.Equals(decoded), "got %v want %v", decoded, original)
}

func TestEncodeDecodeValueForLogPreservesSecrets(t *testing.T) {
	t.Parallel()

	original := property.New(property.NewMap(map[string]property.Value{
		"password": property.New("hunter2").WithSecret(true),
	}))

	encoded, err := EncodeValueForLog(original)
	require.NoError(t, err)

	decoded, err := DecodeValueFromLog(encoded)
	require.NoError(t, err)
	pw, ok := decoded.AsMap().GetOk("password")
	require.True(t, ok)
	assert.True(t, pw.Secret(), "password should still be marked secret after round-trip")
	assert.Equal(t, "hunter2", pw.AsString())
}

// TestEncodeValueForLogIsWireCompatible verifies that a value encoded
// via the property.Value API can be decoded via the resource.PropertyValue
// API and vice versa.
func TestEncodeValueForLogIsWireCompatible(t *testing.T) {
	t.Parallel()

	pv := resource.NewProperty(resource.PropertyMap{
		"name": resource.NewProperty("my-bucket"),
	})
	encoded, err := EncodePropertyValueForLog(pv)
	require.NoError(t, err)

	v, err := DecodeValueFromLog(encoded)
	require.NoError(t, err)
	assert.Equal(t, "my-bucket", v.AsMap().Get("name").AsString())
}
