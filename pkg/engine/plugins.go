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

// pluginSet represents a set of plugins.
type pluginSet map[string]workspace.PluginSpec

// Add adds a plugin to this plugin set.
func (p pluginSet) Add(plug workspace.PluginSpec) {
	p[plug.String()] = plug
}

// Union returns the union of this pluginSet with another pluginSet.
func (p pluginSet) Union(other pluginSet) pluginSet {
	newSet := newPluginSet()
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
func (p pluginSet) Deduplicate() pluginSet {
	existing := map[string]workspace.PluginSpec{}
	newSet := newPluginSet()
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
func (p pluginSet) Values() []workspace.PluginSpec {
	plugins := slice.Prealloc[workspace.PluginSpec](len(p))
	for _, value := range p {
		plugins = append(plugins, value)
	}
	return plugins
}

// newPluginSet creates a new empty pluginSet.
func newPluginSet(plugins ...workspace.PluginSpec) pluginSet {
	var s pluginSet = make(map[string]workspace.PluginSpec, len(plugins))
	for _, p := range plugins {
		s.Add(p)
	}
	return s
}

// gatherPluginsFromProgram inspects the given program and returns the set of plugins that the program requires to
// function. If the language host does not support this operation, the empty set is returned.
func gatherPluginsFromProgram(plugctx *plugin.Context, prog plugin.ProgInfo) (pluginSet, error) {
	logging.V(preparePluginLog).Infof("gatherPluginsFromProgram(): gathering plugins from language host")
	set := newPluginSet()
	langhostPlugins, err := plugin.GetRequiredPlugins(plugctx.Host, plugctx.Root, prog, plugin.AllPlugins)
	if err != nil {
		return set, err
	}
	for _, plug := range langhostPlugins {
		// Ignore language plugins named "client".
		if plug.Name == clientRuntimeName && plug.Kind == workspace.LanguagePlugin {
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
func gatherPluginsFromSnapshot(plugctx *plugin.Context, target *deploy.Target) (pluginSet, error) {
	logging.V(preparePluginLog).Infof("gatherPluginsFromSnapshot(): gathering plugins from snapshot")
	set := newPluginSet()
	if target == nil || target.Snapshot == nil {
		logging.V(preparePluginLog).Infof("gatherPluginsFromSnapshot(): no snapshot available, skipping")
		return set, nil
	}
	for _, res := range target.Snapshot.Resources {
		urn := res.URN
		if !providers.IsProviderType(urn.Type()) {
			logging.V(preparePluginVerboseLog).Infof("gatherPluginsFromSnapshot(): skipping %q, not a provider", urn)
			continue
		}
		pkg := providers.GetProviderPackage(urn.Type())
		version, err := providers.GetProviderVersion(res.Inputs)
		if err != nil {
			return set, err
		}
		downloadURL, err := providers.GetProviderDownloadURL(res.Inputs)
		if err != nil {
			return set, err
		}
		logging.V(preparePluginLog).Infof(
			"gatherPluginsFromSnapshot(): plugin %s %s is required by first-class provider %q", pkg, version, urn)
		set.Add(workspace.PluginSpec{
			Name:              pkg.String(),
			Kind:              workspace.ResourcePlugin,
			Version:           version,
			PluginDownloadURL: downloadURL,
		})
	}
	return set, nil
}

// ensurePluginsAreInstalled inspects all plugins in the plugin set and, if any plugins are not currently installed,
// uses the given backend client to install them. Installations are processed in parallel, though
// ensurePluginsAreInstalled does not return until all installations are completed.
func ensurePluginsAreInstalled(ctx context.Context, plugins pluginSet, projectPlugins []workspace.ProjectPlugin) error {
	logging.V(preparePluginLog).Infof("ensurePluginsAreInstalled(): beginning")
	var installTasks errgroup.Group
	for _, plug := range plugins.Values() {
		if plug.Name == "pulumi" && plug.Kind == workspace.ResourcePlugin {
			logging.V(preparePluginLog).Infof("ensurePluginsAreInstalled(): pulumi is a builtin plugin")
			continue
		}

		path, err := workspace.GetPluginPath(plug.Kind, plug.Name, plug.Version, projectPlugins)
		if err == nil && path != "" {
			logging.V(preparePluginLog).Infof(
				"ensurePluginsAreInstalled(): plugin %s %s already installed", plug.Name, plug.Version)
			continue
		}

		// Launch an install task asynchronously and add it to the current error group.
		info := plug // don't close over the loop induction variable
		installTasks.Go(func() error {
			logging.V(preparePluginLog).Infof(
				"ensurePluginsAreInstalled(): plugin %s %s not installed, doing install", info.Name, info.Version)
			return installPlugin(ctx, info)
		})
	}

	err := installTasks.Wait()
	logging.V(preparePluginLog).Infof("ensurePluginsAreInstalled(): completed")
	return err
}

// ensurePluginsAreLoaded ensures that all of the plugins in the given plugin set that match the given plugin flags are
// loaded.
func ensurePluginsAreLoaded(plugctx *plugin.Context, plugins pluginSet, kinds plugin.Flags) error {
	return plugctx.Host.EnsurePlugins(plugins.Values(), kinds)
}

// installPlugin installs a plugin from the given backend client.
func installPlugin(ctx context.Context, plugin workspace.PluginSpec) error {
	logging.V(preparePluginLog).Infof("installPlugin(%s, %s): beginning install", plugin.Name, plugin.Version)
	if plugin.Kind == workspace.LanguagePlugin {
		logging.V(preparePluginLog).Infof(
			"installPlugin(%s, %s): is a language plugin, skipping install", plugin.Name, plugin.Version)
		return nil
	}

	// If we don't have a version yet try and call GetLatestVersion to fill it in
	if plugin.Version == nil {
		logging.V(preparePluginVerboseLog).Infof(
			"installPlugin(%s): version not specified, trying to lookup latest version", plugin.Name)

		version, err := plugin.GetLatestVersion()
		if err != nil {
			return fmt.Errorf("could not get latest version for plugin %s: %w", plugin.Name, err)
		}
		plugin.Version = version
	}

	logging.V(preparePluginVerboseLog).Infof(
		"installPlugin(%s, %s): initiating download", plugin.Name, plugin.Version)

	withProgress := func(stream io.ReadCloser, size int64) io.ReadCloser {
		return workspace.ReadCloserProgressBar(stream, size, "Downloading plugin", cmdutil.GetGlobalColorization())
	}
	retry := func(err error, attempt int, limit int, delay time.Duration) {
		logging.V(preparePluginVerboseLog).Infof(
			"Error downloading plugin: %s\nWill retry in %v [%d/%d]", err, delay, attempt, limit)
	}

	tarball, err := workspace.DownloadToFile(plugin, withProgress, retry)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %s: %w", plugin, err)
	}
	defer func() { contract.IgnoreError(os.Remove(tarball.Name())) }()

	fmt.Fprintf(os.Stderr, "[%s plugin %s-%s] installing\n", plugin.Kind, plugin.Name, plugin.Version)

	logging.V(preparePluginVerboseLog).Infof(
		"installPlugin(%s, %s): extracting tarball to installation directory", plugin.Name, plugin.Version)
	if err := plugin.InstallWithContext(ctx, workspace.TarPlugin(tarball), false); err != nil {
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
func computeDefaultProviderPlugins(languagePlugins, allPlugins pluginSet) map[tokens.Package]workspace.PluginSpec {
	// Language hosts are not required to specify the full set of plugins they depend on. If the set of plugins received
	// from the language host does not include any resource providers, fall back to the full set of plugins.
	languageReportedProviderPlugins := false
	for _, plug := range languagePlugins.Values() {
		if plug.Kind == workspace.ResourcePlugin {
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
		if p.Kind != workspace.ResourcePlugin {
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
