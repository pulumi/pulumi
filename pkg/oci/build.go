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
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// BuildPackage builds a self-describing package directory into a plugin image and returns
// its convention image ref. The package declares itself in PulumiPlugin.yaml with
// `runtime: oci` and `options.{name, version, build:{image, command, caches}}`, so a
// package describes its own build rather than relying on a consumer's declaration. This is
// the shared implementation behind `pulumi package build` and the language host's
// local-component build (its InstallDependencies), so the two cannot drift.
//
// The build runs in a builder container started from build.image (the toolchain), with the
// target ref delivered as $PULUMI_PACKAGE_IMAGE so the command stays ref-agnostic; the
// default command tags the package Dockerfile as that ref. registry, when non-empty,
// qualifies the ref (so the built image is named where it resolves and where publish pushes).
// Like every build site it needs pod mode only for --volumes-from the engine (to reach the
// source), not the engine netns.
func BuildPackage(ctx context.Context, dir, registry string, stderr io.Writer) (string, error) {
	proj, err := workspace.LoadPluginProject(filepath.Join(dir, "PulumiPlugin.yaml"))
	if err != nil {
		return "", fmt.Errorf("loading PulumiPlugin.yaml in %s: %w", dir, err)
	}
	if proj.Runtime.Name() != "oci" {
		return "", fmt.Errorf("package %s declares runtime %q, want oci", dir, proj.Runtime.Name())
	}
	opts := proj.Runtime.Options()

	name, _ := opts["name"].(string)
	version, _ := opts["version"].(string)
	if name == "" || version == "" {
		return "", fmt.Errorf("package %s must declare runtime.options.name and runtime.options.version", dir)
	}
	build, _ := opts["build"].(map[string]any)
	buildImage, _ := build["image"].(string)
	if buildImage == "" {
		return "", fmt.Errorf("package %s must declare runtime.options.build.image (the build environment)", name)
	}

	ref := ProviderImageRef(registry, name, version)
	command, _ := build["command"].(string)
	if command == "" {
		// Build the package's Dockerfile and tag it as the convention ref (delivered via
		// env so the command stays ref-agnostic).
		command = `docker build -q -t "$PULUMI_PACKAGE_IMAGE" .`
	}

	fmt.Fprintf(stderr, "Building %s (v%s) in %s -> %s\n", name, version, buildImage, ref)
	if _, err := BuildInContainer(
		ctx, buildImage, command, dir, optStringSlice(build["caches"]),
		map[string]string{"PULUMI_PACKAGE_IMAGE": ref}, stderr,
	); err != nil {
		return "", fmt.Errorf("building package %s: %w", name, err)
	}
	return ref, nil
}

// optStringSlice reads a YAML list-of-strings option (parsed as []any) into []string,
// skipping non-string/empty entries. nil-safe.
func optStringSlice(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// BuildInContainer runs a build command in a dedicated builder container and returns
// its stdout (design: "Topology — the build phase"). It is the shared mechanism for
// every build site: the program image build and local-component builds in the OCI
// language host, the build-owned `Link` command, and the `pulumi package build` command
// — all of which run in a builder container whose image supplies the toolchain rather
// than borrowing the engine's rootfs.
//
// The source reaches the builder via --volumes-from the engine container: the builder
// inherits the engine's workspace mount (the program/component source) and docker socket
// at the *same* paths, so workingDir is just the engine-internal directory — no host-path
// translation across the docker-out-of-docker boundary. The socket riding along is the
// artifact sink: a `docker build` inside the builder loads into the shared daemon. Build
// progress (the command's stderr) streams to the given writer; its stdout is returned
// (the program build reads an image ref from it; the component/package builds rely on the
// build tagging by convention and ignore it). env is projected into the builder (e.g. the
// target image ref for `package build`, or the SDK path for the link command).
//
// Over-sharing every engine mount (incl. PULUMI_HOME) is acceptable for a trusted local
// builder image but is what must be replaced with explicit, scoped mounts once the builder
// image is registry-supplied — at which point --volumes-from goes away. This requires pod
// mode only for that reason (to name the engine container for --volumes-from); it does NOT
// need the engine netns, unlike the provider/schema attach paths.
func BuildInContainer(
	ctx context.Context, image, command, workingDir string, caches []string, env map[string]string, stderr io.Writer,
) (string, error) {
	// The builder mounts the engine container's volumes by name; in pod mode the
	// wrapper sets --hostname to the engine container's name, so our hostname is a
	// valid --volumes-from reference.
	engine, err := os.Hostname()
	if err != nil || engine == "" {
		return "", fmt.Errorf("oci: cannot determine engine container name for the build: %w", err)
	}
	podID := os.Getenv("PULUMI_POD_ID")
	if podID == "" {
		podID = engine
	}
	// Each cache path gets a stable, persistent named volume. Docker auto-creates it
	// on first use; it is untracked by the pod manager, so pod Cleanup leaves it —
	// that persistence is the point.
	var volumes []VolumeMount
	for _, c := range caches {
		volumes = append(volumes, VolumeMount{Source: CacheVolumeName(c), Target: c})
	}
	pod := NewDockerPodManager(podID)
	return pod.RunToCompletion(ctx, ContainerConfig{
		Image:       image,
		Name:        "build",
		WorkingDir:  workingDir,
		VolumesFrom: []string{engine},
		Volumes:     volumes,
		Env:         env,
		Entrypoint:  []string{"sh", "-c"},
		Cmd:         []string{command},
	}, stderr)
}

// CacheVolumeName derives a stable, persistent named volume for a build cache path. It is
// path-keyed (global across projects — build caches are content-addressed, so sharing
// helps), with a recognizable prefix so the volumes are identifiable and prunable, since
// by design they outlive the pod and accumulate.
func CacheVolumeName(path string) string {
	sanitized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '.', r == '-':
			return r
		default:
			return '-'
		}
	}, strings.Trim(path, "/"))
	return "pulumi-oci-buildcache-" + sanitized
}
