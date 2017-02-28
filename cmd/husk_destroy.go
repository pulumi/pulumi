// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"bufio"
	"fmt"
	"os"

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
			"this environment and all of its associated state will be gone.\n" +
			"\n" +
			"Warning: although old snapshots can be used to recreate an environment, this command\n" +
			"is generally irreversable and should be used with great care.",
		Run: func(cmd *cobra.Command, args []string) {
			if info := initHuskCmd(cmd, args); info != nil {
				if !dryRun && !yes {
					fmt.Printf("This will permanently delete all resources in the '%v' husk!\n", info.Husk.Name)
					fmt.Printf("Please confirm that this is what you'd like to do by typing (\"yes\"): ")
					reader := bufio.NewReader(os.Stdin)
					if line, _ := reader.ReadString('\n'); line != "yes\n" {
						fmt.Fprintf(os.Stderr, "Confirmation declined -- exiting without destroying the husk\n")
						os.Exit(-1)
					}
				}

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
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with operations anyway")

	return cmd
}
