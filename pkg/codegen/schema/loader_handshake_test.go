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

package schema

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestLoaderServerFromContext_RealProvider verifies the full loader handshake loop against a real provider binary,
// and that the loader target stays usable for ordinary resource operations rather than just at handshake time:
//
//  1. The host boots the test provider and sends it a loader target as part of the handshake.
//  2. A Create on the provider dials that target and asks the engine's loader for the "loadtest" schema.
//  3. The engine's loader boots a second instance of the same provider and retrieves the schema it serves via
//     GetSchema, which the first instance returns as the created resource's state.
//
// Uses t.Setenv, so it cannot be parallel.
func TestLoaderServerFromContext_RealProvider(t *testing.T) {
	// Build the test provider into the plugin layout of a fake PULUMI_HOME, where both the host's plugin loader and
	// the schema loader's plugin resolution will find it.
	home := t.TempDir()
	binName := "pulumi-resource-loadtest"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	pluginDir := filepath.Join(home, "plugins", "resource-loadtest-v1.0.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))

	build := exec.Command("go", "build", "-o", filepath.Join(pluginDir, binName), "./testdata/loader-provider")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building test provider: %s", out)

	t.Setenv("PULUMI_HOME", home)

	sink := diagtest.LogSink(t)
	pluginHost, err := pkghost.New(context.WithoutCancel(t.Context()), sink, sink, nil, nil,
		NewLoaderServerFromContext, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, pluginHost.Close()) }()
	pctx, err := plugin.NewContext(t.Context(), sink, sink, pluginHost, nil, t.TempDir(), nil, false, nil)
	require.NoError(t, err)
	defer pctx.Close()

	p, err := pctx.Host.Provider(pctx, workspace.PluginDescriptor{
		Name: "loadtest",
		Kind: apitype.ResourcePlugin,
	}, env.Global())
	require.NoError(t, err)

	typ := tokens.Type("pulumi:providers:loadtest")
	_, err = p.Configure(t.Context(), plugin.ConfigureRequest{Type: &typ})
	require.NoError(t, err)

	res, err := p.Create(t.Context(), plugin.CreateRequest{
		URN:        resource.NewURN("test", "test", "", "loadtest:index:Res", "res"),
		Properties: resource.PropertyMap{},
	})
	require.NoError(t, err)
	assert.Equal(t, resource.PropertyMap{
		"schema": resource.NewProperty(`{"name":"loadtest","version":"1.0.0"}`),
	}, res.Properties)
}
