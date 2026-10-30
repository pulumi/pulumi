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

package newcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// managedNewBackend is guidedNewTestBackend with managed environments, recording the plaintexts it encrypts.
type managedNewBackend struct {
	backend.MockManagedEnvironmentsBackend
	// The plaintexts of each EncryptEnvironmentSecrets call.
	encrypted [][]string
}

// managedNewTestBackend's stacks manage environment proj/dev when created with ManagedEnvironment.
func managedNewTestBackend(t *testing.T) (*managedNewBackend, *[]string) {
	t.Helper()

	base, created := guidedNewTestBackend(t)
	b := &managedNewBackend{}
	b.MockBackend = *base
	b.CreateStackF = func(ctx context.Context, ref backend.StackReference, root string,
		initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
	) (backend.Stack, error) {
		s, err := base.CreateStackF(ctx, ref, root, initialState, opts)
		if err != nil {
			return nil, err
		}
		ms := s.(*backend.MockStack)
		ms.BackendF = func() backend.Backend { return b }
		ms.OrgNameF = func() string { return "my-org" }
		if opts.ManagedEnvironment {
			ms.ConfigLocationF = func() backend.StackConfigLocation {
				return backend.StackConfigLocation{ManagedEnvironment: "proj/dev"}
			}
		}
		ms.DefaultSecretManagerF = func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			t.Error("the stack secrets manager must not encrypt config for a managed stack")
			return nil, errors.New("unexpected stack secrets manager")
		}
		return ms, nil
	}
	b.EncryptEnvironmentSecretsF = func(
		_ context.Context, org, project, name string, plaintexts [][]byte,
	) ([]string, error) {
		assert.Equal(t, "my-org/proj/dev", org+"/"+project+"/"+name)
		call := make([]string, len(plaintexts))
		ciphertexts := make([]string, len(plaintexts))
		for i, p := range plaintexts {
			call[i] = string(p)
			ciphertexts[i] = fmt.Sprintf("ct%d", len(b.encrypted)+1)
		}
		b.encrypted = append(b.encrypted, call)
		return ciphertexts, nil
	}
	b.EvaluateManagedEnvironmentF = func(
		context.Context, string, string, string, []byte,
	) (*backend.ManagedEnvironmentEvaluation, apitype.EnvironmentDiagnostics, error) {
		t.Error("pulumi new must not evaluate the managed environment")
		return nil, nil, errors.New("unexpected EvaluateManagedEnvironment call")
	}
	return b, created
}

const managedTemplateYAML = `name: ${PROJECT}
description: ${DESCRIPTION}
runtime: yaml
template:
  description: A managed config test template
  config:
    greeting:
      description: The greeting
      default: hello
    apiKey:
      description: The API key
      secret: true
      default: key-secret
    token:
      description: The token
      secret: true
      default: token-secret
`

var managedTemplatePulumiConfig = map[string]any{
	"proj:apiKey":   map[string]any{"fn::secret": map[string]any{"ciphertext": "ct1"}},
	"proj:greeting": "hello",
	"proj:token":    map[string]any{"fn::secret": map[string]any{"ciphertext": "ct2"}},
}

func managedNewArgs(template string) newArgs {
	return newArgs{
		yes:               true,
		prompt:            promptMock("proj", "dev"),
		secretsProvider:   "default",
		templateNameOrURL: template,
		languageTemplate:  languageTemplateMock,
		escConfig:         true,
	}
}

func readStackFile(t *testing.T, dir string) string {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(dir, "Pulumi.dev.yaml"))
	require.NoError(t, err)
	return string(contents)
}

// assertOnlyPulumiConfig compares decoded YAML so the test doesn't depend on the mapping's style.
func assertOnlyPulumiConfig(t *testing.T, dir string, expected map[string]any) {
	t.Helper()

	var file map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(readStackFile(t, dir)), &file))
	assert.Equal(t, map[string]any{"environment": map[string]any{"values": map[string]any{"pulumiConfig": expected}}},
		file)
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestNewEscConfigWritesTemplateConfigToEnvironment(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	b, created := managedNewTestBackend(t)
	mockCurrentBackend(t, b)

	err := runNew(t.Context(),
		managedNewArgs(writeLocalTemplate(t, t.TempDir(), "managed", managedTemplateYAML)))

	require.NoError(t, err)
	assert.Equal(t, []string{"my-org/dev"}, *created)
	assertOnlyPulumiConfig(t, tempdir, managedTemplatePulumiConfig)
	assert.Equal(t, [][]string{{"key-secret"}, {"token-secret"}}, b.encrypted)
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestGuidedNewEscConfigWritesTemplateConfigToEnvironment(t *testing.T) {
	guidedRepoTemplate(t, "aws-python", managedTemplateYAML)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	b, created := managedNewTestBackend(t)
	mockCurrentBackend(t, b)

	selectOne, _ := scriptedSelect(t, "AWS", "Python", confirmYes)
	var out bytes.Buffer
	err := runNew(t.Context(), newArgs{
		interactive:          true,
		name:                 "proj",
		prompt:               promptMock("proj", "dev"),
		promptRuntimeOptions: runtimeOptionsNone,
		languageTemplate:     languageTemplateMock,
		selectOne:            selectOne,
		secretsProvider:      "default",
		stdout:               &out,
		escConfig:            true,
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"my-org/dev"}, *created)
	assertOnlyPulumiConfig(t, tempdir, managedTemplatePulumiConfig)
	assert.Equal(t, [][]string{{"key-secret"}, {"token-secret"}}, b.encrypted)
	assert.Regexp(t, `ESC environment:\s+proj/dev`, out.String())
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestNewEscConfigWritesCommandLineConfigToEnvironment(t *testing.T) {
	tests := []struct {
		name       string
		config     []string
		configPath bool
		expected   map[string]any
	}{
		{
			name:     "key",
			config:   []string{"extra=value"},
			expected: map[string]any{"proj:extra": "value"},
		},
		{
			// What `pulumi config set --path a.b 42` writes.
			name:       "path",
			config:     []string{"a.b=42"},
			configPath: true,
			expected:   map[string]any{"proj:a": map[string]any{"b": 42}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempdir := tempProjectDir(t)
			t.Chdir(tempdir)
			b, _ := managedNewTestBackend(t)
			mockCurrentBackend(t, b)
			args := managedNewArgs(localTemplate(t))
			args.configArray = tt.config
			args.configPath = tt.configPath

			err := runNew(t.Context(), args)

			require.NoError(t, err)
			assertOnlyPulumiConfig(t, tempdir, tt.expected)
			assert.Empty(t, b.encrypted)
		})
	}
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestNewEscConfigEncryptFailureKeepsPlaceholder(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	b, _ := managedNewTestBackend(t)
	b.EncryptEnvironmentSecretsF = func(context.Context, string, string, string, [][]byte) ([]string, error) {
		return nil, errors.New("service unavailable")
	}
	mockCurrentBackend(t, b)

	err := runNew(t.Context(),
		managedNewArgs(writeLocalTemplate(t, t.TempDir(), "managed", managedTemplateYAML)))

	require.ErrorContains(t, err, "service unavailable. Stack my-org/dev and its ESC environment were created; "+
		"no config was saved, so set each value with `pulumi config set` (`--secret` for secrets), or remove them "+
		"with `pulumi stack rm my-org/dev`")
	assert.Equal(t, "environment:\n  values:\n    pulumiConfig: {}\n", readStackFile(t, tempdir))
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestNewEscConfigFailureBeforeConfigSaveNamesRecovery(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	b, created := managedNewTestBackend(t)
	mockCurrentBackend(t, b)
	args := managedNewArgs(writeLocalTemplate(t, t.TempDir(), "managed", managedTemplateYAML))
	args.languageTemplate = func(context.Context, plugin.LanguageRuntime, plugin.ProgramInfo, tokens.PackageName) error {
		return errors.New("template failed")
	}

	err := runNew(t.Context(), args)

	require.ErrorContains(t, err, "language template: template failed. Stack my-org/dev and its ESC environment were "+
		"created; no config was saved, so set each value with `pulumi config set` (`--secret` for secrets), or remove "+
		"them with `pulumi stack rm my-org/dev`")
	assert.Equal(t, []string{"my-org/dev"}, *created)
	assertOnlyPulumiConfig(t, tempdir, map[string]any{})
	assert.Empty(t, b.encrypted)
}

//nolint:paralleltest // changes directory for process
func TestNewEscConfigOnDIYBackendFailsBeforeCopy(t *testing.T) {
	useTempFilestateBackend(t)
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	err := runNew(t.Context(), managedNewArgs(localTemplate(t)))

	require.ErrorIs(t, err, backend.ErrManagedEnvironmentNotSupported)
	assert.NoFileExists(t, filepath.Join(tempdir, "Pulumi.yaml"))
}

func TestNewEscConfigFlagConflicts(t *testing.T) {
	t.Parallel()

	for _, other := range []string{"remote-stack-config", "generate-only"} {
		t.Run(other, func(t *testing.T) {
			t.Parallel()

			cmd := NewNewCmd()
			cmd.SetArgs([]string{"--esc-config", "--" + other})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)

			err := cmd.Execute()

			assert.ErrorContains(t, err, "[esc-config "+other+"] are set none of the others can be")
		})
	}
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestNewExistingStackWithManagedEnvOrEscConfigFailsBeforeCopy(t *testing.T) {
	tests := []struct {
		name       string
		managedEnv string
		escConfig  bool
		err        string
	}{
		{
			name:       "managed",
			managedEnv: "proj/dev",
			err:        "pulumi new --stack is not supported on stacks with an ESC-managed environment yet",
		},
		{
			name:       "managed with --esc-config",
			managedEnv: "proj/dev",
			escConfig:  true,
			err:        "pulumi new --stack is not supported on stacks with an ESC-managed environment yet",
		},
		{
			name:      "unmanaged with --esc-config",
			escConfig: true,
			err:       "--esc-config creates a new stack: stack 'my-org/proj/dev' already exists",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempdir := tempProjectDir(t)
			t.Chdir(tempdir)
			b, created := managedNewTestBackend(t)
			b.GetStackF = func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
				return &backend.MockStack{
					RefF:  func() backend.StackReference { return ref },
					TagsF: func() map[apitype.StackTagName]string { return nil },
					ConfigLocationF: func() backend.StackConfigLocation {
						return backend.StackConfigLocation{ManagedEnvironment: tt.managedEnv}
					},
				}, nil
			}
			mockCurrentBackend(t, b)
			args := managedNewArgs(localTemplate(t))
			args.stack = "my-org/proj/dev"
			args.escConfig = tt.escConfig

			err := runNew(t.Context(), args)

			require.EqualError(t, err, tt.err)
			assert.NoFileExists(t, filepath.Join(tempdir, "Pulumi.yaml"))
			assert.Empty(t, *created)
		})
	}
}
