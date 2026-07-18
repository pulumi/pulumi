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
// The build runs in a builder container started from build.image (the toolchain). It emits a
// runtime-neutral OCI image layout (naming no runtime and no location); runPackageBuild then
// loads that layout below the PodManager seam and applies ref there. registry, when non-empty,
// qualifies the ref (so the built image is named where it resolves and where publish pushes).
// Like every build site it needs pod mode only for --volumes-from the engine (to reach the
// source), not the engine netns.
func BuildPackage(ctx context.Context, dir, registry string, stderr io.Writer) (string, error) {
	// The build runs in a builder container whose working directory is this dir —
	// and a container workdir must be absolute, so a relative dir (e.g.
	// `pulumi package publish greeter`) is resolved against the caller's cwd here.
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
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
	ref := ProviderImageRef(registry, name, version)
	if err := runPackageBuild(ctx, dir, name, version, ref, build, stderr); err != nil {
		return "", err
	}
	return ref, nil
}

// BuildPolicyPack builds a self-describing policy pack directory into an analyzer
// image and returns its convention ref
// (<registry>/pulumi/pulumi-policy-<name>:v<version>). The pack declares its identity
// at the top of PulumiPolicy.yaml — `name` (manifest-declared; packs historically named
// themselves only in code) and `version` — and its build in
// runtime.options.build:{image, command, caches}, the same self-describing contract
// PulumiPlugin.yaml gives components. The built pack still reports its own identity
// when run (GetAnalyzerInfo); publish verifies the manifest claim and the artifact
// report agree.
func BuildPolicyPack(ctx context.Context, dir, registry string, stderr io.Writer) (string, error) {
	dir, err := filepath.Abs(dir) // container workdirs must be absolute; see BuildPackage
	if err != nil {
		return "", err
	}
	proj, err := workspace.LoadPolicyPack(filepath.Join(dir, "PulumiPolicy.yaml"))
	if err != nil {
		return "", fmt.Errorf("loading PulumiPolicy.yaml in %s: %w", dir, err)
	}
	if proj.Runtime.Name() != "oci" {
		return "", fmt.Errorf("policy pack %s declares runtime %q, want oci", dir, proj.Runtime.Name())
	}
	if proj.Name == "" || proj.Version == "" {
		return "", fmt.Errorf(
			"policy pack %s must declare top-level name and version in PulumiPolicy.yaml", dir)
	}
	build, _ := proj.Runtime.Options()["build"].(map[string]any)
	ref := PolicyImageRef(registry, proj.Name, proj.Version)
	if err := runPackageBuild(ctx, dir, proj.Name, proj.Version, ref, build, stderr); err != nil {
		return "", err
	}
	return ref, nil
}

// runPackageBuild runs a package's self-described build (the options.build block shared
// by PulumiPlugin.yaml and PulumiPolicy.yaml) in a builder container, then loads the
// result into the runtime store under ref.
//
// The contract is runtime-neutral (#51): the build writes an OCI image layout to
// $PULUMI_PACKAGE_LAYOUT — naming no runtime and no location — and this function loads it
// below the PodManager seam via ImportImage, applying ref (the location the orchestrator
// resolved) at that sink. The build never touches a runtime store, so any builder on any
// runtime satisfies it; kaniko is the forcing function that proves it (it cannot reach the
// daemon socket to cheat). Identity is baked by the build (from $PULUMI_PACKAGE_NAME/
// _VERSION); location is applied at the sink; the two never mix in the build command.
func runPackageBuild(
	ctx context.Context, dir, name, version, ref string, build map[string]any, stderr io.Writer,
) error {
	buildImage, _ := build["image"].(string)
	if buildImage == "" {
		return fmt.Errorf("package %s must declare runtime.options.build.image (the build environment)", name)
	}
	command, _ := build["command"].(string)
	if command == "" {
		// Default: build the package's Dockerfile with buildkit and write a runtime-neutral
		// OCI layout to $PULUMI_PACKAGE_LAYOUT — no ref, no runtime store. The Dockerfile is
		// named relative to the builder's working directory (the package dir), so it stays
		// package-relative even when $PULUMI_BUILD_CONTEXT is widened past the package (as a
		// monorepo does) — the same way `docker build -f <path> <context>` decouples the two.
		// Needs buildkit's OCI exporter (a containerd image store, or a docker-container
		// buildx builder); see the trial-kit README.
		command = `docker build -f Dockerfile ` +
			`--output type=oci,tar=false,dest="$PULUMI_PACKAGE_LAYOUT" "$PULUMI_BUILD_CONTEXT"`
	}

	// Where the build writes its OCI layout when it takes the neutral path. A pod-provided
	// scratch dir ($PULUMI_POD_BUILD_DIR — a volume the engine mounts and the builder
	// inherits via --volumes-from) is the correct home: it lives OUTSIDE the workspace, so a
	// widened build.context can never sweep the layout into its own build context. Absent
	// that env (transitional, until every pod setup mounts the scratch volume), fall back to
	// a package-dir sibling — safe only while the context is the package dir itself.
	layoutDir := filepath.Join(filepath.Dir(dir), "."+filepath.Base(dir)+".oci-layout")
	if scratch := os.Getenv("PULUMI_POD_BUILD_DIR"); scratch != "" {
		layoutDir = filepath.Join(scratch, filepath.Base(dir)+".oci-layout")
	}
	if err := os.RemoveAll(layoutDir); err != nil {
		return fmt.Errorf("clearing build layout dir %s: %w", layoutDir, err)
	}

	// The build context — what the builder sends to the daemon/kaniko. Defaults to the
	// package dir; build.context widens it (e.g. ".." for a monorepo). Exposed to the build
	// as $PULUMI_BUILD_CONTEXT; shared with the program-image build so both widen the same way.
	contextCfg, _ := build["context"].(string)
	contextDir, err := ResolveBuildContext("package "+name, dir, contextCfg)
	if err != nil {
		return err
	}

	fmt.Fprintf(stderr, "Building %s (v%s) in %s -> %s\n", name, version, buildImage, ref)
	env := map[string]string{
		// The build's inputs are identity (what it is) plus a location-free path to write the
		// OCI layout to. The ref — where it lives — is applied at the sink (ImportImage below),
		// never here, so the build commits to no runtime and no registry.
		"PULUMI_PACKAGE_NAME":    name,
		"PULUMI_PACKAGE_VERSION": version,
		"PULUMI_PACKAGE_LAYOUT":  layoutDir,
		// The build context, defaulting to the package dir; the command references it.
		"PULUMI_BUILD_CONTEXT": contextDir,
	}
	if _, err := BuildInContainer(
		ctx, buildImage, command, dir, optStringSlice(build["caches"]), env, stderr,
	); err != nil {
		return fmt.Errorf("building package %s: %w", name, err)
	}

	// The build must have written an OCI layout — that is the whole contract. Load it below
	// the PodManager seam and apply ref there (the location the orchestrator resolved). A
	// build that emits no layout is an error, not a silent direct-load into some runtime store.
	if _, err := os.Stat(filepath.Join(layoutDir, "index.json")); err != nil {
		return fmt.Errorf("package %s: build wrote no OCI image layout to %s ($PULUMI_PACKAGE_LAYOUT); "+
			"the build command must emit one "+
			"(e.g. `docker build --output type=oci,tar=false,dest=$PULUMI_PACKAGE_LAYOUT ...` "+
			"or a kaniko `--oci-layout-path=$PULUMI_PACKAGE_LAYOUT`)", name, layoutDir)
	}
	defer func() { _ = os.RemoveAll(layoutDir) }() // served its purpose; keep the workspace clean
	fmt.Fprintf(stderr, "Importing %s from OCI layout -> %s\n", name, ref)
	podID := os.Getenv("PULUMI_POD_ID")
	if podID == "" {
		podID, _ = os.Hostname()
	}
	if err := NewPodManager(podID).ImportImage(ctx, layoutDir, ref); err != nil {
		return fmt.Errorf("importing package %s image: %w", name, err)
	}
	return nil
}

// ResolveBuildContext resolves the build context directory for a package or program build.
// It defaults to dir (the thing being built) and build.context widens it — e.g. ".." for a
// monorepo whose Dockerfile COPYs an adjacent package. The context is resolved relative to
// dir and required to be an ancestor of it (the context must contain what is being built);
// the result is what the build should reference as $PULUMI_BUILD_CONTEXT. The Dockerfile path
// stays the author's concern, named relative to the builder's working directory (dir), so it
// remains dir-relative however wide the context grows — the way `docker build -f <path>
// <context>` decouples the two. label names the thing being built, for error messages.
//
// It does NOT check the context stays within the pod's workspace mount: a context that escapes
// it simply isn't visible to the builder (which inherits only the engine's mounts), so the
// build fails on the missing files. Keeping the widened context under the mounted repo root is
// the caller's concern — the wrapper mounts a parent via PULUMI_POD_MOUNT_DIR for exactly this.
func ResolveBuildContext(label, dir, context string) (string, error) {
	if context == "" {
		return dir, nil
	}
	contextDir := filepath.Join(dir, context)
	if fi, err := os.Stat(contextDir); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("%s: build.context %q resolves to %s, not an accessible directory",
			label, context, contextDir)
	}
	if !strings.HasPrefix(dir+string(os.PathSeparator), contextDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s: build.context %q (%s) must be an ancestor of %s",
			label, context, contextDir, dir)
	}
	return contextDir, nil
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
	pod := NewPodManager(podID)
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
