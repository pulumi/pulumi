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
	"sync/atomic"

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

type CachingSecretsManager interface {
	secrets.Manager
	BeginBatchEncryption() (BatchEncrypter, CompleteCrypterBatch)
	BeginBatchDecryption() (BatchDecrypter, CompleteCrypterBatch)
}

type cachingSecretsManager struct {
	manager secrets.Manager
	cache   SecretCache
}

// NewCachingSecretsManager returns a new secrets.Manager that caches the ciphertext for secret property values. A
// secrets.Manager that will be used to encrypt and decrypt values stored in a serialized deployment can be wrapped
// in a caching secrets manager in order to avoid re-encrypting secrets each time the deployment is serialized.
func NewCachingSecretsManager(manager secrets.Manager) CachingSecretsManager {
	sm := &cachingSecretsManager{
		manager: manager,
		cache:   NewSecretCache(),
	}
	return sm
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

func (csm *cachingSecretsManager) BeginBatchEncryption() (BatchEncrypter, CompleteCrypterBatch) {
	return BeginEncryptionBatchWithCache(csm.manager.Encrypter(), csm.cache)
}

func (csm *cachingSecretsManager) BeginBatchDecryption() (BatchDecrypter, CompleteCrypterBatch) {
	return BeginDecryptionBatchWithCache(csm.manager.Decrypter(), csm.cache)
}

// SecretCache allows the bidirectional cached conversion between: `ciphertext <-> plaintext + secret pointer`.
// The same plaintext can be associated with multiple secrets, each of which will have their own ciphertexts which
// should not be shared.
//
//	cache := NewSecretCache()
//	secret := &resource.Secret{}
//	cache.Write("plaintext", "ciphertext", secret)
//	plaintext, ok := cache.TryDecrypt("ciphertext") // "plaintext", true
//	ciphertext, ok := cache.TryEncrypt(secret, "plaintext") // "ciphertext", true
type SecretCache interface {
	// Write stores the plaintext, ciphertext, and secret in the cache, overwriting any previous entry for the secret.
	Write(plaintext, ciphertext string, secret *resource.Secret)
	// TryEncrypt returns the cached ciphertext for the given secret and plaintext, if it exists.
	TryEncrypt(secret *resource.Secret, plaintext string) (string, bool)
	// TryDecrypt returns the cached plaintext for the given ciphertext, if it exists.
	TryDecrypt(ciphertext string) (string, bool)
}

type secretCache struct {
	bySecret     sync.Map
	byCiphertext sync.Map
}

type secretCacheEntry struct {
	plaintext  string
	ciphertext string
	secret     *resource.Secret
}

// NewSecretCache returns a new secretCache which allows the bidirectional cached conversion between:
// `ciphertext <-> plaintext + secret pointer`. The same plaintext can be associated with multiple secrets, each of
// which will have their own ciphertexts which should not be shared.
//
// All methods are thread-safe and can be called concurrently by multiple goroutines.
//
//	cache := NewSecretCache()
//	secret := &resource.Secret{}
//	cache.Write("plaintext", "ciphertext", secret)
//	plaintext, ok := cache.TryDecrypt("ciphertext") // "plaintext", true
//	ciphertext, ok := cache.TryEncrypt(secret, "plaintext") // "ciphertext", true
func NewSecretCache() SecretCache {
	return &secretCache{}
}

// Write stores the plaintext, ciphertext, and secret in the cache, overwriting any previous entry for the secret.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (c *secretCache) Write(plaintext, ciphertext string, secret *resource.Secret) {
	entry := secretCacheEntry{plaintext, ciphertext, secret}
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
	cacheEntry := entry.(secretCacheEntry)
	if cacheEntry.plaintext != plaintext {
		return "", false
	}
	return cacheEntry.ciphertext, true
}

// TryDecrypt returns the cached plaintext for the given ciphertext, if it exists.
// The plaintext is returned as a string, and a boolean is returned to indicate whether the ciphertext was found.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (c *secretCache) TryDecrypt(ciphertext string) (string, bool) {
	entry, ok := c.byCiphertext.Load(ciphertext)
	if !ok {
		return "", false
	}
	return entry.(secretCacheEntry).plaintext, true
}

// BatchEncrypter is a wrapper around an Encrypter that allows encryption operations to be batched to avoid round-trips
// for when the underlying encrypter requires a network call. Encryptions to a secret are enqueued and processed in a
// batch request either when the queue is full or when the wrapping transaction is committed.
type BatchEncrypter interface {
	config.Encrypter

	// Enqueue adds a secret to the queue for encryption. If the cache has an entry for this secret and the plaintext has
	// not changed, the previous ciphertext is used immediately. Otherwise, the secret is added to the queue.
	// This method is thread-safe and can be called concurrently by multiple goroutines.
	Enqueue(ctx context.Context, source *resource.Secret, plaintext string, target *apitype.SecretV1) error
}

// CompleteCrypterBatch is a function that must be called to ensure that all enqueued crypter operations are processed.
type CompleteCrypterBatch func(context.Context) error

type cachingBatchEncrypter struct {
	encrypter     config.Encrypter
	cache         SecretCache
	queue         chan queuedEncryption
	closed        atomic.Bool
	completeMutex sync.Mutex
	maxBatchSize  int
}

type queuedEncryption struct {
	source    *resource.Secret
	target    *apitype.SecretV1
	plaintext string
}

// DefaultMaxBatchEncryptCount is the default maximum number of items that can be enqueued for batch encryption.
const DefaultMaxBatchEncryptCount = 100000

// Ensure that cachingBatchEncrypter implements the BatchEncrypter interface for compatibility.
var _ BatchEncrypter = (*cachingBatchEncrypter)(nil)

// BeginEncryptionBatch returns a new BatchEncrypter and CompleteCrypterBatch function.
// The BatchEncrypter allows encryption operations to be batched to avoid round-trips for when the underlying encrypter
// requires a network call. Encryptions to a secret are enqueued and processed in a batch request either when the queue
// is full or when the wrapping transaction is committed.
//
// The CompleteCrypterBatch function must be called to ensure that all enqueued encryption operations are processed.
//
//	batchEncrypter, completeCrypterBatch := BeginEncryptionBatch(encrypter)
//	defer completeCrypterBatch(ctx)
//	SerializeSecrets(ctx, batchEncrypter, secrets)
func BeginEncryptionBatch(encrypter config.Encrypter) (BatchEncrypter, CompleteCrypterBatch) {
	return BeginEncryptionBatchWithCache(encrypter, NewSecretCache())
}

// BeginEncryptionBatchWithCache returns a new BatchEncrypter and CompleteCrypterBatch function with a custom cache.
// The BatchEncrypter allows encryption operations to be batched to avoid round-trips for when the underlying encrypter
// requires a network call. Encryptions to a secret are enqueued and processed in a batch request either when the queue
// is full or when the wrapping transaction is committed. If the cache has an entry for a secret and the plaintext has
// not changed, the previous ciphertext is used immediately and not enqueued for the batch operation. Results are
// also written to the provided cache.
//
// The CompleteCrypterBatch function must be called to ensure that all enqueued encryption operations are processed.
//
//	batchEncrypter, completeCrypterBatch := BeginEncryptionBatch(encrypter)
//	defer completeCrypterBatch(ctx)
//	SerializeSecrets(ctx, batchEncrypter, secrets)
func BeginEncryptionBatchWithCache(
	encrypter config.Encrypter, cache SecretCache,
) (BatchEncrypter, CompleteCrypterBatch) {
	return beginEncryptionBatch(encrypter, cache, DefaultMaxBatchEncryptCount)
}

func beginEncryptionBatch(
	encrypter config.Encrypter, cache SecretCache, maxBatchSize int,
) (BatchEncrypter, CompleteCrypterBatch) {
	contract.Assertf(encrypter != nil, "encrypter must not be nil")
	contract.Assertf(cache != nil, "cache must not be nil")
	contract.Assertf(maxBatchSize > 0, "maxBatchSize must be greater than 0")
	batchEncrypter := &cachingBatchEncrypter{
		encrypter:    encrypter,
		cache:        cache,
		queue:        make(chan queuedEncryption, maxBatchSize),
		maxBatchSize: maxBatchSize,
	}
	return batchEncrypter, func(ctx context.Context) error {
		wasClosed := batchEncrypter.closed.Swap(true)
		contract.Assertf(!wasClosed, "batch encrypter already completed")
		return batchEncrypter.sendNextBatch(ctx)
	}
}

func (be *cachingBatchEncrypter) Enqueue(ctx context.Context,
	source *resource.Secret, plaintext string, target *apitype.SecretV1,
) error {
	contract.Assertf(source != nil, "source secret must not be nil")
	contract.Assertf(!be.closed.Load(), "batch encrypter must not be closed")
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
			if err := be.sendNextBatch(ctx); err != nil {
				return err
			}
			// Now retry the enqueue.
		}
	}
}

// sendNextBatch processes any pending encryption operations in the queue.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (be *cachingBatchEncrypter) sendNextBatch(ctx context.Context) error {
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
	for range be.maxBatchSize {
		select {
		case q := <-be.queue:
			dequeued = append(dequeued, q)
			plaintexts = append(plaintexts, q.plaintext)
		default: // Queue is empty
			break dequeue
		}
	}

	ciphertexts, err := be.encrypter.BatchEncrypt(ctx, plaintexts)
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

func (be *cachingBatchEncrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	return be.encrypter.EncryptValue(ctx, plaintext)
}

func (be *cachingBatchEncrypter) BatchEncrypt(ctx context.Context, plaintexts []string) ([]string, error) {
	return be.encrypter.BatchEncrypt(ctx, plaintexts)
}

// BatchDecrypter is a wrapper around a Decrypter that allows decryption operations to be batched to avoid round-trips
// for when the underlying decrypter requires a network call. Decryptions to a secret are enqueued and processed in a
// batch request either when the queue is full or when the wrapping transaction is committed.
type BatchDecrypter interface {
	config.Decrypter

	// Enqueue adds a ciphertext to the queue for decryption. If the cache has an entry for this ciphertext, the
	// plaintext is used immediately. Otherwise, the ciphertext is added to the queue.
	// This method is thread-safe and can be called concurrently by multiple goroutines.
	Enqueue(ctx context.Context, ciphertext string, target *resource.Secret) error
}

type cachingBatchDecrypter struct {
	decrypter                      config.Decrypter
	cache                          SecretCache
	deserializeSecretPropertyValue DeserializeSecretPropertyValue
	queue                          chan queuedDecryption
	closed                         atomic.Bool
	completeMutex                  sync.Mutex
	maxBatchSize                   int
}

type queuedDecryption struct {
	target     *resource.Secret
	ciphertext string
}

const DefaultMaxBatchDecryptCount = 100000

// Ensure that cachingBatchDecrypter implements the BatchDecrypter interface for compatibility.
var _ BatchDecrypter = (*cachingBatchDecrypter)(nil)

type DeserializeSecretPropertyValue func(plaintext string) (resource.PropertyValue, error)

func BeginDecryptionBatch(decrypter config.Decrypter) (BatchDecrypter, CompleteCrypterBatch) {
	return BeginDecryptionBatchWithCache(decrypter, NewSecretCache())
}

func BeginDecryptionBatchWithCache(
	decrypter config.Decrypter, cache SecretCache,
) (BatchDecrypter, CompleteCrypterBatch) {
	return beginDecryptionBatch(decrypter, cache, secretPropertyValueFromPlaintext, DefaultMaxBatchDecryptCount)
}

func beginDecryptionBatch(decrypter config.Decrypter, cache SecretCache,
	secretPropertyValueFromPlaintext DeserializeSecretPropertyValue, maxBatchSize int,
) (BatchDecrypter, CompleteCrypterBatch) {
	contract.Assertf(decrypter != nil, "decrypter must not be nil")
	contract.Assertf(cache != nil, "cache must not be nil")
	contract.Assertf(maxBatchSize > 0, "maxBatchSize must be greater than 0")
	batchDecrypter := &cachingBatchDecrypter{
		decrypter:                      decrypter,
		cache:                          cache,
		deserializeSecretPropertyValue: secretPropertyValueFromPlaintext,
		queue:                          make(chan queuedDecryption, maxBatchSize),
		maxBatchSize:                   maxBatchSize,
	}
	return batchDecrypter, func(ctx context.Context) error {
		wasClosed := batchDecrypter.closed.Swap(true)
		contract.Assertf(!wasClosed, "batch decrypter already completed")
		return batchDecrypter.sendNextBatch(ctx)
	}
}

func (bd *cachingBatchDecrypter) Enqueue(ctx context.Context, ciphertext string, target *resource.Secret) error {
	contract.Assertf(target != nil, "target secret must not be nil")
	contract.Assertf(!bd.closed.Load(), "batch decrypter must not be closed")
	// If the cache has an entry for this ciphertext, re-use the previous plaintext for this specific secret instance.
	if plaintext, ok := bd.cache.TryDecrypt(ciphertext); ok {
		propertyValue, err := bd.deserializeSecretPropertyValue(plaintext)
		if err != nil {
			return err
		}
		target.Element = propertyValue
		return nil
	}
	// Add to the queue
	for {
		select {
		case bd.queue <- queuedDecryption{target, ciphertext}:
			return nil
		default:
			// If the queue is full, process the queue to make room.
			if err := bd.sendNextBatch(ctx); err != nil {
				return err
			}
			// Now retry the enqueue.
		}
	}
}

// sendNextBatch processes any pending decryption operations in the queue.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (bd *cachingBatchDecrypter) sendNextBatch(ctx context.Context) error {
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
	for range bd.maxBatchSize {
		select {
		case q := <-bd.queue:
			dequeued = append(dequeued, q)
			ciphertexts = append(ciphertexts, q.ciphertext)
		default: // Queue is empty
			break dequeue
		}
	}

	plaintexts, err := bd.decrypter.BatchDecrypt(ctx, ciphertexts)
	if err != nil {
		return err
	}
	for i, q := range dequeued {
		plaintext := plaintexts[i]
		propertyValue, err := bd.deserializeSecretPropertyValue(plaintext)
		if err != nil {
			return err
		}
		q.target.Element = propertyValue
		bd.cache.Write(plaintext, q.ciphertext, q.target)
	}
	return nil
}

func (bd *cachingBatchDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	return bd.decrypter.DecryptValue(ctx, ciphertext)
}

func (bd *cachingBatchDecrypter) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	return bd.decrypter.BatchDecrypt(ctx, ciphertexts)
}
