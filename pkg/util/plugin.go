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

// knownLanguageRuntime describes a language runtime that is no longer bundled with the
// `pulumi` binary and must be downloaded on demand. The URL points at the release source
// (typically a GitHub repository) and Version pins the release the CLI is built against.
type knownLanguageRuntime struct {
	PluginDownloadURL string
	Version           semver.Version
}

// knownLanguageRuntimes is the set of language runtimes that the CLI knows how to fetch
// without being told a download URL by the user or a project file. As we migrate
// previously-bundled language runtimes off the CLI release tarball, they are added here so
// the CLI continues to know where to find them.
var knownLanguageRuntimes = map[string]knownLanguageRuntime{
	// The HCL language runtime lives in pulumi-labs/pulumi-hcl rather than pulumi/pulumi-hcl,
	// so we have to point downloads at that repo explicitly.
	"hcl": {
		PluginDownloadURL: "github://api.github.com/pulumi-labs/pulumi-hcl",
		// renovate: datasource=github-releases depName=pulumi-labs/pulumi-hcl extractVersion=^v(?<version>.+)$
		Version: semver.MustParse("0.4.0"),
	},
}

// SetKnownPluginDownloadURL fills in metadata on the given PluginDescriptor that the CLI
// knows about for well-known plugins: a PluginDownloadURL, and for unbundled language
// runtimes, a pinned Version. Returns true if it filled in anything.
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

	if spec.Kind == apitype.ConverterPlugin && spec.Name == "hcl" {
		// The HCL converter lives in the pulumi-labs org rather than pulumi, and at a repository
		// name (pulumi-hcl) that doesn't match the default pulumi-converter-<name> convention. We
		// encode both pieces of information here so the auto-install machinery resolves the right
		// release artifact.
		spec.PluginDownloadURL = "github://api.github.com/pulumi-labs/pulumi-hcl"
		return true
	}

	if spec.Kind == apitype.LanguagePlugin {
		if known, ok := knownLanguageRuntimes[spec.Name]; ok {
			spec.PluginDownloadURL = known.PluginDownloadURL
			if spec.Version == nil {
				v := known.Version
				spec.Version = &v
			}
			return true
		}
	}

	return false
}
