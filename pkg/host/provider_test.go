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

package host

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestStartupFailure(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	pluginPath, err := filepath.Abs("./testdata/provider-language")
	require.NoError(t, err)

	path := os.Getenv("PATH")
	t.Setenv("PATH", pluginPath+string(os.PathListSeparator)+path)

	// Check exec.LookPath finds the plugin
	file, err := exec.LookPath("pulumi-language-test")
	require.NoError(t, err)
	require.Contains(t, file, "pulumi-language-test")

	pluginPathRel := filepath.Join("testdata", "test-plugin")
	_, err = plugin.NewProviderFromPath(ctx.Host, ctx, pluginPathRel)
	require.ErrorContains(t, err, "could not read plugin ["+pluginPathRel+"]: not implemented")
}

func TestNonZeroExitcode(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	pluginPath, err := filepath.Abs("./testdata/provider-language")
	require.NoError(t, err)

	path := os.Getenv("PATH")
	t.Setenv("PATH", pluginPath+string(os.PathListSeparator)+path)

	// Check exec.LookPath finds the plugin
	file, err := exec.LookPath("pulumi-language-test")
	require.NoError(t, err)
	require.Contains(t, file, "pulumi-language-test")

	t.Setenv("PULUMI_TEST_PLUGIN_EXITCODE", "1")
	pluginPathRel := filepath.Join("testdata", "test-plugin-exit")
	_, err = plugin.NewProviderFromPath(ctx.Host, ctx, pluginPathRel)
	require.ErrorContains(t, err, "could not read plugin ["+pluginPathRel+"]: exit status 1")

	// Build a tiny go program that will exit with a non-zero code and run that, check it gives the same result.
	tmp := t.TempDir()
	err = os.WriteFile(filepath.Join(tmp, "main.go"), []byte(`
	package main
	import "os"

	func main() {
		os.Exit(1)
	}
	`), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(`
	module test-plugin-exit
	go 1.24
	`), 0o600)
	require.NoError(t, err)

	// Build and run the program. On Windows an executable must carry the .exe extension or it
	// cannot be launched, so name the output accordingly.
	bin := "test-plugin-exit"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = tmp
	stdout, err := cmd.CombinedOutput()
	t.Log(string(stdout))
	require.NoError(t, err)

	_, err = plugin.NewProviderFromPath(ctx.Host, ctx, filepath.Join(tmp, bin))
	// the prefix of the error message is unstable because it's in a temp dir but we can check the start and end
	// separately.
	require.ErrorContains(t, err, "could not read plugin [")
	require.ErrorContains(t, err, bin+"]: exit status 1")
}

// Similar to TestNonZeroExitcode but with a zero exit code, but no port written so it's still an error.
func TestZeroExitcode(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	pluginPath, err := filepath.Abs("./testdata/provider-language")
	require.NoError(t, err)

	path := os.Getenv("PATH")
	t.Setenv("PATH", pluginPath+string(os.PathListSeparator)+path)

	// Check exec.LookPath finds the plugin
	file, err := exec.LookPath("pulumi-language-test")
	require.NoError(t, err)
	require.Contains(t, file, "pulumi-language-test")

	t.Setenv("PULUMI_TEST_PLUGIN_EXITCODE", "0")
	pluginPathRel := filepath.Join("testdata", "test-plugin-exit")
	_, err = plugin.NewProviderFromPath(ctx.Host, ctx, pluginPathRel)
	require.ErrorContains(t, err, "could not read plugin ["+pluginPathRel+"]: EOF")

	// Build a tiny go program that will exit with a non-zero code and run that, check it gives the same result.
	tmp := t.TempDir()
	err = os.WriteFile(filepath.Join(tmp, "main.go"), []byte(`
	package main
	import "os"

	func main() {
		os.Exit(0)
	}
	`), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(`
	module test-plugin-exit
	go 1.24
	`), 0o600)
	require.NoError(t, err)

	// Build and run the program. On Windows an executable must carry the .exe extension or it
	// cannot be launched, so name the output accordingly.
	bin := "test-plugin-exit"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = tmp
	stdout, err := cmd.CombinedOutput()
	t.Log(string(stdout))
	require.NoError(t, err)

	_, err = plugin.NewProviderFromPath(ctx.Host, ctx, filepath.Join(tmp, bin))
	// the prefix of the error message is unstable because it's in a temp dir but we can check the start and end
	// separately.
	require.ErrorContains(t, err, "could not read plugin [")
	require.ErrorContains(t, err, bin+"]: EOF")
}

// Test a provider that has an incompatible version range in its `PulumiPlugin.yaml`.
//
//nolint:paralleltest // Modifying the global version.Version
func TestPulumiVersionRangeYaml(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, h.Close()) })
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)
	t.Cleanup(func() { ctx.Close() })

	oldVersion := version.Version
	version.Version = "3.1.2"
	t.Cleanup(func() { version.Version = oldVersion })

	_, err = plugin.NewProviderFromPath(ctx.Host, ctx, filepath.Join("testdata", "test-plugin-cli-version"))
	require.ErrorContains(t, err,
		"test-plugin-cli-version: Pulumi CLI version 3.1.2 does not satisfy the version range \">=100.0.0\"")
}

// fakeProvider answers the RPCs that the PULUMI_DEBUG_PROVIDERS attach path calls. Handshake is
// left unimplemented, which the attach path accepts as a legacy provider.
type fakeProvider struct {
	pulumirpc.UnimplementedResourceProviderServer

	// onGetPluginInfo, if set, runs before GetPluginInfo answers. The engine calls GetPluginInfo
	// while it boots the provider, so this is the hook that a provider uses to talk back to the
	// engine during its own boot.
	onGetPluginInfo func() error
}

func (f *fakeProvider) Attach(context.Context, *pulumirpc.PluginAttach) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (f *fakeProvider) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	if f.onGetPluginInfo != nil {
		if err := f.onGetPluginInfo(); err != nil {
			return nil, err
		}
	}
	return &pulumirpc.PluginInfo{Version: "1.0.0"}, nil
}

// serveFakeProvider serves prov on an ephemeral port and returns that port.
func serveFakeProvider(t *testing.T, prov *fakeProvider) int {
	cancel := make(chan bool)
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, prov)
			return nil
		},
	})
	require.NoError(t, err)
	// Do not wait for the shutdown to complete. If the test fails because a provider RPC is
	// stuck, a graceful stop never returns and the wait would hang the whole package.
	t.Cleanup(func() { close(cancel) })
	return handle.Port
}

// TestProviderLoadDuringProviderBoot ensures that a provider can request another provider on boot without deadlocking.
func TestProviderLoadDuringProviderBoot(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	host, ok := h.(*defaultHost)
	require.True(t, ok)
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	inner := workspace.PluginDescriptor{Name: "inner", Kind: apitype.ResourcePlugin}
	outer := workspace.PluginDescriptor{Name: "outer", Kind: apitype.ResourcePlugin}

	innerPort := serveFakeProvider(t, &fakeProvider{})
	var innerErr error
	outerPort := serveFakeProvider(t, &fakeProvider{
		onGetPluginInfo: func() error {
			_, innerErr = h.Provider(ctx, inner, env.Global())
			return innerErr
		},
	})
	t.Setenv("PULUMI_DEBUG_PROVIDERS", fmt.Sprintf("outer:%d,inner:%d", outerPort, innerPort))

	done := make(chan error, 1)
	go func() {
		_, err := h.Provider(ctx, outer, env.Global())
		done <- err
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("loading the outer provider deadlocked: its boot blocked the nested load of the inner provider")
	}
	require.NoError(t, innerErr)
	require.Len(t, host.resourcePlugins, 2)
	require.NoError(t, h.Close())
}
