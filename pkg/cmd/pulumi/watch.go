// Copyright 2016-2019, Pulumi Corporation.
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
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// intentionally disabling here for cleaner err declaration/assignment.
// nolint: vetshadow
func newWatchCmd() *cobra.Command {
	var debug bool
	var message string
	var execKind string
	var stack string
	var configArray []string
	var pathArray []string
	var configPath bool

	// Flags for engine.UpdateOptions.
	var policyPackPaths []string
	var policyPackConfigPaths []string
	var parallel int
	var refresh bool
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var secretsProvider string

	var cmd = &cobra.Command{
		Use:        "watch",
		SuggestFor: []string{"developer", "dev"},
		Short:      "[PREVIEW] Continuously update the resources in a stack",
		Long: "Continuously update the resources in a stack.\n" +
			"\n" +
			"This command watches the working directory or specified paths for the current project and updates\n" +
			"the active stack whenever the project changes.  In parallel, logs are collected for all resources\n" +
			"in the stack and displayed along with update progress.\n" +
			"\n" +
			"The program to watch is loaded from the project in the current directory by default. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {

			opts, err := updateFlagsToOptions(false /* interactive */, true /* skippreview*/, true /* autoapprove*/)
			if err != nil {
				return result.FromError(err)
			}

			opts.Display = display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				SuppressOutputs:      true,
				SuppressPermaLink:    true,
				IsInteractive:        false,
				Type:                 display.DisplayWatch,
				Debug:                debug,
			}

			if err := validatePolicyPackConfig(policyPackPaths, policyPackConfigPaths); err != nil {
				return result.FromError(err)
			}

			s, err := requireStack(stack, true, opts.Display, false /*setCurrent*/)
			if err != nil {
				return result.FromError(err)
			}

			// Save any config values passed via flags.
			if err := parseAndSaveConfigArray(s, configArray, configPath); err != nil {
				return result.FromError(err)
			}

			proj, root, err := readProject()
			if err != nil {
				return result.FromError(err)
			}

			m, err := getUpdateMetadata(message, root, execKind, "" /* execAgent */)
			if err != nil {
				return result.FromError(errors.Wrap(err, "gathering environment metadata"))
			}

			sm, err := getStackSecretsManager(s)
			if err != nil {
				return result.FromError(errors.Wrap(err, "getting secrets manager"))
			}

			cfg, err := getStackConfiguration(s, sm)
			if err != nil {
				return result.FromError(errors.Wrap(err, "getting stack configuration"))
			}

			opts.Engine = engine.UpdateOptions{
				LocalPolicyPacks:          engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigPaths),
				Parallel:                  parallel,
				Debug:                     debug,
				Refresh:                   refresh,
				UseLegacyDiff:             useLegacyDiff(),
				DisableProviderPreview:    disableProviderPreview(),
				DisableResourceReferences: disableResourceReferences(),
			}

			res := s.Watch(commandContext(), backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				M:                  m,
				Opts:               opts,
				StackConfiguration: cfg,
				SecretsManager:     sm,
				Scopes:             cancellationScopes,
			}, pathArray)

			switch {
			case res != nil && res.Error() == context.Canceled:
				return result.FromError(errors.New("update cancelled"))
			case res != nil:
				return PrintEngineResult(res)
			default:
				return nil
			}
		}),
	}

	cmd.PersistentFlags().StringArrayVarP(
		&pathArray, "path", "", []string{""},
		"Specify one or more relative or absolute paths that need to be watched. "+
			"A path can point to a folder or a file. Defaults to working directory")
	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	cmd.PersistentFlags().StringArrayVarP(
		&configArray, "config", "c", []string{},
		"Config to use during the update")
	cmd.PersistentFlags().BoolVar(
		&configPath, "config-path", false,
		"Config keys contain a path to a property in a map or list to set")
	cmd.PersistentFlags().StringVar(
		&secretsProvider, "secrets-provider", "default", "The type of the provider that should be used to encrypt and "+
			"decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only"+
			"used when creating a new stack from an existing template")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with each update operation")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().StringSliceVar(
		&policyPackPaths, "policy-pack", []string{},
		"Run one or more policy packs as part of each update")
	cmd.PersistentFlags().StringSliceVar(
		&policyPackConfigPaths, "policy-pack-config", []string{},
		`Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag`)
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", defaultParallel,
		"Allow P resource operations to run in parallel at once (1 for no parallelism). Defaults to unbounded.")
	cmd.PersistentFlags().BoolVarP(
		&refresh, "refresh", "r", false,
		"Refresh the state of the stack's resources before each update")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that don't need be updated because they haven't changed, alongside those that do")

	cmd.PersistentFlags().StringVar(&execKind, "exec-kind", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")

	return cmd
}
