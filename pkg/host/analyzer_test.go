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
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/require"
)

func TestAnalyzerSpawn(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	// Sanity test that from config.Map to envvars we see what we expect to see
	proj := "test-project"
	configMap := config.Map{
		config.MustMakeKey(proj, "bool"):   config.NewTypedValue("true", config.TypeBool),
		config.MustMakeKey(proj, "float"):  config.NewTypedValue("1.5", config.TypeFloat),
		config.MustMakeKey(proj, "string"): config.NewTypedValue("hello", config.TypeString),
		config.MustMakeKey(proj, "obj"):    config.NewObjectValue("{\"key\": \"value\"}"),
	}

	configDecrypted, err := configMap.Decrypt(config.NopDecrypter)
	require.NoError(t, err)

	opts := plugin.PolicyAnalyzerOptions{
		Organization: "test-org",
		Project:      proj,
		Stack:        "test-stack",
		DryRun:       true,
		Config:       configDecrypted,
		Tags:         map[string]string{"tag1": "value1", "tag2": "value2"},
	}

	pluginPath, err := filepath.Abs("./testdata/analyzer")
	require.NoError(t, err)

	path := os.Getenv("PATH")
	t.Setenv("PATH", pluginPath+string(os.PathListSeparator)+path)

	// Check exec.LookPath finds the analyzer
	file, err := exec.LookPath("pulumi-analyzer-policy-test")
	require.NoError(t, err)
	require.Contains(t, file, "pulumi-analyzer-policy-test")

	analyzer, err := plugin.NewPolicyAnalyzer(ctx.Host, ctx, "policypack", "./testdata/policypack", &opts, nil)
	require.NoError(t, err)

	err = analyzer.Close()
	require.NoError(t, err)
}

func TestAnalyzerSpawnNoConfig(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	pluginPath, err := filepath.Abs("./testdata/analyzer-no-config")
	require.NoError(t, err)

	path := os.Getenv("PATH")
	t.Setenv("PATH", pluginPath+string(os.PathListSeparator)+path)

	// Pass `nil` for the config, this is used for example in `pulumi policy
	// publish`, which does not run in the context of a stack.
	analyzer, err := plugin.NewPolicyAnalyzer(ctx.Host, ctx, "policypack", "./testdata/policypack", nil, nil)
	require.NoError(t, err)

	err = analyzer.Close()
	require.NoError(t, err)
}

// TestAnalyzerSpawnBinary verifies that NewPolicyAnalyzer can launch an analyzer plugin
// that is provided as a bare executable binary (no PulumiPolicy.yaml alongside it), mirroring
// the behavior of NewProvider.
func TestAnalyzerSpawnBinary(t *testing.T) {
	t.Parallel()

	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	// Build a tiny go program that exits with a non-zero code; we only need to prove that
	// NewPolicyAnalyzer actually executed the binary (rather than failing while looking for
	// a PulumiPolicy.yaml).
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
	module test-analyzer-exit
	go 1.24
	`), 0o600)
	require.NoError(t, err)

	bin := "test-analyzer-exit"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = tmp
	stdout, err := cmd.CombinedOutput()
	t.Log(string(stdout))
	require.NoError(t, err)

	_, err = plugin.NewPolicyAnalyzer(ctx.Host, ctx, "binary-analyzer", filepath.Join(tmp, bin), nil, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "exit status 1")
}

// TestAnalyzerBinaryVersionFromYaml verifies that when a binary analyzer plugin ships alongside a
// PulumiPolicy.yaml, GetAnalyzerInfo reports the version from the yaml rather than the version the
// plugin returns over the wire.
func TestAnalyzerBinaryVersionFromYaml(t *testing.T) {
	t.Parallel()

	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	binName := "pulumi-analyzer-binary"
	if runtime.GOOS == "windows" {
		binName += ".cmd"
	}
	pluginPath, err := filepath.Abs(filepath.Join("./testdata/analyzer-binary", binName))
	require.NoError(t, err)

	analyzer, err := plugin.NewPolicyAnalyzer(ctx.Host, ctx, "binary-analyzer", pluginPath, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, analyzer.Close()) }()

	info, err := analyzer.GetAnalyzerInfo(t.Context())
	require.NoError(t, err)
	// The binary reports "999.999.999" from GetAnalyzerInfo; the sibling PulumiPolicy.yaml pins 1.2.3.
	require.Equal(t, "1.2.3", info.Version)
}

func TestAnalyzerSpawnViaLanguage(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	// Sanity test that from config.Map to property values we see what we expect to see
	proj := "test-project"
	configMap := config.Map{
		config.MustMakeKey(proj, "bool"):   config.NewTypedValue("true", config.TypeBool),
		config.MustMakeKey(proj, "float"):  config.NewTypedValue("1.5", config.TypeFloat),
		config.MustMakeKey(proj, "string"): config.NewTypedValue("hello", config.TypeString),
		config.MustMakeKey(proj, "obj"):    config.NewObjectValue("{\"key\": \"value\"}"),
	}

	configDecrypted, err := configMap.Decrypt(config.NopDecrypter)
	require.NoError(t, err)

	opts := plugin.PolicyAnalyzerOptions{
		Organization: "test-org",
		Project:      proj,
		Stack:        "test-stack",
		DryRun:       true,
		Config:       configDecrypted,
		Tags:         map[string]string{"tag1": "value1", "tag2": "value2"},
	}

	pluginPath, err := filepath.Abs("./testdata/analyzer-language")
	require.NoError(t, err)

	path := os.Getenv("PATH")
	t.Setenv("PATH", pluginPath+string(os.PathListSeparator)+path)

	// Check exec.LookPath finds the language
	file, err := exec.LookPath("pulumi-language-test")
	require.NoError(t, err)
	require.Contains(t, file, "pulumi-language-test")

	analyzer, err := plugin.NewPolicyAnalyzer(ctx.Host, ctx, "policypack", "./testdata/policypack", &opts, nil)
	require.NoError(t, err)

	err = analyzer.Close()
	require.NoError(t, err)
}
