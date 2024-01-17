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
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testSecretsManager struct {
	encryptCalls int
	decryptCalls int
}

func (t *testSecretsManager) Type() string { return "test" }

func (t *testSecretsManager) State() json.RawMessage { return nil }

func (t *testSecretsManager) Encrypter() (config.Encrypter, error) {
	return t, nil
}

func (t *testSecretsManager) Decrypter() (config.Decrypter, error) {
	return t, nil
}

func (t *testSecretsManager) EncryptValue(
	ctx context.Context, plaintext string,
) (string, error) {
	t.encryptCalls++
	return fmt.Sprintf("%v:%v", t.encryptCalls, plaintext), nil
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
) (map[string]string, error) {
	return config.DefaultBulkDecrypt(ctx, t, ciphertexts)
}

func deserializeProperty(v interface{}, dec config.Decrypter) (resource.PropertyValue, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return resource.PropertyValue{}, err
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return resource.PropertyValue{}, err
	}
	return DeserializePropertyValue(v, dec, config.NewPanicCrypter())
}

func TestCachingCrypter(t *testing.T) {
	t.Parallel()

	sm := &testSecretsManager{}
	csm := NewCachingSecretsManager(sm)

	foo1 := resource.MakeSecret(resource.NewStringProperty("foo"))
	foo2 := resource.MakeSecret(resource.NewStringProperty("foo"))
	bar := resource.MakeSecret(resource.NewStringProperty("bar"))

	enc, err := csm.Encrypter()
	assert.NoError(t, err)

	// Serialize the first copy of "foo". Encrypt should be called once, as this value has not yet been encrypted.
	foo1Ser, err := SerializePropertyValue(foo1, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 1, sm.encryptCalls)

	// Serialize the second copy of "foo". Because this is a different secret instance, Encrypt should be called
	// a second time even though the plaintext is the same as the last value we encrypted.
	foo2Ser, err := SerializePropertyValue(foo2, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 2, sm.encryptCalls)
	assert.NotEqual(t, foo1Ser, foo2Ser)

	// Serialize "bar". Encrypt should be called once, as this value has not yet been encrypted.
	barSer, err := SerializePropertyValue(bar, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)

	// Serialize the first copy of "foo" again. Encrypt should not be called, as this value has already been
	// encrypted.
	foo1Ser2, err := SerializePropertyValue(foo1, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, foo1Ser, foo1Ser2)

	// Serialize the second copy of "foo" again. Encrypt should not be called, as this value has already been
	// encrypted.
	foo2Ser2, err := SerializePropertyValue(foo2, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, foo2Ser, foo2Ser2)

	// Serialize "bar" again. Encrypt should not be called, as this value has already been encrypted.
	barSer2, err := SerializePropertyValue(bar, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, barSer, barSer2)

	dec, err := csm.Decrypter()
	assert.NoError(t, err)

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

	dec, err = csm.Decrypter()
	assert.NoError(t, err)

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

	enc, err = csm.Encrypter()
	assert.NoError(t, err)

	// Serialize the first copy of "foo" again. Encrypt should not be called, as this value has already been
	// cached by the earlier calls to Decrypt.
	foo1Ser2, err = SerializePropertyValue(foo1Dec, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, foo1Ser, foo1Ser2)

	// Serialize the second copy of "foo" again. Encrypt should not be called, as this value has already been
	// cached by the earlier calls to Decrypt.
	foo2Ser2, err = SerializePropertyValue(foo2Dec, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, foo2Ser, foo2Ser2)

	// Serialize "bar" again. Encrypt should not be called, as this value has already been cached by the
	// earlier calls to Decrypt.
	barSer2, err = SerializePropertyValue(barDec, enc, false /* showSecrets */)
	assert.NoError(t, err)
	assert.Equal(t, 3, sm.encryptCalls)
	assert.Equal(t, barSer, barSer2)
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

func (t *mapTestSecretsManager) Encrypter() (config.Encrypter, error) {
	return t.sm.Encrypter()
}

func (t *mapTestSecretsManager) Decrypter() (config.Decrypter, error) {
	d, err := t.sm.Decrypter()
	if err != nil {
		return nil, err
	}
	t.d = &mapTestDecrypter{d: d}
	return t.d, nil
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
) (map[string]string, error) {
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
