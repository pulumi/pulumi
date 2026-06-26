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
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// podLabel keys a label attached to every resource a dockerPodManager creates,
// so a pod's containers/volumes/network can be found and reclaimed even after a
// crash. The value is the pod id.
const podLabel = "com.pulumi.pod"

// commandRunner executes a command and returns its stdout and stderr. It is the
// single seam through which the Docker implementation touches the OS for buffered
// commands, which lets tests assert on the exact argv constructed without a
// running daemon.
type commandRunner func(ctx context.Context, stdin io.Reader, name string, args ...string) (stdout, stderr string, err error)

// execRunner is the default commandRunner; it shells out for real.
func execRunner(ctx context.Context, stdin io.Reader, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	return out.String(), errOut.String(), err
}

// dockerPodManager is a PodManager backed by the `docker` CLI.
type dockerPodManager struct {
	bin   string        // docker binary name or path
	podID string        // unique id for this pod; namespaces resource names and labels
	run   commandRunner // injectable for testing

	mu         sync.Mutex
	containers []Container
	volumes    []Volume
	networks   []Network
}

// Option configures a dockerPodManager.
type Option func(*dockerPodManager)

// WithDockerBinary overrides the docker binary (default "docker").
func WithDockerBinary(bin string) Option {
	return func(m *dockerPodManager) { m.bin = bin }
}

// withRunner overrides the command runner. Unexported: only tests in this
// package use it.
func withRunner(r commandRunner) Option {
	return func(m *dockerPodManager) { m.run = r }
}

// NewDockerPodManager returns a PodManager that drives the `docker` CLI. podID
// scopes the pod's resource names and labels; it should be unique per pod (e.g.
// derived from the stack name plus a random suffix).
func NewDockerPodManager(podID string, opts ...Option) PodManager {
	m := &dockerPodManager{
		bin:   "docker",
		podID: podID,
		run:   execRunner,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// docker runs `docker <args...>` with no stdin and returns trimmed stdout.
func (m *dockerPodManager) docker(ctx context.Context, args ...string) (string, error) {
	stdout, stderr, err := m.run(ctx, nil, m.bin, args...)
	if err != nil {
		return "", fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

func (m *dockerPodManager) CreateNetwork(ctx context.Context) (Network, error) {
	name := m.resourceName("net")
	id, err := m.docker(ctx, "network", "create", "--label", m.label(), name)
	if err != nil {
		return Network{}, err
	}
	net := Network{ID: id, Name: name}
	m.track(func() { m.networks = append(m.networks, net) })
	return net, nil
}

func (m *dockerPodManager) RunContainer(ctx context.Context, cfg ContainerConfig) (Container, error) {
	if cfg.Name == "" {
		return Container{}, errors.New("container config requires a Name")
	}
	if cfg.Image == "" {
		return Container{}, errors.New("container config requires an Image")
	}

	name := m.resourceName(cfg.Name)
	args := []string{"run", "-d", "--name", name, "--label", m.label()}
	if cfg.Network != "" {
		args = append(args, "--network", cfg.Network)
	}
	if cfg.Privileged {
		args = append(args, "--privileged")
	}
	if cfg.HostGateway {
		args = append(args, "--add-host=host.docker.internal:host-gateway")
	}
	// Emit env in sorted key order so the argv is deterministic (and testable).
	for _, k := range sortedKeys(cfg.Env) {
		args = append(args, "-e", k+"="+cfg.Env[k])
	}
	for _, v := range cfg.Volumes {
		args = append(args, "-v", v.mountSpec())
	}

	// docker --entrypoint takes a single executable; any remaining entrypoint
	// tokens become the leading arguments, ahead of Cmd.
	cmdArgs := cfg.Cmd
	if len(cfg.Entrypoint) > 0 {
		args = append(args, "--entrypoint", cfg.Entrypoint[0])
		cmdArgs = append(append([]string{}, cfg.Entrypoint[1:]...), cfg.Cmd...)
	}
	args = append(args, cfg.Image)
	args = append(args, cmdArgs...)

	id, err := m.docker(ctx, args...)
	if err != nil {
		return Container{}, err
	}
	c := Container{ID: id, Name: name}
	m.track(func() { m.containers = append(m.containers, c) })
	return c, nil
}

func (m *dockerPodManager) WaitContainer(ctx context.Context, c Container) (int, error) {
	// `docker wait` blocks until the container exits, then prints its exit code.
	out, err := m.docker(ctx, "wait", c.ID)
	if err != nil {
		return -1, err
	}
	code, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return -1, fmt.Errorf("parsing exit code %q: %w", out, err)
	}
	return code, nil
}

func (m *dockerPodManager) ContainerLogs(ctx context.Context, c Container, follow bool) (io.ReadCloser, error) {
	// Logs stream, so they bypass the buffered commandRunner and exec directly.
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, c.ID)

	streamCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(streamCtx, m.bin, args...)
	pr, pw := io.Pipe()
	// docker writes container output across both stdout and stderr; merge them.
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("docker logs: %w", err)
	}
	go func() { pw.CloseWithError(cmd.Wait()) }()
	return &logStream{reader: pr, cancel: cancel}, nil
}

// logStream couples the log pipe to the underlying process: closing it both
// closes the read end and kills the `docker logs` process (via ctx cancel).
type logStream struct {
	reader *io.PipeReader
	cancel context.CancelFunc
}

func (l *logStream) Read(p []byte) (int, error) { return l.reader.Read(p) }

func (l *logStream) Close() error {
	l.cancel()
	return l.reader.Close()
}

func (m *dockerPodManager) StopContainer(ctx context.Context, c Container) error {
	// `docker rm -f` stops and removes in one step; treat an already-gone
	// container as success.
	if _, err := m.docker(ctx, "rm", "-f", c.ID); err != nil && !isNotFound(err) {
		return err
	}
	return nil
}

func (m *dockerPodManager) CreateVolume(ctx context.Context, name string) (Volume, error) {
	volName := m.resourceName("vol-" + name)
	if _, err := m.docker(ctx, "volume", "create", "--label", m.label(), volName); err != nil {
		return Volume{}, err
	}
	vol := Volume{Name: volName}
	m.track(func() { m.volumes = append(m.volumes, vol) })
	return vol, nil
}

func (m *dockerPodManager) CopyFromImage(ctx context.Context, image, srcPath string, vol Volume, dstPath string) error {
	// Run a throwaway container from `image` with the named volume mounted at
	// dstPath and copy srcPath's contents into it. This is the reliable way to
	// populate a *named* volume from an image's rootfs — `docker cp` only moves
	// data between a container and the host, not into a named volume.
	src := strings.TrimRight(srcPath, "/")
	script := fmt.Sprintf("cp -a %s/. %s/", shellQuote(src), shellQuote(dstPath))
	_, err := m.docker(ctx, "run", "--rm",
		"--label", m.label(),
		"-v", vol.Name+":"+dstPath,
		"--entrypoint", "sh",
		image, "-c", script)
	return err
}

func (m *dockerPodManager) Cleanup(ctx context.Context) error {
	// Snapshot and clear tracked resources under the lock, then tear down without
	// holding it.
	m.mu.Lock()
	containers, volumes, networks := m.containers, m.volumes, m.networks
	m.containers, m.volumes, m.networks = nil, nil, nil
	m.mu.Unlock()

	var errs []error
	// Containers first (they hold references to volumes and the network), then
	// volumes, then the network.
	for _, c := range containers {
		if _, err := m.docker(ctx, "rm", "-f", c.ID); err != nil && !isNotFound(err) {
			errs = append(errs, err)
		}
	}
	for _, v := range volumes {
		if _, err := m.docker(ctx, "volume", "rm", "-f", v.Name); err != nil && !isNotFound(err) {
			errs = append(errs, err)
		}
	}
	for _, n := range networks {
		if _, err := m.docker(ctx, "network", "rm", n.ID); err != nil && !isNotFound(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// resourceName namespaces a logical name to this pod to avoid cross-pod
// collisions on a shared host.
func (m *dockerPodManager) resourceName(suffix string) string {
	return fmt.Sprintf("pulumi-pod-%s-%s", m.podID, suffix)
}

// label returns the "key=value" label applied to every resource in this pod.
func (m *dockerPodManager) label() string {
	return podLabel + "=" + m.podID
}

func (m *dockerPodManager) track(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn()
}

// mountSpec renders a VolumeMount as a `docker -v` argument.
func (v VolumeMount) mountSpec() string {
	spec := v.Source + ":" + v.Target
	if v.ReadOnly {
		spec += ":ro"
	}
	return spec
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isNotFound reports whether err is docker's "no such ..." error, which we treat
// as success when removing resources.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such")
}

// shellQuote single-quotes a string for safe interpolation into the `sh -c`
// script used by CopyFromImage.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
