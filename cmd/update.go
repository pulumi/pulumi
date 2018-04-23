// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newUpdateCmd() *cobra.Command {
	var debug bool
	var message string
	var stack string

	// Flags for engine.UpdateOptions.
	var analyzers []string
	var color colorFlag
	var diffDisplay bool
	var parallel int
	var preview bool
	var force bool
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool

	var cmd = &cobra.Command{
		Use:        "update",
		Aliases:    []string{"up"},
		SuggestFor: []string{"deploy", "push"},
		Short:      "Update the resources in a stack",
		Long: "Update the resources in a stack.\n" +
			"\n" +
			"This command updates an existing stack whose state is represented by the existing checkpoint\n" +
			"file. The new desired state is computed by running a Pulumi program, and extracting all resource\n" +
			"allocations from its resulting object graph. These allocations are then compared against the\n" +
			"existing state to determine what operations must take place to achieve the desired state. This\n" +
			"command results in a checkpoint containing a full snapshot of the stack's new resource state, so\n" +
			"that it may be updated incrementally again later.\n" +
			"\n" +
			"The program to run is loaded from the project in the current directory. Use the `-C` or\n" +
			"`--cwd` flag to use a different directory.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if !force && !preview && !terminal.IsTerminal(int(os.Stdout.Fd())) {
				return errors.New("'update' must be run interactively or be passed the --force or --preview flag")
			}

			if force && preview {
				return errors.New("--force and --preview cannot both be specified")
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

			err = s.Update(proj, root, m, engine.UpdateOptions{
				Analyzers: analyzers,
				Force:     force,
				Preview:   preview,
				Parallel:  parallel,
				Debug:     debug,
			}, backend.DisplayOptions{
				Color:                color.Colorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				DiffDisplay:          diffDisplay,
				Debug:                debug,
			}, cancellationScopes)
			if err == context.Canceled {
				return errors.New("update cancelled")
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
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this update")
	cmd.PersistentFlags().VarP(
		&color, "color", "c", "Colorize output. Choices are: always, never, raw, auto")
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Skip confirmation prompts and preview, and proceed with the update automatically")
	cmd.PersistentFlags().BoolVar(
		&preview, "preview", false,
		"Only show a preview of what will happen, without prompting or making any changes")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that don't need be updated because they haven't changed, alongside those that do")

	return cmd
}
