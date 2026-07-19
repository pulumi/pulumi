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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// criPodManager is a PodManager backed by containerd through the CRI (Container Runtime
// Interface). It is the third runtime the pod model runs on, and unlike the other two it is a
// pure in-process gRPC client — no CLI, no second binary in the engine image. That is the whole
// point of it: the nerdctl manager proved the docker→containerd store coupling is narrow, but hit
// a wall on the *run* path — an in-engine `nerdctl run` does not run the container, it writes an
// OCI spec whose host-side `createRuntime` hook forks a path-identical nerdctl back on the host.
// CRI dissolves that: the CRI plugin runs CNI and rootfs server-side, containers join the pod's
// netns by path with no OCI hook, so the engine drives the full run+network path over the CRI
// socket alone (spike 2026-07-19, findings in scratch).
//
// The elegant part is that CRI's PodSandbox *is* our pod — a shared-netns unit that containers
// join — so the abstraction it was built for is exactly ours. Everything in the sandbox reaches
// everything else, and the engine, over loopback. Two consequences ripple through the verb
// mapping below:
//
//   - There is one network, the sandbox, not a bridge per pod plus per-container netns joins. So
//     CreateNetwork does not mint anything — the sandbox already exists (the wrapper created it
//     host-side and started the engine inside it, the way it creates a docker network + engine
//     container today), and this manager *adopts* it. ContainerConfig.Network — a docker/nerdctl
//     concept (`--network container:<peer>` or a bridge name) — is not consulted; every container
//     goes into the one sandbox.
//   - CRI has no log-read RPC. `docker logs` streams over the daemon API; on CRI a container's
//     output lands in a host log file (`<sandbox log dir>/<container log path>`), read
//     client-side. So the engine needs the pod log directory mounted (the honest asterisk to the
//     "engine needs only the endpoint" story: one shared *data* dir, not a host-binary callback),
//     and ContainerLogs tails that file and strips the K8s log framing before the caller sees it.
//
// This is an early, deliberately partial implementation. Run + network + image existence/pull are
// here; ImportImage (a genuine ctr-vs-proxy-pull design call), RunToCompletion (the build
// primitive, which recovers docker's stdout/stderr split by filtering the log on its stream tag),
// and the volume verbs (CRI has no named-volume analog — the workspace becomes a host dir) return
// documented not-yet-implemented errors rather than pretending to work.
type criPodManager struct {
	podID     string // unique id for this pod; labels containers and scopes log paths
	sandboxID string // the PodSandbox the engine runs in and adds siblings to (from the wrapper)
	logDir    string // the sandbox log directory, mounted into the engine so logs can be tailed
	endpoint  string // the CRI gRPC endpoint (a unix socket target for grpc.NewClient)

	// runtime and image are the CRI services. They are narrow interfaces (not the full generated
	// clients) so tests can supply small fakes; the real runtimeapi clients satisfy them
	// structurally. They are dialed lazily (dialOnce) so construction stays infallible and a down
	// socket surfaces at call time, matching the docker manager's defer-to-call-time behavior —
	// unless injected up front, in which case dialOnce is a no-op.
	dialOnce sync.Once
	dialErr  error
	conn     *grpc.ClientConn
	runtime  criRuntimeService
	image    criImageService

	mu         sync.Mutex
	containers []Container
	// attempts counts how many times a given logical container name has been run in this
	// sandbox. CRI enforces a unique (name, attempt) per sandbox and writes the log to a path
	// derived from it, so a re-run of the same name must bump the attempt or its log collides
	// with the prior run's (the spike's "per-attempt log paths" finding). Most names are already
	// process-unique (see uniqueContainerName), so this is usually 0.
	attempts map[string]uint32
	// logPaths maps a started container's id to its log file path relative to logDir, so
	// ContainerLogs can find the file to tail without re-deriving the attempt.
	logPaths map[string]string
}

// criRuntimeService is the subset of the CRI RuntimeService (k8s.io/cri-api) the pod manager
// calls. Narrowing the ~25-method generated interface to the verbs we actually use keeps the
// dependency surface explicit and lets tests implement a small fake; the generated
// runtimeapi.RuntimeServiceClient satisfies this by method subset.
type criRuntimeService interface {
	CreateContainer(context.Context, *runtimeapi.CreateContainerRequest,
		...grpc.CallOption) (*runtimeapi.CreateContainerResponse, error)
	StartContainer(context.Context, *runtimeapi.StartContainerRequest,
		...grpc.CallOption) (*runtimeapi.StartContainerResponse, error)
	StopContainer(context.Context, *runtimeapi.StopContainerRequest,
		...grpc.CallOption) (*runtimeapi.StopContainerResponse, error)
	RemoveContainer(context.Context, *runtimeapi.RemoveContainerRequest,
		...grpc.CallOption) (*runtimeapi.RemoveContainerResponse, error)
	ContainerStatus(context.Context, *runtimeapi.ContainerStatusRequest,
		...grpc.CallOption) (*runtimeapi.ContainerStatusResponse, error)
}

// criImageService is the subset of the CRI ImageService the pod manager calls.
type criImageService interface {
	ImageStatus(context.Context, *runtimeapi.ImageStatusRequest,
		...grpc.CallOption) (*runtimeapi.ImageStatusResponse, error)
	PullImage(context.Context, *runtimeapi.PullImageRequest,
		...grpc.CallOption) (*runtimeapi.PullImageResponse, error)
}

// CRISandboxIDEnvVar names the PodSandbox the engine container itself runs in. On CRI the wrapper
// creates the sandbox host-side (as it creates the docker network today) and starts the engine
// inside it, then forwards this id so the in-engine manager adds providers and the program to the
// SAME sandbox — the shared netns that lets everything reach the engine on loopback. Without it,
// the manager would have to create a sandbox the engine is not a member of, breaking loopback.
const CRISandboxIDEnvVar = "PULUMI_POD_SANDBOX_ID"

// CRILogDirEnvVar names the sandbox log directory. CRI has no log-read RPC — a container's output
// is written to a file under this directory — so the wrapper mounts it into the engine and
// ContainerLogs tails the file. This is the one shared *data* dir the CRI model needs beyond the
// socket (the honest §6 asterisk).
const CRILogDirEnvVar = "PULUMI_POD_LOG_DIR"

// CRIEndpointEnvVar overrides the CRI gRPC socket the manager dials. It defaults to containerd's
// socket, on which the CRI plugin listens.
const CRIEndpointEnvVar = "PULUMI_POD_CRI_ENDPOINT"

// defaultCRIEndpoint is containerd's socket as a grpc.NewClient target (grpc has a built-in unix
// resolver, so the unix:// scheme dials the socket directly).
const defaultCRIEndpoint = "unix:///run/containerd/containerd.sock"

// criPodNamespace is the sandbox's logical (K8s-style) namespace in its PodSandboxMetadata. It is
// unrelated to the containerd *content* namespace (k8s.io) that the CRI image service uses — that
// one matters for ImportImage, not here.
const criPodNamespace = "pulumi"

// stopGracePeriodSeconds is how long StopContainer lets a container drain before it is killed.
const stopGracePeriodSeconds = 10

// CriOption configures a criPodManager.
type CriOption func(*criPodManager)

// WithCRIClients injects the runtime and image services, bypassing the lazy socket dial.
// Unexported use is intended (tests); it is exported only so out-of-package harnesses can drive
// the manager against a fake.
func WithCRIClients(runtime criRuntimeService, image criImageService) CriOption {
	return func(m *criPodManager) { m.runtime, m.image = runtime, image }
}

// WithCRISandboxID sets the sandbox the manager adopts, overriding the env var.
func WithCRISandboxID(id string) CriOption {
	return func(m *criPodManager) { m.sandboxID = id }
}

// WithCRILogDir sets the log directory the manager tails, overriding the env var.
func WithCRILogDir(dir string) CriOption {
	return func(m *criPodManager) { m.logDir = dir }
}

// NewCriPodManager returns a PodManager that drives containerd through the CRI. By default it
// reads the sandbox id, log directory, and endpoint from the environment (the wrapper forwards
// them into the engine container); options override any of these — WithCRIClients in particular
// supplies a fake for tests without a live containerd.
func NewCriPodManager(podID string, opts ...CriOption) PodManager {
	m := &criPodManager{
		podID:     podID,
		sandboxID: os.Getenv(CRISandboxIDEnvVar),
		logDir:    os.Getenv(CRILogDirEnvVar),
		endpoint:  envOr(CRIEndpointEnvVar, defaultCRIEndpoint),
		attempts:  map[string]uint32{},
		logPaths:  map[string]string{},
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// clients returns the runtime and image services, dialing the CRI socket on first use. It is
// safe for concurrent callers: dialOnce serializes the dial and its happens-before publishes the
// service fields. Injected clients (WithCRIClients) short-circuit the dial.
func (m *criPodManager) clients() (criRuntimeService, criImageService, error) {
	m.dialOnce.Do(func() {
		if m.runtime != nil && m.image != nil {
			return // injected by a test
		}
		conn, err := grpc.NewClient(m.endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			m.dialErr = fmt.Errorf("oci: connecting to the CRI endpoint %q: %w", m.endpoint, err)
			return
		}
		m.conn = conn
		m.runtime = runtimeapi.NewRuntimeServiceClient(conn)
		m.image = runtimeapi.NewImageServiceClient(conn)
	})
	return m.runtime, m.image, m.dialErr
}

func (m *criPodManager) CreateNetwork(ctx context.Context) (Network, error) {
	// Unlike the docker manager, which mints a fresh bridge, CreateNetwork on CRI *adopts* the
	// ambient sandbox. The sandbox already exists — the wrapper created it host-side and started
	// the engine in it — because engine↔container loopback requires the engine to be a member.
	// Creating a new sandbox here would give one the engine is not in, and every provider would
	// be unreachable. So this returns a handle to the sandbox the wrapper forwarded.
	if m.sandboxID == "" {
		return Network{}, fmt.Errorf("oci: no CRI pod sandbox (set %s). On CRI the wrapper creates "+
			"the PodSandbox host-side and runs the engine inside it; the manager adopts that sandbox "+
			"rather than creating one the engine is not a member of", CRISandboxIDEnvVar)
	}
	return Network{ID: m.sandboxID, Name: m.sandboxID}, nil
}

func (m *criPodManager) RunContainer(ctx context.Context, cfg ContainerConfig) (Container, error) {
	if cfg.Name == "" {
		return Container{}, errors.New("container config requires a Name")
	}
	if cfg.Image == "" {
		return Container{}, errors.New("container config requires an Image")
	}
	if m.sandboxID == "" {
		return Container{}, fmt.Errorf("oci: no CRI pod sandbox to run %q in (set %s)", cfg.Name, CRISandboxIDEnvVar)
	}
	runtime, _, err := m.clients()
	if err != nil {
		return Container{}, err
	}

	attempt := m.nextAttempt(cfg.Name)
	logPath := criLogPath(cfg.Name, attempt)
	containerCfg := m.containerConfig(cfg, attempt, logPath)

	// CreateContainer places the container in the pod sandbox but does not start it. SandboxConfig
	// carries the log directory the runtime resolves the container's log path against; see
	// sandboxConfig.
	created, err := runtime.CreateContainer(ctx, &runtimeapi.CreateContainerRequest{
		PodSandboxId:  m.sandboxID,
		Config:        containerCfg,
		SandboxConfig: m.sandboxConfig(),
	})
	if err != nil {
		return Container{}, fmt.Errorf("oci: creating container %q from %s: %w", cfg.Name, cfg.Image, err)
	}
	id := created.GetContainerId()

	if _, err := runtime.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: id}); err != nil {
		// The container was created but not started; remove it so it is not stranded.
		_, _ = runtime.RemoveContainer(context.WithoutCancel(ctx), &runtimeapi.RemoveContainerRequest{ContainerId: id})
		return Container{}, fmt.Errorf("oci: starting container %q (%s): %w", cfg.Name, id, err)
	}

	c := Container{ID: id, Name: cfg.Name}
	m.track(func() {
		m.containers = append(m.containers, c)
		m.logPaths[id] = logPath
	})
	return c, nil
}

// containerConfig translates a runtime-neutral ContainerConfig into a CRI ContainerConfig.
//
// The command mapping is cleaner than docker's: CRI Command overrides the image ENTRYPOINT (the
// whole list) and Args overrides CMD, so cfg.Entrypoint and cfg.Cmd map straight across, with no
// "--entrypoint takes a single executable" splitting. A nil Command/Args leaves the image's own.
//
// Several ContainerConfig fields are docker/nerdctl concepts with no CRI meaning and are
// intentionally not consulted: Network (there is one network, the sandbox — see CreateNetwork),
// HostGateway (the engine-on-host mode, docker-only), and VolumesFrom (a docker inheritance verb;
// on CRI the workspace is a host dir). Volumes ARE mapped, but only host-path binds translate
// today — a named volume has no CRI analog and is the deferred volume work.
func (m *criPodManager) containerConfig(
	cfg ContainerConfig, attempt uint32, logPath string,
) *runtimeapi.ContainerConfig {
	cc := &runtimeapi.ContainerConfig{
		Metadata:   &runtimeapi.ContainerMetadata{Name: cfg.Name, Attempt: attempt},
		Image:      &runtimeapi.ImageSpec{Image: cfg.Image},
		Command:    cfg.Entrypoint,
		Args:       cfg.Cmd,
		WorkingDir: cfg.WorkingDir,
		Envs:       criEnv(cfg.Env),
		Mounts:     criMounts(cfg.Volumes),
		Labels:     map[string]string{podLabel: m.podID},
		LogPath:    logPath,
	}
	if cfg.Privileged {
		cc.Linux = &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{Privileged: true},
		}
	}
	return cc
}

// sandboxConfig builds the PodSandboxConfig passed alongside each CreateContainer. The runtime
// uses it chiefly to resolve the container's log file (LogDirectory + the container's LogPath), so
// LogDirectory is the load-bearing field. The metadata mirrors the pod; matching it exactly to the
// sandbox the wrapper created is a live-environment refinement (it can be fetched via
// PodSandboxStatus if a runtime turns out to require it).
func (m *criPodManager) sandboxConfig() *runtimeapi.PodSandboxConfig {
	return &runtimeapi.PodSandboxConfig{
		Metadata:     &runtimeapi.PodSandboxMetadata{Name: m.podID, Uid: m.podID, Namespace: criPodNamespace},
		LogDirectory: m.logDir,
		Labels:       map[string]string{podLabel: m.podID},
	}
}

func (m *criPodManager) WaitContainer(ctx context.Context, c Container) (int, error) {
	runtime, _, err := m.clients()
	if err != nil {
		return -1, err
	}
	// CRI has no blocking wait RPC, so poll the container status until it leaves the running
	// state. The interval trades a little latency for not hammering the socket; the engine waits
	// on a program or build container that runs for seconds at least.
	const pollInterval = 200 * time.Millisecond
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		st, err := runtime.ContainerStatus(ctx, &runtimeapi.ContainerStatusRequest{ContainerId: c.ID})
		if err != nil {
			return -1, fmt.Errorf("oci: polling status of container %q (%s): %w", c.Name, c.ID, err)
		}
		if st.GetStatus().GetState() == runtimeapi.ContainerState_CONTAINER_EXITED {
			return int(st.GetStatus().GetExitCode()), nil
		}
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *criPodManager) StopContainer(ctx context.Context, c Container) error {
	runtime, _, err := m.clients()
	if err != nil {
		return err
	}
	// Stop (with a grace period), then remove — the two-step docker `rm -f` is one CRI verb each.
	// Both are idempotent here: a container already gone is not an error, matching StopContainer's
	// contract.
	if _, err := runtime.StopContainer(ctx, &runtimeapi.StopContainerRequest{
		ContainerId: c.ID,
		Timeout:     stopGracePeriodSeconds,
	}); err != nil && !isCRINotFound(err) {
		return fmt.Errorf("oci: stopping container %q (%s): %w", c.Name, c.ID, err)
	}
	if _, err := runtime.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{
		ContainerId: c.ID,
	}); err != nil && !isCRINotFound(err) {
		return fmt.Errorf("oci: removing container %q (%s): %w", c.Name, c.ID, err)
	}
	return nil
}

func (m *criPodManager) ContainerLogs(ctx context.Context, c Container, follow bool) (io.ReadCloser, error) {
	// CRI carries no log content over the socket — a container's output goes to a file under the
	// sandbox log directory, which the engine must have mounted. This reads that file and strips
	// the K8s framing (`<ts> {stdout|stderr} {F|P} <content>`) so the caller sees the raw combined
	// stream, exactly as docker's ContainerLogs does over the daemon API.
	if m.logDir == "" {
		return nil, fmt.Errorf("oci: cannot read logs for %q: no pod log directory (set %s). "+
			"CRI has no log-read RPC; the engine must have the sandbox log directory mounted", c.Name, CRILogDirEnvVar)
	}
	logPath := m.logPathFor(c.ID)
	if logPath == "" {
		return nil, fmt.Errorf(
			"oci: no log path recorded for container %q (%s); it was not started by this manager", c.Name, c.ID)
	}
	return newCRILogStream(ctx, filepath.Join(m.logDir, logPath), follow)
}

func (m *criPodManager) ImageExists(ctx context.Context, ref string) (bool, error) {
	_, image, err := m.clients()
	if err != nil {
		return false, err
	}
	// ImageStatus returns a nil Image (and no error) when the image is absent. Treat any failure
	// as "not present", as the docker manager does: the caller's recourse is to pull, and a
	// genuine daemon fault resurfaces there with a clearer error.
	resp, err := image.ImageStatus(ctx, &runtimeapi.ImageStatusRequest{Image: &runtimeapi.ImageSpec{Image: ref}})
	if err != nil {
		return false, nil
	}
	return resp.GetImage() != nil, nil
}

func (m *criPodManager) PullImage(ctx context.Context, ref string) error {
	_, image, err := m.clients()
	if err != nil {
		return err
	}
	// PullImage through the CRI image service lands the image in the k8s.io content namespace,
	// which is the one CRI-run containers see — so a pull is namespace-correct for free (unlike a
	// raw `ctr import`, the wrinkle ImportImage still has to settle).
	if _, err := image.PullImage(ctx, &runtimeapi.PullImageRequest{
		Image: &runtimeapi.ImageSpec{Image: ref},
	}); err != nil {
		return fmt.Errorf("oci: pulling %s: %w", ref, err)
	}
	return nil
}

// ImportImage is not yet implemented on CRI. The verb is de-risked but carries a genuine design
// call (spike 2026-07-19): the CRI image service is the containerd `k8s.io` namespace, so an
// import must target it — either `ctr -n k8s.io images import` (one local op, but needs the ctr
// binary and the right namespace) or push the build's layout through the registry proxy and let
// CRI PullImage fetch it (namespace-correct by construction, but push-then-pull plus the pod-mode
// credential surface). Neither is a clear win; it is the user's call, so it is stubbed rather than
// silently picked.
func (m *criPodManager) ImportImage(ctx context.Context, layoutPath, ref string) error {
	return fmt.Errorf("oci: ImportImage is not yet implemented for the CRI runtime (%s -> %s). "+
		"The open choice is `ctr -n k8s.io images import` vs. proxy-pull through the registry router; "+
		"see the CRI PodManager findings", layoutPath, ref)
}

// RunToCompletion is not yet implemented on CRI. It is the build primitive, and building on CRI
// pulls in the still-docker-coupled build mechanism (the `docker build --output type=oci` command
// plus `--volumes-from` and the socket). The CRI-specific part, when built, recovers docker's
// stdout/stderr split by filtering the container log on its stream tag (only `stdout F` lines feed
// the captured ref).
func (m *criPodManager) RunToCompletion(ctx context.Context, cfg ContainerConfig, stderr io.Writer) (string, error) {
	return "", errors.New("oci: RunToCompletion (the build primitive) is not yet implemented for the CRI runtime")
}

// CreateVolume is not yet implemented on CRI. CRI has no named-volume concept; the shared
// workspace becomes a host directory the containers bind by path. Wiring that host-dir mapping is
// the deferred volume work.
func (m *criPodManager) CreateVolume(ctx context.Context, name string) (Volume, error) {
	return Volume{}, errors.New("oci: CreateVolume is not yet implemented for the CRI runtime " +
		"(CRI has no named-volume analog; the workspace becomes a host dir)")
}

// CopyFromImage is not yet implemented on CRI. It seeds a named volume from an image's filesystem;
// on CRI, with the workspace a host dir, the seeding step needs a different mechanism (an explicit
// copy, since there is no create-time volume copy-up).
func (m *criPodManager) CopyFromImage(ctx context.Context, image, srcPath string, vol Volume, dstPath string) error {
	return errors.New("oci: CopyFromImage is not yet implemented for the CRI runtime")
}

// ReadImageFile is not yet implemented on CRI. Docker reads a file out of an image's layers via
// `docker cp` on a created-but-not-started container; CRI has no equivalent, so reading a baked
// manifest without running the image needs a different approach (e.g. pulling the layers and
// reading the content store).
func (m *criPodManager) ReadImageFile(ctx context.Context, image, path string) ([]byte, error) {
	return nil, errors.New("oci: ReadImageFile is not yet implemented for the CRI runtime")
}

func (m *criPodManager) Cleanup(ctx context.Context) error {
	if _, _, err := m.clients(); err != nil {
		return err
	}
	// Snapshot and clear tracked containers under the lock, then tear down without holding it.
	m.mu.Lock()
	containers := m.containers
	m.containers, m.logPaths = nil, map[string]string{}
	m.mu.Unlock()

	var errs []error
	for _, c := range containers {
		if err := m.StopContainer(ctx, c); err != nil {
			errs = append(errs, err)
		}
	}
	// The sandbox is NOT torn down here: the wrapper created it (the engine lives in it) and owns
	// its lifecycle, the same division as the docker network the wrapper creates. Likewise there
	// are no manager-owned volumes to remove.
	if m.conn != nil {
		errs = append(errs, m.conn.Close())
	}
	return errors.Join(errs...)
}

// nextAttempt returns and records the next attempt number for a logical container name, so
// repeated runs of the same name get distinct (name, attempt) pairs and log paths (see attempts).
func (m *criPodManager) nextAttempt(name string) uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.attempts[name]
	m.attempts[name] = a + 1
	return a
}

// logPathFor returns the recorded log path (relative to logDir) for a started container, or "".
func (m *criPodManager) logPathFor(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logPaths[id]
}

func (m *criPodManager) track(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn()
}

// criLogPath is a container's log file path relative to the sandbox log directory. It embeds the
// attempt so a re-run's log does not collide with the prior one. A flat name (rather than the
// kubelet `<name>/<attempt>.log` subdir form) avoids needing to pre-create a per-container
// directory before the runtime opens the file.
func criLogPath(name string, attempt uint32) string {
	return fmt.Sprintf("%s_%d.log", name, attempt)
}

// criEnv converts an env map to CRI KeyValues, in sorted key order so the request is deterministic
// (and testable).
func criEnv(env map[string]string) []*runtimeapi.KeyValue {
	if len(env) == 0 {
		return nil
	}
	kvs := make([]*runtimeapi.KeyValue, 0, len(env))
	for _, k := range sortedKeys(env) {
		kvs = append(kvs, &runtimeapi.KeyValue{Key: k, Value: env[k]})
	}
	return kvs
}

// criMounts converts VolumeMounts to CRI Mounts. CRI mounts are host-path binds, so Source maps to
// HostPath directly — which is correct for an absolute host path but not for a named volume, whose
// host-dir mapping is the deferred volume work.
func criMounts(vols []VolumeMount) []*runtimeapi.Mount {
	if len(vols) == 0 {
		return nil
	}
	mounts := make([]*runtimeapi.Mount, 0, len(vols))
	for _, v := range vols {
		mounts = append(mounts, &runtimeapi.Mount{
			HostPath:      v.Source,
			ContainerPath: v.Target,
			Readonly:      v.ReadOnly,
		})
	}
	return mounts
}

// isCRINotFound reports whether err is a CRI "not found", which StopContainer/RemoveContainer
// treat as success. containerd returns the gRPC NotFound code for a missing container; fall back
// to a string check for runtimes that only phrase it in the message.
func isCRINotFound(err error) bool {
	if err == nil {
		return false
	}
	if status.Code(err) == codes.NotFound {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

// envOr returns the environment variable value, or def if it is unset or empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var _ PodManager = (*criPodManager)(nil)
