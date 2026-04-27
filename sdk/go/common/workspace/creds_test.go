// Copyright 2020, Pulumi Corporation.
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
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goKeyring "github.com/zalando/go-keyring"
)

//nolint:paralleltest // mutates environment
func TestConcurrentCredentialsWrites(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, tempDir)
	goKeyring.MockInitWithError(errors.New("keyring unavailable"))

	// use test creds that have at least 1 AccessToken to force a
	// disk write and contention
	testCreds := Credentials{
		AccessTokens: map[string]string{
			"token-name": "token-value",
		},
	}

	// using 1000 may trigger sporadic 'Too many open files'
	n := 256

	wg := &sync.WaitGroup{}
	wg.Add(2 * n)

	// Store testCreds initially so asserts in
	// GetStoredCredentials goroutines find the expected data
	err := StoreCredentials(testCreds)
	require.NoError(t, err)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			err := StoreCredentials(testCreds)
			require.NoError(t, err)
		}()
		go func() {
			defer wg.Done()
			creds, err := GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, "token-value", creds.AccessTokens["token-name"])
		}()
	}
	wg.Wait()
}

//nolint:paralleltest // mutates environment
func TestStoreCredentialsUsesKeyringAndWritesMetadataOnly(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, tempDir)
	goKeyring.MockInit()

	creds := Credentials{
		Current: "https://api.pulumi.com",
		Accounts: map[string]Account{
			"https://api.pulumi.com": {
				AccessToken: "token-value",
				Username:    "alice",
			},
		},
		AccessTokens: map[string]string{
			"https://api.pulumi.com": "token-value",
		},
	}

	err := StoreCredentials(creds)
	require.NoError(t, err)

	rawCreds, err := readCredentialsFile()
	require.NoError(t, err)
	assert.Equal(t, creds.Current, rawCreds.Current)
	assert.Empty(t, rawCreds.AccessTokens["https://api.pulumi.com"])
	assert.Empty(t, rawCreds.Accounts["https://api.pulumi.com"].AccessToken)

	storedToken, err := goKeyring.Get(pulumiCredentialsKeyringService, keyringUser("https://api.pulumi.com"))
	require.NoError(t, err)
	assert.Equal(t, "token-value", storedToken)

	hydratedCreds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "token-value", hydratedCreds.AccessTokens["https://api.pulumi.com"])
	assert.Equal(t, "token-value", hydratedCreds.Accounts["https://api.pulumi.com"].AccessToken)
}

//nolint:paralleltest // mutates environment
func TestGetStoredCredentialsMigratesLegacyPlaintextTokens(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, tempDir)
	goKeyring.MockInit()

	legacyCreds := Credentials{
		Current: "https://api.pulumi.com",
		Accounts: map[string]Account{
			"https://api.pulumi.com": {
				AccessToken: "legacy-token",
				Username:    "alice",
			},
		},
		AccessTokens: map[string]string{
			"https://api.pulumi.com": "legacy-token",
		},
	}

	err := writeCredentialsFile(legacyCreds)
	require.NoError(t, err)

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "legacy-token", creds.AccessTokens["https://api.pulumi.com"])
	assert.Equal(t, "legacy-token", creds.Accounts["https://api.pulumi.com"].AccessToken)

	rawCreds, err := readCredentialsFile()
	require.NoError(t, err)
	assert.Empty(t, rawCreds.AccessTokens["https://api.pulumi.com"])
	assert.Empty(t, rawCreds.Accounts["https://api.pulumi.com"].AccessToken)

	storedToken, err := goKeyring.Get(pulumiCredentialsKeyringService, keyringUser("https://api.pulumi.com"))
	require.NoError(t, err)
	assert.Equal(t, "legacy-token", storedToken)
}

//nolint:paralleltest // mutates environment
func TestStoreCredentialsFallsBackToPlaintextWhenKeyringUnavailable(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, tempDir)
	goKeyring.MockInitWithError(errors.New("keyring unavailable"))

	creds := Credentials{
		Current: "https://api.pulumi.com",
		Accounts: map[string]Account{
			"https://api.pulumi.com": {
				AccessToken: "token-value",
			},
		},
		AccessTokens: map[string]string{
			"https://api.pulumi.com": "token-value",
		},
	}

	err := StoreCredentials(creds)
	require.NoError(t, err)

	credsPath := filepath.Join(tempDir, "credentials.json")
	content, err := os.ReadFile(credsPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "token-value")

	readCreds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "token-value", readCreds.AccessTokens["https://api.pulumi.com"])
}
