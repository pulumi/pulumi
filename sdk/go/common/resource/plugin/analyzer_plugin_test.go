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

package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestAnalyzerSpawn(t *testing.T) {
	d := diagtest.LogSink(t)
	ctx, err := NewContext(t.Context(), d, d, nil, nil, "", nil, false, nil, nil)
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

	opts := PolicyAnalyzerOptions{
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

	analyzer, err := NewPolicyAnalyzer(ctx.Host, ctx, "policypack", "./testdata/policypack", &opts, nil)
	require.NoError(t, err)

	err = analyzer.Close()
	require.NoError(t, err)
}

func TestAnalyzerSpawnNoConfig(t *testing.T) {
	d := diagtest.LogSink(t)
	ctx, err := NewContext(t.Context(), d, d, nil, nil, "", nil, false, nil, nil)
	require.NoError(t, err)

	pluginPath, err := filepath.Abs("./testdata/analyzer-no-config")
	require.NoError(t, err)

	path := os.Getenv("PATH")
	t.Setenv("PATH", pluginPath+string(os.PathListSeparator)+path)

	// Pass `nil` for the config, this is used for example in `pulumi policy
	// publish`, which does not run in the context of a stack.
	analyzer, err := NewPolicyAnalyzer(ctx.Host, ctx, "policypack", "./testdata/policypack", nil, nil)
	require.NoError(t, err)

	err = analyzer.Close()
	require.NoError(t, err)
}

func TestConstructEnvWithAdditionalEnv(t *testing.T) {
	t.Parallel()

	opts := &PolicyAnalyzerOptions{
		Organization: "test-org",
		Project:      "test-project",
		Stack:        "test-stack",
		DryRun:       false,
		AdditionalEnv: map[string]string{
			"MY_SECRET":  "secret-value",
			"AWS_REGION": "us-west-2",
		},
	}

	result, err := constructEnv(opts, "nodejs")
	require.NoError(t, err)

	// Verify standard env vars are set.
	val, found := result.GetStore().Raw("PULUMI_ORGANIZATION")
	require.True(t, found)
	require.Equal(t, "test-org", val)

	// Verify AdditionalEnv vars are injected.
	val, found = result.GetStore().Raw("MY_SECRET")
	require.True(t, found)
	require.Equal(t, "secret-value", val)

	val, found = result.GetStore().Raw("AWS_REGION")
	require.True(t, found)
	require.Equal(t, "us-west-2", val)
}

func TestConstructEnvWithoutAdditionalEnv(t *testing.T) {
	t.Parallel()

	opts := &PolicyAnalyzerOptions{
		Organization: "test-org",
		Project:      "test-project",
		Stack:        "test-stack",
		DryRun:       true,
	}

	result, err := constructEnv(opts, "python")
	require.NoError(t, err)

	// Standard vars should still be set.
	val, found := result.GetStore().Raw("PULUMI_DRY_RUN")
	require.True(t, found)
	require.Equal(t, "true", val)

	// Node.js-specific vars should not be set for python runtime.
	_, found = result.GetStore().Raw("PULUMI_NODEJS_ORGANIZATION")
	require.False(t, found)
}

func TestAnalyzerSpawnViaLanguage(t *testing.T) {
	d := diagtest.LogSink(t)
	ctx, err := NewContext(t.Context(), d, d, nil, nil, "", nil, false, nil, nil)
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

	opts := PolicyAnalyzerOptions{
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

	analyzer, err := NewPolicyAnalyzer(ctx.Host, ctx, "policypack", "./testdata/policypack", &opts, nil)
	require.NoError(t, err)

	err = analyzer.Close()
	require.NoError(t, err)
}

// TestAnalyzerGetAnalyzerInfo_Runtime covers the runtime-disambiguation logic:
// the plugin's reported runtime wins, falling back to the cached
// PulumiPolicy.yaml runtime when the plugin omits it (older SDKs).
func TestAnalyzerGetAnalyzerInfo_Runtime(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		fromPlugin  string
		fromProject string
		want        string
	}{
		{"plugin wins", "nodejs", "python", "nodejs"},
		{"fallback to project", "", "python", "python"},
		{"both empty", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := &analyzer{
				name:    tokens.QName("test-pack"),
				runtime: tc.fromProject,
				client: stubAnalyzerClient{info: &pulumirpc.AnalyzerInfo{
					Name: "test-pack", Runtime: tc.fromPlugin,
				}},
			}
			info, err := a.GetAnalyzerInfo()
			require.NoError(t, err)
			assert.Equal(t, tc.want, info.Runtime)
		})
	}
}

// TestAnalyzerServer_ForwardsRuntime verifies analyzerServer copies the
// Runtime field from the wrapped Analyzer onto the outgoing proto.
func TestAnalyzerServer_ForwardsRuntime(t *testing.T) {
	t.Parallel()

	srv := NewAnalyzerServer(stubAnalyzer{info: AnalyzerInfo{Runtime: "opa"}})
	resp, err := srv.GetAnalyzerInfo(t.Context(), &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "opa", resp.GetRuntime())
}

// stubAnalyzerClient is a pulumirpc.AnalyzerClient that only answers
// GetAnalyzerInfo. Embedding the generated interface lets unused methods
// nil-panic if a test accidentally calls them.
type stubAnalyzerClient struct {
	pulumirpc.AnalyzerClient
	info *pulumirpc.AnalyzerInfo
}

func (s stubAnalyzerClient) GetAnalyzerInfo(
	context.Context, *emptypb.Empty, ...grpc.CallOption,
) (*pulumirpc.AnalyzerInfo, error) {
	return s.info, nil
}

// stubAnalyzer is a plugin.Analyzer that only answers GetAnalyzerInfo.
type stubAnalyzer struct {
	Analyzer
	info AnalyzerInfo
}

func (s stubAnalyzer) GetAnalyzerInfo() (AnalyzerInfo, error) { return s.info, nil }
