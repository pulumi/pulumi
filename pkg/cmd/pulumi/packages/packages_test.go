// Copyright 2024-2025, Pulumi Corporation.
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

package packages

import (
	"context"
	"encoding/json"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderFromSource(t *testing.T) {
	t.Parallel()
	t.Skip("TODO: Need to figure out the correct way to mock this," +
		" since packageinstallation doesn't trust host to find plugins")

	test := func(t *testing.T, yaml string, inputSource string) plugin.Provider {
		t.Helper()
		tempDir := t.TempDir()
		if yaml != "" {
			pulumiYaml := filepath.Join(tempDir, "Pulumi.yaml")
			err := os.WriteFile(pulumiYaml, []byte(yaml), 0o600)
			require.NoError(t, err)
		}

		mockProvider := &plugin.MockProvider{
			PkgF: func() tokens.Package {
				return "test-provider"
			},
			GetSchemaF: func(ctx context.Context, req plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
				schemaSpec := schema.PackageSpec{
					Name:    "test-provider",
					Version: "1.0.0",
				}
				schemaBytes, err := json.Marshal(schemaSpec)
				if err != nil {
					return plugin.GetSchemaResponse{}, err
				}
				return plugin.GetSchemaResponse{
					Schema: schemaBytes,
				}, nil
			},
		}

		mockHost := &plugin.MockHost{
			ProviderF: func(descriptor workspace.PluginDescriptor) (plugin.Provider, error) {
				return mockProvider, nil
			},
		}

		mockRegistry := registry.Mock{
			GetPackageF: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				if name == "test-provider" {
					return apitype.PackageMetadata{
						Name:              "test-provider",
						Publisher:         publisher,
						Source:            source,
						Version:           semver.Version{Major: 1, Minor: 0, Patch: 0},
						PluginDownloadURL: "https://example.com/test-provider",
					}, nil
				}
				return apitype.PackageMetadata{}, registry.ErrNotFound
			},
			ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					if name != nil && *name == "test-provider" {
						yield(apitype.PackageMetadata{
							Name:              "test-provider",
							Publisher:         "pulumi",
							Source:            "pulumi",
							Version:           semver.Version{Major: 1, Minor: 0, Patch: 0},
							PluginDownloadURL: "https://example.com/test-provider",
						}, nil)
					}
				}
			},
		}

		pctx, err := plugin.NewContext(
			t.Context(),
			nil,
			nil,
			mockHost,
			nil,
			tempDir,
			nil,
			false,
			nil,
		)
		require.NoError(t, err)
		defer pctx.Close()

		provider, _, err := ProviderFromSource(pctx, inputSource, mockRegistry, env.NewEnv(env.MapStore{
			"PULUMI_EXPERIMENTAL": "true",
		}), 0)
		require.NoError(t, err)
		return provider
	}

	t.Run("empy Pulumi.yaml", func(t *testing.T) {
		t.Parallel()
		provider := test(t, `name: test-project
runtime: yaml
`, "test-provider")
		assert.Equal(t, tokens.Package("test-provider"), provider.Pkg())
	})

	t.Run("no Pulumi.yaml", func(t *testing.T) {
		t.Parallel()
		provider := test(t, "", "test-provider")
		assert.Equal(t, tokens.Package("test-provider"), provider.Pkg())
	})

	t.Run("with Pulumi.yaml", func(t *testing.T) {
		t.Parallel()
		provider := test(t, `name: test-project
runtime: yaml
packages:
    local-name: test-provider
`, "test-provider")
		assert.Equal(t, tokens.Package("test-provider"), provider.Pkg())
	})
}

func TestSetSpecNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pluginDownloadURL string
		wantNamespace     string
	}{
		{
			pluginDownloadURL: "https://pulumi.com/terraform/v1.0.0",
			wantNamespace:     "",
		},
		{
			pluginDownloadURL: "git://github.com/pulumi/pulumi-terraform",
			wantNamespace:     "pulumi",
		},
		{
			pluginDownloadURL: "git://",
			wantNamespace:     "",
		},
		{
			pluginDownloadURL: "git://example.com",
			wantNamespace:     "",
		},
		{
			pluginDownloadURL: "git://example.com/pulumi",
			wantNamespace:     "",
		},
		{
			pluginDownloadURL: "git://example.com/pulumi/a/long/path",
			wantNamespace:     "pulumi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.pluginDownloadURL, func(t *testing.T) {
			t.Parallel()

			pluginSpec := workspace.PluginDescriptor{
				PluginDownloadURL: tt.pluginDownloadURL,
			}
			schemaSpec := &schema.PackageSpec{}
			setSpecNamespace(schemaSpec, pluginSpec)
			assert.Equal(t, tt.wantNamespace, schemaSpec.Namespace)
		})
	}
}
