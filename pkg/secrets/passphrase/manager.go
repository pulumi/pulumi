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
package passphrase

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
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const Type = "passphrase"

var ErrIncorrectPassphrase = errors.New("incorrect passphrase")

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
	// symmetricCrypter does not use ctx, safe to pass context.Background()
	ignoredCtx := context.Background()
	decrypted, err := decrypter.DecryptValue(ignoredCtx, state[indexN(state, ":", 2)+1:])
	if err != nil || decrypted != "pulumi" {
		return nil, ErrIncorrectPassphrase
	}

	return decrypter, nil
}

func indexN(s string, substr string, n int) int {
	contract.Requiref(n > 0, "n", "must be greater than 0")
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

type localSecretsManagerState struct {
	Salt string `json:"salt"`
}

var _ secrets.Manager = &localSecretsManager{}

type localSecretsManager struct {
	state   json.RawMessage
	crypter config.Crypter
}

func (sm *localSecretsManager) Type() string {
	return Type
}

func (sm *localSecretsManager) State() json.RawMessage {
	return sm.state
}

func (sm *localSecretsManager) Decrypter() (config.Decrypter, error) {
	contract.Assertf(sm.crypter != nil, "decrypter not initialized")
	return sm.crypter, nil
}

func (sm *localSecretsManager) Encrypter() (config.Encrypter, error) {
	contract.Assertf(sm.crypter != nil, "encrypter not initialized")
	return sm.crypter, nil
}

func EditProjectStack(info *workspace.ProjectStack, state json.RawMessage) error {
	info.EncryptedKey = ""
	info.SecretsProvider = ""

	var s localSecretsManagerState
	err := json.Unmarshal(state, &s)
	if err != nil {
		return fmt.Errorf("unmarshalling passphrase state: %w", err)
	}
	info.EncryptionSalt = s.Salt
	return nil
}

var (
	lock  sync.Mutex
	cache map[string]secrets.Manager
)

// clearCachedSecretsManagers is used to clear the cache, for tests.
func clearCachedSecretsManagers() {
	lock.Lock()
	defer lock.Unlock()
	cache = nil
}

// getCachedSecretsManager returns a cached secret manager and true, or nil and false if not in the cache.
func getCachedSecretsManager(state string) (secrets.Manager, bool) {
	lock.Lock()
	defer lock.Unlock()
	sm, ok := cache[state]
	return sm, ok
}

// setCachedSecretsManager saves a secret manager in the cache.
func setCachedSecretsManager(state string, sm secrets.Manager) {
	lock.Lock()
	defer lock.Unlock()
	if cache == nil {
		cache = make(map[string]secrets.Manager)
	}
	cache[state] = sm
}

func NewPassphraseSecretsManager(phrase string) (string, secrets.Manager, error) {
	// Produce a new salt.
	salt := make([]byte, 8)
	_, err := cryptorand.Read(salt)
	contract.AssertNoErrorf(err, "could not read from system random")

	// Encrypt a message and store it with the salt so we can test if the password is correct later.
	crypter := config.NewSymmetricCrypterFromPassphrase(phrase, salt)

	// symmetricCrypter does not use ctx, safe to use context.Background()
	ignoredCtx := context.Background()
	msg, err := crypter.EncryptValue(ignoredCtx, "pulumi")
	contract.AssertNoErrorf(err, "could not encrypt message")

	// Encode the salt as the passphrase secrets manager state.
	state := fmt.Sprintf("v1:%s:%s", base64.StdEncoding.EncodeToString(salt), msg)
	jsonState, err := json.Marshal(localSecretsManagerState{
		Salt: state,
	})
	if err != nil {
		return "", nil, fmt.Errorf("marshalling state: %w", err)
	}

	sm := &localSecretsManager{
		crypter: crypter,
		state:   jsonState,
	}
	return state, sm, nil
}

func GetPassphraseSecretsManager(phrase string, state string) (secrets.Manager, error) {
	// Check the cache first, if we have already seen this state before, return a cached value.
	if cached, ok := getCachedSecretsManager(state); ok {
		return cached, nil
	}

	// Wasn't in the cache so try to construct it and add it if there's no error.
	crypter, err := symmetricCrypterFromPhraseAndState(phrase, state)
	if err != nil {
		return nil, err
	}

	jsonState, err := json.Marshal(localSecretsManagerState{
		Salt: state,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling state: %w", err)
	}

	sm := &localSecretsManager{
		crypter: crypter,
		state:   jsonState,
	}
	setCachedSecretsManager(state, sm)
	return sm, nil
}

// newPromptingPassphraseSecretsManagerFromState returns a new passphrase-based secrets manager, from the
// given state. Will use the passphrase found in PULUMI_CONFIG_PASSPHRASE, the file specified by
// PULUMI_CONFIG_PASSPHRASE_FILE, or otherwise will prompt for the passphrase if interactive.
func newPromptingPassphraseSecretsManagerFromState(state string) (secrets.Manager, error) {
	// Check the cache first, if we have already seen this state before, return a cached value.
	if cached, ok := getCachedSecretsManager(state); ok {
		return cached, nil
	}

	// Otherwise, prompt for the password.
	const prompt = "Enter your passphrase to unlock config/secrets\n" +
		"    (set PULUMI_CONFIG_PASSPHRASE or PULUMI_CONFIG_PASSPHRASE_FILE to remember)"
	for {
		phrase, interactive, phraseErr := readPassphrase(prompt, true /*useEnv*/)
		if phraseErr != nil {
			return nil, phraseErr
		}

		sm, smerr := GetPassphraseSecretsManager(phrase, state)
		switch {
		case interactive && smerr == ErrIncorrectPassphrase:
			cmdutil.Diag().Errorf(diag.Message("", "incorrect passphrase"))
			continue
		case smerr != nil:
			return nil, smerr
		default:
			return sm, nil
		}
	}
}

// NewPassphraseSecretsManager returns a new passphrase-based secrets manager, from the
// given state. Will use the passphrase found in PULUMI_CONFIG_PASSPHRASE, the file specified by
// PULUMI_CONFIG_PASSPHRASE_FILE, or otherwise will prompt for the passphrase if interactive.
func NewPromptingPassphraseSecretsManagerFromState(state json.RawMessage) (secrets.Manager, error) {
	var s localSecretsManagerState
	if err := json.Unmarshal(state, &s); err != nil {
		return nil, fmt.Errorf("unmarshalling state: %w", err)
	}

	sm, err := newPromptingPassphraseSecretsManagerFromState(s.Salt)
	switch {
	case err == ErrIncorrectPassphrase:
		return newLockedPasspharseSecretsManager(state), nil
	case err != nil:
		return nil, fmt.Errorf("constructing secrets manager: %w", err)
	default:
		return sm, nil
	}
}

func NewPromptingPassphraseSecretsManager(info *workspace.ProjectStack,
	rotateSecretsProvider bool,
) (secrets.Manager, error) {
	if rotateSecretsProvider {
		info.EncryptionSalt = ""
	}

	// If there are any other secrets providers set in the config, remove them, as the passphrase
	// provider deals only with EncryptionSalt, not EncryptedKey or SecretsProvider.
	info.EncryptedKey = ""
	info.SecretsProvider = ""

	// If we have a salt, we can just use it.
	if info.EncryptionSalt != "" {
		return newPromptingPassphraseSecretsManagerFromState(info.EncryptionSalt)
	}

	// Otherwise, prompt the user for a new passphrase.
	state, sm, err := promptForNewPassphrase(rotateSecretsProvider)
	if err != nil {
		return nil, err
	}

	// Store the salt and save it.
	info.EncryptionSalt = state

	// Return the passphrase secrets manager.
	return sm, nil
}

// promptForNewPassphrase prompts for a new passphrase, and returns the state and the secrets manager.
func promptForNewPassphrase(rotate bool) (string, secrets.Manager, error) {
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

	state, sm, err := NewPassphraseSecretsManager(phrase)
	if err != nil {
		return "", nil, err
	}
	setCachedSecretsManager(state, sm)
	return state, sm, err
}

func readPassphrase(prompt string, useEnv bool) (phrase string, interactive bool, err error) {
	if useEnv {
		if phrase, ok := os.LookupEnv("PULUMI_CONFIG_PASSPHRASE"); ok {
			return phrase, false, nil
		}
		if phraseFile, ok := os.LookupEnv("PULUMI_CONFIG_PASSPHRASE_FILE"); ok && phraseFile != "" {
			phraseFilePath, err := filepath.Abs(phraseFile)
			if err != nil {
				return "", false, fmt.Errorf("unable to construct a path the PULUMI_CONFIG_PASSPHRASE_FILE: %w", err)
			}
			phraseDetails, err := os.ReadFile(phraseFilePath)
			if err != nil {
				return "", false, fmt.Errorf("unable to read PULUMI_CONFIG_PASSPHRASE_FILE: %w", err)
			}
			return strings.TrimSpace(string(phraseDetails)), false, nil
		}
		if !isInteractive() {
			return "", false, errors.New("passphrase must be set with PULUMI_CONFIG_PASSPHRASE or " +
				"PULUMI_CONFIG_PASSPHRASE_FILE environment variables")
		}
	}
	phrase, err = cmdutil.ReadConsoleNoEcho(prompt)
	return phrase, true, err
}

func isInteractive() bool {
	test, ok := os.LookupEnv("PULUMI_TEST_PASSPHRASE")
	return cmdutil.Interactive() || ok && cmdutil.IsTruthy(test)
}

// newLockedPasspharseSecretsManager returns a Passphrase secrets manager that has the correct state, but can not
// encrypt or decrypt anything. This is helpful today for some cases, because we have operations that roundtrip
// checkpoints and we'd like to continue to support these operations even if we don't have the correct passphrase. But
// if we never end up having to call encrypt or decrypt, this provider will be sufficient.  Since it has the correct
// state, we ensure that when we roundtrip, we don't lose the state stored in the deployment.
func newLockedPasspharseSecretsManager(state json.RawMessage) secrets.Manager {
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
