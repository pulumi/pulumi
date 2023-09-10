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

package util

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// SetKnownPluginDownloadURL sets the PluginDownloadURL for the given PluginSpec if it's a known plugin.
// Returns true if it filled in the URL.
func SetKnownPluginDownloadURL(spec *workspace.PluginSpec) bool {
	// If the download url is already set don't touch it
	if spec.PluginDownloadURL != "" {
		return false
	}

	// Zaid's bicep converter so that `pulumi convert --from bicep` just works.
	if spec.Kind == workspace.ConverterPlugin && spec.Name == "bicep" {
		spec.PluginDownloadURL = "github://api.github.com/Zaid-Ajaj"
		return true
	}

	pulumiversePlugins := []string{
		"acme",
		"aquasec",
		"astra",
		"aws-eksa",
		"buildkite",
		"concourse",
		"configcat",
		"doppler",
		"exoscale",
		"gandi",
		"github-credentials",
		"googleworkspace",
		"harbor",
		"hcp",
		"heroku",
		"matchbox",
		"mssql",
		"ngrok",
		"purrl",
		"sentry",
		"statuscake",
		"time",
		"unifi",
		"vra",
		"zitadel",
	}
	if spec.Kind == workspace.ResourcePlugin {
		for _, plugin := range pulumiversePlugins {
			if spec.Name == plugin {
				spec.PluginDownloadURL = "github://api.github.com/pulumiverse"
				return true
			}
		}
	}

	return false
}
