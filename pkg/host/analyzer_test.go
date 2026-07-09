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
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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

func buildExecutableTestPack(t *testing.T) (packDir, binRel string) {
	packDir = t.TempDir()
	binRel = filepath.Join("bin", "policy")
	if goruntime.GOOS == "windows" {
		binRel += ".exe"
	}
	binPath := filepath.Join(packDir, binRel)
	cmd := exec.Command("go", "build", "-o", binPath, "./testdata/analyzer-executable")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return packDir, binRel
}

func writeExecutableManifest(t *testing.T, packDir string, binaries map[string]string) {
	var sb strings.Builder
	sb.WriteString("runtime:\n  name: executable\n  options:\n    binaries:\n")
	for _, platform := range slices.Sorted(maps.Keys(binaries)) {
		fmt.Fprintf(&sb, "      %s: %s\n", platform, filepath.ToSlash(binaries[platform]))
	}
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"), []byte(sb.String()), 0o600))
}

func TestAnalyzerSpawnExecutable(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	packDir, binRel := buildExecutableTestPack(t)
	writeExecutableManifest(t, packDir, map[string]string{workspace.CurrentPlatform(): binRel})

	analyzer, err := plugin.NewPolicyAnalyzer(ctx.Host, ctx, "policypack", packDir, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, analyzer.Close()) }()

	info, err := analyzer.GetAnalyzerInfo(t.Context())
	require.NoError(t, err)
	require.Equal(t, "executable-test-pack", info.Name)
}

func TestAnalyzerSpawnExecutableMissingPlatform(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	otherPlatform := "linux-amd64"
	if workspace.CurrentPlatform() == otherPlatform {
		otherPlatform = "darwin-arm64"
	}
	packDir := t.TempDir()
	writeExecutableManifest(t, packDir, map[string]string{otherPlatform: "bin/policy"})

	_, err = plugin.NewPolicyAnalyzer(ctx.Host, ctx, "policypack", packDir, nil, nil)
	require.ErrorContains(t, err, "does not provide a binary for "+workspace.CurrentPlatform())
	require.ErrorContains(t, err, otherPlatform)
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
