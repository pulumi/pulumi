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

	"golang.org/x/sync/errgroup"

	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/workspace"
)

const (
	preparePluginLog        = 7
	preparePluginVerboseLog = 8
)

// pluginSet represents a set of plugins.
type pluginSet struct {
	plugins map[string]workspace.PluginInfo
}

// Add adds a plugin to this plugin set.
func (p *pluginSet) Add(plug workspace.PluginInfo) {
	p.plugins[plug.String()] = plug
}

// Union returns the union of this pluginSet with another pluginSet.
func (p *pluginSet) Union(other pluginSet) pluginSet {
	newSet := newPluginSet()
	for _, value := range p.plugins {
		newSet.Add(value)
	}
	for _, value := range other.plugins {
		newSet.Add(value)
	}
	return newSet
}

// Values returns a slice of all of the plugins contained within this set.
func (p *pluginSet) Values() []workspace.PluginInfo {
	var plugins []workspace.PluginInfo
	for _, value := range p.plugins {
		plugins = append(plugins, value)
	}
	return plugins
}

// newPluginSet creates a new empty pluginSet.
func newPluginSet() pluginSet {
	return pluginSet{make(map[string]workspace.PluginInfo)}
}

// gatherPluginsFromProgram inspects the given program and returns the set of plugins that the program requires to
// function. If the language host does not support this operation, the empty set is returned.
func gatherPluginsFromProgram(plugctx *plugin.Context, prog plugin.ProgInfo) (pluginSet, error) {
	logging.V(preparePluginLog).Infof("gatherPluginsFromProgram(): gathering plugins from language host")
	set := newPluginSet()
	langhostPlugins, err := plugctx.Host.GetRequiredPlugins(prog, plugin.AllPlugins)
	if err != nil {
		return set, err
	}
	for _, plug := range langhostPlugins {
		logging.V(preparePluginLog).Infof(
			"gatherPluginsFromProgram(): plugin %s %s is required by language host", plug.Name, plug.Version)
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
	for _, plug := range target.Snapshot.Manifest.Plugins {
		logging.V(preparePluginLog).Infof(
			"gatherPluginsFromSnapshot(): plugin %s %s is required by snapshot manifest", plug.Name, plug.Version)
		set.Add(plug)
	}
	return set, nil
}

// ensurePluginsAreInstalled inspects all plugins in the plugin set and, if any plugins are not currently installed,
// uses the given backend client to install them. Installations are processed in parallel, though
// ensurePluginsAreInstalled does not return until all installations are completed.
func ensurePluginsAreInstalled(client deploy.BackendClient, plugins pluginSet) error {
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
		//
		// To avoid closing over a loop induction variable by reference, we instruct the Go compiler to make a copy of
		// plug here by calling a function with plug as the first argument.
		installTasks.Go(func(plug workspace.PluginInfo) func() error {
			return func() error {
				logging.V(preparePluginLog).Infof(
					"ensurePluginsAreInstalled(): plugin %s %s not installed, doing install", plug.Name, plug.Version)
				return installPlugin(client, plug)
			}
		}(plug))
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
func installPlugin(client deploy.BackendClient, plugin workspace.PluginInfo) error {
	logging.V(preparePluginLog).Infof("installPlugin(%s, %s): beginning install", plugin.Name, plugin.Version)
	if plugin.Kind == workspace.LanguagePlugin {
		logging.V(preparePluginLog).Infof(
			"installPlugin(%s, %s): is a language plugin, skipping install", plugin.Name, plugin.Version)
		return nil
	}

	logging.V(preparePluginVerboseLog).Infof(
		"installPlugin(%s, %s): initiating download", plugin.Name, plugin.Version)
	stream, err := client.DownloadPlugin(context.TODO(), plugin)
	if err != nil {
		return err
	}
	logging.V(preparePluginVerboseLog).Infof(
		"installPlugin(%s, %s): extracting tarball to installation directory", plugin.Name, plugin.Version)
	if err := plugin.Install(stream); err != nil {
		return err
	}

	logging.V(7).Infof("installPlugin(%s, %s): successfully installed", plugin.Name, plugin.Version)
	return nil
}
