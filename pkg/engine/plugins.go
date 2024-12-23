// Copyright 2016-2019, Pulumi Corporation.
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
	"sort"
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

// Add adds a plugin to this plugin set.
func (p PluginSet) Add(plug workspace.PluginSpec) {
	p[plug.String()] = plug
}

// Union returns the union of this pluginSet with another pluginSet.
func (p PluginSet) Union(other PluginSet) PluginSet {
	newSet := NewPluginSet()
	for _, value := range p {
		newSet.Add(value)
	}
	for _, value := range other {
		newSet.Add(value)
	}
	return newSet
}

// Removes less specific entries.
//
// For example, the plugin aws would be removed if there was an already existing plugin
// aws-5.4.0.
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

// A PluginUpdate represents an update from one version of a plugin to another.
type PluginUpdate struct {
	// The old plugin version.
	Old workspace.PluginSpec
	// The new plugin version.
	New workspace.PluginSpec
}

// UpdatesTo returns a list of PluginUpdates that represent the updates to the argument PluginSet present in this
// PluginSet. For instance, if the argument contains a plugin P at version 3, and this PluginSet contains the same
// plugin P (as identified by name and kind) at version 5, this method will return an update where the Old field
// contains the version 3 instance from the argument and the New field contains the version 5 instance from this
// PluginSet.
func (p PluginSet) UpdatesTo(old PluginSet) []PluginUpdate {
	var updates []PluginUpdate
	for _, value := range p {
		for _, otherValue := range old {
			if value.Name == otherValue.Name && value.Kind == otherValue.Kind {
				if value.Version != nil && otherValue.Version != nil && value.Version.GT(*otherValue.Version) {
					updates = append(updates, PluginUpdate{Old: otherValue, New: value})
				}
			}
		}
	}

	return updates
}

// Values returns a slice of all of the plugins contained within this set.
func (p PluginSet) Values() []workspace.PluginSpec {
	plugins := slice.Prealloc[workspace.PluginSpec](len(p))
	for _, value := range p {
		plugins = append(plugins, value)
	}
	return plugins
}

// NewPluginSet creates a new empty pluginSet.
func NewPluginSet(plugins ...workspace.PluginSpec) PluginSet {
	var s PluginSet = make(map[string]workspace.PluginSpec, len(plugins))
	for _, p := range plugins {
		s.Add(p)
	}
	return s
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

	return plugins, nil
}

// gatherPluginsFromProgram inspects the given program and returns the set of plugins that the program requires to
// function. If the language host does not support this operation, the empty set is returned.
func gatherPluginsFromProgram(plugctx *plugin.Context, runtime string, prog plugin.ProgramInfo) (PluginSet, error) {
	logging.V(preparePluginLog).Infof("gatherPluginsFromProgram(): gathering plugins from language host")
	set := NewPluginSet()

	langhostPlugins, err := GetRequiredPlugins(plugctx.Host, runtime, prog)
	if err != nil {
		return set, err
	}
	for _, plug := range langhostPlugins {
		// Ignore language plugins
		if plug.Kind == apitype.LanguagePlugin {
			continue
		}

		logging.V(preparePluginLog).Infof(
			"gatherPluginsFromProgram(): plugin %s %s (%s) is required by language host",
			plug.Name, plug.Version, plug.PluginDownloadURL)
		set.Add(plug)
	}
	return set, nil
}

// gatherPluginsFromSnapshot inspects the snapshot associated with the given Target and returns the set of plugins
// required to operate on the snapshot. The set of plugins is derived from first-class providers saved in the snapshot
// and the plugins specified in the deployment manifest.
func gatherPluginsFromSnapshot(plugctx *plugin.Context, target *deploy.Target) (PluginSet, error) {
	logging.V(preparePluginLog).Infof("gatherPluginsFromSnapshot(): gathering plugins from snapshot")
	set := NewPluginSet()
	if target == nil || target.Snapshot == nil {
		logging.V(preparePluginLog).Infof("gatherPluginsFromSnapshot(): no snapshot available, skipping")
		return set, nil
	}
	for _, res := range target.Snapshot.Resources {
		urn := res.URN
		if !providers.IsProviderType(urn.Type()) {
			logging.V(preparePluginVerboseLog).Infof(
				"gatherPluginsFromSnapshot(): skipping %q, not a provider", urn)
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

		logging.V(preparePluginLog).Infof(
			"gatherPluginsFromSnapshot(): plugin %s %s is required by first-class provider %q", name, version, urn)
		set.Add(workspace.PluginSpec{
			Name:              name.String(),
			Kind:              apitype.ResourcePlugin,
			Version:           version,
			PluginDownloadURL: downloadURL,
			Checksums:         checksums,
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

// computeDefaultProviderPlugins computes, for every resource plugin, a mapping from packages to semver versions
// reflecting the version of a provider that should be used as the "default" resource when registering resources. This
// function takes two sets of plugins: a set of plugins given to us from the language host and the full set of plugins.
// If the language host has sent us a non-empty set of plugins, we will use those exclusively to service default
// provider requests. Otherwise, we will use the full set of plugins, which is the existing behavior today.
//
// The justification for favoring language plugins over all else is that, ultimately, it is the language plugin that
// produces resource registrations and therefore it is the language plugin that should dictate exactly what plugins to
// use to satisfy a resource registration. SDKs have the opportunity to specify what plugin (pluginDownloadURL and
// version) they want to use in RegisterResource. If the plugin is left unspecified, we make a best guess effort to
// infer the version and url that the language plugin actually wants.
//
// Whenever a resource arrives via RegisterResource and does not explicitly specify which provider to use, the engine
// injects a "default" provider resource that will serve as that resource's provider. This function computes the map
// that the engine uses to determine which version of a particular provider to load.
//
// it is critical that this function be 100% deterministic.
func computeDefaultProviderPlugins(languagePlugins, allPlugins PluginSet) map[tokens.Package]workspace.PluginSpec {
	// Language hosts are not required to specify the full set of plugins they depend on. If the set of plugins received
	// from the language host does not include any resource providers, fall back to the full set of plugins.
	languageReportedProviderPlugins := false
	for _, plug := range languagePlugins.Values() {
		if plug.Kind == apitype.ResourcePlugin {
			languageReportedProviderPlugins = true
		}
	}

	sourceSet := languagePlugins
	if !languageReportedProviderPlugins {
		logging.V(preparePluginLog).Infoln(
			"computeDefaultProviderPlugins(): language host reported empty set of provider plugins, using all plugins")
		sourceSet = allPlugins
	}

	defaultProviderPlugins := make(map[tokens.Package]workspace.PluginSpec)

	// Sort the set of source plugins by version, so that we iterate over the set of plugins in a deterministic order.
	// Sorting by version gets us two properties:
	//   1. The below loop will never see a nil-versioned plugin after a non-nil versioned plugin, since the sort always
	//      considers nil-versioned plugins to be less than non-nil versioned plugins.
	//   2. The below loop will never see a plugin with a version that is older than a plugin that has already been
	//      seen. The sort will always have placed the older plugin before the newer plugin.
	//
	// Despite these properties, the below loop explicitly handles those cases to preserve correct behavior even if the
	// sort is not functioning properly.
	sourcePlugins := sourceSet.Values()
	sort.Sort(workspace.SortedPluginSpec(sourcePlugins))
	for _, p := range sourcePlugins {
		logging.V(preparePluginLog).Infof("computeDefaultProviderPlugins(): considering %s", p)
		if p.Kind != apitype.ResourcePlugin {
			// Default providers are only relevant for resource plugins.
			logging.V(preparePluginVerboseLog).Infof(
				"computeDefaultProviderPlugins(): skipping %s, not a resource provider", p)
			continue
		}

		if seenPlugin, has := defaultProviderPlugins[tokens.Package(p.Name)]; has {
			if seenPlugin.Version == nil {
				logging.V(preparePluginLog).Infof(
					"computeDefaultProviderPlugins(): plugin %s selected for package %s (override, previous was nil)",
					p, p.Name)
				defaultProviderPlugins[tokens.Package(p.Name)] = p
				continue
			}

			contract.Assertf(p.Version != nil, "p.Version should not be nil if sorting is correct!")
			if p.Version != nil && p.Version.GTE(*seenPlugin.Version) {
				logging.V(preparePluginLog).Infof(
					"computeDefaultProviderPlugins(): plugin %s selected for package %s (override, newer than previous %s)",
					p, p.Name, seenPlugin.Version)
				defaultProviderPlugins[tokens.Package(p.Name)] = p
				continue
			}

			contract.Failf("Should not have seen an older plugin if sorting is correct!\n  %s-%s\n  %s-%s",
				p.Name, p.Version.String(),
				seenPlugin.Name, seenPlugin.Version.String())
		}

		logging.V(preparePluginLog).Infof(
			"computeDefaultProviderPlugins(): plugin %s selected for package %s (first seen)", p, p.Name)
		defaultProviderPlugins[tokens.Package(p.Name)] = p
	}

	if logging.V(preparePluginLog) {
		logging.V(preparePluginLog).Infoln("computeDefaultProviderPlugins(): summary of default plugins:")
		for pkg, info := range defaultProviderPlugins {
			logging.V(preparePluginLog).Infof("  %-15s = %s", pkg, info.Version)
		}
	}

	defaultProviderInfo := make(map[tokens.Package]workspace.PluginSpec)
	for name, plugin := range defaultProviderPlugins {
		defaultProviderInfo[name] = plugin
	}

	return defaultProviderInfo
}
