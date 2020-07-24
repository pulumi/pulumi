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

package main

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/pkg/v2/backend/display"
	"github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

// intentionally disabling here for cleaner err declaration/assignment.
// nolint: vetshadow
func newHostCmd() *cobra.Command {
	var debug bool
	var expectNop bool
	var message string
	var stack string
	var configArray []string
	var path bool

	// Flags for engine.UpdateOptions.
	var policyPackPaths []string
	var policyPackConfigPaths []string
	var diffDisplay bool
	var eventLogPath string
	var parallel int
	var refresh bool
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var showReads bool
	var suppressOutputs bool
	var yes bool
	var secretsProvider string
	var targets []string
	var replaces []string
	var targetReplaces []string
	var targetDependents bool

	var cmd = &cobra.Command{
		Use:   "host",
		Short: "Launch the engine in 'host' mode without launching a language runtime",
		Long: "Launch the engine in 'host' mode without launching a language runtime.\n" +
			"\n" +
			"To support some automation scenarios, the engine may be launched in this mode to prevent it from\n" +
			"needing to spawn its own language runtime. In this case, the lifetime of interactions between the\n" +
			"language and the engine host must be managed manually.",
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {

			// Construct the options based on flags.
			opts, err := updateFlagsToOptions(false, true, true)
			if err != nil {
				return result.FromError(err)
			}

			opts.Display = display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				ShowReads:            showReads,
				SuppressOutputs:      suppressOutputs,
				IsInteractive:        false,
				Type:                 display.DisplayDiff,
				EventLogPath:         eventLogPath,
				Debug:                debug,
			}

			opts.Engine = engine.UpdateOptions{
				Parallel:      parallel,
				Debug:         debug,
				Refresh:       refresh,
				UseLegacyDiff: useLegacyDiff(),
			}

			// we set up the stack in auto.NewStack before execing host
			s, err := requireStack(stack, false /*offerNew*/, opts.Display, false /*setCurrent*/)
			if err != nil {
				return result.FromError(err)
			}

			proj, root, err := readProject()
			if err != nil {
				return result.FromError(err)
			}
			proj.Runtime = workspace.NewProjectRuntimeInfo("host", nil)

			sm, err := getStackSecretsManager(s)
			if err != nil {
				return result.FromError(errors.Wrap(err, "getting secrets manager"))
			}

			cfg, err := getStackConfiguration(s, sm)
			if err != nil {
				return result.FromError(errors.Wrap(err, "getting stack configuration"))
			}

			// Now perform the update. This will stay alive until the user cancels the host.
			changes, res := s.Update(commandContext(), backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				Opts:               opts,
				M:                  &backend.UpdateMetadata{}, // TODO: let this be passed in.
				StackConfiguration: cfg,
				SecretsManager:     sm,
				Scopes:             cancellationScopesWithoutInterrupt,
			})
			switch {
			case res != nil && res.Error() == context.Canceled:
				return result.FromError(errors.New("update cancelled"))
			case res != nil:
				return PrintEngineResult(res)
			case expectNop && changes != nil && changes.HasChanges():
				return result.FromError(errors.New("error: no changes were expected but changes occurred"))
			default:
				return nil
			}
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
	cmd.PersistentFlags().BoolVar(
		&path, "config-path", false,
		"Config keys contain a path to a property in a map or list to set")
	cmd.PersistentFlags().StringVar(
		&secretsProvider, "secrets-provider", "default", "The type of the provider that should be used to encrypt and "+
			"decrypt secrets (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault). Only"+
			"used when creating a new stack from an existing template")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the update operation")

	cmd.PersistentFlags().StringArrayVarP(
		&targets, "target", "t", []string{},
		"Specify a single resource URN to update. Other resources will not be updated."+
			" Multiple resources can be specified using --target urn1 --target urn2")
	cmd.PersistentFlags().StringArrayVar(
		&replaces, "replace", []string{},
		"Specify resources to replace. Multiple resources can be specified using --replace run1 --replace urn2")
	cmd.PersistentFlags().StringArrayVar(
		&targetReplaces, "target-replace", []string{},
		"Specify a single resource URN to replace. Other resources will not be updated."+
			" Shorthand for --target urn --replace urn.")
	cmd.PersistentFlags().BoolVar(
		&targetDependents, "target-dependents", false,
		"Allows updating of dependent targets discovered but not specified in --target list")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().StringSliceVar(
		&policyPackPaths, "policy-pack", []string{},
		"[PREVIEW] Run one or more policy packs as part of this update")
	cmd.PersistentFlags().StringSliceVar(
		&policyPackConfigPaths, "policy-pack-config", []string{},
		`[PREVIEW] Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag`)
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
		&showReads, "show-reads", false,
		"Show resources that are being read in, alongside those being managed directly in the stack")

	cmd.PersistentFlags().BoolVar(
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically approve and perform the update after previewing it")

	if hasDebugCommands() {
		cmd.PersistentFlags().StringVar(
			&eventLogPath, "event-log", "",
			"Log events to a file at this path")
	}
	return cmd
}
