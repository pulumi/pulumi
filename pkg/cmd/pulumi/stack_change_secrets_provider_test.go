// Copyright 2016-2023, Pulumi Corporation.
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
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test that we validate the secrets provider and return an error
func TestChangeSecretsProvider_Invalid(t *testing.T) {
	t.Parallel()

	var stdoutBuff bytes.Buffer
	cmd := stackChangeSecretsProviderCmd{
		stdout: &stdoutBuff,
		stack:  "test",
	}
	err := cmd.Run(context.Background(), []string{"not_a_secret"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown secrets provider type 'not_a_secret' "+
		"(supported values: default,passphrase,awskms,azurekeyvault,gcpkms,hashivault)")
}

func mockStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, err = w.Write([]byte(input))
	require.NoError(t, err)
	err = w.Close()
	require.NoError(t, err)
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })
}

// Test that we can change the secrets provider for a stack, this sets up a mock stack using a mock secret provider and
// then changes it to the passphrase provider, but without any existing secrets.
//
//nolint:paralleltest // mutates global state
func TestChangeSecretsProvider_NoSecrets(t *testing.T) {
	var stdoutBuff bytes.Buffer
	cmd := stackChangeSecretsProviderCmd{
		stdout: &stdoutBuff,

		stack: "testStack",
	}

	// Ideally this would be injected but the cmd functions repeatedly access global state to get the current
	// backend.
	snapshot := &deploy.Snapshot{
		SecretsManager: b64.NewBase64SecretsManager(),
		Resources: []*resource.State{
			{
				URN:     resource.NewURN("testStack", "testProject", "", resource.RootStackType, "testStack"),
				Type:    resource.RootStackType,
				Outputs: resource.PropertyMap{},
			},
		},
	}

	mockStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "testStack",
				NameV:   tokens.MustParseStackName("testStack"),
			}
		},
		SnapshotF: func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
			return snapshot, nil
		},
		ExportDeploymentF: func(ctx context.Context) (*apitype.UntypedDeployment, error) {
			chk, err := stack.SerializeDeployment(snapshot, nil, false)
			if err != nil {
				return nil, err
			}
			data, err := encoding.JSON.Marshal(chk)
			if err != nil {
				return nil, err
			}
			return &apitype.UntypedDeployment{
				Version:    3,
				Deployment: json.RawMessage(data),
			}, nil
		},
		ImportDeploymentF: func(ctx context.Context, deployment *apitype.UntypedDeployment) error {
			snap, err := stack.DeserializeUntypedDeployment(ctx, deployment, stack.DefaultSecretsProvider)
			if err != nil {
				return err
			}
			snapshot = snap
			return nil
		},
	}

	backendInstance = &backend.MockBackend{
		GetStackF: func(ctx context.Context, stackRef backend.StackReference) (backend.Stack, error) {
			return mockStack, nil
		},
	}
	t.Cleanup(func() { backendInstance = nil })

	tmpDir := t.TempDir()
	chdir(t, tmpDir)

	// Setup a dummy project in this directory
	err := os.WriteFile("Pulumi.yaml", []byte(`
name: testProject
runtime: mock
`), 0o600)
	require.NoError(t, err)

	// passphrase will read from stdin for the new passphrase
	mockStdin(t, "password123\npassword123\n")
	err = cmd.Run(context.Background(), []string{"passphrase"})
	require.NoError(t, err)
	require.Equal(t, "Migrating old configuration and state to new secrets provider\n", stdoutBuff.String())

	// Check that the snapshot now has a passphrase secrets manager
	assert.Equal(t, "passphrase", snapshot.SecretsManager.Type())
	// Check the config has been updated with the salt
	project, err := workspace.LoadProject("Pulumi.yaml")
	require.NoError(t, err)
	projectStack, err := workspace.LoadProjectStack(project, "Pulumi.testStack.yaml")
	require.NoError(t, err)
	assert.NotEmpty(t, projectStack.EncryptionSalt)
}

// Test that we can change the secrets provider for a stack, this sets up a mock stack using a mock secret provider and
// then changes it to the passphrase provider, with existing secrets in the state and config.
//
//nolint:paralleltest // mutates global state
func TestChangeSecretsProvider_WithSecrets(t *testing.T) {
	ctx := context.Background()

	var stdoutBuff bytes.Buffer
	cmd := stackChangeSecretsProviderCmd{
		stdout: &stdoutBuff,

		stack: "testStack",
	}

	// Ideally this would be injected but the cmd functions repeatedly access global state to get the current
	// backend.
	secretsManager := b64.NewBase64SecretsManager()
	snapshot := &deploy.Snapshot{
		SecretsManager: secretsManager,
		Resources: []*resource.State{
			{
				URN:  resource.NewURN("testStack", "testProject", "", resource.RootStackType, "testStack"),
				Type: resource.RootStackType,
				Outputs: resource.PropertyMap{
					"foo": resource.MakeSecret(resource.NewStringProperty("bar")),
				},
			},
		},
	}

	mockStack := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV: "testStack",
				NameV:   tokens.MustParseStackName("testStack"),
			}
		},
		SnapshotF: func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
			return snapshot, nil
		},
		ExportDeploymentF: func(ctx context.Context) (*apitype.UntypedDeployment, error) {
			chk, err := stack.SerializeDeployment(snapshot, nil, false)
			if err != nil {
				return nil, err
			}
			data, err := encoding.JSON.Marshal(chk)
			if err != nil {
				return nil, err
			}
			return &apitype.UntypedDeployment{
				Version:    3,
				Deployment: json.RawMessage(data),
			}, nil
		},
		ImportDeploymentF: func(ctx context.Context, deployment *apitype.UntypedDeployment) error {
			snap, err := stack.DeserializeUntypedDeployment(ctx, deployment, stack.DefaultSecretsProvider)
			if err != nil {
				return err
			}
			snapshot = snap
			return nil
		},
		DefaultSecretManagerF: func(_ *workspace.ProjectStack) (secrets.Manager, error) {
			return secretsManager, nil
		},
	}

	backendInstance = &backend.MockBackend{
		GetStackF: func(ctx context.Context, stackRef backend.StackReference) (backend.Stack, error) {
			return mockStack, nil
		},
	}
	t.Cleanup(func() { backendInstance = nil })

	tmpDir := t.TempDir()
	chdir(t, tmpDir)

	// Setup a dummy project in this directory
	err := os.WriteFile("Pulumi.yaml", []byte(`
name: testProject
runtime: mock
`), 0o600)
	require.NoError(t, err)

	// Write a dummy config file with a secret in it
	b64Encrypter, err := secretsManager.Encrypter()
	require.NoError(t, err)
	secretBar, err := b64Encrypter.EncryptValue(ctx, "bar")
	require.NoError(t, err)
	cfgKey := config.MustMakeKey("testStack", "secret")
	cfg := workspace.ProjectStack{
		Config: config.Map{
			cfgKey: config.NewSecureValue(secretBar),
		},
	}
	err = cfg.Save("Pulumi.testStack.yaml")
	require.NoError(t, err)

	// passphrase will read from stdin for the new passphrase
	mockStdin(t, "password123\npassword123\n")
	err = cmd.Run(ctx, []string{"passphrase"})
	require.NoError(t, err)
	require.Equal(t, "Migrating old configuration and state to new secrets provider\n", stdoutBuff.String())

	// Check that the snapshot now has a passphrase secrets manager
	assert.Equal(t, "passphrase", snapshot.SecretsManager.Type())
	passphraseDecrypter, err := snapshot.SecretsManager.Decrypter()
	require.NoError(t, err)
	// Check that the snapshot still records the secret value with the same value
	foo := snapshot.Resources[0].Outputs["foo"]
	assert.True(t, foo.IsSecret())
	assert.Equal(t, resource.NewStringProperty("bar"), foo.SecretValue().Element)
	// Check the config has been updated to the new secret
	project, err := workspace.LoadProject("Pulumi.yaml")
	require.NoError(t, err)
	projectStack, err := workspace.LoadProjectStack(project, "Pulumi.testStack.yaml")
	require.NoError(t, err)
	cfgValue, ok := projectStack.Config[cfgKey]
	require.True(t, ok)
	assert.True(t, cfgValue.Secure())
	val, err := cfgValue.Value(passphraseDecrypter)
	require.NoError(t, err)
	assert.Equal(t, "bar", val)
}
