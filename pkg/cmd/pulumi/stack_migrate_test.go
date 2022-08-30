// Copyright 2016-2018, Pulumi Corporation.
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
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

// In the tests below we use temporary directories and then expect DetectProjectAndPath to return a path to
// that directory. However DetectProjectAndPath will do symlink resolution, while ioutil.TempDir normally does
// not. This can lead to asserts especially on macos where TmpDir will have returned /var/folders/XX, but
// after sym link resolution that is /private/var/folders/XX.
func mkTempDir(t *testing.T, pattern string) string {
	tmpDir, err := ioutil.TempDir("", pattern)
	assert.NoError(t, err)
	result, err := filepath.EvalSymlinks(tmpDir)
	assert.NoError(t, err)
	return result
}

// nolint: paralleltest // This test uses and changes the current working directory
func TestMigrate(t *testing.T) {
	ctx := context.Background()

	saltA, smA, err := passphrase.NewPassphaseSecretsManagerFromPhrase("testA")
	assert.NoError(t, err)

	resources := []*resource.State{
		{
			URN:  resource.NewURN("a", "proj", "d:e:f", "a:b:c", "test"),
			Type: "a:b:c",
			Inputs: resource.PropertyMap{
				resource.PropertyKey("secret"): resource.MakeSecret(resource.NewStringProperty("s3cr3t")),
			},
		},
	}

	snap := deploy.NewSnapshot(deploy.Manifest{}, smA, resources, nil)

	sdep, err := stack.SerializeDeployment(snap, snap.SecretsManager, false /* showSecrsts */)
	assert.NoError(t, err)

	data, err := json.Marshal(sdep)
	assert.NoError(t, err)

	deployment := &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(data),
	}

	mockStack := &backend.MockStack{
		ExportDeploymentF: func(ctx context.Context) (*apitype.UntypedDeployment, error) {
			return deployment, nil
		},
		RefF: func() backend.StackReference {
			return &mockStackReference{name: "some_stack"}
		},
		DefaultSecretManagerF: func(configFile string) (secrets.Manager, error) {
			return b64.NewBase64SecretsManager(), nil
		},
	}

	backendInstance = &backend.MockBackend{
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &mockStackReference{name: s}, nil
		},
		GetStackF: func(ctx context.Context, stack backend.StackReference) (
			backend.Stack, error) {
			assert.Equal(t, "some_stack", stack.String())
			return mockStack, nil
		},
	}

	var mockDeployment *apitype.UntypedDeployment
	mockTargetStack := &backend.MockStack{
		ImportDeploymentF: func(ctx context.Context, deployment *apitype.UntypedDeployment) error {
			mockDeployment = deployment
			return nil
		},
		ExportDeploymentF: func(ctx context.Context) (*apitype.UntypedDeployment, error) {
			return mockDeployment, nil
		},
		RefF: func() backend.StackReference {
			return &mockStackReference{name: "some_stack"}
		},
		DefaultSecretManagerF: func(configFile string) (secrets.Manager, error) {
			if configFile == "" {
				f, err := workspace.DetectProjectStackPath("some_stack")
				if err != nil {
					return nil, err
				}
				configFile = f
			}

			info, err := workspace.LoadProjectStack(configFile)
			if err != nil {
				return nil, err
			}

			// If there are any other secrets providers set in the config, remove them
			info.EncryptionSalt = ""
			info.EncryptedKey = ""
			info.SecretsProvider = ""

			if err = info.Save(configFile); err != nil {
				return nil, err
			}

			return b64.NewBase64SecretsManager(), nil
		},
	}

	targetBackend := &backend.MockBackend{
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &mockStackReference{name: s}, nil
		},
		CreateStackF: func(ctx context.Context, sr backend.StackReference, i interface{}) (backend.Stack, error) {
			assert.Equal(t, "some_stack", sr.String())
			return mockTargetStack, nil
		},
	}

	getBackend := func(ctx context.Context, opts display.Options, url string) (backend.Backend, error) {
		assert.Equal(t, "file://test", url)
		return targetBackend, nil
	}

	tmpDir := mkTempDir(t, "TestMigrate")
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	defer func() { err := os.Chdir(cwd); assert.NoError(t, err) }()
	err = os.Chdir(tmpDir)
	assert.NoError(t, err)

	yamlPath := filepath.Join(tmpDir, "Pulumi.yaml")
	yamlContents :=
		"name: some_project\ndescription: Some project\nruntime: nodejs\n"
	err = os.WriteFile(yamlPath, []byte(yamlContents), 0600)
	assert.NoError(t, err)

	encrypter, err := smA.Encrypter()
	assert.NoError(t, err)
	secretConfig, err := encrypter.EncryptValue(ctx, "password")
	assert.NoError(t, err)

	configPath := filepath.Join(tmpDir, "Pulumi.some_stack.yaml")
	configContents := "" +
		"encryptionsalt: " + saltA + "\n" +
		"config:\n" +
		"  some_project:some_key: some_value\n" +
		"  some_project:some_secret:\n" +
		"    secure: " + secretConfig + "\n"
	err = os.WriteFile(configPath, []byte(configContents), 0600)
	assert.NoError(t, err)

	err = migrateStack(
		ctx, display.Options{},
		getBackend,
		"some_stack", "",
		"file://test", "")
	assert.NoError(t, err)

	// Assert that the new snapshot and config have the correct b64 secrets
	actualConfigContents, err := ioutil.ReadFile(configPath)
	assert.NoError(t, err)
	expectedConfigContents := "" +
		"config:\n" +
		"  some_project:some_key: some_value\n" +
		"  some_project:some_secret:\n" +
		"    secure: cGFzc3dvcmQ=\n"
	assert.Equal(t, expectedConfigContents, string(actualConfigContents))

	actualSnapshot, err := stack.DeserializeUntypedDeployment(ctx, mockDeployment, stack.DefaultSecretsProvider)
	assert.NoError(t, err)
	secretInput := actualSnapshot.Resources[0].Inputs["secret"]
	assert.Equal(t, true, secretInput.IsSecret())
	assert.Equal(t, "s3cr3t", secretInput.SecretValue().Element.StringValue())
}
