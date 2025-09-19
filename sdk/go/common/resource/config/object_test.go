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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestEmptyObject(t *testing.T) {
	t.Parallel()

	// Test that an empty object can be converted to a property value
	// without error.
	o := object{}
	crypter := nopCrypter{}
	v, err := o.toDecryptedPropertyValue(t.Context(), crypter)
	require.NoError(t, err)
	assert.Equal(t, resource.NewNullProperty(), v)
}

func TestDecryptMap(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		result, err := decryptMap(t.Context(), map[Key]object{}, nopCrypter{})
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("plaintext values", func(t *testing.T) {
		input := map[Key]object{
			MustParseKey("ns:foo"): newObject("bar"),
			MustParseKey("ns:num"): newObject(int64(42)),
		}
		result, err := decryptMap(t.Context(), input, nopCrypter{})
		assert.NoError(t, err)
		assert.Equal(t, "bar", result[MustParseKey("ns:foo")].value)
		assert.Equal(t, int64(42), result[MustParseKey("ns:num")].value)
	})

	t.Run("secure values", func(t *testing.T) {
		input := map[Key]object{
			MustParseKey("ns:secret"): newSecureObject("ciphertext"),
		}
		result, err := decryptMap(t.Context(), input, nopCrypter{})
		assert.NoError(t, err)
		assert.Equal(t, "ciphertext", result[MustParseKey("ns:secret")].value)
		assert.True(t, result[MustParseKey("ns:secret")].secure)
	})

	t.Run("mixed values", func(t *testing.T) {
		input := map[Key]object{
			MustParseKey("ns:plain"):  newObject("value"),
			MustParseKey("ns:secret"): newSecureObject("ciphertext"),
		}
		result, err := decryptMap(t.Context(), input, nopCrypter{})
		assert.NoError(t, err)
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
		result, err := decryptMap(t.Context(), input, nopCrypter{})
		assert.NoError(t, err)
		assert.Equal(t, "s1", result[MustParseKey("ns:a")].value)
		assert.Equal(t, "s2", result[MustParseKey("ns:b")].value)
		assert.Equal(t, "s3", result[MustParseKey("ns:c")].value)
		assert.Equal(t, "plain", result[MustParseKey("ns:d")].value)
		assert.True(t, result[MustParseKey("ns:a")].secure)
		assert.True(t, result[MustParseKey("ns:b")].secure)
		assert.True(t, result[MustParseKey("ns:c")].secure)
	})
}
