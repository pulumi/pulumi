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
	"fmt"
	"sync"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Session represents an MCP session with its own provider registry and state.
type Session struct {
	ctx         context.Context
	registry    *providers.Registry
	host        *mcpPluginHost
	providerIDs map[string]providers.Reference // MCP Provider ID -> Registry Reference
	schemaCache map[string]*schema.PackageSpec  // MCP Provider ID -> Schema
	idCounter   int
	mu          sync.RWMutex
}

// NewSession creates a new MCP session.
func NewSession(
	ctx context.Context,
	logCallback func(sev diag.Severity, urn resource.URN, msg string, streamID int32),
	logStatusCallback func(sev diag.Severity, urn resource.URN, msg string, streamID int32),
) (*Session, error) {
	// Create the plugin host
	host, err := newMCPPluginHost(ctx, logCallback, logStatusCallback)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin host: %w", err)
	}

	// Create the provider registry
	registry := providers.NewRegistry(host, false, nil) // false = not a preview

	return &Session{
		ctx:         ctx,
		registry:    registry,
		host:        host,
		providerIDs: make(map[string]providers.Reference),
		schemaCache: make(map[string]*schema.PackageSpec),
		idCounter:   0,
	}, nil
}

// AddProvider configures and adds a new provider to the session.
// Returns the provider ID that should be used for subsequent operations.
func (s *Session) AddProvider(
	id string,
	pkg string,
	version string,
	config map[string]any,
) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate MCP provider ID if not provided
	if id == "" {
		s.idCounter++
		id = fmt.Sprintf("%s-%d", pkg, s.idCounter)
	}

	// Check if ID already exists
	if _, ok := s.providerIDs[id]; ok {
		return "", fmt.Errorf("provider ID %q already exists", id)
	}

	// Parse version if provided
	var ver *semver.Version
	if version != "" {
		v, err := semver.Parse(version)
		if err != nil {
			return "", fmt.Errorf("invalid version %q: %w", version, err)
		}
		ver = &v
	}

	// Convert config to PropertyMap
	configProps, err := JSONToPropertyMap(config)
	if err != nil {
		return "", fmt.Errorf("failed to convert config: %w", err)
	}

	// Set version in the config if provided
	if ver != nil {
		providers.SetProviderVersion(configProps, ver)
	}

	// Create a provider URN using the MCP provider ID
	providerURN := resource.NewURN(
		tokens.QName("mcp-stack"),
		tokens.PackageName("mcp-project"),
		"",
		providers.MakeProviderType(tokens.Package(pkg)),
		id,
	)

	// Use the registry's Check method to load and validate the provider
	// This will load the provider and store it with UnconfiguredID
	checkResp, err := s.registry.Check(s.ctx, plugin.CheckRequest{
		URN:  providerURN,
		Olds: resource.PropertyMap{},
		News: configProps,
	})
	if err != nil {
		return "", fmt.Errorf("failed to check provider config: %w", err)
	}
	if len(checkResp.Failures) > 0 {
		return "", fmt.Errorf("config validation failed: %s", checkResp.Failures[0].Reason)
	}

	// Use the registry's Create method to configure the provider
	// This will configure it and store it with an actual ID
	createResp, err := s.registry.Create(s.ctx, plugin.CreateRequest{
		URN:        providerURN,
		Properties: checkResp.Properties,
		Preview:    false,
	})
	if err != nil {
		return "", fmt.Errorf("failed to configure provider: %w", err)
	}

	// Create a reference to the configured provider
	ref, err := providers.NewReference(providerURN, createResp.ID)
	if err != nil {
		return "", fmt.Errorf("failed to create provider reference: %w", err)
	}

	// Store the MCP provider ID -> Registry reference mapping
	s.providerIDs[id] = ref

	return id, nil
}

// GetProvider retrieves a provider by its MCP ID.
func (s *Session) GetProvider(id string) (plugin.Provider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ref, ok := s.providerIDs[id]
	if !ok {
		return nil, fmt.Errorf("provider ID %q not found", id)
	}

	// Get the provider from the registry using the reference
	provider, ok := s.registry.GetProvider(ref)
	if !ok {
		return nil, fmt.Errorf("provider for ID %q not found in registry", id)
	}

	return provider, nil
}

// GetSchema retrieves the schema for a provider, using the cache if available.
func (s *Session) GetSchema(id string) (*schema.PackageSpec, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check cache first
	if cached, ok := s.schemaCache[id]; ok {
		return cached, nil
	}

	// Get the provider reference
	ref, ok := s.providerIDs[id]
	if !ok {
		return nil, fmt.Errorf("provider ID %q not found", id)
	}

	// Get the provider from the registry
	provider, ok := s.registry.GetProvider(ref)
	if !ok {
		return nil, fmt.Errorf("provider for ID %q not found in registry", id)
	}

	// Get the schema from the provider
	schemaResp, err := provider.GetSchema(s.ctx, plugin.GetSchemaRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	// Parse the schema
	var spec schema.PackageSpec
	if schemaResp.Schema != nil {
		specPtr, err := ParsePackageSpec(schemaResp.Schema)
		if err != nil {
			return nil, fmt.Errorf("failed to parse schema: %w", err)
		}
		spec = *specPtr
	}

	// Cache the schema
	s.schemaCache[id] = &spec

	return &spec, nil
}

// Close cleans up all resources associated with the session.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear provider IDs
	s.providerIDs = make(map[string]providers.Reference)
	s.schemaCache = make(map[string]*schema.PackageSpec)

	// Close the host (which will close all providers)
	return s.host.Close()
}
