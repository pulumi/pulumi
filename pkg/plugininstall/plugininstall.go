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

package plugininstall

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type LogF = func(sev diag.Severity, msg string)

type Args struct {
	// The specification for the plugin to install.
	Spec workspace.PluginSpec
	// The already downloaded content of the spec to unpack.
	//
	// If Content is null, then [Go] will attempt to download the plugin before
	// installing it.
	Content PluginContent

	// Force a new install if the plugin is already installed.
	Force bool

	// LogDownload is called with status updates when a plugin is being downloaded.
	LogDownload LogF
	// LogInstall is called with status updates when a plugin is being installed.
	LogInstall LogF
}

type SpecSource struct{ specs []workspace.PluginSpec }

func FromSpec(spec workspace.PluginSpec) SpecSource {
	return SpecSource{specs: []workspace.PluginSpec{spec}}
}

func FromBaseProject(pluginOrProject workspace.BaseProject) SpecSource {
	return SpecSource{specs: []workspace.PluginSpec{pluginOrProject.GetPackageSpecs()}}
}

type Success struct{}

// Run a plugin installation based on a spec.
func Run(ctx context.Context, specs SpecSource, options ...Option) (Success, error) {
	panic("TODO")
}
