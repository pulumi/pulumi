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

package main

import (
	"errors"
	"fmt"
	"os"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newPackageNewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <boilerplate> <name> [config]",
		Args:  cmdutil.RangeArgs(2, 3),
		Short: "Create a new Pulumi package.",
		Long: `Create a new Pulumi package.

<boilerplate> defines the type of package to create, selecting a valid boilerplate plugin.
[config] is an optional path to a configuration file specific to the boilerplate plugin.`,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current working directory: %w", err)
			}

			pluginName := args[0]
			name := args[1]
			configPath := args[2]

			var config []byte
			if configPath != "" {
				config, err = os.ReadFile(configPath)
				if err != nil {
					return fmt.Errorf("read config file: %w", err)
				}
			}

			pCtx, err := newPluginContext(cwd)
			if err != nil {
				return fmt.Errorf("create plugin context: %w", err)
			}
			defer contract.IgnoreClose(pCtx)
			log := func(sev diag.Severity, msg string) {
				pCtx.Diag.Logf(sev, diag.RawMessage("", msg))
			}

			boilerplate, err := loadBoilerplatePlugin(pCtx, pluginName, log)
			if err != nil {
				return fmt.Errorf("load converter plugin: %w", err)
			}
			defer contract.IgnoreClose(boilerplate)

			_, err = boilerplate.CreatePackage(pCtx.Request(), &plugin.CreatePackageRequest{
				Name:   name,
				Config: config,
			})
			if err != nil {
				return fmt.Errorf("create package: %w", err)
			}

			return nil
		}),
	}
	return cmd
}

func loadBoilerplatePlugin(
	ctx *plugin.Context,
	name string,
	log func(sev diag.Severity, msg string),
) (plugin.Boilerplate, error) {
	// Try and load the boilerplate plugin for this
	boilerplate, err := plugin.NewBoilerplate(ctx, name, nil)
	if err != nil {
		// If NewConverter returns a MissingError, we can try and install the plugin if it was missing and try again,
		// unless auto plugin installs are turned off.
		if env.DisableAutomaticPluginAcquisition.Value() {
			return nil, fmt.Errorf("load %q: %w", name, err)
		}

		var me *workspace.MissingError
		if !errors.As(err, &me) {
			// Not a MissingError, return the original error.
			return nil, fmt.Errorf("load %q: %w", name, err)
		}

		pluginSpec := workspace.PluginSpec{
			Kind: workspace.BoilerplatePlugin,
			Name: name,
		}

		_, err = pkgWorkspace.InstallPlugin(pluginSpec, log)
		if err != nil {
			return nil, fmt.Errorf("install %q: %w", name, err)
		}

		boilerplate, err = plugin.NewBoilerplate(ctx, name, nil)
		if err != nil {
			return nil, fmt.Errorf("load %q: %w", name, err)
		}
	}

	return boilerplate, nil
}
