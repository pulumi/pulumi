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
)

// DefaultSecretsProvider is the default SecretsProvider to use when deserializing deployments.
var DefaultSecretsProvider secrets.Provider = &defaultSecretsProvider{}

// defaultSecretsProvider implements the secrets.ManagerProviderFactory interface. Essentially
// it is the global location where new secrets managers can be registered for use when
// decrypting checkpoints.
type defaultSecretsProvider struct{}

// OfType returns a secrets manager for the given secrets type. Returns an error
// if the type is uknown or the state is invalid.
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

func (csm *cachingSecretsManager) Encrypter() (config.Encrypter, error) {
	enc, err := csm.manager.Encrypter()
	if err != nil {
		return nil, err
	}
	return &cachingCrypter{
		encrypter: enc,
		cache:     csm.cache,
	}, nil
}

func (csm *cachingSecretsManager) Decrypter() (config.Decrypter, error) {
	dec, err := csm.manager.Decrypter()
	if err != nil {
		return nil, err
	}
	return &cachingCrypter{
		decrypter: dec,
		cache:     csm.cache,
	}, nil
}

type cachingCrypter struct {
	encrypter config.Encrypter
	decrypter config.Decrypter
	cache     map[*resource.Secret]cacheEntry
}

func (c *cachingCrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	return c.encrypter.EncryptValue(ctx, plaintext)
}

func (c *cachingCrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	return c.decrypter.DecryptValue(ctx, ciphertext)
}

func (c *cachingCrypter) BulkDecrypt(ctx context.Context, ciphertexts []string) (map[string]string, error) {
	return c.decrypter.BulkDecrypt(ctx, ciphertexts)
}

// encryptSecret encrypts the plaintext associated with the given secret value.
func (c *cachingCrypter) encryptSecret(secret *resource.Secret, plaintext string) (string, error) {
	ctx := context.TODO()

	// If the cache has an entry for this secret and the plaintext has not changed, re-use the ciphertext.
	//
	// Otherwise, re-encrypt the plaintext and update the cache.
	entry, ok := c.cache[secret]
	if ok && entry.plaintext == plaintext {
		return entry.ciphertext, nil
	}
	ciphertext, err := c.encrypter.EncryptValue(ctx, plaintext)
	if err != nil {
		return "", err
	}
	c.insert(secret, plaintext, ciphertext)
	return ciphertext, nil
}

// insert associates the given secret with the given plain- and ciphertext in the cache.
func (c *cachingCrypter) insert(secret *resource.Secret, plaintext, ciphertext string) {
	c.cache[secret] = cacheEntry{plaintext, ciphertext}
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

func (c *mapDecrypter) BulkDecrypt(ctx context.Context, ciphertexts []string) (map[string]string, error) {
	// Loop and find the entries that are already cached, then BulkDecrypt the rest
	secretMap := map[string]string{}
	var toDecrypt []string
	if c.cache == nil {
		// Don't bother searching for the cached subset if the cache is nil
		toDecrypt = ciphertexts
	} else {
		toDecrypt = make([]string, 0)
		for _, ct := range ciphertexts {
			if plaintext, ok := c.cache[ct]; ok {
				secretMap[ct] = plaintext
			} else {
				toDecrypt = append(toDecrypt, ct)
			}
		}
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

	for ct, pt := range decrypted {
		secretMap[ct] = pt
		c.cache[ct] = pt
	}

	return secretMap, nil
}
