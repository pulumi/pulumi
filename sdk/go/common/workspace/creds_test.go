// Copyright 2020-2024, Pulumi Corporation.
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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // mutates environment
func TestConcurrentCredentialsWrites(t *testing.T) {
	// save and remember to restore creds in ~/.pulumi/credentials
	// as the test will be modifying them
	oldCreds, err := GetStoredCredentials()
	require.NoError(t, err)
	defer func() {
		err := StoreCredentials(oldCreds)
		require.NoError(t, err)
	}()

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
	err = StoreCredentials(testCreds)
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
