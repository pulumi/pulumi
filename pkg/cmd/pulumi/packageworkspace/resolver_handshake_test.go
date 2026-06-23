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

package packageworkspace_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/diy/unauthenticatedregistry"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestResolverServerFromContext_RealProvider verifies the full package-resolver handshake loop against a
// real provider binary:
//
//  1. The host boots the test provider and sends it resolver and loader targets as part of the handshake.
//  2. A Create on the provider dials the resolver target and asks the engine's resolver to resolve a
//     package spec naming a local plugin path.
//  3. The resolver runs the spec through the real package installation machinery and returns the
//     concrete package dependency, which the provider then loads through the loader target and returns
//     as the created resource's state.
//
// Uses t.Setenv, so it cannot be parallel.
func TestResolverServerFromContext_RealProvider(t *testing.T) {
	// Build the test provider into the plugin layout of a fake PULUMI_HOME, where the host's plugin loader will
	// find it.
	home := t.TempDir()
	pluginDir := filepath.Join(home, "plugins", "resource-resolvetest-v1.0.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	buildTestProvider(t, "./testdata/resolver-provider", filepath.Join(pluginDir, providerBinName("resolvetest")))

	t.Setenv("PULUMI_HOME", home)
	// Keep resolution local: never attempt to download a plugin we cannot find.
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true")

	sink := diagtest.LogSink(t)
	// The resolver resolves a local plugin path, so the registry is never consulted; an
	// unauthenticated registry is sufficient for the test.
	reg := unauthenticatedregistry.New(sink, env.Global())
	pluginHost, err := pkghost.New(context.WithoutCancel(t.Context()), sink, sink, nil, nil,
		schema.NewLoaderServerFromContext, nil, packageworkspace.NewResolverServer(reg))
	require.NoError(t, err)
	defer func() { require.NoError(t, pluginHost.Close()) }()
	pctx, err := plugin.NewContext(t.Context(), sink, sink, pluginHost, nil, t.TempDir(), nil, false, nil)
	require.NoError(t, err)
	defer pctx.Close()

	p, err := pctx.Host.Provider(pctx, workspace.PluginDescriptor{
		Name: "resolvetest",
		Kind: apitype.ResourcePlugin,
	}, env.Global())
	require.NoError(t, err)

	typ := tokens.Type("pulumi:providers:resolvetest")
	_, err = p.Configure(t.Context(), plugin.ConfigureRequest{Type: &typ})
	require.NoError(t, err)

	binaryPath := filepath.Join(pluginDir, providerBinName("resolvetest"))
	res, err := p.Create(t.Context(), plugin.CreateRequest{
		URN: resource.NewURN("test", "test", "", "resolvetest:index:Res", "res"),
		Properties: resource.PropertyMap{
			"source": resource.NewProperty(binaryPath),
		},
	})
	require.NoError(t, err)
	// The resolver ran the local plugin path through the real package-installation machinery and read its
	// schema, so the dependency names the plugin's own package. The provider then loaded that dependency
	// back through the loader, which returns the same plugin's schema.
	assert.Equal(t, resource.PropertyMap{
		"name":    resource.NewProperty("resolvetest"),
		"kind":    resource.NewProperty("resource"),
		"version": resource.NewProperty("1.0.0"),
		"server":  resource.NewProperty(""),
		"schema":  resource.NewProperty(`{"name":"resolvetest","version":"1.0.0"}`),
	}, res.Properties)
}

// TestResolverServerFromContext_ParameterizedProvider verifies that the resolver surfaces a
// parameterization when the resolved package is produced by parameterizing a plugin, and that the
// resolved dependency can be loaded straight through the loader handshake target. The driving provider
// resolves a plugin named together with parameters; the resolver runs that plugin, applies the
// parameters, and reads the resulting schema, so the returned dependency describes both the base
// plugin and the package the parameterization produces. The provider then feeds that dependency into
// the loader, which runs and parameterizes the plugin again to return the package's schema.
//
// Uses t.Setenv, so it cannot be parallel.
func TestResolverServerFromContext_ParameterizedProvider(t *testing.T) {
	home := t.TempDir()
	resolveDir := filepath.Join(home, "plugins", "resource-resolvetest-v1.0.0")
	require.NoError(t, os.MkdirAll(resolveDir, 0o755))
	buildTestProvider(t, "./testdata/resolver-provider", filepath.Join(resolveDir, providerBinName("resolvetest")))

	// Install the parameterizable provider into the plugin layout so it resolves by name, the way a
	// consumer naming a bridged provider would. The resolver then runs it, applies the parameters, and
	// reads the resulting schema.
	paramDir := filepath.Join(home, "plugins", "resource-paramtest-v1.0.0")
	require.NoError(t, os.MkdirAll(paramDir, 0o755))
	buildTestProvider(t, "./testdata/param-provider", filepath.Join(paramDir, providerBinName("paramtest")))

	t.Setenv("PULUMI_HOME", home)
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true")

	sink := diagtest.LogSink(t)
	reg := unauthenticatedregistry.New(sink, env.Global())
	// The host carries both a resolver and a loader, so the driving provider can resolve a spec and then
	// load the resolved package's schema through the loader handshake target.
	pluginHost, err := pkghost.New(context.WithoutCancel(t.Context()), sink, sink, nil, nil,
		schema.NewLoaderServerFromContext, nil, packageworkspace.NewResolverServer(reg))
	require.NoError(t, err)
	defer func() { require.NoError(t, pluginHost.Close()) }()
	pctx, err := plugin.NewContext(t.Context(), sink, sink, pluginHost, nil, t.TempDir(), nil, false, nil)
	require.NoError(t, err)
	defer pctx.Close()

	p, err := pctx.Host.Provider(pctx, workspace.PluginDescriptor{
		Name: "resolvetest",
		Kind: apitype.ResourcePlugin,
	}, env.Global())
	require.NoError(t, err)

	typ := tokens.Type("pulumi:providers:resolvetest")
	_, err = p.Configure(t.Context(), plugin.ConfigureRequest{Type: &typ})
	require.NoError(t, err)

	res, err := p.Create(t.Context(), plugin.CreateRequest{
		URN: resource.NewURN("test", "test", "", "resolvetest:index:Res", "res"),
		Properties: resource.PropertyMap{
			"source":     resource.NewProperty("paramtest"),
			"parameters": resource.NewProperty([]resource.PropertyValue{resource.NewProperty("hashicorp/random")}),
		},
	})
	require.NoError(t, err)

	// The loaded schema is fetched by feeding the resolved dependency back into the loader, which runs
	// and parameterizes paramtest. Assert it separately, then check the resolve-level coordinates.
	loaded := res.Properties["schema"]
	delete(res.Properties, "schema")
	var loadedSpec schema.PackageSpec
	require.NoError(t, json.Unmarshal([]byte(loaded.StringValue()), &loadedSpec))
	assert.Equal(t, schema.PackageSpec{
		Name:    "random",
		Version: "3.0.0",
		Parameterization: &schema.ParameterizationSpec{
			BaseProvider: schema.BaseProviderSpec{Name: "paramtest", Version: "1.0.0"},
			Parameter:    []byte("random-param-value"),
		},
	}, loadedSpec)

	// The base dependency names the providing plugin, and the parameterization names the package that
	// plugin produces (its name, version, and parameter value, read from the schema).
	assert.Equal(t, resource.PropertyMap{
		"name":          resource.NewProperty("paramtest"),
		"kind":          resource.NewProperty("resource"),
		"version":       resource.NewProperty("1.0.0"),
		"server":        resource.NewProperty(""),
		"param_name":    resource.NewProperty("random"),
		"param_version": resource.NewProperty("3.0.0"),
		"param_value":   resource.NewProperty("random-param-value"),
	}, res.Properties)
}

// providerBinName returns the resource-plugin binary name for a provider, accounting for the Windows
// executable suffix.
func providerBinName(name string) string {
	bin := "pulumi-resource-" + name
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	return bin
}

// buildTestProvider compiles a test provider's main package to outPath.
func buildTestProvider(t *testing.T, src, outPath string) {
	t.Helper()
	build := exec.Command("go", "build", "-o", outPath, src)
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building %s: %s", src, out)
}
