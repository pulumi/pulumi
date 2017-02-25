// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	var dryRun bool
	var summary bool
	var output string
	var cmd = &cobra.Command{
		Use:   "create [blueprint] [-- [args]]",
		Short: "Create a new environment and its resources",
		Long: "Create a new environment and its resources.\n" +
			"\n" +
			"This command creates a new environment and its resources.  These resources are\n" +
			"the result of compiling and evaluating a Nut blueprint, and then extracting all\n" +
			"resource allocations from its CocoGL graph.  This command results in a full snapshot\n" +
			"of the environment's resource state, so that it may be updated incrementally later on.\n" +
			"\n" +
			"By default, the Nut blueprint is loaded from the current directory.  Optionally,\n" +
			"a path to a Nut elsewhere can be provided as the [blueprint] argument.",
		Run: func(cmd *cobra.Command, args []string) {
			apply(cmd, args, "", applyOptions{
				Delete:  false,
				DryRun:  dryRun,
				Summary: summary,
				Output:  output,
			})
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually create resources; just print out the planned creations")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().StringVarP(
		&output, "output", "o", "",
		"Serialize the resulting snapshot to a specific file, instead of the standard location")

	return cmd
}
