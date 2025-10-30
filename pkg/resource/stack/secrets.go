package stack

import stack "github.com/pulumi/pulumi/sdk/v3/pkg/resource/stack"

type Base64SecretsProvider = stack.Base64SecretsProvider

// BatchingSecretsManager is a secrets.Manager that supports batch encryption and decryption operations.
type BatchingSecretsManager = stack.BatchingSecretsManager

// SecretCache allows the bidirectional cached conversion between: `ciphertext <-> plaintext + secret pointer`.
// The same plaintext can be associated with multiple secrets, each of which will have their own ciphertexts which
// should not be shared.
// 
// 	cache := NewSecretCache()
// 	secret := &resource.Secret{}
// 	cache.Write("plaintext", "ciphertext", secret)
// 	plaintext, ok := cache.LookupPlaintext("ciphertext") // "plaintext", true
// 	ciphertext, ok := cache.LookupCiphertext(secret, "plaintext") // "ciphertext", true
type SecretCache = stack.SecretCache

// BatchEncrypter is an extension of an Encrypter which supports processing secret encryption operations in batches.
// This is constructed by calling BatchingSecretsManager.BeginBatchEncryption.
type BatchEncrypter = stack.BatchEncrypter

// CompleteCrypterBatch is a function that must be called to ensure that all enqueued crypter operations are processed.
type CompleteCrypterBatch = stack.CompleteCrypterBatch

// BatchDecrypter is an extension of a Decrypter which supports processing secret decryption operations in batches.
// This is constructed by calling BatchingSecretsManager.BeginBatchDecryption.
type BatchDecrypter = stack.BatchDecrypter

type DeserializeSecretPropertyValue = stack.DeserializeSecretPropertyValue

// DefaultMaxBatchEncryptCount is the default maximum number of items that can be enqueued for batch encryption.
const DefaultMaxBatchEncryptCount = stack.DefaultMaxBatchEncryptCount

const DefaultMaxBatchDecryptCount = stack.DefaultMaxBatchDecryptCount

// NewBatchingCachingSecretsManager returns a new BatchingSecretsManager that caches the ciphertext for secret property
// values. A secrets.Manager that will be used to encrypt and decrypt values stored in a serialized deployment can be
// wrapped in a caching secrets manager in order to avoid re-encrypting secrets each time the deployment is serialized.
// When secrets values are not cached, then operations can be batched when using the batch transaction methods.
func NewBatchingCachingSecretsManager(manager secrets.Manager) BatchingSecretsManager {
	return stack.NewBatchingCachingSecretsManager(manager)
}

// NewSecretCache returns a new secretCache which allows the bidirectional cached conversion between:
// `ciphertext <-> plaintext + secret pointer`. The same plaintext can be associated with multiple secrets, each of
// which will have their own ciphertexts which should not be shared.
// 
// All methods are thread-safe and can be called concurrently by multiple goroutines.
// The cache can be disabled by setting the environment variable PULUMI_DISABLE_SECRET_CACHE to "true".
// 
// 	cache := NewSecretCache()
// 	secret := &resource.Secret{}
// 	cache.Write("plaintext", "ciphertext", secret)
// 	plaintext, ok := cache.LookupPlaintext("ciphertext") // "plaintext", true
// 	ciphertext, ok := cache.LookupCiphertext(secret, "plaintext") // "ciphertext", true
func NewSecretCache() SecretCache {
	return stack.NewSecretCache()
}

// BeginBatchEncryptionWithCache returns a new BatchEncrypter and CompleteCrypterBatch function with a custom cache.
// The BatchEncrypter allows encryption operations to be batched to avoid round-trips for when the underlying encrypter
// requires a network call. Encryptions to a secret are enqueued and processed in a batch request either when the queue
// is full or when the wrapping transaction is committed. If the cache has an entry for a secret and the plaintext has
// not changed, the previous ciphertext is used immediately and not enqueued for the batch operation. Results are
// also written to the provided cache.
// 
// The CompleteCrypterBatch function must be called to ensure that all enqueued encryption operations are processed.
// 
// 	batchEncrypter, completeCrypterBatch := BeginEncryptionBatch(encrypter)
// 	SerializeSecrets(ctx, batchEncrypter, secrets)
// 	err := completeCrypterBatch(ctx)
func BeginBatchEncryptionWithCache(encrypter config.Encrypter, cache SecretCache) (BatchEncrypter, CompleteCrypterBatch) {
	return stack.BeginBatchEncryptionWithCache(encrypter, cache)
}

// BeginDecryptionBatch returns a new BatchDecrypter and CompleteCrypterBatch function.
// The BatchDecrypter allows decryption operations to be batched to avoid round-trips for when the underlying decrypter
// requires a network call. Decryptions to a secret are enqueued and processed in a batch request either when the queue
// is full or when the wrapping transaction is committed.
// 
// The CompleteCrypterBatch function must be called to ensure that all enqueued decryption operations are processed.
// 
// 	batchDecrypter, completeCrypterBatch := BeginDecryptionBatch(decrypter)
// 	DeserializeSecrets(ctx, batchDecrypter, secrets)
// 	err := completeCrypterBatch(ctx)
func BeginDecryptionBatch(decrypter config.Decrypter) (BatchDecrypter, CompleteCrypterBatch) {
	return stack.BeginDecryptionBatch(decrypter)
}

// BeginBatchDecryptionWithCache returns a new BatchDecrypter and CompleteCrypterBatch function with a custom cache.
// The BatchDecrypter allows decryption operations to be batched to avoid round-trips for when the underlying decrypter
// requires a network call. Decryptions to a secret are enqueued and processed in a batch request either when the queue
// is full or when the wrapping transaction is committed. If the cache has an entry for a ciphertext, the plaintext is
// used immediately and not enqueued for the batch operation. Results are also written to the provided cache.
// 
// The CompleteCrypterBatch function must be called to ensure that all enqueued decryption operations are processed.
// 
// 	batchDecrypter, completeCrypterBatch := BeginDecryptionBatch(decrypter)
// 	DeserializeSecrets(ctx, batchDecrypter, secrets)
// 	err := completeCrypterBatch(ctx)
func BeginBatchDecryptionWithCache(decrypter config.Decrypter, cache SecretCache) (BatchDecrypter, CompleteCrypterBatch) {
	return stack.BeginBatchDecryptionWithCache(decrypter, cache)
}

