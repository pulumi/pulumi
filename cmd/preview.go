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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/result"
)

func newPreviewCmd() *cobra.Command {
	var debug bool
	var expectNop bool
	var message string
	var stack string

	// Flags for engine.UpdateOptions.
	var analyzers []string
	var diffDisplay bool
	var jsonDisplay bool
	var parallel int
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var suppressOutputs bool

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

			opts := backend.UpdateOptions{
				Engine: engine.UpdateOptions{
					Analyzers: analyzers,
					Parallel:  parallel,
					Debug:     debug,
				},
				Display: display.Options{
					Color:                cmdutil.GetGlobalColorization(),
					ShowConfig:           showConfig,
					ShowReplacementSteps: showReplacementSteps,
					ShowSameResources:    showSames,
					SuppressOutputs:      suppressOutputs,
					IsInteractive:        cmdutil.Interactive(),
					Type:                 displayType,
					JSONDisplay:          jsonDisplay,
					Debug:                debug,
				},
			}

			s, err := requireStack(stack, true, opts.Display, true /*setCurrent*/)
			if err != nil {
				return result.FromError(err)
			}

			proj, root, err := readProject()
			if err != nil {
				return result.FromError(err)
			}

			m, err := getUpdateMetadata("", root)
			if err != nil {
				return result.FromError(errors.Wrap(err, "gathering environment metadata"))
			}

			cfg, err := getStackConfiguration(s)
			if err != nil {
				return result.FromError(errors.Wrap(err, "getting stack configuration"))
			}

			sm, err := getStackSecretsManager(s)
			if err != nil {
				return result.FromError(errors.Wrap(err, "getting secrets manager"))
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

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the preview operation")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this update")
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.Flags().BoolVarP(
		&jsonDisplay, "json", "j", false,
		"Serialize the preview diffs, operations, and overall output as JSON")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", defaultParallel,
		"Allow P resource operations to run in parallel at once (1 for no parallelism). Defaults to unbounded.")
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
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")

	return cmd
}
