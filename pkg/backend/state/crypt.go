// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package state

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func readPassphrase(prompt string) (string, error) {
	if phrase := os.Getenv("PULUMI_CONFIG_PASSPHRASE"); phrase != "" {
		return phrase, nil
	}
	return cmdutil.ReadConsoleNoEcho(prompt)
}

// DefaultCrypter gets the right value encrypter/decrypter given the project configuration.
func DefaultCrypter(cfg config.Map) (config.Crypter, error) {
	// If there is no config, we can use a standard panic crypter.
	if !cfg.HasSecureValue() {
		return config.NewPanicCrypter(), nil
	}

	// Otherwise, we will use an encrypted one.
	return SymmetricCrypter()
}

// SymmetricCrypter gets the right value encrypter/decrypter for this project.
func SymmetricCrypter() (config.Crypter, error) {
	// First, read the package to see if we've got a key.
	pkg, err := workspace.GetPackage()
	if err != nil {
		return nil, err
	}

	// If there's already a salt, use it.
	if pkg.EncryptionSalt != "" {
		phrase, phraseErr := readPassphrase("Enter your passphrase to unlock config/secrets\n" +
			"    (set PULUMI_CONFIG_PASSPHRASE to remember)")
		if phraseErr != nil {
			return nil, phraseErr
		}

		return symmetricCrypterFromPhraseAndState(phrase, pkg.EncryptionSalt)
	}

	// Read a passphrase and confirm it.
	phrase, err := readPassphrase("Enter your passphrase to protect config/secrets")
	if err != nil {
		return nil, err
	}
	confirm, err := readPassphrase("Re-enter your passphrase to confirm")
	if err != nil {
		return nil, err
	}
	if phrase != confirm {
		return nil, errors.New("passphrases do not match")
	}

	// Produce a new salt.
	salt := make([]byte, 8)
	_, err = cryptorand.Read(salt)
	contract.Assertf(err == nil, "could not read from system random")

	// Encrypt a message and store it with the salt so we can test if the password is correct later.
	crypter := config.NewSymmetricCrypterFromPassphrase(phrase, salt)
	msg, err := crypter.EncryptValue("pulumi")
	contract.Assert(err == nil)

	// Now store the result on the package and save it.
	pkg.EncryptionSalt = fmt.Sprintf("v1:%s:%s", base64.StdEncoding.EncodeToString(salt), msg)
	if err = workspace.SavePackage(pkg); err != nil {
		return nil, err
	}

	return crypter, nil
}

// given a passphrase and an encryption state, construct a Crypter from it. Our encryption
// state value is a version tag followed by version specific state information. Presently, we only have one version
// we support (`v1`) which is AES-256-GCM using a key derived from a passphrase using 1,000,000 iterations of PDKDF2
// using SHA256.
func symmetricCrypterFromPhraseAndState(phrase string, state string) (config.Crypter, error) {
	splits := strings.SplitN(state, ":", 3)
	if len(splits) != 3 {
		return nil, errors.New("malformed state value")
	}

	if splits[0] != "v1" {
		return nil, errors.New("unknown state version")
	}

	salt, err := base64.StdEncoding.DecodeString(splits[1])
	if err != nil {
		return nil, err
	}

	decrypter := config.NewSymmetricCrypterFromPassphrase(phrase, salt)
	decrypted, err := decrypter.DecryptValue(state[indexN(state, ":", 2)+1:])
	if err != nil || decrypted != "pulumi" {
		return nil, errors.New("incorrect passphrase")
	}

	return decrypter, nil
}

func indexN(s string, substr string, n int) int {
	contract.Require(n > 0, "n")
	scratch := s

	for i := n; i > 0; i-- {
		idx := strings.Index(scratch, substr)
		if i == -1 {
			return -1
		}

		scratch = scratch[idx+1:]
	}

	return len(s) - (len(scratch) + len(substr))
}
