// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	var dryRun bool
	var summary bool
	var cmd = &cobra.Command{
		Use:   "delete husk-name",
		Short: "Delete an existing husk (target) and its resources",
		Long: "Delete an existing husk (target) and its resources.\n" +
			"\n" +
			"This command deletes an entire existing husk by name.  The current state is loaded\n" +
			"from the associated snapshot file in the workspace.  After running to completion,\n" +
			"this environment and all of its associated state will be gone.\n" +
			"\n" +
			"Warning: although old snapshots can be used to recreate an environment, this command\n" +
			"is generally irreversable and should be used with great care.",
		Run: func(cmd *cobra.Command, args []string) {
			apply(cmd, args, applyOptions{
				Create:  false,
				Delete:  true,
				DryRun:  dryRun,
				Summary: summary,
			})
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually delete resources; just print out the planned deletions")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")

	return cmd
}
