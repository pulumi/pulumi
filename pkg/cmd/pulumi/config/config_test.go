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
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
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
		c := c
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
				GetStackFilenameF: func(context.Context) (string, bool) {
					return "Pulumi.stack.yaml", true
				},
				LoadF: func(ctx context.Context, project *workspace.Project, configFileOverride string,
				) (*workspace.ProjectStack, error) {
					return workspace.LoadProjectStack(project, "Pulumi.stack.yaml")
				},
				SaveF: func(ctx context.Context, project *workspace.ProjectStack, configFileOverride string) error {
					return project.Save(stack.ConfigFile)
				},
			}

			configSetCmd := &configSetCmd{
				Path: c.path,
			}

			tmpdir := t.TempDir()
			stack.ConfigFile = filepath.Join(tmpdir, "Pulumi.stack.yaml")
			defer func() {
				stack.ConfigFile = ""
			}()

			err := configSetCmd.Run(ctx, c.args, &project, &s)
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
		c := c
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
				LoadF: func(ctx context.Context, project *workspace.Project, configFileOverride string,
				) (*workspace.ProjectStack, error) {
					return workspace.LoadProjectStackBytes(project, []byte{}, "Pulumi.stack.yaml", encoding.YAML)
				},
				SaveF: func(ctx context.Context, project *workspace.ProjectStack, configFileOverride string) error {
					return project.Save(stack.ConfigFile)
				},
				GetStackFilenameF: func(ctx context.Context) (string, bool) {
					return "Pulumi.stack.yaml", true
				},
			}

			configSetCmd := &configSetCmd{
				Path: c.path,
				Type: c.typ,
			}

			tmpdir := t.TempDir()
			stack.ConfigFile = filepath.Join(tmpdir, "Pulumi.stack.yaml")
			defer func() {
				stack.ConfigFile = ""
			}()

			err := configSetCmd.Run(ctx, c.args, &project, &s)
			require.NoError(t, err)

			// verify the config was set
			data, err := os.ReadFile(stack.ConfigFile)
			require.NoError(t, err)

			require.Equal(t, c.expected, string(data))
		})
	}
}
