// Copyright 2016-2018, Pulumi Corporation.
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

package filestate

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
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

// defaultCrypter gets the right value encrypter/decrypter given the project configuration.
func defaultCrypter(stackName tokens.QName, cfg config.Map, configFile string) (config.Crypter, error) {
	// If there is no config, we can use a standard panic crypter.
	if !cfg.HasSecureValue() {
		return config.NewPanicCrypter(), nil
	}

	// Otherwise, we will use an encrypted one.
	return symmetricCrypter(stackName, configFile)
}

// symmetricCrypter gets the right value encrypter/decrypter for this project.
func symmetricCrypter(stackName tokens.QName, configFile string) (config.Crypter, error) {
	contract.Assertf(stackName != "", "stackName %s", "!= \"\"")

	if configFile == "" {
		f, err := workspace.DetectProjectStackPath(stackName)
		if err != nil {
			return nil, err
		}
		configFile = f
	}

	info, err := workspace.LoadProjectStack(configFile)
	if err != nil {
		return nil, err
	}

	// If we have a salt, we can just use it.
	if info.EncryptionSalt != "" {
		phrase, phraseErr := readPassphrase("Enter your passphrase to unlock config/secrets\n" +
			"    (set PULUMI_CONFIG_PASSPHRASE to remember)")
		if phraseErr != nil {
			return nil, phraseErr
		}

		crypter, crypterErr := symmetricCrypterFromPhraseAndState(phrase, info.EncryptionSalt)
		if crypterErr != nil {
			return nil, crypterErr
		}

		return crypter, nil
	}

	// Here, the stack does not have an EncryptionSalt, so we will get a passphrase and create one
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
	contract.AssertNoError(err)

	// Now store the result and save it.
	info.EncryptionSalt = fmt.Sprintf("v1:%s:%s", base64.StdEncoding.EncodeToString(salt), msg)
	if err = info.Save(configFile); err != nil {
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
