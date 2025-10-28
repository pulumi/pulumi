// Copyright 2016-2024, Pulumi Corporation.
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

package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// mcpPluginHost implements the plugin.Host interface for the MCP server.
// It provides a minimal implementation focused on provider loading and diagnostic routing.
type mcpPluginHost struct {
	ctx               context.Context
	pluginCtx         *plugin.Context
	providers         map[plugin.Provider]bool
	logCallback       func(sev diag.Severity, urn resource.URN, msg string, streamID int32)
	logStatusCallback func(sev diag.Severity, urn resource.URN, msg string, streamID int32)
	m                 sync.RWMutex
}

// newMCPPluginHost creates a new MCP plugin host.
func newMCPPluginHost(
	ctx context.Context,
	logCallback func(sev diag.Severity, urn resource.URN, msg string, streamID int32),
	logStatusCallback func(sev diag.Severity, urn resource.URN, msg string, streamID int32),
) (*mcpPluginHost, error) {
	host := &mcpPluginHost{
		ctx:               ctx,
		providers:         make(map[plugin.Provider]bool),
		logCallback:       logCallback,
		logStatusCallback: logStatusCallback,
	}

	// Create a sink that routes to our callbacks
	sink := &mcpDiagSink{
		logCallback: logCallback,
	}

	// Create a plugin context that can load providers
	pluginCtx, err := plugin.NewContext(ctx, sink, sink, host, nil, "", nil, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin context: %w", err)
	}

	host.pluginCtx = pluginCtx

	return host, nil
}

// mcpDiagSink implements diag.Sink to route diagnostics to MCP callbacks.
type mcpDiagSink struct {
	logCallback func(sev diag.Severity, urn resource.URN, msg string, streamID int32)
}

func (s *mcpDiagSink) Logf(sev diag.Severity, d *diag.Diag, args ...any) {
	if s.logCallback != nil {
		msg := fmt.Sprintf(d.Message, args...)
		s.logCallback(sev, "", msg, 0)
	}
}

func (s *mcpDiagSink) Debugf(d *diag.Diag, args ...any) {
	s.Logf(diag.Debug, d, args...)
}

func (s *mcpDiagSink) Infof(d *diag.Diag, args ...any) {
	s.Logf(diag.Info, d, args...)
}

func (s *mcpDiagSink) Infoerrf(d *diag.Diag, args ...any) {
	s.Logf(diag.Infoerr, d, args...)
}

func (s *mcpDiagSink) Errorf(d *diag.Diag, args ...any) {
	s.Logf(diag.Error, d, args...)
}

func (s *mcpDiagSink) Warningf(d *diag.Diag, args ...any) {
	s.Logf(diag.Warning, d, args...)
}

func (s *mcpDiagSink) Stringify(sev diag.Severity, d *diag.Diag, args ...any) (string, string) {
	msg := fmt.Sprintf(d.Message, args...)
	return "", msg
}

// ServerAddr returns the address at which the host's RPC interface may be found.
func (h *mcpPluginHost) ServerAddr() string {
	// Return empty string - providers don't need to callback to us
	// If a provider requires a callback server, this would need to be implemented
	return ""
}

// Log logs a message to the MCP client.
func (h *mcpPluginHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	if h.logCallback != nil {
		h.logCallback(sev, urn, msg, streamID)
	}
}

// LogStatus logs a status message to the MCP client.
func (h *mcpPluginHost) LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	if h.logStatusCallback != nil {
		h.logStatusCallback(sev, urn, msg, streamID)
	}
}

// Analyzer is not supported in this minimal implementation.
func (h *mcpPluginHost) Analyzer(nm tokens.QName) (plugin.Analyzer, error) {
	return nil, errors.New("analyzers not supported")
}

// PolicyAnalyzer is not supported in this minimal implementation.
func (h *mcpPluginHost) PolicyAnalyzer(name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
	return nil, errors.New("policy analyzers not supported")
}

// ListAnalyzers returns an empty list.
func (h *mcpPluginHost) ListAnalyzers() []plugin.Analyzer {
	return nil
}

// Provider loads a provider plugin using the plugin context.
func (h *mcpPluginHost) Provider(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
	h.m.Lock()
	defer h.m.Unlock()

	// Use the plugin context's host to load the provider
	provider, err := h.pluginCtx.Host.Provider(descriptor)
	if err != nil {
		return nil, fmt.Errorf("failed to load provider %q: %w", descriptor.Name, err)
	}

	// Track the provider
	h.providers[provider] = true

	return provider, nil
}

// CloseProvider closes a provider plugin.
func (h *mcpPluginHost) CloseProvider(provider plugin.Provider) error {
	h.m.Lock()
	defer h.m.Unlock()

	delete(h.providers, provider)
	return provider.Close()
}

// LanguageRuntime is not supported in this minimal implementation.
func (h *mcpPluginHost) LanguageRuntime(runtime string, info plugin.ProgramInfo) (plugin.LanguageRuntime, error) {
	return nil, errors.New("language runtimes not supported")
}

// EnsurePlugins is not needed in this implementation.
func (h *mcpPluginHost) EnsurePlugins(plugins []workspace.PluginSpec, kinds plugin.Flags) error {
	// No-op - plugins are loaded on-demand
	return nil
}

// ResolvePlugin resolves a plugin spec.
func (h *mcpPluginHost) ResolvePlugin(spec workspace.PluginSpec) (*workspace.PluginInfo, error) {
	// Delegate to the plugin context's host
	if h.pluginCtx != nil && h.pluginCtx.Host != nil {
		return h.pluginCtx.Host.ResolvePlugin(spec)
	}
	return nil, errors.New("plugin context not available")
}

// GetProjectPlugins returns an empty list.
func (h *mcpPluginHost) GetProjectPlugins() []workspace.ProjectPlugin {
	return nil
}

// SignalCancellation signals all providers to cancel.
func (h *mcpPluginHost) SignalCancellation() error {
	h.m.RLock()
	defer h.m.RUnlock()

	var errs error
	for provider := range h.providers {
		if err := provider.SignalCancellation(h.ctx); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

// StartDebugging is not supported.
func (h *mcpPluginHost) StartDebugging(info plugin.DebuggingInfo) error {
	return errors.New("debugging not supported")
}

// AttachDebugger always returns false.
func (h *mcpPluginHost) AttachDebugger(spec plugin.DebugSpec) bool {
	return false
}

// Close closes all providers and cleans up resources.
func (h *mcpPluginHost) Close() error {
	h.m.Lock()
	defer h.m.Unlock()

	var errs error
	for provider := range h.providers {
		if err := provider.Close(); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	h.providers = make(map[plugin.Provider]bool)

	return errs
}
