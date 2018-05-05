// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newPreviewCmd() *cobra.Command {
	var debug bool
	var message string
	var stack string

	// Flags for engine.UpdateOptions.
	var analyzers []string
	var color colorFlag
	var diffDisplay bool
	var nonInteractive bool
	var parallel int
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool

	var cmd = &cobra.Command{
		Use:        "preview",
		Aliases:    []string{"pre"},
		SuggestFor: []string{"build", "plan"},
		Short:      "Show a preview of updates to a stack's resources",
		Long: "Show a preview of updates a stack's resources.\n" +
			"\n" +
			"This command displays a preview of the updates to an existing stack whose state is\n" +
			"represented by an existing snapshot file. The new desired state is computed by running\n" +
			"a Pulumi program, and extracting all resource allocations from its resulting object graph.\n" +
			"These allocations are then compared against the existing state to determine what\n" +
			"operations must take place to achieve the desired state. No changes to the stack will\n" +
			"actually take place.\n" +
			"\n" +
			"The program to run is loaded from the project in the current directory. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			s, err := requireStack(stack, true)
			if err != nil {
				return err
			}

			proj, root, err := readProject()
			if err != nil {
				return err
			}

			m, err := getUpdateMetadata("", root)
			if err != nil {
				return errors.Wrap(err, "gathering environment metadata")
			}

			opts := backend.UpdateOptions{
				Engine: engine.UpdateOptions{
					Analyzers: analyzers,
					Parallel:  parallel,
					Debug:     debug,
				},
				Display: backend.DisplayOptions{
					Color:                color.Colorization(),
					ShowConfig:           showConfig,
					ShowReplacementSteps: showReplacementSteps,
					ShowSameResources:    showSames,
					IsInteractive:        isInteractive(nonInteractive),
					DiffDisplay:          diffDisplay,
					Debug:                debug,
				},
			}
			return s.Preview(proj, root, m, opts, cancellationScopes)
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose a stack other than the currently selected one")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the preview operation")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this update")
	cmd.PersistentFlags().VarP(
		&color, "color", "c", "Colorize output. Choices are: always, never, raw, auto")
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.PersistentFlags().BoolVar(
		&nonInteractive, "non-interactive", false, "Disable interactive mode")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")

	return cmd
}
