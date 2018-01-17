// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newPreviewCmd() *cobra.Command {
	var debug bool
	var stack string

	var color string
	var opts engine.UpdateOptions

	var cmd = &cobra.Command{
		Use:        "preview",
		Aliases:    []string{"pre"},
		SuggestFor: []string{"build", "plan"},
		Short:      "Show a preview of updates to an stack's resources",
		Long: "Show a preview of updates an stack's resources\n" +
			"\n" +
			"This command displays a preview of the updates to an existing stack whose state is\n" +
			"represented by an existing snapshot file. The new desired state is computed by compiling\n" +
			"and evaluating an executable package, and extracting all resource allocations from its\n" +
			"resulting object graph. These allocations are then compared against the existing state to\n" +
			"determine what operations must take place to achieve the desired state. No changes to the\n" +
			"stack will actually take place.\n" +
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

			// The --color flag doesn't directly change the opts.Color value, so we parse and set it here.
			col, err := parseColorization(debug, color)
			if err != nil {
				return err
			}
			opts.Color = col

			return s.Preview(pkg, root, debug, opts)
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")

	// Flags for setting engine.UpdateOptions.
	cmd.PersistentFlags().StringVar(
		&color, "color", "auto",
		"Colorize output. Choices are: always, never, raw, auto")
	cmd.PersistentFlags().StringSliceVar(
		&opts.Analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this update")
	cmd.PersistentFlags().BoolVarP(
		&opts.DryRun, "dry-run", "r", false,
		"Don't create/delete resources; just preview the planned operations")
	cmd.PersistentFlags().IntVarP(
		&opts.Parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVar(
		&opts.ShowConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&opts.ShowReplacementSteps, "show-replacement-steps", true,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&opts.ShowSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&opts.Summary, "summary", false,
		"Only display summarization of resources and operations")

	return cmd
}
