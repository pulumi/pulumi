// Copyright 2016-2022, Pulumi Corporation.
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

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/proto/go/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	state = `
    {"salt": "v1:fozI5u6B030=:v1:F+6ZduKKd8G0/V7L:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ=="}
`
	brokenState = `
    {"salt": "fozI5u6B030=:v1:F+6ZduL:PGMFeIzwobWRKmEAzUdaQHqC5mMRIQ=="}
`
	emptyPassphraseState = `
	{"salt":"v1:LVuDkcjhUns=:v1:WPbL+tmbxnqWswQ2:lebZxStyHT3IZkmbN+3xCx9zqtoPog=="}
`
)

// unsetenv calls os.Unsetenv(key) and uses Cleanup to
// restore the environment variable to its original value
// after the test.
//
// This cannot be used in parallel tests.
func unsetenv(t *testing.T, key string) {
	prevValue, ok := os.LookupEnv(key)

	if ok {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("cannot unset environment variable: %v", err)
		}

		t.Cleanup(func() {
			os.Setenv(key, prevValue)
		})
	}
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseIncorrectPassphraseReturnsError(t *testing.T) {
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "password123")
	unsetenv(t, "PULUMI_CONFIG_PASSPHRASE_FILE")

	ctx := context.Background()
	plugin := NewPassphraseSecretPlugin()

	_, err := plugin.Configure(ctx, &secrets.ConfigureRequest{State: state})
	assert.Error(t, err)
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseIncorrectStateReturnsError(t *testing.T) {
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "password")
	unsetenv(t, "PULUMI_CONFIG_PASSPHRASE_FILE")

	ctx := context.Background()
	plugin := NewPassphraseSecretPlugin()

	_, err := plugin.Configure(ctx, &secrets.ConfigureRequest{State: brokenState})
	assert.Error(t, err)
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseCorrectPassphraseReturnsSecretsManager(t *testing.T) {
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "password")
	unsetenv(t, "PULUMI_CONFIG_PASSPHRASE_FILE")

	ctx := context.Background()
	plugin := NewPassphraseSecretPlugin()

	resp, err := plugin.Configure(ctx, &secrets.ConfigureRequest{State: state})
	assert.NoError(t, err)
	assert.Nil(t, resp.Prompt)

	// Should be able to encrypt and decrypt now
	enc, err := plugin.Encrypt(ctx, &secrets.EncryptRequest{Plaintexts: []string{"secret"}})
	assert.NoError(t, err)
	assert.Len(t, enc.Ciphertexts, 1)

	dec, err := plugin.Decrypt(ctx, &secrets.DecryptRequest{Ciphertexts: enc.Ciphertexts})
	assert.NoError(t, err)
	assert.Len(t, dec.Plaintexts, 1)
	assert.Equal(t, "secret", dec.Plaintexts[0])
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseNoEnvironmentVariablesReturnsPrompt(t *testing.T) {
	unsetenv(t, "PULUMI_CONFIG_PASSPHRASE")
	unsetenv(t, "PULUMI_CONFIG_PASSPHRASE_FILE")

	ctx := context.Background()
	plugin := NewPassphraseSecretPlugin()
	inputs := make(map[string]string)

	resp, err := plugin.Configure(ctx, &secrets.ConfigureRequest{State: state, Inputs: inputs})
	assert.NoError(t, err)
	assert.NotNil(t, resp.Prompt)
	assert.Equal(t, "Enter your passphrase to unlock config/secrets (set PULUMI_CONFIG_PASSPHRASE or PULUMI_CONFIG_PASSPHRASE_FILE to remember)", resp.Prompt.Text)
	assert.Equal(t, "passphrase must be set with PULUMI_CONFIG_PASSPHRASE or PULUMI_CONFIG_PASSPHRASE_FILE environment variables", resp.Prompt.Error)

	// If we try to configure with the wrong password we should get a prompt about it
	inputs[resp.Prompt.Label] = "password123"
	resp, err = plugin.Configure(ctx, &secrets.ConfigureRequest{State: state, Inputs: inputs})
	assert.NoError(t, err)
	assert.NotNil(t, resp.Prompt)
	assert.Equal(t, "Enter your passphrase to unlock config/secrets (set PULUMI_CONFIG_PASSPHRASE or PULUMI_CONFIG_PASSPHRASE_FILE to remember)", resp.Prompt.Text)
	assert.Equal(t, "incorrect passphrase", resp.Prompt.Error)
	assert.Empty(t, resp.Prompt.Preserve)

	// If now configure with the prompt set to the right password we should be able to decrypt
	inputs[resp.Prompt.Label] = "password"
	resp, err = plugin.Configure(ctx, &secrets.ConfigureRequest{State: state, Inputs: inputs})
	assert.NoError(t, err)
	assert.Nil(t, resp.Prompt)

	// Should be able to encrypt and decrypt now
	enc, err := plugin.Encrypt(ctx, &secrets.EncryptRequest{Plaintexts: []string{"secret"}})
	assert.NoError(t, err)
	assert.Len(t, enc.Ciphertexts, 1)

	dec, err := plugin.Decrypt(ctx, &secrets.DecryptRequest{Ciphertexts: enc.Ciphertexts})
	assert.NoError(t, err)
	assert.Len(t, dec.Plaintexts, 1)
	assert.Equal(t, "secret", dec.Plaintexts[0])
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseEmptyPassphraseIsValid(t *testing.T) {
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "")
	unsetenv(t, "PULUMI_CONFIG_PASSPHRASE_FILE")

	ctx := context.Background()
	plugin := NewPassphraseSecretPlugin()

	resp, err := plugin.Configure(ctx, &secrets.ConfigureRequest{State: emptyPassphraseState})
	assert.NoError(t, err)
	assert.Nil(t, resp.Prompt)

	// Should be able to encrypt and decrypt now
	enc, err := plugin.Encrypt(ctx, &secrets.EncryptRequest{Plaintexts: []string{"secret"}})
	assert.NoError(t, err)
	assert.Len(t, enc.Ciphertexts, 1)

	dec, err := plugin.Decrypt(ctx, &secrets.DecryptRequest{Ciphertexts: enc.Ciphertexts})
	assert.NoError(t, err)
	assert.Len(t, dec.Plaintexts, 1)
	assert.Equal(t, "secret", dec.Plaintexts[0])
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseEmptyPassfileReturnsPrompt(t *testing.T) {
	unsetenv(t, "PULUMI_CONFIG_PASSPHRASE")
	t.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", "")

	ctx := context.Background()
	plugin := NewPassphraseSecretPlugin()
	inputs := make(map[string]string)

	resp, err := plugin.Configure(ctx, &secrets.ConfigureRequest{State: state, Inputs: inputs})
	assert.NoError(t, err)
	assert.NotNil(t, resp.Prompt)
	assert.Equal(t, "Enter your passphrase to unlock config/secrets (set PULUMI_CONFIG_PASSPHRASE or PULUMI_CONFIG_PASSPHRASE_FILE to remember)", resp.Prompt.Text)
	assert.Equal(t, "passphrase must be set with PULUMI_CONFIG_PASSPHRASE or PULUMI_CONFIG_PASSPHRASE_FILE environment variables", resp.Prompt.Error)
}

//nolint:paralleltest // mutates environment variables
func TestPassphraseCorrectPassfileReturnsSecretsManager(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "secret.txt")
	err := os.WriteFile(tmpFile, []byte("password"), 0700)
	require.NoError(t, err)

	unsetenv(t, "PULUMI_CONFIG_PASSPHRASE")
	t.Setenv("PULUMI_CONFIG_PASSPHRASE_FILE", tmpFile)

	ctx := context.Background()
	plugin := NewPassphraseSecretPlugin()

	resp, err := plugin.Configure(ctx, &secrets.ConfigureRequest{State: state})
	assert.NoError(t, err)
	assert.Nil(t, resp.Prompt)

	// Should be able to encrypt and decrypt now
	enc, err := plugin.Encrypt(ctx, &secrets.EncryptRequest{Plaintexts: []string{"secret"}})
	assert.NoError(t, err)
	assert.Len(t, enc.Ciphertexts, 1)

	dec, err := plugin.Decrypt(ctx, &secrets.DecryptRequest{Ciphertexts: enc.Ciphertexts})
	assert.NoError(t, err)
	assert.Len(t, dec.Plaintexts, 1)
	assert.Equal(t, "secret", dec.Plaintexts[0])
}
