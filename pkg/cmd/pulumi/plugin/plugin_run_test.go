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
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain also runs this test executable as a credential helper or plugin child.
// Tests use os.Executable() with -credential-helper-test, followed by "credentials" and a JSON response,
// "environment" and a log path, or "exit" to return code 42. Other invocations run tests normally.
func TestMain(m *testing.M) {
	for i, arg := range os.Args {
		if arg != "-credential-helper-test" {
			continue
		}
		args := os.Args[i+1:]
		if len(args) < 1 {
			os.Exit(1)
		}
		switch args[0] {
		case "credentials":
			if len(args) != 2 {
				os.Exit(1)
			}
			if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
				os.Exit(1)
			}
			if _, err := io.WriteString(os.Stdout, args[1]); err != nil {
				os.Exit(1)
			}
		case "environment":
			if len(args) != 2 {
				os.Exit(1)
			}
			values := map[string]string{}
			for _, name := range []string{"PULUMI_RPC_TARGET", "PULUMI_API", "PULUMI_ACCESS_TOKEN", "HELPER_CHILD"} {
				if value, ok := os.LookupEnv(name); ok {
					values[name] = value
				}
			}
			data, err := json.Marshal(values)
			if err != nil {
				os.Exit(1)
			}
			if err := os.WriteFile(args[1], data, 0o600); err != nil {
				os.Exit(1)
			}
		case "exit":
			os.Exit(42)
		default:
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// testSetup creates a plugin context and RPC server for testing
func testSetup(t *testing.T) (context.Context, *plugin.Context, *plugin.GrpcServer) {
	t.Helper()

	ctx := t.Context()
	pluginHost, err := pkghost.New(context.WithoutCancel(ctx), nil, nil, nil, nil,
		schema.NewLoaderServerFromContext, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { pluginHost.Close() })
	pctx, err := plugin.NewContext(ctx, nil, nil, pluginHost, nil, ".", nil, false, nil, nil)
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

func TestNewInstallPluginFunc_DisabledAcquisition(t *testing.T) {
	// Set environment to disable automatic plugin acquisition
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true")

	pluginHost, err := pkghost.New(context.WithoutCancel(t.Context()), nil, nil, nil, nil,
		schema.NewLoaderServerFromContext, nil, nil)
	require.NoError(t, err)
	defer pluginHost.Close()
	pctx, err := plugin.NewContext(t.Context(), nil, nil, pluginHost, nil, ".", nil, false, nil, nil)
	require.NoError(t, err)
	defer pctx.Close()

	installPlugin := newInstallPluginFunc(pctx)

	// Should return nil when automatic acquisition is disabled
	version := installPlugin("test-provider")
	assert.Nil(t, version, "should return nil when automatic plugin acquisition is disabled")
}

func TestNewInstallPluginFunc_PluginInstallError(t *testing.T) {
	// Clear the environment variable to enable automatic acquisition
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

	pluginHost, err := pkghost.New(context.WithoutCancel(t.Context()), nil, nil, nil, nil,
		schema.NewLoaderServerFromContext, nil, nil)
	require.NoError(t, err)
	defer pluginHost.Close()
	pctx, err := plugin.NewContext(t.Context(), nil, nil, pluginHost, nil, ".", nil, false, nil, nil)
	require.NoError(t, err)
	defer pctx.Close()

	installPlugin := newInstallPluginFunc(pctx)

	// Should return nil when plugin install fails (nonexistent plugin)
	// This will attempt to install but fail, and should return nil without panicking
	version := installPlugin("nonexistent-plugin-that-does-not-exist-12345")
	assert.Nil(t, version, "should return nil when plugin installation fails")
}

func TestPluginRunCommand(t *testing.T) {
	for _, test := range []struct {
		name, response, token           string
		resolveBeforeRun, fails, stored bool
	}{
		{name: "ordinary credentials", token: "ambient-token"},
		{name: "stored credentials", token: "stored-token", stored: true},
		{name: "declined", response: "{}", token: "ambient-token"},
		{
			name:     "token",
			response: `{"version":1,"accessToken":"helper-token","env":{"HELPER_CHILD":"yes"}}`, token: "helper-token",
		},
		{name: "headers", response: `{"version":1,"headers":{"X-Session":["secret"]}}`, token: ""},
		{
			name:     "cached token",
			response: `{"version":1,"accessToken":"helper-token"}`, token: "helper-token", resolveBeforeRun: true,
		},
		{name: "helper failure", response: "not json", fails: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_BACKEND_URL", "https://api.example.com")
			t.Setenv("PULUMI_ACCESS_TOKEN", "ambient-token")
			if test.stored {
				t.Setenv("PULUMI_ACCESS_TOKEN", "")
				require.NoError(t, workspace.StoreAccount("https://api.example.com",
					workspace.Account{AccessToken: test.token}, false))
			}
			ptesting.Unsetenv(t, "HELPER_CHILD")
			session := credentialhelper.NewSession(io.Discard)
			lm := cmdBackend.NewLoginManager(diagtest.LogSink(t), session)
			old := cmdBackend.DefaultLoginManager
			cmdBackend.DefaultLoginManager = lm
			t.Cleanup(func() { cmdBackend.DefaultLoginManager = old })

			dir := t.TempDir()
			pluginPath, err := os.Executable()
			require.NoError(t, err)
			if test.response != "" {
				require.NoError(t, session.Configure(
					pluginPath, []string{"--", "-credential-helper-test", "credentials", test.response}))
			}
			if test.resolveBeforeRun {
				r, err := session.Resolve(t.Context(), "https://api.example.com")
				require.NoError(t, err)
				require.NoError(t, session.SelectBackend(r, r.BackendURL()))
			}

			outputFile := filepath.Join(dir, "env")
			cmd := newPluginRunCmd()
			cmd.SetContext(t.Context())
			cmd.SetArgs([]string{pluginPath, "--", "-credential-helper-test", "environment", outputFile})
			err = cmd.Execute()

			if test.fails {
				require.ErrorContains(t, err, "credentials helper response must be a JSON object")
				_, err = os.Stat(outputFile)
				require.ErrorIs(t, err, os.ErrNotExist, "the plugin must not start after helper failure")
				return
			}
			require.NoError(t, err)
			output, err := os.ReadFile(outputFile)
			require.NoError(t, err)
			var values map[string]string
			require.NoError(t, json.Unmarshal(output, &values))
			assert.NotEmpty(t, values["PULUMI_RPC_TARGET"], "plugin RPC target")
			assert.Equal(t, "https://api.example.com", values["PULUMI_API"])
			if test.token == "" {
				assert.NotContains(t, values, "PULUMI_ACCESS_TOKEN")
			} else {
				assert.Equal(t, test.token, values["PULUMI_ACCESS_TOKEN"])
			}
			if test.name == "token" {
				assert.Equal(t, "yes", values["HELPER_CHILD"], "helper environment reaches the first child")
			}
		})
	}
}

func TestPluginRunCommandError(t *testing.T) {
	t.Parallel()

	pluginPath, err := os.Executable()
	require.NoError(t, err)

	// Create the command
	runCmd := newPluginRunCmd()
	runCmd.SetArgs([]string{pluginPath, "--", "-credential-helper-test", "exit"})

	// Execute the command
	ctx := t.Context()
	runCmd.SetContext(ctx)
	err = runCmd.Execute()
	require.True(t, result.IsBail(err))

	var cec cmd.CustomExitCodeError
	require.ErrorAs(t, err, &cec)
	assert.Equal(t, 42, cec.CustomExitCode())
}
