// Copyright 2016, Pulumi Corporation.
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
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/rogpeppe/go-internal/lockedfile"
	goKeyring "github.com/zalando/go-keyring"
)

// PulumiCredentialsPathEnvVar is a path to the folder where credentials are stored.
// We use this in testing so that tests which log in and out do not impact the local developer's
// credentials or tests interacting with one another
//
//nolint:gosec
const PulumiCredentialsPathEnvVar = "PULUMI_CREDENTIALS_PATH"

const pulumiCredentialsKeyringService = "pulumi"

// GetAccount returns an account underneath a given key.
//
// Note that the account may not be fully populated: it may only have a valid AccessToken. In that case, it is up to
// the caller to fill in the username and last validation time.
func GetAccount(key string) (Account, error) {
	creds, err := GetStoredCredentials()
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

// DeleteAccount deletes an account underneath the given key.
func DeleteAccount(key string) error {
	creds, err := GetStoredCredentials()
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
	return StoreCredentials(creds)
}

func DeleteAllAccounts() error {
	if err := deleteAllKeyringTokens(); err != nil {
		return err
	}

	credsFile, err := getCredsFilePath()
	if err != nil {
		return err
	}

	if err = os.Remove(credsFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// StoreAccount saves the given account underneath the given key.
func StoreAccount(key string, account Account, current bool) error {
	creds, err := GetStoredCredentials()
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
	return StoreCredentials(creds)
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
	Name         string     `json:"name"`                   // The name of the token.
	Organization string     `json:"organization,omitempty"` //nolint:lll // If this was an organization token, the organization it was for.
	Team         string     `json:"team,omitempty"`         // If this was a team token, the team it was for.
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`    // The time when this token expires.
}

type AuthContext struct {
	GrantType    string
	Organization string
	Scope        string
	Token        string
	TokenExpired bool
	Expiration   time.Duration
}

//nolint:gosec // This is an OAuth grant type URN, not a credential
const AuthContextGrantTypeTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"

func NewAuthContextForTokenExchange(organization, team, user, token, expirationDuration string) (AuthContext, error) {
	if token == "" {
		return AuthContext{}, errors.New("oidc token must be specified for token exchange")
	}
	if env.AccessToken.Value() != "" {
		return AuthContext{}, errors.New("cannot perform token exchange when an access token is set as environment variable")
	}
	if organization == "" {
		return AuthContext{}, errors.New("organization must be specified for token exchange")
	}
	if team != "" && user != "" {
		return AuthContext{}, errors.New("only one of team or user may be specified for token exchange")
	}
	scope := ""
	if team != "" {
		scope = "team:" + team
	}
	if user != "" {
		scope = "user:" + user
	}
	expiration := 2 * time.Hour
	if expirationDuration != "" {
		duration, err := time.ParseDuration(expirationDuration)
		if err != nil {
			return AuthContext{}, fmt.Errorf("could not parse expiration duration: %w", err)
		}
		expiration = duration
	}
	return AuthContext{
		GrantType:    AuthContextGrantTypeTokenExchange,
		Organization: organization,
		Scope:        scope,
		Token:        token,
		Expiration:   expiration,
	}, nil
}

// Credentials hold the information necessary for authenticating Pulumi Cloud API requests.  It contains
// a map from the cloud API URL to the associated access token.
type Credentials struct {
	Current      string             `json:"current,omitempty"`      // the currently selected key.
	AccessTokens map[string]string  `json:"accessTokens,omitempty"` // a map of arbitrary key strings to tokens.
	Accounts     map[string]Account `json:"accounts,omitempty"`     // a map of arbitrary keys to account info.
}

// getCredsFilePath returns the path to the Pulumi credentials file on disk, regardless of
// whether it exists or not.
func getCredsFilePath() (string, error) {
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

	return filepath.Join(pulumiFolder, "credentials.json"), nil
}

// GetStoredCredentials returns any credentials stored on the local machine.
func GetStoredCredentials() (Credentials, error) {
	rawCreds, err := readCredentialsFile()
	if err != nil {
		return Credentials{}, err
	}

	creds, migrated, err := hydrateCredentials(rawCreds)
	if err != nil {
		return Credentials{}, err
	}

	if migrated {
		if err := writeCredentialsFile(credentialsMetadata(creds)); err != nil {
			return Credentials{}, err
		}
	}

	secrets := slice.Prealloc[string](len(creds.AccessTokens))
	for _, v := range creds.AccessTokens {
		secrets = append(secrets, v)
	}

	logging.AddGlobalFilter(logging.CreateFilter(secrets, "[credential]"))

	return creds, nil
}

// StoreCredentials updates the stored credentials on the machine, replacing the existing set.  If the credentials
// are empty, the auth file will be deleted rather than just serializing an empty map.
func StoreCredentials(creds Credentials) error {
	existingCreds, err := readCredentialsFile()
	if err != nil {
		return err
	}

	if !hasCredentialMetadata(creds) {
		if err := deleteAllKeyringTokens(); err != nil {
			return err
		}
		return deleteCredentialsFile()
	}

	tokens := credentialsTokens(creds)
	for key, token := range tokens {
		if err := setKeyringToken(key, token); err != nil {
			// Fall back to the legacy plaintext file when a secure store is unavailable.
			return writeCredentialsFile(creds)
		}
	}

	existingKeys := credentialKeys(existingCreds)
	for _, key := range existingKeys {
		if _, ok := tokens[key]; ok {
			continue
		}
		if err := deleteKeyringToken(key); err != nil {
			return err
		}
	}

	return writeCredentialsFile(credentialsMetadata(creds))
}

func readCredentialsFile() (Credentials, error) {
	credsFile, err := getCredsFilePath()
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

	var creds Credentials
	if err = json.Unmarshal(c, &creds); err != nil {
		return Credentials{}, fmt.Errorf("failed to read Pulumi credentials file. Please fix "+
			"or delete invalid credentials file: '%s': %w", credsFile, err)
	}

	return creds, nil
}

func writeCredentialsFile(creds Credentials) error {
	credsFile, err := getCredsFilePath()
	if err != nil {
		return err
	}

	if !hasCredentialMetadata(creds) {
		return deleteCredentialsFile()
	}

	raw, err := json.MarshalIndent(creds, "", "    ")
	if err != nil {
		return fmt.Errorf("marshalling credentials object: %w", err)
	}

	return lockedfile.Write(credsFile, bytes.NewReader(raw), 0o600)
}

func deleteCredentialsFile() error {
	credsFile, err := getCredsFilePath()
	if err != nil {
		return err
	}

	err = os.Remove(credsFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func hydrateCredentials(rawCreds Credentials) (Credentials, bool, error) {
	creds := rawCreds
	if creds.AccessTokens == nil {
		creds.AccessTokens = map[string]string{}
	}

	plaintextTokens := credentialsTokens(rawCreds)
	sawLegacyPlaintext := len(plaintextTokens) > 0
	allLegacyTokensMigrated := true

	for _, key := range credentialKeys(rawCreds) {
		token, hasPlaintext := plaintextTokens[key]
		if hasPlaintext {
			if err := setKeyringToken(key, token); err != nil {
				allLegacyTokensMigrated = false
			}
		}

		if !hasPlaintext {
			storedToken, err := getKeyringToken(key)
			if err == nil {
				token = storedToken
			} else if !errors.Is(err, goKeyring.ErrNotFound) {
				return Credentials{}, false, err
			}
		}

		if token == "" {
			delete(creds.AccessTokens, key)
			if account, ok := creds.Accounts[key]; ok {
				account.AccessToken = ""
				creds.Accounts[key] = account
			}
			continue
		}

		creds.AccessTokens[key] = token
		if account, ok := creds.Accounts[key]; ok {
			account.AccessToken = token
			creds.Accounts[key] = account
		}
	}

	return creds, sawLegacyPlaintext && allLegacyTokensMigrated, nil
}

func credentialsTokens(creds Credentials) map[string]string {
	tokens := map[string]string{}
	for key, token := range creds.AccessTokens {
		if token != "" {
			tokens[key] = token
		}
	}
	for key, account := range creds.Accounts {
		if account.AccessToken != "" {
			tokens[key] = account.AccessToken
		}
	}
	return tokens
}

func credentialKeys(creds Credentials) []string {
	keys := map[string]struct{}{}
	for key := range creds.AccessTokens {
		keys[key] = struct{}{}
	}
	for key := range creds.Accounts {
		keys[key] = struct{}{}
	}

	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	slices.Sort(result)
	return result
}

func credentialsMetadata(creds Credentials) Credentials {
	metadata := Credentials{
		Current:      creds.Current,
		AccessTokens: map[string]string{},
		Accounts:     map[string]Account{},
	}

	for key := range credentialsTokens(creds) {
		metadata.AccessTokens[key] = ""
	}
	if len(metadata.AccessTokens) == 0 {
		metadata.AccessTokens = nil
	}

	for key, account := range creds.Accounts {
		account.AccessToken = ""
		metadata.Accounts[key] = account
	}
	if len(metadata.Accounts) == 0 {
		metadata.Accounts = nil
	}

	return metadata
}

func hasCredentialMetadata(creds Credentials) bool {
	return creds.Current != "" || len(creds.Accounts) > 0 || len(creds.AccessTokens) > 0
}

func keyringUser(key string) string {
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("backend-%x", sum)
}

func getKeyringToken(key string) (string, error) {
	token, err := goKeyring.Get(pulumiCredentialsKeyringService, keyringUser(key))
	if err != nil {
		if errors.Is(err, goKeyring.ErrUnsupportedPlatform) {
			return "", err
		}
		if errors.Is(err, goKeyring.ErrNotFound) {
			return "", err
		}
		return "", fmt.Errorf("reading secure credentials for %q: %w", key, err)
	}
	return token, nil
}

func setKeyringToken(key string, token string) error {
	err := goKeyring.Set(pulumiCredentialsKeyringService, keyringUser(key), token)
	if err != nil {
		if errors.Is(err, goKeyring.ErrUnsupportedPlatform) {
			return err
		}
		return fmt.Errorf("writing secure credentials for %q: %w", key, err)
	}
	return nil
}

func deleteKeyringToken(key string) error {
	err := goKeyring.Delete(pulumiCredentialsKeyringService, keyringUser(key))
	if err != nil && !errors.Is(err, goKeyring.ErrNotFound) && !errors.Is(err, goKeyring.ErrUnsupportedPlatform) {
		return fmt.Errorf("deleting secure credentials for %q: %w", key, err)
	}
	return nil
}

func deleteAllKeyringTokens() error {
	err := goKeyring.DeleteAll(pulumiCredentialsKeyringService)
	if err != nil && !errors.Is(err, goKeyring.ErrUnsupportedPlatform) {
		return fmt.Errorf("deleting secure credentials: %w", err)
	}
	return nil
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
	err = os.Rename(tempConfigFile.Name(), configFile) //nolint:forbidigo // historic usage
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
