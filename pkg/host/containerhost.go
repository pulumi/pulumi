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

package host

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin/oci"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// containerHost decorates a plugin.Host to run container-image policy packs.
// Its PolicyAnalyzer intercepts packs that arrive as a digest-pinned image ref
// (server-enforced), a local directory whose manifest declares runtime "oci",
// or an attach port (PULUMI_POLICY_PACK_ATTACH); every other pack — and every
// other Host method — defers to the base host. Container execution lives at
// the Host seam, not in the engine core, so other plugin kinds can later be
// launched the same way.
type containerHost struct {
	plugin.Host

	// bootMu serializes analyzer boots, mirroring the base host's serialized
	// plugin loads. The tracking map has its own lock so cancellation and
	// teardown never wait behind an in-flight boot.
	bootMu sync.Mutex

	mu        sync.Mutex
	analyzers map[containerAnalyzerKey]*containerAnalyzerEntry
}

type containerAnalyzerKey struct {
	name tokens.QName
	path string
	opts string
}

type containerAnalyzerEntry struct {
	analyzer plugin.Analyzer
	refs     map[*plugin.Context]struct{}
}

// NewContainerHost wraps base with container-image policy pack support. The
// wrapper is a no-op for deployments that reference no container packs. Every
// host whose PolicyAnalyzer can receive policy packs must be wrapped;
// NewPolicyAnalyzer rejects container and attach packs that reach an
// unwrapped host.
func NewContainerHost(base plugin.Host) plugin.Host {
	return &containerHost{
		Host:      base,
		analyzers: map[containerAnalyzerKey]*containerAnalyzerEntry{},
	}
}

func (h *containerHost) PolicyAnalyzer(
	ctx *plugin.Context, name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions,
) (plugin.Analyzer, error) {
	boot, err := h.dispatch(name, path, opts)
	if err != nil {
		return nil, err
	}
	if boot == nil {
		return h.Host.PolicyAnalyzer(ctx, name, path, opts)
	}

	h.bootMu.Lock()
	defer h.bootMu.Unlock()

	// The options are part of the cache key: they configure the analyzer
	// (stack, configuration, environment), so a cached analyzer may only be
	// reused for a call that would boot an identical one. fmt prints maps with
	// sorted keys, making the representation deterministic.
	optsKey := ""
	if opts != nil {
		optsKey = fmt.Sprintf("%v", *opts)
	}
	key := containerAnalyzerKey{name: name, path: path, opts: optsKey}

	// Refs are keyed by the lifetime context, as the base host keys the
	// plugins it boots, so a derived context (WithoutProviderDebugging)
	// releases against the same key it booted under.
	refCtx := ctx.LifetimeContext()

	h.mu.Lock()
	if entry, has := h.analyzers[key]; has {
		entry.refs[refCtx] = struct{}{}
		h.mu.Unlock()
		return entry.analyzer, nil
	}
	h.mu.Unlock()

	analyzer, err := boot(ctx)
	if err != nil {
		return nil, err
	}

	h.mu.Lock()
	h.analyzers[key] = &containerAnalyzerEntry{
		analyzer: analyzer,
		refs:     map[*plugin.Context]struct{}{refCtx: {}},
	}
	h.mu.Unlock()
	return analyzer, nil
}

// dispatch decides whether the pack is one this host boots, returning a nil
// boot function for packs that belong to the base host.
func (h *containerHost) dispatch(
	name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions,
) (func(*plugin.Context) (plugin.Analyzer, error), error) {
	// Attach mode: the pack is already running (a pod sidecar, or under a
	// debugger); connect instead of launching.
	if port, err := plugin.GetPolicyPackAttachPort(name); err != nil {
		return nil, err
	} else if port != nil {
		p := *port
		return func(ctx *plugin.Context) (plugin.Analyzer, error) {
			return plugin.AttachPolicyAnalyzer(h, ctx, name, p, opts)
		}, nil
	}

	// A digest-pinned image ref (server-enforced pack): boot the container
	// directly; there is no local pack directory or manifest.
	if opts != nil && opts.ImageRef != "" {
		return func(ctx *plugin.Context) (plugin.Analyzer, error) {
			return plugin.NewContainerPolicyAnalyzer(h, ctx, name, opts.ImageRef, "", "", opts)
		}, nil
	}

	// A local pack directory: intercept only when its manifest declares
	// runtime "oci". Manifest load failures defer to the base host, which
	// reports them canonically.
	if path == "" {
		return nil, nil
	}
	proj, err := workspace.LoadPolicyPack(filepath.Join(path, "PulumiPolicy.yaml"))
	if err != nil || proj.Runtime.Name() != "oci" {
		return nil, nil
	}
	ref, err := localImageRef(proj, path)
	if err != nil {
		return nil, err
	}
	var description string
	if proj.Description != nil {
		description = *proj.Description
	}
	return func(ctx *plugin.Context) (plugin.Analyzer, error) {
		return plugin.NewContainerPolicyAnalyzer(h, ctx, name, ref, proj.Version, description, opts)
	}, nil
}

// localImageRef resolves the image to run for a local `--policy-pack ./dir`
// pack. The image must have been built locally (the CLI never builds or
// implicitly pulls for local packs).
func localImageRef(proj *workspace.PolicyPackProject, path string) (string, error) {
	image, _ := proj.Runtime.Options()["image"].(string)
	if image == "" {
		return "", fmt.Errorf("policy pack at %q has runtime \"oci\" but no \"image\" runtime option; "+
			"set runtime.options.image in PulumiPolicy.yaml to the pack's registry image", path)
	}
	ref, _, err := oci.ResolveRef(image, proj.Version, "")
	return ref, err
}

func (h *containerHost) ReleaseContext(ctx *plugin.Context) error {
	refCtx := ctx.LifetimeContext()

	h.mu.Lock()
	var toClose []plugin.Analyzer
	for key, entry := range h.analyzers {
		delete(entry.refs, refCtx)
		if len(entry.refs) == 0 {
			toClose = append(toClose, entry.analyzer)
			delete(h.analyzers, key)
		}
	}
	h.mu.Unlock()

	for _, a := range toClose {
		contract.IgnoreClose(a)
	}
	return h.Host.ReleaseContext(ctx)
}

func (h *containerHost) SignalCancellation() error {
	h.mu.Lock()
	analyzers := make([]plugin.Analyzer, 0, len(h.analyzers))
	for _, entry := range h.analyzers {
		analyzers = append(analyzers, entry.analyzer)
	}
	h.mu.Unlock()

	cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, a := range analyzers {
		if err := a.Cancel(cancelCtx); err != nil {
			slog.InfoContext(cancelCtx, "Error cancelling container policy analyzer; ignoring", "err", err)
		}
	}
	return h.Host.SignalCancellation()
}

func (h *containerHost) Close() error {
	h.mu.Lock()
	toClose := make([]plugin.Analyzer, 0, len(h.analyzers))
	for _, entry := range h.analyzers {
		toClose = append(toClose, entry.analyzer)
	}
	clear(h.analyzers)
	h.mu.Unlock()

	for _, a := range toClose {
		contract.IgnoreClose(a)
	}
	return h.Host.Close()
}
