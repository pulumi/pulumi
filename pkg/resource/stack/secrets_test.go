// Copyright 2019-2024, Pulumi Corporation.
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

package stack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
)

type testSecretsManager struct {
	encryptCalls      int
	decryptCalls      int
	batchEncryptCalls int
	batchDecryptCalls int
}

func (t *testSecretsManager) Type() string { return "test" }

func (t *testSecretsManager) State() json.RawMessage { return nil }

func (t *testSecretsManager) Encrypter() config.Encrypter {
	return t
}

func (t *testSecretsManager) Decrypter() config.Decrypter {
	return t
}

func (t *testSecretsManager) EncryptValue(
	ctx context.Context, plaintext string,
) (string, error) {
	t.encryptCalls++
	return fmt.Sprintf("%v:%v", t.encryptCalls, plaintext), nil
}

func (t *testSecretsManager) BatchEncrypt(
	ctx context.Context, plaintexts []string,
) ([]string, error) {
	t.batchEncryptCalls++
	if len(plaintexts) == 0 {
		return nil, nil
	}

	encrypted := make([]string, len(plaintexts))
	for i, plaintext := range plaintexts {
		encrypted[i] = fmt.Sprintf("%v-%v:%v", t.batchEncryptCalls, i+1, plaintext)
	}
	return encrypted, nil
}

func (t *testSecretsManager) DecryptValue(
	ctx context.Context, ciphertext string,
) (string, error) {
	t.decryptCalls++
	i := strings.Index(ciphertext, ":")
	if i == -1 {
		return "", errors.New("invalid ciphertext format")
	}
	return ciphertext[i+1:], nil
}

func (t *testSecretsManager) BatchDecrypt(
	ctx context.Context, ciphertexts []string,
) ([]string, error) {
	t.batchDecryptCalls++
	if len(ciphertexts) == 0 {
		return nil, nil
	}
	decrypted := make([]string, len(ciphertexts))
	for i, ciphertext := range ciphertexts {
		j := strings.Index(ciphertext, ":")
		if j == -1 {
			return nil, errors.New("invalid ciphertext format")
		}
		decrypted[i] = ciphertext[j+1:]
	}
	return decrypted, nil
}

func deserializeProperty(v interface{}, dec config.Decrypter) (resource.PropertyValue, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return resource.PropertyValue{}, err
	}
	var v2 any
	if err := json.Unmarshal(b, &v2); err != nil {
		return resource.PropertyValue{}, err
	}
	return DeserializePropertyValue(v2, dec)
}

func TestCachingSecretsManager(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sm := &testSecretsManager{}
	csm := NewBatchingSecretsManager(sm)

	foo1 := resource.MakeSecret(resource.NewStringProperty("foo"))
	foo2 := resource.MakeSecret(resource.NewStringProperty("foo"))
	bar := resource.MakeSecret(resource.NewStringProperty("bar"))

	// Serialize the first copy of "foo". Encrypt should be called once, as this value has not yet been encrypted.
	enc, completeEnc := csm.BeginBatchEncryption()
	foo1Ser, err := SerializePropertyValue(ctx, foo1, enc, false /* showSecrets */)
	assert.NoError(t, err, "serialize")
	assert.NoError(t, completeEnc(ctx), "complete")
	assert.Equal(t, 1, sm.batchEncryptCalls, "batch encrypt calls")
	assert.Equal(t, &apitype.SecretV1{Sig: "1b47061264138c4ac30d75fd1eb44270", Ciphertext: "1-1:\"foo\""}, foo1Ser)

	// Serialize the second copy of "foo". Because this is a different secret instance, Encrypt should be called
	// a second time even though the plaintext is the same as the last value we encrypted.
	enc, completeEnc = csm.BeginBatchEncryption()
	foo2Ser, err := SerializePropertyValue(ctx, foo2, enc, false /* showSecrets */)
	assert.NoError(t, err, "serialize")
	assert.NoError(t, completeEnc(ctx), "complete")
	assert.Equal(t, 2, sm.batchEncryptCalls, "batch encrypt calls")
	assert.NotEqual(t, foo1Ser, foo2Ser)

	// Serialize "bar". Encrypt should be called once, as this value has not yet been encrypted.
	enc, completeEnc = csm.BeginBatchEncryption()
	barSer, err := SerializePropertyValue(ctx, bar, enc, false /* showSecrets */)
	assert.NoError(t, err, "serialize")
	assert.NoError(t, completeEnc(ctx), "complete")
	assert.Equal(t, 3, sm.batchEncryptCalls, "batch encrypt calls")

	// Serialize the first copy of "foo" again. Encrypt should not be called, as this value has already been
	// encrypted.
	enc, completeEnc = csm.BeginBatchEncryption()
	foo1Ser2, err := SerializePropertyValue(ctx, foo1, enc, false /* showSecrets */)
	assert.NoError(t, err, "serialize")
	assert.NoError(t, completeEnc(ctx), "complete")
	assert.Equal(t, 3, sm.batchEncryptCalls, "batch encrypt calls")
	assert.Equal(t, foo1Ser, foo1Ser2)

	// Serialize the second copy of "foo" again. Encrypt should not be called, as this value has already been
	// encrypted.
	enc, completeEnc = csm.BeginBatchEncryption()
	foo2Ser2, err := SerializePropertyValue(ctx, foo2, enc, false /* showSecrets */)
	assert.NoError(t, err, "serialize")
	assert.NoError(t, completeEnc(ctx), "complete")
	assert.Equal(t, 3, sm.batchEncryptCalls, "batch encrypt calls")
	assert.Equal(t, foo2Ser, foo2Ser2)

	// Serialize "bar" again. Encrypt should not be called, as this value has already been encrypted.
	enc, completeEnc = csm.BeginBatchEncryption()
	barSer2, err := SerializePropertyValue(ctx, bar, enc, false /* showSecrets */)
	assert.NoError(t, err, "serialize")
	assert.NoError(t, completeEnc(ctx), "complete")
	assert.Equal(t, 3, sm.batchEncryptCalls, "batch encrypt calls")
	assert.Equal(t, barSer, barSer2)

	// Decrypt foo1Ser. Shares cache from encrypt.
	dec, completeDec := csm.BeginBatchDecryption()
	foo1Dec, err := deserializeProperty(foo1Ser, dec)
	assert.NoError(t, err, "deserialize")
	assert.NoError(t, completeDec(ctx), "complete")
	assert.Equal(t, 0, sm.batchDecryptCalls, "batch decrypt calls")
	assert.Equal(t, 0, sm.decryptCalls, "decrypt calls")
	assert.True(t, foo1.DeepEquals(foo1Dec))

	// Decrypt foo2Ser. Shares cache from encrypt.
	dec, completeDec = csm.BeginBatchDecryption()
	foo2Dec, err := deserializeProperty(foo2Ser, dec)
	assert.NoError(t, err, "deserialize")
	assert.NoError(t, completeDec(ctx), "complete")
	assert.Equal(t, 0, sm.batchDecryptCalls, "batch decrypt calls")
	assert.Equal(t, 0, sm.decryptCalls, "decrypt calls")
	assert.True(t, foo2.DeepEquals(foo2Dec))

	// Decrypt barSer. Shares cache from encrypt.
	dec, completeDec = csm.BeginBatchDecryption()
	barDec, err := deserializeProperty(barSer, dec)
	assert.NoError(t, err, "deserialize")
	assert.NoError(t, completeDec(ctx), "complete")
	assert.Equal(t, 0, sm.batchDecryptCalls, "batch decrypt calls")
	assert.Equal(t, 0, sm.decryptCalls, "decrypt calls")
	assert.True(t, bar.DeepEquals(barDec))

	// Create a new CachingSecretsManager and re-run the decrypts. Each decrypt should insert the plain- and
	// ciphertext into the cache with the associated secret.
	csm = NewBatchingSecretsManager(sm)

	// Decrypt foo1Ser. Decrypt should be called.
	dec, completeDec = csm.BeginBatchDecryption()
	foo1Dec, err = deserializeProperty(foo1Ser, dec)
	assert.NoError(t, err, "deserialize")
	assert.NoError(t, completeDec(ctx), "complete")
	assert.Equal(t, 1, sm.batchDecryptCalls, "batch decrypt calls")
	assert.True(t, foo1.DeepEquals(foo1Dec))

	// Decrypt foo2Ser. Decrypt should be called.
	dec, completeDec = csm.BeginBatchDecryption()
	foo2Dec, err = deserializeProperty(foo2Ser, dec)
	assert.NoError(t, err, "deserialize")
	assert.NoError(t, completeDec(ctx), "complete")
	assert.Equal(t, 2, sm.batchDecryptCalls, "batch decrypt calls")
	assert.True(t, foo2.DeepEquals(foo2Dec))

	// Decrypt barSer. Decrypt should be called.
	dec, completeDec = csm.BeginBatchDecryption()
	barDec, err = deserializeProperty(barSer, dec)
	assert.NoError(t, err, "deserialize")
	assert.NoError(t, completeDec(ctx), "complete")
	assert.Equal(t, 3, sm.batchDecryptCalls, "batch decrypt calls")
	assert.True(t, bar.DeepEquals(barDec))

	// Serialize the first copy of "foo" again. Encrypt should not be called, as this value has already been
	// cached by the earlier calls to Decrypt.
	enc, completeEnc = csm.BeginBatchEncryption()
	foo1Ser2, err = SerializePropertyValue(ctx, foo1Dec, enc, false /* showSecrets */)
	assert.NoError(t, err, "serialize")
	assert.NoError(t, completeEnc(ctx), "complete")
	assert.Equal(t, 3, sm.batchEncryptCalls, "batch encrypt calls")
	assert.Equal(t, foo1Ser, foo1Ser2)

	// Serialize the second copy of "foo" again. Encrypt should not be called, as this value has already been
	// cached by the earlier calls to Decrypt.
	enc, completeEnc = csm.BeginBatchEncryption()
	foo2Ser2, err = SerializePropertyValue(ctx, foo2Dec, enc, false /* showSecrets */)
	assert.NoError(t, err, "serialize")
	assert.NoError(t, completeEnc(ctx), "complete")
	assert.Equal(t, 3, sm.batchEncryptCalls, "batch encrypt calls")
	assert.Equal(t, foo2Ser, foo2Ser2)

	// Serialize "bar" again. Encrypt should not be called, as this value has already been cached by the
	// earlier calls to Decrypt.
	enc, completeEnc = csm.BeginBatchEncryption()
	barSer2, err = SerializePropertyValue(ctx, barDec, enc, false /* showSecrets */)
	assert.NoError(t, err, "serialize")
	assert.NoError(t, completeEnc(ctx), "complete")
	assert.Equal(t, 3, sm.batchEncryptCalls, "batch encrypt calls")
	assert.Equal(t, barSer, barSer2)
}

func TestSecretCache(t *testing.T) {
	t.Parallel()
	t.Run("empty cache", func(t *testing.T) {
		t.Parallel()
		cache := NewSecretCache()

		ciphertext, encrypted := cache.LookupCiphertext(&resource.Secret{}, "foo")
		plaintext, decrypted := cache.LookupPlaintext("ciphertext")

		assert.False(t, encrypted, "was encrypted")
		assert.Equal(t, "", ciphertext, "ciphertext value")
		assert.False(t, decrypted, "was decrypted")
		assert.Equal(t, "", plaintext, "plaintext value")
	})

	t.Run("cache hits", func(t *testing.T) {
		t.Parallel()
		cache := NewSecretCache()
		secret1 := &resource.Secret{}
		secret2 := &resource.Secret{}
		cache.Write("plaintext1", "ciphertext1", secret1)
		cache.Write("plaintext2", "ciphertext2", secret2)

		ciphertext1, encrypted1 := cache.LookupCiphertext(secret1, "plaintext1")
		plaintext1, decrypted1 := cache.LookupPlaintext("ciphertext1")
		assert.True(t, encrypted1, "was encrypted")
		assert.Equal(t, "ciphertext1", ciphertext1)
		assert.True(t, decrypted1, "was decrypted")
		assert.Equal(t, "plaintext1", plaintext1)

		ciphertext2, encrypted2 := cache.LookupCiphertext(secret2, "plaintext2")
		plaintext2, decrypted2 := cache.LookupPlaintext("ciphertext2")
		assert.True(t, encrypted2, "was encrypted")
		assert.Equal(t, "ciphertext2", ciphertext2)
		assert.True(t, decrypted2, "was decrypted")
		assert.Equal(t, "plaintext2", plaintext2)
	})

	t.Run("cache miss", func(t *testing.T) {
		t.Parallel()
		cache := NewSecretCache()
		secret := &resource.Secret{}
		cache.Write("plaintext", "ciphertext", secret)

		ciphertext, encrypted := cache.LookupCiphertext(secret, "different plaintext")
		plaintext, decrypted := cache.LookupPlaintext("different ciphertext")

		assert.False(t, encrypted, "was encrypted")
		assert.Equal(t, "", ciphertext, "ciphertext value")
		assert.False(t, decrypted, "was decrypted")
		assert.Equal(t, "", plaintext, "plaintext value")
	})

	t.Run("plaintext changed", func(t *testing.T) {
		t.Parallel()
		cache := NewSecretCache()
		secret := &resource.Secret{}

		cache.Write("plaintext", "ciphertext", secret)
		ciphertext, encrypted := cache.LookupCiphertext(secret, "different plaintext")
		plaintext, decrypted := cache.LookupPlaintext("ciphertext")

		assert.False(t, encrypted, "was encrypted")
		assert.Equal(t, "", ciphertext, "ciphertext value")
		assert.True(t, decrypted, "was decrypted")
		assert.Equal(t, "plaintext", plaintext, "original cached value")
	})

	t.Run("overwrite ciphertext", func(t *testing.T) {
		t.Parallel()
		cache := NewSecretCache()
		secret := &resource.Secret{}

		cache.Write("plaintext", "ciphertext", secret)
		cache.Write("plaintext", "updated ciphertext", secret)

		ciphertext, encrypted := cache.LookupCiphertext(secret, "plaintext")
		plaintext, decrypted := cache.LookupPlaintext("ciphertext")

		assert.True(t, encrypted, "was encrypted")
		assert.Equal(t, "updated ciphertext", ciphertext)
		assert.True(t, decrypted, "was decrypted")
		assert.Equal(t, "plaintext", plaintext)
	})

	t.Run("disable cache", func(t *testing.T) {
		t.Parallel()
		cache := secretCache{disableCache: true}
		secret := &resource.Secret{}

		cache.Write("plaintext", "ciphertext", secret)
		ciphertext, encrypted := cache.LookupCiphertext(secret, "plaintext")
		plaintext, decrypted := cache.LookupPlaintext("ciphertext")

		assert.False(t, encrypted, "was encrypted")
		assert.Equal(t, "", ciphertext, "ciphertext value")
		assert.False(t, decrypted, "was decrypted")
		assert.Equal(t, "", plaintext, "plaintext value")
	})
}

func TestBatchEncrypter(t *testing.T) {
	t.Parallel()
	t.Run("empty batch", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}

		_, complete := beginEncryptionBatch(sm.Encrypter(), NewSecretCache(), 999)
		assert.NoError(t, complete(ctx), "complete")

		assert.Equal(t, 0, sm.batchEncryptCalls)
	})

	t.Run("single batch", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}
		secret := &resource.Secret{}
		target := &apitype.SecretV1{}

		enc, complete := beginEncryptionBatch(sm.Encrypter(), NewSecretCache(), 999)
		assert.NoError(t, enc.Enqueue(ctx, secret, "plaintext", target), "enqueue")
		assert.NoError(t, complete(ctx), "complete")

		assert.Equal(t, &apitype.SecretV1{Ciphertext: "1-1:plaintext"}, target)
		assert.Equal(t, 1, sm.batchEncryptCalls)
	})

	t.Run("auto-send on max batch reached", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}
		secret := &resource.Secret{}
		target1 := &apitype.SecretV1{}
		target2 := &apitype.SecretV1{}

		enc, complete := beginEncryptionBatch(sm.Encrypter(), NewSecretCache(), 1)
		assert.NoError(t, enc.Enqueue(ctx, secret, "plaintext1", target1), "enqueue 1")
		assert.NoError(t, enc.Enqueue(ctx, secret, "plaintext2", target2), "enqueue 2")
		assert.Equal(t, 1, sm.batchEncryptCalls, "first batch auto-sent on limit reached")
		assert.Equal(t, &apitype.SecretV1{Ciphertext: "1-1:plaintext1"}, target1)

		assert.NoError(t, complete(ctx), "complete")
		assert.Equal(t, &apitype.SecretV1{Ciphertext: "2-1:plaintext2"}, target2)
		assert.Equal(t, 2, sm.batchEncryptCalls)
	})

	t.Run("leverages cache if whole batch cached", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}
		cache := NewSecretCache()
		secret := &resource.Secret{}
		target := &apitype.SecretV1{}

		cache.Write("plaintext", "ciphertext", secret)
		enc, complete := beginEncryptionBatch(sm.Encrypter(), cache, 999)
		assert.NoError(t, enc.Enqueue(ctx, secret, "plaintext", target), "enqueue")
		assert.NoError(t, complete(ctx), "complete")

		assert.Equal(t, &apitype.SecretV1{Ciphertext: "ciphertext"}, target)
		assert.Equal(t, 0, sm.batchEncryptCalls)
	})

	t.Run("bypass cache on partial miss", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}
		cache := NewSecretCache()
		secret1 := &resource.Secret{}
		secret2 := &resource.Secret{}
		target1 := &apitype.SecretV1{}
		target2 := &apitype.SecretV1{}

		cache.Write("0-1:plaintext", "ciphertext 1", secret1) // Add one value to the cache
		enc, complete := beginEncryptionBatch(sm.Encrypter(), cache, 999)
		assert.NoError(t, enc.Enqueue(ctx, secret1, "plaintext", target1), "enqueue 1")
		assert.NoError(t, enc.Enqueue(ctx, secret2, "plaintext", target2), "enqueue 2")
		assert.NoError(t, complete(ctx), "complete")

		assert.Equal(t, &apitype.SecretV1{Ciphertext: "1-1:plaintext"}, target1)
		assert.Equal(t, &apitype.SecretV1{Ciphertext: "1-2:plaintext"}, target2)
		assert.Equal(t, 1, sm.batchEncryptCalls)
	})

	t.Run("can't enqueue after complete", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}
		enc, complete := beginEncryptionBatch(sm.Encrypter(), NewSecretCache(), 999)
		assert.NoError(t, complete(ctx), "complete")

		assert.Panics(t, func() {
			err := enc.Enqueue(ctx, &resource.Secret{}, "plaintext", &apitype.SecretV1{})
			assert.Error(t, err) // Make lint happy
		}, "can't write to completed batch")
	})
}

func TestBatchDecrypter(t *testing.T) {
	t.Parallel()
	t.Run("empty batch", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}

		_, complete := beginDecryptionBatch(sm.Decrypter(), NewSecretCache(), secretPropertyValueFromPlaintext, 999)
		assert.NoError(t, complete(ctx), "complete")

		assert.Equal(t, 0, sm.batchDecryptCalls)
	})

	t.Run("single batch", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}
		secret := resource.MakeSecret(resource.NewNullProperty()).SecretValue()

		dec, complete := beginDecryptionBatch(sm.Decrypter(), NewSecretCache(), secretPropertyValueFromPlaintext, 999)
		assert.NoError(t, dec.Enqueue(ctx, "1-1:\"plaintext\"", secret), "enqueue")
		assert.NoError(t, complete(ctx), "complete")

		assert.Equal(t, resource.MakeSecret(resource.NewStringProperty("plaintext")).SecretValue(), secret)
		assert.Equal(t, 1, sm.batchDecryptCalls)
	})

	t.Run("auto-send on max batch reached", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}
		secret1 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()
		secret2 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()

		dec, complete := beginDecryptionBatch(sm.Decrypter(), NewSecretCache(), secretPropertyValueFromPlaintext, 1)
		assert.NoError(t, dec.Enqueue(ctx, "1-1:\"plaintext1\"", secret1), "enqueue 1")
		assert.NoError(t, dec.Enqueue(ctx, "2-1:\"plaintext2\"", secret2), "enqueue 2")
		assert.Equal(t, 1, sm.batchDecryptCalls, "first batch auto-sent on limit reached")
		assert.Equal(t, resource.MakeSecret(resource.NewStringProperty("plaintext1")).SecretValue(), secret1)

		assert.NoError(t, complete(ctx), "complete")
		assert.Equal(t, resource.MakeSecret(resource.NewStringProperty("plaintext2")).SecretValue(), secret2)
		assert.Equal(t, 2, sm.batchDecryptCalls)
	})

	t.Run("leverages cache if whole batch cached", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}
		cache := NewSecretCache()
		secret := resource.MakeSecret(resource.NewNullProperty()).SecretValue()

		const ciphertext = "1-1:\"ciphertext\""
		cache.Write("\"plaintext\"", ciphertext, &resource.Secret{}) // Pointer doesn't have to match for decryption
		dec, complete := beginDecryptionBatch(sm.Decrypter(), cache, secretPropertyValueFromPlaintext, 999)
		assert.NoError(t, dec.Enqueue(ctx, ciphertext, secret), "enqueue")

		assert.NoError(t, complete(ctx), "complete")
		assert.Equal(t, resource.MakeSecret(resource.NewStringProperty("plaintext")).SecretValue(), secret)
		assert.Equal(t, 0, sm.batchDecryptCalls)
	})

	t.Run("bypass cache on partial miss", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}
		cache := NewSecretCache()
		secret1 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()
		secret2 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()

		const ciphertext1 = "1:\"plaintext 1\""
		const ciphertext2 = "2:\"plaintext 2\""
		cache.Write("\"plaintext from cache\"", ciphertext1, &resource.Secret{})
		dec, complete := beginDecryptionBatch(sm.Decrypter(), cache, secretPropertyValueFromPlaintext, 999)
		assert.NoError(t, dec.Enqueue(ctx, ciphertext1, secret1), "enqueue 1")
		assert.NoError(t, dec.Enqueue(ctx, ciphertext2, secret2), "enqueue 2")

		assert.NoError(t, complete(ctx), "complete")
		assert.Equal(t, resource.MakeSecret(resource.NewStringProperty("plaintext 1")).SecretValue(), secret1)
		assert.Equal(t, resource.MakeSecret(resource.NewStringProperty("plaintext 2")).SecretValue(), secret2)
		assert.Equal(t, 1, sm.batchDecryptCalls)
	})

	t.Run("can't enqueue after complete", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{}
		dec, complete := beginDecryptionBatch(sm.Decrypter(), NewSecretCache(), secretPropertyValueFromPlaintext, 999)
		assert.NoError(t, complete(ctx), "complete")

		assert.Panics(t, func() {
			err := dec.Enqueue(ctx, "1-1:\"plaintext\"", resource.MakeSecret(resource.NewNullProperty()).SecretValue())
			assert.Error(t, err) // Make lint happy
		}, "can't write to completed batch")
	})
}
