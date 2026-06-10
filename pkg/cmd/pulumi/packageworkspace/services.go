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

package packageworkspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/blang/semver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	pconvert "github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

// NewMapperServerFromHost builds a provider mapper served from a [plugin.Host]. It mirrors the mapper the engine
// serves to language runtimes during a run, but is constructed from a host so that it can be served on the host's own
// gRPC server and offered to providers during [plugin.Provider] Handshake.
//
// It is a [plugin.NewMapperFunc].
func NewMapperServerFromHost(host plugin.Host) codegenrpc.MapperServer {
	return pconvert.NewMapperServer(&hostMapper{host: host})
}

// hostMapper defers construction of the underlying base mapper until the first GetMapping call, since
// [pconvert.NewBasePluginMapper] can fail and a [plugin.NewMapperFunc] cannot return an error.
type hostMapper struct {
	host  plugin.Host
	once  sync.Once
	inner pconvert.Mapper
	err   error
}

func (m *hostMapper) GetMapping(
	ctx context.Context, provider string, hint *pconvert.MapperPackageHint,
) ([]byte, error) {
	m.once.Do(func() {
		log := func(sev diag.Severity, msg string) {
			m.host.Log(sev, "", msg, 0)
		}
		installPlugin := func(pluginName string) *semver.Version {
			if env.DisableAutomaticPluginAcquisition.Value() {
				return nil
			}
			version, err := pkgWorkspace.InstallPlugin(ctx, workspace.PluginDescriptor{
				Name: pluginName,
				Kind: apitype.ResourcePlugin,
			}, log, schema.NewLoaderServerFromHost)
			if err != nil {
				log(diag.Warning, fmt.Sprintf("failed to install provider %q: %v", pluginName, err))
				return nil
			}
			return version
		}
		base, err := pconvert.NewBasePluginMapper(
			pluginstorage.Instance,
			"terraform",
			pconvert.ProviderFactoryFromHost(ctx, m.host),
			installPlugin,
			nil, /*mappings*/
		)
		if err != nil {
			m.err = err
			return
		}
		m.inner = pconvert.NewCachingMapper(base)
	})
	if m.err != nil {
		return nil, m.err
	}
	return m.inner.GetMapping(ctx, provider, hint)
}

// NewPackageResolver returns a [plugin.NewPackageResolverFunc] that serves package resolution backed by reg. The
// returned func builds a [pulumirpc.PackageResolverServer] from a [plugin.Host], which providers may use during
// Handshake to resolve loosely-specified package references (e.g. `hashicorp/aws ~>6.0`, or a Terraform module
// source) into fully-resolved descriptors with any parameterization value baked in.
func NewPackageResolver(reg registry.Registry) plugin.NewPackageResolverFunc {
	return func(host plugin.Host) pulumirpc.PackageResolverServer {
		return &packageResolverServer{registry: reg, host: host}
	}
}

type packageResolverServer struct {
	pulumirpc.UnimplementedPackageResolverServer
	registry registry.Registry
	host     plugin.Host
}

func (s *packageResolverServer) ResolvePackage(
	ctx context.Context, req *pulumirpc.ResolvePackageRequest,
) (*pulumirpc.ResolvePackageResponse, error) {
	protoSpec := req.GetSpec()
	if protoSpec == nil {
		return nil, status.Error(codes.InvalidArgument, "spec must not be nil")
	}

	spec := workspace.PackageSpec{
		Source:            protoSpec.Source,
		Version:           protoSpec.Version,
		Parameters:        protoSpec.Parameters,
		Checksums:         protoSpec.Checksums,
		PluginDownloadURL: protoSpec.Server,
	}

	ws := New(pluginstorage.Instance, pkgWorkspace.Instance, s.host, io.Discard, io.Discard, nil, Options{})

	runPackage, _, _, err := packageinstallation.InstallPlugin(ctx, spec,
		nil, "", packageinstallation.Options{
			Options: packageresolution.Options{
				ResolveWithRegistry:              !env.DisableRegistryResolve.Value(),
				ResolveVersionWithLocalWorkspace: true,
			},
		}, s.registry, ws)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve %q: %w", spec, err)
	}

	// The parameterization value of a package is produced by the (parameterized) provider and surfaced in its
	// schema, so we run the resolved package and read the descriptor out of the schema it emits. This works
	// uniformly for plain packages, registry-parameterized packages, and argument-parameterized packages.
	wd, err := os.MkdirTemp("", "pulumi-package-resolve-")
	if err != nil {
		return nil, fmt.Errorf("creating working directory to resolve %q: %w", spec, err)
	}
	defer func() { contract.IgnoreError(os.RemoveAll(wd)) }()

	provider, err := runPackage(ctx, wd)
	if err != nil {
		return nil, fmt.Errorf("unable to run %q: %w", spec, err)
	}
	defer contract.IgnoreClose(provider)

	schemaResp, err := provider.GetSchema(ctx, plugin.GetSchemaRequest{})
	if err != nil {
		return nil, fmt.Errorf("unable to get schema for %q: %w", spec, err)
	}

	dep, err := packageDependencyFromSchema(schemaResp.Schema)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve descriptor for %q: %w", spec, err)
	}
	return &pulumirpc.ResolvePackageResponse{Package: dep}, nil
}

// packageDependencyFromSchema builds the resolved [pulumirpc.PackageDependency] from the JSON schema emitted by a
// resolved (and, if applicable, parameterized) provider. For a parameterized package the dependency points at the
// base provider and carries the parameterization (name, version, and the baked value); for a plain package it points
// at the package itself.
func packageDependencyFromSchema(schemaJSON []byte) (*pulumirpc.PackageDependency, error) {
	var s struct {
		Name              string `json:"name"`
		Version           string `json:"version"`
		PluginDownloadURL string `json:"pluginDownloadURL"`
		Parameterization  *struct {
			BaseProvider struct {
				Name              string `json:"name"`
				Version           string `json:"version"`
				PluginDownloadURL string `json:"pluginDownloadURL"`
			} `json:"baseProvider"`
			Parameter []byte `json:"parameter"`
		} `json:"parameterization"`
	}
	if err := json.Unmarshal(schemaJSON, &s); err != nil {
		return nil, fmt.Errorf("parsing package schema: %w", err)
	}

	dep := &pulumirpc.PackageDependency{Kind: string(apitype.ResourcePlugin)}
	if s.Parameterization != nil {
		dep.Name = s.Parameterization.BaseProvider.Name
		dep.Version = s.Parameterization.BaseProvider.Version
		dep.Server = s.Parameterization.BaseProvider.PluginDownloadURL
		dep.Parameterization = &pulumirpc.PackageParameterization{
			Name:    s.Name,
			Version: s.Version,
			Value:   s.Parameterization.Parameter,
		}
	} else {
		dep.Name = s.Name
		dep.Version = s.Version
		dep.Server = s.PluginDownloadURL
	}
	return dep, nil
}
