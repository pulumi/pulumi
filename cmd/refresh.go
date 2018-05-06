// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newRefreshCmd() *cobra.Command {
	var debug bool
	var message string
	var stack string

	// Flags for engine.UpdateOptions.
	var analyzers []string
	var color colorFlag
	var diffDisplay bool
	var parallel int
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var nonInteractive bool
	var skipPreview bool
	var yes bool

	var cmd = &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the resources in a stack",
		Long: "Refresh the resources in a stack.\n" +
			"\n" +
			"This command compares the current stack's resource state with the state known to exist in\n" +
			"the actual cloud provider. Any such changes are adopted into the current stack. Note that if\n" +
			"the program text isn't updated accordingly, subsequent updates may still appear to be out of\n" +
			"synch with respect to the cloud provider's source of truth.\n" +
			"\n" +
			"The program to run is loaded from the project in the current directory. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			interactive := isInteractive(nonInteractive)
			if !interactive {
				yes = true // auto-approve changes, since we cannot prompt.
			}

			opts, err := updateFlagsToOptions(interactive, skipPreview, yes)
			if err != nil {
				return err
			}

			s, err := requireStack(stack, true)
			if err != nil {
				return err
			}

			proj, root, err := readProject()
			if err != nil {
				return err
			}

			m, err := getUpdateMetadata(message, root)
			if err != nil {
				return errors.Wrap(err, "gathering environment metadata")
			}

			opts.Engine = engine.UpdateOptions{
				Analyzers: analyzers,
				Parallel:  parallel,
				Debug:     debug,
			}
			opts.Display = backend.DisplayOptions{
				Color:                color.Colorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				IsInteractive:        interactive,
				DiffDisplay:          diffDisplay,
				Debug:                debug,
			}

			err = s.Refresh(proj, root, m, opts, cancellationScopes)
			if err == context.Canceled {
				return errors.New("refresh cancelled")
			}
			return err
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
		"Optional message to associate with the update operation")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", nil,
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
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&skipPreview, "skip-preview", false,
		"Do not perform a preview before performing the refresh")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Automatically approve and perform the refresh after previewing it")

	return cmd
}
