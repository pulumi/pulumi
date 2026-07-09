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

package plugin

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin/oci"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

const integrationImage = "pulumi-test/oci-policy-pack:integration"

func requireDocker(t *testing.T) *oci.Runtime {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping OCI integration test in -short mode")
	}
	rt, err := oci.DetectRuntime(nil)
	if err != nil {
		t.Skip("no container runtime available; skipping OCI integration test")
	}
	return rt
}

func buildIntegrationImage(t *testing.T, rt *oci.Runtime) {
	t.Helper()
	dir := filepath.Join("oci", "testdata", "fake-analyzer")

	build := exec.Command("go", "build", "-o", filepath.Join(dir, "fake-analyzer"), "./"+dir)
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building fake analyzer: %s", out)
	t.Cleanup(func() { _ = os.Remove(filepath.Join(dir, "fake-analyzer")) })

	img := exec.Command(rt.Path, "build", "-t", integrationImage, dir)
	out, err = img.CombinedOutput()
	require.NoError(t, err, "building image: %s", out)
}

func TestOCIPolicyPackEndToEnd(t *testing.T) {
	t.Parallel()
	rt := requireDocker(t)
	buildIntegrationImage(t, rt)

	// A local pack directory whose manifest points at the built image.
	packDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"), []byte(
		"runtime:\n  name: oci\n  options:\n    repository: pulumi-test/oci-policy-pack\nversion: integration\n"), 0o600))

	pctx, err := NewContext(t.Context(), nil, nil, &MockHost{}, nil, t.TempDir(), nil, false, nil)
	require.NoError(t, err)

	a, err := NewPolicyAnalyzer(&fakeHost{addr: "127.0.0.1:1"}, pctx, "oci-integration-pack", packDir, nil, nil)
	require.NoError(t, err)
	// Stop the container even if an assertion below aborts the test; Close is
	// idempotent, so the happy-path Close below is unaffected.
	t.Cleanup(func() { _ = a.Close() })

	info, err := a.GetAnalyzerInfo(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "oci-integration-pack", info.Name)

	analyzeResp, err := a.Analyze(t.Context(), AnalyzerResource{
		URN:        resource.URN("urn:pulumi:stack::proj::aws:s3/bucket:Bucket::b"),
		Type:       tokens.Type("aws:s3/bucket:Bucket"),
		Name:       "b",
		Properties: property.Map{},
	})
	require.NoError(t, err)
	require.Len(t, analyzeResp.Diagnostics, 1)
	assert.Equal(t, "ran-in-container", analyzeResp.Diagnostics[0].Message)

	require.NoError(t, a.Close())

	// The container must be gone after Close (launched with --rm). Ctrl-C /
	// engine shutdown reach the same path: the host closes its analyzers,
	// which invokes this Close — so this assertion also covers the spec's
	// cancellation requirement (no orphaned containers on interrupt).
	out, err := exec.Command(rt.Path, "ps", "--filter", "label="+oci.LabelKey, "--format", "{{.ID}}").Output()
	require.NoError(t, err)
	assert.Empty(t, string(out), "no policy pack containers should survive Close")
}
