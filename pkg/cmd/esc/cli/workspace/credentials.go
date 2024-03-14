// Copyright 2023, Pulumi Corporation.
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
	"io/fs"
	"os"
	"path"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// PulumiBackendURLEnvVar is an environment variable which can be used to set the backend that will be
// used instead of the currently logged in backend or the current projects backend.
const PulumiBackendURLEnvVar = "PULUMI_BACKEND_URL"

// Account holds details about a Pulumi account.
type Account struct {
	workspace.Account

	// The URL of the account's backend.
	BackendURL string
	// The default org for the backend.
	DefaultOrg string
}

// Credentials hold the information necessary for authenticating Pulumi Cloud API requests.
type Credentials struct {
	Current string `json:"name"` // The account to use for login. Accounts are stored in Pulumi creds.
}

// GetAccount returns an account underneath a given key.
//
// Note that the account may not be fully populated: it may only have a valid AccessToken. In that case, it is up to
// the caller to fill in the username and last validation time.
func (w *Workspace) GetAccount(backendURL string) (*Account, error) {
	account, err := w.pulumi.GetAccount(backendURL)
	if err != nil {
		return nil, err
	}

	config, err := w.pulumi.GetPulumiConfig()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	return &Account{
		Account:    account,
		BackendURL: backendURL,
		DefaultOrg: config.BackendConfig[backendURL].DefaultOrg,
	}, nil
}

func (w *Workspace) GetCurrentCloudURL(account *Account) string {
	if account != nil {
		return account.BackendURL
	}

	if backend := os.Getenv(PulumiBackendURLEnvVar); backend != "" {
		return backend
	}

	return "https://api.pulumi.com"
}

// GetCurrentAccount returns information about the currently logged-in account.
func (w *Workspace) GetCurrentAccount(shared bool) (*Account, bool, error) {
	// Read esc account.
	backendURL, err := w.getCurrentAccountName()
	if err != nil {
		return nil, false, fmt.Errorf("reading credentials: %w", err)
	}

	// Read Pulumi creds.
	pulumiCreds, err := w.pulumi.GetStoredCredentials()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, false, fmt.Errorf("reading Pulumi credentials: %w", err)
	}

	// If there is no current account, default to the current `pulumi` account.
	if backendURL == "" || shared {
		backendURL = pulumiCreds.Current
		if backendURL == "" {
			return nil, true, nil
		}
		shared = true
	}

	// If the account does not exist, fail.
	account, ok := pulumiCreds.Accounts[backendURL]
	if !ok {
		return nil, false, fmt.Errorf("account '%s' not found."+
			"Please re-run `esc login` to reset your credentials file.", backendURL)
	}

	config, err := w.pulumi.GetPulumiConfig()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, false, err
	}

	return &Account{
		Account:    account,
		BackendURL: backendURL,
		DefaultOrg: config.BackendConfig[backendURL].DefaultOrg,
	}, shared, nil
}

// DeleteAllAccounts logs out of all accounts.
func (w *Workspace) DeleteAllAccounts() error {
	// Clear the current account.
	if err := w.writeCredsFile(Credentials{}); err != nil {
		return err
	}

	// Log out of all accounts.
	return w.pulumi.DeleteAllAccounts()
}

// DeleteAccount logs out of the given backend. If the backend is the current account, then the current
// account is cleared.
func (w *Workspace) DeleteAccount(backendURL string) error {
	// Read esc account.
	currentBackendURL, err := w.getCurrentAccountName()
	if err != nil {
		return fmt.Errorf("reading credentials: %w", err)
	}

	// Read Pulumi creds.
	pulumiCreds, err := w.pulumi.GetStoredCredentials()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("reading Pulumi credentials: %w", err)
	}

	// If there is no current account, default to the current `pulumi` account.
	if currentBackendURL == "" {
		currentBackendURL = pulumiCreds.Current
	}

	// Clear the current account.
	if currentBackendURL == backendURL {
		if err := w.writeCredsFile(Credentials{}); err != nil {
			return err
		}
	}

	// Log out of the account.
	return w.pulumi.DeleteAccount(backendURL)
}

// SetCurrentAccount sets the currently logged-in account.
func (w *Workspace) SetCurrentAccount(account Account, shared bool) error {
	// Read Pulumi creds.
	pulumiCreds, err := w.pulumi.GetStoredCredentials()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("reading Pulumi credentials: %w", err)
	}

	// If there is no current Pulumi account and we want to share the current Pulumi account, then we need to set the
	// current Pulumi account.
	setCurrent := shared && pulumiCreds.Current == ""

	// Store the account in Pulumi creds.
	if err := w.pulumi.StoreAccount(account.BackendURL, account.Account, setCurrent); err != nil {
		return fmt.Errorf("writing Pulumi credentials: %w", err)
	}

	// Store the current esc account. If 'shared' is true, clear the current account to indicate that the user wants to
	// use the same account that the `pulumi` CLI is logged in to.
	current := account.BackendURL
	if shared {
		current = ""
	}
	return w.writeCredsFile(Credentials{Current: current})
}

func (w *Workspace) getCurrentAccountName() (string, error) {
	credsFile, err := w.getCredsFilePath()
	if err != nil {
		return "", err
	}

	c, err := w.fs.LockedRead(credsFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	var creds Credentials
	if err = json.Unmarshal(c, &creds); err != nil {
		return "", fmt.Errorf("please re-run `esc login` to reset your credentials file. (%w)", err)
	}
	return creds.Current, nil
}

// getCredsFilePath returns the path to the esc credentials file on disk, regardless of
// whether it exists or not.
func (w *Workspace) getCredsFilePath() (string, error) {
	dir, err := w.getBookkeepingDir()
	if err != nil {
		return "", fmt.Errorf("getting bookkeeping directory: %w", err)
	}

	return path.Join(dir, "credentials.json"), nil
}

func (w *Workspace) writeCredsFile(creds Credentials) error {
	credsFile, err := w.getCredsFilePath()
	if err != nil {
		return err
	}

	raw, err := json.MarshalIndent(creds, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}

	if err := w.fs.MkdirAll(path.Dir(credsFile), 0o700); err != nil {
		return err
	}

	return w.fs.LockedWrite(credsFile, bytes.NewReader(raw), 0o600)
}
