// Copyright 2016-2018, Pulumi Corporation.
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

package cmd

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/result"
)

// intentionally disabling here for cleaner err declaration/assignment.
// nolint: vetshadow
func newWatchCmd() *cobra.Command {
	var debug bool
	var expectNop bool
	var message string
	var stack string
	var configArray []string

	// Flags for engine.UpdateOptions.
	var policyPackPaths []string
	var diffDisplay bool
	var parallel int
	var refresh bool
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var skipPreview bool
	var suppressOutputs bool
	var yes bool
	var secretsProvider string

	// up implementation used when the source of the Pulumi program is in the current working directory.
	upWorkingDirectory := func(opts backend.UpdateOptions) result.Result {
		s, err := requireStack(stack, true, opts.Display, true /*setCurrent*/)
		if err != nil {
			return result.FromError(err)
		}

		// Save any config values passed via flags.
		if len(configArray) > 0 {
			commandLineConfig, err := parseConfig(configArray)
			if err != nil {
				return result.FromError(err)
			}

			if err = saveConfig(s, commandLineConfig); err != nil {
				return result.FromError(errors.Wrap(err, "saving config"))
			}
		}

		proj, root, err := readProject()
		if err != nil {
			return result.FromError(err)
		}

		m, err := getUpdateMetadata(message, root)
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
			LocalPolicyPackPaths: policyPackPaths,
			Parallel:             parallel,
			Debug:                debug,
			Refresh:              refresh,
			UseLegacyDiff:        useLegacyDiff(),
		}

		res := s.Watch(commandContext(), backend.UpdateOperation{
			Proj:               proj,
			Root:               root,
			M:                  m,
			Opts:               opts,
			StackConfiguration: cfg,
			SecretsManager:     sm,
			Scopes:             cancellationScopes,
		})
		switch {
		case res != nil && res.Error() == context.Canceled:
			return result.FromError(errors.New("update cancelled"))
		case res != nil:
			return PrintEngineResult(res)
		default:
			return nil
		}
	}

	var cmd = &cobra.Command{
		Use:        "watch",
		Aliases:    []string{"watch"},
		SuggestFor: []string{"developer", "dev"},
		Short:      "Continuously create or update the resources in a stack",
		Long: "Create or update the resources in a stack.\n" +
			"\n" +
			"This command creates or updates resources in a stack. The new desired goal state for the target stack\n" +
			"is computed by running the current Pulumi program and observing all resource allocations to produce a\n" +
			"resource graph. This goal state is then compared against the existing state to determine what create,\n" +
			"read, update, and/or delete operations must take place to achieve the desired goal state, in the most\n" +
			"minimally disruptive way. This command records a full transactional snapshot of the stack's new state\n" +
			"afterwards so that the stack may be updated incrementally again later on.\n" +
			"\n" +
			"The program to run is loaded from the project in the current directory by default. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			interactive := cmdutil.Interactive()
			if !interactive {
				yes = true // auto-approve changes, since we cannot prompt.
			}

			opts, err := updateFlagsToOptions(interactive, skipPreview, yes)
			if err != nil {
				return result.FromError(err)
			}

			opts.Display = display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				SuppressOutputs:      suppressOutputs,
				IsInteractive:        interactive,
				Type:                 display.DisplayWatch,
				Debug:                debug,
			}

			return upWorkingDirectory(opts)
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVar(
		&expectNop, "expect-no-changes", false,
		"Return an error if any changes occur during this update")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	cmd.PersistentFlags().StringArrayVarP(
		&configArray, "config", "c", []string{},
		"Config to use during the update")
	cmd.PersistentFlags().StringVar(
		&secretsProvider, "secrets-provider", "default", "The type of the provider that should be used to encrypt and "+
			"decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only"+
			"used when creating a new stack from an existing template")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the update operation")

	// Flags for engine.UpdateOptions.
	if hasDebugCommands() {
		cmd.PersistentFlags().StringSliceVar(
			&policyPackPaths, "policy-pack", []string{},
			"Run one or more policy packs as part of this update")
	}
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", defaultParallel,
		"Allow P resource operations to run in parallel at once (1 for no parallelism). Defaults to unbounded.")
	cmd.PersistentFlags().BoolVarP(
		&refresh, "refresh", "r", false,
		"Refresh the state of the stack's resources before this update")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that don't need be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&skipPreview, "skip-preview", false,
		"Do not perform a preview before performing the update")
	cmd.PersistentFlags().BoolVar(
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically approve and perform the update after previewing it")

	return cmd
}
