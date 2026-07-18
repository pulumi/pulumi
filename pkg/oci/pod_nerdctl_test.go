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
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nerdctlEnv returns a commandRunner that drives the containerd/nerdctl runtime living inside a
// privileged helper container (the spike env — see scratch/2026-07-18_containerd-podmanager),
// plus that container's name, or skips if it is not reachable. The container name is taken from
// PULUMI_OCI_NERDCTL_ENV (default "nerdenv"); the runner shells `docker exec -i <c> <cmd>`, so
// docker is only the transport that reaches the containerd host — the image operations
// themselves are containerd's, never the docker daemon's. CONTAINERD_SNAPSHOTTER=native is set
// because containerd's default overlayfs snapshotter cannot mount nested on Docker Desktop's own
// overlayfs rootfs (an env fact of running containerd-in-docker, not a property of the code).
func nerdctlEnv(t *testing.T) (commandRunner, string) {
	t.Helper()
	container := os.Getenv("PULUMI_OCI_NERDCTL_ENV")
	if container == "" {
		container = "nerdenv"
	}
	if exec.Command("docker", "exec", container, "nerdctl", "version").Run() != nil {
		t.Skipf("containerd/nerdctl spike env %q not reachable "+
			"(set up per scratch/2026-07-18_containerd-podmanager, or set PULUMI_OCI_NERDCTL_ENV)", container)
	}
	runner := func(ctx context.Context, stdin io.Reader, name string, args ...string) (string, string, error) {
		full := append([]string{"exec", "-e", "CONTAINERD_SNAPSHOTTER=native", "-i", container, name}, args...)
		cmd := exec.CommandContext(ctx, "docker", full...)
		cmd.Stdin = stdin
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		err := cmd.Run()
		return out.String(), errOut.String(), err
	}
	return runner, container
}

// TestNewPodManagerSelectsRuntime pins the selection seam: the default (and an explicit
// "docker") yields the docker manager — so the everyday path is byte-for-byte unchanged — while
// "containerd"/"nerdctl" (case- and space-insensitive) yields the containerd manager.
func TestNewPodManagerSelectsRuntime(t *testing.T) {
	cases := []struct {
		env         string
		wantNerdctl bool
	}{
		{"", false},
		{"docker", false},
		{"anything-else", false},
		{"containerd", true},
		{"nerdctl", true},
		{"  NerdCtl  ", true},
	}
	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			t.Setenv(PodRuntimeEnvVar, c.env)
			_, isNerdctl := NewPodManager("p").(*nerdctlPodManager)
			assert.Equal(t, c.wantNerdctl, isNerdctl, "runtime %q selected the wrong manager", c.env)
		})
	}
}

// TestNerdctlManagerDrivesNerdctlBinary proves the binary swap is wired: an inherited verb
// (CreateNetwork, unchanged from the docker manager) builds the same argv but runs it against
// `nerdctl`, not `docker`. This locks in the probe finding that the run/network/volume verbs are
// argv-identical — without needing a live daemon.
func TestNerdctlManagerDrivesNerdctlBinary(t *testing.T) {
	t.Parallel()
	var gotBin string
	var gotArgs []string
	runner := func(_ context.Context, _ io.Reader, name string, args ...string) (string, string, error) {
		gotBin, gotArgs = name, args
		return "net-id", "", nil
	}
	pm := NewNerdctlPodManager("p1", withRunner(runner))

	_, err := pm.CreateNetwork(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "nerdctl", gotBin, "inherited verbs must drive the nerdctl binary, not docker")
	assert.Equal(t,
		[]string{"network", "create", "--label", "com.pulumi.pod=p1", "pulumi-pod-p1-net"},
		gotArgs, "the inherited argv should match the docker manager's")
}

// TestNerdctlImportImageLoadsLayout is the forcing-verb proof: a runtime-neutral OCI layout —
// the exact artifact the build contract emits, naming no runtime and no location — is imported
// into containerd's content store under the ref the orchestrator supplies, on a runtime that
// has no docker socket. This is the one verb that could not be inherited from the docker
// manager (its docker `daemon.Write` API is absent here), so a live containerd is what proves
// the layout → ctr import → named-in-store path actually works. It is deliberately isolated
// from `package build`, which also drags in the out-of-scope `--volumes-from` build coupling.
func TestNerdctlImportImageLoadsLayout(t *testing.T) {
	t.Parallel()
	run, _ := nerdctlEnv(t)
	ctx := t.Context()

	// A hermetic OCI layout DIRECTORY (same digest-addressed shape kaniko's --oci-layout-path
	// emits), with an attestation manifest alongside the image so we also cover the "layout is
	// not a bare single image" case the shared imageFromLayout handles.
	baseImg, err := random.Image(1024, 1)
	require.NoError(t, err)
	// `ctr images import` applies a platform-match filter (default: the containerd host's
	// platform), so an image whose config declares no platform is filtered out entirely
	// ("image might be filtered out"). Real build artifacts always carry a platform; give the
	// hermetic fixture the containerd host's (linux/<arch>) so it survives the filter.
	cf, err := baseImg.ConfigFile()
	require.NoError(t, err)
	cf = cf.DeepCopy()
	cf.OS, cf.Architecture = "linux", runtime.GOARCH
	img, err := mutate.ConfigFile(baseImg, cf)
	require.NoError(t, err)
	attestation, err := random.Image(512, 1)
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

	ref := "pulumi-oci-nerdctl-import-test:v0"
	m := NewNerdctlPodManager("import-test", withRunner(run))
	t.Cleanup(func() { _, _, _ = run(context.WithoutCancel(ctx), nil, "nerdctl", "rmi", "-f", ref) })

	require.NoError(t, m.ImportImage(ctx, layoutPath, ref),
		"a runtime-neutral layout must import into containerd via ctr import")

	// The image is present in containerd's store under exactly the ref we asked for — the
	// location-free layout was named at the sink by the docker-archive RepoTag.
	exists, err := m.ImageExists(ctx, ref)
	require.NoError(t, err)
	assert.True(t, exists, "imported layout should be present in containerd as %s", ref)

	// The decoupling proof: the image landed in containerd, NOT in the host docker daemon.
	// (A bare tag like this would only resolve if something had loaded it into docker's store.)
	assert.Error(t, exec.Command("docker", "image", "inspect", ref).Run(),
		"the image must NOT be in the host docker daemon — the whole point is containerd's store")
}

// TestNerdctlRunContainerWaitExit exercises the runner-routed run path live on containerd. Where
// the argv probe proved containerd accepts the same flags, this proves the manager's two output
// parses hold on containerd too: RunContainer reads `run -d` stdout as the container id, and
// WaitContainer reads `wait` stdout as an integer exit code. A container that exits 7 is a marker
// a broken parse cannot fake. It also drives PullImage, so five verbs (Pull/Run/Wait/Stop via
// Cleanup) are covered live, not just inferred from shared argv.
//
// Two verbs stay unreachable by this transport and so untested on containerd: RunToCompletion and
// ContainerLogs stream, so they exec the binary directly rather than through the injectable
// runner — on this host they would exec a local `nerdctl` that isn't installed. Validating their
// output handling needs a real containerd-on-host (or a streaming-aware runner), which is
// next-session with full-pod-up.
func TestNerdctlRunContainerWaitExit(t *testing.T) {
	t.Parallel()
	run, _ := nerdctlEnv(t)
	ctx := t.Context()

	// A fully-qualified ref (no bare-name normalization ambiguity between pull and run). Pull
	// first so lazy unpack/pull progress never pollutes the `run -d` stdout the id parse reads.
	const image = "docker.io/library/alpine:3.20"
	m := NewNerdctlPodManager("run-test", withRunner(run))
	t.Cleanup(func() { _ = m.Cleanup(context.WithoutCancel(ctx)) })
	require.NoError(t, m.PullImage(ctx, image))

	// Empty Network omits the flag → nerdctl's default bridge (nerdctl-full ships CNI); this
	// deliberately avoids the pod netns/DNS path, which is the deferred hard wall.
	c, err := m.RunContainer(ctx, ContainerConfig{
		Image:      image,
		Name:       "exit7",
		Entrypoint: []string{"sh", "-c"},
		Cmd:        []string{"exit 7"},
	})
	require.NoError(t, err, "RunContainer must start a detached container and parse its id")
	assert.NotEmpty(t, c.ID, "the container id parsed from `run -d` stdout must be non-empty")

	code, err := m.WaitContainer(ctx, c)
	require.NoError(t, err)
	assert.Equal(t, 7, code, "WaitContainer must parse the container's real exit code from containerd")
}
