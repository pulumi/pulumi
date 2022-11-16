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
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type Mapper interface {
	GetMapping(provider string) ([]byte, error)
}

type pluginMapper struct {
	entries map[string][]byte
}

func NewPluginMapper(host plugin.Host, key string, mappings []string) (Mapper, error) {
	entries := map[string][]byte{}

	// Enumerate _all_ our installed plugins to ask for any mappings they provider. This allows users to
	// convert aws terraform code for example by just having 'pulumi-aws' plugin locally, without needing to
	// specify it anywhere on the command line, and without tf2pulumi needing to know about every possible plugin.
	plugins, err := workspace.GetPlugins()
	if err != nil {
		return nil, fmt.Errorf("could not get plugins: %w", err)
	}
	// We only care about the latest version of each plugin
	latestVersions := make(map[string]semver.Version)
	for _, plugin := range plugins {
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
	// Now go through each of those plugins and ask for any conversion data they have for the given key we're
	// looking for.
	//for pkg, version := range latestVersions {
	// TODO: We have to do a dance here where first we publish a version of pulumi with these RPC structures
	// then add methods to terraform-bridge to implement this method as if it did exist, and then actually add
	// the RPC method and uncomment out the code below. This is all because we currently build these in a loop
	// (pulumi include terraform-bridge, which includes pulumi).

	//provider, err := host.Provider(tokens.Package(pkg), &version)
	//if err != nil {
	//	return nil, fmt.Errorf("could not create provider '%s': %w", pkg, err)
	//}

	//data, mappedProvider, err := provider.GetMapping(key)
	//if err != nil {
	//	return nil, fmt.Errorf("could not get mapping for provider '%s': %w", pkg, err)
	//}
	//entries[mappedProvider] = data
	//}

	// These take precedence over any plugin returned mappings so we do them last and just overwrite the
	// entries
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
		entries: entries,
	}, nil
}

func (l *pluginMapper) GetMapping(provider string) ([]byte, error) {
	entry, ok := l.entries[provider]
	if ok {
		return entry, nil
	}
	return nil, fmt.Errorf("could not find any conversion mapping for %s", provider)
}
