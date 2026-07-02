// Copyright 2025, Pulumi Corporation.
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

// Package packageresolution provides functionality for resolving package sources
// when the source location is unknown beforehand. This is used throughout the
// CLI to determine where to fetch packages from (registry, local paths, or external sources).
//
// This differs from [registry.ResolvePackageFromName] which specifically queries
// the Pulumi registry. This package determines the resolution strategy first,
// then may delegate to registry functions, local file operations, or external
// source handling as appropriate.
package packageresolution

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/util"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var (
	ErrPackageNotFound = errors.New("package not found")
	ErrRegistryQuery   = errors.New("registry query error")
)

type PackageNotFoundError struct {
	Package     string
	Version     string
	OriginalErr error
}

func (e PackageNotFoundError) Error() string {
	if e.Version != "" {
		return fmt.Sprintf("package %s@%s not found", e.Package, e.Version)
	}
	return fmt.Sprintf("package %s not found", e.Package)
}

func (e PackageNotFoundError) Is(target error) bool {
	return target == ErrPackageNotFound
}

func (e PackageNotFoundError) Unwrap() error {
	return e.OriginalErr
}

func (e PackageNotFoundError) Suggestions() []apitype.PackageMetadata {
	return registry.GetSuggestedPackages(e)
}

type Options struct {
	// If [Resolve] should use the passed in registry to resolve packages.
	ResolveWithRegistry bool

	// If the resolution should use already installed plugins when resolving
	// plugins without specific versions provided.
	//
	// Concretely, when resolving a package like aws, if a version of aws is
	// already on disk, then that version will be preferred over latest.
	ResolveVersionWithLocalWorkspace bool

	// Resolve the source directly against the local workspace if possible.
	//
	// For example, consider a spec like workspace.PackageSpec{Source:
	// "example"}. With "AllowNonInvertableLocalWorkspaceResolution: false", resolving
	// "example" may resolve to a parameterized package with a base plugin called
	// "base". [Resolve] wouldn't consider a local plugin called "example", and might
	// fail if "base" couldn't be installed.
	//
	// "AllowNonInvertableLocalWorkspaceResolution: true" instructs [Resolve] to use a
	// package called "example" if example is requested and present in the workspace,
	// regardless of external lookup.
	AllowNonInvertableLocalWorkspaceResolution bool
}

// The result of running [Resolve].
//
// Resolution will be one of 3 types:
//
// - [PackageResolution]: The spec was resolved to a specific Pulumi package.
//
// - [PluginResolution]: The spec was resolved to a specific Pulumi plugin, but
// parameterization makes resolving to a full package impossible.
//
// - [PathResolution]: The spec was resolved to a local plugin path on disk.
type Resolution interface {
	isResolution()
}

func (PackageResolution) isResolution() {}
func (PluginResolution) isResolution()  {}
func (PathResolution) isResolution()    {}

type (
	// A fully resolved package.
	PackageResolution struct {
		Spec                 workspace.PackageSpec
		Pkg                  workspace.PackageDescriptor
		InstalledInWorkspace bool // If package is already installed in the global workplace.
	}
	// A fully resolved plugin with not yet resolved parameterization.
	//
	// For example, this would be the result of [Resolve]ing:
	//
	//	workspace.PackageSpec{
	//		Source: "terraform-provider",
	//		Parameters: []string{"org/example"},
	//	}
	//
	// We wouldn't know the name or version of the package (example@<latest>), but we
	// would know the name and version of the resolved plugin
	// (terraform-provider@<latest>).
	PluginResolution struct {
		Spec                 workspace.PackageSpec
		Pkg                  workspace.UnresolvedPackageDescriptor
		InstalledInWorkspace bool
	}
	// A local path based plugin.
	PathResolution struct {
		Spec workspace.PackageSpec
		// The path to the plugin on disk.
		Path                 string
		ParameterizationArgs []string
	}
)

func naivePackageDescriptor(
	ctx context.Context, spec workspace.PackageSpec,
) (workspace.UnresolvedPackageDescriptor, error) {
	pluginSpecSource := spec.Source
	var version *semver.Version
	if spec.Version != "" {
		if v, err := semver.ParseTolerant(spec.Version); err != nil {
			pluginSpecSource += "@" + spec.Version
		} else {
			version = &v
		}
	}
	pluginDesc, err := workspace.NewPluginDescriptor(ctx, pluginSpecSource, apitype.ResourcePlugin,
		version, spec.PluginDownloadURL, spec.Checksums)
	return workspace.UnresolvedPackageDescriptor{
		PluginDescriptor:     pluginDesc,
		ParameterizationArgs: spec.Parameters,
	}, err
}

func naiveResolution(
	spec workspace.PackageSpec, desc workspace.UnresolvedPackageDescriptor, installed bool,
) Resolution {
	if desc.IsGitPlugin() && spec.PluginDownloadURL == "" {
		spec.Source = strings.TrimPrefix(desc.PluginDownloadURL, "git://")
		if v := desc.Version; v != nil && v.Major == 0 && v.Minor == 0 && v.Patch == 0 &&
			len(v.Build) == 0 && len(v.Pre) == 1 && !v.Pre[0].IsNum &&
			strings.HasPrefix(v.Pre[0].VersionStr, "x") {
			spec.Version = v.Pre[0].VersionStr[1:]
		}
	}

	// If there is no parameters in the spec, then we have fully resolved here.
	if len(spec.Parameters) == 0 {
		return PackageResolution{
			Spec: spec,
			Pkg: workspace.PackageDescriptor{
				PluginDescriptor: desc.PluginDescriptor,
			},
			InstalledInWorkspace: installed,
		}
	}

	// Otherwise we at least have the plugin.
	return PluginResolution{
		Spec:                 spec,
		Pkg:                  desc,
		InstalledInWorkspace: installed,
	}
}

func registryResolution(
	spec workspace.PackageSpec, metadata apitype.PackageMetadata, installed bool,
) (Resolution, error) {
	spec = workspace.PackageSpec{
		Source:     path.Join(metadata.Source, metadata.Publisher, metadata.Name),
		Version:    metadata.Version.String(),
		Parameters: spec.Parameters,
		Checksums:  spec.Checksums,
	}

	if len(spec.Parameters) > 0 && metadata.Parameterization != nil {
		return nil, fmt.Errorf(
			"unable to resolve package: resolved plugin to %s, which is already parameterized",
			spec.Source,
		)
	}

	pluginDescriptor := workspace.PluginDescriptor{
		Name:              metadata.Name,
		Kind:              apitype.ResourcePlugin,
		Version:           &metadata.Version,
		PluginDownloadURL: metadata.PluginDownloadURL,
		Checksums:         spec.Checksums,
	}

	if len(spec.Parameters) > 0 {
		plugin := PluginResolution{
			Spec: spec,
			Pkg: workspace.UnresolvedPackageDescriptor{
				PluginDescriptor:     pluginDescriptor,
				ParameterizationArgs: spec.Parameters,
			},
			InstalledInWorkspace: installed,
		}
		logging.V(3).Infof("Resolved package %q via the registry to plugin %#v (installedInWorkspace=%t)\n",
			spec.Source, plugin, installed)

		return plugin, nil
	}

	pkgDescriptor := workspace.PackageDescriptor{
		PluginDescriptor: pluginDescriptor,
	}

	if metadata.Parameterization != nil {
		pkgDescriptor.Parameterization = &workspace.Parameterization{
			Name:    metadata.Name,
			Version: metadata.Version,
			Value:   metadata.Parameterization.Parameter,
		}
		pkgDescriptor.Name = metadata.Parameterization.BaseProvider.Name
		pkgDescriptor.Version = &metadata.Parameterization.BaseProvider.Version
	}

	pkg := PackageResolution{
		Spec:                 spec,
		Pkg:                  pkgDescriptor,
		InstalledInWorkspace: installed,
	}
	logging.V(3).Infof("Resolved package %q via the registry to package %#v (installedInWorkspace=%t)\n",
		spec.Source, pkg, installed)
	return pkg, nil
}

func IsPluginInstalled(
	ctx context.Context,
	plugin workspace.PluginDescriptor, ws pluginstorage.Context, options Options,
) (bool, *semver.Version, error) {
	if ws.HasPlugin(ctx, plugin) {
		return true, plugin.Version, nil
	}

	if plugin.Version == nil && options.ResolveVersionWithLocalWorkspace {
		has, version, err := ws.HasPluginGTE(ctx, plugin)
		if err != nil || has {
			return has, version, err
		}
	}
	return false, nil, nil
}

func Resolve(
	ctx context.Context,
	reg registry.Registry,
	ws pluginstorage.Context,
	spec workspace.PackageSpec,
	options Options,
) (Resolution, error) {
	logging.V(3).Infof("Resolving package from %#v\n", spec)
	if plugin.IsLocalPluginPath(ctx, spec.Source) {
		return PathResolution{
			Path:                 spec.Source,
			ParameterizationArgs: spec.Parameters,
			Spec:                 spec,
		}, nil
	}

	naivePackageDescriptor, err := naivePackageDescriptor(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("unable to parse spec: %w%s", err, vcsURLHint(spec.Source))
	}

	if options.AllowNonInvertableLocalWorkspaceResolution {
		installed, atVersion, err := IsPluginInstalled(ctx, naivePackageDescriptor.PluginDescriptor, ws, options)
		if err != nil {
			return nil, err
		}
		if installed {
			if atVersion != nil {
				spec.Version = atVersion.String()
				naivePackageDescriptor.Version = atVersion
			}
			return naiveResolution(spec, naivePackageDescriptor, true), nil
		}
	}

	remoteResolution := func() (Resolution, error) {
		logging.V(3).Infof("Resolved package %#v to an external source %#v\n",
			spec, naivePackageDescriptor)
		installed, atVersion, err := IsPluginInstalled(ctx, naivePackageDescriptor.PluginDescriptor, ws, options)
		if err != nil {
			return nil, err
		}
		if installed {
			if atVersion != nil {
				naivePackageDescriptor.Version = atVersion
				spec.Version = atVersion.String()
			}
			return naiveResolution(spec, naivePackageDescriptor, true), nil
		}

		// We still don't have a version, so let's look up the latest version.
		if naivePackageDescriptor.Version == nil {
			v, err := ws.GetLatestVersion(ctx, naivePackageDescriptor.PluginDescriptor)
			if err != nil && !errors.Is(err, workspace.ErrGetLatestVersionNotSupported) {
				return nil, fmt.Errorf("unable to get the latest version of %q: %w",
					naivePackageDescriptor.Name, err)
			}
			if v != nil {
				naivePackageDescriptor.Version = v
				spec.Version = v.String()
			}
		}

		// At this point, we either have a version or aren't going to have one
		return naiveResolution(spec, naivePackageDescriptor, false), nil
	}

	if workspace.IsExternalURL(spec.Source) || naivePackageDescriptor.IsGitPlugin() {
		return remoteResolution()
	}

	var registryNotFoundErr error
	var registryQueryErr error

	if options.ResolveWithRegistry {
		metadata, err := registry.ResolvePackageFromName(ctx, reg, spec.Source, naivePackageDescriptor.Version)
		if err == nil {
			pluginDescriptor := workspace.PluginDescriptor{
				Name:              metadata.Name,
				Kind:              apitype.ResourcePlugin,
				Version:           &metadata.Version,
				PluginDownloadURL: metadata.PluginDownloadURL,
				Checksums:         spec.Checksums,
			}

			// Now that we've resolved to a version, we need to check if we have a good-enough version
			// already installed.

			// If the version was specified in the request, then the only good-enough version is the correct version
			if naivePackageDescriptor.Version != nil {
				return registryResolution(spec, metadata, ws.HasPlugin(ctx, pluginDescriptor))
			}

			// If the version wasn't specified in the request, then good enough is any plugin with the same
			// *major version* as what the registry gave us.
			has, version, err := ws.HasPluginGTE(ctx, func(s workspace.PluginDescriptor) workspace.PluginDescriptor {
				s.Version = &semver.Version{Major: s.Version.Major}
				return s
			}(pluginDescriptor))
			if err != nil {
				return nil, err
			}

			// There is no local version of this plugin that meets our version requirements, so we just
			// request the latest.
			if !has || version == nil {
				return registryResolution(spec, metadata, has)
			}

			// We have a version that's already installed at the right major version, so we should use
			// that... if it's valid in the registry. We need to check.
			newMetadata, err := registry.ResolvePackageFromName(
				ctx, reg, path.Join(metadata.Source, metadata.Publisher, metadata.Name), version)
			if errors.Is(err, registry.ErrNotFound) {
				// The version we have isn't in the registry so it doesn't
				// count. Use the latest version from the registry.
				return registryResolution(spec, metadata, false)
			}
			if err != nil {
				return nil, err
			}
			spec.Version = version.String()
			return registryResolution(spec, newMetadata, true)
		}
		if errors.Is(err, registry.ErrNotFound) {
			registryNotFoundErr = err
		} else {
			registryQueryErr = fmt.Errorf("%w: %w", ErrRegistryQuery, err)
		}
	}

	// If this used to work (like "aws") or if the user has specified a
	// pluginDownloadURL themselves, then pass it through.
	if registry.IsPreRegistryPackage(spec.Source) || spec.PluginDownloadURL != "" ||
		util.SetKnownPluginDownloadURL(&naivePackageDescriptor.PluginDescriptor) {
		return remoteResolution()
	}

	if registryQueryErr != nil {
		logging.V(3).Infof("Failed to resolve package %#v\n", spec)
		return nil, registryQueryErr
	}

	logging.V(3).Infof("Failed to resolve package %#v\n", spec)
	return nil, &PackageNotFoundError{
		Package:     spec.Source,
		Version:     spec.Version,
		OriginalErr: registryNotFoundErr,
	}
}

// vcsURLHint returns a suffix suggesting a repo-boundary marker when source
// looks like an ambiguous VCS URL, or "" otherwise. GitHub, GitLab, and
// Bitbucket use a ".git" suffix on the repo name; Azure DevOps uses a
// "_git/" segment before the repo name.
func vcsURLHint(source string) string {
	u, err := url.Parse(source)
	if err != nil || u == nil {
		return ""
	}
	p := strings.Trim(u.Path, "/")
	if p == "" {
		return ""
	}
	host := strings.TrimPrefix(u.Host, "www.")
	switch host {
	case gitutil.GitHubHostName, gitutil.GitLabHostName, gitutil.BitbucketHostName:
		if strings.HasSuffix(p, ".git") || strings.Contains(p, ".git/") {
			return ""
		}
		if strings.Count(p, "/") < 2 {
			return ""
		}
		return fmt.Sprintf(
			"\n\nhint: URL %q has more than two path segments without a .git marker, "+
				"so the repo boundary is ambiguous. Add .git after the repo name "+
				"(e.g. %q for a nested repo, or owner/repo.git/subdir for a subdirectory).",
			source, source+".git",
		)
	case gitutil.AzureDevOpsHostName:
		if strings.Contains(p, "/_git/") {
			return ""
		}
		if strings.Count(p, "/") < 2 {
			return ""
		}
		return fmt.Sprintf(
			"\n\nhint: Azure DevOps URL %q is missing the \"_git\" segment. "+
				"Use the form https://dev.azure.com/{organization}/{project}/_git/{repository}.",
			source,
		)
	default:
		return ""
	}
}
