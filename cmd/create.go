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
		Use:   "create husk-name [nut-file] [-- [args]]",
		Short: "Create a new husk (target) with a given name and fresh resources",
		Long: "Create a new husk (target) with a given name and fresh resources.\n" +
			"\n" +
			"This command creates a new husk (target) and its resources, with the given name.  These\n" +
			"resources are computed by compiling and evaluating an executable Nut, and then extracting\n" +
			"resource allocations from its resulting object graph.  This command saves full snapshot\n" +
			"of the husk's final resource state, so that it may be updated incrementally later on.\n" +
			"\n" +
			"By default, the Nut to execute is loaded from the current directory.  Optionally, an\n" +
			"explicit path can be provided using the [nut-file] argument.",
		Run: func(cmd *cobra.Command, args []string) {
			apply(cmd, args, applyOptions{
				Create:  true,
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
		"Serialize the resulting husk snapshot to a specific file, instead of the standard location")

	return cmd
}
