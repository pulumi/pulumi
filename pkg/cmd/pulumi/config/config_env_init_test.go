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

package config

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type base64EvalCrypter struct{}

func newBase64EvalCrypter() (evalCrypter, error) {
	return base64EvalCrypter{}, nil
}

func (c base64EvalCrypter) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	ciphertext, err := config.Base64Crypter.EncryptValue(ctx, string(plaintext))
	if err != nil {
		return nil, err
	}
	return []byte(ciphertext), nil
}

func (c base64EvalCrypter) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	plaintext, err := config.Base64Crypter.DecryptValue(ctx, string(ciphertext))
	if err != nil {
		return nil, err
	}
	return []byte(plaintext), nil
}

func TestConfigEnvInit(t *testing.T) {
	t.Parallel()

	projectYAML := `name: test
runtime: yaml`

	t.Run("no config", func(t *testing.T) {
		t.Parallel()

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForInitTest(stdin, &stdout, projectYAML, "", &newStackYAML, envDefMap{})
		init := &configEnvInitCmd{parent: parent, newCrypter: newBase64EvalCrypter, yes: true}
		ctx := context.Background()
		err := init.run(ctx, nil)
		require.NoError(t, err)

		const expectedOut = "Creating environment test/stack for stack stack...\n" +
			"# Value\n" +
			"```json\n" +
			"{\n" +
			"  \"pulumiConfig\": {}\n" +
			"}\n" +
			"```\n" +
			"# Definition\n" +
			"```yaml\n" +
			"values:\n" +
			"  pulumiConfig: {}\n" +
			"\n" +
			"```\n" +
			""

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - test/stack
`

		assert.Equal(t, expectedYAML, newStackYAML)
	})

	t.Run("some config", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		cfg := map[config.Key]config.Value{
			config.MustMakeKey("aws", "region"):   config.NewValue("us-west-2"),
			config.MustMakeKey("app", "password"): config.NewSecureValue("aHVudGVyMg==" /*base64 of hunter2*/),
			config.MustMakeKey("app", "tags"):     config.NewObjectValue(`{"env":"testing","owners":["alice","bob"]}`),
		}

		stackYAML, err := encoding.YAML.Marshal(workspace.ProjectStack{Config: cfg})
		require.NoError(t, err)

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForInitTest(stdin, &stdout, projectYAML, string(stackYAML), &newStackYAML, envDefMap{})
		init := &configEnvInitCmd{parent: parent, newCrypter: newBase64EvalCrypter, yes: true}
		err = init.run(ctx, nil)
		require.NoError(t, err)

		const expectedOut = "Creating environment test/stack for stack stack...\n" +
			"# Value\n" +
			"```json\n" +
			"{\n" +
			"  \"pulumiConfig\": {\n" +
			"    \"app:password\": \"[secret]\",\n" +
			"    \"app:tags\": {\n" +
			"      \"env\": \"testing\",\n" +
			"      \"owners\": [\n" +
			"        \"alice\",\n" +
			"        \"bob\"\n" +
			"      ]\n" +
			"    },\n" +
			"    \"aws:region\": \"us-west-2\"\n" +
			"  }\n" +
			"}\n" +
			"```\n" +
			"# Definition\n" +
			"```yaml\n" +
			"values:\n" +
			"  pulumiConfig:\n" +
			"    app:password:\n" +
			"      fn::secret:\n" +
			"        ciphertext: ZXNjeAAAAAFhSFZ1ZEdWeU1nPT2+gKwa\n" +
			"    app:tags:\n" +
			"      env: testing\n" +
			"      owners:\n" +
			"        - alice\n" +
			"        - bob\n" +
			"    aws:region: us-west-2\n" +
			"\n" +
			"```\n" +
			""

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - test/stack
`
		assert.Equal(t, expectedYAML, newStackYAML)
	})

	t.Run("some config, show secrets", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		cfg := map[config.Key]config.Value{
			config.MustMakeKey("aws", "region"):   config.NewValue("us-west-2"),
			config.MustMakeKey("app", "password"): config.NewSecureValue("aHVudGVyMg==" /*base64 of hunter2*/),
			config.MustMakeKey("app", "tags"):     config.NewObjectValue(`{"env":"testing","owners":["alice","bob"]}`),
		}

		stackYAML, err := encoding.YAML.Marshal(workspace.ProjectStack{Config: cfg})
		require.NoError(t, err)

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForInitTest(stdin, &stdout, projectYAML, string(stackYAML), &newStackYAML, envDefMap{})
		init := &configEnvInitCmd{parent: parent, newCrypter: newBase64EvalCrypter, showSecrets: true, yes: true}
		err = init.run(ctx, nil)
		require.NoError(t, err)

		const expectedOut = "Creating environment test/stack for stack stack...\n" +
			"# Value\n" +
			"```json\n" +
			"{\n" +
			"  \"pulumiConfig\": {\n" +
			"    \"app:password\": \"hunter2\",\n" +
			"    \"app:tags\": {\n" +
			"      \"env\": \"testing\",\n" +
			"      \"owners\": [\n" +
			"        \"alice\",\n" +
			"        \"bob\"\n" +
			"      ]\n" +
			"    },\n" +
			"    \"aws:region\": \"us-west-2\"\n" +
			"  }\n" +
			"}\n" +
			"```\n" +
			"# Definition\n" +
			"```yaml\n" +
			"values:\n" +
			"  pulumiConfig:\n" +
			"    app:password:\n" +
			"      fn::secret: hunter2\n" +
			"    app:tags:\n" +
			"      env: testing\n" +
			"      owners:\n" +
			"        - alice\n" +
			"        - bob\n" +
			"    aws:region: us-west-2\n" +
			"\n" +
			"```\n" +
			""

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - test/stack
`
		assert.Equal(t, expectedYAML, newStackYAML)
	})

	t.Run("other env, some config, show secrets", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		cfg := map[config.Key]config.Value{
			config.MustMakeKey("aws", "region"):   config.NewValue("us-west-2"),
			config.MustMakeKey("app", "password"): config.NewSecureValue("aHVudGVyMg==" /*base64 of hunter2*/),
			config.MustMakeKey("app", "tags"):     config.NewObjectValue(`{"env":"testing","owners":["alice","bob"]}`),
		}

		stackYAML, err := encoding.YAML.Marshal(workspace.ProjectStack{
			Environment: workspace.NewEnvironment([]string{"env"}),
			Config:      cfg,
		})
		require.NoError(t, err)

		var newStackYAML string
		stdin := strings.NewReader("y")
		var stdout bytes.Buffer
		parent := newConfigEnvCmdForInitTest(stdin, &stdout, projectYAML, string(stackYAML), &newStackYAML, envDefMap{
			"env": `{"values": {"pulumiConfig": {"app:tags": {"name": "project"}}}}`,
		})
		init := &configEnvInitCmd{parent: parent, newCrypter: newBase64EvalCrypter, showSecrets: true, yes: true}
		err = init.run(ctx, nil)
		require.NoError(t, err)

		const expectedOut = "Creating environment test/stack for stack stack...\n" +
			"# Value\n" +
			"```json\n" +
			"{\n" +
			"  \"pulumiConfig\": {\n" +
			"    \"app:password\": \"hunter2\",\n" +
			"    \"app:tags\": {\n" +
			"      \"env\": \"testing\",\n" +
			"      \"owners\": [\n" +
			"        \"alice\",\n" +
			"        \"bob\"\n" +
			"      ]\n" +
			"    },\n" +
			"    \"aws:region\": \"us-west-2\"\n" +
			"  }\n" +
			"}\n" +
			"```\n" +
			"# Definition\n" +
			"```yaml\n" +
			"values:\n" +
			"  pulumiConfig:\n" +
			"    app:password:\n" +
			"      fn::secret: hunter2\n" +
			"    app:tags:\n" +
			"      env: testing\n" +
			"      owners:\n" +
			"        - alice\n" +
			"        - bob\n" +
			"    aws:region: us-west-2\n" +
			"\n" +
			"```\n" +
			""

		assert.Equal(t, expectedOut, cleanStdoutIncludingPrompt(stdout.String()))

		const expectedYAML = `environment:
  - env
  - test/stack
`
		assert.Equal(t, expectedYAML, newStackYAML)
	})
}

type migrateTestBackend struct {
	createdYAML  []byte
	updatedYAML  []byte
	existingYAML []byte
	existingEtag string
}

func newConfigEnvCmdForMigrateTest(
	stdin io.Reader,
	stdout io.Writer,
	projectYAML string,
	projectStackYAML string,
	newStackYAML *string,
	configLocation backend.StackConfigLocation,
	tb *migrateTestBackend,
	secretsManager secrets.Manager,
) *configEnvCmd {
	stackRef := "dev"
	return &configEnvCmd{
		stdin:       stdin,
		stdout:      stdout,
		interactive: true,

		ws: &pkgWorkspace.MockContext{
			ReadProjectF: func() (*workspace.Project, string, error) {
				p, err := workspace.LoadProjectBytes([]byte(projectYAML), "Pulumi.yaml", encoding.YAML)
				if err != nil {
					return nil, "", err
				}
				return p, "", nil
			},
		},
		requireStack: func(
			ctx context.Context,
			sink diag.Sink,
			ws pkgWorkspace.Context,
			lm cmdBackend.LoginManager,
			stackName string,
			lopt cmdStack.LoadOption,
			opts display.Options,
		) (backend.Stack, error) {
			return &backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						StringV:             "org/dev",
						NameV:               tokens.MustParseStackName("dev"),
						ProjectV:            "project",
						FullyQualifiedNameV: "org/dev",
					}
				},
				OrgNameF: func() string {
					return "org"
				},
				BackendF: func() backend.Backend {
					return &backend.MockEnvironmentsBackend{
						CreateEnvironmentF: func(
							_ context.Context, _, _, _ string, yamlBytes []byte,
						) (apitype.EnvironmentDiagnostics, error) {
							tb.createdYAML = yamlBytes
							return nil, nil
						},
						GetEnvironmentF: func(
							_ context.Context, _, _, _, _ string, _ bool,
						) ([]byte, string, int, error) {
							if tb.existingYAML != nil {
								return tb.existingYAML, tb.existingEtag, 1, nil
							}
							return nil, "", 0, &apitype.ErrorResponse{Code: http.StatusNotFound}
						},
						UpdateEnvironmentWithProjectF: func(
							_ context.Context, _, _, _ string, yamlBytes []byte, _ string,
						) (apitype.EnvironmentDiagnostics, error) {
							tb.updatedYAML = yamlBytes
							return nil, nil
						},
					}
				},
				DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
					return secretsManager, nil
				},
				ConfigLocationF: func() backend.StackConfigLocation {
					return configLocation
				},
				SaveRemoteF: func(_ context.Context, ps *workspace.ProjectStack) error {
					b, err := encoding.YAML.Marshal(ps)
					if err != nil {
						return err
					}
					*newStackYAML = string(b)
					return nil
				},
			}, nil
		},

		loadProjectStack: func(
			_ context.Context, d diag.Sink, p *workspace.Project, _ backend.Stack,
		) (*workspace.ProjectStack, error) {
			return workspace.LoadProjectStackBytes(d, p, []byte(projectStackYAML), "Pulumi.dev.yaml", encoding.YAML)
		},
		saveProjectStack: func(_ context.Context, _ backend.Stack, ps *workspace.ProjectStack) error {
			b, err := encoding.YAML.Marshal(ps)
			if err != nil {
				return err
			}
			*newStackYAML = string(b)
			return nil
		},
		stackRef: &stackRef,
	}
}

type failingSecretsManager struct{}

func (f *failingSecretsManager) Type() string                { return "failing" }
func (f *failingSecretsManager) State() json.RawMessage      { return nil }
func (f *failingSecretsManager) Encrypter() config.Encrypter { return config.NopEncrypter }
func (f *failingSecretsManager) Decrypter() config.Decrypter {
	return config.NewErrorCrypter("invalid passphrase")
}

func TestConfigEnvInit_RemoteConfig_AllSecretsDecrypted(t *testing.T) {
	t.Parallel()

	projectYAML := "name: myapp\nruntime: yaml"
	cfg := config.Map{
		config.MustMakeKey("myapp", "region"):   config.NewValue("us-east-1"),
		config.MustMakeKey("myapp", "password"): config.NewSecureValue("aHVudGVyMg=="),
	}
	stackYAML, err := encoding.YAML.Marshal(workspace.ProjectStack{Config: cfg})
	require.NoError(t, err)

	var newStackYAML string
	var stdout bytes.Buffer
	tb := &migrateTestBackend{}
	parent := newConfigEnvCmdForMigrateTest(
		strings.NewReader(""), &stdout,
		projectYAML, string(stackYAML), &newStackYAML,
		backend.StackConfigLocation{},
		tb,
		b64.NewBase64SecretsManager(),
	)

	cmd := &configEnvInitCmd{parent: parent, remoteConfig: true, yes: true}
	err = cmd.run(context.Background(), nil)
	require.NoError(t, err)

	require.NotNil(t, tb.createdYAML, "environment should have been created")

	var envDef map[string]any
	require.NoError(t, yaml.Unmarshal(tb.createdYAML, &envDef))
	values, _ := envDef["values"].(map[string]any)
	require.NotNil(t, values)
	pc, _ := values["pulumiConfig"].(map[string]any)
	require.NotNil(t, pc)

	assert.Equal(t, "us-east-1", pc["myapp:region"])

	secret, ok := pc["myapp:password"].(map[string]any)
	require.True(t, ok, "password should be wrapped in fn::secret")
	assert.Equal(t, "hunter2", secret["fn::secret"])

	assert.Contains(t, newStackYAML, "myapp/dev")
	assert.Contains(t, stdout.String(), "Migrated config to environment myapp/dev")
}

func TestConfigEnvInit_RemoteConfig_DecryptionFailure(t *testing.T) {
	t.Parallel()

	projectYAML := "name: myapp\nruntime: yaml"
	cfg := config.Map{
		config.MustMakeKey("myapp", "dbpass"): config.NewSecureValue("aHVudGVyMg=="),
	}
	stackYAML, err := encoding.YAML.Marshal(workspace.ProjectStack{Config: cfg})
	require.NoError(t, err)

	var newStackYAML string
	var stdout bytes.Buffer
	tb := &migrateTestBackend{}
	parent := newConfigEnvCmdForMigrateTest(
		strings.NewReader(""), &stdout,
		projectYAML, string(stackYAML), &newStackYAML,
		backend.StackConfigLocation{},
		tb,
		&failingSecretsManager{},
	)

	cmd := &configEnvInitCmd{parent: parent, remoteConfig: true, yes: true}
	err = cmd.run(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decrypting config")

	assert.Nil(t, tb.createdYAML, "no environment should have been created")
	assert.Nil(t, tb.updatedYAML, "no environment should have been updated")
	assert.Empty(t, newStackYAML, "stack config should not have been saved")
}

func TestConfigEnvInit_RemoteConfig_IdempotentMerge(t *testing.T) {
	t.Parallel()

	projectYAML := "name: myapp\nruntime: yaml"
	cfg := config.Map{
		config.MustMakeKey("myapp", "region"): config.NewValue("us-west-2"),
		config.MustMakeKey("myapp", "newkey"): config.NewValue("newval"),
	}
	stackYAML, err := encoding.YAML.Marshal(workspace.ProjectStack{Config: cfg})
	require.NoError(t, err)

	existingEnvYAML := []byte(`values:
  pulumiConfig:
    myapp:region: us-east-1
    myapp:existing: keep-me
`)

	var newStackYAML string
	var stdout bytes.Buffer
	tb := &migrateTestBackend{
		existingYAML: existingEnvYAML,
		existingEtag: "etag-1",
	}
	parent := newConfigEnvCmdForMigrateTest(
		strings.NewReader(""), &stdout,
		projectYAML, string(stackYAML), &newStackYAML,
		backend.StackConfigLocation{},
		tb,
		b64.NewBase64SecretsManager(),
	)

	cmd := &configEnvInitCmd{parent: parent, remoteConfig: true, yes: true}
	err = cmd.run(context.Background(), nil)
	require.NoError(t, err)

	require.NotNil(t, tb.updatedYAML, "environment should have been updated")
	assert.Nil(t, tb.createdYAML, "should not create when env already exists")

	var envDef map[string]any
	require.NoError(t, yaml.Unmarshal(tb.updatedYAML, &envDef))
	values, _ := envDef["values"].(map[string]any)
	pc, _ := values["pulumiConfig"].(map[string]any)
	require.NotNil(t, pc)

	assert.Equal(t, "us-west-2", pc["myapp:region"], "local value should overwrite existing")
	assert.Equal(t, "keep-me", pc["myapp:existing"], "existing-only key should be preserved")
	assert.Equal(t, "newval", pc["myapp:newkey"], "new key should be added")

	assert.Contains(t, stdout.String(), `overwriting existing key "myapp:region"`)
}

func TestConfigEnvInit_RemoteConfig_EmptyExistingEnvironmentUpdates(t *testing.T) {
	t.Parallel()

	projectYAML := "name: myapp\nruntime: yaml"
	cfg := config.Map{
		config.MustMakeKey("myapp", "region"): config.NewValue("us-west-2"),
	}
	stackYAML, err := encoding.YAML.Marshal(workspace.ProjectStack{Config: cfg})
	require.NoError(t, err)

	var newStackYAML string
	var stdout bytes.Buffer
	tb := &migrateTestBackend{
		existingYAML: []byte{},
		existingEtag: "etag-1",
	}
	parent := newConfigEnvCmdForMigrateTest(
		strings.NewReader(""), &stdout,
		projectYAML, string(stackYAML), &newStackYAML,
		backend.StackConfigLocation{},
		tb,
		b64.NewBase64SecretsManager(),
	)

	cmd := &configEnvInitCmd{parent: parent, remoteConfig: true, yes: true}
	err = cmd.run(context.Background(), nil)
	require.NoError(t, err)

	require.NotNil(t, tb.updatedYAML, "empty existing environments should be updated, not recreated")
	assert.Nil(t, tb.createdYAML)
	assert.Contains(t, newStackYAML, "myapp/dev")
}

func TestConfigEnvInit_RemoteConfig_AlreadyRemoteConfig(t *testing.T) {
	t.Parallel()

	projectYAML := "name: myapp\nruntime: yaml"

	var newStackYAML string
	var stdout bytes.Buffer
	escEnv := "myapp/dev"
	tb := &migrateTestBackend{}
	parent := newConfigEnvCmdForMigrateTest(
		strings.NewReader(""), &stdout,
		projectYAML, "", &newStackYAML,
		backend.StackConfigLocation{IsRemote: true, EscEnv: &escEnv},
		tb,
		b64.NewBase64SecretsManager(),
	)

	cmd := &configEnvInitCmd{parent: parent, remoteConfig: true, yes: true}
	err := cmd.run(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already uses remote configuration")

	assert.Nil(t, tb.createdYAML)
	assert.Nil(t, tb.updatedYAML)
	assert.Empty(t, newStackYAML)
}
