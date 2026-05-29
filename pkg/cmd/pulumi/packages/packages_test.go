// Copyright 2024, Pulumi Corporation.
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
	"path/filepath"
	"runtime"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// launchedProvider is the sentinel [plugin.Provider] returned by mockInstallContext.RunPackage. It
// records what the installation pipeline resolved the source to, so tests can assert on it without a
// real provider process.
type launchedProvider struct {
	plugin.Provider

	pluginPath   string
	originalSpec workspace.PackageSpec
}

type mockInstallContext struct {
	t *testing.T

	// baseProject is returned from LoadBaseProjectFrom. If nil, ErrBaseProjectNotFound is returned.
	baseProject workspace.BaseProject
}

const mockPluginDir = "/fake/plugins/resource-test-provider-v1.0.0"

func (mockInstallContext) HasPlugin(context.Context, workspace.PluginDescriptor) bool { return true }

func (mockInstallContext) HasPluginGTE(
	context.Context, workspace.PluginDescriptor,
) (bool, *semver.Version, error) {
	return true, &semver.Version{Major: 1}, nil
}

func (m mockInstallContext) GetLatestVersion(
	context.Context, workspace.PluginDescriptor,
) (*semver.Version, error) {
	m.t.Error("GetLatestVersion should not be called")
	return nil, assert.AnError
}

func (mockInstallContext) GetPlugins(context.Context) ([]workspace.PluginInfo, error) {
	return nil, nil
}

func (m mockInstallContext) New() (pkgWorkspace.W, error) {
	m.t.Error("New should not be called")
	return nil, assert.AnError
}

func (m mockInstallContext) ReadProject() (*workspace.Project, string, error) {
	m.t.Error("ReadProject should not be called")
	return nil, "", assert.AnError
}

func (m mockInstallContext) LoadPluginProjectAt(
	context.Context, string,
) (*workspace.PluginProject, string, error) {
	// "test-provider" is a binary plugin, so it has no PulumiPlugin.yaml.
	return nil, "", workspace.ErrPluginNotFound
}

func (m mockInstallContext) LoadBaseProjectFrom(
	context.Context, string,
) (workspace.BaseProject, string, error) {
	if m.baseProject == nil {
		return nil, "", workspace.ErrBaseProjectNotFound
	}
	return m.baseProject, "Pulumi.yaml", nil
}

func (mockInstallContext) GetStoredCredentials() (workspace.Credentials, error) {
	return workspace.Credentials{}, nil
}

func (m mockInstallContext) GetPluginPath(context.Context, workspace.PluginDescriptor) (string, error) {
	return mockPluginDir, nil
}

func (m mockInstallContext) IsExecutable(context.Context, string) (bool, error) {
	return true, nil
}

func (m mockInstallContext) InstallPluginAt(context.Context, string, *workspace.PluginProject) error {
	m.t.Error("InstallPluginAt should not be called for an already-installed binary plugin")
	return assert.AnError
}

func (m mockInstallContext) GetRequiredPackages(
	context.Context, string, *workspace.PluginProject,
) ([]workspace.PackageDescriptor, error) {
	m.t.Error("GetRequiredPackages should not be called for a binary plugin")
	return nil, assert.AnError
}

func (m mockInstallContext) DownloadPlugin(
	context.Context, workspace.PluginDescriptor,
) (string, packageinstallation.MarkInstallationDone, error) {
	m.t.Error("DownloadPlugin should not be called for an already-installed plugin")
	return "", nil, assert.AnError
}

func (m mockInstallContext) GenerateLocalSDK(
	context.Context, *workspace.ProjectRuntimeInfo, string, plugin.Provider,
) (workspace.LinkablePackageDescriptor, error) {
	m.t.Error("GenerateLocalSDK should not be called when installing a single plugin")
	return workspace.LinkablePackageDescriptor{}, assert.AnError
}

func (m mockInstallContext) LinkIntoProject(
	context.Context, *workspace.ProjectRuntimeInfo, string, []workspace.LinkablePackageDescriptor,
) error {
	m.t.Error("LinkIntoProject should not be called when installing a single plugin")
	return assert.AnError
}

func (m mockInstallContext) RunPackage(
	_ context.Context, _, pluginPath string,
	_ plugin.ParameterizeParameters, originalSpec workspace.PackageSpec,
) (plugin.Provider, error) {
	return launchedProvider{pluginPath: pluginPath, originalSpec: originalSpec}, nil
}

var _ packageinstallation.Context = mockInstallContext{}

func TestProviderFromSource(t *testing.T) {
	t.Parallel()

	resolvedSpec := workspace.PackageSpec{
		Source:  "test-provider",
		Version: "1.0.0",
	}

	binaryName := "pulumi-resource-test-provider"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	wantProvider := launchedProvider{
		pluginPath:   filepath.Join(mockPluginDir, binaryName),
		originalSpec: resolvedSpec,
	}

	run := func(
		t *testing.T, installCtx mockInstallContext, inputSource string,
	) (plugin.Provider, workspace.PackageSpec) {
		t.Helper()
		installCtx.t = t

		pctx, err := plugin.NewContext(
			t.Context(), nil, nil, nil, nil, t.TempDir(), nil, false, nil, schema.NewLoaderServerFromHost)
		require.NoError(t, err)
		defer func() { require.NoError(t, pctx.Close()) }()

		provider, spec, err := providerFromSource(
			pctx, inputSource, nil,
			env.NewEnv(env.MapStore{"PULUMI_EXPERIMENTAL": "true"}), 0, installCtx)
		require.NoError(t, err)
		return provider, spec
	}

	t.Run("no Pulumi.yaml", func(t *testing.T) {
		t.Parallel()

		provider, spec := run(t, mockInstallContext{}, "test-provider@1.0.0")

		assert.Equal(t, resolvedSpec, spec)
		assert.Equal(t, wantProvider, provider)
	})

	t.Run("with Pulumi.yaml remap", func(t *testing.T) {
		t.Parallel()

		// The project remaps the source "local-name" to the real "test-provider@1.0.0" package.
		installCtx := mockInstallContext{
			baseProject: &workspace.Project{
				Name:    "test-project",
				Runtime: workspace.NewProjectRuntimeInfo("yaml", nil),
				Packages: map[string]workspace.PackageSpec{
					"local-name": {Source: "test-provider", Version: "1.0.0"},
				},
			},
		}

		provider, spec := run(t, installCtx, "local-name")

		assert.Equal(t, resolvedSpec, spec)
		assert.Equal(t, wantProvider, provider)
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
