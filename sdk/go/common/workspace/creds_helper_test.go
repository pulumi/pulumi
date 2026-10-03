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
func TestCredentialsHelperSurvivesAccountUpdates(t *testing.T) {
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

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCredentialsWithoutAccounts(t *testing.T) {
	for _, tt := range []struct {
		name  string
		creds Credentials
		keep  bool
	}{
		{name: "current backend", creds: Credentials{Current: "s3://state"}, keep: true},
		{name: "helper", creds: Credentials{CredentialHelper: &CredentialHelper{Path: "helper"}}, keep: true},
		{name: "version alone", creds: Credentials{Version: 1}},
		{name: "empty"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := ptesting.IsolateCredentials(t).Home
			require.NoError(t, StoreCredentials(testCreds()))
			require.NoError(t, StoreCredentials(tt.creds))
			if !tt.keep {
				assert.NoFileExists(t, filepath.Join(dir, "credentials.json"))
				return
			}
			assert.FileExists(t, filepath.Join(dir, "credentials.json"))
			loaded, err := GetStoredCredentials()
			require.NoError(t, err)
			want := tt.creds
			want.Version = 1
			assert.Equal(t, want, loaded)
		})
	}
}

//nolint:paralleltest // Uses process-wide environment and fake key-store settings.
func TestCredentialsHelperEncryptedRoundTrip(t *testing.T) {
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

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCredentialsLegacyVersion(t *testing.T) {
	dir := ptesting.IsolateCredentials(t).Home
	path := filepath.Join(dir, "credentials.json")
	raw := []byte(`{
		"current":"https://api.example.com",
		"accounts":{"https://api.example.com":{"accessToken":"old-token"}}
	}`)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Zero(t, creds.Version)
	afterRead, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, raw, afterRead)
	require.NoError(t, StoreAccount("s3://state", Account{}, false))
	creds, err = GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, 1, creds.Version)
	assert.Equal(t, "https://api.example.com", creds.Current)
	assert.Equal(t, "old-token", creds.Accounts[creds.Current].AccessToken)
	assert.Contains(t, creds.Accounts, "s3://state")
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCredentialsUnsupportedVersion(t *testing.T) {
	dir := ptesting.IsolateCredentials(t).Home
	path := filepath.Join(dir, "credentials.json")
	raw := []byte(`{"version":2,"credentialHelper":{"path":"future-format"}}`)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	_, err := GetStoredCredentials()
	require.ErrorContains(t, err, "unsupported credentials file version 2")
	err = StoreAccount("https://api.example.com", Account{AccessToken: "token"}, true)
	require.ErrorContains(t, err, "unsupported credentials file version 2")
	err = StoreCredentials(Credentials{Version: 2})
	require.ErrorContains(t, err, "unsupported credentials file version 2")
	_, err = json.Marshal(Credentials{Version: 2})
	require.ErrorContains(t, err, "unsupported credentials file version 2")
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, raw, after)
}
