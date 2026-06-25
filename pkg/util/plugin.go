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

// unbundledLanguageRuntimeSpec is one entry of unbundledLanguageRuntimeData, which is
// generated from the repo-root versions.json (see known_runtimes_gen.go). Repo is the GitHub
// owner/repo that publishes the runtime's releases; Version is the pinned release as a semver
// string without a leading "v".
type unbundledLanguageRuntimeSpec struct {
	Name    string
	Repo    string
	Version string
}

// knownLanguageRuntimes is the set of language runtimes that the CLI knows how to fetch
// without being told a download URL by the user or a project file. As we migrate
// previously-bundled language runtimes off the CLI release tarball, they are recorded in
// versions.json (the single source of truth, updated by renovate) and surfaced here via the
// generated unbundledLanguageRuntimeData so the CLI continues to know where to find them.
var knownLanguageRuntimes = buildKnownLanguageRuntimes()

func buildKnownLanguageRuntimes() map[string]knownLanguageRuntime {
	m := make(map[string]knownLanguageRuntime, len(unbundledLanguageRuntimeData))
	for _, r := range unbundledLanguageRuntimeData {
		m[r.Name] = knownLanguageRuntime{
			PluginDownloadURL: githubReleasesDownloadURL(r.Repo),
			Version:           semver.MustParse(r.Version),
		}
	}
	return m
}

// githubReleasesDownloadURL returns the github:// download URL the CLI uses to fetch a plugin
// published as GitHub releases under the given owner/repo.
func githubReleasesDownloadURL(repo string) string {
	return "github://api.github.com/" + repo
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
		// The HCL converter ships from the same repo as the HCL language runtime
		// (pulumi-labs/pulumi-hcl), which doesn't match the default pulumi-converter-<name>
		// convention, so we point downloads there explicitly. Reusing the language runtime's entry
		// keeps that repo recorded in exactly one place (versions.json).
		if known, ok := knownLanguageRuntimes["hcl"]; ok {
			spec.PluginDownloadURL = known.PluginDownloadURL
			return true
		}
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
