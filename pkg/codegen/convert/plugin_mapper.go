// Copyright 2016-2023, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Workspace is the current workspace.
// This is used to get the list of plugins installed in the workspace.
// It's analogous to the workspace package, but scoped down to just the parts we need.
//
// This should probably be used to replace a load of our currently hardcoded for real world (i.e actual file
// system, actual http calls) plugin workspace code, but for now we're keeping it scoped just to help out with
// testing the mapper code.
type Workspace interface {
	// GetPlugins returns the list of plugins installed in the workspace.
	GetPlugins() ([]workspace.PluginInfo, error)
}

type defaultWorkspace struct{}

func (defaultWorkspace) GetPlugins() ([]workspace.PluginInfo, error) {
	return workspace.GetPlugins()
}

// DefaultWorkspace returns a default workspace implementation
// that uses the workspace module directly to get plugin info.
func DefaultWorkspace() Workspace {
	return defaultWorkspace{}
}

// ProviderFactory creates a provider for a given package and version.
type ProviderFactory func(tokens.Package, *semver.Version) (plugin.Provider, error)

// hostManagedProvider is Provider built from a plugin.Host.
type hostManagedProvider struct {
	plugin.Provider

	host plugin.Host
}

var _ plugin.Provider = (*hostManagedProvider)(nil)

func (pc *hostManagedProvider) Close() error {
	return pc.host.CloseProvider(pc.Provider)
}

// ProviderFactoryFromHost builds a ProviderFactory
// that uses the given plugin host to create providers.
func ProviderFactoryFromHost(host plugin.Host) ProviderFactory {
	return func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
		descriptor := workspace.PackageDescriptor{
			PluginSpec: workspace.PluginSpec{
				Name:    string(pkg),
				Version: version,
			},
		}

		provider, err := host.Provider(descriptor)
		if err != nil {
			desc := pkg.String()
			if version != nil {
				desc += "@" + version.String()
			}
			return nil, fmt.Errorf("load plugin %v: %w", desc, err)
		}

		return &hostManagedProvider{
			Provider: provider,
			host:     host,
		}, nil
	}
}

type mapperPluginSpec struct {
	name    tokens.Package
	version semver.Version
	// An optional list of providers this plugin can map to, only filled if GetMappings is implemented.
	mappings []string
	// Set to true once we've called GetMappings, mappings may still be nil after this if GetMappings wasn't
	// implemented.
	calledGetMappings bool
}

type pluginMapper struct {
	providerFactory ProviderFactory
	conversionKey   string
	plugins         []mapperPluginSpec
	entries         map[string][]byte
	installProvider func(tokens.Package) *semver.Version
	lock            sync.Mutex
}

func NewPluginMapper(ws Workspace,
	providerFactory ProviderFactory,
	key string, mappings []string,
	installProvider func(tokens.Package) *semver.Version,
) (Mapper, error) {
	contract.Requiref(providerFactory != nil, "providerFactory", "must not be nil")
	contract.Requiref(ws != nil, "ws", "must not be nil")

	entries := map[string][]byte{}

	// Enumerate _all_ our installed plugins to ask for any mappings they provide. This allows users to
	// convert aws terraform code for example by just having 'pulumi-aws' plugin locally, without needing to
	// specify it anywhere on the command line, and without tf2pulumi needing to know about every possible
	// plugin.
	allPlugins, err := ws.GetPlugins()
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

	// We now have a list of plugin specs (i.e. a name and version), save that list because we don't want to
	// iterate all the plugins now because the convert might not even ask for any mappings.
	plugins := make([]mapperPluginSpec, 0)
	for _, plugin := range allPlugins {
		if plugin.Kind != apitype.ResourcePlugin {
			continue
		}

		version, has := latestVersions[plugin.Name]
		contract.Assertf(has, "latest version should be in map")

		plugins = append(plugins, mapperPluginSpec{
			name:    tokens.Package(plugin.Name),
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
		providerFactory: providerFactory,
		conversionKey:   key,
		plugins:         plugins,
		entries:         entries,
		installProvider: installProvider,
	}, nil
}

// getMappingForPlugin calls GetMapping on the given plugin and returns it's result. Currently if looking up
// the "terraform" mapping and getting an empty result this will fallback to also asking for the "tf" mapping.
// This is because tfbridge providers originally only replied to "tf", while new ones reply (with the same
// answer) to both "tf" and "terraform".
func (l *pluginMapper) getMappingForPlugin(pluginSpec mapperPluginSpec, provider string) ([]byte, string, error) {
	providerPlugin, err := l.providerFactory(pluginSpec.name, &pluginSpec.version)
	if err != nil {
		// We should maybe be lenient here and ignore errors but for now assume it's better to fail out on
		// things like providers failing to start.
		return nil, "", fmt.Errorf("could not create provider '%s': %w", pluginSpec.name, err)
	}
	defer contract.IgnoreClose(providerPlugin)

	conversionKeys := []string{l.conversionKey}
	if l.conversionKey == "terraform" {
		// TODO: Temporary hack to work around the fact that most of the plugins return a mapping for "tf" but
		// not "terraform" but they're the same thing.
		conversionKeys = append(conversionKeys, "tf")
	}

	// We'll delete this for loop once the plugins have had a chance to update.
	for _, conversionKey := range conversionKeys {
		mapping, err := providerPlugin.GetMapping(context.TODO(), plugin.GetMappingRequest{
			Key:      conversionKey,
			Provider: provider,
		})
		if err != nil {
			// This was an error calling GetMapping, not just that GetMapping returned a nil result. It's fine for
			// GetMapping to return (nil, "", nil) as that simply indicates that the plugin doesn't have a mapping
			// for the requested key.
			return nil, "", fmt.Errorf("could not get mapping for provider '%s': %w", pluginSpec.name, err)
		}
		// A provider should return non-empty results if it has a mapping.
		if mapping.Provider != "" && len(mapping.Data) != 0 {
			return mapping.Data, mapping.Provider, nil
		}
		// If a provider returns (empty, "provider") we also treat that as no mapping, because only the slice part
		// gets returned to the converter plugin and it needs to assume that empty means no mapping, but we warn
		// that this is unexpected.
		if mapping.Provider != "" && len(mapping.Data) == 0 {
			logging.Warningf(
				"provider '%s' returned empty data but a filled provider name '%s' for '%s', "+
					"this is unexpected behaviour assuming no mapping", pluginSpec.name, mapping.Provider, conversionKey)
		}
	}

	return nil, "", err
}

func (l *pluginMapper) getMappingsForPlugin(pluginSpec *mapperPluginSpec, provider string) ([]byte, bool, error) {
	var providerPlugin plugin.Provider
	if !pluginSpec.calledGetMappings {
		var err error
		providerPlugin, err = l.providerFactory(pluginSpec.name, &pluginSpec.version)
		if err != nil {
			// We should maybe be lenient here and ignore errors but for now assume it's better to fail out on
			// things like providers failing to start.
			return nil, false, fmt.Errorf("could not create provider '%s': %w", pluginSpec.name, err)
		}
		defer contract.IgnoreClose(providerPlugin)

		mappings, err := providerPlugin.GetMappings(context.TODO(), plugin.GetMappingsRequest{
			Key: l.conversionKey,
		})
		if err != nil {
			return nil, false, fmt.Errorf("could not get mappings for provider '%s': %w", pluginSpec.name, err)
		}

		pluginSpec.calledGetMappings = true
		pluginSpec.mappings = mappings.Keys
	}

	var hasMapping bool
	for _, mapping := range pluginSpec.mappings {
		if mapping == provider {
			hasMapping = true
			break
		}
	}

	if hasMapping {
		// This reports it has the mapping so just return that
		if providerPlugin == nil {
			var err error
			providerPlugin, err = l.providerFactory(pluginSpec.name, &pluginSpec.version)
			if err != nil {
				return nil, false, fmt.Errorf("could not create provider '%s': %w", pluginSpec.name, err)
			}
			defer contract.IgnoreClose(providerPlugin)
		}

		mapping, err := providerPlugin.GetMapping(context.TODO(), plugin.GetMappingRequest{
			Key:      l.conversionKey,
			Provider: provider,
		})
		if err != nil {
			return nil, false, fmt.Errorf("could not get mapping for provider '%s': %w", pluginSpec.name, err)
		}
		if mapping.Provider != provider {
			return nil, false, fmt.Errorf(
				"mapping call returned unexpected provider, expected '%s', got '%s'",
				provider, mapping.Provider)
		}

		return mapping.Data, true, nil
	}

	return nil, false, nil
}

func (l *pluginMapper) GetMapping(ctx context.Context, provider string, pulumiProvider string) ([]byte, error) {
	// See https://github.com/pulumi/pulumi/issues/14718 for why we need this lock. It may be possible to be
	// smarter about this and only lock when mutating, or at least splitting to a read/write lock, but this is
	// a quick fix to unblock providers. If you do attempt this then write tests to ensure this doesn't
	// regress #14718.
	l.lock.Lock()
	defer l.lock.Unlock()

	// If we already have an entry for this provider, use it
	if entry, has := l.entries[provider]; has {
		return entry, nil
	}

	// Converters might not set pulumiProvider so default it to the same name as the foreign provider.
	if pulumiProvider == "" {
		pulumiProvider = provider
	}
	pulumiProviderPkg := tokens.Package(pulumiProvider)

	// Optimization:
	// If there's a plugin with a name that matches the expected pulumi provider name, move it to the front of
	// the list.
	// This is a common case, so we can avoid an expensive linear search through the rest of the plugins.
	foundPulumiProvider := false
	for i := 0; i < len(l.plugins); i++ {
		pluginSpec := l.plugins[i]
		if pluginSpec.name == pulumiProviderPkg {
			l.plugins[0], l.plugins[i] = l.plugins[i], l.plugins[0]
			foundPulumiProvider = true
			break
		}
	}

	// If we didn't find the pulumi provider in the list of plugins, then we want to try to install it. Note
	// that we don't want to hard fail here because it might turn out the provider hint was bad.
	if !foundPulumiProvider {
		version := l.installProvider(pulumiProviderPkg)
		if version != nil {
			// Insert at the front of the plugins list. Easiest way to do this is just append then swap.
			i := len(l.plugins)
			l.plugins = append(l.plugins, mapperPluginSpec{
				name:    pulumiProviderPkg,
				version: *version,
			})
			l.plugins[0], l.plugins[i] = l.plugins[i], l.plugins[0]
		}
	}

	// Before we begin the GetMappings loop below iff we've got a plugin at the head of the list which is the exact name
	// match we'll try that plugin first (GetMappings, and then GetMapping) as it will normally be right.
	if len(l.plugins) > 0 && l.plugins[0].name == pulumiProviderPkg {
		data, found, err := l.getMappingsForPlugin(&l.plugins[0], provider)
		if err != nil {
			return nil, err
		}
		// Found it via GetMappings lookup just return it
		if found {
			// Don't overwrite entries, the first wins
			if _, has := l.entries[provider]; !has {
				l.entries[provider] = data
			}
			return data, nil
		}

		// Once we call GetMappping("") we'll not use this plugin again so pop it from the list.
		pluginSpec := l.plugins[0]
		l.plugins = l.plugins[1:]
		data, mappedProvider, err := l.getMappingForPlugin(pluginSpec, "")
		if err != nil {
			return nil, err
		}
		if mappedProvider != "" {
			// Don't overwrite entries, the first wins
			if _, has := l.entries[mappedProvider]; !has {
				l.entries[mappedProvider] = data
			}
			// If this was the provider we we're looking for we can now return it
			if mappedProvider == provider {
				return data, nil
			}
		}
	}

	// The first plugin didn't match by name (or did but didn't have the mapping we wanted) so scan is to see if we can
	// find a plugin thats reports this conversion via GetMappings. If one does then ask it for the mapping and return
	// that. Else cache which mappings are reported in case we need one of those later.
	for idx := range l.plugins {
		data, found, err := l.getMappingsForPlugin(&l.plugins[idx], provider)
		if err != nil {
			return nil, err
		}

		if found {
			// Don't overwrite entries, the first wins
			if _, has := l.entries[provider]; !has {
				l.entries[provider] = data
			}
			return data, nil
		}
	}

	// No entry yet, start popping providers off the plugin list and return the first one that returns
	// conversion data for this provider for the given key we're looking for. Second assumption is that only
	// one pulumi provider will provide a mapping for each source mapping. This _might_ change in the future
	// if we for example add support to convert terraform to azure/aws-native, or multiple community members
	// bridge the same terraform provider. But as above the decisions here are based on what's locally
	// installed so the user can manually edit their plugin cache to be the set of plugins they want to use.
	for {
		// If we're in this loop we're looking for the mapping via legacy GetMapping("") calls. We shouldn't make these
		// calls against providers who have told us the set they map against.

		// Find a plugin that doesn't have any mapping information, we'll call GetMapping("") on it.
		var pluginSpec *mapperPluginSpec
		for idx, spec := range l.plugins {
			spec := spec
			contract.Assertf(spec.calledGetMappings, "GetMappings should have been called")
			// If this plugin has mapping information then don't call GetMapping("") on it, if it had the right mapping it
			// would have been picked up in the loop above.
			if spec.mappings == nil {
				pluginSpec = &spec

				// We're going to call GetMapping("") on this plugin, it will never be needed again so remove it from
				// the list by overwriting it with the plugin from the end and then shrinking the slice.
				last := len(l.plugins) - 1
				l.plugins[idx] = l.plugins[last]
				l.plugins = l.plugins[0:last]
				break
			}
		}

		if pluginSpec == nil {
			// No plugins left to look in, return that we don't have a mapping but first save that we'll never
			// find a mapping for this provider key.
			l.entries[provider] = []byte{}
			return []byte{}, nil
		}

		data, mappedProvider, err := l.getMappingForPlugin(*pluginSpec, "")
		if err != nil {
			return nil, err
		}
		if mappedProvider != "" {
			// Don't overwrite entries, the first wins
			if _, has := l.entries[mappedProvider]; !has {
				l.entries[mappedProvider] = data
			}
			// If this was the provider we we're looking for we can now return it
			if mappedProvider == provider {
				return data, nil
			}
		}
	}
}
