// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newUpdateCmd() *cobra.Command {
	var debug bool
	var stack string
	var opts engine.UpdateOptions

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

			return s.Update(pkg, root, debug, opts)
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"Choose an stack other than the currently selected one")

	// Flags for setting engine.UpdateOptions.
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

	// We use a custom flag type so that we can accept colorization options as a color.Colorization type.
	cf := colorFlag{
		Output: &opts.Color,
	}
	// Provide a default. Otherwise if no --color option is specified, the value will be "" which is an invalid
	// state for colors.Colorization.
	opts.Color = colors.Always
	cmd.PersistentFlags().Var(&cf, "color", "Colorize output. Choices are: always, never, raw, auto")

	return cmd
}
