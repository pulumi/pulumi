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
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCriImportImagePushesThenPulls proves the CRI build-contract sink's proxy-pull path: a
// runtime-neutral OCI layout is pushed to ref's registry in-process, then pulled back through the
// CRI image service. Unlike the docker/nerdctl sinks — which ingest straight into a local store —
// CRI has no in-process content-store verb, so this is the one sink that goes out to a registry and
// back. It runs against a real in-memory registry (ggcr's pkg/registry over httptest, which listens
// on 127.0.0.1 — an address ggcr pushes to over plain HTTP), with a fake CRI standing in for the
// image service so the push and the PullImage handoff are both asserted without a live containerd.
func TestCriImportImagePushesThenPulls(t *testing.T) {
	t.Parallel()

	// The "proxy": an in-memory registry the engine pushes to and CRI would pull from. httptest
	// binds 127.0.0.1, which ggcr's scheme detection treats as plaintext HTTP — the same class as
	// the bootstrap proxy a pod setup runs (a private-IP / loopback host, not a TLS registry).
	reg := httptest.NewServer(registry.New())
	t.Cleanup(reg.Close)
	regHost := strings.TrimPrefix(reg.URL, "http://")

	// A location-free layout, as any decoupled builder emits: one single-layer image under a
	// one-manifest index, hermetic (no builder, no network).
	img, err := random.Image(1024, 1)
	require.NoError(t, err)
	layoutPath := filepath.Join(t.TempDir(), "layout")
	_, err = layout.Write(layoutPath, mutate.AppendManifests(empty.Index, mutate.IndexAddendum{Add: img}))
	require.NoError(t, err)

	// The ref the orchestrator resolved — where the image should live. It carries the proxy host
	// (load-bearing on CRI: the push and the pull both dial it).
	ref := regHost + "/pulumiorg/pulumi-policy-greeting:v0.1.0"

	fake := &fakeCRI{}
	m := newFakeCriManager(t, fake, "p1")
	require.NoError(t, m.ImportImage(t.Context(), layoutPath, ref))

	// The push landed: the exact layout image is now servable from the registry under ref.
	pushed, err := remote.Image(mustParseRef(t, ref))
	require.NoError(t, err, "the image must be pullable from the registry after ImportImage")
	assertSameDigest(t, img, pushed)

	// And the sink handed the ref to the CRI image service — the step that lands it in the k8s.io
	// namespace CRI-run containers see. Namespace-correctness comes from reusing PullImage, so the
	// assertion is simply that the pull was issued for exactly ref.
	assert.Equal(t, []string{ref}, fake.pulled, "ImportImage must pull ref back through the CRI image service")
}

// TestCriImportImageSkipsAttestationManifest proves the CRI sink shares the docker/nerdctl layout
// selection: a real build's layout carries a buildx attestation manifest alongside the image, and
// only the runnable image is pushed — the attestation entry is skipped, not mistaken for a second
// image (nor pushed as junk).
func TestCriImportImageSkipsAttestationManifest(t *testing.T) {
	t.Parallel()

	reg := httptest.NewServer(registry.New())
	t.Cleanup(reg.Close)
	regHost := strings.TrimPrefix(reg.URL, "http://")

	img, err := random.Image(1024, 1)
	require.NoError(t, err)
	attestation, err := random.Image(512, 1) // stand-in for the buildx attestation manifest
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

	ref := regHost + "/pulumiorg/pulumi-policy-greeting:v0.1.0"
	fake := &fakeCRI{}
	m := newFakeCriManager(t, fake, "p1")
	require.NoError(t, m.ImportImage(t.Context(), layoutPath, ref), "a 2-manifest layout must import")

	pushed, err := remote.Image(mustParseRef(t, ref))
	require.NoError(t, err)
	assertSameDigest(t, img, pushed) // the image manifest, not the attestation, is what landed
	assert.Equal(t, []string{ref}, fake.pulled)
}

// TestCriImportImageRejectsBadRef proves a malformed ref fails at parse, before any push or pull is
// attempted — the sink does not silently swallow a ref it cannot resolve.
func TestCriImportImageRejectsBadRef(t *testing.T) {
	t.Parallel()

	img, err := random.Image(1024, 1)
	require.NoError(t, err)
	layoutPath := filepath.Join(t.TempDir(), "layout")
	_, err = layout.Write(layoutPath, mutate.AppendManifests(empty.Index, mutate.IndexAddendum{Add: img}))
	require.NoError(t, err)

	fake := &fakeCRI{}
	m := newFakeCriManager(t, fake, "p1")
	err = m.ImportImage(t.Context(), layoutPath, "NOT A REF")
	require.Error(t, err)
	assert.Empty(t, fake.pulled, "a ref that fails to parse must not reach the CRI image service")
}

func mustParseRef(t *testing.T, ref string) name.Reference {
	t.Helper()
	r, err := name.ParseReference(ref)
	require.NoError(t, err)
	return r
}

func assertSameDigest(t *testing.T, want, got v1.Image) {
	t.Helper()
	wantDig, err := want.Digest()
	require.NoError(t, err)
	gotDig, err := got.Digest()
	require.NoError(t, err)
	assert.Equal(t, wantDig, gotDig, "the pushed image digest must match the layout's image")
}
