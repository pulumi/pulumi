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
	"github.com/pulumi/pulumi/sdk/v3/go/common/lazy"
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

type cachingSecretsManager struct {
	manager   secrets.Manager
	encrypter lazy.Lazy[config.Encrypter]
	decrypter lazy.Lazy[config.Decrypter]
	cache     SecretCache
}

// NewCachingSecretsManager returns a new secrets.Manager that caches the ciphertext for secret property values. A
// secrets.Manager that will be used to encrypt and decrypt values stored in a serialized deployment can be wrapped
// in a caching secrets manager in order to avoid re-encrypting secrets each time the deployment is serialized.
func NewCachingSecretsManager(manager secrets.Manager) secrets.Manager {
	sm := &cachingSecretsManager{
		manager: manager,
		cache:   NewSecretCache(),
	}
	if manager != nil {
		sm.encrypter = lazy.New(manager.Encrypter)
		sm.decrypter = lazy.New(manager.Decrypter)
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
	csm.encrypter.Value() // Ensure the encrypter is initialized.
	return csm            // The cachingSecretsManager is also an Encrypter itself.
}

func (csm *cachingSecretsManager) Decrypter() config.Decrypter {
	csm.decrypter.Value() // Ensure the decrypter is initialized.
	return csm            // The cachingSecretsManager is also a Decrypter itself.
}

func (csm *cachingSecretsManager) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	return csm.encrypter.Value().EncryptValue(ctx, plaintext)
}

func (csm *cachingSecretsManager) BatchEncrypt(ctx context.Context, plaintexts []string) ([]string, error) {
	return csm.encrypter.Value().BatchEncrypt(ctx, plaintexts)
}

func (csm *cachingSecretsManager) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	return csm.decrypter.Value().DecryptValue(ctx, ciphertext)
}

func (csm *cachingSecretsManager) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	return csm.decrypter.Value().BatchDecrypt(ctx, ciphertexts)
}

// encryptSecret encrypts the plaintext associated with the given secret value.
func (csm *cachingSecretsManager) encryptSecret(ctx context.Context,
	secret *resource.Secret, plaintext string,
) (string, error) {
	// If the cache has an ciphertext for this secret and the plaintext has not changed, re-use the ciphertext.
	//
	// Otherwise, re-encrypt the plaintext and update the cache.
	if ciphertext, ok := csm.cache.TryEncrypt(secret, plaintext); ok {
		return ciphertext, nil
	}
	ciphertext, err := csm.manager.Encrypter().EncryptValue(ctx, plaintext)
	if err != nil {
		return "", err
	}
	csm.insert(secret, plaintext, ciphertext)
	return ciphertext, nil
}

// insert associates the given secret with the given plain- and ciphertext in the cache.
func (csm *cachingSecretsManager) insert(secret *resource.Secret, plaintext, ciphertext string) {
	csm.cache.Write(plaintext, ciphertext, secret)
}

// mapDecrypter is a Decrypter with a preloaded cache. This decrypter is used specifically for deserialization,
// where the deserializer is expected to prime the cache by scanning each resource for secrets, then decrypting all
// of the discovered secrets en masse. Although each call to Decrypt _should_ hit the cache, a mapDecrypter does
// carry an underlying Decrypter in the event that a secret was missed.
//
// Note that this is intentionally separate from cachingCrypter. A cachingCrypter is intended to prevent repeated
// encryption of secrets when the same snapshot is repeatedly serialized over the lifetime of an update, and
// therefore keys on the identity of the secret value itself. A mapDecrypter is intended to allow the deserializer
// to decrypt secrets up-front and prevent repeated calls to decrypt within the context of a single deserialization,
// and cannot key off of secret identity because secrets do not exist when the cache is initialized.
type mapDecrypter struct {
	decrypter config.Decrypter
	cache     map[string]string
}

func newMapDecrypter(decrypter config.Decrypter, cache map[string]string) config.Decrypter {
	return &mapDecrypter{decrypter: decrypter, cache: cache}
}

func (c *mapDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	if plaintext, ok := c.cache[ciphertext]; ok {
		return plaintext, nil
	}

	// The value is not currently in the cache. Decrypt it and add it to the cache.
	plaintext, err := c.decrypter.DecryptValue(ctx, ciphertext)
	if err != nil {
		return "", err
	}

	if c.cache == nil {
		c.cache = make(map[string]string)
	}
	c.cache[ciphertext] = plaintext

	return plaintext, nil
}

func (c *mapDecrypter) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	// Loop and find the entries that are already cached, then batch decrypt the rest
	decryptedResult := make([]string, len(ciphertexts))
	var toDecrypt []string
	if c.cache == nil {
		// Don't bother searching for the cached subset if the cache is nil
		toDecrypt = ciphertexts
	} else {
		toDecrypt = make([]string, 0, len(ciphertexts))
		for i, ct := range ciphertexts {
			if plaintext, ok := c.cache[ct]; ok {
				decryptedResult[i] = plaintext
			} else {
				toDecrypt = append(toDecrypt, ct)
			}
		}
	}

	if len(toDecrypt) == 0 {
		return decryptedResult, nil
	}

	// try and decrypt the rest in a single batch request
	decrypted, err := c.decrypter.BatchDecrypt(ctx, toDecrypt)
	if err != nil {
		return nil, err
	}

	// And add them to the cache
	if c.cache == nil {
		c.cache = make(map[string]string)
	}
	for i, ct := range toDecrypt {
		pt := decrypted[i]
		c.cache[ct] = pt
	}

	// Re-populate results
	for i, ct := range ciphertexts {
		plaintext, ok := c.cache[ct]
		contract.Assertf(ok, "decrypted value not found in cache after bulk request")
		decryptedResult[i] = plaintext
	}

	return decryptedResult, nil
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

// DefaultMaxBatchEncryptCount is the default maximum number of items that can be enqueued for bulk encryption.
const DefaultMaxBatchEncryptCount = 100000

// Ensure that bulkEncrypter implements the Encrypter interface for compatibility.
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
func BeginEncryptionBatchWithCache(encrypter config.Encrypter, cache SecretCache) (BatchEncrypter, CompleteCrypterBatch) {
	return beginEncryptionBatch(encrypter, cache, DefaultMaxBatchEncryptCount)
}

func beginEncryptionBatch(encrypter config.Encrypter, cache SecretCache, maxBatchSize int) (BatchEncrypter, CompleteCrypterBatch) {
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
