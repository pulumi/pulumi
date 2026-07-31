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
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// ProviderFromImage runs a provider image as a one-shot pod container and attaches a
// provider client to it, so a caller can read its schema (`pulumi package add
// oci://<ref>`). It is the dev-time twin of the runtime provider lifecycle: the engine
// consumes providers as images when it runs, so acquiring one at package time is the
// same move one level up — run the ref, talk to it — rather than spawning a binary at a
// filesystem path. This is the seam that lets the package commands resolve a source to a
// ref instead of a path (the dev-time half of "refs are the internal currency").
//
// It builds the pod machinery from the environment, so it must run inside the engine
// container (pod mode): the schema container runs on the pod network, dialed at the
// well-known port (Attach delivers a reachable engine address back), and the
// configured plugin registry/proxy supplies the image — exactly as for a runtime
// provider. The returned stop function removes the schema container; the caller also Closes
// the returned provider. Crucially this stops only that one container, not the whole pod —
// the engine's own pod must outlive a package-time schema fetch.
func ProviderFromImage(ctx *plugin.Context, ref string) (plugin.Provider, func() error, error) {
	var host plugin.Host
	var err error
	switch os.Getenv("PULUMI_POD_MODE") {
	case "true":
		host, err = NewContainerHostFromEnv(ctx.Host)
	case "host":
		host, err = NewHostEngineHostFromEnv(ctx.Host)
	default:
		return nil, nil, fmt.Errorf(
			"oci: oci:// package sources require pod mode (run inside the engine container, "+
				"or PULUMI_POD_MODE=host for a host engine); PULUMI_POD_MODE is not set for %q", ref)
	}
	if err != nil {
		return nil, nil, err
	}
	return host.(*containerHost).providerFromImage(ctx, ref)
}

// providerFromImage runs ref as a stateless provider container (its own image,
// ENTRYPOINT-driven), scrapes the handshake it prints, and attaches at the
// well-known port, exactly as Provider() does. It mirrors Provider()'s stateless
// archetype but takes a bare image ref rather than a plugin descriptor, and returns
// a per-container stop rather than relying on the host's pod-wide Cleanup — the
// engine's pod is still live around this call.
func (h *containerHost) providerFromImage(
	ctx *plugin.Context, ref string,
) (plugin.Provider, func() error, error) {
	// An oci:// source is by definition a fully-qualified, self-locating ref, so it
	// is pull-eligible like a pinned descriptor: the registry knob qualifies
	// convention refs and has nothing to add here.
	if err := h.ensureImage(ctx.Base(), ref, ref, true /*pinned*/); err != nil {
		return nil, nil, err
	}

	cfg := ContainerConfig{
		// Named for the provider, not the full ref: names are identity, and a
		// short one keeps docker ps and log lines readable. The seq suffix
		// disambiguates concurrent fetches; the ref is in the log line below.
		Name:    uniqueContainerName("schema-" + sanitizeContainerName(providerNameFromRef(ref))),
		Network: h.providerNetwork(),
		Image:   ref,
		// Project the engine's environment as for any pod member; a provider whose
		// GetSchema is parameterized may need credentials to talk to its upstream.
		Env: projectedProviderEnv(),
	}
	// The same bind contract as a runtime provider: the schema container serves
	// the well-known port in its own netns (a shim-wrapped image does the same by
	// proxy).
	h.requestWellKnownBind(&cfg)

	c, err := h.pod.RunContainer(ctx.Base(), cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("oci: starting provider image %q: %w", ref, err)
	}
	// WithoutCancel so a cancelled fetch still tears the container down.
	stop := func() error { return h.pod.StopContainer(context.WithoutCancel(ctx.Base()), c) }

	port, err := scrapeServingPort(ctx.Base(), h.pod, c)
	if err != nil {
		return nil, nil, errStop(stop, fmt.Errorf("oci: discovering port for provider image %q: %w", ref, err))
	}

	// The handshake honesty check, as in Provider(): a schema container announcing
	// any port but the requested one ignored the bind contract and is serving
	// loopback where the engine cannot reach it — name the diagnosis now.
	if port != pluginListenPort {
		return nil, nil, errStop(stop, fmt.Errorf(
			"oci: provider image %q announced port %d instead of the requested listen port %d: the "+
				"plugin ignored PULUMI_PLUGIN_LISTEN_ADDRESS (built against an SDK without the bind "+
				"contract) and its image does not wrap it in the forwarder shim, so it is serving "+
				"loopback inside its own container where the engine cannot reach it. Rebuild the "+
				"plugin against a newer SDK, or use a shim-wrapped image",
			ref, port, pluginListenPort))
	}

	attachAddr := h.attachAddress(c, port)
	fmt.Fprintf(os.Stderr, "oci: provider image %q running as container %s, attaching at %s\n",
		ref, c.Name, attachAddr)

	descriptor := workspace.PluginDescriptor{Name: providerNameFromRef(ref)}
	p, err := plugin.NewProviderAttached(h, ctx, descriptor, attachAddr, ctx.DisableProviderPreview())
	if err != nil {
		return nil, nil, errStop(stop, fmt.Errorf("oci: attaching to provider image %q: %w", ref, err))
	}
	return p, stop, nil
}

// errStop runs the container teardown while returning the original error, joining any
// teardown failure so a leak is not silently swallowed.
func errStop(stop func() error, err error) error {
	if stopErr := stop(); stopErr != nil {
		return fmt.Errorf("%w (and stopping the container failed: %v)", err, stopErr)
	}
	return err
}

// providerNameFromRef extracts a best-effort provider name from an image ref for the
// plugin descriptor's identity (used only for logging and the dial prefix; the schema
// itself carries the authoritative name). It strips any registry/host and tag and the
// pulumi-provider- convention prefix: e.g.
// localhost:5000/pulumi-provider-random:v4.21.0 -> "random". Falls back to "provider".
func providerNameFromRef(ref string) string {
	name := ref
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:] // drop registry/host path
	}
	if i := strings.LastIndex(name, ":"); i >= 0 {
		name = name[:i] // drop :tag
	}
	name = strings.TrimPrefix(name, "pulumi-provider-")
	if name == "" {
		return "provider"
	}
	return name
}
