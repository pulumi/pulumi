// Copyright 2016-2022, Pulumi Corporation.
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
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/pkg/v3/secrets/service"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// DefaultSecretsProvider is the default SecretsProvider to use when deserializing deployments.
var DefaultSecretsProvider secrets.Provider = &defaultSecretsProvider{}

// defaultSecretsProvider implements the secrets.ManagerProviderFactory interface. Essentially
// it is the global location where new secrets managers can be registered for use when
// decrypting checkpoints.
type defaultSecretsProvider struct{}

// OfType returns a secrets manager for the given secrets type. Returns an error
// if the type is unknown or the state is invalid.
func (defaultSecretsProvider) OfType(ty string, state json.RawMessage) (secrets.Manager, error) {
	var sm secrets.Manager
	var err error
	switch ty {
	case passphrase.Type:
		sm, err = passphrase.NewPromptingPassphraseSecretsManagerFromState(state)
	case service.Type:
		sm, err = service.NewServiceSecretsManagerFromState(state)
	case cloud.Type:
		sm, err = cloud.NewCloudSecretsManagerFromState(state)
	default:
		return nil, fmt.Errorf("no known secrets provider for type %q", ty)
	}
	if err != nil {
		return nil, fmt.Errorf("constructing secrets manager of type %q: %w", ty, err)
	}

	return NewCachingSecretsManager(sm), nil
}

// NamedStackSecretsProvider is the same as the default secrets provider,
// but is aware of the stack name for which it is used.  Currently
// this is only used for prompting passphrase secrets managers to show
// the stackname in the prompt for the passphrase.
type NamedStackSecretsProvider struct {
	StackName string
}

// OfType returns a secrets manager for the given secrets type. Returns an error
// if the type is unknown or the state is invalid.
func (s NamedStackSecretsProvider) OfType(ty string, state json.RawMessage) (secrets.Manager, error) {
	var sm secrets.Manager
	var err error
	switch ty {
	case passphrase.Type:
		sm, err = passphrase.NewStackPromptingPassphraseSecretsManagerFromState(state, s.StackName)
	case service.Type:
		sm, err = service.NewServiceSecretsManagerFromState(state)
	case cloud.Type:
		sm, err = cloud.NewCloudSecretsManagerFromState(state)
	default:
		return nil, fmt.Errorf("no known secrets provider for type %q", ty)
	}
	if err != nil {
		return nil, fmt.Errorf("constructing secrets manager of type %q: %w", ty, err)
	}

	return NewCachingSecretsManager(sm), nil
}

// Process any pending encryptions that were enqueued.
type completeBulkOperation func(ctx context.Context) error

type cachingSecretsManager struct {
	manager secrets.Manager
	cache   *secretCache
}

// NewCachingSecretsManager returns a new secrets.Manager that caches the ciphertext for secret property values. A
// secrets.Manager that will be used to encrypt and decrypt values stored in a serialized deployment can be wrapped
// in a caching secrets manager in order to avoid re-encrypting secrets each time the deployment is serialized.
func NewCachingSecretsManager(manager secrets.Manager) secrets.Manager {
	sm := &cachingSecretsManager{
		manager: manager,
		cache:   &secretCache{},
	}
	return sm
}

// BeginBulkEncryption returns a bulk encrypter that wraps the internal encrypter and cache with a queue.
// This returns a bulk-compatible encrypter and a completion function that processes any pending encryption operations.
// Complete must be called to process any pending encryption operations added by the Enqueue method.
func (csm *cachingSecretsManager) BeginBulkEncryption(ctx context.Context) (config.Encrypter, completeBulkOperation) {
	internalEncrypter := csm.manager.Encrypter()
	bulkEncrypter := beginEncryptionBulk(internalEncrypter, csm.cache)
	return bulkEncrypter, bulkEncrypter.Complete
}

// BeginBulkDecryption returns a bulk decrypter that wraps the internal decrypter and cache with a queue.
// This returns a bulk-compatible decrypter and a completion function that processes any pending decryption operations.
// Complete must be called to process any pending decryption operations added by the Enqueue method.
func (csm *cachingSecretsManager) BeginBulkDecryption(ctx context.Context) (config.Decrypter, completeBulkOperation) {
	bulkDecrypter := beginDecryptionBulk(csm.manager.Decrypter(), csm.cache)
	return bulkDecrypter, bulkDecrypter.Complete
}

func (csm *cachingSecretsManager) Type() string {
	return csm.manager.Type()
}

func (csm *cachingSecretsManager) State() json.RawMessage {
	return csm.manager.State()
}

func (csm *cachingSecretsManager) Encrypter() config.Encrypter {
	return csm.manager.Encrypter()
}

func (csm *cachingSecretsManager) Decrypter() config.Decrypter {
	return csm.manager.Decrypter()
}

// The secretCache is a thread-safe cache for secret encryption and decryption results.
type secretCache struct {
	bySecret     sync.Map // Contains cacheEntry values, keyed by *resource.Secret
	byCiphertext sync.Map // Contains cacheEntry values, keyed by ciphertext string
}

type cacheEntry struct {
	plaintext  string
	ciphertext string
	secret     *resource.Secret
}

// Write stores the plaintext, ciphertext, and secret in the cache, overwriting any previous entry for the secret.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (c *secretCache) Write(plaintext, ciphertext string, secret *resource.Secret) {
	entry := cacheEntry{plaintext, ciphertext, secret}
	c.bySecret.Store(secret, entry)
	c.byCiphertext.Store(ciphertext, entry)
}

// TryEncrypt returns the cached ciphertext for the given secret and plaintext, if it exists.
// The ciphertext is returned as a string, and a boolean is returned to indicate whether the secret was found.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (c *secretCache) TryEncrypt(secret *resource.Secret, plaintext string) (string, bool) {
	entry, ok := c.bySecret.Load(secret)
	if !ok {
		return "", false
	}
	return entry.(cacheEntry).ciphertext, true
}

// TryDecrypt returns the cached plaintext for the given ciphertext, if it exists.
// The plaintext is returned as a string, and a boolean is returned to indicate whether the ciphertext was found.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (c *secretCache) TryDecrypt(ciphertext string) (string, bool) {
	entry, ok := c.byCiphertext.Load(ciphertext)
	if !ok {
		return "", false
	}
	return entry.(cacheEntry).plaintext, true
}

// bulkEncrypter is a wrapper around an Encrypter that queries the shared cache for previously encrypted secrets.
// It batches encryption operations to avoid round-trips for when the underlying encrypter requires a network call.
type bulkEncrypter struct {
	encrypter     config.Encrypter
	cache         *secretCache
	queue         chan queuedEncryption
	completeMutex sync.Mutex
}

type queuedEncryption struct {
	source    *resource.Secret
	target    *apitype.SecretV1
	plaintext string
}

// MaxBulkEncryptCount is the maximum number of items that can be enqueued for bulk encryption.
const MaxBulkEncryptCount = 100000

// Ensure that bulkEncrypter implements the Encrypter interface for compatibility.
var _ config.Encrypter = (*bulkEncrypter)(nil)

// beginEncryptionBulk returns a new bulkEncrypter that wraps the given encrypter and cache with a queue.
// Complete must be called to process any pending encryption operations added by the Enqueue method.
func beginEncryptionBulk(encrypter config.Encrypter, cache *secretCache) *bulkEncrypter {
	return &bulkEncrypter{
		encrypter: encrypter,
		cache:     cache,
		queue:     make(chan queuedEncryption, MaxBulkEncryptCount),
	}
}

// Enqueue adds a secret to the queue for encryption. If the cache has an entry for this secret and the plaintext has
// not changed, the previous ciphertext is used immediately. Otherwise, the secret is added to the queue.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (be *bulkEncrypter) Enqueue(ctx context.Context,
	source *resource.Secret, plaintext string, target *apitype.SecretV1,
) error {
	contract.Assertf(source != nil, "source secret must not be nil")
	// If the cache has an entry for this secret and the plaintext has not changed,
	// re-use the previous ciphertext for this specific secret instance.
	if ciphertext, ok := be.cache.TryEncrypt(source, plaintext); ok {
		target.Ciphertext = ciphertext
		return nil
	}
	// Add to the queue
	for {
		select {
		case be.queue <- queuedEncryption{source, target, plaintext}:
			return nil
		default:
			// If the queue is full, process the queue to make room.
			if err := be.Complete(ctx); err != nil {
				return err
			}
			// Now retry the enqueue.
		}
	}
}

// Complete processes any pending encryption operations in the queue.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (be *bulkEncrypter) Complete(ctx context.Context) error {
	if len(be.queue) == 0 {
		return nil
	}
	// Only send 1 batch at a time
	be.completeMutex.Lock()
	defer be.completeMutex.Unlock()

	// Flush the encrypt queue
	dequeued := make([]queuedEncryption, 0, len(be.queue))
	plaintexts := make([]string, 0, len(be.queue))
	// Take up to the maximum number of items from the queue.
	// Other items might be enqueued concurrently and will be sent in the next batch.
dequeue:
	for range MaxBulkEncryptCount {
		select {
		case q := <-be.queue:
			dequeued = append(dequeued, q)
			plaintexts = append(plaintexts, q.plaintext)
		default: // Queue is empty
			break dequeue
		}
	}

	ciphertexts, err := be.encrypter.BulkEncrypt(ctx, plaintexts)
	if err != nil {
		return err
	}
	for i, q := range dequeued {
		ciphertext := ciphertexts[i]
		q.target.Ciphertext = ciphertext
		be.cache.Write(q.plaintext, ciphertext, q.source)
	}
	return nil
}

func (be *bulkEncrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	return be.encrypter.EncryptValue(ctx, plaintext)
}

func (be *bulkEncrypter) BulkEncrypt(ctx context.Context, plaintexts []string) ([]string, error) {
	return be.encrypter.BulkEncrypt(ctx, plaintexts)
}

// bulkDecrypter is a wrapper around a Decrypter that queries the shared cache for previously decrypted secrets.
// It batches decryption operations to avoid round-trips for when the underlying decrypter requires a network call.
type bulkDecrypter struct {
	decrypter     config.Decrypter
	cache         *secretCache
	queue         chan queuedDecryption
	completeMutex sync.Mutex
}

type queuedDecryption struct {
	ciphertext string
	target     *resource.Secret
}

// MaxBulkDecryptCount is the maximum number of items that can be enqueued for bulk decryption.
const MaxBulkDecryptCount = 100000

// Ensure that bulkDecrypter implements the Decrypter interface for compatibility.
var _ config.Decrypter = (*bulkDecrypter)(nil)

// beginDecryptionBulk returns a new bulkDecrypter that wraps the given decrypter and cache with a queue.
// Complete must be called to process any pending decryption operations added by the Enqueue method.
func beginDecryptionBulk(decrypter config.Decrypter, cache *secretCache) *bulkDecrypter {
	return &bulkDecrypter{
		decrypter: decrypter,
		cache:     cache,
		queue:     make(chan queuedDecryption, MaxBulkDecryptCount),
	}
}

// Enqueue adds a ciphertext to the queue for decryption. If the cache has an entry for this ciphertext, the plaintext
// is used immediately. Otherwise, the ciphertext is added to the queue.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (bd *bulkDecrypter) Enqueue(ctx context.Context, ciphertext string, secret *resource.Secret) error {
	// Try the cache first.
	if plaintext, ok := bd.cache.TryDecrypt(ciphertext); ok {
		ev, err := secretPropertyValueFromPlaintext(plaintext)
		if err != nil {
			return err
		}
		secret.Element = ev
		return nil
	}
	// Add to the queue
	for {
		select {
		case bd.queue <- queuedDecryption{ciphertext, secret}:
			return nil
		default:
			// If the queue is full, process the queue to make room.
			if err := bd.Complete(ctx); err != nil {
				return err
			}
			// Now retry the enqueue.
		}
	}
}

// Complete processes any pending decryption operations in the queue.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (bd *bulkDecrypter) Complete(ctx context.Context) error {
	if len(bd.queue) == 0 {
		return nil
	}
	// Only send 1 batch at a time
	bd.completeMutex.Lock()
	defer bd.completeMutex.Unlock()

	// Flush the decrypt queue
	dequeued := make([]queuedDecryption, 0, len(bd.queue))
	ciphertexts := make([]string, 0, len(bd.queue))
	// Take up to the maximum number of items from the queue.
	// Other items might be enqueued concurrently and will be sent in the next batch.
dequeue:
	for range MaxBulkDecryptCount {
		select {
		case q := <-bd.queue:
			dequeued = append(dequeued, q)
			ciphertexts = append(ciphertexts, q.ciphertext)
		default: // Queue is empty
			break dequeue
		}
	}
	plaintexts, err := bd.decrypter.BulkDecrypt(ctx, ciphertexts)
	if err != nil {
		return err
	}
	for i, q := range dequeued {
		ev, err := secretPropertyValueFromPlaintext(plaintexts[i])
		if err != nil {
			return err
		}
		q.target.Element = ev
		bd.cache.Write(plaintexts[i], q.ciphertext, q.target)
	}
	return nil
}

func (bd *bulkDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	return bd.decrypter.DecryptValue(ctx, ciphertext)
}

func (bd *bulkDecrypter) BulkDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	return bd.decrypter.BulkDecrypt(ctx, ciphertexts)
}
