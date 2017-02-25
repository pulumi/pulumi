// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var dryRun bool
	var summary bool
	var output string
	var cmd = &cobra.Command{
		Use:   "update husk-name [nut-file] [-- [args]]",
		Short: "Update an existing husk (target) and its resources",
		Long: "Update an existing husk (target) and its resources.\n" +
			"\n" +
			"This command updates an existing husk environment whose state is represented by the\n" +
			"existing snapshot file.  The new desired state is computed by compiling and evaluating\n" +
			"an executable Nut, and extracting all resource allocations from its resulting object graph.\n" +
			"This graph is compared against the existing state to determine what operations must take\n" +
			"place to achieve the desired state.  This command results in a full snapshot of the\n" +
			"environment's new resource state, so that it may be updated incrementally again later.\n" +
			"\n" +
			"By default, the Nut to execute is loaded from the current directory.  Optionally, an\n" +
			"explicit path can be provided using the [nut-file] argument.",
		Run: func(cmd *cobra.Command, args []string) {
			apply(cmd, args, applyOptions{
				Create:  false,
				Delete:  false,
				DryRun:  dryRun,
				Summary: summary,
				Output:  output,
			})
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually update resources; just print out the planned updates")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().StringVarP(
		&output, "output", "o", "",
		"Serialize the resulting husk snapshot to a specific file, instead of overwriting the existing one")

	return cmd
}
