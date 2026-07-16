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

// Command registry-proxy is the pod's bootstrap registry: one localhost endpoint
// that pretends the "everything is a published image" end state already exists. It
// is a pull-through *router*, dispatching by repository namespace:
//
//   - pulumi-provider-<name> and pulumi/pulumi-provider-<name> — the provider
//     namespace, read-only, synthesized on demand: a manifest request downloads the
//     released plugin binary from get.pulumi.com and wraps it as a minimal image —
//     the binary at /plugin/provider with that as the entrypoint, the exact format
//     the container host already runs (and that run-from-program-image providers
//     extract via CopyFromImage). Nothing is ever pushed here; the proxy conjures
//     the image the moment the daemon pulls. The bare form is the ref the engine
//     computes today; the pulumi/-prefixed form is the same namespace under the
//     org-qualified convention from the publishing design, accepted now so the
//     engine can move to org-qualified refs without a proxy change.
//
//   - every other repository (e.g. <org>/pulumi-provider-<name>) — the local
//     publish target. `docker push localhost:5005/myorg/pulumi-provider-greeting:
//     v0.1.0` lands here and the engine's pull resolves from here, so a
//     publish→consume loop runs hermetically on one machine before any real
//     registry exists. Two backends: by default an embedded, in-memory registry
//     (ggcr's pkg/registry — zero setup, ephemeral); or, with PROXY_UPSTREAM set
//     (e.g. http://registry:5000), a straight reverse proxy to a real registry
//     sidecar, which then owns storage and any persistence.
//
// The proxy is a bootstrap/testing helper, deliberately NOT real infra: no auth, no
// TLS (localhost only), and in embedded mode the store lives and dies with the
// proxy container (the wrapper replaces that container whenever it would run a
// different image, so locally published packages must be re-pushed after — or run a
// sidecar and point PROXY_UPSTREAM at it). The hypothetical end state is providers
// and components published to real registries; the proxy exists so everything
// downstream can act as if that were already true.
//
// This is the "wrap-on-demand belongs in a pull-through proxy, not the CLI" call
// from the design notes, realized: it drops in where the smoke tests stood up a
// registry:2 and hand-wrapped + pushed a provider image, so they no longer
// pre-build anything — and it makes every released provider available as an image
// without re-publishing, which is what gives an oci:// plugin ref something to pull
// across the whole current ecosystem.
//
// It deliberately depends on go-containerregistry (in its own go module, isolated
// from pkg/): ggcr assembles the image and keeps the two digests straight (the
// manifest references layers by their *compressed* digest while the config's
// diff_ids are the *uncompressed* tar digests) — the classic footgun of
// hand-rolling a registry — and its pkg/registry supplies the embedded store's
// entire distribution-API surface.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// wrapSpec parameterizes how a provider binary becomes an image. Today every
// provider takes the default — a scratch image carrying just the binary — which is
// correct for stateless providers (run directly) and for run-from-program-image ones
// (the host extracts /plugin and runs the binary from the *program* image, so the
// provider's own rootfs is never used). The fields are the seam for the exception:
// a provider whose own rootfs needs an ambient toolchain (the docker provider's
// CLI is the motivating case — its tool is the *provider's*, not the program's, so
// baking it into the program image is a leak) would set a richer Base or ExtraFiles
// here. Not wired to any provider yet; it marks where that override lives.
type wrapSpec struct {
	// Base, when set, names a base image whose layers precede the binary layer.
	// Empty means scratch.
	Base string
	// ExtraFiles are additional files (path -> contents) baked alongside the
	// binary, e.g. a CA bundle for providers that call cloud APIs.
	ExtraFiles map[string][]byte
}

type proxy struct {
	arch string

	// backend serves every repository outside the provider namespace — the local
	// publish target. By default it is an embedded in-memory registry (ggcr's
	// pkg/registry), as ephemeral as the proxy's container; with PROXY_UPSTREAM
	// set it is a straight reverse proxy to a real registry sidecar, which then
	// owns storage (and persistence, if it mounts a volume).
	backend http.Handler

	mu                sync.Mutex
	manifests         map[string]*synthesized // keyed by "<repo>:<tag>"
	manifestsByDigest map[string]*synthesized // keyed by manifest digest
	blobs             map[string][]byte       // keyed by blob digest ("sha256:...")
}

type synthesized struct {
	raw       []byte
	digest    string
	mediaType string
}

var (
	manifestRe = regexp.MustCompile(`^/v2/(.+)/manifests/(.+)$`)
	blobRe     = regexp.MustCompile(`^/v2/(.+)/blobs/(sha256:[0-9a-f]+)$`)
	uploadRe   = regexp.MustCompile(`^/v2/(.+)/blobs/uploads(/.*)?$`)
	// A synthesized repository is pulumi-provider-<name>; the tag is v<version>.
	repoRe = regexp.MustCompile(`^pulumi-provider-(.+)$`)
)

// providerRepo reports whether repo belongs to the synthesized provider namespace,
// and if so which released provider it names. Both the bare form the engine
// computes today (pulumi-provider-<name>) and the org-qualified convention form
// (pulumi/pulumi-provider-<name>) land here; any other org's repositories do not.
func providerRepo(repo string) (provider string, ok bool) {
	bare := strings.TrimPrefix(repo, "pulumi/")
	if m := repoRe.FindStringSubmatch(bare); m != nil {
		return m[1], true
	}
	return "", false
}

func newProxy(arch string, backend http.Handler) *proxy {
	return &proxy{
		arch:              arch,
		backend:           backend,
		manifests:         map[string]*synthesized{},
		manifestsByDigest: map[string]*synthesized{},
		blobs:             map[string][]byte{},
	}
}

// upstreamBackend builds the reverse proxy for PROXY_UPSTREAM mode: every
// non-provider request is forwarded verbatim to the sidecar registry at upstream
// (e.g. http://registry:5000). The proxy stores nothing in this mode.
//
// The client's original Host header is deliberately preserved (NOT rewritten to
// the upstream's): registry:2 mints absolute blob-upload Location URLs from the
// Host it sees, and those URLs must route the client back through this proxy —
// rewriting Host sends the docker daemon chasing the sidecar's container-network
// name, which only the proxy can resolve. (Found the hard way: the ggcr test
// registry uses relative Locations and masked this; a real daemon against
// registry:2 did not.)
func upstreamBackend(upstream string) (http.Handler, error) {
	u, err := url.Parse(upstream)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("PROXY_UPSTREAM %q is not a URL like http://registry:5000 (%v)", upstream, err)
	}
	return httputil.NewSingleHostReverseProxy(u), nil
}

func main() {
	addr := envOr("PROXY_ADDR", ":5000")
	arch := envOr("PROXY_TARGET_ARCH", runtime.GOARCH)
	backendDesc := "embedded in-memory registry"
	var backend http.Handler
	if upstream := os.Getenv("PROXY_UPSTREAM"); upstream != "" {
		b, err := upstreamBackend(upstream)
		if err != nil {
			log.Fatalf("registry-proxy: %v", err)
		}
		backend = b
		backendDesc = "reverse proxy to " + upstream
	} else {
		backend = registry.New(registry.Logger(log.Default()))
	}
	p := newProxy(arch, backend)
	log.Printf(
		"registry-proxy: serving on %s — synthesizing linux/%s provider images from get.pulumi.com "+
			"(pulumi-provider-*, pulumi/pulumi-provider-*); %s for everything else",
		addr, arch, backendDesc)
	if err := http.ListenAndServe(addr, p); err != nil { //nolint:gosec // dev tool, no timeouts needed
		log.Fatalf("registry-proxy: %v", err)
	}
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/v2/" || r.URL.Path == "/v2":
		// The distribution API version check, answered here for both namespaces.
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		w.WriteHeader(http.StatusOK)
	case uploadRe.MatchString(r.URL.Path):
		// Blob uploads exist only in the embedded store. Reject pushes into the
		// provider namespace up front (before any blob lands), so a `docker push`
		// of a synthesized name fails at its first request rather than after
		// uploading layers that the manifest PUT would then orphan.
		m := uploadRe.FindStringSubmatch(r.URL.Path)
		if _, ok := providerRepo(m[1]); ok {
			http.Error(w, "the provider namespace is read-only (images are synthesized from released binaries)",
				http.StatusMethodNotAllowed)
			return
		}
		p.backend.ServeHTTP(w, r)
	case manifestRe.MatchString(r.URL.Path):
		m := manifestRe.FindStringSubmatch(r.URL.Path)
		if provider, ok := providerRepo(m[1]); ok {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				http.Error(w, "the provider namespace is read-only (images are synthesized from released binaries)",
					http.StatusMethodNotAllowed)
				return
			}
			p.serveManifest(w, r, m[1], provider, m[2])
			return
		}
		p.backend.ServeHTTP(w, r)
	case blobRe.MatchString(r.URL.Path):
		m := blobRe.FindStringSubmatch(r.URL.Path)
		if _, ok := providerRepo(m[1]); ok {
			p.serveBlob(w, r, m[2])
			return
		}
		p.backend.ServeHTTP(w, r)
	default:
		// Everything else — tag lists, catalog, referrers — belongs to the
		// embedded registry.
		p.backend.ServeHTTP(w, r)
	}
}

func (p *proxy) serveManifest(w http.ResponseWriter, r *http.Request, repo, provider, ref string) {
	var s *synthesized
	if strings.HasPrefix(ref, "sha256:") {
		// A by-digest re-fetch: the daemon learned this digest from the tag
		// manifest and is verifying it. Serve the already-synthesized one verbatim;
		// re-synthesizing would (correctly) refuse to treat a digest as a version.
		p.mu.Lock()
		s = p.manifestsByDigest[ref]
		p.mu.Unlock()
		if s == nil {
			http.NotFound(w, r)
			return
		}
	} else {
		var err error
		s, err = p.synthesize(repo, provider, ref)
		if err != nil {
			log.Printf("registry-proxy: synthesize %s:%s failed: %v", repo, ref, err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
	}
	w.Header().Set("Content-Type", s.mediaType)
	w.Header().Set("Docker-Content-Digest", s.digest)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(s.raw)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(s.raw)
}

func (p *proxy) serveBlob(w http.ResponseWriter, r *http.Request, digest string) {
	p.mu.Lock()
	b, ok := p.blobs[digest]
	p.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(b)
}

// synthesize builds (and caches) the image for one provider reference, registering
// its config and layer blobs by digest so a subsequent /blobs/<digest> request is
// served the exact bytes the manifest points at. The cache is keyed by the repo as
// requested, so the bare and pulumi/-qualified spellings of one provider cache
// separately — harmless, since the build is deterministic and both yield the same
// digests.
func (p *proxy) synthesize(repo, provider, ref string) (*synthesized, error) {
	key := repo + ":" + ref
	p.mu.Lock()
	if s, ok := p.manifests[key]; ok {
		p.mu.Unlock()
		return s, nil
	}
	p.mu.Unlock()

	version := strings.TrimPrefix(ref, "v")

	log.Printf("registry-proxy: synthesizing %s:%s (provider=%s version=%s arch=%s)", repo, ref, provider, version, p.arch)
	img, err := p.buildImage(provider, version, wrapSpec{})
	if err != nil {
		return nil, err
	}

	raw, err := img.RawManifest()
	if err != nil {
		return nil, err
	}
	mt, err := img.MediaType()
	if err != nil {
		return nil, err
	}
	dig, err := img.Digest()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// Register the config blob.
	cfgName, err := img.ConfigName()
	if err != nil {
		return nil, err
	}
	rawCfg, err := img.RawConfigFile()
	if err != nil {
		return nil, err
	}
	p.blobs[cfgName.String()] = rawCfg
	// Register each layer blob by its *compressed* digest (what the manifest cites).
	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}
	for _, l := range layers {
		ld, err := l.Digest()
		if err != nil {
			return nil, err
		}
		rc, err := l.Compressed()
		if err != nil {
			return nil, err
		}
		lb, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		p.blobs[ld.String()] = lb
	}
	s := &synthesized{raw: raw, digest: dig.String(), mediaType: string(mt)}
	p.manifests[key] = s
	p.manifestsByDigest[s.digest] = s
	return s, nil
}

// buildImage downloads the released provider binary and wraps it per spec.
func (p *proxy) buildImage(provider, version string, spec wrapSpec) (v1.Image, error) {
	bin, err := fetchProviderBinary(provider, version, p.arch)
	if err != nil {
		return nil, err
	}

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	if err := tw.WriteHeader(&tar.Header{Name: "plugin/", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
		return nil, err
	}
	// A scratch image has none of the standard writable scratch dirs that tools assume
	// exist. Most providers don't care, but a provider with an embedded builder — notably
	// docker-build's buildkit client — fails to boot ("stat /tmp: no such file or
	// directory") without one. Bake empty /tmp and /var/tmp (sticky, world-writable, like
	// a real filesystem), disk-backed via the container's writable layer so a large build
	// has room — unlike a RAM tmpfs.
	scratchDirs := []struct {
		name string
		mode int64
	}{
		{"tmp/", 0o1777},
		{"var/", 0o755},
		{"var/tmp/", 0o1777},
	}
	for _, d := range scratchDirs {
		if err := tw.WriteHeader(&tar.Header{Name: d.name, Typeflag: tar.TypeDir, Mode: d.mode}); err != nil {
			return nil, err
		}
	}
	// The binary lives at /plugin/provider — a directory — so one image serves both
	// archetypes: stateless providers run it as the entrypoint; run-from-program-image
	// ones have the host CopyFromImage /plugin into a volume and run it from the
	// program image. Mirrors smoketest/Dockerfile.provider.
	if err := writeTarFile(tw, "plugin/provider", bin, 0o755); err != nil {
		return nil, err
	}
	// Bake the system CA bundle so providers that call cloud HTTPS APIs (cloudflare,
	// aws, ...) can verify certificates. A scratch image has no trust store, so a
	// stateless provider running from its own image fails TLS with "certificate signed
	// by unknown authority". Copy the proxy's own bundle to Go's default Linux path.
	if ca := caBundle(); ca != nil {
		if err := writeTarFile(tw, "etc/ssl/certs/ca-certificates.crt", ca, 0o644); err != nil {
			return nil, err
		}
	}
	for path, contents := range spec.ExtraFiles {
		if err := writeTarFile(tw, strings.TrimPrefix(path, "/"), contents, 0o644); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	layer, err := tarball.LayerFromReader(&tarBuf)
	if err != nil {
		return nil, err
	}
	// spec.Base (scratch by default) would be applied here for the override case.
	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return nil, err
	}
	cf, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}
	cf = cf.DeepCopy()
	cf.OS = "linux"
	cf.Architecture = p.arch
	cf.Config.Entrypoint = []string{"/plugin/provider"}
	return mutate.ConfigFile(img, cf)
}

// caBundle returns the system CA certificate bundle from the proxy's own trust store, or
// nil if none is found. The proxy container installs ca-certificates, so this copies that
// host trust store into otherwise-bare (scratch) provider images — enough for a stateless
// provider that runs from its own image to verify cloud HTTPS endpoints. (A workspace-
// coupled provider synthesized fresh would also want a shell/cp for CopyFromImage, i.e. a
// real base like alpine — not needed by the stateless providers that hit this path today.)
func caBundle() []byte {
	paths := []string{}
	if f := os.Getenv("SSL_CERT_FILE"); f != "" {
		paths = append(paths, f)
	}
	paths = append(paths,
		"/etc/ssl/certs/ca-certificates.crt", // Debian/Alpine (with ca-certificates)
		"/etc/ssl/cert.pem",                  // Alpine/BSD
		"/etc/pki/tls/certs/ca-bundle.crt",   // RHEL/Fedora
	)
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			return b
		}
	}
	return nil
}

// fetchProviderBinary downloads a released provider plugin tarball and returns the
// pulumi-resource-<provider> binary from it.
func fetchProviderBinary(provider, version, arch string) ([]byte, error) {
	url := fmt.Sprintf(
		"https://get.pulumi.com/releases/plugins/pulumi-resource-%s-v%s-linux-%s.tar.gz",
		provider, version, arch)
	resp, err := http.Get(url) //nolint:gosec,noctx // dev tool; URL is a fixed get.pulumi.com convention
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: %s", url, resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gunzip %s: %w", url, err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	want := "pulumi-resource-" + provider
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", url, err)
		}
		if hdr.Name == want || strings.HasSuffix(hdr.Name, "/"+want) {
			b, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("reading %s from %s: %w", want, url, err)
			}
			return b, nil
		}
	}
	return nil, fmt.Errorf("%s not found in %s", want, url)
}

func writeTarFile(tw *tar.Writer, name string, contents []byte, mode int64) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(contents))}); err != nil {
		return err
	}
	_, err := tw.Write(contents)
	return err
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
