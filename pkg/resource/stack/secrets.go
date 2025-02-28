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

type cacheEntry struct {
	plaintext  string
	ciphertext string
	secret     *resource.Secret
}

type cachingSecretsManager struct {
	manager secrets.Manager
	cache   *secretCache
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

func (csm *cachingSecretsManager) BeginBulkEncryption(ctx context.Context) (config.Encrypter, completeBulkOperation) {
	internalEncrypter := csm.manager.Encrypter()
	bulkEncrypter := beginEncryptionBulk(ctx, internalEncrypter, csm.cache)
	return bulkEncrypter, bulkEncrypter.Complete
}

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
	encrypter config.Encrypter
	cache     *secretCache
	queue     []queuedEncryption
}

var _ config.Encrypter = (*bulkEncrypter)(nil)

func beginEncryptionBulk(ctx context.Context, encrypter config.Encrypter, cache *secretCache) *bulkEncrypter {
	return &bulkEncrypter{encrypter: encrypter, cache: cache}
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
