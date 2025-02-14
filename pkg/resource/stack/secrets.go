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

	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/pkg/v3/secrets/service"
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

type cacheEntry struct {
	plaintext  string
	ciphertext string
}

type cachingSecretsManager struct {
	manager secrets.Manager
	cache   map[*resource.Secret]cacheEntry
}

// NewCachingSecretsManager returns a new secrets.Manager that caches the ciphertext for secret property values. A
// secrets.Manager that will be used to encrypt and decrypt values stored in a serialized deployment can be wrapped
// in a caching secrets manager in order to avoid re-encrypting secrets each time the deployment is serialized.
func NewCachingSecretsManager(manager secrets.Manager) secrets.Manager {
	return &cachingSecretsManager{
		manager: manager,
		cache:   make(map[*resource.Secret]cacheEntry),
	}
}

func (csm *cachingSecretsManager) Type() string {
	return csm.manager.Type()
}

func (csm *cachingSecretsManager) State() json.RawMessage {
	return csm.manager.State()
}

func (csm *cachingSecretsManager) Encrypter() config.Encrypter {
	return csm // The cachingSecretsManager is also an Encrypter itself.
}

func (csm *cachingSecretsManager) Decrypter() config.Decrypter {
	return csm // The cachingSecretsManager is also a Decrypter itself.
}

func (csm *cachingSecretsManager) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	return csm.manager.Encrypter().EncryptValue(ctx, plaintext)
}

func (csm *cachingSecretsManager) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	return csm.manager.Decrypter().DecryptValue(ctx, ciphertext)
}

func (csm *cachingSecretsManager) BulkDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	return csm.manager.Decrypter().BulkDecrypt(ctx, ciphertexts)
}

// encryptSecret encrypts the plaintext associated with the given secret value.
func (csm *cachingSecretsManager) encryptSecret(ctx context.Context,
	secret *resource.Secret, plaintext string,
) (string, error) {
	// If the cache has an entry for this secret and the plaintext has not changed, re-use the ciphertext.
	//
	// Otherwise, re-encrypt the plaintext and update the cache.
	entry, ok := csm.cache[secret]
	if ok && entry.plaintext == plaintext {
		return entry.ciphertext, nil
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
	csm.cache[secret] = cacheEntry{plaintext, ciphertext}
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

func (c *mapDecrypter) BulkDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	// Loop and find the entries that are already cached, then BulkDecrypt the rest
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

	// try and bulk decrypt the rest
	decrypted, err := c.decrypter.BulkDecrypt(ctx, toDecrypt)
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
