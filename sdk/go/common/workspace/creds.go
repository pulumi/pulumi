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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/rogpeppe/go-internal/lockedfile"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
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
func GetAccount(key string) (Account, error) {
	path, err := getCredsFilePath()
	if err != nil {
		return Account{}, err
	}
	return getAccountAt(path, key)
}

// getAccountAt loads an account from the given credentials file, stamping the source path so
// subsequent account.Save calls write back to the same file.
func getAccountAt(path, key string) (Account, error) {
	creds, err := readCredentialsFile(path)
	if err != nil && !os.IsNotExist(err) {
		return Account{}, err
	}

	if account, ok := creds.Accounts[key]; ok {
		account.source = path
		return account, nil
	}
	token, ok := creds.AccessTokens[key]
	if !ok {
		return Account{}, nil
	}
	return Account{AccessToken: token, source: path}, nil
}

// GetAccountWithAgentFallback returns an account from default credentials, or
// from shared temporary agent credentials when running in a detected agent
// environment without an explicit Pulumi credential path. The boolean return is
// true only when the returned account came from shared agent credentials.
func GetAccountWithAgentFallback(key string) (Account, bool, error) {
	account, err := GetAccount(key)
	if err == nil && account.HasCredential() {
		return account, false, nil
	}

	agent := agentdetect.Detect(os.Getenv)
	if agent == "" || hasExplicitPulumiPathEnv() {
		return account, false, err
	}

	if err != nil {
		logging.V(7).Infof(
			"Could not read account for %q from default credentials in agent mode (%s); "+
				"checking shared agent credentials: %v",
			key, agent, err)
	} else {
		logging.V(7).Infof(
			"No account for %q found in default credentials in agent mode (%s); "+
				"checking shared agent credentials",
			key, agent)
	}

	agentAccount, agentErr := GetAgentAccount(key)
	if agentErr != nil {
		return Account{}, false, errors.Join(err, agentErr)
	}
	if !agentAccount.HasCredential() {
		return Account{}, false, nil
	}
	return agentAccount, true, nil
}

// DeleteAccount deletes an account underneath the given key.
func DeleteAccount(key string) error {
	creds, err := GetStoredCredentials()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = StoreCredentials(deleteAccountFromCredentials(creds, key)); err != nil {
		return err
	}
	return deleteBackendConfig(key)
}

// deleteAccountFromCredentials removes a cloud URL from a credentials object
// and clears it as current if it was selected.
func deleteAccountFromCredentials(creds Credentials, key string) Credentials {
	if creds.AccessTokens != nil {
		delete(creds.AccessTokens, key)
	}
	if creds.Accounts != nil {
		delete(creds.Accounts, key)
	}
	if creds.Current == key {
		creds.Current = ""
	}
	return creds
}

func DeleteAllAccounts() error {
	credsFile, err := getCredsFilePath()
	if err != nil {
		return err
	}

	var result error
	if err = os.Remove(credsFile); err != nil && !os.IsNotExist(err) {
		result = errors.Join(result, err)
	}
	if err = deleteAllBackendConfig(); err != nil {
		result = errors.Join(result, err)
	}
	return result
}

// StoreAccount saves the given account underneath the given key in the default credentials file.
func StoreAccount(key string, account Account, current bool) error {
	path, err := getCredsFilePath()
	if err != nil {
		return err
	}
	return storeAccountAt(path, key, account, current)
}

// storeAccountAt persists an account into the given credentials file (load-modify-write).
func storeAccountAt(path, key string, account Account, current bool) error {
	creds, err := readCredentialsFile(path)
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
	return writeCredentialsFile(path, creds)
}

// Account holds the information associated with a Pulumi account.
type Account struct {
	// The access token for this account.
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

	// source is the credentials file this account was loaded from. Set by the loaders
	// (GetAccount, GetAgentAccount, GetAccountWithAgentFallback); empty for accounts constructed
	// in memory (env-var tokens before persistence, test literals, fresh-login pre-persist).
	// Used by Save to persist credential refreshes back to the file the account came from.
	source string
}

// HasCredential reports whether this account carries anything the CLI can use to authenticate —
// either a current access token or a refresh token to mint one with. Used at credential-selection
// time so that an account with only a refresh token is treated as usable rather than skipped.
func (a Account) HasCredential() bool {
	return a.AccessToken != "" || a.RefreshToken != ""
}

// Save persists this account back to the credentials file it was loaded from. Returns an error if
// the account has no known source (constructed in memory rather than loaded) — callers in that
// case should use StoreAccount / StoreAgentAccount with an explicit destination.
func (a Account) Save(key string, current bool) error {
	if a.source == "" {
		return errors.New("cannot Save an account that was not loaded from a credentials file")
	}
	return storeAccountAt(a.source, key, a, current)
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

// ensurePrivateAgentCredentialDir verifies that agent credentials can be
// written to a private directory, creating it if necessary.
func ensurePrivateAgentCredentialDir(dir string) error {
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		if err := os.Mkdir(dir, 0o700); err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to create temporary Pulumi credentials directory '%s': %w", dir, err)
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return fmt.Errorf("failed to inspect temporary Pulumi credentials directory '%s': %w", dir, err)
	}
	// Refuse symlinks so a process cannot redirect credential writes to an
	// unexpected path outside the shared agent credential directory.
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("temporary Pulumi credentials directory '%s' must not be a symlink", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("temporary Pulumi credentials path '%s' is not a directory", dir)
	}
	// /tmp is shared across users, but the agent credentials are bearer tokens.
	// Only the current OS user should be able to read, write, or traverse this
	// directory.
	if info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("temporary Pulumi credentials directory '%s' has insecure permissions: %w", dir, err)
		}
	}
	return nil
}

// readCredentialsFile loads credentials from a specific file path.
func readCredentialsFile(credsFile string) (Credentials, error) {
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

	secrets := slice.Prealloc[string](len(creds.AccessTokens))
	for _, v := range creds.AccessTokens {
		secrets = append(secrets, v)
	}

	logging.AddGlobalFilter(logging.CreateFilter(secrets, "[credential]"))

	return creds, nil
}

// writeCredentialsFile replaces credentials at a specific file path.
func writeCredentialsFile(credsFile string, creds Credentials) error {
	if len(creds.AccessTokens) == 0 {
		err := os.Remove(credsFile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	raw, err := json.MarshalIndent(creds, "", "    ")
	if err != nil {
		return fmt.Errorf("marshalling credentials object: %w", err)
	}

	return lockedfile.Write(credsFile, bytes.NewReader(raw), 0o600)
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

// StoreCredentials updates the stored credentials on the machine, replacing the existing set.  If the credentials
// are empty, the auth file will be deleted rather than just serializing an empty map.
func StoreCredentials(creds Credentials) error {
	credsFile, err := getCredsFilePath()
	if err != nil {
		return err
	}

	logging.V(7).Infof("Writing Pulumi credentials to %q", credsFile)
	return writeCredentialsFile(credsFile, creds)
}

// AgentClaim is the claim metadata returned when the CLI automatically creates
// an account for an agent.
type AgentClaim struct {
	ClaimURL           string     `json:"claimUrl"`
	ClaimToken         string     `json:"claimToken,omitempty"`
	ValidUntil         time.Time  `json:"validUntil"`
	CloudURL           string     `json:"cloudUrl"`
	ClaimUnavailableAt *time.Time `json:"claimUnavailableAt,omitempty"`
}

// FormatAgentClaimInstruction returns the structured instruction shown to
// coding agents when the CLI has an automatically created agent account claim
// URL to surface. It prefers the access token expiration while the token is
// still valid, then falls back to the claim URL expiration.
func FormatAgentClaimInstruction(
	claimURL string,
	accessTokenExpiresAt *time.Time,
	claimValidUntil time.Time,
	now time.Time,
) string {
	if claimURL == "" {
		return ""
	}
	var expirationLine string
	var instruction string
	if accessTokenExpiresAt != nil && accessTokenExpiresAt.After(now) {
		validFor := FormatAgentClaimValidFor(*accessTokenExpiresAt, now)
		expirationLine = fmt.Sprintf("EPHEMERAL_ACCOUNT_ACCESS_EXPIRES_IN=%s\n", validFor)
		instruction = fmt.Sprintf(
			"Tell the user Pulumi created an ephemeral agent account. "+
				"It is important to surface the claim URL to the user so they can take ownership of the account. "+
				"Include the claim URL and say they must claim the account within %s before agent access expires. "+
				"Tell them claiming the account will lock the Pulumi organization during the claim process, "+
				"so they should let the agent finish its work before claiming it.\n",
			validFor)
	} else if !claimValidUntil.IsZero() && claimValidUntil.After(now) {
		validFor := FormatAgentClaimValidFor(claimValidUntil, now)
		expirationLine = fmt.Sprintf("CLAIM_URL_VALID_FOR=%s\n", validFor)
		instruction = fmt.Sprintf(
			"Tell the user this ephemeral agent account can no longer authenticate, "+
				"but the claim URL is still valid for %s. Include the claim URL and the remaining time. "+
				"Tell them claiming the account will lock the Pulumi organization during the claim process, "+
				"so they should let the agent finish its work before claiming it.\n",
			validFor)
	} else {
		return ""
	}
	message := fmt.Sprintf(
		"PULUMI_EPHEMERAL_AGENT_ACCOUNT\n"+
			"CLAIM_URL=%s\n",
		claimURL)
	message += expirationLine
	message += "ACTION_REQUIRED=Tell the user to claim this Pulumi agent account.\n"
	message += "INSTRUCTION=" + instruction
	return message
}

// AgentLoginRequiredReason identifies why an agent must ask the user to log in.
type AgentLoginRequiredReason int

const (
	// AgentLoginTokenRejected means a locally unexpired agent access token was
	// rejected by the service.
	AgentLoginTokenRejected AgentLoginRequiredReason = iota
	// AgentLoginClaimUnavailable means the service reported the stored claim
	// token is no longer claimable.
	AgentLoginClaimUnavailable
)

// FormatAgentLoginRequiredInstruction returns the structured instruction shown
// to coding agents when an ephemeral agent account can no longer authenticate
// and the user must run pulumi login before retrying.
func FormatAgentLoginRequiredInstruction(
	reason AgentLoginRequiredReason,
	accessTokenExpiresAt *time.Time,
	now time.Time,
) string {
	var message strings.Builder
	message.WriteString("PULUMI_EPHEMERAL_AGENT_ACCOUNT\n")
	if accessTokenExpiresAt != nil {
		fmt.Fprintf(&message,
			"EPHEMERAL_ACCOUNT_ACCESS_EXPIRES_IN=%s\n",
			FormatAgentClaimValidFor(*accessTokenExpiresAt, now))
	}
	message.WriteString(
		"ACTION_REQUIRED=Tell the user to run pulumi login.\n" +
			"INSTRUCTION=Tell the user this Pulumi ephemeral agent account can no longer authenticate")
	switch reason {
	case AgentLoginTokenRejected:
		message.WriteString(" even though local access had not expired. The account was likely claimed or revoked. " +
			"The stacks the agent was working with may have moved to the user's Pulumi account, so the agent's " +
			"existing access to those stacks may have changed. Ask the user to run pulumi login before retrying.\n")
	case AgentLoginClaimUnavailable:
		message.WriteString(", and its claim URL is no longer claimable. The account was likely already claimed, expired, " +
			"or revoked. If it was claimed, the stacks the agent was working with moved to the user's Pulumi account, " +
			"so the agent's existing access to those stacks changed. Ask the user to run pulumi login before retrying.\n")
	default:
		contract.Failf("unknown agent login required reason %v", reason)
	}
	return message.String()
}

// FormatAgentClaimValidFor returns a compact, approximate duration until an
// agent account or claim URL expires.
func FormatAgentClaimValidFor(validUntil, now time.Time) string {
	validFor := validUntil.Sub(now)
	if validFor <= 0 {
		return "expired"
	}
	validFor = validFor.Truncate(time.Minute)
	if validFor < time.Minute {
		return "<1m"
	}

	days := int(validFor / (24 * time.Hour))
	validFor -= time.Duration(days) * 24 * time.Hour
	hours := int(validFor / time.Hour)
	validFor -= time.Duration(hours) * time.Hour
	minutes := int(validFor / time.Minute)

	var b strings.Builder
	if days > 0 {
		fmt.Fprintf(&b, "%dd", days)
	}
	if hours > 0 {
		fmt.Fprintf(&b, "%dh", hours)
	}
	if minutes > 0 || b.Len() == 0 {
		fmt.Fprintf(&b, "%dm", minutes)
	}
	return b.String()
}

// agentAccessTokenExpiresAt returns the agent account access-token expiration,
// plus whether the token is still valid at now.
func agentAccessTokenExpiresAt(account Account, now time.Time) (*time.Time, bool) {
	if account.TokenInformation == nil || account.TokenInformation.ExpiresAt == nil {
		return nil, false
	}
	return account.TokenInformation.ExpiresAt, account.TokenInformation.ExpiresAt.After(now)
}

// AgentAccessTokenExpiresAt returns the locally stored agent account
// access-token expiration, plus whether that expiration is still in the future.
func AgentAccessTokenExpiresAt(account Account, now time.Time) (*time.Time, bool) {
	return agentAccessTokenExpiresAt(account, now)
}

// GetAgentAccessTokenExpiresAt returns the locally stored access-token
// expiration for the shared temporary agent credentials for cloudURL.
func GetAgentAccessTokenExpiresAt(cloudURL string, now time.Time) (*time.Time, bool, error) {
	account, err := GetAgentAccount(cloudURL)
	if err != nil {
		return nil, false, err
	}
	expiresAt, valid := agentAccessTokenExpiresAt(account, now)
	return expiresAt, valid, nil
}

// defaultAgentPulumiDir returns the shared temporary Pulumi directory used for
// agent credentials on this operating system.
func defaultAgentPulumiDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), BookkeepingDir)
	}
	return filepath.Join("/tmp", BookkeepingDir)
}

var agentPulumiDir = defaultAgentPulumiDir()

// pulumiTestAgentPulumiDirEnvVar is an internal test hook for isolating shared
// agent credentials across concurrently running package tests.
const pulumiTestAgentPulumiDirEnvVar = "PULUMI_TEST_AGENT_PULUMI_DIR"

// getAgentPulumiDirPath returns the shared temporary directory path used for
// agent credentials.
func getAgentPulumiDirPath() string {
	if dir := os.Getenv(pulumiTestAgentPulumiDirEnvVar); dir != "" {
		return dir
	}
	return agentPulumiDir
}

// getAgentPulumiDir returns the shared temporary directory used for agent
// credentials, creating it if needed.
func getAgentPulumiDir() (string, error) {
	dir := getAgentPulumiDirPath()
	if err := ensurePrivateAgentCredentialDir(dir); err != nil {
		return "", fmt.Errorf("agent mode requires read/write access to %s: %w", dir, err)
	}
	logging.V(7).Infof("Using shared agent Pulumi directory %q", dir)
	return dir, nil
}

// getAgentCredsFilePath returns the shared temporary agent credentials path.
func getAgentCredsFilePath() (string, error) {
	dir, err := getAgentPulumiDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

// getAgentCredsFilePathNoEnsure returns the agent credentials path without
// creating the agent credentials directory.
func getAgentCredsFilePathNoEnsure() string {
	return filepath.Join(getAgentPulumiDirPath(), "credentials.json")
}

// getAgentClaimFilePath returns the shared temporary agent claim metadata path.
func getAgentClaimFilePath() (string, error) {
	dir, err := getAgentPulumiDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agent-claim.json"), nil
}

// getAgentClaimFilePathNoEnsure returns the agent claim metadata path without
// creating the agent credentials directory.
func getAgentClaimFilePathNoEnsure() string {
	return filepath.Join(getAgentPulumiDirPath(), "agent-claim.json")
}

// getAgentConfigFilePath returns the shared temporary agent config path.
func getAgentConfigFilePath() (string, error) {
	dir, err := getAgentPulumiDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// getAgentConfigFilePathNoEnsure returns the agent config path without
// creating the agent credentials directory.
func getAgentConfigFilePathNoEnsure() string {
	return filepath.Join(getAgentPulumiDirPath(), "config.json")
}

// GetAgentAccount returns the account for the given cloud URL from the shared
// agent credentials file.
func GetAgentAccount(key string) (Account, error) {
	path, err := getAgentCredsFilePath()
	if err != nil {
		return Account{}, err
	}
	logging.V(7).Infof("Reading shared agent credentials from %q", path)
	return getAccountAt(path, key)
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

// StoreAgentAccount saves the account for the given cloud URL in the shared
// temporary agent credentials file.
func StoreAgentAccount(key string, account Account, current bool) error {
	path, err := getAgentCredsFilePath()
	if err != nil {
		return err
	}
	return storeAccountAt(path, key, account, current)
}

// DeleteAgentAccount deletes an account from the shared temporary agent
// credentials file and removes matching claim and backend config metadata.
func DeleteAgentAccount(key string) error {
	creds, err := GetAgentStoredCredentials()
	if err != nil {
		return err
	}
	if err = StoreAgentCredentials(deleteAccountFromCredentials(creds, key)); err != nil {
		return err
	}

	var result error
	if err = deleteAgentClaim(key); err != nil {
		result = errors.Join(result, err)
	}
	if err = deleteAgentBackendConfig(key); err != nil {
		result = errors.Join(result, err)
	}
	return result
}

// deleteAgentClaim removes the singleton shared temporary agent claim file when
// the stored claim belongs to the given cloud URL.
func deleteAgentClaim(key string) error {
	claim, err := GetAgentClaim()
	if err != nil {
		return err
	}
	if claim.CloudURL != key {
		return nil
	}
	claimFile := getAgentClaimFilePathNoEnsure()
	if err = os.Remove(claimFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing '%s': %w", claimFile, err)
	}
	return nil
}

// StoreAgentCredentials replaces the shared temporary agent credentials file.
func StoreAgentCredentials(creds Credentials) error {
	credsFile, err := getAgentCredsFilePath()
	if err != nil {
		return err
	}
	logging.V(7).Infof("Writing shared agent credentials to %q", credsFile)
	return writeCredentialsFile(credsFile, creds)
}

// DeleteAgentCredentials removes shared temporary agent credentials, claim
// metadata, and config.
func DeleteAgentCredentials() error {
	var result error
	for _, path := range []string{
		getAgentCredsFilePathNoEnsure(),
		getAgentClaimFilePathNoEnsure(),
		getAgentConfigFilePathNoEnsure(),
	} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			result = errors.Join(result, fmt.Errorf("removing '%s': %w", path, err))
		}
	}
	return result
}

// GetAgentClaim returns claim metadata for an automatically created agent
// account, if one has been stored.
func GetAgentClaim() (AgentClaim, error) {
	claimFile := getAgentClaimFilePathNoEnsure()

	data, err := os.ReadFile(claimFile)
	if err != nil {
		if os.IsNotExist(err) {
			return AgentClaim{}, nil
		}
		return AgentClaim{}, fmt.Errorf("reading '%s': %w", claimFile, err)
	}

	var claim AgentClaim
	if err := json.Unmarshal(data, &claim); err != nil {
		return AgentClaim{}, fmt.Errorf("failed to read Pulumi agent claim file '%s': %w", claimFile, err)
	}
	return claim, nil
}

// MarkAgentClaimUnavailable records that the shared temporary agent claim is
// no longer claimable.
func MarkAgentClaimUnavailable(unavailableAt time.Time) error {
	claim, err := GetAgentClaim()
	if err != nil {
		return err
	}
	if claim.ClaimURL == "" {
		return nil
	}
	claim.ClaimUnavailableAt = &unavailableAt
	return StoreAgentClaim(claim)
}

// DeleteExpiredAgentCredentials removes shared temporary agent credentials when
// both the claim URL and access token have expired. It returns true when
// credentials were removed.
func DeleteExpiredAgentCredentials(now time.Time) (bool, error) {
	claim, err := GetAgentClaim()
	if err != nil {
		return false, err
	}
	if claim.ClaimURL == "" || claim.ValidUntil.IsZero() || claim.ValidUntil.After(now) {
		if claim.ClaimURL != "" && !claim.ValidUntil.IsZero() {
			logging.V(7).Infof("Shared agent claim metadata is valid until %s", claim.ValidUntil)
		}
		return false, nil
	}
	creds, err := GetAgentStoredCredentials()
	if err != nil {
		return false, err
	}
	if claim.CloudURL != "" {
		if expiresAt, valid := agentAccessTokenExpiresAt(creds.Accounts[claim.CloudURL], now); valid {
			// This is defensive: normally the access token should expire before
			// the claim URL, but do not delete still-usable credentials if the
			// service returns a different ordering.
			logging.V(7).Infof(
				"Shared agent claim metadata expired at %s, but access token for %q is valid until %s",
				claim.ValidUntil, claim.CloudURL, *expiresAt)
			return false, nil
		}
	}
	logging.V(7).Infof("Shared agent claim metadata expired at %s; deleting shared agent state", claim.ValidUntil)
	if err := DeleteAgentCredentials(); err != nil {
		return false, err
	}
	return true, nil
}

// StoreAgentClaim stores claim metadata for an automatically created agent
// account alongside the shared temporary agent credentials.
func StoreAgentClaim(claim AgentClaim) error {
	claimFile, err := getAgentClaimFilePath()
	if err != nil {
		return err
	}
	logging.V(7).Infof("Writing shared agent claim metadata to %q", claimFile)

	raw, err := json.MarshalIndent(claim, "", "    ")
	if err != nil {
		return fmt.Errorf("marshalling agent claim object: %w", err)
	}
	return lockedfile.Write(claimFile, bytes.NewReader(raw), 0o600)
}

// deleteAgentBackendConfig removes backend config for key from the shared agent
// config file, deleting that file if no backend config remains.
func deleteAgentBackendConfig(key string) error {
	configFile := getAgentConfigFilePathNoEnsure()
	return deleteBackendConfigFromFile(configFile, key)
}

// deleteBackendConfig removes backend config for key from the default config
// file, deleting that file if no backend config remains.
func deleteBackendConfig(key string) error {
	configFile, err := getConfigFilePath()
	if err != nil {
		return err
	}
	return deleteBackendConfigFromFile(configFile, key)
}

func deleteBackendConfigFromFile(configFile, key string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading '%s': %w", configFile, err)
	}

	var config PulumiConfig
	if err = json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to read Pulumi agent config file '%s': %w", configFile, err)
	}
	delete(config.BackendConfig, key)
	if len(config.BackendConfig) == 0 {
		if err = os.Remove(configFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing '%s': %w", configFile, err)
		}
		return nil
	}
	return writePulumiConfigFile(configFile, config)
}

// deleteAllBackendConfig removes all backend configuration from the default
// config file.
func deleteAllBackendConfig() error {
	configFile, err := getConfigFilePath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading '%s': %w", configFile, err)
	}

	var config PulumiConfig
	if err = json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to read Pulumi config file '%s': %w", configFile, err)
	}
	config.BackendConfig = nil
	return writePulumiConfigFile(configFile, config)
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

	return filepath.Join(pulumiFolder, "config.json"), nil
}

// hasExplicitPulumiPathEnv reports whether the user explicitly selected a
// Pulumi credential or home path, disabling implicit agent fallback paths.
func hasExplicitPulumiPathEnv() bool {
	return os.Getenv(PulumiCredentialsPathEnvVar) != "" || os.Getenv(env.Home.Var().Name()) != ""
}

// AgentCredentialsFallbackEnabled reports whether shared temporary agent
// credentials may be used as an implicit fallback.
func AgentCredentialsFallbackEnabled() bool {
	return agentdetect.Detect(os.Getenv) != "" && !hasExplicitPulumiPathEnv()
}

func GetPulumiConfig() (PulumiConfig, error) {
	configFile, err := getConfigFilePath()
	if err != nil {
		return getAgentPulumiConfigIfNeeded(err)
	}

	logging.V(7).Infof("Reading Pulumi config from %q", configFile)
	c, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return PulumiConfig{}, nil
		}
		return getAgentPulumiConfigIfNeeded(fmt.Errorf("reading '%s': %w", configFile, err))
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
		return storeAgentPulumiConfigIfNeeded(config, err)
	}
	logging.V(7).Infof("Writing Pulumi config to %q", configFile)

	if err := writePulumiConfigFile(configFile, config); err != nil {
		return storeAgentPulumiConfigIfNeeded(config, err)
	}
	return nil
}

// writePulumiConfigFile atomically replaces a Pulumi config file.
func writePulumiConfigFile(configFile string, config PulumiConfig) error {
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

// getAgentPulumiConfigIfNeeded reads shared agent config when agent mode cannot
// read the default Pulumi config path.
func getAgentPulumiConfigIfNeeded(defaultErr error) (PulumiConfig, error) {
	agent := agentdetect.Detect(os.Getenv)
	if agent == "" || hasExplicitPulumiPathEnv() {
		return PulumiConfig{}, defaultErr
	}

	configFile, err := getAgentConfigFilePath()
	if err != nil {
		return PulumiConfig{}, errors.Join(defaultErr, err)
	}
	logging.V(7).Infof(
		"Could not read default Pulumi config in agent mode (%s); reading shared agent config from %q: %v",
		agent, configFile, defaultErr)
	c, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return PulumiConfig{}, nil
		}
		return PulumiConfig{}, errors.Join(defaultErr, fmt.Errorf("reading '%s': %w", configFile, err))
	}

	var config PulumiConfig
	if err = json.Unmarshal(c, &config); err != nil {
		return PulumiConfig{}, fmt.Errorf("failed to read Pulumi config file: %w", err)
	}
	return config, nil
}

// storeAgentPulumiConfigIfNeeded writes shared agent config when agent mode
// cannot write the default Pulumi config path.
func storeAgentPulumiConfigIfNeeded(config PulumiConfig, defaultErr error) error {
	agent := agentdetect.Detect(os.Getenv)
	if agent == "" || hasExplicitPulumiPathEnv() {
		return defaultErr
	}

	configFile, err := getAgentConfigFilePath()
	if err != nil {
		return errors.Join(defaultErr, err)
	}
	logging.V(7).Infof(
		"Could not write default Pulumi config in agent mode (%s); writing shared agent config to %q: %v",
		agent, configFile, defaultErr)
	if err = writePulumiConfigFile(configFile, config); err != nil {
		return errors.Join(defaultErr, err)
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
