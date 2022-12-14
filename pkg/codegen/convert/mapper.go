// Copyright 2016-2022, Pulumi Corporation.
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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// An interface to map provider names (N.B. These aren't Pulumi provider names, but the names of "providers"
// in the source language being converted from) to plugin specific mapping data.
type Mapper interface {
	// Returns plugin specific mapping data for the given provider name. Returns an empty result if no mapping
	// information was available.
	GetMapping(provider string) ([]byte, error)
}

type mapperPluginSpec struct {
	name    tokens.Package
	version semver.Version
}

type pluginMapper struct {
	host          plugin.Host
	conversionKey string
	plugins       []mapperPluginSpec
	entries       map[string][]byte
}

func NewPluginMapper(host plugin.Host, key string, mappings []string) (Mapper, error) {
	entries := map[string][]byte{}

	// Enumerate _all_ our installed plugins to ask for any mappings they provide. This allows users to
	// convert aws terraform code for example by just having 'pulumi-aws' plugin locally, without needing to
	// specify it anywhere on the command line, and without tf2pulumi needing to know about every possible
	// plugin.
	allPlugins, err := workspace.GetPlugins()
	if err != nil {
		return nil, fmt.Errorf("could not get plugins: %w", err)
	}

	// First assumption we only care about the latest version of each plugin. If we add support to get a
	// mapping for plugin version 1, it seems unlikely that we would remove support for that mapping in v2, so
	// the latest version should in most cases be fine. If a user case comes up where this is not fine we can
	// provide the manual workaround that this is based on what is locally installed, not what is published
	// and so the user can just delete the higher version plugins from their cache.
	latestVersions := make(map[string]semver.Version)
	for _, plugin := range allPlugins {
		if plugin.Kind != workspace.ResourcePlugin {
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

	// We now have a list of plugin specs (i.e. a name and version), save that list because we don't want to
	// iterate all the plugins now because the convert might not even ask for any mappings.
	plugins := make([]mapperPluginSpec, 0)
	for pkg, version := range latestVersions {
		plugins = append(plugins, mapperPluginSpec{
			name:    tokens.Package(pkg),
			version: version,
		})
	}

	// These take precedence over any plugin returned mappings, but we want to error early if we can't read
	// any of these.
	for _, path := range mappings {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("could not read mapping file '%s': %w", path, err)
		}

		// Mapping file names are assumed to be the provider key.
		provider := filepath.Base(path)
		// strip the extension
		dotIndex := strings.LastIndex(provider, ".")
		if dotIndex != -1 {
			provider = provider[0:dotIndex]
		}

		entries[provider] = data
	}
	return &pluginMapper{
		host:          host,
		conversionKey: key,
		plugins:       plugins,
		entries:       entries,
	}, nil
}

func (l *pluginMapper) GetMapping(provider string) ([]byte, error) {
	// Do we already have an entry for this provider, if so use it
	if entry, has := l.entries[provider]; has {
		return entry, nil
	}

	// No entry yet, start popping providers off the plugin list and return the first one that returns
	// conversion data for this provider for the given key we're looking for. Second assumption is that only
	// one pulumi provider will provide a mapping for each source mapping. This _might_ change in the future
	// if we for example add support to convert terraform to azure/aws-native, or multiple community members
	// bridge the same terraform provider. But as above the decisions here are based on what's locally
	// installed so the user can manually edit their plugin cache to be the set of plugins they want to use.
	for {
		if len(l.plugins) == 0 {
			// No plugins left to look in, return that we don't have a mapping
			return nil, nil
		}

		// Pop the first spec off the plugins list
		pluginSpec := l.plugins[0]
		l.plugins = l.plugins[1:]

		providerPlugin, err := l.host.Provider(pluginSpec.name, &pluginSpec.version)
		if err != nil {
			// We should maybe be lenient here and ignore errors but for now assume it's better to fail out on
			// things like providers failing to start.
			return nil, fmt.Errorf("could not create provider '%s': %w", pluginSpec.name, err)
		}

		data, mappedProvider, err := providerPlugin.GetMapping(l.conversionKey)
		if err != nil {
			// This was an error calling GetMapping, not just that GetMapping returned a nil result. It's fine
			// for GetMapping to return (nil, nil).
			return nil, fmt.Errorf("could not get mapping for provider '%s': %w", pluginSpec.name, err)
		}
		// A provider returns empty if it didn't have a mapping
		if mappedProvider != "" && len(data) != 0 {
			// Don't overwrite entries, the first wins
			if _, has := l.entries[mappedProvider]; !has {
				l.entries[mappedProvider] = data

				// If this was the provider we we're looking for we can now return it
				if mappedProvider == provider {
					return data, nil
				}
			}
		}
	}
}
