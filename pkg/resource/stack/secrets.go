// Copyright 2016-2019, Pulumi Corporation.
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
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/pkg/v3/secrets/service"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

// DefaultSecretsProvider is the default SecretsProvider to use when deserializing deployments.
var DefaultSecretsProvider SecretsProvider = &defaultSecretsProvider{}

// SecretsProvider allows for the creation of secrets managers based on a well-known type name.
type SecretsProvider interface {
	// OfType returns a secrets manager for the given type, initialized with its previous state.
	OfType(ty string, state json.RawMessage) (secrets.Manager, error)
}

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
	case b64.Type:
		sm = b64.NewBase64SecretsManager()
	case passphrase.Type:
		sm, err = passphrase.NewPassphaseSecretsManagerFromState(state)
	case service.Type:
		sm, err = service.NewServiceSecretsManagerFromState(state)
	case cloud.Type:
		sm, err = cloud.NewCloudSecretsManagerFromState(state)
	default:
		return nil, errors.Errorf("no known secrets provider for type %q", ty)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "constructing secrets manager of type %q", ty)
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

func (csm *cachingSecretsManager) State() interface{} {
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

func (c *cachingCrypter) EncryptValue(plaintext string) (string, error) {
	return c.encrypter.EncryptValue(plaintext)
}

func (c *cachingCrypter) DecryptValue(ciphertext string) (string, error) {
	return c.decrypter.DecryptValue(ciphertext)
}

// encryptSecret encrypts the plaintext associated with the given secret value.
func (c *cachingCrypter) encryptSecret(secret *resource.Secret, plaintext string) (string, error) {
	// If the cache has an entry for this secret and the plaintext has not changed, re-use the ciphertext.
	//
	// Otherwise, re-encrypt the plaintext and update the cache.
	entry, ok := c.cache[secret]
	if ok && entry.plaintext == plaintext {
		return entry.ciphertext, nil
	}
	ciphertext, err := c.encrypter.EncryptValue(plaintext)
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
