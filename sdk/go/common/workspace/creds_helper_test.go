// Copyright 2026, Pulumi Corporation.
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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/securestore"
)

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCredentialHelperSurvivesAccountUpdates(t *testing.T) {
	dir := ptesting.IsolateCredentials(t).Home
	helper := &CredentialHelper{Path: filepath.Join(dir, "helper"), Args: []string{"--profile", "work"}}
	require.NoError(t, StoreCredentials(Credentials{CredentialHelper: helper}))
	const backend = "https://api.example.com"
	require.NoError(t, StoreAccount(backend, Account{AccessToken: "first-token"}, true))
	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, helper, creds.CredentialHelper)
	assert.Equal(t, 1, creds.Version)
	assert.Equal(t, backend, creds.Current)

	account, err := GetAccount(backend)
	require.NoError(t, err)
	account.AccessToken = "refreshed-token"
	require.NoError(t, account.Save(backend, false))
	creds, err = GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, helper, creds.CredentialHelper)
	assert.Equal(t, "refreshed-token", creds.Accounts[backend].AccessToken)
	raw, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	require.NoError(t, err)
	var legacy struct {
		AccessTokens map[string]string `json:"accessTokens"`
	}
	require.NoError(t, json.Unmarshal(raw, &legacy))
	assert.Equal(t, "refreshed-token", legacy.AccessTokens[backend])

	require.NoError(t, DeleteAccount(backend))
	creds, err = GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, Credentials{Version: 1, CredentialHelper: helper}, creds)
	require.NoError(t, DeleteAllAccounts())
	assert.NoFileExists(t, filepath.Join(dir, "credentials.json"))
}

//nolint:paralleltest // Uses process-wide environment and fake key-store settings.
func TestCredentialHelperEncryptedRoundTrip(t *testing.T) {
	isolateSecureCredentials(t, "auto")
	helper := &CredentialHelper{Path: filepath.Join(t.TempDir(), "helper"), Args: []string{"--profile", "work"}}
	require.NoError(t, StoreCredentials(Credentials{CredentialHelper: helper}))
	path, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, securestore.IsEnvelope(raw))
	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, Credentials{Version: 1, CredentialHelper: helper}, creds)
}
