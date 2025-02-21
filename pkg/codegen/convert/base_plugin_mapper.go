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

package convert

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// basePluginMapper is a Mapper implementation that uses a list of installed plugins to source mappings.
type basePluginMapper struct {
	lock sync.Mutex

	// The key to use when querying provider plugins for mappings, to identify the type of the source provider.
	// "terraform" is an example of a conversion key which identifies mapping requests where the results are expected to
	// map Terraform resources to Pulumi resources, for instance.
	conversionKey string

	// A factory function that the mapper can use to instantiate provider plugins.
	providerFactory ProviderFactory

	// A function that the mapper can use to install plugins when it fails to locate them.
	installPlugin func(pluginName string) *semver.Version

	// A list of plugins that the mapper has enumerated as being available to serve mapping requests.
	pluginSpecs []basePluginMapperSpec
}

type basePluginMapperSpec struct {
	name    string
	version semver.Version
}

// Workspace encapsulates an environment containing an enumerable set of plugins.
type Workspace interface {
	// GetPlugins returns the list of plugins installed in the workspace.
	GetPlugins() ([]workspace.PluginInfo, error)
}

type defaultWorkspace struct{}

func (defaultWorkspace) GetPlugins() ([]workspace.PluginInfo, error) {
	return workspace.GetPlugins()
}

// DefaultWorkspace returns a default workspace implementation that uses the workspace module directly to get plugin
// info.
func DefaultWorkspace() Workspace {
	return defaultWorkspace{}
}

// NewBasePluginMapper creates a new plugin mapper backed by the supplied workspace.
func NewBasePluginMapper(
	ws Workspace,
	conversionKey string,
	providerFactory ProviderFactory,
	installPlugin func(pluginName string) *semver.Version,
	mappings []string,
) (Mapper, error) {
	contract.Requiref(ws != nil, "ws", "must not be nil")
	contract.Requiref(providerFactory != nil, "providerFactory", "must not be nil")

	entries := map[string][]byte{}

	// Enumerate _all_ our installed plugins to ask for any mappings they provide. This allows users to convert aws
	// terraform code for example by just having 'pulumi-aws' plugin locally, without needing to specify it anywhere on
	// the command line, and without tf2pulumi needing to know about every possible plugin.
	allPlugins, err := ws.GetPlugins()
	if err != nil {
		return nil, fmt.Errorf("could not get plugins: %w", err)
	}

	// First assumption we only care about the latest version of each plugin. If we add support to get a mapping for
	// plugin version 1, it seems unlikely that we would remove support for that mapping in v2, so the latest version
	// should in most cases be fine. If a user case comes up where this is not fine we can provide the manual workaround
	// that this is based on what is locally installed, not what is published and so the user can just delete the higher
	// version plugins from their cache.
	latestVersions := make(map[string]semver.Version)
	for _, plugin := range allPlugins {
		if plugin.Kind != apitype.ResourcePlugin {
			continue
		}

		if cur, has := latestVersions[plugin.Name]; has {
			if plugin.Version.GT(cur) {
				latestVersions[plugin.Name] = *plugin.Version
			}
		} else {
			latestVersions[plugin.Name] = *plugin.Version
		}
	}

	// We now have a list of plugin specs (i.e. a name and version). Save that list because we don't want to iterate all
	// the plugins now because the convert might not even ask for any mappings.
	plugins := []basePluginMapperSpec{}
	for _, plugin := range allPlugins {
		if plugin.Kind != apitype.ResourcePlugin {
			continue
		}

		version, has := latestVersions[plugin.Name]
		contract.Assertf(has, "latest version should be in map")

		plugins = append(plugins, basePluginMapperSpec{
			name:    plugin.Name,
			version: version,
		})
	}

	// Explicitly supplied mappings take precedence over any plugin returned mappings, but we want to error early if we
	// can't read any of these.
	for _, path := range mappings {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("could not read mapping file '%s': %w", path, err)
		}

		// Mapping file names are assumed to be the provider key.
		provider := filepath.Base(path)

		// Strip the extension.
		dotIndex := strings.LastIndex(provider, ".")
		if dotIndex != -1 {
			provider = provider[0:dotIndex]
		}

		entries[provider] = data
	}

	return &basePluginMapper{
		conversionKey:   conversionKey,
		providerFactory: providerFactory,
		installPlugin:   installPlugin,
		pluginSpecs:     plugins,
	}, nil
}

// Implements Mapper.GetMapping. A plugin mapper will try to resolve mappings by first building a list of candidate
// plugins as follows:
//
//   - If a hint is provided, the mapper will search for a plugin whose name matches that in the hint. If none is
//     supplied, the source provider name will be used as the plugin name to search for.
//   - The mapper will search its list of enumerated plugins for the name it has chosen. If it does not find a matching
//     plugin, it will attempt to install it using the callback supplied to it at construction time.
//   - If the mapper finds a matching plugin, either by enumeration or by installation, the matching plugin will be
//     moved to the front of the list of plugins to search for mappings, so that it takes priority.
//
// With a list of plugins constructed, the mapper will then query each in turn:
//
//   - If the plugin's name matches that in the hint, the mapper will pass parameterization information to the plugin as
//     part of its instantiation.
//   - With a plugin loaded, GetMappings (note the "s") will be called to see if the plugin reports the set of providers
//     for which it has mappings (e.g. an AWS provider plugin might report `["aws"]`).
//   - If GetMappings returns a non-empty result, the mapper will then call GetMapping (singular) if any of the keys
//     reported matches the source provider name.
//   - If GetMappings returns an empty result, or none of its reported keys match, GetMapping will be called with the
//     fallback behaviour of passing an empty provider name ("") to the plugin.
//
// If at any point a mapping is returned whose enclosed provider name matches that being searched for, it is returned.
// If no matches are encountered, an empty byte array result is returned.
func (m *basePluginMapper) GetMapping(
	ctx context.Context,
	provider string,
	hint *MapperPackageHint,
) ([]byte, error) {
	// See https://github.com/pulumi/pulumi/issues/14718 for why we need this lock. It may be possible to be
	// smarter about this and only lock when mutating, or at least splitting to a read/write lock, but this is
	// a quick fix to unblock providers. If you do attempt this then write tests to ensure this doesn't
	// regress #14718.
	m.lock.Lock()
	defer m.lock.Unlock()

	// If a hint is provided, we will search for a plugin whose name matches that in the hint. If none is supplied, the
	// source provider name will be used as the plugin name to search for.
	pluginName := provider
	if hint != nil {
		pluginName = hint.PluginName
	}

	// Is the plugin we're looking for already in the list of plugins?
	foundPlugin := false
	for i := 0; i < len(m.pluginSpecs); i++ {
		pluginSpec := m.pluginSpecs[i]
		if pluginSpec.name == pluginName {
			// Yes; move it to the head of the list so that we try it first.
			m.pluginSpecs[0], m.pluginSpecs[i] = m.pluginSpecs[i], m.pluginSpecs[0]
			foundPlugin = true
			break
		}
	}

	if !foundPlugin {
		// No; attempt to install it. If we succeed in installing it, we'll put the newly installed plugin at the head of
		// the list so that we try it first in the following loop.
		version := m.installPlugin(pluginName)
		if version != nil {
			i := len(m.pluginSpecs)
			m.pluginSpecs = append(m.pluginSpecs, basePluginMapperSpec{
				name:    pluginName,
				version: *version,
			})
			m.pluginSpecs[0], m.pluginSpecs[i] = m.pluginSpecs[i], m.pluginSpecs[0]
		}
	}

	// Try the list of plugins we have and see if any of them produce a mapping we can return.
	for _, pluginSpec := range m.pluginSpecs {
		descriptor := workspace.PackageDescriptor{
			PluginSpec: workspace.PluginSpec{
				Name:    pluginSpec.name,
				Version: &pluginSpec.version,
			},
		}

		// If the current plugin's name matches that which we are looking for, and we have a hint that includes
		// parameterization information, we will pass that to the plugin as part of its instantiation.
		if pluginSpec.name == pluginName && hint != nil && hint.Parameterization != nil {
			descriptor.Parameterization = hint.Parameterization
		}

		providerPlugin, err := m.providerFactory(descriptor)
		if err != nil {
			return nil, fmt.Errorf("could not create provider for package %s: %w", descriptor.PackageName(), err)
		}

		defer contract.IgnoreClose(providerPlugin)

		mappings, err := providerPlugin.GetMappings(ctx, plugin.GetMappingsRequest{
			Key: m.conversionKey,
		})
		if err != nil {
			return nil, fmt.Errorf(
				"could not get %s mappings for package %s: %w",
				m.conversionKey, descriptor.PackageName(), err,
			)
		}

		for _, mappingKey := range mappings.Keys {
			if mappingKey != provider {
				continue
			}

			mapping, err := providerPlugin.GetMapping(ctx, plugin.GetMappingRequest{
				Key:      m.conversionKey,
				Provider: provider,
			})
			if err != nil {
				return nil, fmt.Errorf("could not get advertized %s mapping for provider %s: %w", m.conversionKey, provider, err)
			}

			if mapping.Provider != provider {
				return nil, fmt.Errorf(
					"unexpected provider in %s mapping response for provider %s: %s",
					m.conversionKey, provider, mapping.Provider,
				)
			}

			return mapping.Data, nil
		}

		// If we get here, it means that either the plugin reported no mappings back from GetMappings, or that it did but
		// none of them matched. We'll try a blind GetMapping call with an empty provider name to see if the plugin has
		// a mapping that matches that way.
		mapping, err := providerPlugin.GetMapping(ctx, plugin.GetMappingRequest{
			Key:      m.conversionKey,
			Provider: "",
		})
		if err != nil {
			return nil, fmt.Errorf("could not get %s mapping for provider %s: %w", m.conversionKey, provider, err)
		}

		if mapping.Provider == provider {
			return mapping.Data, nil
		}
	}

	return []byte{}, nil
}
