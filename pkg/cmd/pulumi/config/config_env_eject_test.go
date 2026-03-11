// Copyright 2024, Pulumi Corporation.
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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- extractPulumiConfig unit tests ---

func TestExtractPulumiConfig_EmptyYAML(t *testing.T) {
	t.Parallel()

	result, hasSecrets, err := extractPulumiConfig(nil)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Empty(t, result)
}

func TestExtractPulumiConfig_PlainValues(t *testing.T) {
	t.Parallel()

	yamlInput := []byte(`
values:
  pulumiConfig:
    myproject:host: localhost
    myproject:port: "5432"
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Equal(t, ejectedConfigValue{plaintext: "localhost"}, result["myproject:host"])
	assert.Equal(t, ejectedConfigValue{plaintext: "5432"}, result["myproject:port"])
}

func TestExtractPulumiConfig_SecretValue_FnSecret(t *testing.T) {
	t.Parallel()

	// With decrypt=true, ESC returns fn::secret with plaintext value.
	yamlInput := []byte(`
values:
  pulumiConfig:
    myproject:dbpass:
      fn::secret: hunter2
    myproject:host: localhost
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.True(t, hasSecrets)
	assert.Equal(t, ejectedConfigValue{plaintext: "hunter2", secret: true}, result["myproject:dbpass"])
	assert.Equal(t, ejectedConfigValue{plaintext: "localhost"}, result["myproject:host"])
}

func TestExtractPulumiConfig_NoValuesSection(t *testing.T) {
	t.Parallel()

	yamlInput := []byte(`
imports:
  - myorg/creds
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Empty(t, result)
}

func TestExtractPulumiConfig_NoPulumiConfigSection(t *testing.T) {
	t.Parallel()

	yamlInput := []byte(`
values:
  environmentVariables:
    MY_VAR: hello
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Empty(t, result)
}

func TestExtractPulumiConfig_NonStringValues(t *testing.T) {
	t.Parallel()

	// YAML integers and booleans should be stringified.
	yamlInput := []byte(`
values:
  pulumiConfig:
    myproject:count: 42
    myproject:enabled: true
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	assert.Equal(t, "42", result["myproject:count"].plaintext)
	assert.Equal(t, "true", result["myproject:enabled"].plaintext)
}

func TestExtractPulumiConfig_NestedMapSkipped(t *testing.T) {
	t.Parallel()

	// A map value that is not fn::secret should be skipped.
	yamlInput := []byte(`
values:
  pulumiConfig:
    myproject:plain: hello
    myproject:nested:
      key: value
`)
	result, hasSecrets, err := extractPulumiConfig(yamlInput)
	require.NoError(t, err)
	assert.False(t, hasSecrets)
	// "myproject:plain" is present; "myproject:nested" (a non-fn::secret map) is absent.
	assert.Contains(t, result, "myproject:plain")
	assert.NotContains(t, result, "myproject:nested")
}
