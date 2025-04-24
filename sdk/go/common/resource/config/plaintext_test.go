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

	plain = NewSecurePlaintext("hello")
	assert.True(t, plain.Secure())

	plain = NewPlaintext(map[string]Plaintext{
		"hello": NewPlaintext([]Plaintext{
			NewPlaintext(true),
			NewPlaintext(int64(42)),
			NewPlaintext(uint64(math.MaxUint64)),
			NewPlaintext(float64(3.14159)),
			NewPlaintext("world"),
			NewSecurePlaintext("moon"),
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
			NewSecurePlaintext("moon"),
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

	plain := NewPlaintext(map[string]Plaintext{
		"hello": NewPlaintext([]Plaintext{
			NewPlaintext(true),
			NewPlaintext(int64(42)),
			NewPlaintext(uint64(math.MaxUint64)),
			NewPlaintext(float64(3.14159)),
			NewPlaintext("world"),
			NewSecurePlaintext("moon"),
		}),
	})
	value, err := plain.Encrypt(context.Background(), NopEncrypter)
	require.NoError(t, err)

	actual, err := value.Decrypt(context.Background(), NopDecrypter)
	require.NoError(t, err)

	rt := NewPlaintext(map[string]Plaintext{
		"hello": NewPlaintext([]Plaintext{
			NewPlaintext(true),
			NewPlaintext(int64(42)),
			// uint64 can't roundtrip through JSON
			NewPlaintext(float64(math.MaxUint64)),
			NewPlaintext(float64(3.14159)),
			NewPlaintext("world"),
			NewSecurePlaintext("moon"),
		}),
	})

	assert.Equal(t, rt, actual)
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
