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

// Package oci provides a pod-oriented abstraction over an OCI container runtime
// (Docker by default). It is the orchestration substrate for containerized
// Pulumi program execution: a "pod" is a single deployment's network of
// containers — the engine/CLI, the program, and any provider or component
// containers — sharing one virtual network and, where needed, a workspace
// volume.
//
// The interface is deliberately runtime-agnostic. The only implementation today
// shells out to the `docker` CLI, but nothing in the PodManager contract assumes
// Docker; a podman/nerdctl/containerd backend can satisfy the same interface.
//
// This package is an early prototype of the design in oci-execution-design.md.
// It lives under pkg/ rather than the public sdk/ surface intentionally, so its
// shape can change freely while the design is proven out.
package oci

import (
	"context"
	"fmt"
	"io"
)

// PodManager orchestrates the containers, network, and volumes that make up a
// single execution pod. A PodManager owns the resources it creates and is
// responsible for tearing them down via Cleanup.
//
// Implementations must be safe for concurrent use: the engine starts provider
// containers lazily and potentially in parallel as the program registers
// resources.
type PodManager interface {
	// CreateNetwork creates the pod's virtual network. Containers attached to it
	// reach each other by name via the runtime's embedded DNS.
	CreateNetwork(ctx context.Context) (Network, error)

	// RunContainer starts a container (detached) on the pod network and returns a
	// handle to it. The container runs until it exits on its own or is stopped.
	// Use WaitContainer to block on exit and ContainerLogs to read its output.
	RunContainer(ctx context.Context, cfg ContainerConfig) (Container, error)

	// WaitContainer blocks until the container exits and returns its exit code. A
	// non-zero exit code is reported via the int return, not as an error; err is
	// non-nil only if waiting itself failed.
	WaitContainer(ctx context.Context, c Container) (int, error)

	// ContainerLogs returns the combined stdout/stderr the container has produced.
	// If follow is true the stream stays open and yields output until the
	// container exits or ctx is cancelled. The caller must Close the reader.
	ContainerLogs(ctx context.Context, c Container, follow bool) (io.ReadCloser, error)

	// StopContainer stops and removes a container. It is idempotent: stopping an
	// already-stopped or already-removed container is not an error.
	StopContainer(ctx context.Context, c Container) error

	// RunToCompletion runs a container attached and blocks until it exits,
	// returning its stdout. The container's stderr is streamed to the provided
	// writer as it runs (e.g. build progress) while stdout is captured (e.g. the
	// build's image ref). It is the build primitive: one-shot, ephemeral (--rm),
	// with stdout and stderr kept separate so a ref printed to stdout can be read
	// cleanly regardless of stderr chatter. A non-zero exit is returned as an
	// error — unlike a program Run, a failed build has no "bail".
	RunToCompletion(ctx context.Context, cfg ContainerConfig, stderr io.Writer) (string, error)

	// CreateVolume creates a named volume scoped to this pod. The name is a short
	// logical label; the manager namespaces it to avoid cross-pod collisions.
	CreateVolume(ctx context.Context, name string) (Volume, error)

	// CopyFromImage populates a named volume with the contents of srcPath taken
	// from an image's filesystem, placing them at dstPath inside the volume. This
	// is the init-copy step that seeds the shared workspace volume before any
	// path-resolving provider (docker build, command/local) starts.
	//
	// The image must contain a POSIX shell and cp (the Pulumi base images do);
	// populating a *named* volume cannot be done with `docker cp` alone, which
	// only moves data between a container and the host.
	CopyFromImage(ctx context.Context, image, srcPath string, vol Volume, dstPath string) error

	// ReadImageFile returns the contents of the file at path inside image's
	// filesystem, or nil (and no error) if no such file exists. It is the
	// read-side counterpart to CopyFromImage: where CopyFromImage seeds a volume
	// from an image, ReadImageFile pulls a single small file's bytes back to the
	// caller — used to read the required-packages manifest baked into a program or
	// plugin image (see ReadRequiredPackagesFromImage) without starting the pod.
	//
	// The image must contain `cat` (the Pulumi program/plugin base images do). A
	// missing file is reported as a nil slice rather than an error, because its
	// consumer (a best-effort pre-pull hint) treats "no manifest" as normal.
	ReadImageFile(ctx context.Context, image, path string) ([]byte, error)

	// ImageExists reports whether an image reference is present in the local image
	// store. It is the "is this plugin installed?" check for the container model:
	// a plugin's installation state is "is its image in the daemon", not "is its
	// binary on disk".
	ImageExists(ctx context.Context, ref string) (bool, error)

	// PullImage pulls an image reference into the local image store. This is how a
	// container plugin is "installed" — its image is fetched from an OCI registry,
	// the same infrastructure used to distribute any other image.
	PullImage(ctx context.Context, ref string) error

	// ImportImage loads a runtime-neutral OCI image layout (a directory of
	// digest-addressed blobs, as any builder can emit) into the local image store
	// under ref. It is the sink for the build contract: a build produces a layout
	// naming no runtime and no location, and this verb — which is runtime-specific
	// by design, and so belongs below this seam — makes it present and tags it with
	// the ref the orchestrator resolved. This is the one place a decoupled build's
	// artifact re-enters a particular runtime; a containerd backend would satisfy
	// it with `ctr images import` rather than a docker load.
	ImportImage(ctx context.Context, layoutPath, ref string) error

	// Cleanup stops and removes every container, volume, and network this manager
	// created. It is best-effort: it attempts to remove all resources and returns
	// a joined error describing any failures, so one failure does not strand the
	// rest.
	Cleanup(ctx context.Context) error
}

// Network is a handle to a pod network.
type Network struct {
	// ID is the runtime-assigned network identifier.
	ID string
	// Name is the network's name, which is also the DNS domain its containers
	// resolve each other within.
	Name string
}

// Volume is a handle to a named volume.
type Volume struct {
	// Name is the runtime-visible (namespaced) volume name.
	Name string
}

// Container is a handle to a running or finished container.
type Container struct {
	// ID is the runtime-assigned container identifier.
	ID string
	// Name is the container's name, which doubles as its DNS name on the pod
	// network. Wire this (via Address) into PULUMI_MONITOR/PULUMI_ENGINE and the
	// like rather than hard-coding a fixed name, so concurrent pods don't collide.
	Name string
}

// Address returns "name:port" for reaching a service in this container from
// another container on the same pod network. The container name doubles as its
// DNS name, so this is simply Name:port.
func (c Container) Address(port int) string {
	return fmt.Sprintf("%s:%d", c.Name, port)
}

// VolumeMount describes a bind of a named volume (or host path) into a container.
type VolumeMount struct {
	// Source is a named volume (from CreateVolume), an absolute host path, or a
	// special path such as the Docker socket. The runtime distinguishes them: an
	// absolute path is a bind mount, anything else is treated as a named volume.
	Source string
	// Target is the absolute path inside the container.
	Target string
	// ReadOnly mounts the source read-only.
	ReadOnly bool
}

// ContainerConfig describes a container to start on the pod network.
type ContainerConfig struct {
	// Image is the OCI image reference to run.
	Image string
	// Name is a short logical name for the container (e.g. "engine", "program").
	// The manager namespaces it to a unique runtime name; the resulting DNS name
	// is reported back on the returned Container.
	Name string
	// Network is the name of the network to attach to (from CreateNetwork). If
	// empty, the container runs on the runtime's default network.
	Network string
	// Env is the set of environment variables to set in the container.
	Env map[string]string
	// Entrypoint overrides the image's ENTRYPOINT. The first element becomes the
	// executable; any remaining elements are prepended to Cmd as arguments.
	Entrypoint []string
	// Cmd overrides the image's CMD (the arguments passed to the entrypoint).
	Cmd []string
	// Volumes are the volume and bind mounts to attach.
	Volumes []VolumeMount
	// Privileged runs the container with extended privileges. Required for the
	// CLI-in-container case, which needs access to the Docker socket.
	Privileged bool
	// HostGateway adds the host-gateway alias (host.docker.internal resolving to
	// the host) so a container on the default network can reach a service running
	// on the host. Used for the engine-on-host execution mode (design Option A);
	// unnecessary — and ignored by callers — when the container joins the engine's
	// pod network and reaches it by container DNS (Option C).
	HostGateway bool
	// WorkingDir sets the container's working directory (docker -w). The build
	// step uses it so the build command runs in the program directory.
	WorkingDir string
	// VolumesFrom mounts every volume of another container into this one (docker
	// --volumes-from). The build container uses it to inherit the engine
	// container's workspace mount (the program source) and docker socket at the
	// same paths — sidestepping host-path translation across the
	// docker-out-of-docker boundary, since the language host only knows the
	// engine-internal path, not the host path the daemon would need for a bind.
	VolumesFrom []string
}
