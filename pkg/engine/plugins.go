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
	"fmt"
	"os"
	"sort"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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
type pluginSet map[string]workspace.PluginInfo

// Add adds a plugin to this plugin set.
func (p pluginSet) Add(plug workspace.PluginInfo) {
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

// Values returns a slice of all of the plugins contained within this set.
func (p pluginSet) Values() []workspace.PluginInfo {
	var plugins []workspace.PluginInfo
	for _, value := range p {
		plugins = append(plugins, value)
	}
	return plugins
}

// newPluginSet creates a new empty pluginSet.
func newPluginSet() pluginSet {
	return make(map[string]workspace.PluginInfo)
}

// gatherPluginsFromProgram inspects the given program and returns the set of plugins that the program requires to
// function. If the language host does not support this operation, the empty set is returned.
func gatherPluginsFromProgram(plugctx *plugin.Context, prog plugin.ProgInfo) (pluginSet, error) {
	logging.V(preparePluginLog).Infof("gatherPluginsFromProgram(): gathering plugins from language host")
	set := newPluginSet()
	langhostPlugins, err := plugin.GetRequiredPlugins(plugctx.Host, prog, plugin.AllPlugins)
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
			plug.Name, plug.Version, plug.ServerURL)
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
		logging.V(preparePluginLog).Infof(
			"gatherPluginsFromSnapshot(): plugin %s %s is required by first-class provider %q", pkg, version, urn)
		set.Add(workspace.PluginInfo{
			Name:    pkg.String(),
			Kind:    workspace.ResourcePlugin,
			Version: version,
		})
	}
	return set, nil
}

// ensurePluginsAreInstalled inspects all plugins in the plugin set and, if any plugins are not currently installed,
// uses the given backend client to install them. Installations are processed in parallel, though
// ensurePluginsAreInstalled does not return until all installations are completed.
func ensurePluginsAreInstalled(plugins pluginSet) error {
	logging.V(preparePluginLog).Infof("ensurePluginsAreInstalled(): beginning")
	var installTasks errgroup.Group
	for _, plug := range plugins.Values() {
		_, path, err := workspace.GetPluginPath(plug.Kind, plug.Name, plug.Version)
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
			return installPlugin(info)
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
func installPlugin(plugin workspace.PluginInfo) error {
	logging.V(preparePluginLog).Infof("installPlugin(%s, %s): beginning install", plugin.Name, plugin.Version)
	if plugin.Kind == workspace.LanguagePlugin {
		logging.V(preparePluginLog).Infof(
			"installPlugin(%s, %s): is a language plugin, skipping install", plugin.Name, plugin.Version)
		return nil
	}

	logging.V(preparePluginVerboseLog).Infof(
		"installPlugin(%s, %s): initiating download", plugin.Name, plugin.Version)
	stream, size, err := plugin.Download()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "[%s plugin %s-%s] installing\n", plugin.Kind, plugin.Name, plugin.Version)
	stream = workspace.ReadCloserProgressBar(stream, size, "Downloading plugin", cmdutil.GetGlobalColorization())

	logging.V(preparePluginVerboseLog).Infof(
		"installPlugin(%s, %s): extracting tarball to installation directory", plugin.Name, plugin.Version)
	if err := plugin.Install(stream); err != nil {
		return errors.Wrapf(err, "installing plugin; run `pulumi plugin install %s %s v%s` to retry manually",
			plugin.Kind, plugin.Name, plugin.Version)
	}

	logging.V(7).Infof("installPlugin(%s, %s): successfully installed", plugin.Name, plugin.Version)
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
// use to satisfy a resource registration. Since we do not today request a particular version of a plugin via
// RegisterResource (pulumi/pulumi#2389), this is the best we can do to infer the version that the language plugin
// actually wants.
//
// Whenever a resource arrives via RegisterResource and does not explicitly specify which provider to use, the engine
// injects a "default" provider resource that will serve as that resource's provider. This function computes the map
// that the engine uses to determine which version of a particular provider to load.
//
// it is critical that this function be 100% deterministic.
func computeDefaultProviderPlugins(languagePlugins, allPlugins pluginSet) map[tokens.Package]*semver.Version {
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

	defaultProviderPlugins := make(map[tokens.Package]workspace.PluginInfo)

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
	sort.Sort(workspace.SortedPluginInfo(sourcePlugins))
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
			if p.Version != nil && p.Version.GT(*seenPlugin.Version) {
				logging.V(preparePluginLog).Infof(
					"computeDefaultProviderPlugins(): plugin %s selected for package %s (override, newer than previous %s)",
					p, p.Name, seenPlugin.Version)
				defaultProviderPlugins[tokens.Package(p.Name)] = p
				continue
			}

			contract.Failf("Should not have seen an older plugin if sorting is correct!\n  %s-%s %s\n  %s-%s - %s",
				p.Name, p.Version.String(), p.Path,
				seenPlugin.Name, seenPlugin.Version.String(), seenPlugin.Path)
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

	defaultProviderVersions := make(map[tokens.Package]*semver.Version)
	for name, plugin := range defaultProviderPlugins {
		defaultProviderVersions[name] = plugin.Version
	}

	return defaultProviderVersions
}
