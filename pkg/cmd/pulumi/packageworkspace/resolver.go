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
	"fmt"
	"os"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/backend/diy/unauthenticatedregistry"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// NewResolverServerFromContext constructs the package resolver service bound to pctx's workspace
// view. It has the signature required by [plugin.NewResolverFunc]. The resolver runs a spec
// through the standard package installation machinery, so the dependency it returns is the same
// one the CLI would resolve and install.
func NewResolverServerFromContext(pctx *plugin.Context) pulumirpc.PackageResolverServer {
	return &resolverServer{pctx: pctx}
}

type resolverServer struct {
	pulumirpc.UnimplementedPackageResolverServer
	pctx *plugin.Context
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

	reg := unauthenticatedregistry.New(s.pctx.Diag, env.Global())
	installCtx := New(pluginstorage.Instance, pkgWorkspace.Instance, s.pctx.Host, os.Stderr, os.Stderr, //nolint:forbidigo
		nil, Options{})

	_, resolved, _, err := packageinstallation.InstallPlugin(ctx, spec, nil, "", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveWithRegistry:                        true,
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
		},
	}, reg, installCtx)
	if err != nil {
		return nil, fmt.Errorf("resolving package %q: %w", spec.Source, err)
	}

	return packageSpecToDependency(ctx, resolved)
}

// packageSpecToDependency converts a resolved package spec into the concrete package dependency it
// names. The spec is parsed into a plugin descriptor with the same source handling the rest of the
// workspace uses, so registry coordinates, git URLs, and plain names all resolve consistently.
func packageSpecToDependency(
	ctx context.Context, spec workspace.PackageSpec,
) (*pulumirpc.PackageDependency, error) {
	source := spec.Source
	var version *semver.Version
	if spec.Version != "" {
		if v, err := semver.ParseTolerant(spec.Version); err != nil {
			// Not a semver (e.g. a git hash); fold it back into the source for parsing.
			source += "@" + spec.Version
		} else {
			version = &v
		}
	}

	desc, err := workspace.NewPluginDescriptor(ctx, source, apitype.ResourcePlugin,
		version, spec.PluginDownloadURL, spec.Checksums)
	if err != nil {
		return nil, fmt.Errorf("building descriptor for %q: %w", spec.Source, err)
	}

	versionStr := ""
	if desc.Version != nil {
		versionStr = desc.Version.String()
	}
	return &pulumirpc.PackageDependency{
		Name:      desc.Name,
		Kind:      string(desc.Kind),
		Version:   versionStr,
		Server:    desc.PluginDownloadURL,
		Checksums: desc.Checksums,
	}, nil
}
