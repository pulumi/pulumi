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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner records every command issued and returns canned output, so the unit
// tests can assert the exact argv the Docker implementation builds without a
// running daemon.
type fakeRunner struct {
	mu      sync.Mutex
	calls   [][]string
	respond func(args []string) (stdout, stderr string, err error)
}

func (f *fakeRunner) run(_ context.Context, _ io.Reader, _ string, args ...string) (string, string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, append([]string(nil), args...))
	f.mu.Unlock()
	if f.respond != nil {
		return f.respond(args)
	}
	return "", "", nil
}

func TestRunContainerArgs(t *testing.T) {
	t.Parallel()
	fake := &fakeRunner{respond: func([]string) (string, string, error) { return "container-abc", "", nil }}
	pm := NewDockerPodManager("p1", withRunner(fake.run))

	c, err := pm.RunContainer(t.Context(), ContainerConfig{
		Image:   "pulumi/python:3.12",
		Name:    "program",
		Network: "pulumi-pod-p1-net",
		// Every long-lived pod member pins this, and it must beat the image's own
		// WORKDIR: a relative asset path only resolves to the same bytes on both sides
		// if writer and reader share a working directory.
		WorkingDir: WorkspaceMountPath,
		// Intentionally out of alphabetical order to prove the argv is sorted.
		Env:        map[string]string{"PULUMI_STACK": "dev", "PULUMI_PROJECT": "demo"},
		Volumes:    []VolumeMount{{Source: "pulumi-pod-p1-vol-workspace", Target: "/app", ReadOnly: true}},
		Privileged: true,
		Entrypoint: []string{"python", "-u"},
		Cmd:        []string{"__main__.py"},
	})
	require.NoError(t, err)
	assert.Equal(t, "container-abc", c.ID)
	assert.Equal(t, "pulumi-pod-p1-program", c.Name)

	require.Len(t, fake.calls, 1)
	want := []string{
		"run", "-d", "--name", "pulumi-pod-p1-program", "--label", "com.pulumi.pod=p1",
		"--network", "pulumi-pod-p1-net",
		"--privileged",
		"-w", "/workspace",
		"-e", "PULUMI_PROJECT=demo", "-e", "PULUMI_STACK=dev", // sorted by key
		"-v", "pulumi-pod-p1-vol-workspace:/app:ro",
		"--entrypoint", "python",
		"pulumi/python:3.12",
		"-u", "__main__.py", // remaining entrypoint token, then Cmd
	}
	assert.Equal(t, want, fake.calls[0])
}

func TestRunContainerNoEntrypoint(t *testing.T) {
	t.Parallel()
	fake := &fakeRunner{respond: func([]string) (string, string, error) { return "cid", "", nil }}
	pm := NewDockerPodManager("p1", withRunner(fake.run))

	_, err := pm.RunContainer(t.Context(), ContainerConfig{
		Image: "alpine:3",
		Name:  "x",
		Cmd:   []string{"echo", "hi"},
	})
	require.NoError(t, err)
	want := []string{
		"run", "-d", "--name", "pulumi-pod-p1-x", "--label", "com.pulumi.pod=p1",
		"alpine:3", "echo", "hi",
	}
	assert.Equal(t, want, fake.calls[0])
}

func TestRunContainerValidation(t *testing.T) {
	t.Parallel()
	pm := NewDockerPodManager("p1", withRunner((&fakeRunner{}).run))
	_, err := pm.RunContainer(t.Context(), ContainerConfig{Image: "alpine:3"})
	assert.ErrorContains(t, err, "Name")
	_, err = pm.RunContainer(t.Context(), ContainerConfig{Name: "x"})
	assert.ErrorContains(t, err, "Image")
}

// runToCompletionArgs is the build container's argv: ephemeral (--rm, no --name),
// inheriting the engine's mounts via --volumes-from, with stdout reserved for the
// ref (so no -t/-i here — that is enforced by the streaming exec, not the argv).
func TestRunToCompletionArgs(t *testing.T) {
	t.Parallel()
	args := runToCompletionArgs("com.pulumi.pod=p1", ContainerConfig{
		Image:       "docker:cli",
		WorkingDir:  "/workspace/app",
		VolumesFrom: []string{"pulumi-pod-p1-engine"},
		// Out of order to prove env is sorted.
		Env:        map[string]string{"B": "2", "A": "1"},
		Entrypoint: []string{"sh", "-c"},
		Cmd:        []string{"docker build -q ."},
	})
	want := []string{
		"run", "--rm", "--label", "com.pulumi.pod=p1",
		"--volumes-from", "pulumi-pod-p1-engine",
		"-w", "/workspace/app",
		"-e", "A=1", "-e", "B=2",
		"--entrypoint", "sh",
		"docker:cli",
		"-c", "docker build -q .",
	}
	assert.Equal(t, want, args)
}

func TestRunToCompletionValidation(t *testing.T) {
	t.Parallel()
	pm := NewDockerPodManager("p1")
	// Missing Image returns before any exec, so no daemon is needed.
	_, err := pm.RunToCompletion(t.Context(), ContainerConfig{}, io.Discard)
	assert.ErrorContains(t, err, "Image")
}

func TestCreateNetworkArgs(t *testing.T) {
	t.Parallel()
	fake := &fakeRunner{respond: func([]string) (string, string, error) { return "net-xyz", "", nil }}
	pm := NewDockerPodManager("p1", withRunner(fake.run))

	net, err := pm.CreateNetwork(t.Context())
	require.NoError(t, err)
	assert.Equal(t, Network{ID: "net-xyz", Name: "pulumi-pod-p1-net"}, net)
	assert.Equal(t, []string{
		"network", "create", "--label", "com.pulumi.pod=p1", "pulumi-pod-p1-net",
	}, fake.calls[0])
}

func TestCreateVolumeArgs(t *testing.T) {
	t.Parallel()
	fake := &fakeRunner{}
	pm := NewDockerPodManager("p1", withRunner(fake.run))

	vol, err := pm.CreateVolume(t.Context(), "workspace")
	require.NoError(t, err)
	assert.Equal(t, Volume{Name: "pulumi-pod-p1-vol-workspace"}, vol)
	assert.Equal(t, []string{
		"volume", "create", "--label", "com.pulumi.pod=p1", "pulumi-pod-p1-vol-workspace",
	}, fake.calls[0])
}

func TestCopyFromImageArgs(t *testing.T) {
	t.Parallel()
	// Seed a named volume from an image's srcPath by CREATING (not starting) a
	// throwaway container with the volume mounted there, then removing it — no shell
	// or `cp` in the image required, so it works with scratch provider images.
	fake := &fakeRunner{respond: func([]string) (string, string, error) { return "seedcid", "", nil }}
	pm := NewDockerPodManager("p1", withRunner(fake.run))

	err := pm.CopyFromImage(t.Context(), "img:1", "/plugin/", Volume{Name: "pulumi-pod-p1-vol-plugin-x"}, "/plugins")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"create",
		"--label", "com.pulumi.pod=p1",
		"-v", "pulumi-pod-p1-vol-plugin-x:/plugin",
		"img:1",
	}, fake.calls[0])
	assert.Equal(t, []string{"rm", "-f", "seedcid"}, fake.calls[1])
}

func TestWaitContainer(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"0", "17"} {
		fake := &fakeRunner{respond: func([]string) (string, string, error) { return code, "", nil }}
		pm := NewDockerPodManager("p1", withRunner(fake.run))
		got, err := pm.WaitContainer(t.Context(), Container{ID: "cid"})
		require.NoError(t, err)
		assert.Equal(t, []string{"wait", "cid"}, fake.calls[0])
		switch code {
		case "0":
			assert.Equal(t, 0, got)
		case "17":
			assert.Equal(t, 17, got)
		}
	}
}

func TestStopContainerIgnoresMissing(t *testing.T) {
	t.Parallel()
	fake := &fakeRunner{respond: func([]string) (string, string, error) {
		return "", "Error: No such container: cid", errors.New("exit status 1")
	}}
	pm := NewDockerPodManager("p1", withRunner(fake.run))
	// "No such container" is treated as success — the container is already gone.
	require.NoError(t, pm.StopContainer(t.Context(), Container{ID: "cid"}))
}

func TestCleanupOrderAndTracking(t *testing.T) {
	t.Parallel()
	fake := &fakeRunner{respond: func(args []string) (string, string, error) {
		switch args[0] {
		case "network":
			return "netid", "", nil
		case "run":
			return "cid", "", nil
		default:
			return "", "", nil
		}
	}}
	pm := NewDockerPodManager("p1", withRunner(fake.run))

	ctx := t.Context()
	net, err := pm.CreateNetwork(ctx)
	require.NoError(t, err)
	_, err = pm.RunContainer(ctx, ContainerConfig{Image: "alpine:3", Name: "c", Network: net.Name})
	require.NoError(t, err)
	_, err = pm.CreateVolume(ctx, "data")
	require.NoError(t, err)

	setup := len(fake.calls)
	require.NoError(t, pm.Cleanup(ctx))

	// Teardown order: containers, then volumes, then network.
	assert.Equal(t, [][]string{
		{"rm", "-f", "cid"},
		{"volume", "rm", "-f", "pulumi-pod-p1-vol-data"},
		{"network", "rm", "netid"},
	}, fake.calls[setup:])

	// Cleanup clears tracking, so a second call is a no-op.
	fake.calls = nil
	require.NoError(t, pm.Cleanup(ctx))
	assert.Empty(t, fake.calls)
}

func TestCleanupJoinsErrorsAndContinues(t *testing.T) {
	t.Parallel()
	fake := &fakeRunner{respond: func(args []string) (string, string, error) {
		if args[0] == "rm" { // container removal fails (not a "no such" error)
			return "", "boom", errors.New("exit status 1")
		}
		if args[0] == "run" || args[0] == "network" {
			return "id", "", nil
		}
		return "", "", nil
	}}
	pm := NewDockerPodManager("p1", withRunner(fake.run))

	ctx := t.Context()
	_, err := pm.CreateNetwork(ctx)
	require.NoError(t, err)
	_, err = pm.RunContainer(ctx, ContainerConfig{Image: "alpine:3", Name: "c"})
	require.NoError(t, err)
	_, err = pm.CreateVolume(ctx, "data")
	require.NoError(t, err)

	setup := len(fake.calls)
	err = pm.Cleanup(ctx)
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")

	// Despite the container removal failing, volume and network teardown still ran.
	var sawVolume, sawNetwork bool
	for _, call := range fake.calls[setup:] {
		if call[0] == "volume" {
			sawVolume = true
		}
		if call[0] == "network" {
			sawNetwork = true
		}
	}
	assert.True(t, sawVolume, "volume teardown should run even after container teardown fails")
	assert.True(t, sawNetwork, "network teardown should run even after container teardown fails")
}

func TestContainerAddress(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "pulumi-pod-p1-engine:50051", Container{Name: "pulumi-pod-p1-engine"}.Address(50051))
}

// --- Integration tests (opt-in: require a real Docker daemon) ---

func requireDocker(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping docker integration test in -short mode")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found in PATH")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not available")
	}
}

func newIntegrationPodManager(t *testing.T) PodManager {
	t.Helper()
	// os.Getpid keeps the id stable within a run but distinct across runs, so a
	// crashed prior run's (labeled) leftovers don't collide with fresh names.
	pm := NewDockerPodManager(fmt.Sprintf("it%d", os.Getpid()))
	t.Cleanup(func() {
		// t.Context() is canceled before cleanup functions run, so use a fresh context.
		if err := pm.Cleanup(context.Background()); err != nil { //nolint:usetesting
			t.Logf("pod cleanup: %v", err)
		}
	})
	return pm
}

func readLogs(t *testing.T, ctx context.Context, pm PodManager, c Container) string {
	t.Helper()
	rc, err := pm.ContainerLogs(ctx, c, false)
	require.NoError(t, err)
	defer rc.Close()
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	return string(b)
}

// TestPodNetworkingIntegration is the Phase 1 milestone: two containers on the
// pod network reach each other by DNS name.
//
//nolint:paralleltest // shares a process-scoped pod id; parallel runs would collide on resource names
func TestPodNetworkingIntegration(t *testing.T) {
	requireDocker(t)
	ctx := t.Context()
	pm := newIntegrationPodManager(t)

	net, err := pm.CreateNetwork(ctx)
	require.NoError(t, err)

	server, err := pm.RunContainer(ctx, ContainerConfig{
		Image:   "alpine:3",
		Name:    "server",
		Network: net.Name,
		Cmd:     []string{"sleep", "60"},
	})
	require.NoError(t, err)

	// The client pings the server by the DNS name the manager assigned it.
	client, err := pm.RunContainer(ctx, ContainerConfig{
		Image:   "alpine:3",
		Name:    "client",
		Network: net.Name,
		Cmd:     []string{"ping", "-c", "1", server.Name},
	})
	require.NoError(t, err)

	code, err := pm.WaitContainer(ctx, client)
	require.NoError(t, err)
	logs := readLogs(t, ctx, pm, client)
	require.Equal(t, 0, code, "client could not reach server over the pod network; logs:\n%s", logs)
	assert.Contains(t, logs, "bytes from")
}

// TestWorkspaceVolumeIntegration is the Phase 5 building block: seed a named
// volume from an image's filesystem, then read it back from another container.
//
//nolint:paralleltest // shares a process-scoped pod id; parallel runs would collide on resource names
func TestWorkspaceVolumeIntegration(t *testing.T) {
	requireDocker(t)
	ctx := t.Context()
	pm := newIntegrationPodManager(t)

	vol, err := pm.CreateVolume(ctx, "workspace")
	require.NoError(t, err)

	// Seed the volume with /etc from the alpine image.
	require.NoError(t, pm.CopyFromImage(ctx, "alpine:3", "/etc", vol, "/workspace"))

	reader, err := pm.RunContainer(ctx, ContainerConfig{
		Image:   "alpine:3",
		Name:    "reader",
		Volumes: []VolumeMount{{Source: vol.Name, Target: "/workspace", ReadOnly: true}},
		// /etc/alpine-release exists in the alpine image; it should now be in the volume.
		Cmd: []string{"cat", "/workspace/alpine-release"},
	})
	require.NoError(t, err)

	code, err := pm.WaitContainer(ctx, reader)
	require.NoError(t, err)
	logs := readLogs(t, ctx, pm, reader)
	require.Equal(t, 0, code, "could not read seeded file from volume; logs:\n%s", logs)
	assert.Contains(t, logs, "3.", "expected an alpine version string from the seeded /etc/alpine-release")
}

// remapPullEndpoint is the docker address layer: it redirects a pull to a
// configured endpoint for the ref's registry host while leaving the identity ref
// (the name the image is tagged and run under) untouched. A host with no endpoint,
// an org-only leading segment, or an empty map all pass through unchanged.
func TestRemapPullEndpoint(t *testing.T) {
	t.Parallel()
	endpoints := map[string]string{
		"pulumi.registry.internal":  "localhost:5064",
		"private.registry.internal": "localhost:5065",
	}
	cases := []struct {
		ref      string
		wantPull string
		wantMap  bool
	}{
		// A configured identity host redirects to its endpoint; the repo/tag is kept.
		{
			"pulumi.registry.internal/pulumi/pulumi-provider-random:v4.21.0",
			"localhost:5064/pulumi/pulumi-provider-random:v4.21.0", true,
		},
		{
			"private.registry.internal/pulumi/pulumi-provider-random:v4.21.0",
			"localhost:5065/pulumi/pulumi-provider-random:v4.21.0", true,
		},
		// An unconfigured host (a real registry) passes through — production pulls direct.
		{
			"ghcr.io/acme/pulumi-provider-greeting:v0.1.0",
			"ghcr.io/acme/pulumi-provider-greeting:v0.1.0", false,
		},
		// A leading org segment is not a host (no "."/":"/localhost), so no remap.
		{"pulumi/pulumi-provider-random:v4.21.0", "pulumi/pulumi-provider-random:v4.21.0", false},
	}
	for _, c := range cases {
		got, remapped := remapPullEndpoint(c.ref, endpoints)
		assert.Equal(t, c.wantMap, remapped, "remap flag for %s", c.ref)
		assert.Equal(t, c.wantPull, got, "pull ref for %s", c.ref)
	}

	// An empty map never remaps.
	got, remapped := remapPullEndpoint("pulumi.registry.internal/pulumi/pulumi-provider-random:v4.21.0", nil)
	assert.False(t, remapped)
	assert.Equal(t, "pulumi.registry.internal/pulumi/pulumi-provider-random:v4.21.0", got)
}
