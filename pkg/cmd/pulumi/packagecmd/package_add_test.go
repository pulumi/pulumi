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

package packagecmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectEnclosingPluginOrProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files map[string]string
		wd    string
		// Note: reg is ignored during comparison, so should be left nil
		expected      pluginOrProject
		expectedError error
	}{
		{
			name: "top level plugin",
			files: map[string]string{
				"PulumiPlugin.yaml": "runtime: nodejs\n",
			},
			wd: ".",
			expected: pluginOrProject{
				installRoot:     ".",
				projectFilePath: "PulumiPlugin.yaml",
				proj: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
				},
			},
		},
		{
			name: "nested in a plugin",
			files: map[string]string{
				"PulumiPlugin.yaml": "runtime: nodejs\n",
			},
			wd: "subdir/nested",
			expected: pluginOrProject{
				installRoot:     ".",
				projectFilePath: "PulumiPlugin.yaml",
				proj: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
				},
			},
		},
		{
			name: "top level project",
			files: map[string]string{
				"Pulumi.yaml": "name: test-project\nruntime: nodejs\n",
			},
			wd: ".",
			expected: pluginOrProject{
				installRoot:     ".",
				projectFilePath: "Pulumi.yaml",
				proj: &workspace.Project{
					Name:    "test-project",
					Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
				},
			},
		},
		{
			name: "nested in a project",
			files: map[string]string{
				"Pulumi.yaml": "name: test-project\nruntime: nodejs\n",
			},
			wd: "nested/deep/subdir",
			expected: pluginOrProject{
				installRoot:     ".",
				projectFilePath: "Pulumi.yaml",
				proj: &workspace.Project{
					Name:    "test-project",
					Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
				},
			},
		},
		{
			name: "plugin nested in a project",
			files: map[string]string{
				"Pulumi.yaml":              "name: test-project\nruntime: nodejs\n",
				"plugin/PulumiPlugin.yaml": "runtime: nodejs\n",
			},
			wd: "plugin/subdir",
			expected: pluginOrProject{
				installRoot:     "plugin",
				projectFilePath: "plugin/PulumiPlugin.yaml",
				proj: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
				},
			},
		},
		{
			name: "project nested in a plugin",
			files: map[string]string{
				"PulumiPlugin.yaml":   "runtime: nodejs\n",
				"project/Pulumi.yaml": "name: test-project\nruntime: nodejs\n",
			},
			wd: "project/subdir",
			expected: pluginOrProject{
				installRoot:     "project",
				projectFilePath: "project/Pulumi.yaml",
				proj: &workspace.Project{
					Name:    "test-project",
					Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
				},
			},
		},
		{
			name: "in neither a plugin nor a project",
			files: map[string]string{
				"some-file.txt": "not a project or plugin\n",
			},
			wd:            ".",
			expectedError: errors.New("unable to find an enclosing plugin or project"),
		},
		{
			name: "both plugin and project at same level",
			files: map[string]string{
				"PulumiPlugin.yaml": "runtime: nodejs\n",
				"Pulumi.yaml":       "name: test-project\nruntime: nodejs\n",
			},
			wd:            ".",
			expectedError: errors.New("detected both PulumiPlugin.yaml and Pulumi.yaml in"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			for path, content := range tt.files {
				path = filepath.Join(root, path)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
			}

			actual, err := detectEnclosingPluginOrProject(t.Context(), filepath.Join(root, tt.wd))
			if tt.expectedError != nil {
				require.ErrorContains(t, err, tt.expectedError.Error())
				return
			}
			require.NoError(t, err)
			require.NotNil(t, actual.reg)
			actual.reg = nil
			tt.expected.installRoot = filepath.Join(root, tt.expected.installRoot)
			tt.expected.projectFilePath = filepath.Join(root, tt.expected.projectFilePath)
			if p, ok := actual.proj.(*workspace.Project); ok {
				// Create a copy of actual.proj up to private keys. This
				// way, we get a clean comparison.
				actual.proj = &workspace.Project{
					Name:           p.Name,
					Runtime:        p.Runtime,
					Main:           p.Main,
					Description:    p.Description,
					Author:         p.Author,
					Website:        p.Website,
					License:        p.License,
					Config:         p.Config,
					StackConfigDir: p.StackConfigDir,
					Template:       p.Template,
					Backend:        p.Backend,
					Options:        p.Options,
					Packages:       p.Packages,
					Plugins:        p.Plugins,
					AdditionalKeys: p.AdditionalKeys,
				}
			}
			assert.Equal(t, tt.expected, actual)
		})
	}
}
