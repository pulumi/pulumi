// Copyright 2023, Pulumi Corporation. All rights reserved.

package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

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

// SetCurrentAccount sets the currently logged-in account.
func (w *Workspace) SetCurrentAccount(account Account, shared bool) error {
	// Store the account in Pulumi creds.
	if err := w.pulumi.StoreAccount(account.BackendURL, account.Account, false); err != nil {
		return fmt.Errorf("writing Pulumi credentials: %w", err)
	}

	// Store the current esc account. If 'shared' is true, clear the current account to indicate that the user wants to
	// use the same account that the `pulumi` CLI is logged in to.
	current := account.BackendURL
	if shared {
		current = ""
	}
	creds := Credentials{Current: current}

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
