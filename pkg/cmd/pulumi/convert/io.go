// Copyright 2024, Pulumi Corporation.
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
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/util"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

func LoadConverterPlugin(
	ctx *plugin.Context,
	name string,
	log func(sev diag.Severity, msg string),
) (plugin.Converter, error) {
	// Default to the known version of the plugin, this ensures we use the version of the yaml-converter
	// that aligns with the yaml codegen we've linked to for this CLI release.
	pluginSpec := workspace.PluginSpec{
		Kind: apitype.ConverterPlugin,
		Name: name,
	}
	if versionSet := util.SetKnownPluginVersion(&pluginSpec); versionSet {
		ctx.Diag.Infof(
			diag.Message("", "Using version %s for pulumi-converter-%s"), pluginSpec.Version, pluginSpec.Name)
	}

	// Try and load the converter plugin for this
	converter, err := plugin.NewConverter(ctx, name, pluginSpec.Version)
	if err != nil {
		// If NewConverter returns a MissingError, we can try and install the plugin if it was missing and try again,
		// unless auto plugin installs are turned off.
		var me *workspace.MissingError
		if !errors.As(err, &me) || env.DisableAutomaticPluginAcquisition.Value() {
			// Not a MissingError, return the original error.
			return nil, fmt.Errorf("load %q: %w", name, err)
		}

		_, err = pkgWorkspace.InstallPlugin(ctx.Base(), pluginSpec, log)
		if err != nil {
			return nil, fmt.Errorf("install %q: %w", name, err)
		}

		converter, err = plugin.NewConverter(ctx, name, pluginSpec.Version)
		if err != nil {
			return nil, fmt.Errorf("load %q: %w", name, err)
		}
	}
	return converter, nil
}
