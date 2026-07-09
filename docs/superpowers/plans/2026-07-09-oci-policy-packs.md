# OCI Policy Packs (`runtime: oci`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run policy packs as digest-pinned container images: the engine launches the pack's container (docker/podman/nerdctl) and dials its analyzer gRPC port instead of booting a language host; `pulumi policy publish` registers a digest-pinned image ref instead of uploading a tarball.

**Architecture:** A third boot path inside `plugin.NewPolicyAnalyzer` delegates to a new container-launcher package `pkg/resource/plugin/oci`. The digest ref for server-enforced packs flows programmatically via a new `PolicyAnalyzerOptions.ImageRef` field (never via the manifest). Attach mode (`PULUMI_POLICY_PACK_ATTACH`) mirrors the existing `PULUMI_DEBUG_PROVIDERS` mechanism. Readiness is gRPC-level retry-dial in all modes.

**Tech Stack:** Go, gRPC (`pulumirpc.Analyzer`), container runtime CLIs (docker/podman/nerdctl — shelled out, **no new Go dependencies**).

**Spec:** `docs/superpowers/specs/2026-07-09-oci-policy-packs-design.md` — read it before starting any task.

## Global Constraints

- All commands run from the repo root; prefix `make` with `mise exec --`.
- After any `.go` change: `mise exec -- make format && mise exec -- make lint` must pass.
- No new Go module dependencies (registry access shells out to the container runtime CLI).
- New files get copyright headers stamped `2026` (copy the header from any 2026 file, e.g. `pkg/engine/policypacks.go`).
- Container port constant inside the pack container: `20851`. Env var contract: `PULUMI_POLICY_PORT`.
- Attach env var: `PULUMI_POLICY_PACK_ATTACH=<pack-name>:<port>[,<pack-name>:<port>…]`.
- Runtime override env var: `PULUMI_CONTAINER_RUNTIME`; probe order `docker`, `podman`, `nerdctl`.
- Errors must be loud and actionable — never silently skip an oci pack.
- Unit tests that execute stub shell scripts must `t.Skip` on Windows (`runtime.GOOS == "windows"`).
- Changelog entries: one sentence, user-visible behavior, no trailing punctuation.
- This work touches the public CLI surface (`--tag` flag) and the engine — both were explicitly approved in the spec; do not expand the surface beyond what tasks state.

---

### Task 1: Container runtime detection (`pkg/resource/plugin/oci`)

**Files:**
- Create: `pkg/resource/plugin/oci/runtime.go`
- Test: `pkg/resource/plugin/oci/runtime_test.go`

**Interfaces:**
- Produces: `oci.DetectRuntime(lookPath func(string) (string, error)) (*Runtime, error)`, `type Runtime struct { Path, Name string }`, `oci.ErrNoContainerRuntime`, const `oci.EnvContainerRuntime = "PULUMI_CONTAINER_RUNTIME"`. Pass `nil` for `lookPath` to use `exec.LookPath`.

- [ ] **Step 1: Write the failing test**

```go
// pkg/resource/plugin/oci/runtime_test.go
package oci

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeLookPath(available map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if p, ok := available[name]; ok {
			return p, nil
		}
		return "", errors.New("not found")
	}
}

func TestDetectRuntimeProbeOrder(t *testing.T) {
	t.Parallel()
	rt, err := DetectRuntime(fakeLookPath(map[string]string{
		"podman": "/usr/bin/podman",
		"docker": "/usr/bin/docker",
	}))
	require.NoError(t, err)
	assert.Equal(t, "docker", rt.Name)
	assert.Equal(t, "/usr/bin/docker", rt.Path)
}

func TestDetectRuntimeFallsBack(t *testing.T) {
	t.Parallel()
	rt, err := DetectRuntime(fakeLookPath(map[string]string{"nerdctl": "/usr/bin/nerdctl"}))
	require.NoError(t, err)
	assert.Equal(t, "nerdctl", rt.Name)
}

func TestDetectRuntimeNoneFound(t *testing.T) {
	t.Parallel()
	_, err := DetectRuntime(fakeLookPath(nil))
	require.ErrorIs(t, err, ErrNoContainerRuntime)
}

func TestDetectRuntimeEnvOverride(t *testing.T) {
	t.Setenv(EnvContainerRuntime, "podman")
	rt, err := DetectRuntime(fakeLookPath(map[string]string{
		"podman": "/opt/podman",
		"docker": "/usr/bin/docker",
	}))
	require.NoError(t, err)
	assert.Equal(t, "podman", rt.Name)
	assert.Equal(t, "/opt/podman", rt.Path)
}

func TestDetectRuntimeEnvOverrideMissing(t *testing.T) {
	t.Setenv(EnvContainerRuntime, "podman")
	_, err := DetectRuntime(fakeLookPath(map[string]string{"docker": "/usr/bin/docker"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "podman")
	assert.Contains(t, err.Error(), EnvContainerRuntime)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd pkg && go test ./resource/plugin/oci/ -run TestDetectRuntime -v`
Expected: FAIL (package does not exist / undefined symbols)

- [ ] **Step 3: Write the implementation**

```go
// pkg/resource/plugin/oci/runtime.go
// [2026 copyright header]

// Package oci launches policy packs published as OCI container images. It
// shells out to a container runtime CLI (docker, podman, or nerdctl) rather
// than linking a registry or daemon client.
package oci

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// EnvContainerRuntime overrides container runtime detection with a specific
// CLI name (or path) to use.
const EnvContainerRuntime = "PULUMI_CONTAINER_RUNTIME"

var probeOrder = []string{"docker", "podman", "nerdctl"}

// ErrNoContainerRuntime is returned when no supported container runtime CLI is
// found on PATH.
var ErrNoContainerRuntime = errors.New(
	"no container runtime found: running a policy pack with runtime \"oci\" requires docker, podman, " +
		"or nerdctl on PATH (or set " + EnvContainerRuntime + " to the runtime to use)")

// Runtime is a resolved container runtime CLI.
type Runtime struct {
	Path string // absolute path to the CLI binary
	Name string // the CLI name, e.g. "docker"
}

// DetectRuntime resolves the container runtime CLI to use. lookPath may be nil
// to use exec.LookPath; tests inject a fake.
func DetectRuntime(lookPath func(string) (string, error)) (*Runtime, error) {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if override := os.Getenv(EnvContainerRuntime); override != "" {
		path, err := lookPath(override)
		if err != nil {
			return nil, fmt.Errorf("container runtime %q (from %s) not found on PATH: %w",
				override, EnvContainerRuntime, err)
		}
		return &Runtime{Path: path, Name: override}, nil
	}
	for _, name := range probeOrder {
		if path, err := lookPath(name); err == nil {
			return &Runtime{Path: path, Name: name}, nil
		}
	}
	return nil, ErrNoContainerRuntime
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd pkg && go test ./resource/plugin/oci/ -run TestDetectRuntime -v`
Expected: PASS (all five tests)

- [ ] **Step 5: Format, lint, commit**

```bash
mise exec -- make format && mise exec -- make lint
git add pkg/resource/plugin/oci/
git commit -m "Add container runtime detection for OCI policy packs"
```

---

### Task 2: Networking mode detection (`pkg/resource/plugin/oci`)

**Files:**
- Create: `pkg/resource/plugin/oci/mode.go`
- Test: `pkg/resource/plugin/oci/mode_test.go`

**Interfaces:**
- Produces: `type Mode int` with `ModeHost`/`ModeSibling`; `DetectMode() Mode` (production entry); `detectMode(getenv func(string) string, fileExists func(string) bool) Mode` (testable core); consts `EnvNetworkMode = "PULUMI_POLICY_CONTAINER_NETWORK"`, `EnvSelfContainerID = "PULUMI_SELF_CONTAINER_ID"`.

- [ ] **Step 1: Write the failing test**

```go
// pkg/resource/plugin/oci/mode_test.go
package oci

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func files(fs ...string) func(string) bool {
	set := map[string]bool{}
	for _, f := range fs {
		set[f] = true
	}
	return func(p string) bool { return set[p] }
}

func TestDetectModeOnHost(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ModeHost, detectMode(env(nil), files()))
}

func TestDetectModeInContainerWithSocket(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ModeSibling, detectMode(env(nil), files("/.dockerenv", "/var/run/docker.sock")))
	assert.Equal(t, ModeSibling, detectMode(env(nil), files("/run/.containerenv", "/var/run/docker.sock")))
}

func TestDetectModeInContainerWithDockerHost(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ModeSibling,
		detectMode(env(map[string]string{"DOCKER_HOST": "unix:///x.sock"}), files("/.dockerenv")))
}

func TestDetectModeInContainerNoSocket(t *testing.T) {
	t.Parallel()
	// In a container with no socket we still return ModeHost; the runtime
	// probe will then fail loudly (no docker CLI / no daemon).
	assert.Equal(t, ModeHost, detectMode(env(nil), files("/.dockerenv")))
}

func TestDetectModeEnvOverride(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ModeSibling, detectMode(env(map[string]string{EnvNetworkMode: "sibling"}), files()))
	assert.Equal(t, ModeHost,
		detectMode(env(map[string]string{EnvNetworkMode: "host"}), files("/.dockerenv", "/var/run/docker.sock")))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd pkg && go test ./resource/plugin/oci/ -run TestDetectMode -v`
Expected: FAIL (undefined: detectMode, ModeHost, …)

- [ ] **Step 3: Write the implementation**

```go
// pkg/resource/plugin/oci/mode.go
// [2026 copyright header]

package oci

import "os"

// Mode selects how the engine reaches a policy pack container's port.
type Mode int

const (
	// ModeHost is used when the CLI runs directly on a host: the container's
	// analyzer port is published to the host loopback interface.
	ModeHost Mode = iota
	// ModeSibling is used when the CLI itself runs inside a container with a
	// runtime socket (e.g. the Deployments executor): the pack container joins
	// the CLI container's network namespace and is dialed on loopback.
	ModeSibling
)

// EnvNetworkMode overrides networking mode detection ("host" or "sibling").
const EnvNetworkMode = "PULUMI_POLICY_CONTAINER_NETWORK"

// EnvSelfContainerID overrides the container ID used for sibling networking
// (defaults to the hostname, which container runtimes set to the container ID).
const EnvSelfContainerID = "PULUMI_SELF_CONTAINER_ID"

// DetectMode determines the networking mode for the current process.
func DetectMode() Mode {
	return detectMode(os.Getenv, func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	})
}

func detectMode(getenv func(string) string, fileExists func(string) bool) Mode {
	switch getenv(EnvNetworkMode) {
	case "host":
		return ModeHost
	case "sibling":
		return ModeSibling
	}
	inContainer := fileExists("/.dockerenv") || fileExists("/run/.containerenv")
	if !inContainer {
		return ModeHost
	}
	if getenv("DOCKER_HOST") != "" || fileExists("/var/run/docker.sock") {
		return ModeSibling
	}
	return ModeHost
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd pkg && go test ./resource/plugin/oci/ -run TestDetectMode -v`
Expected: PASS

- [ ] **Step 5: Format, lint, commit**

```bash
mise exec -- make format && mise exec -- make lint
git add pkg/resource/plugin/oci/
git commit -m "Add networking mode detection for OCI policy packs"
```

---

### Task 3: Container launcher (`pkg/resource/plugin/oci`)

**Files:**
- Create: `pkg/resource/plugin/oci/launcher.go`
- Test: `pkg/resource/plugin/oci/launcher_test.go`

**Interfaces:**
- Consumes: `Runtime` (Task 1), `Mode` (Task 2).
- Produces:
  - const `ContainerPort = 20851`, const `EnvPolicyPort = "PULUMI_POLICY_PORT"`, const `LabelKey = "com.pulumi.policy-pack"`
  - `type LaunchOptions struct { Image, PackName string; Env map[string]string; Mode Mode; SelfContainerID string }`
  - `func (r *Runtime) Launch(ctx context.Context, opts LaunchOptions) (*Container, error)`
  - `type Container struct { Addr string /* "127.0.0.1:<port>" to dial */ ; ... }` with methods `Close() error`, `Running(ctx context.Context) bool`, `Logs(ctx context.Context) string`
  - `func EngineAddressFor(mode Mode, engineAddr string) string` — rewrites host to `host.docker.internal` in `ModeHost`, identity otherwise.

- [ ] **Step 1: Write the failing test**

The tests drive `Launch` against a stub runtime script that records argv and plays back canned responses. Skip on Windows.

```go
// pkg/resource/plugin/oci/launcher_test.go
package oci

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRuntime writes a shell script that records each invocation's argv (one
// line per call) to recordFile and responds per subcommand.
func stubRuntime(t *testing.T) (*Runtime, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub runtime scripts are not supported on Windows")
	}
	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	script := filepath.Join(dir, "docker")
	err := os.WriteFile(script, []byte(`#!/bin/sh
echo "$@" >> "`+record+`"
case "$1" in
  run) echo "cid0123456789" ;;
  port) echo "127.0.0.1:49321" ;;
  inspect) echo "true" ;;
  logs) echo "pack logs here" ;;
  stop) : ;;
esac
`), 0o700)
	require.NoError(t, err)
	return &Runtime{Path: script, Name: "docker"}, record
}

func recorded(t *testing.T, record string) []string {
	t.Helper()
	b, err := os.ReadFile(record)
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestLaunchHostMode(t *testing.T) {
	rt, record := stubRuntime(t)
	c, err := rt.Launch(context.Background(), LaunchOptions{
		Image:    "ghcr.io/acme/pack@sha256:abc",
		PackName: "security",
		Mode:     ModeHost,
	})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:49321", c.Addr)

	calls := recorded(t, record)
	require.Len(t, calls, 2) // run, port
	runCall := calls[0]
	assert.Contains(t, runCall, "run --detach --rm --pull=never")
	assert.Contains(t, runCall, "-e PULUMI_POLICY_PORT=20851")
	assert.Contains(t, runCall, "-p 127.0.0.1:0:20851")
	assert.Contains(t, runCall, "--add-host=host.docker.internal:host-gateway")
	assert.Contains(t, runCall, "--label com.pulumi.policy-pack=security")
	assert.True(t, strings.HasSuffix(runCall, "ghcr.io/acme/pack@sha256:abc"))
	assert.Contains(t, calls[1], "port cid0123456789 20851")
}

func TestLaunchSiblingMode(t *testing.T) {
	rt, record := stubRuntime(t)
	c, err := rt.Launch(context.Background(), LaunchOptions{
		Image:           "ghcr.io/acme/pack@sha256:abc",
		PackName:        "security",
		Mode:            ModeSibling,
		SelfContainerID: "selfctr",
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(c.Addr, "127.0.0.1:"))

	calls := recorded(t, record)
	require.Len(t, calls, 1) // run only; no port mapping to discover
	assert.Contains(t, calls[0], "--network container:selfctr")
	assert.NotContains(t, calls[0], "-p 127.0.0.1")
	assert.NotContains(t, calls[0], "--add-host")
	// The chosen free port is passed into the container.
	assert.Contains(t, calls[0], "-e PULUMI_POLICY_PORT=")
}

func TestLaunchPassesEnvSorted(t *testing.T) {
	rt, record := stubRuntime(t)
	_, err := rt.Launch(context.Background(), LaunchOptions{
		Image: "img", PackName: "p", Mode: ModeHost,
		Env: map[string]string{"B_VAR": "2", "A_VAR": "1"},
	})
	require.NoError(t, err)
	runCall := recorded(t, record)[0]
	assert.Less(t, strings.Index(runCall, "A_VAR=1"), strings.Index(runCall, "B_VAR=2"))
}

func TestContainerLifecycle(t *testing.T) {
	rt, record := stubRuntime(t)
	c, err := rt.Launch(context.Background(), LaunchOptions{Image: "img", PackName: "p", Mode: ModeHost})
	require.NoError(t, err)
	assert.True(t, c.Running(context.Background()))
	assert.Equal(t, "pack logs here", strings.TrimSpace(c.Logs(context.Background())))
	require.NoError(t, c.Close())
	calls := recorded(t, record)
	assert.Contains(t, calls[len(calls)-1], "stop")
	assert.Contains(t, calls[len(calls)-1], "cid0123456789")
}

func TestLaunchRunFailureIncludesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub runtime scripts are not supported on Windows")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "docker")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/sh
echo "manifest for img not found" >&2
exit 125
`), 0o700))
	rt := &Runtime{Path: script, Name: "docker"}
	_, err := rt.Launch(context.Background(), LaunchOptions{Image: "img", PackName: "p", Mode: ModeHost})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest for img not found")
}

func TestEngineAddressFor(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "host.docker.internal:5005", EngineAddressFor(ModeHost, "127.0.0.1:5005"))
	assert.Equal(t, "127.0.0.1:5005", EngineAddressFor(ModeSibling, "127.0.0.1:5005"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd pkg && go test ./resource/plugin/oci/ -run 'TestLaunch|TestContainer|TestEngineAddress' -v`
Expected: FAIL (undefined symbols)

- [ ] **Step 3: Write the implementation**

```go
// pkg/resource/plugin/oci/launcher.go
// [2026 copyright header]

package oci

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// ContainerPort is the fixed port the pack's analyzer listens on inside the
// container network namespace in host mode (communicated via EnvPolicyPort).
const ContainerPort = 20851

// EnvPolicyPort tells a policy pack which port to serve its analyzer on.
const EnvPolicyPort = "PULUMI_POLICY_PORT"

// LabelKey labels containers launched for policy packs.
const LabelKey = "com.pulumi.policy-pack"

// LaunchOptions configures a policy pack container launch.
type LaunchOptions struct {
	Image           string            // image ref to run (never pulled implicitly)
	PackName        string            // for container naming/labels and errors
	Env             map[string]string // additional environment for the pack
	Mode            Mode
	SelfContainerID string // sibling mode: container to share a netns with; defaults to hostname
}

// Container is a running policy pack container.
type Container struct {
	rt   *Runtime
	id   string
	Addr string // host-reachable analyzer address, "127.0.0.1:<port>"
}

// run executes the runtime CLI, returning combined output.
func (r *Runtime) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, r.Path, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", r.Name, strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Launch starts the pack container and returns it with a dialable address.
// The image is never pulled implicitly (--pull=never): local packs must be
// built first, and enforced packs are pulled at install time.
func (r *Runtime) Launch(ctx context.Context, opts LaunchOptions) (*Container, error) {
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	name := fmt.Sprintf("pulumi-policy-%s-%s", sanitizeName(opts.PackName), hex.EncodeToString(suffix))

	args := []string{
		"run", "--detach", "--rm", "--pull=never",
		"--name", name,
		"--label", LabelKey + "=" + sanitizeName(opts.PackName),
	}

	var dialPort int
	switch opts.Mode {
	case ModeHost:
		args = append(args,
			"-e", fmt.Sprintf("%s=%d", EnvPolicyPort, ContainerPort),
			"-p", fmt.Sprintf("127.0.0.1:0:%d", ContainerPort),
			"--add-host=host.docker.internal:host-gateway",
		)
	case ModeSibling:
		port, err := freePort()
		if err != nil {
			return nil, fmt.Errorf("finding a free port for policy pack %q: %w", opts.PackName, err)
		}
		dialPort = port
		self := opts.SelfContainerID
		if self == "" {
			self = os.Getenv(EnvSelfContainerID)
		}
		if self == "" {
			self, _ = os.Hostname()
		}
		args = append(args,
			"-e", fmt.Sprintf("%s=%d", EnvPolicyPort, port),
			"--network", "container:"+self,
		)
	}

	keys := make([]string, 0, len(opts.Env))
	for k := range opts.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", k+"="+opts.Env[k])
	}

	args = append(args, opts.Image)

	id, err := r.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to start container for policy pack %q (image %s): %w",
			opts.PackName, opts.Image, err)
	}

	c := &Container{rt: r, id: id}
	if opts.Mode == ModeHost {
		out, err := r.run(ctx, "port", id, fmt.Sprintf("%d", ContainerPort))
		if err != nil {
			_ = c.Close()
			return nil, fmt.Errorf("discovering mapped port for policy pack %q: %w", opts.PackName, err)
		}
		// Output may span multiple lines (ipv4+ipv6); take the first.
		c.Addr = strings.TrimSpace(strings.Split(out, "\n")[0])
		// nerdctl/podman may print "0.0.0.0:49321"; dial loopback regardless.
		if _, port, splitErr := net.SplitHostPort(c.Addr); splitErr == nil {
			c.Addr = net.JoinHostPort("127.0.0.1", port)
		}
	} else {
		c.Addr = fmt.Sprintf("127.0.0.1:%d", dialPort)
	}
	return c, nil
}

// Running reports whether the container is still running.
func (c *Container) Running(ctx context.Context) bool {
	out, err := c.rt.run(ctx, "inspect", "--format", "{{.State.Running}}", c.id)
	return err == nil && strings.TrimSpace(out) == "true"
}

// Logs returns the container's combined logs (best-effort).
func (c *Container) Logs(ctx context.Context) string {
	out, err := c.rt.run(ctx, "logs", c.id)
	if err != nil {
		return fmt.Sprintf("(could not fetch container logs: %v)", err)
	}
	return out
}

// Close stops the container. The container was started with --rm, so stopping
// also removes it. A missing container (already exited) is not an error.
func (c *Container) Close() error {
	_, err := c.rt.run(context.Background(), "stop", "--time", "2", c.id)
	if err != nil && strings.Contains(err.Error(), "No such container") {
		return nil
	}
	return err
}

// EngineAddressFor rewrites the engine's gRPC address for reachability from
// inside a pack container. In host mode the container cannot reach the host's
// loopback, so the host is rewritten to host.docker.internal (mapped via
// --add-host=host-gateway at launch). In sibling/attach modes the namespace is
// shared and the address passes through unchanged.
func EngineAddressFor(mode Mode, engineAddr string) string {
	if mode != ModeHost {
		return engineAddr
	}
	_, port, err := net.SplitHostPort(engineAddr)
	if err != nil {
		return engineAddr
	}
	return net.JoinHostPort("host.docker.internal", port)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd pkg && go test ./resource/plugin/oci/ -v`
Expected: PASS (all tests in the package)

- [ ] **Step 5: Format, lint, commit**

```bash
mise exec -- make format && mise exec -- make lint
git add pkg/resource/plugin/oci/
git commit -m "Add container launcher for OCI policy packs"
```

---

### Task 4: Registry operations — Pull, ResolveDigest, HasPlatform

**Files:**
- Create: `pkg/resource/plugin/oci/registry.go`
- Test: `pkg/resource/plugin/oci/registry_test.go`

**Interfaces:**
- Consumes: `Runtime` (Task 1).
- Produces:
  - `func (r *Runtime) Pull(ctx context.Context, ref string, output io.Writer) error` — streams pull progress to `output`.
  - `func (r *Runtime) ResolveDigest(ctx context.Context, ref string) (string, error)` — returns the digest-pinned ref (`repo@sha256:…`) for a pulled tagged ref.
  - `func (r *Runtime) HasPlatform(ctx context.Context, ref, platform string) (bool, error)` — checks `platform` (e.g. `"linux/amd64"`) against `manifest inspect`; falls back to `image inspect` for single-platform images.

- [ ] **Step 1: Write the failing test**

```go
// pkg/resource/plugin/oci/registry_test.go
package oci

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scriptRuntime(t *testing.T, script string) *Runtime {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub runtime scripts are not supported on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "docker")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o700))
	return &Runtime{Path: path, Name: "docker"}
}

func TestPullStreamsOutput(t *testing.T) {
	rt := scriptRuntime(t, `#!/bin/sh
echo "Pulling from acme/pack"
`)
	var buf bytes.Buffer
	require.NoError(t, rt.Pull(context.Background(), "ghcr.io/acme/pack:1.0.0", &buf))
	assert.Contains(t, buf.String(), "Pulling from acme/pack")
}

func TestPullFailure(t *testing.T) {
	rt := scriptRuntime(t, `#!/bin/sh
echo "pull access denied" >&2
exit 1
`)
	err := rt.Pull(context.Background(), "ghcr.io/acme/private:1.0.0", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghcr.io/acme/private:1.0.0")
}

func TestResolveDigest(t *testing.T) {
	rt := scriptRuntime(t, `#!/bin/sh
echo "ghcr.io/acme/pack@sha256:deadbeef"
`)
	ref, err := rt.ResolveDigest(context.Background(), "ghcr.io/acme/pack:1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/acme/pack@sha256:deadbeef", ref)
}

func TestHasPlatformManifestList(t *testing.T) {
	rt := scriptRuntime(t, `#!/bin/sh
case "$1" in
  manifest) cat <<'EOF'
{"manifests":[{"platform":{"os":"linux","architecture":"amd64"}},{"platform":{"os":"linux","architecture":"arm64"}}]}
EOF
;;
esac
`)
	ok, err := rt.HasPlatform(context.Background(), "ref", "linux/amd64")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = rt.HasPlatform(context.Background(), "ref", "windows/amd64")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestHasPlatformSingleImageFallback(t *testing.T) {
	// manifest inspect returns a single-image manifest (no "manifests" list);
	// fall back to image inspect of the pulled image.
	rt := scriptRuntime(t, `#!/bin/sh
case "$1" in
  manifest) echo '{"schemaVersion":2,"config":{}}' ;;
  image) echo "linux/arm64" ;;
esac
`)
	ok, err := rt.HasPlatform(context.Background(), "ref", "linux/amd64")
	require.NoError(t, err)
	assert.False(t, ok)
	ok, err = rt.HasPlatform(context.Background(), "ref", "linux/arm64")
	require.NoError(t, err)
	assert.True(t, ok)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd pkg && go test ./resource/plugin/oci/ -run 'TestPull|TestResolveDigest|TestHasPlatform' -v`
Expected: FAIL (undefined symbols)

- [ ] **Step 3: Write the implementation**

```go
// pkg/resource/plugin/oci/registry.go
// [2026 copyright header]

package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Pull pulls an image ref, streaming the runtime's progress output to output.
// Auth-shaped failures get a note: private registries are not yet supported.
func (r *Runtime) Pull(ctx context.Context, ref string, output io.Writer) error {
	cmd := exec.CommandContext(ctx, r.Path, "pull", ref)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pulling policy pack image %s failed: %w "+
			"(note: private registries are not yet supported for policy packs — "+
			"the image must be pullable anonymously)", ref, err)
	}
	return nil
}

// ResolveDigest returns the digest-pinned ref for a tagged ref that has
// already been pulled, e.g. "ghcr.io/acme/pack@sha256:…".
func (r *Runtime) ResolveDigest(ctx context.Context, ref string) (string, error) {
	out, err := r.run(ctx, "image", "inspect", "--format", "{{index .RepoDigests 0}}", ref)
	if err != nil {
		return "", fmt.Errorf("resolving digest for %s: %w", ref, err)
	}
	digestRef := strings.TrimSpace(out)
	if digestRef == "" || !strings.Contains(digestRef, "@sha256:") {
		return "", fmt.Errorf("could not resolve a registry digest for %s: got %q "+
			"(has the image been pushed to its registry?)", ref, digestRef)
	}
	return digestRef, nil
}

// HasPlatform reports whether the (pulled) ref supports platform ("os/arch").
// Multi-arch refs are checked via `manifest inspect`; single-platform images
// fall back to the pulled image's os/architecture.
func (r *Runtime) HasPlatform(ctx context.Context, ref, platform string) (bool, error) {
	out, err := r.run(ctx, "manifest", "inspect", ref)
	if err == nil {
		var manifest struct {
			Manifests []struct {
				Platform struct {
					OS           string `json:"os"`
					Architecture string `json:"architecture"`
				} `json:"platform"`
			} `json:"manifests"`
		}
		if jsonErr := json.Unmarshal([]byte(out), &manifest); jsonErr == nil && len(manifest.Manifests) > 0 {
			for _, m := range manifest.Manifests {
				if m.Platform.OS+"/"+m.Platform.Architecture == platform {
					return true, nil
				}
			}
			return false, nil
		}
	}
	// Single-platform image (or a runtime without manifest support): inspect
	// the pulled image itself.
	out, err = r.run(ctx, "image", "inspect", "--format", "{{.Os}}/{{.Architecture}}", ref)
	if err != nil {
		return false, fmt.Errorf("checking platforms for %s: %w", ref, err)
	}
	return strings.TrimSpace(out) == platform, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd pkg && go test ./resource/plugin/oci/ -v`
Expected: PASS

- [ ] **Step 5: Format, lint, commit**

```bash
mise exec -- make format && mise exec -- make lint
git add pkg/resource/plugin/oci/
git commit -m "Add registry operations for OCI policy packs"
```

---

### Task 5: Attach-mode parser and retry-dial readiness (`pkg/resource/plugin`)

**Files:**
- Create: `pkg/resource/plugin/analyzer_oci.go`
- Test: `pkg/resource/plugin/analyzer_oci_test.go`

**Interfaces:**
- Consumes: nothing new (gRPC, `rpcutil` already imported by the package).
- Produces (package `plugin`):
  - const `EnvPolicyPackAttach = "PULUMI_POLICY_PACK_ATTACH"`
  - `func GetPolicyPackAttachPort(name tokens.QName) (*int, error)` — parses `PULUMI_POLICY_PACK_ATTACH=<name>:<port>[,…]`, nil if the pack isn't listed.
  - `func dialAnalyzerWithRetry(ctx context.Context, addr string, timeout time.Duration, containerCheck func() (running bool, logs string)) (*grpc.ClientConn, error)` — retry-dial until the gRPC channel is READY; if `containerCheck` is non-nil and reports not-running, fail fast with the logs.

- [ ] **Step 1: Write the failing test**

```go
// pkg/resource/plugin/analyzer_oci_test.go
package plugin

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestGetPolicyPackAttachPort(t *testing.T) {
	t.Setenv(EnvPolicyPackAttach, "security:1234,cost-controls:5678")

	port, err := GetPolicyPackAttachPort("security")
	require.NoError(t, err)
	require.NotNil(t, port)
	assert.Equal(t, 1234, *port)

	port, err = GetPolicyPackAttachPort("cost-controls")
	require.NoError(t, err)
	require.NotNil(t, port)
	assert.Equal(t, 5678, *port)

	port, err = GetPolicyPackAttachPort("unlisted")
	require.NoError(t, err)
	assert.Nil(t, port)
}

func TestGetPolicyPackAttachPortBadPort(t *testing.T) {
	t.Setenv(EnvPolicyPackAttach, "security:not-a-port")
	_, err := GetPolicyPackAttachPort("security")
	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvPolicyPackAttach)
}

func TestGetPolicyPackAttachPortUnset(t *testing.T) {
	port, err := GetPolicyPackAttachPort("security")
	require.NoError(t, err)
	assert.Nil(t, port)
}

// fakeAnalyzerServer is a minimal in-process analyzer gRPC server.
type fakeAnalyzerServer struct {
	pulumirpc.UnimplementedAnalyzerServer
}

func startFakeAnalyzer(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	pulumirpc.RegisterAnalyzerServer(srv, &fakeAnalyzerServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func TestDialAnalyzerWithRetrySucceeds(t *testing.T) {
	t.Parallel()
	addr := startFakeAnalyzer(t)
	conn, err := dialAnalyzerWithRetry(context.Background(), addr, 5*time.Second, nil)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func TestDialAnalyzerWithRetryTimesOut(t *testing.T) {
	t.Parallel()
	// A port with nothing listening.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	_, err = dialAnalyzerWithRetry(context.Background(), addr, 500*time.Millisecond, nil)
	require.Error(t, err)
}

func TestDialAnalyzerWithRetryFailsFastOnExit(t *testing.T) {
	t.Parallel()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	start := time.Now()
	_, err = dialAnalyzerWithRetry(context.Background(), addr, 30*time.Second,
		func() (bool, string) { return false, "container crashed: OOM" })
	require.Error(t, err)
	assert.Contains(t, fmt.Sprintf("%v", err), "container crashed: OOM")
	assert.Less(t, time.Since(start), 10*time.Second, "should fail fast, not wait out the timeout")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd pkg && go test ./resource/plugin/ -run 'TestGetPolicyPackAttach|TestDialAnalyzer' -v`
Expected: FAIL (undefined symbols)

- [ ] **Step 3: Write the implementation**

```go
// pkg/resource/plugin/analyzer_oci.go
// [2026 copyright header]

package plugin

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
)

// EnvPolicyPackAttach lists policy packs the engine should attach to at a
// known port instead of launching, in the form "<pack-name>:<port>[,…]".
// This is the policy pack analogue of PULUMI_DEBUG_PROVIDERS: it is how packs
// running as pod sidecars are reached, and a debugging affordance for pack
// authors (run the pack under a debugger and point the CLI at it).
const EnvPolicyPackAttach = "PULUMI_POLICY_PACK_ATTACH"

// GetPolicyPackAttachPort returns the attach port for the named policy pack
// from EnvPolicyPackAttach, or nil if the pack is not listed.
func GetPolicyPackAttachPort(name tokens.QName) (*int, error) {
	envVar, has := os.LookupEnv(EnvPolicyPackAttach)
	if !has {
		return nil, nil
	}
	for _, entry := range strings.Split(envVar, ",") {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 || parts[0] != string(name) {
			continue
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("expected a numeric port for %q in %s, got %q: %w",
				parts[0], EnvPolicyPackAttach, parts[1], err)
		}
		return &port, nil
	}
	return nil, nil
}

// dialAnalyzerWithRetry dials addr and waits until the gRPC channel is READY,
// retrying transient failures until timeout. A raw TCP connect is not
// sufficient readiness — container runtimes bind the host port before the pack
// is listening — so we require the channel itself to become READY. If
// containerCheck is non-nil and reports the container has exited, we fail fast
// with its logs instead of waiting out the timeout.
func dialAnalyzerWithRetry(
	ctx context.Context, addr string, timeout time.Duration,
	containerCheck func() (running bool, logs string),
) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create connection to policy pack at %s: %w", addr, err)
	}

	deadline := time.Now().Add(timeout)
	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return conn, nil
		}
		if containerCheck != nil {
			if running, logs := containerCheck(); !running {
				contractIgnoreClose(conn)
				return nil, fmt.Errorf("policy pack container exited before serving its analyzer; container logs:\n%s", logs)
			}
		}
		waitCtx, cancel := context.WithDeadline(ctx, deadline)
		changed := conn.WaitForStateChange(waitCtx, state)
		cancel()
		if !changed {
			var logs string
			if containerCheck != nil {
				_, logs = containerCheck()
				logs = "; container logs:\n" + logs
			}
			contractIgnoreClose(conn)
			return nil, fmt.Errorf("timed out after %v waiting for policy pack analyzer at %s%s",
				timeout, addr, logs)
		}
	}
}

func contractIgnoreClose(conn *grpc.ClientConn) {
	_ = conn.Close()
}
```

Note: if the package already imports `contract` (`sdk/v3/go/common/util/contract`), use `contract.IgnoreClose(conn)` and delete the local `contractIgnoreClose` helper — check the imports in `analyzer_plugin.go` (it does import `contract`).

- [ ] **Step 4: Run test to verify it passes**

Run: `cd pkg && go test ./resource/plugin/ -run 'TestGetPolicyPackAttach|TestDialAnalyzer' -v`
Expected: PASS

- [ ] **Step 5: Format, lint, commit**

```bash
mise exec -- make format && mise exec -- make lint
git add pkg/resource/plugin/analyzer_oci.go pkg/resource/plugin/analyzer_oci_test.go
git commit -m "Add policy pack attach-port parsing and analyzer retry-dial"
```

---

### Task 6: Wire OCI and attach boot paths into `NewPolicyAnalyzer`

**Files:**
- Modify: `pkg/resource/plugin/analyzer_plugin.go` (`NewPolicyAnalyzer` at :104, `analyzer` struct at :50, `Close` at :661)
- Modify: `pkg/resource/plugin/host.go` (`PolicyAnalyzerOptions` at :318)
- Modify: `pkg/resource/plugin/analyzer_oci.go` (add the boot functions)
- Test: `pkg/resource/plugin/analyzer_oci_test.go` (extend)

**Interfaces:**
- Consumes: `oci.DetectRuntime`, `oci.DetectMode`, `Runtime.Launch`, `Container.{Addr,Running,Logs,Close}`, `oci.EngineAddressFor`, `GetPolicyPackAttachPort`, `dialAnalyzerWithRetry` (Tasks 1–5); existing `handshake` pattern, `configureStack` logic, `analyzer` struct.
- Produces:
  - `PolicyAnalyzerOptions.ImageRef string` — when set, `NewPolicyAnalyzer` boots the pack from this image ref without loading a local manifest. **Later tasks (7, 9) depend on this exact field name.**
  - `NewPolicyAnalyzer` behavior: attach branch → image-ref branch → manifest branch (`runtime: oci`) → existing legacy/language paths.
  - `analyzer` struct gains `closeFn func() error`; `Close()` prefers it.

- [ ] **Step 1: Write the failing tests**

Append to `pkg/resource/plugin/analyzer_oci_test.go`. The fake analyzer server gains `Handshake` and `GetAnalyzerInfo`; a minimal fake `Host` is built by embedding the `Host` interface (only the methods the boot path calls are implemented; anything else panics, which is fine for the test).

```go
// Append to pkg/resource/plugin/analyzer_oci_test.go — extend the fake server:
func (s *fakeAnalyzerServer) Handshake(
	ctx context.Context, req *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

func (s *fakeAnalyzerServer) GetAnalyzerInfo(
	ctx context.Context, req *emptypb.Empty,
) (*pulumirpc.AnalyzerInfo, error) {
	return &pulumirpc.AnalyzerInfo{Name: "fake-pack", Version: "1.0.0"}, nil
}

// fakeHost implements only what the OCI/attach boot paths call.
type fakeHost struct {
	Host
	addr string
}

func (h *fakeHost) ServerAddr() string                { return h.addr }
func (h *fakeHost) AttachDebugger(spec DebugSpec) bool { return false }

func TestNewPolicyAnalyzerAttachMode(t *testing.T) {
	addr := startFakeAnalyzer(t)
	_, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	t.Setenv(EnvPolicyPackAttach, "attach-pack:"+portStr)

	ctx, err := NewContext(nil, nil, nil, nil, "", nil, false, nil)
	require.NoError(t, err)

	a, err := NewPolicyAnalyzer(&fakeHost{addr: "127.0.0.1:1"}, ctx, "attach-pack",
		t.TempDir() /* no manifest needed: attach short-circuits */, nil, nil)
	require.NoError(t, err)

	info, err := a.GetAnalyzerInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "fake-pack", info.Name)
	require.NoError(t, a.Close())
}
```

Check `NewContext`'s current signature before using it (`pkg/resource/plugin/context.go`) and adjust the call to match; the intent is a minimal `*Context` whose `Request()`/`baseContext` work.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd pkg && go test ./resource/plugin/ -run TestNewPolicyAnalyzerAttachMode -v`
Expected: FAIL (attach not implemented — it will try the legacy plugin path and error differently, or compile-fail on the fake server methods)

- [ ] **Step 3: Implement**

3a. Add the field to `PolicyAnalyzerOptions` in `pkg/resource/plugin/host.go`:

```go
type PolicyAnalyzerOptions struct {
	Organization     string
	Project          string
	Stack            string
	Config           map[config.Key]string
	ConfigSecretKeys []config.Key
	DryRun           bool
	Tags             map[string]string // Tags for the current stack.
	AdditionalEnv    map[string]string // Per-pack environment variables (e.g., from ESC).

	// ImageRef, when set, is a digest-pinned OCI image reference for the policy
	// pack (from the pack version's service-side metadata). The pack boots from
	// this image; no local manifest or pack directory is consulted.
	ImageRef string
}
```

3b. In `analyzer_plugin.go`, add `closeFn func() error` to the `analyzer` struct and update `Close`:

```go
func (a *analyzer) Close() error {
	if a.closeFn != nil {
		return a.closeFn()
	}
	if a.plug == nil {
		return nil
	}
	return a.plug.Close()
}
```

3c. In `analyzer_plugin.go`, extract the `ConfigureStack` block (currently `:237-270`, the `if opts != nil { … }` body) into a helper so the OCI/attach paths reuse it, and guard it so publish-style options (only `ImageRef` set) skip stack configuration:

```go
// configureStack sends the stack context to the analyzer if opts carries any.
func configureStack(ctx *Context, client pulumirpc.AnalyzerClient, name tokens.QName, opts *PolicyAnalyzerOptions) error {
	if opts == nil {
		return nil
	}
	if opts.Stack == "" && opts.Project == "" && opts.Organization == "" && len(opts.Config) == 0 {
		// No stack context (e.g. `pulumi policy publish` booting a pack only
		// for GetAnalyzerInfo).
		return nil
	}
	// … body moved verbatim from NewPolicyAnalyzer (the req construction,
	// ConfigureStack call, and Unimplemented handling) …
	return nil
}
```

Replace the inlined block in `NewPolicyAnalyzer` with `if err := configureStack(ctx, client, name, opts); err != nil { return nil, err }`.

3d. At the top of `NewPolicyAnalyzer` (before the `LoadPolicyPack` call at :108), add the attach and image-ref branches:

```go
func NewPolicyAnalyzer(
	host Host, ctx *Context, name tokens.QName, policyPackPath string, opts *PolicyAnalyzerOptions,
	hasPlugin func(workspace.PluginDescriptor) bool,
) (Analyzer, error) {
	// Attach mode: the pack is already running (e.g. as a pod sidecar, or
	// under a debugger); connect to it instead of launching anything.
	if port, err := GetPolicyPackAttachPort(name); err != nil {
		return nil, err
	} else if port != nil {
		return attachPolicyAnalyzer(host, ctx, name, *port, opts)
	}

	// A digest-pinned image ref (server-enforced OCI pack): boot the container
	// directly; there is no local pack directory or manifest.
	if opts != nil && opts.ImageRef != "" {
		return newOCIPolicyAnalyzer(host, ctx, name, opts.ImageRef, opts)
	}

	projPath := filepath.Join(policyPackPath, "PulumiPolicy.yaml")
	proj, err := workspace.LoadPolicyPack(projPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load Pulumi policy project located at %q: %w", policyPackPath, err)
	}

	// A local OCI pack (`--policy-pack ./dir`): run the locally built image
	// named by the manifest's repository (tagged with the pack version).
	if proj.Runtime.Name() == "oci" {
		image, err := localOCIImageRef(proj, policyPackPath)
		if err != nil {
			return nil, err
		}
		return newOCIPolicyAnalyzer(host, ctx, name, image, opts)
	}

	// … existing handshake closure and legacy/language-host paths, unchanged …
}
```

3e. Add the boot functions to `pkg/resource/plugin/analyzer_oci.go`:

```go
// Additional imports: "maps", "net", ociruntime alias if needed —
// "github.com/pulumi/pulumi/pkg/v3/resource/plugin/oci", workspace, pulumirpc,
// codes/status, logging.

const analyzerReadyTimeout = 2 * time.Minute

// localOCIImageRef resolves the image to run for a local `--policy-pack ./dir`
// OCI pack: <repository>:<version>, falling back to :latest when the manifest
// has no version. The image must have been built locally (the CLI never
// builds or implicitly pulls for local packs).
func localOCIImageRef(proj *workspace.PolicyPackProject, path string) (string, error) {
	repo, _ := proj.Runtime.Options()["repository"].(string)
	if repo == "" {
		return "", fmt.Errorf("policy pack at %q has runtime \"oci\" but no \"repository\" runtime option; "+
			"set runtime.options.repository in PulumiPolicy.yaml to the pack's registry repository", path)
	}
	tag := proj.Version
	if tag == "" {
		tag = "latest"
	}
	return repo + ":" + tag, nil
}

// newOCIPolicyAnalyzer launches the pack image in a container and connects to
// its analyzer.
func newOCIPolicyAnalyzer(
	host Host, ctx *Context, name tokens.QName, image string, opts *PolicyAnalyzerOptions,
) (Analyzer, error) {
	rt, err := oci.DetectRuntime(nil)
	if err != nil {
		return nil, fmt.Errorf("policy pack %q: %w", name, err)
	}
	mode := oci.DetectMode()

	var packEnv map[string]string
	if opts != nil && len(opts.AdditionalEnv) > 0 {
		packEnv = opts.AdditionalEnv
	}

	container, err := rt.Launch(ctx.Request(), oci.LaunchOptions{
		Image:    image,
		PackName: string(name),
		Env:      packEnv,
		Mode:     mode,
	})
	if err != nil {
		return nil, fmt.Errorf("policy pack %q: %w "+
			"(for a local pack, build and tag the image as %q first — the CLI never builds it)",
			name, err, image)
	}

	conn, err := dialAnalyzerWithRetry(ctx.Request(), container.Addr, analyzerReadyTimeout,
		func() (bool, string) {
			return container.Running(ctx.Request()), container.Logs(ctx.Request())
		})
	if err != nil {
		contract.IgnoreError(container.Close())
		return nil, fmt.Errorf("policy pack %q: %w", name, err)
	}

	client := pulumirpc.NewAnalyzerClient(conn)

	// Handshake with an engine address the container can reach.
	engineAddr := oci.EngineAddressFor(mode, host.ServerAddr())
	if err := ociHandshake(ctx.Request(), client, name, engineAddr); err != nil {
		contract.IgnoreError(conn.Close())
		contract.IgnoreError(container.Close())
		return nil, err
	}

	if err := configureStack(ctx, client, name, opts); err != nil {
		contract.IgnoreError(conn.Close())
		contract.IgnoreError(container.Close())
		return nil, err
	}

	return &analyzer{
		name:   name,
		client: client,
		closeFn: func() error {
			contract.IgnoreError(conn.Close())
			return container.Close()
		},
	}, nil
}

// attachPolicyAnalyzer connects to a policy pack that is already running at a
// known loopback port (PULUMI_POLICY_PACK_ATTACH).
func attachPolicyAnalyzer(
	host Host, ctx *Context, name tokens.QName, port int, opts *PolicyAnalyzerOptions,
) (Analyzer, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := dialAnalyzerWithRetry(ctx.Request(), addr, analyzerReadyTimeout, nil)
	if err != nil {
		return nil, fmt.Errorf("attaching to policy pack %q at %s (from %s): %w",
			name, addr, EnvPolicyPackAttach, err)
	}
	client := pulumirpc.NewAnalyzerClient(conn)

	if err := ociHandshake(ctx.Request(), client, name, host.ServerAddr()); err != nil {
		contract.IgnoreError(conn.Close())
		return nil, err
	}
	if err := configureStack(ctx, client, name, opts); err != nil {
		contract.IgnoreError(conn.Close())
		return nil, err
	}
	return &analyzer{
		name:    name,
		client:  client,
		closeFn: conn.Close,
	}, nil
}

// ociHandshake performs the analyzer handshake over an established connection.
// Containerized/attached packs get no Root/ProgramDirectory: host paths are
// meaningless inside the pack's filesystem. Unimplemented is tolerated, as in
// the process-launch path.
func ociHandshake(
	reqCtx context.Context, client pulumirpc.AnalyzerClient, name tokens.QName, engineAddr string,
) error {
	_, err := client.Handshake(reqCtx, &pulumirpc.AnalyzerHandshakeRequest{
		EngineAddress: engineAddr,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unimplemented {
			logging.V(7).Infof("Handshake: not supported by policy pack %q", name)
			return nil
		}
		return fmt.Errorf("handshake with policy pack %q failed: %w", name, err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `cd pkg && go test ./resource/plugin/... -v -run 'TestNewPolicyAnalyzerAttachMode|TestGetPolicyPackAttach|TestDialAnalyzer|TestConstructEnv'`
Expected: PASS. Then run the whole package: `cd pkg && go test ./resource/plugin/` — expected PASS (no regressions from the `configureStack` extraction).

- [ ] **Step 5: Verify the host cache still compiles and behaves**

Read `pkg/host/host.go:339-378` (`defaultHost.PolicyAnalyzer` memoization). Confirm the cache key still distinguishes analyzers correctly given the new field (the key includes the options; same pack + same ImageRef must hit the cache, different ImageRef must not). If the key is derived from a subset of fields, add `ImageRef` to it. Run `cd pkg && go build ./...`.

- [ ] **Step 6: Format, lint, commit**

```bash
mise exec -- make format && mise exec -- make lint
git add pkg/resource/plugin/ pkg/host/
git commit -m "Boot OCI policy packs from containers in NewPolicyAnalyzer"
```

---

### Task 7: Consumer plumbing — apitype, engine.RequiredPolicy, install-time pull

**Files:**
- Modify: `sdk/go/common/apitype/policy.go` (`RequiredPolicy` at :79)
- Modify: `pkg/engine/update.go` (`RequiredPolicy` interface at :195, `loadPolicyPlugins` at :723)
- Modify: `pkg/engine/policypacks.go` (`installPolicyPack` at :34)
- Modify: `pkg/backend/httpstate/policypack.go` (`cloudRequiredPolicy`)
- Test: `pkg/engine/policypacks_test.go` (create if absent)

**Interfaces:**
- Consumes: `oci.DetectRuntime`, `Runtime.Pull` (Tasks 1, 4); `PolicyAnalyzerOptions.ImageRef` (Task 6).
- Produces:
  - `apitype.RequiredPolicy` gains `Runtime string` (json `runtime,omitempty`) and `ImageRef string` (json `imageRef,omitempty`).
  - `engine.RequiredPolicy` interface gains `ImageRef() string` (empty for tarball packs). **All implementers must add it.**

- [ ] **Step 1: Add the apitype fields**

In `sdk/go/common/apitype/policy.go`, append to `RequiredPolicy`:

```go
	// Runtime of the required Policy Pack (e.g. "nodejs", "oci"). Empty for
	// packs published before runtimes were recorded.
	Runtime string `json:"runtime,omitempty"`

	// ImageRef is the digest-pinned OCI image reference for the required
	// Policy Pack (e.g. "ghcr.io/acme/pack@sha256:…"). Set only for packs
	// published with runtime "oci"; such packs have no PackLocation.
	ImageRef string `json:"imageRef,omitempty"`
```

- [ ] **Step 2: Extend the engine interface and implementers**

In `pkg/engine/update.go`, add to the `RequiredPolicy` interface (after `Version()`):

```go
	// ImageRef returns the digest-pinned OCI image reference for the
	// PolicyPack, or "" for packs distributed as tarballs.
	ImageRef() string
```

In `pkg/backend/httpstate/policypack.go`, add to `cloudRequiredPolicy` (next to `Name()`/`Version()` at :68):

```go
func (rp *cloudRequiredPolicy) ImageRef() string { return rp.RequiredPolicy.ImageRef }
```

Find every other implementer and add `ImageRef() string { return "" }`:

Run: `cd pkg && go build ./... 2>&1 | head -30` and `cd tests && go build ./... 2>&1 | head -30` — each compile error at a `RequiredPolicy` use site is an implementer (likely test fakes in `pkg/engine` and `pkg/backend`). Add the one-line method to each.

- [ ] **Step 3: Write the failing test for install-time pull**

```go
// pkg/engine/policypacks_test.go (create; package engine)
// [2026 copyright header]
package engine

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	ociruntime "github.com/pulumi/pulumi/pkg/v3/resource/plugin/oci"
)

type fakeOCIRequiredPolicy struct {
	imageRef string
}

func (p *fakeOCIRequiredPolicy) Name() string       { return "fake-oci" }
func (p *fakeOCIRequiredPolicy) Version() string    { return "1.0.0" }
func (p *fakeOCIRequiredPolicy) ImageRef() string   { return p.imageRef }
func (p *fakeOCIRequiredPolicy) Installed() bool    { return false }
func (p *fakeOCIRequiredPolicy) LocalPath() (string, error) { return "", nil }
func (p *fakeOCIRequiredPolicy) Download(
	ctx context.Context, wrapper func(io.ReadCloser, int64) io.ReadCloser,
) (io.ReadCloser, int64, error) {
	panic("Download must not be called for OCI policy packs")
}
func (p *fakeOCIRequiredPolicy) Install(ctx *plugin.Context, content io.ReadCloser, stdout, stderr io.Writer) error {
	panic("Install must not be called for OCI policy packs")
}
func (p *fakeOCIRequiredPolicy) Config() map[string]*json.RawMessage { return nil }
func (p *fakeOCIRequiredPolicy) ResolveEnvironments(ctx context.Context) (*ResolvedPolicyEnvironment, error) {
	return nil, nil
}

func TestInstallPolicyPackPullsOCIImage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub runtime scripts are not supported on Windows")
	}
	// Stub docker on PATH that records the pull.
	dir := t.TempDir()
	record := filepath.Join(dir, "record")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docker"), []byte(
		"#!/bin/sh\necho \"$@\" >> "+record+"\n"), 0o700))
	t.Setenv("PATH", dir)
	t.Setenv(ociruntime.EnvContainerRuntime, "")

	policy := &fakeOCIRequiredPolicy{imageRef: "ghcr.io/acme/pack@sha256:abc"}
	err := installPolicyPack(context.Background(), nil, nil, policy)
	require.NoError(t, err)

	b, err := os.ReadFile(record)
	require.NoError(t, err)
	assert.Contains(t, string(b), "pull ghcr.io/acme/pack@sha256:abc")
}

func TestInstallPolicyPackOCIRuntimeMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // nothing on PATH
	t.Setenv(ociruntime.EnvContainerRuntime, "")
	policy := &fakeOCIRequiredPolicy{imageRef: "ghcr.io/acme/pack@sha256:abc"}
	err := installPolicyPack(context.Background(), nil, nil, policy)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "container runtime")
	assert.Contains(t, err.Error(), "fake-oci")
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `cd pkg && go test ./engine/ -run TestInstallPolicyPack -v`
Expected: FAIL — `installPolicyPack` panics in `Download` (the OCI branch doesn't exist yet). If the interface change from Step 2 broke other engine tests, fix those implementers first.

- [ ] **Step 5: Implement the install branch**

At the top of `installPolicyPack` in `pkg/engine/policypacks.go` (after the `policyID` line at :40, replacing the `Installed()` early-out for OCI packs):

```go
	// OCI policy packs are container images: "install" is an eager pull of the
	// digest-pinned ref. Nothing is written under ~/.pulumi/policies; the
	// container runtime's image store is the cache, and pulling an
	// already-present digest is a cheap no-op.
	if ref := policy.ImageRef(); ref != "" {
		rt, err := oci.DetectRuntime(nil)
		if err != nil {
			return fmt.Errorf("installing policy pack %s: %w", policyID, err)
		}
		fmt.Fprintf(os.Stderr, "Pulling policy pack image %s...\n", ref)
		if err := rt.Pull(ctx, ref, os.Stderr); err != nil {
			return fmt.Errorf("installing policy pack %s: %w", policyID, err)
		}
		logging.V(preparePluginLog).Infof("installPolicyPack(%s): image pulled", policyID)
		return nil
	}
```

Import `oci "github.com/pulumi/pulumi/pkg/v3/resource/plugin/oci"`.

- [ ] **Step 6: Plumb ImageRef into analyzer options in `loadPolicyPlugins`**

In `pkg/engine/update.go`, in the required-policy goroutine (`:752-784`), after `policyOpts := *analyzerOpts` (`:763`), add the ref and skip the local path for OCI packs:

```go
			policyOpts := *analyzerOpts
			policyOpts.ImageRef = policy.ImageRef()
			// … existing resolved-env handling unchanged …

			var policyPath string
			if policyOpts.ImageRef == "" {
				policyPath, err = policy.LocalPath()
				if err != nil {
					errs <- err
					return
				}
			}

			analyzer, err := loadPolicyAnalyzer(
				plugctx.Base(), plugctx, tokens.QName(policy.Name()), policyPath, &policyOpts)
```

(The existing code already calls `loadPolicyAnalyzer` with these variables; the change is setting `policyOpts.ImageRef` and making `policyPath` conditional.)

- [ ] **Step 7: Run tests**

Run: `cd pkg && go test ./engine/ -run TestInstallPolicyPack -v`
Expected: PASS
Run: `cd pkg && go build ./... && go test ./engine/ -count=1 2>&1 | tail -5`
Expected: builds; engine package tests pass.

- [ ] **Step 8: Format, lint, tidy, commit**

```bash
mise exec -- make format && mise exec -- make lint && mise exec -- make tidy
git add sdk/go/common/apitype/policy.go pkg/engine/ pkg/backend/httpstate/policypack.go
git commit -m "Install and load server-enforced OCI policy packs by image ref"
```

---

### Task 8: Publish client — apitype and `PublishPolicyPack` image-ref path

**Files:**
- Modify: `sdk/go/common/apitype/policy.go` (`CreatePolicyPackRequest` at :25)
- Modify: `pkg/backend/httpstate/client/client.go` (`PublishPolicyPack` at :1992)
- Test: `pkg/backend/httpstate/client/client_test.go` (append)

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `apitype.CreatePolicyPackRequest.ImageRef string` (json `imageRef,omitempty`).
  - `PublishPolicyPack(ctx, orgName, runtime string, analyzerInfo plugin.AnalyzerInfo, dirArchive io.Reader, imageRef string, metadata map[string]string) (string, error)` — new `imageRef` parameter after `dirArchive`. When `imageRef != ""`: request carries it, `dirArchive` is ignored (pass nil), and **no upload and no publish-complete call** are made. **Task 9 depends on this exact signature.**

- [ ] **Step 1: Add the apitype field**

Append to `CreatePolicyPackRequest` in `sdk/go/common/apitype/policy.go`:

```go
	// ImageRef is the digest-pinned OCI image reference for a policy pack with
	// runtime "oci" (e.g. "ghcr.io/acme/pack@sha256:…"). When set, the service
	// records the reference as the pack's artifact and no tarball is uploaded:
	// the response carries no upload URL and no publish-complete call follows.
	ImageRef string `json:"imageRef,omitempty"`
```

- [ ] **Step 2: Write the failing test**

Look at existing tests in `pkg/backend/httpstate/client/client_test.go` for the established `httptest.NewServer` pattern (e.g. any test that calls `newMockServer` or similar) and follow it. The test:

```go
func TestPublishPolicyPackWithImageRef(t *testing.T) {
	t.Parallel()

	var gotCreate apitype.CreatePolicyPackRequest
	var sawUpload, sawComplete bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/orgs/acme/policypacks":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotCreate))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":3}`)) // no uploadURI
		case r.Method == "PUT":
			sawUpload = true
			w.WriteHeader(200)
		case strings.Contains(r.URL.Path, "/complete"):
			sawComplete = true
			w.WriteHeader(200)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newMockClient(server) // follow the file's existing helper for constructing a Client against a test server

	version, err := client.PublishPolicyPack(context.Background(), "acme", "oci",
		plugin.AnalyzerInfo{Name: "security", Version: "1.2.3"},
		nil, "ghcr.io/acme/security@sha256:abc", nil)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", version)
	assert.Equal(t, "ghcr.io/acme/security@sha256:abc", gotCreate.ImageRef)
	assert.Equal(t, "oci", gotCreate.Runtime)
	assert.False(t, sawUpload, "no tarball upload for image-ref publishes")
	assert.False(t, sawComplete, "no publish-complete call for image-ref publishes")
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd pkg && go test ./backend/httpstate/client/ -run TestPublishPolicyPackWithImageRef -v`
Expected: FAIL (compile: too many arguments to PublishPolicyPack)

- [ ] **Step 4: Implement**

In `PublishPolicyPack` (`pkg/backend/httpstate/client/client.go:1992`):
- Add the `imageRef string` parameter after `dirArchive io.Reader`.
- Set `ImageRef: imageRef` in the `apitype.CreatePolicyPackRequest` literal.
- Wrap Step 2 (the PUT upload, `:2060-2073`) and Step 3 (the publish-complete POST) in `if imageRef == "" { … }`. The version-fallback logic (`resp.Version` when `analyzerInfo.Version == ""`) stays outside the guard.
- Update the one existing caller (`pkg/backend/httpstate/policypack.go:452`) to pass `""` for `imageRef` — Task 9 replaces this properly for OCI packs.

- [ ] **Step 5: Run tests**

Run: `cd pkg && go test ./backend/httpstate/... -count=1 2>&1 | tail -5`
Expected: PASS including the new test.

- [ ] **Step 6: Format, lint, commit**

```bash
mise exec -- make format && mise exec -- make lint
git add sdk/go/common/apitype/policy.go pkg/backend/httpstate/
git commit -m "Publish policy pack image refs without tarball upload"
```

---

### Task 9: Publish flow — `--tag` flag and the OCI publish branch

**Files:**
- Modify: `pkg/backend/policypack.go` (`PublishOperation` struct, ~:40)
- Modify: `pkg/cmd/pulumi/policy/policy_publish.go` (flag + plumb)
- Modify: `pkg/backend/httpstate/policypack.go` (`Publish` at :397; new `publishOCI`)
- Test: `pkg/backend/httpstate/policypack_test.go` (create or append)

**Interfaces:**
- Consumes: `oci.DetectRuntime`, `Runtime.{Pull,ResolveDigest,HasPlatform}` (Tasks 1, 4); `PolicyAnalyzerOptions.ImageRef` (Task 6); `PublishPolicyPack(…, imageRef, …)` (Task 8).
- Produces: `backend.PublishOperation.Tag string`; `pulumi policy publish --tag <tag>`.

- [ ] **Step 1: Add `Tag` to `PublishOperation`**

In `pkg/backend/policypack.go`, add to the `PublishOperation` struct:

```go
	// Tag is the image tag to publish for policy packs with runtime "oci"
	// (defaults to the pack's version). Ignored for other runtimes.
	Tag string
```

- [ ] **Step 2: Add the `--tag` flag**

In `pkg/cmd/pulumi/policy/policy_publish.go`:
- Add a field `tag string` to `policyPublishCmd` (at :66).
- In `newPolicyPublishCmd` (after `constrictor.AttachArguments` at :56):

```go
	cmd.PersistentFlags().StringVar(&policyPublishCmd.tag, "tag", "",
		"For policy packs with runtime \"oci\": the image tag to publish (defaults to the pack version)")
```

- In `Run`, pass it through the publish operation (at :156):

```go
	err = policyPack.Publish(ctx, backend.PublishOperation{
		Root:       root,
		PlugCtx:    plugctx,
		PolicyPack: proj,
		Tag:        cmd.tag,
		Scopes:     backend.CancellationScopes,
		Metadata:   m,
	})
```

- [ ] **Step 3: Write the failing test**

Test `publishOCI`'s pure decision logic by extracting the ref/tag resolution into a testable function. In `pkg/backend/httpstate/policypack_test.go`:

```go
func TestOCIPublishRefs(t *testing.T) {
	t.Parallel()

	proj := func(repo, version string) *workspace.PolicyPackProject {
		p := &workspace.PolicyPackProject{Version: version}
		// Build the runtime info via YAML round-trip to set unexported fields.
		require.NoError(t, yaml.Unmarshal([]byte(
			"name: oci\noptions:\n  repository: "+repo+"\n"), &p.Runtime))
		return p
	}

	// Tag from --tag wins.
	ref, err := ociPublishRef(proj("ghcr.io/acme/pack", "1.0.0"), "rc1")
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/acme/pack:rc1", ref)

	// Tag defaults to the pack version.
	ref, err = ociPublishRef(proj("ghcr.io/acme/pack", "1.0.0"), "")
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/acme/pack:1.0.0", ref)

	// No tag and no version: loud error.
	_, err = ociPublishRef(proj("ghcr.io/acme/pack", ""), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--tag")

	// Missing repository: loud error.
	_, err = ociPublishRef(&workspace.PolicyPackProject{}, "rc1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository")
}
```

Check how `PolicyPackProject.Runtime` can be constructed in tests (`workspace.NewProjectRuntimeInfo(name, options)` exists — check `sdk/go/common/workspace/project.go` around :1250 for the constructor; if it exists, use it instead of the YAML round-trip).

- [ ] **Step 4: Run test to verify it fails**

Run: `cd pkg && go test ./backend/httpstate/ -run TestOCIPublishRefs -v`
Expected: FAIL (undefined: ociPublishRef)

- [ ] **Step 5: Implement**

In `pkg/backend/httpstate/policypack.go`:

5a. At the top of `Publish` (before the "Obtaining policy metadata" print at :404):

```go
	if strings.EqualFold(op.PolicyPack.Runtime.Name(), "oci") {
		return pack.publishOCI(ctx, op)
	}
```

5b. Add the helper and the branch:

```go
// ociPublishRef resolves the tagged ref to publish: the manifest's repository
// plus the --tag argument (defaulting to the pack version).
func ociPublishRef(proj *workspace.PolicyPackProject, tag string) (string, error) {
	repo, _ := proj.Runtime.Options()["repository"].(string)
	if repo == "" {
		return "", errors.New(`policy packs with runtime "oci" must set the "repository" runtime option ` +
			`in PulumiPolicy.yaml (the pack's registry repository, e.g. ghcr.io/acme/policy-packs/security)`)
	}
	if tag == "" {
		tag = proj.Version
	}
	if tag == "" {
		return "", errors.New("no image tag to publish: pass --tag or set `version` in PulumiPolicy.yaml")
	}
	return repo + ":" + tag, nil
}

// publishOCI publishes a policy pack whose artifact is a container image the
// author has already built and pushed: resolve the tag to its digest, verify
// the platform matrix, boot the image for metadata (which doubles as a boot
// check), and register the digest-pinned ref — nothing is uploaded.
func (pack *cloudPolicyPack) publishOCI(ctx context.Context, op backend.PublishOperation) error {
	taggedRef, err := ociPublishRef(op.PolicyPack, op.Tag)
	if err != nil {
		return err
	}

	rt, err := oci.DetectRuntime(nil)
	if err != nil {
		return fmt.Errorf("publishing an oci policy pack requires a container runtime: %w", err)
	}

	fmt.Printf("Pulling %s\n", taggedRef)
	if err := rt.Pull(ctx, taggedRef, os.Stderr); err != nil {
		return fmt.Errorf("%w\n(the image must be built and pushed before `pulumi policy publish`)", err)
	}

	digestRef, err := rt.ResolveDigest(ctx, taggedRef)
	if err != nil {
		return err
	}

	// Every server-side enforcement point (Deployments executor, Insights
	// evaluator, TF policy check) runs on linux/amd64: an image without it
	// would pass publish and local dev, then break org enforcement.
	ok, err := rt.HasPlatform(ctx, taggedRef, "linux/amd64")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("image %s does not include linux/amd64, which server-side policy "+
			"enforcement requires; build a multi-arch image, e.g. "+
			"`docker buildx build --platform linux/amd64,linux/arm64 --push`", taggedRef)
	}

	fmt.Println("Obtaining policy metadata from policy pack image")
	abs, err := filepath.Abs(op.PlugCtx.Pwd)
	if err != nil {
		return err
	}
	analyzer, err := op.PlugCtx.Host.PolicyAnalyzer(op.PlugCtx, tokens.QName(abs), op.PlugCtx.Pwd,
		&plugin.PolicyAnalyzerOptions{ImageRef: digestRef})
	if err != nil {
		return err
	}
	analyzerInfo, err := analyzer.GetAnalyzerInfo(ctx)
	if err != nil {
		return err
	}
	pack.ref.name = tokens.QName(analyzerInfo.Name)
	pack.ref.versionTag = analyzerInfo.Version

	fmt.Printf("Registering %s\n", digestRef)
	publishedVersion, err := pack.cl.PublishPolicyPack(
		ctx, pack.ref.orgName, "oci", analyzerInfo, nil /*dirArchive*/, digestRef, op.Metadata)
	if err != nil {
		return err
	}

	fmt.Printf("\nPermalink: %s/%s\n", pack.ref.CloudConsoleURL(), publishedVersion)
	return nil
}
```

Add imports: `errors`, `oci "github.com/pulumi/pulumi/pkg/v3/resource/plugin/oci"`. Also update the legacy `Publish` call to `PublishPolicyPack` (from Task 8) if not already done.

- [ ] **Step 6: Run tests**

Run: `cd pkg && go test ./backend/httpstate/... ./cmd/pulumi/policy/... -count=1 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 7: Format, lint, commit**

```bash
mise exec -- make format && mise exec -- make lint
git add pkg/backend/ pkg/cmd/pulumi/policy/
git commit -m "Publish OCI policy packs by digest-pinned image ref"
```

---

### Task 10: Docker-gated integration test

**Files:**
- Create: `pkg/resource/plugin/oci/testdata/fake-analyzer/main.go` (a minimal Go analyzer server)
- Create: `pkg/resource/plugin/oci/testdata/fake-analyzer/Dockerfile`
- Create: `pkg/resource/plugin/integration_oci_test.go` (package `plugin`)

**Interfaces:**
- Consumes: everything from Tasks 1–6.

The fake analyzer is a static Go binary in a `FROM scratch` image — no network needed at image-build time, fast in CI. The test cross-compiles it for linux, builds the image, and boots it through `NewPolicyAnalyzer`'s manifest branch.

- [ ] **Step 1: Write the fake analyzer**

```go
// pkg/resource/plugin/oci/testdata/fake-analyzer/main.go
// [2026 copyright header]

// A minimal policy pack analyzer used by the OCI launcher integration test.
// Serves the Analyzer gRPC service on PULUMI_POLICY_PORT.
package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type server struct {
	pulumirpc.UnimplementedAnalyzerServer
}

func (s *server) Handshake(
	ctx context.Context, req *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

func (s *server) GetAnalyzerInfo(ctx context.Context, _ *emptypb.Empty) (*pulumirpc.AnalyzerInfo, error) {
	return &pulumirpc.AnalyzerInfo{Name: "oci-integration-pack", Version: "0.0.1"}, nil
}

func (s *server) Analyze(ctx context.Context, req *pulumirpc.AnalyzeRequest) (*pulumirpc.AnalyzeResponse, error) {
	return &pulumirpc.AnalyzeResponse{
		Diagnostics: []*pulumirpc.AnalyzeDiagnostic{{
			PolicyName:       "always-fails",
			PolicyPackName:   "oci-integration-pack",
			Description:      "proves the pack ran from its container image",
			Message:          "ran-in-container",
			EnforcementLevel: pulumirpc.EnforcementLevel_MANDATORY,
		}},
	}, nil
}

func main() {
	port := os.Getenv("PULUMI_POLICY_PORT")
	if port == "" {
		port = "0"
	}
	lis, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	srv := grpc.NewServer()
	pulumirpc.RegisterAnalyzerServer(srv, &server{})
	// Announce the port (the plugin contract) even though launched packs are
	// dialed by retry, so packs that pick their own port stay discoverable.
	fmt.Printf("%d\n", lis.Addr().(*net.TCPAddr).Port)
	if err := srv.Serve(lis); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

```dockerfile
# pkg/resource/plugin/oci/testdata/fake-analyzer/Dockerfile
FROM scratch
COPY fake-analyzer /fake-analyzer
ENTRYPOINT ["/fake-analyzer"]
```

- [ ] **Step 2: Write the integration test**

```go
// pkg/resource/plugin/integration_oci_test.go
// [2026 copyright header]
package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin/oci"
)

const integrationImage = "pulumi-test/oci-policy-pack:integration"

func requireDocker(t *testing.T) *oci.Runtime {
	t.Helper()
	rt, err := oci.DetectRuntime(nil)
	if err != nil {
		t.Skip("no container runtime available; skipping OCI integration test")
	}
	if testing.Short() {
		t.Skip("skipping OCI integration test in -short mode")
	}
	return rt
}

func buildIntegrationImage(t *testing.T, rt *oci.Runtime) {
	t.Helper()
	dir := filepath.Join("oci", "testdata", "fake-analyzer")

	build := exec.Command("go", "build", "-o", filepath.Join(dir, "fake-analyzer"), "./"+dir)
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building fake analyzer: %s", out)
	t.Cleanup(func() { _ = os.Remove(filepath.Join(dir, "fake-analyzer")) })

	img := exec.Command(rt.Path, "build", "-t", integrationImage, dir)
	out, err = img.CombinedOutput()
	require.NoError(t, err, "building image: %s", out)
}

func TestOCIPolicyPackEndToEnd(t *testing.T) {
	rt := requireDocker(t)
	buildIntegrationImage(t, rt)

	// A local pack directory whose manifest points at the built image.
	packDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"), []byte(
		"runtime:\n  name: oci\n  options:\n    repository: pulumi-test/oci-policy-pack\nversion: integration\n"), 0o600))

	pctx, err := NewContext(nil, nil, nil, nil, "", nil, false, nil) // match NewContext's actual signature (see Task 6)
	require.NoError(t, err)

	a, err := NewPolicyAnalyzer(&fakeHost{addr: "127.0.0.1:1"}, pctx, "oci-integration-pack", packDir, nil, nil)
	require.NoError(t, err)

	info, err := a.GetAnalyzerInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "oci-integration-pack", info.Name)

	require.NoError(t, a.Close())

	// The container must be gone after Close (launched with --rm). Ctrl-C /
	// engine shutdown reach the same path: the host closes its analyzers,
	// which invokes this Close — so this assertion also covers the spec's
	// cancellation requirement (no orphaned containers on interrupt).
	out, err := exec.Command(rt.Path, "ps", "--filter", "label="+oci.LabelKey, "--format", "{{.ID}}").Output()
	require.NoError(t, err)
	assert.Empty(t, string(out), "no policy pack containers should survive Close")
}
```

Note: `fakeHost` comes from Task 6's test file (same package).

- [ ] **Step 3: Run the test**

Run: `cd pkg && go test ./resource/plugin/ -run TestOCIPolicyPackEndToEnd -v -timeout 300s`
Expected: PASS on a machine with docker; SKIP messages on machines without.

If it fails: debug with `docker ps -a --filter label=com.pulumi.policy-pack` and `docker logs <id>`. Common issues: the image build context path (run tests from `pkg/resource/plugin/`, so `dir` is relative to that), and `GOOS=linux` cross-build (works from macOS since the binary is pure Go).

- [ ] **Step 4: Format, lint, commit**

```bash
mise exec -- make format && mise exec -- make lint
git add pkg/resource/plugin/
git commit -m "Add docker-gated integration test for OCI policy packs"
```

---

### Task 11: Changelog, full verification sweep

**Files:**
- Create: `changelog/pending/20260709--cli--support-oci-policy-packs.yaml`

- [ ] **Step 1: Look at an existing entry for the exact schema**

Run: `ls changelog/pending/ && cat changelog/pending/*.yaml | head -20` (if empty, check `git log --oneline -5 -- changelog/` and `git show` a recent one).

- [ ] **Step 2: Write the entry** (adjust keys to match the observed schema; the content is):

```yaml
changes:
  - type: feat
    scope: cli,engine
    description: Support policy packs published as OCI container images via runtime `oci`
```

- [ ] **Step 3: Full verification**

```bash
mise exec -- make format
mise exec -- make lint
mise exec -- make tidy
mise exec -- make test_fast
```

Expected: all pass. Fix anything that fails before committing (tidy may touch `go.sum` files — commit those too).

- [ ] **Step 4: Commit**

```bash
git add changelog/pending/ .
git commit -m "Add changelog entry for OCI policy packs"
```

After the PR exists, add its number to the entry's `custom.PR` field (per repo convention).

---

## Deferred (do NOT implement — spec section "Out of scope")

- `pulumi policy new` Dockerfile scaffolding.
- Formal conformance beyond the publish boot check.
- Private registries / credential brokering (auth failures already carry the "not yet supported" note from Task 4).
- Service-side work (DB, API endpoints, evaluators) — the apitype fields in Tasks 7–8 are the agreed wire contract.
