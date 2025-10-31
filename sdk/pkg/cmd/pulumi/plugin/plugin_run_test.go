// Copyright 2016-2025, Pulumi Corporation.
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
	"strings"
	"testing"

	pkgWorkspace "github.com/pulumi/pulumi/sdk/v3/pkg/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSetup creates a plugin context and RPC server for testing
func testSetup(t *testing.T) (context.Context, *plugin.Context, *plugin.GrpcServer) {
	t.Helper()

	ctx := context.Background()
	pctx, err := plugin.NewContext(ctx, nil, nil, nil, nil, ".", nil, false, nil)
	require.NoError(t, err)
	t.Cleanup(func() { pctx.Close() })

	server, err := createPluginRPCServer(ctx, pctx)
	require.NoError(t, err)
	require.NotNil(t, server)
	t.Cleanup(func() { server.Close() })

	return ctx, pctx, server
}

// testSetupWithCleanEnv creates a plugin context and RPC server with clean environment
// This clears PULUMI_ACCESS_TOKEN and PULUMI_API to avoid interference from CI/other tests
// Tests using this cannot use t.Parallel()
func testSetupWithCleanEnv(t *testing.T) (context.Context, *plugin.Context, *plugin.GrpcServer) {
	t.Helper()

	// Clear environment variables that might interfere with tests
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	t.Setenv("PULUMI_API", "")

	return testSetup(t)
}

// mockWorkspaceNoProject returns a mock workspace that returns ErrProjectNotFound
func mockWorkspaceNoProject() *pkgWorkspace.MockContext {
	return &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
}

// mockWorkspaceWithProject returns a mock workspace with project and credentials
func mockWorkspaceWithProject(cloudURL, token string) *pkgWorkspace.MockContext {
	return &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{
				Name: "test-project",
			}, "/test/path", nil
		},
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{
				Current:	cloudURL,
				AccessTokens: map[string]string{
					cloudURL: token,
				},
			}, nil
		},
	}
}

// mockWorkspaceWithCredentialsOnly returns a mock workspace with credentials but no project
func mockWorkspaceWithCredentialsOnly(cloudURL, token string) *pkgWorkspace.MockContext {
	return &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", nil
		},
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{
				Current:	cloudURL,
				AccessTokens: map[string]string{
					cloudURL: token,
				},
			}, nil
		},
	}
}

// findEnvVar searches for an environment variable with the given prefix
// Returns the value after the prefix and whether it was found
// If the variable appears multiple times, returns the last value (which takes precedence)
func findEnvVar(env []string, prefix string) (string, bool) {
	var lastValue string
	var found bool
	for _, e := range env {
		if value, ok := strings.CutPrefix(e, prefix); ok {
			lastValue = value
			found = true
		}
	}
	return lastValue, found
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

func TestPreparePluginEnv_SetsRPCTarget(t *testing.T) {
	t.Parallel()

	_, _, server := testSetup(t)

	env := preparePluginEnv(mockWorkspaceNoProject(), server)

	// Verify PULUMI_RPC_TARGET is set
	addr, found := findEnvVar(env, "PULUMI_RPC_TARGET=")
	assert.True(t, found, "environment should contain PULUMI_RPC_TARGET")
	assert.NotEmpty(t, addr, "RPC target address should not be empty")
	assert.Contains(t, addr, ":", "RPC target should contain a port")
}

// Note: Cannot use t.Parallel() because this test uses t.Setenv()
func TestPreparePluginEnv_IncludesExistingEnvironment(t *testing.T) {
	_, _, server := testSetup(t)

	// Set a test environment variable
	testEnvKey := "TEST_PLUGIN_ENV_VAR"
	testEnvValue := "test-value-123"
	t.Setenv(testEnvKey, testEnvValue)

	env := preparePluginEnv(mockWorkspaceNoProject(), server)

	// Verify existing environment variable is included
	value, found := findEnvVar(env, testEnvKey+"=")
	assert.True(t, found, "existing environment variables should be included")
	assert.Equal(t, testEnvValue, value)
}

//nolint:paralleltest // Cannot use t.Parallel() because this test uses t.Setenv via testSetupWithCleanEnv
func TestPreparePluginEnv_WithMockWorkspace_NoProject(t *testing.T) {
	_, _, server := testSetupWithCleanEnv(t)

	env := preparePluginEnv(mockWorkspaceNoProject(), server)

	// Verify PULUMI_RPC_TARGET is always set
	rpcValue, foundRPCTarget := findEnvVar(env, "PULUMI_RPC_TARGET=")
	apiValue, foundAPI := findEnvVar(env, "PULUMI_API=")
	tokenValue, foundToken := findEnvVar(env, "PULUMI_ACCESS_TOKEN=")

	assert.True(t, foundRPCTarget, "PULUMI_RPC_TARGET should always be set")
	assert.NotEmpty(t, rpcValue, "PULUMI_RPC_TARGET should have a value")
	// API and token should either not be found or be empty when ReadProject fails
	if foundAPI {
		assert.Empty(t, apiValue, "PULUMI_API should be empty when ReadProject fails")
	}
	if foundToken {
		assert.Empty(t, tokenValue, "PULUMI_ACCESS_TOKEN should be empty when ReadProject fails")
	}
}

//nolint:paralleltest // Cannot use t.Parallel() because this test uses t.Setenv via testSetupWithCleanEnv
func TestPreparePluginEnv_WithMockWorkspace_WithProject(t *testing.T) {
	_, _, server := testSetupWithCleanEnv(t)

	cloudURL := "https://api.test-pulumi.com"
	token := "test-token-123"

	env := preparePluginEnv(mockWorkspaceWithProject(cloudURL, token), server)

	// Verify all environment variables are set correctly
	_, foundRPCTarget := findEnvVar(env, "PULUMI_RPC_TARGET=")
	apiValue, foundAPI := findEnvVar(env, "PULUMI_API=")
	tokenValue, foundToken := findEnvVar(env, "PULUMI_ACCESS_TOKEN=")

	assert.True(t, foundRPCTarget, "PULUMI_RPC_TARGET should be set")
	assert.True(t, foundAPI, "PULUMI_API should be set from workspace credentials")
	assert.Equal(t, cloudURL, apiValue, "PULUMI_API should match mocked cloud URL")
	assert.True(t, foundToken, "PULUMI_ACCESS_TOKEN should be set from workspace credentials")
	assert.Equal(t, token, tokenValue, "PULUMI_ACCESS_TOKEN should match mocked token")
}

//nolint:paralleltest // Cannot use t.Parallel() because this test uses t.Setenv via testSetupWithCleanEnv
func TestPreparePluginEnv_WithMockWorkspace_NoProject_WithCredentials(t *testing.T) {
	_, _, server := testSetupWithCleanEnv(t)

	cloudURL := "https://api.test-pulumi.com"
	token := "test-token-123"

	env := preparePluginEnv(mockWorkspaceWithCredentialsOnly(cloudURL, token), server)

	// Verify all environment variables are set correctly even without a project
	_, foundRPCTarget := findEnvVar(env, "PULUMI_RPC_TARGET=")
	apiValue, foundAPI := findEnvVar(env, "PULUMI_API=")
	tokenValue, foundToken := findEnvVar(env, "PULUMI_ACCESS_TOKEN=")

	assert.True(t, foundRPCTarget, "PULUMI_RPC_TARGET should be set")
	assert.True(t, foundAPI, "PULUMI_API should be set from stored credentials even without project")
	assert.Equal(t, cloudURL, apiValue, "PULUMI_API should match cloud URL from credentials")
	assert.True(t, foundToken, "PULUMI_ACCESS_TOKEN should be set from stored credentials")
	assert.Equal(t, token, tokenValue, "PULUMI_ACCESS_TOKEN should match token from credentials")
}

//nolint:paralleltest // Cannot use t.Parallel() because this test uses t.Setenv
func TestNewInstallPluginFunc_DisabledAcquisition(t *testing.T) {
	// Set environment to disable automatic plugin acquisition
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true")

	ctx := context.Background()
	pctx, err := plugin.NewContext(ctx, nil, nil, nil, nil, ".", nil, false, nil)
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

	ctx := context.Background()
	pctx, err := plugin.NewContext(ctx, nil, nil, nil, nil, ".", nil, false, nil)
	require.NoError(t, err)
	defer pctx.Close()

	installPlugin := newInstallPluginFunc(pctx)

	// Should return nil when plugin install fails (nonexistent plugin)
	// This will attempt to install but fail, and should return nil without panicking
	version := installPlugin("nonexistent-plugin-that-does-not-exist-12345")
	assert.Nil(t, version, "should return nil when plugin installation fails")
}

//nolint:paralleltest // Cannot use t.Parallel() because this test executes a subprocess
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

	// Create mock workspace with credentials
	cloudURL := "https://api.test-pulumi.com"
	token := "test-token-123"
	mockWs := mockWorkspaceWithProject(cloudURL, token)

	// Create the command
	cmd := newPluginRunCmd(mockWs)
	cmd.SetArgs([]string{pluginPath})

	// Execute the command
	ctx := context.Background()
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
