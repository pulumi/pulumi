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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/natefinch/atomic"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// basePluginMapper is a Mapper implementation that uses a list of installed plugins to source mappings.
type basePluginMapper struct {
	lock sync.Mutex

	// The key to use when querying provider plugins for mappings, to identify the type of the source provider.
	// "terraform" is an example of a conversion key which identifies mapping requests where the results are expected to
	// map Terraform resources to Pulumi resources, for instance.
	conversionKey string

	// The plugin storage context used to enumerate installed plugins.
	pluginContext pluginstorage.Context

	// A factory function that the mapper can use to instantiate provider plugins.
	providerFactory ProviderFactory

	// A function that the mapper can use to install plugins when it fails to locate them.
	installPlugin func(pluginName string) *semver.Version

	// A list of plugins that the mapper has enumerated as being available to serve mapping requests.
	pluginSpecs []basePluginMapperSpec

	// A list of hardcoded mappings read from files supplied to the mapper at construction time that will take priority
	// over any mappings returned by plugins.
	entries map[string][]byte

	// Options for tuning caching behaviour, exposed for testing.
	cacheOptions mapperCacheOptions
}

// mapperCacheOptions is a bag of flags for tuning the caching behaviour of a basePluginMapper, for testing.
type mapperCacheOptions struct {
	// disableFileCache disables the on-disk mapping cache in $PULUMI_HOME/mappings.
	disableFileCache bool
}

type basePluginMapperSpec struct {
	name    string
	version semver.Version

	// Metadata for the installed plugin, when known. Its install time governs the freshness of on-disk cached
	// mappings; specs without metadata (e.g. providers attached via PULUMI_DEBUG_PROVIDERS) are never cached.
	info *workspace.PluginInfo
}

// parseDebugProviderNames returns the provider names listed in a
// PULUMI_DEBUG_PROVIDERS env var (format: name:port[,name:port…]).
func parseDebugProviderNames(env string) []string {
	if env == "" {
		return nil
	}
	var out []string
	for entry := range strings.SplitSeq(env, ",") {
		k, _, ok := strings.Cut(entry, ":")
		if !ok {
			continue
		}
		out = append(out, strings.TrimSpace(k))
	}
	return out
}

// Workspace encapsulates an environment containing an enumerable set of plugins.
// NewBasePluginMapper creates a new plugin mapper backed by the supplied plugin context.
func NewBasePluginMapper(
	pluginContext pluginstorage.Context,
	conversionKey string,
	providerFactory ProviderFactory,
	installPlugin func(pluginName string) *semver.Version,
	mappings []string,
) (Mapper, error) {
	return newBasePluginMapper(
		pluginContext, conversionKey, providerFactory, installPlugin, mappings, mapperCacheOptions{})
}

func newBasePluginMapper(
	pluginContext pluginstorage.Context,
	conversionKey string,
	providerFactory ProviderFactory,
	installPlugin func(pluginName string) *semver.Version,
	mappings []string,
	cacheOptions mapperCacheOptions,
) (Mapper, error) {
	contract.Requiref(pluginContext != nil, "pluginContext", "must not be nil")
	contract.Requiref(providerFactory != nil, "providerFactory", "must not be nil")

	// Enumerate _all_ our installed plugins to ask for any mappings they provide. This allows users to convert aws
	// terraform code for example by just having 'pulumi-aws' plugin locally, without needing to specify it anywhere on
	// the command line, and without tf2pulumi needing to know about every possible plugin.
	allPlugins, err := pluginContext.GetPlugins(context.Background())
	if err != nil {
		return nil, fmt.Errorf("could not get plugins: %w", err)
	}

	// First assumption we only care about the latest version of each plugin. If we add support to get a mapping for
	// plugin version 1, it seems unlikely that we would remove support for that mapping in v2, so the latest version
	// should in most cases be fine. If a user case comes up where this is not fine we can provide the manual workaround
	// that this is based on what is locally installed, not what is published and so the user can just delete the higher
	// version plugins from their cache.
	latestVersions := make(map[string]workspace.PluginInfo)
	for _, plugin := range allPlugins {
		if plugin.Kind != apitype.ResourcePlugin {
			continue
		}

		if cur, has := latestVersions[plugin.Name]; !has || plugin.Version.GT(*cur.Version) {
			latestVersions[plugin.Name] = plugin
		}
	}

	// We now have a list of plugin specs (i.e. a name and version). Save that list because we don't want to iterate all
	// the plugins now because the convert might not even ask for any mappings.
	plugins := []basePluginMapperSpec{}
	seen := map[string]bool{}
	for _, plugin := range allPlugins {
		if plugin.Kind != apitype.ResourcePlugin {
			continue
		}

		info, has := latestVersions[plugin.Name]
		contract.Assertf(has, "latest version should be in map")

		plugins = append(plugins, basePluginMapperSpec{
			name:    plugin.Name,
			version: *info.Version,
			info:    &info,
		})
		seen[plugin.Name] = true
	}

	// Also enumerate providers attached via PULUMI_DEBUG_PROVIDERS so the
	// mapper considers them alongside installed plugins. The host's Provider
	// loader respects the same env var, so providerFactory will dial them.
	for _, name := range parseDebugProviderNames(os.Getenv("PULUMI_DEBUG_PROVIDERS")) {
		if seen[name] {
			continue
		}
		plugins = append(plugins, basePluginMapperSpec{name: name})
	}

	// Explicitly supplied mappings take precedence over any plugin returned mappings, but we want to error early if we
	// can't read any of these.
	entries := map[string][]byte{}
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
		pluginContext:   pluginContext,
		providerFactory: providerFactory,
		installPlugin:   installPlugin,
		pluginSpecs:     plugins,
		entries:         entries,
		cacheOptions:    cacheOptions,
	}, nil
}

// mappingFilePath returns the path to the mapping cache file for the given request. Mappings are cached in
// $PULUMI_HOME/mappings/ with a filename encoding the conversion key, source provider, plugin name and version, and a
// hash of the parameterization so different sources remain distinct.
func mappingFilePath(
	key, provider, pluginName string,
	version semver.Version,
	parameterization *workspace.Parameterization,
) (string, error) {
	fileName := fmt.Sprintf("%s-%s-%s-%s", key, provider, pluginName, version)
	if parameterization != nil {
		paramBytes, err := json.Marshal(parameterization)
		contract.AssertNoErrorf(err, "Parameterization should be marshalable to JSON")
		h := sha256.Sum256(paramBytes)
		fileName += "-" + hex.EncodeToString(h[:6])
	}
	fileName += ".mapping"
	return workspace.GetPulumiPath("mappings", fileName)
}

// loadCachedMapping reads a cached mapping from the given path. It returns false if the plugin's install time is
// unknown, or if the cache file does not exist or predates the plugin's installation.
func loadCachedMapping(path string, pluginInstallTime time.Time) ([]byte, bool) {
	if pluginInstallTime.IsZero() {
		return nil, false
	}

	stat, err := os.Stat(path)
	if err != nil || pluginInstallTime.After(stat.ModTime()) {
		return nil, false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

// writeCachedMapping writes a mapping to the given cache path. Writes are best-effort, matching the schema cache in
// pkg/codegen/schema: failures are logged and otherwise ignored.
func writeCachedMapping(path string, data []byte) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		logging.V(3).Infof("failed to create mapping cache directory for %s: %v", path, err)
		return
	}
	if err := atomic.WriteFile(path, bytes.NewReader(data)); err != nil {
		logging.V(3).Infof("failed to cache mapping at %s: %v", path, err)
	}
}

// findPluginInfo re-enumerates installed plugins to locate metadata for the named plugin at the given version,
// typically after a mid-run install. It returns nil if the plugin cannot be found.
func (m *basePluginMapper) findPluginInfo(
	ctx context.Context, name string, version semver.Version,
) *workspace.PluginInfo {
	allPlugins, err := m.pluginContext.GetPlugins(ctx)
	if err != nil {
		return nil
	}
	for _, p := range allPlugins {
		if p.Kind == apitype.ResourcePlugin && p.Name == name && p.Version != nil && p.Version.EQ(version) {
			return &p
		}
	}
	return nil
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
	ecosystem string,
) ([]byte, error) {
	// The ecosystem passed on the request identifies the mappings the caller consumes and takes precedence over
	// the conversion key this mapper was configured with. When empty, fall back to the configured key.
	key := m.conversionKey
	if ecosystem != "" {
		key = ecosystem
	}

	// See https://github.com/pulumi/pulumi/issues/14718 for why we need this lock. It may be possible to be
	// smarter about this and only lock when mutating, or at least splitting to a read/write lock, but this is
	// a quick fix to unblock providers. If you do attempt this then write tests to ensure this doesn't
	// regress #14718.
	m.lock.Lock()
	defer m.lock.Unlock()

	// If we have a perfect match in our hardcoded mappings, return that.
	if entry, has := m.entries[provider]; has {
		return entry, nil
	}

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
				info:    m.findPluginInfo(ctx, pluginName, *version),
			})
			m.pluginSpecs[0], m.pluginSpecs[i] = m.pluginSpecs[i], m.pluginSpecs[0]
		}
	}

	// Try the list of plugins we have and see if any of them produce a mapping we can return.
	for _, mapperSpec := range m.pluginSpecs {
		pluginSpec, err := workspace.NewPluginDescriptor(ctx, mapperSpec.name, apitype.ResourcePlugin, nil, "", nil)
		if err != nil {
			return nil, fmt.Errorf("could not create plugin spec for plugin %s: %w", pluginSpec.Name, err)
		}

		descriptor := workspace.NewPackageDescriptor(pluginSpec, nil)

		// If the current plugin's name matches that which we are looking for, and we have a hint that includes
		// parameterization information, we will pass that to the plugin as part of its instantiation.
		if mapperSpec.name == pluginName && hint != nil && hint.Parameterization != nil {
			descriptor.Parameterization = hint.Parameterization
		}

		// Successful mapping responses are cached on disk, since a mapping is fully determined by the plugin (with
		// any parameterization) and the request. On a cache hit we can skip booting the plugin entirely.
		var cachePath string
		if !m.cacheOptions.disableFileCache && mapperSpec.info != nil {
			cachePath, err = mappingFilePath(key, provider, mapperSpec.name, mapperSpec.version, descriptor.Parameterization)
			if err != nil {
				// Non-fatal: proceed without file caching.
				cachePath = ""
			}
			if mapping, ok := loadCachedMapping(cachePath, mapperSpec.info.InstallTime()); ok {
				return mapping, nil
			}
		}

		providerPlugin, err := m.providerFactory(descriptor)
		if err != nil {
			return nil, fmt.Errorf("could not create provider for package %s: %w", descriptor.PackageName(), err)
		}

		defer contract.IgnoreClose(providerPlugin)

		mappings, err := providerPlugin.GetMappings(ctx, plugin.GetMappingsRequest{
			Key: key,
		})
		if err != nil {
			return nil, fmt.Errorf(
				"could not get %s mappings for package %s: %w",
				key, descriptor.PackageName(), err,
			)
		}

		for _, mappingKey := range mappings.Keys {
			if mappingKey != provider {
				continue
			}

			mapping, err := providerPlugin.GetMapping(ctx, plugin.GetMappingRequest{
				Key:      key,
				Provider: provider,
			})
			if err != nil {
				return nil, fmt.Errorf("could not get advertized %s mapping for provider %s: %w", key, provider, err)
			}

			if mapping.Provider != provider {
				return nil, fmt.Errorf(
					"unexpected provider in %s mapping response for provider %s: %s",
					key, provider, mapping.Provider,
				)
			}

			writeCachedMapping(cachePath, mapping.Data)
			return mapping.Data, nil
		}

		// If we get here, it means that either the plugin reported no mappings back from GetMappings, or that it did but
		// none of them matched. We'll try a blind GetMapping call with an empty provider name to see if the plugin has
		// a mapping that matches that way.
		mapping, err := providerPlugin.GetMapping(ctx, plugin.GetMappingRequest{
			Key:      key,
			Provider: "",
		})
		if err != nil {
			return nil, fmt.Errorf("could not get %s mapping for provider %s: %w", key, provider, err)
		}

		if mapping.Provider == provider {
			writeCachedMapping(cachePath, mapping.Data)
			return mapping.Data, nil
		}
	}

	return []byte{}, nil
}
