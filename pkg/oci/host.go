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
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// containerHost wraps a base plugin.Host so that resource providers run as
// containers on the pod instead of as host child processes. Every method except
// Provider and Close is inherited unchanged from the base host (the language
// runtime, analyzers, schema loader, and so on still behave normally); only the
// provider lifecycle is rerouted through the pod.
//
// Providers join the engine container's network namespace, so a stock provider
// binary — which binds 127.0.0.1 and which the engine dials at a hardcoded
// 127.0.0.1 — works unmodified over the shared loopback. The host starts the
// container, reads the port the provider prints, and attaches to it.
type containerHost struct {
	plugin.Host

	pod            PodManager
	engineHost     string                                  // engine container name; providers share its netns
	programImage   string                                  // program image; workspace-coupled providers run from it
	pluginRegistry string                                  // OCI registry to pull absent provider images from ("" = assume present)
	podID          string                                  // pod id; names the shared workspace volume both hosts mount
	imageFor       func(workspace.PluginDescriptor) string // provider descriptor -> image ref
}

// WorkspaceMountPath is where the per-pod shared workspace volume is mounted, in both
// the program container and provider containers — the program's working directory, so
// any workspace path the program references (a Pulumi asset, or a provider's file-path
// property such as cloudflare's asset directory) resolves identically in the provider.
const WorkspaceMountPath = "/app"

// WorkspaceVolumeLogical is the logical name passed to PodManager.CreateVolume for the
// shared workspace volume; WorkspaceVolumeName derives the runtime name it resolves to.
const WorkspaceVolumeLogical = "workspace"

// WorkspaceVolumeName is the runtime name of the per-pod shared workspace volume. It is
// derived purely from the pod id so the language host (which creates it and mounts it
// into the program) and the plugin host (which mounts it into providers) agree on one
// volume without coordinating. It must match dockerPodManager.CreateVolume's naming.
func WorkspaceVolumeName(podID string) string {
	return fmt.Sprintf("pulumi-pod-%s-vol-%s", podID, WorkspaceVolumeLogical)
}

// Assert containerHost still satisfies the full Host interface after wrapping.
var _ plugin.Host = (*containerHost)(nil)

// NewContainerHost wraps base so that Provider() runs the provider as a container
// in engineHost's network namespace (via pod) and attaches to it. engineHost is
// the engine container's name; peers reach its loopback by sharing its netns.
// programImage is the program's image; workspace-coupled providers (command,
// docker-build, ...) run from it — rooted in the program's filesystem — rather
// than from their own image. It may be empty when no such provider is used.
// pluginRegistry, if non-empty, is an OCI registry from which absent provider
// images are pulled (and retagged to the bare convention) before use — the
// container-model "install" step. Empty preserves the prior behaviour: an absent
// image is assumed prebuilt/loaded and surfaces at run time if it is not.
func NewContainerHost(base plugin.Host, pod PodManager, engineHost, programImage, pluginRegistry, podID string) plugin.Host {
	return &containerHost{
		Host:           base,
		pod:            pod,
		engineHost:     engineHost,
		programImage:   programImage,
		pluginRegistry: pluginRegistry,
		podID:          podID,
		imageFor: func(spec workspace.PluginDescriptor) string {
			return providerImageRef(pluginRegistry, spec)
		},
	}
}

// NewContainerHostFromEnv builds a container host from the pod environment:
// PULUMI_POD_ADVERTISE_HOST (else the process hostname) names the engine
// container whose netns providers join, PULUMI_POD_ID labels the pod so its
// containers can be cleaned up as a group, and PULUMI_POD_PROGRAM_IMAGE is the
// program image that workspace-coupled providers run from.
func NewContainerHostFromEnv(base plugin.Host) (plugin.Host, error) {
	engineHost := os.Getenv("PULUMI_POD_ADVERTISE_HOST")
	if engineHost == "" {
		h, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("oci: determining engine container hostname: %w", err)
		}
		engineHost = h
	}
	podID := os.Getenv("PULUMI_POD_ID")
	if podID == "" {
		podID = engineHost
	}
	programImage := os.Getenv("PULUMI_POD_PROGRAM_IMAGE")
	// Optional: an OCI registry to pull provider plugin images from on demand.
	pluginRegistry := os.Getenv("PULUMI_POD_PLUGIN_REGISTRY")
	return NewContainerHost(base, NewDockerPodManager(podID), engineHost, programImage, pluginRegistry, podID), nil
}

// ProviderImageRef returns the OCI image reference for a provider plugin, by
// convention. When registry is non-empty the ref is *qualified* with it
// (<registry>/pulumi-provider-<name>:v<version>) so that resolution, pull, and a
// future publish-by-push all share one fully-qualified name; an empty registry
// yields the bare ref (unchanged prior behaviour). Docker tags cannot contain
// '+', so semver build metadata (e.g. a dev build's 0.1.0-alpha.0+dev) is mapped
// to a tag-safe '_'.
//
// This is the single source of truth for the convention: the container host uses
// it to resolve provider images, and the OCI language host uses it to tag the
// local component images it builds, so the two cannot drift.
func ProviderImageRef(registry, name, version string) string {
	tag := ""
	if version != "" {
		tag = "v" + strings.ReplaceAll(version, "+", "_")
	}
	ref := fmt.Sprintf("pulumi-provider-%s:%s", name, tag)
	if registry != "" {
		ref = registry + "/" + ref
	}
	return ref
}

// providerImageRef resolves a plugin descriptor to its image ref via the
// convention above, qualified with registry when one is configured.
func providerImageRef(registry string, spec workspace.PluginDescriptor) string {
	version := ""
	if spec.Version != nil {
		version = spec.Version.String()
	}
	return ProviderImageRef(registry, spec.Name, version)
}

// providerBinDir is the directory in which a (prototype) provider image lays its
// binary, named "provider". It is a directory so the dir-oriented CopyFromImage
// can inject it wholesale into a program-image run; injectedBinPath is where it
// lands and is exec'd from. Stateless providers ignore this and run their image's
// own ENTRYPOINT (which points at providerBinDir/provider).
const (
	providerBinDir   = "/plugin"
	injectedBinDir   = "/plugins"
	injectedBinPath  = injectedBinDir + "/provider"
	pluginVolumePrfx = "plugin-"
)

// workspaceCoupled reports whether a provider must run rooted in the program's
// filesystem — for its workspace and toolchain — rather than its own image. The
// `command` provider shells out to the user's toolchain; `docker-build` resolves
// a build context from the workspace. This is the prototype's convention table;
// pre-start image labels are the generalizing layer (see the design doc). Cloud
// providers are not workspace-coupled — they run from their own image.
func workspaceCoupled(name string) bool {
	switch name {
	case "command", "docker", "docker-build":
		return true
	}
	return false
}

// roleEnvVar selects which entrypoint a program image boots into. The program
// image's bootstrap shim reads it: unset → run the program (the run harness);
// roleDynamicProvider → serve the SDK's dynamic-provider entrypoint instead;
// rolePolicyPack → serve the policy pack's analyzer (the run-policy-pack harness).
// The host stays language-agnostic — it only sets the role — while the image owns
// the translation, exactly as the shim already owns the PULUMI_* → run-harness
// mapping.
const (
	roleEnvVar          = "PULUMI_OCI_ROLE"
	roleDynamicProvider = "dynamic-provider"
	rolePolicyPack      = "policy-pack"
)

// isDynamicProvider reports whether a provider package is a language SDK's
// dynamic-provider package. Unlike a stock provider, these are not standalone
// plugins: the CRUD code is serialized from the program and ships in-band as a
// resource property, and the binary that runs it is the SDK's own dynamic-provider
// entrypoint — already present in (and welded to) the program image. So a dynamic
// provider runs from the program image with nothing to inject. The engine
// special-cases the same two package names (pkg/resource/deploy/target.go).
func isDynamicProvider(name string) bool {
	return name == "pulumi-nodejs" || name == "pulumi-python"
}

// capability is a symbolic host resource a provider asks the pod to project into
// its container (the docker socket, an SSH agent, cloud credentials). The provider
// declares the need; the pod resolves it to a concrete, pod-conventional source —
// so the provider never sees the host-side path, which is environment-dependent
// (e.g. $DOCKER_HOST). Prototype: a convention table; image labels generalize it.
type capability string

const capDockerSocket capability = "docker-socket"

// dockerSocketPath is the pod-conventional docker socket location. The driver
// mounts the host's $DOCKER_HOST socket here when it creates the pod, so inside
// the pod the socket is always at this fixed path regardless of where it lives on
// the host.
const dockerSocketPath = "/var/run/docker.sock"

// providerCapabilities lists the capabilities a provider needs projected.
// docker-build needs a docker/buildkit endpoint to run builds and reaches it over
// the projected socket. Convention table for the prototype; pre-start image labels
// (com.pulumi.needs: docker-socket, ...) are the generalizing layer.
func providerCapabilities(name string) []capability {
	switch name {
	case "docker", "docker-build":
		return []capability{capDockerSocket}
	}
	return nil
}

// capabilityMount resolves a capability to the mount that satisfies it.
func capabilityMount(need capability) (VolumeMount, bool) {
	switch need {
	case capDockerSocket:
		return VolumeMount{Source: dockerSocketPath, Target: dockerSocketPath}, true
	}
	return VolumeMount{}, false
}

// contextOwnedEnv reports whether an environment variable belongs to the engine's
// own container context and so must NOT be projected onto a provider container. The
// provider image owns PATH/HOME/etc. (projecting the engine's would clobber them —
// e.g. a program image with its language venv on PATH); PYTHONPATH/NODE_PATH are
// language module paths the provider image likewise owns; DOCKER_HOST points at a
// socket path valid only on the host (the docker socket is bind-mounted at a fixed
// path instead, via the docker-socket capability); PULUMI_HOME and PULUMI_BACKEND_URL
// point at engine-orchestration state that lives inside the engine's workspace mount
// (the pod home and the file backend), for which a provider container has neither the
// path nor a need — and an MLC/component container, itself a Pulumi program, would
// actively misread a PULUMI_HOME pointing at a path it does not have; and the
// PULUMI_POD_* family is pod-control state the provider is not party to.
//
// This is the same denylist shape the pulumi-pod wrapper applies on the host → engine
// hop; both exist because the env crosses a filesystem boundary (host → container, or
// engine image → provider image) where path-valued vars stop being valid.
func contextOwnedEnv(key string) bool {
	switch key {
	case "PATH", "HOME", "PWD", "HOSTNAME", "SHLVL", "TERM", "DOCKER_HOST",
		"PYTHONPATH", "NODE_PATH", "PULUMI_HOME", "PULUMI_BACKEND_URL":
		return true
	}
	return strings.HasPrefix(key, "PULUMI_POD_")
}

// projectedProviderEnv copies the engine's environment for a provider container —
// the container analogue of a spawned provider inheriting the engine's os.Environ().
//
// This is the deliberate "project the whole environment" policy, symmetric with how
// providers are exec'd today: credentials (AWS_*, GOOGLE_*, ARM_*, …) and other
// runtime env ride along. The design decision behind it: **provider credentials
// travel as environment (plus ESC via config), never via a host-filesystem mount.**
// A host mount (e.g. ~/.aws) would couple to host layout and break remote execution
// (a remote executor has no host home), whereas env travels with the execution unit.
//
// Context-owned vars (see contextOwnedEnv) are dropped so they don't clobber the
// provider image's own environment. Per-provider scoping of *what* is projected —
// letting a provider declare the creds/mounts it needs so it can be treated as
// untrusted-ish — is a deliberate future enhancement (design §8 isolation), not done
// here: today providers run fully privileged, exactly as under the process model.
func projectedProviderEnv() map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || contextOwnedEnv(k) {
			continue
		}
		out[k] = v
	}
	return out
}

// Provider starts the provider as a container sharing the engine's network
// namespace and attaches to it, rather than spawning a plugin binary. Stateless
// providers run from their own image; workspace-coupled providers run from the
// program image with their binary injected (see providerContainer).
func (h *containerHost) Provider(
	ctx *plugin.Context, descriptor workspace.PluginDescriptor, _ env.Env,
) (plugin.Provider, error) {
	cfg, err := h.providerContainer(ctx.Base(), descriptor)
	if err != nil {
		return nil, err
	}

	c, err := h.pod.RunContainer(ctx.Base(), cfg)
	if err != nil {
		return nil, fmt.Errorf("oci: starting provider container %q for %s: %w", cfg.Image, descriptor.Name, err)
	}

	port, err := scrapeServingPort(ctx.Base(), h.pod, c)
	if err != nil {
		_ = h.pod.StopContainer(context.Background(), c)
		return nil, fmt.Errorf("oci: discovering port for provider %s: %w", descriptor.Name, err)
	}

	fmt.Fprintf(os.Stderr, "oci: provider %s running as container %s, attaching at 127.0.0.1:%d\n",
		descriptor.Name, c.Name, port)
	return plugin.NewProviderAttached(h, ctx, descriptor, port, ctx.DisableProviderPreview())
}

// providerContainer builds the spec for a provider container, on the engine's
// netns so the provider binds 127.0.0.1 and the engine reaches it over the shared
// loopback. A stateless provider runs from its own image. A workspace-coupled
// provider instead runs from the *program* image — rooted in the program's
// filesystem so it sees the workspace and toolchain — with its binary injected
// from the provider image via an ephemeral, pod-scoped volume. See the design
// doc's "execution as one primitive" section.
func (h *containerHost) providerContainer(
	ctx context.Context, descriptor workspace.PluginDescriptor,
) (ContainerConfig, error) {
	cfg := ContainerConfig{
		Name:    "provider-" + descriptor.Name,
		Network: "container:" + h.engineHost,
		// Project the engine's environment onto every provider — the container
		// analogue of a spawned provider inheriting os.Environ(). This is how a
		// provider's credentials reach it; see projectedProviderEnv.
		Env: projectedProviderEnv(),
	}

	// Dynamic providers are native to the program image: the SDK's dynamic-provider
	// entrypoint already ships in it, and the serialized CRUD closure resolves
	// against the program's own dependency closure — baked into that image. So,
	// unlike a workspace-coupled provider, there is nothing to inject: no separate
	// provider image, no binary copy, no ensure step. Run a fresh container from the
	// program image and let its bootstrap shim boot the dynamic-provider entrypoint,
	// selected by roleEnvVar.
	//
	// This is the same `docker run program-image` primitive as the other
	// workspace-context providers (command, docker) — deliberately, so execution
	// stays one uniform, recursive primitive with no coupling to a live program
	// container's lifetime. A dynamic provider that needs the program's *live*
	// runtime filesystem (files the program wrote at run time, not baked into the
	// image) is the documented exception — served by the runtime volume-mount escape
	// hatch or the procfs/exec-into-live fallback (see the design doc), not by making
	// dynamic the odd one out here.
	if isDynamicProvider(descriptor.Name) {
		if h.programImage == "" {
			return ContainerConfig{}, fmt.Errorf(
				"oci: dynamic provider %s needs the program image, but PULUMI_POD_PROGRAM_IMAGE is unset",
				descriptor.Name)
		}
		fmt.Fprintf(os.Stderr,
			"oci: provider %s is a dynamic provider — running from program image %s (SDK entrypoint, nothing injected)\n",
			descriptor.Name, h.programImage)
		cfg.Image = h.programImage
		cfg.Env[roleEnvVar] = roleDynamicProvider
		return cfg, nil
	}

	// Resolve the provider image and ensure it is present — pulling it from the
	// configured registry if absent (the container-model install step). Both
	// archetypes need it: a stateless provider runs it directly; a workspace-coupled
	// provider copies its binary out of it.
	providerImage := h.imageFor(descriptor)
	if err := h.ensureImage(ctx, descriptor.Name, providerImage); err != nil {
		return ContainerConfig{}, err
	}

	if workspaceCoupled(descriptor.Name) {
		if h.programImage == "" {
			return ContainerConfig{}, fmt.Errorf(
				"oci: provider %s needs the program filesystem, but PULUMI_POD_PROGRAM_IMAGE is unset",
				descriptor.Name)
		}
		// Inject the provider binary into an ephemeral volume, then run it from the
		// program image. The volume is pod-scoped and torn down by Close()/Cleanup().
		vol, err := h.pod.CreateVolume(ctx, pluginVolumePrfx+descriptor.Name)
		if err != nil {
			return ContainerConfig{}, fmt.Errorf("oci: creating plugin volume for %s: %w", descriptor.Name, err)
		}
		if err := h.pod.CopyFromImage(ctx, providerImage, providerBinDir, vol, injectedBinDir); err != nil {
			return ContainerConfig{}, fmt.Errorf("oci: injecting %s provider binary: %w", descriptor.Name, err)
		}
		fmt.Fprintf(os.Stderr,
			"oci: provider %s is workspace-coupled — running from program image %s with injected binary\n",
			descriptor.Name, h.programImage)
		cfg.Image = h.programImage
		cfg.Volumes = append(cfg.Volumes, VolumeMount{Source: vol.Name, Target: injectedBinDir})
		cfg.Entrypoint = []string{injectedBinPath}
	} else {
		// A stateless provider runs from its own image, but it may still need to read
		// the program's workspace: a Pulumi asset, or a provider file-path property
		// (e.g. cloudflare's WorkerVersion asset directory) points at a path under the
		// program's working dir. Mount the shared workspace volume at the same path so
		// those reads resolve. The program populates this volume at runtime (and Docker
		// seeds it from the program image), so the provider sees the live workspace —
		// including build outputs the program produced during evaluation. Providers that
		// do not read files simply ignore the mount.
		cfg.Image = providerImage
		cfg.Volumes = append(cfg.Volumes,
			VolumeMount{Source: WorkspaceVolumeName(h.podID), Target: WorkspaceMountPath})
	}

	// Project the host capabilities the provider declares it needs (docker socket,
	// etc.) — applies to both archetypes; a cloud provider could ask for creds.
	for _, need := range providerCapabilities(descriptor.Name) {
		m, ok := capabilityMount(need)
		if !ok {
			return ContainerConfig{}, fmt.Errorf(
				"oci: provider %s requested unknown capability %q", descriptor.Name, need)
		}
		cfg.Volumes = append(cfg.Volumes, m)
		fmt.Fprintf(os.Stderr, "oci: provider %s gets capability %q at %s\n", descriptor.Name, need, m.Target)
	}
	return cfg, nil
}

// ensureImage makes a provider image present in the local store before it is run
// or copied from — the container-model install step (the OCI analogue of
// downloading a plugin binary). If it is already present, this is a no-op.
// Otherwise, when a plugin registry is configured, the image is pulled. The ref
// is already registry-qualified by providerImageRef, so it is pulled and run
// under the same fully-qualified name — no retag.
//
// With no registry configured and the image absent, it cannot be installed, so we
// bail out here with an actionable message rather than letting the downstream
// `docker run`/CopyFromImage fail with a cryptic "Unable to find image / pull
// access denied" — the error a user hits when they forget `pulumi install` for a
// local component, or have not set a registry for a published one.
//
// This runs in Provider() (acquire-on-use), which is provably reached for every
// provider. Hoisting it to a pre-flight ensure step (parallel pre-pull, fail-fast,
// the `pulumi install` hook) is a natural follow-on — same pull, earlier.
func (h *containerHost) ensureImage(ctx context.Context, name, ref string) error {
	has, err := h.pod.ImageExists(ctx, ref)
	if err != nil {
		return fmt.Errorf("oci: checking for plugin image %s: %w", ref, err)
	}
	if has {
		return nil
	}
	if h.pluginRegistry == "" {
		return fmt.Errorf(
			"oci: provider %q has no image: %s is not present locally and no plugin registry "+
				"is configured to install it from. Run `pulumi install` if it is a local "+
				"component, or set PULUMI_POD_PLUGIN_REGISTRY to a registry that has it",
			name, ref)
	}
	fmt.Fprintf(os.Stderr, "oci: plugin image %s not present — pulling\n", ref)
	if err := h.pod.PullImage(ctx, ref); err != nil {
		return fmt.Errorf("oci: pulling plugin image %s: %w", ref, err)
	}
	fmt.Fprintf(os.Stderr, "oci: installed plugin %s by pulling its image\n", ref)
	return nil
}

// PolicyAnalyzer runs a policy pack as a container in the pod and attaches an
// analyzer client to it, rather than booting the policy pack as a host process via
// a language plugin. A policy pack is just another containerized program — its
// PulumiPolicy.yaml is the analyzer analogue of Pulumi.yaml — so a pack that
// declares `runtime: oci` (with an `image` option) is built and run exactly like a
// program or MLC, and the engine drives its Analyzer gRPC surface
// (GetAnalyzerInfo/Analyze/AnalyzeStack) over the shared loopback. This reuses the
// same build-and-run-from-image mechanism as MLCs; the one genuinely new bit is the
// analyzer protocol, which the engine speaks to a *server* (the pack binds
// 127.0.0.1, prints its port, raises its own message-size limit), so unlike a
// provider there is no Attach RPC to issue — we dial and hand the engine a client.
//
// A pack that does not opt into the OCI runtime falls back to the base host's
// normal spawn path, so non-containerized policy packs keep working unchanged.
//
// The `path` the engine passes is, in the normal form, an *image ref* the host has
// already resolved — the engine consumes a ref and never reads a manifest off a
// mount. A local *directory* still works as a dev-time input (we read its
// PulumiPolicy.yaml in-place), but that path is the exception, not the currency:
// refs are what cross into the engine. This is what lets the pod ship the pack's
// manifest nowhere — only its image ref + its image. See policyPackImage.
func (h *containerHost) PolicyAnalyzer(
	ctx *plugin.Context, name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions,
) (plugin.Analyzer, error) {
	image, ok, err := policyPackImage(path)
	if err != nil {
		return nil, err
	}
	if !ok {
		// Not an OCI policy pack — defer to the base host's spawn path.
		return h.Host.PolicyAnalyzer(ctx, name, path, opts)
	}

	if err := h.ensureImage(ctx.Base(), string(name), image); err != nil {
		return nil, err
	}

	cfg := ContainerConfig{
		Name:    "policy-" + sanitizeContainerName(filepath.Base(path)),
		Network: "container:" + h.engineHost,
		Image:   image,
		// Project the engine's environment (credentials and so on), as for any pod
		// member; a policy that performs provider invokes needs them just like a
		// provider does. See projectedProviderEnv.
		Env: projectedProviderEnv(),
	}
	cfg.Env[roleEnvVar] = rolePolicyPack
	// Hand the pack the engine address it would normally receive as argv. The pack
	// is a server the engine calls, but it may dial back for invokes/logging; it
	// shares the engine netns, so the engine's own ServerAddr is reachable.
	cfg.Env["PULUMI_ENGINE"] = h.ServerAddr()

	c, err := h.pod.RunContainer(ctx.Base(), cfg)
	if err != nil {
		return nil, fmt.Errorf("oci: starting policy pack container %q for %s: %w", image, name, err)
	}

	port, err := scrapeServingPort(ctx.Base(), h.pod, c)
	if err != nil {
		_ = h.pod.StopContainer(context.Background(), c)
		return nil, fmt.Errorf("oci: discovering port for policy pack %s: %w", name, err)
	}

	fmt.Fprintf(os.Stderr, "oci: policy pack %s running as container %s, attaching at 127.0.0.1:%d\n",
		name, c.Name, port)

	client, err := dialAnalyzer(ctx.Base(), port)
	if err != nil {
		_ = h.pod.StopContainer(context.Background(), c)
		return nil, fmt.Errorf("oci: attaching to policy pack %s: %w", name, err)
	}
	return plugin.NewAnalyzerWithClient(name, client), nil
}

// policyPackImage resolves a policy pack reference to the container image to run it
// from. The reference is one of two things:
//
//   - An image ref (the normal form). When path is not a local directory, it *is*
//     the image — the host resolved the pack to its image before handing it to the
//     engine, so the engine runs a ref and reads no manifest. This is the currency:
//     a ref, like a provider's, crosses into the engine.
//   - A local directory (a dev-time path input). We read its PulumiPolicy.yaml in
//     place; if it declares `runtime: oci` we take its `image` option, otherwise ok
//     is false and the caller falls back to the base host's spawn path. An OCI pack
//     that names no image is an error — there is nothing to run.
//
// Distinguishing the two by "is this a directory?" keeps the dev convenience
// (point at a pack on disk) without making a filesystem path the thing the engine
// depends on — the manifest-projection hack the ref form removes.
func policyPackImage(path string) (image string, ok bool, err error) {
	if info, statErr := os.Stat(path); statErr != nil || !info.IsDir() {
		// Not a directory we can read a manifest from → an image ref the host
		// already resolved. The engine consumes it directly.
		return path, true, nil
	}

	projPath := filepath.Join(path, "PulumiPolicy.yaml")
	proj, err := workspace.LoadPolicyPack(projPath)
	if err != nil {
		return "", false, fmt.Errorf("oci: loading policy pack project %q: %w", projPath, err)
	}
	if proj.Runtime.Name() != "oci" {
		return "", false, nil
	}
	image, _ = proj.Runtime.Options()["image"].(string)
	if image == "" {
		return "", false, fmt.Errorf(
			"oci: policy pack %q declares runtime oci but sets no image option", path)
	}
	return image, true, nil
}

// dialAnalyzer connects to a policy pack's analyzer server on the shared loopback
// and returns a client. It raises the gRPC message-size limit to match the engine's
// other plugin connections (rpcutil.GrpcChannelOptions) — AnalyzeStack ships the
// whole resource set, so the 4 MB default is far too small. The pack already printed
// its port, meaning it is bound and serving, so we just wait for the connection to
// report ready before handing the client to the engine.
func dialAnalyzer(ctx context.Context, port int) (pulumirpc.AnalyzerClient, error) {
	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("dialing analyzer: %w", err)
	}
	conn.Connect()

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		s := conn.GetState()
		if s == connectivity.Ready {
			return pulumirpc.NewAnalyzerClient(conn), nil
		}
		if !conn.WaitForStateChange(waitCtx, s) {
			_ = conn.Close()
			return nil, errors.New("analyzer did not begin responding before timeout")
		}
	}
}

// sanitizeContainerName maps an arbitrary string to a Docker-safe container-name
// fragment (alphanumerics, dash, underscore, dot). A policy pack is identified to
// the engine by its filesystem path, which is not a legal container name.
func sanitizeContainerName(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, s)
}

// Close tears down the provider containers this host started, then closes the
// base host. The promoted SignalCancellation/ReleaseContext never reach these
// containers (the base host has no record of them), so this is the cleanup hook.
func (h *containerHost) Close() error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return errors.Join(h.pod.Cleanup(cleanupCtx), h.Host.Close())
}

// scrapeServingPort follows a provider container's logs until it prints the port
// line of the plugin handshake — a bare integer on its own line — ignoring any
// interleaved diagnostics. It gives up after a timeout or once output ends.
func scrapeServingPort(ctx context.Context, pod PodManager, c Container) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rc, err := pod.ContainerLogs(ctx, c, true)
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		if port, err := strconv.Atoi(strings.TrimSpace(scanner.Text())); err == nil {
			return port, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, errors.New("container output ended before a port was printed")
}
