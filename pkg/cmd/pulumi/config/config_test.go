// Copyright 2025, Pulumi Corporation.
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
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // changes global ConfigFile variable
func TestConfigSet(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name     string
		args     []string
		expected string
		path     bool
	}{
		{
			name:     "toplevel bool",
			args:     []string{"testProject:test", "true"},
			expected: "config:\n  testProject:test: \"true\"\n",
		},
		{
			name:     "toplevel int",
			args:     []string{"testProject:test", "123"},
			expected: "config:\n  testProject:test: \"123\"\n",
		},
		{
			name:     "toplevel float",
			args:     []string{"testProject:test", "123.456"},
			expected: "config:\n  testProject:test: \"123.456\"\n",
		},
		{
			name:     "path'd bool",
			args:     []string{"testProject:test[0]", "true"},
			expected: "config:\n  testProject:test:\n    - true\n",
			path:     true,
		},
		{
			name:     "path'd int",
			args:     []string{"testProject:test[0]", "123"},
			expected: "config:\n  testProject:test:\n    - 123\n",
			path:     true,
		},
		{
			name:     "path'd float",
			args:     []string{"testProject:test[0]", "123.456"},
			expected: "config:\n  testProject:test:\n    - \"123.456\"\n",
			path:     true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			project := workspace.Project{
				Name: "testProject",
			}

			s := backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						NameV: tokens.MustParseStackName("testStack"),
					}
				},
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{}
				},
			}

			configSetCmd := &configSetCmd{
				Path: c.path,
				LoadProjectStack: func(
					_ context.Context,
					diags diag.Sink,
					project *workspace.Project,
					_ backend.Stack,
				) (*workspace.ProjectStack, error) {
					return workspace.LoadProjectStackBytes(diags, project, []byte{}, "Pulumi.stack.yaml", encoding.YAML)
				},
			}

			tmpdir := t.TempDir()
			stack.ConfigFile = filepath.Join(tmpdir, "Pulumi.stack.yaml")
			defer func() {
				stack.ConfigFile = ""
			}()

			ws := &pkgWorkspace.MockContext{}

			err := configSetCmd.Run(ctx, ws, c.args, &project, &s)
			require.NoError(t, err)

			// verify the config was set
			data, err := os.ReadFile(stack.ConfigFile)
			require.NoError(t, err)

			require.Equal(t, c.expected, string(data))
		})
	}
}

//nolint:paralleltest // changes global ConfigFile variable
func TestConfigSetTypes(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name     string
		args     []string
		expected string
		typ      string
		path     bool
	}{
		{
			name:     "toplevel bool",
			args:     []string{"testProject:test", "true"},
			typ:      "bool",
			expected: "config:\n  testProject:test: true\n",
		},
		{
			name:     "toplevel int",
			args:     []string{"testProject:test", "123"},
			typ:      "int",
			expected: "config:\n  testProject:test: 123\n",
		},
		{
			name:     "toplevel float",
			args:     []string{"testProject:test", "123.456"},
			typ:      "float",
			expected: "config:\n  testProject:test: 123.456\n",
		},
		{
			name:     "toplevel string",
			args:     []string{"testProject:test", "123"},
			typ:      "string",
			expected: "config:\n  testProject:test: \"123\"\n",
		},
		{
			name:     "path'd bool",
			args:     []string{"testProject:test[0]", "true"},
			typ:      "bool",
			expected: "config:\n  testProject:test:\n    - true\n",
			path:     true,
		},
		{
			name:     "path'd int",
			args:     []string{"testProject:test[0]", "123"},
			typ:      "int",
			expected: "config:\n  testProject:test:\n    - 123\n",
			path:     true,
		},
		{
			name:     "path'd float",
			args:     []string{"testProject:test[0]", "123.456"},
			typ:      "float",
			expected: "config:\n  testProject:test:\n    - 123.456\n",
			path:     true,
		},
		{
			name:     "path'd string",
			args:     []string{"testProject:test[0]", "123"},
			typ:      "string",
			expected: "config:\n  testProject:test:\n    - \"123\"\n",
			path:     true,
		},
	}

	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			project := workspace.Project{
				Name: "testProject",
			}

			s := backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						NameV: tokens.MustParseStackName("testStack"),
					}
				},
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{}
				},
			}

			configSetCmd := &configSetCmd{
				Path: c.path,
				Type: c.typ,
				LoadProjectStack: func(_ context.Context, d diag.Sink, project *workspace.Project, _ backend.Stack,
				) (*workspace.ProjectStack, error) {
					return workspace.LoadProjectStackBytes(d, project, []byte{}, "Pulumi.stack.yaml", encoding.YAML)
				},
			}

			tmpdir := t.TempDir()
			stack.ConfigFile = filepath.Join(tmpdir, "Pulumi.stack.yaml")
			defer func() {
				stack.ConfigFile = ""
			}()

			ws := &pkgWorkspace.MockContext{}

			err := configSetCmd.Run(ctx, ws, c.args, &project, &s)
			require.NoError(t, err)

			// verify the config was set
			data, err := os.ReadFile(stack.ConfigFile)
			require.NoError(t, err)

			require.Equal(t, c.expected, string(data))
		})
	}
}

//nolint:paralleltest // changes global ConfigFile variable
func TestConfigSetAll(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name          string
		plaintextArgs []string
		secretArgs    []string
		jsonArg       string
		path          bool
		expected      string
		expectError   string
	}{
		{
			name:          "plaintext values",
			plaintextArgs: []string{"testProject:key1=value1", "testProject:key2=value2"},
			expected:      "config:\n  testProject:key1: value1\n  testProject:key2: value2\n",
		},
		{
			name:          "plaintext with path",
			plaintextArgs: []string{"testProject:nested.key1=value1", "testProject:nested.key2=value2"},
			path:          true,
			expected:      "config:\n  testProject:nested:\n    key1: value1\n    key2: value2\n",
		},
		{
			name:       "secret values",
			secretArgs: []string{"testProject:secretKey1=secret1", "testProject:secretKey2=secret2"},
			expected: "config:\n  testProject:secretKey1:\n    secure: c2VjcmV0MQ==\n" +
				"  testProject:secretKey2:\n    secure: c2VjcmV0Mg==\n",
		},
		{
			name:       "secret with path",
			secretArgs: []string{"testProject:nested.secret1=secret1"},
			path:       true,
			expected:   "config:\n  testProject:nested:\n    secret1:\n      secure: c2VjcmV0MQ==\n",
		},
		{
			name:          "mixed plaintext and secret",
			plaintextArgs: []string{"testProject:plainKey=plainValue"},
			secretArgs:    []string{"testProject:secretKey=secretValue"},
			expected: "config:\n  testProject:plainKey: plainValue\n" +
				"  testProject:secretKey:\n    secure: c2VjcmV0VmFsdWU=\n",
		},
		{
			name:     "json plaintext values",
			jsonArg:  `{"testProject:key1": {"value": "value1"}, "testProject:key2": {"value": "value2"}}`,
			expected: "config:\n  testProject:key1: value1\n  testProject:key2: value2\n",
		},
		{
			name: "json secret values",
			jsonArg: `{"testProject:secretKey1": {"value": "secret1", "secret": true}, ` +
				`"testProject:secretKey2": {"value": "secret2", "secret": true}}`,
			expected: "config:\n  testProject:secretKey1:\n    secure: c2VjcmV0MQ==\n" +
				"  testProject:secretKey2:\n    secure: c2VjcmV0Mg==\n",
		},
		{
			name: "json mixed plaintext and secret",
			jsonArg: `{"testProject:plainKey": {"value": "plainValue"}, ` +
				`"testProject:secretKey": {"value": "secretValue", "secret": true}}`,
			expected: "config:\n  testProject:plainKey: plainValue\n" +
				"  testProject:secretKey:\n    secure: c2VjcmV0VmFsdWU=\n",
		},
		{
			name:          "json with plaintext flag should error",
			jsonArg:       `{"testProject:key": {"value": "val"}}`,
			plaintextArgs: []string{"testProject:otherkey=value"},
			expectError:   "the --json option cannot be used with the --plaintext, --secret or --path options",
		},
		{
			name:        "json with secret flag should error",
			jsonArg:     `{"testProject:key": {"value": "val"}}`,
			secretArgs:  []string{"testProject:secretkey=secretvalue"},
			expectError: "the --json option cannot be used with the --plaintext, --secret or --path options",
		},
		{
			name:        "json with path flag should error",
			jsonArg:     `{"testProject:key": {"value": "val"}}`,
			path:        true,
			expectError: "the --json option cannot be used with the --plaintext, --secret or --path options",
		},
		{
			name:    "json with invalid key",
			jsonArg: `{"testProject:key1:invalid": {"value": "value"}}`,
			expectError: "invalid --json object key \"testProject:key1:invalid\": " +
				"could not parse testProject:key1:invalid as a configuration key " +
				"(configuration keys should be of the form `<namespace>:<name>`)",
		},
		{
			name:        "json with nil value",
			jsonArg:     `{"testProject:key1": {"value": null}}`,
			expected:    "config:\n  testProject:key1: null\n",
			expectError: `value for --json object key "testProject:key1" is nil`,
		},
		{
			name:        "json with malformed input",
			jsonArg:     `{`, // missing closing braces
			expectError: "could not parse --json argument: unexpected end of JSON input",
		},
		{
			name:     "json with object value",
			jsonArg:  `{"testProject:key1": {"value": "{\"inner\":\"value2\"}", "objectValue": {"inner": "value2"}}}`,
			expected: "config:\n  testProject:key1:\n    inner: value2\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						NameV: tokens.MustParseStackName("testStack"),
					}
				},
				ConfigLocationF: func() backend.StackConfigLocation {
					return backend.StackConfigLocation{}
				},
			}

			tmpdir := t.TempDir()
			stack.ConfigFile = filepath.Join(tmpdir, "Pulumi.stack.yaml")
			defer func() {
				stack.ConfigFile = ""
			}()

			ws := &pkgWorkspace.MockContext{
				ReadProjectF: func() (*workspace.Project, string, error) {
					return &workspace.Project{
						Name: "testProject",
					}, "", nil
				},
			}

			// Create the command
			stackName := "testStack"
			lm := &cmdBackend.MockLoginManager{
				CurrentF: func(
					ctx context.Context,
					ws pkgWorkspace.Context,
					sink diag.Sink,
					url string,
					project *workspace.Project,
					setCurrent bool,
				) (backend.Backend, error) {
					return &backend.MockBackend{
						GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
							return &s, nil
						},
					}, nil
				},
				LoginF: func(
					ctx context.Context,
					ws pkgWorkspace.Context,
					sink diag.Sink,
					url string,
					project *workspace.Project,
					setCurrent bool,
					insecure bool,
					color colors.Colorization,
				) (backend.Backend, error) {
					return &backend.MockBackend{
						GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
							return &s, nil
						},
					}, nil
				},
			}

			// Create mock encrypter factory
			mockEncrypterFactory := &mockEncrypterFactory{
				encrypter: &secrets.MockEncrypter{
					EncryptValueF: func(plaintext string) string {
						return base64.StdEncoding.EncodeToString([]byte(plaintext))
					},
				},
			}

			cmd := newConfigSetAllCmd(ws, &stackName, lm, mockEncrypterFactory)
			cmd.SetContext(ctx)

			// Set flags based on test case
			if c.jsonArg != "" {
				err := cmd.PersistentFlags().Set("json", c.jsonArg)
				require.NoError(t, err)
			}
			for _, pt := range c.plaintextArgs {
				err := cmd.PersistentFlags().Set("plaintext", pt)
				require.NoError(t, err)
			}

			for _, sec := range c.secretArgs {
				err := cmd.PersistentFlags().Set("secret", sec)
				require.NoError(t, err)
			}
			if c.path {
				err := cmd.PersistentFlags().Set("path", "true")
				require.NoError(t, err)
			}

			// Execute the command
			err := cmd.RunE(cmd, []string{})

			// Check for expected error
			if c.expectError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expectError)
				return
			}

			require.NoError(t, err)

			// Verify the config was set correctly
			data, err := os.ReadFile(stack.ConfigFile)
			require.NoError(t, err)

			require.Equal(t, c.expected, string(data))
		})
	}
}

type mockEncrypterFactory struct {
	encrypter config.Encrypter
}

func (m *mockEncrypterFactory) GetEncrypter(
	_ context.Context,
	_ backend.Stack,
	_ *workspace.ProjectStack,
) (config.Encrypter, stack.SecretsManagerState, error) {
	return m.encrypter, stack.SecretsManagerUnchanged, nil
}
