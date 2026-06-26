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
	"strconv"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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
	imageFor       func(workspace.PluginDescriptor) string // provider descriptor -> image ref
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
func NewContainerHost(base plugin.Host, pod PodManager, engineHost, programImage, pluginRegistry string) plugin.Host {
	return &containerHost{
		Host:           base,
		pod:            pod,
		engineHost:     engineHost,
		programImage:   programImage,
		pluginRegistry: pluginRegistry,
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
	return NewContainerHost(base, NewDockerPodManager(podID), engineHost, programImage, pluginRegistry), nil
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
	}

	// Resolve the provider image and ensure it is present — pulling it from the
	// configured registry if absent (the container-model install step). Both
	// archetypes need it: a stateless provider runs it directly; a workspace-coupled
	// provider copies its binary out of it.
	providerImage := h.imageFor(descriptor)
	if err := h.ensureImage(ctx, providerImage); err != nil {
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
		cfg.Image = providerImage
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
// under the same fully-qualified name — no retag. With no registry configured a
// missing image is left alone, so the later run/copy surfaces the absence (the
// prior assume-prebuilt behaviour).
//
// This runs in Provider() (acquire-on-use), which is provably reached for every
// provider. Hoisting it to a pre-flight ensure step (parallel pre-pull, fail-fast,
// the `pulumi install` hook) is a natural follow-on — same pull, earlier.
func (h *containerHost) ensureImage(ctx context.Context, ref string) error {
	has, err := h.pod.ImageExists(ctx, ref)
	if err != nil {
		return fmt.Errorf("oci: checking for plugin image %s: %w", ref, err)
	}
	if has {
		return nil
	}
	if h.pluginRegistry == "" {
		return nil // no registry to install from; let the run/copy report the absence
	}
	fmt.Fprintf(os.Stderr, "oci: plugin image %s not present — pulling\n", ref)
	if err := h.pod.PullImage(ctx, ref); err != nil {
		return fmt.Errorf("oci: pulling plugin image %s: %w", ref, err)
	}
	fmt.Fprintf(os.Stderr, "oci: installed plugin %s by pulling its image\n", ref)
	return nil
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
