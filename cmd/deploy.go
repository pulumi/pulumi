// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"
)

func newDeployCmd() *cobra.Command {
	var dryRun bool
	var showConfig bool
	var showReplaceSteps bool
	var showUnchanged bool
	var summary bool
	var output string
	var cmd = &cobra.Command{
		Use:     "deploy <env> [<package>] [-- [<args>]]",
		Aliases: []string{"up", "update"},
		Short:   "Deploy resource updates, creations, and deletions to an environment",
		Long: "Deploy resource updates, creations, and deletions to an environment\n" +
			"\n" +
			"This command updates an existing environment whose state is represented by the\n" +
			"existing snapshot file.  The new desired state is computed by compiling and evaluating an\n" +
			"executable package, and extracting all resource allocations from its resulting object graph.\n" +
			"This graph is compared against the existing state to determine what operations must take\n" +
			"place to achieve the desired state.  This command results in a full snapshot of the\n" +
			"environment's new resource state, so that it may be updated incrementally again later.\n" +
			"\n" +
			"By default, the package to execute is loaded from the current directory.  Optionally, an\n" +
			"explicit path can be provided using the [package] argument.",
		Run: runFunc(func(cmd *cobra.Command, args []string) error {
			info, err := initEnvCmd(cmd, args)
			if err != nil {
				return err
			}
			defer info.Close()
			apply(cmd, info, applyOptions{
				Delete:           false,
				DryRun:           dryRun,
				ShowConfig:       showConfig,
				ShowReplaceSteps: showReplaceSteps,
				ShowUnchanged:    showUnchanged,
				Summary:          summary,
				Output:           output,
			})
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually update resources; just print out the planned updates")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplaceSteps, "show-replace-steps", false,
		"Show detailed resource replacement creates and deletes; normally shows as a single step")
	cmd.PersistentFlags().BoolVar(
		&showUnchanged, "show-unchanged", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().StringVarP(
		&output, "output", "o", "",
		"Serialize the resulting checkpoint to a specific file, instead of overwriting the existing one")

	return cmd
}
