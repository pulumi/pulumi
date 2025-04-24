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
	"runtime/debug"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// SetKnownPluginDownloadURL sets the PluginDownloadURL for the given PluginSpec if it's a known plugin.
// Returns true if it filled in the URL.
func SetKnownPluginDownloadURL(spec *workspace.PluginSpec) bool {
	// If the download url is already set don't touch it
	if spec.PluginDownloadURL != "" {
		return false
	}

	if spec.Kind == apitype.ResourcePlugin {
		for _, plugin := range pulumiversePlugins {
			if spec.Name == plugin {
				spec.PluginDownloadURL = "github://api.github.com/pulumiverse"
				return true
			}
		}
	}

	return false
}

// SetKnownPluginVersion sets the Version for the given PluginSpec if it's a known plugin.
// Returns true if it filled in the version.
func SetKnownPluginVersion(spec *workspace.PluginSpec) bool {
	// If the version is already set don't touch it
	if spec.Version != nil {
		return false
	}

	if spec.Kind == apitype.ConverterPlugin && spec.Name == "yaml" {
		// By default use the version of yaml we've linked to. N.B. This has to be tested manually because
		// ReadBuildInfo doesn't return anything in test builds (https://github.com/golang/go/issues/33976).
		info, ok := debug.ReadBuildInfo()
		contract.Assertf(ok, "expected to be able to read build info")
		for _, dep := range info.Deps {
			if dep.Path == "github.com/pulumi/pulumi-yaml" {
				v, err := semver.ParseTolerant(dep.Version)
				contract.AssertNoErrorf(err, "expected to be able to parse version for yaml got %q", dep.Version)
				spec.Version = &v
				return true
			}
		}
	}

	return false
}
