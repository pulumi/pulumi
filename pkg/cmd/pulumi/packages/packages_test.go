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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
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

func (mockInstallContext) HasPlugin(
	context.Context, workspace.PluginDescriptor,
) pluginstorage.InstallState {
	return pluginstorage.PluginInstalled
}

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

func (m mockInstallContext) New(string) (pkgWorkspace.W, error) {
	m.t.Error("New should not be called")
	return nil, assert.AnError
}

func (m mockInstallContext) ReadProject(string) (*workspace.Project, string, error) {
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
) ([]workspace.PackageDescriptor, []workspace.PackageSpec, error) {
	m.t.Error("GetRequiredPackages should not be called for a binary plugin")
	return nil, nil, assert.AnError
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
		t *testing.T, installCtx mockInstallContext, inputSource, pluginDownloadURL string,
	) (plugin.Provider, workspace.PackageSpec) {
		t.Helper()
		installCtx.t = t

		pluginHost, err := pkghost.New(context.WithoutCancel(t.Context()), nil, nil, nil, nil,
			schema.NewLoaderServerFromContext, nil, nil)
		require.NoError(t, err)
		defer func() { require.NoError(t, pluginHost.Close()) }()
		pctx, err := plugin.NewContext(
			t.Context(), nil, nil, pluginHost, nil, t.TempDir(), nil, false, nil,
			nil,
		)
		require.NoError(t, err)
		defer func() { require.NoError(t, pctx.Close()) }()

		provider, spec, err := providerFromSource(
			pctx, inputSource, nil,
			env.NewEnv(env.MapStore{"PULUMI_EXPERIMENTAL": "true"}), 0, pluginDownloadURL, installCtx,
		)
		require.NoError(t, err)
		return provider, spec
	}

	t.Run("no Pulumi.yaml", func(t *testing.T) {
		t.Parallel()

		provider, spec := run(t, mockInstallContext{}, "test-provider@1.0.0", "")

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

		provider, spec := run(t, installCtx, "local-name", "")

		assert.Equal(t, resolvedSpec, spec)
		assert.Equal(t, wantProvider, provider)
	})

	t.Run("with pluginDownloadURL adds to remap", func(t *testing.T) {
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

		// And we pass pluginDownloadURL, which should be added to the PackageSpec from the project.
		provider, spec := run(t, installCtx, "local-name", "https://example.com/plugins")

		expected := resolvedSpec
		expected.PluginDownloadURL = "https://example.com/plugins"

		assert.Equal(t, expected, spec)

		want := wantProvider
		want.originalSpec.PluginDownloadURL = "https://example.com/plugins"

		assert.Equal(t, want, provider)
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

type linkPackagesTestRuntime struct {
	plugin.LanguageRuntime

	calls      []string
	installErr error
	linkErr    error
}

func (r *linkPackagesTestRuntime) Link(
	context.Context, plugin.ProgramInfo, []workspace.LinkablePackageDescriptor, string,
) (string, error) {
	r.calls = append(r.calls, "link")
	if len(r.calls) == 3 && r.linkErr != nil {
		return "", r.linkErr
	}
	if len(r.calls) == 1 {
		return "import instructions", nil
	}
	return "duplicate import instructions", nil
}

func (r *linkPackagesTestRuntime) InstallDependencies(
	context.Context, plugin.InstallDependenciesRequest,
) (io.Reader, io.Reader, <-chan error, error) {
	r.calls = append(r.calls, "install")
	done := make(chan error, 1)
	done <- r.installErr
	close(done)
	return strings.NewReader(""), strings.NewReader(""), done, nil
}

func (r *linkPackagesTestRuntime) Close() error {
	return nil
}

func TestLinkPackagesLifecycle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runtime     string
		install     bool
		installErr  error
		linkErr     error
		wantCalls   []string
		wantErr     string
		wantPrinted bool
	}{
		{
			name:        "Go relinks after successful installation",
			runtime:     "go",
			install:     true,
			wantCalls:   []string{"link", "install", "link"},
			wantPrinted: true,
		},
		{
			name:        "non-Go runtime does not relink",
			runtime:     "nodejs",
			install:     true,
			wantCalls:   []string{"link", "install"},
			wantPrinted: true,
		},
		{
			name:       "Go does not relink after failed installation",
			runtime:    "go",
			install:    true,
			installErr: errors.New("install failed"),
			wantCalls:  []string{"link", "install"},
			wantErr:    "installing dependencies",
		},
		{
			name:      "Go relink errors are returned",
			runtime:   "go",
			install:   true,
			linkErr:   errors.New("relink failed"),
			wantCalls: []string{"link", "install", "link"},
			wantErr:   "linking package",
		},
		{
			name:        "Go does not relink when installation is disabled",
			runtime:     "go",
			wantCalls:   []string{"link"},
			wantPrinted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			pkg, diags, err := schema.BindSpec(schema.PackageSpec{
				Name:      "file",
				Namespace: "example",
				Version:   "0.1.0",
				Meta:      &schema.MetadataSpec{SupportPack: true},
			}, schema.NewNullLoader(), schema.ValidationOptions{})
			require.NoError(t, err)
			require.False(t, diags.HasErrors(), "%v", diags)

			languageRuntime := &linkPackagesTestRuntime{
				installErr: tt.installErr,
				linkErr:    tt.linkErr,
			}
			host := &plugin.MockHost{
				LanguageRuntimeF: func(_ *plugin.Context, name string) (plugin.LanguageRuntime, error) {
					require.Equal(t, tt.runtime, name)
					return languageRuntime, nil
				},
			}
			pctx, err := plugin.NewContextWithHost(t.Context(), nil, nil, host, root, root, nil, nil)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, pctx.Close()) })

			project := &workspace.Project{
				Runtime: workspace.NewProjectRuntimeInfo(tt.runtime, nil),
			}
			var output bytes.Buffer
			err = LinkPackages(&LinkPackagesContext{
				Project:       project,
				Language:      tt.runtime,
				Root:          root,
				Packages:      []PackageToLink{{Pkg: pkg, Out: filepath.Join(root, "sdks", "example-file")}},
				PluginContext: pctx,
				Install:       tt.install,
				Writer:        &output,
			})

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantCalls, languageRuntime.calls)
			if tt.wantPrinted {
				require.Equal(t, 1, strings.Count(output.String(), "import instructions"))
				require.NotContains(t, output.String(), "duplicate import instructions")
			} else {
				require.Empty(t, output.String())
			}
		})
	}
}

// TestBindSpecReportsAllDiagnostics checks that an invalid schema surfaces every validation
// diagnostic, rather than the first one plus "and N other diagnostic(s)".
// Regression test for https://github.com/pulumi/pulumi/issues/21609.
func TestBindSpecReportsAllDiagnostics(t *testing.T) {
	t.Parallel()

	const invalid = `{
	  "name": "bad",
	  "version": "1.0.0",
	  "resources": {
	    "bad:index:One": { "type": "object", "properties": { "a": { "type": "bogus" } } },
	    "bad:index:Two": { "type": "object", "properties": { "b": { "type": "alsoBogus" } } }
	  }
	}`

	var spec schema.PackageSpec
	require.NoError(t, json.Unmarshal([]byte(invalid), &spec))

	_, err := BindSpec(spec, schema.NewNullLoader())
	require.Error(t, err) //nolint:forbidigo // pre-existing
	assert.NotContains(t, err.Error(), "other diagnostic(s)")
	assert.Contains(t, err.Error(), "#/resources/bad:index:One/properties/a/type: unknown type kind bogus")
	assert.Contains(t, err.Error(), "#/resources/bad:index:Two/properties/b/type: unknown type kind alsoBogus")
}
