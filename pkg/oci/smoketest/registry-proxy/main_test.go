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

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// newEmbeddedProxy builds a proxy in its default configuration: the embedded
// in-memory registry as the backend.
func newEmbeddedProxy() *proxy {
	return newProxy("arm64", registry.New())
}

// TestProviderRepo pins the routing rule: which repositories are the synthesized
// provider namespace (bare and pulumi/-qualified) and which fall through to the
// embedded registry (every other org).
func TestProviderRepo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		repo     string
		provider string
		ok       bool
	}{
		{"pulumi-provider-random", "random", true},
		{"pulumi/pulumi-provider-random", "random", true},
		{"pulumi-provider-docker-build", "docker-build", true},
		// Other orgs are the embedded registry's, even with the convention leaf.
		{"myorg/pulumi-provider-greeting", "", false},
		{"pulumi-labs/pulumi-provider-greeting", "", false},
		// Nested paths under pulumi/ are not the provider namespace.
		{"pulumi/nested/pulumi-provider-x", "", false},
		// Unrelated repositories.
		{"pulumi/templates", "", false},
		{"alpine", "", false},
	}
	for _, c := range cases {
		provider, ok := providerRepo(c.repo)
		if provider != c.provider || ok != c.ok {
			t.Errorf("providerRepo(%q) = (%q, %v), want (%q, %v)", c.repo, provider, ok, c.provider, c.ok)
		}
	}
}

// TestEmbeddedPushPullRoundTrip drives the embedded registry through the proxy's
// front door with a real distribution-API client (ggcr remote — the same protocol
// the docker daemon speaks): push an image to an org-namespaced repository, pull it
// back, and compare digests. This is the local publish target working end to end.
func TestEmbeddedPushPullRoundTrip(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newEmbeddedProxy())
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}
	wantDigest, err := img.Digest()
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	tag, err := name.NewTag(host + "/myorg/pulumi-provider-greeting:v0.1.0")
	if err != nil {
		t.Fatalf("parse tag: %v", err)
	}
	if err := remote.Write(tag, img); err != nil {
		t.Fatalf("push to embedded registry: %v", err)
	}

	got, err := remote.Image(tag)
	if err != nil {
		t.Fatalf("pull from embedded registry: %v", err)
	}
	gotDigest, err := got.Digest()
	if err != nil {
		t.Fatalf("pulled digest: %v", err)
	}
	if gotDigest != wantDigest {
		t.Fatalf("round trip changed the image: pushed %s, pulled %s", wantDigest, gotDigest)
	}
}

// TestProviderNamespaceReadOnly pins that pushes into the synthesized namespace are
// rejected at the first request (the blob-upload POST), for both spellings.
func TestProviderNamespaceReadOnly(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newEmbeddedProxy())
	defer srv.Close()

	for _, repo := range []string{"pulumi-provider-random", "pulumi/pulumi-provider-random"} {
		resp, err := http.Post(srv.URL+"/v2/"+repo+"/blobs/uploads/", "application/octet-stream", nil)
		if err != nil {
			t.Fatalf("POST upload to %s: %v", repo, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST upload to %s: got %d, want %d", repo, resp.StatusCode, http.StatusMethodNotAllowed)
		}

		req, err := http.NewRequest(http.MethodPut, srv.URL+"/v2/"+repo+"/manifests/v1.0.0", strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("build manifest PUT: %v", err)
		}
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT manifest to %s: %v", repo, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("PUT manifest to %s: got %d, want %d", repo, resp.StatusCode, http.StatusMethodNotAllowed)
		}
	}
}

// TestUpstreamBackendRoundTrip exercises PROXY_UPSTREAM mode: the backend is a
// reverse proxy to a sidecar registry (played here by a second server), the proxy
// stores nothing, and the pushed image is really in the sidecar — proven by pulling
// it back through the sidecar directly, bypassing the proxy.
func TestUpstreamBackendRoundTrip(t *testing.T) {
	t.Parallel()
	sidecar := httptest.NewServer(registry.New())
	defer sidecar.Close()
	backend, err := upstreamBackend(sidecar.URL)
	if err != nil {
		t.Fatalf("upstreamBackend(%q): %v", sidecar.URL, err)
	}
	srv := httptest.NewServer(newProxy("arm64", backend))
	defer srv.Close()

	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}
	wantDigest, err := img.Digest()
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	proxyHost := strings.TrimPrefix(srv.URL, "http://")
	tag, err := name.NewTag(proxyHost + "/myorg/pulumi-provider-greeting:v0.1.0")
	if err != nil {
		t.Fatalf("parse tag: %v", err)
	}
	if err := remote.Write(tag, img); err != nil {
		t.Fatalf("push through proxy to sidecar: %v", err)
	}

	// Read it back from the sidecar directly: the bytes must live there, not in
	// the proxy.
	sidecarHost := strings.TrimPrefix(sidecar.URL, "http://")
	direct, err := name.NewTag(sidecarHost + "/myorg/pulumi-provider-greeting:v0.1.0")
	if err != nil {
		t.Fatalf("parse sidecar tag: %v", err)
	}
	got, err := remote.Image(direct)
	if err != nil {
		t.Fatalf("pull from sidecar directly: %v", err)
	}
	gotDigest, err := got.Digest()
	if err != nil {
		t.Fatalf("pulled digest: %v", err)
	}
	if gotDigest != wantDigest {
		t.Fatalf("push through proxy did not land in sidecar intact: pushed %s, sidecar has %s", wantDigest, gotDigest)
	}
}

// TestPing pins the distribution version check both clients start with.
func TestPing(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(newEmbeddedProxy())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v2/")
	if err != nil {
		t.Fatalf("GET /v2/: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v2/: got %d, want 200", resp.StatusCode)
	}
}
