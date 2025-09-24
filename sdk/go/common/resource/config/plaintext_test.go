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
	"encoding/json"
	"math"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestPlaintextReserved(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		newPlaintext(map[string]plaintext{
			"secure": newPlaintext("hello"),
		})
	})

	newPlaintext(map[string]plaintext{
		"secure": newPlaintext(int64(42)),
	})
}

func TestPlaintextSecure(t *testing.T) {
	t.Parallel()

	plain := newPlaintext("hello")
	assert.False(t, plain.Secure())

	plain = newSecurePlaintext("hello")
	assert.True(t, plain.Secure())

	plain = newPlaintext(map[string]plaintext{
		"hello": newPlaintext([]plaintext{
			newPlaintext(true),
			newPlaintext(int64(42)),
			newPlaintext(uint64(math.MaxUint64)),
			newPlaintext(float64(3.14159)),
			newPlaintext("world"),
			newSecurePlaintext("moon"),
		}),
	})
	assert.True(t, plain.Secure())

	plain = newPlaintext(map[string]plaintext{
		"hello": newPlaintext([]plaintext{
			newPlaintext(true),
			newPlaintext(int64(42)),
			newPlaintext(uint64(math.MaxUint64)),
			newPlaintext(float64(3.14159)),
			newPlaintext("world"),
		}),
	})
	assert.False(t, plain.Secure())
}

func TestPlaintextEncrypt(t *testing.T) {
	t.Parallel()

	plain := newPlaintext(map[string]plaintext{
		"hello": newPlaintext([]plaintext{
			newPlaintext(true),
			newPlaintext(int64(42)),
			newPlaintext(uint64(math.MaxUint64)),
			newPlaintext(float64(3.14159)),
			newPlaintext("world"),
			newSecurePlaintext("moon"),
		}),
	})
	actual, err := plain.encrypt(context.Background(), nil, NopEncrypter)
	require.NoError(t, err)

	expected := newObject(map[string]object{
		"hello": newObject([]object{
			newObject(true),
			newObject(int64(42)),
			newObject(uint64(math.MaxUint64)),
			newObject(float64(3.14159)),
			newObject("world"),
			newSecureObject("moon"),
		}),
	})
	assert.Equal(t, expected, actual)
}

func TestPlaintextRoundtrip(t *testing.T) {
	t.Parallel()

	plain := newPlaintext(map[string]plaintext{
		"hello": newPlaintext([]plaintext{
			newPlaintext(true),
			newPlaintext(int64(42)),
			newPlaintext(uint64(math.MaxUint64)),
			newPlaintext(float64(3.14159)),
			newPlaintext("world"),
			newSecurePlaintext("moon"),
		}),
	})
	obj, err := plain.Encrypt(context.Background(), NopEncrypter)
	require.NoError(t, err)

	actual, err := obj.Decrypt(context.Background(), NopDecrypter)
	require.NoError(t, err)

	assert.Equal(t, plain, actual)
}

func TestMarshalPlaintext(t *testing.T) {
	t.Parallel()

	plain := newPlaintext(int64(42))

	assert.Panics(t, func() {
		_, err := json.Marshal(plain)
		contract.IgnoreError(err)
	})

	assert.Panics(t, func() {
		_, err := yaml.Marshal(plain)
		contract.IgnoreError(err)
	})

	assert.Panics(t, func() {
		err := json.Unmarshal([]byte("42"), &plain)
		contract.IgnoreError(err)
	})

	assert.Panics(t, func() {
		err := yaml.Unmarshal([]byte("42"), &plain)
		contract.IgnoreError(err)
	})
}

//nolint:paralleltest // changes global defaultMaxChunkSize variable
func TestEncryptMap(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		result, err := encryptMap(t.Context(), map[Key]plaintext{}, nopCrypter{})
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("plaintext values", func(t *testing.T) {
		input := map[Key]plaintext{
			MustParseKey("ns:foo"): newPlaintext("bar"),
			MustParseKey("ns:num"): newPlaintext(int64(42)),
		}
		result, err := encryptMap(t.Context(), input, nopCrypter{})
		assert.NoError(t, err)
		assert.Equal(t, "bar", result[MustParseKey("ns:foo")].value)
		assert.Equal(t, int64(42), result[MustParseKey("ns:num")].value)
	})

	t.Run("secure values", func(t *testing.T) {
		input := map[Key]plaintext{
			MustParseKey("ns:secret"): newSecurePlaintext("plaintext"),
		}
		result, err := encryptMap(t.Context(), input, nopCrypter{})
		assert.NoError(t, err)
		assert.Equal(t, "plaintext", result[MustParseKey("ns:secret")].value)
		assert.True(t, result[MustParseKey("ns:secret")].secure)
	})

	t.Run("mixed values", func(t *testing.T) {
		input := map[Key]plaintext{
			MustParseKey("ns:plain"):  newPlaintext("value"),
			MustParseKey("ns:secret"): newSecurePlaintext("plaintext"),
		}
		result, err := encryptMap(t.Context(), input, nopCrypter{})
		assert.NoError(t, err)
		assert.Equal(t, "value", result[MustParseKey("ns:plain")].value)
		assert.Equal(t, "plaintext", result[MustParseKey("ns:secret")].value)
		assert.True(t, result[MustParseKey("ns:secret")].secure)
	})

	t.Run("chunking", func(t *testing.T) {
		origChunkSize := defaultMaxChunkSize
		defaultMaxChunkSize = 2 // force batching for test
		defer func() { defaultMaxChunkSize = origChunkSize }()

		input := map[Key]plaintext{
			MustParseKey("ns:a"): newSecurePlaintext("s1"),
			MustParseKey("ns:b"): newSecurePlaintext("s2"),
			MustParseKey("ns:c"): newSecurePlaintext("s3"),
			MustParseKey("ns:d"): newPlaintext("plain"),
		}
		result, err := encryptMap(t.Context(), input, nopCrypter{})
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
