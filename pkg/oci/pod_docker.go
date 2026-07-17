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
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/layout"
)

// podLabel keys a label attached to every resource a dockerPodManager creates,
// so a pod's containers/volumes/network can be found and reclaimed even after a
// crash. The value is the pod id.
const podLabel = "com.pulumi.pod"

// commandRunner executes a command and returns its stdout and stderr. It is the
// single seam through which the Docker implementation touches the OS for buffered
// commands, which lets tests assert on the exact argv constructed without a
// running daemon.
type commandRunner func(
	ctx context.Context, stdin io.Reader, name string, args ...string,
) (stdout, stderr string, err error)

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
	// Pin the working directory, overriding whatever WORKDIR the image happens to set.
	// Every long-lived pod member asks for WorkspaceMountPath here, which is what makes a
	// relative path mean the same thing to the program that writes it and the provider
	// that reads it (see WorkspaceMountPath).
	if cfg.WorkingDir != "" {
		args = append(args, "-w", cfg.WorkingDir)
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

// runToCompletionArgs builds the `docker run` argv for a one-shot, attached
// container. It is pure (no I/O) so the argv can be asserted in tests; the
// streaming execution in RunToCompletion bypasses the buffered commandRunner.
func runToCompletionArgs(label string, cfg ContainerConfig) []string {
	// --rm: the build container is ephemeral and synchronous, so it removes itself
	// on exit; the pod label is only a crash backstop. No tracking is needed (cf.
	// CopyFromImage).
	args := []string{"run", "--rm", "--label", label}
	if cfg.Network != "" {
		args = append(args, "--network", cfg.Network)
	}
	if cfg.Privileged {
		args = append(args, "--privileged")
	}
	for _, src := range cfg.VolumesFrom {
		args = append(args, "--volumes-from", src)
	}
	if cfg.WorkingDir != "" {
		args = append(args, "-w", cfg.WorkingDir)
	}
	for _, k := range sortedKeys(cfg.Env) {
		args = append(args, "-e", k+"="+cfg.Env[k])
	}
	for _, v := range cfg.Volumes {
		args = append(args, "-v", v.mountSpec())
	}
	cmdArgs := cfg.Cmd
	if len(cfg.Entrypoint) > 0 {
		args = append(args, "--entrypoint", cfg.Entrypoint[0])
		cmdArgs = append(append([]string{}, cfg.Entrypoint[1:]...), cfg.Cmd...)
	}
	args = append(args, cfg.Image)
	args = append(args, cmdArgs...)
	return args
}

func (m *dockerPodManager) RunToCompletion(ctx context.Context, cfg ContainerConfig, stderr io.Writer) (string, error) {
	if cfg.Image == "" {
		return "", errors.New("container config requires an Image")
	}
	args := runToCompletionArgs(m.label(), cfg)

	// Exec directly rather than through the buffered commandRunner: stderr streams
	// live (build progress is visible as it happens) while stdout is captured for
	// the ref. No -t/-i, so the two streams stay separate and stdout isn't polluted
	// by progress chatter.
	cmd := exec.CommandContext(ctx, m.bin, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %s: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
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

// CopyFromImage's last argument is the caller's later mount point for the seeded
// volume; it is not needed to populate the volume, so it is unused here (see below).
func (m *dockerPodManager) CopyFromImage(ctx context.Context, image, srcPath string, vol Volume, _ string) error {
	// Populate the named volume with the contents of the image's srcPath by CREATING
	// (never starting) a throwaway container with the volume mounted THERE. Docker seeds
	// an empty named volume from the image's content at the mount path when the container
	// is created, so this needs no shell or `cp` in the image — it works with scratch
	// provider images (the registry proxy's default, and any minimal provider image),
	// unlike `sh -c cp`, which a bare image cannot run. The caller mounts the seeded
	// volume at its dstPath in the provider container; the volume's root holds srcPath's
	// contents, so they land there.
	src := strings.TrimRight(srcPath, "/")
	cid, err := m.docker(ctx, "create",
		"--label", m.label(),
		"-v", vol.Name+":"+src,
		image)
	if err != nil {
		return err
	}
	// The container exists only to seed the volume; remove it, keeping the volume.
	if _, err := m.docker(ctx, "rm", "-f", cid); err != nil {
		return fmt.Errorf("removing seed container %s: %w", cid, err)
	}
	return nil
}

func (m *dockerPodManager) ReadImageFile(ctx context.Context, image, path string) ([]byte, error) {
	// Create a container from the image WITHOUT starting it, `docker cp` the file
	// out of its filesystem, then remove it. `docker cp` reads the layer filesystem
	// directly through the daemon, so — unlike a `cat` entrypoint — no process runs
	// and the image needs no shell or coreutils. That matters because this also
	// reads manifests out of arbitrary provider/component images (potentially
	// distroless/scratch) we did not build.
	cid, err := m.docker(ctx, "create", "--label", m.label(), image)
	if err != nil {
		return nil, fmt.Errorf("oci: creating container to read %s from %s: %w", path, image, err)
	}
	// WithoutCancel so a cancelled ctx still reaps the throwaway container.
	defer func() { _, _ = m.docker(context.WithoutCancel(ctx), "rm", "-f", cid) }()

	// `docker cp <cid>:<path> -` streams a tar archive of the file to stdout. Call
	// the runner directly (not m.docker) so the binary tar is returned untrimmed and
	// stderr is available to distinguish "file absent".
	stdout, stderr, err := m.run(ctx, nil, m.bin, "cp", cid+":"+path, "-")
	if err != nil {
		// A missing file is normal for the manifest's best-effort consumer, so report
		// it as absence (nil), not an error. docker phrases this as either "Could not
		// find the file ... in container" or an "lstat ...: no such file" — match both.
		low := strings.ToLower(stderr)
		if strings.Contains(low, "could not find the file") || strings.Contains(low, "no such file") {
			return nil, nil
		}
		return nil, fmt.Errorf("docker cp %s:%s: %w: %s", cid, path, err, strings.TrimSpace(stderr))
	}
	return singleFileFromTar([]byte(stdout))
}

// singleFileFromTar returns the contents of the first regular file in a tar
// archive — the shape `docker cp <container>:<file> -` produces for a single file.
// An archive with no regular file (e.g. the path was a directory) yields nil.
func singleFileFromTar(archive []byte) ([]byte, error) {
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("reading docker cp tar stream: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}
}

func (m *dockerPodManager) ImageExists(ctx context.Context, ref string) (bool, error) {
	// `docker image inspect` exits non-zero when the image is absent. Any failure
	// (absent, or a malformed ref) is reported as "not present" — the caller's only
	// recourse is to pull, and a genuine daemon problem resurfaces there with a
	// clearer error than an inspect failure would give.
	_, _, err := m.run(ctx, nil, m.bin, "image", "inspect", ref)
	return err == nil, nil
}

func (m *dockerPodManager) PullImage(ctx context.Context, ref string) error {
	_, err := m.docker(ctx, "pull", ref)
	return err
}

// ImportImage loads the OCI image layout at layoutPath into the docker daemon and
// tags it as ref. Unlike the rest of this implementation — which shells out to the
// `docker` CLI — the load is done in-process via go-containerregistry, because
// `docker load` cannot ingest an OCI layout *directory* and the PodManager owns the
// loading step, so the dependency belongs here rather than behind a sibling
// container. It reads the layout's single image and streams it to the daemon's load
// endpoint (which dedups already-present layers on ingest, so a warm re-import does
// not re-materialize the base), applying ref as the location the orchestrator
// resolved.
func (m *dockerPodManager) ImportImage(ctx context.Context, layoutPath, ref string) error {
	idx, err := layout.ImageIndexFromPath(layoutPath)
	if err != nil {
		return fmt.Errorf("oci: reading image layout at %s: %w", layoutPath, err)
	}
	manifest, err := idx.IndexManifest()
	if err != nil {
		return fmt.Errorf("oci: reading layout index at %s: %w", layoutPath, err)
	}
	// A layout is not always a single image: buildx attaches a provenance/SBOM attestation
	// manifest by default, and a multi-arch build carries one image per platform. Pick the
	// image manifest for this host rather than assuming exactly one entry.
	digest, err := selectImageManifest(manifest)
	if err != nil {
		return fmt.Errorf("oci: %w in layout at %s", err, layoutPath)
	}
	img, err := idx.Image(digest)
	if err != nil {
		return fmt.Errorf("oci: extracting image from layout at %s: %w", layoutPath, err)
	}
	tag, err := name.NewTag(ref)
	if err != nil {
		return fmt.Errorf("oci: parsing target ref %q: %w", ref, err)
	}
	if _, err := daemon.Write(tag, img, daemon.WithContext(ctx)); err != nil {
		return fmt.Errorf("oci: loading image %s into the daemon: %w", ref, err)
	}
	return nil
}

// selectImageManifest picks the runnable image manifest for the current host from a layout
// index. It skips buildx attestation manifests (provenance/SBOM, annotated
// attestation-manifest) and, in a multi-arch layout, entries for other platforms; a nil
// platform — the common single-image case — matches anything. This is why ImportImage does
// not assume the index holds exactly one manifest.
func selectImageManifest(mf *v1.IndexManifest) (v1.Hash, error) {
	for i := range mf.Manifests {
		d := mf.Manifests[i]
		if d.Annotations["vnd.docker.reference.type"] == "attestation-manifest" {
			continue
		}
		if !d.MediaType.IsImage() {
			continue // a nested index (manifest list) or non-image artifact, not runnable as-is
		}
		if d.Platform != nil && (d.Platform.OS != runtime.GOOS || d.Platform.Architecture != runtime.GOARCH) {
			continue
		}
		return d.Digest, nil
	}
	return v1.Hash{}, fmt.Errorf("no image manifest for %s/%s (index holds %d manifest(s))",
		runtime.GOOS, runtime.GOARCH, len(mf.Manifests))
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
