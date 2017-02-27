// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/tokens"
)

func newHuskDeployCmd() *cobra.Command {
	var dryRun bool
	var showUnchanged bool
	var summary bool
	var output string
	var cmd = &cobra.Command{
		Use:   "deploy <husk> [<nut>] [-- [<args>]]",
		Short: "Deploy resource updates, creations, and deletions to a husk",
		Long: "Deploy resource updates, creations, and deletions to a husk\n" +
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
			// Read in the name of the husk to use.
			if len(args) == 0 {
				fmt.Fprintf(os.Stderr, "fatal: missing required husk name\n")
				os.Exit(-1)
			}

			husk := tokens.QName(args[0])
			apply(cmd, args[1:], husk, applyOptions{
				Delete:        false,
				DryRun:        dryRun,
				ShowUnchanged: showUnchanged,
				Summary:       summary,
				Output:        output,
			})
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually update resources; just print out the planned updates")
	cmd.PersistentFlags().BoolVar(
		&showUnchanged, "show-unchanged", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().StringVarP(
		&output, "output", "o", "",
		"Serialize the resulting husk snapshot to a specific file, instead of overwriting the existing one")

	return cmd
}
