// Copyright 2025, Pulumi Corporation.
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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// recordingDecrypter records calls and returns "pt:"+ciphertext for decrypts.
type recordingDecrypter struct {
	received []string
}

func (r *recordingDecrypter) DecryptValue(ctx context.Context, ct string) (string, error) {
	r.received = append(r.received, ct)
	return "pt:" + ct, nil
}

func (r *recordingDecrypter) BatchDecrypt(ctx context.Context, cts []string) ([]string, error) {
	r.received = append(r.received, cts...)
	out := make([]string, len(cts))
	for i, ct := range cts {
		out[i] = "pt:" + ct
	}
	return out, nil
}

// recordingEncrypter increments a producedSecretsCount and embeds it in the ciphertext to ensure
// each encrypt call produces a new unique ciphertext.
type recordingEncrypter struct {
	producedSecretsCount int
}

func (r *recordingEncrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	r.producedSecretsCount++
	return fmt.Sprintf("ct:%d:%s", r.producedSecretsCount, plaintext), nil
}

func (r *recordingEncrypter) BatchEncrypt(ctx context.Context, secrets []string) ([]string, error) {
	out := make([]string, len(secrets))
	for i, s := range secrets {
		r.producedSecretsCount++
		out[i] = fmt.Sprintf("ct:%d:%s", r.producedSecretsCount, s)
	}
	return out, nil
}

func TestCiphertextToPlaintextCachedCrypter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("EncryptValue stores mapping", func(t *testing.T) {
		c := NewCiphertextToPlaintextCachedCrypter(NopEncrypter, NewPanicCrypter())
		ct, err := c.EncryptValue(ctx, "secret")
		require.NoError(t, err)
		require.Equal(t, "secret", ct)

		got, ok := c.ciphertextToPlaintextCache[ct]
		require.True(t, ok)
		require.Equal(t, "secret", got)
	})

	t.Run("EncryptValue always calls underlying encrypter and produces new ciphertexts", func(t *testing.T) {
		rec := &recordingEncrypter{}
		// Use panic decrypter to ensure decrypt isn't used in this test.
		c := NewCiphertextToPlaintextCachedCrypter(rec, NewPanicCrypter())

		ct1, err := c.EncryptValue(ctx, "duplicate")
		require.NoError(t, err)
		ct2, err := c.EncryptValue(ctx, "duplicate")
		require.NoError(t, err)

		require.NotEqual(t, ct1, ct2, "encrypt should produce distinct ciphertexts for repeated plaintexts")
		require.Equal(t, 2, rec.producedSecretsCount, "underlying encrypter should have been called twice")

		// Both ciphertexts should be cached to plaintext after encryption.
		got1, ok1 := c.ciphertextToPlaintextCache[ct1]
		got2, ok2 := c.ciphertextToPlaintextCache[ct2]
		require.True(t, ok1)
		require.True(t, ok2)
		require.Equal(t, "duplicate", got1)
		require.Equal(t, "duplicate", got2)
	})

	t.Run("BatchEncrypt stores mapping", func(t *testing.T) {
		c := NewCiphertextToPlaintextCachedCrypter(NopEncrypter, NewPanicCrypter())
		pts := []string{"a", "b", "c"}
		cts, err := c.BatchEncrypt(ctx, pts)
		require.NoError(t, err)
		require.Equal(t, len(pts), len(cts))

		for i, ct := range cts {
			require.Equal(t, pts[i], ct)
			got, ok := c.ciphertextToPlaintextCache[ct]
			require.True(t, ok)
			require.Equal(t, pts[i], got)
		}
	})

	t.Run("BatchEncrypt produces unique ciphertexts for duplicates and increments call count", func(t *testing.T) {
		rec := &recordingEncrypter{}
		c := NewCiphertextToPlaintextCachedCrypter(rec, NewPanicCrypter())

		pts := []string{"dup", "dup", "uniq"}
		cts, err := c.BatchEncrypt(ctx, pts)
		require.NoError(t, err)
		require.Equal(t, len(pts), len(cts))
		// The two duplicate plaintexts should have different ciphertexts.
		require.NotEqual(t, cts[0], cts[1])
		// producedSecretsCount should equal number of items encrypted.
		require.Equal(t, 3, rec.producedSecretsCount)

		// All ciphertexts should be cached.
		for i, ct := range cts {
			got, ok := c.ciphertextToPlaintextCache[ct]
			require.True(t, ok)
			require.Equal(t, pts[i], got)
		}
	})

	t.Run("DecryptValue uses cache and avoids underlying decrypter", func(t *testing.T) {
		// Use panic decrypter to ensure it's not called when value is cached.
		c := NewCiphertextToPlaintextCachedCrypter(NopEncrypter, NewPanicCrypter())
		// Populate cache.
		ct, err := c.EncryptValue(ctx, "hidden")
		require.NoError(t, err)
		// Should return from cache and not panic.
		pt, err := c.DecryptValue(ctx, ct)
		require.NoError(t, err)
		require.Equal(t, "hidden", pt)

		// When not cached, underlying decrypter should be called.
		rec := &recordingDecrypter{}
		c2 := NewCiphertextToPlaintextCachedCrypter(NopEncrypter, rec)
		out, err := c2.DecryptValue(ctx, "miss")
		require.NoError(t, err)
		require.Equal(t, "pt:miss", out)
		require.Equal(t, []string{"miss"}, rec.received)
	})

	t.Run("BatchDecrypt uses cache and only calls underlying for misses", func(t *testing.T) {
		rec := &recordingDecrypter{}
		c := NewCiphertextToPlaintextCachedCrypter(NopEncrypter, rec)

		// Prepopulate cache for "a" and "b"
		ct1, err := c.EncryptValue(ctx, "a")
		require.NoError(t, err)
		ct2, err := c.EncryptValue(ctx, "b")
		require.NoError(t, err)

		input := []string{ct1, "c", ct2, "d"}
		out, err := c.BatchDecrypt(ctx, input)
		require.NoError(t, err)

		expected := []string{"a", "pt:c", "b", "pt:d"}
		require.Equal(t, expected, out)

		// recordingDecrypter should have been called only for the misses in order.
		require.Equal(t, []string{"c", "d"}, rec.received)
	})
}
