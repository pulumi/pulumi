// Copyright 2016, Pulumi Corporation.
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

package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSetup creates a plugin context and RPC server for testing
func testSetup(t *testing.T) (context.Context, *plugin.Context, *plugin.GrpcServer) {
	t.Helper()

	ctx := t.Context()
	pluginHost, err := pkghost.New(context.WithoutCancel(ctx), nil, nil, nil, nil,
		schema.NewLoaderServerFromContext, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { pluginHost.Close() })
	pctx, err := plugin.NewContext(ctx, nil, nil, pluginHost, nil, ".", nil, false, nil)
	require.NoError(t, err)
	t.Cleanup(func() { pctx.Close() })

	server, err := createPluginRPCServer(ctx, pctx)
	require.NoError(t, err)
	require.NotNil(t, server)
	t.Cleanup(func() { server.Close() })

	return ctx, pctx, server
}

func TestCreatePluginRPCServer(t *testing.T) {
	t.Parallel()

	ctx, pctx, server := testSetup(t)

	// Verify server address is set
	addr := server.Addr()
	assert.NotEmpty(t, addr, "gRPC server should have an address")
	assert.Contains(t, addr, ":", "address should contain a port")

	// Verify server can be closed without error
	err := server.Close()
	require.NoError(t, err, "server should close without error")

	// Verify we can recreate with same context
	server2, err := createPluginRPCServer(ctx, pctx)
	require.NoError(t, err)
	require.NotNil(t, server2)
	defer server2.Close()
}

//nolint:paralleltest // Cannot use t.Parallel() because this test uses t.Setenv
func TestNewInstallPluginFunc_DisabledAcquisition(t *testing.T) {
	// Set environment to disable automatic plugin acquisition
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true")

	pluginHost, err := pkghost.New(context.WithoutCancel(t.Context()), nil, nil, nil, nil,
		schema.NewLoaderServerFromContext, nil, nil)
	require.NoError(t, err)
	defer pluginHost.Close()
	pctx, err := plugin.NewContext(t.Context(), nil, nil, pluginHost, nil, ".", nil, false, nil)
	require.NoError(t, err)
	defer pctx.Close()

	installPlugin := newInstallPluginFunc(pctx)

	// Should return nil when automatic acquisition is disabled
	version := installPlugin("test-provider")
	assert.Nil(t, version, "should return nil when automatic plugin acquisition is disabled")
}

//nolint:paralleltest // Cannot use t.Parallel() because this test uses t.Setenv
func TestNewInstallPluginFunc_PluginInstallError(t *testing.T) {
	// Clear the environment variable to enable automatic acquisition
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

	pluginHost, err := pkghost.New(context.WithoutCancel(t.Context()), nil, nil, nil, nil,
		schema.NewLoaderServerFromContext, nil, nil)
	require.NoError(t, err)
	defer pluginHost.Close()
	pctx, err := plugin.NewContext(t.Context(), nil, nil, pluginHost, nil, ".", nil, false, nil)
	require.NoError(t, err)
	defer pctx.Close()

	installPlugin := newInstallPluginFunc(pctx)

	// Should return nil when plugin install fails (nonexistent plugin)
	// This will attempt to install but fail, and should return nil without panicking
	version := installPlugin("nonexistent-plugin-that-does-not-exist-12345")
	assert.Nil(t, version, "should return nil when plugin installation fails")
}

//nolint:paralleltest // Cannot use t.Parallel() because this test uses t.Setenv
func TestPluginRunCommand(t *testing.T) {
	// Skip on Windows - test uses bash script which is not cross-platform
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - test requires bash")
	}

	// Create a temporary directory for our test plugin
	tmpDir := t.TempDir()

	// Create output file path
	outputFile := filepath.Join(tmpDir, "env-output.txt")

	// Create a simple test plugin that writes its environment variables to a file
	pluginPath := filepath.Join(tmpDir, "pulumi-tool-testplugin")
	pluginScript := `#!/bin/bash
cat > ` + outputFile + ` <<EOF
PULUMI_RPC_TARGET=$PULUMI_RPC_TARGET
PULUMI_API=$PULUMI_API
PULUMI_ACCESS_TOKEN=$PULUMI_ACCESS_TOKEN
EOF
exit 0
`
	//nolint:gosec // G306: File needs to be executable (0755)
	err := os.WriteFile(pluginPath, []byte(pluginScript), 0o755)
	require.NoError(t, err)

	// The API address and token reach the plugin through the plugin context, resolved from the
	// active login; drive that resolution from the environment here.
	cloudURL := "https://api.test-pulumi.com"
	token := "test-token-123"
	t.Setenv("PULUMI_BACKEND_URL", cloudURL)
	t.Setenv("PULUMI_ACCESS_TOKEN", token)

	// Create the command
	cmd := newPluginRunCmd()
	cmd.SetArgs([]string{pluginPath})

	// Execute the command
	ctx := t.Context()
	cmd.SetContext(ctx)
	err = cmd.Execute()
	require.NoError(t, err)

	// Read the output file written by the plugin
	output, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	outputStr := string(output)

	// Verify the plugin received the environment variables
	assert.Contains(t, outputStr, "PULUMI_RPC_TARGET=", "Plugin should receive PULUMI_RPC_TARGET")
	assert.Contains(t, outputStr, "PULUMI_API="+cloudURL, "Plugin should receive PULUMI_API")
	assert.Contains(t, outputStr, "PULUMI_ACCESS_TOKEN="+token, "Plugin should receive PULUMI_ACCESS_TOKEN")
}

func TestPluginRunCommandError(t *testing.T) {
	t.Parallel()

	// Skip on Windows - test uses bash script which is not cross-platform
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - test requires bash")
	}

	// Create a temporary directory for our test plugin
	tmpDir := t.TempDir()

	// Create a simple test plugin that writes its environment variables to a file
	pluginPath := filepath.Join(tmpDir, "pulumi-tool-testplugin")
	pluginScript := `#!/bin/bash
exit 42
`
	//nolint:gosec // G306: File needs to be executable (0755)
	err := os.WriteFile(pluginPath, []byte(pluginScript), 0o755)
	require.NoError(t, err)

	// Create the command
	runCmd := newPluginRunCmd()
	runCmd.SetArgs([]string{pluginPath})

	// Execute the command
	ctx := t.Context()
	runCmd.SetContext(ctx)
	err = runCmd.Execute()
	require.True(t, result.IsBail(err))

	var cec cmd.CustomExitCodeError
	require.ErrorAs(t, err, &cec)
	assert.Equal(t, 42, cec.CustomExitCode())
}
