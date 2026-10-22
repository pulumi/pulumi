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

package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Emulator describes one floci emulator: how to reach it and how to start it.
type Emulator struct {
	// Name is the emulator's name in sandbox mode's rules, e.g. "aws".
	Name string
	// DisplayName is used in messages.
	DisplayName string
	// DefaultEndpoint is where the emulator listens by default.
	DefaultEndpoint string
	// EndpointEnv names the environment variable that overrides the endpoint; floci's own CLI reads it too.
	EndpointEnv string
	// Image is the release started with Docker when the floci CLI isn't installed.
	Image string
	// Container matches the floci CLI's default container name, so `floci <cloud> stop` and `pulumi sandbox stop`
	// operate on the same container however it was started.
	Container string
	// ContainerPort is the port the emulator listens on inside its container.
	ContainerPort string
	// CLICommand is the floci CLI's subcommand for this emulator.
	CLICommand string
	// HealthPath is the emulator's readiness endpoint.
	HealthPath string
	// StorageModeEnv makes the emulator persist its state under /app/data.
	StorageModeEnv string
}

// Emulators lists the floci emulators sandbox stacks can use.
var Emulators = []Emulator{
	{
		Name:            "aws",
		DisplayName:     "floci (AWS)",
		DefaultEndpoint: "http://localhost:4566",
		EndpointEnv:     "FLOCI_ENDPOINT",
		Image:           "floci/floci:2.1.0",
		Container:       "floci",
		ContainerPort:   "4566",
		CLICommand:      "aws",
		HealthPath:      "/_floci/health",
		StorageModeEnv:  "FLOCI_STORAGE_MODE",
	},
	{
		Name:            "gcp",
		DisplayName:     "floci-gcp (Google Cloud)",
		DefaultEndpoint: "http://localhost:4588",
		EndpointEnv:     "FLOCI_GCP_ENDPOINT",
		Image:           "floci/floci-gcp:0.9.0",
		Container:       "floci-gcp",
		ContainerPort:   "4588",
		CLICommand:      "gcp",
		HealthPath:      "/_floci-gcp/health",
		StorageModeEnv:  "FLOCI_GCP_STORAGE_MODE",
	},
}

// LookupEmulator returns the named emulator.
func LookupEmulator(name string) (Emulator, bool) {
	for _, e := range Emulators {
		if e.Name == name {
			return e, true
		}
	}
	return Emulator{}, false
}

// ErrNotRunning is returned when an emulator isn't running and couldn't be started.
var ErrNotRunning = errors.New("not running")

// Floci finds, starts and stops a floci emulator.
type Floci struct {
	Emulator
	// Endpoint is the emulator's base URL.
	Endpoint string
	// DataDir is where floci persists emulated resources, so they survive a restart alongside the stack's state.
	DataDir string
	// Progress receives human-readable progress messages.
	Progress io.Writer

	lookPath      func(file string) (string, error)
	run           func(ctx context.Context, name string, args ...string) ([]byte, error)
	client        *http.Client
	pollInterval  time.Duration
	pullTimeout   time.Duration
	launchTimeout time.Duration
	startTimeout  time.Duration
	now           func() time.Time
}

// NewFloci returns a Floci for the emulator's endpoint, which its endpoint environment variable can override.
func NewFloci(emulator Emulator, progress io.Writer) (*Floci, error) {
	endpoint := os.Getenv(emulator.EndpointEnv)
	if endpoint == "" {
		endpoint = emulator.DefaultEndpoint
	}
	dataDir, err := workspace.GetPulumiPath("sandbox", "floci", emulator.Name)
	if err != nil {
		return nil, err
	}
	return &Floci{
		Emulator:      emulator,
		Endpoint:      strings.TrimSuffix(endpoint, "/"),
		DataDir:       dataDir,
		Progress:      progress,
		lookPath:      exec.LookPath,
		run:           runCommand,
		client:        &http.Client{Timeout: 2 * time.Second},
		pollInterval:  500 * time.Millisecond,
		pullTimeout:   10 * time.Minute,
		launchTimeout: 2 * time.Minute,
		startTimeout:  60 * time.Second,
		now:           time.Now,
	}, nil
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// Status is what a running emulator reports about itself.
type Status struct {
	// Version is the emulator's release, e.g. "0.9.0".
	Version string
	// Services is the set of services the emulator offers.
	Services map[string]bool
}

// Status asks the emulator for its version and services. It fails when the emulator isn't answering.
func (f *Floci) Status(ctx context.Context) (Status, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.Endpoint+f.HealthPath, nil)
	if err != nil {
		return Status{}, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return Status{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Status{}, fmt.Errorf("%s health check returned %s", f.DisplayName, resp.Status)
	}

	var health struct {
		Version  string            `json:"version"`
		Services map[string]string `json:"services"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		// A healthy emulator that doesn't describe itself is still usable.
		return Status{}, nil
	}
	status := Status{Version: health.Version}
	if health.Services != nil {
		status.Services = make(map[string]bool, len(health.Services))
		for name, state := range health.Services {
			status.Services[name] = state != "disabled"
		}
	}
	return status, nil
}

// Healthy reports whether the emulator is answering its health check.
func (f *Floci) Healthy(ctx context.Context) bool {
	_, err := f.Status(ctx)
	return err == nil
}

// PinnedVersion returns the emulator release Pulumi starts, taken from its image tag.
func (e Emulator) PinnedVersion() string {
	_, tag, _ := strings.Cut(e.Image, ":")
	return tag
}

// OlderThanPinned reports whether a running emulator's version is older than the pinned one. Versions that aren't
// semantic versions, such as nightly builds, are never reported as older.
func (e Emulator) OlderThanPinned(version string) bool {
	running, err := semver.ParseTolerant(version)
	if err != nil {
		return false
	}
	pinned, err := semver.ParseTolerant(e.PinnedVersion())
	if err != nil {
		return false
	}
	return running.LT(pinned)
}

// Launcher names the tool used to start floci.
type Launcher string

const (
	LauncherNone     Launcher = ""
	LauncherFlociCLI Launcher = "floci"
	LauncherDocker   Launcher = "docker"
)

// Launcher returns the tool that would be used to start floci: the floci CLI if it is installed, otherwise Docker if
// it is installed and its daemon is reachable.
func (f *Floci) Launcher(ctx context.Context) Launcher {
	if _, err := f.lookPath("floci"); err == nil {
		return LauncherFlociCLI
	}
	if _, err := f.lookPath("docker"); err == nil {
		if _, err := f.run(ctx, "docker", "info", "--format", "{{.ServerVersion}}"); err == nil {
			return LauncherDocker
		}
	}
	return LauncherNone
}

// EnsureRunning checks that floci is running and, if autostart is set, starts it when it isn't. When floci can't be
// started, the returned error wraps ErrNotRunning and says how to start it.
func (f *Floci) EnsureRunning(ctx context.Context, autostart bool) error {
	if f.Healthy(ctx) {
		return nil
	}

	port, local := f.localPort()
	if !autostart || !local {
		return f.notRunningError()
	}

	launcher := f.Launcher(ctx)
	if launcher == LauncherNone {
		return f.notRunningError()
	}

	if err := f.pullImage(ctx); err != nil {
		return err
	}

	fmt.Fprintf(f.Progress, "Starting %s at %s using %s...\n", f.DisplayName, f.Endpoint, launcher)
	if err := os.MkdirAll(f.DataDir, 0o700); err != nil {
		return fmt.Errorf("creating floci data directory: %w", err)
	}
	launchCtx, cancel := context.WithTimeout(ctx, f.launchTimeout)
	defer cancel()
	if err := f.start(launchCtx, launcher, port); err != nil {
		if errors.Is(launchCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("starting %s timed out after %v; check `pulumi sandbox status` and `docker ps`",
				f.DisplayName, f.launchTimeout)
		}
		return fmt.Errorf("starting %s: %w", f.DisplayName, err)
	}
	return f.waitHealthy(ctx)
}

// pullImage pulls the emulator's image if it isn't present yet, so the first start shows its progress instead of
// pausing silently. It needs Docker, which the floci CLI uses too; without it, starting reports the problem.
func (f *Floci) pullImage(ctx context.Context) error {
	if _, err := f.lookPath("docker"); err != nil {
		return nil
	}
	if _, err := f.run(ctx, "docker", "image", "inspect", "--format", "{{.Id}}", f.Image); err == nil {
		return nil
	}

	fmt.Fprintf(f.Progress, "Pulling %s (first run; this can take a few minutes)...\n", f.Image)
	started := f.now()
	pullCtx, cancel := context.WithTimeout(ctx, f.pullTimeout)
	defer cancel()
	if _, err := f.run(pullCtx, "docker", "pull", f.Image); err != nil {
		if errors.Is(pullCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("pulling %s timed out after %v; pull it yourself with `docker pull %s`",
				f.Image, f.pullTimeout, f.Image)
		}
		return fmt.Errorf("pulling %s: %w", f.Image, err)
	}
	fmt.Fprintf(f.Progress, "Pulled %s in %v\n", f.Image, f.now().Sub(started).Round(time.Second))
	return nil
}

func (f *Floci) start(ctx context.Context, launcher Launcher, port string) error {
	switch launcher {
	case LauncherFlociCLI:
		_, err := f.run(ctx, "floci", f.CLICommand, "start", "--detach",
			"--image", f.Image, "--pull", "missing", "--port", port, "--persist", f.DataDir)
		return err
	case LauncherDocker:
		// A stopped container from an earlier run would make `docker run --name` fail, so restart it instead.
		if _, err := f.run(ctx, "docker", "start", f.Container); err == nil {
			return nil
		}
		_, err := f.run(ctx, "docker", "run", "--detach",
			"--name", f.Container,
			"--publish", "127.0.0.1:"+port+":"+f.ContainerPort,
			"--volume", "/var/run/docker.sock:/var/run/docker.sock",
			"--volume", filepath.ToSlash(f.DataDir)+":/app/data",
			"--env", f.StorageModeEnv+"=persistent",
			f.Image)
		return err
	case LauncherNone:
		return errors.New("neither the floci CLI nor Docker is available")
	default:
		return fmt.Errorf("unknown launcher %q", launcher)
	}
}

func (f *Floci) waitHealthy(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, f.startTimeout)
	defer cancel()
	ticker := time.NewTicker(f.pollInterval)
	defer ticker.Stop()
	for {
		if f.Healthy(ctx) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s did not become healthy at %s within %v", f.DisplayName, f.Endpoint, f.startTimeout)
		case <-ticker.C:
		}
	}
}

// Stop stops the emulator if it is running in a container this machine can reach.
func (f *Floci) Stop(ctx context.Context) error {
	switch f.Launcher(ctx) {
	case LauncherFlociCLI:
		_, err := f.run(ctx, "floci", f.CLICommand, "stop")
		return err
	case LauncherDocker:
		_, err := f.run(ctx, "docker", "stop", f.Container)
		return err
	case LauncherNone:
	}
	return fmt.Errorf("neither the floci CLI nor Docker is available to stop %s", f.DisplayName)
}

// localPort returns the port of the endpoint and whether it's on this machine, which is the only case we can start
// floci for.
func (f *Floci) localPort() (string, bool) {
	u, err := url.Parse(f.Endpoint)
	if err != nil {
		return "", false
	}
	host, port := u.Hostname(), u.Port()
	if port == "" {
		port = "80"
	}
	if host == "localhost" || host == "localhost.floci.io" {
		return port, true
	}
	ip := net.ParseIP(host)
	return port, ip != nil && ip.IsLoopback()
}

func (f *Floci) notRunningError() error {
	return fmt.Errorf("%s is %w at %s; sandbox stacks deploy against local cloud emulators\n%s",
		f.DisplayName, ErrNotRunning, f.Endpoint, f.startHint())
}

func (f *Floci) startHint() string {
	if _, local := f.localPort(); !local {
		return fmt.Sprintf("Start it at that endpoint, or unset %s to use a local one.", f.EndpointEnv)
	}
	if _, err := f.lookPath("floci"); err == nil {
		return fmt.Sprintf("Start it with:\n\n    floci %s start\n", f.CLICommand)
	}

	var b strings.Builder
	b.WriteString("Install the floci CLI (it requires Docker):\n\n")
	switch runtime.GOOS {
	case "windows":
		b.WriteString("    irm https://floci.io/install.ps1 | iex\n")
	case "darwin":
		b.WriteString("    brew install floci-io/floci/floci\n")
	default:
		b.WriteString("    curl -fsSL https://floci.io/install.sh | sh\n")
	}
	fmt.Fprintf(&b, "\nthen run `floci %s start`, or run it directly with Docker:\n\n"+
		"    docker run -d --name %s -p %s:%s -v /var/run/docker.sock:/var/run/docker.sock %s\n",
		f.CLICommand, f.Container, f.ContainerPort, f.ContainerPort, f.Image)
	return b.String()
}
