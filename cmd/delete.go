// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	var detailed bool
	var dryRun bool
	var cmd = &cobra.Command{
		Use:   "delete [snapshot]",
		Short: "Delete an existing environment and its resources",
		Long: "Delete an existing environment and its resources.\n" +
			"\n" +
			"This command deletes an entire existing environment whose state is represented by the\n" +
			"existing snapshot file.  After running to completion, this environment will be gone.",
		Run: func(cmd *cobra.Command, args []string) {
			applyExisting(cmd, args, applyOptions{
				Delete:   true,
				Detailed: detailed,
				DryRun:   dryRun,
			})
		},
	}

	cmd.PersistentFlags().BoolVar(
		&detailed, "detailed", false,
		"Display detailed output during the application of changes")
	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually delete resources; just print out the planned deletions")

	return cmd
}
