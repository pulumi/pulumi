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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/rogpeppe/go-internal/lockedfile"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// PulumiCredentialsPathEnvVar is a path to the folder where credentials are stored.
// We use this in testing so that tests which log in and out do not impact the local developer's
// credentials or tests interacting with one another
//
//nolint:gosec
const PulumiCredentialsPathEnvVar = "PULUMI_CREDENTIALS_PATH"

// Account holds the information associated with a Pulumi account.
type Account struct { // The access token for this account.
	AccessToken string `json:"accessToken,omitempty"`
	// The OAuth refresh token, if the server issued one alongside the access token. When set, the
	// CLI exchanges this token at /api/oauth/token for a fresh access token whenever the current
	// one expires. Held off-the-wire — only sent to the token endpoint, not on every request.
	RefreshToken string `json:"refreshToken,omitempty"`
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

	// sourcePath is the credentials file this account was loaded from. Set by the loaders
	// (GetAccount, GetAgentAccount, GetAccountWithAgentFallback); empty for accounts constructed
	// in memory (env-var tokens before persistence, test literals, fresh-login pre-persist).
	// Used by Save to persist credential refreshes back to the file the account came from.
	sourcePath string
}

// HasCredential reports whether this account carries anything the CLI can use to authenticate —
// either a current access token or a refresh token to mint one with. Used at credential-selection
// time so that an account with only a refresh token is treated as usable rather than skipped.
func (a Account) HasCredential() bool {
	return a.AccessToken != "" || a.RefreshToken != ""
}

// SetCredentials writes a credential-grant result to this account. A zero
// accessTokenExpiresAt or empty refreshToken keeps the existing value — for grant
// responses that omit the field (e.g. a refresh that didn't rotate the refresh token).
func (a *Account) SetCredentials(accessToken string, accessTokenExpiresAt time.Time, refreshToken string) {
	a.AccessToken = accessToken
	if !accessTokenExpiresAt.IsZero() {
		if a.TokenInformation == nil {
			a.TokenInformation = &TokenInformation{}
		}
		a.TokenInformation.ExpiresAt = &accessTokenExpiresAt
	}
	if refreshToken != "" {
		a.RefreshToken = refreshToken
	}
}

// Information about the token that was used to authenticate the current user. One (or none) of Team or Organization
// will be set, but not both.
type TokenInformation struct {
	Name         string     `json:"name"`                   // The name of the token.
	Organization string     `json:"organization,omitempty"` //nolint:lll // If this was an organization token, the organization it was for.
	Team         string     `json:"team,omitempty"`         // If this was a team token, the team it was for.
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`    // The time when this token expires.
}

// Credentials hold the information necessary for authenticating Pulumi Cloud API requests.  It contains
// a map from the cloud API URL to the associated access token.
type Credentials struct {
	Current      string             `json:"current,omitempty"`      // the currently selected key.
	AccessTokens map[string]string  `json:"accessTokens,omitempty"` // a map of arbitrary key strings to tokens.
	Accounts     map[string]Account `json:"accounts,omitempty"`     // a map of arbitrary keys to account info.
}

// GetAccountWithAgentFallback returns an account from default credentials, or from shared
// temporary agent credentials when running in a detected agent environment without an explicit
// Pulumi credential path.
func GetAccountWithAgentFallback(key string) (Account, bool, error) {
	account, err := getAccount(key)
	if err == nil && account.HasCredential() {
		return account, false, nil
	}

	agent := agentdetect.Detect(os.Getenv)
	if agent == "" || hasExplicitPulumiPathEnv() {
		return account, false, err
	}

	agentAccount, agentErr := getAgentAccount(key)
	if agentErr != nil {
		return Account{}, false, errors.Join(err, agentErr)
	}
	if !agentAccount.HasCredential() {
		return Account{}, false, nil
	}
	return agentAccount, true, nil
}

func getAccount(key string) (Account, error) {
	path, err := getCredsFilePath()
	if err != nil {
		return Account{}, err
	}
	return getAccountAt(path, key)
}

func getAccountAt(path, key string) (Account, error) {
	creds, err := readCredentialsFile(path)
	if err != nil {
		return Account{}, err
	}
	if account, ok := creds.Accounts[key]; ok {
		return account, nil
	}
	token, ok := creds.AccessTokens[key]
	if !ok {
		return Account{}, nil
	}
	return Account{AccessToken: token}, nil
}

// GetStoredCredentials returns any credentials stored on the local machine.
func GetStoredCredentials() (Credentials, error) {
	credsFile, err := getCredsFilePath()
	if err != nil {
		return Credentials{}, err
	}

	logging.V(7).Infof("Reading Pulumi credentials from %q", credsFile)
	return readCredentialsFile(credsFile)
}

// AgentCredentialsFallbackEnabled reports whether shared temporary agent credentials may be used
// as an implicit fallback.
func AgentCredentialsFallbackEnabled() bool {
	return agentdetect.Detect(os.Getenv) != "" && !hasExplicitPulumiPathEnv()
}

// GetAgentStoredCredentials returns credentials stored in the shared temporary
// agent credentials file.
func GetAgentStoredCredentials() (Credentials, error) {
	credsFile, err := getAgentCredsFilePath()
	if err != nil {
		return Credentials{}, err
	}
	logging.V(7).Infof("Reading shared agent credentials from %q", credsFile)
	return readCredentialsFile(credsFile)
}

func getAgentAccount(key string) (Account, error) {
	path, err := getAgentCredsFilePath()
	if err != nil {
		return Account{}, err
	}
	return getAccountAt(path, key)
}

func getCredsFilePath() (string, error) {
	// Allow the folder we use to store config in to be overridden by tests
	pulumiFolder := os.Getenv(PulumiCredentialsPathEnvVar)
	if pulumiFolder == "" {
		logging.V(7).Infof("Using default Pulumi config path")
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

func getAgentCredsFilePath() (string, error) {
	dir, err := getAgentPulumiDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

func getAgentPulumiDir() (string, error) {
	dir := agentPulumiDir
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create temporary Pulumi credentials directory '%s': %w", dir, err)
	}
	return dir, nil
}

func defaultAgentPulumiDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), BookkeepingDir)
	}
	return filepath.Join("/tmp", BookkeepingDir)
}

var agentPulumiDir = defaultAgentPulumiDir()

func readCredentialsFile(credsFile string) (Credentials, error) {
	c, err := lockedfile.Read(credsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return Credentials{}, nil
		}
		return Credentials{}, fmt.Errorf("reading '%s': %w", credsFile, err)
	}
	if len(c) == 0 {
		return Credentials{}, nil
	}

	var creds Credentials
	if err = json.Unmarshal(c, &creds); err != nil {
		return Credentials{}, fmt.Errorf("failed to read Pulumi credentials file. Please fix "+
			"or delete invalid credentials file: '%s': %w", credsFile, err)
	}

	secrets := slice.Prealloc[string](len(creds.AccessTokens) + len(creds.Accounts))
	for _, v := range creds.AccessTokens {
		secrets = append(secrets, v)
	}
	for _, account := range creds.Accounts {
		if account.RefreshToken != "" {
			secrets = append(secrets, account.RefreshToken)
		}
	}
	logging.AddGlobalFilter(logging.CreateFilter(secrets, "[credential]"))

	return creds, nil
}

func hasExplicitPulumiPathEnv() bool {
	return os.Getenv(PulumiCredentialsPathEnvVar) != "" || os.Getenv(env.Home.Var().Name()) != ""
}
