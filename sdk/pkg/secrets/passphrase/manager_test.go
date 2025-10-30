// Copyright 2021-2024, Pulumi Corporation.
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

package passphrase

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	state		= `{"salt":"v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ=="}`
	brokenState	= `{"salt":"fozI5u6B030=:v1:F+6ZduL:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ=="}`
)

func TestMain(m *testing.M) {
	if runtime.GOOS == "windows" {
		// These tests are skipped as part of enabling running unit tests on windows and MacOS in
		// https://github.com/pulumi/pulumi/pull/19653. These tests currently fail on Windows, and
		// re-enabling them is left as future work.
		// TODO[pulumi/pulumi#19675]: Re-enable tests on windows once they are fixed.
		fmt.Println("Skip tests on windows until they are fixed")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestPassphraseManagerIncorrectPassphraseReturnsErrorCrypter(t *testing.T) {
	clearCachedSecretsManagers()

	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "password123")

	// There is no t.Unsetenv, so for variables we want to genuinely unset (and not just set to ""), we set the
	// environment variables to empty strings using t.Setenv and then unset them using os.Unsetenv. In doing do, the
	// cleanup of t.Setenv takes care of resetting the environment variables when the test has completed.
	t.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", "")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE_FILE")

	manager, err := NewPromptingPassphraseSecretsManagerFromState([]byte(state))
	require.NoError(t, err)	// even if we pass the wrong provider, we should get a lockedPassphraseProvider

	state, err := json.Marshal(localSecretsManagerState{
		Salt: "v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ==",
	})
	require.NoError(t, err)

	assert.Equal(t, manager, &localSecretsManager{
		state:		state,
		crypter:	&errorCrypter{},
	})
}

func TestPassphraseManagerIncorrectStateReturnsError(t *testing.T) {
	clearCachedSecretsManagers()

	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "password")

	// There is no t.Unsetenv, so for variables we want to genuinely unset (and not just set to ""), we set the
	// environment variables to empty strings using t.Setenv and then unset them using os.Unsetenv. In doing do, the
	// cleanup of t.Setenv takes care of resetting the environment variables when the test has completed.
	t.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", "")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE_FILE")

	_, err := NewPromptingPassphraseSecretsManagerFromState([]byte(brokenState))
	assert.Error(t, err)
}

func TestPassphraseManagerCorrectPassphraseReturnsSecretsManager(t *testing.T) {
	clearCachedSecretsManagers()

	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "password")

	// There is no t.Unsetenv, so for variables we want to genuinely unset (and not just set to ""), we set the
	// environment variables to empty strings using t.Setenv and then unset them using os.Unsetenv. In doing do, the
	// cleanup of t.Setenv takes care of resetting the environment variables when the test has completed.
	t.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", "")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE_FILE")

	sm, err := NewPromptingPassphraseSecretsManagerFromState([]byte(state))
	require.NoError(t, err)
	require.NotNil(t, sm)
}

func TestPassphraseManagerNoEnvironmentVariablesReturnsError(t *testing.T) {
	clearCachedSecretsManagers()

	// There is no t.Unsetenv, so for variables we want to genuinely unset (and not just set to ""), we set the
	// environment variables to empty strings using t.Setenv and then unset them using os.Unsetenv. In doing do, the
	// cleanup of t.Setenv takes care of resetting the environment variables when the test has completed.
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")
	t.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", "")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE_FILE")

	_, err := NewPromptingPassphraseSecretsManagerFromState([]byte(state))
	assert.ErrorContains(t, err, "passphrase must be set with "+
		"PULUMI_CONFIG_PASSPHRASE or PULUMI_CONFIG_PASSPHRASE_FILE environment variables")
}

func TestPassphraseManagerEmptyPassphraseIsValid(t *testing.T) {
	clearCachedSecretsManagers()

	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "")

	// There is no t.Unsetenv, so for variables we want to genuinely unset (and not just set to ""), we set the
	// environment variables to empty strings using t.Setenv and then unset them using os.Unsetenv. In doing do, the
	// cleanup of t.Setenv takes care of resetting the environment variables when the test has completed.
	t.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", "")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE_FILE")

	sm, err := NewPromptingPassphraseSecretsManagerFromState([]byte(state))
	require.NoError(t, err)
	require.NotNil(t, sm)
}

func TestPassphraseManagerCorrectPassfileReturnsSecretsManager(t *testing.T) {
	clearCachedSecretsManagers()

	tmpFile, err := os.CreateTemp(t.TempDir(), "pulumi-secret-test")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.WriteString("password")
	require.NoError(t, err)

	// There is no t.Unsetenv, so for variables we want to genuinely unset (and not just set to ""), we set the
	// environment variables to empty strings using t.Setenv and then unset them using os.Unsetenv. In doing do, the
	// cleanup of t.Setenv takes care of resetting the environment variables when the test has completed.
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")

	t.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", tmpFile.Name())

	sm, err := NewPromptingPassphraseSecretsManagerFromState([]byte(state))
	require.NoError(t, err)
	require.NotNil(t, sm)
}

func TestPassphraseManagerEmptyPassfileReturnsError(t *testing.T) {
	clearCachedSecretsManagers()

	// There is no t.Unsetenv, so for variables we want to genuinely unset (and not just set to ""), we set the
	// environment variables to empty strings using t.Setenv and then unset them using os.Unsetenv. In doing do, the
	// cleanup of t.Setenv takes care of resetting the environment variables when the test has completed.
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "")
	os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")

	t.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", "")

	_, err := NewPromptingPassphraseSecretsManagerFromState([]byte(state))
	assert.ErrorContains(t, err, "passphrase must be set with "+
		"PULUMI_CONFIG_PASSPHRASE or PULUMI_CONFIG_PASSPHRASE_FILE environment variables")
}
