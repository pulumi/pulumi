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

package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	preparePluginLog        = 7
	preparePluginVerboseLog = 8
)

// PluginSet represents a set of plugins.
type PluginSet map[string]workspace.PluginSpec

// NewPluginSet creates a new PluginSet from the specified PluginSpecs.
func NewPluginSet(plugins ...workspace.PluginSpec) PluginSet {
	var s PluginSet = make(map[string]workspace.PluginSpec, len(plugins))
	for _, p := range plugins {
		s.Add(p)
	}
	return s
}

// Add adds a plugin to this plugin set.
func (p PluginSet) Add(plug workspace.PluginSpec) {
	p[plug.String()] = plug
}

// Removes less-specific entries.
//
// For example, the plugin aws would be removed if there was an already existing plugin aws-5.4.0.
func (p PluginSet) Deduplicate() PluginSet {
	existing := map[string]workspace.PluginSpec{}
	newSet := NewPluginSet()
	add := func(p workspace.PluginSpec) {
		prev, ok := existing[p.Name]
		if ok {
			// If either `pluginDownloadURL`, `Version` or both are set we consider the
			// plugin fully specified and keep it. It is ok to keep `pkg v1.2.3` `pkg
			// v2.3.4` and `pkg example.com` in a single set. What we don't want to do is
			// keep `pkg` in that same set since there are more specific versions used. In
			// general, there will be a `pky vX.Y.Y` in the plugin set because the user
			// depended on a language package `pulumi-pkg` with version `x.y.z`.

			if p.Version == nil && p.PluginDownloadURL == "" {
				// no new information
				return
			}

			if prev.Version == nil && prev.PluginDownloadURL == "" {
				// New plugin is more specific then the old one
				delete(newSet, prev.String())
			}
		}

		newSet.Add(p)
		existing[p.Name] = p
	}
	for _, value := range p {
		add(value)
	}
	return newSet
}

// Values returns a slice of all of the plugins contained within this set.
func (p PluginSet) Values() []workspace.PluginSpec {
	plugins := slice.Prealloc[workspace.PluginSpec](len(p))
	for _, value := range p {
		plugins = append(plugins, value)
	}
	return plugins
}

// PackageSet represents a set of packages.
type PackageSet map[string]workspace.PackageDescriptor

// NewPackageSet creates a new PackageSet from the specified PackageDescriptors.
func NewPackageSet(pkgs ...workspace.PackageDescriptor) PackageSet {
	var s PackageSet = make(map[string]workspace.PackageDescriptor, len(pkgs))
	for _, p := range pkgs {
		s.Add(p)
	}
	return s
}

// Add adds a package to this package set.
func (p PackageSet) Add(pkg workspace.PackageDescriptor) {
	p[pkg.String()] = pkg
}

// Union returns the union of this PackageSet with another PackageSet.
func (p PackageSet) Union(other PackageSet) PackageSet {
	newSet := NewPackageSet()
	for _, value := range p {
		newSet.Add(value)
	}
	for _, value := range other {
		newSet.Add(value)
	}
	return newSet
}

// ToPluginSet converts this PackageSet to a PluginSet by discarding all parameterization information.
func (p PackageSet) ToPluginSet() PluginSet {
	newSet := NewPluginSet()
	for _, value := range p {
		newSet.Add(value.PluginSpec)
	}
	return newSet
}

// Values returns a slice of all of the packages contained within this set.
func (p PackageSet) Values() []workspace.PackageDescriptor {
	pkgs := slice.Prealloc[workspace.PackageDescriptor](len(p))
	for _, value := range p {
		pkgs = append(pkgs, value)
	}
	return pkgs
}

// A PackageUpdate represents an update from one version of a package to another.
type PackageUpdate struct {
	// The old package version.
	Old workspace.PackageDescriptor
	// The new package version.
	New workspace.PackageDescriptor
}

// UpdatesTo returns a list of PackageUpdates that represent the updates to the argument PackageSet present in this
// PackageSet. For instance, if the argument contains a package P at version 3, and this PackageSet contains the same
// package P (as identified by name and kind) at version 5, this method will return an update where the Old field
// contains the version 3 instance from the argument and the New field contains the version 5 instance from this
// PackageSet. This also considers parameterization information, so a parameterized package P at version 3 will be
// considered different from a parameterized package P at version 5 even if the base plugin is the same.
func (p PackageSet) UpdatesTo(old PackageSet) []PackageUpdate {
	var updates []PackageUpdate
	for _, value := range p {
		for _, otherValue := range old {
			// This is comparing _package_ names. i.e. the plugin name if parameterization is nil, or the parameterization
			// name if it's present. This means that, if we see a package `aws v1.2.3`, and a parameterized package `aws
			// v1.2.4 (base: terraform-provider)`, say, we _will_ consider the latter an update of the former, since there can
			// only really be one instance of a package name in a Pulumi program.

			name := value.PackageName()
			otherName := otherValue.PackageName()

			namesAndKindsEqual := name == otherName && value.Kind == otherValue.Kind

			if namesAndKindsEqual {
				version := value.PackageVersion()
				otherVersion := otherValue.PackageVersion()

				// If both versions have been explicitly specified, we can compare them. If one is missing, we don't have enough
				// information to work out if one is a later version of the other.
				if version != nil && otherVersion != nil && version.GT(*otherVersion) {
					updates = append(updates, PackageUpdate{Old: otherValue, New: value})
				}
			}
		}
	}

	return updates
}

// GetRequiredPlugins lists a full set of plugins that will be required by the given program.
func GetRequiredPlugins(
	host plugin.Host,
	runtime string,
	info plugin.ProgramInfo,
) ([]workspace.PluginSpec, error) {
	plugins := make([]workspace.PluginSpec, 0, 1)

	// First make sure the language plugin is present.  We need this to load the required resource plugins.
	// TODO: we need to think about how best to version this.  For now, it always picks the latest.
	lang, err := host.LanguageRuntime(runtime, info)
	if lang == nil || err != nil {
		return nil, fmt.Errorf("failed to load language plugin %s: %w", runtime, err)
	}
	// Query the language runtime plugin for its version.
	langInfo, err := lang.GetPluginInfo()
	if err != nil {
		// Don't error if this fails, just warn and return the version as unknown.
		host.Log(diag.Warning, "", fmt.Sprintf("failed to get plugin info for language plugin %s: %v", runtime, err), 0)
		plugins = append(plugins, workspace.PluginSpec{
			Name: runtime,
			Kind: apitype.LanguagePlugin,
		})
	} else {
		plugins = append(plugins, workspace.PluginSpec{
			Name:    langInfo.Name,
			Kind:    langInfo.Kind,
			Version: langInfo.Version,
		})
	}

	// Use the language plugin to compute this project's set of plugin dependencies.
	// TODO: we want to support loading precisely what the project needs, rather than doing a static scan of resolved
	//     packages.  Doing this requires that we change our RPC interface and figure out how to configure plugins
	//     later than we do (right now, we do it up front, but at that point we don't know the version).
	deps, err := lang.GetRequiredPackages(info)
	if err != nil {
		return nil, fmt.Errorf("failed to discover plugin requirements: %w", err)
	}
	for _, dep := range deps {
		plugins = append(plugins, dep.PluginSpec)
	}

	var newPlugins []workspace.PluginSpec
	for _, plugin := range plugins {
		if strings.HasPrefix(plugin.PluginDownloadURL, "git://") {
			// This plugin is downloaded from a git source, so we need to rename it.
			url := strings.TrimPrefix(plugin.PluginDownloadURL, "git://")
			plugin.Name = strings.ReplaceAll(url, "/", "_")
		}
		newPlugins = append(newPlugins, plugin)
	}
	return newPlugins, nil
}

// gatherPackagesFromProgram inspects the given program and returns the set of packages that the program requires to
// function. If the language host does not support this operation, the empty set is returned.
func gatherPackagesFromProgram(plugctx *plugin.Context, runtime string, info plugin.ProgramInfo) (PackageSet, error) {
	logging.V(preparePluginLog).Infof("gatherPackagesFromProgram(): gathering plugins from language host")

	lang, err := plugctx.Host.LanguageRuntime(runtime, info)
	if lang == nil || err != nil {
		return nil, fmt.Errorf("failed to load language plugin %s: %w", runtime, err)
	}

	pkgs, err := lang.GetRequiredPackages(info)
	if err != nil {
		return nil, fmt.Errorf("failed to discover package requirements: %w", err)
	}

	set := NewPackageSet()
	for _, pkg := range pkgs {
		logging.V(preparePluginLog).Infof(
			"gatherPackagesFromProgram(): package %s (%s) is required by language host",
			pkg.String(), pkg.PluginDownloadURL)
		set.Add(pkg)
	}
	return set, nil
}

// gatherPackagesFromSnapshot inspects the snapshot associated with the given Target and returns the set of packages
// required to operate on the snapshot. The set of packages is derived from first-class providers saved in the snapshot
// and the plugins specified in the deployment manifest.
func gatherPackagesFromSnapshot(plugctx *plugin.Context, target *deploy.Target) (PackageSet, error) {
	logging.V(preparePluginLog).Infof("gatherPackagesFromSnapshot(): gathering plugins from snapshot")
	set := NewPackageSet()
	if target == nil || target.Snapshot == nil {
		logging.V(preparePluginLog).Infof("gatherPackagesFromSnapshot(): no snapshot available, skipping")
		return set, nil
	}
	for _, res := range target.Snapshot.Resources {
		urn := res.URN
		if !providers.IsProviderType(urn.Type()) {
			logging.V(preparePluginVerboseLog).Infof(
				"gatherPackagesFromSnapshot(): skipping %q, not a provider", urn)
			continue
		}
		pkg := providers.GetProviderPackage(urn.Type())

		name, err := providers.GetProviderName(pkg, res.Inputs)
		if err != nil {
			return set, err
		}
		version, err := providers.GetProviderVersion(res.Inputs)
		if err != nil {
			return set, err
		}
		downloadURL, err := providers.GetProviderDownloadURL(res.Inputs)
		if err != nil {
			return set, err
		}
		checksums, err := providers.GetProviderChecksums(res.Inputs)
		if err != nil {
			return set, err
		}
		parameterization, err := providers.GetProviderParameterization(pkg, res.Inputs)
		if err != nil {
			return set, err
		}
		var packageParameterization *workspace.Parameterization
		if parameterization != nil {
			packageParameterization = &workspace.Parameterization{
				Name:    string(parameterization.Name),
				Version: parameterization.Version,
				Value:   parameterization.Value,
			}
		}

		logging.V(preparePluginLog).Infof(
			"gatherPackagesFromSnapshot(): package %s %s is required by first-class provider %q", name, version, urn)
		set.Add(workspace.PackageDescriptor{
			PluginSpec: workspace.PluginSpec{
				Name:              name.String(),
				Kind:              apitype.ResourcePlugin,
				Version:           version,
				PluginDownloadURL: downloadURL,
				Checksums:         checksums,
			},
			Parameterization: packageParameterization,
		})
	}
	return set, nil
}

// EnsurePluginsAreInstalled inspects all plugins in the plugin set and, if any plugins are not currently installed,
// uses the given backend client to install them. Installations are processed in parallel, though
// ensurePluginsAreInstalled does not return until all installations are completed.
func EnsurePluginsAreInstalled(ctx context.Context, opts *deploymentOptions, d diag.Sink, plugins PluginSet,
	projectPlugins []workspace.ProjectPlugin, reinstall, explicitInstall bool,
) error {
	logging.V(preparePluginLog).Infof("ensurePluginsAreInstalled(): beginning")
	var installTasks errgroup.Group
	for _, plug := range plugins.Values() {
		if plug.Name == "pulumi" && plug.Kind == apitype.ResourcePlugin {
			logging.V(preparePluginLog).Infof("ensurePluginsAreInstalled(): pulumi is a builtin plugin")
			continue
		}

		path, err := workspace.GetPluginPath(d, plug.Kind, plug.Name, plug.Version, projectPlugins)
		if err == nil && path != "" {
			logging.V(preparePluginLog).Infof(
				"ensurePluginsAreInstalled(): plugin %s %s already installed", plug.Name, plug.Version)

			if !reinstall {
				continue
			}
		}

		if !reinstall {
			// If the plugin already exists, don't download it unless `reinstall` was specified.
			label := fmt.Sprintf("%s plugin %s", plug.Kind, plug)
			if plug.Version != nil {
				if workspace.HasPlugin(plug) {
					logging.V(1).Infof("%s skipping install (existing == match)", label)
					continue
				}
			} else {
				if has, _ := workspace.HasPluginGTE(plug); has {
					logging.V(1).Infof("%s skipping install (existing >= match)", label)
					continue
				}
			}
		}

		if workspace.IsPluginBundled(plug.Kind, plug.Name) {
			return fmt.Errorf(
				"the %v %v plugin is bundled with Pulumi, and cannot be directly installed."+
					" Reinstall Pulumi via your package manager or install script",
				plug.Name,
				plug.Kind,
			)
		}

		info := plug // don't close over the loop induction variable

		// If DISABLE_AUTOMATIC_PLUGIN_ACQUISITION is set just add an error to the error group and continue.
		if !explicitInstall && env.DisableAutomaticPluginAcquisition.Value() {
			installTasks.Go(func() error {
				return fmt.Errorf("plugin %s %s not installed", info.Name, info.Version)
			})
			continue
		}

		// Launch an install task asynchronously and add it to the current error group.
		installTasks.Go(func() error {
			logging.V(preparePluginLog).Infof(
				"EnsurePluginsAreInstalled(): plugin %s %s not installed, doing install", info.Name, info.Version)
			return installPlugin(ctx, opts, info)
		})
	}

	err := installTasks.Wait()
	logging.V(preparePluginLog).Infof("EnsurePluginsAreInstalled(): completed")
	return err
}

// ensurePluginsAreLoaded ensures that all of the plugins in the given plugin set that match the given plugin flags are
// loaded.
func ensurePluginsAreLoaded(plugctx *plugin.Context, plugins PluginSet, kinds plugin.Flags) error {
	return plugctx.Host.EnsurePlugins(plugins.Values(), kinds)
}

// installPlugin installs a plugin from the given backend client.
func installPlugin(
	ctx context.Context,
	opts *deploymentOptions,
	plugin workspace.PluginSpec,
) error {
	logging.V(preparePluginLog).Infof("installPlugin(%s, %s): beginning install", plugin.Name, plugin.Version)

	// If we don't have a version yet try and call GetLatestVersion to fill it in
	if plugin.Version == nil {
		logging.V(preparePluginVerboseLog).Infof(
			"installPlugin(%s): version not specified, trying to lookup latest version", plugin.Name)

		version, err := plugin.GetLatestVersion(ctx)
		if err != nil {
			return fmt.Errorf("could not get latest version for plugin %s: %w", plugin.Name, err)
		}
		plugin.Version = version
	}

	logging.V(preparePluginVerboseLog).Infof(
		"installPlugin(%s, %s): initiating download", plugin.Name, plugin.Version)

	pluginID := fmt.Sprintf("%s-%s", plugin.Name, plugin.Version)
	downloadMessage := "Downloading plugin " + pluginID

	// We want to report download progress so that users are not left wondering if
	// their program has hung. To do this we wrap the downloading ReadCloser with
	// one that observes the bytes read and renders a progress bar in some
	// fashion. If we have an event emitter available, we'll use that to report
	// program by publishing progress events. If not, we'll wrap with a ReadCloser
	// that renders progress directly to the console itself.
	var withDownloadProgress func(io.ReadCloser, int64) io.ReadCloser
	if opts == nil {
		withDownloadProgress = func(stream io.ReadCloser, size int64) io.ReadCloser {
			return workspace.ReadCloserProgressBar(
				stream,
				size,
				downloadMessage,
				cmdutil.GetGlobalColorization(),
			)
		}
	} else {
		withDownloadProgress = func(stream io.ReadCloser, size int64) io.ReadCloser {
			return NewProgressReportingCloser(
				opts.Events,
				PluginDownload,
				string(PluginDownload)+":"+pluginID,
				downloadMessage,
				size,
				100*time.Millisecond, /*reportingInterval */
				stream,
			)
		}
	}
	retry := func(err error, attempt int, limit int, delay time.Duration) {
		logging.V(preparePluginVerboseLog).Infof(
			"Error downloading plugin: %s\nWill retry in %v [%d/%d]", err, delay, attempt, limit)
	}

	tarball, err := workspace.DownloadToFile(ctx, plugin, withDownloadProgress, retry)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %s: %w", plugin, err)
	}
	defer func() { contract.IgnoreError(os.Remove(tarball.Name())) }()

	logging.V(preparePluginVerboseLog).Infof(
		"installPlugin(%s, %s): extracting tarball to installation directory", plugin.Name, plugin.Version)

	// In a similar manner to downloads, we'll use a progress bar to show install
	// progress by wrapping the download stream with a progress reporting
	// ReadCloser where possible.
	var withInstallProgress func(io.ReadCloser) io.ReadCloser
	stat, err := tarball.Stat()
	if opts == nil || err != nil {
		withInstallProgress = func(stream io.ReadCloser) io.ReadCloser {
			return stream
		}

		installing := fmt.Sprintf("[%s plugin %s] installing", plugin.Kind, pluginID)
		fmt.Fprintln(os.Stderr, installing)
	} else {
		withInstallProgress = func(stream io.ReadCloser) io.ReadCloser {
			return NewProgressReportingCloser(
				opts.Events,
				PluginInstall,
				string(PluginInstall)+":"+pluginID,
				"Installing plugin "+pluginID,
				stat.Size(),
				100*time.Millisecond, /*reportingInterval */
				tarball,
			)
		}
	}

	if err := plugin.InstallWithContext(
		ctx,
		workspace.TarPlugin(withInstallProgress(tarball)),
		false,
	); err != nil {
		return fmt.Errorf("installing plugin; run `pulumi plugin install %s %s v%s` to retry manually: %w",
			plugin.Kind, plugin.Name, plugin.Version, err)
	}

	logging.V(7).Infof("installPlugin(%s, %s): installation complete", plugin.Name, plugin.Version)
	return nil
}

// computeDefaultProviderPackages computes, for every package, a mapping from packages to semver versions reflecting the
// version of a provider that should be used as the "default" resource when registering resources. This function takes
// two sets of packages:
//
// - a set given to us from the language host; and
// - the full set of packages.
//
// If the language host has sent us a non-empty set of packages, we will use those exclusively to service default
// provider requests. Otherwise, we will use the full set of packages, which is the existing behavior today.
//
// The justification for favoring the language host is that, ultimately, it is the language host that produces resource
// registrations and therefore it is the language host that should dictate exactly what package to use to satisfy a
// resource registration. SDKs have the opportunity to specify what plugin (pluginDownloadURL and version) they want to
// use in RegisterResource. If the plugin is left unspecified, we make a best-guess effort to infer the version and URL
// that the language host actually wants.
//
// Whenever a resource arrives via RegisterResource and does not explicitly specify which provider to use, the engine
// injects a "default" provider resource that will serve as that resource's provider. This function computes the map
// that the engine uses to determine which version of a particular provider to load.
//
// Note: it is critical that this function be 100% deterministic.
func computeDefaultProviderPackages(
	languagePackages PackageSet,
	allPackages PackageSet,
) map[tokens.Package]workspace.PackageDescriptor {
	// Language hosts are not required to specify the full set of plugins they depend on. If the set of plugins received
	// from the language host does not include any resource providers, fall back to the full set of plugins.
	languageReportedProviderPlugins := false
	for _, plug := range languagePackages.Values() {
		if plug.Kind == apitype.ResourcePlugin {
			languageReportedProviderPlugins = true
		}
	}

	sourceSet := languagePackages
	if !languageReportedProviderPlugins {
		logging.V(preparePluginLog).Infoln(
			"computeDefaultProviderPlugins(): language host reported empty set of provider plugins, using all plugins")
		sourceSet = allPackages
	}

	defaultProviderPlugins := make(map[tokens.Package]workspace.PackageDescriptor)

	// Sort the set of source plugins by version, so that we iterate over the set of plugins in a deterministic order.
	// Sorting by version gets us two properties:
	//   1. The below loop will never see a nil-versioned plugin after a non-nil versioned plugin, since the sort always
	//      considers nil-versioned plugins to be less than non-nil versioned plugins.
	//   2. The below loop will never see a plugin with a version that is older than a plugin that has already been
	//      seen. The sort will always have placed the older plugin before the newer plugin.
	//
	// Despite these properties, the below loop explicitly handles those cases to preserve correct behavior even if the
	// sort is not functioning properly.
	sourcePackages := sourceSet.Values()
	slices.SortFunc(sourcePackages, workspace.SortPackageDescriptors)
	for _, p := range sourcePackages {
		logging.V(preparePluginLog).Infof("computeDefaultProviderPlugins(): considering %s", p)
		if p.Kind != apitype.ResourcePlugin {
			// Default providers are only relevant for resource plugins.
			logging.V(preparePluginVerboseLog).Infof(
				"computeDefaultProviderPlugins(): skipping %s, not a resource provider", p)
			continue
		}

		name := tokens.Package(p.PackageName())

		if seenPlugin, has := defaultProviderPlugins[name]; has {
			if seenPlugin.Version == nil {
				logging.V(preparePluginLog).Infof(
					"computeDefaultProviderPlugins(): plugin %s selected for package %s (override, previous was nil)",
					p, p.Name)
				defaultProviderPlugins[name] = p
				continue
			}

			contract.Assertf(p.Version != nil, "p.Version should not be nil if sorting is correct!")
			if p.Version != nil && p.Version.GTE(*seenPlugin.Version) {
				logging.V(preparePluginLog).Infof(
					"computeDefaultProviderPlugins(): plugin %s selected for package %s (override, newer than previous %s)",
					p, p.Name, seenPlugin.Version)
				defaultProviderPlugins[name] = p
				continue
			}

			contract.Failf("Should not have seen an older plugin if sorting is correct!\n  %s-%s\n  %s-%s",
				p.Name, p.Version.String(),
				seenPlugin.Name, seenPlugin.Version.String())
		}

		logging.V(preparePluginLog).Infof(
			"computeDefaultProviderPlugins(): plugin %s selected for package %s (first seen)", p, p.Name)
		defaultProviderPlugins[name] = p
	}

	if logging.V(preparePluginLog) {
		logging.V(preparePluginLog).Infoln("computeDefaultProviderPlugins(): summary of default plugins:")
		for pkg, info := range defaultProviderPlugins {
			logging.V(preparePluginLog).Infof("  %-15s = %s", pkg, info.Version)
		}
	}

	defaultProviderInfo := make(map[tokens.Package]workspace.PackageDescriptor)
	for name, plugin := range defaultProviderPlugins {
		defaultProviderInfo[name] = plugin
	}

	return defaultProviderInfo
}
