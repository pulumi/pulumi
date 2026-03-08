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

package backend

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests that an errorCatchingSecretsProvider correctly delegates its OfType method to the underlying provider.
func TestErrorCatchingSecretsProvider_OfType_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	delegate := &MockProviderManager{}
	provider := newErrorCatchingSecretsProvider(delegate, func(err error) error { return err })

	// Act
	manager, err := provider.OfType(t.Context(), "test", nil)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, manager)
	assert.IsType(t, &errorCatchingSecretsManager{}, manager)
	assert.True(t, delegate.ofTypeCalled)
}

// Tests that an errorCatchingSecretsProvider's OfType method returns a batching manager when the delegate manager
// supports batching.
func TestErrorCatchingSecretsProvider_Batching_OfType_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	delegate := &MockProviderManager{batching: true}
	provider := newErrorCatchingSecretsProvider(delegate, func(err error) error { return err })

	// Act
	manager, err := provider.OfType(t.Context(), "test", nil)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, manager)
	assert.Implements(t, (*stack.BatchingSecretsManager)(nil), manager)
	assert.True(t, delegate.ofTypeCalled)
}

// Tests that an errorCatchingSecretsProvider correctly propagates an error from the underlying provider's OfType
// method.
func TestErrorCatchingSecretsProvider_OfType_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	delegateErr := errors.New("delegate error")
	delegate := &MockProviderManager{ofTypeErr: delegateErr}

	provider := newErrorCatchingSecretsProvider(delegate, func(err error) error { return err })

	// Act
	manager, err := provider.OfType(t.Context(), "test", nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, manager)
	assert.Equal(t, delegateErr, err)
	assert.True(t, delegate.ofTypeCalled)
}

// Tests that an errorCatchingSecretsManager correctly delegates its Type method to the underlying provider.
func TestErrorCatchingSecretsManager_Type(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		expectedType := "testType"
		delegateManager := &MockProviderManager{batching: batching, typeStr: expectedType}

		manager := &errorCatchingSecretsManager{delegateManager: delegateManager}

		// Act
		result := manager.Type()

		// Assert
		assert.Equal(t, expectedType, result)
	})
}

// Tests that an errorCatchingSecretsManager correctly delegates its State method to the underlying provider.
func TestErrorCatchingSecretsManager_State(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		expectedState := json.RawMessage(`{"key": "value"}`)
		delegateManager := &MockProviderManager{batching: batching, state: expectedState}

		manager := &errorCatchingSecretsManager{delegateManager: delegateManager}

		// Act
		result := manager.State()

		// Assert
		assert.Equal(t, expectedState, result)
	})
}

// Tests that an errorCatchingSecretsManager is its own Encrypter.
func TestErrorCatchingSecretsManager_Encrypter(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		delegateManager := &MockProviderManager{batching: batching}
		manager := &errorCatchingSecretsManager{delegateManager: delegateManager}

		// Act
		encrypter := manager.Encrypter()

		// Assert
		assert.Equal(t, manager, encrypter)
	})
}

// Tests that an errorCatchingSecretsManager is its own Decrypter.
func TestErrorCatchingSecretsManager_Decrypter(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		delegateManager := &MockProviderManager{batching: batching}
		manager := &errorCatchingSecretsManager{delegateManager: delegateManager}

		// Act
		decrypter := manager.Decrypter()

		// Assert
		assert.Equal(t, manager, decrypter)
	})
}

// Tests that an errorCatchingSecretsManager's encrypter (itself) does not support encryption.
func TestErrorCatchingSecretsManager_EncryptValue(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		delegateManager := &MockProviderManager{batching: batching}
		manager := &errorCatchingSecretsManager{delegateManager: delegateManager}

		// Act
		_, err := manager.EncryptValue(context.Background(), "test")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not support encryption")
	})
}

// Tests that an errorCatchingSecretsManager's encrypter (itself) does not support batch encryption.
func TestErrorCatchingSecretsManager_BatchEncrypt(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		delegateManager := &MockProviderManager{batching: batching}
		manager := &errorCatchingSecretsManager{delegateManager: delegateManager}

		// Act
		_, err := manager.BatchEncrypt(context.Background(), []string{"test"})

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not support batch encryption")
	})
}

// Tests that an errorCatchingSecretsManager's decrypter (itself) supports decryption by delegating to the underlying
// provider.
func TestErrorCatchingSecretsManager_DecryptValue_Success(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		expectedValue := "decrypted"
		encDecrypter := &MockEncrypterDecrypter{decryptValue: expectedValue}

		delegateManager := &MockProviderManager{batching: batching, encrypterDecrypter: encDecrypter}

		manager := &errorCatchingSecretsManager{
			delegateManager: delegateManager,
			onDecryptError:  func(err error) error { return err },
		}

		// Act
		plaintext, err := manager.DecryptValue(context.Background(), "encrypted")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedValue, plaintext)
		assert.True(t, encDecrypter.decryptValueCalled)
	})
}

// Tests that an errorCatchingSecretsManager's decrypter (itself) propagates decryption errors from the underlying
// provider to the supplied onDecryptError callback, propagating the error to the caller if the callback returns an
// error.
func TestErrorCatchingSecretsManager_DecryptValue_ErrorPropagated(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		decryptErr := errors.New("decryption error")
		encDecrypter := &MockEncrypterDecrypter{decryptErr: decryptErr}

		delegateManager := &MockProviderManager{batching: batching, encrypterDecrypter: encDecrypter}

		onDecryptErrorCalled := false
		manager := &errorCatchingSecretsManager{
			delegateManager: delegateManager,
			onDecryptError: func(err error) error {
				onDecryptErrorCalled = true
				return err
			},
		}

		// Act
		_, err := manager.DecryptValue(context.Background(), "encrypted")

		// Assert
		assert.Error(t, err)
		assert.Equal(t, decryptErr, err)
		assert.True(t, encDecrypter.decryptValueCalled)
		assert.True(t, onDecryptErrorCalled)
	})
}

// Tests that an errorCatchingSecretsManager's decrypter (itself) propagates decryption errors from the underlying
// provider to the supplied onDecryptError callback, ignoring the error if the callback returns nil.
func TestErrorCatchingSecretsManager_DecryptValue_ErrorIgnored(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		encDecrypter := &MockEncrypterDecrypter{decryptErr: errors.New("decryption error")}
		delegateManager := &MockProviderManager{batching: batching, encrypterDecrypter: encDecrypter}

		onDecryptErrorCalled := false
		manager := &errorCatchingSecretsManager{
			delegateManager: delegateManager,
			onDecryptError: func(err error) error {
				onDecryptErrorCalled = true
				return nil
			},
		}

		// Act
		plaintext, err := manager.DecryptValue(context.Background(), "encrypted")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "{}", plaintext)
		assert.True(t, encDecrypter.decryptValueCalled)
		assert.True(t, onDecryptErrorCalled)
	})
}

// Tests that an errorCatchingSecretsManager's decrypter (itself) returns an error when decrypting if the underlying
// provider's decrypter is nil.
func TestErrorCatchingSecretsManager_DecryptValue_NilDecrypter(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		delegateManager := &MockProviderManager{batching: batching}
		manager := &errorCatchingSecretsManager{
			delegateManager: delegateManager,
			onDecryptError:  func(err error) error { return err },
		}

		// Act
		_, err := manager.DecryptValue(context.Background(), "encrypted")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil decrypter")
	})
}

// Tests that an errorCatchingSecretsManager's decrypter (itself) supports batch decryption by delegating to the
// underlying provider.
func TestErrorCatchingSecretsManager_BatchDecrypt_Success(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		expectedValues := []string{"decrypted1", "decrypted2"}
		encDecrypter := &MockEncrypterDecrypter{batchDecryptValues: expectedValues}
		delegateManager := &MockProviderManager{batching: batching, encrypterDecrypter: encDecrypter}

		manager := &errorCatchingSecretsManager{
			delegateManager: delegateManager,
			onDecryptError:  func(err error) error { return err },
		}

		// Act
		plaintexts, err := manager.BatchDecrypt(context.Background(), []string{"encrypted1", "encrypted2"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedValues, plaintexts)
		assert.True(t, encDecrypter.batchDecryptCalled)
	})
}

// Tests that an errorCatchingSecretsManager supports BatchingSecretsManager when the underlying provider does.
func TestErrorCatchingSecretsManager_Batching_Enqueue_Success(t *testing.T) {
	t.Parallel()

	expectedValues := []string{"\"decrypted1\"", "\"decrypted2\""}
	encDecrypter := &MockEncrypterDecrypter{batchDecryptValues: expectedValues}
	delegateManager := &MockProviderManager{batching: true, encrypterDecrypter: encDecrypter}

	provider := newErrorCatchingSecretsProvider(delegateManager, func(err error) error { return err })
	manager, err := provider.OfType(t.Context(), "test", nil)
	require.NoError(t, err)

	batchingManager, ok := manager.(stack.BatchingSecretsManager)
	require.True(t, ok)
	batchDecrypter, completeCrypterBatch := batchingManager.BeginBatchDecryption()

	secret1 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()
	secret2 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()

	err = batchDecrypter.Enqueue(t.Context(), "encrypted1", secret1)
	require.NoError(t, err)
	err = batchDecrypter.Enqueue(t.Context(), "encrypted2", secret2)
	require.NoError(t, err)

	err = completeCrypterBatch(t.Context())
	require.NoError(t, err)

	assert.Equal(t, resource.MakeSecret(resource.NewPropertyValue("decrypted1")).SecretValue(), secret1)
	assert.Equal(t, resource.MakeSecret(resource.NewPropertyValue("decrypted2")).SecretValue(), secret2)
	assert.True(t, encDecrypter.batchDecryptCalled)
}

// Tests that an errorCatchingSecretsManager's decrypter (itself) propagates batch decryption errors from the underlying
// provider to the supplied onDecryptError callback, propagating the error to the caller if the callback returns an
// error.
func TestErrorCatchingSecretsManager_BatchDecrypt_ErrorPropagated(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		batchDecryptErr := errors.New("batch decryption error")
		encDecrypter := &MockEncrypterDecrypter{batchDecryptErr: batchDecryptErr}
		delegateManager := &MockProviderManager{batching: batching, encrypterDecrypter: encDecrypter}

		onDecryptErrorCalled := false
		manager := &errorCatchingSecretsManager{
			delegateManager: delegateManager,
			onDecryptError: func(err error) error {
				onDecryptErrorCalled = true
				return err
			},
		}

		// Act
		_, err := manager.BatchDecrypt(context.Background(), []string{"encrypted1", "encrypted2"})

		// Assert
		assert.Error(t, err)
		assert.Equal(t, batchDecryptErr, err)
		assert.True(t, encDecrypter.batchDecryptCalled)
		assert.True(t, onDecryptErrorCalled)
	})
}

// Tests that an errorCatchingSecretsManager's decrypter (itself) propagates batch decryption errors from the underlying
// provider to the supplied onDecryptError callback, propagating the error to the caller if the callback returns an
// error.
func TestErrorCatchingSecretsManager_Batching_ErrorPropagated(t *testing.T) {
	t.Parallel()

	batchDecryptErr := errors.New("batch decryption error")
	encDecrypter := &MockEncrypterDecrypter{batchDecryptErr: batchDecryptErr}
	delegateManager := &MockProviderManager{batching: true, encrypterDecrypter: encDecrypter}

	onDecryptErrorCalled := false
	provider := newErrorCatchingSecretsProvider(delegateManager, func(err error) error {
		onDecryptErrorCalled = true
		return err
	})
	manager, err := provider.OfType(t.Context(), "test", nil)
	require.NoError(t, err)

	batchingManager, ok := manager.(stack.BatchingSecretsManager)
	require.True(t, ok)
	batchDecrypter, completeCrypterBatch := batchingManager.BeginBatchDecryption()

	secret1 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()
	secret2 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()

	err = batchDecrypter.Enqueue(t.Context(), "encrypted1", secret1)
	require.NoError(t, err)
	err = batchDecrypter.Enqueue(t.Context(), "encrypted2", secret2)
	require.NoError(t, err)

	err = completeCrypterBatch(t.Context())

	assert.Error(t, err)
	assert.Equal(t, batchDecryptErr, err)
	assert.True(t, encDecrypter.batchDecryptCalled)
	assert.True(t, onDecryptErrorCalled)
}

// Tests that an errorCatchingSecretsManager's decrypter (itself) propagates batch decryption errors from the underlying
// provider to the supplied onDecryptError callback, ignoring the error if the callback returns nil.
func TestErrorCatchingSecretsManager_BatchDecrypt_ErrorIgnored(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		encDecrypter := &MockEncrypterDecrypter{batchDecryptErr: errors.New("batch decryption error")}
		delegateManager := &MockProviderManager{batching: batching, encrypterDecrypter: encDecrypter}

		onDecryptErrorCalled := false
		manager := &errorCatchingSecretsManager{
			delegateManager: delegateManager,
			onDecryptError: func(err error) error {
				onDecryptErrorCalled = true
				return nil
			},
		}

		// Act
		plaintexts, err := manager.BatchDecrypt(context.Background(), []string{"encrypted1", "encrypted2"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"{}", "{}"}, plaintexts)
		assert.True(t, encDecrypter.batchDecryptCalled)
		assert.True(t, onDecryptErrorCalled)
	})
}

// Tests that an errorCatchingSecretsManager's decrypter (itself) propagates batch decryption errors from the underlying
// provider to the supplied onDecryptError callback, ignoring the error if the callback returns nil.
func TestErrorCatchingSecretsManager_Batching_ErrorIgnored(t *testing.T) {
	t.Parallel()

	batchDecryptErr := errors.New("batch decryption error")
	encDecrypter := &MockEncrypterDecrypter{batchDecryptErr: batchDecryptErr}
	delegateManager := &MockProviderManager{batching: true, encrypterDecrypter: encDecrypter}

	onDecryptErrorCalled := false
	provider := newErrorCatchingSecretsProvider(delegateManager, func(err error) error {
		onDecryptErrorCalled = true
		return nil
	})
	manager, err := provider.OfType(t.Context(), "test", nil)
	require.NoError(t, err)

	batchingManager, ok := manager.(stack.BatchingSecretsManager)
	require.True(t, ok)
	batchDecrypter, completeCrypterBatch := batchingManager.BeginBatchDecryption()

	secret1 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()
	secret2 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()

	err = batchDecrypter.Enqueue(t.Context(), "encrypted1", secret1)
	require.NoError(t, err)
	err = batchDecrypter.Enqueue(t.Context(), "encrypted2", secret2)
	require.NoError(t, err)

	err = completeCrypterBatch(t.Context())
	require.NoError(t, err)

	assert.Equal(t, resource.MakeSecret(resource.NewProperty(resource.PropertyMap{})).SecretValue(), secret1)
	assert.Equal(t, resource.MakeSecret(resource.NewProperty(resource.PropertyMap{})).SecretValue(), secret2)
	assert.True(t, encDecrypter.batchDecryptCalled)
	assert.True(t, onDecryptErrorCalled)
}

// Tests that an errorCatchingSecretsManager's decrypter (itself) returns an error when batch decrypting if the
// underlying provider's decrypter is nil.
func TestErrorCatchingSecretsManager_BatchDecrypt_NilDecrypter(t *testing.T) {
	t.Parallel()

	testNonBatchingAndBatching(t, func(t *testing.T, batching bool) {
		// Arrange
		delegateManager := &MockProviderManager{batching: batching, encrypterDecrypter: nil}
		manager := &errorCatchingSecretsManager{
			delegateManager: delegateManager,
			onDecryptError:  func(err error) error { return err },
		}

		// Act
		_, err := manager.BatchDecrypt(context.Background(), []string{"encrypted1", "encrypted2"})

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil decrypter")
	})
}

// Tests that an errorCatchingSecretsManager's decrypter (itself) returns an error when batch decrypting if the
// underlying provider's decrypter is nil.
func TestErrorCatchingSecretsManager_Batching_NilDecrypter(t *testing.T) {
	t.Parallel()

	delegateManager := &MockProviderManager{batching: true, encrypterDecrypter: nil}

	provider := newErrorCatchingSecretsProvider(delegateManager, func(err error) error { return err })
	manager, err := provider.OfType(t.Context(), "test", nil)
	require.NoError(t, err)

	batchingManager, ok := manager.(stack.BatchingSecretsManager)
	require.True(t, ok)
	batchDecrypter, completeCrypterBatch := batchingManager.BeginBatchDecryption()

	secret1 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()
	secret2 := resource.MakeSecret(resource.NewNullProperty()).SecretValue()

	err = batchDecrypter.Enqueue(t.Context(), "encrypted1", secret1)
	require.NoError(t, err)
	err = batchDecrypter.Enqueue(t.Context(), "encrypted2", secret2)
	require.NoError(t, err)

	err = completeCrypterBatch(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil decrypter")
}

func testNonBatchingAndBatching(t *testing.T, testFunc func(t *testing.T, batching bool)) {
	t.Run("NonBatching", func(t *testing.T) {
		t.Parallel()
		testFunc(t, false)
	})
	t.Run("Batching", func(t *testing.T) {
		t.Parallel()
		testFunc(t, true)
	})
}

// MockProviderManager implements both secrets.Manager and secrets.Provider interfaces for testing.
type MockProviderManager struct {
	typeStr            string
	state              json.RawMessage
	encrypterDecrypter *MockEncrypterDecrypter
	ofTypeCalled       bool
	ofTypeErr          error
	batching           bool
}

func (m *MockProviderManager) Type() string {
	return m.typeStr
}

func (m *MockProviderManager) State() json.RawMessage {
	return m.state
}

func (m *MockProviderManager) Encrypter() config.Encrypter {
	// If m.encrypterDecrypter is nil, we return an explicit nil value to avoid falling foul of Go's "nil interface" pain,
	// whereby a caller will receive a nil interface value, which does not equal nil but will panic if used.
	if m.encrypterDecrypter != nil {
		return m.encrypterDecrypter
	}

	return nil
}

func (m *MockProviderManager) Decrypter() config.Decrypter {
	// If m.encrypterDecrypter is nil, we return an explicit nil value to avoid falling foul of Go's "nil interface" pain,
	// whereby a caller will receive a nil interface value, which does not equal nil but will panic if used.
	if m.encrypterDecrypter != nil {
		return m.encrypterDecrypter
	}

	return nil
}

func (m *MockProviderManager) OfType(_ context.Context, ty string, state json.RawMessage) (secrets.Manager, error) {
	m.ofTypeCalled = true
	if m.ofTypeErr != nil {
		return nil, m.ofTypeErr
	}

	if m.batching {
		return stack.NewBatchingCachingSecretsManager(m), nil
	}

	return m, nil
}

// MockEncrypterDecrypter combines the config.Encrypter and config.Decrypter interfaces for testing.
type MockEncrypterDecrypter struct {
	encryptValueCalled bool
	encryptValueErr    error
	batchEncryptCalled bool
	batchEncryptErr    error
	decryptValueCalled bool
	decryptErr         error
	decryptValue       string
	batchDecryptCalled bool
	batchDecryptErr    error
	batchDecryptValues []string
}

func (m *MockEncrypterDecrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	m.encryptValueCalled = true
	return "encrypted", m.encryptValueErr
}

func (m *MockEncrypterDecrypter) BatchEncrypt(ctx context.Context, secrets []string) ([]string, error) {
	m.batchEncryptCalled = true
	result := make([]string, len(secrets))
	for i := range secrets {
		result[i] = "encrypted" + string(rune(i+'0'))
	}
	return result, m.batchEncryptErr
}

func (m *MockEncrypterDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	m.decryptValueCalled = true
	return m.decryptValue, m.decryptErr
}

func (m *MockEncrypterDecrypter) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	m.batchDecryptCalled = true
	return m.batchDecryptValues, m.batchDecryptErr
}
