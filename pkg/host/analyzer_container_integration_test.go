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

package host

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin/oci"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

const integrationImage = "pulumi-test/oci-policy-pack:integration"

// fakeAnalyzerDir is the shared fake-analyzer build context; it stays with the
// launcher package it exercises.
const fakeAnalyzerDir = "../resource/plugin/oci/testdata/fake-analyzer"

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
	bin := filepath.Join(fakeAnalyzerDir, "fake-analyzer")

	build := exec.Command("go", "build", "-o", "fake-analyzer", ".")
	build.Dir = fakeAnalyzerDir
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building fake analyzer: %s", out)
	t.Cleanup(func() { _ = os.Remove(bin) })

	img := exec.Command(rt.Path, "build", "-t", integrationImage, fakeAnalyzerDir)
	out, err = img.CombinedOutput()
	require.NoError(t, err, "building image: %s", out)
}

// TestContainerPolicyPackEndToEnd boots a local runtime "oci" pack through the
// production path — defaultHost.PolicyAnalyzer dispatching to a container
// launch — and verifies the host's teardown reaps the container.
func TestContainerPolicyPackEndToEnd(t *testing.T) {
	t.Parallel()
	rt := requireDocker(t)
	buildIntegrationImage(t, rt)

	// A local pack directory whose manifest points at the built image. No tag
	// in the image option: the pack version is the tag.
	packDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"), []byte(
		"runtime:\n  name: oci\n  options:\n    image: pulumi-test/oci-policy-pack\nversion: integration\n"), 0o600))

	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	pctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	a, err := h.PolicyAnalyzer(pctx, "oci-integration-pack", packDir, nil)
	require.NoError(t, err)

	info, err := a.GetAnalyzerInfo(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "oci-integration-pack", info.Name)

	analyzeResp, err := a.Analyze(t.Context(), plugin.AnalyzerResource{
		URN:        resource.URN("urn:pulumi:stack::proj::aws:s3/bucket:Bucket::b"),
		Type:       tokens.Type("aws:s3/bucket:Bucket"),
		Name:       "b",
		Properties: property.Map{},
	})
	require.NoError(t, err)
	require.Len(t, analyzeResp.Diagnostics, 1)
	assert.Equal(t, "ran-in-container", analyzeResp.Diagnostics[0].Message)

	// ReleaseContext is the production teardown path (Context.Close calls it):
	// it must stop the container booted for this context.
	require.NoError(t, h.ReleaseContext(pctx))

	// The container must be gone (launched with --rm). Ctrl-C / engine
	// shutdown reach the same path, so this also covers the spec's
	// cancellation requirement (no orphaned containers on interrupt).
	out, err := exec.Command(rt.Path, "ps", "--filter", "label="+oci.LabelKey, "--format", "{{.ID}}").Output()
	require.NoError(t, err)
	assert.Empty(t, string(out), "no policy pack containers should survive ReleaseContext")
}
