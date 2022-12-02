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

// Package passphrase implements support for a local passphrase secret manager.
package main

/*
import (
	"bufio"
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)


// PromptForNewPassphrase prompts for a new passphrase, and returns the state and the secrets manager.
func PromptForNewPassphrase(rotate bool) (string, secrets.Manager, error) {
	var phrase string

	// Get a the passphrase from the user, ensuring that they match.
	for {
		firstMessage := "Enter your passphrase to protect config/secrets"
		if rotate {
			firstMessage = "Enter your new passphrase to protect config/secrets"

			if !isInteractive() {
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()
				phrase = strings.TrimSpace(scanner.Text())
				break
			}
		}
		// Here, the stack does not have an EncryptionSalt, so we will get a passphrase and create one
		first, _, err := readPassphrase(firstMessage, !rotate)
		if err != nil {
			return "", nil, err
		}
		secondMessage := "Re-enter your passphrase to confirm"
		if rotate {
			secondMessage = "Re-enter your new passphrase to confirm"
		}
		second, _, err := readPassphrase(secondMessage, !rotate)
		if err != nil {
			return "", nil, err
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
	_, err := cryptorand.Read(salt)
	contract.AssertNoErrorf(err, "could not read from system random")

	// Encrypt a message and store it with the salt so we can test if the password is correct later.
	crypter := config.NewSymmetricCrypterFromPassphrase(phrase, salt)

	// symmetricCrypter does not use ctx, safe to use context.Background()
	ignoredCtx := context.Background()
	msg, err := crypter.EncryptValue(ignoredCtx, "pulumi")
	contract.AssertNoError(err)

	// Encode the salt as the passphrase secrets manager state.
	state := fmt.Sprintf("v1:%s:%s", base64.StdEncoding.EncodeToString(salt), msg)

	// Create the secrets manager using the state.
	sm, err := NewPassphaseSecretsManager(phrase, state)
	if err != nil {
		return "", nil, err
	}

	// Return both the state and the secrets manager.
	return state, sm, nil
}

// newLockedPasspharseSecretsManager returns a Passphrase secrets manager that has the correct state, but can not
// encrypt or decrypt anything. This is helpful today for some cases, because we have operations that roundtrip
// checkpoints and we'd like to continue to support these operations even if we don't have the correct passphrase. But
// if we never end up having to call encrypt or decrypt, this provider will be sufficient.  Since it has the correct
// state, we ensure that when we roundtrip, we don't lose the state stored in the deployment.
func newLockedPasspharseSecretsManager(state localSecretsManagerState) secrets.Manager {
	return &localSecretsManager{
		state:   state,
		crypter: &errorCrypter{},
	}
}

type errorCrypter struct{}

func (ec *errorCrypter) EncryptValue(ctx context.Context, _ string) (string, error) {
	return "", errors.New("failed to encrypt: incorrect passphrase, please set PULUMI_CONFIG_PASSPHRASE to the " +
		"correct passphrase or set PULUMI_CONFIG_PASSPHRASE_FILE to a file containing the passphrase")
}

func (ec *errorCrypter) DecryptValue(ctx context.Context, _ string) (string, error) {
	return "", errors.New("failed to decrypt: incorrect passphrase, please set PULUMI_CONFIG_PASSPHRASE to the " +
		"correct passphrase or set PULUMI_CONFIG_PASSPHRASE_FILE to a file containing the passphrase")
}

func (ec *errorCrypter) BulkDecrypt(ctx context.Context, _ []string) (map[string]string, error) {
	return nil, errors.New("failed to decrypt: incorrect passphrase, please set PULUMI_CONFIG_PASSPHRASE to the " +
		"correct passphrase or set PULUMI_CONFIG_PASSPHRASE_FILE to a file containing the passphrase")
}

func NewPassphraseSecretsManager(stackName tokens.Name, configFile string,
	rotatePassphraseSecretsProvider bool) (secrets.Manager, error) {
	contract.Assertf(stackName != "", "stackName %s", "!= \"\"")

	project, _, err := workspace.DetectProjectStackPath(stackName.Q())
	if err != nil {
		return nil, err
	}

	info, err := workspace.LoadProjectStack(project, configFile)
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
		return passphrase.NewPromptingPassphraseSecretsManager(info.EncryptionSalt)
	}

	// Otherwise, prompt the user for a new passphrase.
	salt, sm, err := passphrase.PromptForNewPassphrase(rotatePassphraseSecretsProvider)
	if err != nil {
		return nil, err
	}

	// Store the salt and save it.
	info.EncryptionSalt = salt
	if err = info.Save(configFile); err != nil {
		return nil, err
	}

	// Return the passphrase secrets manager.
	return sm, nil
}
*/
