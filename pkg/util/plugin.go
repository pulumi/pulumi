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

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// SetKnownPluginDownloadURL sets the PluginDownloadURL for the given PluginSpec if it's a known plugin.
// Returns true if it filled in the URL.
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

	return false
}
