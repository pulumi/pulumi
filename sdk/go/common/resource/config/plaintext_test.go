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
		NewPlaintext(map[string]Plaintext{
			"secure": NewPlaintext("hello"),
		})
	})

	NewPlaintext(map[string]Plaintext{
		"secure": NewPlaintext(int64(42)),
	})
}

func TestPlaintextSecure(t *testing.T) {
	t.Parallel()

	plain := NewPlaintext("hello")
	assert.False(t, plain.Secure())

	plain = NewPlaintext(PlaintextSecret("hello"))
	assert.True(t, plain.Secure())

	plain = NewPlaintext(map[string]Plaintext{
		"hello": NewPlaintext([]Plaintext{
			NewPlaintext(true),
			NewPlaintext(int64(42)),
			NewPlaintext(uint64(math.MaxUint64)),
			NewPlaintext(float64(3.14159)),
			NewPlaintext("world"),
			NewPlaintext(PlaintextSecret("moon")),
		}),
	})
	assert.True(t, plain.Secure())

	plain = NewPlaintext(map[string]Plaintext{
		"hello": NewPlaintext([]Plaintext{
			NewPlaintext(true),
			NewPlaintext(int64(42)),
			NewPlaintext(uint64(math.MaxUint64)),
			NewPlaintext(float64(3.14159)),
			NewPlaintext("world"),
		}),
	})
	assert.False(t, plain.Secure())
}

func TestPlaintextEncrypt(t *testing.T) {
	t.Parallel()

	plain := NewPlaintext(map[string]Plaintext{
		"hello": NewPlaintext([]Plaintext{
			NewPlaintext(true),
			NewPlaintext(int64(42)),
			NewPlaintext(uint64(math.MaxUint64)),
			NewPlaintext(float64(3.14159)),
			NewPlaintext("world"),
			NewPlaintext(PlaintextSecret("moon")),
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
			newObject(CiphertextSecret{"moon"}),
		}),
	})
	assert.Equal(t, expected, actual)
}

func TestPlaintextRoundtrip(t *testing.T) {
	t.Parallel()

	plain := NewPlaintext(map[string]Plaintext{
		"hello": NewPlaintext([]Plaintext{
			NewPlaintext(true),
			NewPlaintext(int64(42)),
			NewPlaintext(uint64(math.MaxUint64)),
			NewPlaintext(float64(3.14159)),
			NewPlaintext("world"),
			NewPlaintext(PlaintextSecret("moon")),
		}),
	})
	obj, err := plain.encrypt(context.Background(), nil, NopEncrypter)
	require.NoError(t, err)

	actual, err := obj.decrypt(context.Background(), nil, NopDecrypter)
	require.NoError(t, err)

	assert.Equal(t, plain, actual)
}

func TestMarshalPlaintext(t *testing.T) {
	t.Parallel()

	plain := NewPlaintext(int64(42))

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
	ctx := context.Background()

	t.Run("empty map", func(t *testing.T) {
		result, err := encryptMap(ctx, map[Key]Plaintext{}, nopCrypter{})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("Plaintext values", func(t *testing.T) {
		input := map[Key]Plaintext{
			MustParseKey("ns:foo"): NewPlaintext("bar"),
			MustParseKey("ns:num"): NewPlaintext(int64(42)),
		}
		result, err := encryptMap(ctx, input, nopCrypter{})
		require.NoError(t, err)
		assert.Equal(t, "bar", result[MustParseKey("ns:foo")].value)
		assert.Equal(t, int64(42), result[MustParseKey("ns:num")].value)
	})

	t.Run("secure values", func(t *testing.T) {
		input := map[Key]Plaintext{
			MustParseKey("ns:secret"): NewPlaintext(PlaintextSecret("Plaintext")),
		}
		result, err := encryptMap(ctx, input, nopCrypter{})
		require.NoError(t, err)
		assert.Equal(t, CiphertextSecret{"Plaintext"}, result[MustParseKey("ns:secret")].value)
		assert.True(t, result[MustParseKey("ns:secret")].Secure())
	})

	t.Run("nested secure values", func(t *testing.T) {
		input := map[Key]Plaintext{
			MustParseKey("ns:secret"): NewPlaintext(map[string]Plaintext{
				"foo": NewPlaintext(PlaintextSecret("Plaintext")),
			}),
		}
		result, err := encryptMap(context.Background(), input, nopCrypter{})
		require.NoError(t, err)
		assert.Equal(t, newObject(CiphertextSecret{"Plaintext"}), result[MustParseKey("ns:secret")].value.(map[string]object)["foo"])
		assert.True(t, result[MustParseKey("ns:secret")].Secure())
	})

	t.Run("mixed values", func(t *testing.T) {
		input := map[Key]Plaintext{
			MustParseKey("ns:plain"):  NewPlaintext("value"),
			MustParseKey("ns:secret"): NewPlaintext(PlaintextSecret("Plaintext")),
		}
		result, err := encryptMap(ctx, input, nopCrypter{})
		require.NoError(t, err)
		assert.Equal(t, "value", result[MustParseKey("ns:plain")].value)
		assert.Equal(t, CiphertextSecret{"Plaintext"}, result[MustParseKey("ns:secret")].value)
		assert.True(t, result[MustParseKey("ns:secret")].Secure())
	})

	t.Run("chunking", func(t *testing.T) {
		origChunkSize := defaultMaxChunkSize
		defaultMaxChunkSize = 2 // force batching for test
		defer func() { defaultMaxChunkSize = origChunkSize }()

		input := map[Key]Plaintext{
			MustParseKey("ns:a"): NewPlaintext(PlaintextSecret("s1")),
			MustParseKey("ns:b"): NewPlaintext(PlaintextSecret("s2")),
			MustParseKey("ns:c"): NewPlaintext(PlaintextSecret("s3")),
			MustParseKey("ns:d"): NewPlaintext("plain"),
		}
		result, err := encryptMap(ctx, input, nopCrypter{})
		require.NoError(t, err)
		assert.Equal(t, CiphertextSecret{"s1"}, result[MustParseKey("ns:a")].value)
		assert.Equal(t, CiphertextSecret{"s2"}, result[MustParseKey("ns:b")].value)
		assert.Equal(t, CiphertextSecret{"s3"}, result[MustParseKey("ns:c")].value)
		assert.Equal(t, "plain", result[MustParseKey("ns:d")].value)
		assert.True(t, result[MustParseKey("ns:a")].Secure())
		assert.True(t, result[MustParseKey("ns:b")].Secure())
		assert.True(t, result[MustParseKey("ns:c")].Secure())
	})
}
