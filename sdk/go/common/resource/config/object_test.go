// Copyright 2016-2023, Pulumi Corporation.
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

package config

import (
	"context"
	"math"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmptyObject(t *testing.T) {
	t.Parallel()

	// Test that an empty object can be converted to a property value
	// without error.
	o := object{}
	crypter := nopCrypter{}
	v, err := o.toDecryptedPropertyValue(context.Background(), crypter)
	require.NoError(t, err)
	assert.Equal(t, resource.NewNullProperty(), v)
}

func TestMarshallingRoundtrip(t *testing.T) {
	t.Parallel()

	obj := newObject(map[string]object{
		"hello": newObject([]object{
			newObject(true),
			newObject(int64(42)),
			newObject(uint64(math.MaxUint64)),
			newObject(float64(3.14159)),
			newObject("world"),
			newSecureObject("moon"),
		}),
	})

	bytes, err := obj.MarshalJSON()
	require.NoError(t, err)

	err = obj.UnmarshalJSON(bytes)
	require.NoError(t, err)

	rt := newObject(map[string]object{
		"hello": newObject([]object{
			newObject(true),
			newObject(int64(42)),
			// uint64 can't roundtrip through JSON
			newObject(float64(math.MaxUint64)),
			newObject(float64(3.14159)),
			newObject("world"),
			newSecureObject("moon"),
		}),
	})

	assert.Equal(t, rt, obj)
}

//nolint:paralleltest // changes global defaultMaxChunkSize variable
func TestDecryptMap(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		result, err := decryptMap(context.Background(), map[Key]object{}, nopCrypter{})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("Plaintext values", func(t *testing.T) {
		input := map[Key]object{
			MustParseKey("ns:foo"): newObject("bar"),
			MustParseKey("ns:num"): newObject(int64(42)),
		}
		result, err := decryptMap(context.Background(), input, nopCrypter{})
		require.NoError(t, err)
		assert.Equal(t, "bar", result[MustParseKey("ns:foo")].value)
		assert.Equal(t, int64(42), result[MustParseKey("ns:num")].value)
	})

	t.Run("secure values", func(t *testing.T) {
		input := map[Key]object{
			MustParseKey("ns:secret"): newSecureObject("ciphertext"),
		}
		result, err := decryptMap(context.Background(), input, nopCrypter{})
		require.NoError(t, err)
		assert.Equal(t, "ciphertext", result[MustParseKey("ns:secret")].value)
		assert.True(t, result[MustParseKey("ns:secret")].secure)
	})

	t.Run("mixed values", func(t *testing.T) {
		input := map[Key]object{
			MustParseKey("ns:plain"):  newObject("value"),
			MustParseKey("ns:secret"): newSecureObject("ciphertext"),
		}
		result, err := decryptMap(context.Background(), input, nopCrypter{})
		require.NoError(t, err)
		assert.Equal(t, "value", result[MustParseKey("ns:plain")].value)
		assert.Equal(t, "ciphertext", result[MustParseKey("ns:secret")].value)
		assert.True(t, result[MustParseKey("ns:secret")].secure)
	})

	t.Run("chunking", func(t *testing.T) {
		origChunkSize := defaultMaxChunkSize
		defaultMaxChunkSize = 2 // force batching for test
		defer func() { defaultMaxChunkSize = origChunkSize }()

		input := map[Key]object{
			MustParseKey("ns:a"): newSecureObject("s1"),
			MustParseKey("ns:b"): newSecureObject("s2"),
			MustParseKey("ns:c"): newSecureObject("s3"),
			MustParseKey("ns:d"): newObject("plain"),
		}
		result, err := decryptMap(context.Background(), input, nopCrypter{})
		require.NoError(t, err)
		assert.Equal(t, "s1", result[MustParseKey("ns:a")].value)
		assert.Equal(t, "s2", result[MustParseKey("ns:b")].value)
		assert.Equal(t, "s3", result[MustParseKey("ns:c")].value)
		assert.Equal(t, "plain", result[MustParseKey("ns:d")].value)
		assert.True(t, result[MustParseKey("ns:a")].secure)
		assert.True(t, result[MustParseKey("ns:b")].secure)
		assert.True(t, result[MustParseKey("ns:c")].secure)
	})
}
