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
	"os"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testSecretsManager struct {
	encryptCalls         int
	decryptCalls         int
	enableBulkEncryption bool
	bulkEncryptCalls     int
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

func (t *testSecretsManager) SupportsBulkEncryption(ctx context.Context) bool {
	return t.enableBulkEncryption
}

func (t *testSecretsManager) BulkEncrypt(
	ctx context.Context, plaintexts []string,
) ([]string, error) {
	if !t.enableBulkEncryption {
		return nil, errors.New("bulk encryption not supported")
	}
	t.bulkEncryptCalls++
	if len(plaintexts) == 0 {
		return nil, nil
	}

	encrypted := make([]string, len(plaintexts))
	for i, plaintext := range plaintexts {
		encrypted[i] = fmt.Sprintf("%v-%v:%v", t.bulkEncryptCalls, i+1, plaintext)
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

func (t *testSecretsManager) BulkDecrypt(
	ctx context.Context, ciphertexts []string,
) ([]string, error) {
	return config.DefaultBulkDecrypt(ctx, t, ciphertexts)
}

func deserializeProperty(v interface{}, dec config.Decrypter) (resource.PropertyValue, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return resource.PropertyValue{}, err
	}
	var v2 interface{}
	if err := json.Unmarshal(b, &v2); err != nil {
		return resource.PropertyValue{}, err
	}
	return DeserializePropertyValue(v2, dec)
}

func TestCachingCrypter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sm := &testSecretsManager{}
	csm := NewCachingSecretsManager(sm)

	foo1 := resource.MakeSecret(resource.NewStringProperty("foo"))
	foo2 := resource.MakeSecret(resource.NewStringProperty("foo"))
	bar := resource.MakeSecret(resource.NewStringProperty("bar"))

	enc := csm.Encrypter()

	// Serialize the first copy of "foo". Encrypt should be called once, as this value has not yet been encrypted.
	foo1Ser, err := SerializePropertyValue(ctx, foo1, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 1, sm.encryptCalls)

	// Serialize the second copy of "foo". Because this is a different secret instance, Encrypt should be called
	// a second time even though the plaintext is the same as the last value we encrypted.
	foo2Ser, err := SerializePropertyValue(ctx, foo2, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 2, sm.encryptCalls)
	assert.NotEqual(t, foo1Ser, foo2Ser)

	// Serialize "bar". Encrypt should be called once, as this value has not yet been encrypted.
	barSer, err := SerializePropertyValue(ctx, bar, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)

	// Serialize the first copy of "foo" again. Encrypt should not be called, as this value has already been
	// encrypted.
	foo1Ser2, err := SerializePropertyValue(ctx, foo1, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, foo1Ser, foo1Ser2)

	// Serialize the second copy of "foo" again. Encrypt should not be called, as this value has already been
	// encrypted.
	foo2Ser2, err := SerializePropertyValue(ctx, foo2, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, foo2Ser, foo2Ser2)

	// Serialize "bar" again. Encrypt should not be called, as this value has already been encrypted.
	barSer2, err := SerializePropertyValue(ctx, bar, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, barSer, barSer2)

	dec := csm.Decrypter()

	// Decrypt foo1Ser. Decrypt should be called.
	foo1Dec, err := deserializeProperty(foo1Ser, dec)
	assert.NoError(t, err)
	assert.True(t, foo1.DeepEquals(foo1Dec))
	assert.Equal(t, 1, sm.decryptCalls)

	// Decrypt foo2Ser. Decrypt should be called.
	foo2Dec, err := deserializeProperty(foo2Ser, dec)
	assert.NoError(t, err)
	assert.True(t, foo2.DeepEquals(foo2Dec))
	assert.Equal(t, 2, sm.decryptCalls)

	// Decrypt barSer. Decrypt should be called.
	barDec, err := deserializeProperty(barSer, dec)
	assert.NoError(t, err)
	assert.True(t, bar.DeepEquals(barDec))
	assert.Equal(t, 3, sm.decryptCalls)

	// Create a new CachingSecretsManager and re-run the decrypts. Each decrypt should insert the plain- and
	// ciphertext into the cache with the associated secret.
	csm = NewCachingSecretsManager(sm)

	dec = csm.Decrypter()

	// Decrypt foo1Ser. Decrypt should be called.
	foo1Dec, err = deserializeProperty(foo1Ser, dec)
	assert.NoError(t, err)
	assert.True(t, foo1.DeepEquals(foo1Dec))
	assert.Equal(t, 4, sm.decryptCalls)

	// Decrypt foo2Ser. Decrypt should be called.
	foo2Dec, err = deserializeProperty(foo2Ser, dec)
	assert.NoError(t, err)
	assert.True(t, foo2.DeepEquals(foo2Dec))
	assert.Equal(t, 5, sm.decryptCalls)

	// Decrypt barSer. Decrypt should be called.
	barDec, err = deserializeProperty(barSer, dec)
	assert.NoError(t, err)
	assert.True(t, bar.DeepEquals(barDec))
	assert.Equal(t, 6, sm.decryptCalls)

	enc = csm.Encrypter()

	// Serialize the first copy of "foo" again. Encrypt should not be called, as this value has already been
	// cached by the earlier calls to Decrypt.
	foo1Ser2, err = SerializePropertyValue(ctx, foo1Dec, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, foo1Ser, foo1Ser2)

	// Serialize the second copy of "foo" again. Encrypt should not be called, as this value has already been
	// cached by the earlier calls to Decrypt.
	foo2Ser2, err = SerializePropertyValue(ctx, foo2Dec, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, foo2Ser, foo2Ser2)

	// Serialize "bar" again. Encrypt should not be called, as this value has already been cached by the
	// earlier calls to Decrypt.
	barSer2, err = SerializePropertyValue(ctx, barDec, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, barSer, barSer2)
}

func TestBulkEncrypt(t *testing.T) {
	t.Parallel()

	t.Run("without beginning bulk", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{enableBulkEncryption: true}
		csm := NewCachingSecretsManager(sm)
		enc := csm.Encrypter()

		source := resource.MakeSecret(resource.NewStringProperty("foo"))

		// Serialize the first copy of "foo". Encrypt should be called once, as this value has not yet been encrypted.
		_, err := SerializePropertyValue(ctx, source, enc, false /* showSecrets */)
		assert.NoError(t, err)
		assert.Equal(t, 1, sm.encryptCalls)
	})

	t.Run("delay via bulk", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{enableBulkEncryption: true}
		csm := NewCachingSecretsManager(sm)
		completeBulkOperation := csm.(*cachingSecretsManager).BeginBulkEncryption(ctx)
		enc := csm.Encrypter()

		foo1 := resource.MakeSecret(resource.NewStringProperty("foo"))
		foo2 := resource.MakeSecret(resource.NewStringProperty("foo"))

		// Serialize the first copy of "foo". Encrypt should be called once, as this value has not yet been encrypted.
		serializedFoo1, err := SerializePropertyValue(ctx, foo1, enc, false /* showSecrets */)
		assert.NoError(t, err)
		serializedFoo2, err := SerializePropertyValue(ctx, foo2, enc, false /* showSecrets */)
		assert.NoError(t, err)
		assert.Equal(t, 0, sm.encryptCalls) // Not yet completed bulk operation
		// Cyphertext not yet populated
		assert.Equal(t, "", serializedFoo1.(*apitype.SecretV1).Ciphertext)
		assert.Equal(t, "", serializedFoo2.(*apitype.SecretV1).Ciphertext)

		err = completeBulkOperation() // Complete the operation
		assert.NoError(t, err)
		assert.Equal(t, 0, sm.encryptCalls)
		assert.Equal(t, 1, sm.bulkEncryptCalls)
		assert.Equal(t, "1-1:\"foo\"", serializedFoo1.(*apitype.SecretV1).Ciphertext)
		assert.Equal(t, "1-2:\"foo\"", serializedFoo2.(*apitype.SecretV1).Ciphertext)
	})

	t.Run("partially cached bulk", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		sm := &testSecretsManager{enableBulkEncryption: true}
		csm := NewCachingSecretsManager(sm)
		completeBulkOperation := csm.(*cachingSecretsManager).BeginBulkEncryption(ctx)
		enc := csm.Encrypter()

		foo1 := resource.MakeSecret(resource.NewStringProperty("foo"))
		foo2 := resource.MakeSecret(resource.NewStringProperty("foo"))

		serialized, err := SerializePropertyValue(ctx, foo1, enc, false /* showSecrets */)
		assert.NoError(t, err)
		assert.Equal(t, 0, sm.encryptCalls)
		assert.Equal(t, "", serialized.(*apitype.SecretV1).Ciphertext, "not yet be populated")

		err = completeBulkOperation()
		assert.NoError(t, err)
		assert.Equal(t, 1, sm.bulkEncryptCalls)
		assert.Equal(t, "1-1:\"foo\"", serialized.(*apitype.SecretV1).Ciphertext, "now populated in bulk")

		completeBulkOperation = csm.(*cachingSecretsManager).BeginBulkEncryption(ctx)
		serializedFoo1, err := SerializePropertyValue(ctx, foo1, enc, false /* showSecrets */)
		assert.NoError(t, err)
		assert.Equal(t, 1, sm.bulkEncryptCalls, "new bulk not sent")
		assert.Equal(t, "1-1:\"foo\"", serializedFoo1.(*apitype.SecretV1).Ciphertext, "populated immediately from cache")

		serializedFoo2, err := SerializePropertyValue(ctx, foo2, enc, false /* showSecrets */)
		assert.NoError(t, err)
		assert.Equal(t, 1, sm.bulkEncryptCalls, "second bulk not yet sent")
		assert.Equal(t, "", serializedFoo2.(*apitype.SecretV1).Ciphertext, "not yet be populated")

		err = completeBulkOperation()
		assert.NoError(t, err)
		assert.Equal(t, 2, sm.bulkEncryptCalls, "second batch completed")
		assert.Equal(t, "2-1:\"foo\"", serializedFoo2.(*apitype.SecretV1).Ciphertext, "second ciphertext now populated")

		assert.Equal(t, 0, sm.encryptCalls, "only used bulk endpoint")
	})
}

func TestBulkDecrypt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sm := &testSecretsManager{}
	decrypter := sm.Decrypter()
	csm := newMapDecrypter(decrypter, map[string]string{})

	decrypted, err := csm.BulkDecrypt(ctx, []string{"1:foo", "2:bar", "3:baz"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar", "baz"}, decrypted)
	assert.Equal(t, 3, sm.decryptCalls)

	decryptedReordered, err := csm.BulkDecrypt(ctx, []string{"2:bar", "1:foo", "3:baz"}) // Re-ordered
	assert.NoError(t, err)
	assert.Equal(t, []string{"bar", "foo", "baz"}, decryptedReordered)
	assert.Equal(t, 3, sm.decryptCalls) // No additional calls made

	decrypted2, err := csm.BulkDecrypt(ctx, []string{"2:bar", "1:foo", "4:qux", "3:baz"}) // Add a new value
	assert.NoError(t, err)
	assert.Equal(t, []string{"bar", "foo", "qux", "baz"}, decrypted2)
	assert.Equal(t, 4, sm.decryptCalls) // Only 1 additional call made
}

type mapTestSecretsProvider struct {
	m *mapTestSecretsManager
}

func (p *mapTestSecretsProvider) OfType(ty string, state json.RawMessage) (secrets.Manager, error) {
	m, err := b64.Base64SecretsProvider.OfType(ty, state)
	if err != nil {
		return nil, err
	}
	p.m = &mapTestSecretsManager{sm: m}
	return p.m, nil
}

type mapTestSecretsManager struct {
	sm secrets.Manager

	d *mapTestDecrypter
}

func (t *mapTestSecretsManager) Type() string { return t.sm.Type() }

func (t *mapTestSecretsManager) State() json.RawMessage { return t.sm.State() }

func (t *mapTestSecretsManager) Encrypter() config.Encrypter {
	return t.sm.Encrypter()
}

func (t *mapTestSecretsManager) Decrypter() config.Decrypter {
	d := t.sm.Decrypter()
	t.d = &mapTestDecrypter{d: d}
	return t.d
}

type mapTestDecrypter struct {
	d config.Decrypter

	decryptCalls     int
	bulkDecryptCalls int
}

func (t *mapTestDecrypter) DecryptValue(
	ctx context.Context, ciphertext string,
) (string, error) {
	t.decryptCalls++
	return t.d.DecryptValue(ctx, ciphertext)
}

func (t *mapTestDecrypter) BulkDecrypt(
	ctx context.Context, ciphertexts []string,
) ([]string, error) {
	t.bulkDecryptCalls++
	return config.DefaultBulkDecrypt(ctx, t.d, ciphertexts)
}

func TestMapCrypter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	bytes, err := os.ReadFile("testdata/checkpoint-secrets.json")
	require.NoError(t, err)

	chk, err := UnmarshalVersionedCheckpointToLatestCheckpoint(encoding.JSON, bytes)
	require.NoError(t, err)

	var prov mapTestSecretsProvider

	_, err = DeserializeDeploymentV3(ctx, *chk.Latest, &prov)
	require.NoError(t, err)

	d := prov.m.d
	assert.Equal(t, 1, d.bulkDecryptCalls)
	assert.Equal(t, 0, d.decryptCalls)
}
