// Copyright 2016, Pulumi Corporation.
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

//nolint:revive // Legacy package name we don't want to change
package util

import (
	"slices"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// knownLanguageRuntimes pins the release of each language runtime that is no longer
// bundled with the `pulumi` binary and must be downloaded on demand.
var knownLanguageRuntimes = map[string]semver.Version{
	// renovate: datasource=github-releases depName=pulumi/pulumi-hcl extractVersion=^v(?<version>.+)$
	"hcl": semver.MustParse("0.13.0"),
}

// SetKnownPluginDownloadURL fills in metadata on the given PluginDescriptor that the CLI
// knows about for well-known plugins: a PluginDownloadURL for plugins hosted outside the
// default locations, and a pinned Version for unbundled language runtimes. Returns true
// if the descriptor names a plugin the CLI knows how to fetch.
func SetKnownPluginDownloadURL(spec *workspace.PluginDescriptor) bool {
	// If the download url is already set don't touch it
	if spec.PluginDownloadURL != "" {
		return false
	}

	if spec.Kind == apitype.ResourcePlugin {
		if slices.Contains(pulumiversePlugins, spec.Name) {
			spec.PluginDownloadURL = "github://api.github.com/pulumiverse"
			return true
		}
	}

	if spec.Kind == apitype.LanguagePlugin {
		if version, ok := knownLanguageRuntimes[spec.Name]; ok && spec.Version == nil {
			spec.Version = &version
			return true
		}
	}

	return false
}
