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
	"os"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// NewResolverServer returns a [plugin.NewResolverFunc] that builds a package resolver service bound
// to a context's workspace view. The resolver runs a spec through the standard package installation
// machinery, so the dependency it returns is the same one the CLI would resolve and install. The
// registry is supplied by the caller rather than constructed here, so that a logged-in user resolves
// against their authenticated registry.
func NewResolverServer(reg registry.Registry) plugin.NewResolverFunc {
	contract.Requiref(reg != nil, "reg", "must not be nil")
	return func(pctx *plugin.Context) pulumirpc.PackageResolverServer {
		w := New(pluginstorage.Instance, pkgWorkspace.Instance, pctx, os.Stderr, os.Stderr, //nolint:forbidigo
			nil, Options{})
		return &resolverServer{reg: reg, w: w, m: new(sync.Mutex)}
	}
}

type resolverServer struct {
	pulumirpc.UnimplementedPackageResolverServer
	reg registry.Registry
	w   Workspace

	m     *sync.Mutex
	state packageinstallation.State
}

func (s *resolverServer) ResolvePackage(
	ctx context.Context, req *pulumirpc.PackageSpec,
) (*pulumirpc.PackageDependency, error) {
	spec := workspace.PackageSpec{
		Source:            req.Source,
		Version:           req.Version,
		Parameters:        req.Parameters,
		Checksums:         req.Checksums,
		PluginDownloadURL: req.Server,
	}

	s.m.Lock()
	run, resolved, state, err := packageinstallation.InstallPlugin(ctx, spec, nil, "", packageinstallation.Options{
		PriorState: s.state,
		Options: packageresolution.Options{
			ResolveWithRegistry:                        true,
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
		},
	}, s.reg, s.w)
	if err != nil {
		s.m.Unlock()
		return nil, fmt.Errorf("resolving package %q: %w", spec.Source, err)
	}
	s.state = state
	s.m.Unlock()

	prov, err := run(ctx, s.w.pctx.Pwd)
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(prov)

	resp, err := prov.GetSchema(ctx, plugin.GetSchemaRequest{})
	if err != nil {
		return nil, fmt.Errorf("getting schema: %w", err)
	}

	var pkg schema.PartialPackageSpec
	if err := json.Unmarshal(resp.Schema, &pkg); err != nil {
		return nil, fmt.Errorf("unmarshaling schema: %w", err)
	}

	result := pulumirpc.PackageDependency{
		Name:      pkg.Name,
		Kind:      string(apitype.ResourcePlugin),
		Version:   pkg.Version,
		Server:    pkg.PluginDownloadURL,
		Checksums: resolved.Checksums,
	}

	if pkg.Parameterization != nil {
		result.Name = pkg.Parameterization.BaseProvider.Name
		result.Version = pkg.Parameterization.BaseProvider.Version
		result.Parameterization = &pulumirpc.PackageParameterization{
			Name:    pkg.Name,
			Version: pkg.Version,
			Value:   pkg.Parameterization.Parameter,
		}
	}

	return &result, nil
}
