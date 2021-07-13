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

package main

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func readPassphraseImpl(prompt string, useEnv bool) (phrase string, interactive bool, err error) {
	if useEnv {
		if phrase, ok := os.LookupEnv("PULUMI_CONFIG_PASSPHRASE"); ok {
			return phrase, false, nil
		}
		if phraseFile, ok := os.LookupEnv("PULUMI_CONFIG_PASSPHRASE_FILE"); ok {
			phraseFilePath, err := filepath.Abs(phraseFile)
			if err != nil {
				return "", false, errors.Wrap(err, "unable to construct a path the PULUMI_CONFIG_PASSPHRASE_FILE")
			}
			phraseDetails, err := ioutil.ReadFile(phraseFilePath)
			if err != nil {
				return "", false, errors.Wrap(err, "unable to read PULUMI_CONFIG_PASSPHRASE_FILE")
			}
			return strings.TrimSpace(string(phraseDetails)), false, nil
		}
		if !cmdutil.Interactive() {
			return "", false, errors.New("passphrase must be set with PULUMI_CONFIG_PASSPHRASE or " +
				"PULUMI_CONFIG_PASSPHRASE_FILE environment variables")
		}
	}
	phrase, err = cmdutil.ReadConsoleNoEcho(prompt)
	return phrase, true, err
}

func readPassphrase(prompt string) (phrase string, interactive bool, err error) {
	return readPassphraseImpl(prompt, true)
}

func newPassphraseSecretsManager(stackName tokens.QName, configFile string,
	rotatePassphraseSecretsProvider bool) (secrets.Manager, error) {
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

	if rotatePassphraseSecretsProvider {
		info.EncryptionSalt = ""
	}

	// If there are any other secrets providers set in the config, remove them, as the passphrase
	// provider deals only with EncryptionSalt, not EncryptedKey or SecretsProvider.
	if info.EncryptedKey != "" || info.SecretsProvider != "" {
		info.EncryptedKey = ""
		info.SecretsProvider = ""
	}

	// If we have a salt, we can just use it.
	if info.EncryptionSalt != "" {
		for {
			phrase, interactive, phraseErr := readPassphrase("Enter your passphrase to unlock config/secrets\n" +
				"    (set PULUMI_CONFIG_PASSPHRASE or PULUMI_CONFIG_PASSPHRASE_FILE to remember)")
			if phraseErr != nil {
				return nil, phraseErr
			}

			sm, smerr := passphrase.NewPassphaseSecretsManager(phrase, info.EncryptionSalt)
			switch {
			case interactive && smerr == passphrase.ErrIncorrectPassphrase:
				cmdutil.Diag().Errorf(diag.Message("", "incorrect passphrase"))
				continue
			case smerr != nil:
				return nil, smerr
			default:
				return sm, nil
			}
		}
	}

	var phrase string

	// Get a the passphrase from the user, ensuring that they match.
	for {
		firstMessage := "Enter your passphrase to protect config/secrets"
		if rotatePassphraseSecretsProvider {
			firstMessage = "Enter your new passphrase to protect config/secrets"

			if test, ok := os.LookupEnv("PULUMI_TEST_PASSPHRASE"); (!ok || !cmdutil.IsTruthy(test)) && !cmdutil.Interactive() {
				return nil, fmt.Errorf("passphrase rotation requires an interactive terminal")
			}
		}
		// Here, the stack does not have an EncryptionSalt, so we will get a passphrase and create one
		first, _, err := readPassphraseImpl(firstMessage, !rotatePassphraseSecretsProvider)
		if err != nil {
			return nil, err
		}
		secondMessage := "Re-enter your passphrase to confirm"
		if rotatePassphraseSecretsProvider {
			secondMessage = "Re-enter your new passphrase to confirm"
		}
		second, _, err := readPassphraseImpl(secondMessage, !rotatePassphraseSecretsProvider)
		if err != nil {
			return nil, err
		}

		if first == second {
			phrase = first
			break
		}
		// If they didn't match, print an error and try again
		cmdutil.Diag().Errorf(diag.Message("", "passphrases do not match"))
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

	// Finally, build the full secrets manager from the state we just saved
	return passphrase.NewPassphaseSecretsManager(phrase, info.EncryptionSalt)
}
