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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/stretchr/testify/require"
)

func TestStartupFailure(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)

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
	h, err := New(t.Context(), d, d, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)

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

	// Build and run the program
	cmd := exec.Command("go", "build", "-o", "test-plugin-exit", ".")
	cmd.Dir = tmp
	stdout, err := cmd.CombinedOutput()
	t.Log(string(stdout))
	require.NoError(t, err)

	_, err = plugin.NewProviderFromPath(ctx.Host, ctx, filepath.Join(tmp, "test-plugin-exit"))
	// the prefix of the error message is unstable because it's in a temp dir but we can check the start and end
	// separately.
	require.ErrorContains(t, err, "could not read plugin [")
	require.ErrorContains(t, err, "test-plugin-exit]: exit status 1")
}

// Similar to TestNonZeroExitcode but with a zero exit code, but no port written so it's still an error.
func TestZeroExitcode(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)

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

	// Build and run the program
	cmd := exec.Command("go", "build", "-o", "test-plugin-exit", ".")
	cmd.Dir = tmp
	stdout, err := cmd.CombinedOutput()
	t.Log(string(stdout))
	require.NoError(t, err)

	_, err = plugin.NewProviderFromPath(ctx.Host, ctx, filepath.Join(tmp, "test-plugin-exit"))
	// the prefix of the error message is unstable because it's in a temp dir but we can check the start and end
	// separately.
	require.ErrorContains(t, err, "could not read plugin [")
	require.ErrorContains(t, err, "test-plugin-exit]: EOF")
}

// Test a provider that has an incompatible version range in its `PulumiPlugin.yaml`.
//
//nolint:paralleltest // Modifying the global version.Version
func TestPulumiVersionRangeYaml(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, h.Close()) })
	ctx := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	t.Cleanup(func() { ctx.Close() })

	oldVersion := version.Version
	version.Version = "3.1.2"
	t.Cleanup(func() { version.Version = oldVersion })

	_, err = plugin.NewProviderFromPath(ctx.Host, ctx, filepath.Join("testdata", "test-plugin-cli-version"))
	require.ErrorContains(t, err,
		"test-plugin-cli-version: Pulumi CLI version 3.1.2 does not satisfy the version range \">=100.0.0\"")
}
