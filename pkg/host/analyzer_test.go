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
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
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

func buildBinaryTestPack(t *testing.T, binRel string) string {
	packDir := t.TempDir()
	binPath := filepath.Join(packDir, filepath.FromSlash(binRel))
	require.NoError(t, os.MkdirAll(filepath.Dir(binPath), 0o755))
	cmd := exec.Command("go", "build", "-o", binPath, "./testdata/analyzer-binary")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	// The manifest still names a language runtime; the declared binary must win dispatch.
	manifest := fmt.Sprintf("runtime: nodejs\nbinary:\n  %s: %s\n", workspace.CurrentPlatform(), binRel)
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"), []byte(manifest), 0o600))
	return packDir
}

func newAnalyzerTestContext(t *testing.T) *plugin.Context {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, h.Close()) })
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)
	return ctx
}

// TestAnalyzerSpawnBinaryAtRoot spawns a pack laid out like an installed binary
// artifact: the binary at the pack root under its canonical name.
func TestAnalyzerSpawnBinaryAtRoot(t *testing.T) {
	t.Parallel()

	binName := "pulumi-analyzer-binary-test-pack"
	if goruntime.GOOS == "windows" {
		binName += ".exe"
	}
	packDir := buildBinaryTestPack(t, binName)
	ctx := newAnalyzerTestContext(t)

	analyzer, err := plugin.NewPolicyAnalyzer(ctx.Host, ctx, "policypack", packDir, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, analyzer.Close()) }()

	info, err := analyzer.GetAnalyzerInfo(t.Context())
	require.NoError(t, err)
	require.Equal(t, "binary-test-pack", info.Name)
}

// TestAnalyzerSpawnBinaryInBin spawns a pack laid out like a local build: the binary
// under bin/, declared in the manifest.
func TestAnalyzerSpawnBinaryInBin(t *testing.T) {
	t.Parallel()

	binName := "pulumi-analyzer-binary-test-pack-" + workspace.CurrentPlatform()
	if goruntime.GOOS == "windows" {
		binName += ".exe"
	}
	packDir := buildBinaryTestPack(t, "bin/"+binName)
	ctx := newAnalyzerTestContext(t)

	analyzer, err := plugin.NewPolicyAnalyzer(ctx.Host, ctx, "policypack", packDir, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, analyzer.Close()) }()

	info, err := analyzer.GetAnalyzerInfo(t.Context())
	require.NoError(t, err)
	require.Equal(t, "binary-test-pack", info.Name)
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
