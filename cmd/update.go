// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newUpdateCmd() *cobra.Command {
	var debug bool
	var stack string

	var message string

	// Flags for engine.UpdateOptions.
	var analyzers []string
	var color colorFlag
	var parallel int
	var preview bool
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var summary bool

	var cmd = &cobra.Command{
		Use:        "update",
		Aliases:    []string{"up"},
		SuggestFor: []string{"deploy", "push"},
		Short:      "Update the resources in an stack",
		Long: "Update the resources in an stack\n" +
			"\n" +
			"This command updates an existing stack whose state is represented by the\n" +
			"existing snapshot file. The new desired state is computed by compiling and evaluating an\n" +
			"executable package, and extracting all resource allocations from its resulting object graph.\n" +
			"These allocations are then compared against the existing state to determine what operations\n" +
			"must take place to achieve the desired state. This command results in a full snapshot of the\n" +
			"stack's new resource state, so that it may be updated incrementally again later.\n" +
			"\n" +
			"The package to execute is loaded from the current directory. Use the `-C` or `--cwd` flag to\n" +
			"use a different directory.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			s, err := requireStack(tokens.QName(stack))
			if err != nil {
				return err
			}
			pkg, root, err := readPackage()
			if err != nil {
				return err
			}

			m, err := getUpdateMetadata(message, root)
			if err != nil {
				return errors.Wrap(err, "gathering environment metadata")
			}

			return s.Update(pkg, root, debug, m, engine.UpdateOptions{
				Analyzers:            analyzers,
				DryRun:               preview,
				Parallel:             parallel,
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSames:            showSames,
				Summary:              summary,
			}, backend.DisplayOptions{
				Color: color.Colorization(),
			})
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")

	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the update operation")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().VarP(
		&color, "color", "c", "Colorize output. Choices are: always, never, raw, auto")
	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this update")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVarP(
		&preview, "preview", "n", false,
		"Don't create/delete resources; just preview the planned operations")
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
		&summary, "summary", false,
		"Only display summarization of resources and operations")

	return cmd
}
