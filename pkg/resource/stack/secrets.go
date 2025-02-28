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

// Process any pending encryptions that were enqueued.
type completeBulkOperation func() error

// bulkSecretEncrypter is a special crypter which supports cached and delayed encryption of secrets.
type bulkSecretEncrypter interface {
	secrets.Manager
	BeginBulkEncryption(ctx context.Context) completeBulkOperation
	// EnqueueEncryption takes a pointer to a secret, the plaintext value of the secret, and an optional target
	//  to assign the resulting ciphertext to. The writing of the ciphertext to the target might be immediate or delayed
	// until pending encryptions are processed.
	EnqueueEncryption(ctx context.Context, source *resource.Secret, plaintext string, target *apitype.SecretV1) error
}

type cacheEntry struct {
	plaintext  string
	ciphertext string
	secret     *resource.Secret
}

type cachingSecretsManager struct {
	manager      secrets.Manager
	encrypter    lazy.Lazy[config.Encrypter]
	decrypter    lazy.Lazy[config.Decrypter]
	cache        *secretCache
	encryptQueue []queuedEncryption
	decryptQueue []queuedDecryption
	isInBulkMode bool
}

type queuedEncryption struct {
	source    *resource.Secret
	target    *apitype.SecretV1
	plaintext string
}

type queuedDecryption struct {
	ciphertext string
	target     *resource.Secret
}

var _ bulkSecretEncrypter = (*cachingSecretsManager)(nil)

// NewCachingSecretsManager returns a new secrets.Manager that caches the ciphertext for secret property values. A
// secrets.Manager that will be used to encrypt and decrypt values stored in a serialized deployment can be wrapped
// in a caching secrets manager in order to avoid re-encrypting secrets each time the deployment is serialized.
func NewCachingSecretsManager(manager secrets.Manager) secrets.Manager {
	sm := &cachingSecretsManager{
		manager: manager,
		cache:   &secretCache{},
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

func (csm *cachingSecretsManager) SupportsBulkEncryption(ctx context.Context) bool {
	return csm.manager.Encrypter().SupportsBulkEncryption(ctx)
}

func (csm *cachingSecretsManager) BulkEncrypt(ctx context.Context, plaintexts []string) ([]string, error) {
	return csm.manager.Encrypter().BulkEncrypt(ctx, plaintexts)
}

func (csm *cachingSecretsManager) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	if plaintext, ok := csm.cache.TryDecrypt(ciphertext); ok {
		return plaintext, nil
	}
	return csm.decrypter.Value().DecryptValue(ctx, ciphertext)
}

func (csm *cachingSecretsManager) BulkDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	return csm.decrypter.Value().BulkDecrypt(ctx, ciphertexts)
}

func (csm *cachingSecretsManager) EnqueueEncryption(ctx context.Context,
	source *resource.Secret, plaintext string, target *apitype.SecretV1,
) error {
	contract.Assertf(source != nil, "source secret must not be nil")
	// If the cache has an entry for this secret and the plaintext has not changed, re-use the ciphertext.
	//
	// Otherwise, re-encrypt the plaintext and update the cache.
	// Note: target may be nil if the caller does not need the ciphertext
	// e.g. when priming the cache during deserialization.
	if ciphertext, ok := csm.cache.TryEncrypt(source, plaintext); ok {
		target.Ciphertext = ciphertext
		return nil
	}
	// If the encrypter supports bulk encryption, queue the secret for bulk encryption during finalize.
	if csm.isInBulkMode {
		csm.encryptQueue = append(csm.encryptQueue, queuedEncryption{source, target, plaintext})
		return nil
	}
	// Otherwise, encrypt the secret immediately.
	ciphertext, err := csm.Encrypter().EncryptValue(ctx, plaintext)
	if err != nil {
		return err
	}
	csm.cacheAndAssign(source, target, plaintext, ciphertext)
	return nil
}

func (csm *cachingSecretsManager) BeginBulkEncryption(ctx context.Context) completeBulkOperation {
	contract.Assertf(!csm.isInBulkMode, "cannot begin bulk encryption while already in bulk mode")
	supportsBulk := csm.Encrypter().SupportsBulkEncryption(ctx)
	if !supportsBulk {
		// Don't enter bulk mode if the encrypter doesn't support it.
		// Calls to EnqueueEncryption will encrypt secrets immediately.
		return func() error {
			return nil
		}
	}
	csm.isInBulkMode = true
	return func() error {
		if len(csm.encryptQueue) == 0 {
			return nil
		}
		// Flush the encrypt queue.

		plaintexts := make([]string, len(csm.encryptQueue))
		for i, q := range csm.encryptQueue {
			plaintexts[i] = q.plaintext
		}
		ciphertexts, err := csm.manager.Encrypter().BulkEncrypt(ctx, plaintexts)
		if err != nil {
			return err
		}
		for i, q := range csm.encryptQueue {
			csm.cacheAndAssign(q.source, q.target, q.plaintext, ciphertexts[i])
		}
		// Reset bulk mode
		csm.isInBulkMode = false
		csm.encryptQueue = nil
		return nil
	}
}

// cacheAndAssign associates the given secret with the given plain- and ciphertext in the cache.
func (csm *cachingSecretsManager) cacheAndAssign(secret *resource.Secret, target *apitype.SecretV1,
	plaintext, ciphertext string,
) {
	csm.cache.Write(plaintext, ciphertext, secret)
	if target != nil {
		target.Ciphertext = ciphertext
	}
}

func (csm *cachingSecretsManager) BeginBulkDecryption(ctx context.Context) completeBulkOperation {
	contract.Assertf(!csm.isInBulkMode, "cannot begin bulk decryption while already in bulk mode")
	csm.isInBulkMode = true
	return func() error {
		if len(csm.decryptQueue) == 0 {
			return nil
		}
		// Flush the decrypt queue.
		ciphertexts := make([]string, len(csm.decryptQueue))
		for i, q := range csm.decryptQueue {
			ciphertexts[i] = q.ciphertext
		}
		plaintexts, err := csm.manager.Decrypter().BulkDecrypt(ctx, ciphertexts)
		if err != nil {
			return err
		}
		for i, q := range csm.decryptQueue {
			ev, err := secretPropertyValueFromPlaintext(plaintexts[i])
			if err != nil {
				return err
			}
			q.target.Element = ev
			csm.cache.Write(plaintexts[i], q.ciphertext, q.target)
		}
		csm.isInBulkMode = false
		csm.decryptQueue = nil
		return nil
	}
}

func (csm *cachingSecretsManager) EnqueueDecryption(ctx context.Context, ciphertext string, secret *resource.Secret) error {
	// Try the cache first.
	if plaintext, ok := csm.cache.TryDecrypt(ciphertext); ok {
		ev, err := secretPropertyValueFromPlaintext(plaintext)
		if err != nil {
			return err
		}
		secret.Element = ev
	}
	if csm.isInBulkMode {
		// Add to queue
		csm.decryptQueue = append(csm.decryptQueue, queuedDecryption{ciphertext, secret})
		return nil
	}
	// Otherwise, decrypt the secret immediately.
	plaintext, err := csm.Decrypter().DecryptValue(ctx, ciphertext)
	if err != nil {
		return err
	}
	ev, err := secretPropertyValueFromPlaintext(plaintext)
	if err != nil {
		return err
	}
	secret.Element = ev
	csm.cache.Write(plaintext, ciphertext, secret)
	return nil
}

type secretCache struct {
	bySecret     sync.Map
	byCiphertext sync.Map
}

func (c *secretCache) Write(plaintext, ciphertext string, secret *resource.Secret) {
	entry := cacheEntry{plaintext, ciphertext, secret}
	c.bySecret.Store(secret, entry)
	c.byCiphertext.Store(ciphertext, entry)
}

func (c *secretCache) TryEncrypt(secret *resource.Secret, plaintext string) (string, bool) {
	entry, ok := c.bySecret.Load(secret)
	if !ok {
		return "", false
	}
	return entry.(cacheEntry).ciphertext, true
}

func (c *secretCache) TryDecrypt(ciphertext string) (string, bool) {
	entry, ok := c.byCiphertext.Load(ciphertext)
	if !ok {
		return "", false
	}
	return entry.(cacheEntry).plaintext, true
}

type bulkEncrypter struct {
	encrypter    config.Encrypter
	supportsBulk bool
	cache        *secretCache
	queue        []queuedEncryption
}

var _ config.Encrypter = (*bulkEncrypter)(nil)

func beginEncryptionBulk(ctx context.Context, encrypter config.Encrypter, cache *secretCache) *bulkEncrypter {
	supportsBulk := encrypter.SupportsBulkEncryption(ctx)
	return &bulkEncrypter{encrypter: encrypter, supportsBulk: supportsBulk, cache: cache}
}

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
	if !be.supportsBulk {
		// If the underlying encrypter does not support bulk encryption, encrypt the value immediately.
		ciphertext, err := be.encrypter.EncryptValue(ctx, plaintext)
		if err != nil {
			return err
		}
		target.Ciphertext = ciphertext
		be.cache.Write(plaintext, ciphertext, source)
		return nil
	}
	// Add to the queue
	be.queue = append(be.queue, queuedEncryption{source, target, plaintext})
	return nil
}

func (be *bulkEncrypter) Complete(ctx context.Context) error {
	if len(be.queue) == 0 {
		return nil
	}
	// Flush the encrypt queue
	plaintexts := make([]string, len(be.queue))
	for i, q := range be.queue {
		plaintexts[i] = q.plaintext
	}
	ciphertexts, err := be.encrypter.BulkEncrypt(ctx, plaintexts)
	if err != nil {
		return err
	}
	for i, q := range be.queue {
		q.target.Ciphertext = ciphertexts[i]
		be.cache.Write(q.plaintext, ciphertexts[i], q.source)
	}
	// Empty the queue
	be.queue = nil
	return nil
}

func (be *bulkEncrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	return be.encrypter.EncryptValue(ctx, plaintext)
}

func (be *bulkEncrypter) SupportsBulkEncryption(ctx context.Context) bool {
	return be.encrypter.SupportsBulkEncryption(ctx)
}

func (be *bulkEncrypter) BulkEncrypt(ctx context.Context, plaintexts []string) ([]string, error) {
	return be.encrypter.BulkEncrypt(ctx, plaintexts)
}

type bulkDecrypter struct {
	decrypter config.Decrypter
	cache     *secretCache
	queue     []queuedDecryption
}

var _ config.Decrypter = (*bulkDecrypter)(nil)

func beginDecryptionBulk(decrypter config.Decrypter, cache *secretCache) *bulkDecrypter {
	return &bulkDecrypter{decrypter: decrypter, cache: cache}
}

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
	bd.queue = append(bd.queue, queuedDecryption{ciphertext, secret})
	return nil
}

func (bd *bulkDecrypter) Complete(ctx context.Context) error {
	if len(bd.queue) == 0 {
		return nil
	}
	// Flush the decrypt queue
	ciphertexts := make([]string, len(bd.queue))
	for i, q := range bd.queue {
		ciphertexts[i] = q.ciphertext
	}
	plaintexts, err := bd.decrypter.BulkDecrypt(ctx, ciphertexts)
	if err != nil {
		return err
	}
	for i, q := range bd.queue {
		ev, err := secretPropertyValueFromPlaintext(plaintexts[i])
		if err != nil {
			return err
		}
		q.target.Element = ev
		bd.cache.Write(plaintexts[i], q.ciphertext, q.target)
	}
	// Empty the queue
	bd.queue = nil
	return nil
}

func (bd *bulkDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	return bd.decrypter.DecryptValue(ctx, ciphertext)
}

func (bd *bulkDecrypter) BulkDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	return bd.decrypter.BulkDecrypt(ctx, ciphertexts)
}
