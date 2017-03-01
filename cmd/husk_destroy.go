// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
)

func newHuskDestroyCmd() *cobra.Command {
	var dryRun bool
	var summary bool
	var yes bool
	var cmd = &cobra.Command{
		Use:   "destroy <husk>",
		Short: "Destroy an existing husk and its resources",
		Long: "Destroy an existing husk and its resources\n" +
			"\n" +
			"This command deletes an entire existing husk by name.  The current state is loaded\n" +
			"from the associated snapshot file in the workspace.  After running to completion,\n" +
			"all of this husk's resources and associated state will be gone.\n" +
			"\n" +
			"Warning: although old snapshots can be used to recreate an environment, this command\n" +
			"is generally irreversable and should be used with great care.",
		Run: func(cmd *cobra.Command, args []string) {
			info := initHuskCmd(cmd, args)
			if dryRun || yes ||
				confirmPrompt("This will permanently destroy all resources in the '%v' husk!", info.Husk.Name) {
				apply(cmd, info, applyOptions{
					Delete:  true,
					DryRun:  dryRun,
					Summary: summary,
				})
			}
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually delete resources; just print out the planned deletions")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with the destruction anyway")

	return cmd
}
