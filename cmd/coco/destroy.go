// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/tokens"
)

func newDestroyCmd() *cobra.Command {
	var dryRun bool
	var env string
	var summary bool
	var yes bool
	var cmd = &cobra.Command{
		Use:   "destroy",
		Short: "Destroy an existing environment and its resources",
		Long: "Destroy an existing environment and its resources\n" +
			"\n" +
			"This command deletes an entire existing environment by name.  The current state is\n" +
			"loaded from the associated snapshot file in the workspace.  After running to completion,\n" +
			"all of this environment's resources and associated state will be gone.\n" +
			"\n" +
			"Warning: although old snapshots can be used to recreate an environment, this command\n" +
			"is generally irreversable and should be used with great care.",
		Run: runFunc(func(cmd *cobra.Command, args []string) error {
			info, err := initEnvCmdName(tokens.QName(env), args)
			if err != nil {
				return err
			}
			defer info.Close()
			if dryRun || yes ||
				confirmPrompt("This will permanently destroy all resources in the '%v' environment!", info.Env.Name) {
				apply(cmd, info, applyOptions{
					Delete:  true,
					DryRun:  dryRun,
					Summary: summary,
				})
			}
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually delete resources; just print out the planned deletions")
	cmd.PersistentFlags().StringVarP(
		&env, "env", "e", "",
		"Choose an environment other than the currently selected one")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with the destruction anyway")

	return cmd
}
