package events

import (
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

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
