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
	"fmt"
	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// nerdctlPodManager is a PodManager backed by containerd, driven through the `nerdctl` CLI.
// It is the second runtime the pod model runs on, and its whole purpose is to prove the
// PodManager contract is genuinely runtime-agnostic: containerd never touches the docker
// socket, so anything that still assumed docker below the seam is forced into the open here.
//
// What the spike learned (2026-07-18, findings in scratch): nerdctl's docker-CLI fidelity is
// high enough that every verb that reaches the runtime *through the CLI* is argv-identical to
// the docker manager — create-time volume copy-up (CopyFromImage), `cp` from a non-running
// container to stdout (ReadImageFile), `--volumes-from`, and `--network container:<peer>` netns
// join were all probed against a live containerd and behave the same. So those verbs are
// inherited from dockerPodManager with the binary swapped to `nerdctl`; this reuse *encodes the
// finding* rather than assuming it.
//
// The reuse is verified to two different depths, worth being precise about. Verbs routed through
// the injectable runner (CreateNetwork, RunContainer, WaitContainer, PullImage, ImageExists,
// StopContainer, Cleanup) are exercised live on containerd — RunContainer/WaitContainer include
// the two *output parses* the argv probe could not reach (the container id from `run -d`, the
// exit code from `wait`). But RunToCompletion and ContainerLogs *stream*, so they exec the binary
// directly rather than through the runner seam; their argv is shared, but their containerd output
// handling is not yet exercised (this transport cannot reach them — it needs a host with nerdctl
// installed, i.e. full-pod-up). Named here so the gap is not mistaken for coverage.
//
// The one genuine divergence is ImportImage. The docker manager loads a layout in-process via
// go-containerregistry's `daemon.Write`, which speaks the docker API over a socket that does
// not exist on a containerd host. That is the build contract's forcing verb by design (a
// decoupled build emits a runtime-neutral layout; the sink that re-couples it to a runtime is
// the one place a runtime dependency belongs), so it is overridden below to load into
// containerd's content store instead.
type nerdctlPodManager struct {
	// Embed the docker manager configured with bin="nerdctl". Method promotion gives us every
	// CLI-compatible verb for free; nerdctlPodManager only declares the verbs that diverge, and
	// those shadow the promoted ones.
	*dockerPodManager
}

// NewNerdctlPodManager returns a PodManager that drives containerd through `nerdctl`. It takes
// the same Options as NewDockerPodManager (WithDockerBinary to override the binary — e.g. an
// absolute path or a wrapper that reaches a remote containerd — and withRunner for tests).
func NewNerdctlPodManager(podID string, opts ...Option) PodManager {
	inner := &dockerPodManager{
		bin:   "nerdctl",
		podID: podID,
		run:   execRunner,
	}
	for _, o := range opts {
		o(inner)
	}
	return &nerdctlPodManager{dockerPodManager: inner}
}

// containerdNamespace is the containerd namespace the pod's images and containers live in. It
// matters because content is namespace-scoped in containerd (unlike a docker daemon's single
// global image store): `ctr images import` here and the inherited `nerdctl` run verbs must name
// the SAME namespace or a just-imported image is invisible to the container that needs it.
// `default` is what bare `nerdctl` uses, so aligning the import to it keeps the two in sync.
const containerdNamespace = "default"

// ImportImage loads a runtime-neutral OCI image layout into containerd's content store under
// ref — the containerd analogue of dockerPodManager.ImportImage, and the verb that could not be
// inherited. It cannot reuse `daemon.Write` (the docker API is absent on a containerd host), so
// it takes the CLI path: serialize the image to a docker-archive tar and stream it to
// `ctr images import` over stdin.
//
// Three findings shaped this (spike 2026-07-18):
//   - Two binaries, not one. `nerdctl load` was the obvious first try, but on containerd 2.x /
//     nerdctl 2.3.4 it fails its eager-unpack init ("no unpack platforms defined") even for a
//     plain `docker save` archive — while `pull` and `run` unpack the same images fine.
//     `ctr images import` is the lower-level primitive (the one the design doc named). So the
//     containerd manager drives TWO binaries — `nerdctl` for the high-level run/network/volume
//     verbs, `ctr` for the content-store import — where docker needed only one.
//   - Import and unpack are separate operations on containerd. Docker's `load` conflates
//     ingesting content with unpacking it into a snapshotter; containerd splits them. ImportImage
//     does only the content-store import (`--no-unpack`); the first `nerdctl run` unpacks lazily,
//     when the snapshotter is actually chosen. This side-steps the eager-unpack platform-matcher
//     issue entirely and is the cleaner division of labor — the import commits to no snapshotter.
//   - Naming. The build's layout is deliberately location-free (no org.opencontainers.image.
//     ref.name), so importing it raw would land the image unnamed. Serializing through
//     go-containerregistry's tarball writer (the `docker save` format) embeds ref as the
//     archive's RepoTag, which `ctr images import` reads to name the image — so it lands already
//     named as the ref the orchestrator resolved, no separate tag step. The host-arch image is
//     selected from the layout exactly as the docker sink does (shared imageFromLayout).
func (m *nerdctlPodManager) ImportImage(ctx context.Context, layoutPath, ref string) error {
	img, err := imageFromLayout(layoutPath)
	if err != nil {
		return err
	}
	tag, err := name.NewTag(ref)
	if err != nil {
		return fmt.Errorf("oci: parsing target ref %q: %w", ref, err)
	}

	// Serialize the image to a docker-archive tar. A temp file (rather than an in-memory pipe)
	// keeps the import a single synchronous command through the same commandRunner seam the other
	// verbs use — no goroutine coordinating a writer against the CLI's stdin reader.
	tarFile, err := os.CreateTemp("", "pulumi-oci-load-*.tar")
	if err != nil {
		return fmt.Errorf("oci: creating temp archive for %s: %w", ref, err)
	}
	defer func() {
		_ = tarFile.Close()
		_ = os.Remove(tarFile.Name())
	}()
	if err := tarball.Write(tag, img, tarFile); err != nil {
		return fmt.Errorf("oci: writing image archive for %s: %w", ref, err)
	}
	if _, err := tarFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("oci: rewinding image archive for %s: %w", ref, err)
	}

	// `ctr images import -` ingests a docker/OCI archive from stdin into containerd's content
	// store — the sink `docker load` fills for the daemon, but on a store that has no docker
	// socket at all. Scoped to containerdNamespace so RunContainer (nerdctl, same namespace) sees
	// it. --no-unpack keeps this to a pure content ingest: unpacking is deferred to the first run,
	// which is where the snapshotter is chosen (and avoids the eager-unpack platform matcher).
	if _, stderr, err := m.run(ctx, tarFile,
		"ctr", "-n", containerdNamespace, "images", "import", "--no-unpack", "-"); err != nil {
		return fmt.Errorf("oci: ctr images import %s: %w: %s", ref, err, stderr)
	}
	return nil
}
