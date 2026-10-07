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

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func inlineEnvironmentStack(t *testing.T, yaml string) *workspace.ProjectStack {
	t.Helper()
	ps, err := workspace.LoadProjectStackBytes(
		diagtest.LogSink(t),
		&workspace.Project{Name: "proj"}, []byte(yaml), "Pulumi.stack.yaml", encoding.YAML)
	require.NoError(t, err)
	return ps
}

func stackFileYAML(t *testing.T, ps *workspace.ProjectStack) string {
	t.Helper()
	out, err := encoding.YAML.Marshal(ps)
	require.NoError(t, err)
	return string(out)
}

func TestSetStackConfigValueInlineEnvironment(t *testing.T) {
	t.Parallel()

	t.Run("plain values go to the environment and leave the config block", func(t *testing.T) {
		t.Parallel()
		ps := inlineEnvironmentStack(t, "environment:\n  values:\n    pulumiConfig: {}\nconfig:\n  proj:count: \"1\"\n")

		require.NoError(t, setStackConfigValue(ps, config.MustMakeKey("proj", "count"), config.NewValue("3"), false))
		require.NoError(t, setStackConfigValue(ps, config.MustMakeKey("aws", "region"), config.NewValue("us-west-2"), false))
		require.NoError(t, setStackConfigValue(
			ps, config.MustMakeKey("proj", "limits.max"), config.NewValue("10"), true))
		require.NoError(t, setStackConfigValue(
			ps, config.MustMakeKey("proj", "typed"), config.NewTypedValue("true", config.TypeBool), false))

		assert.Equal(t, `environment:
  values:
    pulumiConfig:
      proj:count: "3"
      aws:region: us-west-2
      proj:limits:
        max: 10
      proj:typed: true
`, stackFileYAML(t, ps))
	})

	t.Run("secrets stay in the config block and leave the environment", func(t *testing.T) {
		t.Parallel()
		ps := inlineEnvironmentStack(t, "environment:\n  values:\n    pulumiConfig:\n      proj:token: plain\n")

		require.NoError(t, setStackConfigValue(
			ps, config.MustMakeKey("proj", "token"), config.NewSecureValue("ciphertext"), false))

		assert.Equal(t, `environment:
  values:
    pulumiConfig: {}
config:
  proj:token:
    secure: ciphertext
`, stackFileYAML(t, ps))
	})

	t.Run("without an inline environment the config block is used", func(t *testing.T) {
		t.Parallel()
		ps := inlineEnvironmentStack(t, "environment:\n  - shared\n")

		require.NoError(t, setStackConfigValue(ps, config.MustMakeKey("proj", "count"), config.NewValue("3"), false))
		assert.Equal(t, "environment:\n  - shared\nconfig:\n  proj:count: \"3\"\n", stackFileYAML(t, ps))
	})
}

func TestRemoveStackConfigValueInlineEnvironment(t *testing.T) {
	t.Parallel()

	ps := inlineEnvironmentStack(t, `environment:
  values:
    pulumiConfig:
      proj:count: "3"
      proj:limits:
        max: 10
        min: 1
config:
  proj:count: "1"
  proj:token:
    secure: ciphertext
`)

	require.NoError(t, removeStackConfigValue(ps, config.MustMakeKey("proj", "count"), false))
	require.NoError(t, removeStackConfigValue(ps, config.MustMakeKey("proj", "limits.max"), true))
	require.NoError(t, removeStackConfigValue(ps, config.MustMakeKey("proj", "missing"), false))

	assert.Equal(t, `environment:
  values:
    pulumiConfig:
      proj:limits:
        min: 1
config:
  proj:token:
    secure: ciphertext
`, stackFileYAML(t, ps))
}

func TestConfigSetWritesInlineEnvironment(t *testing.T) {
	t.Parallel()

	project := workspace.Project{Name: "testProject"}
	s := backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName("testStack"),
				FullyQualifiedNameV: "org/testProject/testStack",
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{} },
	}
	configSetCmd := &configSetCmd{
		LoadProjectStack: func(
			_ context.Context, diags diag.Sink, project *workspace.Project, _ backend.Stack, _ string,
		) (*workspace.ProjectStack, error) {
			return workspace.LoadProjectStackBytes(diags, project,
				[]byte("environment:\n  values:\n    pulumiConfig:\n      testProject:existing: keep\n"),
				"Pulumi.stack.yaml", encoding.YAML)
		},
	}

	configFile := filepath.Join(t.TempDir(), "Pulumi.stack.yaml")
	err := configSetCmd.Run(t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:test", "123"},
		&project, &s, configFile)
	require.NoError(t, err)

	data, err := os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Equal(t, "environment:\n  values:\n    pulumiConfig:\n      testProject:existing: keep\n"+
		"      testProject:test: \"123\"\n", string(data))
}

func TestSetSecretStackConfigValueInlineEnvironment(t *testing.T) {
	t.Parallel()

	localEncrypt := func(plaintext string) (string, error) { return "local:" + plaintext, nil }

	t.Run("encrypts with the stack environment's key", func(t *testing.T) {
		t.Parallel()
		ps := inlineEnvironmentStack(t,
			"environment:\n  values:\n    pulumiConfig: {}\nconfig:\n  proj:token:\n    secure: old\n")
		var encrypted []string
		stack := &backend.MockStack{BackendF: func() backend.Backend {
			return &backend.MockStackEnvironmentsBackend{
				EncryptStackEnvironmentSecretF: func(
					_ context.Context, _ backend.Stack, plaintext string,
				) (*backend.StackEnvironmentSecret, error) {
					encrypted = append(encrypted, plaintext)
					return &backend.StackEnvironmentSecret{Environment: "proj/stack", Ciphertext: "esc:" + plaintext}, nil
				},
			}
		}}

		require.NoError(t, setSecretStackConfigValue(
			t.Context(), stack, ps, config.MustMakeKey("proj", "token"), false, "hunter2", localEncrypt))

		assert.Equal(t, []string{"hunter2"}, encrypted)
		assert.Equal(t, `environment:
  values:
    pulumiConfig:
      proj:token:
        fn::secret:
          ciphertext: esc:hunter2
`, stackFileYAML(t, ps))
	})

	t.Run("falls back to the config block on an older service", func(t *testing.T) {
		t.Parallel()
		ps := inlineEnvironmentStack(t, "environment:\n  values:\n    pulumiConfig: {}\n")
		stack := &backend.MockStack{BackendF: func() backend.Backend {
			return &backend.MockStackEnvironmentsBackend{
				EncryptStackEnvironmentSecretF: func(
					context.Context, backend.Stack, string,
				) (*backend.StackEnvironmentSecret, error) {
					return nil, backend.ErrStackEnvironmentSyncUnsupported
				},
			}
		}}

		require.NoError(t, setSecretStackConfigValue(
			t.Context(), stack, ps, config.MustMakeKey("proj", "token"), false, "hunter2", localEncrypt))
		assert.Equal(t, "environment:\n  values:\n    pulumiConfig: {}\nconfig:\n  proj:token:\n    secure: local:hunter2\n",
			stackFileYAML(t, ps))
	})

	t.Run("uses the config block without an inline environment", func(t *testing.T) {
		t.Parallel()
		ps := inlineEnvironmentStack(t, "environment:\n  - shared\n")
		stack := &backend.MockStack{BackendF: func() backend.Backend { return &backend.MockStackEnvironmentsBackend{} }}

		require.NoError(t, setSecretStackConfigValue(
			t.Context(), stack, ps, config.MustMakeKey("proj", "token"), false, "hunter2", localEncrypt))
		assert.Equal(t, "environment:\n  - shared\nconfig:\n  proj:token:\n    secure: local:hunter2\n", stackFileYAML(t, ps))
	})
}
