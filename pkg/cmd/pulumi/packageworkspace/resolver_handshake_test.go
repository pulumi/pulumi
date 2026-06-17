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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/diy/unauthenticatedregistry"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
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
//  1. The host boots the test provider and sends it a resolver target as part of the handshake.
//  2. A Create on the provider dials that target and asks the engine's resolver to resolve a package
//     spec naming a local plugin path.
//  3. The resolver runs the spec through the real package installation machinery and returns the
//     concrete package dependency, which the provider returns as the created resource's state.
//
// Uses t.Setenv, so it cannot be parallel.
func TestResolverServerFromContext_RealProvider(t *testing.T) {
	// Build the test provider into the plugin layout of a fake PULUMI_HOME, where the host's plugin loader will
	// find it.
	home := t.TempDir()
	binName := "pulumi-resource-resolvetest"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	pluginDir := filepath.Join(home, "plugins", "resource-resolvetest-v1.0.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))

	build := exec.Command("go", "build", "-o", filepath.Join(pluginDir, binName), "./testdata/resolver-provider")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building test provider: %s", out)

	t.Setenv("PULUMI_HOME", home)
	// Keep resolution local: never attempt to download a plugin we cannot find.
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true")

	sink := diagtest.LogSink(t)
	// The resolver resolves a local plugin path, so the registry is never consulted; an
	// unauthenticated registry is sufficient for the test.
	reg := unauthenticatedregistry.New(sink, env.Global())
	pluginHost, err := pkghost.New(context.WithoutCancel(t.Context()), sink, sink, nil, nil,
		nil, nil, packageworkspace.NewResolverServer(reg))
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

	binaryPath := filepath.Join(pluginDir, binName)
	res, err := p.Create(t.Context(), plugin.CreateRequest{
		URN: resource.NewURN("test", "test", "", "resolvetest:index:Res", "res"),
		Properties: resource.PropertyMap{
			"source": resource.NewProperty(binaryPath),
		},
	})
	require.NoError(t, err)
	// The resolver ran the local plugin path through the real package-installation machinery and
	// handed back the concrete dependency it names: a resource plugin identified by that path.
	assert.Equal(t, resource.PropertyMap{
		"name":    resource.NewProperty(binaryPath),
		"kind":    resource.NewProperty("resource"),
		"version": resource.NewProperty(""),
		"server":  resource.NewProperty(""),
	}, res.Properties)
}
