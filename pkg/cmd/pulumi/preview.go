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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func newPreviewCmd() *cobra.Command {
	var debug bool
	var expectNop bool
	var message string
	var execKind string
	var execAgent string
	var stack string
	var configArray []string
	var configPath bool
	var client string

	// Flags for engine.UpdateOptions.
	var jsonDisplay bool
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
	var suppressPermaLink string
	var targets []string
	var replaces []string
	var targetReplaces []string
	var targetDependents bool

	var cmd = &cobra.Command{
		Use:        "preview",
		Aliases:    []string{"pre"},
		SuggestFor: []string{"build", "plan"},
		Short:      "Show a preview of updates to a stack's resources",
		Long: "Show a preview of updates a stack's resources.\n" +
			"\n" +
			"This command displays a preview of the updates to an existing stack whose state is\n" +
			"represented by an existing state file. The new desired state is computed by running\n" +
			"a Pulumi program, and extracting all resource allocations from its resulting object graph.\n" +
			"These allocations are then compared against the existing state to determine what\n" +
			"operations must take place to achieve the desired state. No changes to the stack will\n" +
			"actually take place.\n" +
			"\n" +
			"The program to run is loaded from the project in the current directory. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			var displayType = display.DisplayProgress
			if diffDisplay {
				displayType = display.DisplayDiff
			}

			displayOpts := display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				ShowReads:            showReads,
				SuppressOutputs:      suppressOutputs,
				IsInteractive:        cmdutil.Interactive(),
				Type:                 displayType,
				JSONDisplay:          jsonDisplay,
				EventLogPath:         eventLogPath,
				Debug:                debug,
			}

			// we only suppress permalinks if the user passes true. the default is an empty string
			// which we pass as 'false'
			if suppressPermaLink == "true" {
				displayOpts.SuppressPermaLink = true
			} else {
				displayOpts.SuppressPermaLink = false
			}
			filestateBackend, err := isFilestateBackend(displayOpts)
			if err != nil {
				return result.FromError(err)
			}

			// by default, we are going to suppress the permalink when using self-managed backends
			// this can be re-enabled by explicitly passing "false" to the `supppress-permalink` flag
			if suppressPermaLink != "false" && filestateBackend {
				displayOpts.SuppressPermaLink = true
			}

			if err := validatePolicyPackConfig(policyPackPaths, policyPackConfigPaths); err != nil {
				return result.FromError(err)
			}

			s, err := requireStack(stack, true, displayOpts, false /*setCurrent*/)
			if err != nil {
				return result.FromError(err)
			}

			// Save any config values passed via flags.
			if err = parseAndSaveConfigArray(s, configArray, configPath); err != nil {
				return result.FromError(err)
			}

			proj, root, err := readProjectForUpdate(client)
			if err != nil {
				return result.FromError(err)
			}

			m, err := getUpdateMetadata(message, root, execKind, execAgent)
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

			targetURNs := []resource.URN{}
			for _, t := range targets {
				targetURNs = append(targetURNs, resource.URN(t))
			}

			replaceURNs := []resource.URN{}
			for _, r := range replaces {
				replaceURNs = append(replaceURNs, resource.URN(r))
			}

			for _, tr := range targetReplaces {
				targetURNs = append(targetURNs, resource.URN(tr))
				replaceURNs = append(replaceURNs, resource.URN(tr))
			}

			opts := backend.UpdateOptions{
				Engine: engine.UpdateOptions{
					LocalPolicyPacks:          engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigPaths),
					Parallel:                  parallel,
					Debug:                     debug,
					Refresh:                   refresh,
					ReplaceTargets:            replaceURNs,
					UseLegacyDiff:             useLegacyDiff(),
					DisableProviderPreview:    disableProviderPreview(),
					DisableResourceReferences: disableResourceReferences(),
					UpdateTargets:             targetURNs,
					TargetDependents:          targetDependents,
				},
				Display: displayOpts,
			}

			changes, res := s.Preview(commandContext(), backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				M:                  m,
				Opts:               opts,
				StackConfiguration: cfg,
				SecretsManager:     sm,
				Scopes:             cancellationScopes,
			})

			switch {
			case res != nil:
				return PrintEngineResult(res)
			case expectNop && changes != nil && changes.HasChanges():
				return result.FromError(errors.New("error: no changes were expected but changes were proposed"))
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
		"Return an error if any changes are proposed by this preview")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")
	cmd.PersistentFlags().StringArrayVarP(
		&configArray, "config", "c", []string{},
		"Config to use during the preview")
	cmd.PersistentFlags().BoolVar(
		&configPath, "config-path", false,
		"Config keys contain a path to a property in a map or list to set")

	cmd.PersistentFlags().StringVar(
		&client, "client", "", "The address of an existing language runtime host to connect to")
	_ = cmd.PersistentFlags().MarkHidden("client")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the preview operation")

	cmd.PersistentFlags().StringArrayVarP(
		&targets, "target", "t", []string{},
		"Specify a single resource URN to update. Other resources will not be updated."+
			" Multiple resources can be specified using --target urn1 --target urn2")
	cmd.PersistentFlags().StringArrayVar(
		&replaces, "replace", []string{},
		"Specify resources to replace. Multiple resources can be specified using --replace urn1 --replace urn2")
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
		"Run one or more policy packs as part of this update")
	cmd.PersistentFlags().StringSliceVar(
		&policyPackConfigPaths, "policy-pack-config", []string{},
		`Path to JSON file containing the config for the policy pack of the corresponding "--policy-pack" flag`)
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.Flags().BoolVarP(
		&jsonDisplay, "json", "j", false,
		"Serialize the preview diffs, operations, and overall output as JSON")
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
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&showReads, "show-reads", false,
		"Show resources that are being read in, alongside those being managed directly in the stack")
	cmd.PersistentFlags().BoolVar(
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")

	cmd.PersistentFlags().StringVar(
		&suppressPermaLink, "suppress-permalink", "",
		"Suppress display of the state permalink")
	cmd.Flag("suppress-permalink").NoOptDefVal = "false"

	if hasDebugCommands() {
		cmd.PersistentFlags().StringVar(
			&eventLogPath, "event-log", "",
			"Log events to a file at this path")
	}

	// internal flags
	cmd.PersistentFlags().StringVar(&execKind, "exec-kind", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")
	cmd.PersistentFlags().StringVar(&execAgent, "exec-agent", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-agent")

	return cmd
}
