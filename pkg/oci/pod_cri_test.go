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
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// fakeCRI records the requests the manager makes and returns canned responses, so the CRI verb
// construction can be asserted without a live containerd — the CRI analogue of the docker
// manager's argv-capturing commandRunner. It satisfies both criRuntimeService and criImageService.
type fakeCRI struct {
	createReqs []*runtimeapi.CreateContainerRequest
	started    []string
	stopped    []string
	removed    []string

	nextID    string                        // id CreateContainer returns
	statuses  []*runtimeapi.ContainerStatus // returned by successive ContainerStatus calls
	statusIdx int
	removeErr error // returned by RemoveContainer (e.g. a NotFound)
	stopErr   error // returned by StopContainer

	imageStatus *runtimeapi.Image // ImageStatus result (nil = absent)
	imageErr    error             // ImageStatus error
	pulled      []string
}

func (f *fakeCRI) CreateContainer(_ context.Context, req *runtimeapi.CreateContainerRequest,
	_ ...grpc.CallOption,
) (*runtimeapi.CreateContainerResponse, error) {
	f.createReqs = append(f.createReqs, req)
	id := f.nextID
	if id == "" {
		id = "ctr-id"
	}
	return &runtimeapi.CreateContainerResponse{ContainerId: id}, nil
}

func (f *fakeCRI) StartContainer(_ context.Context, req *runtimeapi.StartContainerRequest,
	_ ...grpc.CallOption,
) (*runtimeapi.StartContainerResponse, error) {
	f.started = append(f.started, req.GetContainerId())
	return &runtimeapi.StartContainerResponse{}, nil
}

func (f *fakeCRI) StopContainer(_ context.Context, req *runtimeapi.StopContainerRequest,
	_ ...grpc.CallOption,
) (*runtimeapi.StopContainerResponse, error) {
	f.stopped = append(f.stopped, req.GetContainerId())
	return &runtimeapi.StopContainerResponse{}, f.stopErr
}

func (f *fakeCRI) RemoveContainer(_ context.Context, req *runtimeapi.RemoveContainerRequest,
	_ ...grpc.CallOption,
) (*runtimeapi.RemoveContainerResponse, error) {
	f.removed = append(f.removed, req.GetContainerId())
	return &runtimeapi.RemoveContainerResponse{}, f.removeErr
}

func (f *fakeCRI) ContainerStatus(_ context.Context, _ *runtimeapi.ContainerStatusRequest,
	_ ...grpc.CallOption,
) (*runtimeapi.ContainerStatusResponse, error) {
	st := f.statuses[min(f.statusIdx, len(f.statuses)-1)]
	f.statusIdx++
	return &runtimeapi.ContainerStatusResponse{Status: st}, nil
}

func (f *fakeCRI) ImageStatus(_ context.Context, _ *runtimeapi.ImageStatusRequest,
	_ ...grpc.CallOption,
) (*runtimeapi.ImageStatusResponse, error) {
	if f.imageErr != nil {
		return nil, f.imageErr
	}
	return &runtimeapi.ImageStatusResponse{Image: f.imageStatus}, nil
}

func (f *fakeCRI) PullImage(_ context.Context, req *runtimeapi.PullImageRequest,
	_ ...grpc.CallOption,
) (*runtimeapi.PullImageResponse, error) {
	f.pulled = append(f.pulled, req.GetImage().GetImage())
	return &runtimeapi.PullImageResponse{ImageRef: req.GetImage().GetImage()}, nil
}

// newFakeCriManager builds a criPodManager wired to a fakeCRI with a fixed sandbox and log dir.
func newFakeCriManager(t *testing.T, fake *fakeCRI, podID string) *criPodManager {
	t.Helper()
	m := NewCriPodManager(podID,
		WithCRIClients(fake, fake),
		WithCRISandboxID("sb-1"),
		WithCRILogDir(t.TempDir()),
	).(*criPodManager)
	return m
}

// TestNewPodManagerSelectsCriRuntime pins the selection seam for the CRI runtime: PULUMI_POD_RUNTIME
// "cri" (case- and space-insensitive) yields the CRI manager, and nothing else does — so the
// default docker path and the containerd/nerdctl path are unaffected.
func TestNewPodManagerSelectsCriRuntime(t *testing.T) {
	cases := []struct {
		env     string
		wantCri bool
	}{
		{"", false},
		{"docker", false},
		{"containerd", false},
		{"nerdctl", false},
		{"cri", true},
		{"  CRI  ", true},
	}
	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			t.Setenv(PodRuntimeEnvVar, c.env)
			_, isCri := NewPodManager("p").(*criPodManager)
			assert.Equal(t, c.wantCri, isCri, "runtime %q selected the wrong manager", c.env)
		})
	}
}

// TestRuntimeIsCRI pins the predicate the language host uses to switch the program→engine wiring
// to loopback; it must agree with NewPodManager's selection (both read normalizedPodRuntime).
func TestRuntimeIsCRI(t *testing.T) {
	for env, want := range map[string]bool{"": false, "docker": false, "containerd": false, "cri": true, " CRI ": true} {
		t.Run(env, func(t *testing.T) {
			t.Setenv(PodRuntimeEnvVar, env)
			assert.Equal(t, want, RuntimeIsCRI())
		})
	}
}

// TestCriRunContainerBuildsSpec is the CRI analogue of the docker argv assertions: it pins the
// translation from a runtime-neutral ContainerConfig to a CRI CreateContainerRequest —
// Entrypoint→Command, Cmd→Args, sorted Envs, host-path Mounts, the pod label, a per-attempt log
// path, and privileged security context — and that the container is then started.
func TestCriRunContainerBuildsSpec(t *testing.T) {
	t.Parallel()
	fake := &fakeCRI{nextID: "ctr-abc"}
	m := newFakeCriManager(t, fake, "p1")

	c, err := m.RunContainer(t.Context(), ContainerConfig{
		Image:      "provider:1",
		Name:       "provider-x",
		Entrypoint: []string{"/bin/prov", "serve"},
		Cmd:        []string{"--flag"},
		WorkingDir: "/workspace",
		Env:        map[string]string{"B": "2", "A": "1"},
		Volumes:    []VolumeMount{{Source: "/host/ws", Target: "/workspace"}},
		Privileged: true,
		// Docker-only fields the CRI manager must ignore rather than choke on.
		Network:     "container:engine",
		HostGateway: true,
		VolumesFrom: []string{"engine"},
	})
	require.NoError(t, err)
	assert.Equal(t, Container{ID: "ctr-abc", Name: "provider-x"}, c)

	require.Len(t, fake.createReqs, 1)
	req := fake.createReqs[0]
	assert.Equal(t, "sb-1", req.GetPodSandboxId(), "the container must be created in the adopted sandbox")

	cfg := req.GetConfig()
	assert.Equal(t, "provider-x", cfg.GetMetadata().GetName())
	assert.Equal(t, uint32(0), cfg.GetMetadata().GetAttempt())
	assert.Equal(t, "provider:1", cfg.GetImage().GetImage())
	assert.Equal(t, []string{"/bin/prov", "serve"}, cfg.GetCommand(), "Entrypoint maps to CRI Command")
	assert.Equal(t, []string{"--flag"}, cfg.GetArgs(), "Cmd maps to CRI Args")
	assert.Equal(t, "/workspace", cfg.GetWorkingDir())
	assert.Equal(t, []*runtimeapi.KeyValue{{Key: "A", Value: "1"}, {Key: "B", Value: "2"}}, cfg.GetEnvs(),
		"envs must be emitted in sorted key order")
	assert.Equal(t, []*runtimeapi.Mount{{HostPath: "/host/ws", ContainerPath: "/workspace"}}, cfg.GetMounts())
	assert.Equal(t, "p1", cfg.GetLabels()[podLabel])
	assert.Equal(t, "provider-x_0.log", cfg.GetLogPath())
	assert.True(t, cfg.GetLinux().GetSecurityContext().GetPrivileged())

	assert.Equal(t, []string{"ctr-abc"}, fake.started, "the created container must be started")
}

// TestCriRunContainerPerAttemptLogPath pins the spike's per-attempt requirement: a second run of
// the same logical name gets a distinct attempt and log path, so their logs cannot cross-
// contaminate (CRI enforces a unique (name, attempt) per sandbox).
func TestCriRunContainerPerAttemptLogPath(t *testing.T) {
	t.Parallel()
	fake := &fakeCRI{}
	m := newFakeCriManager(t, fake, "p1")

	_, err := m.RunContainer(t.Context(), ContainerConfig{Image: "i", Name: "program"})
	require.NoError(t, err)
	_, err = m.RunContainer(t.Context(), ContainerConfig{Image: "i", Name: "program"})
	require.NoError(t, err)

	require.Len(t, fake.createReqs, 2)
	assert.Equal(t, uint32(0), fake.createReqs[0].GetConfig().GetMetadata().GetAttempt())
	assert.Equal(t, "program_0.log", fake.createReqs[0].GetConfig().GetLogPath())
	assert.Equal(t, uint32(1), fake.createReqs[1].GetConfig().GetMetadata().GetAttempt())
	assert.Equal(t, "program_1.log", fake.createReqs[1].GetConfig().GetLogPath())
}

// TestCriCreateNetworkAdoptsSandbox proves CreateNetwork does not create anything — it returns a
// handle to the sandbox the wrapper forwarded — and errors clearly when none was forwarded.
func TestCriCreateNetworkAdoptsSandbox(t *testing.T) {
	t.Parallel()

	adopting := NewCriPodManager("p", WithCRISandboxID("sb-42"))
	net, err := adopting.CreateNetwork(t.Context())
	require.NoError(t, err)
	assert.Equal(t, Network{ID: "sb-42", Name: "sb-42"}, net)

	orphan := NewCriPodManager("p", WithCRISandboxID("")) // no sandbox id, independent of ambient env
	_, err = orphan.CreateNetwork(t.Context())
	require.Error(t, err, "with no sandbox the manager cannot adopt one and must say so")
	assert.Contains(t, err.Error(), CRISandboxIDEnvVar)
}

// TestCriWaitContainerPollsToExit proves WaitContainer polls the container status until it exits
// and returns the real exit code (there is no blocking wait RPC on CRI).
func TestCriWaitContainerPollsToExit(t *testing.T) {
	t.Parallel()
	fake := &fakeCRI{statuses: []*runtimeapi.ContainerStatus{
		{State: runtimeapi.ContainerState_CONTAINER_RUNNING},
		{State: runtimeapi.ContainerState_CONTAINER_RUNNING},
		{State: runtimeapi.ContainerState_CONTAINER_EXITED, ExitCode: 7},
	}}
	m := newFakeCriManager(t, fake, "p1")

	code, err := m.WaitContainer(t.Context(), Container{ID: "c", Name: "n"})
	require.NoError(t, err)
	assert.Equal(t, 7, code)
	assert.GreaterOrEqual(t, fake.statusIdx, 3, "it should have polled until the exited status")
}

// TestCriStopContainerIsIdempotent proves Stop then Remove are both issued and that a NotFound
// from either (the container is already gone) is treated as success.
func TestCriStopContainerIsIdempotent(t *testing.T) {
	t.Parallel()
	fake := &fakeCRI{removeErr: status.Error(codes.NotFound, "an error occurred when try to find container: not found")}
	m := newFakeCriManager(t, fake, "p1")

	err := m.StopContainer(t.Context(), Container{ID: "c-1", Name: "n"})
	require.NoError(t, err, "a NotFound on remove means already-gone, not a failure")
	assert.Equal(t, []string{"c-1"}, fake.stopped)
	assert.Equal(t, []string{"c-1"}, fake.removed)
}

// TestCriImageExistsAndPull covers the image verbs: ImageStatus with a nil image reads as absent
// and with an image as present, and PullImage reaches the CRI image service (which lands the image
// in the k8s.io namespace CRI-run containers see).
func TestCriImageExistsAndPull(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	absent := &fakeCRI{imageStatus: nil}
	m := newFakeCriManager(t, absent, "p1")
	exists, err := m.ImageExists(ctx, "img:1")
	require.NoError(t, err)
	assert.False(t, exists)

	present := &fakeCRI{imageStatus: &runtimeapi.Image{Id: "sha256:abc"}}
	m = newFakeCriManager(t, present, "p1")
	exists, err = m.ImageExists(ctx, "img:1")
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, m.PullImage(ctx, "img:2"))
	assert.Equal(t, []string{"img:2"}, present.pulled)
}

// TestCriUnimplementedVerbs pins that the deferred verbs fail loudly (with a pointer to the design
// call) rather than silently misbehaving.
func TestCriUnimplementedVerbs(t *testing.T) {
	t.Parallel()
	m := newFakeCriManager(t, &fakeCRI{}, "p1")
	ctx := t.Context()

	assert.Error(t, m.ImportImage(ctx, "/layout", "ref:1"))
	_, err := m.RunToCompletion(ctx, ContainerConfig{Image: "i"}, io.Discard)
	assert.Error(t, err)
	assert.Error(t, m.CopyFromImage(ctx, "img", "/src", Volume{Name: "v"}, "/dst"))
	_, err = m.ReadImageFile(ctx, "img", "/path")
	assert.Error(t, err)
}

// TestCriCreateVolumeCreatesHostDir proves CreateVolume creates a host directory under the
// configured volume root and returns the logical volume name (matching docker's convention, so
// WorkspaceVolumeName and CreateVolume agree). criMounts resolves the logical name to a host path.
func TestCriCreateVolumeCreatesHostDir(t *testing.T) {
	t.Parallel()
	volDir := t.TempDir()
	m := NewCriPodManager("p1",
		WithCRIClients(&fakeCRI{}, &fakeCRI{}),
		WithCRISandboxID("sb-1"),
		WithCRILogDir(t.TempDir()),
		WithCRIVolumeDir(volDir),
	).(*criPodManager)

	vol, err := m.CreateVolume(t.Context(), "workspace")
	require.NoError(t, err)
	assert.Equal(t, "pulumi-pod-p1-vol-workspace", vol.Name,
		"Volume.Name must be the logical name, matching WorkspaceVolumeName's convention")

	// The host directory is at volumeDir/volName.
	hostDir := filepath.Join(volDir, vol.Name)
	info, err := os.Stat(hostDir)
	require.NoError(t, err, "the host dir must exist")
	assert.True(t, info.IsDir())

	// criMounts resolves the bare name to the host path.
	mounts := m.criMounts([]VolumeMount{{Source: vol.Name, Target: "/workspace"}})
	require.Len(t, mounts, 1)
	assert.Equal(t, hostDir, mounts[0].HostPath, "bare volume name must resolve to host path")

	// An absolute path passes through unchanged.
	absMounts := m.criMounts([]VolumeMount{{Source: "/run/containerd/containerd.sock", Target: "/sock"}})
	require.Len(t, absMounts, 1)
	assert.Equal(t, "/run/containerd/containerd.sock", absMounts[0].HostPath)

	// A second volume with a different name creates a distinct directory.
	vol2, err := m.CreateVolume(t.Context(), "plugin-random")
	require.NoError(t, err)
	assert.NotEqual(t, vol.Name, vol2.Name)

	// Cleanup removes the directories.
	require.NoError(t, m.Cleanup(t.Context()))
	_, err = os.Stat(hostDir)
	assert.True(t, os.IsNotExist(err), "volume dir should be removed by Cleanup")
}

// TestDeframeCRILogLine is the pure-function table for undoing the K8s log framing:
// "<ts> {stdout|stderr} {F|P} <content>".
func TestDeframeCRILogLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		line        string
		wantContent string
		wantStream  string
		wantFull    bool
		wantOK      bool
	}{
		{"full stdout", "2026-07-19T00:00:00.000000000Z stdout F 50051", "50051", "stdout", true, true},
		{"partial stdout", "2026-07-19T00:00:00.000000000Z stdout P chunk", "chunk", "stdout", false, true},
		{"stderr", "2026-07-19T00:00:00Z stderr F oops", "oops", "stderr", true, true},
		{"content with spaces", "2026-07-19T00:00:00Z stdout F a b c", "a b c", "stdout", true, true},
		{"empty content no trailing space", "2026-07-19T00:00:00Z stdout F", "", "stdout", true, true},
		{"empty content trailing space", "2026-07-19T00:00:00Z stdout F ", "", "stdout", true, true},
		{"unframed line", "just some text", "", "", false, false},
		{"unknown stream", "2026-07-19T00:00:00Z other F x", "", "", false, false},
		{"unknown tag", "2026-07-19T00:00:00Z stdout X x", "", "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			content, stream, full, ok := deframeCRILogLine([]byte(c.line))
			assert.Equal(t, c.wantOK, ok)
			if !ok {
				return
			}
			assert.Equal(t, c.wantContent, string(content))
			assert.Equal(t, c.wantStream, string(stream))
			assert.Equal(t, c.wantFull, full)
		})
	}
}

// TestCriContainerLogsTailsFile proves the whole read-side handshake path: ContainerLogs finds the
// log file for a started container (under the mounted log dir, at its per-attempt path), tails it,
// and yields the de-framed combined stream — reassembling a partial+full line pair into one
// logical line, exactly what scrapeServingPort scans.
func TestCriContainerLogsTailsFile(t *testing.T) {
	t.Parallel()
	fake := &fakeCRI{nextID: "ctr-1"}
	m := newFakeCriManager(t, fake, "p1")
	ctx := t.Context()

	c, err := m.RunContainer(ctx, ContainerConfig{Image: "i", Name: "provider-y"})
	require.NoError(t, err)

	// containerd would write this file; place it where the manager expects (logDir/<name>_<attempt>.log).
	framed := "" +
		"2026-07-19T00:00:00Z stderr F starting up\n" +
		"2026-07-19T00:00:00Z stdout P 500" +
		"\n2026-07-19T00:00:00Z stdout F 51\n"
	require.NoError(t, os.WriteFile(filepath.Join(m.logDir, "provider-y_0.log"), []byte(framed), 0o600))

	rc, err := m.ContainerLogs(ctx, c, false)
	require.NoError(t, err)
	defer rc.Close()
	out, err := io.ReadAll(rc)
	require.NoError(t, err)
	// stderr line, then the partial+full stdout pair reassembled as "50051" on one line.
	assert.Equal(t, "starting up\n50051\n", string(out))
}

// TestCriContainerLogsNeedsLogDir proves the honest asterisk is surfaced, not swallowed: with no
// pod log directory mounted, ContainerLogs fails with an actionable error naming the env var.
func TestCriContainerLogsNeedsLogDir(t *testing.T) {
	t.Parallel()
	fake := &fakeCRI{nextID: "ctr-1"}
	// Force an empty log dir independent of the ambient env, to prove the missing-dir path.
	m := NewCriPodManager("p1",
		WithCRIClients(fake, fake), WithCRISandboxID("sb-1"), WithCRILogDir(""),
	).(*criPodManager)

	c, err := m.RunContainer(t.Context(), ContainerConfig{Image: "i", Name: "provider-z"})
	require.NoError(t, err)
	_, err = m.ContainerLogs(t.Context(), c, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), CRILogDirEnvVar)
}
