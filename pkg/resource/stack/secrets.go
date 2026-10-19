// Copyright 2016, Pulumi Corporation.
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
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel"

	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type Base64SecretsProvider struct{}

func (Base64SecretsProvider) OfType(_ context.Context, ty string, state json.RawMessage) (secrets.Manager, error) {
	if ty != "b64" {
		return nil, fmt.Errorf("no known secrets provider for type %q", ty)
	}
	return NewBatchingCachingSecretsManager(b64.NewBase64SecretsManager()), nil
}

// BatchingSecretsManager is a secrets.Manager that supports batch encryption and decryption operations.
type BatchingSecretsManager interface {
	secrets.Manager
	// BeginBatchEncryption returns a new BatchEncrypter and CompleteCrypterBatch function.
	// The BatchEncrypter allows encryption operations to be enqueued then processed in a batch request to avoid
	// round-trips for when doing so with the underlying encrypter incurs overhead. Encryptions to a secret are enqueued
	// and processed in a batch request either when the wrapping transaction is committed or earlier if an implementation
	// supports or requires it.
	//
	// The CompleteCrypterBatch function must be called to ensure that all enqueued encryption operations are processed.
	//
	//	batchEncrypter, completeCrypterBatch := batchingSecretsManager.BeginBatchEncryption()
	//	err = batchEncrypter.Enqueue(ctx, sourceSecret, plaintext, targetSerializedSecret)
	//	err = completeCrypterBatch(ctx)
	BeginBatchEncryption() (BatchEncrypter, CompleteCrypterBatch)
	// BeginBatchDecryption returns a new BatchDecrypter and CompleteCrypterBatch function.
	// The BatchDecrypter allows decryption operations to be batched to avoid round-trips for when the underlying
	// decrypter incurs overhead. Decryptions to a secret are enqueued and processed in a batch request either when the
	// queue is full or earlier if an implementation supports or requires it.
	//
	// The CompleteCrypterBatch function must be called to ensure that all enqueued decryption operations are processed.
	//
	//	batchDecrypter, completeCrypterBatch := batchingSecretsManager.BeginBatchDecryption()
	//	err = batchDecrypter.Enqueue(ctx, ciphertext)
	//	err = completeCrypterBatch(ctx)
	//	plaintext, err := batchDecrypter.DecryptValue(ctx, ciphertext) // served from the batch
	BeginBatchDecryption() (BatchDecrypter, CompleteCrypterBatch)
}

type batchingCachingSecretsManager struct {
	manager secrets.Manager
	cache   SecretCache
}

// NewBatchingCachingSecretsManager returns a new BatchingSecretsManager that caches the ciphertext for secret property
// values. A secrets.Manager that will be used to encrypt and decrypt values stored in a serialized deployment can be
// wrapped in a caching secrets manager in order to avoid re-encrypting secrets each time the deployment is serialized.
// When secrets values are not cached, then operations can be batched when using the batch transaction methods.
func NewBatchingCachingSecretsManager(manager secrets.Manager) BatchingSecretsManager {
	sm := &batchingCachingSecretsManager{
		manager: manager,
		cache:   NewSecretCache(),
	}
	return sm
}

func (csm *batchingCachingSecretsManager) Type() string {
	return csm.manager.Type()
}

func (csm *batchingCachingSecretsManager) State() json.RawMessage {
	return csm.manager.State()
}

func (csm *batchingCachingSecretsManager) Encrypter() config.Encrypter {
	return csm.manager.Encrypter()
}

func (csm *batchingCachingSecretsManager) Decrypter() config.Decrypter {
	return csm.manager.Decrypter()
}

func (csm *batchingCachingSecretsManager) BeginBatchEncryption() (BatchEncrypter, CompleteCrypterBatch) {
	return BeginBatchEncryptionWithCache(csm.manager.Encrypter(), csm.cache)
}

func (csm *batchingCachingSecretsManager) BeginBatchDecryption() (BatchDecrypter, CompleteCrypterBatch) {
	// We don't use the cache here to ensure that we always re-encrypt all secrets at least once per operation.
	return BeginDecryptionBatch(csm.manager.Decrypter())
}

// SecretID identifies a particular secret value within one process. Callers obtain IDs from
// [property.Value.SecretID] on the serialisation side, or from the address of a
// [*resource.Secret] on the deserialisation side. The value is opaque; it is only ever
// compared for equality and used as a map key.
type SecretID uintptr

// SecretCache allows the bidirectional cached conversion between: `ciphertext <-> plaintext + secret id`.
// The same plaintext can be associated with multiple secrets, each of which will have their own ciphertexts which
// should not be shared.
//
//	cache := NewSecretCache()
//	id := SecretID(uintptr(unsafe.Pointer(&resource.Secret{})))
//	cache.Write("plaintext", "ciphertext", id)
//	plaintext, ok := cache.LookupPlaintext("ciphertext") // "plaintext", true
//	ciphertext, ok := cache.LookupCiphertext(id, "plaintext") // "ciphertext", true
type SecretCache interface {
	// Write stores the plaintext, ciphertext, and secret id in the cache, overwriting any previous entry for the id.
	Write(plaintext, ciphertext string, id SecretID)
	// LookupCiphertext returns the cached ciphertext for the given secret id and plaintext, if it exists.
	LookupCiphertext(id SecretID, plaintext string) (string, bool)
	// LookupPlaintext returns the cached plaintext for the given ciphertext, if it exists.
	LookupPlaintext(ciphertext string) (string, bool)
}

type nullSecretCache struct{}

func (nullSecretCache) Write(plaintext, ciphertext string, id SecretID) {}
func (nullSecretCache) LookupCiphertext(id SecretID, plaintext string) (string, bool) {
	return "", false
}

func (nullSecretCache) LookupPlaintext(ciphertext string) (string, bool) {
	return "", false
}

type secretCache struct {
	byID         sync.Map
	byCiphertext sync.Map
}

type secretCacheEntry struct {
	plaintext  string
	ciphertext string
	id         SecretID
}

// NewSecretCache returns a new secretCache which allows the bidirectional cached conversion between:
// `ciphertext <-> plaintext + secret pointer`. The same plaintext can be associated with multiple secrets, each of
// which will have their own ciphertexts which should not be shared.
//
// All methods are thread-safe and can be called concurrently by multiple goroutines.
// The cache can be disabled by setting the environment variable PULUMI_DISABLE_SECRET_CACHE to "true".
//
//	cache := NewSecretCache()
//	secret := &resource.Secret{}
//	cache.Write("plaintext", "ciphertext", secret)
//	plaintext, ok := cache.LookupPlaintext("ciphertext") // "plaintext", true
//	ciphertext, ok := cache.LookupCiphertext(secret, "plaintext") // "ciphertext", true
func NewSecretCache() SecretCache {
	// If the environment variable PULUMI_DISABLE_SECRET_CACHE is set to "true", the secret cache will be disabled and
	// no entries will be stored. This is a short-term escape hatch in case there's unforeseen issues with expanded
	// caching scopes and should be removable once we're confident that customers are not affected.
	if env.DisableSecretCache.Value() {
		return nullSecretCache{}
	}
	return &secretCache{}
}

// Write stores the plaintext, ciphertext, and secret in the cache, overwriting any previous entry for the secret.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (c *secretCache) Write(plaintext, ciphertext string, id SecretID) {
	entry := secretCacheEntry{plaintext, ciphertext, id}
	c.byID.Store(id, entry)
	c.byCiphertext.Store(ciphertext, entry)
}

// LookupCiphertext returns the cached ciphertext for the given secret id and plaintext, if it exists.
// The ciphertext is returned as a string, and a boolean is returned to indicate whether the id was found.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (c *secretCache) LookupCiphertext(id SecretID, plaintext string) (string, bool) {
	entry, ok := c.byID.Load(id)
	if !ok {
		return "", false
	}
	cacheEntry := entry.(secretCacheEntry)
	if cacheEntry.plaintext != plaintext {
		return "", false
	}
	return cacheEntry.ciphertext, true
}

// LookupPlaintext returns the cached plaintext for the given ciphertext, if it exists.
// The plaintext is returned as a string, and a boolean is returned to indicate whether the ciphertext was found.
// This method is thread-safe and can be called concurrently by multiple goroutines.
func (c *secretCache) LookupPlaintext(ciphertext string) (string, bool) {
	entry, ok := c.byCiphertext.Load(ciphertext)
	if !ok {
		return "", false
	}
	return entry.(secretCacheEntry).plaintext, true
}

// BatchEncrypter is an extension of an Encrypter which supports processing secret encryption operations in batches.
// This is constructed by calling BatchingSecretsManager.BeginBatchEncryption.
type BatchEncrypter interface {
	config.Encrypter

	// Enqueue a secret for encryption at some point in the future. The ciphertext will be written to the target secret
	// object when the batch operation is processed. id identifies the source secret for cache reuse.
	// This method is thread-safe and can be called concurrently by multiple goroutines.
	Enqueue(ctx context.Context, id SecretID, plaintext string, target *apitype.SecretV1) error
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
	id        SecretID
	target    *apitype.SecretV1
	plaintext string
}

// DefaultMaxBatchEncryptCount is the default maximum number of items that can be enqueued for batch encryption.
const DefaultMaxBatchEncryptCount = 1000

// Ensure that cachingBatchEncrypter implements the BatchEncrypter interface for compatibility.
var _ BatchEncrypter = (*cachingBatchEncrypter)(nil)

// BeginBatchEncryptionWithCache returns a new BatchEncrypter and CompleteCrypterBatch function with a custom cache.
// The BatchEncrypter allows encryption operations to be batched to avoid round-trips for when the underlying encrypter
// requires a network call. Encryptions to a secret are enqueued and processed in a batch request either when the queue
// is full or when the wrapping transaction is committed. If the cache has an entry for a secret and the plaintext has
// not changed, the previous ciphertext is used immediately and not enqueued for the batch operation. Results are
// also written to the provided cache.
//
// The CompleteCrypterBatch function must be called to ensure that all enqueued encryption operations are processed.
//
//	batchEncrypter, completeCrypterBatch := BeginEncryptionBatch(encrypter)
//	SerializeSecrets(ctx, batchEncrypter, secrets)
//	err := completeCrypterBatch(ctx)
func BeginBatchEncryptionWithCache(
	encrypter config.Encrypter, cache SecretCache,
) (BatchEncrypter, CompleteCrypterBatch) {
	return beginBatchEncryption(encrypter, cache, DefaultMaxBatchEncryptCount)
}

func beginBatchEncryption(
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
	id SecretID, plaintext string, target *apitype.SecretV1,
) error {
	contract.Assertf(!be.closed.Load(), "batch encrypter must not be closed")
	// Add to the queue
	for {
		select {
		case be.queue <- queuedEncryption{id, target, plaintext}:
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

	ciphertexts := make([]string, len(dequeued))
	// If the cache has entries for all secrets, re-use the previous ciphertexts to save the re-encryption cost.
	cacheMissed := false
	for i, q := range dequeued {
		if ciphertext, ok := be.cache.LookupCiphertext(q.id, q.plaintext); ok {
			ciphertexts[i] = ciphertext
		} else {
			cacheMissed = true
			ciphertexts = nil
			break
		}
	}
	if cacheMissed {
		var err error
		ciphertexts, err = be.encrypter.BatchEncrypt(ctx, plaintexts)
		if err != nil {
			return err
		}
	}
	for i, q := range dequeued {
		ciphertext := ciphertexts[i]
		q.target.Ciphertext = ciphertext
		be.cache.Write(q.plaintext, ciphertext, q.id)
	}
	return nil
}

func (be *cachingBatchEncrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	return be.encrypter.EncryptValue(ctx, plaintext)
}

func (be *cachingBatchEncrypter) BatchEncrypt(ctx context.Context, plaintexts []string) ([]string, error) {
	return be.encrypter.BatchEncrypt(ctx, plaintexts)
}

// BatchDecrypter is an extension of a Decrypter which supports pre-batching secret decryptions.
// This is constructed by calling BatchingSecretsManager.BeginBatchDecryption.
//
// Callers register ciphertexts up front via Enqueue, flush with CompleteCrypterBatch, and then
// consume plaintexts through the normal DecryptValue / BatchDecrypt methods, which are served
// from the batch's local cache without issuing another decrypt. This two-phase shape lets a
// caller amortise many decryptions into a single round-trip without threading batch state
// through the deserialization code paths.
type BatchDecrypter interface {
	config.Decrypter

	// Enqueue a ciphertext for decryption as part of this batch. After CompleteCrypterBatch returns
	// successfully, DecryptValue(ctx, ciphertext) will serve the plaintext from the batch's local
	// cache without issuing another decrypt. This method is thread-safe.
	Enqueue(ctx context.Context, ciphertext string) error
}

type cachingBatchDecrypter struct {
	decrypter       config.Decrypter
	cache           SecretCache
	queue           chan string
	closed          atomic.Bool
	completeMutex   sync.Mutex
	maxBatchSize    int
	batchPlaintexts sync.Map // ciphertext -> plaintext, populated by sendNextBatch
}

const DefaultMaxBatchDecryptCount = 1000

// Ensure that cachingBatchDecrypter implements the BatchDecrypter interface for compatibility.
var _ BatchDecrypter = (*cachingBatchDecrypter)(nil)

// BeginDecryptionBatch returns a new BatchDecrypter and CompleteCrypterBatch function.
// The BatchDecrypter allows decryption operations to be batched to avoid round-trips for when the underlying decrypter
// requires a network call. Decryptions to a secret are enqueued and processed in a batch request either when the queue
// is full or when the wrapping transaction is committed.
//
// The CompleteCrypterBatch function must be called to ensure that all enqueued decryption operations are processed.
//
//	batchDecrypter, completeCrypterBatch := BeginDecryptionBatch(decrypter)
//	DeserializeSecrets(ctx, batchDecrypter, secrets)
//	err := completeCrypterBatch(ctx)
func BeginDecryptionBatch(decrypter config.Decrypter) (BatchDecrypter, CompleteCrypterBatch) {
	return BeginBatchDecryptionWithCache(decrypter, nullSecretCache{})
}

// BeginBatchDecryptionWithCache returns a new BatchDecrypter and CompleteCrypterBatch function with a custom cache.
// The BatchDecrypter allows decryption operations to be batched to avoid round-trips for when the underlying decrypter
// requires a network call. Decryptions to a secret are enqueued and processed in a batch request either when the queue
// is full or when the wrapping transaction is committed. If the cache has an entry for a ciphertext, the plaintext is
// used immediately and not enqueued for the batch operation. Results are also written to the provided cache.
//
// The CompleteCrypterBatch function must be called to ensure that all enqueued decryption operations are processed.
//
//	batchDecrypter, completeCrypterBatch := BeginDecryptionBatch(decrypter)
//	DeserializeSecrets(ctx, batchDecrypter, secrets)
//	err := completeCrypterBatch(ctx)
func BeginBatchDecryptionWithCache(
	decrypter config.Decrypter, cache SecretCache,
) (BatchDecrypter, CompleteCrypterBatch) {
	return beginBatchDecryption(decrypter, cache, DefaultMaxBatchDecryptCount)
}

func beginBatchDecryption(decrypter config.Decrypter, cache SecretCache, maxBatchSize int,
) (BatchDecrypter, CompleteCrypterBatch) {
	contract.Assertf(decrypter != nil, "decrypter must not be nil")
	contract.Assertf(cache != nil, "cache must not be nil")
	contract.Assertf(maxBatchSize > 0, "maxBatchSize must be greater than 0")
	batchDecrypter := &cachingBatchDecrypter{
		decrypter:    decrypter,
		cache:        cache,
		queue:        make(chan string, maxBatchSize),
		maxBatchSize: maxBatchSize,
	}
	return batchDecrypter, func(ctx context.Context) error {
		wasClosed := batchDecrypter.closed.Swap(true)
		contract.Assertf(!wasClosed, "batch decrypter already completed")
		return batchDecrypter.sendNextBatch(ctx)
	}
}

func (bd *cachingBatchDecrypter) Enqueue(ctx context.Context, ciphertext string) error {
	contract.Assertf(!bd.closed.Load(), "batch decrypter must not be closed")
	// Already in the batch cache (e.g. the same ciphertext was enqueued twice): skip.
	if _, ok := bd.batchPlaintexts.Load(ciphertext); ok {
		return nil
	}
	for {
		select {
		case bd.queue <- ciphertext:
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

// sendNextBatch processes any pending decryption operations in the queue, writing resulting
// plaintexts into bd.batchPlaintexts. Thread-safe.
func (bd *cachingBatchDecrypter) sendNextBatch(ctx context.Context) error {
	if len(bd.queue) == 0 {
		return nil
	}
	// Only send 1 batch at a time
	bd.completeMutex.Lock()
	defer bd.completeMutex.Unlock()

	// Flush the decrypt queue. Take up to the maximum number of items from the queue; other items
	// might be enqueued concurrently and will be sent in the next batch.
	ciphertexts := make([]string, 0, len(bd.queue))
dequeue:
	for range bd.maxBatchSize {
		select {
		case ct := <-bd.queue:
			ciphertexts = append(ciphertexts, ct)
		default: // Queue is empty
			break dequeue
		}
	}

	plaintexts := make([]string, len(ciphertexts))
	// If the cache has entries for all ciphertexts, re-use the previous plaintexts to save the re-decryption cost.
	cacheMissed := false
	for i, ct := range ciphertexts {
		if plaintext, ok := bd.cache.LookupPlaintext(ct); ok {
			plaintexts[i] = plaintext
		} else {
			cacheMissed = true
			plaintexts = nil
			break
		}
	}
	if cacheMissed {
		var err error
		plaintexts, err = bd.decrypter.BatchDecrypt(ctx, ciphertexts)
		if err != nil {
			return err
		}
	}
	for i, ct := range ciphertexts {
		bd.batchPlaintexts.Store(ct, plaintexts[i])
	}
	return nil
}

// DecryptValue returns the plaintext for ciphertext. If ciphertext was previously enqueued and the
// batch has been flushed, the plaintext is served from the batch's local cache without re-issuing
// a decrypt; otherwise it falls through to the underlying decrypter.
func (bd *cachingBatchDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	if v, ok := bd.batchPlaintexts.Load(ciphertext); ok {
		return v.(string), nil
	}
	return bd.decrypter.DecryptValue(ctx, ciphertext)
}

// BatchDecrypt returns plaintexts for ciphertexts, serving any that were previously enqueued (and
// whose batch has been flushed) from the local cache and delegating the remainder to the
// underlying decrypter in a single call.
func (bd *cachingBatchDecrypter) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	plaintexts := make([]string, len(ciphertexts))
	var missing []string
	var missingIdx []int
	for i, ct := range ciphertexts {
		if v, ok := bd.batchPlaintexts.Load(ct); ok {
			plaintexts[i] = v.(string)
		} else {
			missing = append(missing, ct)
			missingIdx = append(missingIdx, i)
		}
	}
	if len(missing) == 0 {
		return plaintexts, nil
	}
	fetched, err := bd.decrypter.BatchDecrypt(ctx, missing)
	if err != nil {
		return nil, err
	}
	for j, i := range missingIdx {
		plaintexts[i] = fetched[j]
	}
	return plaintexts, nil
}

// BatchEncrypt is a helper function that encapsulates the pattern of checking if a secrets manager
// supports batching, starting a batch encryption operation, calling a function with the encrypter,
// and completing the batch operation.
func BatchEncrypt[T any](
	ctx context.Context,
	sm secrets.Manager,
	fn func(context.Context, config.Encrypter) (T, error),
) (result T, err error) {
	enc := sm.Encrypter()

	if batchingSecretsManager, ok := sm.(BatchingSecretsManager); ok {
		var completeBatch CompleteCrypterBatch
		enc, completeBatch = batchingSecretsManager.BeginBatchEncryption()
		defer func() { err = errors.Join(err, completeBatch(ctx)) }()
	}

	result, fnerr := fn(ctx, enc)
	err = errors.Join(err, fnerr)

	return result, err
}

// BatchDecrypt is a helper function that encapsulates the pattern of checking if a secrets manager
// supports batching, starting a batch decryption operation, calling a function with the decrypter,
// and completing the batch operation.
func BatchDecrypt[T any](
	ctx context.Context,
	sm secrets.Manager,
	fn func(context.Context, config.Decrypter) (T, error),
) (result T, err error) {
	tracer := otel.Tracer("pulumi-cli")
	ctx, span := cmdutil.StartSpan(ctx, tracer, "stack.BatchDecrypt")
	defer span.End()

	var dec config.Decrypter

	if sm != nil {
		if batchingSecretsManager, ok := sm.(BatchingSecretsManager); ok {
			var completeBatch CompleteCrypterBatch
			dec, completeBatch = batchingSecretsManager.BeginBatchDecryption()
			defer func() { err = errors.Join(err, completeBatch(ctx)) }()
		} else {
			dec = sm.Decrypter()
		}
	} else {
		// We'll attempt to continue without a decrypter, but fail if we encounter encrypted secrets.
		dec = config.NewErrorCrypter("snapshot contains encrypted secrets but no secrets manager could be found")
	}

	result, fnerr := fn(ctx, dec)
	err = errors.Join(err, fnerr)
	return result, err
}
