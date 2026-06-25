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

	pod        PodManager
	engineHost string                                  // engine container name; providers share its netns
	imageFor   func(workspace.PluginDescriptor) string // provider descriptor -> image ref
}

// Assert containerHost still satisfies the full Host interface after wrapping.
var _ plugin.Host = (*containerHost)(nil)

// NewContainerHost wraps base so that Provider() runs the provider as a container
// in engineHost's network namespace (via pod) and attaches to it. engineHost is
// the engine container's name; peers reach its loopback by sharing its netns.
func NewContainerHost(base plugin.Host, pod PodManager, engineHost string) plugin.Host {
	return &containerHost{
		Host:       base,
		pod:        pod,
		engineHost: engineHost,
		imageFor:   providerImageRef,
	}
}

// NewContainerHostFromEnv builds a container host from the pod environment:
// PULUMI_POD_ADVERTISE_HOST (else the process hostname) names the engine
// container whose netns providers join, and PULUMI_POD_ID labels the pod so its
// containers can be cleaned up as a group.
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
	return NewContainerHost(base, NewDockerPodManager(podID), engineHost), nil
}

// providerImageRef maps a provider plugin descriptor to its container image by
// convention. For the prototype the image is assumed already present (the smoke
// test prebuilds it from the stock binary); a registry pull or an install-time
// wrap would slot in here without changing the rest of the host.
func providerImageRef(spec workspace.PluginDescriptor) string {
	version := ""
	if spec.Version != nil {
		version = "v" + spec.Version.String()
	}
	return fmt.Sprintf("pulumi-provider-%s:%s", spec.Name, version)
}

// Provider starts the provider as a container sharing the engine's network
// namespace and attaches to it, rather than spawning a plugin binary.
func (h *containerHost) Provider(
	ctx *plugin.Context, descriptor workspace.PluginDescriptor, _ env.Env,
) (plugin.Provider, error) {
	image := h.imageFor(descriptor)
	c, err := h.pod.RunContainer(ctx.Base(), ContainerConfig{
		Image: image,
		Name:  "provider-" + descriptor.Name,
		// container:<engine> shares the engine's netns: the provider binds
		// 127.0.0.1 and the engine reaches it over the shared loopback, so the
		// stock binary and the engine's hardcoded 127.0.0.1 dial both work as-is.
		Network: "container:" + h.engineHost,
	})
	if err != nil {
		return nil, fmt.Errorf("oci: starting provider container %q for %s: %w", image, descriptor.Name, err)
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
