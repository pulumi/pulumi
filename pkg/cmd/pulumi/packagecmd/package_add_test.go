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
	"bytes"
	"context"
	"errors"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEnclosingTarget(t *testing.T) {
	t.Parallel()

	ptr := func(s string) *string { return &s }

	tests := []struct {
		name  string
		files map[string]string
		wd    string
		// Note: reg is ignored during comparison, so should be left nil
		expected      addTarget
		expectedError error
	}{
		{
			name: "top level plugin",
			files: map[string]string{
				"PulumiPlugin.yaml": "runtime: nodejs\n",
			},
			wd: ".",
			expected: addTarget{
				installRoot:     ".",
				projectFilePath: ptr("PulumiPlugin.yaml"),
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
			expected: addTarget{
				installRoot:     ".",
				projectFilePath: ptr("PulumiPlugin.yaml"),
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
			expected: addTarget{
				installRoot:     ".",
				projectFilePath: ptr("Pulumi.yaml"),
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
			expected: addTarget{
				installRoot:     ".",
				projectFilePath: ptr("Pulumi.yaml"),
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
			expected: addTarget{
				installRoot:     "plugin",
				projectFilePath: ptr("plugin/PulumiPlugin.yaml"),
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
			expected: addTarget{
				installRoot:     "project",
				projectFilePath: ptr("project/Pulumi.yaml"),
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
			expectedError: workspace.ErrBaseProjectNotFound,
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

			actual, err := loadEnclosingTarget(t.Context(), filepath.Join(root, tt.wd))
			if tt.expectedError != nil {
				require.ErrorContains(t, err, tt.expectedError.Error())
				return
			}
			require.NoError(t, err)
			require.NotNil(t, actual.reg)
			actual.reg = nil
			tt.expected.installRoot = filepath.Join(root, tt.expected.installRoot)
			require.NotNil(t, tt.expected.projectFilePath)
			tt.expected.projectFilePath = ptr(filepath.Join(root, *tt.expected.projectFilePath))
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

func TestPrintRegistryDocsHint(t *testing.T) {
	t.Parallel()

	ver := semver.MustParse("4.19.1")
	pkgFor := func(name string, version *semver.Version) *schema.Package {
		return &schema.Package{Name: name, Version: version}
	}

	matchingMeta := apitype.PackageMetadata{
		Source: "pulumi", Publisher: "pulumi", Name: "random", Version: ver,
	}
	resolverFromMatching := func(meta apitype.PackageMetadata) registry.Mock {
		return registry.Mock{
			ListPackagesF: func(_ context.Context, _ *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					yield(meta, nil)
				}
			},
			GetPackageF: func(
				_ context.Context, source, publisher, name string, _ *semver.Version,
			) (apitype.PackageMetadata, error) {
				if source == meta.Source && publisher == meta.Publisher && name == meta.Name {
					return meta, nil
				}
				return apitype.PackageMetadata{}, registry.ErrNotFound
			},
		}
	}
	notFoundResolver := registry.Mock{
		ListPackagesF: func(_ context.Context, _ *string) iter.Seq2[apitype.PackageMetadata, error] {
			return func(yield func(apitype.PackageMetadata, error) bool) {}
		},
		GetPackageF: func(
			_ context.Context, _, _, _ string, _ *semver.Version,
		) (apitype.PackageMetadata, error) {
			return apitype.PackageMetadata{}, registry.ErrNotFound
		},
	}

	expectedBase := "/api/registry/packages/pulumi/pulumi/random/versions/4.19.1"
	cmdLine := func(suffix, comment string) string {
		return "  pulumi api --output=markdown '" + expectedBase + suffix + "'" + comment + "\n"
	}
	expectedOutput := "Documentation:\n" +
		cmdLine("/readme", "                    # package readme") +
		cmdLine("/nav", "                       # doc tree (modules)") +
		cmdLine("/nav?q=<term>&depth=full", "   # search for resources/functions") +
		cmdLine("/docs/<type-token>", "         # one resource or function (type token from /nav)")

	tests := []struct {
		name  string
		agent string
		reg   registry.Registry
		pkg   *schema.Package
		want  string
	}{
		{
			name:  "agent on, happy path",
			agent: "claude",
			reg:   resolverFromMatching(matchingMeta),
			pkg:   pkgFor("random", &ver),
			want:  expectedOutput,
		},
		{
			name:  "agent off skips output",
			agent: "",
			reg:   resolverFromMatching(matchingMeta),
			pkg:   pkgFor("random", &ver),
			want:  "",
		},
		{
			name:  "nil package skips output",
			agent: "claude",
			reg:   resolverFromMatching(matchingMeta),
			pkg:   nil,
			want:  "",
		},
		{
			name:  "missing version skips output",
			agent: "claude",
			reg:   resolverFromMatching(matchingMeta),
			pkg:   pkgFor("random", nil),
			want:  "",
		},
		{
			name:  "nil registry skips output",
			agent: "claude",
			reg:   nil,
			pkg:   pkgFor("random", &ver),
			want:  "",
		},
		{
			name:  "resolver not found skips output",
			agent: "claude",
			reg:   notFoundResolver,
			pkg:   pkgFor("random", &ver),
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			printRegistryDocsHint(&buf, tt.agent, t.Context(), tt.reg, tt.pkg)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}
