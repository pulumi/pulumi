// Copyright 2016-2021, Pulumi Corporation.
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

package workspace

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/keystore"
	"github.com/rogpeppe/go-internal/lockedfile"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// PulumiCredentialsPathEnvVar is a path to the folder where credentials are stored.
// We use this in testing so that tests which log in and out do not impact the local developer's
// credentials or tests interacting with one another
//
//nolint:gosec
const PulumiCredentialsPathEnvVar = "PULUMI_CREDENTIALS_PATH"

// GetAccount returns an account underneath a given key.
//
// Note that the account may not be fully populated: it may only have a valid AccessToken. In that case, it is up to
// the caller to fill in the username and last validation time.
func GetAccountWithKeyStore(ks keystore.KeyStore, key string) (Account, error) {
	creds, err := GetStoredCredentialsWithKeyStore(ks)
	if err != nil && !os.IsNotExist(err) {
		return Account{}, err
	}

	// Try the account
	if account, ok := creds.Accounts[key]; ok {
		return account, nil
	}
	token, ok := creds.AccessTokens[key]
	if !ok {
		return Account{}, nil
	}
	return Account{AccessToken: token}, nil
}

func GetAccount(key string) (Account, error) {
	// Hack for ESC CLI dependency
	return Account{}, nil
}

// DeleteAccountWithKeyStore deletes an account underneath the given key.
func DeleteAccountWithKeyStore(ks keystore.KeyStore, key string) error {
	creds, err := GetStoredCredentialsWithKeyStore(ks)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if creds.AccessTokens != nil {
		delete(creds.AccessTokens, key)
	}
	if creds.Accounts != nil {
		delete(creds.Accounts, key)
	}
	if creds.Current == key {
		creds.Current = ""
	}
	return storeCredentials(ks, creds)
}

func DeleteAccount(key string) error {
	// Hack for ESC CLI dependency
	return nil
}

func DeleteAllAccountsWithKeyStore(ks keystore.KeyStore) error {
	credsFile, err := getCredsFilePath(ks)
	if err != nil {
		return err
	}

	if err = os.Remove(credsFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func DeleteAllAccounts() error {
	// Hack for ESC CLI dependency
	return nil
}

// StoreAccountWithKeyStore saves the given account underneath the given key.
func StoreAccountWithKeyStore(ks keystore.KeyStore, key string, account Account, current bool) error {
	creds, err := GetStoredCredentialsWithKeyStore(ks)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if creds.AccessTokens == nil {
		creds.AccessTokens = make(map[string]string)
	}
	if creds.Accounts == nil {
		creds.Accounts = make(map[string]Account)
	}
	creds.AccessTokens[key], creds.Accounts[key] = account.AccessToken, account
	if current {
		creds.Current = key
	}
	return storeCredentials(ks, creds)
}

func StoreAccount(key string, account Account, current bool) error {
	// Hack for ESC CLI dependency
	return nil
}

// Account holds the information associated with a Pulumi account.
type Account struct {
	// The access token for this account.
	AccessToken string `json:"accessToken,omitempty"`
	// The username for this account.
	Username string `json:"username,omitempty"`
	// The organizations for this account.
	Organizations []string `json:"organizations,omitempty"`
	// The last time this token was validated.
	LastValidatedAt time.Time `json:"lastValidatedAt,omitempty"`
	// Allow insecure server connections when using SSL.
	Insecure bool `json:"insecure,omitempty"`
	// Information about the token used to authenticate.
	TokenInformation *TokenInformation `json:"tokenInformation,omitempty"`
}

// Information about the token that was used to authenticate the current user. One (or none) of Team or Organization
// will be set, but not both.
type TokenInformation struct {
	Name         string `json:"name"`                   // The name of the token.
	Organization string `json:"organization,omitempty"` // If this was an organization token, the organization it was for.
	Team         string `json:"team,omitempty"`         // If this was a team token, the team it was for.
}

// Credentials hold the information necessary for authenticating Pulumi Cloud API requests.  It contains
// a map from the cloud API URL to the associated access token.
type Credentials struct {
	Current      string             `json:"current,omitempty"`      // the currently selected key.
	AccessTokens map[string]string  `json:"accessTokens,omitempty"` // a map of arbitrary key strings to tokens.
	Accounts     map[string]Account `json:"accounts,omitempty"`     // a map of arbitrary keys to account info.
}

func getCredsFolderPath() (string, error) {
	// Allow the folder we use to store credentials to be overridden by tests
	pulumiFolder := os.Getenv(PulumiCredentialsPathEnvVar)
	if pulumiFolder == "" {
		folder, err := GetPulumiHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get the home path: %w", err)
		}
		pulumiFolder = folder
	}

	err := os.MkdirAll(pulumiFolder, 0o700)
	if err != nil {
		return "", fmt.Errorf("failed to create '%s': %w", pulumiFolder, err)
	}

	return pulumiFolder, nil
}

// getCredsFilePath returns the path to the Pulumi credentials file on disk, regardless of
// whether it exists or not.
func getCredsFilePath(ks keystore.KeyStore) (string, error) {
	pulumiFolder, err := getCredsFolderPath()
	if err != nil {
		return "", err
	}
	fileName := fmt.Sprintf("credentials.%s.json", ks.Name)
	return filepath.Join(pulumiFolder, fileName), nil
}

// GetStoredCredentialsWithKeyStore returns any credentials stored on the local machine.
func GetStoredCredentialsWithKeyStore(ks keystore.KeyStore) (Credentials, error) {
	err := migrateCredentials(ks)
	if err != nil {
		return Credentials{}, fmt.Errorf("migrating credentials: %w", err)
	}

	credsFile, err := getCredsFilePath(ks)
	if err != nil {
		return Credentials{}, err
	}

	c, err := lockedfile.Read(credsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return Credentials{}, nil
		}
		return Credentials{}, fmt.Errorf("reading '%s': %w", credsFile, err)
	}

	// If the file is empty, we can act as if it doesn't exist rather than trying
	// (and failing) to deserialize its contents. This allows us to recover from
	// situations where a write to the file was interrupted or it was otherwise
	// clobbered.
	if len(c) == 0 {
		return Credentials{}, nil
	}

	key, err := ks.GetOrCreateKey()
	if err != nil {
		return Credentials{}, fmt.Errorf("getting key: %w", err)
	}
	creds, err := decryptCredentials(key, c)
	if err != nil {
		return Credentials{}, fmt.Errorf("decrypting credentials: %w", err)
	}

	secrets := slice.Prealloc[string](len(creds.AccessTokens))
	for _, v := range creds.AccessTokens {
		secrets = append(secrets, v)
	}

	logging.AddGlobalFilter(logging.CreateFilter(secrets, "[credential]"))

	return creds, nil
}

// TODO: If pulumi CLI migrates credentials it deletes for example ESC CLI credentials. How to handle this?
//  1. Use the static key keystore if a legacy file exists?
//  2. Don't care and let user login again
func migrateCredentials(ks keystore.KeyStore) error {
	credsFolder, err := getCredsFolderPath()
	if err != nil {
		return err
	}
	credsFile := filepath.Join(credsFolder, "credentials.json")

	c, err := os.ReadFile(credsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading '%s': %w", credsFile, err)
	}

	var legacyCreds Credentials
	if err = json.Unmarshal(c, &legacyCreds); err == nil {
		err = storeCredentials(ks, legacyCreds)
		if err != nil {
			return err
		}
		if err = os.Remove(credsFile); err != nil {
			return err
		}
	}

	return nil
}

func GetStoredCredentials() (Credentials, error) {
	// Hack for ESC CLI dependency
	return Credentials{}, nil
}

// storeCredentials updates the stored credentials on the machine, replacing the existing set.  If the credentials
// are empty, the auth file will be deleted rather than just serializing an empty map.
func storeCredentials(ks keystore.KeyStore, creds Credentials) error {
	key, err := ks.GetOrCreateKey()
	if err != nil {
		return fmt.Errorf("getting key: %w", err)
	}
	raw, err := encryptCredentials(key, creds)
	if err != nil {
		return fmt.Errorf("encrypting credentials: %w", err)
	}
	credsFile, err := getCredsFilePath(ks)
	return lockedfile.Write(credsFile, bytes.NewReader(raw), 0o600)
}

func encryptCredentials(key []byte, creds Credentials) ([]byte, error) {
	plaintext, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("marshalling credentials: %w", err)
	}

	nonce := make([]byte, 12)
	_, err = cryptorand.Read(nonce)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)

	return slices.Concat(nonce, ciphertext), nil
}

func decryptCredentials(key []byte, nonceCiphertext []byte) (Credentials, error) {
	nonce := nonceCiphertext[:12]
	ciphertext := nonceCiphertext[12:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return Credentials{}, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return Credentials{}, err
	}
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)

	var creds Credentials
	if err = json.Unmarshal(plaintext, &creds); err != nil {
		return Credentials{}, err
	}
	return creds, nil
}

type BackendConfig struct {
	DefaultOrg string `json:"defaultOrg,omitempty"` // The default org for this backend config.
}

type PulumiConfig struct {
	BackendConfig map[string]BackendConfig `json:"backends,omitempty"` // a map of arbitrary backends configs.
}

func getConfigFilePath() (string, error) {
	// Allow the folder we use to store config in to be overridden by tests
	pulumiFolder := os.Getenv(PulumiCredentialsPathEnvVar)
	if pulumiFolder == "" {
		folder, err := GetPulumiHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get the home path: %w", err)
		}
		pulumiFolder = folder
	}

	err := os.MkdirAll(pulumiFolder, 0o700)
	if err != nil {
		return "", fmt.Errorf("failed to create '%s': %w", pulumiFolder, err)
	}

	return filepath.Join(pulumiFolder, "config.json"), nil
}

func GetPulumiConfig() (PulumiConfig, error) {
	configFile, err := getConfigFilePath()
	if err != nil {
		return PulumiConfig{}, err
	}

	c, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return PulumiConfig{}, nil
		}
		return PulumiConfig{}, fmt.Errorf("reading '%s': %w", configFile, err)
	}

	var config PulumiConfig
	if err = json.Unmarshal(c, &config); err != nil {
		return PulumiConfig{}, fmt.Errorf("failed to read Pulumi config file: %w", err)
	}

	return config, nil
}

func StorePulumiConfig(config PulumiConfig) error {
	configFile, err := getConfigFilePath()
	if err != nil {
		return err
	}

	raw, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("marshalling config object: %w", err)
	}

	// Use a temporary file and atomic os.Rename to ensure the file contents are
	// updated atomically to ensure concurrent `pulumi` CLI operations are safe.
	tempConfigFile, err := os.CreateTemp(filepath.Dir(configFile), "config-*.json")
	if err != nil {
		return err
	}
	_, err = tempConfigFile.Write(raw)
	if err != nil {
		return err
	}
	err = tempConfigFile.Close()
	if err != nil {
		return err
	}
	err = os.Rename(tempConfigFile.Name(), configFile)
	if err != nil {
		contract.IgnoreError(os.Remove(tempConfigFile.Name()))
		return err
	}

	return nil
}

func SetBackendConfigDefaultOrg(backendURL, defaultOrg string) error {
	config, err := GetPulumiConfig()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if config.BackendConfig == nil {
		config.BackendConfig = make(map[string]BackendConfig)
	}

	config.BackendConfig[backendURL] = BackendConfig{
		DefaultOrg: defaultOrg,
	}

	return StorePulumiConfig(config)
}
