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

package oci

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportImageLoadsLayout proves the build contract's sink: a runtime-neutral OCI
// image layout (as any decoupled builder emits, naming no runtime and no location) is
// loaded into the docker daemon and tagged with the ref the orchestrator supplies. This
// is the one verb that re-couples a decoupled build's artifact to a particular runtime,
// so it is exercised against a real daemon rather than mocked. It is daemon-gated: it
// skips where docker is absent (CI without a daemon).
func TestImportImageLoadsLayout(t *testing.T) {
	t.Parallel()
	if !dockerAvailable(t) {
		t.Skip("docker daemon not available")
	}
	ctx := context.Background()

	// Build a small OCI layout DIRECTORY with go-containerregistry — the same digest-
	// addressed shape kaniko's --oci-layout-path emits, but hermetic (no builder, no
	// network). One random single-layer image under a one-manifest index.
	img, err := random.Image(1024, 1)
	require.NoError(t, err)
	layoutPath := filepath.Join(t.TempDir(), "layout")
	_, err = layout.Write(layoutPath, mutate.AppendManifests(empty.Index, mutate.IndexAddendum{Add: img}))
	require.NoError(t, err)

	// A location the build never chose: the ref is applied here, at the sink.
	ref := "pulumi-oci-import-test:v0"
	m := NewDockerPodManager("import-test")
	t.Cleanup(func() { _, _ = dockerCmd(context.WithoutCancel(ctx), "rmi", "-f", ref) })

	require.NoError(t, m.ImportImage(ctx, layoutPath, ref))

	// The image is now present in the store under exactly the ref we asked for.
	exists, err := m.ImageExists(ctx, ref)
	require.NoError(t, err)
	assert.True(t, exists, "imported layout should be present in the daemon as %s", ref)
}

// TestImportImageSkipsAttestationManifest proves ImportImage handles a layout that is not a
// bare single image: buildx attaches a provenance/SBOM attestation manifest by default, so a
// real build's layout often carries two manifests. The image must still import — the
// attestation entry is skipped, not mistaken for a second image.
func TestImportImageSkipsAttestationManifest(t *testing.T) {
	t.Parallel()
	if !dockerAvailable(t) {
		t.Skip("docker daemon not available")
	}
	ctx := context.Background()

	img, err := random.Image(1024, 1)
	require.NoError(t, err)
	attestation, err := random.Image(512, 1) // stand-in for the attestation manifest
	require.NoError(t, err)
	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: img},
		mutate.IndexAddendum{Add: attestation, Descriptor: v1.Descriptor{
			Annotations: map[string]string{"vnd.docker.reference.type": "attestation-manifest"},
		}},
	)
	layoutPath := filepath.Join(t.TempDir(), "layout")
	_, err = layout.Write(layoutPath, idx)
	require.NoError(t, err)

	ref := "pulumi-oci-attestation-test:v0"
	m := NewDockerPodManager("attestation-test")
	t.Cleanup(func() { _, _ = dockerCmd(context.WithoutCancel(ctx), "rmi", "-f", ref) })

	require.NoError(t, m.ImportImage(ctx, layoutPath, ref), "a 2-manifest layout (image + attestation) must import")
	exists, err := m.ImageExists(ctx, ref)
	require.NoError(t, err)
	assert.True(t, exists, "the image manifest should be imported, the attestation skipped")
}

// dockerAvailable reports whether a usable docker daemon is reachable.
func dockerAvailable(t *testing.T) bool {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	return exec.Command("docker", "version").Run() == nil
}
