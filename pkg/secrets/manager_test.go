// Copyright 2016-2024, Pulumi Corporation.
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

// Package secrets defines the interface common to all secret managers.
package secrets

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		projectStack  workspace.ProjectStack
		expectedType  string
		expectedState string
	}{
		{
			name:          "no manager",
			projectStack:  workspace.ProjectStack{},
			expectedType:  "",
			expectedState: "",
		},
		{
			name: "EncryptionSalt",
			projectStack: workspace.ProjectStack{
				EncryptionSalt: "v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ==",
			},
			expectedType:  "passphrase",
			expectedState: `{"salt":"v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ=="}`,
		},
		{
			name: "SecretsProvider",
			projectStack: workspace.ProjectStack{
				SecretsProvider: "awskms://mykey",
			},
			expectedType:  "cloud",
			expectedState: `{"url":"awskms://mykey"}`,
		},
		{
			name: "SecretsProvider+EncryptedKey",
			projectStack: workspace.ProjectStack{
				SecretsProvider: "azurekeyvault://mykey",
				EncryptedKey:    "Ti1qQklqTnlP", // Note not a full key, not important for this test
			},
			expectedType:  "cloud",
			expectedState: `{"url":"azurekeyvault://mykey","encryptedkey":"Ti1qQklqTnlP"}`,
		},
		// Historically we would have picked the cloud provider here just by chance of how the if conditions for
		// checking each field we're laid out. Ensure we continue to do so.
		{
			name: "SecretsProvider+EncryptionSalt",
			projectStack: workspace.ProjectStack{
				SecretsProvider: "awskms://mykey",
				EncryptionSalt:  "v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ==",
			},
			expectedType:  "cloud",
			expectedState: `{"url":"awskms://mykey"}`,
		},
		// Historically SecretsProvider could have been explicitly set to passphrase instead of a cloud URL.
		// These test checks that still works.
		{
			name: "SecretsProvider=passphrase+EncryptionSalt",
			projectStack: workspace.ProjectStack{
				SecretsProvider: "passphrase",
				EncryptionSalt:  "v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ==",
			},
			expectedType:  "passphrase",
			expectedState: `{"salt":"v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ=="}`,
		},
		// Passphrase alone isn't enough to build a passphrase provider, we need the salt. So even if
		// SecretsProvider is set to passphrase we should return that the system needs to fall back to the
		// default provider.
		{
			name: "SecretsProvider=passphrase",
			projectStack: workspace.ProjectStack{
				SecretsProvider: "passphrase",
			},
			expectedType:  "",
			expectedState: ``,
		},
		// Historically we could explicitly set "default" as the secrets provider, this should return the empty
		// type and no state (because we know it's not cloud, and if Salt was present we'd use passphrase even
		// if SecretsProvider was set to "default").
		{
			name: "SecretsProvider=default",
			projectStack: workspace.ProjectStack{
				SecretsProvider: "default",
				EncryptedKey:    "Ti1qQklqTnlP", // Note not a full key, not important for this test, this is ignored.
			},
			expectedType:  "",
			expectedState: "",
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			typ, state, err := GetConfig(&tt.projectStack)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, typ)
			assert.Equal(t, tt.expectedState, string(state))

			// Test round tripping. Note that the new projectStack might not equal the original one e.g. in the case of
			// both SecretsProvider and EncryptionSalt being set we would have parsed that to just SecretsProvider. But
			// once written to ProjectStack and read back again we should get the same result.
			var projectStack workspace.ProjectStack
			err = SetConfig(typ, state, &projectStack)
			require.NoError(t, err)

			rtTyp, rtState, err := GetConfig(&projectStack)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, rtTyp)
			assert.Equal(t, tt.expectedState, string(rtState))
		})
	}
}

func TestExplictPassphrase(t *testing.T) {
	t.Parallel()

	// Test that if the config explicitly had SecretProvider set to "passphrase" we maintain that when setting
	// passphrase config.
	projectStack := workspace.ProjectStack{
		SecretsProvider: "passphrase",
	}

	state := `{"salt":"v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ=="}`
	err := SetConfig("passphrase", []byte(state), &projectStack)
	require.NoError(t, err)

	assert.Equal(t, "passphrase", projectStack.SecretsProvider)
}

/*
func TestPassphraseWithoutSalt(t *testing.T) {
	t.Parallel()

	// Test that if we set the state to a passphrase without a salt, we explicitly set SecretsProvider.
	// In practice we shouldn't ever hit this, but it's a good safety net.
	projectStack := workspace.ProjectStack{
		EncryptionSalt: "v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ==",
	}
	state := `{}`
	err := SetConfig("passphrase", []byte(state), &projectStack)
	require.NoError(t, err)

	assert.Equal(t, "passphrase", projectStack.SecretsProvider)
}
*/
